package repository

import (
	"context"
	"jobservice/domain/models"
	"github.com/shahal0/skillsync-protos/gen/authpb"
)

type JobRepository interface {
	PostJob(ctx context.Context, job *models.Job, employerid string) error
	GetJobs(ctx context.Context, filters map[string]interface{}) ([]models.Job, error)
	ApplyToJob(ctx context.Context, candidateid string, jobid string) (string, error)
	AddJobSkills(ctx context.Context, skills []models.JobSkill) error
	UpdateJobStatus(ctx context.Context, jobID string, employerID string, status string) error
	GetJobByID(ctx context.Context, jobID string) (*models.Job, error)
	GetApplicationsByCandidate(ctx context.Context, candidateID string, status string) ([]models.ApplicationResponse, error)
	GetApplicationByID(ctx context.Context, applicationID uint) (*models.ApplicationResponse, error)
	FilterApplicationsByJob(ctx context.Context, jobID uint64, filterOptions map[string]interface{}, authClient authpb.AuthServiceClient) ([]models.RankedApplication, error)
}
