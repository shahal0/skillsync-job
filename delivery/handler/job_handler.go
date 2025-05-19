package handler

import (
	"jobservice/domain/models"
	"jobservice/usecase"
	"net/http"

	//"github.com/shahal0/skillsync-protos/gen/authpb"
	"github.com/gin-gonic/gin"
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
	// Fetch EmployerID from the context
	employerID, ok := c.Get("user_id")
	if !ok || employerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to fetch employer ID from token"})
		return
	}
	var job models.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job data: " + err.Error()})
		return
	}

	employerIDStr, ok := employerID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Employer ID is not a valid string"})
		return
	}
	if err := h.usecase.PostJob(c.Request.Context(), &job, employerIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post job: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Job posted successfully"})
}

func (h *JobHandler) GetJobs(c *gin.Context) {
	filters := map[string]interface{}{}
	if category := c.Query("category"); category != "" {
		filters["category"] = category
	}
	if keyword := c.Query("keyword"); keyword != "" {
		filters["keyword"] = keyword
	}

	jobs, err := h.usecase.GetJobs(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve jobs+" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, jobs)
}

// ApplyToJob ensures only candidates can apply for jobs
func (h *JobHandler) ApplyToJob(c *gin.Context) {

	candidateID, ok := c.Get("user_id")
	if !ok || candidateID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to fetch candidate ID from token"})
		return
	}
	candidateIDStr, ok := candidateID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Candidate ID is not a valid string"})
		return
	}
	jobid := c.Query("job_id")
	if jobid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}
	if err := h.usecase.ApplyToJob(c.Request.Context(), candidateIDStr, jobid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply to job: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Applied to job successfully"})
}

func (h *JobHandler) AddJobSkills(c *gin.Context) {
	var skills []models.JobSkill
	if err := c.ShouldBindJSON(&skills); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid skills data: " + err.Error()})
		return
	}

	jobID := c.Param("job_id")
	for i := range skills {
		skills[i].JobID = jobID
	}

	if err := h.usecase.AddJobSkills(c.Request.Context(), skills); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add job skills: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job skills added successfully"})
}
