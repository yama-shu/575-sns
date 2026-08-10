# 性能測定 0002: タイムラインの実行計画

| 項目 | 内容 |
| --- | --- |
| 測定日 | 2026-08-10 |
| 対象 | 全体タイムライン / フォロー中タイムライン |
| 関連 Issue | [#40](https://github.com/yama-shu/575-sns/issues/40)（実装は [#38](https://github.com/yama-shu/575-sns/issues/38)） |
| 根拠 | [基本設計 03 §3](../design/basic/03-database.md#実行計画の確認を必須とする) / [ADR-0005](../adr/0005-timeline-strategy.md) |

---

## 1. 結論

| 対象 | 結果 |
| --- | --- |
| 全体タイムライン | **問題なし。** 20 行を読んで 20 件を返す。1.08 ms |
| フォロー中タイムライン（通常） | **問題なし。** 35 行を読んで 20 件を返す。1.00 ms |
| フォロー中タイムライン（フォロー比率が低い） | **要注意。** 1,729 行 / 3.27 ms |
| フォロー中タイムライン（極端な例） | **要改善。** 70,756 行 / 47.50 ms |
| カーソル方式 vs `OFFSET` | **設計の判断は正しい。** 深いページで 77 倍の差 |
| `blocks` の双方向除外 | 現規模では追加のインデックス不要 |

[NFR-01-02](../requirements/01-requirements.md#nfr-01-性能)（タイムライン取得の P95 < 300ms）は、
最悪の測定値 47.50 ms でも満たしている。ただし**フォロー中タイムラインだけは
投稿総数に比例して悪化する経路がある**。詳細は §6。

改善案（`LATERAL` への書き換え）は測定済みで、極端な例で **47.50 ms → 0.46 ms**。
別 Issue とする（§8）。

---

## 2. 測定対象

[基本設計 03 §3](../design/basic/03-database.md#主要クエリと使われるインデックス) が
「575 で最も頻繁に実行される」とし、実行計画の取得を必須としている2クエリ。

実装は [api/internal/infra/postgres/timeline.go](../../api/internal/infra/postgres/timeline.go)。

---

## 3. 測定方法

### 実行方法

```bash
./scripts/bench/timeline_explain.sh            # 投入してから測定する
./scripts/bench/timeline_explain.sh --no-seed  # すでにあるデータで測定する
```

データの投入は [scripts/bench/seed_timeline.sql](../../scripts/bench/seed_timeline.sql)。

### 計測の条件

- `EXPLAIN (ANALYZE, BUFFERS)` を使う
- **同じクエリを2回実行し、2回目を採る。** 1回目はキャッシュが冷えており、
  ディスク読み込みの時間が混ざる
- 投入後に `ANALYZE` を実行する。統計が古いと、プランナが実際の行数を知らないまま
  計画を立て、その計画は本番で再現しない

---

## 4. 測定条件

### 投入したデータ

| テーブル | 件数 | 備考 |
| --- | ---: | --- |
| users | 1,000 | |
| posts | 120,000 | うち published かつ public が 102,585 |
| follows | 50,100 | |
| blocks | 549 | |
| likes | 13,909 | 測定の主体（利用者1）が 3,000 件 |

基本設計 03 §3 が求める「posts 10万行以上」を満たす。

### 分布の偏り

全員が同じ数の投稿を持つデータでは、実際とは違う計画が選ばれうる。

| 層 | 人数 | 1人あたりの投稿数 |
| --- | ---: | ---: |
| 上位 1% | 10 | 2,000 |
| 中位 19% | 190 | 400 |
| 下位 80% | 800 | 30 |

- 公開範囲は `public` : `followers` = 9 : 1
- 5% を論理削除済みにする（部分インデックスが削除済みの蓄積に強いことを確かめるため）
- ブロックは 0 件にしない。0 件だとプランナが Seq Scan を選び、判断できない（[#38](https://github.com/yama-shu/575-sns/issues/38)）

### 測定の主体

| 利用者 | フォロー数 | フォロー先の投稿が全体に占める割合 |
| --- | ---: | ---: |
| 利用者1 | 150 | 61.99% |
| 利用者700 | 50 | 1.25% |
| 利用者999（作為的） | 1 | 0.03%（29 件） |

フォロー数 150 は、[ADR-0005](../adr/0005-timeline-strategy.md) が
「1ユーザーあたりの平均フォロー数」を監視指標としているため、
平均より多い側を想定して選んだ。

---

## 5. 測定環境

| 項目 | 値 |
| --- | --- |
| PostgreSQL | 18.4 (Debian) / aarch64 |
| CPU | 4 コア（Docker に割り当て） |
| メモリ | 4.1 GB（Docker に割り当て） |
| shared_buffers | 160 MB |
| work_mem | 4 MB |
| effective_cache_size | 5 GB |
| random_page_cost | 4 |

**本番構成ではない。** ConoHa VPS 2 GB 上での測定は
M5 の「本番構成で性能を再測定する」で行う（[ADR-0007](../adr/0007-hosting-conoha-vps.md)）。

---

## 6. 測定結果

### A. 全体タイムライン

```
Limit  (actual time=0.938..1.083 rows=20.00 loops=1)
  ->  Nested Loop  (actual time=0.937..1.080 rows=20.00 loops=1)
        ->  Nested Loop Anti Join  (actual time=0.111..0.200 rows=20.00 loops=1)
              Join Filter: (((b.blocker_id = 1) AND (b.blocked_id = p.author_id))
                         OR ((b.blocker_id = p.author_id) AND (b.blocked_id = 1)))
              ->  Index Scan Backward using posts_pkey on posts p
                    (actual time=0.064..0.070 rows=20.00 loops=1)
                    Filter: status = 'published' AND visibility = 'public'
                    Rows Removed by Filter: 2
Execution Time: 1.083 ms
```

| 確認項目 | 期待 | 実測 |
| --- | --- | --- |
| `posts` に Seq Scan が出ない | Index Scan | Index Scan Backward |
| 早期終了している | 読んだ行数が 20 前後 | **22 行**（20 + 除外 2） |

**部分インデックス #6 ではなく主キーが使われている。** 述語（published かつ public）が
全体の 85% に当てはまるため、プランナは主キーを逆順に辿って絞るほうが安いと判断した。
どちらでも「並べ替えずに上位 20 件を取れている」ことに変わりはない。

### B. フォロー中タイムライン

フォロー先の投稿が全体に占める割合によって、読む行数が変わる。

| 測定の主体 | 割合 | 読んだ行数 | 実行時間 |
| --- | ---: | ---: | ---: |
| 利用者1 | 61.99% | 35 | 1.00 ms |
| 利用者700 | 1.25% | 1,729 | 3.27 ms |
| 利用者999（1人だけフォロー） | 0.03% | **70,756** | **47.50 ms** |

いずれも `posts_pkey` を逆順に辿り、1行ずつ `follows` を引いて絞る計画になる。

```
->  Index Scan Backward using posts_pkey on posts p  (rows=70756.00 loops=1)
      Filter: (status)::text = 'published'
->  Memoize  (loops=70756)
      ->  Index Only Scan using follows_pkey on follows f
            Index Cond: ((follower_id = 999) AND (followee_id = p.author_id))
```

**設計が想定したインデックス #7（`(author_id, id DESC)`）は使われていない。**

読む行数は「20 ÷ フォロー先の投稿の割合」に比例する。
割合が同じなら投稿総数が増えても行数は変わらないが、
**割合が下がるほど直線的に増える**。

### C. カーソルの深さによる違い

全体タイムラインを深い位置から取得した場合。

| cursor | 実行時間 |
| ---: | ---: |
| なし（先頭） | 1.08 ms |
| 119,000 | 0.87 ms |
| 60,000 | 1.41 ms |
| 1,000（末尾付近） | 0.23 ms |

**深さによる劣化はない。** ADR-0005 がカーソル方式を選んだ狙いどおりである。

### D. `OFFSET` 方式との比較

[基本設計 03 §5](../design/basic/03-database.md#なぜ-offset-を使わないか) の
「`OFFSET` はページが深くなるほど線形に遅くなる」を数値で確かめた。

同じ条件（`status='published' AND visibility='public'`、20件取得）で比較する。

| 位置 | `OFFSET` 方式 | カーソル方式 | 差 |
| ---: | ---: | ---: | ---: |
| 先頭 | 0.206 ms | 0.233 ms | ほぼ同じ |
| 1,000 件目 | 0.201 ms | 0.085 ms | 2.4 倍 |
| 20,000 件目 | 3.062 ms | 0.131 ms | 23 倍 |
| 100,000 件目 | **15.284 ms** | **0.199 ms** | **77 倍** |

読み込んだバッファ数の差がそのまま出ている。

| 位置 | `OFFSET` のバッファ | カーソルのバッファ |
| ---: | ---: | ---: |
| 先頭 | 6 | 9 |
| 20,000 件目 | 633 | 9 |
| 100,000 件目 | **3,151** | **8** |

`OFFSET 100000` は 10 万行を読み飛ばすために 10 万行を実際に読む。
カーソル方式はインデックス上の位置へ直接ジャンプするため、
**どの位置でも読むバッファ数が変わらない**。

無限スクロール（[FR-03-03](../requirements/01-requirements.md#fr-03-閲覧)）で
利用者が深いページへ進むほど差が開く。設計の判断は正しかった。

### E. `blocks` の双方向除外

```
->  Materialize  (actual time=0.004..0.004 rows=50.00 loops=20)
      ->  Seq Scan on blocks b  (actual time=0.008..0.030 rows=50.00 loops=1)
            Filter: ((blocker_id = 1) OR (blocked_id = 1))
            Rows Removed by Filter: 499
            Buffers: shared hit=4
```

Seq Scan だが、**テーブル全体を1回だけ読んで結果を保持している**（`loops=1`）。
自分に関係する 50 行を取り出したあとは、Anti Join がその 50 行だけを見る。
549 行のテーブルを 4 ページ読むだけであり、これがボトルネックになることはない。

**追加のインデックスは不要。** [基本設計 03 §3](../design/basic/03-database.md) の
「必要な場所にだけ張る」に従い、予防的に足さない。

`blocks` が数万行規模になったときにプランナが主キーを使う計画へ切り替えるかは、
その時点で再測定する。主キーは `(blocker_id, blocked_id)` の順だが、
双方向の条件は**両方の枝で `blocker_id` を等値で絞る**ため、そのまま使える。

### F. `liked_by_me` の取得

```
SubPlan 2
  ->  Bitmap Heap Scan on likes l  (actual time=0.101..0.301 rows=3000.00 loops=1)
        Recheck Cond: (user_id = 1)
        Buffers: shared hit=36
```

`EXISTS` が**ハッシュ化された副問い合わせ**になり、
閲覧者のいいねを1回だけ読んでからハッシュで照合している。
投稿20件ぶんのループにはなっていない（`loops=1`）。N+1 は起きていない。

ただし**閲覧者のいいね数に比例する**。3,000 件で 0.30 ms・36 バッファ。
10 万件なら 10 ミリ秒前後になる計算であり、無視できなくなる。

| いいね数 | 見込み |
| ---: | --- |
| 3,000 | 0.30 ms（実測） |
| 100,000 | 10 ms 前後（外挿） |

いいね機能の実装後、実際の分布を見て再確認する。

---

## 7. 測定設計の誤りと訂正

### 投稿 ID と著者 ID を相関させてしまった

1回目の測定では、フォロー中タイムライン（利用者1・フォロー比率 62%）が
**41,440 行を読んで 20 件を返した**。フォロー比率が高いのに読む行数が多く、辻褄が合わない。

原因は投入スクリプトにあった。著者ごとにまとめて投稿を挿入したため、
`BIGSERIAL` が著者順に並び、**投稿 ID と著者 ID が相関**していた。

```
利用者1〜10    → 投稿 ID の若い側に集中
利用者800〜1000 → 投稿 ID の新しい側に集中
```

タイムラインは ID 降順に読む。新しい投稿がフォローしていない利用者に偏っていれば、
フォロー先の投稿にたどり着くまで大量に読み飛ばすことになる。
これは**現実には存在しない分布**であり、測定していたのはデータの作り方の癖だった。

投入時に `ORDER BY random()` を加えて相関を消した。

```
corr(id, author_id) = -0.0037
```

再測定で 41,440 行 → 35 行になった。

**この誤りに気づけたのは、フォロー比率と読んだ行数が矛盾していたためである。**
実行計画の数値どうしが整合するかを確かめないと、こうした癖を実装の問題と誤診する。
[#9](https://github.com/yama-shu/575-sns/issues/9) で飽和試験を応答時間の測定と取り違えたのと同じ種類の誤りである。

---

## 8. 改善案（別 Issue とする）

フォロー中タイムラインを `LATERAL` で書き換えると、
設計が想定したインデックス #7 を使う計画になる。

```sql
SELECT t.* FROM (
  SELECT p.* FROM follows f
  JOIN LATERAL (
    SELECT * FROM posts p
    WHERE p.author_id = f.followee_id AND p.status = 'published' AND p.id < :cursor
    ORDER BY p.id DESC LIMIT :limit
  ) p ON true
  WHERE f.follower_id = :me
) t ORDER BY t.id DESC LIMIT :limit;
```

```
->  Index Only Scan using posts_author_timeline_idx on posts p
      (actual time=0.009..0.010 rows=20.00 loops=50)
```

| 測定の主体 | 現行 | `LATERAL` |
| --- | ---: | ---: |
| 利用者700（1.25%） | 1,729 行 / 3.27 ms | 1,000 行 / 1.05 ms |
| 利用者999（1人だけ） | 70,756 行 / **47.50 ms** | 20 行 / **0.46 ms** |

読む行数が「フォロー数 × `limit`」で頭打ちになり、**投稿総数に依存しない**。

一方、フォロー比率が高い利用者では現行のほうが読む行数が少ない
（利用者1 は 35 行、`LATERAL` なら 150 × 20 = 3,000 行）。
どちらが常に速いというものではない。

### いま書き換えない理由

- 最悪でも 47.50 ms であり、[NFR-01-02](../requirements/01-requirements.md#nfr-01-性能) の 300ms に対して余裕がある
- [ADR-0005](../adr/0005-timeline-strategy.md) は方式変更の判断基準を
  「P95 が 300ms を超え続けること」としており、その段階にない
- 書き換えは挙動の同一性を検証する必要があり、測定の記録とは別の作業である

**[#41](https://github.com/yama-shu/575-sns/issues/41) として起票した。** 悪化の条件（フォロー比率の低下・投稿総数の増加）は
本記録に残したため、判断の材料は揃っている。

---

## 9. 未解決の事項

| 事項 | 対応 |
| --- | --- |
| フォロー中タイムラインが投稿総数に比例して悪化する経路 | [#41](https://github.com/yama-shu/575-sns/issues/41)（§8） |
| `liked_by_me` が閲覧者のいいね数に比例する | いいね実装後に再確認（§6-F） |
| 本番構成（ConoHa VPS 2 GB）での測定 | M5「本番構成で性能を再測定する」 |
| `blocks` が数万行規模になったときの計画 | その時点で再測定（§6-E） |

---

## 関連ドキュメント

- [ADR-0005: タイムラインの構築方式](../adr/0005-timeline-strategy.md)
- [基本設計 03: データベース設計](../design/basic/03-database.md)
- [性能測定 0001: 判定エンジン](0001-prosody-benchmark.md)
