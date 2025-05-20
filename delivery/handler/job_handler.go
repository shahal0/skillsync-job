package handler

import (
	"jobservice/domain/models"
	"jobservice/usecase"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shahal0/skillsync-protos/gen/jobpb"
)

type AuthClient interface {
	GetUserRole(userID string) (string, error)
}

type JobHandler struct {
	usecase *usecase.JobUsecase
}

func NewJobHandler(uc *usecase.JobUsecase) *JobHandler {
	return &JobHandler{usecase: uc}
}

// PostJob ensures only employers can post jobs
func (h *JobHandler) PostJob(c *gin.Context) {
	employerID, ok := c.Get("user_id")
	if !ok || employerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Missing employer ID"})
		return
	}
	
	// Convert employerID to string
	employerIDStr, ok := employerID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid employer ID format"})
		return
	}
	
	// Parse request body into jobpb model
	var jobRequest jobpb.PostJobRequest
	if err := c.ShouldBindJSON(&jobRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job data: " + err.Error()})
		return
	}
	
	// Set the employer ID in the job model
	jobRequest.EmployerId = employerIDStr
	
	// Convert jobpb model to domain model
	job := &models.Job{
		Title:              jobRequest.Title,
		Description:        jobRequest.Description,
		Category:           jobRequest.Category,
		SalaryMin:          jobRequest.SalaryMin,
		SalaryMax:          jobRequest.SalaryMax,
		Location:           jobRequest.Location,
		ExperienceRequired: int(jobRequest.ExperienceRequired),
		EmployerID:         employerIDStr,
	}
	
	// Call the usecase to create the job
	if err := h.usecase.PostJob(c.Request.Context(), job, employerIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post job: " + err.Error()})
		return
	}

	// Create response using jobpb model
	response := &jobpb.PostJobResponse{
		Message: "Job posted successfully",
	}
	
	c.JSON(http.StatusCreated, response)
}

func (h *JobHandler) GetJobs(c *gin.Context) {
	// Create request from query parameters
	request := &jobpb.GetJobsRequest{
		Category: c.Query("category"),
		Keyword: c.Query("keyword"),
		Location: c.Query("location"),
	}
	
	// Convert to filters map for usecase
	filters := map[string]interface{}{}
	if request.Category != "" {
		filters["category"] = request.Category
	}
	if request.Keyword != "" {
		filters["keyword"] = request.Keyword
	}
	if request.Location != "" {
		filters["location"] = request.Location
	}

	// Call usecase
	jobs, err := h.usecase.GetJobs(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve jobs: " + err.Error()})
		return
	}

	// Convert domain models to protobuf models
	response := &jobpb.GetJobsResponse{
		Jobs: make([]*jobpb.Job, 0, len(jobs)),
	}
	
	for _, job := range jobs {
		pbJob := &jobpb.Job{
			Id: strconv.FormatUint(uint64(job.ID), 10),
			EmployerId: job.EmployerID,
			Title: job.Title,
			Description: job.Description,
			Category: job.Category,
			SalaryMin: job.SalaryMin,
			SalaryMax: job.SalaryMax,
			Location: job.Location,
			ExperienceRequired: int32(job.ExperienceRequired),
			Status: job.Status,
		}
		response.Jobs = append(response.Jobs, pbJob)
	}
	
	c.JSON(http.StatusOK, response)
}

// ApplyToJob ensures only candidates can apply for jobs
func (h *JobHandler) ApplyToJob(c *gin.Context) {
	log.Println("ApplyToJob called")
	candidateID, ok := c.Get("user_id")
	if !ok || candidateID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Missing candidate ID"})
		return
	}
	
	candidateIDStr, ok := candidateID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid candidate ID format"})
		return
	}
	
	jobid := c.Query("job_id")	
	
	if jobid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}	
	
	// Create the request using jobpb model
	request := &jobpb.ApplyToJobRequest{
		JobId: jobid,
		CandidateId: candidateIDStr,
	}
	
	appid, err := h.usecase.ApplyToJob(c.Request.Context(), request.CandidateId, request.JobId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply to job: " + err.Error()})
		return
	}

	// Create the response using jobpb model
	response := &jobpb.ApplyToJobResponse{
		ApplicationId: appid,
		Message: "Applied to job successfully",
	}
	
	c.JSON(http.StatusOK, response)
}

func (h *JobHandler) AddJobSkills(c *gin.Context) {
	// Parse request using jobpb model
	var request jobpb.AddJobSkillsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid skills data: " + err.Error()})
		return
	}

	// If job_id is not in the request body, try to get it from the URL parameter
	if request.JobId == "" {
		request.JobId = c.Param("job_id")
	}

	// Convert to domain model
	skills := make([]models.JobSkill, 0, len(request.Skills))
	for _, skill := range request.Skills {
		skills = append(skills, models.JobSkill{
			JobID:      request.JobId,
			Skill:      skill.Skill,
			Proficiency: skill.Proficiency,
		})
	}

	// Call usecase
	if err := h.usecase.AddJobSkills(c.Request.Context(), skills); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add job skills: " + err.Error()})
		return
	}

	// Return response using jobpb model
	response := &jobpb.AddJobSkillsResponse{
		Message: "Job skills added successfully",
	}

	c.JSON(http.StatusOK, response)
}
