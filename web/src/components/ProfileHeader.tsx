/**
 * ユーザーページの見出し（S-04）。
 *
 * 数はすべて api が返した値をそのまま出す。**閲覧者から見える数**であり、
 * 一覧に並ぶ件数と一致する（#58）。
 */
import Link from "next/link";

import { RelationButtons } from "./RelationButtons";
import type { Profile } from "@/lib/api/user";

import styles from "./ProfileHeader.module.css";

type Props = {
  profile: Profile;
  /** 自分のページか。フォローもブロックも自分には行えない。 */
  isMine: boolean;
  signedIn: boolean;
  /** 操作のあとに戻る経路（`/@handle`）。 */
  path: string;
};

export function ProfileHeader({ profile, isMine, signedIn, path }: Props) {
  return (
    <header className={styles.profile}>
      <div className={styles.identity}>
        <h2 className={styles.displayName}>{profile.display_name}</h2>
        <p className={styles.handle}>@{profile.handle}</p>
      </div>

      {profile.bio && <p className={styles.bio}>{profile.bio}</p>}

      <dl className={styles.counts}>
        <div className={styles.count}>
          <dt>句</dt>
          <dd>{profile.post_count}</dd>
        </div>
        {/* 数から一覧へ行けるようにする。数字のままでは開けない。 */}
        <div className={styles.count}>
          <dt>
            <Link className={styles.countLink} href={`${path}/following`}>
              フォロー
            </Link>
          </dt>
          <dd>{profile.following_count}</dd>
        </div>
        <div className={styles.count}>
          <dt>
            <Link className={styles.countLink} href={`${path}/followers`}>
              フォロワー
            </Link>
          </dt>
          <dd>{profile.follower_count}</dd>
        </div>
      </dl>

      <p className={styles.joined}>
        {new Date(profile.created_at).toLocaleDateString("ja-JP", {
          year: "numeric",
          month: "long",
          // サーバーとブラウザで食い違わないよう固定する（#56）。
          timeZone: "Asia/Tokyo",
        })}
        から
      </p>

      {profile.blocking && (
        <p className={styles.blocked} role="status">
          この人をブロックしています。句は表示されません。
        </p>
      )}

      {/* 自分には出さない。未ログインではログインへ誘導する。 */}
      {!isMine &&
        (signedIn ? (
          <RelationButtons
            blocking={profile.blocking}
            following={profile.following}
            handle={profile.handle}
            path={path}
          />
        ) : (
          <p className={styles.signedOut}>
            <Link href="/login">ログイン</Link>するとフォローできます。
          </p>
        ))}
    </header>
  );
}
