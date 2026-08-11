"use client";

/**
 * タイムラインの一覧と、続きの読み込み。
 *
 * # 無限スクロールにボタンを併置する理由
 *
 * FR-03-03 は無限スクロールを求め、NFR-06-03 は「主要な操作はキーボードのみで
 * 完結できる」ことを求めている。**スクロール検知だけで実装すると、
 * キーボードだけの利用者と JavaScript を無効にしている利用者が過去に遡れない。**
 *
 * 「もっと読む」を常に置き、下端に達したときはそれを自動で押す。
 * ボタンはリンクでもあり、JavaScript が無くても ?cursor= で次のページへ行ける。
 */
import Link from "next/link";
import { useCallback, useEffect, useRef, useState, useTransition } from "react";

import { PostCard } from "./PostCard";
import { loadMore, type TimelineKind } from "@/lib/timeline-actions";
import type { Post } from "@/lib/api/timeline";

import styles from "./Timeline.module.css";

type Props = {
  kind: TimelineKind;
  initialPosts: Post[];
  initialCursor: string | null;
  /** 続きへのリンク先。JavaScript が無い環境で使う。 */
  moreHref: string;
  empty: { title: string; hint: React.ReactNode };
  /** ログインしているか。いいねを押せるかの判断に使う。 */
  signedIn: boolean;
};

export function Timeline({
  kind,
  initialPosts,
  initialCursor,
  moreHref,
  empty,
  signedIn,
}: Props) {
  const [posts, setPosts] = useState(initialPosts);
  const [cursor, setCursor] = useState(initialCursor);
  const [error, setError] = useState<string | undefined>(undefined);
  const [pending, startTransition] = useTransition();
  const sentinel = useRef<HTMLDivElement | null>(null);

  const fetchMore = useCallback(() => {
    if (cursor === null || pending) return;
    startTransition(async () => {
      const result = await loadMore(kind, cursor);
      setError(result.error);
      if (result.error) return;
      // 重複を避ける。カーソルは api が返した値をそのまま使う。
      setPosts((current) => [...current, ...result.posts]);
      setCursor(result.nextCursor);
    });
  }, [cursor, kind, pending]);

  // 下端に達したら自動で読み込む。ボタンの代わりではなく、押す手間を省くもの。
  useEffect(() => {
    const target = sentinel.current;
    if (!target || cursor === null) return;

    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) fetchMore();
    });
    observer.observe(target);
    return () => observer.disconnect();
  }, [cursor, fetchMore]);

  if (posts.length === 0) {
    return (
      <div className={styles.empty}>
        <p className={styles.emptyTitle}>{empty.title}</p>
        <p className={styles.emptyHint}>{empty.hint}</p>
      </div>
    );
  }

  return (
    <>
      <div className={styles.list}>
        {posts.map((post) => (
          <PostCard key={post.id} post={post} signedIn={signedIn} />
        ))}
      </div>

      {error && (
        <p className={styles.end} role="alert">
          {error}
        </p>
      )}

      {cursor === null ? (
        <p className={styles.end}>これ以上はありません</p>
      ) : (
        <div className={styles.more} ref={sentinel}>
          {/*
            JavaScript が有効なときはボタンとして押し、無効なときはリンクとして辿る。
            Link に onClick を付けて既定の遷移を止める形にすると、
            JavaScript が無い環境でそのまま動く。
          */}
          <Link
            className={styles.moreButton}
            href={`${moreHref}${moreHref.includes("?") ? "&" : "?"}cursor=${cursor}`}
            onClick={(event) => {
              event.preventDefault();
              fetchMore();
            }}
            aria-busy={pending}
          >
            {pending ? "読み込み中…" : "もっと読む"}
          </Link>
        </div>
      )}
    </>
  );
}
