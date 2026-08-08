import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // 本番イメージを最小にする。依存を含めた実行可能な形が .next/standalone に出力され、
  // node_modules を丸ごとイメージへ入れずに済む。
  output: "standalone",
};

export default nextConfig;
