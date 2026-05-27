package linkedin

import (
	"errors"
	"io"
	"jobSearching/models"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets tests intercept HTTP requests without a real server.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// mockResponse builds a minimal *http.Response for use in roundTripFunc handlers.
func mockResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// swapHTTPClient replaces the package-level httpClient and restores it after the test.
func swapHTTPClient(t *testing.T, fn roundTripFunc) {
	t.Helper()
	orig := httpClient
	httpClient = &http.Client{Transport: fn}
	t.Cleanup(func() { httpClient = orig })
}

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

// --- SearchJobs ---

func TestSearchJobs_InvalidOptions(t *testing.T) {
	resp, err := SearchJobs(SearchOptions{})
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected error for empty SearchOptions, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on validation error, got %v", resp)
	}
}

func TestSearchJobs_URLParams(t *testing.T) {
	var gotURL string
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return mockResponse(200, ""), nil
	})

	opts := SearchOptions{
		Keywords:   KeywordsSoftwareEngineer,
		TimePosted: OneDay,
		WorkType:   models.Remote,
		JobType:    FullTime,
		Start:      20,
	}
	resp, err := SearchJobs(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	checks := []struct {
		param string
		want  string
	}{
		{"keywords", "software%2Bengineer"},
		{"location", "United%2BStates"},
		{"geoId", "103644278"},
		{"f_TPR", string(OneDay)},
		{"f_WT", "2"}, // Remote = 2
		{"f_JT", "F"}, // FullTime
		{"start", "20"},
	}
	for _, c := range checks {
		if !strings.Contains(gotURL, c.param+"="+c.want) {
			t.Errorf("URL missing %s=%s in %s", c.param, c.want, gotURL)
		}
	}
}

func TestSearchJobs_NoJobType(t *testing.T) {
	var gotURL string
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return mockResponse(200, ""), nil
	})

	opts := SearchOptions{
		Keywords:   KeywordsSoftwareEngineer,
		TimePosted: OneDay,
		WorkType:   models.Remote,
	}
	resp, err := SearchJobs(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	if strings.Contains(gotURL, "f_JT") {
		t.Errorf("URL should not contain f_JT when JobType is empty, got %s", gotURL)
	}
}

func TestSearchJobs_NetworkError(t *testing.T) {
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	opts := SearchOptions{
		Keywords:   KeywordsSoftwareEngineer,
		TimePosted: OneDay,
		WorkType:   models.Remote,
	}
	resp, err := SearchJobs(opts)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected network error, got nil")
	}
}

// --- SearchJobId ---

func TestSearchJobId_URL(t *testing.T) {
	var gotURL string
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return mockResponse(200, ""), nil
	})

	resp, err := SearchJobId("abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	const wantSuffix = "/jobPosting/abc123"
	if !strings.HasSuffix(gotURL, wantSuffix) {
		t.Errorf("URL: got %q, want suffix %q", gotURL, wantSuffix)
	}
}

func TestSearchJobId_NetworkError(t *testing.T) {
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	resp, err := SearchJobId("abc123")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected network error, got nil")
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
