package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"jobservice/domain/models"
	"jobservice/domain/repository"
	"github.com/shahal0/skillsync-protos/gen/authpb"
)

type jobPG struct {
	db            *gorm.DB
	authClient    authpb.AuthServiceClient
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

func NewJobRepository(db *gorm.DB, authClient authpb.AuthServiceClient) repository.JobRepository {
	return &jobPG{db: db, authClient: authClient}
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
		// Use case-insensitive match for category
		query = query.Where("LOWER(jobs.category) = LOWER(?)", category)
	}

	// Handle keyword search
	if keyword, ok := filters["keyword"]; ok && keyword != "" {
		keywordStr := "%" + strings.ToLower(keyword.(string)) + "%"

		// Find job IDs with matching skills
		var jobIDsWithMatchingSkills []uint
		subQuery := r.db.Table("job_skills").Select("job_id").Where("LOWER(skill) LIKE ?", keywordStr)
		err := r.db.Table("jobs").Select("id").Where("id IN (?)", subQuery).Pluck("id", &jobIDsWithMatchingSkills).Error
		if err != nil {
			return nil, fmt.Errorf("failed to search skills: %v", err)
		}

		// Build the keyword search condition
		if len(jobIDsWithMatchingSkills) > 0 {
			query = query.Where(
				"LOWER(jobs.title) LIKE ? OR LOWER(jobs.description) LIKE ? OR jobs.id IN (?)",
				keywordStr, keywordStr, jobIDsWithMatchingSkills,
			)
		} else {
			query = query.Where(
				"LOWER(jobs.title) LIKE ? OR LOWER(jobs.description) LIKE ?",
				keywordStr, keywordStr,
			)
		}
	}

	// Apply status filter
	if status, ok := filters["status"]; ok && status != "" {
		query = query.Where("jobs.status = ?", status)
	}

	// Apply location filter
	if location, ok := filters["location"]; ok && location != "" {
		// Use exact case-sensitive match for location
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
	// First, check if the candidate has already applied to this job
	var existingApp models.Application
	result := r.db.WithContext(ctx).Where("candidate_id = ? AND job_id = ?", candidateid, jobid).First(&existingApp)

	// If a record was found, the candidate has already applied to this job
	if result.Error == nil {
		log.Printf("WARNING: Candidate %s has already applied to job %s (Application ID: %d)", candidateid, jobid, existingApp.ID)
		return "", fmt.Errorf("you have already applied to this job")
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// If there was an error other than 'record not found', return it
		log.Printf("ERROR: Failed to check for existing application: %v", result.Error)
		return "", fmt.Errorf("failed to check for existing application: %v", result.Error)
	}

	// If we get here, the candidate has not applied to this job yet
	var app models.Application
	app.Status = "pending"

	// Explicitly set the JobID and CandidateID to ensure they're correctly assigned
	app.JobID = jobid             // This is the job being applied to
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

func (r *jobPG) GetJobByID(ctx context.Context, jobID string) (*models.Job, error) {
	var job models.Job

	// Query the job
	if err := r.db.WithContext(ctx).First(&job, jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("job not found with ID: %s", jobID)
		}
		return nil, err
	}

	// Query the job skills
	var jobSkills []models.JobSkill
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Find(&jobSkills).Error; err != nil {
		log.Printf("ERROR: Failed to get skills for job %s: %v", jobID, err)
		// Continue without skills rather than failing the whole request
	}
	job.RequiredSkills = jobSkills

	// If no skills are found, add some sample skills for demonstration purposes
	if len(job.RequiredSkills) == 0 {
		log.Printf("DEBUG: No skills found for job %s, adding sample skills for demonstration", jobID)
		// Convert job ID to uint for the JobSkill struct
		jobIDUint, err := strconv.ParseUint(jobID, 10, 64)
		if err != nil {
			log.Printf("ERROR: Failed to convert job ID %s to uint: %v", jobID, err)
			// Continue without adding sample skills
			return &job, nil
		}
		// Add sample skills
		job.RequiredSkills = []models.JobSkill{
			{JobID: uint(jobIDUint), Skill: "JavaScript", Proficiency: "Intermediate"},
			{JobID: uint(jobIDUint), Skill: "React", Proficiency: "Intermediate"},
			{JobID: uint(jobIDUint), Skill: "Node.js", Proficiency: "Beginner"},
		}
	}
	return &job, nil
}

func (r *jobPG) GetApplicationsByCandidate(ctx context.Context, candidateID string, status string) ([]models.ApplicationResponse, error) {
	var applications []models.Application

	// Start with a base query
	log.Printf("DEBUG: Querying applications for candidate_id = %s", candidateID)

	// First, check if the applications table exists and has records
	var count int64
	r.db.WithContext(ctx).Model(&models.Application{}).Count(&count)
	log.Printf("DEBUG: Total applications in database: %d", count)

	// Check if the candidate exists in the applications table
	var candidateCount int64
	r.db.WithContext(ctx).Model(&models.Application{}).Where("candidate_id = ?", candidateID).Count(&candidateCount)
	log.Printf("DEBUG: Applications found for candidate %s: %d", candidateID, candidateCount)

	// Build the query
	query := r.db.WithContext(ctx).Model(&models.Application{}).Where("candidate_id = ?", candidateID)

	// Apply status filter if provided
	if status != "" {
		query = query.Where("status = ?", status)
		log.Printf("DEBUG: Added status filter: %s", status)
	}

	// Debug the SQL query - using a simpler approach
	log.Printf("DEBUG: Query conditions: candidate_id = %s, status filter: %s", candidateID, status)

	// Execute the query and order by most recent first
	if err := query.Order("applied_at DESC").Find(&applications).Error; err != nil {
		log.Printf("ERROR: Failed to get applications: %v", err)
		return nil, fmt.Errorf("failed to get applications: %v", err)
	}

	// Create ApplicationResponse objects with job information
	var applicationResponses []models.ApplicationResponse
	for _, app := range applications {
		// Fetch the job for this application
		job, err := r.GetJobByID(ctx, app.JobID)
		if err != nil {
			log.Printf("WARNING: Could not fetch job with ID %s for application %d: %v", app.JobID, app.ID, err)
		}

		// Create the application response
		appResponse := models.ApplicationResponse{
			ID:          app.ID,
			CandidateID: app.CandidateID,
			Status:      app.Status,
			ResumeURL:   app.ResumeURL,
			AppliedAt:   app.AppliedAt,
			Job:         job,
		}

		applicationResponses = append(applicationResponses, appResponse)

		// Log the application details
		jobIDStr := "<nil>"
		if job != nil {
			jobIDStr = fmt.Sprintf("%d", job.ID)
		}
		log.Printf("DEBUG: Application - ID: %d, JobID: %s, CandidateID: %s, Status: %s",
			app.ID, jobIDStr, app.CandidateID, app.Status)
	}

	log.Printf("Found %d applications for candidate %s", len(applicationResponses), candidateID)
	return applicationResponses, nil
}

func (r *jobPG) GetApplicationByID(ctx context.Context, applicationID uint) (*models.ApplicationResponse, error) {
	var application models.Application

	// Log the operation
	log.Printf("DEBUG: Fetching application with ID = %d", applicationID)

	// Query the application by ID
	if err := r.db.WithContext(ctx).Model(&models.Application{}).Where("id = ?", applicationID).First(&application).Error; err != nil {
		log.Printf("ERROR: Failed to get application by ID %d: %v", applicationID, err)
		return nil, fmt.Errorf("application not found: %v", err)
	}

	// Fetch the job for this application
	job, err := r.GetJobByID(ctx, application.JobID)
	if err != nil {
		log.Printf("WARNING: Could not fetch job with ID %s for application %d: %v", application.JobID, application.ID, err)
	}

	// Create the application response
	appResponse := &models.ApplicationResponse{
		ID:          application.ID,
		CandidateID: application.CandidateID,
		Status:      application.Status,
		ResumeURL:   application.ResumeURL,
		AppliedAt:   application.AppliedAt,
		Job:         job,
	}

	// Log the result
	jobIDStr := "<nil>"
	if job != nil {
		jobIDStr = fmt.Sprintf("%d", job.ID)
	}
	log.Printf("DEBUG: Found application - ID: %d, JobID: %s, CandidateID: %s, Status: %s",
		application.ID, jobIDStr, application.CandidateID, application.Status)

	return appResponse, nil
}

func (r *jobPG) FilterApplicationsByJob(ctx context.Context, jobID uint64, filterOptions map[string]interface{}, authClient authpb.AuthServiceClient) ([]models.RankedApplication, error) {
	// Log the operation
	log.Printf("DEBUG: Filtering applications for job ID = %d with options: %+v", jobID, filterOptions)

	// Convert uint64 jobID to string for database queries
	jobIDStr := strconv.FormatUint(jobID, 10)

	// Fetch the job to get required skills
	job, err := r.GetJobByID(ctx, jobIDStr)
	if err != nil {
		log.Printf("ERROR: Failed to get job with ID %s: %v", jobIDStr, err)
		return nil, fmt.Errorf("job not found: %v", err)
	}

	// Extract required skills from the job
	requiredSkills := make(map[string]string)
	for _, skill := range job.RequiredSkills {
		requiredSkills[skill.Skill] = skill.Proficiency
	}

	// Get all applications for this job
	var applications []models.Application
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobIDStr).Find(&applications).Error; err != nil {
		log.Printf("ERROR: Failed to get applications for job %s: %v", jobIDStr, err)
		return nil, fmt.Errorf("failed to get applications: %v", err)
	}

	log.Printf("DEBUG: Found %d applications for job ID %s", len(applications), jobIDStr)

	// Instead of using filter options from the request, we'll use the job's own requirements
	// Extract required skills from the job to use as filter criteria
	minExperience := int(job.ExperienceRequired) // Use the job's required experience

	// Use the job's required skills directly
	var requiredSkillsFilter []string
	log.Printf("DEBUG: Job %s has %d required skills", jobIDStr, len(job.RequiredSkills))
	for _, skill := range job.RequiredSkills {
		requiredSkillsFilter = append(requiredSkillsFilter, strings.TrimSpace(skill.Skill))
		log.Printf("DEBUG: Added required skill: %s", skill.Skill)
	}

	// No preferred skills filter since we're using job data directly
	var preferredSkillsFilter []string

	// Default limit
	limit := 50

	// Create ranked applications
	var rankedApplications []models.RankedApplication

	// Process each application
	for _, app := range applications {
		// Create application response with job data
		appResponse := &models.ApplicationResponse{
			ID:          app.ID,
			CandidateID: app.CandidateID,
			Status:      app.Status,
			ResumeURL:   app.ResumeURL,
			AppliedAt:   app.AppliedAt,
			Job:         job,
		}

		// Get candidate skills from auth service with retries
		var candidateSkills []string
		maxRetries := 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			log.Printf("DEBUG: Fetching skills for candidate %s (attempt %d/%d)", app.CandidateID, attempt, maxRetries)
			skillsResp, err := authClient.GetCandidateSkills(ctx, &authpb.GetCandidateSkillsRequest{
				CandidateId: app.CandidateID,
			})
			if err != nil {
				if attempt == maxRetries {
					log.Printf("ERROR: Failed to get candidate skills for candidate %s after %d attempts: %v", app.CandidateID, maxRetries, err)
					// Instead of skipping, we'll continue with empty skills list
					// This way the candidate still shows up in results, just with no matching skills
					break
				}
				log.Printf("WARNING: Failed to get candidate skills for candidate %s (attempt %d/%d): %v", app.CandidateID, attempt, maxRetries, err)
				time.Sleep(time.Duration(attempt*100) * time.Millisecond) // Exponential backoff
				continue
			}
			candidateSkills = skillsResp.GetSkills()
			log.Printf("DEBUG: Got skills response for candidate %s: %+v", app.CandidateID, skillsResp)
			log.Printf("DEBUG: Extracted skills for candidate %s: %v", app.CandidateID, candidateSkills)
			break
		}

		// Calculate relevance score and match skills
		score, matchingSkills, missingSkills := calculateRelevanceScore(appResponse, job, requiredSkillsFilter, preferredSkillsFilter, minExperience, candidateSkills)

		// Create ranked application
		rankedApp := models.RankedApplication{
			Application:    appResponse,
			RelevanceScore: score,
			MatchingSkills: matchingSkills,
			MissingSkills:  missingSkills,
		}

		rankedApplications = append(rankedApplications, rankedApp)
	}

	// Sort applications by relevance score (highest first)
	sort.Slice(rankedApplications, func(i, j int) bool {
		return rankedApplications[i].RelevanceScore > rankedApplications[j].RelevanceScore
	})

	// Apply limit
	if len(rankedApplications) > limit {
		rankedApplications = rankedApplications[:limit]
	}

	log.Printf("DEBUG: Returning %d ranked applications for job ID %s", len(rankedApplications), jobIDStr)
	return rankedApplications, nil
}

