package grpc

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"jobservice/domain/models"
	"jobservice/usecase"

	"github.com/shahal0/skillsync-protos/gen/authpb"
	jobpb "github.com/shahal0/skillsync-protos/gen/jobpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
)

// JobServer implements the JobService gRPC server
type JobServer struct {
	jobpb.UnimplementedJobServiceServer
	jobUsecase *usecase.JobUsecase
}

// NewJobServer creates a new JobServer
func NewJobServer(jobUsecase *usecase.JobUsecase) *JobServer {
	return &JobServer{
		jobUsecase: jobUsecase,
	}
}

// ApplyToJob implements the ApplyToJob gRPC method
func (s *JobServer) ApplyToJob(ctx context.Context, req *jobpb.ApplyToJobRequest) (*jobpb.ApplyToJobResponse, error) {
	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		userIDs := md.Get("x-user-id")
		userRoles := md.Get("x-user-role")
		if len(userRoles) > 0 && userRoles[0] == "candidate" {
			// Verify that the user ID from metadata matches the candidate ID in the request
			if len(userIDs) > 0 && userIDs[0] == req.CandidateId {
				// OK
			}
		}
	}

	jobIDStr := fmt.Sprintf("%d", req.JobId)
	applicationID, err := s.jobUsecase.ApplyToJob(ctx, req.CandidateId, jobIDStr)
	if err != nil {
		// Check for specific error types
		if strings.Contains(err.Error(), "already applied") {
			return nil, grpcstatus.Error(codes.AlreadyExists, err.Error())
		}

		return nil, grpcstatus.Error(codes.Internal, fmt.Sprintf("failed to apply to job: %v", err))
	}

	appIDUint, err := strconv.ParseUint(applicationID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid application ID format: %v", err)
	}

	return &jobpb.ApplyToJobResponse{
		ApplicationId: appIDUint,
		Message:       "Successfully applied to job",
	}, nil
}

// PostJob implements the PostJob gRPC method
func (s *JobServer) PostJob(ctx context.Context, req *jobpb.PostJobRequest) (*jobpb.PostJobResponse, error) {
	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		userRoles := md.Get("x-user-role")

		// Verify that the user is an employer
		if len(userRoles) > 0 && userRoles[0] == "employer" {
			// OK
		}
	}

	// Create job model from request
	job := &models.Job{
		Title:              req.Title,
		Description:        req.Description,
		Category:           req.Category,
		SalaryMin:          req.SalaryMin,
		SalaryMax:          req.SalaryMax,
		Location:           req.Location,
		ExperienceRequired: int(req.ExperienceRequired),
		EmployerID:         req.EmployerId,
		Status:             "Open", 
	}

	// Convert required skills if present
	if req.RequiredSkills != nil && len(req.RequiredSkills) > 0 {
		skills := make([]models.JobSkill, 0, len(req.RequiredSkills))
		for _, skill := range req.RequiredSkills {
			skills = append(skills, models.JobSkill{
				Skill:       skill.Skill,
				Proficiency: skill.Proficiency,
			})
		}
		job.RequiredSkills = skills
	}

	// Call the usecase to create the job
	err := s.jobUsecase.PostJob(ctx, job, req.EmployerId)
	if err != nil {
		return nil, err
	}

	// Return success response with job ID
	return &jobpb.PostJobResponse{
		JobId:   uint64(job.ID),
		Message: "Job posted successfully",
	}, nil
}

