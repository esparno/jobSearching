package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Keywords string

const (
	KeywordsSoftwareEngineer Keywords = "software+engineer"
	KewwordsDataEngineer     Keywords = "data+engineer"
)

type timePostedLookup string

const (
	OneDay  timePostedLookup = "r86400"
	OneHr   timePostedLookup = "r3600"
	OneWeek timePostedLookup = "r86400"
)

type workType int

const (
	onsite workType = 1
	remote workType = 2
	hybrid workType = 3
)

type jobType string

const (
	fulltime  jobType = "F"
	parttime  jobType = "P"
	contract  jobType = "C"
	temporary jobType = "T"
)

type SearchOptions struct {
	JobType  jobType
	WorkType workType
	Start    int
}

//location=New+York
//locationId=         # LinkedIn's internal geo ID
//geoId=              # alternative geo identifier
//f_TPR=r86400        # time posted: r3600=1hr, r86400=24hr, r604800=week
//f_WT=2              # work type: 1=onsite, 2=remote, 3=hybrid
//f_E=2,3             # experience: 1=internship, 2=entry, 3=associate, 4=mid, 5=director, 6=executive
//f_JT=F,P,C          # job type: F=fulltime, P=parttime, C=contract, T=temporary
//start=0             # pagination offset, increments of 10

func SearchJobs(searchTerm Keywords, timePeriod timePostedLookup, searchOptions SearchOptions) (*http.Response, error) {

	searchUrl := "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"
	location := "United+States"

	params := url.Values{}
	params.Set("keywords", string(searchTerm))
	params.Set("location", location)
	params.Set("f_TPR", string(timePeriod))
	searchOptions.AddSearchParams(params)

	fmt.Println(params.Encode())

	getUrl := searchUrl + "?" + params.Encode()
	return http.Get(getUrl)
}

func (searchOptions SearchOptions) AddSearchParams(params url.Values) {
	if searchOptions.JobType != "" {
		params.Set("f_JT", string(searchOptions.JobType))
	}
	if searchOptions.WorkType > 0 {
		params.Set("f_WT", strconv.Itoa(int(searchOptions.WorkType)))
	}
	params.Set("start", strconv.Itoa(searchOptions.Start))

}

func SearchJobId(jobId string) (*http.Response, error) {
	postingUrl := "https://www.linkedin.com/jobs-guest/jobs/api/jobPosting/" + jobId
	return http.Get(postingUrl)
}
