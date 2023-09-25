package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v55/github"
)

func main() {
	// Create a new GitHub client
	client := github.NewClient(nil)

	// Get information about a workflow run
	owner := "aws"
	repo := "amazon-cloudwatch-agent"
	var runID int64 = 6202732096
	run, _, err := client.Actions.GetWorkflowRunByID(context.Background(), owner, repo, runID)
	//run, _, err:= client.Actions.
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	jobs, _, err := client.Actions.ListWorkflowJobs(context.TODO(), owner, repo, runID, &github.ListWorkflowJobsOptions{
		Filter: "latest",
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 300,
		},
	})
	fmt.Println("ID:", run.GetID())
	fmt.Println("Status:", run.GetStatus())
	fmt.Println("Conclusion:", run.GetConclusion())
	fmt.Println("Created At:", run.GetCreatedAt())
	fmt.Println("jobs", *jobs.TotalCount)
}
