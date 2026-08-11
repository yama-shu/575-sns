/**
 * S-02 フォロー中タイムライン（基本設計 04 §1）。
 */
import Link from "next/link";
import { redirect } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { Timeline } from "@/components/Timeline";
import { currentUser } from "@/lib/api/session";
import { fetchTimeline } from "@/lib/api/timeline";

export const metadata = { title: "フォロー中 | 575" };

export const dynamic = "force-dynamic";

export default async function HomeTimelinePage({ searchParams }: PageProps<"/home">) {
  const user = await currentUser();
  // 未ログイン、またはセッション切れ。画面遷移図の「セッション切れ → S-08」。
  if (!user) redirect("/login");

  const params = await searchParams;
  const cursor = typeof params.cursor === "string" ? params.cursor : undefined;
  const timeline = await fetchTimeline("home", cursor);

  if (!timeline.ok) {
    return (
      <AppShell title="フォロー中" user={user} current="/home">
        <p role="alert">タイムラインを読み込めませんでした。時間をおいてお試しください。</p>
      </AppShell>
    );
  }

  return (
    <AppShell title="フォロー中" user={user} current="/home">
      <Timeline
        kind="home"
        initialPosts={timeline.data.items}
        initialCursor={timeline.data.next_cursor}
        signedIn={user !== null}
        moreHref="/home"
        empty={{
          title: "フォロー中の一句はまだありません",
          hint: (
            <>
              <Link href="/">全体タイムライン</Link>で気になる人を見つけてみてください。
            </>
          ),
        }}
      />
    </AppShell>
  );
}
