package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeGateway answers like AI Gateway. reply decides the answer per model; it records what was sent.
type fakeGateway struct {
	mu    sync.Mutex
	calls []map[string]any // request bodies, with "_model" added
	reply func(path, model string, body map[string]any) (int, any)
}

func (f *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	model := r.Header.Get("ai-model-id")
	if m, ok := body["model"].(string); ok {
		model = m
	}
	body["_model"], body["_path"] = model, r.URL.Path
	f.mu.Lock()
	f.calls = append(f.calls, body)
	f.mu.Unlock()
	code, out := f.reply(r.URL.Path, model, body)
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(out)
}

func newTest(t *testing.T, reply func(path, model string, body map[string]any) (int, any)) (*Service, *fakeGateway) {
	f := &fakeGateway{reply: reply}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return New(Config{Key: "k", BaseURL: srv.URL,
		STT: "microsoft/mai", STTFallback: "google/stt", LLM: "llm-main", LLMFallback: "llm-fb",
		Jev: "jev", JevFallback: "jev-fb", TTS: "tts-main", TTSFallback: "tts-fb"}), f
}

func chatReply(text string) map[string]any {
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": text}}}}
}

func audioFile(t *testing.T, ext string) string {
	p := filepath.Join(t.TempDir(), "clip"+ext)
	if err := os.WriteFile(p, []byte("fake audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTranscribeSendsUrduLocaleAndFallsBackOnWebm(t *testing.T) {
	s, f := newTest(t, func(path, model string, body map[string]any) (int, any) {
		if model == "microsoft/mai" && body["mediaType"] == "audio/webm" {
			return 400, map[string]any{"error": "Bad Request"} // MAI rejects webm
		}
		return 200, map[string]any{"text": " میرا نام اکرم ہے " + model}
	})
	ctx := context.Background()

	got, err := s.Transcribe(ctx, audioFile(t, ".wav"))
	if err != nil || got != "میرا نام اکرم ہے microsoft/mai" {
		t.Fatalf("wav: %q %v", got, err)
	}
	if opts, _ := json.Marshal(f.calls[0]["providerOptions"]); !strings.Contains(string(opts), "ur-PK") {
		t.Errorf("MAI call must carry the ur-PK locale, got %s", opts)
	}

	got, err = s.Transcribe(ctx, audioFile(t, ".webm"))
	if err != nil || !strings.HasSuffix(got, "google/stt") {
		t.Fatalf("webm should fall back: %q %v", got, err)
	}
	if f.calls[2]["providerOptions"] != nil {
		t.Error("non-Microsoft models get no Azure options")
	}
}

func TestTranscribeRewritesHindiScriptToUrdu(t *testing.T) {
	s, _ := newTest(t, func(path, model string, body map[string]any) (int, any) {
		if strings.Contains(path, "transcription") {
			return 200, map[string]any{"text": "आपका नाम क्या है?"}
		}
		return 200, chatReply("آپ کا نام کیا ہے؟")
	})
	got, err := s.Transcribe(context.Background(), audioFile(t, ".wav"))
	if err != nil || got != "آپ کا نام کیا ہے؟" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestTranscribeFailsOnlyWhenBothModelsFail(t *testing.T) {
	s, _ := newTest(t, func(string, string, map[string]any) (int, any) { return 500, map[string]any{} })
	if _, err := s.Transcribe(context.Background(), audioFile(t, ".wav")); err == nil {
		t.Fatal("want error when both models fail")
	}
}

func TestUrduMessageRejectsNonUrduAndUsesFallback(t *testing.T) {
	s, f := newTest(t, func(path, model string, body map[string]any) (int, any) {
		if model == "llm-main" {
			return 200, chatReply("AC theek karna hai") // Latin: invalid
		}
		return 200, chatReply("اے سی ٹھیک کرنا ہے۔")
	})
	got, err := s.UrduMessage(context.Background(), "bhai AC theek karna hai")
	if err != nil || got != "اے سی ٹھیک کرنا ہے۔" {
		t.Fatalf("got %q %v", got, err)
	}
	if r, _ := json.Marshal(f.calls[0]["reasoning"]); string(r) != `{"enabled":false}` {
		t.Errorf("thinking must be off, got %s", r)
	}
}

func TestUrduMessageKeepsSectorCodes(t *testing.T) {
	if !isUrdu("کام G-10 میں ہے، 2 کلومیٹر دور۔") {
		t.Error("a Latin sector code in Urdu text is valid output")
	}
	if isUrdu("नया काम आया है") {
		t.Error("Devanagari is not Urdu")
	}
}

func TestParsePrice(t *testing.T) {
	cases := map[string]any{ // nil = no price but valid; -1 = invalid
		`{"price": 1500}`:                 1500,
		"```json\n{\"price\": 2000}\n```": 2000,
		`{"price": null}`:                 nil,
		`{"price": -5}`:                   -1,
		`{"price": 50000000}`:             -1,
		`about 1500 rupees`:               -1,
	}
	for in, want := range cases {
		p, ok := parsePrice(in)
		switch want {
		case -1:
			if ok {
				t.Errorf("%q should be invalid", in)
			}
		case nil:
			if !ok || p != nil {
				t.Errorf("%q: want valid null, got %v %v", in, p, ok)
			}
		default:
			if !ok || p == nil || *p != want.(int) {
				t.Errorf("%q: want %v, got %v %v", in, want, p, ok)
			}
		}
	}
}

func TestHasPriceAndReviewTagsUseThreshold(t *testing.T) {
	s, _ := newTest(t, func(path, model string, body map[string]any) (int, any) {
		return 200, map[string]any{"answers": []any{
			map[string]any{"name": "has_price", "probability": 0.98},
			map[string]any{"name": "on_time", "probability": 0.95},
			map[string]any{"name": "fair_price", "probability": 0.59},
			map[string]any{"name": "abusive_or_spam", "probability": 0.09},
			map[string]any{"type": "refusal", "name": "polite"},
		}}
	})
	ctx := context.Background()
	if has, err := s.HasPrice(ctx, "پندرہ سو روپے"); err != nil || !has {
		t.Errorf("HasPrice = %v %v", has, err)
	}
	tags, err := s.ReviewTags(ctx, "وقت پر آیا")
	if err != nil || strings.Join(tags, ",") != "on_time" {
		t.Errorf("tags = %v %v (0.59 and a refusal must not count)", tags, err)
	}
}

func TestSpeakDetectsFormatAndFallsBack(t *testing.T) {
	wav := base64.StdEncoding.EncodeToString([]byte("RIFF....WAVEfmt "))
	s, f := newTest(t, func(path, model string, body map[string]any) (int, any) {
		if model == "tts-main" {
			return 500, map[string]any{}
		}
		return 200, map[string]any{"audio": wav}
	})
	audio, ext, err := s.Speak(context.Background(), "نیا کام آیا ہے")
	if err != nil || ext != ".wav" || len(audio) == 0 {
		t.Fatalf("got %d bytes %q %v", len(audio), ext, err)
	}
	if f.calls[0]["language"] != "ur" {
		t.Errorf("TTS must be asked for Urdu, got %v", f.calls[0]["language"])
	}
}
