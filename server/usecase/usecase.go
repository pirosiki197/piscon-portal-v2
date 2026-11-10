package usecase

import (
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/services/github"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
)

//go:generate go tool mockgen -source=$GOFILE -destination=mock/$GOFILE -package=mock -typed=true
type UseCase interface {
	TeamUseCase
	UserUseCase
	BenchmarkUseCase
	ScoreUseCase
	AdminUseCase
	DocumentUseCase
	InstanceUseCase
	EnvironmentUseCase
}

type useCaseImpl struct {
	TeamUseCase
	UserUseCase
	BenchmarkUseCase
	ScoreUseCase
	AdminUseCase
	DocumentUseCase
	InstanceUseCase
	EnvironmentUseCase
}

type Config struct {
	// BenchTarget は、ベンチの対象に固定するサーバーの名前である。空なら、チームが対象を選ぶ。
	BenchTarget string
}

func New(
	config Config,
	repo repository.Repository,
	instanceManager instance.Manager,
	envManager instance.EnvironmentManager,
	githubService github.Service,
) UseCase {
	return &useCaseImpl{
		TeamUseCase:        NewTeamUseCase(repo),
		UserUseCase:        NewUserUseCase(repo),
		BenchmarkUseCase:   NewBenchmarkUseCase(repo, envManager, instanceManager, config.BenchTarget),
		ScoreUseCase:       NewScoreUseCase(repo),
		AdminUseCase:       NewAdminUseCase(repo),
		DocumentUseCase:    NewDocumentUseCase(repo),
		InstanceUseCase:    NewInstanceUseCase(repo, instanceManager),
		EnvironmentUseCase: NewEnvironmentUseCase(repo, envManager, instanceManager, githubService, config.BenchTarget),
	}
}
