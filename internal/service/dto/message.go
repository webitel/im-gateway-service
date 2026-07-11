package dto

type EditMessageRequest struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type EditMessageResponse struct {
	ID       string `json:"id"`
	EditedAt int64  `json:"edited_at"`
}
