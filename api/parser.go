package api

import (
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Job struct {
	ID           string
	Title        string
	Company      string
	Location     string
	URL          string
	LinkedInDate string
	FirstSeen    time.Time
}

type JobDetail struct {
	ID             string
	Title          string
	Company        string
	Location       string
	PostedAgo      string
	Applicants     string
	Description    string
	Seniority      string
	EmploymentType string
	JobFunction    string
	Industries     string
}

func ParseJobs(html string) ([]Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var jobs []Job
	doc.Find("div.job-search-card").Each(func(_ int, s *goquery.Selection) {
		job := Job{
			ID:           s.AttrOr("data-entity-urn", ""),
			Title:        strings.TrimSpace(s.Find("h3.base-search-card__title").Text()),
			Company:      strings.TrimSpace(s.Find("h4.base-search-card__subtitle a").Text()),
			Location:     strings.TrimSpace(s.Find("span.job-search-card__location").Text()),
			URL:          s.Find("a.base-card__full-link").AttrOr("href", ""),
			LinkedInDate: s.Find("time").AttrOr("datetime", ""),
			FirstSeen:    time.Now(),
		}
		jobs = append(jobs, job)
	})

	return jobs, nil
}

func ParseJobDetail(html string) (JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return JobDetail{}, err
	}

	detail := JobDetail{
		ID:          strings.TrimSpace(doc.Find("code#decoratedJobPostingId").Text()),
		Title:       strings.TrimSpace(doc.Find("h2.top-card-layout__title").Text()),
		Company:     strings.TrimSpace(doc.Find("a.topcard__org-name-link").Text()),
		Location:    strings.TrimSpace(doc.Find("span.topcard__flavor--bullet").First().Text()),
		PostedAgo:   strings.TrimSpace(doc.Find("span.posted-time-ago__text").Text()),
		Applicants:  strings.TrimSpace(doc.Find("span.num-applicants__caption").Text()),
		Description: strings.TrimSpace(doc.Find("div.show-more-less-html__markup").Text()),
	}

	doc.Find("li.description__job-criteria-item").Each(func(_ int, s *goquery.Selection) {
		header := strings.TrimSpace(s.Find("h3.description__job-criteria-subheader").Text())
		value := strings.TrimSpace(s.Find("span.description__job-criteria-text").Text())
		switch header {
		case "Seniority level":
			detail.Seniority = value
		case "Employment type":
			detail.EmploymentType = value
		case "Job function":
			detail.JobFunction = value
		case "Industries":
			detail.Industries = value
		}
	})

	return detail, nil
}