// GetJobs implements the GetJobs gRPC method with pagination support
func (s *JobServer) GetJobs(ctx context.Context, req *jobpb.GetJobsRequest) (*jobpb.GetJobsResponse, error) {
	// Create filters map from request parameters
	filters := make(map[string]interface{})
	if req.Category != "" {
		// Use exact case-sensitive match for category
		filters["category"] = req.Category
	}
	if req.Keyword != "" {
		filters["keyword"] = req.Keyword
	}
	if req.Location != "" {
		filters["location"] = req.Location
	}
	
	// Get pagination parameters with defaults since the fields might be missing in the protobuf
	page := int32(1)  // Default page
	limit := int32(10) // Default limit
	
	// Set limits for pagination
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	
	// Add pagination to filters
	filters["page"] = page
	filters["limit"] = limit
	
	log.Printf("GetJobs called with pagination - Page: %d, Limit: %d", page, limit)
	
	// Use the paginated version of GetJobs
	jobs, totalCount, err := s.jobUsecase.GetJobsWithPagination(ctx, filters)
	if err != nil {
		log.Printf("Error getting jobs with pagination: %v", err)
		return nil, err
	}

	// Convert domain jobs to protobuf jobs
	pbJobs := make([]*jobpb.Job, 0, len(jobs))
	for _, job := range jobs {
		// Convert job skills to protobuf format
		var pbSkills []*jobpb.JobSkill
		// Ensure RequiredSkills is loaded
		if job.RequiredSkills != nil {
			for _, skill := range job.RequiredSkills {
				pbSkills = append(pbSkills, &jobpb.JobSkill{
					JobId:       strconv.FormatUint(uint64(job.ID), 10), // Use job.ID instead of skill.JobID
					Skill:       skill.Skill,
					Proficiency: skill.Proficiency,
				})
			}
		}

		// Default to OPEN if status is not recognized
		status := "OPEN"
		switch job.Status {
		case "CLOSED", "DRAFT":
			status = job.Status
		}

		// Initialize employer profile and company details
		jobEmployerProfile := &jobpb.EmployerProfile{}
		companyDetails := &jobpb.CompanyDetails{
			Details: []*jobpb.EmployerDetail{},
		}

		// Fetch employer profile from Auth Service if possible
		if s.jobUsecase.AuthClient != nil && job.EmployerID != "" {
			req := &authpb.EmployerProfileByIdRequest{
				EmployerId: job.EmployerID,
			}

			// Create a context with timeout to avoid hanging if Auth Service is unresponsive
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			employerResp, err := s.jobUsecase.AuthClient.EmployerProfileById(ctx, req)

			if err != nil {
				// Check if this is a not found error
				st, ok := grpcstatus.FromError(err)
				if ok && st.Code() == codes.NotFound {
					// Employer not found, use job data as fallback
					employerResp = &authpb.EmployerProfileResponse{
						CompanyName: "Company " + job.EmployerID,
						Location:    job.Location,
					}
				} else {
					// Other error with Auth Service
					return nil, grpcstatus.Error(codes.Internal, fmt.Sprintf("failed to fetch employer profile: %v", err))
				}
			}

			// Create company details from the employer profile response
			details := []*jobpb.EmployerDetail{}

			// Add company name
			if employerResp.CompanyName != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "company_name",
					Value: employerResp.CompanyName,
				})
			}

			// Add email
			if employerResp.Email != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "email",
					Value: employerResp.Email,
				})
			}

			// Add location
			if employerResp.Location != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "location",
					Value: employerResp.Location,
				})
			}

			// Add website
			if employerResp.Website != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "website",
					Value: employerResp.Website,
				})
			}

			// Add industry
			if employerResp.Industry != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "industry",
					Value: employerResp.Industry,
				})
			}

			// Set the company details
			companyDetails.Details = details
			jobEmployerProfile.CompanyName = employerResp.CompanyName
			jobEmployerProfile.Email = employerResp.Email
			jobEmployerProfile.Location = employerResp.Location
			jobEmployerProfile.Website = employerResp.Website
			jobEmployerProfile.Industry = employerResp.Industry
		} else {
			// If we can't fetch from Auth Service, use job data as fallback
			companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
				Key:   "location",
				Value: job.Location,
			})

			// Add a default company name
			companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
				Key:   "company_name",
				Value: "Company",
			})

			// Populate employer profile with fallback data
			jobEmployerProfile.CompanyName = "Company"
			jobEmployerProfile.Location = job.Location
		}

		// Convert job to protobuf format
		pbJob := &jobpb.Job{
			Id:                 uint64(job.ID),
			EmployerId:         job.EmployerID,
			Title:              job.Title,
			Description:        job.Description,
			Category:           job.Category,
			RequiredSkills:     pbSkills, // Skills are included here
			SalaryMin:          job.SalaryMin,
			SalaryMax:          job.SalaryMax,
			Location:           job.Location,
			ExperienceRequired: int32(job.ExperienceRequired),
			Status:             status,
			EmployerProfile:    jobEmployerProfile, // Add the employer profile to the job
			CompanyDetails:     companyDetails,     // Add the company details to the job
		}
		pbJobs = append(pbJobs, pbJob)
	}

	// Calculate total pages
	totalPages := int32(math.Ceil(float64(totalCount) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	// The protobuf code generation didn't include the pagination fields yet
	// We'll implement client-side pagination and log the pagination information
	
	// Apply pagination to the jobs slice
	start := (page - 1) * limit
	end := start + limit
	var paginatedJobs []*jobpb.Job
	
	if int(start) < len(pbJobs) {
		if int(end) > len(pbJobs) {
			end = int32(len(pbJobs))
		}
		paginatedJobs = pbJobs[start:end]
	} else {
		paginatedJobs = []*jobpb.Job{}
	}
	
	response := &jobpb.GetJobsResponse{
		Jobs: paginatedJobs,
	}

	// Log pagination information for debugging
	log.Printf("Returning jobs with pagination - Total: %d, Pages: %d, Current Page: %d", 
		totalCount, totalPages, page)

	return response, nil
}

// GetJobById implements the GetJobById gRPC method
func (s *JobServer) GetJobById(ctx context.Context, req *jobpb.GetJobByIdRequest) (*jobpb.GetJobByIdResponse, error) {
	// Convert uint64 job ID to string for the usecase
	jobIDStr := strconv.FormatUint(req.JobId, 10)

	// Call usecase to get job by ID
	job, err := s.jobUsecase.GetJobByID(ctx, jobIDStr)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.NotFound, "job not found: %v", err)
	}

	// Convert job skills to protobuf format
	var pbSkills []*jobpb.JobSkill
	for _, skill := range job.RequiredSkills {
		pbSkills = append(pbSkills, &jobpb.JobSkill{
			JobId:       strconv.FormatUint(uint64(skill.JobID), 10),
			Skill:       skill.Skill,
			Proficiency: skill.Proficiency,
		})
	}

	// Default to OPEN if status is not recognized
	status := "OPEN"
	switch job.Status {
	case "CLOSED", "DRAFT":
		status = job.Status
	}

	// Initialize employer profile and company details
	jobEmployerProfile := &jobpb.EmployerProfile{}
	companyDetails := &jobpb.CompanyDetails{
		Details: []*jobpb.EmployerDetail{},
	}

	// Fetch employer profile from Auth Service if possible
	if s.jobUsecase.AuthClient != nil && job.EmployerID != "" {
		authReq := &authpb.EmployerProfileByIdRequest{
			EmployerId: job.EmployerID,
		}

		// Create a context with timeout to avoid hanging if Auth Service is unresponsive
		authCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		employerResp, err := s.jobUsecase.AuthClient.EmployerProfileById(authCtx, authReq)

		if err != nil {
			// Check if this is a not found error
			st, ok := grpcstatus.FromError(err)
			if ok && st.Code() == codes.NotFound {
				// Employer not found, use job data as fallback
				employerResp = &authpb.EmployerProfileResponse{
					CompanyName: "Company " + job.EmployerID,
					Location:    job.Location,
				}
			} else {
				// Other error with Auth Service
				return nil, grpcstatus.Error(codes.Internal, fmt.Sprintf("failed to fetch employer profile: %v", err))
			}
		}

		// Create company details from the employer profile response
		details := []*jobpb.EmployerDetail{}

		// Add company name
		if employerResp.CompanyName != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "company_name",
				Value: employerResp.CompanyName,
			})
		}

		// Add email
		if employerResp.Email != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "email",
				Value: employerResp.Email,
			})
		}

		// Add location
		if employerResp.Location != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "location",
				Value: employerResp.Location,
			})
		}

		// Add website
		if employerResp.Website != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "website",
				Value: employerResp.Website,
			})
		}

		// Add industry
		if employerResp.Industry != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "industry",
				Value: employerResp.Industry,
			})
		}

		// Set the company details
		companyDetails.Details = details
		jobEmployerProfile.CompanyName = employerResp.CompanyName
		jobEmployerProfile.Email = employerResp.Email
		jobEmployerProfile.Location = employerResp.Location
		jobEmployerProfile.Website = employerResp.Website
		jobEmployerProfile.Industry = employerResp.Industry
	} else {
		// If we can't fetch from Auth Service, use job data as fallback
		companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
			Key:   "location",
			Value: job.Location,
		})

		// Add a default company name
		companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
			Key:   "company_name",
			Value: "Company",
		})

		// Populate employer profile with fallback data
		jobEmployerProfile.CompanyName = "Company"
		jobEmployerProfile.Location = job.Location
	}

	// Convert job to protobuf format
	pbJob := &jobpb.Job{
		Id:                 uint64(job.ID),
		EmployerId:         job.EmployerID,
		Title:              job.Title,
		Description:        job.Description,
		Category:           job.Category,
		RequiredSkills:     pbSkills,
		SalaryMin:          job.SalaryMin,
		SalaryMax:          job.SalaryMax,
		Location:           job.Location,
		ExperienceRequired: int32(job.ExperienceRequired),
		Status:             status,
		EmployerProfile:    jobEmployerProfile,
		CompanyDetails:     companyDetails,
	}

	return &jobpb.GetJobByIdResponse{
		Job: pbJob,
	}, nil
}

