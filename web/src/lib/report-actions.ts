"use server";

/**
 * 通報の Server Action（S-12）。
 */
import { reportPost } from "./api/user";
import type { ReportReason } from "./api/user";

/** 画面に出す理由の一覧。**api の enum と対応させる。** */
const REASONS: ReportReason[] = ["spam", "harassment", "inappropriate", "other"];

export type ReportState = {
  /** 受け付けたか。**送れたこと自体を伝える必要がある。** */
  accepted?: boolean;
  error?: string;
};

/**
 * 断られ方で文言を変える。
 *
 * **同じ文言にしない。** どうすればよいかが変わる。
 * ただし 404 は理由を区別しない（BR-10）。
 */
const MESSAGES: Record<string, string> = {
  ALREADY_REPORTED: "この句はすでに通報済みです。運営に届いています。",
  CANNOT_REPORT_SELF: "自分の句は通報できません。消したい場合は削除してください。",
  NOT_FOUND: "この句は見つかりませんでした。",
  UNAUTHENTICATED: "通報するにはログインが必要です。",
};

export async function submitReport(
  _prev: ReportState,
  formData: FormData,
): Promise<ReportState> {
  const postId = String(formData.get("id") ?? "");
  const reason = String(formData.get("reason") ?? "");
  const comment = String(formData.get("comment") ?? "").trim();

  // **`other` を既定にしない。** 選ばれていなければ送らない。
  if (!REASONS.includes(reason as ReportReason)) {
    return { error: "理由を選んでください。" };
  }

  const result = await reportPost(postId, reason as ReportReason, comment);
  if (!result.ok) {
    return { error: MESSAGES[result.error.code] ?? "通報できませんでした。時間をおいてお試しください。" };
  }
  return { accepted: true };
}
