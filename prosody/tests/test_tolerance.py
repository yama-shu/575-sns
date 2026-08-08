"""許容ルールのテスト。

詳細設計 04 §2 で洗い出した TC-TR-01〜28 をそのまま実装する。
設計書の表と1対1で対応させるため、ケース ID を pytest の parametrize の
id として与えている。`pytest -k TC_TR_18` のように ID で絞り込めるため、
設計書とテストコードを双方向に辿れる（詳細設計 04 §9）。

このクラスは ADR-0001 の決定そのものであり、575 で最も重要なテスト対象である。
"""

import pytest

from prosody.tolerance import ToleranceRule
from prosody.verdict import Verdict

rule = ToleranceRule()

TEIKEI = Verdict.TEIKEI
KYOYO = Verdict.KYOYO
HACHO = Verdict.HACHO


# ----------------------------------------------------------------------
# TC-TR-01〜15: 各句の境界値（他の2句は規定値に固定する）
# ----------------------------------------------------------------------

BOUNDARY_CASES = [
    # 上五の境界（中七=7 / 下五=5 に固定）
    pytest.param(3, 7, 5, HACHO, id="TC_TR_01"),  # 上五が下限の1つ外
    pytest.param(4, 7, 5, KYOYO, id="TC_TR_02"),  # 上五が下限ちょうど
    pytest.param(5, 7, 5, TEIKEI, id="TC_TR_03"),  # 上五が規定値
    pytest.param(6, 7, 5, KYOYO, id="TC_TR_04"),  # 上五が上限ちょうど
    pytest.param(7, 7, 5, HACHO, id="TC_TR_05"),  # 上五が上限の1つ外
    # 中七の境界（上五=5 / 下五=5 に固定）
    pytest.param(5, 5, 5, HACHO, id="TC_TR_06"),  # 中七が下限の1つ外
    pytest.param(5, 6, 5, KYOYO, id="TC_TR_07"),  # 中七が下限ちょうど
    pytest.param(5, 7, 5, TEIKEI, id="TC_TR_08"),  # 中七が規定値
    pytest.param(5, 8, 5, KYOYO, id="TC_TR_09"),  # 中七が上限ちょうど
    pytest.param(5, 9, 5, HACHO, id="TC_TR_10"),  # 中七が上限の1つ外
    # 下五の境界（上五=5 / 中七=7 に固定）
    pytest.param(5, 7, 3, HACHO, id="TC_TR_11"),  # 下五が下限の1つ外
    pytest.param(5, 7, 4, KYOYO, id="TC_TR_12"),  # 下五が下限ちょうど
    pytest.param(5, 7, 5, TEIKEI, id="TC_TR_13"),  # 下五が規定値
    pytest.param(5, 7, 6, KYOYO, id="TC_TR_14"),  # 下五が上限ちょうど
    pytest.param(5, 7, 7, HACHO, id="TC_TR_15"),  # 下五が上限の1つ外
]


@pytest.mark.parametrize(("kami", "naka", "shimo", "expected"), BOUNDARY_CASES)
def test_各句の境界値(kami: int, naka: int, shimo: int, expected: Verdict) -> None:
    assert rule.verdict_of(kami, naka, shimo) == expected


# ----------------------------------------------------------------------
# TC-TR-16〜21: ズレ数の境界 ← このテスト設計で最も重要
#
# **各句は範囲内なのに、組み合わせとして不正になるケース。**
# 各句を個別にチェックするだけの実装（条件1 だけ）は TC-TR-01〜15 を
# すべて通過しながら、ここで落ちる。ADR-0001 の第2条件（ズレ数の制限）が
# 実装から抜け落ちる典型的なバグの検出器である。
# ----------------------------------------------------------------------

DEVIATION_CASES = [
    pytest.param(5, 7, 5, 0, TEIKEI, id="TC_TR_16"),  # ズレ0
    pytest.param(4, 7, 5, 1, KYOYO, id="TC_TR_17"),  # ズレ1
    pytest.param(4, 6, 5, 2, HACHO, id="TC_TR_18"),  # すべて範囲内だがズレ2
    pytest.param(6, 8, 5, 2, HACHO, id="TC_TR_19"),  # すべて範囲内だがズレ2
    pytest.param(4, 7, 6, 2, HACHO, id="TC_TR_20"),  # すべて範囲内だがズレ2
    pytest.param(4, 6, 4, 3, HACHO, id="TC_TR_21"),  # すべて範囲内だがズレ3
]


