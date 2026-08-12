/**
 * S-06 フォロワー一覧（基本設計 04 §1）。
 *
 * 外向きの経路は `/@:handle/followers` である（`next.config.ts` の rewrite）。
 */
import { RelationListPage } from "@/components/RelationListPage";

export const metadata = { title: "フォロワー | 575" };

export const dynamic = "force-dynamic";

export default async function FollowersPage({
  params,
  searchParams,
}: PageProps<"/users/[handle]/followers">) {
  const { handle } = await params;
  const query = await searchParams;
  const cursor = typeof query.cursor === "string" ? query.cursor : undefined;

  return <RelationListPage cursor={cursor} handle={handle} kind="followers" />;
}
