package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func MapToSearchContactRequest(in *impb.SearchContactRequest) *dto.SearchContactRequest {
	if in == nil {
		return nil
	}

	return &dto.SearchContactRequest{
		Page:     int(in.GetPage()),
		Size:     int(in.GetSize()),
		Q:        in.GetQ(),
		Sort:     in.GetSort(),
		Fields:   in.GetFields(),
		Type:     in.GetType(),
		Subjects: in.GetSubjects(),
	}
}

func MapToContact(in *dto.Contact) *impb.Contact {
	return &impb.Contact{
		IssId:     in.IssID,
		AppId:     in.AppID,
		Type:      in.Type,
		Name:      in.Type,
		Username:  in.Username,
		Metadata:  in.Metadata,
		CreatedAt: in.CreatedAt,
		UpdatedAt: in.UpdatedAt,
		Subject:   in.Subject,
	}
}

func MapToContactList(in *dto.ContactList) *impb.ContactList {
	if in == nil {
		return nil
	}

	out := &impb.ContactList{
		Page:  int32(in.Page),
		Size:  int32(in.Size),
		Next:  in.Next,
		Items: make([]*impb.Contact, len(in.Items)),
	}
	for i, item := range in.Items {
		out.Items[i] = MapToContact(item)
	}
	return out
}
