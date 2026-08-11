/**
 * api を呼ぶ土台。
 *
 * **サーバー側でだけ使う。** ブラウザは web としか通信せず、web が api へ中継する
 * （基本設計 01 §6「web → api: 利用者の Cookie を転送」）。
 *
 * ブラウザから api を直接呼ぶと、開発環境（localhost:3000 → localhost:8080）が
 * 別オリジンになり CORS の設定が要る。本番は単一オリジンの構成であり、
 * 開発のためだけに api へ CORS を足すと、本番では不要な設定が残る。
 */
import "server-only";

import { cookies } from "next/headers";

import type { components } from "./schema";

/** セッション Cookie の名前。api 側（handler.SessionCookieName）と揃える。 */
export const SESSION_COOKIE = "session";

/**
 * api の場所。
 *
 * クラスタ内の名前で解決する。ブラウザからは到達できない場所でよい。
 */
const API_URL = process.env.WEB_API_INTERNAL_URL ?? "http://api:8080";

/**
 * 1リクエストの上限（基本設計 01 §6 の「web → api: 5秒」）。
 */
const TIMEOUT_MS = 5_000;

/** api のエラー本文。生成した型をそのまま使う。 */
export type ApiError = components["schemas"]["Error"]["error"];

/**
 * api の呼び出し結果。
 *
 * **例外を投げない。** 業務上のエラー（識別名が使用済み、破調など）は
 * 例外ではなく応答であり、呼び出し側が分岐して画面に反映する必要がある。
 * 例外にすると、try/catch の中で分岐を書くことになり見通しが悪くなる。
 */
export type ApiResult<T> =
  | { ok: true; status: number; data: T; setCookie: string[] }
  | { ok: false; status: number; error: ApiError };

type RequestOptions = {
  method?: string;
  body?: unknown;
  /** 利用者の Cookie を転送するか。未ログインでも使える経路では false にできる。 */
  forwardSession?: boolean;
};

/**
 * api を呼ぶ。
 *
 * 到達できない場合も ApiResult を返す。画面は「api に到達できない」ことを
 * 利用者に伝える必要があり、落ちてはならない。
 */
export async function callApi<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiResult<T>> {
  const { method = "GET", body, forwardSession = true } = options;

  const headers: Record<string, string> = {};
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (forwardSession) {
    const session = (await cookies()).get(SESSION_COOKIE);
    if (session) {
      headers["Cookie"] = `${SESSION_COOKIE}=${session.value}`;
    }
  }

  let response: Response;
  try {
    response = await fetch(`${API_URL}/api/v1${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      // 認証状態で内容が変わるため、キャッシュしない。
      cache: "no-store",
      signal: AbortSignal.timeout(TIMEOUT_MS),
    });
  } catch {
    return {
      ok: false,
      status: 0,
      error: { code: "API_UNREACHABLE", message: "サーバーに接続できませんでした" },
    };
  }

  if (response.status === 204) {
    return { ok: true, status: 204, data: undefined as T, setCookie: readSetCookie(response) };
  }

  const text = await response.text();
  let payload: unknown = undefined;
  if (text !== "") {
    try {
      payload = JSON.parse(text);
    } catch {
      return {
        ok: false,
        status: response.status,
        error: { code: "INVALID_RESPONSE", message: "サーバーの応答を解釈できませんでした" },
      };
    }
  }

  if (!response.ok) {
    const error = (payload as components["schemas"]["Error"] | undefined)?.error;
    return {
      ok: false,
      status: response.status,
      // 形式が想定と違う応答（プロキシが返す HTML など）でも落ちないようにする。
      error: error ?? { code: "UNKNOWN", message: "エラーが発生しました" },
    };
  }

  return {
    ok: true,
    status: response.status,
    data: payload as T,
    setCookie: readSetCookie(response),
  };
}

/**
 * api が返した Set-Cookie を取り出す。
 *
 * ログイン・登録・ログアウトではこれをブラウザへ引き渡す必要がある。
 */
function readSetCookie(response: Response): string[] {
  // getSetCookie は複数の Set-Cookie を分けて返す。
  // headers.get("set-cookie") はカンマで連結され、Expires の中のカンマと
  // 区別がつかなくなる。
  return response.headers.getSetCookie();
}
