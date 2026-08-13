"use client";

/**
 * 通報1件と、その処理（S-13）。
 *
 * **`PostCard` を使わない。** あちらにはいいねと通報のボタンが付いている。
 * 運営が通報を裁く画面に、いいねや通報は要らない。
 * 句ごとの改行だけを踏襲する。
 */
import { useActionState } from "react";

import { reject, resolve, type AdminActionState } from "@/lib/admin-actions";
import type { PendingReport } from "@/lib/api/user";
import { RelativeTime } from "./RelativeTime";

import styles from "./ReportCard.module.css";

/** 通報理由の文言。#64 で決めたものと揃える。 */
const REASON_LABEL: Record<string, string> = {
  spam: "スパム・宣伝",
  harassment: "嫌がらせ・攻撃",
  inappropriate: "不適切な内容",
  other: "その他",
};

export function ReportCard({ report }: { report: PendingReport }) {
  // **結果は経路で受け取る。** 失敗しても一覧へ戻るため、カードに紐づけた
  // 文言は出る場所を失う（#76）。状態は押している間の表示にだけ使う。
  const [, resolveAction, resolving] = useActionState(resolve, {} as AdminActionState);
  const [, rejectAction, rejecting] = useActionState(reject, {} as AdminActionState);

  return (
    <article className={styles.card}>
      <header className={styles.header}>
        <span className={styles.reason}>
          {REASON_LABEL[report.reason] ?? report.reason}
        </span>
        <RelativeTime className={styles.time} iso={report.created_at} />
      </header>

      {report.comment && <p className={styles.comment}>{report.comment}</p>}

      <div className={styles.post}>
        <p className={styles.author}>
          {report.post.author.display_name} <span>@{report.post.author.handle}</span>
        </p>
        {/* 句ごとに <p> を並べる。運営も同じ形で読む。 */}
        <div className={styles.body}>
          {report.post.segments.map((segment, index) => (
            <p className={styles.segment} key={index}>
              {segment}
            </p>
          ))}
        </div>
      </div>

      <p className={styles.reporter}>
        通報者: {report.reporter.display_name} @{report.reporter.handle}
      </p>

      <div className={styles.actions}>
        {/* 却下は投稿を変えない。一段でよい。 */}
        <form action={rejectAction}>
          <input name="id" type="hidden" value={report.id} />
          <button
            aria-busy={rejecting}
            className={styles.reject}
            disabled={rejecting || resolving}
            type="submit"
          >
            {rejecting ? "却下しています…" : "問題なし（却下）"}
          </button>
        </form>

        {/*
          対応すると投稿が非表示になり、**画面から戻す手段は無い**。
          一度で実行できると、誤って触ったときに戻せない（#60 の削除と同じ形）。
        */}
        <details className={styles.resolve}>
          <summary className={styles.resolveSummary}>非表示にする</summary>
          <div className={styles.resolveBody}>
            <p className={styles.warning}>
              この句が誰からも見えなくなります。画面から元に戻す手段はありません。
              同じ句への他の通報もまとめて閉じます。
            </p>
            <form action={resolveAction}>
              <input name="id" type="hidden" value={report.id} />
              <button
                aria-busy={resolving}
                className={styles.danger}
                disabled={resolving || rejecting}
                type="submit"
              >
                {resolving ? "処理しています…" : "この句を非表示にする"}
              </button>
            </form>
          </div>
        </details>
      </div>
    </article>
  );
}
