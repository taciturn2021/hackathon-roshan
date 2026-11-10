package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/ai"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/jobs"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/queue"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

// The methods each handler uses; satisfied by *jobs.Service, *reviews.Service, *workers.Service,
// platform.Media and *ai.Service, and by fakes in tests.
type (
	Jobs interface {
		MessageByID(ctx context.Context, messageID int64) (jobs.Message, error)
		JobRow(ctx context.Context, jobID int64) (jobs.Row, error)
		SetTranscript(ctx context.Context, messageID int64, body string, suggestedPrice *int) error
		SetMessageSpeech(ctx context.Context, messageID int64, bodyUr, speechURL string) error
		SetBrief(ctx context.Context, jobID int64, briefUr, audioURL string) error
	}
	Reviews interface {
		ByJob(ctx context.Context, jobID int64) (workers.Review, error)
		ForWorker(ctx context.Context, workerID int64) ([]workers.Review, error)
		SetTags(ctx context.Context, jobID int64, tags []string) error
	}
	Workers interface {
		Card(ctx context.Context, userID int64, from *workers.LatLng) (*workers.WorkerCard, error)
		SetTrustSummary(ctx context.Context, workerID int64, summary string) error
	}
	Media interface {
		Path(url string) (string, error)
		SaveBytes(kind, ext string, data []byte) (string, error)
	}
	AI interface {
		Transcribe(ctx context.Context, path string) (string, error)
		HasPrice(ctx context.Context, text string) (bool, error)
		Price(ctx context.Context, text string) (*int, error)
		UrduMessage(ctx context.Context, text string) (string, error)
		JobBrief(ctx context.Context, in ai.BriefInput) (string, error)
		Speak(ctx context.Context, text string) ([]byte, string, error)
		ReviewTags(ctx context.Context, text string) ([]string, error)
		TrustSummary(ctx context.Context, reviews []string) (string, error)
	}
)

type Deps struct {
	Jobs    Jobs
	Reviews Reviews
	Workers Workers
	Media   Media
	AI      AI
}

// Register mounts one handler per task type in internal/platform/queue.
// A returned error makes asynq retry (3 times, with backoff); SkipRetry marks a task that can never succeed.
func Register(mux *asynq.ServeMux, d Deps) {
	mux.HandleFunc(queue.TypeTranscribe, withPayload(func(ctx context.Context, p queue.MessagePayload) error {
		return d.transcribe(ctx, p.MessageID)
	}))
	mux.HandleFunc(queue.TypeMessageSpeech, withPayload(func(ctx context.Context, p queue.MessagePayload) error {
		return d.messageSpeech(ctx, p.MessageID)
	}))
	mux.HandleFunc(queue.TypeJobBrief, withPayload(func(ctx context.Context, p queue.JobPayload) error {
		return d.jobBrief(ctx, p.JobID)
	}))
	mux.HandleFunc(queue.TypeReviewAnalyze, withPayload(func(ctx context.Context, p queue.JobPayload) error {
		return d.reviewAnalyze(ctx, p.JobID)
	}))
}

// withPayload decodes the task payload; a bad payload or a deleted row is never retried.
func withPayload[P any](fn func(context.Context, P) error) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p P
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%s payload: %v: %w", t.Type(), err, asynq.SkipRetry)
		}
		err := fn(ctx, p)
		if errors.Is(err, ai.ErrBudget) {
			log.Printf("task %s %s dropped: %v", t.Type(), t.Payload(), err)
			return fmt.Errorf("%s: %w: %w", t.Type(), err, asynq.SkipRetry)
		}
		if errors.Is(err, platform.ErrNotFound) {
			return fmt.Errorf("%s: %v: %w", t.Type(), err, asynq.SkipRetry)
		}
		if err != nil {
			log.Printf("task %s %s: %v", t.Type(), t.Payload(), err)
		}
		return err
	}
}

// transcribe: voice note → transcript; if Jev hears a price, the LLM extracts it as a one-tap offer suggestion.
func (d Deps) transcribe(ctx context.Context, messageID int64) error {
	msg, err := d.Jobs.MessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.Kind != "voice" || msg.AudioURL == nil {
		return fmt.Errorf("message %d is not a voice note: %w", messageID, asynq.SkipRetry)
	}
	path, err := d.Media.Path(*msg.AudioURL)
	if err != nil {
		return fmt.Errorf("message %d audio: %v: %w", messageID, err, asynq.SkipRetry)
	}
	text, err := d.AI.Transcribe(ctx, path)
	if err != nil {
		return err
	}
	var price *int
	if text != "" {
		// A failed price step must not hold back the transcript, so its errors are only logged.
		if has, err := d.AI.HasPrice(ctx, text); err != nil {
			log.Printf("message %d price check: %v", messageID, err)
		} else if has {
			if price, err = d.AI.Price(ctx, text); err != nil {
				log.Printf("message %d price extraction: %v", messageID, err)
			}
		}
	}
	return d.Jobs.SetTranscript(ctx, messageID, text, price)
}

