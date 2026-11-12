package sourceip_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/runner/benchmarker/internal/sourceip"
)

func TestFor(t *testing.T) {
	t.Parallel()

	ip, err := sourceip.For("127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", ip)
}
