package models

import "time"

const (
	LinkedIn = "linkedin"
	Indeed   = "indeed"
)

type WorkType string

const (
	Remote = "remote"
	Onsite = "onsite"
	Hybrid = "hybrid"
)

type Job struct {
	Source     string
	SourceID   string
	Title      string
	Company    string
	Location   string
	URL        string
	PostedDate string // YYYY-MM-DD
}

type ScrapeRun struct {
	Source      string
	Keywords    string
	TimePosted  string
	WorkType    string
	JobType     string
	StartedAt   time.Time
	FinishedAt  time.Time
	JobsFound   int64
	JobsNew     int64
	JobsSkipped int64
}

type JobDetail struct {
	SourceID       string
	Title          string
	Company        string
	Location       string
	PostedAgo      string
	Applicants     string
	Description    string
	Seniority      string
	EmploymentType string
	WorkType       string
	JobFunction    string
	Industries     string
	SalaryMin      *float64
	SalaryMax      *float64
	SalaryText     string
}
