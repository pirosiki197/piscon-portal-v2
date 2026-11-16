package incus

import (
	"fmt"
	"time"

	"github.com/traPtitech/piscon-portal-v2/server/domain"
)

const (
	// createTimeout は、最初のコンテナを作ってから、全サーバーの準備が終わるまでを待つ時間である。
	createTimeout = 15 * time.Minute
	// deleteTimeout は、削除を依頼してから、コンテナが消えるまでを待つ時間である。
	deleteTimeout = 5 * time.Minute
)

// serverView は、チーム環境の 1 台のサーバーについて、Incus から読んだ値である。
type serverView struct {
	// Server は、テンプレートでの論理 ID である。
	Server string
	// Container は、コンテナの名前である。
	Container string
	// Exists が false なら、ほかのフィールドは意味を持たない。
	Exists    bool
	CreatedAt time.Time
	// DeletingSince は、削除を依頼した時刻である。依頼していなければ nil である。
	DeletingSince *time.Time
	// Ready は、準備が終わった印のファイルがあるかである。
	Ready bool
	// FailureReason は、準備が失敗した印のファイルの中身である。印がなければ nil である。
	FailureReason *string
}

// environmentStatus は、テンプレートの全サーバーについて読んだ値から、チーム環境の状態を決める。
// 表の上の行から順に当てはめる。
func environmentStatus(servers []serverView, now time.Time) domain.InfraEnvironment {
	var existing []serverView
	for _, s := range servers {
		if s.Exists {
			existing = append(existing, s)
		}
	}
	if len(existing) == 0 {
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted}
	}

	for _, s := range existing {
		if s.DeletingSince != nil && now.Sub(*s.DeletingSince) > deleteTimeout {
			return domain.InfraEnvironment{
				Status:        domain.EnvironmentStatusDeleteFailed,
				FailureReason: fmt.Sprintf("container %s was not deleted within %s", s.Container, deleteTimeout),
			}
		}
	}
	for _, s := range existing {
		if s.DeletingSince != nil {
			return domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleting}
		}
	}
	for _, s := range existing {
		if s.FailureReason != nil {
			return domain.InfraEnvironment{
				Status:        domain.EnvironmentStatusFailed,
				FailureReason: fmt.Sprintf("%s: %s", s.Server, *s.FailureReason),
			}
		}
	}

	if len(existing) == len(servers) && allReady(existing) {
		infra := make([]domain.InfraServer, 0, len(servers))
		for _, s := range servers {
			infra = append(infra, domain.InfraServer{Name: s.Server, ProviderInstanceID: s.Container})
		}
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusReady, Servers: infra}
	}

	oldest := existing[0].CreatedAt
	for _, s := range existing[1:] {
		if s.CreatedAt.Before(oldest) {
			oldest = s.CreatedAt
		}
	}
	if now.Sub(oldest) > createTimeout {
		return domain.InfraEnvironment{
			Status:        domain.EnvironmentStatusFailed,
			FailureReason: fmt.Sprintf("servers were not ready within %s", createTimeout),
		}
	}
	return domain.InfraEnvironment{Status: domain.EnvironmentStatusCreating}
}

func allReady(servers []serverView) bool {
	for _, s := range servers {
		if !s.Ready {
			return false
		}
	}
	return true
}
