"""処理段階ごとの所要時間を測る。

HTTP を挟まずプロセス内で測る。ボトルネックが判定エンジンのどこにあるかを
知るのが目的であり、ネットワークやサーバの寄与を混ぜたくない。

    正規化 / 形態素解析(C) / 読み解決 + モーラ計算 / 区切り探索 / 形態素解析(A)

**同じ文を繰り返さない。** 入力は generate_inputs.py と同じ生成器を使う。
"""

from __future__ import annotations

import argparse
import json
import statistics
import sys
import time
from collections.abc import Callable

from prosody.normalize import normalize
from prosody.reading import ReadingResolver
from prosody.segment import SegmentSearcher, early_reason
from prosody.sudachi import SudachiTokenizer
from prosody.token import SplitMode


def measure(label: str, call: Callable[[], object], repeat: int) -> dict[str, float]:
    """1件あたりの所要時間をミリ秒で返す。"""
    samples: list[float] = []
    for _ in range(repeat):
        started = time.perf_counter()
        call()
        samples.append((time.perf_counter() - started) * 1000)
    samples.sort()
    return {
        "label": label,  # type: ignore[dict-item]
        "mean_ms": round(statistics.fmean(samples), 3),
        "p50_ms": round(samples[len(samples) // 2], 3),
        "p95_ms": round(samples[int(len(samples) * 0.95)], 3),
        "max_ms": round(samples[-1], 3),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repeat", type=int, default=200, help="1段階あたりの試行回数")
    args = parser.parse_args()

    started = time.perf_counter()
    tokenizer = SudachiTokenizer()
    dictionary_load_ms = (time.perf_counter() - started) * 1000

    resolver = ReadingResolver()
    searcher = SegmentSearcher()

    # 代表的な入力。定型（C単位で完結）と最悪ケース（A単位へ落ちる）を分けて測る。
    teikei = "今日もまた会議のための会議かな"
    fallback = "選挙管理委員会の担当かな"

    results = []

    for name, text in (("定型", teikei), ("最悪ケース", fallback)):
        normalized = normalize(text)
        coarse_tokens = tokenizer.tokenize(normalized, SplitMode.C)
        resolved = resolver.resolve(coarse_tokens)

        results.append(measure(f"{name}/正規化", lambda t=text: normalize(t), args.repeat))
        results.append(
            measure(
                f"{name}/形態素解析(C単位)",
                lambda n=normalized: tokenizer.tokenize(n, SplitMode.C),
                args.repeat,
            )
        )
        results.append(
            measure(
                f"{name}/読み解決+モーラ計算",
                lambda c=coarse_tokens: resolver.resolve(c),
                args.repeat,
            )
        )
        results.append(
            measure(
                f"{name}/区切り探索",
                lambda r=resolved: searcher.best_for(r.tokens),
                args.repeat,
            )
        )
        results.append(
            measure(
                f"{name}/形態素解析(A単位)",
                lambda n=normalized: tokenizer.tokenize(n, SplitMode.A),
                args.repeat,
            )
        )
        results.append(
            measure(
                f"{name}/早期判定",
                lambda r=resolved: early_reason(r.total_mora),
                args.repeat,
            )
        )

    payload = {
        "dictionary_load_ms": round(dictionary_load_ms, 1),
        "repeat": args.repeat,
        "stages": results,
    }
    print(json.dumps(payload, ensure_ascii=False, indent=2))

    print("\n段階ごとの平均（ミリ秒）", file=sys.stderr)
    for row in results:
        print(f"  {row['label']:<34} {row['mean_ms']:>8.3f}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
