"""判定エンジンの制御（詳細設計 01 §1 / 詳細設計 02）。

`ProsodyAnalyzer` は **流れの制御だけ**を行う。モーラの数え方も許容ルールも
区切りコストも知らない。それぞれの計算は協力者に委ねる。

このクラスを読めば、詳細設計 01 §1 のフローチャートがそのまま追える状態を保つ。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Final

from prosody.normalize import normalize
from prosody.reading import ReadingResolver, ResolvedToken
from prosody.segment import Candidate, SegmentSearcher, early_reason
from prosody.token import SplitMode
from prosody.tokenizer import Tokenizer
from prosody.tolerance import EXPECTED_KAMI, EXPECTED_NAKA, EXPECTED_SHIMO
from prosody.verdict import Reason, Verdict

# 上五・中七・下五の規定値
EXPECTED_MORA: Final = (EXPECTED_KAMI, EXPECTED_NAKA, EXPECTED_SHIMO)


@dataclass(frozen=True)
class Segment:
    """区切られた1つの句。"""

    text: str
    """正規化後の本文のうち、この句にあたる部分。

    **前後に空白が含まれることがある。** 利用者が空白で明示的に区切った場合、
    その空白はいずれかの句に属する。3つの `text` を連結すると
    `AnalysisResult.normalized_text` に戻る。この不変条件があるため、
    api は文字数から区切り位置（`break1` / `break2`）を算出できる。
    表示の際に前後の空白を落とすかは表示側の判断とする。
    """

    reading: str
    mora: int
    expected: int
    """期待されるモーラ数（5 / 7 / 5）。"""
    diff: int
    """`mora - expected`。0 なら規定どおり。"""


@dataclass(frozen=True)
class AnalysisResult:
    """判定の結果。"""

    verdict: Verdict
    normalized_text: str
    """正規化した本文。

    区切り位置はこの文字列上の位置である。**api はこちらを保存する。**
    元の入力を保存すると、全角空白の圧縮などで位置がずれる。
    """
    reading: str | None = None
    total_mora: int | None = None
    segments: list[Segment] | None = None
    reason: Reason | None = None
    unreadable: list[str] | None = None

    @classmethod
    def unknown(cls, normalized_text: str, unreadable: list[str]) -> AnalysisResult:
        """読みを確定できなかった場合。

        **破調と区別する。** 読めなかっただけの句に
        「五七五になっていません」と伝えると、直しようがない。
        """
        return cls(
            verdict=Verdict.UNKNOWN,
            normalized_text=normalized_text,
            reason=Reason.READING_UNAVAILABLE,
            unreadable=unreadable,
        )

    @classmethod
    def hacho(
        cls, normalized_text: str, reason: Reason, reading: str, total_mora: int
    ) -> AnalysisResult:
        """許容範囲を超えた場合。

        `segments` は None にする。五七五に区切れないため区切りが定義できない。
        """
        return cls(
            verdict=Verdict.HACHO,
            normalized_text=normalized_text,
            reading=reading,
            total_mora=total_mora,
            reason=reason,
        )


class ProsodyAnalyzer:
    """本文を受け取り、音数律の判定結果を返す。

    **575 の業務ルールを持たない。** 「破調なら投稿を拒否する」の判断は
    api の責務である（基本設計 01 §2）。ここは純粋な変換器に徹する。
    """

    def __init__(
        self,
        tokenizer: Tokenizer,
        resolver: ReadingResolver | None = None,
        searcher: SegmentSearcher | None = None,
    ) -> None:
        self._tokenizer = tokenizer
        self._resolver = resolver or ReadingResolver()
        self._searcher = searcher or SegmentSearcher()

    def analyze(self, text: str) -> AnalysisResult:
        """本文を判定する。"""
        normalized = normalize(text)

        resolved = self._resolver.resolve(self._tokenizer.tokenize(normalized, SplitMode.C))
        if not resolved.is_readable:
            return AnalysisResult.unknown(normalized, resolved.unreadable)

        total = resolved.total_mora
        reason = early_reason(total)
        if reason is not None:
            # 破調の大半はここで弾かれる。無駄な区切り探索を行わない。
            return AnalysisResult.hacho(normalized, reason, resolved.reading, total)

        # 区切りの添字は「探索に使った形態素列」に対するものである。
        # A単位へ落ちた場合は列そのものが変わるため、
        # どちらが使われたかをクロージャで捕まえておく。
        tokens = resolved.tokens

        def refine() -> list[ResolvedToken]:
            nonlocal tokens
            tokens = self._resolver.resolve(
                self._tokenizer.tokenize(normalized, SplitMode.A)
            ).tokens
            return tokens

        best = self._searcher.search_staged(resolved.tokens, refine)
        if best is None:
            return AnalysisResult.hacho(normalized, Reason.NO_VALID_SPLIT, resolved.reading, total)

        return AnalysisResult(
            verdict=best.verdict,
            normalized_text=normalized,
            reading=self._reading_of(tokens),
            total_mora=best.kami + best.naka + best.shimo,
            segments=self._build_segments(tokens, best),
            reason=None,
        )

    # ------------------------------------------------------------------
    # 内部
    # ------------------------------------------------------------------

    def _reading_of(self, tokens: list[ResolvedToken]) -> str:
        return "".join(token.reading for token in tokens)

    def _build_segments(self, tokens: list[ResolvedToken], candidate: Candidate) -> list[Segment]:
        bounds = [
            (0, candidate.kami_end),
            (candidate.kami_end, candidate.naka_end),
            (candidate.naka_end, len(tokens)),
        ]
        morae = (candidate.kami, candidate.naka, candidate.shimo)

        return [
            Segment(
                text="".join(t.surface for t in tokens[start:end]),
                reading="".join(t.reading for t in tokens[start:end]),
                mora=mora,
                expected=expected,
                diff=mora - expected,
            )
            for (start, end), mora, expected in zip(bounds, morae, EXPECTED_MORA, strict=True)
        ]
