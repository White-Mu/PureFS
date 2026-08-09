package search

import (
	"io"
	"strings"
)

// ExtractText reads from reader and returns extracted plain text suitable for
// full-text indexing. The mimeType hint is used to select an extraction
// strategy. If the format is not supported, an empty string and nil error are
// returned so the caller can still index the file (just without content).
func ExtractText(name, mimeType string, reader io.Reader) (string, error) {
	ext := strings.ToLower(name)
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			ext = strings.ToLower(name[i:])
			break
		}
	}

	switch {
	case isTextExtension(ext) || isTextMime(mimeType):
		return readAllText(reader)

	default:
		// Format not supported for text extraction.
		return "", nil
	}
}

// textExtensions lists extensions whose content can be read as plain text.
var textExtensions = map[string]bool{
	".txt":  true,
	".md":   true,
	".markdown": true,
	".csv":  true,
	".json": true,
	".xml":  true,
	".html": true,
	".htm":  true,
	".css":  true,
	".js":   true,
	".ts":   true,
	".tsx":  true,
	".jsx":  true,
	".yaml": true,
	".yml":  true,
	".toml": true,
	".ini":  true,
	".cfg":  true,
	".conf": true,
	".go":   true,
	".py":   true,
	".rs":   true,
	".java": true,
	".c":    true,
	".cpp":  true,
	".h":    true,
	".hpp":  true,
	".sh":   true,
	".bat":  true,
	".ps1":  true,
	".sql":  true,
	".log":  true,
	".svg":  true,
}

func isTextExtension(ext string) bool {
	return textExtensions[ext]
}

func isTextMime(mime string) bool {
	return strings.HasPrefix(mime, "text/") ||
		mime == "application/json" ||
		mime == "application/xml" ||
		mime == "application/javascript" ||
		mime == "application/x-yaml"
}

func readAllText(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	// Truncate to a reasonable size for indexing (1 MB).
	const maxLen = 1 << 20 // 1 MB
	if len(data) > maxLen {
		data = data[:maxLen]
	}
	return string(data), nil
}
