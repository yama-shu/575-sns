"""SudachiPy による `Tokenizer` の実装（ADR-0003）。

**SudachiPy を import してよいのはこのモジュールだけである。**
他のモジュールが直接 import していないことは ruff の TID251 で静的に担保する。
テストだけで守ろうとすると、後から誰かが直接 import しても気づけない。

解析は常に C単位で行い、A単位・B単位が要求された場合は
C単位の形態素を分割して得る。こうすると「その形態素が C単位の境界から
始まるか」が分割の構造からそのまま分かり、
A単位探索でのペナルティ（詳細設計 01 §9）に使える。
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, Final

from prosody.token import PartOfSpeech, SplitMode, Token

if TYPE_CHECKING:  # pragma: no cover
    from collections.abc import Iterable

# Sudachi の品詞体系を、区切りコストが必要とする粒度へ正規化する。
_POS_MAP: Final = {
    "名詞": PartOfSpeech.NOUN,
    "動詞": PartOfSpeech.VERB,
    "形容詞": PartOfSpeech.ADJECTIVE,
    "副詞": PartOfSpeech.ADVERB,
    "連体詞": PartOfSpeech.ADNOMINAL,
    "接続詞": PartOfSpeech.CONJUNCTION,
    "感動詞": PartOfSpeech.INTERJECTION,
    "助詞": PartOfSpeech.PARTICLE,
    "助動詞": PartOfSpeech.AUXILIARY_VERB,
    "接頭辞": PartOfSpeech.PREFIX,
    "接尾辞": PartOfSpeech.SUFFIX,
    "補助記号": PartOfSpeech.PUNCTUATION,
    "記号": PartOfSpeech.PUNCTUATION,
    "空白": PartOfSpeech.PUNCTUATION,
}


def to_part_of_speech(sudachi_pos: str) -> PartOfSpeech:
    """Sudachi の品詞名を `PartOfSpeech` へ変換する。"""
    return _POS_MAP.get(sudachi_pos, PartOfSpeech.OTHER)


class SudachiTokenizer:
    """SudachiPy を用いた `Tokenizer` の実装。

    辞書のロードは重いため、インスタンスを使い回す。
    ロード後の辞書は読み取り専用として扱うため、排他は不要である
    （詳細設計 02 §6）。
    """

    def __init__(self, dict_type: str = "core") -> None:
        from sudachipy import Dictionary  # noqa: TID251  ここだけが SudachiPy を知る

        self._dictionary = Dictionary(dict=dict_type)
        self._tokenizer = self._dictionary.create()

    def tokenize(self, text: str, mode: SplitMode) -> list[Token]:
        """`text` を `mode` の粒度で形態素に分割する。"""
        from sudachipy import SplitMode as SudachiSplitMode  # noqa: TID251

        if not text:
            return []

        # 常に C単位で解析する。A / B は C単位を分割して得る。
        coarse = self._tokenizer.tokenize(text, SudachiSplitMode.C)

        if mode is SplitMode.C:
            return [_to_token(morpheme, unit_boundary=True) for morpheme in coarse]

        finer = SudachiSplitMode.A if mode is SplitMode.A else SudachiSplitMode.B
        tokens: list[Token] = []
        for morpheme in coarse:
            # 分割の先頭だけが C単位の境界と一致する
            for index, part in enumerate(morpheme.split(finer)):
                tokens.append(_to_token(part, unit_boundary=index == 0))
        return tokens


def _to_token(morpheme: Any, *, unit_boundary: bool) -> Token:
    """Sudachi の形態素を `Token` へ変換する。

    読みが空の場合は None にする。「読めない」と確定させるのではなく、
    補完の余地を残すためである（詳細設計 01 §4）。
    """
    reading = morpheme.reading_form()
    pos: Iterable[str] = morpheme.part_of_speech()
    return Token(
        surface=morpheme.surface(),
        reading=reading or None,
        pos=to_part_of_speech(next(iter(pos), "")),
        unit_boundary=unit_boundary,
    )
