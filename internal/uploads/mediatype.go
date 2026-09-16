package uploads

import (
	"mime"
	"net/http"
	"strings"
)

// DefaultContentType is what an unidentifiable upload is recorded as.
const DefaultContentType = "application/octet-stream"

// extByType maps a resolved content type to the extension of its stored file.
//
// This table is hardcoded on purpose. mime.ExtensionsByType seeds itself from
// the Windows registry, which would make the chosen extension depend on
// whatever software happens to be installed on the machine running the binary.
// The submitted file name is never consulted: it is user-controlled, and a
// value like "x.a/../../../app.db" would otherwise write outside the
// attachments directory — on Windows via "\" as well.
var extByType = map[string]string{
	// images
	"image/png":                "png",
	"image/jpeg":               "jpg",
	"image/gif":                "gif",
	"image/webp":               "webp",
	"image/avif":               "avif",
	"image/bmp":                "bmp",
	"image/tiff":               "tif",
	"image/svg+xml":            "svg",
	"image/svg":                "svg",
	"image/x-icon":             "ico",
	"image/vnd.microsoft.icon": "ico",
	// video
	"video/mp4":        "mp4",
	"video/webm":       "webm",
	"video/quicktime":  "mov",
	"video/x-matroska": "mkv",
	"video/x-msvideo":  "avi",
	"video/mpeg":       "mpeg",
	// audio
	"audio/mpeg": "mp3",
	"audio/wav":  "wav",
	"audio/ogg":  "ogg",
	"audio/webm": "weba",
	// documents and archives
	"application/pdf":             "pdf",
	"application/zip":             "zip",
	"application/x-7z-compressed": "7z",
	"application/gzip":            "gz",
	"application/x-tar":           "tar",
	"application/json":            "json",
	"application/xml":             "xml",
	"application/msword":          "doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"application/vnd.ms-excel": "xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.ms-powerpoint":                                             "ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	// text
	"text/plain":      "txt",
	"text/csv":        "csv",
	"text/markdown":   "md",
	"text/html":       "html",
	"text/xml":        "xml",
	"text/css":        "css",
	"text/javascript": "js",
}

// BaseType strips parameters from a media type and lower-cases it, so
// "Image/PNG; charset=binary" compares as "image/png".
func BaseType(contentType string) string {
	base, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		base, _, _ = strings.Cut(contentType, ";")
	}
	return strings.ToLower(strings.TrimSpace(base))
}

// ExtFor returns the storage extension for a content type, falling back to
// "bin". The result is always a bare alphanumeric token, so it can never carry
// a path separator or a "..".
func ExtFor(contentType string) string {
	if ext, ok := extByType[BaseType(contentType)]; ok {
		return ext
	}
	return "bin"
}

// ResolveContentType decides what an uploaded part actually is: the type the
// client declared, or — when that is missing, unparseable or the generic
// octet-stream — a sniff of the first bytes. Any type is accepted; this only
// decides what gets recorded.
func ResolveContentType(declared string, head []byte) string {
	base := BaseType(declared)
	if base != "" && base != DefaultContentType {
		return base
	}
	if len(head) > 0 {
		if sniffed := BaseType(http.DetectContentType(head)); sniffed != "" {
			return sniffed
		}
	}
	if base != "" {
		return base
	}
	return DefaultContentType
}

// IsInline reports whether a content type may be rendered in the browser
// rather than downloaded.
//
// SVG is carved out of the image branch deliberately: it is a scriptable
// document rather than a picture, and X-Content-Type-Options: nosniff is no
// help when the declared type genuinely is image/svg+xml. Serving one inline
// would let an uploaded file execute inside the app's own origin.
func IsInline(contentType string) bool {
	base := BaseType(contentType)
	if base == "image/svg+xml" || base == "image/svg" {
		return false
	}
	return strings.HasPrefix(base, "image/") || strings.HasPrefix(base, "video/")
}
