"""prosody の実行時設定。

設定値をコードに埋め込まず、すべて環境変数から読み込む。
既定値はローカル開発で動く値とし、本番では compose / Kubernetes から上書きする。
"""

from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    """環境変数から組み立てる設定。生成後は変更しない。"""

    host: str
    port: int
    log_level: str
    sudachi_dict: str

    @classmethod
    def from_env(cls) -> Settings:
        return cls(
            host=os.getenv("PROSODY_HOST", "0.0.0.0"),
            port=int(os.getenv("PROSODY_PORT", "8000")),
            log_level=os.getenv("PROSODY_LOG_LEVEL", "info"),
            # ADR-0003 で採用した SudachiDict-core を既定とする
            sudachi_dict=os.getenv("PROSODY_SUDACHI_DICT", "core"),
        )
