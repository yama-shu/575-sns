"use client";

/**
 * いいねボタン（FR-04-01）。
 *
 * **`<form>` で作る。** JavaScript が無効でも押せる（NFR-06-03）。
 * 押すと api を呼び、ページを描画し直す。数は api が返した値がそのまま出る。
 */
import { useActionState } from "react";
import Link from "next/link";

import { toggleLike, type LikeActionState } from "@/lib/relation-actions";

import styles from "./Actions.module.css";

type Props = {
  postId: string;
  likeCount: number;
  likedByMe: boolean;
  /** ログインしているか。未ログインならログインへ誘導する。 */
  signedIn: boolean;
};

export function LikeButton({ postId, likeCount, likedByMe, signedIn }: Props) {
  const [state, action, pending] = useActionState(toggleLike, {} as LikeActionState);

  // **api が返した値を優先する。** 押す前の値は初回の描画にだけ使う。
  const liked = state.liked ?? likedByMe;
  const count = state.likeCount ?? likeCount;

  // 未ログインでは押させない。押してから 401 で弾くより、
  // 何をすればよいかを先に示す（NFR-06-02）。
  if (!signedIn) {
    return (
      <Link className={styles.likeSignedOut} href="/login">
        ♡ {likeCount}
      </Link>
    );
  }

  return (
    <form action={action} className={styles.inline}>
      <input name="id" type="hidden" value={postId} />
      <input name="liked" type="hidden" value={String(liked)} />
      <button
        aria-busy={pending}
        aria-label={liked ? "いいねを取り消す" : "いいねする"}
        aria-pressed={liked}
        className={`${styles.like} ${liked ? styles.liked : ""}`}
        disabled={pending}
        type="submit"
      >
        {liked ? "♥" : "♡"} {count}
      </button>
      {state.error && (
        <span className={styles.error} role="alert">
          {state.error}
        </span>
      )}
    </form>
  );
}
