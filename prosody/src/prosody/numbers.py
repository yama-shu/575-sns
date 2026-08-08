"""数値の読み変換。

形態素解析器は数字に読みを付けないことがある。その場合にここで補う
（詳細設計 01 §4）。

    2024 → ニセンニジュウヨン

音便（サンビャク・ロッピャク・ハッセン等）を含む。これを無視すると
モーラ数がずれるため、判定結果に直接影響する。

**助数詞の連濁には対応しない**（`1本` → `イッポン` ではなく `イチホン`）。
詳細設計 01 が既知の誤差 L3 として受け入れている範囲である。
"""

from __future__ import annotations

from typing import Final

ZERO: Final = "ゼロ"

_ONES: Final = {
    1: "イチ",
    2: "ニ",
    3: "サン",
    4: "ヨン",
    5: "ゴ",
    6: "ロク",
    7: "ナナ",
    8: "ハチ",
    9: "キュウ",
}

# 十・百・千は音便で読みが変わる。規則で導かず表で持つ。
# 「サンビャク」「ロッピャク」「ハッセン」を取りこぼすとモーラ数がずれる。
_TENS: Final = {
    1: "ジュウ",
    2: "ニジュウ",
    3: "サンジュウ",
    4: "ヨンジュウ",
    5: "ゴジュウ",
    6: "ロクジュウ",
    7: "ナナジュウ",
    8: "ハチジュウ",
    9: "キュウジュウ",
}

_HUNDREDS: Final = {
    1: "ヒャク",
    2: "ニヒャク",
    3: "サンビャク",
    4: "ヨンヒャク",
    5: "ゴヒャク",
    6: "ロッピャク",
    7: "ナナヒャク",
    8: "ハッピャク",
    9: "キュウヒャク",
}

_THOUSANDS: Final = {
    1: "セン",
    2: "ニセン",
    3: "サンゼン",
    4: "ヨンセン",
    5: "ゴセン",
    6: "ロクセン",
    7: "ナナセン",
    8: "ハッセン",
    9: "キュウセン",
}

# 4桁ごとの単位。扱える上限は 9999兆（16桁）まで。
_GROUP_UNITS: Final = ("", "マン", "オク", "チョウ")

# 「チョウ」の直前で起きる音便。1兆 = イッチョウ、8兆 = ハッチョウ、10兆 = ジュッチョウ。
_SOKUON_BEFORE_CHO: Final = {"イチ": "イッ", "ハチ": "ハッ", "ジュウ": "ジュッ"}

_MAX_SUPPORTED: Final = 10 ** (4 * len(_GROUP_UNITS)) - 1


def _read_group(value: int) -> str:
    """1〜9999 の読みを返す。"""
    parts: list[str] = []
    thousands, rest = divmod(value, 1000)
    hundreds, rest = divmod(rest, 100)
    tens, ones = divmod(rest, 10)

    if thousands:
        parts.append(_THOUSANDS[thousands])
    if hundreds:
        parts.append(_HUNDREDS[hundreds])
    if tens:
        parts.append(_TENS[tens])
    if ones:
        parts.append(_ONES[ones])
    return "".join(parts)


def _apply_sokuon(group_reading: str, unit: str) -> str:
    """単位の直前で起きる音便を適用する。"""
    if unit != "チョウ":
        return group_reading
    for tail, replacement in _SOKUON_BEFORE_CHO.items():
        if group_reading.endswith(tail):
            return group_reading[: -len(tail)] + replacement
    return group_reading


def read_integer(value: int) -> str:
    """非負整数の読みを返す。

    扱える範囲を超える場合は空文字列を返す。呼び出し側で
    1桁ずつの読み（`read_digits`）へ切り替える。
    """
    if value == 0:
        return ZERO
    if value < 0 or value > _MAX_SUPPORTED:
        return ""

    # 下位の4桁ずつに分けて、単位を付けながら上位から並べる。
    # 上限を確認済みのため、単位の数を超えることはない。
    groups: list[tuple[int, str]] = []
    remaining = value
    index = 0
    while remaining:
        remaining, group = divmod(remaining, 10000)
        if group:
            groups.append((group, _GROUP_UNITS[index]))
        index += 1

    return "".join(
        _apply_sokuon(_read_group(group), unit) + unit for group, unit in reversed(groups)
    )


def read_digits(digits: str) -> str:
    """1桁ずつ読む。

    先頭が 0 の数字列（`007` など）や、扱える範囲を超える桁数で使う。
    番号として読まれることが多いためである。
    """
    return "".join(ZERO if ch == "0" else _ONES[int(ch)] for ch in digits)


def read(digits: str) -> str:
    """数字だけからなる文字列の読みを返す。

    先頭が 0 の複数桁は1桁ずつ読む。`007` を「ナナ」と読むのは不自然なため。
    """
    if not digits or not digits.isdecimal():
        return ""

    if len(digits) > 1 and digits.startswith("0"):
        return read_digits(digits)

    reading = read_integer(int(digits))
    return reading if reading else read_digits(digits)
