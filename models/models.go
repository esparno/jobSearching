package models

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
