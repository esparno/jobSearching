package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func buildTsquery(terms []string) (string, error) {
	nonWord := regexp.MustCompile(`\W+`)
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		t = nonWord.ReplaceAllString(t, "")
		if t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no valid search terms after sanitization")
	}
	return strings.Join(parts, " & "), nil
}

func buildDescriptionExclusions(terms []string) string {
	nonWord := regexp.MustCompile(`\W+`)
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		t = nonWord.ReplaceAllString(t, "")
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " | ")
}

func buildTitlePattern(exclusions []string) string {
	if len(exclusions) == 0 {
		return ""
	}
	escaped := make([]string, 0, len(exclusions))
	for _, e := range exclusions {
		e = strings.TrimSpace(e)
		if e != "" {
			escaped = append(escaped, regexp.QuoteMeta(e))
		}
	}
	return strings.Join(escaped, "|")
}

func filterNilStrings(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func internalErr(format string, args ...any) error {
	log.Printf(format, args...)
	return status.Error(codes.Internal, "search failed")
}