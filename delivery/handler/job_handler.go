package handler

import (
	"fmt"
	"jobservice/domain/models"
	"jobservice/usecase"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shahal0/skillsync-protos/gen/authpb"
	"github.com/shahal0/skillsync-protos/gen/jobpb"
	"google.golang.org/grpc/metadata"
)

type AuthClient interface {
	GetUserRole(userID string) (string, error)
}

type JobHandler struct {
	usecase    *usecase.JobUsecase
	authClient authpb.AuthServiceClient
}

func NewJobHandler(uc *usecase.JobUsecase, authClient authpb.AuthServiceClient) *JobHandler {
	return &JobHandler{
		usecase:    uc,
		authClient: authClient,
	}
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
	log.Println("GetJobs called")

	// Parse request parameters
	request := &jobpb.GetJobsRequest{}
	if c.ShouldBindQuery(request) != nil {
		// If binding fails, just proceed with empty filters
		request = &jobpb.GetJobsRequest{}
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

	// Extract unique employer IDs for batch processing
	employerIDs := make(map[string]struct{})
	for _, job := range jobs {
		employerIDs[job.EmployerID] = struct{}{}
	}

	// Fetch employer profiles in batch
	employerProfiles := make(map[string]*models.EmployerProfile)
	// Store company details as arrays for each employer
	companyDetailsMap := make(map[string][]map[string]string)
	for employerID := range employerIDs {
		// Create a context with metadata for auth service
		ctx := metadata.NewOutgoingContext(
			c.Request.Context(),
			metadata.New(map[string]string{"authorization": "Bearer " + employerID}),
		)

		// Call AuthService to get employer profile
		log.Printf("Fetching employer profile for ID: %s", employerID)
		profileResp, err := h.authClient.EmployerProfile(ctx, &authpb.EmployerProfileRequest{
			Token: employerID,
		})

		if err != nil {
			log.Printf("Error fetching employer profile: %v", err)
		} else if profileResp == nil {
			log.Printf("No profile response received for employer ID: %s", employerID)
		} else {
			log.Printf("Successfully fetched profile for employer ID %s: %+v", employerID, profileResp)
			// Store employer details in a map for later use
			employerDetails := map[string]interface{}{
				"employer_id": employerID,
				"company_name": profileResp.GetCompanyName(),
				"email": profileResp.GetEmail(),
				"industry": profileResp.GetIndustry(),
				"website": profileResp.GetWebsite(),
				"location": profileResp.GetLocation(),
				"is_verified": profileResp.GetIsVerified(),
				"is_trusted": profileResp.GetIsTrusted(),
			}

			// Add phone if available
			if profileResp.GetPhone() > 0 {
				employerDetails["phone"] = profileResp.GetPhone()
			}

			// Create an array of company details
			var companyDetailsArray []map[string]string
			for key, value := range employerDetails {
				if key == "employer_id" {
					continue // Skip employer_id as it's not a detail
				}

				// Convert value to string
				var strValue string
				switch v := value.(type) {
				case string:
					strValue = v
				case bool:
					strValue = strconv.FormatBool(v)
				case int64:
					strValue = strconv.FormatInt(v, 10)
				default:
					strValue = fmt.Sprintf("%v", v)
				}

				companyDetailsArray = append(companyDetailsArray, map[string]string{
					"key":   key,
					"value": strValue,
				})
			}

			// Store the employer details for later use
			employerProfiles[employerID] = &models.EmployerProfile{
				CompanyName: profileResp.GetCompanyName(),
				Industry:    profileResp.GetIndustry(),
				Website:     profileResp.GetWebsite(),
				Location:    profileResp.GetLocation(),
				IsVerified:  profileResp.GetIsVerified(),
				IsTrusted:   profileResp.GetIsTrusted(),
			}
			
			// Store the company details array in the map
			companyDetailsMap[employerID] = companyDetailsArray
		}
	}

	// Define the response structure with company as an inner object
	type CompanyInfo struct {
		CompanyName string `json:"company_name"`
		Email       string `json:"email,omitempty"`
		Industry    string `json:"industry"`
		Website     string `json:"website"`
		Location    string `json:"location"`
		IsVerified  bool   `json:"is_verified"`
		IsTrusted   bool   `json:"is_trusted"`
		Phone       string `json:"phone,omitempty"`
	}

	type JobResponse struct {
		ID                 uint           `json:"id"`
		EmployerID         string         `json:"employer_id"`
		Title              string         `json:"title"`
		Description        string         `json:"description"`
		Category           string         `json:"category"`
		SalaryMin          int64          `json:"salary_min"`
		SalaryMax          int64          `json:"salary_max"`
		ExperienceRequired int            `json:"experience_required"`
		Status             string         `json:"status"`
		RequiredSkills     []models.JobSkill `json:"skills"`
		Company            *CompanyInfo   `json:"company"`
	}

	type JobsResponse struct {
		Jobs []JobResponse `json:"jobs"`
	}

	// Build response with employer profiles and company details
	jobsResponse := JobsResponse{
		Jobs: make([]JobResponse, 0, len(jobs)),
	}

	// Prepare the protobuf response for compatibility
	response := &jobpb.GetJobsResponse{
		Jobs: make([]*jobpb.Job, 0, len(jobs)),
	}

	// Process each job
	for _, job := range jobs {
		// Create a job response with basic job details
		jobResp := JobResponse{
			ID:                 job.ID,
			EmployerID:         job.EmployerID,
			Title:              job.Title,
			Description:        job.Description,
			Category:           job.Category,
			SalaryMin:          job.SalaryMin,
			SalaryMax:          job.SalaryMax,
			ExperienceRequired: job.ExperienceRequired,
			Status:             job.Status,
			RequiredSkills:     job.RequiredSkills,
		}

		// Create the protobuf job
		pbJob := &jobpb.Job{
			Id:                 uint64(job.ID),
			EmployerId:         job.EmployerID,
			Title:              job.Title,
			Description:        job.Description,
			Category:           job.Category,
			SalaryMin:          job.SalaryMin,
			SalaryMax:          job.SalaryMax,
			Location:           job.Location,
			ExperienceRequired: int32(job.ExperienceRequired),
			Status:             job.Status,
			RequiredSkills:     make([]*jobpb.JobSkill, 0),
		}

		// Create company information for the response
		companyInfo := &CompanyInfo{
			Location: job.Location, // Default to job location if no company info available
		}

		// Add employer profile information if available
		if profile, exists := employerProfiles[job.EmployerID]; exists {
			// Set the employer profile in the job model
			job.EmployerProfile = profile

			// Update company info with profile data
			companyInfo.CompanyName = profile.CompanyName
			companyInfo.Industry = profile.Industry
			companyInfo.Website = profile.Website
			companyInfo.Location = profile.Location // Override with company location if available
			companyInfo.IsVerified = profile.IsVerified
			companyInfo.IsTrusted = profile.IsTrusted
		}

		// Log the company info we're adding to the response
		log.Printf("Adding company info for job %d: %+v", job.ID, companyInfo)

		// Add email and phone if available in the employer details
		if details, exists := companyDetailsMap[job.EmployerID]; exists {
			for _, detail := range details {
				if detail["key"] == "email" {
					companyInfo.Email = detail["value"]
				} else if detail["key"] == "phone" {
					companyInfo.Phone = detail["value"]
				}
			}
		}

		// Always set the company information in the job response
		jobResp.Company = companyInfo

		// Add job skills if available
		for _, skill := range job.RequiredSkills {
			pbJob.RequiredSkills = append(pbJob.RequiredSkills, &jobpb.JobSkill{
				JobId:       strconv.FormatUint(uint64(job.ID), 10),
				Skill:       skill.Skill,
				Proficiency: skill.Proficiency,
			})
			response.Jobs = append(response.Jobs, pbJob)
		}

		// Add the job to both responses
		jobsResponse.Jobs = append(jobsResponse.Jobs, jobResp)
	}

	// Return the response with company details as a nested object
	c.JSON(http.StatusOK, jobsResponse)
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

	// Convert job_id from string to uint64
	jobIDUint, err := strconv.ParseUint(jobid, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	// Create the request using jobpb model
	request := &jobpb.ApplyToJobRequest{
		JobId:       jobIDUint,
		CandidateId: candidateIDStr,
	}

	appid, err := h.usecase.ApplyToJob(c.Request.Context(), request.CandidateId, jobid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply to job: " + err.Error()})
		return
	}

	// Convert application ID to uint64 and create the response
	appIDUint, err := strconv.ParseUint(appid, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid application ID format"})
		return
	}

	// Create the response using jobpb model
	response := &jobpb.ApplyToJobResponse{
		ApplicationId: appIDUint,
		Message:       "Applied to job successfully",
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

	// Get job_id from URL parameter if not in request body
	jobIDStr := c.Param("job_id")
	if jobIDStr != "" {
		// Parse the string job ID to uint64
		jobID, err := strconv.ParseUint(jobIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID format in URL"})
			return
		}
		// Set the parsed job ID in the request
		request.JobId = jobID
	}

	// If job_id is still 0, it means it wasn't provided in URL or request body
	if request.JobId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	jobID := request.JobId

	// Convert to domain model - AddJobSkillsRequest contains a single skill
	skills := []models.JobSkill{
		{
			JobID:       uint(jobID),
			Skill:       request.Skill,
			Proficiency: request.Proficiency,
		},
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
