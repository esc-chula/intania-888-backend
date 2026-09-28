package color

// Response is the HTTP representation of a color's completed-match standings.
type Response struct {
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Won        int64  `json:"won"`
	Drawn      int64  `json:"drawn"`
	Lost       int64  `json:"lost"`
	TotalMatch int64  `json:"total_matches"`
} // @name model.ColorDto
