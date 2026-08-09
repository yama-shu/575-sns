// prosody の判定 API に負荷をかける（#9 / NFR-01-01）。
//
// **同じ文を繰り返さない。** 250 件の互いに異なる本文を順に送る。
// 同じ文を送り続けると、辞書の内部キャッシュが効いて実態より速く見える。
//
// 3つのシナリオを続けて実行する。目的が異なるため分けている。
//
//   cold : 250 件をちょうど1周だけ流す（1件も重複しない）
//   rate : 一定のレートで流し、**想定負荷での応答時間**を測る ← NFR-01-01 はこれ
//   load : 待ち時間なしで限界まで流し、**捌ける上限**を測る（飽和試験）
//
// load だけを見て応答時間を語ってはならない。待ち時間なしの constant-vus は
// サーバが飽和するまで投げ続けるため、得られるのは「限界での待ち行列」であって
// 想定負荷での応答時間ではない。NFR-01-01 は rate シナリオで判断する。

import http from "k6/http";
import { check } from "k6";
import { SharedArray } from "k6/data";
import { Trend } from "k6/metrics";

const URL = __ENV.TARGET_URL || "http://prosody:8000/v1/analyze";
const CONNECTIONS = Number(__ENV.CONNECTIONS || 50);
const DURATION = __ENV.DURATION || "30s";
// 想定負荷。判定はデバウンス 300ms を前提とし、レート制限は 60回/分/ユーザー
// （基本設定 05）。同時接続 1,000（NFR-01-03）のうち一部が同時に推敲していると
// 見て 100 req/s を想定負荷とする。
const RATE = Number(__ENV.RATE || 100);

// SharedArray は全 VU で1つの実体を共有する。VU ごとに複製するとメモリを食う。
const inputs = new SharedArray("inputs", () => JSON.parse(open("./inputs.json")));

// 種別ごとの応答時間。最悪ケース（A単位へ落ちる文）が
// 全体を押し上げていないかを見る。
const byKind = {
  teikei: new Trend("latency_teikei", true),
  kyoyo: new Trend("latency_kyoyo", true),
  early_reject: new Trend("latency_early_reject", true),
  fallback_to_a: new Trend("latency_fallback_to_a", true),
};

export const options = {
  scenarios: {
    cold: {
      executor: "shared-iterations",
      vus: CONNECTIONS,
      iterations: inputs.length, // ちょうど1周。1件も重複しない。
      maxDuration: "2m",
      exec: "once",
      tags: { scenario: "cold" },
    },
    rate: {
      executor: "constant-arrival-rate",
      rate: RATE,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: CONNECTIONS,
      maxVUs: CONNECTIONS * 2,
      startTime: "10s", // cold の完了を待つ
      exec: "loop",
      tags: { scenario: "rate" },
    },
    load: {
      executor: "constant-vus",
      vus: CONNECTIONS,
      duration: DURATION,
      startTime: "50s", // rate の完了を待つ
      exec: "loop",
      tags: { scenario: "load" },
    },
  },
  thresholds: {
    // NFR-01-01: 判定のレスポンスタイムは P95 < 150ms（想定負荷での値）
    "http_req_duration{scenario:rate}": ["p(95)<150"],
    // 想定負荷では1件も失敗してはならない
    "http_req_failed{scenario:rate}": ["rate==0"],
  },
  summaryTrendStats: ["avg", "min", "med", "p(50)", "p(95)", "p(99)", "max"],
};

function send(entry) {
  const response = http.post(URL, JSON.stringify({ text: entry.text }), {
    headers: { "Content-Type": "application/json" },
  });

  check(response, {
    "status is 200": (r) => r.status === 200,
    "verdict is present": (r) => {
      try {
        return typeof r.json("verdict") === "string";
      } catch {
        return false;
      }
    },
  });

  const trend = byKind[entry.kind];
  if (trend) {
    trend.add(response.timings.duration);
  }
}

// cold: 全 VU で 250 件を分け合い、1件も重複させない
export function once() {
  send(inputs[__ITER % inputs.length]);
}

// load: 250 件を周回する
export function loop() {
  send(inputs[(__VU * 7919 + __ITER) % inputs.length]);
}
