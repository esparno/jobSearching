package main

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBuildTsquery(t *testing.T) {
	tests := []struct {
		name    string
		terms   []string
		want    string
		wantErr bool
	}{
		{name: "single term", terms: []string{"scala"}, want: "scala"},
		{name: "multiple terms", terms: []string{"scala", "python"}, want: "scala & python"},
		{name: "trims whitespace", terms: []string{" scala "}, want: "scala"},
		{name: "strips special chars", terms: []string{"scala!"}, want: "scala"},
		{name: "empty slice", terms: []string{}, wantErr: true},
		{name: "all invalid after sanitize", terms: []string{"!!!", "---"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTsquery(tt.terms)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDescriptionExclusions(t *testing.T) {
	tests := []struct {
		name  string
		terms []string
		want  string
	}{
		{name: "empty", terms: []string{}, want: ""},
		{name: "single term", terms: []string{"management"}, want: "management"},
		{name: "multiple terms", terms: []string{"management", "consulting"}, want: "management | consulting"},
		{name: "strips special chars", terms: []string{"management!"}, want: "management"},
		{name: "trims whitespace", terms: []string{" management "}, want: "management"},
		{name: "skips empty after sanitize", terms: []string{"!!!"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDescriptionExclusions(tt.terms)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildTitlePattern(t *testing.T) {
	tests := []struct {
		name string
		excl []string
		want string
	}{
		{name: "empty", excl: []string{}, want: ""},
		{name: "single", excl: []string{"manager"}, want: "manager"},
		{name: "multiple", excl: []string{"manager", "director"}, want: "manager|director"},
		{name: "escapes regex special chars", excl: []string{"C++"}, want: `C\+\+`},
		{name: "trims whitespace", excl: []string{" manager "}, want: "manager"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTitlePattern(tt.excl)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterNilStrings(t *testing.T) {
	got := filterNilStrings([]string{"a", "", "b", "", "c"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFilterNilStrings_AllEmpty(t *testing.T) {
	got := filterNilStrings([]string{"", "", ""})
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestInternalErr(t *testing.T) {
	err := internalErr("test: %v", "something bad")
	s, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if s.Code() != codes.Internal {
		t.Errorf("code = %v, want Internal", s.Code())
	}
	if s.Message() != "search failed" {
		t.Errorf("message = %q, want %q", s.Message(), "search failed")
	}
}