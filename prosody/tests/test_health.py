"""ヘルスチェックのテスト。

判定ロジックのテスト（TC-MC / TC-TR / TC-SS / TC-RR / TC-AC）は
詳細設計 04 にもとづき #4〜#8 で追加する。
本ファイルは開発環境が組み上がったことを確認する最小のテストである。
"""

import pytest
from fastapi.testclient import TestClient

from prosody import main
from prosody.main import app


def test_healthz_はプロセスが生きていれば200を返す() -> None:
    with TestClient(app) as client:
        response = client.get("/healthz")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_readyz_は辞書のロード完了後に200を返す() -> None:
    # TestClient を with で使うと lifespan が実行され、辞書がロードされる
    with TestClient(app) as client:
        response = client.get("/readyz")

    assert response.status_code == 200
    body = response.json()
    assert body["ready"] is True
    assert body["dictionary_loaded"] is True


def test_readyz_は辞書がロードされていなければ503を返す() -> None:
    # lifespan を実行せずに呼ぶ（＝辞書ロード前の状態を再現する）
    client = TestClient(app)
    response = client.get("/readyz")

    assert response.status_code == 503
    assert response.json() == {"ready": False, "dictionary_loaded": False}


def test_辞書のロードに失敗したら起動せずに例外を送出する(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """半端に起動して全リクエストを失敗させ続けるより、起動しない方がよい（詳細設計 02 §6）。

    例外を握りつぶすと、辞書が無いまま起動して 500 を返し続ける状態になる。
    """

    def 失敗する(_dict_type: str) -> object:
        raise RuntimeError("辞書ファイルが見つかりません")

    monkeypatch.setattr(main, "_load_tokenizer", 失敗する)

    # lifespan の実行中に落ちるため、with の本体には到達しない
    with pytest.raises(RuntimeError, match="辞書ファイルが見つかりません"), TestClient(app):
        pass
