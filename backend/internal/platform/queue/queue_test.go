package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/testdb"
)

func TestEnqueue(t *testing.T) {
	rdb := testdb.Redis(t)
	o := rdb.Options()
	opt := asynq.RedisClientOpt{Addr: o.Addr, DB: o.DB}
	q := NewClient(opt)
	defer q.Close()
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	ctx := context.Background()

	if err := q.Transcribe(ctx, 11); err != nil {
		t.Fatal(err)
	}
	if err := q.JobBrief(ctx, 22); err != nil {
		t.Fatal(err)
	}
	if err := q.ReviewAnalyze(ctx, 33); err != nil {
		t.Fatal(err)
	}
	// A duplicate for the same object is accepted silently and not queued twice.
	if err := q.Transcribe(ctx, 11); err != nil {
		t.Fatalf("duplicate enqueue: %v", err)
	}

	tasks, err := insp.ListPendingTasks("default")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, ti := range tasks {
		got[ti.Type] = string(ti.Payload)
		if ti.MaxRetry != 3 {
			t.Errorf("%s MaxRetry = %d, want 3", ti.Type, ti.MaxRetry)
		}
	}
	want := map[string]string{
		TypeTranscribe:    `{"message_id":11}`,
		TypeJobBrief:      `{"job_id":22}`,
		TypeReviewAnalyze: `{"job_id":33}`,
	}
	if len(tasks) != 3 {
		t.Fatalf("pending tasks = %d (%v), want 3", len(tasks), got)
	}
	for typ, payload := range want {
		if got[typ] != payload {
			t.Errorf("%s payload = %s, want %s", typ, got[typ], payload)
		}
	}
	var mp MessagePayload
	if err := json.Unmarshal([]byte(got[TypeTranscribe]), &mp); err != nil || mp.MessageID != 11 {
		t.Errorf("payload does not decode into MessagePayload: %v %+v", err, mp)
	}
}
