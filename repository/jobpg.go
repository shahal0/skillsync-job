package repository

import (
	"context"
	"errors"
	"fmt"
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

// UpdateJobStatus updates the status of a job if the requester is the employer who posted it
func (r *jobPG) UpdateJobStatus(ctx context.Context, jobID string, employerID string, status string) error {
	// Start a transaction
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	// First, verify the job exists and is owned by the employer
	var job models.Job
	if err := tx.First(&job, "id = ? AND employer_id = ?", jobID, employerID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("job not found or you don't have permission to update it")
		}
		return fmt.Errorf("failed to verify job ownership: %v", err)
	}

	// Update the job status
	if err := tx.Model(&models.Job{}).Where("id = ?", jobID).
		Update("status", status).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update job status: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Successfully updated status of job ID %s to %s", jobID, status)
	return nil
}
func NewJobRepository(db *gorm.DB) repository.JobRepository {
	return &jobPG{db: db}
}

func (r *jobPG) PostJob(ctx context.Context, job *models.Job, employerid string) error {
	// Start a transaction
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Set employer ID and create the job
	job.EmployerID = employerid
	if err := tx.Create(job).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create job: %v", err)
	}

	// If there are required skills, create them
	if len(job.RequiredSkills) > 0 {
		for i := range job.RequiredSkills {
			job.RequiredSkills[i].JobID = job.ID
			if err := tx.Create(&job.RequiredSkills[i]).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create job skill: %v", err)
			}
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (r *jobPG) GetJobs(ctx context.Context, filters map[string]interface{}) ([]models.Job, error) {
	var jobs []models.Job

	// Start with a base query
	query := r.db.WithContext(ctx).Model(&models.Job{})

	// Apply filters
	if category, ok := filters["category"]; ok && category != "" {
		query = query.Where("jobs.category = ?", category)
	}

	// Handle keyword search
	if keyword, ok := filters["keyword"]; ok && keyword != "" {
		keywordStr := "%" + keyword.(string) + "%"
		
		// Find job IDs with matching skills
		var jobIDsWithMatchingSkills []uint
		subQuery := r.db.Table("job_skills").Select("job_id").Where("skill ILIKE ?", keywordStr)
		err := r.db.Table("jobs").Select("id").Where("id IN (?)", subQuery).Pluck("id", &jobIDsWithMatchingSkills).Error
		if err != nil {
			return nil, fmt.Errorf("failed to search skills: %v", err)
		}

		// Build the keyword search condition
		if len(jobIDsWithMatchingSkills) > 0 {
			query = query.Where("jobs.title ILIKE ? OR jobs.description ILIKE ? OR jobs.id IN (?)", 
				keywordStr, keywordStr, jobIDsWithMatchingSkills)
		} else {
			query = query.Where("jobs.title ILIKE ? OR jobs.description ILIKE ?", keywordStr, keywordStr)
		}
	}

	// Apply location filter
	if location, ok := filters["location"]; ok && location != "" {
		query = query.Where("jobs.location = ?", location)
	}

	// Order by most recent first
	query = query.Order("jobs.created_at DESC")

	// Execute query with preloading of required skills
	err := query.Preload("RequiredSkills").Find(&jobs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs: %v", err)
	}

	return jobs, nil
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
	// Validate that we have skills to add
	if len(skills) == 0 {
		return errors.New("no skills provided to add")
	}

	// Start a transaction
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	// Get the job ID from the first skill (all skills should be for the same job)
	jobID := skills[0].JobID

	// Check if the job exists
	var job models.Job
	if err := tx.First(&job, jobID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("job with ID %d not found", jobID)
		}
		return fmt.Errorf("failed to verify job: %v", err)
	}

	// Log the operation
	log.Printf("Adding %d skills for job ID: %d", len(skills), jobID)

	// Add the new skills
	for i := range skills {
		skills[i].ID = 0 // Ensure we're creating new records
		if err := tx.Create(&skills[i]).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to add job skill: %v", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	
	if len(skills) > 0 {
		log.Printf("Successfully added %d skills for job ID: %d", len(skills), skills[0].JobID)
	}
	return nil
}