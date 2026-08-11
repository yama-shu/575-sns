"use client";

/**
 * 「詠む」ボタンと、投稿モーダル（S-07）。
 *
 * # なぜリンクでもあるのか
 *
 * [#50](https://github.com/yama-shu/575-sns/issues/50) の「もっと読む」と同じ形にしている。
 * ボタンは `/compose` へのリンクであり、JavaScript が動くときだけ
 * 既定の遷移を止めてモーダルを開く。
 *
 * **詠むことは主要な操作である**（NFR-06-03）。
 * JavaScript が無い環境で詠めないのは許容できない。
 */
import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { BODY_FIELD_ID, ComposeForm } from "./ComposeForm";

import styles from "./ComposeButton.module.css";

export function ComposeButton() {
  const [open, setOpen] = useState(false);
  const dialog = useRef<HTMLDialogElement | null>(null);

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;

    // **showModal を使う。** 背後の要素へのフォーカス移動と操作を
    // ブラウザが止めてくれる（NFR-06-03）。
    if (open && !element.open) {
      element.showModal();
      // **autoFocus では移らない。** React が focus() を呼ぶのは子の副作用の時点で、
      // まだダイアログが開いていない。showModal() はダイアログ自身へ移すため、
      // 開いた後に本文へ移し直す。
      document.getElementById(BODY_FIELD_ID)?.focus();
    }
    if (!open && element.open) element.close();
  }, [open]);

  return (
    <>
      <Link
        className={styles.button}
        href="/compose"
        onClick={(event) => {
          event.preventDefault();
          setOpen(true);
        }}
      >
        詠む
      </Link>

      <dialog
        className={styles.dialog}
        // Esc で閉じたときも状態を合わせる。dialog は自前で閉じるため。
        onClose={() => setOpen(false)}
        ref={dialog}
      >
        {/* 開いたときだけ組み立てる。閉じたあとに入力が残らない。 */}
        {open && (
          <div className={styles.content}>
            <header className={styles.header}>
              <h2 className={styles.title}>詠む</h2>
              <button
                aria-label="閉じる"
                className={styles.close}
                onClick={() => setOpen(false)}
                type="button"
              >
                ✕
              </button>
            </header>
            <ComposeForm onDone={() => setOpen(false)} />
          </div>
        )}
      </dialog>
    </>
  );
}
