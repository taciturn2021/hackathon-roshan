package platform

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ponytail: media on a local volume; S3-compatible storage when the API runs on more than one host.
type Media struct{ Dir string }

const maxUpload = 20 << 20

var allowedExt = map[string]map[string]bool{
	"audio": {".webm": true, ".ogg": true, ".mp3": true, ".m4a": true, ".mp4": true, ".wav": true},
	"photo": {".jpg": true, ".jpeg": true, ".png": true, ".webp": true},
}

var mimeExt = map[string]string{
	"audio/webm":  ".webm",
	"audio/ogg":   ".ogg",
	"audio/mpeg":  ".mp3",
	"audio/mp4":   ".m4a",
	"audio/wav":   ".wav",
	"audio/x-wav": ".wav",
	"image/jpeg":  ".jpg",
	"image/png":   ".png",
	"image/webp":  ".webp",
}

// ParseMultipart limits the body to maxUpload and parses the form.
func ParseMultipart(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return BadRequest("invalid upload")
	}
	return nil
}

// Save stores form file `field` under Dir/<kind>/ and returns its public URL.
// A missing field returns ("", nil).
func (m Media) Save(r *http.Request, field, kind string) (string, error) {
	allow, ok := allowedExt[kind]
	if !ok {
		panic("platform: media kind must be audio or photo, got " + kind)
	}
	f, hdr, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", BadRequest("invalid upload")
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if !allow[ext] {
		ct := strings.ToLower(hdr.Header.Get("Content-Type"))
		if i := strings.Index(ct, ";"); i >= 0 {
			ct = ct[:i]
		}
		ext = mimeExt[strings.TrimSpace(ct)]
		if !allow[ext] {
			return "", BadRequest("unsupported file type")
		}
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	name := hex.EncodeToString(buf) + ext
	dir := filepath.Join(m.Dir, kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	out, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, f); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return "/media/" + kind + "/" + name, nil
}

// SaveBytes stores generated media (e.g. text-to-speech audio from the queue) and returns its public URL.
// ext must be in the allowlist for kind (".mp3", ".wav", ... for audio).
func (m Media) SaveBytes(kind, ext string, data []byte) (string, error) {
	allow, ok := allowedExt[kind]
	if !ok || !allow[ext] {
		return "", fmt.Errorf("media: %q not allowed for %s", ext, kind)
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	name := hex.EncodeToString(buf) + ext
	dir := filepath.Join(m.Dir, kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", err
	}
	return "/media/" + kind + "/" + name, nil
}

// Path maps a public URL from Save back to the file path on disk.
func (m Media) Path(url string) (string, error) {
	if !strings.HasPrefix(url, "/media/") || strings.Contains(url, "..") {
		return "", BadRequest("invalid media url")
	}
	root := filepath.Clean(m.Dir)
	p := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(url, "/media/")))
	if !strings.HasPrefix(p, root+string(filepath.Separator)) {
		return "", BadRequest("invalid media url")
	}
	return p, nil
}
