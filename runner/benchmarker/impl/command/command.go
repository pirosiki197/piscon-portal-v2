// Package command は、自作問題のベンチマーカーを設定したコマンドラインで実行する。
// ベンチマーカーは、fd 3 に JSON Lines で進捗を報告する。
package command

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker/internal/reportfd"
	"github.com/traPtitech/piscon-portal-v2/runner/config"
	"github.com/traPtitech/piscon-portal-v2/runner/domain"
)

const (
	defaultTimeout = 10 * time.Minute
	// タイムアウト後にプロセスグループを停止してから、パイプが閉じるのを待つ上限。
	waitDelay = 10 * time.Second
)

// templateData は、引数のテンプレートから参照できる値である。
type templateData struct {
	TargetIP    string
	InstanceIPs []string
}

var templateFuncs = template.FuncMap{"join": strings.Join}

type Command struct {
	dir     string
	args    []*template.Template
	timeout time.Duration

	proc   *reportfd.Process
	ctx    context.Context
	cancel context.CancelFunc
}

var _ benchmarker.Benchmarker = (*Command)(nil)

// New は、問題の設定を検証し、引数のテンプレートを解析する。
//
// 設定の項目:
//   - dir: ベンチマーカーを実行するディレクトリ。必須。
//   - command: コマンドラインを、Go の text/template の文字列のリストで指定する。必須。
//   - timeout: [time.ParseDuration] の形式の文字列。省略すると10分になる。
func New(conf config.Problem) (*Command, error) {
	dir, ok := conf.Options["dir"].(string)
	if !ok || dir == "" {
		return nil, fmt.Errorf("invalid or missing 'dir' option in problem configuration")
	}

	rawArgs, ok := conf.Options["command"].([]any)
	if !ok || len(rawArgs) == 0 {
		return nil, fmt.Errorf("invalid or missing 'command' option in problem configuration")
	}
	args := make([]*template.Template, 0, len(rawArgs))
	for i, raw := range rawArgs {
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("'command' option must be a list of strings: element %d is %T", i, raw)
		}
		tmpl, err := template.New(fmt.Sprintf("command[%d]", i)).Funcs(templateFuncs).Parse(s)
		if err != nil {
			return nil, fmt.Errorf("parse 'command' option: %w", err)
		}
		args = append(args, tmpl)
	}
	// 存在しないフィールドの参照は実行時まで分からないので、起動時に試しに展開して検出する。
	if _, err := renderArgs(args, templateData{TargetIP: "192.0.2.1", InstanceIPs: []string{"192.0.2.1"}}); err != nil {
		return nil, fmt.Errorf("invalid 'command' option: %w", err)
	}

	timeout := defaultTimeout
	if v, exists := conf.Options["timeout"]; exists {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid 'timeout' option in problem configuration")
		}
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("invalid 'timeout' option in problem configuration")
		}
		timeout = d
	}

	return &Command{dir: dir, args: args, timeout: timeout}, nil
}

func renderArgs(args []*template.Template, data templateData) ([]string, error) {
	rendered := make([]string, 0, len(args))
	for _, tmpl := range args {
		var sb strings.Builder
		if err := tmpl.Execute(&sb, data); err != nil {
			return nil, fmt.Errorf("render %s: %w", tmpl.Name(), err)
		}
		rendered = append(rendered, sb.String())
	}
	return rendered, nil
}

func (b *Command) Start(ctx context.Context, job *domain.Job) (benchmarker.Outputs, time.Time, error) {
	args, err := renderArgs(b.args, templateData{
		TargetIP:    job.GetTargetIPAdress(),
		InstanceIPs: job.GetInstanceIPAddresses(),
	})
	if err != nil {
		return benchmarker.Outputs{}, time.Time{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	started := false
	defer func() {
		if !started {
			cancel()
		}
	}()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = b.dir
	// ベンチマーカーが起動した子プロセスも、タイムアウト時にまとめて停止する。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = waitDelay

	proc, out, startedAt, err := reportfd.Start(cmd, newJSONDecoder)
	if err != nil {
		return benchmarker.Outputs{}, time.Time{}, fmt.Errorf("start benchmarker: %w", err)
	}
	started = true
	b.proc, b.ctx, b.cancel = proc, ctx, cancel

	return out, startedAt, nil
}

func (b *Command) Wait(_ context.Context) (domain.Result, time.Time, error) {
	defer b.cancel()
	result, finishedAt, err := b.proc.Wait()
	if errors.Is(b.ctx.Err(), context.DeadlineExceeded) {
		return domain.ResultError, finishedAt, fmt.Errorf("benchmarker timed out after %s: %w", b.timeout, err)
	}
	if err != nil {
		return result, finishedAt, fmt.Errorf("wait benchmarker: %w", err)
	}
	return result, finishedAt, nil
}

func (b *Command) CalculateScore(_ context.Context, _, _ string) (int, error) {
	if b.proc == nil {
		return 0, nil
	}
	return b.proc.LatestScore(), nil
}

// jsonReport は、レポートのストリームの1行である。
// 必須のフィールドが欠けた行を検出できるよう、ポインタで受ける。
type jsonReport struct {
	Score    *int64 `json:"score"`
	Passed   *bool  `json:"passed"`
	Finished *bool  `json:"finished"`
}

type jsonDecoder struct {
	scanner *bufio.Scanner
}

func newJSONDecoder(r io.Reader) reportfd.Decoder {
	return &jsonDecoder{scanner: bufio.NewScanner(r)}
}

func (d *jsonDecoder) Decode() (reportfd.Report, error) {
	if !d.scanner.Scan() {
		if err := d.scanner.Err(); err != nil {
			return reportfd.Report{}, fmt.Errorf("read report: %w", err)
		}
		return reportfd.Report{}, fmt.Errorf("read report: %w", io.EOF)
	}

	var r jsonReport
	if err := json.Unmarshal(d.scanner.Bytes(), &r); err != nil {
		return reportfd.Report{}, fmt.Errorf("decode report %q: %w", d.scanner.Text(), err)
	}
	if r.Score == nil || r.Finished == nil {
		return reportfd.Report{}, fmt.Errorf("report must contain score and finished: %q", d.scanner.Text())
	}
	if *r.Finished && r.Passed == nil {
		return reportfd.Report{}, fmt.Errorf("final report must contain passed: %q", d.scanner.Text())
	}

	return reportfd.Report{
		Score:    *r.Score,
		Passed:   r.Passed != nil && *r.Passed,
		Finished: *r.Finished,
	}, nil
}
