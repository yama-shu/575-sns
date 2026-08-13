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

export type RelationList = components["schemas"]["RelationList"];
export type RelationUser = components["schemas"]["RelationUser"];

/** 関係の一覧の種類。`/me/blocks` だけ経路の形が違う。 */
export type RelationKind = "following" | "followers" | "blocks";

/**
 * 関係の一覧を取得する。
 *
 * `handle` はフォロー中・フォロワーのときだけ使う。
 * ブロック中は本人の一覧しか無いため、識別名を取らない。
 */
export function fetchRelationList(
  kind: RelationKind,
  handle: string,
  cursor?: string,
  limit = 20,
): Promise<ApiResult<RelationList>> {
  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set("cursor", cursor);

  const path =
    kind === "blocks"
      ? `/me/blocks?${query}`
      : `/users/${encodeURIComponent(handle)}/${kind}?${query}`;
  return callApi<RelationList>(path);
}

export type PendingReports = components["schemas"]["PendingReports"];
export type PendingReport = components["schemas"]["PendingReport"];

/**
 * 未対応の通報を取得する（S-13）。
 *
 * **古い順に返る。** 待たせている順に処理するため、カーソルの向きも
 * タイムラインと逆になる。
 *
 * 運営でなければ api が 404 を返す。403 にすると運営向けの経路が
 * 存在することを教えることになるため（#74）。
 */
export function fetchPendingReports(
  cursor?: string,
  limit = 20,
): Promise<ApiResult<PendingReports>> {
  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set("cursor", cursor);
  return callApi<PendingReports>(`/admin/reports?${query}`);
}

/** 通報に対応する（投稿を非表示にする）。 */
export function resolveReport(id: string): Promise<ApiResult<undefined>> {
  return callApi<undefined>(`/admin/reports/${encodeURIComponent(id)}/resolve`, {
    method: "POST",
  });
}

/** 通報を却下する。投稿は変わらない。 */
export function rejectReport(id: string): Promise<ApiResult<undefined>> {
  return callApi<undefined>(`/admin/reports/${encodeURIComponent(id)}/reject`, {
    method: "POST",
  });
}
