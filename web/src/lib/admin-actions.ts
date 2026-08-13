"use server";

/**
 * 通報の処理の Server Action（S-13）。
 */
import { redirect } from "next/navigation";

import { rejectReport, resolveReport } from "./api/user";
import type { ApiResult } from "./api/client";

export type AdminActionState = { error?: string };

/** 通報に対応する（投稿を非表示にする）。 */
export async function resolve(
  _prev: AdminActionState,
  formData: FormData,
): Promise<AdminActionState> {
  return finish(await resolveReport(String(formData.get("id") ?? "")));
}

/** 通報を却下する。 */
export async function reject(
  _prev: AdminActionState,
  formData: FormData,
): Promise<AdminActionState> {
  return finish(await rejectReport(String(formData.get("id") ?? "")));
}

/**
 * 処理の後は、成功でも失敗でも一覧へ戻す。
 *
 * **一覧そのものが変わる。** 処理した通報は消え、同じ投稿への他の通報も
 * まとめて閉じる（#74）。ボタンの周りだけを差し替えても画面が食い違う。
 *
 * **失敗も画面側の状態に持たせない。** すでに処理済みだった場合、
 * 再描画すると対象のカードごと消えるため、カードに紐づけた文言は
 * 出る場所を失う。実際に JavaScript が無い環境で消えることを確認した（#76）。
 * 経路に載せて、一覧の上に出す。
 */
function finish(result: ApiResult<unknown>): AdminActionState {
  if (result.ok) {
    redirect("/admin/reports");
  }
  redirect(`/admin/reports?failed=${encodeURIComponent(result.error.code)}`);
}
