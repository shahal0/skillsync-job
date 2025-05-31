# Job Service

## Overview
The Job Service is a core component of the SkillSync platform that handles job posting, job searching, and application management. It provides both RESTful HTTP endpoints via Gin and gRPC interfaces for other services to interact with.

## Features
- Job posting and management for employers
- Job search and filtering for candidates
- Job application submission and tracking
- Integration with Auth Service for user validation

## Technical Stack
- Go (Golang)
- gRPC/Protocol Buffers for service-to-service communication
- Gin framework for REST API
- PostgreSQL with GORM ORM
- JWT for authentication validation (via Auth Service)

## Service Architecture
The service follows a clean architecture pattern:
- **Domain**: Contains business models and repository interfaces
- **Repository**: Implements data access logic
- **Usecase**: Implements business logic
- **Delivery**: Handles HTTP and gRPC endpoints

## API Endpoints
### HTTP Endpoints (port 8002)
- Job CRUD operations
- Job search and filtering
- Application submission and management

### gRPC Endpoints (port 50052)
- Job operations for internal service communication

## Authentication
The Job Service communicates with the Auth Service (port 50051) to validate user tokens and permissions. All protected endpoints require a valid JWT token in the Authorization header with the format "Bearer {token}".

## Database Models
- Job: Represents job postings with details like title, description, and requirements
- Application: Tracks job applications from candidates
- JobSkill: Maps skills required for specific jobs

## Running the Service
1. Ensure PostgreSQL is running
2. Ensure the Auth Service is running on port 50051
3. Set up environment variables or `.env` file
4. Run the service:
   ```
   go run main.go
   ```

## Service Communication
- HTTP server runs on port 8002
- gRPC server runs on port 50052
- Connects to Auth Service on port 50051 for token validation
