/**
 * 関係の操作（S-04 / S-02 / S-05）。
 *
 * **2人が要る。** 別々のブラウザ文脈を使い、セッションを混ぜない。
 *
 * 表題のケース ID は詳細設計 04 §11 の表に対応する（同 §9 の管理方針）。
 */
import { expect, test } from "@playwright/test";

import { cardOf, compose, signUpIn } from "./helpers";

test("TC_E2E_04 フォローすると、フォロー中タイムラインと一覧に出る", async ({ browser }) => {
  const author = await signUpIn(browser, "wf");
  await compose(author.page, "柿くへば鐘が鳴るなり法隆寺");

  const reader = await signUpIn(browser, "rf");
  await reader.page.goto(`/@${author.handle}`);
  await reader.page.getByRole("button", { name: "フォローする" }).click();
  await expect(reader.page.getByRole("button", { name: "フォロー中" })).toBeVisible();

  // フォロー中タイムライン（S-02）に相手の句が出る。
  await reader.page.goto("/home");
  await expect(cardOf(reader.page, author.handle)).toHaveCount(1);

  // フォロー中一覧（S-05）に相手が出る。
  await reader.page.goto(`/@${reader.handle}/following`);
  await expect(reader.page.getByText(`@${author.handle}`, { exact: true })).toBeVisible();

  await author.page.context().close();
  await reader.page.context().close();
});

/** BR-09（ブロックした相手の句は見えない）が画面まで効くこと。 */
test("TC_E2E_05 ブロックすると、その人の句が全体タイムラインから消える", async ({ browser }) => {
  const author = await signUpIn(browser, "wb");
  await compose(author.page, "柿くへば鐘が鳴るなり法隆寺");

  const reader = await signUpIn(browser, "rb");
  // **先に見えることを確かめる。** これが無いと、消えたのか元から無かったのかを
  // 区別できず、ブロックが効いた証拠にならない。
  await reader.page.goto("/");
  await expect(cardOf(reader.page, author.handle)).toHaveCount(1);

  await reader.page.goto(`/@${author.handle}`);
  await reader.page.getByRole("button", { name: "ブロックする" }).click();
  await expect(reader.page.getByText("この人をブロックしています。句は表示されません。")).toBeVisible();

  await reader.page.goto("/");
  await expect(cardOf(reader.page, author.handle)).toHaveCount(0);

  await author.page.context().close();
  await reader.page.context().close();
});
