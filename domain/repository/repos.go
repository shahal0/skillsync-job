package repository

import (
	"context"
	models"jobservice/domain/models"
)

type JobRepository interface {
	PostJob(ctx context.Context, job *models.Job,employerid string) error
	GetJobs(ctx context.Context, filters map[string]interface{}) ([]models.Job, error)
	ApplyToJob(ctx context.Context,candidateid string,jobid string) error
	AddJobSkills(ctx context.Context, skills []models.JobSkill) error
}
