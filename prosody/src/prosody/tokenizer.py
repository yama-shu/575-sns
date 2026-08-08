"""形態素解析器のインターフェース。

ADR-0003 の「解析器を差し替え可能にする」を実装したもの。

`MoraCounter` / `SegmentSearcher` / `ToleranceRule` は
**SudachiPy の存在を一切知らない**。このインターフェースと `Token` にだけ依存する。

これにより次の2つが得られる。

1. 解析器を差し替えるときの影響が実装クラス1つに閉じる
2. 単体テストで解析器のモックを注入でき、**辞書のロードなしにアルゴリズムをテストできる**

`Protocol` を使うのは、モックが継承なしで差し込めるようにするためである。
テスト用の偽物を書くのに本番の基底クラスを import させたくない。
"""

from __future__ import annotations

from typing import Protocol, runtime_checkable

from prosody.token import SplitMode, Token


@runtime_checkable
class Tokenizer(Protocol):
    """文を形態素に分割し、読みと品詞を付与する。"""

    def tokenize(self, text: str, mode: SplitMode) -> list[Token]:
        """`text` を `mode` の粒度で形態素に分割する。

        読みを取得できなかった形態素は `Token.reading` を None にする。
        **ここで数字やラテン文字の読みを補完しない。** それは
        `ReadingResolver` の責務である（詳細設計 01 §4）。
        """
        ...
