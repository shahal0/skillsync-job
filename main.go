package main

import (
	"jobservice/config"
	jobgrpc "jobservice/delivery/grpc"
	"jobservice/delivery/handler"
	"jobservice/delivery/routes"
	"jobservice/domain/models"
	"jobservice/middleware"
	"jobservice/repository"
	"jobservice/usecase"
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shahal0/skillsync-protos/gen/authpb"
	"github.com/shahal0/skillsync-protos/gen/jobpb"

	_ "net/http/pprof" // Import pprof for profiling
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	db := config.ConnectDB()

	// Run auto migrations
	log.Println("Running auto migrations...")
	err := db.AutoMigrate(&models.Job{}, &models.Application{}, &models.JobSkill{})
	if err != nil {
		panic("Migration failed: " + err.Error())
	}

	// Initialize Auth Service client with detailed logging
	log.Println("Connecting to Auth Service on localhost:50051...")
	authConn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to auth service: %v", err)
	}
	authClient := authpb.NewAuthServiceClient(authConn)
	log.Println("Successfully created Auth Service client connection")

	// Dependency injection
	jobRepo := repository.NewJobRepository(db, authClient)
	jobUC := usecase.NewJobUsecase(jobRepo, authClient)
	authMiddleware := middleware.AuthMiddleware
	jobHandler := handler.NewJobHandler(jobUC, authClient)

	// Setup Gin router
	r := gin.Default()

	// Register routes from routes package
	routes.JobRoutes(r, jobHandler, authMiddleware)

	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Create the job server implementation with job usecase
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
	
	// Start pprof HTTP server for profiling
	go func() {
		log.Println("Starting pprof profiling server on port 6061")
		if err := http.ListenAndServe("localhost:6061", nil); err != nil {
			log.Printf("Pprof server failed: %v", err)
		}
	}()

	// Start HTTP server
	err = r.Run(":8002")
	if err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
