| 項目 | 値 |
| --- | --- |
| タイトル | `feat: 五七五の許容ルール判定を実装する` |
| ラベル | `feature` |
| マイルストーン | M1 判定エンジン |
| 起票済み | [#5](https://github.com/yama-shu/575-sns/issues/5) |

---

## 背景・目的

[ADR-0001](../adr/0001-onsuritsu-tolerance.md) で決めた許容ルールを実装する。

上五・中七・下五のモーラ数を受け取り、
**定型 / 許容 / 破調** のいずれかを返す。

このクラスは ADR-0001 の決定そのものであり、
**将来変更される可能性が最も高い部分**である。
ADR-0001 自身が「リリース後の実データを見てから緩和を判断するのが望ましい」と述べている。
そのため探索ロジックから分離した独立のクラスとして実装する。

## やること

- [ ] `ToleranceRule.is_valid(kami, naka, shimo) -> bool` を実装する
- [ ] `ToleranceRule.deviation_count(kami, naka, shimo) -> int` を実装する
- [ ] `ToleranceRule.verdict_of(kami, naka, shimo) -> Verdict` を実装する
- [ ] [詳細設計 04 §2](../design/detail/04-test-design.md#2-許容ルールのテスト設計) の TC-TR-01〜28 をテストとして書く

## 完了条件

- [ ] TC-TR-01〜28 がすべて通る
- [ ] とくに **TC-TR-18〜21（各句は範囲内だがズレ数が2以上）** が通る
- [ ] 投稿可能な7パターン（TC-TR-22〜28）がすべて正しく判定される
- [ ] C1 カバレッジ 100%

## やらないこと

- 区切り位置の探索（`feat: 区切り探索を実装する`）
- モーラ数の計算（`feat: モーラ分割・カウント処理を実装する`）

## 実装上の注意

### 条件は2つある。1つではない

```python
def is_valid(kami: int, naka: int, shimo: int) -> bool:
    # 条件1: 各句が許容範囲に収まる
    if not (4 <= kami <= 6 and 6 <= naka <= 8 and 4 <= shimo <= 6):
        return False
    # 条件2: 規定値から外れている句が高々1つ  ← これを忘れやすい
    deviations = (kami != 5) + (naka != 7) + (shimo != 5)
    return deviations <= 1
```

**条件1だけを実装すると、TC-TR-01〜15 は全部通る。**
そして TC-TR-18〜21 で落ちる。

`4/6/5`（各句は範囲内、ズレ2）を通してしまうと、
[ADR-0001](../adr/0001-onsuritsu-tolerance.md#4-比較) で
「甘すぎる」として却下した案2 を実装したことになる。

### 合計モーラ数のチェックを書かない

[ADR-0001](../adr/0001-onsuritsu-tolerance.md#案3-についての補足-合計モーラ数の制約は不要である) のとおり、
「合計 16〜18」は上記2条件から自動的に導かれる。

念のためと思って第3の条件として書くと、条件が重複する。
片方だけを直したときに矛盾が生じるため、**書かない**。

## 参考

- [ADR-0001: 字余り・字足らずの許容範囲](../adr/0001-onsuritsu-tolerance.md)
- [詳細設計 04 §2: 許容ルールのテスト設計](../design/detail/04-test-design.md#2-許容ルールのテスト設計)
