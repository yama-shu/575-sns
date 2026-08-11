"use server";

/**
 * いいね・フォロー・ブロック・削除の Server Action。
 *
 * # なぜフォームなのか
 *
 * どれも `<form action={...}>` から呼ぶ前提で作っている。`onClick` で
 * 呼ぶ形にすると **JavaScript が無効な環境で操作できなくなる**。
 * いいねもフォローも要件では Must であり（FR-04-01 / FR-04-02）、
 * 主要な操作はキーボードだけで完結する必要がある（NFR-06-03）。
 *
 * # 画面をどう新しくするか
 *
 * **返ってきた数を画面が自分で足し引きしない。** 連打や通信の失敗でずれる。
 * api が返した値、または描画し直した結果だけを出す。
 *
 * 影響する範囲で手段を分けている。
 *
 *	いいね         : api が返す `liked` / `like_count` をそのまま描く
 *	フォロー・ブロック: 同じ経路へ `redirect` してページを描き直す
 *
 * フォローとブロックは、そのボタンだけでなく**フォロワー数や句の一覧まで変える**
 * （ブロックすると句が0件になる）。ボタンの周りだけを差し替えても画面が食い違う。
 *
 * # revalidatePath と router.refresh() は使わない
 *
 * 最初は `revalidatePath` で済ませようとしたが、**効かなかった**。
 * 画面はどれも `dynamic = "force-dynamic"` で取得も `cache: "no-store"` であり、
 * 捨てるキャッシュが無い。いいねは記録されるのに数が `♡ 0` のまま、という形で表に出た。
 * 加えてユーザーページは実体が `/users/[handle]` にあり、
 * `revalidatePath("/@handle")` は**どのルートにも当たらず黙って何もしない**。
 *
 * 次に `router.refresh()` を副作用から呼ぶ形にしたが、**結果が安定しなかった**。
 * 押した直後に反映されることもあれば、まったく変わらないこともある。
 * `redirect` は経路が決まれば挙動も決まるため、そちらに寄せた。
 */
import { redirect } from "next/navigation";

import { deletePost, setBlock, setFollow, setLike } from "./api/user";
import { toFormError } from "./api/errors";
import type { ApiResult } from "./api/client";

/**
 * 操作の結果。
 *
 * **初期値の定数をここに置かない。** `"use server"` のファイルは非同期関数しか
 * 輸出できず、オブジェクトを輸出すると実行時に落ちる（#48 でも同じ誤りをした）。
 * 呼び出し側で `{}` を渡す。
 */
export type ActionState = { error?: string };

/** いいねの結果。api が返した値をそのまま持つ。 */
export type LikeActionState = ActionState & { liked?: boolean; likeCount?: number };

/**
 * いいねを切り替える。
 *
 * `liked` は**現在の状態**であり、その逆にする。
 * PUT / DELETE はどちらも冪等なので、二重に送られても結果は変わらない。
 */
export async function toggleLike(
  _prev: LikeActionState,
  formData: FormData,
): Promise<LikeActionState> {
  const id = String(formData.get("id") ?? "");
  const liked = formData.get("liked") === "true";

  const result = await setLike(id, !liked);
  if (!result.ok) {
    return { error: messageOf(result) };
  }
  // **足し引きしない。** api が数えた結果をそのまま返す。
  return { liked: result.data.liked, likeCount: result.data.like_count };
}

/** フォローを切り替える。 */
export async function toggleFollow(
  _prev: ActionState,
  formData: FormData,
): Promise<ActionState> {
  const handle = String(formData.get("handle") ?? "");
  const following = formData.get("following") === "true";

  const result = await setFollow(handle, !following);
  if (!result.ok) {
    return { error: messageOf(result) };
  }
  redirect(pathOf(formData));
}

/**
 * ブロックを切り替える。
 *
 * ブロックするとフォロー関係が双方向に外れる（BR-08）。
 * その処理は api が1トランザクションで行う。
 */
export async function toggleBlock(_prev: ActionState, formData: FormData): Promise<ActionState> {
  const handle = String(formData.get("handle") ?? "");
  const blocking = formData.get("blocking") === "true";

  const result = await setBlock(handle, !blocking);
  if (!result.ok) {
    return { error: messageOf(result) };
  }
  redirect(pathOf(formData));
}

/**
 * 投稿を削除する。
 *
 * **取り返しがつかない。** 投稿は編集できない（FR-02-07）ため、
 * 画面側で二段階にしている（DeletePostButton の `<details>`）。
 */
export async function removePost(_prev: ActionState, formData: FormData): Promise<ActionState> {
  const id = String(formData.get("id") ?? "");
  const result = await deletePost(id);
  if (!result.ok) {
    return { error: messageOf(result) };
  }
  // 消した投稿のページには戻れない。全体タイムラインへ送る。
  redirect("/");
}

/**
 * 戻り先の経路。
 *
 * **フォームが持つ値をそのまま信じない。** 外部の URL を入れられると
 * 別の場所へ飛ばせてしまう。自分のサイトの中だけを許す。
 */
function pathOf(formData: FormData): string {
  const path = String(formData.get("path") ?? "/");
  return path.startsWith("/") && !path.startsWith("//") ? path : "/";
}

function messageOf(result: ApiResult<unknown>): string {
  if (result.ok) return "";
  const error = toFormError(result.error);
  return error.message ?? Object.values(error.fields)[0] ?? "操作できませんでした";
}
