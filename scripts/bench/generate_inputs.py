"""ベンチマークの入力データを生成する。

#9 の測定の設計にもとづき、種別ごとに**互いに異なる**本文を集める。
同じ文を繰り返すと、辞書の内部キャッシュが効いて実態より速く見える。

語を組み合わせて候補を作り、実際に判定器へ通して種別を確定させる。
手で書くと種別の判定が推測になり、再現もできない。

出力は本文と種別の JSON 配列。k6 が読み込んで負荷に使う。

    ./scripts/bench/run.sh  から呼ばれる
"""

from __future__ import annotations

import argparse
import itertools
import json
import random
import sys

from prosody.analyzer import ProsodyAnalyzer
from prosody.segment import SegmentSearcher, early_reason
from prosody.sudachi import SudachiTokenizer
from prosody.token import SplitMode
from prosody.verdict import Reason, Verdict

# 日常のつぶやきに現れそうな語を、句のまとまりとして持つ。
# 個々のモーラ数は判定器に数えさせるため、ここでは指定しない。
OPENING = [
    "今日もまた",
    "朝からずっと",
    "昼休みには",
    "帰り道には",
    "気がつけばもう",
    "目が覚めたら",
    "雨降る朝は",
    "風が冷たい",
    "空が青くて",
    "月が綺麗で",
    "電車の中で",
    "机の上に",
    "コーヒー片手に",
    "画面の前で",
    "静かな夜に",
    "眠れないまま",
    "締切前に",
    "資料を抱えて",
    "会議のあとに",
    "駅のホームで",
]

MIDDLE = [
    "会議のための",
    "終わらぬ仕事を",
    "積まれた書類が",
    "鳴り止まぬ通知",
    "同じ話を",
    "また繰り返す",
    "誰も聞かない",
    "答えの出ない",
    "小さな幸せ",
    "遠い記憶が",
    "見慣れた景色が",
    "変わらぬ日々に",
    "少しの勇気を",
    "重たいカバンを",
    "冷めたお茶を",
    "眠気と戦い",
    "電池の切れた",
    "指先だけが",
    "止まらぬ雨に",
    "答えを探して",
]

CLOSING = [
    "会議かな",
    "見つめてる",
    "笑ってた",
    "眠りたい",
    "帰りたい",
    "考える",
    "立ち尽くす",
    "沈んでく",
    "待っている",
    "動き出す",
    "染みてくる",
    "抱えてる",
    "溶けていく",
    "祈るだけ",
    "空を見る",
    "息を吐く",
    "歩き出す",
    "手を伸ばす",
    "夢を見る",
    "耐えている",
]

# C単位が粗すぎて区切れない文をつくるための複合語。
# Sudachi は C単位でこれらを1つの形態素にまとめるため、
# 句の境界が複合語の内部に落ちると C単位では区切れず、A単位へ落ちる。
COMPOUNDS = [
    "選挙管理委員会",
    "東京特許許可局",
    "個人情報保護法",
    "情報処理技術者",
    "環境問題対策室",
    "高速道路交通情報",
    "電気通信事業者",
    "自動車運転免許",
    "総合病院受付",
    "国際連合本部",
    "地域包括支援",
    "危機管理対策本部",
    "労働基準監督署",
    "文化財保護委員",
    "宇宙航空研究所",
]

# 複合語の前後に置く短い語。組み合わせの幅を広げ、
# 句の境界が複合語の内部に落ちる配置を探す。
PARTICLES = ["の", "は", "も", "を", "と", "で", "に", "が", "から", "まで"]
TAILS = [
    "かな",
    "だろう",
    "らしい",
    "だった",
    "みたい",
    "なのか",
    "ですね",
    "でした",
    "かもね",
    "そうだ",
]

