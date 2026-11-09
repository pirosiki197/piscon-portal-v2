package reportfd

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type step struct {
	report Report
	err    error
}

type fakeDecoder struct {
	steps []step
}

func (d *fakeDecoder) Decode() (Report, error) {
	s := d.steps[0]
	d.steps = d.steps[1:]
	return s.report, s.err
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

func TestWatchReport(t *testing.T) {
	errBroken := errors.New("broken report")

	tests := map[string]struct {
		steps          []step
		wantPassed     bool
		wantScore      int
		wantErr        error
		wantNoFinalErr bool
	}{
		"passed": {
			steps: []step{
				{report: Report{Score: 100}},
				{report: Report{Score: 200, Passed: true, Finished: true}},
			},
			wantPassed: true,
			wantScore:  200,
		},
		"failed": {
			steps:     []step{{report: Report{Score: 50, Finished: true}}},
			wantScore: 50,
		},
		"stops reading at final report": {
			steps: []step{
				{report: Report{Score: 10, Passed: true, Finished: true}},
				{err: errBroken},
			},
			wantPassed: true,
			wantScore:  10,
		},
		"ends without final report": {
			steps: []step{
				{report: Report{Score: 100}},
				{err: fmt.Errorf("read: %w", io.EOF)},
			},
			wantScore:      100,
			wantNoFinalErr: true,
		},
		"broken report": {
			steps: []step{
				{report: Report{Score: 100}},
				{err: errBroken},
			},
			wantScore: 100,
			wantErr:   errBroken,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := &Process{resultCh: make(chan result, 1)}

			p.watchReport(nopCloser{}, &fakeDecoder{steps: tt.steps})
			res := <-p.resultCh

			assert.Equal(t, tt.wantScore, p.LatestScore())
			switch {
			case tt.wantNoFinalErr:
				assert.ErrorIs(t, res.err, ErrNoFinalReport)
			case tt.wantErr != nil:
				assert.ErrorIs(t, res.err, tt.wantErr)
				assert.NotErrorIs(t, res.err, ErrNoFinalReport)
			default:
				require.NoError(t, res.err)
				assert.Equal(t, tt.wantPassed, res.passed)
			}
		})
	}
}
