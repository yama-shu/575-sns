/**
 * タイムラインの取得。
 */
import "server-only";

import { callApi, type ApiResult } from "./client";
import type { components } from "./schema";

export type Post = components["schemas"]["Post"];
export type Timeline = components["schemas"]["Timeline"];

/** 1ページの件数。api の既定と同じ。 */
export const PAGE_SIZE = 20;

type Kind = "public" | "home";

/**
 * タイムラインを取得する。
 *
 * cursor は api が返した `next_cursor` をそのまま渡す。
 * **web で組み立てない。** カーソルの意味は api が持つ（基本設計 03 §5）。
 */
export function fetchTimeline(kind: Kind, cursor?: string): Promise<ApiResult<Timeline>> {
  const query = new URLSearchParams({ limit: String(PAGE_SIZE) });
  if (cursor) query.set("cursor", cursor);

  return callApi<Timeline>(`/timelines/${kind}?${query.toString()}`, {
    // 全体タイムラインは未ログインでも見られるが、ログイン済みなら
    // ブロックの除外と liked_by_me が効くため、常に Cookie を転送する。
    forwardSession: true,
  });
}
