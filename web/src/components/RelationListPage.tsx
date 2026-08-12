/**
 * フォロー中・フォロワー一覧の中身（S-05 / S-06）。
 *
 * 2つの画面で違うのは種類と文言だけなので、1つにまとめる。
 */
import Link from "next/link";
import { notFound } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { UserList } from "@/components/UserList";
import { currentUser } from "@/lib/api/session";
import { fetchProfile, fetchRelationList } from "@/lib/api/user";

type Props = {
  handle: string;
  kind: "following" | "followers";
  cursor?: string;
};

export async function RelationListPage({ handle, kind, cursor }: Props) {
  const user = await currentUser();
  const profile = await fetchProfile(handle);

  // 見えない相手は 404。理由を出し分けない（BR-10）。
  if (!profile.ok) {
    if (profile.status === 404) notFound();
    return (
      <AppShell current="/" title="一覧" user={user}>
        <p role="alert">読み込めませんでした。時間をおいてお試しください。</p>
      </AppShell>
    );
  }

  const list = await fetchRelationList(kind, handle, cursor);
  const isMine = user !== null && user.handle === profile.data.handle;
  const path = `/@${handle}/${kind}`;
  const label = kind === "following" ? "フォロー中" : "フォロワー";

  return (
    <AppShell current="/" title={`${profile.data.display_name} の${label}`} user={user}>
      <p>
        <Link href={`/@${handle}`}>← @{handle} の句へ</Link>
      </p>

      {list.ok ? (
        <UserList
          empty={{
            title: kind === "following" ? "まだ誰もフォローしていません" : "まだフォロワーがいません",
            hint:
              kind === "following" ? (
                <>
                  <Link href="/">全体タイムライン</Link>で気になる人を見つけてみてください。
                </>
              ) : isMine ? (
                <>句を詠むと、読んだ人が見つけてくれます。</>
              ) : (
                <>この人をフォローする最初の一人になれます。</>
              ),
          }}
          handle={handle}
          initialCursor={list.data.next_cursor}
          initialUsers={list.data.items}
          kind={kind}
          moreHref={path}
        />
      ) : (
        <p role="alert">一覧を読み込めませんでした。時間をおいてお試しください。</p>
      )}
    </AppShell>
  );
}
