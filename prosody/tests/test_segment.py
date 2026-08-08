"""区切り探索のテスト（詳細設計 04 §4 の TC-SS-01〜16）。

**形態素解析器を使わない。** `ResolvedToken` を直接組み立てて与えるため、
辞書のロードなしに実行できる（詳細設計 02）。
"""

import pytest

from prosody.breakcost import INNER_BREAK_PENALTY, BreakCostTable
from prosody.reading import ResolvedToken
from prosody.segment import (
    DEVIATION_WEIGHT,
    MAX_TOTAL_MORA,
    MIN_TOTAL_MORA,
    SegmentSearcher,
    early_reason,
)
from prosody.token import PartOfSpeech
from prosody.tolerance import ToleranceRule
from prosody.verdict import Reason, Verdict

searcher = SegmentSearcher()


def tok(
    mora: int,
    pos: PartOfSpeech = PartOfSpeech.NOUN,
    *,
    surface: str = "",
    unit_boundary: bool = True,
) -> ResolvedToken:
    """モーラ数と品詞だけを指定して形態素をつくる。

    区切り探索が見るのはモーラ数・品詞・単位境界だけなので、
    表記と読みは検証に必要な場合のみ与える。
    """
    return ResolvedToken(
        surface=surface or ("あ" * mora),
        reading="ア" * mora,
        mora=mora,
        pos=pos,
        unit_boundary=unit_boundary,
    )


def space() -> ResolvedToken:
    return ResolvedToken(
        surface=" ", reading="", mora=0, pos=PartOfSpeech.PUNCTUATION, unit_boundary=True
    )


# ----------------------------------------------------------------------
# 総モーラ数による早期判定（詳細設計 01 §5）
# ----------------------------------------------------------------------

EARLY_CASES = [
    pytest.param(0, Reason.TOO_FEW_MORA, id="空"),
    pytest.param(8, Reason.TOO_FEW_MORA, id="今日は疲れた_相当"),
    pytest.param(15, Reason.TOO_FEW_MORA, id="下限の1つ外"),
    pytest.param(16, None, id="下限ちょうど"),
    pytest.param(17, None, id="定型"),
    pytest.param(18, None, id="上限ちょうど"),
    pytest.param(19, Reason.TOO_MANY_MORA, id="上限の1つ外"),
    pytest.param(30, Reason.TOO_MANY_MORA, id="長文"),
]


@pytest.mark.parametrize(("total", "expected"), EARLY_CASES)
def test_総モーラ数による早期判定(total: int, expected: Reason | None) -> None:
    assert early_reason(total) == expected


def test_早期判定の範囲は許容ルールから導かれる() -> None:
    """`16〜18` を独立した第3の条件として持っていないことを確かめる。

    ADR-0001 は「合計の制約は各句の制約から自動的に導かれるため、
    明示的に書くと条件の重複によってバグを生む」としている。
    早期判定はその帰結を高速化に使っているだけであり、
    許容ルールを変えたときに食い違ってはならない。
    """
    rule = ToleranceRule()
    totals = {
        kami + naka + shimo
        for kami in range(0, 12)
        for naka in range(0, 12)
        for shimo in range(0, 12)
        if rule.is_valid(kami, naka, shimo)
    }

    assert min(totals) == MIN_TOTAL_MORA
    assert max(totals) == MAX_TOTAL_MORA


# ----------------------------------------------------------------------
# TC-SS-01〜08: 探索と選択
# ----------------------------------------------------------------------


def test_TC_SS_01_有効な区切りが存在しなければNoneを返す() -> None:
    # 5/7/5 に切れない配置（17モーラだが境界が合わない）
    tokens = [tok(8), tok(8), tok(1)]

    assert searcher.best_for(tokens) is None


def test_TC_SS_02_有効な区切りがちょうど1つならそれを返す() -> None:
    tokens = [tok(5), tok(7), tok(5)]

    best = searcher.best_for(tokens)

    assert best is not None
    assert (best.kami, best.naka, best.shimo) == (5, 7, 5)
    assert best.verdict is Verdict.TEIKEI


