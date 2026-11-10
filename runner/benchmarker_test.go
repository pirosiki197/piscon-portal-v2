package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/runner"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker/impl/command"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker/impl/isucon13"
	"github.com/traPtitech/piscon-portal-v2/runner/config"
)

func writeBundle(t *testing.T, bench string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "bench"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manual.md"), nil, 0o644))
	template := "Resources:\n  Webapp:\n    Type: AWS::EC2::Instance\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(template), 0o644))
	manifest := "name: sample\nmanual: manual.md\nenvironment:\n  template: template.yaml\nbench:\n" + bench
	require.NoError(t, os.WriteFile(filepath.Join(dir, "problem.yaml"), []byte(manifest), 0o644))
	return dir
}

func TestNewBenchmarkerFromBundle(t *testing.T) {
	t.Run("command", func(t *testing.T) {
		dir := writeBundle(t, "  kind: command\n  dir: bench\n  command: [\"./bench\", \"{{ .TargetIP }}\"]\n")

		b, err := runner.NewBenchmarker(config.Config{Bundle: dir})

		require.NoError(t, err)
		assert.IsType(t, &command.Command{}, b)
	})

	t.Run("adapter", func(t *testing.T) {
		dir := writeBundle(t, "  kind: adapter\n  adapter: isucon13\n  options:\n    benchmarker-path: /home/isucon/bench\n    benchmarker-dir: /home/isucon\n    dns-port: 53\n")

		b, err := runner.NewBenchmarker(config.Config{Bundle: dir})

		require.NoError(t, err)
		assert.IsType(t, &isucon13.Isucon13{}, b)
	})

	t.Run("unknown adapter", func(t *testing.T) {
		dir := writeBundle(t, "  kind: adapter\n  adapter: isucon99\n")

		_, err := runner.NewBenchmarker(config.Config{Bundle: dir})

		assert.ErrorContains(t, err, "unknown problem")
	})

	t.Run("invalid command template", func(t *testing.T) {
		dir := writeBundle(t, "  kind: command\n  dir: bench\n  command: [\"./bench\", \"{{ .Target }}\"]\n")

		_, err := runner.NewBenchmarker(config.Config{Bundle: dir})

		assert.Error(t, err)
	})
}