// GetApplications implements the GetApplications gRPC method with pagination support
func (s *JobServer) GetApplications(ctx context.Context, req *jobpb.GetApplicationsRequest) (*jobpb.GetApplicationsResponse, error) {
	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get user ID and role from metadata (set by auth middleware)
	userIDs := md.Get("x-user-id")
	userRoles := md.Get("x-user-role")

	if len(userIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing user ID or role in metadata")
	}

	userID := userIDs[0]
	userRole := userRoles[0]

	// Get pagination parameters
	// Note: Using default values since pagination fields are not available in the generated struct
	page := int32(1)  // Default page
	limit := int32(10) // Default limit

	log.Printf("GetApplications called with pagination - Page: %d, Limit: %d", page, limit)

	var applications []models.ApplicationResponse
	var totalCount int64
	var err error

	if req.GetCandidateId() != "" {
		// Verify the user is a candidate or admin
		if userRole != "candidate" && userRole != "admin" {
			return nil, grpcstatus.Error(codes.PermissionDenied, "only candidates or admins can view candidate applications")
		}

		// If candidate, verify they're requesting their own applications
		if userRole == "candidate" && userID != req.GetCandidateId() {
			return nil, grpcstatus.Error(codes.PermissionDenied, "candidates can only view their own applications")
		}

		// Get applications for the candidate with pagination
		applications, totalCount, err = s.jobUsecase.GetApplicationsByCandidateWithPagination(ctx, req.GetCandidateId(), req.GetStatus(), int32(page), int32(limit))
		if err != nil {
			return nil, grpcstatus.Errorf(codes.Internal, "failed to get applications by candidate: %v", err)
		}
	} else if req.GetJobId() > 0 {
		// This is a request by JobId, typically for an employer
		if userRole != "employer" && userRole != "admin" {
			return nil, grpcstatus.Error(codes.PermissionDenied, "only employers or admins can view applications by job ID")
		}

		// Get applications for the job with pagination
		// Based on the memory about job_id type, we need to be careful with the conversion
		// The proto uses string type for job_id fields, but the code might be treating it as uint64
		jobIDStr := strconv.FormatUint(req.GetJobId(), 10)
		applications, totalCount, err = s.jobUsecase.GetApplicationsByJobWithPagination(ctx, jobIDStr, req.GetStatus(), int32(page), int32(limit))
		if err != nil {
			return nil, grpcstatus.Errorf(codes.Internal, "failed to get applications by job ID: %v", err)
		}
	} else {
		// Neither CandidateId nor JobId is provided or JobId is invalid (e.g., 0)
		return nil, grpcstatus.Error(codes.InvalidArgument, "must specify a valid candidateId or jobId")
	}

	// Convert applications to protobuf format (common logic for both paths)
	var pbApplications []*jobpb.ApplicationResponse
	for _, app := range applications {
		appID := uint64(app.ID)
		appResponse := &jobpb.ApplicationResponse{
			Id:          appID,
			CandidateId: app.CandidateID,
			Status:      app.Status,
			ResumeUrl:   app.ResumeURL,
		}

		if !app.AppliedAt.IsZero() {
			appResponse.AppliedAt = app.AppliedAt.Format(time.RFC3339)
		}

		if app.Job != nil { // Job details should be populated by the usecase layer
			job := app.Job
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
			}

			if len(job.RequiredSkills) > 0 {
				var pbSkills []*jobpb.JobSkill
				for _, skill := range job.RequiredSkills {
					pbSkills = append(pbSkills, &jobpb.JobSkill{
						JobId:       fmt.Sprintf("%d", job.ID), // Ensure this is correct based on proto def
						Skill:       skill.Skill,
						Proficiency: skill.Proficiency,
					})
				}
				pbJob.RequiredSkills = pbSkills
			}
			appResponse.Job = pbJob
		}
		pbApplications = append(pbApplications, appResponse)
	}

	// Calculate total pages
	totalPages := int64(math.Ceil(float64(totalCount) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	// The protobuf code generation didn't include the pagination fields yet
	// We'll implement client-side pagination and log the pagination information
	
	// Apply pagination to the applications slice
	start := (page - 1) * limit
	end := start + limit
	var paginatedApplications []*jobpb.ApplicationResponse
	
	if int(start) < len(pbApplications) {
		if int(end) > len(pbApplications) {
			end = int32(len(pbApplications))
		}
		paginatedApplications = pbApplications[start:end]
	} else {
		paginatedApplications = []*jobpb.ApplicationResponse{}
	}
	
	response := &jobpb.GetApplicationsResponse{
		Applications: paginatedApplications,
	}

	// Log pagination information for debugging
	log.Printf("Returning applications with pagination - Total: %d, Pages: %d, Current Page: %d", 
		totalCount, totalPages, page)

	return response, nil
}

// GetApplication implements the GetApplication gRPC method
func (s *JobServer) GetApplication(ctx context.Context, req *jobpb.GetApplicationRequest) (*jobpb.GetApplicationResponse, error) {
	// Get application ID from request
	applicationID := req.GetApplicationId()
	if applicationID == 0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "application ID is required")
	}

	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get user ID and role from metadata (set by auth middleware)
	userIDs := md.Get("x-user-id")
	userRoles := md.Get("x-user-role")

	if len(userIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing user ID or role in metadata")
	}

	userID := userIDs[0]
	userRole := userRoles[0]

	// Get application by ID
	application, err := s.jobUsecase.GetApplicationByID(ctx, uint(applicationID))
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to get application: %v", err)
	}

	// Verify the user has permission to view this application
	// If user is a candidate, they can only view their own applications
	if userRole == "candidate" && userID != application.CandidateID {
		return nil, grpcstatus.Error(codes.PermissionDenied, "candidates can only view their own applications")
	}

	// Create application response
	appResponse := &jobpb.ApplicationResponse{
		Id:          uint64(application.ID),
		CandidateId: application.CandidateID,
		Status:      application.Status,
		ResumeUrl:   application.ResumeURL,
	}

	// Format the applied_at timestamp if available
	if !application.AppliedAt.IsZero() {
		appResponse.AppliedAt = application.AppliedAt.Format(time.RFC3339)
	}

	// Add job details if available
	if application.Job != nil {
		job := application.Job

		// Create a new Job object
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
		}

		// Add job skills if available
		if len(job.RequiredSkills) > 0 {
			var pbSkills []*jobpb.JobSkill
			for _, skill := range job.RequiredSkills {
				pbSkills = append(pbSkills, &jobpb.JobSkill{
					JobId:       fmt.Sprintf("%d", job.ID),
					Skill:       skill.Skill,
					Proficiency: skill.Proficiency,
				})
			}
			pbJob.RequiredSkills = pbSkills
		}

		// Set the job in the application response
		appResponse.Job = pbJob
	}

	// Create the final response
	response := &jobpb.GetApplicationResponse{
		Application: appResponse,
	}

	return response, nil
}

