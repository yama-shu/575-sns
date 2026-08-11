/**
 * ログイン・登録・ログアウトの Server Actions。
 *
 * ブラウザはこれらを呼び、web が api へ中継する（基本設計 01 §6）。
 */
"use server";

import { redirect } from "next/navigation";

import { callApi } from "./api/client";
import { toFormError, type FormError } from "./api/errors";
import { relaySetCookie, type User } from "./api/session";

/** フォームの状態。エラーが無ければ NO_ERROR。 */
export type AuthState = FormError;

/**
 * ログインする。
 *
 * **識別名の存在有無を漏らさない。** api が「識別名またはパスワードが違います」を
 * 返すため、web はそれをそのまま出す（ADR-0006）。
 */
export async function logIn(_prev: AuthState, form: FormData): Promise<AuthState> {
  const result = await callApi<User>("/auth/login", {
    method: "POST",
    forwardSession: false,
    body: {
      handle: String(form.get("handle") ?? ""),
      password: String(form.get("password") ?? ""),
    },
  });

  if (!result.ok) return toFormError(result.error);

  await relaySetCookie(result.setCookie);
  redirect("/home");
}

/** アカウントを登録する。成功するとログイン済みになる。 */
export async function signUp(_prev: AuthState, form: FormData): Promise<AuthState> {
  const result = await callApi<User>("/auth/signup", {
    method: "POST",
    forwardSession: false,
    body: {
      handle: String(form.get("handle") ?? ""),
      email: String(form.get("email") ?? ""),
      password: String(form.get("password") ?? ""),
      display_name: String(form.get("display_name") ?? ""),
    },
  });

  if (!result.ok) return toFormError(result.error);

  await relaySetCookie(result.setCookie);
  redirect("/home");
}

/**
 * ログアウトする。
 *
 * api がセッション行を消し、空の Cookie を返す。**次のリクエストから即座に効く。**
 */
export async function logOut(): Promise<void> {
  const result = await callApi<void>("/auth/logout", { method: "POST" });
  if (result.ok) {
    await relaySetCookie(result.setCookie);
  }
  redirect("/login");
}
