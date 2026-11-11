package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/services/github"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
	"github.com/traPtitech/piscon-portal-v2/server/utils/optional"
)

// チーム環境を返すメソッドは、読み出す前に DB の記録をプロバイダーの状態に合わせる（同期する）。
// 同期では、チーム環境が ready になっていればサーバーを記録し、deleted になっていればチーム環境を削除済みにする。
type EnvironmentUseCase interface {
	// GetTeamEnvironment は、チームの削除されていないチーム環境を返す。なければ [ErrNotFound] を返す。
	GetTeamEnvironment(ctx context.Context, teamID uuid.UUID) (TeamEnvironment, error)
	// CreateEnvironment は、チーム環境の作成をプロバイダーに依頼し、記録する。作成の完了は待たない。
	// チームに削除されていないチーム環境がすでにあれば、[UseCaseError] を返す。
	CreateEnvironment(ctx context.Context, teamID uuid.UUID) (TeamEnvironment, error)
	// DeleteEnvironment は、チーム環境の削除をプロバイダーに依頼する。削除の完了は待たない。
	// チーム環境がなければ [ErrNotFound] を、チームのベンチが待機中か実行中なら [UseCaseError] を返す。
	DeleteEnvironment(ctx context.Context, teamID uuid.UUID) error
	// UpdateEnvironment は、チーム環境のすべてのサーバーを停止または起動する。
	// 一部のサーバーで失敗しても残りのサーバーの操作は続け、失敗をまとめて返す。
	// チーム環境がなければ [ErrNotFound] を、ready でなければ [UseCaseError] を返す。
	UpdateEnvironment(ctx context.Context, teamID uuid.UUID, op domain.InstanceOperation) error
}

// TeamEnvironment は、チームに見せるチーム環境である。
type TeamEnvironment struct {
	domain.Environment
	// BenchTargetID は、ベンチの対象が問題で固定されているときの、対象サーバーの ID である。
	BenchTargetID optional.Of[uuid.UUID]
}

type environmentUseCaseImpl struct {
	repo          repository.Repository
	syncer        environmentSyncer
	envManager    instance.EnvironmentManager
	manager       instance.Manager
	githubService github.Service
	benchTarget   string
}

func NewEnvironmentUseCase(
	repo repository.Repository,
	envManager instance.EnvironmentManager,
	manager instance.Manager,
	githubService github.Service,
	benchTarget string,
) EnvironmentUseCase {
	return &environmentUseCaseImpl{
		repo:          repo,
		syncer:        environmentSyncer{repo: repo, envManager: envManager, manager: manager},
		envManager:    envManager,
		manager:       manager,
		githubService: githubService,
		benchTarget:   benchTarget,
	}
}

func (u *environmentUseCaseImpl) GetTeamEnvironment(ctx context.Context, teamID uuid.UUID) (TeamEnvironment, error) {
	env, err := u.findTeamEnvironment(ctx, teamID)
	if err != nil {
		return TeamEnvironment{}, err
	}
	return u.toTeamEnvironment(env), nil
}

func (u *environmentUseCaseImpl) CreateEnvironment(ctx context.Context, teamID uuid.UUID) (TeamEnvironment, error) {
	_, err := u.findTeamEnvironment(ctx, teamID)
	if err == nil {
		return TeamEnvironment{}, NewUseCaseErrorFromMsg("team already has an environment")
	}
	if !errors.Is(err, ErrNotFound) {
		return TeamEnvironment{}, err
	}

	team, err := u.repo.FindTeam(ctx, teamID)
	if err != nil {
		return TeamEnvironment{}, fmt.Errorf("find team: %w", err)
	}
	sshKeys, err := u.githubService.GetSSHKeys(ctx, team.GitHubIDs)
	if err != nil {
		if github.IsNotFound(err) {
			return TeamEnvironment{}, NewUseCaseError(err)
		}
		return TeamEnvironment{}, fmt.Errorf("get SSH keys from GitHub: %w", err)
	}

	env := domain.NewEnvironment(teamID)
	env.ProviderEnvironmentID, err = u.envManager.Create(ctx, environmentName(env.ID), instance.EnvironmentParams{
		TeamID:        teamID.String(),
		SSHPublicKeys: sshKeys,
	})
	if err != nil {
		return TeamEnvironment{}, fmt.Errorf("create infra environment: %w", err)
	}

	// 一意制約が、同時に作られた 2 つ目のチーム環境を弾く。記録できなかったチーム環境は、プロバイダーからも消す。
	err = u.repo.CreateEnvironment(ctx, env)
	if err != nil {
		_ = u.envManager.Delete(ctx, env.ProviderEnvironmentID)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return TeamEnvironment{}, NewUseCaseErrorFromMsg("team already has an environment")
		}
		return TeamEnvironment{}, fmt.Errorf("create environment: %w", err)
	}

	synced, err := u.syncer.sync(ctx, env)
	if err != nil {
		return TeamEnvironment{}, err
	}
	return u.toTeamEnvironment(synced), nil
}

