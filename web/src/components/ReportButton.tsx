"use client";

/**
 * 「通報」と、そのモーダル（S-12）。
 *
 * **リンクでもある。** JavaScript が動くときだけ既定の遷移を止めてモーダルを開く。
 * 無効な環境では `/posts/:id/report` のページへ行く（#52 の「詠む」と同じ形）。
 */
import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { ReportForm } from "./ReportForm";

import styles from "./ReportForm.module.css";

export function ReportButton({ postId }: { postId: string }) {
  const [open, setOpen] = useState(false);
  const dialog = useRef<HTMLDialogElement | null>(null);

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    if (!open && element.open) element.close();
  }, [open]);

  return (
    <>
      <Link
        className={styles.trigger}
        href={`/posts/${postId}/report`}
        onClick={(event) => {
          event.preventDefault();
          setOpen(true);
        }}
      >
        通報
      </Link>

      <dialog className={styles.dialog} onClose={() => setOpen(false)} ref={dialog}>
        {/* 開いたときだけ組み立てる。閉じたあとに入力が残らない。 */}
        {open && (
          <div className={styles.content}>
            <h2 className={styles.title}>この句を通報する</h2>
            <ReportForm onDone={() => setOpen(false)} postId={postId} />
          </div>
        )}
      </dialog>
    </>
  );
}
