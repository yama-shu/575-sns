"""SudachiTokenizer のテスト。

**このファイルだけは辞書のロードを伴う。** 他のテストは Tokenizer の
モックを使うため辞書を必要としない（詳細設計 02）。

ここで確かめるのは「Sudachi の出力を Token へ正しく変換できているか」であり、
Sudachi そのものの解析精度ではない。
"""

import pytest

from prosody.sudachi import SudachiTokenizer, to_part_of_speech
from prosody.token import PartOfSpeech, SplitMode


@pytest.fixture(scope="module")
def tokenizer() -> SudachiTokenizer:
    """辞書のロードは重いため、モジュール内で使い回す。"""
    return SudachiTokenizer()


def test_C単位では複合語がまとまる(tokenizer: SudachiTokenizer) -> None:
    """ADR-0003 が C単位を基本に選んだ理由そのもの。

    複合語がまとまっていれば、単語の途中で切る事故が起きにくい。
    """
    tokens = tokenizer.tokenize("選挙管理委員会", SplitMode.C)

    assert [t.surface for t in tokens] == ["選挙管理委員会"]
    assert all(t.unit_boundary for t in tokens)


def test_A単位では複合語が分割される(tokenizer: SudachiTokenizer) -> None:
    """C単位で区切りが見つからないときに候補を増やすための粒度。"""
    tokens = tokenizer.tokenize("選挙管理委員会", SplitMode.A)

    assert len(tokens) > 1
    assert "".join(t.surface for t in tokens) == "選挙管理委員会"


def test_A単位では先頭だけがC単位の境界になる(tokenizer: SudachiTokenizer) -> None:
    """A単位探索でのペナルティ（詳細設計 01 §9）が使う情報。

    C単位を分断する位置かどうかが、この値で判断できる必要がある。
    """
    tokens = tokenizer.tokenize("選挙管理委員会", SplitMode.A)

    assert tokens[0].unit_boundary is True
    assert any(t.unit_boundary is False for t in tokens[1:])


def test_B単位も扱える(tokenizer: SudachiTokenizer) -> None:
    tokens = tokenizer.tokenize("選挙管理委員会", SplitMode.B)

    assert "".join(t.surface for t in tokens) == "選挙管理委員会"


def test_読みが取得できる(tokenizer: SudachiTokenizer) -> None:
    tokens = tokenizer.tokenize("東京", SplitMode.C)

    assert tokens[0].reading == "トウキョウ"


def test_品詞が正規化される(tokenizer: SudachiTokenizer) -> None:
    tokens = tokenizer.tokenize("古池や", SplitMode.C)

    assert tokens[-1].pos is PartOfSpeech.PARTICLE


def test_空文字列では形態素が返らない(tokenizer: SudachiTokenizer) -> None:
    assert tokenizer.tokenize("", SplitMode.C) == []


def test_数字にも読みが付く(tokenizer: SudachiTokenizer) -> None:
    """SudachiDict-core は数字に1桁ずつの読みを付ける。

    `2024` は `ニレイニヨン`（5モーラ）になり、
    詳細設計 01 §4 の数値読み変換（`ニセンニジュウヨン` / 8モーラ）と食い違う。

    **この読みは採用しない。** `ReadingResolver` が数字を解析器より先に扱い、
    数値読み変換を優先する（#53）。ここで固定しているのは
    「そのままでは使えない読みが返る」という前提そのものである。
    """
    tokens = tokenizer.tokenize("2024", SplitMode.C)

    assert [t.reading for t in tokens] == ["ニレイニヨン"]


def test_読みが空の形態素はNoneになる() -> None:
    """解析器が読みを返さなかった場合の変換。

    SudachiDict-core は記号にも読みを付けるため、実際の解析では再現しにくい。
    変換そのものを確かめるため、形態素を模したオブジェクトを直接渡す。
    """
    from prosody.sudachi import _to_token

    class MorphemeWithoutReading:
        def surface(self) -> str:
            return "甃"

        def reading_form(self) -> str:
            return ""

        def part_of_speech(self) -> tuple[str, ...]:
            return ("名詞", "普通名詞", "一般")

    token = _to_token(MorphemeWithoutReading(), unit_boundary=True)

    assert token.reading is None
    assert token.surface == "甃"
    assert token.pos is PartOfSpeech.NOUN


POS_CASES = [
    ("名詞", PartOfSpeech.NOUN),
    ("動詞", PartOfSpeech.VERB),
    ("形容詞", PartOfSpeech.ADJECTIVE),
    ("副詞", PartOfSpeech.ADVERB),
    ("連体詞", PartOfSpeech.ADNOMINAL),
    ("接続詞", PartOfSpeech.CONJUNCTION),
    ("感動詞", PartOfSpeech.INTERJECTION),
    ("助詞", PartOfSpeech.PARTICLE),
    ("助動詞", PartOfSpeech.AUXILIARY_VERB),
    ("接頭辞", PartOfSpeech.PREFIX),
    ("接尾辞", PartOfSpeech.SUFFIX),
    ("補助記号", PartOfSpeech.PUNCTUATION),
    ("記号", PartOfSpeech.PUNCTUATION),
    ("空白", PartOfSpeech.PUNCTUATION),
    ("知らない品詞", PartOfSpeech.OTHER),
]


