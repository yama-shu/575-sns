/**
 * 投稿カード（基本設計 04 §4）。
 *
 * **句ごとに改行して表示する**（FR-03-06）。改行位置は api が返す `segments` を
 * 並べるだけで復元でき、**再判定しない**。web で分割し直すと、辞書の更新で
 * 表示が変わりうるうえ、prosody が落ちているときに閲覧できなくなる。
 */
import Link from "next/link";

import { LikeButton } from "./LikeButton";
import { ReportButton } from "./ReportButton";
import { RelativeTime } from "./RelativeTime";
import type { Post } from "@/lib/api/timeline";

import styles from "./PostCard.module.css";

const VERDICT_LABEL: Record<string, string> = {
  teikei: "定型",
  kyoyo: "許容",
};

type Props = {
  post: Post;
  /** ログインしているか。いいねを押せるかの判断に使う。 */
  signedIn: boolean;
  /** 投稿詳細ではリンクにしない。いま見ているページへのリンクは意味がない。 */
  standalone?: boolean;
  /**
   * 閲覧者の識別名。**自分の句に通報を出さないため**に使う（BR-07）。
   * 未ログインなら undefined。
   */
  viewerHandle?: string;
};

export function PostCard({ post, signedIn, standalone = false, viewerHandle }: Props) {
  const isTeikei = post.verdict === "teikei";
  // 自分の句は通報できない（BR-07）。未ログインでも出さない（api が 401 で断る）。
  const canReport = signedIn && viewerHandle !== undefined && viewerHandle !== post.author.handle;

  return (
    <article className={`${styles.card} ${standalone ? styles.standalone : ""}`}>
      <header className={styles.header}>
        <Link className={styles.author} href={`/@${post.author.handle}`}>
          <span className={styles.displayName}>{post.author.display_name}</span>
          <span className={styles.handle}>@{post.author.handle}</span>
        </Link>
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
        {/* 一覧では本文をたどれるようにする。詳細では自分自身への導線になるため出さない。 */}
        {!standalone && (
          <Link className={styles.detail} href={`/posts/${post.id}`}>
            この句を見る
          </Link>
        )}
        {canReport && <ReportButton postId={post.id} />}
        <span className={styles.likes}>
          <LikeButton
            likeCount={post.like_count}
            likedByMe={post.liked_by_me}
            postId={post.id}
            signedIn={signedIn}
          />
        </span>
      </footer>
    </article>
  );
}
