package mapper

import (
	"strconv"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func MapToSendTextRequest(in *impb.SendTextRequest) *dto.SendTextRequest {
	if in == nil {
		return nil
	}
	return &dto.SendTextRequest{
		To:   MapPeerFromProto(in.GetTo()),
		Body: in.GetBody(),
	}
}

func MapPeerFromProto(pb *impb.Peer) shared.Peer {
	if pb == nil {
		return shared.Peer{}
	}

	var p shared.Peer
	switch kind := pb.Kind.(type) {
	case *impb.Peer_ContactId:
		id, err := strconv.ParseInt(kind.ContactId, 10, 64)
		if err == nil {
			p.ID = id
		}
		p.Type = shared.PeerContact

	case *impb.Peer_GroupId:
		id, err := strconv.ParseInt(kind.GroupId, 10, 64)
		if err == nil {
			p.ID = id
		}
		p.Type = shared.PeerGroup

	case *impb.Peer_ChannelId:
		id, err := strconv.ParseInt(kind.ChannelId, 10, 64)
		if err == nil {
			p.ID = id
		}
		p.Type = shared.PeerChannel
	}
	return p
}

func MapToSendTextResponse(out *dto.SendTextResponse) *impb.SendTextResponse {
	if out == nil {
		return nil
	}
	return &impb.SendTextResponse{
		Id: out.ID.String(),
		To: MapPeerToProto(out.To),
	}
}

func MapPeerToProto(p shared.Peer) *impb.Peer {
	res := &impb.Peer{}
	switch p.Type {
	case shared.PeerContact:
		res.Kind = &impb.Peer_ContactId{ContactId: p.IDString()}
	case shared.PeerGroup:
		res.Kind = &impb.Peer_GroupId{GroupId: p.IDString()}
	case shared.PeerChannel:
		res.Kind = &impb.Peer_ChannelId{ChannelId: p.IDString()}
	}
	return res
}
