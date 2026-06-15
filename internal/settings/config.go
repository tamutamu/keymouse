// Package settings はアプリケーションの設定を扱い、config.json への永続化を行う。
// ホットキーやグリッド構成、表示・動作に関する各種オプションの読み書きと、
// デフォルト値の提供を担う。
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tamutamu/keymouse/internal/input"
	"github.com/tamutamu/keymouse/internal/spatial"
)

const appName = "keymouse"

// Config は config.json に永続化されるアプリケーション設定である。
// ホットキー種別とラベルサイズは、実行時の型(input.HotkeyConfig / spatial.LabelSize)を
// そのまま用いることで、永続化用の重複した型定義と変換処理を避けている。
type Config struct {
	// ホットキー
	HotkeyLeft   input.HotkeyConfig `json:"hotkey_left"`
	HotkeyRight  input.HotkeyConfig `json:"hotkey_right"`
	HotkeyDouble input.HotkeyConfig `json:"hotkey_double"`

	// グリッド(希望する最大グリッドと、ラベルが読める最小サイズ)
	Cols       int     `json:"cols"`
	Rows       int     `json:"rows"`
	MinLabelPx float64 `json:"min_label_px"`
	MaxDepth   int     `json:"max_depth"` // 段数の安全上限

	// 表示
	LabelSize spatial.LabelSize `json:"label_size"`

	// 動作
	AutoStart        bool `json:"auto_start"`
	ShowTutorialOnce bool `json:"show_tutorial_once"`
}

// Default はデフォルト値の Config を返す。
func Default() Config {
	hk := input.DefaultHotkeys()
	return Config{
		HotkeyLeft:   hk[spatial.ClickLeft],
		HotkeyRight:  hk[spatial.ClickRight],
		HotkeyDouble: hk[spatial.ClickDouble],

		Cols:       5,
		Rows:       5,
		MinLabelPx: 20,
		MaxDepth:   8,

		LabelSize: spatial.LabelNormal,

		AutoStart:        false,
		ShowTutorialOnce: true,
	}
}

// configPath は %APPDATA%\keymouse\config.json のパスを返す。
func configPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA env var not set")
	}
	return filepath.Join(appData, appName, "config.json"), nil
}

// Load はディスクから設定を読み込む。ファイルが無ければデフォルト値を返す。
// 破損していれば config.json.bak に退避してデフォルト値を返す。
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Default(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Default(), fmt.Errorf("settings.Load: read error: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// 破損ファイルを退避してデフォルト値を返す。
		bakPath := path + ".bak"
		if renameErr := os.Rename(path, bakPath); renameErr != nil {
			log.Printf("settings: could not back up corrupt config: %v", renameErr)
		} else {
			log.Printf("settings: corrupt config backed up to %s", bakPath)
		}
		return Default(), nil
	}

	return cfg, nil
}

// Save は設定をディスクに書き込む。必要ならディレクトリを作成する。
func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("settings.Save: mkdir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("settings.Save: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("settings.Save: write: %w", err)
	}
	return nil
}
