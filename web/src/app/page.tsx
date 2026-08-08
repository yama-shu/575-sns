/**
 * 開発環境が組み上がったことを確認するための暫定ページ。
 *
 * 本来の画面（S-01 全体タイムライン）は M4 で実装する。
 * ここでは web → api → prosody / db の疎通が取れていることだけを表示する。
 */

// 常にサーバ側で取得する。開発環境の「今の状態」を見るページのため、キャッシュしない。
export const dynamic = "force-dynamic";

type Readiness = {
  ready: boolean;
  dependencies: Record<string, boolean>;
};

const API_INTERNAL_URL = process.env.WEB_API_INTERNAL_URL ?? "http://api:8080";

async function fetchReadiness(): Promise<Readiness | null> {
  try {
    const response = await fetch(`${API_INTERNAL_URL}/readyz`, { cache: "no-store" });
    return (await response.json()) as Readiness;
  } catch {
    // api 自体に到達できない場合。画面は表示し、到達できないことを伝える。
    return null;
  }
}

export default async function Home() {
  const readiness = await fetchReadiness();

  return (
    <main>
      <h1>575</h1>
      <p>言いたいことは、五七五で。</p>

      <h2>サービスの疎通状況</h2>
      {readiness === null ? (
        <p>api に到達できません（{API_INTERNAL_URL}）</p>
      ) : (
        <ul>
          <li>api: 到達できました</li>
          {Object.entries(readiness.dependencies).map(([name, ok]) => (
            <li key={name}>
              {name}: {ok ? "到達できました" : "到達できません"}
            </li>
          ))}
        </ul>
      )}

      <p>
        設計ドキュメントは{" "}
        <a href="https://github.com/yama-shu/575-sns/tree/main/docs">docs/</a> にあります。
      </p>
    </main>
  );
}
