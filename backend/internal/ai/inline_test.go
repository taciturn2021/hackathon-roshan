package ai

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taciturn2021/hackathon-roshan/backend/internal/problems"
)

// inlineGateway answers STT with stt, the LLM with llm, and Jev with probs.
func inlineGateway(stt, llm string, probs map[string]float64) func(path, model string, body map[string]any) (int, any) {
	return func(path, model string, body map[string]any) (int, any) {
		switch path {
		case "/v4/ai/transcription-model":
			return 200, map[string]any{"text": stt}
		case "/v1/chat/completions":
			return 200, chatReply(llm)
		case "/v1/decisions":
			var as []any
			for k, v := range probs {
				as = append(as, map[string]any{"name": k, "probability": v})
			}
			return 200, map[string]any{"answers": as}
		}
		return 404, nil
	}
}

func TestExtractTradeUsesJevConfidence(t *testing.T) {
	s, _ := newTest(t, inlineGateway("میں اکرم ہوں، پلمبر کا کام کرتا ہوں",
		"```json\n{\"name\":\"Akram\",\"trade\":\"plumber\",\"skills\":[\"leaks\"],\"area\":null,\"years_exp\":null}\n```",
		map[string]float64{"plumber": 0.93, "electrician": 0.1}))
	ex, err := s.Extract(context.Background(), audioFile(t, ".wav"), "trade")
	if err != nil {
		t.Fatal(err)
	}
	if ex.Profile.Trade == nil || *ex.Profile.Trade != "plumber" || ex.Confidence != 0.93 {
		t.Fatalf("trade = %v conf %v", ex.Profile.Trade, ex.Confidence)
	}
	if ex.Profile.Name == nil || *ex.Profile.Name != "Akram" || len(ex.Profile.Skills) != 1 || ex.Transcript == "" {
		t.Fatalf("other fields lost: %+v", ex)
	}
}

func TestExtractUnsureTradeKeepsGuessWithLowConfidence(t *testing.T) {
	s, _ := newTest(t, inlineGateway("میں کام کرتا ہوں", `{"trade":"carpenter"}`, map[string]float64{"carpenter": 0.4}))
	ex, _ := s.Extract(context.Background(), audioFile(t, ".wav"), "trade")
	if ex.Profile.Trade == nil || *ex.Profile.Trade != "carpenter" || ex.Confidence >= minConfidence {
		t.Fatalf("want carpenter below threshold, got %v %v", ex.Profile.Trade, ex.Confidence)
	}
}

func TestExtractOtherFieldConfidence(t *testing.T) {
	s, _ := newTest(t, inlineGateway("دس سال", `{"years_exp":10}`, nil))
	ex, _ := s.Extract(context.Background(), audioFile(t, ".wav"), "years_exp")
	if ex.Profile.YearsExp == nil || *ex.Profile.YearsExp != 10 || ex.Confidence != 0.9 || ex.Profile.Trade != nil {
		t.Fatalf("got %+v", ex)
	}
	ex, _ = s.Extract(context.Background(), audioFile(t, ".wav"), "name") // name not in the JSON
	if ex.Confidence != 0 {
		t.Fatalf("missing name should have confidence 0, got %v", ex.Confidence)
	}
}

func TestExtractSilenceSkipsTheModels(t *testing.T) {
	s, f := newTest(t, inlineGateway("", "{}", nil))
	ex, err := s.Extract(context.Background(), audioFile(t, ".wav"), "name")
	if err != nil || ex.Transcript != "" {
		t.Fatalf("%+v %v", ex, err)
	}
	for _, c := range f.calls {
		if c["_path"] != "/v4/ai/transcription-model" {
			t.Fatalf("unexpected call %v after empty transcript", c["_path"])
		}
	}
}

