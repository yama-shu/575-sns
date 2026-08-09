// Package db はデータベースのスキーマ管理を担う。
//
// マイグレーションの SQL ファイルはバイナリに埋め込む（embed）。
// 実行時にファイルを配置する必要がなくなり、distroless の runtime イメージや
// Kubernetes の Job からもそのまま実行できる。
//
// 基本設計 03 §6 のとおり、api の起動時には適用しない。
// 複数の api Pod が同時に起動してマイグレーションが競合するのを防ぐため、
// 明示的なコマンド（cmd/migrate）でのみ実行する。
package db

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx5:// スキームを登録する
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// ErrNoChange は適用すべきマイグレーションが無かったことを示す。
// 何度実行しても同じ結果になるよう、これは異常として扱わない。
var ErrNoChange = migrate.ErrNoChange

// Up は未適用のマイグレーションをすべて適用する。
// 適用すべきものが無い場合は ErrNoChange を返す。
func Up(databaseURL string) error {
	m, err := open(databaseURL)
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return ErrNoChange
		}
		return fmt.Errorf("マイグレーションの適用に失敗しました: %w", err)
	}
	return nil
}

// Down は直近の n 件をロールバックする。n が 0 以下ならすべてを巻き戻す。
//
// すべてのマイグレーションに down を用意している（基本設計 03 §6）ため、
// どの時点へも戻せる。
func Down(databaseURL string, n int) error {
	m, err := open(databaseURL)
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	if n <= 0 {
		err = m.Down()
	} else {
		err = m.Steps(-n)
	}
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return ErrNoChange
		}
		return fmt.Errorf("マイグレーションのロールバックに失敗しました: %w", err)
	}
	return nil
}

// Version は現在のバージョンと、前回の適用が中断された状態かどうかを返す。
//
// dirty が true のとき、マイグレーションは途中で失敗している。
// この状態では以降の適用が拒否されるため、手動での確認と復旧が必要になる。
func Version(databaseURL string) (version uint, dirty bool, err error) {
	m, err := open(databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer closeMigrate(m)

	version, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		// 一度も適用されていない状態。異常ではない。
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("バージョンを取得できません: %w", err)
	}
	return version, dirty, nil
}

// open は埋め込んだ SQL と接続先から migrate インスタンスを組み立てる。
func open(databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("マイグレーションファイルを読み込めません: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, toPgxURL(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("マイグレーションを初期化できません: %w", err)
	}
	return m, nil
}

// toPgxURL は接続文字列のスキームを pgx5:// に差し替える。
//
// api 本体は pgxpool へ postgres:// で渡すため、設定は1つの環境変数
// （API_DATABASE_URL）で共有したい。golang-migrate は登録されたドライバを
// スキームで選ぶため、ここで吸収する。
func toPgxURL(databaseURL string) string {
	for _, scheme := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(databaseURL, scheme) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, scheme)
		}
	}
	return databaseURL
}

// closeMigrate は source と database の両方を閉じる。
//
// 閉じる際のエラーは処理の成否を左右しないが、握りつぶすと
// 接続リークの兆候を見逃すため、警告として記録する。
func closeMigrate(m *migrate.Migrate) {
	sourceErr, dbErr := m.Close()
	if sourceErr != nil {
		slog.Warn("マイグレーションのソースを閉じられませんでした", "error", sourceErr)
	}
	if dbErr != nil {
		slog.Warn("マイグレーションのデータベース接続を閉じられませんでした", "error", dbErr)
	}
}
