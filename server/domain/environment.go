package domain

import (
	"time"

	"github.com/google/uuid"
)

// Environment は、1 チームに割り当てたサーバー一式（チーム環境）である。
type Environment struct {
	ID     uuid.UUID
	TeamID uuid.UUID
	// ProviderEnvironmentID は、プロバイダーがチーム環境を識別する値（CloudFormation のスタック名）である。
	ProviderEnvironmentID string
	// Infra は、プロバイダーから読んだ状態である。DB には保存しない。
	Infra InfraEnvironment
	// Instances は、チーム環境のサーバーを名前の順に並べたものである。チーム環境が ready になるまでは空である。
	Instances []Instance
	CreatedAt time.Time
	DeletedAt *time.Time
}

// InfraEnvironment は、プロバイダーから読んだチーム環境の状態である。
type InfraEnvironment struct {
	Status EnvironmentStatus
	// Servers は、Status が ready のときだけ入る。
	Servers []InfraServer
	// FailureReason は、Status が failed か deleteFailed のときだけ入る。
	FailureReason string
}

// InfraServer は、プロバイダーが作ったサーバーである。
type InfraServer struct {
	Name               string
	ProviderInstanceID string
}

type EnvironmentStatus string

const (
	EnvironmentStatusCreating     EnvironmentStatus = "creating"
	EnvironmentStatusReady        EnvironmentStatus = "ready"
	EnvironmentStatusFailed       EnvironmentStatus = "failed"
	EnvironmentStatusDeleting     EnvironmentStatus = "deleting"
	EnvironmentStatusDeleted      EnvironmentStatus = "deleted"
	EnvironmentStatusDeleteFailed EnvironmentStatus = "deleteFailed"
)

// NewEnvironment は、プロバイダーに作成を依頼する前のチーム環境を作る。ProviderEnvironmentID は空である。
func NewEnvironment(teamID uuid.UUID) Environment {
	return Environment{
		ID:        uuid.New(),
		TeamID:    teamID,
		CreatedAt: time.Now(),
	}
}

// NewInstances は、プロバイダーが作ったサーバーから、チーム環境のサーバーの行を作る。
func (e Environment) NewInstances(servers []InfraServer) []Instance {
	now := time.Now()
	instances := make([]Instance, 0, len(servers))
	for _, s := range servers {
		instances = append(instances, Instance{
			ID:            uuid.New(),
			TeamID:        e.TeamID,
			EnvironmentID: e.ID,
			Name:          s.Name,
			Infra:         InfraInstance{ProviderInstanceID: s.ProviderInstanceID},
			CreatedAt:     now,
		})
	}
	return instances
}

// BenchTarget は、fixedTarget の名前のサーバーを返す。fixedTarget が空か、その名前のサーバーがなければ false を返す。
func (e Environment) BenchTarget(fixedTarget string) (Instance, bool) {
	if fixedTarget == "" {
		return Instance{}, false
	}
	for _, i := range e.Instances {
		if i.Name == fixedTarget {
			return i, true
		}
	}
	return Instance{}, false
}
