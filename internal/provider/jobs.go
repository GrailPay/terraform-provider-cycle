package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	cycle "github.com/cycleplatform/api-client-go"
)

var (
	defaultJobTimeout = 10 * time.Minute
	jobPollInterval   = 3 * time.Second
	jobNotFoundLimit  = 10
)

// ErrJobNotFound is returned when GET /v1/jobs/{id} keeps 404ing. Cycle
// sometimes deletes a job shortly after it finishes, so callers should try
// to recover from the resulting resource instead of failing hard.
var ErrJobNotFound = errors.New("cycle job no longer available")

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

	notFound := 0
	seen := false
	for {
		resp, err := client.Client.GetJobWithResponse(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("polling job %s: %w", jobID, err)
		}
		if resp.StatusCode() == http.StatusNotFound {
			if seen || notFound+1 >= jobNotFoundLimit {
				return nil, fmt.Errorf("%w: %s", ErrJobNotFound, jobID)
			}
			notFound++
		} else {
			notFound = 0
			if resp.JSON200 == nil {
				return nil, apiError(fmt.Sprintf("polling job %s", jobID), resp.StatusCode(), resp.JSONDefault)
			}

			job := resp.JSON200.Data
			seen = true
			switch job.State.Current {
			case cycle.JobStateCurrentCompleted:
				return &job, nil
			case cycle.JobStateCurrentError, cycle.JobStateCurrentExpired:
				return nil, fmt.Errorf("job %s (%s) finished in state %q: %s",
					jobID, job.Caption, job.State.Current, jobErrorDetail(job))
			}
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for job %s: %w", jobID, ctx.Err())
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

func waitForJobIgnoreMissing(ctx context.Context, client *CycleClient, jobID string) error {
	_, err := waitForJob(ctx, client, jobID)
	if errors.Is(err, ErrJobNotFound) {
		return nil
	}
	return err
}

func resolveServerAfterProvisionJob(ctx context.Context, client *CycleClient, job *cycle.Job, waitErr error, plan serverResourceModel) (string, error) {
	if waitErr != nil && !errors.Is(waitErr, ErrJobNotFound) {
		return "", waitErr
	}
	return resolveProvisionedServerID(ctx, client, job, plan)
}