@pytest.mark.parametrize(("kami", "naka", "shimo", "deviations", "expected"), DEVIATION_CASES)
def test_ズレ数の境界(kami: int, naka: int, shimo: int, deviations: int, expected: Verdict) -> None:
    # 前提: いずれのケースも各句は許容範囲に収まっている
    assert 4 <= kami <= 6
    assert 6 <= naka <= 8
    assert 4 <= shimo <= 6

    assert rule.deviation_count(kami, naka, shimo) == deviations
    assert rule.verdict_of(kami, naka, shimo) == expected


# ----------------------------------------------------------------------
# TC-TR-22〜28: 投稿可能な7パターンの網羅
#
# ADR-0001 が「投稿可能なのは定型1通り + 許容6通りの計7通り」と明示している。
# 仕様が有限の集合として定義されているため、網羅が可能である。
# ----------------------------------------------------------------------

POSTABLE_PATTERNS = [
    pytest.param(5, 7, 5, TEIKEI, id="TC_TR_22"),  # 定型
    pytest.param(4, 7, 5, KYOYO, id="TC_TR_23"),  # 上五が字足らず
    pytest.param(6, 7, 5, KYOYO, id="TC_TR_24"),  # 上五が字余り
    pytest.param(5, 6, 5, KYOYO, id="TC_TR_25"),  # 中七が字足らず
    pytest.param(5, 8, 5, KYOYO, id="TC_TR_26"),  # 中七が字余り
    pytest.param(5, 7, 4, KYOYO, id="TC_TR_27"),  # 下五が字足らず
    pytest.param(5, 7, 6, KYOYO, id="TC_TR_28"),  # 下五が字余り
]


@pytest.mark.parametrize(("kami", "naka", "shimo", "expected"), POSTABLE_PATTERNS)
def test_投稿可能な7パターン(kami: int, naka: int, shimo: int, expected: Verdict) -> None:
    assert rule.is_valid(kami, naka, shimo) is True
    assert rule.verdict_of(kami, naka, shimo) == expected
    assert expected.is_postable() is True


def test_投稿可能な組み合わせは7通りしかない() -> None:
    """総当たりで、投稿可能な組み合わせが設計どおり7通りだけであることを確かめる。

    個別ケースの積み上げでは「他に通ってしまう組み合わせが無いか」を示せない。
    取りうる範囲を全探索して、集合として一致することを確認する。
    """
    postable = {
        (kami, naka, shimo)
        for kami in range(0, 12)
        for naka in range(0, 12)
        for shimo in range(0, 12)
        if rule.is_valid(kami, naka, shimo)
    }

    assert postable == {
        (5, 7, 5),  # 定型
        (4, 7, 5),
        (6, 7, 5),
        (5, 6, 5),
        (5, 8, 5),
        (5, 7, 4),
        (5, 7, 6),
    }


def test_投稿可能な組み合わせの合計モーラ数は16から18に収まる() -> None:
    """合計の制約を書かなくても 16〜18 に収まることを確かめる。

    ADR-0001 は「合計 16〜18」を第3の条件として書くと冗長かつバグの温床になる、
    として書かないことを決めた。その判断が成立していることを、
    条件を追加せずに検証する。
    """
    totals = {
        kami + naka + shimo
        for kami in range(0, 12)
        for naka in range(0, 12)
        for shimo in range(0, 12)
        if rule.is_valid(kami, naka, shimo)
    }

    assert totals == {16, 17, 18}


# ----------------------------------------------------------------------
# 破調の判定は投稿できない
# ----------------------------------------------------------------------


def test_破調は投稿できない() -> None:
    assert Verdict.HACHO.is_postable() is False


def test_読みを確定できない場合は投稿できない() -> None:
    assert Verdict.UNKNOWN.is_postable() is False
