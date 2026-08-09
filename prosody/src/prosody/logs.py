"""構造化ログ（詳細設計 03 §3）。

1行1レコードの JSON として標準出力に書く。ファイルには書かない。
コンテナのローカルディスクに書いたログは Pod の再起動で消えるためである。

**`message` で検索させない。** `event` を別に持つことで、
文言を変えてもログの集計が壊れないようにする。

**リクエストボディを記録しない。** 投稿本文が含まれるためである
（詳細設計 03「ログに出してはいけないもの」）。
"""

from __future__ import annotations

import contextvars
import datetime
import json
import logging
import sys
from typing import Any, Final

SERVICE_NAME: Final = "prosody"

# サービスをまたいで1つの操作を追跡するための ID（基本設計 01 §7）。
# ミドルウェアが設定し、以降のログ出力すべてに自動で付く。
request_id_var: contextvars.ContextVar[str | None] = contextvars.ContextVar(
    "request_id", default=None
)

# LogRecord が標準で持つ属性。これ以外を追加フィールドとして扱う。
_STANDARD_ATTRS: Final = frozenset(
    logging.LogRecord("", 0, "", 0, "", None, None).__dict__.keys()
) | {"message", "asctime", "taskName"}


def _timestamp(created: float) -> str:
    """ミリ秒まで、タイムゾーン付きの ISO 8601 で返す。"""
    moment = datetime.datetime.fromtimestamp(created, tz=datetime.UTC).astimezone()
    return moment.isoformat(timespec="milliseconds")


class JsonFormatter(logging.Formatter):
    """LogRecord を JSON の1行にする。"""

    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            # logging.Formatter.formatTime は time.strftime を使うため
            # マイクロ秒（%f）を解釈せず、文字列 "%f" がそのまま残る。
            # 詳細設計 03 はミリ秒までを必須としているため、自前で組み立てる。
            "timestamp": _timestamp(record.created),
            "level": record.levelname,
            "service": SERVICE_NAME,
            "request_id": request_id_var.get(),
            "event": getattr(record, "event", record.name),
            "message": record.getMessage(),
        }

        # logger.info("...", extra={"key": value}) で渡された値を取り込む
        for key, value in record.__dict__.items():
            if key not in _STANDARD_ATTRS and key != "event":
                payload[key] = value

        if record.exc_info:
            # 想定外のエラーは発生箇所を特定できる情報を残す（NFR-05-02）
            payload["stacktrace"] = self.formatException(record.exc_info)

        return json.dumps(payload, ensure_ascii=False)


def configure(level: str = "info") -> None:
    """ルートロガーを構造化ログに切り替える。

    uvicorn が既定で入れるハンドラを置き換えるため、既存のものは取り除く。
    """
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JsonFormatter())

    root = logging.getLogger()
    for existing in list(root.handlers):
        root.removeHandler(existing)
    root.addHandler(handler)
    root.setLevel(level.upper())

    # uvicorn は独自のハンドラを持つため、ルートへ委譲させる
    for name in ("uvicorn", "uvicorn.error"):
        uvicorn_logger = logging.getLogger(name)
        uvicorn_logger.handlers.clear()
        uvicorn_logger.propagate = True

    # アクセスログは自前のミドルウェアが request_id 付きで出す。
    # uvicorn 側のアクセスログは request_id を持たず、1リクエストにつき
    # 同じ内容が2行になるだけなので止める。
    access = logging.getLogger("uvicorn.access")
    access.handlers.clear()
    access.propagate = False
    access.disabled = True
