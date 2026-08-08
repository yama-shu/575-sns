"""形態素解析の結果を表す型。

**この型は特定の形態素解析器を知らない。** SudachiPy 固有の型を
上位のモジュールへ漏らさないための境界であり、ADR-0003 で決めた
「解析器を差し替え可能にする」を型のレベルで実現するものである。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class SplitMode(StrEnum):
    """分割の粒度（ADR-0003）。

    Sudachi が A / B / C の3段階を提供することを選定理由に挙げた。
    575 が使うのは C単位（自然な区切りを優先）と A単位（候補を増やす）の2つで、
    B単位は使わないが、解析器の能力として定義には含める。
    """

    A = "A"
    """最小の単位。選挙 / 管理 / 委員 / 会"""

    B = "B"
    """中間の単位。"""

    C = "C"
    """最大の単位。選挙管理委員会"""


class PartOfSpeech(StrEnum):
    """品詞。

    区切りコスト（詳細設計 01 §8）が必要とする粒度に絞る。
    解析器ごとの細かい体系をそのまま持ち込まず、ここで正規化する。
    """

    NOUN = "名詞"
    VERB = "動詞"
    ADJECTIVE = "形容詞"
    ADVERB = "副詞"
    ADNOMINAL = "連体詞"
    CONJUNCTION = "接続詞"
    INTERJECTION = "感動詞"
    PARTICLE = "助詞"
    AUXILIARY_VERB = "助動詞"
    PREFIX = "接頭辞"
    SUFFIX = "接尾辞"
    PUNCTUATION = "補助記号"
    """句読点。区切りとして最も自然な位置（コスト 0）。"""

    OTHER = "その他"
    """上記のいずれにも当てはまらないもの。"""


@dataclass(frozen=True)
class Token:
    """1つの形態素。"""

    surface: str
    """表記。本文中に現れるとおりの文字列。"""

    reading: str | None
    """カタカナの読み。解析器が読みを返さなかった場合は None。

    None は「読めない」と確定した状態ではない。数字やラテン文字は
    ReadingResolver が補完する（詳細設計 01 §4）。
    """

    pos: PartOfSpeech
    """品詞。"""

    unit_boundary: bool = True
    """この形態素の先頭が C単位の境界と一致するか。

    A単位で再探索するとき、C単位を分断する位置にペナルティを与えるために使う
    （詳細設計 01 §9）。C単位で解析した場合は常に True になる。
    """
