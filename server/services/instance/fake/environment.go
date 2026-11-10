package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
)

const (
	environmentFileName = "environments.json"
	// environmentDelay は、作成と削除にかかる時間である。
	environmentDelay = 5 * time.Second
)

type environmentRecord struct {
	Name              string               `json:"name"`
	Servers           []domain.InfraServer `json:"servers"`
	CreatedAt         time.Time            `json:"created_at"`
	DeleteRequestedAt *time.Time           `json:"delete_requested_at,omitempty"`
}

// EnvironmentManager は、instance.EnvironmentManager の fake の実装である。
// 作成を依頼すると、決まった名前のサーバーを fake のインスタンスとして作る。
// 状態は、作成と削除を依頼した時刻からの経過時間で決まるので、ポータルを再起動しても進み続ける。
type EnvironmentManager struct {
	mu        sync.Mutex
	root      *os.Root
	instances *Manager
	servers   []string
}

var _ instance.EnvironmentManager = &EnvironmentManager{}

// NewEnvironmentManager は、servers の名前のサーバーを持つチーム環境を作る fake を返す。
// サーバーは instances に作る。
func NewEnvironmentManager(root *os.Root, instances *Manager, servers []string) (*EnvironmentManager, error) {
	m := &EnvironmentManager{root: root, instances: instances, servers: servers}
	if _, err := root.Stat(environmentFileName); errors.Is(err, os.ErrNotExist) {
		if err := m.store(map[string]environmentRecord{}); err != nil {
			return nil, fmt.Errorf("initialize environments file: %w", err)
		}
	}
	return m, nil
}

func (m *EnvironmentManager) Create(_ context.Context, name string, _ instance.EnvironmentParams) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, err := m.read()
	if err != nil {
		return "", err
	}
	if _, ok := records[name]; ok {
		return "", fmt.Errorf("environment %s already exists", name)
	}

	servers := make([]domain.InfraServer, 0, len(m.servers))
	for _, s := range m.servers {
		id, err := m.instances.createInstance()
		if err != nil {
			return "", fmt.Errorf("create instance: %w", err)
		}
		servers = append(servers, domain.InfraServer{Name: s, ProviderInstanceID: id})
	}
	records[name] = environmentRecord{Name: name, Servers: servers, CreatedAt: time.Now()}
	if err := m.store(records); err != nil {
		return "", err
	}
	return name, nil
}

func (m *EnvironmentManager) Get(_ context.Context, name string) (domain.InfraEnvironment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, err := m.read()
	if err != nil {
		return domain.InfraEnvironment{}, err
	}
	r, ok := records[name]
	if !ok {
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted}, nil
	}
	now := time.Now()
	switch {
	case r.DeleteRequestedAt != nil && now.Sub(*r.DeleteRequestedAt) < environmentDelay:
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleting}, nil
	case r.DeleteRequestedAt != nil:
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted}, nil
	case now.Sub(r.CreatedAt) < environmentDelay:
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusCreating}, nil
	default:
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusReady, Servers: r.Servers}, nil
	}
}

// Delete は、チーム環境のサーバーをすぐに消し、チーム環境をしばらく deleting にする。
func (m *EnvironmentManager) Delete(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, err := m.read()
	if err != nil {
		return err
	}
	r, ok := records[name]
	if !ok || r.DeleteRequestedAt != nil {
		return nil
	}
	for _, s := range r.Servers {
		if err := m.instances.deleteInstance(s.ProviderInstanceID); err != nil {
			return fmt.Errorf("delete instance: %w", err)
		}
	}
	now := time.Now()
	r.DeleteRequestedAt = &now
	records[name] = r
	return m.store(records)
}

func (m *EnvironmentManager) read() (map[string]environmentRecord, error) {
	data, err := m.root.ReadFile(environmentFileName)
	if err != nil {
		return nil, fmt.Errorf("read environments file: %w", err)
	}
	var records map[string]environmentRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode environments: %w", err)
	}
	return records, nil
}

func (m *EnvironmentManager) store(records map[string]environmentRecord) error {
	data, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("encode environments: %w", err)
	}
	if err := m.root.WriteFile(environmentFileName, data, 0o644); err != nil {
		return fmt.Errorf("write environments file: %w", err)
	}
	return nil
}
