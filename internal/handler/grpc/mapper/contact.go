package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// goverter:converter
// goverter:matchIgnoreCase
// goverter:extend Int32ToInt
// goverter:extend IntToInt32
// goverter:output:file ./generated/contact.go
type ContactMapper interface {
	// goverter:ignore AppID
	// goverter:ignore IssID
	// goverter:ignore IDs
	// goverter:ignore DomainID
	ToSearchContactRequest(*impb.SearchContactRequest) *dto.SearchContactRequest

	// goverter:ignoreUnexported
	ToContactList(in *dto.ContactList) *impb.ContactList

	// goverter:ignoreUnexported
	ToContact(*dto.Contact) *impb.Contact
}

func Int32ToInt(i int32) int {
	return int(i)
}

func IntToInt32(i int) int32 {
	return int32(i)
}
