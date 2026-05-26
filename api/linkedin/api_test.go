package linkedin

import (
	"jobSearching/models"
	"net/http"
	"testing"
)

// --- SearchOptions.Validate ---

var validateTests = []struct {
	name    string
	opts    SearchOptions
	wantErr string
}{
	{
		name: "valid",
		opts: SearchOptions{
			Keywords:   KeywordsSoftwareEngineer,
			TimePosted: OneDay,
			WorkType:   models.Remote,
		},
	},
	{
		name: "valid with job type",
		opts: SearchOptions{
			Keywords:   KeywordsDataEngineer,
			TimePosted: OneWeek,
			WorkType:   models.Hybrid,
			JobType:    FullTime,
		},
	},
	{
		name:    "missing keywords",
		opts:    SearchOptions{TimePosted: OneDay, WorkType: models.Remote},
		wantErr: "Keywords is required",
	},
	{
		name:    "missing time posted",
		opts:    SearchOptions{Keywords: KeywordsSoftwareEngineer, WorkType: models.Remote},
		wantErr: "TimePosted is required",
	},
	{
		name:    "missing work type",
		opts:    SearchOptions{Keywords: KeywordsSoftwareEngineer, TimePosted: OneDay},
		wantErr: "WorkType is required",
	},
	{
		name:    "all empty",
		opts:    SearchOptions{},
		wantErr: "Keywords is required",
	},
}

func TestSearchOptions_Validate(t *testing.T) {
	for _, tt := range validateTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %q, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("error: got %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// --- SearchJobs validation ---

func TestSearchJobs_InvalidOptions(t *testing.T) {
	resp, err := SearchJobs(SearchOptions{})
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected error for empty SearchOptions, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on validation error, got %v", resp)
	}
}

// --- headersToJSON ---

var headersToJSONTests = []struct {
	name    string
	headers http.Header
	want    string
}{
	{
		name:    "empty",
		headers: http.Header{},
		want:    "{}",
	},
	{
		name:    "single header",
		headers: http.Header{"Content-Type": {"application/json"}},
		want:    `{"Content-Type":"application/json"}`,
	},
	{
		name:    "multi-value header joined",
		headers: http.Header{"Accept": {"text/html", "application/json"}},
		want:    `{"Accept":"text/html, application/json"}`,
	},
}

func TestHeadersToJSON(t *testing.T) {
	for _, tt := range headersToJSONTests {
		t.Run(tt.name, func(t *testing.T) {
			got := headersToJSON(tt.headers)
			if got != tt.want {
				t.Errorf("headersToJSON: got %q, want %q", got, tt.want)
			}
		})
	}
}