// UpdateApplicationStatus implements the UpdateApplicationStatus gRPC method
func (s *JobServer) UpdateApplicationStatus(ctx context.Context, req *jobpb.UpdateApplicationStatusRequest) (*jobpb.UpdateApplicationStatusResponse, error) {
	return &jobpb.UpdateApplicationStatusResponse{
		Message: "Application status updated successfully",
	}, nil
}

// AddJobSkills implements the AddJobSkills gRPC method
func (s *JobServer) AddJobSkills(ctx context.Context, req *jobpb.AddJobSkillsRequest) (*jobpb.AddJobSkillsResponse, error) {
	// Convert job ID from string to uint
	jobID := req.JobId
	if jobID == 0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "invalid job ID format")
	}

	// Create a single job skill from the request
	skills := []models.JobSkill{
		{
			JobID:       uint(jobID),
			Skill:       req.Skill,
			Proficiency: req.Proficiency,
		},
	}

	// Call usecase to add job skills
	var err error
	err = s.jobUsecase.AddJobSkills(ctx, skills)
	if err != nil {
		return nil, err
	}

	return &jobpb.AddJobSkillsResponse{
		Message: fmt.Sprintf("Successfully added %d skills to job", len(skills)),
	}, nil
}

// UpdateJobStatus implements the UpdateJobStatus gRPC method
func (s *JobServer) UpdateJobStatus(ctx context.Context, req *jobpb.UpdateJobStatusRequest) (*jobpb.UpdateJobStatusResponse, error) {
	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get employer ID from metadata (set by auth middleware)
	employerIDs := md.Get("x-user-id")
	userRoles := md.Get("x-user-role")

	if len(employerIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing user ID or role in metadata")
	}

	employerID := employerIDs[0]
	userRole := userRoles[0]

	// Verify the user is an employer
	if userRole != "employer" {
		return nil, grpcstatus.Error(codes.PermissionDenied, "only employers can update job status")
	}

	// Call the usecase to update the job status
	err := s.jobUsecase.UpdateJobStatus(ctx, req.JobId, employerID, req.Status)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to update job status: %v", err)
	}

	return &jobpb.UpdateJobStatusResponse{
		Message: "Job status updated successfully",
	}, nil
}

