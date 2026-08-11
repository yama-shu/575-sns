/**
 * S-04 ユーザーページ（基本設計 04 §1）。
 *
 * 外向きの経路は `/@:handle` である。`next.config.ts` の rewrite で
 * ここへ割り当てている。
 */
import Link from "next/link";
import { notFound } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { ProfileHeader } from "@/components/ProfileHeader";
import { Timeline } from "@/components/Timeline";
import { currentUser } from "@/lib/api/session";
import { fetchProfile, fetchUserPosts } from "@/lib/api/user";

export const dynamic = "force-dynamic";

export default async function UserPage({ params, searchParams }: PageProps<"/users/[handle]">) {
  const { handle } = await params;
  const user = await currentUser();
  const profile = await fetchProfile(handle);

  // 見えない相手は 404。理由を出し分けない（BR-10）。
  if (!profile.ok) {
    if (profile.status === 404) notFound();
    return (
      <AppShell current="/" title="ユーザー" user={user}>
        <p role="alert">読み込めませんでした。時間をおいてお試しください。</p>
      </AppShell>
    );
  }

  const query = await searchParams;
  const cursor = typeof query.cursor === "string" ? query.cursor : undefined;
  const posts = await fetchUserPosts(handle, cursor);
  // 外向きの経路。リンク先に使う（基本設計 04 §1）。
  const publicPath = `/@${handle}`;
  const isMine = user !== null && user.handle === profile.data.handle;

  return (
    <AppShell current="/" title={profile.data.display_name} user={user}>
      <ProfileHeader
        isMine={isMine}
        path={publicPath}
        profile={profile.data}
        signedIn={user !== null}
      />

      {posts.ok ? (
        <Timeline
          empty={{
            title: profile.data.blocking
              ? "ブロック中のため表示していません"
              : "まだ一句もありません",
            hint: profile.data.blocking ? (
              <>ブロックを解除すると、この人の句が見えるようになります。</>
            ) : isMine ? (
              <>右上の「詠む」から最初の一句を。</>
            ) : (
              <>
                <Link href="/">全体タイムライン</Link>で他の句を読んでみてください。
              </>
            ),
          }}
          initialCursor={posts.data.next_cursor}
          initialPosts={posts.data.items}
          kind={`user:${handle}`}
          moreHref={publicPath}
          signedIn={user !== null}
          viewerHandle={user?.handle}
        />
      ) : (
        <p role="alert">句を読み込めませんでした。時間をおいてお試しください。</p>
      )}
    </AppShell>
  );
}
