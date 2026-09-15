package files

import (
	"bytes"
	"io"
	"net/http"
)

// sniffLen is what http.DetectContentType looks at.
const sniffLen = 512

// sniffCompatible says, for each declared (client-supplied) content type,
// which types the leading bytes may sniff as. The declared type is what gets
// stored and served, so the point is only to refuse a body that is provably
// something else — an HTML page uploaded as image/png, say. Types the sniffer
// cannot recognise (AAC, M4A, QuickTime, an ID3-less MP3 frame) come back as
// application/octet-stream, which is accepted for the container formats it
// genuinely can't tell apart; a text/* or image/svg+xml result is never in a
// set, so active content is always rejected.
var sniffCompatible = map[string][]string{
	"application/pdf": {"application/pdf"},
	"image/png":       {"image/png"},
	"image/jpeg":      {"image/jpeg"},
	"image/webp":      {"image/webp"},
	"image/gif":       {"image/gif"},
	"audio/mpeg":      {"audio/mpeg", "application/octet-stream"},
	"audio/mp4":       {"audio/mp4", "video/mp4", "application/octet-stream"},
	"audio/aac":       {"audio/aac", "application/octet-stream"},
	"audio/wav":       {"audio/wave"},
	"audio/x-wav":     {"audio/wave"},
	"audio/ogg":       {"application/ogg", "audio/ogg"},
	"audio/webm":      {"video/webm", "application/octet-stream"},
	"video/mp4":       {"video/mp4", "application/octet-stream"},
	"video/webm":      {"video/webm"},
	"video/quicktime": {"video/quicktime", "video/mp4", "application/octet-stream"},
}

// sniffMatches reads the head of r, checks it against the declared type, and
// returns a reader that replays the head followed by the rest. It returns
// ErrUnsupportedType when the bytes contradict the declaration.
func sniffMatches(declared string, r io.Reader) (io.Reader, error) {
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	head = head[:n]

	detected := normalizeContentType(http.DetectContentType(head))
	ok := false
	for _, want := range sniffCompatible[declared] {
		if detected == want {
			ok = true
			break
		}
	}
	if !ok {
		return nil, ErrUnsupportedType
	}
	return io.MultiReader(bytes.NewReader(head), r), nil
}
