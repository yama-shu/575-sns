"use client";

/**
 * プロフィール編集フォーム（S-10 / FR-01-03）。
 *
 * **アイコンは扱わない。** 画像を置く場所が設計されていない（#62）。
 */
import { useActionState, useState } from "react";

import { saveProfile, type ProfileFormState } from "@/lib/profile-actions";

import styles from "./ProfileForm.module.css";

/** api の `domain.DisplayNameMaxLength` / `BioMaxLength` と揃える。 */
const DISPLAY_NAME_MAX = 50;
const BIO_MAX = 200;

type Props = { displayName: string; bio: string; handle: string };

export function ProfileForm({ displayName, bio, handle }: Props) {
  const [state, action, pending] = useActionState(saveProfile, {} as ProfileFormState);
  const [name, setName] = useState(state.displayName ?? displayName);
  const [text, setText] = useState(state.bio ?? bio);

  const nameError = state.error?.fields["display_name"];
  const bioError = state.error?.fields["bio"];

  return (
    <form action={action} className={styles.form}>
      <div className={styles.field}>
        <label className={styles.label} htmlFor="display_name">
          表示名
        </label>
        <input
          aria-describedby={`display_name-count${nameError ? " display_name-error" : ""}`}
          aria-invalid={nameError ? true : undefined}
          className={`${styles.input} ${nameError ? styles.invalid : ""}`}
          id="display_name"
          maxLength={DISPLAY_NAME_MAX}
          name="display_name"
          onChange={(event) => setName(event.target.value)}
          required
          value={name}
        />
        <p className={styles.count} id="display_name-count">
          {name.length} / {DISPLAY_NAME_MAX} 文字
        </p>
        {nameError && (
          <p className={styles.error} id="display_name-error" role="alert">
            {nameError}
          </p>
        )}
      </div>

      <div className={styles.field}>
        <label className={styles.label} htmlFor="bio">
          自己紹介
        </label>
        <textarea
          aria-describedby={`bio-count${bioError ? " bio-error" : ""}`}
          aria-invalid={bioError ? true : undefined}
          className={`${styles.textarea} ${bioError ? styles.invalid : ""}`}
          id="bio"
          maxLength={BIO_MAX}
          name="bio"
          onChange={(event) => setText(event.target.value)}
          rows={4}
          value={text}
        />
        <p className={styles.count} id="bio-count">
          {text.length} / {BIO_MAX} 文字
        </p>
        {/* 消せることを明示する。書いたら消せないと思わせない。 */}
        <p className={styles.hint}>
          五七五である必要はありません。空にすると消えます。
        </p>
        {bioError && (
          <p className={styles.error} id="bio-error" role="alert">
            {bioError}
          </p>
        )}
      </div>

      <p className={styles.fixed}>
        識別名（@{handle}）とアイコンは変えられません。
      </p>

      {state.error?.message && (
        <p className={styles.error} role="alert">
          {state.error.message}
        </p>
      )}

      <button aria-busy={pending} className={styles.submit} disabled={pending} type="submit">
        {pending ? "保存しています…" : "保存する"}
      </button>
    </form>
  );
}
