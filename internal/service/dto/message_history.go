package dto

import api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"

type HistoryMessageCursor struct {
	ID     string `json:"id"`
	Before bool
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
	CallerID    string
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

type ApiInteractiveCallbackWrapper struct {
	*api.InteractiveCallback

	ContactID string
}

type HistoryReplyTo struct {
	ID             string  `json:"id"`
	SenderID       string  `json:"sender_id"`
	Type           int32   `json:"type"`
	Body           string  `json:"body"`
	CreatedAt      int64   `json:"created_at"`
	AttachmentKind *string `json:"attachment_kind,omitempty"`
	AttachmentName *string `json:"attachment_name,omitempty"`
	AttachmentMime *string `json:"attachment_mime,omitempty"`

	Sender *MessageSender `json:"sender,omitempty"`
}

type HistoryMessage struct {
	ID        string            `json:"id"`
	ThreadID  string            `json:"thread_id"`
	SenderID  string            `json:"sender_id"`
	Type      int32             `json:"type"`
	Body      string            `json:"body"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
	CreatedAt int64             `json:"created_at"`
	UpdatedAt int64             `json:"updated_at"`
	Seq       int64             `json:"seq,omitempty"`
	Documents []HistoryDocument `json:"documents,omitempty"`
	Images    []HistoryImage    `json:"images,omitempty"`

	Sender          *MessageSender       `json:"sender"`
	Location        *api.MessageLocation `json:"location"`
	Contact         *api.MessageContact  `json:"contact"`
	Interactive     *api.Interactive     `json:"interactive"`
	System          *api.System          `json:"system"`
	ReactedMetadata *ApiInteractiveCallbackWrapper
	ReplyTo         *HistoryReplyTo `json:"reply_to,omitempty"`

	// ForwardOrigin carries no enriched sender: the original author is usually
	// not a member of this chat, so SenderName is the only usable label.
	ForwardOrigin *api.ForwardOrigin `json:"forward_origin,omitempty"`

	// DeliveryStatus is the aggregate across recipients; UNSPECIFIED for
	// messages without per-recipient tracking (historical).
	DeliveryStatus api.MessageDeliveryStatus     `json:"delivery_status,omitempty"`
	Statuses       []*api.MessageRecipientStatus `json:"statuses,omitempty"`

	// Deleted marks a message removed by its author; its content fields arrive
	// empty from im-thread-service. The removed text stays reachable through
	// GetMessageRevisions.
	Deleted   bool   `json:"deleted,omitempty"`
	DeletedAt int64  `json:"deleted_at,omitempty"`
	DeletedBy string `json:"deleted_by,omitempty"`

	RevisionCount int32 `json:"revision_count,omitempty"`
}

type MessageRevision struct {
	Version   int32                     `json:"version"`
	Action    api.MessageRevisionAction `json:"action"`
	Body      string                    `json:"body"`
	ChangedBy string                    `json:"changed_by"`
	ChangedAt int64                     `json:"changed_at"`
}

type GetMessageRevisionsRequest struct {
	MessageID string
	DomainID  int32
	CallerID  string
}

type Cursors struct {
	After  *HistoryMessageCursor `json:"after,omitempty"`
	Before *HistoryMessageCursor `json:"before,omitempty"`
}

type Paging struct {
	Cursors Cursors `json:"cursors"`
}

type SearchMessageHistoryResponse struct {
	Messages       []*HistoryMessage     `json:"messages"`
	NextCursor     *HistoryMessageCursor `json:"next_cursor,omitempty"`
	PrevCursor     *HistoryMessageCursor
	MessageSenders []*MessageSender `json:"message_senders"`
}

type SearchLeftThreadsMessageHistoryRequest struct {
	Fields     []string              `json:"fields,omitempty"`
	ThreadID   string                `json:"thread_id"`
	SenderIDs  []string              `json:"sender_ids,omitempty"`
	Types      []int32               `json:"types,omitempty"`
	PeriodFrom int64                 `json:"period_from,omitempty"`
	PeriodTo   int64                 `json:"period_to,omitempty"`
	DomainID   int32                 `json:"domain_id"`
	Cursor     *HistoryMessageCursor `json:"cursor,omitempty"`
	Size       uint32                `json:"size"`
}

type MessageSender struct {
	Sub      string `json:"subject"`
	Iss      string `json:"issuer"`
	Type     string `json:"type"`
	Name     string `json:"user_name"`
	IsBot    bool   `json:"is_bot"`
	MemberID string `json:"member_id"`
	Role     int    `json:"role"`
	Username string `json:"username"`
}

func NewMessageSender(sub, iss, senderType, name, username string, isBot bool) *MessageSender {
	return &MessageSender{
		Sub:      sub,
		Iss:      iss,
		Type:     senderType,
		Name:     name,
		IsBot:    isBot,
		Username: username,
	}
}
