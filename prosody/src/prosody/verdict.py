"""判定の種別。

要件定義 3.1 の3値（定型 / 許容 / 破調）に、読みを確定できない場合の
`UNKNOWN` を加えた4値とする。`UNKNOWN` を `HACHO` に含めないのは、
読めなかっただけの句に「五七五になっていません」と返すと、
正しく詠んだ利用者が直しようがなくなるためである（詳細設計 01 §4）。
"""

from __future__ import annotations

from enum import StrEnum


class Verdict(StrEnum):
    """判定結果。値は API のレスポンスでそのまま使う（基本設計 05）。"""

    TEIKEI = "teikei"
    """定型。上五5 / 中七7 / 下五5 に過不足なく収まっている。"""

    KYOYO = "kyoyo"
    """許容。字余り・字足らずが許容範囲に収まっている。"""

    HACHO = "hacho"
    """破調。許容範囲を超えている。投稿できない。"""

    UNKNOWN = "unknown"
    """読みを確定できず判定できない。投稿できない。"""

    def is_postable(self) -> bool:
        """この判定で投稿できるか。

        「破調なら拒否する」の判断自体は api の責務だが、
        投稿可能な判定がどれかは音数律の側の定義であるためここに置く。
        """
        return self in (Verdict.TEIKEI, Verdict.KYOYO)


class Reason(StrEnum):
    """`hacho` / `unknown` のときに、なぜそう判定したかを示す。

    値は API のレスポンスでそのまま使う（基本設計 05）。
    利用者への案内文を出し分けるために必要で、
    「五七五になっていません」だけでは直しようがない。
    """

    TOO_FEW_MORA = "TOO_FEW_MORA"
    """総モーラ数が少なすぎる。"""

    TOO_MANY_MORA = "TOO_MANY_MORA"
    """総モーラ数が多すぎる。"""

    NO_VALID_SPLIT = "NO_VALID_SPLIT"
    """モーラ数は範囲内だが、許容範囲に収まる区切りが見つからない。"""

    READING_UNAVAILABLE = "READING_UNAVAILABLE"
    """読みを取得できない語が含まれる。"""
