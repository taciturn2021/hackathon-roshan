package platform

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"regexp"
	"strings"
	"testing"
)

func uploadReq(t *testing.T, field, filename, ctype string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if field != "" {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+filename+`"`)
		h.Set("Content-Type", ctype)
		p, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		p.Write(content)
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if err := ParseMultipart(httptest.NewRecorder(), r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMediaSave(t *testing.T) {
	tests := []struct {
		name, filename, ctype, kind, wantExt string
		wantErr                              bool
	}{
		{"ext from filename", "voice.OGG", "application/octet-stream", "audio", ".ogg", false},
		{"ext from content-type", "blob", "audio/webm;codecs=opus", "audio", ".webm", false},
		{"photo from content-type", "blob", "image/jpeg", "photo", ".jpg", false},
		{"unsupported", "x.exe", "application/octet-stream", "audio", "", true},
		{"wrong kind ext", "x.png", "application/octet-stream", "audio", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Media{Dir: t.TempDir()}
			content := []byte("hello-bytes")
			r := uploadReq(t, "file", tt.filename, tt.ctype, content)
			url, err := m.Save(r, "file", tt.kind)
			if tt.wantErr {
				if e, ok := err.(*Error); !ok || e.Status != 400 {
					t.Fatalf("want 400, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			re := regexp.MustCompile(`^/media/` + tt.kind + `/[0-9a-f]{32}` + regexp.QuoteMeta(tt.wantExt) + `$`)
			if !re.MatchString(url) {
				t.Fatalf("url %q", url)
			}
			p, err := m.Path(url)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(p)
			if err != nil || !bytes.Equal(got, content) {
				t.Fatalf("file content %q, %v", got, err)
			}
		})
	}
}

func TestMediaSaveMissing(t *testing.T) {
	m := Media{Dir: t.TempDir()}
	r := uploadReq(t, "", "", "", nil)
	url, err := m.Save(r, "file", "audio")
	if url != "" || err != nil {
		t.Fatalf("got %q, %v", url, err)
	}
}

func TestMediaPathReject(t *testing.T) {
	m := Media{Dir: t.TempDir()}
	for _, u := range []string{"/media/../etc/passwd", "/other/x", "/media/audio/../../x", "media/x", "/media/"} {
		if _, err := m.Path(u); err == nil {
			t.Errorf("%q: want error", u)
		}
	}
}

func TestSaveBytes(t *testing.T) {
	m := Media{Dir: t.TempDir()}
	url, err := m.SaveBytes("audio", ".mp3", []byte("ID3"))
	if err != nil || !strings.HasPrefix(url, "/media/audio/") || !strings.HasSuffix(url, ".mp3") {
		t.Fatalf("SaveBytes = %q, %v", url, err)
	}
	p, err := m.Path(url)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "ID3" {
		t.Fatalf("file = %q, %v", b, err)
	}
	if _, err := m.SaveBytes("audio", ".exe", nil); err == nil {
		t.Fatal(".exe accepted")
	}
	if _, err := m.SaveBytes("video", ".mp4", nil); err == nil {
		t.Fatal("unknown kind accepted")
	}
}
