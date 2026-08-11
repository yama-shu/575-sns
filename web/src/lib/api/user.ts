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

export type ProfileUpdated = components["schemas"]["ProfileUpdated"];

/**
 * プロフィールを更新する（FR-01-03）。
 *
 * **省略と空文字を区別する。** 触れていない項目は送らない。
 * 空文字を送るとその項目が消える。自己紹介は消せなければならない。
 */
export function updateProfile(body: {
  display_name?: string;
  bio?: string;
}): Promise<ApiResult<ProfileUpdated>> {
  return callApi<ProfileUpdated>("/me/profile", { method: "PATCH", body });
}

export type ReportReason = components["schemas"]["ReportReason"];
export type Report = components["schemas"]["Report"];

/**
 * 投稿を通報する（FR-05-01）。
 *
 * **冪等ではない。** 同じ投稿への2回目は 409 が返る。
 * 黙って成功にすると「通報が届いた」と誤解する（#36）。
 */
export function reportPost(
  postId: string,
  reason: ReportReason,
  comment: string,
): Promise<ApiResult<Report>> {
  return callApi<Report>(`/posts/${encodeURIComponent(postId)}/report`, {
    method: "POST",
    body: comment === "" ? { reason } : { reason, comment },
  });
}
