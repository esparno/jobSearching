package linkedin

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"jobSearching/models"

	"github.com/PuerkitoBio/goquery"
)

var (
	payRangeRe          = regexp.MustCompile(`\$([\d,]+(?:\.\d+)?)\s*(k|K)?\+?\s*(?:–|—|-|to\b|and\b)\s*\$?([\d,]+(?:\.\d+)?)\s*(k|K)?\+?`)
	singlePayRe         = regexp.MustCompile(`\$([\d,]+(?:\.\d+)?)\s*(k|K)?\+?`)
	hourlyRe              = regexp.MustCompile(`(?i)/hr\b|/hour\b|per\s+hour\b|\bhourly`)
	weeklyRe              = regexp.MustCompile(`(?i)/wk\b|/week\b|per\s+week\b|\bweekly`)
	monthlyRe             = regexp.MustCompile(`(?i)/mo\b|/month\b|per\s+month\b|\bmonthly`)
	annualRe              = regexp.MustCompile(`(?i)/yr\b|/year\b|per\s+year\b|\bannually`)
	nonAnnualPayKeywordRe = regexp.MustCompile(`(?i)/hr\b|/hour\b|per\s+hour\b|\bhourly|/wk\b|/week\b|per\s+week\b|\bweekly|/mo\b|/month\b|per\s+month\b|\bmonthly`)
	applicantsTextRe        = regexp.MustCompile(`(?i)[\w ]+applicants?`)
	applicantsRe            = regexp.MustCompile(`(?i)(\d+)\s+applicants?`)
	applicantsGreaterThanRe = regexp.MustCompile(`(?i)\b(over|more than|greater than)\b`)
	applicantsLessThanRe    = regexp.MustCompile(`(?i)\b(less than|under|among the first|fewer than)\b`)
)

// ParseJobs extracts job listing summaries from a LinkedIn search results HTML page.
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

// ParseJobDetail extracts structured detail data from a LinkedIn job posting HTML page.
func ParseJobDetail(html string) (models.JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return models.JobDetail{}, err
	}

	applyURL := ""
	if href, exists := doc.Find("a.topcard__link").First().Attr("href"); exists {
		if u, err := url.Parse(href); err == nil {
			u.RawQuery = ""
			applyURL = u.String()
		}
	}

	applicantsText := parseApplicantsText(doc.Text())
	detail := models.JobDetail{
		SourceID:       strings.TrimSpace(doc.Find("code#decoratedJobPostingId").Text()),
		Title:          strings.TrimSpace(doc.Find("h2.top-card-layout__title").Text()),
		Company:        strings.TrimSpace(doc.Find("a.topcard__org-name-link").Text()),
		Location:       strings.TrimSpace(doc.Find("span.topcard__flavor--bullet").First().Text()),
		PostedAgo:      strings.TrimSpace(doc.Find("span.posted-time-ago__text").Text()),
		ApplicantsText:      applicantsText,
		Applicants:          parseApplicantsCount(applicantsText),
		ApplicantsQualifier: parseApplicantsQualifier(applicantsText),
		Description:    strings.TrimSpace(doc.Find("div.show-more-less-html__markup").Text()),
		ApplyURL:       applyURL,
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
			detail.PayText = value
		}
	})

	if detail.PayText != "" {
		_, detail.PayMin, detail.PayMax, detail.PayType = parsePayFromText(detail.PayText)
	} else {
		detail.PayText, detail.PayMin, detail.PayMax, detail.PayType = parsePayFromText(detail.Description)
	}

	return detail, nil
}

// parsePayFromText scans free-form text for a pay range or single pay value.
// Returns empty values when no recognisable pay is found.
func parsePayFromText(text string) (payText string, payMin, payMax *float64, payType models.PayType) {
	loc := payRangeRe.FindStringIndex(text)
	if loc != nil {
		payText = strings.TrimSpace(text[loc[0]:loc[1]])
		groups := payRangeRe.FindStringSubmatch(text)

		minVal := parseAmount(groups[1], groups[2])
		maxVal := parseAmount(groups[3], groups[4])

		// Handle "$65-85K" where K applies to both (e.g. shorthand for $65K-$85K)
		if strings.EqualFold(groups[4], "k") && !strings.EqualFold(groups[2], "k") && minVal < 1000 {
			minVal *= 1000
		}

		payMin = &minVal
		payMax = &maxVal

		ctxStart := max(0, loc[0]-100)
		ctxEnd := min(len(text), loc[1]+100)
		payType = detectPayType(text[ctxStart:ctxEnd], minVal)
		return
	}

	// Try single value. Require an explicit non-annual pay keyword nearby to
	// avoid misclassifying benefit dollar amounts like "up to $10,000 per year".
	for _, m := range singlePayRe.FindAllStringSubmatchIndex(text, -1) {
		matchStart, matchEnd := m[0], m[1]
		ctxStart := max(0, matchStart-100)
		ctxEnd := min(len(text), matchEnd+100)
		context := text[ctxStart:ctxEnd]

		if !nonAnnualPayKeywordRe.MatchString(context) {
			continue
		}

		k := ""
		if m[4] >= 0 {
			k = text[m[4]:m[5]]
		}
		val := parseAmount(text[m[2]:m[3]], k)
		payText = strings.TrimSpace(text[matchStart:matchEnd])
		payMin = &val
		payType = detectPayType(context, val)
		return
	}

	return
}

// detectPayType infers pay frequency from surrounding text, falling back to magnitude.
func detectPayType(context string, val float64) models.PayType {
	switch {
	case hourlyRe.MatchString(context):
		return models.PayTypeHourly
	case weeklyRe.MatchString(context):
		return models.PayTypeWeekly
	case monthlyRe.MatchString(context):
		return models.PayTypeMonthly
	case annualRe.MatchString(context):
		return models.PayTypeSalary
	case val < 500:
		return models.PayTypeHourly
	default:
		return models.PayTypeSalary
	}
}

// parseApplicantsText extracts the most informative applicants phrase from text.
// Prefers a phrase containing an actual number over a vague one like "Be an early applicant".
func parseApplicantsText(text string) string {
	matches := applicantsTextRe.FindAllString(text, -1)
	for _, m := range matches {
		if applicantsRe.MatchString(m) {
			return strings.TrimSpace(m)
		}
	}
	if len(matches) > 0 {
		return strings.TrimSpace(matches[0])
	}
	return ""
}

// parseApplicantsQualifier returns whether the applicant count is exact, a lower bound, or an upper bound.
// Returns an empty string when no numeric count is present.
func parseApplicantsQualifier(text string) models.ApplicantsQualifier {
	if text == "" || !applicantsRe.MatchString(text) {
		return ""
	}
	if applicantsGreaterThanRe.MatchString(text) {
		return models.ApplicantsGreaterThan
	}
	if applicantsLessThanRe.MatchString(text) {
		return models.ApplicantsLessThan
	}
	return models.ApplicantsEqual
}

// parseApplicantsCount extracts the numeric applicant count from a phrase like "142 applicants".
// Returns nil when no number is found.
func parseApplicantsCount(text string) *int {
	groups := applicantsRe.FindStringSubmatch(text)
	if groups == nil {
		return nil
	}
	n, err := strconv.Atoi(groups[1])
	if err != nil {
		return nil
	}
	return &n
}

// parseAmount converts a digit string and optional "k"/"K" suffix into a numeric value.
func parseAmount(digits, k string) float64 {
	val, _ := strconv.ParseFloat(strings.ReplaceAll(digits, ",", ""), 64)
	if strings.EqualFold(k, "k") {
		val *= 1000
	}
	return val
}