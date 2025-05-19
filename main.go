package main

import (
	"jobservice/config"
	"jobservice/delivery/handler"
	"jobservice/delivery/routes"
	"jobservice/domain/models"
	"jobservice/middleware"
	"jobservice/repository"
	"jobservice/usecase"

	"github.com/gin-gonic/gin"
)

func main() {
	db := config.ConnectDB()

	// Run migrations
	err := db.AutoMigrate(&models.Job{}, &models.Application{}, &models.Jobskills{})
	if err != nil {
		panic("Migration failed: " + err.Error())
	}

	// Dependency injection
	jobRepo := repository.NewJobRepository(db)
	jobUC := usecase.NewJobUsecase(jobRepo)
	authMiddleware := middleware.AuthMiddleware
	jobHandler := handler.NewJobHandler(jobUC)

	// Setup Gin router
	r := gin.Default()

	// Register routes from routes package
	routes.JobRoutes(r, jobHandler, authMiddleware)

	// Start gRPC server in a separate goroutine
	go func() {
		middleware.StartGRPCServer("localhost:50051")
	}()

	// Start HTTP server
	err = r.Run(":8002")
	if err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
