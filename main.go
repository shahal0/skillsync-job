package main

import (
	"jobservice/config"
	jobgrpc "jobservice/delivery/grpc"
	"jobservice/delivery/handler"
	"jobservice/delivery/routes"
	"jobservice/domain/models"
	"jobservice/middleware"
	"jobservice/migrations"
	"jobservice/repository"
	"jobservice/usecase"
	"log"
	"net"

	"github.com/gin-gonic/gin"
	"github.com/shahal0/skillsync-protos/gen/jobpb"
	stdgrpc "google.golang.org/grpc"
)

func main() {
	db := config.ConnectDB()

	// Run auto migrations
	log.Println("Running auto migrations...")
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

	// Create a new gRPC server
	grpcServer := stdgrpc.NewServer()
	
	// Create the job server implementation
	jobServer := jobgrpc.NewJobServer(jobUC)
	
	// Register the job server with the gRPC server
	jobpb.RegisterJobServiceServer(grpcServer, jobServer)
	
	// Start gRPC server in a separate goroutine
	go func() {
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("Failed to listen on port 50052: %v", err)
		}
		
		log.Println("gRPC server is running on port 50052")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start HTTP server
	err = r.Run(":8002")
	if err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
