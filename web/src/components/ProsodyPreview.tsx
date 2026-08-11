"use client";

/**
 * 判定結果の表示（基本設計 04 §3）。
 *
 * # なぜ区切りとモーラ数を必ず見せるのか
 *
 * 判定エンジンは 100% 正確にはなり得ない（要件定義 3.2）。
 * 区切りとモーラ数を隠すと、破調と判定されたときに
 * **なぜ弾かれたのか分からず、直しようがない**。
 *
 * 読みも見せる。システムが「一日」を イチニチ と読んだのか
 * ツイタチ と読んだのかが分かれば、モーラ数が合わない理由にたどり着ける。
 */
import { shortfall, TOTAL_MORA } from "@/lib/post-rules";
import type { CheckResult, CheckSegment } from "@/lib/api/post";
import type { ProsodyStatus } from "./useProsodyCheck";

import styles from "./ProsodyPreview.module.css";

/**
 * 判定ごとのバッジ（基本設計 04 §3 の表）。
 *
 * 投稿できるかはここで決めない。`isPostable` が持つ。
 */
const BADGE: Record<string, { label: string; tone: string }> = {
  teikei: { label: "✅ 定型", tone: "teikei" },
  kyoyo: { label: "🔵 許容", tone: "kyoyo" },
  hacho: { label: "⚠️ 破調", tone: "hacho" },
  unknown: { label: "❓ 読み方が分かりません", tone: "unknown" },
};

/** 句の呼び名。ズレている場所を指すのに使う。 */
const PHRASE_NAMES = ["上五", "中七", "下五"];

export function ProsodyPreview({ status }: { status: ProsodyStatus }) {
  if (status.kind === "empty") {
    return (
      <div className={styles.panel}>
        <p className={styles.hint}>五七五で詠むと、区切りと音数がここに出ます。</p>
      </div>
    );
  }

  if (status.kind === "checking") {
    return (
      <div className={styles.panel} aria-busy="true">
        <p className={styles.hint}>判定中…</p>
      </div>
    );
  }

  if (status.kind === "unavailable") {
    // **破調と区別する。** 判定できていないだけであり、句が悪いとは限らない。
    return (
      <div className={`${styles.panel} ${styles.unknown}`} role="status">
        <p className={styles.badge}>⚠️ 判定できません</p>
        <p className={styles.reason}>{status.message}</p>
      </div>
    );
  }

  return <Result result={status.result} />;
}

function Result({ result }: { result: CheckResult }) {
  const badge = BADGE[result.verdict];

  return (
    <div className={`${styles.panel} ${styles[badge?.tone ?? "unknown"]}`} role="status">
      {result.segments ? (
        <Segments segments={result.segments} />
      ) : (
        <p className={styles.reading}>{result.reading}</p>
      )}

      <p className={styles.badge}>{label(result) ?? result.verdict}</p>
      <Reason result={result} />
    </div>
  );
}

/**
 * バッジの文言。
 *
 * 許容は**どちらにズレているか**まで出す（基本設計 04 §3 の
 * 「🔵 字余り / 字足らず」）。「許容」とだけ出しても何が起きたか分からない。
 */
function label(result: CheckResult): string | undefined {
  if (result.verdict !== "kyoyo" || result.total_mora === null) return BADGE[result.verdict]?.label;
  if (result.total_mora > TOTAL_MORA) return "🔵 字余り";
  if (result.total_mora < TOTAL_MORA) return "🔵 字足らず";
  // 17音に収まっているが、いずれかの句が規定とずれている。
  return "🔵 許容";
}

/** 句ごとの本文・読み・モーラ数。ズレている句を強調する。 */
function Segments({ segments }: { segments: CheckSegment[] }) {
  return (
    <ol className={styles.segments}>
      {segments.map((segment, index) => (
        <li
          className={`${styles.segment} ${segment.diff === 0 ? "" : styles.segmentOff}`}
          key={index}
        >
          <span className={styles.segmentText}>{segment.text}</span>
          <span className={styles.segmentReading}>{segment.reading}</span>
          <span className={styles.segmentMora}>
            {segment.mora}音
            {segment.diff !== 0 && (
              <span className={styles.segmentDiff}>
                （{PHRASE_NAMES[index] ?? ""}は{Math.abs(segment.diff)}音
                {segment.diff > 0 ? "多い" : "足りない"}）
              </span>
            )}
          </span>
        </li>
      ))}
    </ol>
  );
}

/**
 * 直すための手がかりを出す。
 *
 * 「五七五になっていません」だけで終わらせない（基本設計 04 §3 の「❌ 悪い例」）。
 */
function Reason({ result }: { result: CheckResult }) {
  if (result.verdict === "unknown") {
    const words = result.unreadable ?? [];
    return (
      <p className={styles.reason}>
        {words.length === 0
          ? "読み方を確定できませんでした。別の表記でお試しください。"
          : `「${words.join("」「")}」の読み方が分かりませんでした。別の表記でお試しください。`}
      </p>
    );
  }

  if (result.verdict === "hacho") {
    // 五七五に区切れないため区切りが無い。全体の音数から手がかりを出す。
    return (
      <p className={styles.reason}>
        {result.total_mora}音です。{shortfall(result.total_mora ?? 0)}。
      </p>
    );
  }

  return <p className={styles.reason}>全体で{result.total_mora}音です。</p>;
}
