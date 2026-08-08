"""読みの解決。

形態素解析は読みを必ず返すとは限らない。数字・ラテン文字・未知の固有名詞などで
読みが空になる。その場合の補完と、補完できない場合の記録を行う
（詳細設計 01 §4）。

**読めない語があるとき `hacho` を返してはならない。**
読めなかっただけの句に「五七五になっていません」と伝えるのは誤りである。
利用者は正しく詠んでいるかもしれず、そう言われても直しようがない。
`unknown` として、読めなかった語を列挙して返す。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Final

from prosody import numbers
from prosody.mora import MoraCounter, is_ignorable
from prosody.token import PartOfSpeech, Token

# ラテン文字1文字ずつの読み。`AI` → `エーアイ`。
_LATIN_READINGS: Final = {
    "A": "エー",
    "B": "ビー",
    "C": "シー",
    "D": "ディー",
    "E": "イー",
    "F": "エフ",
    "G": "ジー",
    "H": "エイチ",
    "I": "アイ",
    "J": "ジェー",
    "K": "ケー",
    "L": "エル",
    "M": "エム",
    "N": "エヌ",
    "O": "オー",
    "P": "ピー",
    "Q": "キュー",
    "R": "アール",
    "S": "エス",
    "T": "ティー",
    "U": "ユー",
    "V": "ブイ",
    "W": "ダブリュー",
    "X": "エックス",
    "Y": "ワイ",
    "Z": "ゼット",
}


@dataclass(frozen=True)
class ResolvedToken:
    """読みとモーラ数が確定した形態素。"""

    surface: str
    reading: str
    """カタカナの読み。記号など0モーラのものは空文字列。"""
    mora: int
    pos: PartOfSpeech
    unit_boundary: bool


@dataclass(frozen=True)
class ResolvedReading:
    """本文全体の読み解決の結果。"""

    tokens: list[ResolvedToken] = field(default_factory=list)
    unreadable: list[str] = field(default_factory=list)
    """読みを確定できなかった語の表記。空でなければ判定は `unknown` になる。"""

    @property
    def reading(self) -> str:
        """全体の読み。"""
        return "".join(token.reading for token in self.tokens)

    @property
    def total_mora(self) -> int:
        """全体のモーラ数。"""
        return sum(token.mora for token in self.tokens)

    @property
    def is_readable(self) -> bool:
        """すべての語の読みを確定できたか。"""
        return not self.unreadable


def read_latin(surface: str) -> str:
    """ラテン文字をアルファベット読みにする。

    読めない文字が1つでもあれば空文字列を返す。
    「一部だけ読めた」状態を作らないためである。
    """
    readings = []
    for ch in surface.upper():
        reading = _LATIN_READINGS.get(ch)
        if reading is None:
            return ""
        readings.append(reading)
    return "".join(readings)


class ReadingResolver:
    """形態素の列を受け取り、読みとモーラ数を確定させる。

    形態素解析器の存在を知らない。`Token` にだけ依存する。
    """

    def __init__(self, counter: MoraCounter | None = None) -> None:
        self._counter = counter or MoraCounter()

    def resolve(self, tokens: list[Token]) -> ResolvedReading:
        """読みを解決する。読めない語があれば `unreadable` に記録する。"""
        resolved: list[ResolvedToken] = []
        unreadable: list[str] = []

        for token in tokens:
            reading = self._resolve_one(token)
            if reading is None:
                unreadable.append(token.surface)
                continue
            resolved.append(
                ResolvedToken(
                    surface=token.surface,
                    reading=reading,
                    mora=self._counter.count(reading),
                    pos=token.pos,
                    unit_boundary=token.unit_boundary,
                )
            )

        return ResolvedReading(tokens=resolved, unreadable=unreadable)

    def _resolve_one(self, token: Token) -> str | None:
        """1つの形態素の読みを決める。読めない場合は None を返す。

        優先順位は詳細設計 01 §4 のとおり。
        """
        surface = token.surface

        # 空の表層は 0 モーラとして扱う。異常な入力でクラッシュさせない。
        if not surface:
            return ""

        # 記号・空白 → 0モーラ（規則 R6）。
        #
        # **この判定を解析器の読みより先に行う。** SudachiPy は記号や空白にも
        # 読みを付けるためである。実測では次のような読みが返る。
        #
        #     " " → "キゴウ"   ※ → "キゴウ"   ♪ → "キゴウ"
        #
        # 読みを優先すると空白1つが3モーラになり、
        # 明示的な区切りに空白を使う仕様（詳細設計 01 §7）が成立しなくなる。
        if all(is_ignorable(ch) for ch in surface):
            return ""

        # 解析器が読みを返したならそれを使う
        if token.reading:
            return token.reading

        # 数字 → 数値読み
        if surface.isdecimal():
            return numbers.read(surface)

        # ラテン文字 → アルファベット読み
        latin = read_latin(surface)
        if latin:
            return latin

        # ここまで来たものは読めない語として記録する
        return None
