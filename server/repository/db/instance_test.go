package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/utils/ptr"
	"github.com/traPtitech/piscon-portal-v2/server/utils/testutil"
)

func TestFindInstance(t *testing.T) {
	t.Parallel()

	repo, db := setupRepository(t)

	instance := domain.Instance{
		ID:            uuid.New(),
		TeamID:        uuid.New(),
		EnvironmentID: uuid.New(),
		Name:          "Webapp",
		Infra: domain.InfraInstance{
			ProviderInstanceID: "prov-instance-id",
		},
		CreatedAt: time.Now(),
	}
	mustMakeInstance(t, db, instance)

	got, err := repo.FindInstance(t.Context(), instance.ID)
	assert.NoError(t, err)

	testutil.CompareInstance(t, instance, got)
}

func TestFindInstance_NotFound(t *testing.T) {
	t.Parallel()

	repo, _ := setupRepository(t)

	_, err := repo.FindInstance(t.Context(), uuid.New())
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestGetTeamInstances(t *testing.T) {
	t.Parallel()

	repo, db := setupRepository(t)

	teamID := uuid.New()
	otherTeamID := uuid.New()
	newInstance := func(teamID uuid.UUID, name string, deletedAt *time.Time) domain.Instance {
		return domain.Instance{
			ID:            uuid.New(),
			TeamID:        teamID,
			EnvironmentID: uuid.New(),
			Name:          name,
			Infra:         domain.InfraInstance{ProviderInstanceID: "prov-" + name},
			CreatedAt:     time.Now(),
			DeletedAt:     deletedAt,
		}
	}
	instances := []domain.Instance{
		newInstance(teamID, "A", nil),
		newInstance(teamID, "B", ptr.Of(time.Now())),
		newInstance(otherTeamID, "C", nil),
		newInstance(otherTeamID, "D", ptr.Of(time.Now())),
	}
	for _, inst := range instances {
		mustMakeInstance(t, db, inst)
	}

	t.Run("削除済みも含める", func(t *testing.T) {
		got, err := repo.GetTeamInstances(t.Context(), teamID, true)
		assert.NoError(t, err)
		assert.Len(t, got, 2)
		testutil.ContainsInstance(t, got, instances[0])
		testutil.ContainsInstance(t, got, instances[1])
	})

	t.Run("削除済みを除く", func(t *testing.T) {
		got, err := repo.GetTeamInstances(t.Context(), teamID, false)
		assert.NoError(t, err)
		assert.Len(t, got, 1)
		testutil.ContainsInstance(t, got, instances[0])
	})
}