def test_TC_SS_03_ズレ数が同じなら区切りコストが小さい方を選ぶ() -> None:
    """同じ 5/7/5 に切れる区切りが2通りある配置をつくる。

    片方は助詞の直後（コスト0）、もう片方は名詞どうし（コスト2）で切れる。
    """
    tokens = [
        tok(3),
        tok(2, PartOfSpeech.PARTICLE),  # ここで切ると直前が助詞 → コスト0
        tok(4),
        tok(3, PartOfSpeech.PARTICLE),
        tok(5),
    ]
    # 累積: 3,5,9,12,17 → (i=2, j=4) が 5/7/5

    best = searcher.best_for(tokens)

    assert best is not None
    assert (best.kami_end, best.naka_end) == (2, 4)
    assert best.score == 0  # 助詞の直後で2回とも切れている


def test_TC_SS_04_定型候補と許容候補があれば定型を選ぶ() -> None:
    """重み 100 が効いていること。

    許容の候補が「助詞の直後（コスト0）」で、定型の候補が
    「名詞どうし（コスト2）」でも、定型が勝たなければならない。
    """
    tokens = [
        tok(4),
        tok(1, PartOfSpeech.PARTICLE),  # i=1 で切ると 4/…（許容側・コスト0）
        tok(7),
        tok(5),
    ]
    # 累積: 4,5,12,17
    #   (i=1, j=2) → 4/7/6? いや 4 / 8 / 5 …
    #   (i=2, j=3) → 5/7/5（定型・名詞どうしでコスト2）

    candidates = searcher.search(tokens)
    verdicts = {c.verdict for c in candidates}
    assert Verdict.TEIKEI in verdicts, "定型の候補が存在する前提のテスト"

    best = searcher.best_for(tokens)

    assert best is not None
    assert best.verdict is Verdict.TEIKEI
    assert best.score < DEVIATION_WEIGHT  # ズレ0 のスコアはコストのみ


def test_TC_SS_05_形態素が2つならNoneを返す() -> None:
    """3つに分けられない。"""
    assert searcher.best_for([tok(5), tok(12)]) is None
    assert searcher.search([tok(5), tok(12)]) == []


def test_TC_SS_06_形態素が3つでちょうど5と7と5なら定型() -> None:
    best = searcher.best_for([tok(5), tok(7), tok(5)])

    assert best is not None
    assert best.verdict is Verdict.TEIKEI


def test_TC_SS_07_総モーラ16でも有効な区切りがあれば許容() -> None:
    best = searcher.best_for([tok(4), tok(7), tok(5)])

    assert best is not None
    assert (best.kami, best.naka, best.shimo) == (4, 7, 5)
    assert best.verdict is Verdict.KYOYO


def test_TC_SS_08_総モーラ18でも有効な区切りがあれば許容() -> None:
    best = searcher.best_for([tok(6), tok(7), tok(5)])

    assert best is not None
    assert (best.kami, best.naka, best.shimo) == (6, 7, 5)
    assert best.verdict is Verdict.KYOYO


# ----------------------------------------------------------------------
# TC-SS-09〜12: 段階的探索（C単位 → A単位）
# ----------------------------------------------------------------------


def test_TC_SS_09_C単位で見つかればA単位の解析を行わない() -> None:
    """性能に直結するため、結果だけでなく**呼び出し回数**を検証する。"""
    calls = 0

    def refine() -> list[ResolvedToken]:
        nonlocal calls
        calls += 1
        return []

    best = searcher.search_staged([tok(5), tok(7), tok(5)], refine)

    assert best is not None
    assert best.verdict is Verdict.TEIKEI
    assert calls == 0, "C単位で見つかったのに A単位の解析が呼ばれている"


def test_TC_SS_10_C単位で見つからなければA単位で再探索する() -> None:
    calls = 0

    def refine() -> list[ResolvedToken]:
        nonlocal calls
        calls += 1
        return [tok(5), tok(7), tok(5)]

    # C単位では 8/8/1 のように切れない配置
    best = searcher.search_staged([tok(8), tok(8), tok(1)], refine)

    assert calls == 1
    assert best is not None
    assert best.verdict is Verdict.TEIKEI


def test_TC_SS_11_どちらでも見つからなければNoneを返す() -> None:
    best = searcher.search_staged([tok(8), tok(8), tok(1)], lambda: [tok(9), tok(8)])

    assert best is None


