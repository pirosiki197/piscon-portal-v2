package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stephenafamo/bob"
	bobmysql "github.com/stephenafamo/bob/dialect/mysql"
	"github.com/stephenafamo/bob/dialect/mysql/dialect"
	"github.com/stephenafamo/bob/dialect/mysql/im"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/repository/db/models"
)

const mysqlErrDuplicateEntry = 1062

func (r *Repository) CreateEnvironment(ctx context.Context, env domain.Environment) error {
	_, err := models.Environments.Insert(&models.EnvironmentSetter{
		ID:                    lo.ToPtr(env.ID.String()),
		TeamID:                lo.ToPtr(env.TeamID.String()),
		ProviderEnvironmentID: lo.ToPtr(env.ProviderEnvironmentID),
		CreatedAt:             lo.Ternary(!env.CreatedAt.IsZero(), &env.CreatedAt, nil),
	}).Exec(ctx, r.executor(ctx))
	if mysqlErr, ok := errors.AsType[*mysql.MySQLError](err); ok && mysqlErr.Number == mysqlErrDuplicateEntry {
		return repository.ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("insert environment: %w", err)
	}
	return nil
}

func (r *Repository) FindEnvironment(ctx context.Context, id uuid.UUID) (domain.Environment, error) {
	return r.findEnvironment(ctx, models.SelectWhere.Environments.ID.EQ(id.String()))
}

func (r *Repository) FindTeamEnvironment(ctx context.Context, teamID uuid.UUID) (domain.Environment, error) {
	return r.findEnvironment(ctx,
		models.SelectWhere.Environments.TeamID.EQ(teamID.String()),
		models.SelectWhere.Environments.DeletedAt.IsNull(),
	)
}

func (r *Repository) findEnvironment(ctx context.Context, mods ...bob.Mod[*dialect.SelectQuery]) (domain.Environment, error) {
	env, err := models.Environments.Query(mods...).One(ctx, r.executor(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Environment{}, repository.ErrNotFound
	}
	if err != nil {
		return domain.Environment{}, fmt.Errorf("find environment: %w", err)
	}
	envs, err := r.withInstances(ctx, models.EnvironmentSlice{env})
	if err != nil {
		return domain.Environment{}, err
	}
	return envs[0], nil
}

// withInstances は、チーム環境の行をドメインの型に変え、それぞれのサーバーを名前の順に入れる。
func (r *Repository) withInstances(ctx context.Context, envs models.EnvironmentSlice) ([]domain.Environment, error) {
	if len(envs) == 0 {
		return []domain.Environment{}, nil
	}
	ids := lo.Map(envs, func(e *models.Environment, _ int) string { return e.ID })
	instances, err := models.Instances.Query(
		models.SelectWhere.Instances.EnvironmentID.In(ids...),
	).All(ctx, r.executor(ctx))
	if err != nil {
		return nil, fmt.Errorf("get environment instances: %w", err)
	}
	domainInstances, err := toDomainInstances(instances)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(domainInstances, func(a, b domain.Instance) int { return strings.Compare(a.Name, b.Name) })
	byEnv := lo.GroupBy(domainInstances, func(i domain.Instance) uuid.UUID { return i.EnvironmentID })

	result := make([]domain.Environment, 0, len(envs))
	for _, e := range envs {
		id, err := uuid.Parse(e.ID)
		if err != nil {
			return nil, fmt.Errorf("parse environment id: %w", err)
		}
		teamID, err := uuid.Parse(e.TeamID)
		if err != nil {
			return nil, fmt.Errorf("parse team id: %w", err)
		}
		result = append(result, domain.Environment{
			ID:                    id,
			TeamID:                teamID,
			ProviderEnvironmentID: e.ProviderEnvironmentID,
			Instances:             lo.CoalesceSliceOrEmpty(byEnv[id]),
			CreatedAt:             e.CreatedAt,
			DeletedAt:             SQLNullToPtr(e.DeletedAt),
		})
	}
	return result, nil
}

func (r *Repository) CreateInstances(ctx context.Context, instances []domain.Instance) error {
	if len(instances) == 0 {
		return nil
	}
	setters := lo.Map(instances, func(i domain.Instance, _ int) *models.InstanceSetter { return buildInstanceSetter(i) })
	// (environment_id, name) が重複した行は、何も変えずに残す。
	_, err := models.Instances.Insert(
		bob.ToMods(setters...),
		im.OnDuplicateKeyUpdate(im.Update(bobmysql.Raw("`id` = `id`"))),
	).Exec(ctx, r.executor(ctx))
	if err != nil {
		return fmt.Errorf("insert instances: %w", err)
	}
	return nil
}

func (r *Repository) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	now := ToSQLNull(time.Now())
	_, err := models.Environments.Update(
		models.UpdateWhere.Environments.ID.EQ(id.String()),
		models.UpdateWhere.Environments.DeletedAt.IsNull(),
		models.EnvironmentSetter{DeletedAt: now}.UpdateMod(),
	).Exec(ctx, r.executor(ctx))
	if err != nil {
		return fmt.Errorf("delete environment: %w", err)
	}
	_, err = models.Instances.Update(
		models.UpdateWhere.Instances.EnvironmentID.EQ(id.String()),
		models.UpdateWhere.Instances.DeletedAt.IsNull(),
		models.InstanceSetter{DeletedAt: now}.UpdateMod(),
	).Exec(ctx, r.executor(ctx))
	if err != nil {
		return fmt.Errorf("delete environment instances: %w", err)
	}
	return nil
}
