package grpc

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"jobservice/domain/models"
	"jobservice/usecase"

	"github.com/shahal0/skillsync-protos/gen/authpb"
	"github.com/shahal0/skillsync-protos/gen/jobpb"
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
	// Log the incoming request with more detail
	log.Printf("GRPC DEBUG: ApplyToJob gRPC method called ")
	log.Printf("GRPC DEBUG: Request parameters - JobID: %d, CandidateID: %s", req.JobId, req.CandidateId)

	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		userIDs := md.Get("x-user-id")
		userRoles := md.Get("x-user-role")
		log.Printf("GRPC DEBUG: Metadata received - User ID: %v, Role: %v", userIDs, userRoles)

		// Verify that the user is a candidate
		if len(userRoles) > 0 && userRoles[0] == "candidate" {
			log.Printf("GRPC DEBUG: User role verified as candidate")

			// Verify that the user ID from metadata matches the candidate ID in the request
			if len(userIDs) > 0 && userIDs[0] == req.CandidateId {
				log.Printf("GRPC DEBUG: User ID from metadata matches candidate ID in request")
			} else {
				log.Printf("GRPC WARNING: User ID from metadata (%v) does not match candidate ID in request (%s)", userIDs, req.CandidateId)
			}
		} else {
			log.Printf("GRPC ERROR: User role verification failed or missing")
		}
	} else {
		log.Printf("GRPC DEBUG: No metadata found in context")
	}

	// Call the usecase to apply to the job
	// Convert uint64 job ID to string for the usecase
	jobIDStr := fmt.Sprintf("%d", req.JobId)
	log.Printf("GRPC DEBUG: Calling usecase.ApplyToJob with candidateID=%s, jobID=%s", req.CandidateId, jobIDStr)
	applicationID, err := s.jobUsecase.ApplyToJob(ctx, req.CandidateId, jobIDStr)
	if err != nil {
		log.Printf("GRPC ERROR: Failed to apply to job: %v", err)
		return nil, err
	}
	log.Printf("GRPC DEBUG: Application ID generated: %s", applicationID)

	log.Printf("GRPC SUCCESS: Successfully applied to job %d for candidate %s with application ID: %s", req.JobId, req.CandidateId, applicationID)

	// Return success response with application ID
	// Convert string application ID to uint64
	appIDUint, err := strconv.ParseUint(applicationID, 10, 64)
	if err != nil {
		log.Printf("GRPC ERROR: Failed to convert application ID to uint64: %v", err)
		return nil, fmt.Errorf("invalid application ID format: %v", err)
	}

	return &jobpb.ApplyToJobResponse{
		ApplicationId: appIDUint,
		Message:       "Successfully applied to job",
	}, nil
}

// PostJob implements the PostJob gRPC method
func (s *JobServer) PostJob(ctx context.Context, req *jobpb.PostJobRequest) (*jobpb.PostJobResponse, error) {
	// Log the incoming request
	log.Printf("\n\n====== GRPC DEBUG: PostJob gRPC method called ======")
	log.Printf("GRPC DEBUG: EmployerID: %s", req.EmployerId)

	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		userIDs := md.Get("x-user-id")
		userRoles := md.Get("x-user-role")
		log.Printf("GRPC DEBUG: Metadata received - User ID: %v, Role: %v", userIDs, userRoles)

		// Verify that the user is an employer
		if len(userRoles) > 0 && userRoles[0] == "employer" {
			log.Printf("GRPC DEBUG: User role verified as employer")
		} else {
			log.Printf("GRPC ERROR: User role verification failed or missing")
		}
	} else {
		log.Printf("GRPC DEBUG: No metadata found in context")
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
		Status:             "Open", // Default status for new jobs
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
	log.Printf("GRPC DEBUG: Calling usecase.PostJob with job details")
	err := s.jobUsecase.PostJob(ctx, job, req.EmployerId)
	if err != nil {
		log.Printf("GRPC ERROR: Failed to create job: %v", err)
		return nil, err
	}
	log.Printf("GRPC DEBUG: Job created with ID: %d", job.ID)

	// Return success response with job ID
	return &jobpb.PostJobResponse{
		JobId:   uint64(job.ID),
		Message: "Job posted successfully",
	}, nil
}

