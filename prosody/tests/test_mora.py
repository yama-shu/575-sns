"""モーラ分割のテスト。

詳細設計 04 §3 で洗い出した TC-MC-01〜20 と、性質ベースのテスト
TC-MC-P1〜P3 を実装する。ケース ID は parametrize の id として与える。

規則 R1〜R6 ごとにテストを対応させ、どの規則が壊れたかが分かるようにする。
"""

import unicodedata

import pytest
from hypothesis import HealthCheck, given, settings
from hypothesis import strategies as st

from prosody.mora import COMBINING, INDEPENDENT, MoraCounter, is_ignorable

counter = MoraCounter()


# ----------------------------------------------------------------------
# TC-MC-01〜14: 規則 R1〜R6 と、その組み合わせ
# ----------------------------------------------------------------------

RULE_CASES = [
    # R1: カタカナ1文字を1モーラとする
    pytest.param("カ", ["カ"], id="TC_MC_01"),  # R1
    pytest.param("カキクケコ", ["カ", "キ", "ク", "ケ", "コ"], id="TC_MC_02"),  # R1
    # R2: 小書きの母音・拗音は直前と結合する
    pytest.param("キョ", ["キョ"], id="TC_MC_03"),  # R2 拗音
    pytest.param("ファ", ["ファ"], id="TC_MC_04"),  # R2 外来語
    pytest.param("ヴァ", ["ヴァ"], id="TC_MC_05"),  # R2
    pytest.param("クヮ", ["クヮ"], id="TC_MC_06"),  # R2
    # R3〜R5: 単独で1モーラ
    pytest.param("キッ", ["キ", "ッ"], id="TC_MC_07"),  # R3 促音
    pytest.param("カン", ["カ", "ン"], id="TC_MC_08"),  # R4 撥音
    pytest.param("カー", ["カ", "ー"], id="TC_MC_09"),  # R5 長音
    # R6: 記号は0モーラ
    pytest.param("カ、キ", ["カ", "キ"], id="TC_MC_10"),  # R6 記号
    # 複合
    pytest.param("トウキョウ", ["ト", "ウ", "キョ", "ウ"], id="TC_MC_11"),  # 東京 = 4
    pytest.param("シュッチョウ", ["シュ", "ッ", "チョ", "ウ"], id="TC_MC_12"),  # 出張 = 4
    pytest.param("シンカンセン", ["シ", "ン", "カ", "ン", "セ", "ン"], id="TC_MC_13"),  # 新幹線 = 6
    pytest.param("コーヒー", ["コ", "ー", "ヒ", "ー"], id="TC_MC_14"),  # コーヒー = 4
]


@pytest.mark.parametrize(("reading", "expected"), RULE_CASES)
def test_規則ごとのモーラ分割(reading: str, expected: list[str]) -> None:
    assert counter.split(reading) == expected
    assert counter.count(reading) == len(expected)


# ----------------------------------------------------------------------
# TC-MC-15〜20: 境界と異常系
#
# TC-MC-15〜17 は「結合先が無い場合」の境界を固定する。
#
# **これらは「ッ を結合対象に入れる誤り」を検出しない。** 実装が結合先の有無を
# 確認する（`ch in COMBINING and morae`）ため、単独の「ッ」は誤りの有無に
# かかわらず1モーラになる。その誤りを実際に捉えるのは TC-MC-07 と TC-MC-12、
# および下の集合定義のテストである（詳細設計 04 §3）。
# ----------------------------------------------------------------------

TRICKY_CASES = [
    pytest.param("ッ", 1, id="TC_MC_15"),  # 結合先が無い促音
    pytest.param("ン", 1, id="TC_MC_16"),  # 結合先が無い撥音
    pytest.param("ー", 1, id="TC_MC_17"),  # 結合先が無い長音
    pytest.param("ャ", 1, id="TC_MC_18"),  # 結合対象が先頭に来た異常系
    pytest.param("", 0, id="TC_MC_19"),  # 空文字列
    pytest.param("キョキョキョ", 3, id="TC_MC_20"),  # 拗音の連続
]


@pytest.mark.parametrize(("reading", "expected"), TRICKY_CASES)
def test_境界と異常系(reading: str, expected: int) -> None:
    assert counter.count(reading) == expected


def test_結合対象と単独モーラの集合は交わらない() -> None:
    """「ッ」「ン」「ー」を結合対象に入れる誤りを、集合の定義レベルで固定する。

    TC-MC-07 / TC-MC-12 は結果から誤りを検出するが、
    こちらは実装の書き方に依存せず、定義そのものを守る。
    """
    assert frozenset() == COMBINING & INDEPENDENT
    assert frozenset("ッンー") == INDEPENDENT


# ----------------------------------------------------------------------
# 性質ベースのテスト（TC-MC-P1〜P3）
#
# 個別ケースでは思いつかない入力の組み合わせを拾う。
# 完了条件どおり1万件を生成して検証する。
# ----------------------------------------------------------------------

# カタカナ（結合対象・単独モーラ・記号を含む）からランダムな文字列をつくる
KATAKANA = st.sampled_from(
    "アイウエオカキクケコサシスセソタチツテトナニヌネノ"
    "ハヒフヘホマミムメモヤユヨラリルレロワヲンヴ"
    "ァィゥェォャュョヮッー"
    "、。！？ "  # 0モーラになる文字も混ぜる
)
READINGS = st.lists(KATAKANA, max_size=30).map("".join)

ONE_MANKEN = settings(
    max_examples=10_000,
    deadline=None,
    suppress_health_check=[HealthCheck.too_slow],
)


@ONE_MANKEN
@given(READINGS)
def test_TC_MC_P1_分割したモーラを連結すると記号を除いた元の読みに一致する(
    reading: str,
) -> None:
    """文字を落とさない・増やさない。"""
    expected = "".join(ch for ch in reading if not is_ignorable(ch))
    assert "".join(counter.split(reading)) == expected


@ONE_MANKEN
@given(READINGS)
def test_TC_MC_P2_モーラ数は読みの文字数を超えない(reading: str) -> None:
    """結合はするが分裂はしない。"""
    assert counter.count(reading) <= len(reading)


@ONE_MANKEN
@given(READINGS)
def test_TC_MC_P3_記号だけでない読みは1モーラ以上になる(reading: str) -> None:
    """読みが空でなく記号だけでもないなら、必ず1モーラ以上になる。"""
    if any(not is_ignorable(ch) for ch in reading):
        assert counter.count(reading) >= 1
    else:
        assert counter.count(reading) == 0


# ----------------------------------------------------------------------
# R6 の対象範囲
# ----------------------------------------------------------------------

IGNORABLE_CASES = [
    pytest.param("、", id="読点"),
    pytest.param("。", id="句点"),
    pytest.param("！", id="感嘆符"),
    pytest.param(" ", id="半角空白"),
    pytest.param("　", id="全角空白"),
    pytest.param("\n", id="改行"),
    pytest.param("😀", id="絵文字"),
]


@pytest.mark.parametrize("ch", IGNORABLE_CASES)
def test_記号や空白は0モーラになる(ch: str) -> None:
    assert is_ignorable(ch) is True
    assert counter.count(ch) == 0


def test_長音記号は無視されない() -> None:
    """「ー」は Unicode 上は修飾文字であり、記号として無視してはならない。

    カテゴリで一括除外する実装にすると、ここを巻き込んで
    「コーヒー」が 2モーラになる。
    """
    assert unicodedata.category("ー") == "Lm"
    assert is_ignorable("ー") is False
    assert counter.count("コーヒー") == 4
