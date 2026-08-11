/**
 * S-08 ログイン（基本設計 04 §1）。
 */
import { redirect } from "next/navigation";

import { AuthForm, type FieldSpec } from "@/components/AuthForm";
import { currentUser } from "@/lib/api/session";
import { logIn } from "@/lib/auth-actions";

export const metadata = { title: "ログイン | 575" };

// ログイン状態で内容が変わるため、キャッシュしない。
export const dynamic = "force-dynamic";

const FIELDS: FieldSpec[] = [
  { name: "handle", label: "識別名", type: "text", autoComplete: "username", maxLength: 20 },
  {
    name: "password",
    label: "パスワード",
    type: "password",
    autoComplete: "current-password",
  },
];

export default async function LoginPage() {
  // ログイン済みなら入る意味がない。画面遷移図のとおり /home へ送る。
  if (await currentUser()) redirect("/home");

  return (
    <AuthForm
      title="ログイン"
      fields={FIELDS}
      submitLabel="ログイン"
      action={logIn}
      switchPrompt="はじめての方は"
      switchHref="/signup"
      switchLabel="アカウントを作る"
    />
  );
}
