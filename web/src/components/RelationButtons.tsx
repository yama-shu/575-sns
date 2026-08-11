"use client";

/**
 * フォローとブロックのボタン（S-04）。
 *
 * **`<form>` で作る。** JavaScript が無効でも操作できる（NFR-06-03）。
 */
import { useActionState } from "react";

import { toggleBlock, toggleFollow, type ActionState } from "@/lib/relation-actions";

import styles from "./Actions.module.css";

type Props = {
  handle: string;
  following: boolean;
  blocking: boolean;
  /** 操作のあとに戻る経路。ページごと描き直すために渡す。 */
  path: string;
};

export function RelationButtons({ handle, following, blocking, path }: Props) {
  const [followState, followAction, followPending] = useActionState(toggleFollow, {} as ActionState);
  const [blockState, blockAction, blockPending] = useActionState(toggleBlock, {} as ActionState);

  return (
    <div className={styles.relations}>
      {/*
        ブロック中はフォローを出さない。api は「ブロックしている相手はフォローできない」
        として 422 を返すため、押せるボタンを置くと必ず失敗する。
      */}
      {!blocking && (
        <form action={followAction}>
          <input name="handle" type="hidden" value={handle} />
          <input name="following" type="hidden" value={String(following)} />
          <input name="path" type="hidden" value={path} />
          <button
            aria-busy={followPending}
            className={following ? styles.secondary : styles.primary}
            disabled={followPending}
            type="submit"
          >
            {following ? "フォロー中" : "フォローする"}
          </button>
        </form>
      )}

      <form action={blockAction}>
        <input name="handle" type="hidden" value={handle} />
        <input name="blocking" type="hidden" value={String(blocking)} />
        <input name="path" type="hidden" value={path} />
        <button
          aria-busy={blockPending}
          className={styles.quiet}
          disabled={blockPending}
          type="submit"
        >
          {blocking ? "ブロックを解除" : "ブロックする"}
        </button>
      </form>

      {(followState.error ?? blockState.error) && (
        <p className={styles.error} role="alert">
          {followState.error ?? blockState.error}
        </p>
      )}
    </div>
  );
}
