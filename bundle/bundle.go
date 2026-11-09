// Package bundle は、作問者が用意する問題バンドルを読み込み、マニフェストを検証する。
//
// 問題バンドルは、マニフェスト（problem.yaml）と、マニフェストが参照するファイルを含むディレクトリである。
// runner とポータルは、このパッケージを通してだけマニフェストを読む。
package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

// ManifestFile は、問題バンドルのディレクトリ直下に置くマニフェストのファイル名である。
const ManifestFile = "problem.yaml"

// Bundle は、検証済みの問題バンドルである。パスはすべて絶対パスで持つ。
type Bundle struct {
	Name string
	// ImageID は、問題サーバーの AMI ID である。マニフェストで省略されていれば空になる。
	ImageID    string
	Bench      Bench
	ManualPath string
}

// Bench は、ベンチの起動方法である。Command と Adapter のどちらか一方だけが nil でない。
type Bench struct {
	Command *CommandBench
	Adapter *AdapterBench
}

// CommandBench は、作問者のベンチマーカーを共通の形式で実行する設定である。
type CommandBench struct {
	Dir     string
	Command []string
	// Timeout は、マニフェストで省略されていれば 0 になる。
	Timeout time.Duration
}

// AdapterBench は、runner 内の既存の実装でベンチを実行する設定である。
type AdapterBench struct {
	Adapter string
	Options map[string]any
}

type manifest struct {
	Name        string `yaml:"name"`
	Environment struct {
		ImageID string `yaml:"imageId"`
	} `yaml:"environment"`
	Bench struct {
		Kind    string         `yaml:"kind"`
		Dir     string         `yaml:"dir"`
		Command []string       `yaml:"command"`
		Timeout string         `yaml:"timeout"`
		Adapter string         `yaml:"adapter"`
		Options map[string]any `yaml:"options"`
	} `yaml:"bench"`
	Manual string `yaml:"manual"`
}

// Load は、dir にある問題バンドルを読み込んで検証する。
// マニフェストに知らないキーがある場合や、参照するファイルがバンドルの外を指す場合、存在しない場合はエラーを返す。
func Load(dir string) (Bundle, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolve bundle directory: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return Bundle{}, fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := yaml.UnmarshalStrict(data, &m); err != nil {
		return Bundle{}, fmt.Errorf("parse manifest: %w", err)
	}

	b, err := m.validate(dir)
	if err != nil {
		return Bundle{}, fmt.Errorf("invalid manifest %s: %w", filepath.Join(dir, ManifestFile), err)
	}
	return b, nil
}

func (m manifest) validate(dir string) (Bundle, error) {
	if m.Name == "" {
		return Bundle{}, errors.New("name is required")
	}
	bench, err := m.validateBench(dir)
	if err != nil {
		return Bundle{}, fmt.Errorf("bench: %w", err)
	}
	manual, err := resolveFile(dir, m.Manual, false)
	if err != nil {
		return Bundle{}, fmt.Errorf("manual: %w", err)
	}
	return Bundle{
		Name:       m.Name,
		ImageID:    m.Environment.ImageID,
		Bench:      bench,
		ManualPath: manual,
	}, nil
}

func (m manifest) validateBench(dir string) (Bench, error) {
	b := m.Bench
	switch b.Kind {
	case "command":
		if b.Adapter != "" || b.Options != nil {
			return Bench{}, errors.New("adapter and options are only for kind adapter")
		}
		benchDir, err := resolveFile(dir, b.Dir, true)
		if err != nil {
			return Bench{}, fmt.Errorf("dir: %w", err)
		}
		if len(b.Command) == 0 {
			return Bench{}, errors.New("command is required")
		}
		var timeout time.Duration
		if b.Timeout != "" {
			timeout, err = time.ParseDuration(b.Timeout)
			if err != nil || timeout <= 0 {
				return Bench{}, fmt.Errorf("timeout must be a positive duration such as 5m: %q", b.Timeout)
			}
		}
		return Bench{Command: &CommandBench{Dir: benchDir, Command: b.Command, Timeout: timeout}}, nil
	case "adapter":
		if b.Dir != "" || b.Command != nil || b.Timeout != "" {
			return Bench{}, errors.New("dir, command and timeout are only for kind command")
		}
		if b.Adapter == "" {
			return Bench{}, errors.New("adapter is required")
		}
		return Bench{Adapter: &AdapterBench{Adapter: b.Adapter, Options: b.Options}}, nil
	case "":
		return Bench{}, errors.New("kind is required")
	default:
		return Bench{}, fmt.Errorf("unknown kind %q: must be command or adapter", b.Kind)
	}
}

// resolveFile は、バンドル内の相対パス p を絶対パスにし、存在と種類を確かめる。
func resolveFile(dir, p string, wantDir bool) (string, error) {
	if p == "" {
		return "", errors.New("path is required")
	}
	if !filepath.IsLocal(p) {
		return "", fmt.Errorf("%q must be a relative path inside the bundle", p)
	}
	abs := filepath.Join(dir, p)
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", p, err)
	}
	if info.IsDir() != wantDir {
		if wantDir {
			return "", fmt.Errorf("%q is not a directory", p)
		}
		return "", fmt.Errorf("%q is a directory", p)
	}
	return abs, nil
}
