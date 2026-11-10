package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	repomock "github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	usecasemock "github.com/traPtitech/piscon-portal-v2/server/usecase/mock"
	"github.com/traPtitech/piscon-portal-v2/server/utils/optional"
	"go.uber.org/mock/gomock"
)

func newEnvironmentContext(t *testing.T, method string, teamID uuid.UUID, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(method, "/teams/"+teamID.String()+"/environment", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("teamID")
	c.SetParamValues(teamID.String())
	return c, rec
}

func TestGetTeamEnvironment(t *testing.T) {
	teamID := uuid.New()
	instanceID := uuid.New()
	env := usecase.TeamEnvironment{
		Environment: domain.Environment{
			ID:     uuid.New(),
			TeamID: teamID,
			Infra:  domain.InfraEnvironment{Status: domain.EnvironmentStatusReady},
			Instances: []domain.Instance{{
				ID:     instanceID,
				TeamID: teamID,
				Name:   "Router",
				Infra:  domain.InfraInstance{Status: domain.InstanceStatusRunning},
			}},
			CreatedAt: time.Now(),
		},
		BenchTargetID: optional.From(instanceID),
	}

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := usecasemock.NewMockUseCase(ctrl)
		uc.EXPECT().GetTeamEnvironment(gomock.Any(), teamID).Return(env, nil)
		c, rec := newEnvironmentContext(t, http.MethodGet, teamID, "")

		require.NoError(t, NewHandler(uc, repomock.NewMockRepository(ctrl), nil).GetTeamEnvironment(c))

		assert.Equal(t, http.StatusOK, rec.Code)
		var res map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		assert.Equal(t, "ready", res["status"])
		assert.Equal(t, instanceID.String(), res["benchTargetInstanceId"])
		assert.NotContains(t, res, "failureReason")
		instances := res["instances"].([]any)
		require.Len(t, instances, 1)
		assert.Equal(t, "Router", instances[0].(map[string]any)["name"])
	})

	t.Run("not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := usecasemock.NewMockUseCase(ctrl)
		uc.EXPECT().GetTeamEnvironment(gomock.Any(), teamID).Return(usecase.TeamEnvironment{}, usecase.ErrNotFound)
		c, rec := newEnvironmentContext(t, http.MethodGet, teamID, "")

		require.NoError(t, NewHandler(uc, repomock.NewMockRepository(ctrl), nil).GetTeamEnvironment(c))

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestCreateTeamEnvironment(t *testing.T) {
	teamID := uuid.New()

	t.Run("already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := usecasemock.NewMockUseCase(ctrl)
		uc.EXPECT().CreateEnvironment(gomock.Any(), teamID).
			Return(usecase.TeamEnvironment{}, usecase.NewUseCaseErrorFromMsg("team already has an environment"))
		c, rec := newEnvironmentContext(t, http.MethodPost, teamID, "")

		require.NoError(t, NewHandler(uc, repomock.NewMockRepository(ctrl), nil).CreateTeamEnvironment(c))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatchTeamEnvironment(t *testing.T) {
	teamID := uuid.New()

	tests := map[string]struct {
		body         string
		op           domain.InstanceOperation
		expectCall   bool
		expectedCode int
	}{
		"start":             {body: `{"operation":"start"}`, op: domain.InstanceOperationStart, expectCall: true, expectedCode: http.StatusOK},
		"stop":              {body: `{"operation":"stop"}`, op: domain.InstanceOperationStop, expectCall: true, expectedCode: http.StatusOK},
		"invalid operation": {body: `{"operation":"reboot"}`, expectedCode: http.StatusBadRequest},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := usecasemock.NewMockUseCase(ctrl)
			if tt.expectCall {
				uc.EXPECT().UpdateEnvironment(gomock.Any(), teamID, tt.op).Return(nil)
			}
			c, rec := newEnvironmentContext(t, http.MethodPatch, teamID, tt.body)

			require.NoError(t, NewHandler(uc, repomock.NewMockRepository(ctrl), nil).PatchTeamEnvironment(c))

			assert.Equal(t, tt.expectedCode, rec.Code)
		})
	}
}

func TestDeleteTeamEnvironment(t *testing.T) {
	teamID := uuid.New()

	t.Run("benchmark running", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := usecasemock.NewMockUseCase(ctrl)
		uc.EXPECT().DeleteEnvironment(gomock.Any(), teamID).
			Return(usecase.NewUseCaseErrorFromMsg("cannot delete the environment while a benchmark is waiting or running"))
		c, rec := newEnvironmentContext(t, http.MethodDelete, teamID, "")

		require.NoError(t, NewHandler(uc, repomock.NewMockRepository(ctrl), nil).DeleteTeamEnvironment(c))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
