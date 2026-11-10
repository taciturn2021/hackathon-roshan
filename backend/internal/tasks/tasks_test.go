package tasks

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/ai"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/jobs"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/platform/queue"
	"github.com/taciturn2021/hackathon-roshan/backend/internal/workers"
)

func ptr[T any](v T) *T { return &v }

// fake implements every dependency and records what the handlers wrote back.
type fake struct {
	msg      jobs.Message
	row      jobs.Row
	review   workers.Review
	reviews  []workers.Review
	hasPrice bool
	priceErr error
	speakErr error
	gotBrief ai.BriefInput

	transcript *string
	price      *int
	speech     [][2]string // (bodyUr, url) per SetMessageSpeech / SetBrief call
	tags       []string
	tagsSet    bool
	summary    *string
	lines      []string
}

func (f *fake) MessageByID(context.Context, int64) (jobs.Message, error) {
	if f.msg.ID == 0 {
		return jobs.Message{}, platform.ErrNotFound
	}
	return f.msg, nil
}
func (f *fake) JobRow(context.Context, int64) (jobs.Row, error) { return f.row, nil }
func (f *fake) SetTranscript(_ context.Context, _ int64, body string, p *int) error {
	f.transcript, f.price = &body, p
	return nil
}
func (f *fake) SetMessageSpeech(_ context.Context, _ int64, ur, url string) error {
	f.speech = append(f.speech, [2]string{ur, url})
	return nil
}
func (f *fake) SetBrief(_ context.Context, _ int64, ur, url string) error {
	f.speech = append(f.speech, [2]string{ur, url})
	return nil
}
func (f *fake) ByJob(context.Context, int64) (workers.Review, error)       { return f.review, nil }
func (f *fake) ForWorker(context.Context, int64) ([]workers.Review, error) { return f.reviews, nil }
func (f *fake) SetTags(_ context.Context, _ int64, tags []string) error {
	f.tags, f.tagsSet = tags, true
	return nil
}
func (f *fake) SetTrustSummary(_ context.Context, _ int64, s string) error {
	f.summary = &s
	return nil
}
func (f *fake) Path(url string) (string, error) { return "/disk" + url, nil }
func (f *fake) SaveBytes(kind, ext string, _ []byte) (string, error) {
	return "/media/" + kind + "/x" + ext, nil
}
func (f *fake) Card(context.Context, int64, *workers.LatLng) (*workers.WorkerCard, error) {
	return &workers.WorkerCard{DistanceKm: ptr(1.2)}, nil
}
func (f *fake) Transcribe(_ context.Context, path string) (string, error) {
	return "spoken:" + path, nil
}
func (f *fake) HasPrice(context.Context, string) (bool, error)          { return f.hasPrice, f.priceErr }
func (f *fake) Price(context.Context, string) (*int, error)             { return ptr(1500), nil }
func (f *fake) UrduMessage(_ context.Context, t string) (string, error) { return "اردو:" + t, nil }
func (f *fake) JobBrief(_ context.Context, in ai.BriefInput) (string, error) {
	f.gotBrief = in
	return "نیا کام", nil
}
func (f *fake) Speak(context.Context, string) ([]byte, string, error) {
	return []byte("mp3"), ".mp3", f.speakErr
}
func (f *fake) ReviewTags(_ context.Context, text string) ([]string, error) {
	return []string{"on_time"}, nil
}
func (f *fake) TrustSummary(_ context.Context, lines []string) (string, error) {
	f.lines = lines
	return "Punctual.", nil
}

func deps(f *fake) Deps { return Deps{Jobs: f, Reviews: f, Workers: f, Media: f, AI: f} }

