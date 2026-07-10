package whatsmeow

import (
	"testing"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
)

// A PtvMessage (round video note) shares the VideoMessage proto shape and must
// be treated as media, otherwise newsletter sends fall through to type=text with
// no mediatype attr and the server rejects the upload.
func TestGetMediaTypeFromMessagePtv(t *testing.T) {
	msg := &waE2E.Message{PtvMessage: &waE2E.VideoMessage{}}
	if got := getMediaTypeFromMessage(msg); got != "video" {
		t.Fatalf("getMediaTypeFromMessage(PtvMessage) = %q, want %q", got, "video")
	}
	if got := getTypeFromMessage(msg); got != "media" {
		t.Fatalf("getTypeFromMessage(PtvMessage) = %q, want %q", got, "media")
	}
}
