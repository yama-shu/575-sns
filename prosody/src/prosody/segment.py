"""五七五の区切り探索（詳細設計 01 §5〜§9）。

形態素の列を上五・中七・下五の3つに区切る。
**区切りは形態素の境界にしか置けない。** 単語の途中で切ってはならない。

    ✅ 古池や / 蛙飛び込む / 水の音
    ❌ 古池や蛙 / 飛び込む水 / の音       ← 「水の音」を分断している
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from typing import Final

from prosody.breakcost import BreakCostTable
from prosody.reading import ResolvedToken
from prosody.tolerance import (
    EXPECTED_KAMI,
    EXPECTED_NAKA,
    EXPECTED_SHIMO,
    RANGE_KAMI,
    RANGE_NAKA,
    ToleranceRule,
)
from prosody.verdict import Reason, Verdict

# 投稿可能な総モーラ数の範囲（詳細設計 01 §5）。
#
# ADR-0001 の決定により、投稿可能な組み合わせは7通りに限られ、
# その合計は 16・17・18 のいずれかにしかならない。
# **これは第3の条件ではなく、許容ルールから導かれる帰結である。**
# 区切りを探す前に総モーラ数だけで結論が出る場合の早期リターンに使う。
#
# 値がルールと食い違わないことは、ToleranceRule を総当たりして検証する。
MIN_TOTAL_MORA: Final = 16
MAX_TOTAL_MORA: Final = 18


def early_reason(total_mora: int) -> Reason | None:
    """総モーラ数だけで破調と分かる場合に理由を返す。

    破調の入力の大半はここで弾かれる。「今日は疲れた」（8モーラ）のような
    短い入力に対して、無駄な区切り探索を行わずに済む。
    """
    if total_mora < MIN_TOTAL_MORA:
        return Reason.TOO_FEW_MORA
    if total_mora > MAX_TOTAL_MORA:
        return Reason.TOO_MANY_MORA
    return None


# ズレ数を区切りコストより常に優先させるための重み（詳細設計 01 §8）。
#
# 区切りコストの合計は最大でも 20 程度にしかならないため、
# ズレ数が1つでも少ない候補が必ず勝つ。
# これにより「定型に区切れるなら、多少不自然でも定型を選ぶ」挙動になる。
# 利用者にとって「定型」と判定される方が価値が高いためである。
DEVIATION_WEIGHT: Final = 100


@dataclass(frozen=True)
class Candidate:
    """区切りの候補。"""

    kami_end: int
    """上五の終わり（この添字の直前で切る）。"""
    naka_end: int
    """中七の終わり。"""

    kami: int
    naka: int
    shimo: int

    verdict: Verdict
    score: int
    """小さいほど良い。"""

    @property
    def deviations(self) -> int:
        """規定値から外れている句の数。"""
        return (
            (self.kami != EXPECTED_KAMI)
            + (self.naka != EXPECTED_NAKA)
            + (self.shimo != EXPECTED_SHIMO)
        )


def is_whitespace_token(token: ResolvedToken) -> bool:
    """明示的な区切りとして置かれた空白か。"""
    return token.surface.strip() == "" and token.surface != ""


class SegmentSearcher:
    """区切り候補の探索と、最良候補の選択。

    形態素解析器を知らない。`ResolvedToken` の列だけを見る。
    """

    def __init__(
        self, rule: ToleranceRule | None = None, costs: BreakCostTable | None = None
    ) -> None:
        self._rule = rule or ToleranceRule()
        self._costs = costs or BreakCostTable()

    # ------------------------------------------------------------------
    # 探索
    # ------------------------------------------------------------------

    def search(
        self, tokens: list[ResolvedToken], *, penalize_inner_breaks: bool = False
    ) -> list[Candidate]:
        """有効な区切り候補をすべて返す。

        累積モーラ数は単調増加するため、範囲を超えた時点で打ち切れる。
        入力サイズに構造的な上限（形態素18個以下）があるため、
        素朴な二重ループで十分に速い。動的計画法などは要らない。
        """
        n = len(tokens)
        if n < 3:
            # 3つに分けられない
            return []

        cumulative = self._cumulative_mora(tokens)
        total = cumulative[n]
        candidates: list[Candidate] = []

        for i in range(1, n):
            kami = cumulative[i]
            if kami > RANGE_KAMI[1]:
                # 累積は単調増加。これ以上 i を進めても上五が範囲外。
                break
            if kami < RANGE_KAMI[0]:
                continue
            for j in range(i + 1, n):
                naka = cumulative[j] - kami
                if naka > RANGE_NAKA[1]:
                    # 同様に j を進めても中七が範囲外
                    break
                shimo = total - cumulative[j]
                if not self._rule.is_valid(kami, naka, shimo):
                    continue
                candidates.append(
                    self._make_candidate(
                        tokens, i, j, kami, naka, shimo, penalize_inner_breaks=penalize_inner_breaks
                    )
                )

        return candidates

    def best(self, candidates: list[Candidate]) -> Candidate | None:
        """スコアが最良の候補を返す。候補が無ければ None。

        同点の場合は先に見つかった（上五が短い）方を選ぶ。
        結果を実装依存にしないため、`min` の安定性に頼らず明示する。
        """
        if not candidates:
            return None
        return min(candidates, key=lambda c: (c.score, c.kami_end, c.naka_end))

    def best_for(
        self, tokens: list[ResolvedToken], *, penalize_inner_breaks: bool = False
    ) -> Candidate | None:
        """最良の区切りを返す。

        利用者が空白で明示的に区切っている場合は、それを優先する（詳細設計 01 §7）。
        """
        explicit = self.explicit(tokens)
        if explicit is not None:
            return explicit
        return self.best(self.search(tokens, penalize_inner_breaks=penalize_inner_breaks))

    def search_staged(
        self,
        coarse_tokens: list[ResolvedToken],
        refine: Callable[[], list[ResolvedToken]],
    ) -> Candidate | None:
        """段階的探索（詳細設計 01 §9）。

        まず C単位で探し、見つからなければ `refine()` で A単位の形態素を取得して
        再探索する。**C単位で見つかった場合 `refine` は呼ばない。**
        A単位は分割が細かく候補が見つかりやすいが、複合語の途中で切れやすいため、
        C単位で成立するならそれが最も自然である。

        `refine` を呼び出し側から渡すのは、この探索器に形態素解析器への依存を
        持たせないためである。
        """
        best = self.best_for(coarse_tokens)
        if best is not None:
            return best
        return self.best_for(refine(), penalize_inner_breaks=True)

    # ------------------------------------------------------------------
    # 明示的な区切り
    # ------------------------------------------------------------------

    def explicit(self, tokens: list[ResolvedToken]) -> Candidate | None:
        """空白による明示的な区切りを候補にする（詳細設計 01 §7）。

        空白がちょうど2つあり、その位置での分割が許容ルールを満たす場合のみ採用する。
        満たさない場合は None を返し、通常の探索に進む。利用者の指定が有効でないときに
        「あなたの指定では破調です」と突き放すのではなく、
        別の区切りで成立するならそれを提示する方が親切である。
        """
        spaces = [index for index, token in enumerate(tokens) if is_whitespace_token(token)]
        if len(spaces) != 2:
            return None

        i, j = spaces
        if not (0 < i < j < len(tokens)):
            return None

        cumulative = self._cumulative_mora(tokens)
        kami = cumulative[i]
        naka = cumulative[j] - kami
        shimo = cumulative[len(tokens)] - cumulative[j]

        if not self._rule.is_valid(kami, naka, shimo):
            return None

        # 明示的な区切りはスコアリングの対象にしない。利用者の意図をそのまま採る。
        return self._make_candidate(tokens, i, j, kami, naka, shimo, penalize_inner_breaks=False)

    # ------------------------------------------------------------------
    # 内部
    # ------------------------------------------------------------------

    def _cumulative_mora(self, tokens: list[ResolvedToken]) -> list[int]:
        """累積モーラ数。`cumulative[k]` は先頭から k 個目までの合計。"""
        cumulative = [0]
        for token in tokens:
            cumulative.append(cumulative[-1] + token.mora)
        return cumulative

    def _make_candidate(
        self,
        tokens: list[ResolvedToken],
        i: int,
        j: int,
        kami: int,
        naka: int,
        shimo: int,
        *,
        penalize_inner_breaks: bool,
    ) -> Candidate:
        deviations = self._rule.deviation_count(kami, naka, shimo)
        score = (
            deviations * DEVIATION_WEIGHT
            + self._costs.cost_at(tokens, i, penalize_inner_break=penalize_inner_breaks)
            + self._costs.cost_at(tokens, j, penalize_inner_break=penalize_inner_breaks)
        )
        return Candidate(
            kami_end=i,
            naka_end=j,
            kami=kami,
            naka=naka,
            shimo=shimo,
            verdict=self._rule.verdict_of(kami, naka, shimo),
            score=score,
        )
