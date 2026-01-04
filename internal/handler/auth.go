package handler

import (
	"net/http"
	"time"

	"matter-core/internal/config"
	"matter-core/internal/service"
	"matter-core/pkg/utils"

	"github.com/gin-gonic/gin"
)

const (
	SessionCookieName = "session_token"
	SessionDuration   = 7 * 24 * time.Hour
)

type AuthHandler struct {
	authService  *service.AuthService
	sessionStore *service.SessionStore
	cfg          *config.Config
}

func NewAuthHandler(authService *service.AuthService, sessionStore *service.SessionStore, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		sessionStore: sessionStore,
		cfg:          cfg,
	}
}

// GET /api/v1/auth/signin/:provider - 跳转到 OAuth 提供商
func (h *AuthHandler) SignIn(c *gin.Context) {
	provider := c.Param("provider")

	url, err := h.authService.GetAuthURL(provider)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	c.Redirect(http.StatusFound, url)
}

// GET /api/v1/auth/callback/:provider - OAuth 回调
func (h *AuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")

	if code == "" {
		c.Redirect(http.StatusFound, h.cfg.FrontendURL+"?error=missing_code")
		return
	}

	user, err := h.authService.HandleCallback(c.Request.Context(), provider, code)
	if err != nil {
		c.Redirect(http.StatusFound, h.cfg.FrontendURL+"?error=auth_failed")
		return
	}

	// 创建 session
	token, err := h.sessionStore.Create(c.Request.Context(), user.ID, user.Role, SessionDuration)
	if err != nil {
		c.Redirect(http.StatusFound, h.cfg.FrontendURL+"?error=session_failed")
		return
	}

	// 设置 Cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		SessionCookieName,
		token,
		int(SessionDuration.Seconds()),
		"/",
		"",
		h.cfg.SecureCookie,
		true, // HttpOnly
	)

	c.Redirect(http.StatusFound, h.cfg.FrontendURL)
}

// GET /api/v1/auth/session - 获取当前用户信息
func (h *AuthHandler) Session(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Success(c, gin.H{"user": nil})
		return
	}

	user, err := h.authService.GetUserByID(c.Request.Context(), userID.(string))
	if err != nil {
		utils.Success(c, gin.H{"user": nil})
		return
	}

	utils.Success(c, gin.H{"user": user})
}

// POST /api/v1/auth/signout - 登出
func (h *AuthHandler) SignOut(c *gin.Context) {
	token, err := c.Cookie(SessionCookieName)
	if err == nil {
		h.sessionStore.Delete(c.Request.Context(), token)
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", h.cfg.SecureCookie, true)

	utils.Success(c, nil)
}
