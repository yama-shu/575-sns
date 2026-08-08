"""prosody の HTTP エントリポイント。

本モジュールは開発環境構築（#2）時点の骨組みである。
判定 API（POST /v1/analyze）は #8 で実装する。ここでは
「辞書がロードでき、疎通できる」ことまでを担保する。
"""

from __future__ import annotations

import logging
import time
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI
from fastapi.responses import JSONResponse

from prosody.config import Settings

logger = logging.getLogger("prosody")

settings = Settings.from_env()


def _load_dictionary(dict_type: str) -> Any:
    """形態素解析辞書をメモリへ展開する。

    詳細設計 02 §6 のとおり **遅延初期化しない**。
    起動時に同期的にロードすることで、複数リクエストによる二重ロードと、
    そのためのロックのオーバーヘッドを構造的に発生させない。
    """
    from sudachipy import Dictionary  # 起動時にのみ import する（辞書の展開が重いため）

    return Dictionary(dict=dict_type)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    started = time.perf_counter()
    try:
        app.state.dictionary = _load_dictionary(settings.sudachi_dict)
    except Exception:
        # 半端に起動して全リクエストを失敗させ続けるより、起動しない方がよい（詳細設計 02 §6）
        logger.critical("辞書のロードに失敗しました。プロセスを終了します。", exc_info=True)
        raise
    elapsed_ms = (time.perf_counter() - started) * 1000
    logger.info(
        "辞書のロードが完了しました dict=%s elapsed_ms=%.1f", settings.sudachi_dict, elapsed_ms
    )
    app.state.dictionary_load_ms = elapsed_ms

    yield

    app.state.dictionary = None


app = FastAPI(
    title="prosody",
    description="575 の五七五判定エンジン。音数律の判定のみを行い、業務ルールを持たない。",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/healthz", tags=["health"])
def healthz() -> dict[str, str]:
    """liveness probe。プロセスが生きているかだけを返す。"""
    return {"status": "ok"}


@app.get("/readyz", tags=["health"])
def readyz() -> JSONResponse:
    """readiness probe。**辞書のロードが完了しているか**を返す。

    これを実装しないと、Pod の再起動やスケールアウトのたびに
    ロード中の Pod へトラフィックが流れてタイムアウトする（基本設計 05 §4）。
    """
    loaded = getattr(app.state, "dictionary", None) is not None
    body: dict[str, object] = {"ready": loaded, "dictionary_loaded": loaded}
    if loaded:
        body["dictionary_load_ms"] = round(app.state.dictionary_load_ms, 1)
    return JSONResponse(content=body, status_code=200 if loaded else 503)
