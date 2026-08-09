// migrate はデータベースのスキーマを操作するコマンド。
//
// 基本設計 03 §6 のとおり、api の起動時には自動適用しない。
// 複数の api Pod が同時に起動してマイグレーションが競合するのを防ぐため、
// デプロイ手順の中でこのコマンドを明示的に実行する。
//
//	migrate up            未適用のものをすべて適用する
//	migrate down          直近の1件をロールバックする
//	migrate down -n 3     直近の3件をロールバックする
//	migrate down -all     すべて巻き戻す
//	migrate version       現在のバージョンを表示する
//
// 接続先は api 本体と同じ環境変数 API_DATABASE_URL から読む。
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/yama-shu/575-sns/api/internal/db"
)

func main() {
	// api 本体と同じ構造化ログにする（詳細設計 03 §3）。
	// service を付けないと、3サービスのログが混ざったときに出所が分からなくなる。
	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	).With("service", "migrate"))

	if err := run(); err != nil {
		slog.Error("マイグレーションに失敗しました", "error", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("API_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("環境変数 API_DATABASE_URL が設定されていません")
	}

	if len(os.Args) < 2 {
		return errors.New("サブコマンドを指定してください（up / down / version）")
	}
	// サブコマンドより後ろだけをフラグとして解析する。
	// flag.Parse() をそのまま使うと、最初の非フラグ引数（サブコマンド）で
	// 解析が止まり、`migrate down -all` の -all が読まれない。
	subcommand, args := os.Args[1], os.Args[2:]

	switch subcommand {
	case "up":
		return up(databaseURL)

	case "down":
		fs := flag.NewFlagSet("down", flag.ContinueOnError)
		steps := fs.Int("n", 1, "ロールバックする件数")
		all := fs.Bool("all", false, "すべて巻き戻す")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if *all {
			return down(databaseURL, 0)
		}
		if *steps < 1 {
			return fmt.Errorf("-n は1以上である必要があります: %d", *steps)
		}
		return down(databaseURL, *steps)

	case "version":
		return version(databaseURL)

	default:
		return fmt.Errorf("不明なサブコマンドです: %s（up / down / version のいずれか）", subcommand)
	}
}

func up(databaseURL string) error {
	err := db.Up(databaseURL)
	if errors.Is(err, db.ErrNoChange) {
		slog.Info("適用すべきマイグレーションはありませんでした")
		return nil
	}
	if err != nil {
		return err
	}
	return reportVersion(databaseURL, "マイグレーションを適用しました")
}

func down(databaseURL string, n int) error {
	err := db.Down(databaseURL, n)
	if errors.Is(err, db.ErrNoChange) {
		slog.Info("巻き戻すマイグレーションはありませんでした")
		return nil
	}
	if err != nil {
		return err
	}
	return reportVersion(databaseURL, "マイグレーションを巻き戻しました")
}

func version(databaseURL string) error {
	return reportVersion(databaseURL, "現在のバージョン")
}

// reportVersion は操作後のバージョンを記録する。
//
// dirty は前回の適用が途中で失敗した状態を指す。放置すると以降の適用が
// 拒否されるため、成功扱いにせず終了コードで異常を伝える。
func reportVersion(databaseURL, message string) error {
	v, dirty, err := db.Version(databaseURL)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf(
			"バージョン %d が dirty です。前回の適用が中断されています。"+
				"スキーマを確認し、schema_migrations テーブルを手動で修正してください", v)
	}
	slog.Info(message, "version", v)
	return nil
}
