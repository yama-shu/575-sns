/**
 * テストから使う共通の操作。
 *
 * **画面の役割（role）と名前で要素を選ぶ。** CSS クラスで選ぶと、
 * 見た目を直しただけでテストが落ちる。
 */
import { expect, type Browser, type Page } from "@playwright/test";

/** 登録に使うパスワード。api の下限（8文字）を満たす。 */
export const PASSWORD = "e2e-passw0rd";

/**
 * テストごとに一意な識別名を作る。
 *
 * **固定の利用者を共有しない。** 並列で走らせたときに互いの状態を壊し、
 * 「単体では通るのに揃えると落ちる」形の不安定さになる。
 * 575 は登録に招待もメール確認も要らないため、これができる。
 *
 * 識別名は英数字と `_` のみ、20文字以内（DB の CHECK 制約）。
 */
export function uniqueHandle(prefix: string): string {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`.slice(0, 20);
}

/** 新しい利用者を登録する。登録するとログイン済みで /home に着く。 */
export async function signUp(page: Page, prefix: string): Promise<string> {
  const handle = uniqueHandle(prefix);

  await page.goto("/signup");
  await page.getByLabel("識別名").fill(handle);
  await page.getByLabel("表示名").fill(handle);
  await page.getByLabel("メールアドレス").fill(`${handle}@example.test`);
  await page.getByLabel("パスワード").fill(PASSWORD);
  await page.getByRole("button", { name: "登録する" }).click();

  await expect(page).toHaveURL("/home");
  await expect(page.getByText(`@${handle}`)).toBeVisible();
  return handle;
}

/** 新しい利用者を、別のブラウザ文脈（別セッション）で登録する。 */
export async function signUpIn(
  browser: Browser,
  prefix: string,
): Promise<{ page: Page; handle: string }> {
  const context = await browser.newContext();
  const page = await context.newPage();
  return { page, handle: await signUp(page, prefix) };
}

/**
 * 句を詠む。投稿すると全体タイムライン（S-01）へ移る。
 *
 * **判定が出るまで待つ。** 入力が止まってから 300ms 後に判定を呼ぶ
 * （基本設計 04 §3）。判定前に押すと、画面が確かめたい状態を通らない。
 */
export async function compose(page: Page, body: string): Promise<void> {
  await page.getByRole("link", { name: "詠む", exact: true }).click();
  await page.getByLabel("本文").fill(body);

  await expect(page.getByText("✅ 定型")).toBeVisible();
  await page.getByRole("button", { name: "詠む", exact: true }).click();
  await expect(page).toHaveURL("/");
}

/** その利用者の句のカード。誰の句かで絞る（本文は他のテストと重なりうる）。 */
export function cardOf(page: Page, handle: string) {
  return page.locator("article").filter({ hasText: `@${handle}` });
}
