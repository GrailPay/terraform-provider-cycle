package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWaitForJobCompleted(t *testing.T) {
	withJobPolling(t, time.Millisecond, 3)

	var polls atomic.Int32
	client := testCycleClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/jobs/6a91e91282ea716853c1e0cf" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		polls.Add(1)
		writeJSON(w, http.StatusOK, map[string]any{
			"data": jobPayload("6a91e91282ea716853c1e0cf", "completed", map[string]string{
				"server_id": "6a91e911a985dde810c41b5c",
			}),
		})
	}))

	job, err := waitForJob(context.Background(), client, "6a91e91282ea716853c1e0cf")
	if err != nil {
		t.Fatalf("waitForJob: %v", err)
	}
	if job == nil || job.Id != "6a91e91282ea716853c1e0cf" {
		t.Fatalf("job = %+v", job)
	}
	if got := serverIDFromJob(job); got != "6a91e911a985dde810c41b5c" {
		t.Fatalf("serverIDFromJob = %q", got)
	}
	if polls.Load() != 1 {
		t.Fatalf("polls = %d, want 1", polls.Load())
	}
}

func TestWaitForJobErrorState(t *testing.T) {
	withJobPolling(t, time.Millisecond, 3)

	client := testCycleClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"id":      "6a91e91282ea716853c1e0cf",
				"caption": "provision servers",
				"state": map[string]any{
					"current": "error",
					"error":   map[string]any{"message": "capacity"},
				},
				"tasks": []any{},
			},
		})
	}))

	_, err := waitForJob(context.Background(), client, "6a91e91282ea716853c1e0cf")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrJobNotFound) {
		t.Fatalf("got ErrJobNotFound, want a real job failure: %v", err)
	}
}

func TestWaitForJob404AfterRunningIsJobNotFound(t *testing.T) {
	withJobPolling(t, time.Millisecond, 10)

	var polls atomic.Int32
	client := testCycleClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := polls.Add(1)
		if n == 1 {
			writeJSON(w, http.StatusOK, map[string]any{
				"data": jobPayload("6a91e91282ea716853c1e0cf", "running", nil),
			})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"title": "unable to find job", "code": "404.job"},
		})
	}))

	job, err := waitForJob(context.Background(), client, "6a91e91282ea716853c1e0cf")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
	if job != nil {
		t.Fatalf("job = %+v, want nil", job)
	}
	if polls.Load() != 2 {
		t.Fatalf("polls = %d, want 2 (do not keep retrying 404 after the job was seen)", polls.Load())
	}
}

func TestWaitForJob404FromStartIsJobNotFound(t *testing.T) {
	withJobPolling(t, time.Millisecond, 3)

	var polls atomic.Int32
	client := testCycleClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls.Add(1)
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"title": "unable to find job", "code": "404.job"},
		})
	}))

	_, err := waitForJob(context.Background(), client, "6a91e91282ea716853c1e0cf")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
	if polls.Load() != 3 {
		t.Fatalf("polls = %d, want 3", polls.Load())
	}
}

func TestWaitForJobIgnoreMissingTreatsGoneJobAsSuccess(t *testing.T) {
	withJobPolling(t, time.Millisecond, 2)

	client := testCycleClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"title": "unable to find job", "code": "404.job"},
		})
	}))

	if err := waitForJobIgnoreMissing(context.Background(), client, "6a91e91282ea716853c1e0cf"); err != nil {
		t.Fatalf("waitForJobIgnoreMissing: %v", err)
	}
}

func TestResolveServerAfterProvisionJobRecoversFromGoneJob(t *testing.T) {
	client := testCycleClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/infrastructure/servers" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []any{
				serverPayload("6a91e911aaaaaaaaaaaaaaaa", "production", "loc-1", "model-old", "us-east-1a", "2026-08-28T20:00:00Z"),
				serverPayload("6a91e911bbbbbbbbbbbbbbbb", "production", "loc-1", "model-new", "us-east-1b", "2026-08-28T20:12:00Z"),
			},
		})
	}))

	plan := serverResourceModel{
		Cluster:    types.StringValue("production"),
		LocationID: types.StringValue("loc-1"),
		ModelID:    types.StringValue("model-new"),
		Zone:       types.StringValue("us-east-1b"),
	}

	id, err := resolveServerAfterProvisionJob(context.Background(), client, nil, ErrJobNotFound, plan)
	if err != nil {
		t.Fatalf("resolveServerAfterProvisionJob: %v", err)
	}
	if id != "6a91e911bbbbbbbbbbbbbbbb" {
		t.Fatalf("id = %q, want the newest matching server", id)
	}
}

func TestResolveProvisionedServerIDMatchesZone(t *testing.T) {
	client := testCycleClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []any{
				serverPayload("6a91e911aaaaaaaaaaaaaaaa", "production", "loc-1", "model-new", "us-east-1a", "2026-08-28T20:13:00Z"),
				serverPayload("6a91e911bbbbbbbbbbbbbbbb", "production", "loc-1", "model-new", "us-east-1b", "2026-08-28T20:12:00Z"),
			},
		})
	}))

	plan := serverResourceModel{
		Cluster:    types.StringValue("production"),
		LocationID: types.StringValue("loc-1"),
		ModelID:    types.StringValue("model-new"),
		Zone:       types.StringValue("us-east-1b"),
	}

	id, err := resolveProvisionedServerID(context.Background(), client, nil, plan)
	if err != nil {
		t.Fatalf("resolveProvisionedServerID: %v", err)
	}
	if id != "6a91e911bbbbbbbbbbbbbbbb" {
		t.Fatalf("id = %q, want the zone match, not the newer server in another AZ", id)
	}
}

func withJobPolling(t *testing.T, interval time.Duration, limit int) {
	t.Helper()
	oldInterval, oldLimit := jobPollInterval, jobNotFoundLimit
	jobPollInterval = interval
	jobNotFoundLimit = limit
	t.Cleanup(func() {
		jobPollInterval = oldInterval
		jobNotFoundLimit = oldLimit
	})
}

func testCycleClient(t *testing.T, handler http.Handler) *CycleClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	base := srv.URL
	api, err := cycle.NewAuthenticatedClient(cycle.ClientConfig{
		APIKey:  "test",
		HubID:   "hub",
		BaseURL: &base,
	})
	if err != nil {
		t.Fatalf("NewAuthenticatedClient: %v", err)
	}
	return &CycleClient{Client: api, HubID: "hub"}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		panic(err)
	}
}

func jobPayload(id, state string, output map[string]string) map[string]any {
	payload := map[string]any{
		"id":      id,
		"caption": "provision servers",
		"state":   map[string]any{"current": state},
		"tasks":   []any{},
	}
	if output != nil {
		payload["tasks"] = []any{
			map[string]any{"output": output},
		}
	}
	return payload
}

func serverPayload(id, cluster, locationID, modelID, zone, created string) map[string]any {
	return map[string]any{
		"id":          id,
		"cluster":     cluster,
		"location_id": locationID,
		"model_id":    modelID,
		"hostname":    "host-" + id,
		"hub_id":      "hub",
		"state":       map[string]any{"current": "live"},
		"events": map[string]any{
			"created": created,
		},
		"provider": map[string]any{
			"integration_id": "6a91d890927da18bfc4ceb93",
			"zone":           zone,
		},
		"constraints": map[string]any{
			"allow": map[string]any{},
			"tags":  []string{},
		},
	}
}
