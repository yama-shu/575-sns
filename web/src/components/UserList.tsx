"use client";

/**
 * 利用者の一覧と、続きの読み込み（S-05 / S-06 / S-11）。
 *
 * # なぜ Timeline と別の部品なのか
 *
 * 並びと「もっと読む」の作りは [#50](https://github.com/yama-shu/575-sns/issues/50) の `Timeline` と同じだが、
 * あちらは投稿カードを前提にしている。`Post` と `RelationUser` を1つの部品で
 * 扱うと、項目の描き方ごとに分岐が増える。
 *
 * **操作を置かない。** api は `following` を返すが、一覧の中で押すと
 * ページごと描き直され、どこを押したのか分からなくなる（#60 の redirect の形）。
 * 一覧は「誰がいるか」を見て、ユーザーページへ移るための画面にする。
 */
import Link from "next/link";
import { useCallback, useEffect, useRef, useState, useTransition } from "react";

import { loadMoreUsers } from "@/lib/relation-list-actions";
import type { RelationKind, RelationUser } from "@/lib/api/user";

import styles from "./UserList.module.css";

type Props = {
  kind: RelationKind;
  /** 誰の一覧か。ブロック中では使わない。 */
  handle: string;
  initialUsers: RelationUser[];
  initialCursor: string | null;
  /** 続きへのリンク先。JavaScript が無い環境で使う。 */
  moreHref: string;
  empty: { title: string; hint: React.ReactNode };
};

export function UserList({
  kind,
  handle,
  initialUsers,
  initialCursor,
  moreHref,
  empty,
}: Props) {
  const [users, setUsers] = useState(initialUsers);
  const [cursor, setCursor] = useState(initialCursor);
  const [error, setError] = useState<string | undefined>(undefined);
  const [pending, startTransition] = useTransition();
  const sentinel = useRef<HTMLDivElement | null>(null);

  const fetchMore = useCallback(() => {
    if (cursor === null || pending) return;
    startTransition(async () => {
      const result = await loadMoreUsers(kind, handle, cursor);
      setError(result.error);
      if (result.error) return;
      setUsers((current) => [...current, ...result.users]);
      setCursor(result.nextCursor);
    });
  }, [cursor, handle, kind, pending]);

  useEffect(() => {
    const target = sentinel.current;
    if (!target || cursor === null) return;

    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) fetchMore();
    });
    observer.observe(target);
    return () => observer.disconnect();
  }, [cursor, fetchMore]);

  if (users.length === 0) {
    return (
      <div className={styles.empty}>
        <p className={styles.emptyTitle}>{empty.title}</p>
        <p className={styles.emptyHint}>{empty.hint}</p>
      </div>
    );
  }

  return (
    <>
      <ul className={styles.list}>
        {users.map((user) => (
          <li key={user.handle}>
            <Link className={styles.item} href={`/@${user.handle}`}>
              <span className={styles.displayName}>{user.display_name}</span>
              <span className={styles.handle}>@{user.handle}</span>
              {user.bio && <span className={styles.bio}>{user.bio}</span>}
            </Link>
          </li>
        ))}
      </ul>

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
            #50 の「もっと読む」と同じ形。
          */}
          <Link
            aria-busy={pending}
            className={styles.moreButton}
            href={`${moreHref}${moreHref.includes("?") ? "&" : "?"}cursor=${cursor}`}
            onClick={(event) => {
              event.preventDefault();
              fetchMore();
            }}
          >
            {pending ? "読み込み中…" : "もっと見る"}
          </Link>
        </div>
      )}
    </>
  );
}
