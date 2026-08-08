"""区切り位置の不自然さのコスト化（詳細設計 01 §8）。

同じ本文が複数の区切り方で五七五に収まることがある。そのとき
「どれを選ぶか」を決めるための重み付けである。

コストは経験的に設定した値であり、**唯一の正解はない**。
実装後に実際の投稿で不自然な区切りが観測されたら調整する。
調整したことは記録に残す。
"""

from __future__ import annotations

from typing import Final

from prosody.reading import ResolvedToken
from prosody.token import PartOfSpeech

# A単位で探索するとき、C単位の内部を分断する位置に加えるペナルティ（詳細設計 01 §9）。
# これにより、A単位でも「なるべく C単位の境界で切る」候補が優先される。
INNER_BREAK_PENALTY: Final = 5

_PARTICLE_LIKE: Final = frozenset({PartOfSpeech.PARTICLE, PartOfSpeech.AUXILIARY_VERB})


class BreakCostTable:
    """形態素と形態素のあいだで切ることのコストを返す。

    **規則は上から順に評価し、最初に当てはまったものを採用する。**
    詳細設計 01 §8 の表は優先順位を明示していないが、
    「直前が助詞」と「直後が助詞」のように同時に当てはまる組み合わせがあるため、
    順序を決めないと結果が実装依存になる。表の並び順を優先順位として採用する。
    """

    def cost(self, before: ResolvedToken, after: ResolvedToken) -> int:
        """`before` と `after` のあいだで切るコストを返す。"""
        # 句読点の直後。最も自然な切れ目。
        if before.pos is PartOfSpeech.PUNCTUATION:
            return 0
        # 助詞・助動詞の直後。文節の切れ目。「古池や / 蛙…」
        if before.pos in _PARTICLE_LIKE:
            return 0
        # 助詞・助動詞の直前。語とその助詞を分断する。「会議 / のための」は不自然。
        if after.pos in _PARTICLE_LIKE:
            return 4
        # 接頭辞の直後。「お / 弁当」を分断する。
        if before.pos is PartOfSpeech.PREFIX:
            return 4
        # 接尾辞の直前。「山田 / さん」を分断する。
        if after.pos is PartOfSpeech.SUFFIX:
            return 4
        # 名詞どうし。複合名詞を分断している可能性がある。
        if before.pos is PartOfSpeech.NOUN and after.pos is PartOfSpeech.NOUN:
            return 2
        # 動詞の直前。「飛び込ん / でいる」のような分断を避ける。
        #
        # 詳細設計 01 は「動詞の活用語尾・補助動詞」と書いているが、
        # Token が持つ品詞は解析器の体系を正規化した粗い粒度であり、
        # 補助動詞と自立動詞を区別できない。動詞全体に適用する近似とする。
        if after.pos is PartOfSpeech.VERB:
            return 3
        return 1

    def cost_at(
        self, tokens: list[ResolvedToken], index: int, *, penalize_inner_break: bool = False
    ) -> int:
        """`tokens[index - 1]` と `tokens[index]` のあいだで切るコストを返す。

        `penalize_inner_break` は A単位で探索するときに True にする。
        C単位の内部を分断する位置にペナルティを加える。
        """
        base = self.cost(tokens[index - 1], tokens[index])
        if penalize_inner_break and not tokens[index].unit_boundary:
            return base + INNER_BREAK_PENALTY
        return base
