package handler

import (
	"matter-core/internal/service"
	"matter-core/pkg/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Login(c *gin.Context) {
	provider := c.Param("provider")

	url, err := h.authService.GetAuthURL(provider)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	c.Redirect(302, url)
}

func (h *AuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")

	if code == "" {
		utils.BadRequest(c, "missing code parameter")
		return
	}

	user, token, err := h.authService.HandleCallback(c.Request.Context(), provider, code)
	if err != nil {
		utils.InternalError(c, "authentication failed: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"user":  user,
		"token": token,
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "not authenticated")
		return
	}

	user, err := h.authService.GetUserByID(c.Request.Context(), userID.(string))
	if err != nil {
		utils.InternalError(c, "failed to get user")
		return
	}

	utils.Success(c, user)
}
