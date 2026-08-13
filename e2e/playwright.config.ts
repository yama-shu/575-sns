import { defineConfig, devices } from "@playwright/test";

/**
 * E2E の設定（詳細設計 04 のテストの配分の頂点）。
 *
 * **対象は動いているスタック全体である。** db・prosody・api・web を
 * `docker compose up` で立ててから実行する。ここではサーバを起動しない。
 * 起動まで面倒を見ると、CI と手元で別の起動経路を持つことになる。
 */
export default defineConfig({
  testDir: "./tests",
  // 利用者をテストごとに作るため、並列で走らせても互いに影響しない。
  fullyParallel: true,
  /**
   * 並列数を絞る。
   *
   * **判定エンジンが詰まる。** prosody の既定のワーカー数は 1 で（compose.yaml）、
   * api は 1 秒で見切りをつける（`API_PROSODY_TIMEOUT`）。既定の並列数
   * （CPU の半分）で走らせたところ、登録の bcrypt と判定が CPU を奪い合い、
   * 「⚠️ 判定できません」（504）が出た。**画面の不具合ではない。**
   */
  workers: 2,
  // **CI では .only を失敗させる。** 絞り込んだまま入れると、他が動かなくなる。
  forbidOnly: !!process.env.CI,
  // 落ちたら1度だけやり直す。**手元では再試行しない**（不安定さを隠さないため）。
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : [["list"]],

  /**
   * 既定の 5 秒から延ばす。
   *
   * 判定は入力が止まってから 300ms 後に prosody まで往復し、Server Action は
   * 応答を待ってから遷移する。**開発モードでは初回のコンパイルがこれに乗る**
   * （[#71](https://github.com/yama-shu/575-sns/issues/71) で実装を疑った）。
   */
  expect: { timeout: 10_000 },

  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    // 失敗したときに何が起きたか分かるようにする。
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    // 開発モードの初回コンパイルは数秒かかる（#71 で踏んだ）。
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
  },

  // 初版は Chromium だけにする。複数ブラウザは費用に見合う理由ができてから。
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
