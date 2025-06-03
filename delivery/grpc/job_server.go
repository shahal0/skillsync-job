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


type JobServer struct {
	jobpb.UnimplementedJobServiceServer
	jobUsecase *usecase.JobUsecase
}


func NewJobServer(jobUsecase *usecase.JobUsecase) *JobServer {
	return &JobServer{
		jobUsecase: jobUsecase,
	}
}


func (s *JobServer) ApplyToJob(ctx context.Context, req *jobpb.ApplyToJobRequest) (*jobpb.ApplyToJobResponse, error) {
	
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	
	userIDs := md.Get("user-id")
	userRoles := md.Get("user-role")
	
	
	if len(userRoles) == 0 || userRoles[0] != "candidate" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "only candidates can apply to jobs")
	}
	
	
	if len(userIDs) == 0 || userIDs[0] != req.CandidateId {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "candidate ID in request does not match authenticated user")
	}
	
	// Validate required fields
	if req.CandidateId == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "candidate ID is required")
	}
	
	if req.JobId == 0 {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job ID is required")
	}
	
	// Validate resume URL if provided
	if req.ResumeUrl != "" {
		if !strings.HasPrefix(req.ResumeUrl, "http://") && !strings.HasPrefix(req.ResumeUrl, "https://") {
			return nil, grpcstatus.Errorf(codes.InvalidArgument, "resume URL must be a valid URL")
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


func (s *JobServer) PostJob(ctx context.Context, req *jobpb.PostJobRequest) (*jobpb.PostJobResponse, error) {
	
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	
	userRoles := md.Get("user-role")
	// Verify that the user is an employer
	if len(userRoles) == 0 || userRoles[0] != "employer" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "only employers can post jobs")
	}
	
	// Validate required fields
	if req.Title == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job title is required")
	}
	
	if req.Description == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job description is required")
	}
	
	if req.Category == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job category is required")
	}
	
	if req.Location == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job location is required")
	}
	
	if req.EmployerId == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "employer ID is required")
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
	err := s.jobUsecase.PostJob(ctx, job, req.EmployerId)
	if err != nil {
		return nil, err
	}
	return &jobpb.PostJobResponse{
		JobId:   uint64(job.ID),
		Message: "Job posted successfully",
	}, nil
}

