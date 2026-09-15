package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// magic returns a reader whose leading bytes sniff as the given type — the
// minimum a real file of that kind would start with.
func magic(contentType string) io.Reader {
	var b []byte
	switch contentType {
	case "application/pdf":
		b = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	case "image/png":
		b = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	case "image/jpeg":
		b = []byte("\xff\xd8\xff\xe0\x00\x10JFIF")
	case "image/gif":
		b = []byte("GIF89a\x01\x00\x01\x00")
	case "image/webp":
		b = []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	case "audio/mpeg":
		b = []byte("ID3\x03\x00\x00\x00\x00\x00\x00")
	case "audio/wav", "audio/x-wav":
		b = []byte("RIFF\x00\x00\x00\x00WAVEfmt ")
	case "audio/ogg":
		b = []byte("OggS\x00\x02\x00\x00\x00\x00")
	case "video/mp4", "audio/mp4":
		b = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")
	case "video/webm", "audio/webm":
		b = []byte("\x1a\x45\xdf\xa3\x01\x00\x00\x00")
	case "video/quicktime":
		// Go's sniffer has no QuickTime signature; "qt  " isn't an mp4
		// brand, so this lands on application/octet-stream — accepted.
		b = []byte("\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00")
	case "audio/aac":
		b = []byte("\xff\xf1\x50\x80\x00\x1f\xfc")
	default:
		b = []byte("\x00\x01\x02\x03binary")
	}
	return bytes.NewReader(append(b, bytes.Repeat([]byte{0}, 32)...))
}

func TestSniff_EveryAllowedTypeAcceptsItsOwnMagic(t *testing.T) {
	for ct := range allowedContentTypes {
		if _, err := sniffMatches(ct, magic(ct)); err != nil {
			t.Errorf("%s: real %s bytes rejected: %v", ct, ct, err)
		}
	}
}

func TestSniff_RejectsActiveContentUnderAnyLabel(t *testing.T) {
	payloads := map[string]string{
		"html": "<!DOCTYPE html><html><script>alert(1)</script></html>",
		"svg":  `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>1</script></svg>`,
		"text": "just some plain text that is not a file",
	}
	for ct := range allowedContentTypes {
		for name, body := range payloads {
			if _, err := sniffMatches(ct, strings.NewReader(body)); !errors.Is(err, ErrUnsupportedType) {
				t.Errorf("%s labelled as %s: err=%v, want ErrUnsupportedType", name, ct, err)
			}
		}
	}
}

func TestSniff_RejectsCrossTypeMismatch(t *testing.T) {
	// A real PNG uploaded as a PDF (and vice versa) is a lie either way.
	if _, err := sniffMatches("application/pdf", magic("image/png")); !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("png as pdf: %v", err)
	}
	if _, err := sniffMatches("image/png", magic("application/pdf")); !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("pdf as png: %v", err)
	}
}

func TestSniff_ReplaysHeadIntact(t *testing.T) {
	src := magic("application/pdf")
	want, _ := io.ReadAll(src)
	r, err := sniffMatches("application/pdf", bytes.NewReader(want))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, want) {
		t.Errorf("bytes after sniffing differ from input")
	}
}

func TestSniff_ShortFileStillWorks(t *testing.T) {
	// Fewer than 512 bytes must not be an error — most PDFs in tests are.
	if _, err := sniffMatches("application/pdf", strings.NewReader("%PDF-1.7")); err != nil {
		t.Errorf("short pdf: %v", err)
	}
}

func TestService_Upload_RejectsMislabelledHTML(t *testing.T) {
	svc := NewService(&fakeRepo{}, &memBlob{}, discardLogger())
	html := "<html><body><script>document.cookie</script></body></html>"
	_, err := svc.Upload(context.Background(), uuid.New(), "innocent.png", "image/png", int64(len(html)), strings.NewReader(html))
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("html as png: err=%v, want ErrUnsupportedType", err)
	}
}
