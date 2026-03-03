package dto

type ReadMessageRequest struct {
	MessageID string `json:"message_id"`
	ThreadID  string `json:"thread_id"`
}
