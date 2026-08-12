/**
 * S-05 フォロー中一覧（基本設計 04 §1）。
 *
 * 外向きの経路は `/@:handle/following` である（`next.config.ts` の rewrite）。
 */
import { RelationListPage } from "@/components/RelationListPage";

export const metadata = { title: "フォロー中 | 575" };

export const dynamic = "force-dynamic";

export default async function FollowingPage({
  params,
  searchParams,
}: PageProps<"/users/[handle]/following">) {
  const { handle } = await params;
  const query = await searchParams;
  const cursor = typeof query.cursor === "string" ? query.cursor : undefined;

  return <RelationListPage cursor={cursor} handle={handle} kind="following" />;
}