func (s *JobServer) GetJobs(ctx context.Context, req *jobpb.GetJobsRequest) (*jobpb.GetJobsResponse, error) {
	filters := make(map[string]interface{})
	if req.Category != "" {
		allowedCategories := []string{"Technology", "Healthcare", "Finance", "Education", "Marketing", "Sales", "Engineering", "Other"}
		validCategory := false
		for _, cat := range allowedCategories {
			if req.Category == cat {
				validCategory = true
				break
			}
		}
		if !validCategory {
			return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid job category")
		}
		filters["category"] = req.Category
	}
	if req.Keyword != "" {
		keyword := strings.TrimSpace(req.Keyword)
		if len(keyword) < 2 {
			return nil, grpcstatus.Errorf(codes.InvalidArgument, "keyword must be at least 2 characters")
		}
		filters["keyword"] = keyword
	}
	if req.Location != "" {
		location := strings.TrimSpace(req.Location)
		if len(location) < 2 {
			return nil, grpcstatus.Errorf(codes.InvalidArgument, "location must be at least 2 characters")
		}
		filters["location"] = location
	}
	page := int32(1)
	limit := int32(10)
	if pageParam, ok := filters["page"].(int32); ok && pageParam > 0 {
		page = pageParam
	}
	if limitParam, ok := filters["limit"].(int32); ok && limitParam > 0 {
		limit = limitParam
	}
	if page < 1 {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "page must be greater than 0")
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	filters["page"] = page
	filters["limit"] = limit
	log.Printf("GetJobs called with pagination - Page: %d, Limit: %d", page, limit)
	jobs, totalCount, err := s.jobUsecase.GetJobsWithPagination(ctx, filters)
	if err != nil {
		log.Printf("Error getting jobs with pagination: %v", err)
		return nil, err
	}
	pbJobs := make([]*jobpb.Job, 0, len(jobs))
	for _, job := range jobs {
		pbSkills := make([]*jobpb.JobSkill, 0, len(job.RequiredSkills))
		for _, skill := range job.RequiredSkills {
			pbSkills = append(pbSkills, &jobpb.JobSkill{
				JobId:       strconv.FormatUint(uint64(job.ID), 10),
				Skill:       skill.Skill,
				Proficiency: skill.Proficiency,
			})
		}
		status := "OPEN"
		switch job.Status {
		case "CLOSED", "DRAFT":
			status = job.Status
		}
		jobEmployerProfile := &jobpb.EmployerProfile{}
		companyDetails := &jobpb.CompanyDetails{
			Details: []*jobpb.EmployerDetail{},
		}
		if s.jobUsecase.AuthClient != nil && job.EmployerID != "" {
			req := &authpb.EmployerProfileByIdRequest{
				EmployerId: job.EmployerID,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			employerResp, err := s.jobUsecase.AuthClient.EmployerProfileById(ctx, req)
			if err != nil {
				st, ok := grpcstatus.FromError(err)
				if ok && st.Code() == codes.NotFound {
					employerResp = &authpb.EmployerProfileResponse{
						CompanyName: "Company " + job.EmployerID,
						Location:    job.Location,
					}
				} else {
					return nil, grpcstatus.Error(codes.Internal, fmt.Sprintf("failed to fetch employer profile: %v", err))
				}
			}
			details := []*jobpb.EmployerDetail{}
			if employerResp.CompanyName != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "company_name",
					Value: employerResp.CompanyName,
				})
			}
			if employerResp.Email != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "email",
					Value: employerResp.Email,
				})
			}
			if employerResp.Location != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "location",
					Value: employerResp.Location,
				})
			}
			if employerResp.Website != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "website",
					Value: employerResp.Website,
				})
			}
			if employerResp.Industry != "" {
				details = append(details, &jobpb.EmployerDetail{
					Key:   "industry",
					Value: employerResp.Industry,
				})
			}
			companyDetails.Details = details
			jobEmployerProfile.CompanyName = employerResp.CompanyName
			jobEmployerProfile.Email = employerResp.Email
			jobEmployerProfile.Location = employerResp.Location
			jobEmployerProfile.Website = employerResp.Website
			jobEmployerProfile.Industry = employerResp.Industry
		} else {
			companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
				Key:   "location",
				Value: job.Location,
			})
			companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
				Key:   "company_name",
				Value: "Company",
			})
			jobEmployerProfile.CompanyName = "Company"
			jobEmployerProfile.Location = job.Location
		}
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
		pbJobs = append(pbJobs, pbJob)
	}
	totalPages := int32(math.Ceil(float64(totalCount) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
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
	log.Printf("Returning jobs with pagination - Total: %d, Pages: %d, Current Page: %d", totalCount, totalPages, page)
	return response, nil
}

func (s *JobServer) GetJobById(ctx context.Context, req *jobpb.GetJobByIdRequest) (*jobpb.GetJobByIdResponse, error) {
	if req.JobId == 0 {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job ID is required")
	}
	jobIDStr := strconv.FormatUint(req.JobId, 10)
	job, err := s.jobUsecase.GetJobByID(ctx, jobIDStr)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.NotFound, "job not found: %v", err)
	}
	pbSkills := make([]*jobpb.JobSkill, 0, len(job.RequiredSkills))
	for _, skill := range job.RequiredSkills {
		pbSkills = append(pbSkills, &jobpb.JobSkill{
			JobId:       strconv.FormatUint(uint64(skill.JobID), 10),
			Skill:       skill.Skill,
			Proficiency: skill.Proficiency,
		})
	}
	status := "OPEN"
	switch job.Status {
	case "CLOSED", "DRAFT":
		status = job.Status
	}
	jobEmployerProfile := &jobpb.EmployerProfile{}
	companyDetails := &jobpb.CompanyDetails{
		Details: []*jobpb.EmployerDetail{},
	}
	if s.jobUsecase.AuthClient != nil && job.EmployerID != "" {
		authReq := &authpb.EmployerProfileByIdRequest{
			EmployerId: job.EmployerID,
		}
		authCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		employerResp, err := s.jobUsecase.AuthClient.EmployerProfileById(authCtx, authReq)
		if err != nil {
			st, ok := grpcstatus.FromError(err)
			if ok && st.Code() == codes.NotFound {
				employerResp = &authpb.EmployerProfileResponse{
					CompanyName: "Company " + job.EmployerID,
					Location:    job.Location,
				}
			} else {
				return nil, grpcstatus.Error(codes.Internal, fmt.Sprintf("failed to fetch employer profile: %v", err))
			}
		}
		details := []*jobpb.EmployerDetail{}
		if employerResp.CompanyName != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "company_name",
				Value: employerResp.CompanyName,
			})
		}
		if employerResp.Email != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "email",
				Value: employerResp.Email,
			})
		}
		if employerResp.Location != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "location",
				Value: employerResp.Location,
			})
		}
		if employerResp.Website != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "website",
				Value: employerResp.Website,
			})
		}
		if employerResp.Industry != "" {
			details = append(details, &jobpb.EmployerDetail{
				Key:   "industry",
				Value: employerResp.Industry,
			})
		}
		companyDetails.Details = details
		jobEmployerProfile.CompanyName = employerResp.CompanyName
		jobEmployerProfile.Email = employerResp.Email
		jobEmployerProfile.Location = employerResp.Location
		jobEmployerProfile.Website = employerResp.Website
		jobEmployerProfile.Industry = employerResp.Industry
	} else {
		companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
			Key:   "location",
			Value: job.Location,
		})
		companyDetails.Details = append(companyDetails.Details, &jobpb.EmployerDetail{
			Key:   "company_name",
			Value: "Company",
		})
		jobEmployerProfile.CompanyName = "Company"
		jobEmployerProfile.Location = job.Location
	}
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

