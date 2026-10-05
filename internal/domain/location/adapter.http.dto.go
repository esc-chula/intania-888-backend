package location

// Response is the location identifier and display title returned by HTTP.
type Response struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// CreateRequest defines an immutable catalogue ID and mutable display title.
type CreateRequest struct {
	ID    string `json:"id" validate:"required,min=1,max=100"`
	Title string `json:"title" validate:"required,min=1,max=100"`
}

// ValidateRequest checks the ID and normalized title.
func (r CreateRequest) ValidateRequest() map[string]string {
	details := titleDetails(r.Title)
	if details == nil {
		details = make(map[string]string)
	}
	if !validID(r.ID) {
		details["id"] = "must be a nonempty ASCII string of at most 100 characters"
	}

	if len(details) == 0 {
		return nil
	}

	return details
}

// UpdateRequest renames a location without changing its ID.
type UpdateRequest struct {
	Title string `json:"title" validate:"required,min=1,max=100"`
}

// ValidateRequest requires a title with 1–100 characters after trimming.
func (r UpdateRequest) ValidateRequest() map[string]string {
	return titleDetails(r.Title)
}

func titleDetails(title string) map[string]string {
	if _, err := normalizeTitle(title); err != nil {
		return map[string]string{"title": "must contain 1–100 characters after trimming"}
	}

	return nil
}
