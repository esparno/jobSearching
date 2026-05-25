package main

import (
	"fmt"
	"io"
	"jobSearching/api"
	"log"
)

func main() {
	//searchOptions := api.SearchOptions{}
	//resp, err := api.SearchJobs(api.KeywordsSoftwareEngineer, api.OneHr, searchOptions)
	//logFatal(err)
	//
	//defer resp.Body.Close()
	//
	//body, err := io.ReadAll(resp.Body)
	//logFatal(err)
	//
	//jobs, err := api.ParseJobs(string(body))
	//logFatal(err)
	//
	//for _, job := range jobs {
	//	fmt.Printf("%s | %s | %s | %s\n", job.ID, job.Title, job.Company, job.Location)
	//}

	resp, err := api.SearchJobId("4415252386")
	logFatal(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	logFatal(err)

	detail, err := api.ParseJobDetail(string(body))
	logFatal(err)

	fmt.Printf("ID:          %s\n", detail.ID)
	fmt.Printf("Title:       %s\n", detail.Title)
	fmt.Printf("Company:     %s\n", detail.Company)
	fmt.Printf("Location:    %s\n", detail.Location)
	fmt.Printf("Posted:      %s\n", detail.PostedAgo)
	fmt.Printf("Applicants:  %s\n", detail.Applicants)
	fmt.Printf("Seniority:   %s\n", detail.Seniority)
	fmt.Printf("Type:        %s\n", detail.EmploymentType)
	fmt.Printf("Function:    %s\n", detail.JobFunction)
	fmt.Printf("Industries:  %s\n", detail.Industries)
	fmt.Printf("Description: %s\n", detail.Description[:200])
}

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
