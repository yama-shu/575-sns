/**
 * S-12 通報（基本設計 04 §1）。
 *
 * **本来はモーダルである。** このページは JavaScript が無い環境のための経路で、
 * 「通報」リンクの飛び先にあたる（NFR-06-03）。
 */
import Link from "next/link";
import { notFound, redirect } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { PostCard } from "@/components/PostCard";
import { ReportForm } from "@/components/ReportForm";
import { currentUser } from "@/lib/api/session";
import { fetchPost } from "@/lib/api/user";

export const metadata = { title: "通報 | 575" };

export const dynamic = "force-dynamic";

export default async function ReportPage({ params }: PageProps<"/posts/[id]/report">) {
  const { id } = await params;
  const user = await currentUser();
  if (!user) redirect("/login");

  const result = await fetchPost(id);
  if (!result.ok) {
    if (result.status === 404) notFound();
    return (
      <AppShell current="/" title="通報" user={user}>
        <p role="alert">読み込めませんでした。時間をおいてお試しください。</p>
      </AppShell>
    );
  }

  // 自分の句は通報できない（BR-07）。api も 422 で断る。
  if (result.data.author.handle === user.handle) {
    return (
      <AppShell current="/" title="通報" user={user}>
        <p role="alert">自分の句は通報できません。消したい場合は削除してください。</p>
        <p>
          <Link href={`/posts/${id}`}>句へ戻る</Link>
        </p>
      </AppShell>
    );
  }

  return (
    <AppShell current="/" title="この句を通報する" user={user}>
      <PostCard post={result.data} signedIn standalone />
      <ReportForm postId={id} />
    </AppShell>
  );
}