# 総モーラ数で弾かれる文（早期リターンの確認用）
TOO_SHORT = [
    "今日は疲れた",
    "眠い",
    "腹が減った",
    "帰りたい",
    "会議が多い",
    "雨が降る",
    "電車が遅れた",
    "コーヒーが苦い",
    "眠れない夜",
    "書類の山",
]
TOO_LONG_PARTS = [
    "今日は本当に疲れたので早く家に帰ってゆっくり休みたいと思っている",
    "朝から晩まで会議が続いて結局自分の仕事はまったく進まなかった一日だった",
    "積み上がった書類の山を眺めながらどこから手を付けるべきか悩み続けている",
]


def classify(analyzer: ProsodyAnalyzer, searcher: SegmentSearcher, text: str) -> str:
    """本文を測定用の種別に分ける。

    「C単位で失敗し A単位で成功する文」は最悪ケース（2回解析する）であり、
    判定結果からは区別できない。C単位だけで探索して確かめる。
    """
    result = analyzer.analyze(text)

    if result.verdict is Verdict.HACHO:
        if result.reason in (Reason.TOO_FEW_MORA, Reason.TOO_MANY_MORA):
            return "early_reject"
        return "no_valid_split"
    if result.verdict is Verdict.UNKNOWN:
        return "unknown"

    # C単位だけで区切れるかを確かめ、A単位へ落ちたものを最悪ケースとする
    tokenizer: SudachiTokenizer = analyzer._tokenizer  # type: ignore[assignment]
    resolver = analyzer._resolver  # type: ignore[attr-defined]
    coarse = resolver.resolve(tokenizer.tokenize(result.normalized_text, SplitMode.C))
    if early_reason(coarse.total_mora) is None and searcher.best_for(coarse.tokens) is None:
        return "fallback_to_a"

    return "teikei" if result.verdict is Verdict.TEIKEI else "kyoyo"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--seed", type=int, default=575, help="生成を再現するための種")
    args = parser.parse_args()

    random.seed(args.seed)
    analyzer = ProsodyAnalyzer(SudachiTokenizer())
    searcher = SegmentSearcher()

    buckets: dict[str, list[str]] = {
        "teikei": [],
        "kyoyo": [],
        "early_reject": [],
        "fallback_to_a": [],
    }
    wanted = {"teikei": 100, "kyoyo": 50, "early_reject": 50, "fallback_to_a": 50}

    # 3つの句の組み合わせと、複合語を挟んだ組み合わせを総当たりし、
    # 判定器で種別を確定させる
    combinations = [a + b + c for a, b, c in itertools.product(OPENING, MIDDLE, CLOSING)]
    combinations += [
        a + b + c + d
        for a, b, c, d in itertools.product(OPENING, COMPOUNDS, PARTICLES, TAILS)
    ]
    combinations += [
        a + b + c for a, b, c in itertools.product(COMPOUNDS, PARTICLES, CLOSING)
    ]
    random.shuffle(combinations)

    for text in combinations:
        kind = classify(analyzer, searcher, text)
        if kind in buckets and len(buckets[kind]) < wanted[kind]:
            buckets[kind].append(text)
        if all(len(buckets[k]) >= wanted[k] for k in wanted):
            break

    # 総モーラ数で弾かれる文は、短い側と長い側の両方を混ぜる
    for text in TOO_SHORT + TOO_LONG_PARTS:
        if len(buckets["early_reject"]) < wanted["early_reject"]:
            buckets["early_reject"].append(text)

    for kind, count in wanted.items():
        actual = len(buckets[kind])
        print(f"{kind:<16} {actual:>4} / {count}", file=sys.stderr)
        if actual < count:
            print(f"  ! {kind} が {count} 件に届かなかった", file=sys.stderr)

    entries = [
        {"text": text, "kind": kind} for kind, bucket in buckets.items() for text in bucket
    ]
    texts = [e["text"] for e in entries]
    assert len(texts) == len(set(texts)), "同じ文が重複している"
    random.shuffle(entries)

    print(json.dumps(entries, ensure_ascii=False, indent=2))
    print(f"合計 {len(entries)} 件（すべて異なる）", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
