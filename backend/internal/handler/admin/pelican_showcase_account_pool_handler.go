package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *PelicanGroupTestHandler) GetAccountPool(c *gin.Context) {
	groupID, ok := pelicanGroupTestID(c, "id")
	if !ok {
		return
	}
	view, err := h.svc.GetAccountPool(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *PelicanGroupTestHandler) UpdateAccountPool(c *gin.Context) {
	groupID, ok := pelicanGroupTestID(c, "id")
	if !ok {
		return
	}
	var input service.PelicanAccountPool
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	view, err := h.svc.UpdateAccountPool(c.Request.Context(), groupID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}
