/**
 * S-02 フォロー中タイムライン（/home）。
 *
 * **本 Issue では遷移先として最小限にとどめる。** 画面遷移図はログイン成功後の
 * 遷移先を /home としており、遷移先を変えると設計と食い違う（#48）。
 * タイムラインの中身は次の Issue で実装する。
 */
import { redirect } from "next/navigation";

import { currentUser } from "@/lib/api/session";
import { logOut } from "@/lib/auth-actions";

import styles from "./page.module.css";

export const metadata = { title: "フォロー中 | 575" };

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const user = await currentUser();
  // 未ログイン、またはセッション切れ。画面遷移図の「セッション切れ → S-08」。
  if (!user) redirect("/login");

  return (
    <main className={styles.screen}>
      <header className={styles.header}>
        <h1 className={styles.brand}>575</h1>
        <div className={styles.account}>
          <span>
            {user.display_name} <span className={styles.handle}>@{user.handle}</span>
          </span>
          {/* form + button で組み、キーボードだけで操作できるようにする。 */}
          <form action={logOut}>
            <button className={styles.logout} type="submit">
              ログアウト
            </button>
          </form>
        </div>
      </header>

      <p className={styles.placeholder}>
        フォロー中タイムラインは次の Issue で実装します。
      </p>
    </main>
  );
}
