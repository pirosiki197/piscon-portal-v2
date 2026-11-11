package usecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	repomock "github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	githubmock "github.com/traPtitech/piscon-portal-v2/server/services/github/mock"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
	instancemock "github.com/traPtitech/piscon-portal-v2/server/services/instance/mock"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	"github.com/traPtitech/piscon-portal-v2/server/utils/ptr"
	"go.uber.org/mock/gomock"
)

type environmentMocks struct {
	repo       *repomock.MockRepository
	envManager *instancemock.MockEnvironmentManager
	manager    *instancemock.MockManager
	github     *githubmock.MockService
}

func newEnvironmentUseCase(t *testing.T, benchTarget string) (usecase.EnvironmentUseCase, environmentMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := environmentMocks{
		repo:       repomock.NewMockRepository(ctrl),
		envManager: instancemock.NewMockEnvironmentManager(ctrl),
		manager:    instancemock.NewMockManager(ctrl),
		github:     githubmock.NewMockService(ctrl),
	}
	return usecase.NewEnvironmentUseCase(m.repo, m.envManager, m.manager, m.github, benchTarget), m
}

func TestGetTeamEnvironment(t *testing.T) {
	t.Parallel()

	teamID := uuid.New()
	env := domain.Environment{ID: uuid.New(), TeamID: teamID, ProviderEnvironmentID: "piscon-env"}
	servers := []domain.InfraServer{
		{Name: "Router", ProviderInstanceID: "i-router"},
		{Name: "Webapp", ProviderInstanceID: "i-webapp"},
	}
	recorded := env
	recorded.Instances = env.NewInstances(servers)

	t.Run("チーム環境がない", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(domain.Environment{}, repository.ErrNotFound)

		_, err := uc.GetTeamEnvironment(t.Context(), teamID)
		assert.ErrorIs(t, err, usecase.ErrNotFound)
	})

	t.Run("作成中はサーバーを記録しない", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(env, nil)
		m.envManager.EXPECT().Get(gomock.Any(), "piscon-env").Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusCreating}, nil)

		got, err := uc.GetTeamEnvironment(t.Context(), teamID)
		require.NoError(t, err)
		assert.Equal(t, domain.EnvironmentStatusCreating, got.Infra.Status)
		assert.Empty(t, got.Instances)
	})

	t.Run("ready になったらサーバーを記録し、固定された対象を返す", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "Router")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(env, nil)
		m.envManager.EXPECT().Get(gomock.Any(), "piscon-env").
			Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusReady, Servers: servers}, nil)
		m.repo.EXPECT().CreateInstances(gomock.Any(), gomock.Len(2)).Return(nil)
		m.repo.EXPECT().FindEnvironment(gomock.Any(), env.ID).Return(recorded, nil)
		m.manager.EXPECT().GetByIDs(gomock.Any(), []string{"i-router", "i-webapp"}).Return([]domain.InfraInstance{
			{ProviderInstanceID: "i-router", Status: domain.InstanceStatusRunning, PrivateIP: ptr.Of("10.0.0.1")},
			{ProviderInstanceID: "i-webapp", Status: domain.InstanceStatusRunning, PrivateIP: ptr.Of("10.0.0.2")},
		}, nil)

		got, err := uc.GetTeamEnvironment(t.Context(), teamID)
		require.NoError(t, err)
		assert.Equal(t, domain.EnvironmentStatusReady, got.Infra.Status)
		require.Len(t, got.Instances, 2)
		assert.Equal(t, domain.InstanceStatusRunning, got.Instances[1].Infra.Status)
		require.True(t, got.BenchTargetID.IsSet())
		assert.Equal(t, recorded.Instances[0].ID, got.BenchTargetID.Get())
	})

	t.Run("サーバーを記録済みなら記録し直さない", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(recorded, nil)
		m.envManager.EXPECT().Get(gomock.Any(), "piscon-env").
			Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusReady, Servers: servers}, nil)
		m.manager.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)

		got, err := uc.GetTeamEnvironment(t.Context(), teamID)
		require.NoError(t, err)
		assert.False(t, got.BenchTargetID.IsSet())
	})

	t.Run("削除が完了していたら削除済みにして ErrNotFound を返す", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		deleted := recorded
		deleted.DeletedAt = ptr.Of(time.Now())
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(recorded, nil)
		m.envManager.EXPECT().Get(gomock.Any(), "piscon-env").Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted}, nil)
		m.repo.EXPECT().DeleteEnvironment(gomock.Any(), env.ID).Return(nil)
		m.repo.EXPECT().FindEnvironment(gomock.Any(), env.ID).Return(deleted, nil)
		m.manager.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)

		_, err := uc.GetTeamEnvironment(t.Context(), teamID)
		assert.ErrorIs(t, err, usecase.ErrNotFound)
	})
}

