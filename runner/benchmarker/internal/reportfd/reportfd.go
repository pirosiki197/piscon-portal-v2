// Package reportfd は、追加のファイルディスクリプタに進捗レポートを書くベンチマーカーを
// 子プロセスとして実行し、最後のレポートと終了ステータスから結果を判定する。
//
// レポートのエンコード方式は、[Decoder] を差し替えて選ぶ。
package reportfd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker"
	"github.com/traPtitech/piscon-portal-v2/runner/domain"
)

// FD は、子プロセスがレポートを書くファイルディスクリプタの番号である。
// [exec.Cmd.ExtraFiles] の先頭が、子プロセスではfd 3になる。
const FD = 3

// ErrNoFinalReport は、最終レポート(Finished=true)を受け取る前にレポートのストリームが
// レポートの区切りで終わったことを表す。
var ErrNoFinalReport = errors.New("report stream ended without final report")

// Report は、ベンチマーカーが書いた1件の進捗レポートである。
type Report struct {
	Score    int64
	Passed   bool
	Finished bool
}

// Decoder は、ストリームからレポートを1件ずつ読む。
//
// Decode は、ストリームがレポートの区切りで終わったときに限り、[io.EOF] に一致するエラーを返す。
// レポートの途中でストリームが終わったときは、[io.ErrUnexpectedEOF] などの別のエラーを返さなければならない。
type Decoder interface {
	Decode() (Report, error)
}

// Process は、実行中のベンチマーカーの子プロセスである。
type Process struct {
	cmd *exec.Cmd
	// resultCh は、レポートのストリームが終わったときに値をちょうど1つ受け取る。
	resultCh    chan result
	latestScore atomic.Int64
}

type result struct {
	passed bool
	err    error
}

// Start は、レポート用のパイプを [FD] として cmd に渡して起動し、
// newDecoder が返す Decoder でレポートをバックグラウンドで読み始める。
// 呼び出し側は cmd.ExtraFiles、cmd.Stdout、cmd.Stderr を設定してはならない。
//
// 成功したら、呼び出し側はリソースを解放するために [Process.Wait] を呼ばなければならない。
func Start(cmd *exec.Cmd, newDecoder func(io.Reader) Decoder) (*Process, benchmarker.Outputs, time.Time, error) {
	reportReader, reportWriter, err := os.Pipe()
	if err != nil {
		return nil, benchmarker.Outputs{}, time.Time{}, fmt.Errorf("create report pipe: %w", err)
	}
	// 書き込み側は子プロセスに渡した後、親では不要になる。
	// 起動に失敗した場合は読み取り側も解放する。
	defer reportWriter.Close()
	started := false
	defer func() {
		if !started {
			reportReader.Close()
		}
	}()

	cmd.ExtraFiles = []*os.File{reportWriter}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, benchmarker.Outputs{}, time.Time{}, fmt.Errorf("get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, benchmarker.Outputs{}, time.Time{}, fmt.Errorf("get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, benchmarker.Outputs{}, time.Time{}, fmt.Errorf("start command: %w", err)
	}
	started = true

	p := &Process{
		cmd:      cmd,
		resultCh: make(chan result, 1),
	}
	go p.watchReport(reportReader, newDecoder(reportReader))

	return p, benchmarker.Outputs{Stdout: stdout, Stderr: stderr}, time.Now(), nil
}

// LatestScore は、これまでに受け取った最後のレポートのスコアを返す。
func (p *Process) LatestScore() int {
	return int(p.latestScore.Load())
}

// Wait は、プロセスの終了とレポートのストリームの終了を待ち、最後のレポートから結果を判定する。
// 呼び出し側は、stdout と stderr を読み終えてから Wait を呼ばなければならない。
//
// 終了ステータスが0以外なら、レポートの内容にかかわらず [domain.ResultError] を返す。
// 呼び出しが返った時点で、最終レポートの内容が [Process.LatestScore] に反映されている。
func (p *Process) Wait() (domain.Result, time.Time, error) {
	waitErr := p.cmd.Wait()
	finishedAt := time.Now()

	// 子プロセスが終了すればパイプの書き込み側が閉じるため、watchReportも必ず終わる。
	res := <-p.resultCh

	if waitErr != nil {
		return domain.ResultError, finishedAt, fmt.Errorf("wait command: %w", waitErr)
	}
	if res.err != nil {
		return domain.ResultError, finishedAt, res.err
	}
	if res.passed {
		return domain.ResultPassed, finishedAt, nil
	}
	return domain.ResultFailed, finishedAt, nil
}

func (p *Process) watchReport(report io.Closer, dec Decoder) {
	defer report.Close()
	for {
		r, err := dec.Decode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = fmt.Errorf("%w: %w", ErrNoFinalReport, err)
			}
			p.resultCh <- result{err: err}
			return
		}
		p.latestScore.Store(r.Score)
		if r.Finished {
			p.resultCh <- result{passed: r.Passed}
			return
		}
	}
}
