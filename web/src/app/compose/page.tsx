/**
 * S-07 投稿作成（基本設計 04 §3）。
 *
 * **本来はモーダルである。** このページは JavaScript が無い環境のための経路で、
 * 「詠む」ボタンのリンク先にあたる（NFR-06-03）。
 *
 * モーダルと同じ `ComposeForm` を置く。入力できる内容が環境で変わらないようにする。
 */
import { redirect } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { ComposeForm } from "@/components/ComposeForm";
import { currentUser } from "@/lib/api/session";

export const metadata = { title: "詠む | 575" };

export const dynamic = "force-dynamic";

export default async function ComposePage() {
  const user = await currentUser();
  if (!user) redirect("/login");

  return (
    <AppShell title="詠む" user={user} current="/compose">
      <ComposeForm />
    </AppShell>
  );
}