func TestCreateEnvironment(t *testing.T) {
	t.Parallel()

	teamID := uuid.New()
	team := domain.Team{ID: teamID, GitHubIDs: []string{"user1"}}

	t.Run("作成を依頼して記録する", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(domain.Environment{}, repository.ErrNotFound)
		m.repo.EXPECT().FindTeam(gomock.Any(), teamID).Return(team, nil)
		m.github.EXPECT().GetSSHKeys(gomock.Any(), []string{"user1"}).Return([]string{"ssh-ed25519 key"}, nil)
		m.envManager.EXPECT().Create(gomock.Any(), gomock.Any(), instance.EnvironmentParams{
			TeamID:        teamID.String(),
			SSHPublicKeys: []string{"ssh-ed25519 key"},
		}).DoAndReturn(func(_ context.Context, name string, _ instance.EnvironmentParams) (string, error) { return name, nil })
		m.repo.EXPECT().CreateEnvironment(gomock.Any(), gomock.Any()).Return(nil)
		m.envManager.EXPECT().Get(gomock.Any(), gomock.Any()).Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusCreating}, nil)

		got, err := uc.CreateEnvironment(t.Context(), teamID)
		require.NoError(t, err)
		assert.Equal(t, teamID, got.TeamID)
		assert.Equal(t, "piscon-"+got.ID.String(), got.ProviderEnvironmentID)
		assert.Equal(t, domain.EnvironmentStatusCreating, got.Infra.Status)
	})

	t.Run("削除されていないチーム環境があれば作らない", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).
			Return(domain.Environment{ID: uuid.New(), ProviderEnvironmentID: "piscon-env"}, nil)
		m.envManager.EXPECT().Get(gomock.Any(), "piscon-env").Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusFailed}, nil)

		_, err := uc.CreateEnvironment(t.Context(), teamID)
		assert.True(t, usecase.IsUseCaseError(err))
	})

	t.Run("同時に作られて記録できなければ、作成を依頼したチーム環境を消す", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(domain.Environment{}, repository.ErrNotFound)
		m.repo.EXPECT().FindTeam(gomock.Any(), teamID).Return(team, nil)
		m.github.EXPECT().GetSSHKeys(gomock.Any(), gomock.Any()).Return(nil, nil)
		m.envManager.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return("piscon-new", nil)
		m.repo.EXPECT().CreateEnvironment(gomock.Any(), gomock.Any()).Return(repository.ErrAlreadyExists)
		m.envManager.EXPECT().Delete(gomock.Any(), "piscon-new").Return(nil)

		_, err := uc.CreateEnvironment(t.Context(), teamID)
		assert.True(t, usecase.IsUseCaseError(err))
	})

	t.Run("作成の依頼が失敗したら、作り始めているかもしれないチーム環境を消す", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(domain.Environment{}, repository.ErrNotFound)
		m.repo.EXPECT().FindTeam(gomock.Any(), teamID).Return(team, nil)
		m.github.EXPECT().GetSSHKeys(gomock.Any(), gomock.Any()).Return(nil, nil)
		var name string
		m.envManager.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, n string, _ instance.EnvironmentParams) (string, error) {
				name = n
				return "", assert.AnError
			})
		m.envManager.EXPECT().Delete(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, id string) error {
				assert.Equal(t, name, id)
				return nil
			})

		_, err := uc.CreateEnvironment(t.Context(), teamID)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("プロバイダーが受け付けない入力なら、チームのエラーとして返す", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(domain.Environment{}, repository.ErrNotFound)
		m.repo.EXPECT().FindTeam(gomock.Any(), teamID).Return(team, nil)
		m.github.EXPECT().GetSSHKeys(gomock.Any(), gomock.Any()).Return(nil, nil)
		m.envManager.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
			Return("", fmt.Errorf("%w: too long", instance.ErrInvalidEnvironmentParams))

		_, err := uc.CreateEnvironment(t.Context(), teamID)
		assert.True(t, usecase.IsUseCaseError(err))
	})
}

