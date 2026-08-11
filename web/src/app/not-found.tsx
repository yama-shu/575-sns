/**
 * 見つからなかったときの画面。
 *
 * **理由を出し分けない。** 削除済み・ブロック関係・フォロワー限定・
 * 存在しない識別名のいずれもここに来る。区別すると、
 * 「あるが見せない」ことを教えてしまう（BR-10）。
 *
 * 何が起きたかと次にできることは示す（NFR-06-02）。
 */
import Link from "next/link";

import styles from "./not-found.module.css";

export const metadata = { title: "見つかりません | 575" };

export default function NotFound() {
  return (
    <main className={styles.screen}>
      <p className={styles.code}>404</p>
      <h1 className={styles.title}>見つかりませんでした</h1>
      <p className={styles.hint}>
        削除されたか、もともと無いか、いまは見られない句かもしれません。
      </p>
      <Link className={styles.back} href="/">
        全体タイムラインへ
      </Link>
    </main>
  );
}