func (s *JobServer) GetApplications(ctx context.Context, req *jobpb.GetApplicationsRequest) (*jobpb.GetApplicationsResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	userIDs := md.Get("user-id")
	userRoles := md.Get("user-role")
	if len(userIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing user ID or role in metadata")
	}
	userID := userIDs[0]
	userRole := userRoles[0]
	if userRole != "candidate" && userRole != "employer" && userRole != "admin" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "invalid user role")
	}
	page := int32(1)
	limit := int32(10)
	if limit > 50 {
		limit = 50
	}
	if req.Status != "" {
		validStatuses := []string{"PENDING", "APPROVED", "REJECTED", "WITHDRAWN"}
		validStatus := false
		for _, s := range validStatuses {
			if req.Status == s {
				validStatus = true
				break
			}
		}
		if !validStatus {
			return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid status value: must be one of PENDING, APPROVED, REJECTED, or WITHDRAWN")
		}
	}
	log.Printf("GetApplications called with pagination - Page: %d, Limit: %d", page, limit)
	var applications []models.ApplicationResponse
	var totalCount int64
	var err error
	if req.GetCandidateId() != "" {
		if userRole != "candidate" && userRole != "admin" {
			return nil, grpcstatus.Error(codes.PermissionDenied, "only candidates or admins can view candidate applications")
		}
		if userRole == "candidate" && userID != req.GetCandidateId() {
			return nil, grpcstatus.Error(codes.PermissionDenied, "candidates can only view their own applications")
		}
		applications, totalCount, err = s.jobUsecase.GetApplicationsByCandidateWithPagination(ctx, req.GetCandidateId(), req.GetStatus(), int32(page), int32(limit))
		if err != nil {
			return nil, grpcstatus.Errorf(codes.Internal, "failed to get applications by candidate: %v", err)
		}
	} else if req.GetJobId() > 0 {
		if userRole != "employer" && userRole != "admin" {
			return nil, grpcstatus.Error(codes.PermissionDenied, "only employers or admins can view applications by job ID")
		}
		jobIDStr := strconv.FormatUint(req.GetJobId(), 10)
		applications, totalCount, err = s.jobUsecase.GetApplicationsByJobWithPagination(ctx, jobIDStr, req.GetStatus(), int32(page), int32(limit))
		if err != nil {
			return nil, grpcstatus.Errorf(codes.Internal, "failed to get applications by job ID: %v", err)
		}
	} else {
		return nil, grpcstatus.Error(codes.InvalidArgument, "must specify a valid candidateId or jobId")
	}
	pbApplications := make([]*jobpb.ApplicationResponse, 0, len(applications))
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
		if app.Job != nil {
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
						JobId:       fmt.Sprintf("%d", job.ID),
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
	totalPages := int64(math.Ceil(float64(totalCount) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
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
	log.Printf("Returning applications with pagination - Total: %d, Pages: %d, Current Page: %d", totalCount, totalPages, page)
	return response, nil
}

func (s *JobServer) GetApplication(ctx context.Context, req *jobpb.GetApplicationRequest) (*jobpb.GetApplicationResponse, error) {
	applicationID := req.GetApplicationId()
	if applicationID == 0 {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "application ID is required")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	userIDs := md.Get("user-id")
	userRoles := md.Get("user-role")
	if len(userIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing user ID or role in metadata")
	}
	userID := userIDs[0]
	userRole := userRoles[0]
	if userRole != "candidate" && userRole != "employer" && userRole != "admin" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "invalid user role")
	}
	application, err := s.jobUsecase.GetApplicationByID(ctx, uint(applicationID))
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to get application: %v", err)
	}
	if userRole == "candidate" && userID != application.CandidateID {
		return nil, grpcstatus.Error(codes.PermissionDenied, "candidates can only view their own applications")
	}
	appResponse := &jobpb.ApplicationResponse{
		Id:          uint64(application.ID),
		CandidateId: application.CandidateID,
		Status:      application.Status,
		ResumeUrl:   application.ResumeURL,
	}
	if !application.AppliedAt.IsZero() {
		appResponse.AppliedAt = application.AppliedAt.Format(time.RFC3339)
	}
	if application.Job != nil {
		job := application.Job
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
					JobId:       fmt.Sprintf("%d", job.ID),
					Skill:       skill.Skill,
					Proficiency: skill.Proficiency,
				})
			}
			pbJob.RequiredSkills = pbSkills
		}
		appResponse.Job = pbJob
	}
	response := &jobpb.GetApplicationResponse{
		Application: appResponse,
	}
	return response, nil
}

