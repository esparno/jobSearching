package linkedin

import (
	"fmt"
	"jobSearching/models"
	"testing"
)

const jobsHTML = `
<div class="job-search-card" data-entity-urn="urn:li:jobPosting:123456">
    <a class="base-card__full-link" href="https://linkedin.com/jobs/view/123456"></a>
    <h3 class="base-search-card__title">Software Engineer</h3>
    <h4 class="base-search-card__subtitle"><a>Acme Corp</a></h4>
    <span class="job-search-card__location">New York, NY</span>
    <time datetime="2026-05-01"></time>
</div>
<div class="job-search-card" data-entity-urn="urn:li:jobPosting:789012">
    <a class="base-card__full-link" href="https://linkedin.com/jobs/view/789012"></a>
    <h3 class="base-search-card__title">Data Engineer</h3>
    <h4 class="base-search-card__subtitle"><a>Globex</a></h4>
    <span class="job-search-card__location">Remote</span>
    <time datetime="2026-05-02"></time>
</div>`

func TestParseJobs(t *testing.T) {
	jobs, err := ParseJobs(jobsHTML, models.LinkedIn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	j := jobs[0]
	if j.SourceID != "123456" {
		t.Errorf("SourceID: got %q, want %q", j.SourceID, "123456")
	}
	if j.Title != "Software Engineer" {
		t.Errorf("Title: got %q, want %q", j.Title, "Software Engineer")
	}
	if j.Company != "Acme Corp" {
		t.Errorf("Company: got %q, want %q", j.Company, "Acme Corp")
	}
	if j.Location != "New York, NY" {
		t.Errorf("Location: got %q, want %q", j.Location, "New York, NY")
	}
	if j.PostedDate != "2026-05-01" {
		t.Errorf("PostedDate: got %q, want %q", j.PostedDate, "2026-05-01")
	}
	if j.Source != models.LinkedIn {
		t.Errorf("Source: got %q, want %q", j.Source, models.LinkedIn)
	}
}

func TestParseJobs_Empty(t *testing.T) {
	jobs, err := ParseJobs("<html></html>", models.LinkedIn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

const detailHTML = `
<h2 class="top-card-layout__title">Backend Engineer</h2>
<a class="topcard__link" href="https://www.linkedin.com/jobs/view/backend-engineer-at-techco-999?trk=public_jobs_topcard-title">Backend Engineer</a>
<a class="topcard__org-name-link">TechCo</a>
<span class="topcard__flavor--bullet">San Francisco, CA</span>
<span class="posted-time-ago__text">3 days ago</span>
<figcaption class="num-applicants__caption">142 applicants</figcaption>
<div class="show-more-less-html__markup">
    We are looking for a backend engineer to join our team.
</div>
<ul>
    <li class="description__job-criteria-item">
        <h3 class="description__job-criteria-subheader">Seniority level</h3>
        <span class="description__job-criteria-text">Mid-Senior level</span>
    </li>
    <li class="description__job-criteria-item">
        <h3 class="description__job-criteria-subheader">Employment type</h3>
        <span class="description__job-criteria-text">Full-time</span>
    </li>
    <li class="description__job-criteria-item">
        <h3 class="description__job-criteria-subheader">Job function</h3>
        <span class="description__job-criteria-text">Engineering</span>
    </li>
    <li class="description__job-criteria-item">
        <h3 class="description__job-criteria-subheader">Industries</h3>
        <span class="description__job-criteria-text">Software Development</span>
    </li>
</ul>
<code id="decoratedJobPostingId" style="display:none"><!--"999"--></code>`

func TestParseJobDetail(t *testing.T) {
	detail, err := ParseJobDetail(detailHTML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail.Title != "Backend Engineer" {
		t.Errorf("Title: got %q, want %q", detail.Title, "Backend Engineer")
	}
	if detail.Company != "TechCo" {
		t.Errorf("Company: got %q, want %q", detail.Company, "TechCo")
	}
	if detail.Location != "San Francisco, CA" {
		t.Errorf("Location: got %q, want %q", detail.Location, "San Francisco, CA")
	}
	if detail.PostedAgo != "3 days ago" {
		t.Errorf("PostedAgo: got %q, want %q", detail.PostedAgo, "3 days ago")
	}
	if detail.Seniority != "Mid-Senior level" {
		t.Errorf("Seniority: got %q, want %q", detail.Seniority, "Mid-Senior level")
	}
	if detail.EmploymentType != "Full-time" {
		t.Errorf("EmploymentType: got %q, want %q", detail.EmploymentType, "Full-time")
	}
	if detail.JobFunction != "Engineering" {
		t.Errorf("JobFunction: got %q, want %q", detail.JobFunction, "Engineering")
	}
	if detail.Industries != "Software Development" {
		t.Errorf("Industries: got %q, want %q", detail.Industries, "Software Development")
	}
	if detail.Description == "" {
		t.Error("Description should not be empty")
	}
	if detail.ApplyURL != "https://www.linkedin.com/jobs/view/backend-engineer-at-techco-999" {
		t.Errorf("ApplyURL: got %q, want %q", detail.ApplyURL, "https://www.linkedin.com/jobs/view/backend-engineer-at-techco-999")
	}
	if detail.ApplicantsText != "142 applicants" {
		t.Errorf("ApplicantsText: got %q, want %q", detail.ApplicantsText, "142 applicants")
	}
	if detail.Applicants == nil || *detail.Applicants != 142 {
		t.Errorf("Applicants: got %v, want 142", detail.Applicants)
	}
}

func TestParseJobDetail_SeniorityNotApplicable(t *testing.T) {
	html := `
<ul>
    <li class="description__job-criteria-item">
        <h3 class="description__job-criteria-subheader">Seniority level</h3>
        <span class="description__job-criteria-text">Not Applicable</span>
    </li>
</ul>`
	detail, err := ParseJobDetail(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Seniority != "" {
		t.Errorf("Seniority should be empty for 'Not Applicable', got %q", detail.Seniority)
	}
}

func TestParseJobDetail_PayFromCriteria(t *testing.T) {
	html := `
<ul>
    <li class="description__job-criteria-item">
        <h3 class="description__job-criteria-subheader">Base pay range</h3>
        <span class="description__job-criteria-text">$120,000 - $160,000/yr</span>
    </li>
</ul>`
	detail, err := ParseJobDetail(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.PayText == "" {
		t.Error("PayText should not be empty")
	}
	if detail.PayType != models.PayTypeSalary {
		t.Errorf("PayType: got %q, want %q", detail.PayType, models.PayTypeSalary)
	}
}

func ptr(f float64) *float64 { return &f }

var payTests = []struct {
	name        string
	input       string
	wantText    string
	wantMin     *float64
	wantMax     *float64
	wantPayType models.PayType
}{
	{
		name:        "annual range with commas",
		input:       "Salary range: $120,000 - $160,000",
		wantText:    "$120,000 - $160,000",
		wantMin:     ptr(120000),
		wantMax:     ptr(160000),
		wantPayType: models.PayTypeSalary,
	},
	{
		name:        "hourly range with /hr",
		input:       "Pay: $30 - $50/hr",
		wantText:    "$30 - $50",
		wantMin:     ptr(30),
		wantMax:     ptr(50),
		wantPayType: models.PayTypeHourly,
	},
	{
		name:        "K shorthand both",
		input:       "Compensation: $140k - $180k",
		wantText:    "$140k - $180k",
		wantMin:     ptr(140000),
		wantMax:     ptr(180000),
		wantPayType: models.PayTypeSalary,
	},
	{
		name:        "K shorthand on max only",
		input:       "Range: $65-85K",
		wantText:    "$65-85K",
		wantMin:     ptr(65000),
		wantMax:     ptr(85000),
		wantPayType: models.PayTypeSalary,
	},
	{
		name:        "annual with 'to' separator",
		input:       "between $157,250 and $230,000 annually",
		wantText:    "$157,250 and $230,000",
		wantMin:     ptr(157250),
		wantMax:     ptr(230000),
		wantPayType: models.PayTypeSalary,
	},
	{
		name:        "hourly inferred from magnitude",
		input:       "Starting at $25 - $40",
		wantText:    "$25 - $40",
		wantMin:     ptr(25),
		wantMax:     ptr(40),
		wantPayType: models.PayTypeHourly,
	},
	{
		name:        "annual keyword",
		input:       "$160,000 - $220,000 per year",
		wantText:    "$160,000 - $220,000",
		wantMin:     ptr(160000),
		wantMax:     ptr(220000),
		wantPayType: models.PayTypeSalary,
	},
	{
		name:        "no pay info",
		input:       "We offer competitive compensation",
		wantText:    "",
		wantMin:     nil,
		wantMax:     nil,
		wantPayType: "",
	},
	{
		name:        "single hourly value with per hour",
		input:       "The hourly rate for this role is $25.00 per hour.",
		wantText:    "$25.00",
		wantMin:     ptr(25),
		wantMax:     nil,
		wantPayType: models.PayTypeHourly,
	},
	{
		name:        "single hourly value with plus sign",
		input:       "Pay Rate: $70.00+ hourly",
		wantText:    "$70.00+",
		wantMin:     ptr(70),
		wantMax:     nil,
		wantPayType: models.PayTypeHourly,
	},
	{
		name:        "hourly keyword concatenated with next word",
		input:       "Pay Rate: $70.00+ hourlyPosition Overview",
		wantText:    "$70.00+",
		wantMin:     ptr(70),
		wantMax:     nil,
		wantPayType: models.PayTypeHourly,
	},
	{
		name:        "single weekly value",
		input:       "Salary: $1,200/week, 40hrs/week",
		wantText:    "$1,200",
		wantMin:     ptr(1200),
		wantMax:     nil,
		wantPayType: models.PayTypeWeekly,
	},
	{
		name:        "weekly range",
		input:       "$1,000 - $1,500/week",
		wantText:    "$1,000 - $1,500",
		wantMin:     ptr(1000),
		wantMax:     ptr(1500),
		wantPayType: models.PayTypeWeekly,
	},
	{
		name:        "monthly range",
		input:       "$4,000 - $6,000 per month",
		wantText:    "$4,000 - $6,000",
		wantMin:     ptr(4000),
		wantMax:     ptr(6000),
		wantPayType: models.PayTypeMonthly,
	},
	{
		name:        "benefit dollar amount not mistaken for pay",
		input:       "Fertility HRA (up to $10,000 per year) Parental leave: up to 16 weeks at 100% pay",
		wantText:    "",
		wantMin:     nil,
		wantMax:     nil,
		wantPayType: "",
	},
}

func TestParsePayFromText(t *testing.T) {
	for _, tt := range payTests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotMin, gotMax, gotType := parsePayFromText(tt.input)

			if gotText != tt.wantText {
				t.Errorf("payText: got %q, want %q", gotText, tt.wantText)
			}
			if tt.wantMin == nil && gotMin != nil {
				t.Errorf("payMin: expected nil, got %v", *gotMin)
			} else if tt.wantMin != nil && (gotMin == nil || *gotMin != *tt.wantMin) {
				got := "<nil>"
				if gotMin != nil {
					got = fmt.Sprintf("%v", *gotMin)
				}
				t.Errorf("payMin: got %s, want %v", got, *tt.wantMin)
			}
			if tt.wantMax == nil && gotMax != nil {
				t.Errorf("payMax: expected nil, got %v", *gotMax)
			} else if tt.wantMax != nil && (gotMax == nil || *gotMax != *tt.wantMax) {
				got := "<nil>"
				if gotMax != nil {
					got = fmt.Sprintf("%v", *gotMax)
				}
				t.Errorf("payMax: got %s, want %v", got, *tt.wantMax)
			}
			if gotType != tt.wantPayType {
				t.Errorf("payType: got %q, want %q", gotType, tt.wantPayType)
			}
		})
	}
}

var applicantsTests = []struct {
	name      string
	input     string
	wantText  string
	wantCount *int
}{
	{name: "standard", input: "142 applicants", wantText: "142 applicants", wantCount: ptrInt(142)},
	{name: "over prefix", input: "Over 200 applicants", wantText: "Over 200 applicants", wantCount: ptrInt(200)},
	{name: "be among first", input: "Be among the first 25 applicants", wantText: "Be among the first 25 applicants", wantCount: ptrInt(25)},
	{name: "less than", input: "Less than 67 applicants", wantText: "Less than 67 applicants", wantCount: ptrInt(67)},
	{name: "case insensitive", input: "142 Applicants", wantText: "142 Applicants", wantCount: ptrInt(142)},
	{name: "singular", input: "1 applicant", wantText: "1 applicant", wantCount: ptrInt(1)},
	{name: "no number", input: "Be an early applicant", wantText: "Be an early applicant", wantCount: nil},
	{name: "prefers number over no-number", input: "Be an early applicant\n142 applicants", wantText: "142 applicants", wantCount: ptrInt(142)},
	{name: "empty", input: "", wantText: "", wantCount: nil},
}

func TestParseApplicantsText(t *testing.T) {
	for _, tt := range applicantsTests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseApplicantsText(tt.input)
			if got != tt.wantText {
				t.Errorf("got %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestParseApplicantsCount(t *testing.T) {
	for _, tt := range applicantsTests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseApplicantsCount(tt.input)
			if tt.wantCount == nil && got != nil {
				t.Errorf("got %d, want nil", *got)
			} else if tt.wantCount != nil && (got == nil || *got != *tt.wantCount) {
				gotStr := "<nil>"
				if got != nil {
					gotStr = fmt.Sprintf("%d", *got)
				}
				t.Errorf("got %s, want %d", gotStr, *tt.wantCount)
			}
		})
	}
}

func ptrInt(n int) *int { return &n }

func TestParseApplicantsQualifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  models.ApplicantsQualifier
	}{
		{name: "exact", input: "142 applicants", want: models.ApplicantsEqual},
		{name: "over", input: "Over 200 applicants", want: models.ApplicantsGreaterThan},
		{name: "more than", input: "More than 200 applicants", want: models.ApplicantsGreaterThan},
		{name: "less than", input: "Less than 67 applicants", want: models.ApplicantsLessThan},
		{name: "under", input: "Under 50 applicants", want: models.ApplicantsLessThan},
		{name: "among the first", input: "Be among the first 25 applicants", want: models.ApplicantsLessThan},
		{name: "fewer than", input: "Fewer than 10 applicants", want: models.ApplicantsLessThan},
		{name: "no number", input: "Be an early applicant", want: ""},
		{name: "empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseApplicantsQualifier(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
