package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/handler/openapi"
	repomock "github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	usecasemock "github.com/traPtitech/piscon-portal-v2/server/usecase/mock"
	"github.com/traPtitech/piscon-portal-v2/server/utils/ptr"
	"go.uber.org/mock/gomock"
)

func TestGetTeamInstances(t *testing.T) {
	teamID := uuid.New()
	instances := []domain.Instance{
		{
			ID:     uuid.New(),
			TeamID: teamID,
			Name:   "Webapp",
			Infra: domain.InfraInstance{
				PublicIP:  ptr.Of("1.2.3.4"),
				PrivateIP: ptr.Of("10.0.0.1"),
				Status:    domain.InstanceStatusRunning,
			},
			CreatedAt: time.Now(),
		},
	}
	tests := []struct {
		name         string
		mockSetup    func(*usecasemock.MockUseCase)
		expectedCode int
		expectedLen  int
		expectErr    bool
	}{
		{
			name: "success",
			mockSetup: func(m *usecasemock.MockUseCase) {
				m.EXPECT().GetTeamInstances(gomock.Any(), teamID).Return(instances, nil)
			},
			expectedCode: http.StatusOK,
			expectedLen:  1,
			expectErr:    false,
		},
		{
			name: "error",
			mockSetup: func(m *usecasemock.MockUseCase) {
				m.EXPECT().GetTeamInstances(gomock.Any(), teamID).Return(nil, errors.New("unexpected error"))
			},
			expectedCode: http.StatusInternalServerError,
			expectedLen:  0,
			expectErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repoMock := repomock.NewMockRepository(ctrl)
			useCaseMock := usecasemock.NewMockUseCase(ctrl)
			e := echo.New()
			tt.mockSetup(useCaseMock)

			req := httptest.NewRequest(http.MethodGet, "/teams/"+teamID.String()+"/instances", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("teamID")
			c.SetParamValues(teamID.String())
			h := NewHandler(useCaseMock, repoMock, nil)

			err := h.GetTeamInstances(c)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, rec.Code)
			if tt.expectedCode == http.StatusOK {
				var res []openapi.Instance
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
				assert.Len(t, res, tt.expectedLen)
				if tt.expectedLen > 0 {
					assert.Equal(t, instances[0].ID, uuid.UUID(res[0].ID))
				}
			}
		})
	}
}
