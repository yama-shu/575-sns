"use server";

/**
 * タイムラインの続きを取る Server Action。
 *
 * ブラウザは web としか通信しない（基本設計 01 §6）。
 */
import { fetchTimeline, type Post } from "./api/timeline";

export type MoreResult = {
  posts: Post[];
  nextCursor: string | null;
  /** 取得に失敗したときの文言。成功時は undefined。 */
  error?: string;
};

export async function loadMore(
  kind: "public" | "home",
  cursor: string,
): Promise<MoreResult> {
  const result = await fetchTimeline(kind, cursor);
  if (!result.ok) {
    return { posts: [], nextCursor: cursor, error: "続きを読み込めませんでした" };
  }
  return { posts: result.data.items, nextCursor: result.data.next_cursor };
}
