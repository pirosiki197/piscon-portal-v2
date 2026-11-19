package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stephenafamo/bob"
	bobmysql "github.com/stephenafamo/bob/dialect/mysql"
	"github.com/stephenafamo/bob/dialect/mysql/sm"
	"github.com/stephenafamo/bob/dialect/mysql/um"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/repository/db/dbinfo"
	"github.com/traPtitech/piscon-portal-v2/server/repository/db/models"
)

func (r *Repository) FindUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return findUser(ctx, r.executor(ctx), id.String())
}

func (r *Repository) FindUserByIdentity(ctx context.Context, provider, subject string) (domain.User, error) {
	user, err := models.Users.Query(
		models.SelectWhere.Users.Provider.EQ(provider),
		models.SelectWhere.Users.Subject.EQ(subject),
	).One(ctx, r.executor(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, repository.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user by identity: %w", err)
	}
	return toDomainUser(user)
}

func (r *Repository) CountUsers(ctx context.Context) (int64, error) {
	count, err := models.Users.Query().Count(ctx, r.executor(ctx))
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (r *Repository) CreateUser(ctx context.Context, user domain.User) error {
	return createUser(ctx, r.executor(ctx), user)
}

func (r *Repository) GetUsers(ctx context.Context) ([]domain.User, error) {
	users, err := models.Users.Query().All(ctx, r.executor(ctx))
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}

	res := make([]domain.User, 0, len(users))
	for _, user := range users {
		domainUser, err := toDomainUser(user)
		if err != nil {
			return nil, fmt.Errorf("convert user: %w", err)
		}
		res = append(res, domainUser)
	}

	return res, nil
}

func findUser(ctx context.Context, executor bob.Executor, id string) (domain.User, error) {
	user, err := models.FindUser(ctx, executor, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, repository.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("find user: %w", err)
	}

	return toDomainUser(user)
}

func createUser(ctx context.Context, executor bob.Executor, user domain.User) error {
	_, err := models.Users.Insert(&models.UserSetter{
		ID:       lo.ToPtr(user.ID.String()),
		Name:     lo.ToPtr(user.Name),
		Provider: lo.ToPtr(user.Provider),
		Subject:  lo.ToPtr(user.Subject),
		IconURL:  lo.ToPtr(sql.Null[string]{V: user.IconURL, Valid: user.IconURL != ""}),
		IsAdmin:  &user.IsAdmin,
	}).Exec(ctx, executor)
	if mysqlErr, ok := errors.AsType[*mysql.MySQLError](err); ok && mysqlErr.Number == mysqlErrDuplicateEntry {
		return repository.ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *Repository) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.User, error) {
	if len(ids) == 0 {
		return []domain.User{}, nil
	}

	anyIDs := make([]any, len(ids))
	for i, id := range ids {
		anyIDs[i] = id.String()
	}
	users, err := models.Users.Query(
		sm.Where(models.Users.Columns.ID.In(bobmysql.Arg(anyIDs...))),
	).All(ctx, r.executor(ctx))
	if err != nil {
		return nil, fmt.Errorf("get users by ids: %w", err)
	}
	res := make([]domain.User, 0, len(users))
	for _, user := range users {
		domainUser, err := toDomainUser(user)
		if err != nil {
			return nil, fmt.Errorf("convert user: %w", err)
		}
		res = append(res, domainUser)
	}

	return res, nil
}

func (r *Repository) GetAdmins(ctx context.Context) ([]domain.User, error) {
	adminUsers, err := models.Users.Query(
		sm.Where(models.Users.Columns.IsAdmin.EQ(bobmysql.Arg(true))),
	).All(ctx, r.executor(ctx))
	if err != nil {
		return nil, fmt.Errorf("get admins: %w", err)
	}

	res := make([]domain.User, 0, len(adminUsers))
	for _, user := range adminUsers {
		domainUser, err := toDomainUser(user)
		if err != nil {
			return nil, fmt.Errorf("convert user: %w", err)
		}
		res = append(res, domainUser)
	}

	return res, nil
}

func (r *Repository) AddAdmins(ctx context.Context, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}

	anyIDs := make([]any, len(userIDs))
	for i, id := range userIDs {
		anyIDs[i] = id.String()
	}
	_, err := models.Users.Update(
		um.SetCol(dbinfo.Users.Columns.IsAdmin.Name).To(true),
		um.Where(models.Users.Columns.ID.In(bobmysql.Arg(anyIDs...))),
	).Exec(ctx, r.executor(ctx))
	if err != nil {
		return fmt.Errorf("add admins: %w", err)
	}

	return nil
}

func (r *Repository) DeleteAdmins(ctx context.Context, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}

	anyIDs := make([]any, len(userIDs))
	for i, id := range userIDs {
		anyIDs[i] = id.String()
	}
	_, err := models.Users.Update(
		um.SetCol(dbinfo.Users.Columns.IsAdmin.Name).To(false),
		um.Where(models.Users.Columns.ID.In(bobmysql.Arg(anyIDs...))),
	).Exec(ctx, r.executor(ctx))
	if err != nil {
		return fmt.Errorf("delete admins: %w", err)
	}

	return nil
}

func toDomainUser(user *models.User) (domain.User, error) {
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		return domain.User{}, fmt.Errorf("parse user ID: %w", err)
	}

	var teamID uuid.NullUUID
	if user.TeamID.Valid {
		parsedID, err := uuid.Parse(user.TeamID.V)
		if err != nil {
			return domain.User{}, fmt.Errorf("parse team ID: %w", err)
		}
		teamID = uuid.NullUUID{
			UUID:  parsedID,
			Valid: true,
		}
	}

	return domain.User{
		ID:       userID,
		Name:     user.Name,
		Provider: user.Provider,
		Subject:  user.Subject,
		IconURL:  user.IconURL.V,
		IsAdmin:  user.IsAdmin,
		TeamID:   teamID,
	}, nil
}
