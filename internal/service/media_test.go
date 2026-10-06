package service

import "testing"

func TestHasPreview(t *testing.T) {
	for mime, want := range map[string]bool{
		"video/mp4":       true,
		"image/jpeg":      true,
		"audio/mpeg":      false,
		"application/pdf": false,
		"":                false,
	} {
		if got := hasPreview(mime); got != want {
			t.Errorf("hasPreview(%q) = %v, want %v", mime, got, want)
		}
	}
}