func TestTranscribeWithPrice(t *testing.T) {
	f := &fake{msg: jobs.Message{ID: 7, Kind: "voice", AudioURL: ptr("/media/audio/a.wav")}, hasPrice: true}
	if err := deps(f).transcribe(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if *f.transcript != "spoken:/disk/media/audio/a.wav" || f.price == nil || *f.price != 1500 {
		t.Fatalf("transcript %q price %v", *f.transcript, f.price)
	}
}

func TestTranscribeKeepsTranscriptWhenPriceStepFails(t *testing.T) {
	f := &fake{msg: jobs.Message{ID: 7, Kind: "voice", AudioURL: ptr("/media/audio/a.wav")}, priceErr: errors.New("jev down")}
	if err := deps(f).transcribe(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if f.transcript == nil || f.price != nil {
		t.Fatalf("transcript must still be saved without a price: %v %v", f.transcript, f.price)
	}
}

func TestTranscribeSkipsNonVoice(t *testing.T) {
	f := &fake{msg: jobs.Message{ID: 7, Kind: "text", Body: ptr("hi")}}
	if err := deps(f).transcribe(context.Background(), 7); !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("want SkipRetry, got %v", err)
	}
}

func TestMessageSpeech(t *testing.T) {
	f := &fake{msg: jobs.Message{ID: 3, Kind: "text", Body: ptr("bhai kitne paise?")}}
	if err := deps(f).messageSpeech(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if len(f.speech) != 1 || f.speech[0] != [2]string{"اردو:bhai kitne paise?", "/media/audio/x.mp3"} {
		t.Fatalf("got %v", f.speech)
	}
}

func TestMessageSpeechSavesTextWhenTTSFails(t *testing.T) {
	f := &fake{msg: jobs.Message{ID: 3, Kind: "text", Body: ptr("hello")}, speakErr: errors.New("tts down")}
	if err := deps(f).messageSpeech(context.Background(), 3); err == nil {
		t.Fatal("want an error so asynq retries the audio")
	}
	if len(f.speech) != 1 || f.speech[0][1] != "" || f.speech[0][0] == "" {
		t.Fatalf("Urdu text must be saved without audio, got %v", f.speech)
	}
}

func TestJobBrief(t *testing.T) {
	f := &fake{row: jobs.Row{ID: 9, Trade: "plumber", Urgency: "now", Description: "tank leak", WorkerID: 4}}
	if err := deps(f).jobBrief(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if f.gotBrief.Trade != "plumber" || f.gotBrief.DistanceKm == nil || *f.gotBrief.DistanceKm != 1.2 {
		t.Fatalf("brief input %+v", f.gotBrief)
	}
	if f.speech[0] != [2]string{"نیا کام", "/media/audio/x.mp3"} {
		t.Fatalf("got %v", f.speech)
	}
}

func TestReviewAnalyzeUsesTextAndVoice(t *testing.T) {
	f := &fake{
		review:  workers.Review{JobID: 9, Rating: 5, Body: ptr("acha kaam"), AudioURL: ptr("/media/audio/r.wav")},
		row:     jobs.Row{ID: 9, WorkerID: 4},
		reviews: []workers.Review{{JobID: 9, Rating: 5, Body: ptr("acha kaam")}, {JobID: 2, Rating: 3}},
	}
	if err := deps(f).reviewAnalyze(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.tags, ",") != "on_time" || *f.summary != "Punctual." {
		t.Fatalf("tags %v summary %v", f.tags, f.summary)
	}
	if f.lines[0] != "5/5: acha kaam spoken:/disk/media/audio/r.wav" || f.lines[1] != "3/5: " {
		t.Fatalf("summary input %q", f.lines)
	}
}

func TestReviewAnalyzeStarsOnly(t *testing.T) {
	f := &fake{review: workers.Review{JobID: 9, Rating: 4}, row: jobs.Row{WorkerID: 4}, reviews: []workers.Review{{JobID: 9, Rating: 4}}}
	if err := deps(f).reviewAnalyze(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if f.tagsSet || f.summary == nil {
		t.Fatalf("no tags for stars only, but the summary still refreshes: tagsSet=%v summary=%v", f.tagsSet, f.summary)
	}
}

func TestHandlersNeverRetryBadPayloadOrMissingRow(t *testing.T) {
	mux := asynq.NewServeMux()
	Register(mux, deps(&fake{}))
	ctx := context.Background()
	if err := mux.ProcessTask(ctx, asynq.NewTask(queue.TypeTranscribe, []byte("{bad"))); !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("bad payload: want SkipRetry, got %v", err)
	}
	if err := mux.ProcessTask(ctx, asynq.NewTask(queue.TypeMessageSpeech, []byte(`{"message_id":5}`))); !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("missing message: want SkipRetry, got %v", err)
	}
}
