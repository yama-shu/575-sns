/**
 * 判定と投稿の呼び出し。
 */
import "server-only";

import { callApi, type ApiResult } from "./client";
import type { components } from "./schema";
import type { Post } from "./timeline";

export type CheckResult = components["schemas"]["CheckResult"];
export type CheckSegment = components["schemas"]["CheckSegment"];
export type Verdict = components["schemas"]["Verdict"];
export type Visibility = components["schemas"]["Visibility"];

/** 入力中の判定。何も保存しない。 */
export function checkProsody(body: string): Promise<ApiResult<CheckResult>> {
  return callApi<CheckResult>("/prosody/check", { method: "POST", body: { body } });
}

/**
 * 投稿する。
 *
 * **判定結果を送らない。** api は受け取っても読まず、必ず再判定する
 * （基本設計 01 §4 / FR-02-05）。送る実装があると
 * 「クライアントの判定で決まる」という誤解を生むため、型の上でも持たせない。
 */
export function createPost(body: string, visibility: Visibility): Promise<ApiResult<Post>> {
  return callApi<Post>("/posts", { method: "POST", body: { body, visibility } });
}
