package routes

import (
	"jobservice/delivery/handler"

	"github.com/gin-gonic/gin"
)

// Auth service address - centralized for easier maintenance
var (
	AuthServiceAddr = "localhost:50051" // Auth service gRPC address - matches the Auth Service port
)

// JobRoutes sets up all job-related routes with appropriate middleware
func JobRoutes(router *gin.Engine, jobHandler *handler.JobHandler, authMiddleware func(string, string) gin.HandlerFunc) {
	// Create a job routes group
	jobRoutes := router.Group("/jobs")
	{
		// Public routes - no authentication required
		jobRoutes.GET("/", jobHandler.GetJobs) // Get all jobs

		// Protected routes - require authentication
		jobRoutes.POST("/post", authMiddleware("employer", AuthServiceAddr), jobHandler.PostJob)    // Only employers can post jobs
		jobRoutes.POST("/apply", authMiddleware("candidate", AuthServiceAddr), jobHandler.ApplyToJob) // Only candidates can apply to jobs
	}
}
