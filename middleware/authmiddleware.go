package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware is a simplified middleware that trusts the API gateway's authentication
// and just extracts user information from the request headers
func AuthMiddleware(requiredRole string, _ string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log that we're using the simplified middleware
		log.Printf("Using simplified AuthMiddleware for path: %s", c.Request.URL.Path)
		
		// Get user ID from header (set by API gateway)
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			log.Printf("AUTH ERROR: Missing X-User-ID header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing user identification"})
			c.Abort()
			return
		}
		log.Printf("AUTH: User ID from header: %s", userID)

		// Get user role from header (set by API gateway)
		userRole := c.GetHeader("X-User-Role")
		if userRole == "" {
			log.Printf("AUTH ERROR: Missing X-User-Role header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing user role"})
			c.Abort()
			return
		}
		log.Printf("AUTH: User role from header: %s", userRole)

		// Check if the user has the required role
		if userRole != requiredRole {
			log.Printf("AUTH ERROR: User role %s does not match required role %s", userRole, requiredRole)
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			c.Abort()
			return
		}

		// Store user info in Gin context for handlers to use
		c.Set("user_id", userID)
		c.Set("role", userRole)

		c.Next()
	}
}
