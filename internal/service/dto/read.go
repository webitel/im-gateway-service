package dto

type ReadMessageRequest struct {
	MessageID string `json:"message_id"`
	ThreadID  string `json:"thread_id"`
	// UpToSeq reads up to this message seq; wins over MessageID when set.
	UpToSeq int64 `json:"up_to_seq"`
}
