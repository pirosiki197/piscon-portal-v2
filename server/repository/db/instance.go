package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/mysql/dialect"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/repository/db/models"
)

func (r *Repository) FindInstance(ctx context.Context, id uuid.UUID) (domain.Instance, error) {
	instance, err := models.FindInstance(ctx, r.executor(ctx), id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Instance{}, repository.ErrNotFound
		}
		return domain.Instance{}, fmt.Errorf("find instance: %w", err)
	}

	return toDomainInstance(instance)
}

func (r *Repository) GetTeamInstances(ctx context.Context, teamID uuid.UUID, includeDeleted bool) ([]domain.Instance, error) {
	conditions := []bob.Mod[*dialect.SelectQuery]{
		models.SelectWhere.Instances.TeamID.EQ(teamID.String()),
	}
	if !includeDeleted {
		conditions = append(conditions, models.SelectWhere.Instances.DeletedAt.IsNull())
	}

	instances, err := models.Instances.Query(conditions...).All(ctx, r.executor(ctx))
	if err != nil {
		return nil, fmt.Errorf("get team instances: %w", err)
	}
	return toDomainInstances(instances)
}

func toDomainInstances(instances models.InstanceSlice) ([]domain.Instance, error) {
	result := make([]domain.Instance, 0, len(instances))
	for _, inst := range instances {
		domainInst, err := toDomainInstance(inst)
		if err != nil {
			return nil, err
		}
		result = append(result, domainInst)
	}
	return result, nil
}

func toDomainInstance(instance *models.Instance) (domain.Instance, error) {
	id, err := uuid.Parse(instance.ID)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("parse instance id: %w", err)
	}
	teamID, err := uuid.Parse(instance.TeamID)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("parse team id: %w", err)
	}
	environmentID, err := uuid.Parse(instance.EnvironmentID)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("parse environment id: %w", err)
	}

	return domain.Instance{
		ID:            id,
		TeamID:        teamID,
		EnvironmentID: environmentID,
		Name:          instance.Name,
		Infra: domain.InfraInstance{
			ProviderInstanceID: instance.ProviderInstanceID,
		},
		CreatedAt: instance.CreatedAt,
		DeletedAt: SQLNullToPtr(instance.DeletedAt),
	}, nil
}

func buildInstanceSetter(instance domain.Instance) *models.InstanceSetter {
	return &models.InstanceSetter{
		ID:                 lo.ToPtr(instance.ID.String()),
		ProviderInstanceID: lo.ToPtr(instance.Infra.ProviderInstanceID),
		TeamID:             lo.ToPtr(instance.TeamID.String()),
		EnvironmentID:      lo.ToPtr(instance.EnvironmentID.String()),
		Name:               lo.ToPtr(instance.Name),
		CreatedAt:          lo.Ternary(!instance.CreatedAt.IsZero(), &instance.CreatedAt, nil),
	}
}
