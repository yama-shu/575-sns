"""字余り・字足らずの許容ルール。

ADR-0001 の決定そのものを実装する。

    1. 上五が 4〜6、中七が 6〜8、下五が 4〜6 モーラであること
    2. 規定値（5 / 7 / 5）から外れている句が、多くとも1句であること

**条件は2つある。1つではない。** 条件1 だけを実装すると `4/6/5`
（各句は範囲内だがズレが2つ）のような句を通してしまい、
ADR-0001 が「甘すぎる」として却下した案2 を実装したことになる。

**合計モーラ数のチェックは書かない。** ズレる句が高々1つである以上、
他の2句は規定値ちょうどになるため、合計は必然的に 17±1 に収まる。
上記2条件から導出できるものを第3の条件として書くと、条件が重複し、
片方だけを直したときに矛盾が生じる（ADR-0001 の補足）。

本モジュールは ADR-0001 が「リリース後の実データを見てから緩和を判断するのが望ましい」
と述べているとおり、**将来変更される可能性が最も高い部分**である。
そのため区切り探索から分離した独立のモジュールとして置く（詳細設計 02）。
"""

from __future__ import annotations

from typing import Final

from prosody.verdict import Verdict

# 各句の規定値（上五 / 中七 / 下五）
EXPECTED_KAMI: Final = 5
EXPECTED_NAKA: Final = 7
EXPECTED_SHIMO: Final = 5

# 各句の許容範囲（下限・上限を含む）
RANGE_KAMI: Final = (4, 6)
RANGE_NAKA: Final = (6, 8)
RANGE_SHIMO: Final = (4, 6)

# 規定値から外れてよい句の数
MAX_DEVIATIONS: Final = 1


class ToleranceRule:
    """3つの句のモーラ数を受け取り、投稿できるかを判定する。

    状態を持たないため、どのインスタンスも同じ結果を返す。
    """

    def deviation_count(self, kami: int, naka: int, shimo: int) -> int:
        """規定値（5 / 7 / 5）から外れている句の数を返す。

        **範囲内かどうかは見ない。** 範囲の判定は `is_valid` の責務であり、
        ここでは「いくつの句が規定値からずれているか」だけを数える。
        """
        return (kami != EXPECTED_KAMI) + (naka != EXPECTED_NAKA) + (shimo != EXPECTED_SHIMO)

    def is_valid(self, kami: int, naka: int, shimo: int) -> bool:
        """投稿できる区切りかを返す（定型または許容）。"""
        # 条件1: 各句が許容範囲に収まる
        if not (
            RANGE_KAMI[0] <= kami <= RANGE_KAMI[1]
            and RANGE_NAKA[0] <= naka <= RANGE_NAKA[1]
            and RANGE_SHIMO[0] <= shimo <= RANGE_SHIMO[1]
        ):
            return False
        # 条件2: 規定値から外れている句が高々1つ ← これを忘れやすい
        return self.deviation_count(kami, naka, shimo) <= MAX_DEVIATIONS

    def verdict_of(self, kami: int, naka: int, shimo: int) -> Verdict:
        """判定を返す。

        `UNKNOWN` は読みを確定できない場合の判定であり、
        モーラ数が確定している本メソッドでは返さない。
        """
        if not self.is_valid(kami, naka, shimo):
            return Verdict.HACHO
        if self.deviation_count(kami, naka, shimo) == 0:
            return Verdict.TEIKEI
        return Verdict.KYOYO
