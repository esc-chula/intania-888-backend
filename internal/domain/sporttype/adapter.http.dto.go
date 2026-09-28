package sporttype

// Response is the sport catalogue identifier and title returned by HTTP.
type Response struct {
	ID    string `json:"id"`
	Title string `json:"title"`
} // @name model.SportTypeDto