func (s *JobServer) UpdateApplicationStatus(ctx context.Context, req *jobpb.UpdateApplicationStatusRequest) (*jobpb.UpdateApplicationStatusResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	userIDs := md.Get("user-id")
	userRoles := md.Get("user-role")
	if len(userIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing user ID or role in metadata")
	}
	userRole := userRoles[0]
	if userRole != "employer" && userRole != "admin" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "only employers or admins can update application status")
	}
	if req.ApplicationId == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "application ID is required")
	}
	if req.Status == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "status is required")
	}
	validStatuses := []string{"PENDING", "APPROVED", "REJECTED", "WITHDRAWN"}
	validStatus := false
	for _, s := range validStatuses {
		if req.Status == s {
			validStatus = true
			break
		}
	}
	if !validStatus {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid status value: must be one of PENDING, APPROVED, REJECTED, or WITHDRAWN")
	}
	return &jobpb.UpdateApplicationStatusResponse{
		Message: "Application status updated successfully",
	}, nil
}

func (s *JobServer) AddJobSkills(ctx context.Context, req *jobpb.AddJobSkillsRequest) (*jobpb.AddJobSkillsResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	userRoles := md.Get("user-role")
	if len(userRoles) == 0 || userRoles[0] != "employer" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "only employers can add skills to jobs")
	}
	jobID := req.JobId
	if jobID == 0 {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job ID is required")
	}
	if req.Skill == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "skill name is required")
	}
	if req.Proficiency == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "proficiency level is required")
	}
	validProficiencies := []string{"Beginner", "Intermediate", "Advanced", "Expert"}
	validProficiency := false
	for _, p := range validProficiencies {
		if req.Proficiency == p {
			validProficiency = true
			break
		}
	}
	if !validProficiency {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid proficiency level")
	}
	skills := []models.JobSkill{
		{
			JobID:       uint(jobID),
			Skill:       req.Skill,
			Proficiency: req.Proficiency,
		},
	}
	err := s.jobUsecase.AddJobSkills(ctx, skills)
	if err != nil {
		return nil, err
	}
	return &jobpb.AddJobSkillsResponse{
		Message: fmt.Sprintf("Successfully added %d skills to job", len(skills)),
	}, nil
}

