package repository

import (
	"context"
	"jobservice/domain/models"
	"jobservice/domain/repository"
	"log"
	"strconv"
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

func (r *jobPG) ApplyToJob(ctx context.Context, candidateid string, jobid string) (string, error) {
	// Double-check that we're saving the correct IDs
	var app models.Application
	app.Status = "pending"
	
	// Explicitly set the JobID and CandidateID to ensure they're correctly assigned
	app.JobID = jobid         // This is the job being applied to
	app.CandidateID = candidateid // This is the user applying for the job
	
	// TODO: In a real implementation, you would fetch the resume URL from a user profile service
	// For now, we'll add a placeholder URL based on the candidate ID
	app.ResumeURL = "https://skillsync-resumes.example.com/" + candidateid + "/resume.pdf"
	
	app.AppliedAt = time.Now()
	
	// Log the application details before saving to help with debugging
	log.Printf("Creating application with JobID: %s, CandidateID: %s, ResumeURL: %s", 
		app.JobID, app.CandidateID, app.ResumeURL)
	
	err := r.db.WithContext(ctx).Create(&app).Error
	
	// Convert the uint ID to string for the gRPC response
	return strconv.FormatUint(uint64(app.ID), 10), err
}

func (r *jobPG) AddJobSkills(ctx context.Context, skills []models.JobSkill) error {
    return r.db.WithContext(ctx).Create(&skills).Error
}