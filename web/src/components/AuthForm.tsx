"use client";

/**
 * ログイン・登録に共通するフォームの骨格。
 *
 * **フォーム要素で組む。** `<form>` と `<button type="submit">` を使うことで、
 * Enter での送信とキーボードだけの操作が既定で成立する（NFR-06-03）。
 */
import Link from "next/link";
import { useActionState } from "react";

import type { AuthState } from "@/lib/auth-actions";
import { NO_ERROR } from "@/lib/api/errors";

import styles from "./AuthForm.module.css";

export type FieldSpec = {
  name: string;
  label: string;
  type: "text" | "email" | "password";
  autoComplete: string;
  hint?: string;
  required?: boolean;
  maxLength?: number;
};

type Props = {
  title: string;
  fields: FieldSpec[];
  submitLabel: string;
  action: (prev: AuthState, form: FormData) => Promise<AuthState>;
  switchPrompt: string;
  switchHref: string;
  switchLabel: string;
};

export function AuthForm({
  title,
  fields,
  submitLabel,
  action,
  switchPrompt,
  switchHref,
  switchLabel,
}: Props) {
  const [state, formAction, pending] = useActionState(action, NO_ERROR);

  return (
    <main className={styles.screen}>
      <div className={styles.card}>
        <div className={styles.brand}>
          <h1 className={styles.brandName}>575</h1>
          <p className={styles.brandTagline}>言いたいことは、五七五で。</p>
        </div>

        <h2 className={styles.title}>{title}</h2>

        <form className={styles.form} action={formAction} noValidate>
          {/*
            フォーム全体のエラー。role="alert" で、送信後に読み上げへ伝わるようにする。
          */}
          {state.message && (
            <p className={styles.formError} role="alert">
              {state.message}
            </p>
          )}

          {fields.map((field) => {
            const error = state.fields[field.name];
            const errorId = `${field.name}-error`;
            const hintId = `${field.name}-hint`;
            const describedBy = [error && errorId, field.hint && hintId]
              .filter(Boolean)
              .join(" ");

            return (
              <div className={styles.field} key={field.name}>
                <label className={styles.label} htmlFor={field.name}>
                  {field.label}
                </label>
                {field.hint && (
                  <span className={styles.hint} id={hintId}>
                    {field.hint}
                  </span>
                )}
                <input
                  className={`${styles.input} ${error ? styles.inputInvalid : ""}`}
                  id={field.name}
                  name={field.name}
                  type={field.type}
                  autoComplete={field.autoComplete}
                  required={field.required ?? true}
                  maxLength={field.maxLength}
                  aria-invalid={error ? true : undefined}
                  aria-describedby={describedBy || undefined}
                />
                {error && (
                  <span className={styles.fieldError} id={errorId} role="alert">
                    {error}
                  </span>
                )}
              </div>
            );
          })}

          <button className={styles.submit} type="submit" disabled={pending}>
            {pending ? "送信中…" : submitLabel}
          </button>
        </form>

        <p className={styles.switch}>
          {switchPrompt} <Link href={switchHref}>{switchLabel}</Link>
        </p>
      </div>
    </main>
  );
}