func (s *JobServer) UpdateJobStatus(ctx context.Context, req *jobpb.UpdateJobStatusRequest) (*jobpb.UpdateJobStatusResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	employerIDs := md.Get("user-id")
	userRoles := md.Get("user-role")
	if len(employerIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing user ID or role in metadata")
	}
	employerID := employerIDs[0]
	userRole := userRoles[0]
	if userRole != "employer" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "only employers can update job status")
	}
	if req.JobId == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job ID is required")
	}
	if req.Status == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "status is required")
	}
	validStatuses := []string{"OPEN", "CLOSED", "DRAFT"}
	validStatus := false
	for _, s := range validStatuses {
		if req.Status == s {
			validStatus = true
			break
		}
	}
	if !validStatus {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "invalid status value: must be one of OPEN, CLOSED, or DRAFT")
	}
	err := s.jobUsecase.UpdateJobStatus(ctx, req.JobId, employerID, req.Status)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to update job status: %v", err)
	}
	return &jobpb.UpdateJobStatusResponse{
		Message: "Job status updated successfully",
	}, nil
}

func (s *JobServer) FilterApplications(ctx context.Context, req *jobpb.FilterApplicationsRequest) (*jobpb.FilterApplicationsResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing metadata")
	}
	employerIDs := md.Get("user-id")
	userRoles := md.Get("user-role")
	if len(employerIDs) == 0 || len(userRoles) == 0 {
		return nil, grpcstatus.Errorf(codes.Unauthenticated, "missing user ID or role in metadata")
	}
	employerID := employerIDs[0]
	userRole := userRoles[0]
	if userRole != "employer" && userRole != "admin" {
		return nil, grpcstatus.Errorf(codes.PermissionDenied, "only employers or admins can filter applications")
	}
	if req.JobId == 0 {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "job ID is required")
	}
	if employerID == "" {
		return nil, grpcstatus.Errorf(codes.InvalidArgument, "employer ID is required")
	}
	filterOptions := map[string]interface{}{}
	applications, err := s.jobUsecase.FilterApplicationsByJob(ctx, fmt.Sprintf("%d", req.JobId), filterOptions)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to filter applications: %v", err)
	}

	
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

		
		if !app.AppliedAt.IsZero() {
			pbApp.AppliedAt = app.AppliedAt.Format(time.RFC3339)
		}

		
		if app.Job != nil {
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
						JobId:       fmt.Sprintf("%d", job.ID),
						Skill:       skill.Skill,
						Proficiency: skill.Proficiency,
					})
				}
				pbJob.RequiredSkills = pbSkills
			}

			
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

		
		
		var matchingSkills []string
		var missingSkills []string

		
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

	
	totalApplications := uint32(len(applications))

	
	response := &jobpb.FilterApplicationsResponse{
		RankedApplications: pbRankedApps,
		TotalApplications:  int32(totalApplications),
		Message:            fmt.Sprintf("Successfully filtered %d applications for job ID %d", totalApplications, req.JobId),
	}

	return response, nil
}