def test_TC_SS_12_A単位ではC単位の境界を跨がない候補が優先される() -> None:
    """+5 のペナルティが効いていること。

    どちらも 5/7/5 に切れるが、片方は C単位の内部を分断している。
    """
    tokens = [
        tok(5, unit_boundary=True),
        tok(7, unit_boundary=False),  # ここで切ると C単位を分断する
        tok(5, unit_boundary=True),
        # 別の切り方をつくるための埋め合わせ
    ]

    without_penalty = searcher.best_for(tokens, penalize_inner_breaks=False)
    with_penalty = searcher.best_for(tokens, penalize_inner_breaks=True)

    assert without_penalty is not None
    assert with_penalty is not None
    assert with_penalty.score == without_penalty.score + INNER_BREAK_PENALTY


def test_A単位で境界を跨ぐ候補と跨がない候補があれば跨がない方を選ぶ() -> None:
    # 区切りは添字 2（上五の終わり）と 4（中七の終わり）に入る。
    # 境界を判定するのは「切った直後の形態素」なので、
    # フラグを立てるのも添字 2 と 4 である。
    tokens = [
        tok(3),
        tok(2),
        tok(4, unit_boundary=True),  # i=2 の直後。C単位の境界
        tok(3),
        tok(5, unit_boundary=True),  # j=4 の直後。C単位の境界
    ]
    inner = [
        tok(3),
        tok(2),
        tok(4, unit_boundary=False),  # C単位の内部を分断する
        tok(3),
        tok(5, unit_boundary=False),
    ]

    on_boundary = searcher.best_for(tokens, penalize_inner_breaks=True)
    crossing = searcher.best_for(inner, penalize_inner_breaks=True)

    assert on_boundary is not None
    assert crossing is not None
    assert on_boundary.score < crossing.score


# ----------------------------------------------------------------------
# TC-SS-13〜16: 明示的な区切り（詳細設計 01 §7）
# ----------------------------------------------------------------------


def test_TC_SS_13_空白2つで区切られその区切りが有効なら採用する() -> None:
    tokens = [tok(5), space(), tok(7), space(), tok(5)]

    best = searcher.best_for(tokens)

    assert best is not None
    assert (best.kami_end, best.naka_end) == (1, 3)
    assert (best.kami, best.naka, best.shimo) == (5, 7, 5)


def test_TC_SS_14_空白2つでもその区切りが無効なら通常探索へ進む() -> None:
    """利用者の指定が有効でないときに突き放さず、別の区切りを提示する。"""
    # 空白の位置（添字 1 と 3）で切ると 2/3/12 になり許容できない。
    # 一方 (i=3, j=5) では 5/7/5 に切れる。
    tokens = [tok(2), space(), tok(3), space(), tok(7), tok(5)]

    assert searcher.explicit(tokens) is None, "明示的な区切りは無効である前提のテスト"

    best = searcher.best_for(tokens)

    assert best is not None
    assert (best.kami, best.naka, best.shimo) == (5, 7, 5)
    assert best.verdict is Verdict.TEIKEI
    # 区切りの添字は固定しない。空白は0モーラのため、空白の前で切っても
    # 後で切っても音数は同じになり、複数の (i, j) が同じ結果を与える。
    # どれを選ぶかは区切りコスト次第であり、音数の正しさとは独立している。
    assert (best.kami_end, best.naka_end) != (1, 3), "明示的な区切りが採用されている"


def test_空白の前後どちらで切っても音数は変わらない() -> None:
    """空白が0モーラであることの帰結。

    同じ音数の候補が複数生まれるため、区切りコストの低い方
    （句読点の直後 = コスト0）が選ばれる。表示のために
    句の前後の空白を落とすのは呼び出し側の責務である（#8）。
    """
    tokens = [tok(5), space(), tok(7), tok(5)]

    candidates = searcher.search(tokens)
    splits = {(c.kami, c.naka, c.shimo) for c in candidates}

    assert splits == {(5, 7, 5)}
    assert len(candidates) > 1, "空白の前と後で別々の候補になる"


def test_TC_SS_15_空白が1つなら通常探索へ進む() -> None:
    tokens = [tok(5), space(), tok(7), tok(5)]

    best = searcher.best_for(tokens)

    assert best is not None
    assert (best.kami, best.naka, best.shimo) == (5, 7, 5)


def test_TC_SS_16_空白が3つ以上なら通常探索へ進む() -> None:
    tokens = [tok(5), space(), tok(3), space(), tok(4), space(), tok(5)]

    assert searcher.explicit(tokens) is None


