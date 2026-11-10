package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/handler/openapi"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
)

func (h *Handler) GetTeamEnvironment(c echo.Context) error {
	ctx := c.Request().Context()
	teamID, err := uuid.Parse(c.Param("teamID"))
	if err != nil {
		return badRequestResponse(c, "invalid team id")
	}

	env, err := h.useCase.GetTeamEnvironment(ctx, teamID)
	if errors.Is(err, usecase.ErrNotFound) {
		return notFoundResponse(c)
	}
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	return c.JSON(http.StatusOK, toOpenAPITeamEnvironment(env))
}

func (h *Handler) CreateTeamEnvironment(c echo.Context) error {
	ctx := c.Request().Context()
	teamID, err := uuid.Parse(c.Param("teamID"))
	if err != nil {
		return badRequestResponse(c, "invalid team id")
	}

	env, err := h.useCase.CreateEnvironment(ctx, teamID)
	if usecase.IsUseCaseError(err) {
		return badRequestResponse(c, err.Error())
	}
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	return c.JSON(http.StatusCreated, toOpenAPITeamEnvironment(env))
}

func (h *Handler) DeleteTeamEnvironment(c echo.Context) error {
	ctx := c.Request().Context()
	teamID, err := uuid.Parse(c.Param("teamID"))
	if err != nil {
		return badRequestResponse(c, "invalid team id")
	}

	err = h.useCase.DeleteEnvironment(ctx, teamID)
	if errors.Is(err, usecase.ErrNotFound) {
		return notFoundResponse(c)
	}
	if usecase.IsUseCaseError(err) {
		return badRequestResponse(c, err.Error())
	}
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) PatchTeamEnvironment(c echo.Context) error {
	ctx := c.Request().Context()
	teamID, err := uuid.Parse(c.Param("teamID"))
	if err != nil {
		return badRequestResponse(c, "invalid team id")
	}
	var req openapi.PatchTeamEnvironmentReq
	if err := c.Bind(&req); err != nil {
		return badRequestResponse(c, err.Error())
	}

	var op domain.InstanceOperation
	switch req.Operation {
	case openapi.InstanceOperationStart:
		op = domain.InstanceOperationStart
	case openapi.InstanceOperationStop:
		op = domain.InstanceOperationStop
	default:
		return badRequestResponse(c, "invalid operation")
	}

	err = h.useCase.UpdateEnvironment(ctx, teamID, op)
	if errors.Is(err, usecase.ErrNotFound) {
		return notFoundResponse(c)
	}
	if usecase.IsUseCaseError(err) {
		return badRequestResponse(c, err.Error())
	}
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) GetEnvironments(c echo.Context) error {
	envs, err := h.useCase.GetEnvironments(c.Request().Context())
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	res := lo.Map(envs, func(env domain.Environment, _ int) *openapi.Environment {
		return toOpenAPIEnvironment(env)
	})
	return c.JSON(http.StatusOK, res)
}

func toOpenAPITeamEnvironment(env usecase.TeamEnvironment) *openapi.Environment {
	res := toOpenAPIEnvironment(env.Environment)
	if env.BenchTargetID.IsSet() {
		res.BenchTargetInstanceId = openapi.NewOptInstanceId(openapi.InstanceId(env.BenchTargetID.Get()))
	}
	return res
}

// toOpenAPIEnvironment は、ogen の MarshalJSON で空の Opt 型を省くため、ポインタを返す。
func toOpenAPIEnvironment(env domain.Environment) *openapi.Environment {
	res := &openapi.Environment{
		ID:        openapi.EnvironmentId(env.ID),
		TeamId:    openapi.TeamId(env.TeamID),
		Status:    openapi.EnvironmentStatus(env.Infra.Status),
		Instances: lo.Map(env.Instances, toOpenAPIInstance),
		CreatedAt: env.CreatedAt,
	}
	if env.Infra.FailureReason != "" {
		res.FailureReason = openapi.NewOptString(env.Infra.FailureReason)
	}
	return res
}
