"""読み解決のテスト（詳細設計 04 §5 の TC-RR-01〜09）。

**形態素解析器を使わない。** `Token` を直接組み立てて与えるため、
辞書のロードなしに実行できる（詳細設計 02）。
"""

import pytest

from prosody.reading import ReadingResolver, read_latin
from prosody.token import PartOfSpeech, Token

resolver = ReadingResolver()


def token(surface: str, reading: str | None, pos: PartOfSpeech = PartOfSpeech.NOUN) -> Token:
    return Token(surface=surface, reading=reading, pos=pos)


# ----------------------------------------------------------------------
# TC-RR-01: 解析器の読みをそのまま使う
# ----------------------------------------------------------------------


def test_TC_RR_01_通常の日本語は解析器の読みをそのまま使う() -> None:
    tokens = [token("今日", "キョウ"), token("も", "モ", PartOfSpeech.PARTICLE)]

    resolved = resolver.resolve(tokens)

    assert resolved.reading == "キョウモ"
    assert resolved.total_mora == 3  # キョ/ウ/モ
    assert resolved.is_readable is True
    assert resolved.unreadable == []


# ----------------------------------------------------------------------
# TC-RR-02〜05: 数値読み変換（読みが空のとき）
# ----------------------------------------------------------------------

NUMERIC_CASES = [
    pytest.param("1", "イチ", 2, id="TC_RR_02"),
    pytest.param("10", "ジュウ", 2, id="TC_RR_03"),
    pytest.param("2024", "ニセンニジュウヨン", 8, id="TC_RR_04"),
    pytest.param("0", "ゼロ", 2, id="TC_RR_05"),
]


@pytest.mark.parametrize(("surface", "expected_reading", "expected_mora"), NUMERIC_CASES)
def test_数字は読みが空でも補完される(
    surface: str, expected_reading: str, expected_mora: int
) -> None:
    resolved = resolver.resolve([token(surface, None)])

    assert resolved.reading == expected_reading
    assert resolved.total_mora == expected_mora
    assert resolved.is_readable is True


# ----------------------------------------------------------------------
# TC-RR-06: ラテン文字
# ----------------------------------------------------------------------


def test_TC_RR_06_ラテン文字はアルファベット読みへフォールバックする() -> None:
    resolved = resolver.resolve([token("AI", None)])

    assert resolved.reading == "エーアイ"
    assert resolved.total_mora == 4  # エ/ー/ア/イ
    assert resolved.is_readable is True


def test_小文字のラテン文字も読める() -> None:
    assert read_latin("ai") == "エーアイ"


def test_ラテン文字以外が混ざると読めない扱いにする() -> None:
    """「一部だけ読めた」状態を作らない。"""
    assert read_latin("A1") == ""
    assert read_latin("Aあ") == ""


@pytest.mark.parametrize("ch", list("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
def test_すべてのアルファベットに読みがある(ch: str) -> None:
    assert read_latin(ch) != ""


# ----------------------------------------------------------------------
# TC-RR-07: 記号
# ----------------------------------------------------------------------


def test_TC_RR_07_記号のみは0モーラになる() -> None:
    resolved = resolver.resolve([token("！？", None, PartOfSpeech.PUNCTUATION)])

    assert resolved.total_mora == 0
    assert resolved.reading == ""
    assert resolved.is_readable is True


def test_空白も0モーラになる() -> None:
    resolved = resolver.resolve([token(" ", None, PartOfSpeech.PUNCTUATION)])

    assert resolved.total_mora == 0
    assert resolved.is_readable is True


def test_表層が空でもクラッシュしない() -> None:
    resolved = resolver.resolve([token("", None)])

    assert resolved.total_mora == 0
    assert resolved.is_readable is True


# ----------------------------------------------------------------------
# TC-RR-08〜09: 読めない語
#
# **hacho ではなく unknown を返すこと**を検証する。
# この2つを取り違えると、正しく詠んだ利用者に
# 「五七五になっていません」と伝えることになり、直しようがなくなる。
# ----------------------------------------------------------------------


def test_TC_RR_08_読めない語があるとき不読語として記録される() -> None:
    tokens = [token("甃", None), token("や", "ヤ", PartOfSpeech.PARTICLE)]

    resolved = resolver.resolve(tokens)

    assert resolved.is_readable is False
    assert resolved.unreadable == ["甃"]


def test_TC_RR_09_読めない語が複数あればすべて記録される() -> None:
    tokens = [token("甃", None), token("と", "ト", PartOfSpeech.PARTICLE), token("鵺", None)]

    resolved = resolver.resolve(tokens)

    assert resolved.is_readable is False
    assert resolved.unreadable == ["甃", "鵺"]


def test_読めない語は読みにもモーラにも数えない() -> None:
    """読めなかった語を勝手に0モーラとして通してはならない。

    総モーラ数が合ってしまい、破調でない句として判定が進む恐れがある。
    unreadable が空でない時点で unknown として扱う（#8 で API に反映する）。
    """
    resolved = resolver.resolve([token("甃", None)])

    assert resolved.reading == ""
    assert resolved.total_mora == 0
    assert resolved.is_readable is False


# ----------------------------------------------------------------------
# 形態素の属性が引き継がれること（#7 の区切り探索が使う）
# ----------------------------------------------------------------------


def test_品詞と単位境界が引き継がれる() -> None:
    tokens = [
        Token(surface="古池", reading="フルイケ", pos=PartOfSpeech.NOUN, unit_boundary=True),
        Token(surface="や", reading="ヤ", pos=PartOfSpeech.PARTICLE, unit_boundary=False),
    ]

    resolved = resolver.resolve(tokens)

    assert [t.pos for t in resolved.tokens] == [PartOfSpeech.NOUN, PartOfSpeech.PARTICLE]
    assert [t.unit_boundary for t in resolved.tokens] == [True, False]
    assert [t.mora for t in resolved.tokens] == [4, 1]


def test_形態素が無ければ空の結果になる() -> None:
    resolved = resolver.resolve([])

    assert resolved.reading == ""
    assert resolved.total_mora == 0
    assert resolved.is_readable is True
