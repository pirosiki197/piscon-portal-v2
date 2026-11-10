package domain

import (
	"time"

	"github.com/google/uuid"
)

// Instance は、チーム環境の中の 1 台のサーバーである。
type Instance struct {
	ID            uuid.UUID
	TeamID        uuid.UUID
	EnvironmentID uuid.UUID
	// Name は、環境テンプレートでの論理 ID である。1 つのチーム環境の中で一意である。
	Name      string
	Infra     InfraInstance
	CreatedAt time.Time
	DeletedAt *time.Time
}

type InfraInstance struct {
	ProviderInstanceID string
	Status             InstanceStatus
	PrivateIP          *string
	PublicIP           *string
}

type InstanceStatus string

const (
	InstanceStatusUnknown  InstanceStatus = "unknown"
	InstanceStatusPending  InstanceStatus = "pending"
	InstanceStatusRunning  InstanceStatus = "running"
	InstanceStatusBuilding InstanceStatus = "building"
	InstanceStatusStarting InstanceStatus = "starting"
	InstanceStatusStopping InstanceStatus = "stopping"
	InstanceStatusStopped  InstanceStatus = "stopped"
	InstanceStatusDeleting InstanceStatus = "deleting"
	InstanceStatusDeleted  InstanceStatus = "deleted"
)

type InstanceOperation int

const (
	InstanceOperationStart InstanceOperation = iota
	InstanceOperationStop
)
