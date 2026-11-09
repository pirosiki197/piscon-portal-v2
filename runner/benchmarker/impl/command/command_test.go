package command

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/bundle"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker/internal/reportfd"
	"github.com/traPtitech/piscon-portal-v2/runner/domain"
)

type runResult struct {
	stdout string
	result domain.Result
	score  int
	err    error
}

func run(t *testing.T, b *Command, job *domain.Job) runResult {
	t.Helper()
	ctx := context.Background()

	out, _, err := b.Start(ctx, job)
	require.NoError(t, err)
	stdout, err := io.ReadAll(out.Stdout)
	require.NoError(t, err)
	_, err = io.ReadAll(out.Stderr)
	require.NoError(t, err)

	result, _, err := b.Wait(ctx)
	score, scoreErr := b.CalculateScore(ctx, "", "")
	require.NoError(t, scoreErr)
	return runResult{stdout: string(stdout), result: result, score: score, err: err}
}

func newShell(t *testing.T, script string, timeout time.Duration) *Command {
	t.Helper()
	b, err := New(bundle.CommandBench{
		Dir:     t.TempDir(),
		Command: []string{"sh", "-c", script},
		Timeout: timeout,
	})
	require.NoError(t, err)
	return b
}

func TestResult(t *testing.T) {
	tests := map[string]struct {
		script         string
		wantResult     domain.Result
		wantScore      int
		wantErr        bool
		wantNoFinalErr bool
	}{
		"passed": {
			script:     `echo '{"score":100,"finished":false}' >&3; echo '{"score":200,"passed":true,"finished":true}' >&3`,
			wantResult: domain.ResultPassed,
			wantScore:  200,
		},
		"failed with exit 0": {
			script:     `echo '{"score":0,"passed":false,"finished":true}' >&3`,
			wantResult: domain.ResultFailed,
		},
		"passed with non-zero exit": {
			script:     `echo '{"score":200,"passed":true,"finished":true}' >&3; exit 1`,
			wantResult: domain.ResultError,
			wantScore:  200,
			wantErr:    true,
		},
		"failed with non-zero exit": {
			script:     `echo '{"score":0,"passed":false,"finished":true}' >&3; exit 1`,
			wantResult: domain.ResultError,
			wantErr:    true,
		},
		"no final report": {
			script:         `echo '{"score":100,"finished":false}' >&3`,
			wantResult:     domain.ResultError,
			wantScore:      100,
			wantErr:        true,
			wantNoFinalErr: true,
		},
		"invalid json": {
			script:     `echo 'score: 100' >&3`,
			wantResult: domain.ResultError,
			wantErr:    true,
		},
		"final report without passed": {
			script:     `echo '{"score":100,"finished":true}' >&3`,
			wantResult: domain.ResultError,
			wantErr:    true,
		},
		"report without score": {
			script:     `echo '{"finished":false}' >&3`,
			wantResult: domain.ResultError,
			wantErr:    true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := newShell(t, tt.script, 0)

			got := run(t, b, domain.NewJob("job-id", "192.0.2.1"))

			assert.Equal(t, tt.wantResult, got.result)
			assert.Equal(t, tt.wantScore, got.score)
			if tt.wantErr {
				assert.Error(t, got.err)
			} else {
				assert.NoError(t, got.err)
			}
			if tt.wantNoFinalErr {
				assert.ErrorIs(t, got.err, reportfd.ErrNoFinalReport)
			}
		})
	}
}

func TestTimeoutStopsChildProcesses(t *testing.T) {
	// バックグラウンドの子プロセスがstdoutを開いたまま残ると、runnerのログの読み取りが終わらない。
	b := newShell(t, `sleep 30 & sleep 30`, 200*time.Millisecond)

	start := time.Now()
	got := run(t, b, domain.NewJob("job-id", "192.0.2.1"))

	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, domain.ResultError, got.result)
	assert.ErrorContains(t, got.err, "timed out")
}

func TestStartRendersArguments(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"$@\"\necho '{\"score\":1,\"passed\":true,\"finished\":true}' >&3\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bench.sh"), []byte(script), 0o755))

	b, err := New(bundle.CommandBench{
		Dir:     dir,
		Command: []string{"./bench.sh", "--target", "{{ .TargetIP }}", "--all", `{{ join .InstanceIPs "," }}`},
	})
	require.NoError(t, err)

	// 相対パスのコマンドはdirを基準に解決する。2回続けて実行できることも確かめる。
	for range 2 {
		got := run(t, b, domain.NewJob("job-id", "10.0.0.1", "10.0.0.1", "10.0.0.2"))

		require.NoError(t, got.err)
		assert.Equal(t, "--target 10.0.0.1 --all 10.0.0.1,10.0.0.2\n", got.stdout)
		assert.Equal(t, domain.ResultPassed, got.result)
	}
}

func TestNewRejectsInvalidCommand(t *testing.T) {
	tests := map[string][]string{
		"empty command":   {},
		"unknown field":   {"./bench", "{{ .Target }}"},
		"broken template": {"./bench", "{{ .TargetIP"},
	}

	for name, command := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New(bundle.CommandBench{Dir: "/tmp", Command: command})
			assert.Error(t, err)
		})
	}
}