// messageSpeech: a customer's typed message (Urdu, Roman Urdu or English) → Urdu script + its audio for the worker.
func (d Deps) messageSpeech(ctx context.Context, messageID int64) error {
	msg, err := d.Jobs.MessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.Kind != "text" || msg.Body == nil || strings.TrimSpace(*msg.Body) == "" {
		return fmt.Errorf("message %d has no text: %w", messageID, asynq.SkipRetry)
	}
	ur, err := d.AI.UrduMessage(ctx, *msg.Body)
	if err != nil {
		return err
	}
	url, err := d.speech(ctx, ur)
	if err != nil {
		// Show the Urdu text now; the retry adds the audio (the text comes from the AI cache).
		if e := d.Jobs.SetMessageSpeech(ctx, messageID, ur, ""); e != nil {
			return e
		}
		return err
	}
	return d.Jobs.SetMessageSpeech(ctx, messageID, ur, url)
}

// jobBrief: a new job → short Urdu alert + its audio for the worker.
func (d Deps) jobBrief(ctx context.Context, jobID int64) error {
	row, err := d.Jobs.JobRow(ctx, jobID)
	if err != nil {
		return err
	}
	in := ai.BriefInput{Trade: row.Trade, Urgency: row.Urgency, Description: row.Description}
	if card, err := d.Workers.Card(ctx, row.WorkerID, &workers.LatLng{Lat: row.Lat, Lng: row.Lng}); err == nil && card != nil {
		in.DistanceKm = card.DistanceKm
	}
	ur, err := d.AI.JobBrief(ctx, in)
	if err != nil {
		return err
	}
	url, err := d.speech(ctx, ur)
	if err != nil {
		if e := d.Jobs.SetBrief(ctx, jobID, ur, ""); e != nil {
			return e
		}
		return err
	}
	return d.Jobs.SetBrief(ctx, jobID, ur, url)
}

// reviewAnalyze: review text and/or voice → Jev tags, then a fresh trust summary from the worker's reviews.
func (d Deps) reviewAnalyze(ctx context.Context, jobID int64) error {
	rv, err := d.Reviews.ByJob(ctx, jobID)
	if err != nil {
		return err
	}
	text := ""
	if rv.Body != nil {
		text = strings.TrimSpace(*rv.Body)
	}
	if rv.AudioURL != nil {
		path, err := d.Media.Path(*rv.AudioURL)
		if err != nil {
			return fmt.Errorf("review %d audio: %v: %w", jobID, err, asynq.SkipRetry)
		}
		spoken, err := d.AI.Transcribe(ctx, path)
		if err != nil {
			return err
		}
		text = strings.TrimSpace(text + "\n" + spoken)
	}
	if text != "" { // stars only: nothing to tag
		tags, err := d.AI.ReviewTags(ctx, text)
		if err != nil {
			return err
		}
		if err := d.Reviews.SetTags(ctx, jobID, tags); err != nil {
			return err
		}
	}

	row, err := d.Jobs.JobRow(ctx, jobID)
	if err != nil {
		return err
	}
	all, err := d.Reviews.ForWorker(ctx, row.WorkerID) // abusive reviews are already excluded
	if err != nil {
		return err
	}
	lines := make([]string, 0, len(all))
	for _, r := range all {
		t := ""
		if r.Body != nil {
			t = *r.Body
		}
		if r.JobID == jobID {
			t = text // includes the voice transcript, which is not stored
		}
		lines = append(lines, fmt.Sprintf("%d/5: %s", r.Rating, strings.ReplaceAll(t, "\n", " ")))
	}
	summary := ""
	if len(lines) > 0 {
		if summary, err = d.AI.TrustSummary(ctx, lines); err != nil {
			return err
		}
	}
	return d.Workers.SetTrustSummary(ctx, row.WorkerID, summary)
}

// speech generates audio for Urdu text and stores it under /media/audio.
func (d Deps) speech(ctx context.Context, text string) (string, error) {
	audio, ext, err := d.AI.Speak(ctx, text)
	if err != nil {
		return "", err
	}
	return d.Media.SaveBytes("audio", ext, audio)
}
