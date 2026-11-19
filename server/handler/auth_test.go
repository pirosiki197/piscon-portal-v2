package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	repomock "github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	usecasemock "github.com/traPtitech/piscon-portal-v2/server/usecase/mock"
	"go.uber.org/mock/gomock"
)

func TestGetAuthProviders(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := NewPortalServer(usecasemock.NewMockUseCase(ctrl), repomock.NewMockRepository(ctrl))
	client := NewClient(server)

	res, err := client.Get(joinPath(t, server.URL, "/api/auth/providers"))
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	var got []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
	assert.Equal(t, []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{{ID: "traq", Name: "traQ"}, {ID: "dev", Name: "dev"}}, got)
}

func TestLoginWithTraQ(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRepo := repomock.NewMockRepository(ctrl)
	mockUseCase := usecasemock.NewMockUseCase(ctrl)

	server := NewPortalServer(mockUseCase, mockRepo)
	client := NewClient(server)
	subject := uuid.NewString()
	userID := uuid.New()

	mockUseCase.EXPECT().Login(gomock.Any(), domain.Identity{
		Provider:    "traq",
		Subject:     subject,
		DefaultName: "test-user",
		IconURL:     oauth2ServerURL + "/api/v3/public/icon/test-user",
	}).Return(domain.User{ID: userID}, nil)
	mockRepo.EXPECT().
		CreateSession(gomock.Any(), gomock.Cond(func(s domain.Session) bool { return s.UserID == userID })).
		Return(nil)

	if err := Login(t, server, client, subject); err != nil {
		t.FailNow()
	}
}

func TestLoginWithDev(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRepo := repomock.NewMockRepository(ctrl)
	mockUseCase := usecasemock.NewMockUseCase(ctrl)

	server := NewPortalServer(mockUseCase, mockRepo)
	client := NewClient(server)
	userID := uuid.New()

	mockUseCase.EXPECT().Login(gomock.Any(), domain.Identity{
		Provider:    "dev",
		Subject:     "alice",
		DefaultName: "alice",
	}).Return(domain.User{ID: userID}, nil)
	mockRepo.EXPECT().
		CreateSession(gomock.Any(), gomock.Cond(func(s domain.Session) bool { return s.UserID == userID })).
		Return(nil)

	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	loginRes, err := client.Get(joinPath(t, server.URL, "/api/auth/dev/login"))
	require.NoError(t, err)
	defer loginRes.Body.Close()
	require.Equal(t, http.StatusSeeOther, loginRes.StatusCode)

	// 偽の認可ページでフォームを送ったときと同じリクエストを、state を引き継いで送る
	authorizeURL, err := url.Parse(loginRes.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/api/auth/dev/authorize", authorizeURL.Path)
	query := url.Values{"state": {authorizeURL.Query().Get("state")}, "name": {"alice"}}

	callbackRes, err := client.Get(joinPath(t, server.URL, "/api/auth/dev/callback") + "?" + query.Encode())
	require.NoError(t, err)
	defer callbackRes.Body.Close()
	assert.Equal(t, http.StatusSeeOther, callbackRes.StatusCode)
	assert.Equal(t, "/", callbackRes.Header.Get("Location"))
}

func TestLoginNameAlreadyUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRepo := repomock.NewMockRepository(ctrl)
	mockUseCase := usecasemock.NewMockUseCase(ctrl)

	server := NewPortalServer(mockUseCase, mockRepo)
	client := NewClient(server)

	mockUseCase.EXPECT().Login(gomock.Any(), gomock.Any()).
		Return(domain.User{}, usecase.NewUseCaseErrorFromMsg("user name is already used"))

	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	loginRes, err := client.Get(joinPath(t, server.URL, "/api/auth/dev/login"))
	require.NoError(t, err)
	defer loginRes.Body.Close()
	authorizeURL, err := url.Parse(loginRes.Header.Get("Location"))
	require.NoError(t, err)
	query := url.Values{"state": {authorizeURL.Query().Get("state")}, "name": {"alice"}}

	res, err := client.Get(joinPath(t, server.URL, "/api/auth/dev/callback") + "?" + query.Encode())
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestLoginWithInvalidState(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := NewPortalServer(usecasemock.NewMockUseCase(ctrl), repomock.NewMockRepository(ctrl))
	client := NewClient(server)

	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	// traQ でログインを始めて得た state を、dev のコールバックに渡す
	loginRes, err := client.Get(joinPath(t, server.URL, "/api/auth/traq/login"))
	require.NoError(t, err)
	defer loginRes.Body.Close()
	authURL, err := url.Parse(loginRes.Header.Get("Location"))
	require.NoError(t, err)
	query := url.Values{"state": {authURL.Query().Get("state")}, "name": {"alice"}}

	res, err := client.Get(joinPath(t, server.URL, "/api/auth/dev/callback") + "?" + query.Encode())
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestLoginWithUnknownProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := NewPortalServer(usecasemock.NewMockUseCase(ctrl), repomock.NewMockRepository(ctrl))
	client := NewClient(server)

	res, err := client.Get(joinPath(t, server.URL, "/api/auth/unknown/login"))
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestLogout(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRepo := repomock.NewMockRepository(ctrl)
	mockUseCase := usecasemock.NewMockUseCase(ctrl)

	server := NewPortalServer(mockUseCase, mockRepo)
	client := NewClient(server)
	userID := uuid.New()

	mockUseCase.EXPECT().Login(gomock.Any(), gomock.Any()).Return(domain.User{ID: userID}, nil)
	mockRepo.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(nil)
	if err := Login(t, server, client, uuid.NewString()); err != nil {
		t.FailNow()
	}

	// return not expired session
	mockRepo.EXPECT().FindSession(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, sid string) (domain.Session, error) {
			return domain.Session{
				ID:        sid,
				UserID:    userID,
				ExpiresAt: time.Now().Add(time.Hour),
			}, nil
		})
	mockRepo.EXPECT().DeleteSession(gomock.Any(), gomock.Any()).Return(nil)

	res, err := client.Post(joinPath(t, server.URL, "/api/auth/logout"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(res.Body)
		t.Fatalf("status code is %d: %s", res.StatusCode, msg)
	}
}
