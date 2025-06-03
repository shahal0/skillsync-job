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

## Configuration
Configuration is loaded from environment variables or a `.env` file. A sample environment file (`.env.sample`) is provided as a template.

### Environment Setup

```bash
# Copy the sample env file
cp .env.sample .env

# Edit the .env file with your specific values
```

### Required Environment Variables

Key environment variables include:

- **Server Configuration**
  - `PORT`: The port on which the service will listen (default: 50052)
  - `ENV`: Environment mode (`development` or `production`)

- **Database Configuration**
  - `DB_HOST`: Database host address
  - `DB_PORT`: Database port
  - `DB_USER`: Database username
  - `DB_PASSWORD`: Database password
  - `DB_NAME`: Database name for job service
  - `DB_SSL_MODE`: SSL mode for database connection

- **Service Dependencies**
  - `AUTH_SERVICE_URL`: URL for the Auth Service
  - `NOTIFICATION_SERVICE_URL`: URL for the Notification Service

- **Search Engine Configuration**
  - `ELASTICSEARCH_URL`: Elasticsearch server URL
  - `ELASTICSEARCH_USERNAME`: Elasticsearch username
  - `ELASTICSEARCH_PASSWORD`: Elasticsearch password

- **File Storage Configuration**
  - `STORAGE_TYPE`: Storage type (`local`, `s3`, `gcs`)
  - `STORAGE_PATH`: Local storage path (if using local storage)
  - Cloud storage credentials (if applicable)

- **Logging**
  - `LOG_LEVEL`: Logging level (`debug`, `info`, `warn`, `error`)

## Running the Service
1. Ensure PostgreSQL is running
2. Ensure the Auth Service is running on port 50051
3. Set up environment variables or `.env` file as described above
4. Run the service:
   ```
   go run main.go
   ```

## Profiling
The Job Service includes built-in profiling capabilities using Go's `pprof` package. The profiling server runs on port 6061.

### Accessing Profiling Data
1. While the service is running, access the profiling interface at: http://localhost:6061/debug/pprof/
2. Available profiles include:
   - CPU profiling: http://localhost:6061/debug/pprof/profile
   - Heap profiling: http://localhost:6061/debug/pprof/heap
   - Goroutine profiling: http://localhost:6061/debug/pprof/goroutine
   - Block profiling: http://localhost:6061/debug/pprof/block
   - Thread creation profiling: http://localhost:6061/debug/pprof/threadcreate

### Using the Go Tool
You can also use the Go tool to analyze profiles:

```bash
# CPU profile (30-second sample)
go tool pprof http://localhost:6061/debug/pprof/profile

# Memory profile
go tool pprof http://localhost:6061/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:6061/debug/pprof/goroutine
```

Once in the pprof interactive mode, you can use commands like `top`, `web`, `list`, etc. to analyze the profile.

## Service Communication
- HTTP server runs on port 8002
- gRPC server runs on port 50052
- Connects to Auth Service on port 50051 for token validation
