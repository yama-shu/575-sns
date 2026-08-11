/**
 * api のエラーを画面の表示に対応付ける。
 *
 * **`code` で分岐する**（基本設計 05 §1）。`message` は文言が変わりうるため、
 * これで分岐するとクライアントが壊れる。
 *
 * NFR-06-02 が「単に『エラー』とだけ表示しない」ことを求めている。
 * 対応表をここ1箇所に置き、知らない `code` の既定も決める。
 */
import type { ApiError } from "./client";

/** 画面に出すエラー。項目に紐づくものと、フォーム全体に出すものを分ける。 */
export type FormError = {
  /** フォーム全体に出す文言。項目に紐づくものだけのときは undefined。 */
  message?: string;
  /** 入力項目ごとの文言。キーは api の `details.field`。 */
  fields: Record<string, string>;
};

/** `code` ごとの文言。api の message をそのまま出さず、画面の文脈に合わせる。 */
const MESSAGES: Record<string, string> = {
  INVALID_CREDENTIALS: "識別名またはパスワードが違います",
  ACCOUNT_SUSPENDED: "このアカウントは利用を停止されています",
  HANDLE_TAKEN: "この識別名はすでに使われています",
  EMAIL_TAKEN: "このメールアドレスはすでに登録されています",
  UNAUTHENTICATED: "ログインが必要です",
  FORBIDDEN: "この操作は行えません",
  NOT_FOUND: "見つかりませんでした",
  PROSODY_UNAVAILABLE: "いま詠めません。しばらく経ってからお試しください",
  UPSTREAM_TIMEOUT: "判定に時間がかかっています。しばらく経ってからお試しください",
  API_UNREACHABLE: "サーバーに接続できませんでした。時間をおいてお試しください",
  INVALID_RESPONSE: "サーバーの応答を解釈できませんでした",
};

/** `code` を、その項目に紐づけたい場合の対応。 */
const FIELD_OF: Record<string, string> = {
  HANDLE_TAKEN: "handle",
  EMAIL_TAKEN: "email",
};

/**
 * api のエラーを画面用に変換する。
 *
 * `VALIDATION_FAILED` は `details.field` を見て項目に紐づける。
 * 紐づけ先が分からないものはフォーム全体のエラーにする。
 */
export function toFormError(error: ApiError): FormError {
  const field = FIELD_OF[error.code] ?? fieldOfValidation(error);
  const message = MESSAGES[error.code] ?? error.message;

  if (field) {
    return { fields: { [field]: message } };
  }
  return { message: withRequestId(message, error), fields: {} };
}

function fieldOfValidation(error: ApiError): string | undefined {
  if (error.code !== "VALIDATION_FAILED") return undefined;
  const field = error.details?.["field"];
  return typeof field === "string" ? field : undefined;
}

/**
 * 想定外のエラーには `request_id` を添える。
 *
 * 利用者が問い合わせたときに、ログを引く手がかりになる（詳細設計 03）。
 */
function withRequestId(message: string, error: ApiError): string {
  const requestId = error.details?.["request_id"];
  if (typeof requestId === "string" && requestId !== "") {
    return `${message}（問い合わせ番号: ${requestId}）`;
  }
  return message;
}

/** 空のエラー。フォームの初期状態に使う。 */
export const NO_ERROR: FormError = { fields: {} };
