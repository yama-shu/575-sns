/**
 * 投稿の規則のうち、画面が知る必要のあるもの。
 *
 * **`server-only` を付けない。** 入力欄の文字数表示や投稿ボタンの活性は
 * ブラウザ側で決めるため、クライアントからも読む。
 *
 * ここにあるのは api の規則の**写し**である。判断の正はすべて api が持つ
 * （基本設計 01 §4）。画面はそれを先に伝えるためだけに使う。
 */
import type { components } from "./api/schema";

type Verdict = components["schemas"]["Verdict"];

/** 本文の上限。api の `domain.BodyMaxLength`（= DB の VARCHAR(100)）と揃える。 */
export const BODY_MAX_LENGTH = 100;

/** 規定のモーラ数の合計（5 + 7 + 5）。 */
export const TOTAL_MORA = 17;

/** 投稿できる判定（FR-02-04）。 */
const POSTABLE: readonly Verdict[] = ["teikei", "kyoyo"];

/**
 * この判定で投稿できるか。
 *
 * **防御ではない。** api は投稿時に必ず再判定する（FR-02-05）。
 * ここで false を返すのは、押しても弾かれるボタンを押させないためである。
 */
export function isPostable(verdict: Verdict): boolean {
  return POSTABLE.includes(verdict);
}

/**
 * 規定との差を日本語にする。
 *
 * 「五七五になっていません」だけでは直しようがない（基本設計 04 §3）。
 */
export function shortfall(total: number): string {
  if (total < TOTAL_MORA) return `あと${TOTAL_MORA - total}音必要です`;
  if (total > TOTAL_MORA) return `${total - TOTAL_MORA}音多いようです`;
  // 17音でも五七五に区切れないことがある（NO_VALID_SPLIT）。
  return "区切る位置が見つかりませんでした";
}
