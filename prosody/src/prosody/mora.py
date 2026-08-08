"""モーラ分割。

判定エンジンの最下層。カタカナの読みを受け取り、モーラの列に分割する。
ここが間違っていると、上に載るすべての判定が狂う。

**「文字数を数える」実装との違いが最も出る部分である。**

    シュッチョウ（6文字）→ シュ / ッ / チョ / ウ = 4モーラ

規則は詳細設計 01 §3 の R1〜R6 に対応する。

| 規則 | 内容 |
| --- | --- |
| R1 | 原則としてカタカナ1文字を1モーラとする |
| R2 | 小書きの母音・拗音は、直前の文字と結合して1モーラとする |
| R3 | 促音「ッ」は単独で1モーラとする |
| R4 | 撥音「ン」は単独で1モーラとする |
| R5 | 長音「ー」は単独で1モーラとする |
| R6 | 記号・空白は0モーラとする |
"""

from __future__ import annotations

import unicodedata
from typing import Final

# R2: 直前のモーラと結合する小書き文字。
COMBINING: Final = frozenset("ァィゥェォャュョヮ")

# R3・R4・R5: 単独で1モーラを構成する文字。
#
# **この3つを COMBINING に含めてはならない。**
# 「ッ」は見た目が小さいため拗音（ャュョ）と同じ扱いにしてしまいやすく、
# そうするとモーラ数が1つ少なくなる。詳細設計 01 が
# 「最も起きやすいバグ」と指摘しているのがこれである。
# 集合として明示的に定義し、テスト（TC-MC-15〜17）で固定する。
INDEPENDENT: Final = frozenset("ッンー")

# R6: 0モーラとして無視する Unicode の一般カテゴリ。
#
#   P* 句読点・記号     … 「、」「。」「!」
#   S* 記号             … 絵文字・数学記号・通貨記号
#   Z* 区切り           … 空白
#   C* 制御・書式       … 制御文字・異体字セレクタ
_IGNORABLE_CATEGORIES: Final = ("P", "S", "Z", "C")


def is_ignorable(ch: str) -> bool:
    """0モーラとして無視する文字か（R6）。"""
    return unicodedata.category(ch).startswith(_IGNORABLE_CATEGORIES)


class MoraCounter:
    """読み（カタカナ）をモーラの列に分割し、数える。

    状態を持たない。形態素解析器の存在を一切知らないため、
    辞書のロードなしにテストできる（詳細設計 02）。
    """

    def split(self, reading: str) -> list[str]:
        """読みをモーラの列に分割する。"""
        morae: list[str] = []
        for ch in reading:
            if ch in COMBINING and morae:
                # 直前のモーラに結合する（R2）
                morae[-1] += ch
            elif is_ignorable(ch):
                # 記号・空白は0モーラ（R6）
                continue
            else:
                # 結合対象が先頭に来た場合もここに落ちる。
                # 正常な読みでは起こらないが、異常な入力でクラッシュさせない。
                morae.append(ch)
        return morae

    def count(self, reading: str) -> int:
        """読みのモーラ数を返す。"""
        return len(self.split(reading))
