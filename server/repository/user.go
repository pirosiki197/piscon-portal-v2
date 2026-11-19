package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
)

type UserRepository interface {
	// FindUser finds a user by id. If the user is not found, it returns [ErrNotFound].
	FindUser(ctx context.Context, id uuid.UUID) (domain.User, error)
	// FindUserByIdentity は、外部アカウントの認証手段と subject でユーザーを探す。見つからなければ [ErrNotFound] を返す。
	FindUserByIdentity(ctx context.Context, provider, subject string) (domain.User, error)
	// GetUsers returns all users.
	GetUsers(ctx context.Context) ([]domain.User, error)
	// CountUsers は、ユーザーの数を返す。
	CountUsers(ctx context.Context) (int64, error)
	// CreateUser は、ユーザーを作る。名前か、認証手段と subject の組が既存のユーザーと重なれば [ErrAlreadyExists] を返す。
	CreateUser(ctx context.Context, user domain.User) error
	// GetUsersByIDs returns users by ids.
	// If the user is not found, it returns an empty slice.
	// If the ids is empty, it returns an empty slice.
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.User, error)
	// GetAdmins returns all admin users.
	GetAdmins(ctx context.Context) ([]domain.User, error)
	// AddAdmins updates admin users.
	// If userIDs contains an id that does not exist, it is ignored.
	// If userIDs is empty, it does nothing.
	// If userIDs contains an id that is already an admin, it is ignored.
	AddAdmins(ctx context.Context, userIDs []uuid.UUID) error
	// DeleteAdmins deletes admin users.
	// If userIDs contains an id that does not exist, it is ignored.
	// If userIDs is empty, it does nothing.
	// If userIDs contains an id that is not an admin, it is ignored.
	DeleteAdmins(ctx context.Context, userIDs []uuid.UUID) error
}
