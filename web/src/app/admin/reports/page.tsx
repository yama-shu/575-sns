/**
 * S-13 通報一覧（運営）（基本設計 04 §1）。
 *
 * **運営でなければ 404 の画面にする。** api も 404 を返す（#74）。
 * 403 にすると、この経路が存在すること自体を教えることになる。
 */
import Link from "next/link";
import { notFound } from "next/navigation";

import { AppShell } from "@/components/AppShell";
import { ReportCard } from "@/components/ReportCard";
import { currentUser } from "@/lib/api/session";
import { fetchPendingReports } from "@/lib/api/user";

import styles from "./page.module.css";

export const metadata = { title: "通報一覧 | 575" };

export const dynamic = "force-dynamic";

/**
 * 処理できなかった理由。
 *
 * **409 を握りつぶさない。** 別の運営が先に処理した可能性があり、
 * 黙って成功にすると二重に判断することになる（#74）。
 */
const FAILURE_MESSAGES: Record<string, string> = {
  ALREADY_HANDLED: "この通報はすでに処理されています。別の運営が先に対応した可能性があります。",
  NOT_FOUND: "この通報は見つかりませんでした。",
};

export default async function AdminReportsPage({ searchParams }: PageProps<"/admin/reports">) {
  const user = await currentUser();
  const query = await searchParams;
  const cursor = typeof query.cursor === "string" ? query.cursor : undefined;

  // 処理の結果は経路で受け取る（admin-actions の但し書き）。
  const failed = typeof query.failed === "string" ? query.failed : undefined;

  const reports = await fetchPendingReports(cursor);
  // 未ログイン・一般利用者・運営でない、のいずれも 404 に落とす。
  if (!reports.ok) {
    if (reports.status === 404) notFound();
    return (
      <AppShell current="/admin/reports" title="通報一覧" user={user}>
        <p role="alert">読み込めませんでした。時間をおいてお試しください。</p>
      </AppShell>
    );
  }

  const items = reports.data.items;

  return (
    <AppShell current="/admin/reports" title="通報一覧" user={user}>
      {failed && (
        <p className={styles.failed} role="alert">
          {FAILURE_MESSAGES[failed] ?? "処理できませんでした。時間をおいてお試しください。"}
        </p>
      )}

      <p className={styles.note}>
        未対応の通報を古い順に出しています。待たせている順に処理してください。
      </p>

      {items.length === 0 ? (
        <div className={styles.empty}>
          <p className={styles.emptyTitle}>未対応の通報はありません</p>
          <p className={styles.emptyHint}>新しい通報が届くとここに出ます。</p>
        </div>
      ) : (
        <div className={styles.list}>
          {items.map((report) => (
            <ReportCard key={report.id} report={report} />
          ))}
        </div>
      )}

      {reports.data.next_cursor && (
        <p className={styles.more}>
          <Link href={`/admin/reports?cursor=${reports.data.next_cursor}`}>次を見る</Link>
        </p>
      )}
    </AppShell>
  );
}
