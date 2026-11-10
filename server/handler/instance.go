package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/handler/openapi"
)

func (h *Handler) GetTeamInstances(c echo.Context) error {
	ctx := c.Request().Context()
	teamID, err := uuid.Parse(c.Param("teamID"))
	if err != nil {
		return badRequestResponse(c, err.Error())
	}

	instances, err := h.useCase.GetTeamInstances(ctx, teamID)
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	res := lo.Map(instances, toOpenAPIInstance)

	return c.JSON(http.StatusOK, res)
}

func (h *Handler) GetInstances(c echo.Context) error {
	instances, err := h.useCase.GetAllInstances(c.Request().Context())
	if err != nil {
		return internalServerErrorResponse(c, err)
	}
	return c.JSON(http.StatusOK, lo.Map(instances, toOpenAPIInstance))
}

func toOpenAPIInstance(instance domain.Instance, _ int) openapi.Instance {
	privateIP, publicIP := "", ""
	if instance.Infra.PrivateIP != nil {
		privateIP = *instance.Infra.PrivateIP
	}
	if instance.Infra.PublicIP != nil {
		publicIP = *instance.Infra.PublicIP
	}
	return openapi.Instance{
		ID:               openapi.InstanceId(instance.ID),
		TeamId:           openapi.TeamId(instance.TeamID),
		Name:             instance.Name,
		PublicIPAddress:  openapi.IPAddress(publicIP),
		PrivateIPAddress: openapi.IPAddress(privateIP),
		Status:           openapi.InstanceStatus(instance.Infra.Status),
		CreatedAt:        instance.CreatedAt,
	}
}
