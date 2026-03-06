package mapper_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
)

// boolPtr is a helper to get a pointer to a bool literal.
func boolPtr(v bool) *bool { return &v }

// TestConvert_MatchingFields verifies that all fields shared by both proto
// types are copied correctly when converting gateway → upstream contact.
func TestConvert_MatchingFields(t *testing.T) {
	in := &pb.SearchContactRequest{
		Page:     1,
		Size:     10,
		Sort:     "+name",
		Fields:   []string{"id", "name"},
		Q:        "alice",
		Type:     []string{"telegram"},
		Subjects: []string{"subj-1"},
		OnlyBots: boolPtr(false),
	}

	got, err := mapper.Convert(in, &contactv1.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := &contactv1.SearchContactRequest{
		Page:     1,
		Size:     10,
		Sort:     "+name",
		Fields:   []string{"id", "name"},
		Q:        "alice",
		Type:     []string{"telegram"},
		Subjects: []string{"subj-1"},
		OnlyBots: boolPtr(false),
	}

	if !proto.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestConvert_EmptySource verifies that converting an empty message produces
// an empty destination (all fields at zero value).
func TestConvert_EmptySource(t *testing.T) {
	got, err := mapper.Convert(&pb.SearchContactRequest{}, &contactv1.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !proto.Equal(got, &contactv1.SearchContactRequest{}) {
		t.Errorf("expected empty message, got %v", got)
	}
}

// TestConvert_SourceFieldsNotInDstAreDiscarded verifies that fields present in
// the source type but absent from the destination schema do not cause an error
// and do not corrupt the destination (DiscardUnknown: true).
// contact.SearchContactRequest has domain_id / app_id which gateway does not
// expose — converting contact → gateway must silently drop those extras.
func TestConvert_SourceFieldsNotInDstAreDiscarded(t *testing.T) {
	in := &contactv1.SearchContactRequest{
		Page:     3,
		Q:        "bot",
		DomainId: 999,   // not present in gateway.SearchContactRequest
		AppId:    []string{"app-1"}, // not present in gateway.SearchContactRequest
	}

	got, err := mapper.Convert(in, &pb.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the shared fields should be populated.
	want := &pb.SearchContactRequest{Page: 3, Q: "bot"}
	if !proto.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestConvert_DstOnlyFieldsRemainZero verifies that fields that exist only in
// the destination type and are absent from the source are left at zero value.
func TestConvert_DstOnlyFieldsRemainZero(t *testing.T) {
	in := &pb.SearchContactRequest{Page: 2, Size: 5}

	got, err := mapper.Convert(in, &contactv1.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// dst-only fields (AppId, IssId, Ids, DomainId) must be zero.
	if len(got.AppId) != 0 {
		t.Errorf("AppId should be empty, got %v", got.AppId)
	}
	if len(got.IssId) != 0 {
		t.Errorf("IssId should be empty, got %v", got.IssId)
	}
	if got.DomainId != 0 {
		t.Errorf("DomainId should be 0, got %v", got.DomainId)
	}
	// Shared fields must be set.
	if got.Page != 2 || got.Size != 5 {
		t.Errorf("shared fields incorrect: page=%d size=%d", got.Page, got.Size)
	}
}

// TestConvert_OptionalBoolTrue verifies that an optional bool field set to
// true is preserved across the conversion.
func TestConvert_OptionalBoolTrue(t *testing.T) {
	in := &pb.SearchContactRequest{OnlyBots: boolPtr(true)}

	got, err := mapper.Convert(in, &contactv1.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.OnlyBots == nil {
		t.Fatal("OnlyBots should not be nil")
	}
	if !*got.OnlyBots {
		t.Errorf("OnlyBots: got false, want true")
	}
}

// TestConvert_OptionalBoolNil verifies that an unset optional bool field
// remains nil after conversion.
func TestConvert_OptionalBoolNil(t *testing.T) {
	in := &pb.SearchContactRequest{Page: 1} // OnlyBots intentionally unset

	got, err := mapper.Convert(in, &contactv1.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.OnlyBots != nil {
		t.Errorf("OnlyBots: got %v, want nil", got.OnlyBots)
	}
}

// TestConvert_SameType verifies that converting a message to its own type
// produces a value equal to the source.
func TestConvert_SameType(t *testing.T) {
	in := &contactv1.SearchContactRequest{
		Page:     7,
		Size:     20,
		Q:        "test",
		DomainId: 42,
		OnlyBots: boolPtr(true),
	}

	got, err := mapper.Convert(in, &contactv1.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !proto.Equal(got, in) {
		t.Errorf("same-type conversion: got %v, want %v", got, in)
	}
}

// TestConvert_RepeatedFields verifies that repeated (slice) fields are
// copied correctly and independently (no aliasing).
func TestConvert_RepeatedFields(t *testing.T) {
	subjects := []string{"s1", "s2", "s3"}
	in := &pb.SearchContactRequest{Subjects: subjects}

	got, err := mapper.Convert(in, &contactv1.SearchContactRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Subjects) != len(subjects) {
		t.Fatalf("subjects len: got %d, want %d", len(got.Subjects), len(subjects))
	}
	for i, s := range subjects {
		if got.Subjects[i] != s {
			t.Errorf("subjects[%d]: got %q, want %q", i, got.Subjects[i], s)
		}
	}
}
