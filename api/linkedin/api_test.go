package linkedin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"jobSearching/models"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/imroc/req/v3"
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

// disableDelays replaces delayFn with a no-op for the duration of the test.
func disableDelays(t *testing.T) {
	t.Helper()
	orig := delayFn
	delayFn = func() {}
	t.Cleanup(func() { delayFn = orig })
}

// jobSearchHTML returns minimal HTML containing one parseable job card.
func jobSearchHTML(sourceID string) string {
	return fmt.Sprintf(`<div class="job-search-card" data-entity-urn="urn:li:jobPosting:%s">
		<a class="base-card__full-link" href="https://linkedin.com/jobs/view/%s"></a>
		<h3 class="base-search-card__title">Software Engineer</h3>
		<h4 class="base-search-card__subtitle"><a>Acme Corp</a></h4>
		<span class="job-search-card__location">Remote</span>
		<time datetime="2026-05-01"></time>
	</div>`, sourceID, sourceID)
}

// swapHTTPClient replaces the package-level httpClient and restores it after the test.
func swapHTTPClient(t *testing.T, fn roundTripFunc) {
	t.Helper()
	orig := httpClient
	c := req.NewClient()
	c.GetTransport().WrapRoundTripFunc(func(_ http.RoundTripper) req.HttpRoundTripFunc {
		return req.HttpRoundTripFunc(fn)
	})
	httpClient = c
	t.Cleanup(func() { httpClient = orig })
}

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

func TestSearchJobs_InvalidOptions(t *testing.T) {
	resp, err := SearchJobs(context.Background(), SearchOptions{})
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected error for empty SearchOptions, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on validation error, got %v", resp)
	}
}

func TestSearchJobs_URLParams(t *testing.T) {
	disableDelays(t)
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
	resp, err := SearchJobs(context.Background(), opts)
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
	disableDelays(t)
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
	resp, err := SearchJobs(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	if strings.Contains(gotURL, "f_JT") {
		t.Errorf("URL should not contain f_JT when JobType is empty, got %s", gotURL)
	}
}

func TestSearchJobs_NetworkError(t *testing.T) {
	disableDelays(t)
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	opts := SearchOptions{
		Keywords:   KeywordsSoftwareEngineer,
		TimePosted: OneDay,
		WorkType:   models.Remote,
	}
	resp, err := SearchJobs(context.Background(), opts)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected network error, got nil")
	}
}

func TestSearchJobId_URL(t *testing.T) {
	disableDelays(t)
	var gotURL string
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return mockResponse(200, ""), nil
	})

	resp, err := SearchJobId(context.Background(), "abc123")
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
	disableDelays(t)
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	resp, err := SearchJobId(context.Background(), "abc123")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected network error, got nil")
	}
}

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

func TestProcessJob_NetworkError(t *testing.T) {
	disableDelays(t)
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	var found atomic.Int64
	opts := SearchOptions{Keywords: KeywordsSoftwareEngineer, TimePosted: OneDay, WorkType: models.Remote}

	_, err := processJob(context.Background(), nil, opts, &found, "run-1")
	if err == nil {
		t.Error("expected error on network failure, got nil")
	}
	if found.Load() != 0 {
		t.Errorf("found: got %d, want 0", found.Load())
	}
}

func TestProcessJob_EmptyPage(t *testing.T) {
	disableDelays(t)
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		return mockResponse(200, ""), nil
	})

	var found atomic.Int64
	opts := SearchOptions{Keywords: KeywordsSoftwareEngineer, TimePosted: OneDay, WorkType: models.Remote}

	n, err := processJob(context.Background(), nil, opts, &found, "run-1")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("n: got %d, want 0", n)
	}
	if found.Load() != 0 {
		t.Errorf("found: got %d, want 0", found.Load())
	}
}

func TestProcessAllJobs_StopsAfterConsecutiveEmpties(t *testing.T) {
	disableDelays(t)
	var requests atomic.Int32
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		return mockResponse(200, ""), nil
	})

	opts := SearchOptions{Keywords: KeywordsSoftwareEngineer, TimePosted: OneDay, WorkType: models.Remote}
	processAllJobs(context.Background(), nil, opts, 100, new(atomic.Int64), "run-1")

	if int(requests.Load()) != maxConsecutiveEmpties {
		t.Errorf("requests: got %d, want %d", requests.Load(), maxConsecutiveEmpties)
	}
}

func TestProcessAllJobs_ResetsConsecutiveEmptiesOnResults(t *testing.T) {
	disableDelays(t)
	var requests atomic.Int32
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		n := requests.Add(1)
		if n <= 2 {
			return mockResponse(200, jobSearchHTML(fmt.Sprintf("job-%d", n))), nil
		}
		return mockResponse(200, ""), nil
	})

	opts := SearchOptions{Keywords: KeywordsSoftwareEngineer, TimePosted: OneDay, WorkType: models.Remote}
	processAllJobs(context.Background(), nil, opts, 100, new(atomic.Int64), "run-1")

	want := int32(2 + maxConsecutiveEmpties)
	if requests.Load() != want {
		t.Errorf("requests: got %d, want %d", requests.Load(), want)
	}
}

func TestProcessAllJobs_ProcessesInOrder(t *testing.T) {
	disableDelays(t)
	var starts []string
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		starts = append(starts, r.URL.Query().Get("start"))
		return mockResponse(200, ""), nil
	})

	opts := SearchOptions{Keywords: KeywordsSoftwareEngineer, TimePosted: OneDay, WorkType: models.Remote}
	processAllJobs(context.Background(), nil, opts, 100, new(atomic.Int64), "run-1")

	for i := 1; i < len(starts); i++ {
		if starts[i] <= starts[i-1] {
			t.Errorf("out of order at index %d: %q after %q", i, starts[i], starts[i-1])
		}
	}
}

func TestProcessAllJobs_ErrorResetsConsecutiveEmpties(t *testing.T) {
	disableDelays(t)
	var requests atomic.Int32
	swapHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		n := requests.Add(1)
		if n == 3 {
			return nil, errors.New("connection refused")
		}
		return mockResponse(200, ""), nil
	})

	opts := SearchOptions{Keywords: KeywordsSoftwareEngineer, TimePosted: OneDay, WorkType: models.Remote}
	processAllJobs(context.Background(), nil, opts, 100, new(atomic.Int64), "run-1")

	// 2 empties → error resets to 0 → 5 more empties = 8 total
	want := int32(2 + 1 + maxConsecutiveEmpties)
	if requests.Load() != want {
		t.Errorf("requests: got %d, want %d", requests.Load(), want)
	}
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