func TestDeleteEnvironment(t *testing.T) {
	t.Parallel()

	teamID := uuid.New()
	env := domain.Environment{ID: uuid.New(), TeamID: teamID, ProviderEnvironmentID: "piscon-env"}

	t.Run("削除を依頼する", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(env, nil)
		m.repo.EXPECT().GetBenchmarks(gomock.Any(), gomock.Any()).Return(nil, nil)
		m.envManager.EXPECT().Delete(gomock.Any(), "piscon-env").Return(nil)

		assert.NoError(t, uc.DeleteEnvironment(t.Context(), teamID))
	})

	t.Run("ベンチが待機中なら削除しない", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(env, nil)
		m.repo.EXPECT().GetBenchmarks(gomock.Any(), gomock.Any()).
			Return([]domain.Benchmark{{ID: uuid.New(), Status: domain.BenchmarkStatusWaiting}}, nil)

		err := uc.DeleteEnvironment(t.Context(), teamID)
		assert.True(t, usecase.IsUseCaseError(err))
	})

	t.Run("チーム環境がない", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(domain.Environment{}, repository.ErrNotFound)

		assert.ErrorIs(t, uc.DeleteEnvironment(t.Context(), teamID), usecase.ErrNotFound)
	})
}

func TestUpdateEnvironment(t *testing.T) {
	t.Parallel()

	teamID := uuid.New()
	env := domain.Environment{ID: uuid.New(), TeamID: teamID, ProviderEnvironmentID: "piscon-env"}
	env.Instances = env.NewInstances([]domain.InfraServer{
		{Name: "A", ProviderInstanceID: "i-a"},
		{Name: "B", ProviderInstanceID: "i-b"},
	})
	infraA := domain.InfraInstance{ProviderInstanceID: "i-a", Status: domain.InstanceStatusRunning}
	infraB := domain.InfraInstance{ProviderInstanceID: "i-b", Status: domain.InstanceStatusRunning}

	t.Run("一部のサーバーで失敗しても残りのサーバーを操作する", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(env, nil)
		m.envManager.EXPECT().Get(gomock.Any(), "piscon-env").Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusReady}, nil)
		m.manager.EXPECT().GetByIDs(gomock.Any(), []string{"i-a", "i-b"}).Return([]domain.InfraInstance{infraA, infraB}, nil)
		m.manager.EXPECT().Stop(gomock.Any(), infraA).Return(assert.AnError)
		m.manager.EXPECT().Stop(gomock.Any(), infraB).Return(nil)

		err := uc.UpdateEnvironment(t.Context(), teamID, domain.InstanceOperationStop)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "server A")
	})

	t.Run("ready でなければ操作しない", func(t *testing.T) {
		t.Parallel()
		uc, m := newEnvironmentUseCase(t, "")
		m.repo.EXPECT().FindTeamEnvironment(gomock.Any(), teamID).Return(domain.Environment{ID: env.ID, ProviderEnvironmentID: "piscon-env"}, nil)
		m.envManager.EXPECT().Get(gomock.Any(), "piscon-env").Return(domain.InfraEnvironment{Status: domain.EnvironmentStatusCreating}, nil)

		err := uc.UpdateEnvironment(t.Context(), teamID, domain.InstanceOperationStart)
		assert.True(t, usecase.IsUseCaseError(err))
	})
}
