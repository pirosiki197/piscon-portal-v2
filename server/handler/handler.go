package handler

import (
	"net/http"

	"github.com/jellydator/ttlcache/v3"
	"github.com/labstack/echo/v4"
	"github.com/traPtitech/piscon-portal-v2/server/handler/internal"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
)

type Handler struct {
	repo           repository.Repository
	sessionManager SessionManager
	authProviders  []AuthProvider
	loginStates    *ttlcache.Cache[string, loginState]

	useCase usecase.UseCase
}

type Config struct {
	Debug         bool
	SessionSecret string
	// AuthProviders は、ログインに使える認証手段である。この順でログインボタンに並ぶ。
	AuthProviders []AuthProvider
	// if SessionManager is nil, use default session manager
	SessionManager SessionManager
}

func New(useCase usecase.UseCase, repo repository.Repository, config Config) *Handler {
	var sessionManager SessionManager
	if config.SessionManager == nil {
		sessionManager = internal.NewSessionManager(config.SessionSecret, config.Debug)
	} else {
		sessionManager = config.SessionManager
	}

	return &Handler{
		repo:           repo,
		sessionManager: sessionManager,
		authProviders:  config.AuthProviders,
		loginStates:    newLoginStateStore(),
		useCase:        useCase,
	}
}

func (h *Handler) SetupRoutes(e *echo.Echo) {
	api := e.Group("/api")
	h.sessionManager.Init(api)

	api.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	auth := api.Group("/auth")
	auth.GET("/providers", h.GetAuthProviders)
	auth.GET("/:providerID/login", h.GetAuthLogin)
	auth.GET("/:providerID/callback", h.GetAuthCallback)
	auth.POST("/logout", h.Logout, h.AuthMiddleware())
	for _, p := range h.authProviders {
		if p, ok := p.(AuthorizePageProvider); ok {
			e.GET(AuthorizePagePath(p.ID()), echo.WrapHandler(p.AuthorizePage()))
		}
	}

	users := api.Group("/users", h.AuthMiddleware())
	users.GET("", h.GetUsers)
	users.GET("/me", h.GetUserMe)

	teams := api.Group("/teams", h.AuthMiddleware())
	teams.GET("", h.GetTeams)
	teams.POST("", h.CreateTeam)
	teams.GET("/:teamID", h.GetTeam)
	teams.PATCH("/:teamID", h.UpdateTeam, h.TeamOrAdminAuthMiddleware())
	teams.GET("/:teamID/benchmarks", h.GetTeamBenchmarks, h.TeamOrAdminAuthMiddleware())
	teams.GET("/:teamID/benchmarks/:benchmarkID", h.GetTeamBenchmark, h.TeamOrAdminAuthMiddleware())
	teams.GET("/:teamID/instances", h.GetTeamInstances, h.TeamOrAdminAuthMiddleware())
	teams.GET("/:teamID/environment", h.GetTeamEnvironment, h.TeamOrAdminAuthMiddleware())
	teams.POST("/:teamID/environment", h.CreateTeamEnvironment, h.TeamOrAdminAuthMiddleware())
	teams.DELETE("/:teamID/environment", h.DeleteTeamEnvironment, h.TeamOrAdminAuthMiddleware())
	teams.PATCH("/:teamID/environment", h.PatchTeamEnvironment, h.TeamOrAdminAuthMiddleware())

	instances := api.Group("/instances", h.AuthMiddleware())
	instances.GET("", h.GetInstances, h.AdminAuthMiddleware())

	benchmarks := api.Group("/benchmarks", h.AuthMiddleware())
	benchmarks.GET("", h.GetBenchmarks, h.AdminAuthMiddleware())
	benchmarks.POST("", h.EnqueueBenchmark)
	benchmarks.GET("/queue", h.GetQueuedBenchmarks)
	benchmarks.GET("/:benchmarkID", h.GetBenchmark, h.AdminAuthMiddleware())
	benchmarks.DELETE("/:benchmarkID", h.DeleteBenchmark, h.AdminAuthMiddleware())

	scores := api.Group("/scores", h.AuthMiddleware())
	scores.GET("", h.GetScores)
	scores.GET("/ranking", h.GetRanking)

	docs := api.Group("/docs", h.AuthMiddleware())
	docs.GET("", h.GetDocument)
	docs.PATCH("", h.PatchDocument, h.AdminAuthMiddleware())

	admins := api.Group("/admins", h.AuthMiddleware(), h.AdminAuthMiddleware())
	admins.PUT("", h.PutAdmins)
}
