// Package catalogueid validates catalogue IDs shared by sports, locations, and standings filters.
package catalogueid

// Valid reports whether id contains 1–100 ASCII letters, digits, underscores, or hyphens.
// Both letter cases are accepted, and IDs are validated without normalization.
func Valid(id string) bool {
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
