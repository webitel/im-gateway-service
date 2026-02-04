package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

//go:generate goverter gen .

func ToThreadKind(source impb.ThreadKind) (dto.ThreadKind) {
	switch source {
	case impb.ThreadKind_CHANNEL:
		return dto.ThreadKindChannel
	case impb.ThreadKind_DIRECT:
		return dto.ThreadKindDirect
	case impb.ThreadKind_GROUP:
		return dto.ThreadKindGroup
	default:
		return dto.ThreadKindUnknown
	}
}

func FromThreadKind(source dto.ThreadKind) impb.ThreadKind {
	switch source {
	case dto.ThreadKindChannel:
		return impb.ThreadKind_CHANNEL
	case dto.ThreadKindDirect:
		return impb.ThreadKind_DIRECT
	case dto.ThreadKindGroup:
		return impb.ThreadKind_GROUP
	default:
		return impb.ThreadKind_UNKNOWN
	}
}

func FromDTOToThreadV1Kind(source dto.ThreadKind) threadv1.ThreadKind {
	switch source {
	case dto.ThreadKindChannel:
		return threadv1.ThreadKind_CHANNEL
	case dto.ThreadKindDirect:
		return threadv1.ThreadKind_DIRECT
	case dto.ThreadKindGroup:
		return threadv1.ThreadKind_GROUP
	default:
		return threadv1.ThreadKind_UNKNOWN
	}
}

func FromThreadV1KindToDTOKind(source threadv1.ThreadKind) dto.ThreadKind {
	switch source {
	case threadv1.ThreadKind_CHANNEL:
		return dto.ThreadKindChannel
	case threadv1.ThreadKind_DIRECT:
		return dto.ThreadKindDirect
	case threadv1.ThreadKind_GROUP:
		return dto.ThreadKindGroup
	default:
		return dto.ThreadKindUnknown
	}
}

func StringToParticipant(id string) dto.ExternalParticipantDTO {
    return dto.ExternalParticipantDTO{
        InternalID: id,
    }
}

func PeerIdentityToSharedPeer(source *impb.PeerIdentity) shared.Peer {
	return shared.Peer{
		ID:     source.Sub,
		Issuer: source.Iss,
		Type:   shared.PeerContact,
	}
}

// goverter:converter
// goverter:extend ToThreadKind StringToParticipant PeerIdentityToSharedPeer
// goverter:extend FromThreadKind
// goverter:extend FromDTOToThreadV1Kind
// goverter:extend FromThreadV1KindToDTOKind
// goverter:matchIgnoreCase
// goverter:ignoreUnexported
// goverter:useZeroValueOnPointerInconsistency
type ThreadConverter interface {
    ProtoToDTO(source impb.ThreadKind) dto.ThreadKind
    DTOToExternalParticipantProto(source *dto.ExternalParticipantDTO) *impb.ExternalParticipant 
    DTOToThreadDirectSettingsProto(source *dto.ThreadDirectSettingsDTO) *impb.ThreadDirectSettings
    DTOToThreadMemberProto(source *dto.ThreadMemberDTO) *impb.ThreadMember

	ProtoThreadSearchRequestToDTO(source *impb.ThreadSearchRequest) *dto.ThreadSearchRequestDTO
	DTOToProto(source []*dto.ThreadDTO) []*impb.Thread	
	
	// goverter:map Id Member
    ToThreadMemberDTO(source *threadv1.ThreadMember) *dto.ThreadMemberDTO
	ThreadV1ToThreadDTO(source *threadv1.Thread) *dto.ThreadDTO
	ThreadV1ListToThreadDTOList(source []*threadv1.Thread) []*dto.ThreadDTO
	DTOKindsToThreadV1Kinds(source []dto.ThreadKind) []threadv1.ThreadKind
}

