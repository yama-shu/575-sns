import type { Metadata } from "next";

import "./globals.css";

export const metadata: Metadata = {
  title: "575",
  description: "言いたいことは、五七五で。投稿本文が五七五に収まっていないと投稿できない SNS",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ja">
      <body>{children}</body>
    </html>
  );
}
