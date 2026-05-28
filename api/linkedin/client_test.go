package linkedin

import (
	"net/http"
	"testing"

	"github.com/imroc/req/v3"
)

// cloneWithTransport clones httpClient (preserving headers/settings) and intercepts its transport.
func cloneWithTransport(fn roundTripFunc) *req.Client {
	c := httpClient.Clone()
	c.GetTransport().WrapRoundTripFunc(func(_ http.RoundTripper) req.HttpRoundTripFunc {
		return req.HttpRoundTripFunc(fn)
	})
	return c
}

func TestConfigure_MissingProxy(t *testing.T) {
	t.Setenv("DECODO_PROXY_URL", "")
	if err := Configure(); err == nil {
		t.Error("expected error when DECODO_PROXY_URL is not set")
	}
}

func TestConfigure_SetsProxy(t *testing.T) {
	orig := httpClient
	httpClient = req.NewClient()
	t.Cleanup(func() { httpClient = orig })

	t.Setenv("DECODO_PROXY_URL", "http://user:pass@gate.decodo.com:10001")
	if err := Configure(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAcceptLanguageHeader(t *testing.T) {
	var gotLang string
	orig := httpClient
	httpClient = cloneWithTransport(func(r *http.Request) (*http.Response, error) {
		gotLang = r.Header.Get("Accept-Language")
		return mockResponse(200, ""), nil
	})
	t.Cleanup(func() { httpClient = orig })

	_, _ = SearchJobId("test123")

	const want = "en-US,en;q=0.9"
	if gotLang != want {
		t.Errorf("Accept-Language: got %q, want %q", gotLang, want)
	}
}

func TestNoCookieJar(t *testing.T) {
	if _, err := httpClient.GetCookies("https://www.linkedin.com"); err == nil {
		t.Error("expected GetCookies to fail when cookie jar is disabled")
	}
}

func TestNoCookieForwarding(t *testing.T) {
	callN := 0
	var secondReqCookies []*http.Cookie

	orig := httpClient
	httpClient = cloneWithTransport(func(r *http.Request) (*http.Response, error) {
		callN++
		if callN == 1 {
			resp := mockResponse(200, "")
			resp.Header = http.Header{"Set-Cookie": {"session=abc123; Path=/"}}
			return resp, nil
		}
		secondReqCookies = r.Cookies()
		return mockResponse(200, ""), nil
	})
	t.Cleanup(func() { httpClient = orig })

	_, _ = SearchJobId("111")
	_, _ = SearchJobId("222")

	if len(secondReqCookies) > 0 {
		t.Errorf("cookies leaked into second request: %v", secondReqCookies)
	}
}