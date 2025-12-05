package servers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lechgu/tichy/internal/auth"
)

// AuthMiddleware provides auth middleware to extract user information
// from passed JWT token and put it into context
func AuthMiddleware(jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Expect: Authorization: Bearer <token>
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid auth header"})
			return
		}

		tokenString := parts[1]

		user, err := auth.ParseToken(tokenString, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}

		// Inject into request context
		ctx := auth.WithUser(c.Request.Context(), user)

		// Continue chain with new context
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
