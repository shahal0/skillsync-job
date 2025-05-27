package models

import (
	"time" 
)

type EmployerProfile struct {
	CompanyName string `json:"company_name"`
	Industry    string `json:"industry"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	IsVerified  bool   `json:"is_verified"`
	IsTrusted   bool   `json:"is_trusted"`
}

type Job struct {
	ID                uint            `gorm:"primaryKey;autoIncrement" json:"id"` 
	EmployerID        string          `gorm:"not null" json:"employer_id"`
	EmployerProfile   *EmployerProfile `gorm:"-" json:"employer_profile,omitempty"`
	Title             string          `gorm:"not null" json:"title"`
	Description       string          `gorm:"type:text;not null" json:"description"`
	Category          string          `gorm:"not null" json:"category"`
	RequiredSkills    []JobSkill      `gorm:"foreignKey:JobID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"required_skills"`
	SalaryMin         int64           `gorm:"not null" json:"salary_min"`
	SalaryMax         int64           `gorm:"not null" json:"salary_max"`
	Location          string          `gorm:"not null" json:"location"`
	ExperienceRequired int            `gorm:"not null" json:"experience_required"`
	Status            string          `gorm:"default:'OPEN';not null" json:"status"`
	CreatedAt         time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}
type Application struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID       string    `json:"job_id"`
	CandidateID string    `json:"candidate_id"`
	Status      string    `json:"status"` // Applied, Viewed, Shortlisted, Rejected
	ResumeURL   string    `json:"resume_url"`
	AppliedAt   time.Time `json:"applied_at"`
}

// TableName specifies the table name for the Application model
func (Application) TableName() string {
	return "applications"
}
// TableName specifies the table name for the Job model
func (Job) TableName() string {
	return "jobs"
}

// TableName specifies the table name for the JobSkill model
func (JobSkill) TableName() string {
	return "job_skills"
}

type JobSkill struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID       uint      `gorm:"not null;index" json:"job_id"`
	Skill       string    `gorm:"not null" json:"skill"`
	Proficiency string    `gorm:"not null" json:"proficiency"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Job is the parent model that this skill belongs to
	Job         *Job      `gorm:"foreignKey:JobID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}
type JobPostRequest struct {
    Title             string       `json:"title" binding:"required"`
    Description       string       `json:"description" binding:"required"`
    Category          string       `json:"category" binding:"required"`
    RequiredSkills    []JobSkill   `json:"required_skills" binding:"required"`
    SalaryMin         int64        `json:"salary_min" binding:"required"`
    SalaryMax         int64        `json:"salary_max" binding:"required"`
    Location          string       `json:"location" binding:"required"`
    ExperienceRequired int         `json:"experience_required" binding:"required"`
}
type ApplicationRequest struct {
    JobID       string `json:"job_id" binding:"required"`
    CandidateID string `json:"candidate_id" binding:"required"`
    ResumeURL   string `json:"resume_url" binding:"required"`
}
type ApplicationResponse struct {
    ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    Job         *Job      `json:"job"`
    CandidateID string    `json:"candidate_id"`
    Status      string    `json:"status"`	
    ResumeURL   string    `json:"resume_url"`
    AppliedAt   time.Time `json:"applied_at"`
}
type RankedApplication struct {
    Application    *ApplicationResponse `json:"application"`
    RelevanceScore float64             `json:"relevance_score"`   // Score from 0-100 indicating relevance
    MatchingSkills []string            `json:"matching_skills"`  // Skills that matched job requirements
    MissingSkills  []string            `json:"missing_skills"`   // Skills that were required but missing
}