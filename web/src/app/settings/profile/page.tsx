/**
 * S-10 プロフィール編集（基本設計 04 §1）。
 */
import Link from "next/link";
import { redirect } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { ProfileForm } from "@/components/ProfileForm";
import { currentUser } from "@/lib/api/session";

export const metadata = { title: "プロフィール編集 | 575" };

export const dynamic = "force-dynamic";

export default async function ProfileSettingsPage() {
  const user = await currentUser();
  if (!user) redirect("/login");

  return (
    <AppShell current="/settings/profile" title="プロフィール編集" user={user}>
      <ProfileForm bio={user.bio ?? ""} displayName={user.display_name} handle={user.handle} />
      <p>
        <Link href="/settings/blocks">ブロック中の一覧を見る</Link>
      </p>
    </AppShell>
  );
}
