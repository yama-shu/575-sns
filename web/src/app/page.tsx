/**
 * S-01 全体タイムライン（基本設計 04 §1）。
 *
 * 未ログインでも閲覧できる。ログイン済みならブロック関係の投稿が除外され、
 * liked_by_me が返る（api 側で処理される）。
 */
import Link from "next/link";

import { AppShell } from "@/components/AppShell";
import { Timeline } from "@/components/Timeline";
import { currentUser } from "@/lib/api/session";
import { fetchTimeline } from "@/lib/api/timeline";

export const metadata = { title: "全体タイムライン | 575" };

// ログイン状態で内容が変わるため、キャッシュしない。
export const dynamic = "force-dynamic";

export default async function PublicTimelinePage({
  searchParams,
}: PageProps<"/">) {
  const params = await searchParams;
  const cursor = typeof params.cursor === "string" ? params.cursor : undefined;

  const [user, timeline] = await Promise.all([currentUser(), fetchTimeline("public", cursor)]);

  if (!timeline.ok) {
    return (
      <AppShell title="全体タイムライン" user={user} current="/">
        <p role="alert">タイムラインを読み込めませんでした。時間をおいてお試しください。</p>
      </AppShell>
    );
  }

  return (
    <AppShell title="全体タイムライン" user={user} current="/">
      <Timeline
        kind="public"
        initialPosts={timeline.data.items}
        initialCursor={timeline.data.next_cursor}
        moreHref="/"
        empty={{
          title: "まだ一句もありません",
          hint:
            user === null ? (
              <>
                <Link href="/signup">アカウントを作る</Link>と、最初の一句を詠めます。
              </>
            ) : (
              "最初の一句を詠んでみてください。"
            ),
        }}
      />
    </AppShell>
  );
}
