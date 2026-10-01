package sporttype

import (
	"strings"
	"unicode/utf8"
)

func validID(id string) bool {
	if len(id) == 0 || len(id) > 100 {
		return false
	}

	for _, char := range id {
		letter := (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
		digit := char >= '0' && char <= '9'
		if !letter && !digit && char != '_' && char != '-' {
			return false
		}
	}

	return true
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	length := utf8.RuneCountInString(title)
	if length == 0 || length > 100 {
		return "", ErrInvalidSportType
	}

	return title, nil
}
