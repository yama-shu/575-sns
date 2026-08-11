/**
 * 画面の骨格とグローバルナビゲーション。
 *
 * **項目を配列で持つ。** 基本設計 04 §6 が「グローバルナビゲーションの構造は、
 * （通知・検索・お題を）後から追加できる形にしておく」としている。
 * 後付けで組み替えると既存利用者の操作感を壊すため。
 */
import Link from "next/link";

import { ComposeButton } from "./ComposeButton";
import { logOut } from "@/lib/auth-actions";
import type { User } from "@/lib/api/session";

import styles from "./AppShell.module.css";

/**
 * ナビゲーションの項目。追加は1行で済む。
 *
 * 基本設計 04 §2 が「すべての画面から S-01 / S-02 / S-04（自分）/ S-10 へ
 * 遷移できる」としている。自分のページは識別名が要るため、
 * `href` を利用者から組み立てる。
 */
const NAV_ITEMS: { href: string | ((handle: string) => string); label: string; requiresAuth: boolean }[] = [
  { href: "/", label: "全体", requiresAuth: false },
  { href: "/home", label: "フォロー中", requiresAuth: true },
  { href: (handle) => `/@${handle}`, label: "自分の句", requiresAuth: true },
  { href: "/settings/profile", label: "設定", requiresAuth: true },
];

type Props = {
  title: string;
  user: User | null;
  /** いま表示している画面のパス。ナビゲーションの強調に使う。 */
  current: string;
  children: React.ReactNode;
};

export function AppShell({ title, user, current, children }: Props) {
  const items = NAV_ITEMS.filter((item) => !item.requiresAuth || user !== null).map((item) => ({
    label: item.label,
    href: typeof item.href === "string" ? item.href : item.href(user?.handle ?? ""),
  }));

  return (
    <div className={styles.shell}>
      <nav className={styles.nav} aria-label="メインナビゲーション">
        {items.map((item) => (
          <Link
            className={`${styles.navItem} ${item.href === current ? styles.navItemActive : ""}`}
            href={item.href}
            key={item.href}
            aria-current={item.href === current ? "page" : undefined}
          >
            {item.label}
          </Link>
        ))}
      </nav>

      <main className={styles.main}>
        <header className={styles.header}>
          <h1 className={styles.title}>{title}</h1>
          <div className={styles.account}>
            {user === null ? (
              <Link href="/login">ログイン</Link>
            ) : (
              <>
                {/* 未ログインでは出さない。詠むにはログインが要る（S-07）。 */}
                <ComposeButton />
                <span>@{user.handle}</span>
                <form action={logOut}>
                  <button className={styles.logout} type="submit">
                    ログアウト
                  </button>
                </form>
              </>
            )}
          </div>
        </header>

        {children}
      </main>
    </div>
  );
}