def test_空白が先頭にあると明示的な区切りとして扱わない() -> None:
    """`0 < i` を満たさないため、上五が空になる区切りは採らない。"""
    tokens = [space(), tok(5), space(), tok(7), tok(5)]

    assert searcher.explicit(tokens) is None


# ----------------------------------------------------------------------
# 区切りコスト表（詳細設計 01 §8）
# ----------------------------------------------------------------------

costs = BreakCostTable()

COST_CASES = [
    pytest.param(PartOfSpeech.PUNCTUATION, PartOfSpeech.NOUN, 0, id="直前が句読点"),
    pytest.param(PartOfSpeech.PARTICLE, PartOfSpeech.NOUN, 0, id="直前が助詞"),
    pytest.param(PartOfSpeech.AUXILIARY_VERB, PartOfSpeech.NOUN, 0, id="直前が助動詞"),
    pytest.param(PartOfSpeech.NOUN, PartOfSpeech.PARTICLE, 4, id="直後が助詞"),
    pytest.param(PartOfSpeech.NOUN, PartOfSpeech.AUXILIARY_VERB, 4, id="直後が助動詞"),
    pytest.param(PartOfSpeech.PREFIX, PartOfSpeech.NOUN, 4, id="直前が接頭辞"),
    pytest.param(PartOfSpeech.VERB, PartOfSpeech.SUFFIX, 4, id="直後が接尾辞"),
    pytest.param(PartOfSpeech.NOUN, PartOfSpeech.NOUN, 2, id="名詞どうし"),
    pytest.param(PartOfSpeech.NOUN, PartOfSpeech.VERB, 3, id="直後が動詞"),
    pytest.param(PartOfSpeech.ADVERB, PartOfSpeech.ADJECTIVE, 1, id="上記以外"),
]


@pytest.mark.parametrize(("before", "after", "expected"), COST_CASES)
def test_区切りコスト(before: PartOfSpeech, after: PartOfSpeech, expected: int) -> None:
    assert costs.cost(tok(1, before), tok(1, after)) == expected


def test_規則は上から順に評価される() -> None:
    """「直前が助詞」と「直後が助詞」が同時に当てはまる場合。

    詳細設計 01 §8 の表は優先順位を明示していないため、
    表の並び順を優先順位として採用することを固定する。
    """
    assert costs.cost(tok(1, PartOfSpeech.PARTICLE), tok(1, PartOfSpeech.PARTICLE)) == 0


def test_区切りコストの合計は重みより小さい() -> None:
    """ズレ数が1つでも少ない候補が必ず勝つ、という前提の確認。

    詳細設計 01 §8 は「区切りコストの合計は最大でも 20 程度」と述べている。
    最大コスト（ペナルティ込み）2箇所分が重み 100 を超えないことを確かめる。
    """
    max_single = max(costs.cost(tok(1, b), tok(1, a)) for b in PartOfSpeech for a in PartOfSpeech)

    assert (max_single + INNER_BREAK_PENALTY) * 2 < DEVIATION_WEIGHT


# ----------------------------------------------------------------------
# 枝刈りの境界
# ----------------------------------------------------------------------


def test_上五が範囲に届かないまま形態素が尽きる() -> None:
    """モーラ数が小さく、上五の下限にすら達しない配置。

    外側のループが break せずに終わる経路。
    """
    assert searcher.search([tok(1), tok(1), tok(1)]) == []


def test_中七が上限を超えたら内側のループを打ち切る() -> None:
    """累積は単調増加するため、それ以上 j を進めても中七は範囲内に戻らない。"""
    # i=1 で上五=5、j=2 で中七=9（上限8超）。ここで打ち切る。
    assert searcher.search([tok(5), tok(9), tok(3)]) == []


def test_上五が上限を超えたら外側のループを打ち切る() -> None:
    assert searcher.search([tok(7), tok(7), tok(5)]) == []


# ----------------------------------------------------------------------
# 候補の情報
# ----------------------------------------------------------------------


def test_候補はズレ数を返せる() -> None:
    teikei = searcher.best_for([tok(5), tok(7), tok(5)])
    kyoyo = searcher.best_for([tok(4), tok(7), tok(5)])

    assert teikei is not None
    assert kyoyo is not None
    assert teikei.deviations == 0
    assert kyoyo.deviations == 1
