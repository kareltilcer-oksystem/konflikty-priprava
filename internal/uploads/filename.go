package uploads

import (
	"fmt"
	"strings"
)

// MaxFilenameLen bounds the metadata copy of the submitted file name.
const MaxFilenameLen = 255

// FallbackFilename is used when sanitising leaves nothing usable.
const FallbackFilename = "soubor"

// SanitizeFilename reduces a submitted file name to safe metadata.
//
// The result never reaches the filesystem — the stored file is named by a
// generated UUID — but it is echoed back in Content-Disposition, so the
// directory part and any control characters have to go: a raw CR or LF would
// let the value inject a header.
func SanitizeFilename(name string) string {
	// Take the last segment under both separators: a Windows client may send a
	// full path, and "\" is not a separator on Linux.
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	name = strings.TrimSpace(b.String())

	// "." and ".." survive the steps above but are not names.
	if name == "" || name == "." || name == ".." {
		return FallbackFilename
	}
	if len(name) > MaxFilenameLen {
		name = truncateUTF8(name, MaxFilenameLen)
	}
	if name == "" {
		return FallbackFilename
	}
	return name
}

// truncateUTF8 cuts a string to at most n bytes without splitting a rune.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8Start(s[n]) {
		n--
	}
	return s[:n]
}

// utf8Start reports whether b begins a UTF-8 sequence.
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// isAttrChar reports whether b may appear unescaped in an RFC 5987 value.
//
//	attr-char = ALPHA / DIGIT / "!" / "#" / "$" / "&" / "+" / "-" / "." /
//	            "^" / "_" / "`" / "|" / "~"
func isAttrChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", b) >= 0
}

// encodeRFC5987 percent-encodes a value for the filename* parameter.
//
// Neither url.PathEscape nor url.QueryEscape can be used here: PathEscape
// leaves "$ & + , ; = : @" unescaped and QueryEscape turns spaces into "+",
// and both produce values that violate attr-char.
func encodeRFC5987(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; isAttrChar(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// asciiFallback reduces a name to the plain ASCII that goes in the legacy
// filename parameter, dropping the two characters that could end the quoted
// string early.
func asciiFallback(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || c > 0x7e || c == '"' || c == '\\' {
			continue
		}
		b.WriteByte(c)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return FallbackFilename
	}
	return out
}

// ContentDisposition builds the header for an attachment download.
//
// Both forms are emitted: the quoted ASCII filename for anything old, and the
// RFC 5987 filename* that carries the real, accented name.
func ContentDisposition(inline bool, filename string) string {
	kind := "attachment"
	if inline {
		kind = "inline"
	}
	name := SanitizeFilename(filename)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`,
		kind, asciiFallback(name), encodeRFC5987(name))
}
