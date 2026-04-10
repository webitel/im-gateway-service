package dto

import (
	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
)

type ThreadKind int

const (
	ThreadKindUnknown ThreadKind = iota
	ThreadKindDirect
	ThreadKindGroup
	ThreadKindChannel
)

type (
	ExternalParticipantDTO struct {
		InternalID string

		Iss   string
		Sub   string
		Type  string
		Name  string
		IsBot bool
	}

	ThreadDirectSettingsDTO struct {
		ID        string
		DomainID  int32
		CreatedAt int64
		UpdatedAt int64
		Title     string
	}

	ThreadMemberDTO struct {
		Member         *ExternalParticipantDTO
		DirectSettings *ThreadDirectSettingsDTO
	}

	ThreadDTO struct {
		ID          string
		DomainID    int32
		CreatedAt   int64
		UpdatedAt   int64
		Kind        ThreadKind
		Subject     string
		Description string
		Members     []*ThreadMemberDTO
		LastMsg     *HistoryMessage
		Variables   *ThreadVariablesDTO
	}

	ThreadSearchRequestDTO struct {
		Fields []string
		IDs    []string
		Kinds  []ThreadKind
		Owners []shared.Peer
		Q      string
		Size   int32
		Sort   string
		Page   int32
	}
)

type ThreadVariablesDTO struct {
	ThreadID  string                       `json:"thread_id"`
	Variables map[string]*VariableEntryDTO `json:"variables,omitempty"`
}

type VariableEntryDTO struct {
	Value           map[string]any     `json:"value,omitempty"`
	SetBy           *gtwthread.Contact `json:"set_by,omitempty"`
	SetByInternalID string             `json:"set_by_internal_id,omitempty"`
	SetAt           int64              `json:"set_at,omitempty"`
}
