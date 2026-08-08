"""本文の正規化。

形態素解析にかける前に表記のゆれを吸収する（詳細設計 01 §2）。

| 処理 | 内容 | 理由 |
| --- | --- | --- |
| Unicode 正規化 | NFKC | 全角英数を半角に、半角カナを全角に統一する |
| 改行の統一 | `\\r\\n` / `\\r` → `\\n` | 環境による差異を吸収する |
| 連続する空白の圧縮 | 連続する空白を1つに | 明示的な区切りの判定で誤検出しないため |
| 前後の空白の除去 | trim | |

**ひらがなをカタカナに変換しない。** 本文の表記はそのまま保つ。
カタカナ化は読みに対してのみ行う。本文を書き換えると、
投稿として表示するときに入力と異なるものが出てしまう。
"""

from __future__ import annotations

import re
import unicodedata
from typing import Final

# 連続する空白（改行・タブ・全角空白を含む）
_WHITESPACE_RUN: Final = re.compile(r"\s+")


def normalize(text: str) -> str:
    """本文を正規化する。"""
    # NFKC。全角英数 → 半角、半角カナ → 全角。
    # 全角空白（U+3000）も半角空白に変換される。
    normalized = unicodedata.normalize("NFKC", text)

    # 改行コードを統一する
    normalized = normalized.replace("\r\n", "\n").replace("\r", "\n")

    # 連続する空白を半角空白1つに圧縮する。
    # 改行もここで空白になる。利用者が改行で句を区切った場合も、
    # 空白による明示的な区切り（詳細設計 01 §7）として扱えるようにするため。
    normalized = _WHITESPACE_RUN.sub(" ", normalized)

    return normalized.strip()
