/**
 * 投稿カード（基本設計 04 §4）。
 *
 * **句ごとに改行して表示する**（FR-03-06）。改行位置は api が返す `segments` を
 * 並べるだけで復元でき、**再判定しない**。web で分割し直すと、辞書の更新で
 * 表示が変わりうるうえ、prosody が落ちているときに閲覧できなくなる。
 */
import { RelativeTime } from "./RelativeTime";
import type { Post } from "@/lib/api/timeline";

import styles from "./PostCard.module.css";

const VERDICT_LABEL: Record<string, string> = {
  teikei: "定型",
  kyoyo: "許容",
};

export function PostCard({ post }: { post: Post }) {
  const isTeikei = post.verdict === "teikei";

  return (
    <article className={styles.card}>
      <header className={styles.header}>
        <span className={styles.displayName}>{post.author.display_name}</span>
        <span className={styles.handle}>@{post.author.handle}</span>
        <RelativeTime className={styles.time} iso={post.created_at} />
      </header>

      {/* 句ごとに <p> を並べる。CSS が効かない状況でも改行が保たれる。 */}
      <div className={styles.body}>
        {post.segments.map((segment, index) => (
          <p className={styles.segment} key={index}>
            {segment.text}
          </p>
        ))}
      </div>

      <footer className={styles.footer}>
        <span
          className={`${styles.verdict} ${isTeikei ? styles.verdictTeikei : styles.verdictKyoyo}`}
        >
          {VERDICT_LABEL[post.verdict] ?? post.verdict}
        </span>
        <span className={styles.likes}>♡ {post.like_count}</span>
      </footer>
    </article>
  );
}
