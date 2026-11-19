package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/labstack/echo/v4"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/handler/openapi"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	"github.com/traPtitech/piscon-portal-v2/server/utils/random"
)

// AuthProvider は、外部でユーザーを確かめて外部アカウントを返す認証手段である。
// ブラウザを AuthURL に送り出し、認証手段がコールバックのパスに state を付けて戻すと、Complete で外部アカウントを得る。
type AuthProvider interface {
	// ID は、認証手段の ID を返す。ルートのパスと users に使うので、一度使い始めたら変えない。
	ID() string
	// DisplayName は、ログインボタンに出す名前を返す。
	DisplayName() string
	// AuthURL は、ブラウザを送り出す先の URL を返す。state はポータルが発行し、照合もポータルが行う。
	AuthURL(state string) string
	// Complete は、コールバックのクエリから外部アカウントを得る。認証に失敗したらエラーを返す。
	Complete(ctx context.Context, state string, query url.Values) (domain.Identity, error)
}

// AuthorizePageProvider は、ポータルの中に認可ページを持つ認証手段である。
// ポータルは AuthorizePage を [AuthorizePagePath] に公開する。
type AuthorizePageProvider interface {
	AuthProvider
	AuthorizePage() http.Handler
}

// CallbackPath は、認証手段から戻ってくるコールバックのパスを返す。
func CallbackPath(providerID string) string {
	return "/api/auth/" + providerID + "/callback"
}

// AuthorizePagePath は、[AuthorizePageProvider] の認可ページを公開するパスを返す。
func AuthorizePagePath(providerID string) string {
	return "/api/auth/" + providerID + "/authorize"
}

const sessionMaxAge = 7 * 24 * time.Hour

// loginState は、ログインを始めたときに発行した state と、その認証手段の ID の組である。
type loginState struct {
	state      string
	providerID string
}

func newLoginStateStore() *ttlcache.Cache[string, loginState] {
	store := ttlcache.New(
		ttlcache.WithTTL[string, loginState](15 * time.Minute),
	)
	go store.Start()
	return store
}

func (h *Handler) GetAuthProviders(c echo.Context) error {
	res := make([]openapi.AuthProvider, 0, len(h.authProviders))
	for _, p := range h.authProviders {
		res = append(res, openapi.AuthProvider{
			ID:   openapi.AuthProviderId(p.ID()),
			Name: p.DisplayName(),
		})
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) findAuthProvider(id string) (AuthProvider, bool) {
	for _, p := range h.authProviders {
		if p.ID() == id {
			return p, true
		}
	}
	return nil, false
}

func (h *Handler) GetAuthLogin(c echo.Context) error {
	provider, ok := h.findAuthProvider(c.Param("providerID"))
	if !ok {
		return notFoundResponse(c)
	}

	sessID, err := h.sessionManager.SetSessionID(c, sessionMaxAge)
	if err != nil {
		return internalServerErrorResponse(c, err)
	}

	state := random.String(16)
	h.loginStates.Set(sessID, loginState{state: state, providerID: provider.ID()}, ttlcache.DefaultTTL)

	return c.Redirect(http.StatusSeeOther, provider.AuthURL(state))
}

func (h *Handler) GetAuthCallback(c echo.Context) error {
	ctx := c.Request().Context()

	provider, ok := h.findAuthProvider(c.Param("providerID"))
	if !ok {
		return notFoundResponse(c)
	}

	sessionID, err := h.sessionManager.GetSessionID(c)
	if err != nil {
		return internalServerErrorResponse(c, err)
	}

	state := c.QueryParam("state")
	issued, found := h.loginStates.GetAndDelete(sessionID)
	if !found || issued.Value().state != state || issued.Value().providerID != provider.ID() {
		return c.String(http.StatusBadRequest, "invalid state")
	}

	identity, err := provider.Complete(ctx, state, c.QueryParams())
	if err != nil {
		c.Logger().Warn(err)
		return c.String(http.StatusBadRequest, "authentication failed")
	}

	user, err := h.useCase.Login(ctx, identity)
	if useCaseErr, ok := errors.AsType[usecase.UseCaseError](err); ok {
		return c.String(http.StatusBadRequest, useCaseErr.Error())
	}
	if err != nil {
		return internalServerErrorResponse(c, err)
	}

	// ログイン前と別のセッション ID にして、セッション固定攻撃を防ぐ
	newSessionID, err := h.sessionManager.SetSessionID(c, sessionMaxAge)
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	session := domain.NewSession(newSessionID, user.ID, time.Now().Add(sessionMaxAge))
	if err := h.repo.CreateSession(ctx, session); err != nil {
		return internalServerErrorResponse(c, err)
	}

	return c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) Logout(c echo.Context) error {
	ctx := c.Request().Context()

	sessID, err := h.sessionManager.GetSessionID(c)
	if err != nil {
		return internalServerErrorResponse(c, err)
	}

	// delete sessionID
	err = h.repo.DeleteSession(ctx, sessID)
	if err != nil {
		return internalServerErrorResponse(c, err)
	}

	if err := h.sessionManager.ClearSessionID(c); err != nil {
		return internalServerErrorResponse(c, err)
	}

	return c.NoContent(http.StatusOK)
}
