package usecase_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	repomock "github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	instancemock "github.com/traPtitech/piscon-portal-v2/server/services/instance/mock"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	"github.com/traPtitech/piscon-portal-v2/server/utils/ptr"
	"github.com/traPtitech/piscon-portal-v2/server/utils/testutil"
	"go.uber.org/mock/gomock"
)

func TestGetTeamInstances(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	teamID := uuid.New()
	instance1 := domain.Instance{
		ID:     uuid.New(),
		TeamID: teamID,
		Name:   "Server1",
		Infra: domain.InfraInstance{
			ProviderInstanceID: "infra1",
			Status:             domain.InstanceStatusRunning,
		},
	}
	instance2 := domain.Instance{
		ID:     uuid.New(),
		TeamID: teamID,
		Name:   "Server2",
		Infra: domain.InfraInstance{
			ProviderInstanceID: "infra2",
			Status:             domain.InstanceStatusStopped,
		},
	}
	instance3 := domain.Instance{
		ID:     uuid.New(),
		TeamID: teamID,
		Name:   "Server3",
		Infra: domain.InfraInstance{
			ProviderInstanceID: "infra3",
			Status:             domain.InstanceStatusDeleted,
		},
		DeletedAt: ptr.Of(time.Now()),
	}

	testCases := map[string]struct {
		teamID              uuid.UUID
		instances           []domain.Instance
		GetTeamInstancesErr error
		queriedIDs          []string
		infraInstances      []domain.InfraInstance
		GetByIDsErr         error
		result              []domain.Instance
		expectErr           error
	}{
		"正しく取得できる": {
			teamID:         teamID,
			instances:      []domain.Instance{instance1, instance2, instance3},
			queriedIDs:     []string{"infra1", "infra2"},
			infraInstances: []domain.InfraInstance{instance1.Infra, instance2.Infra},
			result:         []domain.Instance{instance1, instance2, instance3},
		},
		"GetTeamInstancesでエラー": {
			teamID:              teamID,
			GetTeamInstancesErr: assert.AnError,
			expectErr:           assert.AnError,
		},
		// 終了したサーバーの ID を渡すと、EC2 はリクエスト全体を失敗させることがある。
		"削除済みのサーバーだけならプロバイダーに問い合わせない": {
			teamID:    teamID,
			instances: []domain.Instance{instance3},
			result:    []domain.Instance{instance3},
		},
		"GetByIDsでエラー": {
			teamID:      teamID,
			instances:   []domain.Instance{instance1, instance2},
			queriedIDs:  []string{"infra1", "infra2"},
			GetByIDsErr: assert.AnError,
			expectErr:   assert.AnError,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := repomock.NewMockRepository(ctrl)
			manager := instancemock.NewMockManager(ctrl)
			u := usecase.NewInstanceUseCase(repo, manager)

			repo.EXPECT().GetTeamInstances(gomock.Any(), testCase.teamID, true).
				Return(testCase.instances, testCase.GetTeamInstancesErr)

			if testCase.queriedIDs != nil {
				manager.EXPECT().GetByIDs(gomock.Any(), testCase.queriedIDs).
					Return(testCase.infraInstances, testCase.GetByIDsErr)
			}

			result, err := u.GetTeamInstances(t.Context(), testCase.teamID)
			if testCase.expectErr != nil {
				assert.ErrorIs(t, err, testCase.expectErr)
			} else {
				assert.NoError(t, err)
			}

			if testCase.expectErr != nil {
				return
			}

			assert.Len(t, result, len(testCase.result))

			for i, inst := range result {
				testutil.CompareInstance(t, testCase.result[i], inst)
			}
		})
	}
}
