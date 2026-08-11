"use server";

/**
 * プロフィール編集の Server Action（S-10）。
 */
import { redirect } from "next/navigation";

import { updateProfile } from "./api/user";
import { toFormError, type FormError } from "./api/errors";

/**
 * 編集フォームの状態。
 *
 * **失敗しても入力を消さない。** JavaScript が無い環境ではページが
 * 再描画されるため、状態に持たせないと書いた内容が消える（#52 と同じ）。
 */
export type ProfileFormState = {
  error?: FormError;
  displayName?: string;
  bio?: string;
};

export async function saveProfile(
  _prev: ProfileFormState,
  formData: FormData,
): Promise<ProfileFormState> {
  const displayName = String(formData.get("display_name") ?? "");
  const bio = String(formData.get("bio") ?? "");

  const result = await updateProfile({ display_name: displayName, bio });
  if (!result.ok) {
    return { error: toFormError(result.error), displayName, bio };
  }
  // 反映を確かめられる場所へ送る。
  redirect(`/@${result.data.handle}`);
}
