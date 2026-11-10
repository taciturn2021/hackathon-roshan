// Package queue is the contract between the API (which enqueues) and internal/tasks (which handles).
//
// The API enqueues a task when something happens that needs slow AI work the user is not waiting on
// (TRD "inline vs queue" rule). internal/tasks registers one asynq handler per Type below; each handler
// does its AI work and writes the result back through the module services, which also push the
// WebSocket update:
//
//	TypeTranscribe     → speech-to-text, Jev "contains a price?", LLM price extraction → jobs.SetTranscript
//	TypeJobBrief       → Urdu text-to-speech of the job summary                       → jobs.SetBriefAudio
//	TypeReviewAnalyze  → transcript of a voice review, Jev review tags                → reviews.SetTags,
//	                     then a new trust summary from the worker's reviews            → workers.SetTrustSummary
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Task types.
const (
	TypeTranscribe    = "message:transcribe" // payload MessagePayload: a voice message in job chat
	TypeJobBrief      = "job:brief"          // payload JobPayload: a newly created job
	TypeReviewAnalyze = "review:analyze"     // payload JobPayload: the job whose review was just saved
)

// MessagePayload identifies a chat message.
type MessagePayload struct {
	MessageID int64 `json:"message_id"`
}

// JobPayload identifies a job (and, for TypeReviewAnalyze, its review).
type JobPayload struct {
	JobID int64 `json:"job_id"`
}

// TRD: queued steps retry 3 times with backoff; each AI call has a 15s timeout, so 60s covers a task's calls.
const (
	maxRetry = 3
	timeout  = 60 * time.Second
)

// Client enqueues tasks. Safe for concurrent use.
type Client struct{ c *asynq.Client }

// NewClient connects to the same Redis the worker process reads from.
func NewClient(opt asynq.RedisConnOpt) *Client { return &Client{asynq.NewClient(opt)} }

// Close releases the Redis connection.
func (q *Client) Close() error { return q.c.Close() }

// enqueue adds a task. The task id makes enqueueing idempotent: a duplicate for the same object is rejected
// by asynq (ErrTaskIDConflict), which is not an error for callers.
func (q *Client) enqueue(ctx context.Context, typ string, id int64, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = q.c.EnqueueContext(ctx, asynq.NewTask(typ, b),
		asynq.MaxRetry(maxRetry), asynq.Timeout(timeout), asynq.TaskID(fmt.Sprintf("%s:%d", typ, id)))
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

// Transcribe queues speech-to-text and price detection for a voice message.
func (q *Client) Transcribe(ctx context.Context, messageID int64) error {
	return q.enqueue(ctx, TypeTranscribe, messageID, MessagePayload{messageID})
}

// JobBrief queues the Urdu read-aloud summary of a new job for the worker.
func (q *Client) JobBrief(ctx context.Context, jobID int64) error {
	return q.enqueue(ctx, TypeJobBrief, jobID, JobPayload{jobID})
}

// ReviewAnalyze queues AI tagging of a new review and a refresh of the worker's trust summary.
func (q *Client) ReviewAnalyze(ctx context.Context, jobID int64) error {
	return q.enqueue(ctx, TypeReviewAnalyze, jobID, JobPayload{jobID})
}
