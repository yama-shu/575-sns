"use server";

/**
 * 入力中の判定と、投稿の Server Action。
 *
 * ブラウザは web としか通信しない（基本設計 01 §6）。
 *
 * # なぜ Route Handler ではないか
 *
 * 当初は「Server Action は呼び出しのたびにページを再描画するため、
 * 入力中に何度も走る判定には重い」と考えていた。**測ったところ誤りだった。**
 * 検証用のページで描画回数を数えたところ、`revalidatePath` などを呼ばない限り
 * 応答は戻り値だけで、ページの再描画は起きなかった（#52）。
 *
 * web に2つ目の HTTP 面を増やさずに済み、Origin 検査による CSRF 対策も
 * 他の画面と同じ形で効くため、Server Action に揃えた。
 */
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { checkProsody, createPost, type CheckResult, type Visibility } from "./api/post";
import { toFormError, type FormError } from "./api/errors";

/** 入力中の判定の結果。 */
export type CheckState =
  | { status: "ok"; result: CheckResult }
  /** 判定そのものができなかった（prosody 障害・通信断）。破調とは異なる。 */
  | { status: "unavailable"; message: string };

/**
 * 入力中の本文を判定する。何も保存しない。
 *
 * **破調でも `ok` を返す。** 判定を求めて判定が返っているためエラーではない
 * （基本設計 05）。投稿できるかは `verdict` で決まる。
 */
export async function checkBody(body: string): Promise<CheckState> {
  const result = await checkProsody(body);
  if (result.ok) {
    return { status: "ok", result: result.data };
  }
  const error = toFormError(result.error);
  return {
    status: "unavailable",
    message: error.message ?? Object.values(error.fields)[0] ?? "判定できませんでした",
  };
}

/** 投稿フォームの状態。`useActionState` に渡す。 */
export type ComposeState = {
  error?: FormError;
  /**
   * 直前に入力していた本文。
   *
   * **失敗しても入力を消さない。** JavaScript が無い環境ではページが再描画されるため、
   * 状態に持たせないと詠んだ句が消える。
   */
  body?: string;
  visibility?: Visibility;
};

/**
 * 投稿する。
 *
 * **判定結果を送らない。** api は必ず再判定する（基本設計 01 §4 / FR-02-05）。
 * 画面が投稿ボタンを無効にするのは体験のためであり、防御ではない。
 */
export async function submitPost(
  _prev: ComposeState,
  formData: FormData,
): Promise<ComposeState> {
  const body = String(formData.get("body") ?? "");
  const visibility: Visibility = formData.get("visibility") === "followers" ? "followers" : "public";

  const result = await createPost(body, visibility);
  if (!result.ok) {
    return { error: toFormError(result.error), body, visibility };
  }

  // **全体タイムラインへ送る。** 基本設計 04 §2 の遷移図は S-02（フォロー中）を
  // 指しているが、そこに自分の投稿は出ない（follows を起点にしているため）。
  // 詠んだ直後に何も無い画面へ送ることになるため、見える方へ送る。
  revalidatePath("/");
  redirect("/");
}
