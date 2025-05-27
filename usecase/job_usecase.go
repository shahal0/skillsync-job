package usecase

import (
	"context"
	"errors"
	"fmt"
	"jobservice/domain/models"
	"jobservice/domain/repository"
	"jobservice/middleware"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/shahal0/skillsync-protos/gen/authpb"
)

type JobUsecase struct {
	jobRepo    repository.JobRepository
	AuthClient authpb.AuthServiceClient
}

func NewJobUsecase(repo repository.JobRepository, authClient authpb.AuthServiceClient) *JobUsecase {
	return &JobUsecase{jobRepo: repo, AuthClient: authClient}
}

func (uc *JobUsecase) PostJob(ctx context.Context, job *models.Job, employerid string) error {
	// Fetch EmployerID from the context
	if employerid == "" {
		return errors.New("failed to fetch employer ID from token")
	}

	// Set EmployerID in the job model

	return uc.jobRepo.PostJob(ctx, job, employerid)
}

func (uc *JobUsecase) GetJobs(ctx context.Context, filters map[string]interface{}) ([]models.Job, error) {
	return uc.jobRepo.GetJobs(ctx, filters)
}

func (uc *JobUsecase) ApplyToJob(ctx context.Context, candidateID string, jobid string) (string, error) {
	return uc.jobRepo.ApplyToJob(ctx, candidateID, jobid)
}

func (uc *JobUsecase) AddJobSkills(ctx context.Context, skills []models.JobSkill) error {
	return uc.jobRepo.AddJobSkills(ctx, skills)
}

// UpdateJobStatus updates the status of a job if the requester is the employer who posted it
func (uc *JobUsecase) UpdateJobStatus(ctx context.Context, jobID string, employerID string, status string) error {
	// Validate status is one of the allowed values
	validStatuses := map[string]bool{
		"OPEN":        true,
		"IN_PROGRESS": true,
		"COMPLETED":   true,
		"CANCELLED":   true,
	}

	if !validStatuses[status] {
		return errors.New("invalid status. Must be one of: OPEN, IN_PROGRESS, COMPLETED, CANCELLED")
	}

	// Validate jobID is not empty
	if jobID == "" {
		return errors.New("job ID cannot be empty")
	}

	// Validate employerID is not empty
	if employerID == "" {
		return errors.New("employer ID cannot be empty")
	}

	return uc.jobRepo.UpdateJobStatus(ctx, jobID, employerID, status)
}

// VerifyToken implements the TokenVerifier interface
func (uc *JobUsecase) VerifyToken(token string) (*middleware.Claims, error) {
	// Handle Bearer token format if present
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	// Use the same JWT secret as the Auth Service
	jwtSecretStr := "your_jwt_secret" // Must match the value in Auth Service main.go

	jwtSecret := []byte(jwtSecretStr)

	// Parse and validate the token
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !parsedToken.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Extract user ID and role
	userID, ok1 := claims["user_id"].(string)
	role, ok2 := claims["role"].(string)

	if !ok1 || !ok2 {
		return nil, errors.New("missing claims")
	}

	return &middleware.Claims{
		UserID: userID,
		Role:   role,
	}, nil
}

func (uc *JobUsecase) GetJobByID(ctx context.Context, jobID string) (*models.Job, error) {
	return uc.jobRepo.GetJobByID(ctx, jobID)
}

func (uc *JobUsecase) GetApplicationsByCandidate(ctx context.Context, candidateID string, status string) ([]models.ApplicationResponse, error) {
	// Validate candidate ID
	if candidateID == "" {
		return nil, errors.New("candidate ID is required")
	}

	// Get applications from repository
	return uc.jobRepo.GetApplicationsByCandidate(ctx, candidateID, status)
}

func (uc *JobUsecase) GetApplicationByID(ctx context.Context, applicationID uint) (*models.ApplicationResponse, error) {
	// Validate application ID
	if applicationID == 0 {
		return nil, errors.New("application ID is required")
	}

	// Get application from repository
	return uc.jobRepo.GetApplicationByID(ctx, applicationID)
}

// FilterApplicationsByJob filters and ranks applications for a specific job based on various criteria
func (uc *JobUsecase) FilterApplicationsByJob(ctx context.Context, jobID string, filterOptions map[string]interface{}) ([]models.RankedApplication, error) {
	// Convert string jobID to uint64
	jobIDUint, err := strconv.ParseUint(jobID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid job ID format: %v", err)
	}

	// Call repository method
	return uc.jobRepo.FilterApplicationsByJob(ctx, jobIDUint, filterOptions, uc.AuthClient)
}
