"use client";

/**
 * 投稿フォーム（S-07 / 基本設計 04 §3）。
 *
 * モーダルと `/compose` ページの両方で使う。**同じ部品を置く**ことで、
 * JavaScript の有無で入力できる内容が変わらないようにする。
 */
import { useActionState, useState } from "react";

import { ProsodyPreview } from "./ProsodyPreview";
import { useProsodyCheck } from "./useProsodyCheck";
import { submitPost, type ComposeState } from "@/lib/compose-actions";
import { BODY_MAX_LENGTH, isPostable } from "@/lib/post-rules";

import styles from "./ComposeForm.module.css";

const INITIAL: ComposeState = {};

/**
 * 本文の入力欄の id。
 *
 * モーダルから参照する。`showModal()` はダイアログ自身にフォーカスを移すため、
 * 開いた後に明示的に移し直す必要がある（ComposeButton）。
 */
export const BODY_FIELD_ID = "compose-body";

export function ComposeForm({ onDone }: { onDone?: () => void }) {
  const [state, formAction, pending] = useActionState(submitPost, INITIAL);
  const [body, setBody] = useState(state.body ?? "");
  /**
   * 日本語入力の変換中か。
   *
   * 変換中の文字列は最終的な本文ではないため、判定しても意味がない
   * （基本設計 04 §3）。
   */
  const [composing, setComposing] = useState(false);

  const { status, retry } = useProsodyCheck(body, composing);
  const bodyError = state.error?.fields["body"];

  // **判定できたものだけを通す。** 判定前・判定中・判定不能では押させない。
  // これは体験のためであり、防御ではない（api が必ず再判定する）。
  const canSubmit =
    status.kind === "checked" &&
    isPostable(status.result.verdict) &&
    body.length <= BODY_MAX_LENGTH;

  return (
    <form action={formAction} className={styles.form}>
      <label className={styles.label} htmlFor={BODY_FIELD_ID}>
        本文
      </label>
      <textarea
        aria-describedby={`compose-count${bodyError ? " compose-body-error" : ""}`}
        aria-invalid={bodyError ? true : undefined}
        autoFocus
        className={styles.textarea}
        id={BODY_FIELD_ID}
        maxLength={BODY_MAX_LENGTH}
        name="body"
        onChange={(event) => setBody(event.target.value)}
        onCompositionEnd={() => setComposing(false)}
        onCompositionStart={() => setComposing(true)}
        placeholder="今日もまた会議のための会議かな"
        required
        rows={3}
        value={body}
      />

      <p className={styles.count} id="compose-count">
        {body.length} / {BODY_MAX_LENGTH} 文字
      </p>

      {/*
        判定結果。JavaScript が無ければ「判定中…」のまま動かないため、
        その環境では投稿してからサーバー側の判定結果を受け取ることになる。
      */}
      <ProsodyPreview status={status} />

      {status.kind === "unavailable" && (
        <button className={styles.retry} onClick={retry} type="button">
          もう一度判定する
        </button>
      )}

      {bodyError && (
        <p className={styles.error} id="compose-body-error" role="alert">
          {bodyError}
        </p>
      )}
      {state.error?.message && (
        <p className={styles.error} role="alert">
          {state.error.message}
        </p>
      )}

      <div className={styles.actions}>
        <label className={styles.visibility} htmlFor="compose-visibility">
          公開範囲
          <select
            className={styles.select}
            defaultValue={state.visibility ?? "public"}
            id="compose-visibility"
            name="visibility"
          >
            <option value="public">全体公開</option>
            <option value="followers">フォロワーのみ</option>
          </select>
        </label>

        {onDone && (
          <button className={styles.cancel} onClick={onDone} type="button">
            やめる
          </button>
        )}
        <button
          aria-busy={pending}
          className={styles.submit}
          /*
            **本文が空のときは無効にしない。** サーバー側で描画した時点の本文は
            常に空であり、ここで無効にすると JavaScript が無い環境で
            永久に押せないボタンになる（判定を呼べないため canSubmit が false のまま）。

            空でないときだけ判定結果に従う。この分岐は JavaScript が動く環境でのみ
            意味を持つ。
          */
          disabled={pending || (body.trim() !== "" && !canSubmit)}
          type="submit"
        >
          {pending ? "詠んでいます…" : "詠む"}
        </button>
      </div>
    </form>
  );
}
