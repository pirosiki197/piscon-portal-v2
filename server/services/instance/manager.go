package instance

import (
	"context"
	"errors"

	"github.com/traPtitech/piscon-portal-v2/server/domain"
)

//go:generate go tool mockgen -source=$GOFILE -destination=mock/$GOFILE -package=mock -typed=true

// Manager is an interface for managing infrastructure instances.
type Manager interface {
	// GetByIDs retrieves infrastructure instances by their provider instance IDs.
	// Instances that no longer exist are not included.
	GetByIDs(ctx context.Context, ids []string) ([]domain.InfraInstance, error)
	// Stop stops the infrastructure instance with the given provider ID.
	// Returns an error if the stop operation fails.
	Stop(ctx context.Context, instance domain.InfraInstance) error
	// Start starts the infrastructure instance with the given provider ID.
	// Returns an error if the start operation fails.
	Start(ctx context.Context, instance domain.InfraInstance) error
}

// ErrInvalidEnvironmentParams は、プロバイダーが受け付けない [EnvironmentParams] を表す。
var ErrInvalidEnvironmentParams = errors.New("invalid environment params")

// EnvironmentManager は、プロバイダーにチーム環境の作成と削除を依頼し、その状態を読む。
type EnvironmentManager interface {
	// Create は、name という名前でチーム環境の作成を依頼し、プロバイダーがチーム環境を識別する値を返す。
	// 作成の完了は待たない。params をプロバイダーが受け付けなければ、[ErrInvalidEnvironmentParams] を包んだエラーを返す。
	Create(ctx context.Context, name string, params EnvironmentParams) (string, error)
	// Get は、チーム環境の状態を返す。プロバイダーにチーム環境がなければ、状態は deleted になる。
	Get(ctx context.Context, providerEnvironmentID string) (domain.InfraEnvironment, error)
	// Delete は、チーム環境の削除を依頼する。削除の完了は待たない。
	Delete(ctx context.Context, providerEnvironmentID string) error
}

// EnvironmentParams は、チーム環境ごとに変わる、テンプレートへの入力である。
type EnvironmentParams struct {
	TeamID        string
	SSHPublicKeys []string
}
