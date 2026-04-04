package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// NewConversationSlug builds a filesystem-safe, human-readable id from the first-line title.
func NewConversationSlug(title string) string {
	base := slugifyTitle(title)
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s", base, hex.EncodeToString(suffix))
}

func slugifyTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII:
			b.WriteRune(r)
			prevDash = false
		case unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if len(out) > 40 {
		out = out[:40]
		out = strings.TrimRight(out, "-")
	}
	if out == "" {
		out = "chat"
	}
	return out
}
