"""判定 API のテスト（基本設計 05 §4）。

HTTP の入出力と、レスポンスの形を確かめる。判定そのものの正しさは
`test_analyzer.py` で確かめているため、ここでは重複させない。
"""

import json
import logging

import pytest
from fastapi.testclient import TestClient

from prosody import logs
from prosody.main import REQUEST_ID_HEADER, app


@pytest.fixture(scope="module")
def client() -> TestClient:
    """with で使うと lifespan が実行され、辞書がロードされる。"""
    with TestClient(app) as test_client:
        yield test_client


def test_定型の本文を判定できる(client: TestClient) -> None:
    response = client.post("/v1/analyze", json={"text": "今日もまた会議のための会議かな"})

    assert response.status_code == 200
    body = response.json()
    assert body["verdict"] == "teikei"
    assert body["total_mora"] == 17
    assert body["reason"] is None
    assert [s["mora"] for s in body["segments"]] == [5, 7, 5]
    assert [s["text"] for s in body["segments"]] == ["今日もまた", "会議のための", "会議かな"]


def test_破調でも200を返す(client: TestClient) -> None:
    """**破調はエラーではない。**

    判定を求められて判定を返しているため、リクエストは成功している。
    「破調だから投稿を拒否する」のは 575 の業務ルールであり api の責務である。
    """
    response = client.post("/v1/analyze", json={"text": "今日は疲れた"})

    assert response.status_code == 200
    body = response.json()
    assert body["verdict"] == "hacho"
    assert body["reason"] == "TOO_FEW_MORA"
    assert body["segments"] is None


def test_必須項目が欠けていれば422を返す(client: TestClient) -> None:
    """FastAPI が型ヒントから生成するバリデーション。"""
    response = client.post("/v1/analyze", json={})

    assert response.status_code == 422


def test_リクエストIDを引き継ぐ(client: TestClient) -> None:
    """上流が付けた ID をそのまま返す（基本設計 01 §7）。"""
    response = client.post(
        "/v1/analyze",
        json={"text": "今日もまた会議のための会議かな"},
        headers={REQUEST_ID_HEADER: "test-request-id"},
    )

    assert response.headers[REQUEST_ID_HEADER] == "test-request-id"


def test_リクエストIDが無ければ生成する(client: TestClient) -> None:
    response = client.post("/v1/analyze", json={"text": "あ"})

    assert response.headers.get(REQUEST_ID_HEADER)


def test_healthz(client: TestClient) -> None:
    response = client.get("/healthz")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_readyz(client: TestClient) -> None:
    response = client.get("/readyz")

    assert response.status_code == 200
    body = response.json()
    assert body["ready"] is True
    assert body["dictionary_loaded"] is True


def test_readyz_は辞書のロード前は503を返す() -> None:
    """辞書ロード前の状態を再現する。

    `app` は単一のインスタンスであり、他のテストが lifespan を実行して
    解析器を設定済みのことがある。状態を退避してから確かめる。
    """
    saved = getattr(app.state, "analyzer", None)
    app.state.analyzer = None
    try:
        response = TestClient(app).get("/readyz")
    finally:
        app.state.analyzer = saved

    assert response.status_code == 503
    assert response.json() == {"ready": False, "dictionary_loaded": False}


def test_辞書のロードに失敗したら起動せずに例外を送出する(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """半端に起動して全リクエストを失敗させ続けるより、起動しない方がよい。"""
    from prosody import main

    def 失敗する(_dict_type: str) -> object:
        raise RuntimeError("辞書ファイルが見つかりません")

    monkeypatch.setattr(main, "_load_tokenizer", 失敗する)

    with pytest.raises(RuntimeError, match="辞書ファイルが見つかりません"), TestClient(app):
        pass


# ----------------------------------------------------------------------
# 構造化ログ（詳細設計 03 §3）
# ----------------------------------------------------------------------


def test_ログはJSONで必須フィールドを含む(caplog: pytest.LogCaptureFixture) -> None:
    formatter = logs.JsonFormatter()
    token = logs.request_id_var.set("abc123")
    try:
        record = logging.LogRecord(
            name="prosody",
            level=logging.INFO,
            pathname="",
            lineno=0,
            msg="判定しました",
            args=None,
            exc_info=None,
        )
        record.event = "analyzed"
        record.verdict = "teikei"
        payload = json.loads(formatter.format(record))
    finally:
        logs.request_id_var.reset(token)

    # 詳細設計 03 が必須としているフィールド
    for field in ("timestamp", "level", "service", "request_id", "event", "message"):
        assert field in payload, f"{field} が無い"

    assert payload["service"] == "prosody"
    assert payload["request_id"] == "abc123"
    assert payload["event"] == "analyzed"
    assert payload["verdict"] == "teikei", "extra で渡した値が取り込まれる"


def test_例外を伴うログはスタックトレースを含む() -> None:
    """想定外のエラーは発生箇所を特定できる情報を残す（NFR-05-02）。"""
    formatter = logs.JsonFormatter()
    try:
        raise ValueError("試験用")
    except ValueError:
        import sys

        record = logging.LogRecord(
            name="prosody",
            level=logging.CRITICAL,
            pathname="",
            lineno=0,
            msg="失敗しました",
            args=None,
            exc_info=sys.exc_info(),
        )
    payload = json.loads(formatter.format(record))

    assert "stacktrace" in payload
    assert "ValueError" in payload["stacktrace"]


def test_ログ設定はuvicornのハンドラを置き換える() -> None:
    logs.configure("debug")

    root = logging.getLogger()
    assert len(root.handlers) == 1
    assert isinstance(root.handlers[0].formatter, logs.JsonFormatter)
    # アクセスログは自前のミドルウェアが request_id 付きで出すため、uvicorn 側は止める
    assert logging.getLogger("uvicorn.access").disabled is True


# ----------------------------------------------------------------------
# OpenAPI 定義
# ----------------------------------------------------------------------


def test_OpenAPI定義がリポジトリの内容と一致する() -> None:
    """`scripts/openapi.sh` が書き出した定義が最新であることを確かめる。

    api（Go）と web（TypeScript）はこの定義を契約として型を生成するため、
    ずれると実行時まで気づけない（ADR-0002）。
    """
    from pathlib import Path

    committed = Path(__file__).resolve().parent.parent / "openapi.json"
    assert committed.exists(), "openapi.json が無い。./scripts/openapi.sh を実行する"

    expected = json.dumps(app.openapi(), ensure_ascii=False, indent=2, sort_keys=True)

    assert committed.read_text(encoding="utf-8").rstrip("\n") == expected.rstrip("\n"), (
        "openapi.json が最新ではない。./scripts/openapi.sh を実行して差分をコミットする"
    )


def test_タイムスタンプはミリ秒とタイムゾーンを含む() -> None:
    """logging.Formatter.formatTime は %f を解釈せず "%f" が残る。

    詳細設計 03 はミリ秒までを必須としているため、自前で組み立てている。
    """
    import re

    payload = json.loads(
        logs.JsonFormatter().format(
            logging.LogRecord("prosody", logging.INFO, "", 0, "確認", None, None)
        )
    )

    assert re.fullmatch(
        r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{2}:\d{2}", payload["timestamp"]
    ), payload["timestamp"]
