package usecase

import (
	"context"
	"errors"
	"jobservice/domain/models"
	"jobservice/domain/repository"
)

type JobUsecase struct {
	jobRepo repository.JobRepository
}

func NewJobUsecase(repo repository.JobRepository) *JobUsecase {
	return &JobUsecase{jobRepo: repo}
}

func (uc *JobUsecase) PostJob(ctx context.Context, job *models.Job,employerid string) error {
	// Fetch EmployerID from the context
	if  employerid == "" {
		return errors.New("failed to fetch employer ID from token")
	}

	// Set EmployerID in the job model

	return uc.jobRepo.PostJob(ctx, job,employerid)
}

func (uc *JobUsecase) GetJobs(ctx context.Context, filters map[string]interface{}) ([]models.Job, error) {
	return uc.jobRepo.GetJobs(ctx, filters)
}

func (uc *JobUsecase) ApplyToJob(ctx context.Context, candidateID string,jobid string) error {
	// Fetch CandidateID from the context
	if candidateID == "" {
		return errors.New("failed to fetch candidate ID from token")
	}



	return uc.jobRepo.ApplyToJob(ctx, candidateID,jobid)
}

func (uc *JobUsecase) AddJobSkills(ctx context.Context, skills []models.JobSkill) error {
	return uc.jobRepo.AddJobSkills(ctx, skills)
}
