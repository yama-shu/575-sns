"use server";

/**
 * 関係の一覧の続きを取る Server Action。
 *
 * ブラウザは web としか通信しない（基本設計 01 §6）。
 */
import { fetchRelationList, type RelationKind, type RelationUser } from "./api/user";

export type MoreUsersResult = {
  users: RelationUser[];
  nextCursor: string | null;
  /** 取得に失敗したときの文言。成功時は undefined。 */
  error?: string;
};

export async function loadMoreUsers(
  kind: RelationKind,
  handle: string,
  cursor: string,
): Promise<MoreUsersResult> {
  const result = await fetchRelationList(kind, handle, cursor);
  if (!result.ok) {
    return { users: [], nextCursor: cursor, error: "続きを読み込めませんでした" };
  }
  return { users: result.data.items, nextCursor: result.data.next_cursor };
}
