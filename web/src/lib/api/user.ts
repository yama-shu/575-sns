/**
 * プロフィール・関係の操作。
 */
import "server-only";

import { callApi, type ApiResult } from "./client";
import type { components } from "./schema";
import type { Timeline } from "./timeline";

export type Profile = components["schemas"]["Profile"];
export type FollowState = components["schemas"]["FollowState"];
export type BlockState = components["schemas"]["BlockState"];
export type LikeState = components["schemas"]["LikeState"];

/** プロフィールを取得する。見えない相手は 404 が返る。 */
export function fetchProfile(handle: string): Promise<ApiResult<Profile>> {
  return callApi<Profile>(`/users/${encodeURIComponent(handle)}`);
}

/** ある利用者の投稿一覧を取得する。 */
export function fetchUserPosts(handle: string, cursor?: string, limit = 20) {
  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set("cursor", cursor);
  return callApi<Timeline>(`/users/${encodeURIComponent(handle)}/posts?${query}`);
}

/** フォローする／解除する。**冪等**（基本設計 05 §2）。 */
export function setFollow(handle: string, following: boolean): Promise<ApiResult<FollowState>> {
  return callApi<FollowState>(`/users/${encodeURIComponent(handle)}/follow`, {
    method: following ? "PUT" : "DELETE",
  });
}

/** ブロックする／解除する。 */
export function setBlock(handle: string, blocked: boolean): Promise<ApiResult<BlockState>> {
  return callApi<BlockState>(`/users/${encodeURIComponent(handle)}/block`, {
    method: blocked ? "PUT" : "DELETE",
  });
}

/** いいねする／取り消す。 */
export function setLike(postId: string, liked: boolean): Promise<ApiResult<LikeState>> {
  return callApi<LikeState>(`/posts/${encodeURIComponent(postId)}/like`, {
    method: liked ? "PUT" : "DELETE",
  });
}

/** 投稿を取得する。見えない投稿は 404 が返る。 */
export function fetchPost(id: string) {
  return callApi<components["schemas"]["Post"]>(`/posts/${encodeURIComponent(id)}`);
}

/** 投稿を削除する。削除できるのは投稿者本人だけ（BR-03）。 */
export function deletePost(id: string): Promise<ApiResult<undefined>> {
  return callApi<undefined>(`/posts/${encodeURIComponent(id)}`, { method: "DELETE" });
}
