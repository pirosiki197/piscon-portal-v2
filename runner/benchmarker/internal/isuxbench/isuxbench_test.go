package isuxbench

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/isucon/isucon10-portal/proto.go/isuxportal/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker/internal/reportfd"
	"google.golang.org/protobuf/proto"
)

func encodeReport(t *testing.T, res *resources.BenchmarkResult) []byte {
	t.Helper()
	data, err := proto.Marshal(res)
	require.NoError(t, err)
	buf := binary.BigEndian.AppendUint16(nil, uint16(len(data)))
	return append(buf, data...)
}

// decodeAll は、ストリームがエラーで終わるまでレポートを読む。
func decodeAll(data []byte) ([]reportfd.Report, error) {
	dec := decoder{r: bytes.NewReader(data)}
	var reports []reportfd.Report
	for {
		r, err := dec.Decode()
		if err != nil {
			return reports, err
		}
		reports = append(reports, r)
	}
}

func TestDecoder(t *testing.T) {
	progress := &resources.BenchmarkResult{Finished: false, Score: 100}

	tests := map[string]struct {
		reports         func(t *testing.T) []byte
		want            []reportfd.Report
		wantErrUnexpEOF bool
	}{
		"progress and final": {
			reports: func(t *testing.T) []byte {
				return append(encodeReport(t, progress),
					encodeReport(t, &resources.BenchmarkResult{Finished: true, Passed: true, Score: 200})...)
			},
			want: []reportfd.Report{
				{Score: 100},
				{Score: 200, Passed: true, Finished: true},
			},
		},
		"no reports": {
			reports: func(*testing.T) []byte { return nil },
		},
		"truncated length": {
			reports:         func(*testing.T) []byte { return []byte{0x00} },
			wantErrUnexpEOF: true,
		},
		"missing body": {
			reports:         func(*testing.T) []byte { return []byte{0x00, 0x05} },
			wantErrUnexpEOF: true,
		},
		"truncated body": {
			reports: func(t *testing.T) []byte {
				b := encodeReport(t, progress)
				return b[:len(b)-1]
			},
			wantErrUnexpEOF: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := decodeAll(tt.reports(t))

			assert.Equal(t, tt.want, got)
			if tt.wantErrUnexpEOF {
				assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
				assert.NotErrorIs(t, err, io.EOF)
			} else {
				assert.ErrorIs(t, err, io.EOF)
			}
		})
	}
}
