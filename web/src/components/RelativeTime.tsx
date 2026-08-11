"use client";

/**
 * 相対時刻（「2時間前」）。
 *
 * **相対時刻をサーバー側で計算しない。** 計算した HTML がキャッシュされると
 * 古い表示が残り、閲覧者との時差でもずれる。絶対時刻を `dateTime` に持たせ、
 * 表示はクライアントで作る。
 *
 * 絶対時刻を属性に残すことで、読み上げと機械可読性も保てる。
 *
 * **絶対時刻の整形はタイムゾーンを固定する。** `toLocaleString("ja-JP")` は
 * 実行環境のタイムゾーンに依存するため、サーバー（コンテナ = UTC）と
 * ブラウザ（JST）で違う文字列になり、ハイドレーションが食い違っていた（#56）。
 *
 *     SSR     : 2026/8/11 6:22:16
 *     ブラウザ : 2026/8/11 15:22:16
 *
 * 絶対時刻は時間が経っても古くならないため、サーバーで整形してよい。
 * 問題は環境依存であることなので、`timeZone` を明示してどこで整形しても
 * 同じ文字列にする。日本語話者向けのサービスであり、閲覧者の地域に
 * 合わせる要件は無い。全員が同じ時刻表記を見る方が、投稿の前後関係を
 * 話題にするときに食い違わない。
 */
import { useSyncExternalStore } from "react";

/** 表示に使うタイムゾーン。実行環境に依存させないために明示する。 */
const TIME_ZONE = "Asia/Tokyo";

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/**
 * 「今」を配る小さなストア。
 *
 * **描画中に `Date.now()` を呼ばない。** 呼び出しごとに違う値を返す関数を
 * 描画で使うと、再描画のたびに結果が変わり、React の純粋性の規則に反する
 * （eslint の react-hooks/purity が検出する）。
 *
 * 値を外に持ち、1分ごとに更新して購読者へ知らせる。
 * 副産物として、画面を開いたままでも表示が古くならない。
 */
const REFRESH_INTERVAL = MINUTE;

let currentTime = Date.now();
const listeners = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | null = null;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  if (timer === null) {
    timer = setInterval(() => {
      currentTime = Date.now();
      for (const notify of listeners) notify();
    }, REFRESH_INTERVAL);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

/** サーバーでは null を返し、絶対時刻を描画する。hydration を食い違わせないため。 */
const getServerSnapshot = () => null;

/** 絶対時刻を日本語の相対表現にする。 */
function toRelative(iso: string, now: number): string {
  const elapsed = now - new Date(iso).getTime();

  if (elapsed < MINUTE) return "たった今";
  if (elapsed < HOUR) return `${Math.floor(elapsed / MINUTE)}分前`;
  if (elapsed < DAY) return `${Math.floor(elapsed / HOUR)}時間前`;
  if (elapsed < 7 * DAY) return `${Math.floor(elapsed / DAY)}日前`;

  return new Date(iso).toLocaleDateString("ja-JP", {
    year: "numeric",
    month: "long",
    day: "numeric",
    timeZone: TIME_ZONE,
  });
}

export function RelativeTime({ iso, className }: { iso: string; className?: string }) {
  const now = useSyncExternalStore(subscribe, () => currentTime, getServerSnapshot);
  const absolute = new Date(iso).toLocaleString("ja-JP", { timeZone: TIME_ZONE });

  return (
    <time className={className} dateTime={iso} title={absolute}>
      {now === null ? absolute : toRelative(iso, now)}
    </time>
  );
}