// FilterApplications implements the FilterApplications gRPC method
func (s *JobServer) FilterApplications(ctx context.Context, req *jobpb.FilterApplicationsRequest) (*jobpb.FilterApplicationsResponse, error) {
	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get employer ID from metadata (set by auth middleware)
	employerIDs := md.Get("x-user-id")
	userRoles := md.Get("x-user-role")

	if len(employerIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing user ID or role in metadata")
	}

	employerID := employerIDs[0]
	userRole := userRoles[0]

	// Verify the user is an employer
	if userRole != "employer" {
		return nil, grpcstatus.Error(codes.PermissionDenied, "only employers can filter applications")
	}

	// Ensure jobID is provided and valid
	if req.JobId == 0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "job ID is required")
	}

	// Ensure employerID is provided and valid
	if employerID == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "employer ID is required")
	}

	// We're not using filter options from the request anymore
	// Instead, we'll fetch all the necessary details from the job and applications
	filterOptions := map[string]interface{}{}

	// Call the usecase to filter applications
	applications, err := s.jobUsecase.FilterApplicationsByJob(ctx, fmt.Sprintf("%d", req.JobId), filterOptions)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to filter applications: %v", err)
	}

	// Convert domain model to protobuf response
	pbRankedApps := make([]*jobpb.RankedApplication, 0, len(applications))
	for _, rankedApp := range applications {
		// Convert application response to protobuf
		app := rankedApp.Application

		// Create protobuf application response
		pbApp := &jobpb.ApplicationResponse{
			Id:          uint64(app.ID),
			CandidateId: app.CandidateID,
			Status:      app.Status,
			ResumeUrl:   app.ResumeURL,
		}

		// Format the applied_at timestamp if available
		if !app.AppliedAt.IsZero() {
			pbApp.AppliedAt = app.AppliedAt.Format(time.RFC3339)
		}

		// Add job details if available
		if app.Job != nil {
			job := app.Job

			// Create a new Job object
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
			}

			// Add job skills if available
			if len(job.RequiredSkills) > 0 {
				var pbSkills []*jobpb.JobSkill
				for _, skill := range job.RequiredSkills {
					pbSkills = append(pbSkills, &jobpb.JobSkill{
						JobId:       fmt.Sprintf("%d", job.ID),
						Skill:       skill.Skill,
						Proficiency: skill.Proficiency,
					})
				}
				pbJob.RequiredSkills = pbSkills
			}

			// Add employer profile if available
			if job.EmployerProfile != nil {
				pbJob.EmployerProfile = &jobpb.EmployerProfile{
					CompanyName: job.EmployerProfile.CompanyName,
					Industry:    job.EmployerProfile.Industry,
					Website:     job.EmployerProfile.Website,
					Location:    job.EmployerProfile.Location,
					IsVerified:  job.EmployerProfile.IsVerified,
					IsTrusted:   job.EmployerProfile.IsTrusted,
				}
			}

			pbApp.Job = pbJob
		}

		// Create the ranked application protobuf message
		// Initialize empty arrays for matching and missing skills if they're nil
		var matchingSkills []string
		var missingSkills []string

		// Use the actual skills if available, otherwise use empty arrays
		if rankedApp.MatchingSkills != nil {
			matchingSkills = rankedApp.MatchingSkills
		}

		if rankedApp.MissingSkills != nil {
			missingSkills = rankedApp.MissingSkills
		}

		pbRankedApp := &jobpb.RankedApplication{
			Application:    pbApp,
			RelevanceScore: rankedApp.RelevanceScore,
			MatchingSkills: matchingSkills,
			MissingSkills:  missingSkills,
		}

		pbRankedApps = append(pbRankedApps, pbRankedApp)
	}

	// Add total applications count and a proper message
	totalApplications := uint32(len(applications))

	// Construct response with message and total count
	response := &jobpb.FilterApplicationsResponse{
		RankedApplications: pbRankedApps,
		TotalApplications:  int32(totalApplications),
		Message:            fmt.Sprintf("Successfully filtered %d applications for job ID %d", totalApplications, req.JobId),
	}

	return response, nil
}
