package incus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
)

func TestEnvironmentStatus(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time {
		t := now.Add(-d)
		return &t
	}
	reason := "ansible failed"
	server := func(name string, mod func(*serverView)) serverView {
		s := serverView{Server: name, Container: "env-" + name, Exists: true, CreatedAt: now.Add(-time.Minute)}
		mod(&s)
		return s
	}
	ready := func(s *serverView) { s.Ready = true }
	missing := func(s *serverView) { s.Exists = false }
	same := func(*serverView) {}

	tests := map[string]struct {
		servers []serverView
		want    domain.InfraEnvironment
	}{
		"コンテナが 1 つもなければ deleted": {
			servers: []serverView{server("Server1", missing), server("Server2", missing)},
			want:    domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted},
		},
		"削除の依頼から 5 分を過ぎて残っていれば deleteFailed": {
			servers: []serverView{server("Server1", func(s *serverView) { s.DeletingSince = ago(6 * time.Minute) }), server("Server2", missing)},
			want: domain.InfraEnvironment{
				Status:        domain.EnvironmentStatusDeleteFailed,
				FailureReason: "container env-Server1 was not deleted within 5m0s",
			},
		},
		"削除を依頼していれば、準備が終わっていても deleting": {
			servers: []serverView{server("Server1", func(s *serverView) { s.Ready = true; s.DeletingSince = ago(time.Minute) }), server("Server2", ready)},
			want:    domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleting},
		},
		"失敗の印があれば、ほかのサーバーの準備が終わっていても failed": {
			servers: []serverView{server("Server1", ready), server("Server2", func(s *serverView) { s.FailureReason = &reason })},
			want:    domain.InfraEnvironment{Status: domain.EnvironmentStatusFailed, FailureReason: "Server2: ansible failed"},
		},
		"全サーバーの準備が終われば ready": {
			servers: []serverView{server("Server1", ready), server("Server2", ready)},
			want: domain.InfraEnvironment{Status: domain.EnvironmentStatusReady, Servers: []domain.InfraServer{
				{Name: "Server1", ProviderInstanceID: "env-Server1"},
				{Name: "Server2", ProviderInstanceID: "env-Server2"},
			}},
		},
		// 作成の途中でポータルが落ちると、一部のコンテナだけが残る。これを ready として記録しない。
		"一部のコンテナがなければ、残りの準備が終わっていても creating": {
			servers: []serverView{server("Server1", ready), server("Server2", missing)},
			want:    domain.InfraEnvironment{Status: domain.EnvironmentStatusCreating},
		},
		"準備中なら creating": {
			servers: []serverView{server("Server1", ready), server("Server2", same)},
			want:    domain.InfraEnvironment{Status: domain.EnvironmentStatusCreating},
		},
		"最初のコンテナを作ってから 15 分を過ぎても準備が終わらなければ failed": {
			servers: []serverView{
				server("Server1", func(s *serverView) { s.CreatedAt = now.Add(-16 * time.Minute) }),
				server("Server2", ready),
			},
			want: domain.InfraEnvironment{Status: domain.EnvironmentStatusFailed, FailureReason: "servers were not ready within 15m0s"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, environmentStatus(tt.servers, now))
		})
	}
}