func TestClassifyTextAndPhoto(t *testing.T) {
	s, f := newTest(t, inlineGateway("",
		`{"trade":"plumber","urgency":"now","description":"Kitchen sink pipe leaking."}`,
		map[string]float64{"plumber": 0.88}))
	photo := filepath.Join(t.TempDir(), "p.jpg")
	os.WriteFile(photo, []byte("\xff\xd8\xff\xe0fakejpeg"), 0o644)
	c, err := s.Classify(context.Background(), problems.Input{Text: "sink se pani nikal raha hai, jaldi", PhotoPath: photo})
	if err != nil {
		t.Fatal(err)
	}
	if c.Trade == nil || *c.Trade != "plumber" || c.TradeConfidence != 0.88 || c.Urgency != "now" || c.Description != "Kitchen sink pipe leaking." {
		t.Fatalf("got %+v", c)
	}
	var sawImage bool
	for _, call := range f.calls {
		if call["_path"] == "/v1/chat/completions" && strings.Contains(toJSON(call["messages"]), "data:image/jpeg;base64,") {
			sawImage = true
		}
	}
	if !sawImage {
		t.Fatal("photo was not sent to the LLM")
	}
}

func TestClassifyFallsBackToLLMTradeWithLowConfidence(t *testing.T) {
	s, _ := newTest(t, func(path, model string, body map[string]any) (int, any) {
		if path == "/v1/decisions" {
			return 500, nil
		}
		return 200, chatReply(`{"trade":"electrician","urgency":"today","description":"Fan not working."}`)
	})
	c, err := s.Classify(context.Background(), problems.Input{Text: "pankha nahi chal raha"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Trade == nil || *c.Trade != "electrician" || c.TradeConfidence >= minConfidence || c.Urgency != "today" {
		t.Fatalf("got %+v", c)
	}
}

func TestClassifyNothingSent(t *testing.T) {
	s, f := newTest(t, inlineGateway("", "", nil))
	c, err := s.Classify(context.Background(), problems.Input{})
	if err != nil || c.Trade != nil || len(f.calls) != 0 {
		t.Fatalf("%+v %v calls=%d", c, err, len(f.calls))
	}
}

// TestLiveGateway exercises the real models end to end: TTS writes an Urdu answer, STT reads it back,
// then Extract and Classify run on it. Run with: AI_LIVE=1 AI_GATEWAY_API_KEY=... go test -run Live -v ./internal/ai
func TestLiveGateway(t *testing.T) {
	if os.Getenv("AI_LIVE") == "" || os.Getenv("AI_GATEWAY_API_KEY") == "" {
		t.Skip("set AI_LIVE=1 and AI_GATEWAY_API_KEY to call the real gateway")
	}
	s := FromEnv(nil)
	ctx := context.Background()
	audio, ext, err := s.Speak(ctx, "میرا نام محمد اکرم ہے، میں پلمبر ہوں اور جی ٹین میں دس سال سے کام کر رہا ہوں۔")
	if err != nil {
		t.Fatal("speak:", err)
	}
	p := filepath.Join(t.TempDir(), "answer"+ext)
	os.WriteFile(p, audio, 0o644)
	t.Logf("tts: %d bytes %s", len(audio), ext)

	ex, err := s.Extract(ctx, p, "trade")
	t.Logf("extract: transcript=%q profile=%s conf=%.2f err=%v", ex.Transcript, toJSON(ex.Profile), ex.Confidence, err)
	if ex.Profile.Trade == nil || *ex.Profile.Trade != "plumber" {
		t.Error("expected plumber")
	}

	c, err := s.Classify(ctx, problems.Input{Text: "AC bilkul thandi hawa nahi de raha, aaj hi koi aa jaye"})
	t.Logf("classify: %s err=%v", toJSON(c), err)
	if c.Trade == nil || *c.Trade != "ac_technician" {
		t.Error("expected ac_technician")
	}

	brief, err := s.JobBrief(ctx, BriefInput{Trade: "plumber", Urgency: "now", Description: "Kitchen sink leak ho raha hai, G-10/2"})
	t.Logf("brief: %q err=%v", brief, err)
	tags, err := s.ReviewTags(ctx, "Time pe aaya, kaam saaf kiya aur paise bhi theek liye")
	t.Logf("tags: %v err=%v", tags, err)
	has, err := s.HasPrice(ctx, "میں پندرہ سو روپے لوں گا")
	price, perr := s.Price(ctx, "میں پندرہ سو روپے لوں گا")
	t.Logf("price: has=%v %v value=%v %v", has, err, deref(price), perr)
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