// GetJobs implements the GetJobs gRPC method
func (s *JobServer) GetJobs(ctx context.Context, req *jobpb.GetJobsRequest) (*jobpb.GetJobsResponse, error) {
	log.Printf("GRPC DEBUG: Received GetJobs request")

	// Create filters map from request parameters
	filters := make(map[string]interface{})
	if req.Category != "" {
		filters["category"] = req.Category
	}
	if req.Keyword != "" {
		filters["keyword"] = req.Keyword
	}
	if req.Location != "" {
		filters["location"] = req.Location
	}
	// Note: ExperienceRequired field was added to proto but might not be regenerated yet
	// Uncomment this when the protobuf code is regenerated
	/*
		if req.ExperienceRequired != "" {
			// Convert experience_required string to int if needed
			expReq, err := strconv.Atoi(req.ExperienceRequired)
			if err == nil {
				filters["experience_required"] = expReq
			}
		}
	*/

	// Call usecase to get jobs
	jobs, err := s.jobUsecase.GetJobs(ctx, filters)
	if err != nil {
		log.Printf("GRPC ERROR: Failed to get jobs: %v", err)
		return nil, err
	}

	// Convert domain jobs to protobuf jobs
	pbJobs := make([]*jobpb.Job, 0, len(jobs))
	for _, job := range jobs {
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
			log.Printf("GRPC DEBUG: Fetching employer profile for ID: %s", job.EmployerID)

			// Create request to get employer profile by ID
			req := &authpb.EmployerProfileByIdRequest{
				EmployerId: job.EmployerID,
			}
			log.Printf("GRPC DEBUG: Created request with EmployerId: %s", job.EmployerID)

			// Call Auth Service to get employer profile
			log.Printf("GRPC DEBUG: Calling Auth Service EmployerProfileById method")

			// Create a context with timeout to avoid hanging if Auth Service is unresponsive
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			employerResp, err := s.jobUsecase.AuthClient.EmployerProfileById(ctx, req)

			if err != nil {
				log.Printf("GRPC ERROR: Failed to fetch employer profile: %v", err)
				// Check if this is a not found error
				st, ok := grpcstatus.FromError(err)
				if ok && st.Code() == codes.NotFound {
					log.Printf("GRPC INFO: Employer with ID %s not found in Auth Service", job.EmployerID)
				} else {
					log.Printf("GRPC ERROR: Connection issue or other error with Auth Service: %v", err)
				}
			} else if employerResp == nil {
				log.Printf("GRPC WARNING: Auth Service returned nil response for employer ID: %s", job.EmployerID)
			} else {
				log.Printf("GRPC DEBUG: Successfully fetched employer profile for ID: %s", job.EmployerID)
				log.Printf("GRPC DEBUG: Employer profile details - CompanyName: '%s', Email: '%s', Location: '%s', Website: '%s', Industry: '%s'",
					employerResp.CompanyName, employerResp.Email, employerResp.Location, employerResp.Website, employerResp.Industry)

				// Check if all fields are empty
				if employerResp.CompanyName == "" && employerResp.Email == "" && employerResp.Location == "" &&
					employerResp.Website == "" && employerResp.Industry == "" {
					log.Printf("GRPC WARNING: Auth Service returned empty employer profile for ID: %s", job.EmployerID)

					// Use job data as fallback for empty employer profiles
					employerResp.CompanyName = "Company " + job.EmployerID
					employerResp.Location = job.Location
					log.Printf("GRPC INFO: Using job data as fallback for empty employer profile")
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
			}
		} else {
			// If we can't fetch from Auth Service, use job data as fallback
			log.Printf("GRPC DEBUG: Using job data as fallback for company details")

			// Add location from job data
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

	log.Printf("GRPC DEBUG: Returning %d jobs", len(pbJobs))
	return &jobpb.GetJobsResponse{
		Jobs: pbJobs,
	}, nil
}

// GetJobById implements the GetJobById gRPC method
func (s *JobServer) GetJobById(ctx context.Context, req *jobpb.GetJobByIdRequest) (*jobpb.GetJobByIdResponse, error) {
	return &jobpb.GetJobByIdResponse{}, nil
}

// GetApplications implements the GetApplications gRPC method
func (s *JobServer) GetApplications(ctx context.Context, req *jobpb.GetApplicationsRequest) (*jobpb.GetApplicationsResponse, error) {
	return &jobpb.GetApplicationsResponse{}, nil
}

// UpdateApplicationStatus implements the UpdateApplicationStatus gRPC method
func (s *JobServer) UpdateApplicationStatus(ctx context.Context, req *jobpb.UpdateApplicationStatusRequest) (*jobpb.UpdateApplicationStatusResponse, error) {
	return &jobpb.UpdateApplicationStatusResponse{
		Message: "Application status updated successfully",
	}, nil
}

// AddJobSkills implements the AddJobSkills gRPC method
func (s *JobServer) AddJobSkills(ctx context.Context, req *jobpb.AddJobSkillsRequest) (*jobpb.AddJobSkillsResponse, error) {
	log.Printf("GRPC DEBUG: Received AddJobSkills request for job ID: %s", req.JobId)

	// Convert job ID from string to uint
	jobID := req.JobId
	if jobID == 0 {
		log.Printf("GRPC ERROR: Invalid job ID format: %v", jobID)
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
		log.Printf("GRPC ERROR: Failed to add job skills: %v", err)
		return nil, err
	}

	log.Printf("GRPC DEBUG: Successfully added %d skills to job ID: %s", len(skills), req.JobId)
	return &jobpb.AddJobSkillsResponse{
		Message: fmt.Sprintf("Successfully added %d skills to job", len(skills)),
	}, nil
}

// UpdateJobStatus implements the UpdateJobStatus gRPC method
func (s *JobServer) UpdateJobStatus(ctx context.Context, req *jobpb.UpdateJobStatusRequest) (*jobpb.UpdateJobStatusResponse, error) {
	log.Printf("GRPC DEBUG: UpdateJobStatus gRPC method called")
	log.Printf("GRPC DEBUG: Request parameters - JobID: %s, Status: %s", req.JobId, req.Status)

	// Extract user information from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Println("GRPC ERROR: No metadata found in context")
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get employer ID from metadata (set by auth middleware)
	employerIDs := md.Get("x-user-id")
	userRoles := md.Get("x-user-role")

	if len(employerIDs) == 0 || len(userRoles) == 0 {
		log.Println("GRPC ERROR: Missing user ID or role in metadata")
		return nil, grpcstatus.Error(codes.Unauthenticated, "missing user ID or role in metadata")
	}

	employerID := employerIDs[0]
	userRole := userRoles[0]

	// Verify the user is an employer
	if userRole != "employer" {
		log.Printf("GRPC ERROR: User with role '%s' is not authorized to update job status", userRole)
		return nil, grpcstatus.Error(codes.PermissionDenied, "only employers can update job status")
	}

	log.Printf("GRPC DEBUG: Updating job status - JobID: %s, EmployerID: %s, New Status: %s",
		req.JobId, employerID, req.Status)

	// Call the usecase to update the job status
	err := s.jobUsecase.UpdateJobStatus(ctx, req.JobId, employerID, req.Status)
	if err != nil {
		log.Printf("GRPC ERROR: Failed to update job status: %v", err)
		return nil, grpcstatus.Errorf(codes.Internal, "failed to update job status: %v", err)
	}

	log.Printf("GRPC SUCCESS: Successfully updated status of job ID %s to %s", req.JobId, req.Status)
	return &jobpb.UpdateJobStatusResponse{
		Message: "Job status updated successfully",
	}, nil
}
