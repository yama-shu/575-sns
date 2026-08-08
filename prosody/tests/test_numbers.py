"""数値読み変換のテスト（詳細設計 01 §4）。

音便（サンビャク・ロッピャク・ハッセン等）を取りこぼすとモーラ数がずれ、
判定結果が変わる。桁ごとの境界と音便を網羅する。
"""

import pytest

from prosody import numbers
from prosody.mora import MoraCounter

counter = MoraCounter()

BASIC_CASES = [
    pytest.param("0", "ゼロ", id="TC_RR_05"),  # 0
    pytest.param("1", "イチ", id="TC_RR_02"),  # 1
    pytest.param("10", "ジュウ", id="TC_RR_03"),  # 10
    pytest.param("2024", "ニセンニジュウヨン", id="TC_RR_04"),  # 2024
]


@pytest.mark.parametrize(("digits", "expected"), BASIC_CASES)
def test_設計書の例(digits: str, expected: str) -> None:
    assert numbers.read(digits) == expected


DIGIT_CASES = [
    pytest.param("2", "ニ"),
    pytest.param("3", "サン"),
    pytest.param("4", "ヨン"),
    pytest.param("5", "ゴ"),
    pytest.param("6", "ロク"),
    pytest.param("7", "ナナ"),
    pytest.param("8", "ハチ"),
    pytest.param("9", "キュウ"),
]


@pytest.mark.parametrize(("digits", "expected"), DIGIT_CASES)
def test_一桁(digits: str, expected: str) -> None:
    assert numbers.read(digits) == expected


ONBIN_CASES = [
    # 百の音便
    pytest.param("100", "ヒャク", id="100_イチを付けない"),
    pytest.param("300", "サンビャク", id="300_連濁"),
    pytest.param("600", "ロッピャク", id="600_促音と半濁音"),
    pytest.param("800", "ハッピャク", id="800_促音と半濁音"),
    pytest.param("200", "ニヒャク", id="200_音便なし"),
    # 千の音便
    pytest.param("1000", "セン", id="1000_イチを付けない"),
    pytest.param("3000", "サンゼン", id="3000_連濁"),
    pytest.param("8000", "ハッセン", id="8000_促音"),
    pytest.param("2000", "ニセン", id="2000_音便なし"),
    # 十
    pytest.param("11", "ジュウイチ", id="11"),
    pytest.param("20", "ニジュウ", id="20"),
    pytest.param("99", "キュウジュウキュウ", id="99"),
    # 万・億・兆
    pytest.param("10000", "イチマン", id="1万_イチを付ける"),
    pytest.param("12345", "イチマンニセンサンビャクヨンジュウゴ", id="12345"),
    pytest.param("100000000", "イチオク", id="1億"),
    pytest.param("1000000000000", "イッチョウ", id="1兆_促音便"),
    pytest.param("8000000000000", "ハッチョウ", id="8兆_促音便"),
    pytest.param("10000000000000", "ジュッチョウ", id="10兆_促音便"),
    pytest.param("2000000000000", "ニチョウ", id="2兆_音便なし"),
]


@pytest.mark.parametrize(("digits", "expected"), ONBIN_CASES)
def test_音便と桁(digits: str, expected: str) -> None:
    assert numbers.read(digits) == expected


def test_先頭が0の複数桁は一桁ずつ読む() -> None:
    """`007` を「ナナ」と読むのは不自然なため、番号として読む。"""
    assert numbers.read("007") == "ゼロゼロナナ"
    assert numbers.read("0120") == "ゼロイチニゼロ"


def test_扱える桁を超えたら一桁ずつ読む() -> None:
    """京（17桁）以上。クラッシュさせず、読める形に落とす。"""
    huge = "1" + "0" * 16
    assert numbers.read(huge) == "イチ" + "ゼロ" * 16


def test_数字でない文字列は空を返す() -> None:
    assert numbers.read("") == ""
    assert numbers.read("あ") == ""
    assert numbers.read("12a") == ""


def test_read_integer_は負の数を扱わない() -> None:
    assert numbers.read_integer(-1) == ""


# ----------------------------------------------------------------------
# 設計書のモーラ数との突き合わせ
# ----------------------------------------------------------------------


def test_設計書の例のモーラ数() -> None:
    """詳細設計 01 §4 の表を、モーラ分割の実装で検算する。

    表は当初モーラ数ではなく文字数を載せていた（`ジュウ` を 3 としていた）。
    `ジュ` は同じ設計書が定める規則 R2（拗音は2文字で1モーラ）の対象である。
    """
    assert counter.count(numbers.read("1")) == 2  # イ/チ
    assert counter.count(numbers.read("10")) == 2  # ジュ/ウ
    assert counter.count(numbers.read("2024")) == 8  # ニ/セ/ン/ニ/ジュ/ウ/ヨ/ン
