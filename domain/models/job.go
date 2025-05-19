package models

import (
	"time" 
)

type Job struct {
	ID                uint        `gorm:"primaryKey;autoIncrement" json:"id"` 
	EmployerID        string      `json:"employer_id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	Category          string    `json:"category"`
	RequiredSkills    []Jobskills `gorm:"type:text[]" json:"required_skills"`
	SalaryMin         int64     `json:"salary_min"`
	SalaryMax         int64     `json:"salary_max"`
	Location          string    `json:"location"`
	ExperienceRequired int      `json:"experience_required"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}
type Application struct {
	ID          string    `gorm:"primaryKey;default:gen_random_uuid()" json:"id"`
	JobID       string    `json:"job_id"`
	CandidateID string    `json:"candidate_id"`
	Status      string    `json:"status"` // Applied, Viewed, Shortlisted, Rejected
	AppliedAt   time.Time `json:"applied_at"`
}
type Jobskills struct {
	JobID      string    `json:"job_id"`
	Skill      string    `json:"skill"`
	Proficiency string   `json:"proficiency"` 
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
type JobSkill struct {
	JobID       string `json:"job_id" binding:"required"`
    Skill       string `json:"skill" binding:"required"`
    Proficiency string `json:"proficiency" binding:"required"` // e.g., Beginner, Intermediate, Expert
}