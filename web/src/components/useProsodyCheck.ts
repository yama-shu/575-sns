"use client";

/**
 * 入力中の判定を呼ぶ（基本設計 04 §3「判定を呼ぶタイミング」）。
 *
 * **キーストロークごとに呼ばない。** 17文字の入力で17回の形態素解析が走ると
 * prosody の CPU を無駄に使い、入力途中の判定結果が高速で切り替わって読めない。
 */
import { useEffect, useState } from "react";

import { checkBody } from "@/lib/compose-actions";
import type { CheckResult } from "@/lib/api/post";

/** 入力が一段落したと判断する間隔（基本設計 04 §3）。 */
export const DEBOUNCE_MS = 300;

/** 画面に出す判定の状態。 */
export type ProsodyStatus =
  | { kind: "empty" }
  | { kind: "checking" }
  | { kind: "checked"; result: CheckResult }
  /** 判定そのものができなかった。**破調とは違う。** */
  | { kind: "unavailable"; message: string };

/** 判定が終わった本文と、その結果。 */
type Completed =
  | { kind: "checked"; body: string; result: CheckResult }
  | { kind: "unavailable"; body: string; message: string };

/**
 * 本文を判定し、その状態を返す。
 *
 * @param body 入力中の本文
 * @param composing 日本語入力の変換中か。**変換中は判定しない。**
 *   変換途中の文字列は最終的な本文ではないため、判定しても意味がない。
 */
export function useProsodyCheck(
  body: string,
  composing: boolean,
): { status: ProsodyStatus; retry: () => void } {
  const [completed, setCompleted] = useState<Completed | null>(null);
  const text = body.trim();

  useEffect(() => {
    if (text === "" || composing) return;
    // 前回判定した本文と同じなら呼ばない。
    if (completed?.body === text) return;

    // **応答の追い越しに備える。** 判定は非同期で、後から投げた要求が
    // 先に返ることがある。そのまま表示すると、古い本文に対する判定が
    // 新しい本文の結果を上書きする。
    //
    // 本文が変わると effect が作り直され、この後片付けが走る。
    // 古い要求の応答はここで捨てられる。
    let current = true;

    const timer = setTimeout(() => {
      void checkBody(text).then((state) => {
        if (!current) return;
        setCompleted(
          state.status === "ok"
            ? { kind: "checked", body: text, result: state.result }
            : { kind: "unavailable", body: text, message: state.message },
        );
      });
    }, DEBOUNCE_MS);

    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [text, composing, completed]);

  return { status: toStatus(text, completed), retry: () => setCompleted(null) };
}

/**
 * 表示する状態を導出する。
 *
 * **effect の中で setState しない。** 空になったことや判定待ちであることは
 * 本文と直前の結果から決まるため、状態として持つ必要がない
 * （持つと React の規則 react-hooks/set-state-in-effect に反する）。
 */
function toStatus(text: string, completed: Completed | null): ProsodyStatus {
  if (text === "") return { kind: "empty" };
  // 判定済みの本文と違うなら、結果はまだ無い。
  // **古い判定を出さない。** 直した本文に対して前の判定が残ると誤解を招く。
  if (completed === null || completed.body !== text) return { kind: "checking" };
  if (completed.kind === "checked") return { kind: "checked", result: completed.result };
  return { kind: "unavailable", message: completed.message };
}
