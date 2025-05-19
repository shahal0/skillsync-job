package middleware

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/shahal0/skillsync-protos/gen/authpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func AuthMiddleware(requiredRole string, authServiceAddr string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		conn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to AuthService: " + err.Error()})
			c.Abort()
			return
		}
		defer conn.Close()

		client := authpb.NewAuthServiceClient(conn)

		res, err := client.VerifyToken(context.Background(), &authpb.VerifyTokenRequest{
			Token: token,
		})
		log.Printf("VerifyToken response: %+v, error: %v", res, err)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		if res.Role != requiredRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			c.Abort()
			return
		}

		// Store user info in Gin context if needed
		c.Set("user_id", res.UserId)
		c.Set("role", res.Role)

		c.Next()
	}
}

type AuthGRPCServer struct {
	authpb.UnimplementedAuthServiceServer
	CandidateUsecase interface{}
}

// VerifyToken is a placeholder implementation to satisfy the authpb.AuthServiceServer interface.
func (s *AuthGRPCServer) VerifyToken(ctx context.Context, req *authpb.VerifyTokenRequest) (*authpb.VerifyTokenResponse, error) {
	// Implement actual token verification logic here
	// For example, decode the JWT token and extract user_id and role
	token := req.Token
	userId, role, err := decodeToken(token) // Replace with your actual token decoding logic
	if err != nil {
		return nil, err
	}

	return &authpb.VerifyTokenResponse{
		UserId: userId,
		Role:   role,
	}, nil
}

var jwtSecret = []byte("your_jwt_secret") // Make sure this matches the one used to sign tokens

func decodeToken(tokenStr string) (string, string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		// You can add algorithm check here
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return "", "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid token claims")
	}

	userID, ok1 := claims["user_id"].(string)
	role, ok2 := claims["role"].(string)

	if !ok1 || !ok2 {
		return "", "", errors.New("missing claims")
	}

	return userID, role, nil
}

// GetCandidateDetails is a placeholder implementation to satisfy the authpb.AuthServiceServer interface.
func (s *AuthGRPCServer) GetCandidateDetails(ctx context.Context, req *authpb.CandidateRequest) (*authpb.CandidateResponse, error) {
	// Implement the actual logic here or return a placeholder response.
	return &authpb.CandidateResponse{}, nil
}

// GetEmployerDetails is a placeholder implementation to satisfy the authpb.AuthServiceServer interface.
func (s *AuthGRPCServer) GetEmployerDetails(ctx context.Context, req *authpb.EmployerRequest) (*authpb.EmployerResponse, error) {
	return &authpb.EmployerResponse{}, nil
}

func StartGRPCServer(candidateUsecase interface{}) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServiceServer(grpcServer, &AuthGRPCServer{CandidateUsecase: candidateUsecase})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("Shutting down server...")
		grpcServer.GracefulStop()
		os.Exit(0)
	}()

	log.Println("gRPC server is running on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
