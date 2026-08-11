/**
 * S-03 投稿詳細（基本設計 04 §1）。
 */
import { notFound } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { DeletePostButton } from "@/components/DeletePostButton";
import { PostCard } from "@/components/PostCard";
import { currentUser } from "@/lib/api/session";
import { fetchPost } from "@/lib/api/user";

export const metadata = { title: "句 | 575" };

export const dynamic = "force-dynamic";

export default async function PostPage({ params }: PageProps<"/posts/[id]">) {
  const { id } = await params;
  const [user, result] = await Promise.all([currentUser(), fetchPost(id)]);

  // **理由を出し分けない。** 削除済み・ブロック関係・フォロワー限定のいずれも
  // api が 404 を返す。画面で区別すると存在を教えてしまう（BR-10）。
  if (!result.ok) {
    if (result.status === 404) notFound();
    return (
      <AppShell current="/" title="句" user={user}>
        <p role="alert">読み込めませんでした。時間をおいてお試しください。</p>
      </AppShell>
    );
  }

  const post = result.data;
  const isMine = user !== null && user.handle === post.author.handle;

  return (
    <AppShell current="/" title="句" user={user}>
      <PostCard post={post} signedIn={user !== null} standalone />
      {isMine && <DeletePostButton postId={post.id} />}
    </AppShell>
  );
}
