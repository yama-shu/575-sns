"""prosody の HTTP エントリポイント。

判定 API（`POST /v1/analyze`）とヘルスチェックを公開する。

**認証を持たない。** クラスタ内部からのみ到達可能とし、
外部からの到達は NetworkPolicy で塞ぐ（基本設計 01 §6）。

**破調はエラーではない。** 判定を求められて判定を返しているため、
破調の本文に対しても HTTP 200 を返す。「破調だから投稿を拒否する」のは
575 の業務ルールであり api の責務である。
"""

from __future__ import annotations

import logging
import secrets
import time
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field

from prosody import logs
from prosody.analyzer import AnalysisResult, ProsodyAnalyzer
from prosody.config import Settings
from prosody.tokenizer import Tokenizer
from prosody.verdict import Reason, Verdict

logger = logging.getLogger("prosody")

settings = Settings.from_env()

REQUEST_ID_HEADER = "X-Request-ID"


def _load_tokenizer(dict_type: str) -> Tokenizer:
    """形態素解析器を組み立てる（辞書をメモリへ展開する）。

    詳細設計 02 §6 のとおり **遅延初期化しない**。
    起動時に同期的にロードすることで、複数リクエストによる二重ロードと、
    そのためのロックのオーバーヘッドを構造的に発生させない。

    SudachiPy の import をこの関数の中に置くのは、辞書の展開が重く、
    起動時にのみ読み込みたいためである。
    """
    from prosody.sudachi import SudachiTokenizer

    return SudachiTokenizer(dict_type=dict_type)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    logs.configure(settings.log_level)

    started = time.perf_counter()
    try:
        tokenizer = _load_tokenizer(settings.sudachi_dict)
    except Exception:
        # 半端に起動して全リクエストを失敗させ続けるより、起動しない方がよい（詳細設計 02 §6）
        logger.critical(
            "辞書のロードに失敗しました。プロセスを終了します。",
            extra={"event": "dictionary_load_failed"},
            exc_info=True,
        )
        raise
    elapsed_ms = (time.perf_counter() - started) * 1000

    app.state.tokenizer = tokenizer
    app.state.analyzer = ProsodyAnalyzer(tokenizer)
    app.state.dictionary_load_ms = elapsed_ms
    logger.info(
        "辞書のロードが完了しました",
        extra={
            "event": "dictionary_loaded",
            "dictionary": settings.sudachi_dict,
            "elapsed_ms": round(elapsed_ms, 1),
        },
    )

    yield

    app.state.tokenizer = None
    app.state.analyzer = None


app = FastAPI(
    title="prosody",
    description="575 の五七五判定エンジン。音数律の判定のみを行い、業務ルールを持たない。",
    version="0.1.0",
    lifespan=lifespan,
)


@app.middleware("http")
async def request_context(
    request: Request, call_next: Callable[[Request], Awaitable[Response]]
) -> Response:
    """リクエスト ID を引き継ぎ、アクセスログを出す（基本設計 01 §7）。

    上流（Ingress → web → api）が付けた ID をそのまま使う。無ければ生成する。
    これが無いと、3サービスに分散したログから1つの操作を再構成することが
    事実上不可能になる。
    """
    request_id = request.headers.get(REQUEST_ID_HEADER) or secrets.token_hex(16)
    token = logs.request_id_var.set(request_id)
    started = time.perf_counter()
    try:
        response = await call_next(request)
        response.headers[REQUEST_ID_HEADER] = request_id
        logger.info(
            "リクエストを処理しました",
            extra={
                "event": "http_request",
                "method": request.method,
                "path": request.url.path,
                "status": response.status_code,
                "duration_ms": round((time.perf_counter() - started) * 1000, 1),
            },
        )
        return response
    finally:
        logs.request_id_var.reset(token)


# ----------------------------------------------------------------------
# スキーマ
# ----------------------------------------------------------------------


