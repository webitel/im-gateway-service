package dto

type HistoryMessageCursor struct {
	CreatedAt int64  `json:"created_at"`
	ID        string `json:"id"`
	Direction bool   `json:"direction"`
}

type SearchMessageHistoryRequest struct {
	Fields      []string              `json:"fields,omitempty"`
	IDs         []string              `json:"ids,omitempty"`
	ThreadIDs   []string              `json:"thread_ids,omitempty"`
	SenderIDs   []string              `json:"sender_ids,omitempty"`
	ReceiverIDs []string              `json:"receiver_ids,omitempty"`
	Types       []int32               `json:"types,omitempty"`
	DomainID    int32                 `json:"domain_id"`
	Cursor      *HistoryMessageCursor `json:"cursor,omitempty"`
	Size        uint32                `json:"size"`
}

type HistoryDocument struct {
	ID        string `json:"id"`
	MessageID string `json:"message_id"`
	FileID    int64  `json:"file_id"`
	Name      string `json:"name"`
	Mime      string `json:"mime"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"created_at"`
	URL       string `json:"url"`
}

type HistoryImage struct {
	ID        string `json:"id"`
	MessageID string `json:"message_id"`
	FileID    int64  `json:"file_id"`
	Mime      string `json:"mime"`
	Width     int32  `json:"width"`
	Height    int32  `json:"height"`
	CreatedAt int64  `json:"created_at"`
	URL       string `json:"url"`
}

type HistoryMessage struct {
	ID         string            `json:"id"`
	ThreadID   string            `json:"thread_id"`
	SenderID   string            `json:"sender_id"`
	ReceiverID string            `json:"receiver_id"`
	Type       int32             `json:"type"`
	Body       string            `json:"body"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	CreatedAt  int64             `json:"created_at"`
	UpdatedAt  int64             `json:"updated_at"`
	Documents  []HistoryDocument `json:"documents,omitempty"`
	Images     []HistoryImage    `json:"images,omitempty"`

	Receiver *MessageSender `json:"receiver"`
	Sender   *MessageSender `json:"sender"`
}

type Cursors struct {
	After  *HistoryMessageCursor `json:"after,omitempty"`
	Before *HistoryMessageCursor `json:"before,omitempty"`
}

type Paging struct {
	Cursors Cursors `json:"cursors"`
}

type SearchMessageHistoryResponse struct {
	Messages       []HistoryMessage      `json:"messages"`
	NextCursor     *HistoryMessageCursor `json:"next_cursor,omitempty"`
	Next           bool                  `json:"next"`
	Paging         Paging                `json:"paging"`
	MessageSenders []*MessageSender      `json:"message_senders"`
}

type MessageSender struct {
	Subject string `json:"subject"`
	Issuer  string `json:"issuer"`
	Type    string `json:"type"`
}

func NewMessageSender(sub, iss, senderType string) *MessageSender {
	return &MessageSender{
		Subject: sub,
		Issuer:  iss,
		Type:    senderType,
	}
}
