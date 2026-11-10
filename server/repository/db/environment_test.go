package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
)

func newTestEnvironment(teamID uuid.UUID) domain.Environment {
	env := domain.NewEnvironment(teamID)
	env.ProviderEnvironmentID = "piscon-" + env.ID.String()
	return env
}

func TestCreateAndFindEnvironment(t *testing.T) {
	t.Parallel()

	repo, _ := setupRepository(t)
	ctx := t.Context()
	teamID := uuid.New()

	_, err := repo.FindTeamEnvironment(ctx, teamID)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	env := newTestEnvironment(teamID)
	require.NoError(t, repo.CreateEnvironment(ctx, env))

	got, err := repo.FindTeamEnvironment(ctx, teamID)
	require.NoError(t, err)
	assert.Equal(t, env.ID, got.ID)
	assert.Equal(t, teamID, got.TeamID)
	assert.Equal(t, env.ProviderEnvironmentID, got.ProviderEnvironmentID)
	assert.Empty(t, got.Instances)
	assert.Nil(t, got.DeletedAt)

	// チームごとに、削除されていないチーム環境は 1 つだけである。
	err = repo.CreateEnvironment(ctx, newTestEnvironment(teamID))
	assert.ErrorIs(t, err, repository.ErrAlreadyExists)

	// 削除済みになれば、新しいチーム環境を作れる。
	require.NoError(t, repo.DeleteEnvironment(ctx, env.ID))
	_, err = repo.FindTeamEnvironment(ctx, teamID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
	require.NoError(t, repo.CreateEnvironment(ctx, newTestEnvironment(teamID)))

	deleted, err := repo.FindEnvironment(ctx, env.ID)
	require.NoError(t, err)
	assert.NotNil(t, deleted.DeletedAt)
}

func TestCreateInstances(t *testing.T) {
	t.Parallel()

	repo, _ := setupRepository(t)
	ctx := t.Context()

	env := newTestEnvironment(uuid.New())
	require.NoError(t, repo.CreateEnvironment(ctx, env))

	servers := []domain.InfraServer{
		{Name: "Webapp", ProviderInstanceID: "i-webapp"},
		{Name: "Router", ProviderInstanceID: "i-router"},
	}
	require.NoError(t, repo.CreateInstances(ctx, env.NewInstances(servers)))
	// 同期が 2 回走っても、サーバーは重複しない。
	require.NoError(t, repo.CreateInstances(ctx, env.NewInstances(servers)))

	got, err := repo.FindEnvironment(ctx, env.ID)
	require.NoError(t, err)
	require.Len(t, got.Instances, 2)
	assert.Equal(t, "Router", got.Instances[0].Name)
	assert.Equal(t, "i-router", got.Instances[0].Infra.ProviderInstanceID)
	assert.Equal(t, "Webapp", got.Instances[1].Name)
	assert.Equal(t, env.TeamID, got.Instances[1].TeamID)
}

func TestDeleteEnvironment(t *testing.T) {
	t.Parallel()

	repo, _ := setupRepository(t)
	ctx := t.Context()

	env := newTestEnvironment(uuid.New())
	require.NoError(t, repo.CreateEnvironment(ctx, env))
	require.NoError(t, repo.CreateInstances(ctx, env.NewInstances([]domain.InfraServer{{Name: "Webapp", ProviderInstanceID: "i-1"}})))

	require.NoError(t, repo.DeleteEnvironment(ctx, env.ID))
	// すでに削除済みでもエラーにしない。
	require.NoError(t, repo.DeleteEnvironment(ctx, env.ID))

	got, err := repo.FindEnvironment(ctx, env.ID)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), *got.DeletedAt, time.Minute)
	require.Len(t, got.Instances, 1)
	assert.NotNil(t, got.Instances[0].DeletedAt)

	instances, err := repo.GetTeamInstances(ctx, env.TeamID, false)
	require.NoError(t, err)
	assert.Empty(t, instances)
}

func TestGetEnvironments(t *testing.T) {
	t.Parallel()

	repo, _ := setupRepository(t)
	ctx := t.Context()

	active := newTestEnvironment(uuid.New())
	deleted := newTestEnvironment(uuid.New())
	require.NoError(t, repo.CreateEnvironment(ctx, active))
	require.NoError(t, repo.CreateEnvironment(ctx, deleted))
	require.NoError(t, repo.DeleteEnvironment(ctx, deleted.ID))

	got, err := repo.GetEnvironments(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, active.ID, got[0].ID)
}