class AnalyzeRequest(BaseModel):
    """判定の要求。"""

    text: str = Field(
        description="判定する本文。正規化はサーバ側で行う。",
        examples=["今日もまた会議のための会議かな"],
    )


class SegmentResponse(BaseModel):
    """区切られた1つの句。"""

    text: str = Field(description="正規化後の本文のうち、この句にあたる部分")
    reading: str = Field(description="この句の読み")
    mora: int = Field(description="この句のモーラ数")
    expected: int = Field(description="期待されるモーラ数（5 / 7 / 5）")
    diff: int = Field(description="mora - expected。0 なら規定どおり")


class AnalyzeResponse(BaseModel):
    """判定の結果。

    `verdict` が `hacho` / `unknown` のとき `segments` は `null` になる。
    五七五に区切れないため、区切りが定義できないためである。
    """

    verdict: Verdict = Field(description="判定")
    normalized_text: str = Field(
        description=(
            "正規化した本文。区切り位置はこの文字列上の位置であり、"
            "api はこちらを保存する。元の入力を保存すると位置がずれる。"
        )
    )
    reading: str | None = Field(default=None, description="全体の読み")
    total_mora: int | None = Field(default=None, description="全体のモーラ数")
    segments: list[SegmentResponse] | None = Field(default=None, description="上五・中七・下五")
    reason: Reason | None = Field(default=None, description="hacho / unknown のときの理由")
    unreadable: list[str] | None = Field(
        default=None, description="unknown のとき、読みを確定できなかった語"
    )


def to_response(result: AnalysisResult) -> AnalyzeResponse:
    """判定結果を API のレスポンスへ変換する。"""
    return AnalyzeResponse(
        verdict=result.verdict,
        normalized_text=result.normalized_text,
        reading=result.reading,
        total_mora=result.total_mora,
        segments=(
            None
            if result.segments is None
            else [
                SegmentResponse(
                    text=s.text, reading=s.reading, mora=s.mora, expected=s.expected, diff=s.diff
                )
                for s in result.segments
            ]
        ),
        reason=result.reason,
        unreadable=result.unreadable,
    )


# ----------------------------------------------------------------------
# エンドポイント
# ----------------------------------------------------------------------


@app.post("/v1/analyze", tags=["prosody"], summary="五七五かどうかを判定する")
def analyze(request: AnalyzeRequest, http_request: Request) -> AnalyzeResponse:
    """本文を判定し、区切りと各句のモーラ数を返す。

    **破調でも 200 を返す。** 判定を求められて判定を返しているためである。
    """
    analyzer: ProsodyAnalyzer = http_request.app.state.analyzer
    result = analyzer.analyze(request.text)

    logger.info(
        "判定しました",
        extra={
            "event": "analyzed",
            "verdict": result.verdict.value,
            "reason": result.reason.value if result.reason else None,
            "total_mora": result.total_mora,
            # 本文は記録しない（投稿本文が含まれるため）。長さのみ残す。
            "text_length": len(result.normalized_text),
        },
    )
    return to_response(result)


@app.get("/healthz", tags=["health"], summary="プロセスが生きているか")
def healthz() -> dict[str, str]:
    """liveness probe。プロセスが生きているかだけを返す。"""
    return {"status": "ok"}


@app.get("/readyz", tags=["health"], summary="判定を受け付けられるか")
def readyz() -> JSONResponse:
    """readiness probe。**辞書のロードが完了しているか**を返す。

    これを実装しないと、Pod の再起動やスケールアウトのたびに
    ロード中の Pod へトラフィックが流れてタイムアウトする（基本設計 05 §4）。
    """
    loaded = getattr(app.state, "analyzer", None) is not None
    body: dict[str, object] = {"ready": loaded, "dictionary_loaded": loaded}
    if loaded:
        body["dictionary_load_ms"] = round(app.state.dictionary_load_ms, 1)
    return JSONResponse(content=body, status_code=200 if loaded else 503)
