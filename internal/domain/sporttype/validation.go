package sporttype

import (
	"strings"
	"unicode/utf8"
)

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	length := utf8.RuneCountInString(title)
	if length == 0 || length > 100 {
		return "", ErrInvalidSportType
	}

	return title, nil
}
