package routes

import (
	"jobservice/delivery/handler"

	"github.com/gin-gonic/gin"
)

func JobRoutes(router *gin.Engine, jobHandler *handler.JobHandler, authMiddleware func(string, string) gin.HandlerFunc) {
	jobRoutes := router.Group("/jobs")
	{
		jobRoutes.POST("/post", authMiddleware("employer", "localhost:50051"), jobHandler.PostJob)
		jobRoutes.GET("/", jobHandler.GetJobs)
		jobRoutes.POST("/apply", authMiddleware("candidate", "localhost:50051"), jobHandler.ApplyToJob)
	}
}
