package repository

import (
	"context"
	"jobservice/domain/models"
	"github.com/shahal0/skillsync-protos/gen/authpb"
)

type JobRepository interface {
	PostJob(ctx context.Context, job *models.Job, employerid string) error
	
	// Original method maintained for backward compatibility
	GetJobs(ctx context.Context, filters map[string]interface{}) ([]models.Job, error)
	
	// New method with pagination support
	GetJobsWithPagination(ctx context.Context, filters map[string]interface{}) ([]models.Job, int64, error)
	
	ApplyToJob(ctx context.Context, candidateid string, jobid string) (string, error)
	AddJobSkills(ctx context.Context, skills []models.JobSkill) error
	UpdateJobStatus(ctx context.Context, jobID string, employerID string, status string) error
	GetJobByID(ctx context.Context, jobID string) (*models.Job, error)
	
	// Original methods maintained for backward compatibility
	GetApplicationsByCandidate(ctx context.Context, candidateID string, status string) ([]models.ApplicationResponse, error)
	GetApplicationByID(ctx context.Context, applicationID uint) (*models.ApplicationResponse, error)
	GetApplicationsByJob(ctx context.Context, jobID string, status string) ([]models.ApplicationResponse, error)
	
	// New methods with pagination support
	GetApplicationsByCandidateWithPagination(ctx context.Context, candidateID string, status string, page, limit int32) ([]models.ApplicationResponse, int64, error)
	GetApplicationsByJobWithPagination(ctx context.Context, jobID string, status string, page, limit int32) ([]models.ApplicationResponse, int64, error)
	
	FilterApplicationsByJob(ctx context.Context, jobID uint64, filterOptions map[string]interface{}, authClient authpb.AuthServiceClient) ([]models.RankedApplication, error)
}
