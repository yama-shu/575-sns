import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // 本番イメージを最小にする。依存を含めた実行可能な形が .next/standalone に出力され、
  // node_modules を丸ごとイメージへ入れずに済む。
  output: "standalone",
  /*
   * ユーザーページは基本設計 04 §1 で `/@:handle` と定めている。
   *
   * **App Router で `@` 始まりのディレクトリは使えない。** `app/@[handle]/` は
   * 並列ルートのスロットとして解釈される。実体を `app/users/[handle]/` に置き、
   * 外向きの経路をここで割り当てる。
   *
   * `app/[handle]/` を根に置く案は採らない。あらゆる未知のパスが
   * ユーザーページに吸われる形になり、画面を足すたびに衝突を気にすることになる。
   */
  async rewrites() {
    return [{ source: "/@:handle", destination: "/users/:handle" }];
  },
};

export default nextConfig;
