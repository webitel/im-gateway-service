package grpc

import (
	"testing"

	"google.golang.org/protobuf/proto"

	contactservice "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
)

func Test_mapContactsToGatewayResponseProto(t *testing.T) {
	tests := []struct {
		name     string
		internal *contactservice.ContactList
		want     *impb.ContactList
	}{
		{
			name: "maps contact id to gateway response",
			internal: &contactservice.ContactList{
				Page: 1,
				Size: 10,
				Next: false,
				Contacts: []*contactservice.Contact{
					{
						Id:       "contact-uuid-1",
						IssId:    "webitel",
						Subject:  "user-1",
						Name:     "John Doe",
						Username: "john.doe",
						AppId:    "app-1",
						Type:     "person",
						IsBot:    false,
					},
					{
						Id:       "contact-uuid-2",
						IssId:    "webitel",
						Subject:  "user-2",
						Name:     "Jane Smith",
						Username: "jane.smith",
						AppId:    "app-1",
						Type:     "person",
						IsBot:    false,
					},
				},
			},
			want: &impb.ContactList{
				Page: 1,
				Size: 10,
				Next: false,
				Items: []*impb.Contact{
					{
						Id:       "contact-uuid-1",
						Iss:      "webitel",
						Sub:      "user-1",
						Name:     "John Doe",
						Username: "john.doe",
						AppId:    "app-1",
						Type:     "person",
						IsBot:    false,
					},
					{
						Id:       "contact-uuid-2",
						Iss:      "webitel",
						Sub:      "user-2",
						Name:     "Jane Smith",
						Username: "jane.smith",
						AppId:    "app-1",
						Type:     "person",
						IsBot:    false,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapContactsToGatewayResponseProto(tt.internal)
			if got.GetPage() != tt.want.GetPage() || got.GetSize() != tt.want.GetSize() || got.GetNext() != tt.want.GetNext() {
				t.Errorf("pagination mismatch: got page=%d size=%d next=%v, want page=%d size=%d next=%v",
					got.GetPage(), got.GetSize(), got.GetNext(), tt.want.GetPage(), tt.want.GetSize(), tt.want.GetNext())
			}

			if len(got.GetItems()) != len(tt.want.GetItems()) {
				t.Errorf("items count mismatch: got %d, want %d", len(got.GetItems()), len(tt.want.GetItems()))
			}

			for i, item := range got.GetItems() {
				wantItem := tt.want.GetItems()[i]
				if item.GetId() != wantItem.GetId() {
					t.Errorf("contact[%d].Id mismatch: got %q, want %q", i, item.GetId(), wantItem.GetId())
				}

				if item.GetName() != wantItem.GetName() {
					t.Errorf("contact[%d].Name mismatch: got %q, want %q", i, item.GetName(), wantItem.GetName())
				}

				if item.GetIss() != wantItem.GetIss() {
					t.Errorf("contact[%d].Iss mismatch: got %q, want %q", i, item.GetIss(), wantItem.GetIss())
				}
			}
		})
	}
}

// Gateway and contact-service number ids/only_bots differently, so a wire proxy alone
// would drop ids and turn only_bots into domain_id.
func Test_mapSearchContactRequest(t *testing.T) {
	onlyBots := true

	tests := []struct {
		name string
		in   *impb.SearchContactRequest
		want *contactservice.SearchContactRequest
	}{
		{
			name: "ids and only_bots are mapped by name",
			in: &impb.SearchContactRequest{
				Page: 2, Size: 10, Q: "jo", Sort: "-name", Fields: []string{"id"},
				Type: []string{"telegram"}, Subjects: []string{"s1"},
				Ids: []string{"0190a6f6-0000-7000-8000-000000000001"}, OnlyBots: &onlyBots,
			},
			want: &contactservice.SearchContactRequest{
				Page: 2, Size: 10, Q: "jo", Sort: "-name", Fields: []string{"id"},
				Type: []string{"telegram"}, Subjects: []string{"s1"},
				Ids: []string{"0190a6f6-0000-7000-8000-000000000001"}, OnlyBots: &onlyBots,
			},
		},
		{
			name: "empty filters stay empty",
			in:   &impb.SearchContactRequest{Page: 1, Size: 20},
			want: &contactservice.SearchContactRequest{Page: 1, Size: 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapSearchContactRequest(tt.in)
			if err != nil {
				t.Fatalf("mapSearchContactRequest() error = %v", err)
			}

			if !proto.Equal(got, tt.want) {
				t.Errorf("mapSearchContactRequest() = %v, want %v", got, tt.want)
			}

			if got.GetDomainId() != 0 {
				t.Errorf("domain_id leaked from only_bots: %d", got.GetDomainId())
			}
		})
	}
}
