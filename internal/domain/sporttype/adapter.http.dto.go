package sporttype

import "github.com/esc-chula/intania-888-backend/internal/catalogueid"

// Response is the sport catalogue identifier and title returned by HTTP.
type Response struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// CreateRequest defines an immutable catalogue ID and a mutable display title.
type CreateRequest struct {
	ID    string `json:"id" validate:"required,min=1,max=100"`
	Title string `json:"title" validate:"required,min=1,max=100"`
}

// ValidateRequest validates ASCII identifiers and trimmed Unicode titles.
func (r CreateRequest) ValidateRequest() map[string]string {
	details := titleDetails(r.Title)
	if !catalogueid.Valid(r.ID) {
		details["id"] = "must contain 1–100 ASCII letters, digits, underscores, or hyphens"
	}

	return details
}

// UpdateRequest renames a sport type; its ID cannot be changed.
type UpdateRequest struct {
	Title string `json:"title" validate:"required,min=1,max=100"`
}

// ValidateRequest requires a title with 1–100 characters after trimming.
func (r UpdateRequest) ValidateRequest() map[string]string {
	return titleDetails(r.Title)
}

func titleDetails(title string) map[string]string {
	details := make(map[string]string)
	if _, err := normalizeTitle(title); err != nil {
		details["title"] = "must contain 1–100 characters after trimming"
	}

	return details
}
