package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	cycle "github.com/cycleplatform/api-client-go"
)

const (
	defaultJobTimeout = 10 * time.Minute
	jobPollInterval   = 3 * time.Second
)

// waitForJob polls GET /v1/jobs/{id} until the job reaches a terminal state.
// It returns the final job on success ("completed") and an error containing
// the job/task error messages on failure ("error" or "expired"). The wait is
// bounded by defaultJobTimeout in addition to any deadline already on ctx.
//
// Many Cycle mutations (environment delete, DNS zone tasks, cluster tasks,
// ...) return a cycle.JobDescriptor; pass descriptor.Job.Id here.
func waitForJob(ctx context.Context, client *CycleClient, jobID string) (*cycle.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()

	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	for {
		resp, err := client.Client.GetJobWithResponse(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("polling job %s: %w", jobID, err)
		}
		if resp.JSON200 == nil {
			return nil, apiError(fmt.Sprintf("polling job %s", jobID), resp.StatusCode(), resp.JSONDefault)
		}

		job := resp.JSON200.Data
		switch job.State.Current {
		case cycle.JobStateCurrentCompleted:
			return &job, nil
		case cycle.JobStateCurrentError, cycle.JobStateCurrentExpired:
			return nil, fmt.Errorf("job %s (%s) finished in state %q: %s",
				jobID, job.Caption, job.State.Current, jobErrorDetail(job))
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for job %s (%s), last state %q: %w",
				jobID, job.Caption, job.State.Current, ctx.Err())
		case <-ticker.C:
		}
	}
}

// jobErrorDetail collects the job-level error message plus any per-task
// error messages into a single readable string.
func jobErrorDetail(job cycle.Job) string {
	var parts []string
	if job.State.Error != nil && job.State.Error.Message != "" {
		parts = append(parts, job.State.Error.Message)
	}
	for _, task := range job.Tasks {
		if task.Error != nil && task.Error.Message != "" {
			parts = append(parts, fmt.Sprintf("task %q: %s", task.Caption, task.Error.Message))
		}
	}
	if len(parts) == 0 {
		return "no error message provided"
	}
	return strings.Join(parts, "; ")
}
