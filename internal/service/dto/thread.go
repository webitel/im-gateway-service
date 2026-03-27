package dto

import "github.com/webitel/im-gateway-service/internal/domain/shared"

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
		Owner       *ExternalParticipantDTO
		Admins      []*ExternalParticipantDTO
		MemberIDs   []*ExternalParticipantDTO
		Subject     string
		Description string
		Members     []*ThreadMemberDTO
		LastMsg     *HistoryMessage
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
