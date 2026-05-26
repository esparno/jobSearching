package linkedin

import (
	"strings"

	"jobSearching/models"

	"github.com/PuerkitoBio/goquery"
)

func ParseJobs(html string, source string) ([]models.Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var jobs []models.Job
	doc.Find("div.job-search-card").Each(func(_ int, s *goquery.Selection) {
		urn := s.AttrOr("data-entity-urn", "")
		parts := strings.Split(urn, ":")
		sourceID := parts[len(parts)-1]

		job := models.Job{
			Source:     source,
			SourceID:   sourceID,
			Title:      strings.TrimSpace(s.Find("h3.base-search-card__title").Text()),
			Company:    strings.TrimSpace(s.Find("h4.base-search-card__subtitle a").Text()),
			Location:   strings.TrimSpace(s.Find("span.job-search-card__location").Text()),
			URL:        s.Find("a.base-card__full-link").AttrOr("href", ""),
			PostedDate: s.Find("time").AttrOr("datetime", ""),
		}
		jobs = append(jobs, job)
	})

	return jobs, nil
}

func ParseJobDetail(html string) (models.JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return models.JobDetail{}, err
	}

	detail := models.JobDetail{
		SourceID:    strings.TrimSpace(doc.Find("code#decoratedJobPostingId").Text()),
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
			if value != "Not Applicable" {
				detail.Seniority = value
			}
		case "Employment type":
			detail.EmploymentType = value
		case "Job function":
			detail.JobFunction = value
		case "Industries":
			detail.Industries = value
		case "Base pay range", "Salary range", "Compensation":
			detail.SalaryText = value
		}
	})

	return detail, nil
}
