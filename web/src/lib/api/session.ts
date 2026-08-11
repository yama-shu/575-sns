/**
 * ログイン状態の取得と、セッション Cookie の中継。
 */
import "server-only";

import { cookies } from "next/headers";

import { callApi, SESSION_COOKIE } from "./client";
import type { components } from "./schema";

export type User = components["schemas"]["User"];

/**
 * ログイン中の利用者を返す。未ログインなら null。
 *
 * **毎回 api に問い合わせる。** Cookie の有無だけで判断すると、
 * ログアウト済み・利用停止・期限切れのセッションを「ログイン中」と誤認する。
 * サーバー側セッションを選んだ理由（ADR-0006）がここで効く。
 */
export async function currentUser(): Promise<User | null> {
  const session = (await cookies()).get(SESSION_COOKIE);
  if (!session) return null;

  const result = await callApi<User>("/me");
  return result.ok ? result.data : null;
}

/**
 * api が返した Set-Cookie をブラウザへ引き渡す。
 *
 * **HttpOnly / Secure / SameSite を落とさない。** 落とすと JavaScript から
 * セッション ID を読めてしまい、ADR-0006 が Cookie を選んだ理由
 * （XSS でセッションを盗まれない）が消える。
 *
 * api が組み立てた属性をそのまま使い、web 側で作り直さない。
 * 作り直すと、api の設定（Secure の有無など）と食い違う。
 */
export async function relaySetCookie(setCookie: string[]): Promise<void> {
  const store = await cookies();
  for (const raw of setCookie) {
    const parsed = parseSetCookie(raw);
    if (parsed?.name !== SESSION_COOKIE) continue;
    store.set({
      name: parsed.name,
      value: parsed.value,
      httpOnly: parsed.httpOnly,
      secure: parsed.secure,
      sameSite: parsed.sameSite,
      path: parsed.path,
      maxAge: parsed.maxAge,
      expires: parsed.expires,
    });
  }
}

type ParsedCookie = {
  name: string;
  value: string;
  httpOnly: boolean;
  secure: boolean;
  sameSite: "lax" | "strict" | "none" | undefined;
  path: string | undefined;
  maxAge: number | undefined;
  expires: Date | undefined;
};

/** Set-Cookie を属性ごとに分解する。 */
function parseSetCookie(raw: string): ParsedCookie | null {
  const [pair, ...attributes] = raw.split(";");
  const separator = pair.indexOf("=");
  if (separator < 0) return null;

  const parsed: ParsedCookie = {
    name: pair.slice(0, separator).trim(),
    value: pair.slice(separator + 1).trim(),
    httpOnly: false,
    secure: false,
    sameSite: undefined,
    path: undefined,
    maxAge: undefined,
    expires: undefined,
  };

  for (const attribute of attributes) {
    const [rawKey, ...rest] = attribute.split("=");
    const key = rawKey.trim().toLowerCase();
    const value = rest.join("=").trim();

    switch (key) {
      case "httponly":
        parsed.httpOnly = true;
        break;
      case "secure":
        parsed.secure = true;
        break;
      case "samesite":
        parsed.sameSite = value.toLowerCase() as ParsedCookie["sameSite"];
        break;
      case "path":
        parsed.path = value;
        break;
      case "max-age":
        parsed.maxAge = Number(value);
        break;
      case "expires":
        parsed.expires = new Date(value);
        break;
    }
  }
  return parsed;
}
