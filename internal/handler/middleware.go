package handler

import (
	"matter-core/internal/service"
	"matter-core/pkg/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(sessionStore *service.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookieName)
		if err != nil {
			utils.Unauthorized(c, "not authenticated")
			c.Abort()
			return
		}

		session, valid := sessionStore.IsValid(c.Request.Context(), token)
		if !valid {
			utils.Unauthorized(c, "session expired")
			c.Abort()
			return
		}

		c.Set("user_id", session.UserID.Hex())
		c.Set("user_role", session.Role)
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role != "admin" {
			utils.Forbidden(c, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func OptionalAuthMiddleware(sessionStore *service.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookieName)
		if err != nil {
			c.Next()
			return
		}

		session, valid := sessionStore.IsValid(c.Request.Context(), token)
		if valid {
			c.Set("user_id", session.UserID.Hex())
			c.Set("user_role", session.Role)
		}
		c.Next()
	}
}