@pytest.mark.parametrize(("sudachi_pos", "expected"), POS_CASES)
def test_品詞の対応表(sudachi_pos: str, expected: PartOfSpeech) -> None:
    assert to_part_of_speech(sudachi_pos) is expected


def test_Tokenizer_インターフェースを満たす(tokenizer: SudachiTokenizer) -> None:
    from prosody.tokenizer import Tokenizer

    assert isinstance(tokenizer, Tokenizer)


def test_記号や空白の読みを信用しない(tokenizer: SudachiTokenizer) -> None:
    """SudachiDict-core は記号や空白にも読みを付ける。

    実測では次のような読みが返る。

        " " → "キゴウ"   ※ → "キゴウ"   ♪ → "キゴウ"

    読みをそのまま使うと空白1つが3モーラになり、明示的な区切りに
    空白を使う仕様（詳細設計 01 §7）が成立しなくなる。
    ReadingResolver が記号・空白を読みより先に判定することで防いでいる。
    """
    from prosody.reading import ReadingResolver

    resolved = ReadingResolver().resolve(tokenizer.tokenize(" ", SplitMode.C))

    assert resolved.total_mora == 0


def test_空白を挟んだ本文のモーラ数が空白の有無で変わらない(
    tokenizer: SudachiTokenizer,
) -> None:
    """利用者が区切りとして空白を入れても、音数は変わってはならない。"""
    from prosody.reading import ReadingResolver

    resolver = ReadingResolver()
    without = resolver.resolve(tokenizer.tokenize("今日もまた会議のための会議かな", SplitMode.C))
    with_spaces = resolver.resolve(
        tokenizer.tokenize("今日もまた 会議のための 会議かな", SplitMode.C)
    )

    assert with_spaces.total_mora == without.total_mora


def test_A単位でも本文全体の形態素がそろう(tokenizer: SudachiTokenizer) -> None:
    """Morpheme.split() を使った実装では、ここが壊れていた。

    SudachiPy の Morpheme は MorphemeList への参照であり、
    split() は親のリストを壊す。反復しながら split() を呼ぶと
    最後に分割した形態素の結果しか残らない。

        C単位: 古池 / や / 蛙 / 飛び込む / 水 / の / 音
        split を使った A単位: 飛び / 込む      ← 全体が消えていた

    表層を連結して本文に戻ることで、取りこぼしを検出する。
    """
    text = "古池や蛙飛び込む水の音"

    for mode in (SplitMode.A, SplitMode.B, SplitMode.C):
        tokens = tokenizer.tokenize(text, mode)
        assert "".join(t.surface for t in tokens) == text, f"{mode} で本文に戻らない"


def test_A単位はC単位以上に細かく分割される(tokenizer: SudachiTokenizer) -> None:
    text = "選挙管理委員会で古池や蛙飛び込む"

    coarse = tokenizer.tokenize(text, SplitMode.C)
    fine = tokenizer.tokenize(text, SplitMode.A)

    assert len(fine) > len(coarse)


def test_単位境界はC単位の開始位置と一致する(tokenizer: SudachiTokenizer) -> None:
    """A単位探索でのペナルティ（詳細設計 01 §9）が使う情報の正しさ。"""
    text = "選挙管理委員会で古池や"

    coarse = tokenizer.tokenize(text, SplitMode.C)
    fine = tokenizer.tokenize(text, SplitMode.A)

    # C単位の各形態素の開始位置
    offset = 0
    coarse_starts = []
    for t in coarse:
        coarse_starts.append(offset)
        offset += len(t.surface)

    offset = 0
    for t in fine:
        assert t.unit_boundary == (offset in coarse_starts), f"{t.surface} の境界判定が誤り"
        offset += len(t.surface)


def test_古池やはC単位のまま定型に区切れる(tokenizer: SudachiTokenizer) -> None:
    """詳細設計 01 §8 の適用例。

    設計書は当初「5/7/5 に区切れる形態素境界が存在しないため
    C単位では NO_VALID_SPLIT になる」と記述していたが、実測では区切れる。
    古池 = フルイケ = 4モーラ、飛び込む = トビコム = 4モーラであり、
    設計書が挙げていた 3 と 5 は誤りだった。

    ここは実装の正しさではなく**設計書の記述と実測の一致**を固定する。
    """
    from prosody.reading import ReadingResolver
    from prosody.segment import SegmentSearcher
    from prosody.verdict import Verdict

    resolved = ReadingResolver().resolve(tokenizer.tokenize("古池や蛙飛び込む水の音", SplitMode.C))

    assert [t.mora for t in resolved.tokens] == [4, 1, 3, 4, 2, 1, 2]
    assert resolved.total_mora == 17

    best = SegmentSearcher().best_for(resolved.tokens)

    assert best is not None
    assert (best.kami, best.naka, best.shimo) == (5, 7, 5)
    assert best.verdict is Verdict.TEIKEI
    # 古池や / 蛙飛び込む / 水の音
    assert (best.kami_end, best.naka_end) == (2, 4)
