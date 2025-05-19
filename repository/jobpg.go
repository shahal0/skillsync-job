package repository

import (
	"context"
	"jobservice/domain/models"
	"jobservice/domain/repository"
	"time"

	"gorm.io/gorm"
)

type jobPG struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) repository.JobRepository {
	return &jobPG{db: db}
}

func (r *jobPG) PostJob(ctx context.Context, job *models.Job,employerid string) error {
	job.EmployerID = employerid
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *jobPG) GetJobs(ctx context.Context, filters map[string]interface{}) ([]models.Job, error) {
	var jobs []models.Job
	query := r.db.WithContext(ctx).Model(&models.Job{})

	// Apply filters if any
	if category, ok := filters["category"]; ok {
		query = query.Where("category = ?", category)
	}
	if keyword, ok := filters["keyword"]; ok {
		query = query.Where("? = ANY(keywords)", keyword)
	}

	err := query.Find(&jobs).Error
	return jobs, err
}

func (r *jobPG) ApplyToJob(ctx context.Context, candidateid string,jobid string) error {
	var app models.Application
	app.Status = "pending"
	app.JobID = jobid
	app.CandidateID = candidateid
	app.AppliedAt=time.Now()
	return r.db.WithContext(ctx).Create(&app).Error
}
func (r *jobPG) AddJobSkills(ctx context.Context, skills []models.JobSkill) error {
    return r.db.WithContext(ctx).Create(&skills).Error
}