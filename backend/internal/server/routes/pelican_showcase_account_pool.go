package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerPelicanAccountPoolRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	admin.GET("/pelican-showcase/groups/:id/account-pool", h.Admin.PelicanGroupTest.GetAccountPool)
	admin.PUT("/pelican-showcase/groups/:id/account-pool", h.Admin.PelicanGroupTest.UpdateAccountPool)
}
