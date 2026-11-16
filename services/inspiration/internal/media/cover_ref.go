package media

import (
	"strings"
	"unicode"

	"omnimux-inspiration/internal/errs"
)

const CoverRefPrefix = "r2/"

// ParseCoverRef returns the complete object key, without an upload prefix.
func ParseCoverRef(ref string) (string, error) {
	if len(ref) > 255 || !strings.HasPrefix(ref, CoverRefPrefix+"publications/") {
		return "", errs.ErrInvalidParam
	}
	key := strings.TrimPrefix(ref, CoverRefPrefix)
	if strings.ContainsAny(key, "\\%?#:") || strings.IndexFunc(key, unicode.IsControl) >= 0 {
		return "", errs.ErrInvalidParam
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.TrimSpace(segment) != segment {
			return "", errs.ErrInvalidParam
		}
	}
	return key, nil
}