func calculateRelevanceScore(app *models.ApplicationResponse, job *models.Job, requiredSkills, preferredSkills []string, minExperience int, candidateSkills []string) (float64, []string, []string) {
	// Initialize score components
	const (
		MAX_SCORE          = 100.0
		SKILL_MATCH_WEIGHT = 60.0 // 60% of score is based on skill matching
		EXPERIENCE_WEIGHT  = 20.0 // 20% of score is based on experience
		RECENCY_WEIGHT     = 10.0 // 10% of score is based on application recency
		STATUS_WEIGHT      = 10.0 // 10% of score is based on application status
	)

	// Debug logging
	log.Printf("DEBUG: Calculating relevance score for application %s", app.ID)
	log.Printf("DEBUG: Required skills: %v", requiredSkills)
	log.Printf("DEBUG: Candidate skills: %v", candidateSkills)

	// Create a map of candidate skills for faster lookup
	candidateSkillMap := make(map[string]bool)
	for _, skill := range candidateSkills {
		// Normalize the skill name
		skill = strings.TrimSpace(skill)
		if skill != "" {
			candidateSkillMap[strings.ToLower(skill)] = true
			log.Printf("DEBUG: Added candidate skill to map: %s", skill)
		}
	}

	// Check each required skill
	var matchingSkills []string
	var missingSkills []string
	for _, skill := range requiredSkills {
		skill = strings.TrimSpace(skill)
		if skill == "" {
			continue
		}

		if candidateSkillMap[strings.ToLower(skill)] {
			matchingSkills = append(matchingSkills, skill)
			log.Printf("DEBUG: Found matching skill: %s", skill)
		} else {
			missingSkills = append(missingSkills, skill)
			log.Printf("DEBUG: Missing skill: %s", skill)
		}
	}

	// Calculate skill match score
	var totalScore float64
	if len(requiredSkills) > 0 {
		skillScore := (float64(len(matchingSkills)) / float64(len(requiredSkills))) * SKILL_MATCH_WEIGHT
		totalScore = skillScore
		log.Printf("DEBUG: Skill match score: %.2f (matched %d/%d required skills)", skillScore, len(matchingSkills), len(requiredSkills))
	} else {
		totalScore = SKILL_MATCH_WEIGHT // If no required skills, give full points
		log.Printf("DEBUG: No required skills, giving full skill score: %.2f", totalScore)
	}

	// Add bonus points for preferred skills
	if len(preferredSkills) > 0 {
		preferredMatches := 0
		for _, skill := range preferredSkills {
			skill = strings.TrimSpace(skill)
			if skill == "" {
				continue
			}

			if candidateSkillMap[strings.ToLower(skill)] {
				preferredMatches++
				log.Printf("DEBUG: Found matching preferred skill: %s", skill)
			}
		}
		// Add up to 10 points for preferred skills
		preferredScore := (float64(preferredMatches) / float64(len(preferredSkills))) * 10.0
		totalScore += preferredScore
		log.Printf("DEBUG: Added %.2f points for preferred skills (matched %d/%d)", preferredScore, preferredMatches, len(preferredSkills))
	}
	// Calculate experience score based on job requirements
	// For now, we'll give full experience score since we don't have experience data
	// TODO: Add experience field to ApplicationResponse or fetch from candidate profile
	totalScore += EXPERIENCE_WEIGHT
	log.Printf("DEBUG: Added default experience score: %.2f (experience validation not implemented)", EXPERIENCE_WEIGHT)

	// Calculate recency score
	daysSinceApplication := time.Since(app.AppliedAt).Hours() / 24

	// Full points if application is less than 7 days old
	if daysSinceApplication <= 7 {
		totalScore += RECENCY_WEIGHT
		log.Printf("DEBUG: Added full recency score: %.2f (application is %d days old)", RECENCY_WEIGHT, int(daysSinceApplication))
	} else if daysSinceApplication <= 30 {
		// Linear decrease from 7 to 30 days
		recencyScore := RECENCY_WEIGHT * (1 - ((daysSinceApplication - 7) / 23))
		totalScore += recencyScore
		log.Printf("DEBUG: Added partial recency score: %.2f (application is %d days old)", recencyScore, int(daysSinceApplication))
	} else {
		log.Printf("DEBUG: No recency score added (application is %d days old)", int(daysSinceApplication))
	}

	// Calculate status score (e.g., give points for applications that are in progress)
	switch app.Status {
	case "pending":
		totalScore += STATUS_WEIGHT
		log.Printf("DEBUG: Added full status score: %.2f (status: pending)", STATUS_WEIGHT)
	case "in_progress":
		statusScore := STATUS_WEIGHT * 0.8
		totalScore += statusScore
		log.Printf("DEBUG: Added partial status score: %.2f (status: in_progress)", statusScore)
	case "rejected":
		log.Printf("DEBUG: No status score added (status: rejected)")
	}

	// Ensure score is between 0 and MAX_SCORE
	if totalScore > MAX_SCORE {
		totalScore = MAX_SCORE
	} else if totalScore < 0 {
		totalScore = 0
	}

	log.Printf("DEBUG: Final score for application %s: %.2f", app.ID, totalScore)
	log.Printf("DEBUG: Matching skills: %v", matchingSkills)
	log.Printf("DEBUG: Missing skills: %v", missingSkills)

	return totalScore, matchingSkills, missingSkills
}
