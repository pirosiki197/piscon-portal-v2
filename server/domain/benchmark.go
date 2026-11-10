package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type BenchmarkStatus string

const (
	BenchmarkStatusWaiting  BenchmarkStatus = "waiting"
	BenchmarkStatusReadying BenchmarkStatus = "readying"
	BenchmarkStatusRunning  BenchmarkStatus = "running"
	BenchmarkStatusFinished BenchmarkStatus = "finished"
)

type Benchmark struct {
	ID         uuid.UUID
	Instance   Instance
	TeamID     uuid.UUID
	UserID     uuid.UUID
	Status     BenchmarkStatus
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	Score      int64
	Result     *BenchmarkResult
	ErrorMes   *string
}

// BenchmarkJob contains a benchmark and its team's current problem server IPs.
type BenchmarkJob struct {
	Benchmark           Benchmark
	InstanceIPAddresses []string
}

// NewBenchmark は、env のサーバー instance を対象にするベンチを作る。
// fixedTarget は、ベンチの対象に固定するサーバーの名前である。空なら、チーム環境のどのサーバーも対象にできる。
func NewBenchmark(env Environment, instance Instance, user User, fixedTarget string) (Benchmark, error) {
	if !user.TeamID.Valid {
		return Benchmark{}, errors.New("user is not in a team")
	}
	if instance.TeamID != user.TeamID.UUID {
		return Benchmark{}, errors.New("teamID does not match")
	}
	if instance.EnvironmentID != env.ID {
		return Benchmark{}, errors.New("instance is not in the environment")
	}
	if env.DeletedAt != nil || env.Infra.Status != EnvironmentStatusReady {
		return Benchmark{}, errors.New("environment is not ready")
	}
	if fixedTarget != "" && instance.Name != fixedTarget {
		return Benchmark{}, fmt.Errorf("benchmark target must be %s", fixedTarget)
	}
	if instance.Infra.Status != InstanceStatusRunning {
		return Benchmark{}, errors.New("instance is not running")
	}

	return Benchmark{
		ID:        uuid.New(),
		Instance:  instance,
		TeamID:    user.TeamID.UUID,
		UserID:    user.ID,
		Status:    BenchmarkStatusWaiting,
		CreatedAt: time.Now(),
	}, nil
}

type BenchmarkResult string

const (
	BenchmarkResultStatusPassed BenchmarkResult = "passed"
	BenchmarkResultStatusFailed BenchmarkResult = "failed"
	BenchmarkResultStatusError  BenchmarkResult = "error"
)

type BenchmarkLog struct {
	UserLog  string
	AdminLog string
}
