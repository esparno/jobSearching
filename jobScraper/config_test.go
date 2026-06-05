package main

import (
	"jobSearching/api/linkedin"
	"jobSearching/models"
	"testing"
)

func TestParseConfig_Flags(t *testing.T) {
	cfg, err := parseConfig([]string{
		"--keywords=data+engineer",
		"--time-posted=r604800",
		"--work-type=hybrid",
		"--job-type=C",
		"--jobs=20",
		"--start=50",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.opts.Keywords != "data+engineer" {
		t.Errorf("Keywords: got %q, want %q", cfg.opts.Keywords, "data+engineer")
	}
	if cfg.opts.TimePosted != "r604800" {
		t.Errorf("TimePosted: got %q, want %q", cfg.opts.TimePosted, "r604800")
	}
	if cfg.opts.WorkType != models.WorkType("hybrid") {
		t.Errorf("WorkType: got %q, want %q", cfg.opts.WorkType, "hybrid")
	}
	if cfg.opts.JobType != linkedin.JobType("C") {
		t.Errorf("JobType: got %q, want %q", cfg.opts.JobType, "C")
	}
	if cfg.numJobs != 20 {
		t.Errorf("numJobs: got %d, want %d", cfg.numJobs, 20)
	}
	if cfg.opts.Start != 50 {
		t.Errorf("Start: got %d, want %d", cfg.opts.Start, 50)
	}
}

func TestParseConfig_EnvFallback(t *testing.T) {
	t.Setenv("KEYWORDS", "software+engineer")
	t.Setenv("TIME_POSTED", "r86400")
	t.Setenv("WORK_TYPE", "remote")
	t.Setenv("JOB_TYPE", "F")
	t.Setenv("NUM_JOBS", "5")
	t.Setenv("START", "30")

	cfg, err := parseConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.opts.Keywords != "software+engineer" {
		t.Errorf("Keywords: got %q, want %q", cfg.opts.Keywords, "software+engineer")
	}
	if cfg.opts.TimePosted != "r86400" {
		t.Errorf("TimePosted: got %q, want %q", cfg.opts.TimePosted, "r86400")
	}
	if cfg.opts.WorkType != "remote" {
		t.Errorf("WorkType: got %q, want %q", cfg.opts.WorkType, "remote")
	}
	if cfg.opts.JobType != "F" {
		t.Errorf("JobType: got %q, want %q", cfg.opts.JobType, "F")
	}
	if cfg.numJobs != 5 {
		t.Errorf("numJobs: got %d, want %d", cfg.numJobs, 5)
	}
	if cfg.opts.Start != 30 {
		t.Errorf("Start: got %d, want %d", cfg.opts.Start, 30)
	}
}

func TestParseConfig_FlagOverridesEnv(t *testing.T) {
	t.Setenv("KEYWORDS", "data+engineer")

	cfg, err := parseConfig([]string{"--keywords=software+engineer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.opts.Keywords != "software+engineer" {
		t.Errorf("Keywords: got %q, want %q", cfg.opts.Keywords, "software+engineer")
	}
}

func TestParseConfig_DefaultNumJobs(t *testing.T) {
	cfg, err := parseConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.numJobs != 10 {
		t.Errorf("numJobs: got %d, want %d", cfg.numJobs, 10)
	}
}

func TestParseConfig_InvalidNumJobs(t *testing.T) {
	t.Setenv("NUM_JOBS", "not-a-number")

	_, err := parseConfig([]string{})
	if err == nil {
		t.Error("expected error for invalid NUM_JOBS, got nil")
	}
}

func TestParseConfig_DefaultStart(t *testing.T) {
	cfg, err := parseConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.opts.Start != 0 {
		t.Errorf("Start: got %d, want 0", cfg.opts.Start)
	}
}

func TestParseConfig_InvalidStart(t *testing.T) {
	t.Setenv("START", "not-a-number")

	_, err := parseConfig([]string{})
	if err == nil {
		t.Error("expected error for invalid START, got nil")
	}
}

func TestParseConfig_InvalidFlag(t *testing.T) {
	_, err := parseConfig([]string{"--unknown-flag"})
	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}
