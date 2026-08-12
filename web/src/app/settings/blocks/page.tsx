/**
 * S-11 ブロック中一覧（基本設計 04 §1）。
 *
 * **本人の一覧しか無い。** 誰をブロックしたかは他人に見せない（api も 401 で断る）。
 */
import Link from "next/link";
import { redirect } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { UserList } from "@/components/UserList";
import { currentUser } from "@/lib/api/session";
import { fetchRelationList } from "@/lib/api/user";

export const metadata = { title: "ブロック中 | 575" };

export const dynamic = "force-dynamic";

export default async function BlocksPage({ searchParams }: PageProps<"/settings/blocks">) {
  const user = await currentUser();
  if (!user) redirect("/login");

  const query = await searchParams;
  const cursor = typeof query.cursor === "string" ? query.cursor : undefined;
  const list = await fetchRelationList("blocks", user.handle, cursor);

  return (
    <AppShell current="/settings/blocks" title="ブロック中" user={user}>
      <p>
        <Link href="/settings/profile">← プロフィール編集へ</Link>
      </p>

      {list.ok ? (
        <UserList
          empty={{
            title: "ブロックしている人はいません",
            hint: <>ブロックは相手のページから行えます。相手には知らされません。</>,
          }}
          handle={user.handle}
          initialCursor={list.data.next_cursor}
          initialUsers={list.data.items}
          kind="blocks"
          moreHref="/settings/blocks"
        />
      ) : (
        <p role="alert">一覧を読み込めませんでした。時間をおいてお試しください。</p>
      )}
    </AppShell>
  );
}
