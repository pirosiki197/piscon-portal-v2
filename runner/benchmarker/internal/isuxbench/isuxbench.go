// Package isuxbench runs a benchmarker child process that reports its results
// through the ISUXBENCH_REPORT_FD protocol of the isuxportal supervisor.
//
// ISUCON11の予選・本選のベンチマーカーはどちらもISUCON10のReporterを利用しており、
// 結果の通知方法が共通である。
// https://github.com/isucon/isucon10-portal/blob/master/bench-tool.go/benchrun/reporter.go
package isuxbench

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/isucon/isucon10-portal/proto.go/isuxportal/resources"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker/internal/reportfd"
	"google.golang.org/protobuf/proto"
)

// ErrNoFinalReport は、最終レポート(finished=true)を受け取る前にレポートのストリームが
// レポートの区切りで終わったことを表す。
// ベンチマーカーによっては、競技上の失敗でも最終レポートを送らずに終了することがある。
var ErrNoFinalReport = reportfd.ErrNoFinalReport

// Process は、実行中のベンチマーカーの子プロセスである。
type Process = reportfd.Process

// Start は、ISUXBENCH_REPORT_FD を設定して cmd を起動し、レポートをバックグラウンドで読み始める。
// 呼び出し側は cmd.ExtraFiles、cmd.Stdout、cmd.Stderr を設定してはならない。
// cmd.Env は設定してよく、Start はそこに ISUXBENCH_REPORT_FD を追加する。
//
// 成功したら、呼び出し側はリソースを解放するために [Process.Wait] を呼ばなければならない。
func Start(cmd *exec.Cmd) (*Process, benchmarker.Outputs, time.Time, error) {
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("ISUXBENCH_REPORT_FD=%d", reportfd.FD))
	proc, out, startedAt, err := reportfd.Start(cmd, func(r io.Reader) reportfd.Decoder { return decoder{r: r} })
	if err != nil {
		return nil, benchmarker.Outputs{}, time.Time{}, fmt.Errorf("start benchmarker process: %w", err)
	}
	return proc, out, startedAt, nil
}

type decoder struct {
	r io.Reader
}

func (d decoder) Decode() (reportfd.Report, error) {
	res, err := readResult(d.r)
	if err != nil {
		return reportfd.Report{}, err
	}
	return reportfd.Report{
		Score:    res.GetScore(),
		Passed:   res.GetPassed(),
		Finished: res.GetFinished(),
	}, nil
}

func readResult(report io.Reader) (*resources.BenchmarkResult, error) {
	var sizeData [2]byte
	if _, err := io.ReadFull(report, sizeData[:]); err != nil {
		return nil, fmt.Errorf("parse benchmark result: %w", err)
	}

	size := int(binary.BigEndian.Uint16(sizeData[:]))
	data := make([]byte, size)
	if _, err := io.ReadFull(report, data); err != nil {
		// 長さを読んだ後に本体がない場合はレポートの途中で切れている。
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("parse benchmark result: %w", err)
	}

	var res resources.BenchmarkResult
	if err := proto.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("decode benchmark result: %w", err)
	}
	return &res, nil
}
