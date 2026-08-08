// Package config は api の実行時設定を扱う。
//
// 設定値をコードに埋め込まず、すべて環境変数から読み込む。
// 既定値はローカル開発で動く値とし、本番では Kubernetes の Secret / ConfigMap から上書きする。
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config は api の実行時設定。組み立て後は変更しない。
type Config struct {
	Port int
	// DatabaseURL は接続文字列。パスワードを含むため、ログに出してはならない。
	DatabaseURL     string
	DatabaseTimeout time.Duration
	ProsodyURL      string
	// ProsodyTimeout は基本設計 01 §6 の 1 秒。
	// NFR-01-01 の 150ms に対して十分な余裕があり、これを超えるのは異常である。
	ProsodyTimeout time.Duration
	LogLevel       string
}

// Load は環境変数から設定を組み立てる。必須項目が欠けていれば error を返す。
func Load() (Config, error) {
	port, err := intFromEnv("API_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	dbTimeout, err := durationFromEnv("API_DATABASE_TIMEOUT", 3*time.Second)
	if err != nil {
		return Config{}, err
	}
	prosodyTimeout, err := durationFromEnv("API_PROSODY_TIMEOUT", 1*time.Second)
	if err != nil {
		return Config{}, err
	}

	databaseURL := os.Getenv("API_DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("環境変数 API_DATABASE_URL が設定されていません")
	}

	return Config{
		Port:            port,
		DatabaseURL:     databaseURL,
		DatabaseTimeout: dbTimeout,
		ProsodyURL:      stringFromEnv("API_PROSODY_URL", "http://prosody:8000"),
		ProsodyTimeout:  prosodyTimeout,
		LogLevel:        stringFromEnv("API_LOG_LEVEL", "info"),
	}, nil
}

func stringFromEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intFromEnv(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("環境変数 %s は整数である必要があります: %w", key, err)
	}
	return v, nil
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("環境変数 %s は期間である必要があります（例: 1s, 500ms）: %w", key, err)
	}
	return v, nil
}
