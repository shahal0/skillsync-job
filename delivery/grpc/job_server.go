package grpc

import (
	"context"
	"log"

	"jobservice/usecase"

	"github.com/shahal0/skillsync-protos/gen/jobpb"
	"google.golang.org/grpc/metadata"
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
	log.Printf("\n\n====== GRPC DEBUG: ApplyToJob gRPC method called ======")
	log.Printf("GRPC DEBUG: Request parameters - JobID: %s, CandidateID: %s", req.JobId, req.CandidateId)

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
	log.Printf("GRPC DEBUG: Calling usecase.ApplyToJob with candidateID=%s, jobID=%s", req.CandidateId, req.JobId)
	applicationID, err := s.jobUsecase.ApplyToJob(ctx, req.CandidateId, req.JobId)
	if err != nil {
		log.Printf("GRPC ERROR: Failed to apply to job: %v", err)
		return nil, err
	}
	log.Printf("GRPC DEBUG: Application ID generated: %s", applicationID)

	log.Printf("GRPC SUCCESS: Successfully applied to job %s for candidate %s with application ID: %s", req.JobId, req.CandidateId, applicationID)

	// Return success response with application ID
	return &jobpb.ApplyToJobResponse{
		ApplicationId: applicationID,
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

	// Implementation for posting a job
	// This is just a stub - you'll need to implement this with your actual logic
	return &jobpb.PostJobResponse{
		Message: "Job posted successfully",
	}, nil
}

// GetJobs implements the GetJobs gRPC method
func (s *JobServer) GetJobs(ctx context.Context, req *jobpb.GetJobsRequest) (*jobpb.GetJobsResponse, error) {
	// Implementation for getting jobs
	// This is just a stub - you'll need to implement this
	return &jobpb.GetJobsResponse{}, nil
}

// GetJobById implements the GetJobById gRPC method
func (s *JobServer) GetJobById(ctx context.Context, req *jobpb.GetJobByIdRequest) (*jobpb.GetJobByIdResponse, error) {
	// Implementation for getting a job by ID
	// This is just a stub - you'll need to implement this
	return &jobpb.GetJobByIdResponse{}, nil
}

// GetApplications implements the GetApplications gRPC method
func (s *JobServer) GetApplications(ctx context.Context, req *jobpb.GetApplicationsRequest) (*jobpb.GetApplicationsResponse, error) {
	// Implementation for getting applications
	// This is just a stub - you'll need to implement this
	return &jobpb.GetApplicationsResponse{}, nil
}

// UpdateApplicationStatus implements the UpdateApplicationStatus gRPC method
func (s *JobServer) UpdateApplicationStatus(ctx context.Context, req *jobpb.UpdateApplicationStatusRequest) (*jobpb.UpdateApplicationStatusResponse, error) {
	// Implementation for updating application status
	// This is just a stub - you'll need to implement this
	return &jobpb.UpdateApplicationStatusResponse{
		Message: "Application status updated successfully",
	}, nil
}

// AddJobSkills implements the AddJobSkills gRPC method
func (s *JobServer) AddJobSkills(ctx context.Context, req *jobpb.AddJobSkillsRequest) (*jobpb.AddJobSkillsResponse, error) {
	// Implementation for adding job skills
	// This is just a stub - you'll need to implement this
	return &jobpb.AddJobSkillsResponse{
		Message: "Job skills added successfully",
	}, nil
}
