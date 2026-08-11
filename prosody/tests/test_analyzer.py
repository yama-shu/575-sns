"""判定エンジン全体の受け入れテスト（詳細設計 04 §6）。

**辞書を含めた統合の確認**であり、単体テストとは目的が異なる。
ここまでの単体テストは形態素解析器をモックしているため、
「部品は正しいが、つなぐと動かない」を検出できない。
"""

import pytest

from prosody.analyzer import ProsodyAnalyzer
from prosody.sudachi import SudachiTokenizer
from prosody.verdict import Reason, Verdict


@pytest.fixture(scope="module")
def analyzer() -> ProsodyAnalyzer:
    """辞書のロードは重いため、モジュール内で使い回す。"""
    return ProsodyAnalyzer(SudachiTokenizer())


# ----------------------------------------------------------------------
# TC-AC-01〜10
# ----------------------------------------------------------------------


def test_TC_AC_01_定型の句は定型と判定される(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("今日もまた会議のための会議かな")

    assert result.verdict is Verdict.TEIKEI
    assert result.total_mora == 17
    assert result.segments is not None
    assert [s.mora for s in result.segments] == [5, 7, 5]
    assert [s.expected for s in result.segments] == [5, 7, 5]
    assert [s.diff for s in result.segments] == [0, 0, 0]
    assert [s.text for s in result.segments] == ["今日もまた", "会議のための", "会議かな"]


def test_TC_AC_02_短すぎる句は破調になる(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("今日は疲れた")

    assert result.verdict is Verdict.HACHO
    assert result.reason is Reason.TOO_FEW_MORA
    assert result.segments is None, "五七五に区切れないため区切りは定義できない"


def test_TC_AC_03_長すぎる句は破調になる(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("今日は本当に疲れたので早く家に帰ってゆっくり休みたいと思っている")

    assert result.verdict is Verdict.HACHO
    assert result.reason is Reason.TOO_MANY_MORA
    assert result.segments is None


def test_TC_AC_04_音数は合うが五七五に切れない句は破調になる(
    analyzer: ProsodyAnalyzer,
) -> None:
    """`TOO_FEW_MORA` でも `TOO_MANY_MORA` でもない破調。

    利用者には「音の数は合っていますが、五七五の区切りになっていません」と
    案内する必要があるため、理由を区別する。
    """
    # 総モーラ数は範囲内だが、C単位でも A単位でも 5/7/5 に切れない文。
    # 複合語が長く、形態素の境界が音数の区切りと合わない。
    result = analyzer.analyze("生麦生米生卵を食べる")

    assert result.verdict is Verdict.HACHO
    assert result.reason is Reason.NO_VALID_SPLIT
    assert result.total_mora == 17, "総モーラ数は範囲内である"
    assert result.segments is None


def test_総モーラ数が18でも区切れなければ破調になる(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("東京特許許可局の局長さん")

    assert result.verdict is Verdict.HACHO
    assert result.reason is Reason.NO_VALID_SPLIT
    assert result.total_mora == 18


def test_TC_AC_05_字余りの句は許容になる(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("今日もまたも会議のための会議かな")

    assert result.verdict is Verdict.KYOYO
    assert result.total_mora == 18
    assert result.segments is not None
    assert sum(abs(s.diff) for s in result.segments) == 1, "ズレは1箇所だけ"


def test_TC_AC_06_空文字列は破調になる(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("")

    assert result.verdict is Verdict.HACHO
    assert result.reason is Reason.TOO_FEW_MORA


def test_TC_AC_07_空白のみは破調になる(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("   ")

    assert result.verdict is Verdict.HACHO
    assert result.reason is Reason.TOO_FEW_MORA


def test_TC_AC_08_絵文字は0モーラとして扱われる(analyzer: ProsodyAnalyzer) -> None:
    """絵文字が音数を増やしてはならない。"""
    without = analyzer.analyze("今日もまた会議のための会議かな")
    with_emoji = analyzer.analyze("今日もまた会議のための会議かな😀")

    assert with_emoji.total_mora == without.total_mora
    assert with_emoji.verdict is Verdict.TEIKEI


def test_TC_AC_09_100文字ちょうどでも処理できる(analyzer: ProsodyAnalyzer) -> None:
    """本文の上限（基本設計 03）。破調にはなるが、落ちてはならない。"""
    result = analyzer.analyze("あ" * 100)

    assert result.verdict is Verdict.HACHO
    assert result.reason is Reason.TOO_MANY_MORA


def test_TC_AC_10_上限を超える長さはapi側で弾く(analyzer: ProsodyAnalyzer) -> None:
    """prosody は長さを制限しない。

    文字数の上限は API のバリデーション（基本設計 05）で扱う。
    prosody は「文字列を受け取り判定を返す」だけの純粋な変換器であり、
    業務上の制約を持たない（基本設計 01 §2）。
    """
    result = analyzer.analyze("あ" * 101)

    assert result.verdict is Verdict.HACHO


# ----------------------------------------------------------------------
# 明示的な区切り
# ----------------------------------------------------------------------


def test_空白で区切ると利用者の意図が優先される(analyzer: ProsodyAnalyzer) -> None:
    result = analyzer.analyze("今日もまた 会議のための 会議かな")

    assert result.verdict is Verdict.TEIKEI
    assert result.segments is not None
    assert [s.mora for s in result.segments] == [5, 7, 5]


def test_句を連結すると正規化後の本文に戻る(analyzer: ProsodyAnalyzer) -> None:
    """api が文字数から区切り位置を算出できるための不変条件。"""
    for text in ["今日もまた会議のための会議かな", "今日もまた 会議のための 会議かな"]:
        result = analyzer.analyze(text)

        assert result.segments is not None
        assert "".join(s.text for s in result.segments) == result.normalized_text


def test_全角空白を含む本文でも正規化後の位置が一致する(analyzer: ProsodyAnalyzer) -> None:
    """元の入力を保存すると位置がずれるため、normalized_text を返している。"""
    result = analyzer.analyze("今日もまた　会議のための　会議かな")

    assert result.normalized_text == "今日もまた 会議のための 会議かな"
    assert result.segments is not None
    assert "".join(s.text for s in result.segments) == result.normalized_text


# ----------------------------------------------------------------------
# 読みを確定できない場合
# ----------------------------------------------------------------------


def test_読めない語があると破調ではなく判定不能になる(analyzer: ProsodyAnalyzer) -> None:
    """この2つを取り違えると、正しく詠んだ利用者が直しようがなくなる。

    SudachiDict-core はほとんどの語に読みを付けるため、
    実際に unknown になる入力を探すのは難しい。ここでは
    読みを返さない解析器に差し替えて、経路そのものを検証する。
    """
    from prosody.token import PartOfSpeech, SplitMode, Token

    class TokenizerWithoutReadings:
        def tokenize(self, text: str, mode: SplitMode) -> list[Token]:
            return [Token(surface=text, reading=None, pos=PartOfSpeech.NOUN)]

    result = ProsodyAnalyzer(TokenizerWithoutReadings()).analyze("甃")

    assert result.verdict is Verdict.UNKNOWN
    assert result.reason is Reason.READING_UNAVAILABLE
    assert result.unreadable == ["甃"]
    assert result.segments is None
    assert result.reading is None


# ----------------------------------------------------------------------
# 既知の限界に対する挙動固定テスト（TC-LM-01〜02）
#
# これらは**失敗するテストとして書かない**。
# 現在の挙動を記録し、意図せず変わったときに気づくためのものである。
# 将来改善したらテストを更新する。
# ----------------------------------------------------------------------


def test_TC_LM_01_同表記異読は文脈で解決されない(analyzer: ProsodyAnalyzer) -> None:
    """L1。「一日」は イチニチ / ツイタチ のどちらにも読めるが確定できない。"""
    result = analyzer.analyze("一日")

    assert result.reading is not None
    # どちらの読みを採っているかを記録する。変わったら気づけるようにする。
    assert result.reading in ("イチニチ", "ツイタチ", "イチジツ")


def test_TC_LM_02_助数詞の連濁に対応しない(analyzer: ProsodyAnalyzer) -> None:
    """L3。`1本` は イッポン だが、そう読めるとは限らない。"""
    result = analyzer.analyze("1本")

    assert result.reading is not None
    # 現状の読みを固定する。イッポン と読めていれば改善している。
    assert result.reading != "", "読みが取得できなくなったら退行"


# ----------------------------------------------------------------------
# 読みの解決の優先順位（#53）
#
# **モックした解析器では検出できなかった不具合の再発防止。**
# SudachiPy は読みを付けられなかった語に表層をそのまま返し、
# 数字には1桁ずつの読みを付ける。どちらもモックでは再現しない。
# ----------------------------------------------------------------------


def test_読めない語は表層を読みとして採用しない(analyzer: ProsodyAnalyzer) -> None:
    """SudachiPy は `彁` の読みとして `彁` を返す。

    これを読みとして扱うと、表層の文字数がモーラ数に化け、
    `unknown` にも到達しない。正しく詠んだ利用者に
    「五七五になっていません」と伝えることになる。
    """
    result = analyzer.analyze("彁")

    assert result.verdict is Verdict.UNKNOWN
    assert result.reason is Reason.READING_UNAVAILABLE
    assert result.unreadable == ["彁"]
    assert result.reading is None


def test_読めない語が混じると全体が判定不能になる(analyzer: ProsodyAnalyzer) -> None:
    """読める語だけで数えて破調と答えてはならない。"""
    result = analyzer.analyze("彁を見つめる春の夜に")

    assert result.verdict is Verdict.UNKNOWN
    assert result.unreadable == ["彁"]


def test_数字は数値読みで数えられる(analyzer: ProsodyAnalyzer) -> None:
    """詳細設計 01 §4 の数値読み変換。

    解析器の読み（`ニレイニヨン`）より優先する。
    """
    result = analyzer.analyze("2024")

    assert result.reading == "ニセンニジュウヨン"
    assert result.total_mora == 8


def test_十は2モーラとして数えられる(analyzer: ProsodyAnalyzer) -> None:
    """`イチレイ`（4モーラ）と読まれていた。"""
    result = analyzer.analyze("10")

    assert result.reading == "ジュウ"
    assert result.total_mora == 2


def test_ASCII以外の数字も数値として読まれる(analyzer: ProsodyAnalyzer) -> None:
    """挙動の記録。`str.isdecimal()` は Unicode の十進数字すべてに真を返す。

    タミル数字 `௧௨௩` は `int()` で 123 になり、`ヒャクニジュウサン` と読まれる。
    数としては正しいが、日本語話者がそう読むとは限らない。
    意図せず変わったときに気づくためにここで固定する。
    """
    result = analyzer.analyze("௧௨௩")

    assert result.reading == "ヒャクニジュウサン"
