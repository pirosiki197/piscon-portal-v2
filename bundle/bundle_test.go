package bundle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBundleDir は、bench ディレクトリと manual.md を含むバンドルを作り、manifest を書き込む。
func newBundleDir(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "bench"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manual.md"), []byte("# manual"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o644))
	return dir
}

func TestLoadCommand(t *testing.T) {
	dir := newBundleDir(t, `
name: sample
environment:
  imageId: ami-0123
bench:
  kind: command
  dir: bench
  command: ["./bench", "--target", "{{ .TargetIP }}"]
  timeout: 2m
manual: manual.md
`)

	b, err := Load(dir)
	require.NoError(t, err)

	assert.Equal(t, Bundle{
		Name:    "sample",
		ImageID: "ami-0123",
		Bench: Bench{Command: &CommandBench{
			Dir:     filepath.Join(dir, "bench"),
			Command: []string{"./bench", "--target", "{{ .TargetIP }}"},
			Timeout: 2 * time.Minute,
		}},
		ManualPath: filepath.Join(dir, "manual.md"),
	}, b)
}

func TestLoadAdapter(t *testing.T) {
	dir := newBundleDir(t, `
name: isucon13
bench:
  kind: adapter
  adapter: isucon13
  options:
    benchmarker-path: /home/isucon/bench
    dns-port: 53
    enable-ssl: true
manual: manual.md
`)

	b, err := Load(dir)
	require.NoError(t, err)

	assert.Nil(t, b.Bench.Command)
	assert.Equal(t, &AdapterBench{
		Adapter: "isucon13",
		Options: map[string]any{"benchmarker-path": "/home/isucon/bench", "dns-port": 53, "enable-ssl": true},
	}, b.Bench.Adapter)
}

func TestLoadRejectsInvalidManifest(t *testing.T) {
	const commandBench = `
bench:
  kind: command
  dir: bench
  command: ["./bench"]
`
	tests := map[string]string{
		"missing name":            commandBench + "manual: manual.md\n",
		"unknown key":             "name: x\nmanual: manual.md\nunknown: 1\n" + commandBench,
		"missing kind":            "name: x\nmanual: manual.md\nbench:\n  dir: bench\n",
		"unknown kind":            "name: x\nmanual: manual.md\nbench:\n  kind: shell\n",
		"missing command":         "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: bench\n",
		"bench dir outside":       "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: ../bench\n  command: [a]\n",
		"bench dir not found":     "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: nope\n  command: [a]\n",
		"bench dir is a file":     "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: manual.md\n  command: [a]\n",
		"invalid timeout":         "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: bench\n  command: [a]\n  timeout: soon\n",
		"command with adapter":    "name: x\nmanual: manual.md\nbench:\n  kind: command\n  dir: bench\n  command: [a]\n  adapter: isucon13\n",
		"adapter without name":    "name: x\nmanual: manual.md\nbench:\n  kind: adapter\n",
		"adapter with command":    "name: x\nmanual: manual.md\nbench:\n  kind: adapter\n  adapter: isucon13\n  command: [a]\n",
		"missing manual":          "name: x\n" + commandBench,
		"manual not found":        "name: x\nmanual: nope.md\n" + commandBench,
		"manual outside (abs)":    "name: x\nmanual: /etc/passwd\n" + commandBench,
		"manual is the bench dir": "name: x\nmanual: bench\n" + commandBench,
	}

	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(newBundleDir(t, manifest))
			assert.Error(t, err)
		})
	}
}

func TestLoadMissingManifest(t *testing.T) {
	_, err := Load(t.TempDir())
	assert.ErrorIs(t, err, os.ErrNotExist)
}
