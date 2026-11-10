package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
)

// EnvironmentRepository は、チーム環境とそのサーバーを読み書きする。
// チーム環境を返すメソッドは、Instances に、そのチーム環境のサーバーを名前の順に入れて返す。Infra は空である。
type EnvironmentRepository interface {
	// CreateEnvironment は、チーム環境を記録する。
	// チームに削除されていないチーム環境がすでにあれば、[ErrAlreadyExists] を返す。
	CreateEnvironment(ctx context.Context, env domain.Environment) error
	// FindEnvironment は、削除済みも含めて ID でチーム環境を探す。見つからなければ [ErrNotFound] を返す。
	FindEnvironment(ctx context.Context, id uuid.UUID) (domain.Environment, error)
	// FindTeamEnvironment は、チームの削除されていないチーム環境を返す。なければ [ErrNotFound] を返す。
	FindTeamEnvironment(ctx context.Context, teamID uuid.UUID) (domain.Environment, error)
	// GetEnvironments は、削除されていないチーム環境をすべて返す。
	GetEnvironments(ctx context.Context) ([]domain.Environment, error)
	// CreateInstances は、チーム環境のサーバーを記録する。
	// 同じチーム環境に同じ名前のサーバーがすでにあれば、そのサーバーは記録せずに無視する。
	CreateInstances(ctx context.Context, instances []domain.Instance) error
	// DeleteEnvironment は、チーム環境とそのサーバーを削除済みにする。すでに削除済みなら何もしない。
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error
}
