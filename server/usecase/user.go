package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
)

type UserUseCase interface {
	GetUser(ctx context.Context, userID uuid.UUID) (domain.User, error)
	GetUsers(ctx context.Context) ([]domain.User, error)
	// Login は、外部アカウントに対応するユーザーを返す。対応するユーザーがいなければ作り、それが最初のユーザーなら admin にする。
	// 外部アカウントの名前の初期値が他のユーザーに使われていれば、[UseCaseError] を返す。
	Login(ctx context.Context, identity domain.Identity) (domain.User, error)
}

type userUseCaseImpl struct {
	repo repository.Repository
}

func NewUserUseCase(repo repository.Repository) UserUseCase {
	return &userUseCaseImpl{
		repo: repo,
	}
}

func (u *userUseCaseImpl) GetUser(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	user, err := u.repo.FindUser(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (u *userUseCaseImpl) GetUsers(ctx context.Context) ([]domain.User, error) {
	users, err := u.repo.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}

	return users, nil
}

func (u *userUseCaseImpl) Login(ctx context.Context, identity domain.Identity) (domain.User, error) {
	var user domain.User
	err := u.repo.Transaction(ctx, func(ctx context.Context) error {
		found, err := u.repo.FindUserByIdentity(ctx, identity.Provider, identity.Subject)
		if err == nil {
			user = found
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("find user by identity: %w", err)
		}

		count, err := u.repo.CountUsers(ctx)
		if err != nil {
			return fmt.Errorf("count users: %w", err)
		}
		user = domain.NewUser(identity)
		user.IsAdmin = count == 0

		err = u.repo.CreateUser(ctx, user)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return NewUseCaseError(fmt.Errorf("user name %q is already used", user.Name))
		}
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("transaction: %w", err)
	}
	return user, nil
}
