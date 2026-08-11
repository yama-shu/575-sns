"use client";

/**
 * 投稿の削除（FR-02-07）。
 *
 * # なぜ二段にするのか
 *
 * **投稿は編集できない。** 消したら取り返しがつかない。
 * 一度の操作で消せると、誤って触ったときに戻せない。
 *
 * **`confirm()` を使わない。** JavaScript が無効な環境では確認が出ないまま
 * 消えてしまう。`<details>` で畳んでおけば、開く操作そのものが確認になり、
 * ブラウザの標準機能だけで成立する。
 */
import { useActionState } from "react";

import { removePost, type ActionState } from "@/lib/relation-actions";

import styles from "./Actions.module.css";

export function DeletePostButton({ postId }: { postId: string }) {
  const [state, action, pending] = useActionState(removePost, {} as ActionState);

  return (
    <details className={styles.delete}>
      <summary className={styles.deleteSummary}>削除</summary>
      <div className={styles.deleteBody}>
        <p className={styles.deleteWarning}>
          消すと元に戻せません。投稿の編集はできないため、詠み直す場合も新しい投稿になります。
        </p>
        <form action={action}>
          <input name="id" type="hidden" value={postId} />
          <button
            aria-busy={pending}
            className={styles.danger}
            disabled={pending}
            type="submit"
          >
            {pending ? "削除しています…" : "この句を削除する"}
          </button>
        </form>
        {state.error && (
          <p className={styles.error} role="alert">
            {state.error}
          </p>
        )}
      </div>
    </details>
  );
}
