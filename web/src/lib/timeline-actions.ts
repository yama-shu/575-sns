"use server";

/**
 * タイムラインの続きを取る Server Action。
 *
 * ブラウザは web としか通信しない（基本設計 01 §6）。
 */
import { fetchTimeline, type Post } from "./api/timeline";
import { fetchUserPosts } from "./api/user";

/**
 * どの一覧の続きを取るか。
 *
 * 利用者ぶんは `user:<識別名>` の形にする。**別の Server Action を作らない。**
 * 無限スクロールと「もっと読む」の両立を2箇所に書くと、片方だけ直したときに食い違う。
 */
export type TimelineKind = "public" | "home" | `user:${string}`;

export type MoreResult = {
  posts: Post[];
  nextCursor: string | null;
  /** 取得に失敗したときの文言。成功時は undefined。 */
  error?: string;
};

export async function loadMore(kind: TimelineKind, cursor: string): Promise<MoreResult> {
  const handle = kind.startsWith("user:") ? kind.slice("user:".length) : null;
  const result = handle
    ? await fetchUserPosts(handle, cursor)
    : await fetchTimeline(kind as "public" | "home", cursor);
  if (!result.ok) {
    return { posts: [], nextCursor: cursor, error: "続きを読み込めませんでした" };
  }
  return { posts: result.data.items, nextCursor: result.data.next_cursor };
}
