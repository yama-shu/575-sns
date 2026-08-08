"""正規化のテスト（詳細設計 01 §2）。"""

import pytest

from prosody.normalize import normalize

CASES = [
    # NFKC: 全角英数 → 半角
    pytest.param("ＡＢＣ１２３", "ABC123", id="全角英数を半角にする"),
    # NFKC: 半角カナ → 全角
    pytest.param("ｶﾀｶﾅ", "カタカナ", id="半角カナを全角にする"),
    # 改行の統一と空白への圧縮
    pytest.param("あ\r\nい", "あ い", id="CRLF を空白にする"),
    pytest.param("あ\rい", "あ い", id="CR を空白にする"),
    pytest.param("あ\nい", "あ い", id="LF を空白にする"),
    # 連続する空白の圧縮
    pytest.param("あ   い", "あ い", id="連続する半角空白を1つにする"),
    pytest.param("あ　　い", "あ い", id="全角空白も1つの半角空白にする"),
    pytest.param("あ \n\t い", "あ い", id="種類の違う空白が続いても1つにする"),
    # 前後の空白の除去
    pytest.param("  あい  ", "あい", id="前後の空白を落とす"),
    pytest.param("　あい　", "あい", id="前後の全角空白を落とす"),
    # 変わらないもの
    pytest.param("", "", id="空文字列"),
    pytest.param(
        "今日もまた 会議のための 会議かな",
        "今日もまた 会議のための 会議かな",
        id="明示的な区切りは保つ",
    ),
]


@pytest.mark.parametrize(("text", "expected"), CASES)
def test_正規化(text: str, expected: str) -> None:
    assert normalize(text) == expected


def test_ひらがなをカタカナに変換しない() -> None:
    """本文の表記はそのまま保つ。

    書き換えると、投稿として表示するときに入力と異なるものが出てしまう。
    カタカナ化は読みに対してのみ行う（詳細設計 01 §2）。
    """
    assert normalize("ふるいけや") == "ふるいけや"


def test_明示的な区切りの空白は数えられる形で残る() -> None:
    """詳細設計 01 §7 は「空白がちょうど2つ」で明示的な区切りと判定する。

    連続する空白を圧縮しても、区切りとしての空白の個数は変わらない。
    """
    assert normalize("今日もまた   会議のための   会議かな").count(" ") == 2
