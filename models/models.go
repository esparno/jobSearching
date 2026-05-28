package models

import "time"

const (
	LinkedIn = "linkedin"
	Indeed   = "indeed"
)

type WorkType string

const (
	Remote WorkType = "remote"
	Onsite WorkType = "onsite"
	Hybrid WorkType = "hybrid"
)

type PayType string

const (
	PayTypeHourly  PayType = "hourly"
	PayTypeWeekly  PayType = "weekly"
	PayTypeMonthly PayType = "monthly"
	PayTypeSalary  PayType = "salary"
)

type ApplicantsQualifier string

const (
	ApplicantsEqual       ApplicantsQualifier = "="
	ApplicantsGreaterThan ApplicantsQualifier = ">"
	ApplicantsLessThan    ApplicantsQualifier = "<"
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

type RequestLog struct {
	Source         string
	JobSourceID    string
	URL            string
	RequestHeaders string
	StatusCode     *int
	Error          string
	Message        string
	ResponseBody   string
	IsIssue        bool
}

type JobDetail struct {
	SourceID       string
	Title          string
	Company        string
	Location       string
	PostedAgo      string
	ApplicantsText      string
	Applicants          *int
	ApplicantsQualifier ApplicantsQualifier
	Description    string
	Seniority      string
	EmploymentType string
	WorkType       string
	JobFunction    string
	Industries     string
	PayType        PayType
	PayMin         *float64
	PayMax         *float64
	PayText        string
}
