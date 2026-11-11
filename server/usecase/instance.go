package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
)

type InstanceUseCase interface {
	// GetTeamInstances returns all instances for the given team. Deleted instances are included for historical references.
	GetTeamInstances(ctx context.Context, teamID uuid.UUID) ([]domain.Instance, error)
	// GetAllInstances returns all instances. Deleted instances are included for historical references.
	GetAllInstances(ctx context.Context) ([]domain.Instance, error)
}

type InstanceUseCaseImpl struct {
	repo    repository.Repository
	manager instance.Manager
}

func NewInstanceUseCase(repo repository.Repository, manager instance.Manager) InstanceUseCase {
	return &InstanceUseCaseImpl{
		repo:    repo,
		manager: manager,
	}
}

func (i *InstanceUseCaseImpl) GetTeamInstances(ctx context.Context, teamID uuid.UUID) ([]domain.Instance, error) {
	instances, err := i.repo.GetTeamInstances(ctx, teamID, true)
	if err != nil {
		return nil, fmt.Errorf("get team instances: %w", err)
	}

	return withInfraInstances(ctx, i.manager, instances)
}

func (i *InstanceUseCaseImpl) GetAllInstances(ctx context.Context) ([]domain.Instance, error) {
	instances, err := i.repo.GetAllInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all instances: %w", err)
	}
	return withInfraInstances(ctx, i.manager, instances)
}

// withInfraInstances は、instances のそれぞれにプロバイダーから読んだ状態を入れた、新しいスライスを返す。
// 削除済みのサーバーはプロバイダーに問い合わせず、deleted とする。
func withInfraInstances(ctx context.Context, manager instance.Manager, instances []domain.Instance) ([]domain.Instance, error) {
	ids := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst.DeletedAt == nil {
			ids = append(ids, inst.Infra.ProviderInstanceID)
		}
	}
	if len(ids) == 0 {
		return setInfraInstances(instances, nil), nil
	}
	infraInstances, err := manager.GetByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get infra instances: %w", err)
	}
	return setInfraInstances(instances, infraInstances), nil
}

// setInfraInstances は、instances のそれぞれにプロバイダーから読んだ状態を入れた、新しいスライスを返す。
// infraInstances にないサーバーは、削除済みとする。
func setInfraInstances(instances []domain.Instance, infraInstances []domain.InfraInstance) []domain.Instance {
	infraInstancesMap := make(map[string]domain.InfraInstance, len(infraInstances))
	for _, infra := range infraInstances {
		infraInstancesMap[infra.ProviderInstanceID] = infra
	}

	result := make([]domain.Instance, 0, len(instances))
	for _, inst := range instances {
		infra, ok := infraInstancesMap[inst.Infra.ProviderInstanceID]
		if !ok {
			infra = domain.InfraInstance{
				ProviderInstanceID: inst.Infra.ProviderInstanceID,
				Status:             domain.InstanceStatusDeleted,
			}
		}
		inst.Infra = infra
		result = append(result, inst)
	}
	return result
}
