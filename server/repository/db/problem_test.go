package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
)

func TestProblem(t *testing.T) {
	t.Parallel()

	repo, _ := setupRepository(t)
	ctx := context.Background()

	_, err := repo.GetProblemName(ctx)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	require.NoError(t, repo.CreateProblem(ctx, "sample-fib"))

	name, err := repo.GetProblemName(ctx)
	require.NoError(t, err)
	assert.Equal(t, "sample-fib", name)

	// DB に記録できる問題は 1 つだけである。
	assert.Error(t, repo.CreateProblem(ctx, "another"))
}