func (u *environmentUseCaseImpl) DeleteEnvironment(ctx context.Context, teamID uuid.UUID) error {
	env, err := u.repo.FindTeamEnvironment(ctx, teamID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find team environment: %w", err)
	}

	// 対象サーバーが消えると、runner のジョブがエラーになる。
	benchmarks, err := u.repo.GetBenchmarks(ctx, repository.BenchmarkQuery{
		TeamID: optional.From(teamID),
		StatusIn: optional.From([]domain.BenchmarkStatus{
			domain.BenchmarkStatusWaiting, domain.BenchmarkStatusReadying, domain.BenchmarkStatusRunning,
		}),
	})
	if err != nil {
		return fmt.Errorf("get benchmarks: %w", err)
	}
	if len(benchmarks) > 0 {
		return NewUseCaseErrorFromMsg("cannot delete the environment while a benchmark is waiting or running")
	}

	if err := u.envManager.Delete(ctx, env.ProviderEnvironmentID); err != nil {
		return fmt.Errorf("delete infra environment: %w", err)
	}
	return nil
}

func (u *environmentUseCaseImpl) UpdateEnvironment(ctx context.Context, teamID uuid.UUID, op domain.InstanceOperation) error {
	env, err := u.findTeamEnvironment(ctx, teamID)
	if err != nil {
		return err
	}
	if env.Infra.Status != domain.EnvironmentStatusReady {
		return NewUseCaseErrorFromMsg("environment is not ready")
	}

	var errs []error
	for _, inst := range env.Instances {
		switch op {
		case domain.InstanceOperationStart:
			err = u.manager.Start(ctx, inst.Infra)
		case domain.InstanceOperationStop:
			err = u.manager.Stop(ctx, inst.Infra)
		default:
			return NewUseCaseErrorFromMsg("invalid operation")
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("server %s: %w", inst.Name, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("update servers: %w", err)
	}
	return nil
}

// findTeamEnvironment は、チームの削除されていないチーム環境を同期してから返す。
// 同期の結果、チーム環境が削除済みになったら [ErrNotFound] を返す。
func (u *environmentUseCaseImpl) findTeamEnvironment(ctx context.Context, teamID uuid.UUID) (domain.Environment, error) {
	env, err := u.repo.FindTeamEnvironment(ctx, teamID)
	if errors.Is(err, repository.ErrNotFound) {
		return domain.Environment{}, ErrNotFound
	}
	if err != nil {
		return domain.Environment{}, fmt.Errorf("find team environment: %w", err)
	}
	env, err = u.syncer.sync(ctx, env)
	if err != nil {
		return domain.Environment{}, err
	}
	if env.DeletedAt != nil {
		return domain.Environment{}, ErrNotFound
	}
	return env, nil
}

func (u *environmentUseCaseImpl) toTeamEnvironment(env domain.Environment) TeamEnvironment {
	te := TeamEnvironment{Environment: env}
	if target, ok := env.BenchTarget(u.benchTarget); ok {
		te.BenchTargetID = optional.From(target.ID)
	}
	return te
}

// environmentName は、プロバイダーに渡すチーム環境の名前である。CloudFormation のスタック名の規則を満たす。
func environmentName(id uuid.UUID) string {
	return "piscon-" + id.String()
}

// environmentSyncer は、チーム環境の DB の記録をプロバイダーの状態に合わせる。
type environmentSyncer struct {
	repo       repository.Repository
	envManager instance.EnvironmentManager
	manager    instance.Manager
}

// sync は、env の状態をプロバイダーから読み、DB の記録を合わせてから、サーバーの状態を入れて返す。
// 何度呼んでも、記録は 1 回呼んだときと同じになる。
func (s environmentSyncer) sync(ctx context.Context, env domain.Environment) (domain.Environment, error) {
	if env.DeletedAt != nil {
		env.Infra = domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted}
		return s.withInfraInstances(ctx, env)
	}

	infra, err := s.envManager.Get(ctx, env.ProviderEnvironmentID)
	if err != nil {
		return domain.Environment{}, fmt.Errorf("get infra environment: %w", err)
	}

	switch {
	case infra.Status == domain.EnvironmentStatusReady && len(env.Instances) == 0:
		if err := s.repo.CreateInstances(ctx, env.NewInstances(infra.Servers)); err != nil {
			return domain.Environment{}, fmt.Errorf("create instances: %w", err)
		}
	case infra.Status == domain.EnvironmentStatusDeleted:
		if err := s.repo.DeleteEnvironment(ctx, env.ID); err != nil {
			return domain.Environment{}, fmt.Errorf("delete environment: %w", err)
		}
	default:
		env.Infra = infra
		return s.withInfraInstances(ctx, env)
	}

	env, err = s.repo.FindEnvironment(ctx, env.ID)
	if err != nil {
		return domain.Environment{}, fmt.Errorf("find environment: %w", err)
	}
	env.Infra = infra
	return s.withInfraInstances(ctx, env)
}

func (s environmentSyncer) withInfraInstances(ctx context.Context, env domain.Environment) (domain.Environment, error) {
	instances, err := withInfraInstances(ctx, s.manager, env.Instances)
	if err != nil {
		return domain.Environment{}, err
	}
	env.Instances = instances
	return env, nil
}
