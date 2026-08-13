/**
 * 詠む導線（S-07 / S-01）。
 *
 * 表題のケース ID は詳細設計 04 §11 の表に対応する（同 §9 の管理方針）。
 */
import { expect, test } from "@playwright/test";

import { cardOf, compose, signUp } from "./helpers";

/**
 * 575 の中核。**これが通らなければ SNS として成り立たない**
 * （要件定義 5.1 が MVP の最小の価値としているもの）。
 */
test("TC_E2E_01 登録して詠むと、句ごとに改行されて全体タイムラインに出る", async ({ page }) => {
  const handle = await signUp(page, "poet");

  await compose(page, "柿くへば鐘が鳴るなり法隆寺");

  const card = cardOf(page, handle);
  await expect(card).toHaveCount(1);
  // 句ごとに <p> で並ぶ（FR-03-06）。改行を1文字ずつではなく、句の単位で確かめる。
  for (const segment of ["柿くへば", "鐘が鳴るなり", "法隆寺"]) {
    await expect(card.getByText(segment, { exact: true })).toBeVisible();
  }
  await expect(card.getByText("定型", { exact: true })).toBeVisible();
});

/**
 * 判定エンジンが画面まで効いていること。
 *
 * 破調は投稿できない（FR-02-04）。ボタンを無効にするのは体験のためで、
 * 防御は api にある。ここで見るのは**判定が画面に届いているか**である。
 */
test("TC_E2E_02 破調は詠めず、あと何音必要かが分かる", async ({ page }) => {
  await signUp(page, "hacho");

  await page.getByRole("link", { name: "詠む", exact: true }).click();
  await page.getByLabel("本文").fill("今日はとても良い天気ですね");

  await expect(page.getByText("⚠️ 破調")).toBeVisible();
  // **「五七五になっていません」で終わらせない**（基本設計 04 §3）。
  await expect(page.getByText(/あと\d+音必要です/)).toBeVisible();
  await expect(page.getByRole("button", { name: "詠む", exact: true })).toBeDisabled();
});

/** 認証の境目。未ログインでは詠めない。 */
test("TC_E2E_03 未ログインでは詠む導線が出ず、投稿の画面はログインへ送られる", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("link", { name: "詠む", exact: true })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "ログイン" })).toBeVisible();

  await page.goto("/compose");
  await expect(page).toHaveURL("/login");
});
