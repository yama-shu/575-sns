"use client";

/**
 * 通報フォーム（S-12 / FR-05-01）。
 *
 * モーダルと `/posts/:id/report` ページの両方で使う。
 */
import { useActionState } from "react";

import { submitReport, type ReportState } from "@/lib/report-actions";

import styles from "./ReportForm.module.css";

/**
 * 理由の文言。
 *
 * api の `reason` は `spam` / `harassment` / `inappropriate` / `other` である。
 * **日本語の文言は設計に無い**ため、ここで決めている（#64）。
 */
const REASONS: { value: string; label: string }[] = [
  { value: "spam", label: "スパム・宣伝" },
  { value: "harassment", label: "嫌がらせ・攻撃" },
  { value: "inappropriate", label: "不適切な内容" },
  { value: "other", label: "その他" },
];

/** 補足の上限。api の `ReportRequest.comment` と揃える。 */
const COMMENT_MAX = 500;

export function ReportForm({ postId, onDone }: { postId: string; onDone?: () => void }) {
  const [state, action, pending] = useActionState(submitReport, {} as ReportState);

  // **閉じて終わりにしない。** 送れたこと自体を伝える必要がある。
  // 通報は結果がすぐ見える操作ではない。
  if (state.accepted) {
    return (
      <div className={styles.done} role="status">
        <p className={styles.doneTitle}>通報を受け付けました</p>
        <p className={styles.doneHint}>
          運営が内容を確認します。結果の個別の連絡はありません。
        </p>
        {onDone && (
          <button className={styles.close} onClick={onDone} type="button">
            閉じる
          </button>
        )}
      </div>
    );
  }

  return (
    <form action={action} className={styles.form}>
      <input name="id" type="hidden" value={postId} />

      <fieldset className={styles.reasons}>
        <legend className={styles.legend}>理由</legend>
        {REASONS.map((reason, index) => (
          <label className={styles.reason} key={reason.value}>
            {/*
              **既定で選ばない。** `other` のまま送られると運営の判断材料にならない。
              最初の項目にだけ autoFocus を置き、開いたときの行き先を決める。
            */}
            <input
              autoFocus={index === 0}
              name="reason"
              required
              type="radio"
              value={reason.value}
            />
            {reason.label}
          </label>
        ))}
      </fieldset>

      <div className={styles.field}>
        <label className={styles.label} htmlFor="report-comment">
          補足（任意）
        </label>
        <textarea
          className={styles.textarea}
          id="report-comment"
          maxLength={COMMENT_MAX}
          name="comment"
          placeholder="どこが問題かを書くと、確認が早くなります"
          rows={3}
        />
      </div>

      {state.error && (
        <p className={styles.error} role="alert">
          {state.error}
        </p>
      )}

      <div className={styles.actions}>
        {onDone && (
          <button className={styles.cancel} onClick={onDone} type="button">
            やめる
          </button>
        )}
        <button aria-busy={pending} className={styles.submit} disabled={pending} type="submit">
          {pending ? "送っています…" : "通報する"}
        </button>
      </div>
    </form>
  );
}
