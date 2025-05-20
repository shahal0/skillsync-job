package client

import (
	"context"
	"log"
	"time"

	"github.com/shahal0/skillsync-protos/gen/authpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthClient provides methods to interact with the Auth Service
type AuthClient struct {
	conn   *grpc.ClientConn
	client authpb.AuthServiceClient
}

// NewAuthClient creates a new client to interact with the Auth Service
func NewAuthClient(authServiceAddr string) (*AuthClient, error) {
	// Set up connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to the Auth Service
	log.Printf("Connecting to Auth Service at %s", authServiceAddr)
	conn, err := grpc.DialContext(
		ctx,
		authServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	// Create the client
	client := authpb.NewAuthServiceClient(conn)
	return &AuthClient{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the connection to the Auth Service
func (c *AuthClient) Close() error {
	return c.conn.Close()
}

// VerifyToken verifies a JWT token with the Auth Service
func (c *AuthClient) VerifyToken(ctx context.Context, token string) (*authpb.VerifyTokenResponse, error) {
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Log the token being verified
	log.Printf("Verifying token with Auth Service: %s", token)

	// Call the Auth Service
	resp, err := c.client.VerifyToken(ctx, &authpb.VerifyTokenRequest{
		Token: token,
	})

	// Log the result
	if err != nil {
		log.Printf("Token verification failed: %v", err)
	} else {
		log.Printf("Token verified successfully for user %s with role %s", resp.UserId, resp.Role)
	}

	return resp, err
}

// GetCandidateDetails gets candidate details from the Auth Service using a token
func (c *AuthClient) GetCandidateDetails(ctx context.Context, token string) (*authpb.CandidateProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Log the token being used
	log.Printf("Getting candidate details with token: %s", token)

	return c.client.CandidateProfile(ctx, &authpb.CandidateProfileRequest{
		Token: token,
	})
}

// GetEmployerDetails gets employer details from the Auth Service using a token
func (c *AuthClient) GetEmployerDetails(ctx context.Context, token string) (*authpb.EmployerProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Log the token being used
	log.Printf("Getting employer details with token: %s", token)

	return c.client.EmployerProfile(ctx, &authpb.EmployerProfileRequest{
		Token: token,
	})
}
