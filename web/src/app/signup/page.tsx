/**
 * S-09 アカウント登録（基本設計 04 §1）。
 *
 * 登録に成功すると、そのままログイン済みの状態で /home へ移動する。
 */
import { redirect } from "next/navigation";

import { AuthForm, type FieldSpec } from "@/components/AuthForm";
import { currentUser } from "@/lib/api/session";
import { signUp } from "@/lib/auth-actions";

export const metadata = { title: "アカウント登録 | 575" };

export const dynamic = "force-dynamic";

// 上限は api の domain と揃える（handler 側でも検証される）。
const FIELDS: FieldSpec[] = [
  {
    name: "handle",
    label: "識別名",
    type: "text",
    autoComplete: "username",
    hint: "半角英数字と _ 。あとから変更できません",
    maxLength: 20,
  },
  {
    name: "display_name",
    label: "表示名",
    type: "text",
    autoComplete: "nickname",
    hint: "画面に出る名前です",
    maxLength: 50,
  },
  {
    name: "email",
    label: "メールアドレス",
    type: "email",
    autoComplete: "email",
    maxLength: 255,
  },
  {
    name: "password",
    label: "パスワード",
    type: "password",
    autoComplete: "new-password",
    hint: "8 文字以上",
  },
];

export default async function SignUpPage() {
  if (await currentUser()) redirect("/home");

  return (
    <AuthForm
      title="アカウントを作る"
      fields={FIELDS}
      submitLabel="登録する"
      action={signUp}
      switchPrompt="アカウントをお持ちの方は"
      switchHref="/login"
      switchLabel="ログイン"
    />
  );
}
