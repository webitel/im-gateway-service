package service

import (
	"context"
	stderrors "errors"
	"io"
	"log/slog"
	"math"
	"slices"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	crmcontacts "github.com/webitel/im-gateway-service/infra/client/crm-contacts"
)

type fakeThreadReader struct {
	err  error
	reqs []*threadv1.GetThreadRequest
}

func (f *fakeThreadReader) Get(_ context.Context, req *threadv1.GetThreadRequest) (*threadv1.Thread, error) {
	f.reqs = append(f.reqs, req)

	return &threadv1.Thread{Id: req.GetId()}, f.err
}

type fakeCandidateLister struct {
	resp *threadv1.ListContactCandidatesResponse
	reqs []*threadv1.ListContactCandidatesRequest
}

func (f *fakeCandidateLister) ListCandidates(_ context.Context, req *threadv1.ListContactCandidatesRequest) (*threadv1.ListContactCandidatesResponse, error) {
	f.reqs = append(f.reqs, req)

	return f.resp, nil
}

type fakeVisibility struct {
	visible  map[string]bool
	writable map[string]bool
	err      error
	writeErr error
	calls    int
	// unbounded counts CRM calls made without a deadline.
	unbounded int
}

func (f *fakeVisibility) VisibleContacts(ctx context.Context, ids []string) (map[string]bool, error) {
	f.calls++
	f.countUnbounded(ctx)

	return subset(ids, f.visible, f.err)
}

func (f *fakeVisibility) WritableContacts(ctx context.Context, ids []string) (map[string]bool, error) {
	f.calls++
	f.countUnbounded(ctx)

	return subset(ids, f.writable, f.writeErr)
}

func (f *fakeVisibility) countUnbounded(ctx context.Context) {
	if _, ok := ctx.Deadline(); !ok {
		f.unbounded++
	}
}

func subset(ids []string, allowed map[string]bool, err error) (map[string]bool, error) {
	if err != nil {
		return nil, err
	}

	out := map[string]bool{}

	for _, id := range ids {
		if allowed[id] {
			out[id] = true
		}
	}

	return out, nil
}

type stubIdentity struct {
	auth.Identifier

	contactID string
	domainID  int64
	issuer    string
}

func (s stubIdentity) GetContactID() string { return s.contactID }
func (s stubIdentity) GetDomainID() int64   { return s.domainID }
func (s stubIdentity) GetIssuer() string    { return s.issuer }

// endUserCtx is a portal / widget / messenger user: it also sends
// x-webitel-access (its own session token), but it is not a Webitel user.
func endUserCtx() context.Context {
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, stubIdentity{contactID: "11111111-1111-4111-8111-111111111111", domainID: 7, issuer: "portal"})

	return metadata.NewIncomingContext(ctx, metadata.Pairs("x-webitel-access", "test-token-end-user"))
}

func operatorCtx() context.Context {
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, stubIdentity{contactID: "11111111-1111-4111-8111-111111111111", domainID: 7, issuer: WebitelIssuer})

	return metadata.NewIncomingContext(ctx, metadata.Pairs("x-webitel-access", "test-token-operator"))
}

func schemaCtx() context.Context { return auth.WithAuthType(operatorCtx(), auth.XWebitelTypeSchema) }

func engineCtx() context.Context { return auth.WithAuthType(operatorCtx(), auth.XWebitelTypeEngine) }

func candidatesResponse() *threadv1.ListContactCandidatesResponse {
	return &threadv1.ListContactCandidatesResponse{
		CrmContactId:     "1",
		CrmContactSource: threadv1.CrmContactSource_CRM_CONTACT_SOURCE_AUTO,
		Candidates: []*threadv1.ContactCandidate{
			{CrmContactId: "1", Trust: threadv1.IdentificationTrust_IDENTIFICATION_TRUST_VERIFIED, Reasons: []*threadv1.ContactMatchReason{{
				Type: threadv1.IdentificationCriterionType_IDENTIFICATION_CRITERION_TYPE_MESSENGER_ID, Trust: threadv1.IdentificationTrust_IDENTIFICATION_TRUST_VERIFIED, Origin: threadv1.IdentificationOrigin_IDENTIFICATION_ORIGIN_CHANNEL,
			}}},
			{CrmContactId: "2", Trust: threadv1.IdentificationTrust_IDENTIFICATION_TRUST_UNVERIFIED},
		},
	}
}

func TestListContactCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ctx        func() context.Context
		threadErr  error
		visible    map[string]bool
		visErr     error
		wantCode   codes.Code
		wantIDs    []string
		wantCRMHit bool
	}{
		{name: "operator sees only the contacts the crm lets them read", ctx: operatorCtx, visible: map[string]bool{"1": true}, wantIDs: []string{"1"}, wantCRMHit: true},
		{name: "nothing readable", ctx: operatorCtx, visible: map[string]bool{}, wantIDs: []string{}, wantCRMHit: true},
		{name: "not a member of the thread", ctx: operatorCtx, threadErr: errors.NotFound("thread not found"), wantCode: codes.NotFound},
		{name: "end user (not a webitel user) reaches neither im-thread nor the crm", ctx: endUserCtx, wantCode: codes.PermissionDenied},
		{name: "token rejected by the crm", ctx: operatorCtx, visErr: crmcontacts.ErrNoAccessToken, wantCode: codes.PermissionDenied, wantCRMHit: true},
		{name: "role without access to contacts sees none", ctx: operatorCtx, visErr: crmcontacts.ErrNoCRMAccess, wantIDs: []string{}, wantCRMHit: true},
		{name: "crm down", ctx: operatorCtx, visErr: stderrors.New("dial"), wantCode: codes.Unavailable, wantCRMHit: true},
		{name: "no identity", ctx: context.Background, wantCode: codes.PermissionDenied},
		{
			name:    "trusted schema call is not filtered",
			ctx:     schemaCtx,
			wantIDs: []string{"1", "2"},
		},
		{name: "engine call is filtered with the user's token", ctx: engineCtx, visible: map[string]bool{"2": true}, wantIDs: []string{"2"}, wantCRMHit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			threads := &fakeThreadReader{err: tt.threadErr}
			lister := &fakeCandidateLister{resp: candidatesResponse()}
			vis := &fakeVisibility{visible: tt.visible, err: tt.visErr}
			svc := NewContactCandidates(slog.New(slog.NewTextHandler(io.Discard, nil)), threads, lister, vis)

			resp, err := svc.ListContactCandidates(tt.ctx(), &gtwthread.ListContactCandidatesRequest{ThreadId: "22222222-2222-4222-8222-222222222222"})
			if got := vis.calls > 0; got != tt.wantCRMHit {
				t.Fatalf("crm called = %v, want %v", got, tt.wantCRMHit)
			}

			if vis.unbounded != 0 {
				t.Fatalf("%d crm calls without a deadline", vis.unbounded)
			}

			if tt.wantCode != codes.OK {
				if err == nil {
					t.Fatal("expected an error")
				}

				if errors.Code(err) != tt.wantCode {
					t.Fatalf("code = %v, want %v (%v)", errors.Code(err), tt.wantCode, err)
				}

				if tt.threadErr == nil && (len(lister.reqs) > 0 || len(threads.reqs) > 0) && !tt.wantCRMHit {
					t.Fatal("im-thread was called for a rejected caller")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			ids := []string{}
			for _, c := range resp.GetCandidates() {
				ids = append(ids, c.GetCrmContactId())
			}

			if !slices.Equal(ids, tt.wantIDs) {
				t.Fatalf("candidates = %v, want %v", ids, tt.wantIDs)
			}

			// The bound contact 1 and its source are shown only when the caller may read it.
			wantBound, wantSource := "", gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_UNSPECIFIED
			if slices.Contains(tt.wantIDs, "1") {
				wantBound, wantSource = "1", gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_AUTO
			}

			if resp.GetCrmContactId() != wantBound || resp.GetCrmContactSource() != wantSource {
				t.Fatalf("binding = %q/%v, want %q/%v", resp.GetCrmContactId(), resp.GetCrmContactSource(), wantBound, wantSource)
			}

			// The domain comes from the caller's identity, never from the payload.
			if lister.reqs[0].GetDomainId() != 7 || threads.reqs[0].GetDomainId() != 7 {
				t.Fatalf("domain = %d/%d, want 7", lister.reqs[0].GetDomainId(), threads.reqs[0].GetDomainId())
			}

			wantCaller := "11111111-1111-4111-8111-111111111111"
			if auth.IsSystemCall(tt.ctx()) {
				wantCaller = "" // trusted service calls read unrestricted, like Get
			}

			if got := threads.reqs[0].GetCallerId(); got != wantCaller {
				t.Fatalf("caller = %q, want %q", got, wantCaller)
			}

			if c := resp.GetCandidates(); len(c) > 0 && c[0].GetCrmContactId() == "1" {
				if got := c[0].GetReasons()[0].GetType(); got != gtwthread.IdentificationCriterionType_IDENTIFICATION_CRITERION_TYPE_MESSENGER_ID {
					t.Fatalf("reason type = %v", got)
				}
			}
		})
	}
}

func TestCheckCRMContactAccess(t *testing.T) {
	t.Parallel()

	svc := func(v *fakeVisibility) *contactCandidates {
		return NewContactCandidates(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeThreadReader{}, &fakeCandidateLister{}, v)
	}

	tests := []struct {
		name     string
		ctx      func() context.Context
		vis      *fakeVisibility
		id       string
		wantLink bool
		wantCode codes.Code
	}{
		{name: "clearing needs no access", ctx: operatorCtx, vis: &fakeVisibility{err: stderrors.New("unused")}, id: ""},
		{name: "writable contact: bound and linked", ctx: operatorCtx, vis: &fakeVisibility{writable: map[string]bool{"5": true}}, id: "5", wantLink: true},
		{name: "read-only contact: bound, not linked", ctx: operatorCtx, vis: &fakeVisibility{visible: map[string]bool{"5": true}}, id: "5"},
		{name: "role without write access to contacts: read decides", ctx: operatorCtx, vis: &fakeVisibility{writeErr: crmcontacts.ErrNoCRMAccess, visible: map[string]bool{"5": true}}, id: "5"},
		{name: "unreadable contact", ctx: operatorCtx, vis: &fakeVisibility{}, id: "5", wantCode: codes.PermissionDenied},
		{name: "role without access to contacts", ctx: operatorCtx, vis: &fakeVisibility{err: crmcontacts.ErrNoCRMAccess, writeErr: crmcontacts.ErrNoCRMAccess}, id: "5", wantCode: codes.PermissionDenied},
		{name: "crm down", ctx: operatorCtx, vis: &fakeVisibility{writeErr: stderrors.New("dial"), err: stderrors.New("dial")}, id: "5", wantCode: codes.Unavailable},
		{name: "write check fails, contact readable: bound without the link", ctx: operatorCtx, vis: &fakeVisibility{writeErr: stderrors.New("deadline"), visible: map[string]bool{"5": true}}, id: "5"},
		{name: "write check fails, contact unreadable", ctx: operatorCtx, vis: &fakeVisibility{writeErr: stderrors.New("deadline")}, id: "5", wantCode: codes.PermissionDenied},
		{name: "no access token", ctx: operatorCtx, vis: &fakeVisibility{writeErr: crmcontacts.ErrNoAccessToken, err: crmcontacts.ErrNoAccessToken}, id: "5", wantCode: codes.PermissionDenied},
		{name: "schema call binds but never links, without asking the crm", ctx: schemaCtx, vis: &fakeVisibility{err: stderrors.New("unused"), writeErr: stderrors.New("unused")}, id: "5"},
		{name: "engine call acts for a user and is checked", ctx: engineCtx, vis: &fakeVisibility{}, id: "5", wantCode: codes.PermissionDenied},
		{name: "end user never reaches the crm", ctx: endUserCtx, vis: &fakeVisibility{writable: map[string]bool{"5": true}, visible: map[string]bool{"5": true}}, id: "5", wantCode: codes.PermissionDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			link, err := svc(tt.vis).CheckCRMContactAccess(tt.ctx(), tt.id)
			if isSchemaCall(tt.ctx()) && tt.vis.calls != 0 {
				t.Fatalf("schema call reached the crm %d times", tt.vis.calls)
			}

			if tt.vis.unbounded != 0 {
				t.Fatalf("%d crm calls without a deadline", tt.vis.unbounded)
			}

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if link != tt.wantLink {
					t.Fatalf("link = %v, want %v", link, tt.wantLink)
				}

				return
			}

			if errors.Code(err) != tt.wantCode {
				t.Fatalf("code = %v, want %v", errors.Code(err), tt.wantCode)
			}
		})
	}
}

type fakeAccessChecker struct {
	link bool
	err  error
}

func (f fakeAccessChecker) CheckCRMContactAccess(context.Context, string) (bool, error) {
	return f.link, f.err
}

type fakeBinder struct {
	reqs []*threadv1.SetCrmContactRequest
}

func (f *fakeBinder) SetCrmContact(_ context.Context, req *threadv1.SetCrmContactRequest) (*threadv1.SetCrmContactResponse, error) {
	f.reqs = append(f.reqs, req)

	return &threadv1.SetCrmContactResponse{CrmContactId: req.GetCrmContactId()}, nil
}

func TestSetCrmContact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ctx        func() context.Context
		access     fakeAccessChecker
		source     gtwthread.CrmContactSource
		wantCode   codes.Code
		wantLink   bool
		wantSource threadv1.CrmContactSource
	}{
		{name: "writable contact is bound and linked", ctx: operatorCtx, access: fakeAccessChecker{link: true}, source: gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_SELECTED, wantLink: true, wantSource: threadv1.CrmContactSource_CRM_CONTACT_SOURCE_SELECTED},
		{name: "read-only contact is bound, not linked", ctx: operatorCtx, access: fakeAccessChecker{}, source: gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_CREATED, wantSource: threadv1.CrmContactSource_CRM_CONTACT_SOURCE_CREATED},
		{name: "unreadable contact never reaches im-thread", ctx: operatorCtx, access: fakeAccessChecker{err: errors.Forbidden("no")}, wantCode: codes.PermissionDenied},
		{name: "no identity", ctx: context.Background, access: fakeAccessChecker{}, wantCode: codes.PermissionDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			binder := &fakeBinder{}
			th := &thread{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), crmAccess: tt.access, crmBinder: binder}

			resp, err := th.SetCrmContact(tt.ctx(), &gtwthread.SetCrmContactRequest{
				ThreadId: "22222222-2222-4222-8222-222222222222", CrmContactId: "5", Source: tt.source,
			})
			if tt.wantCode != codes.OK {
				if errors.Code(err) != tt.wantCode {
					t.Fatalf("code = %v, want %v (%v)", errors.Code(err), tt.wantCode, err)
				}

				if len(binder.reqs) != 0 {
					t.Fatal("im-thread was called")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			req := binder.reqs[0]
			if resp.GetCrmContactId() != "5" || req.GetLinkImContact() != tt.wantLink || req.GetSource() != tt.wantSource {
				t.Fatalf("request = %v, want link %v source %v", req, tt.wantLink, tt.wantSource)
			}

			// Domain and initiator come from the caller, never from the payload.
			if req.GetDomainId() != 7 || req.GetInitiatorContactId() != "11111111-1111-4111-8111-111111111111" {
				t.Fatalf("domain/initiator = %d/%s", req.GetDomainId(), req.GetInitiatorContactId())
			}
		})
	}
}

// The bound contact, and with it its source, is shown only to a caller who may
// read it; it is checked even when it is not one of the candidates.
func TestListContactCandidates_Binding(t *testing.T) {
	t.Parallel()

	selected := threadv1.CrmContactSource_CRM_CONTACT_SOURCE_SEARCHED

	tests := []struct {
		name       string
		resp       *threadv1.ListContactCandidatesResponse
		visible    map[string]bool
		wantBound  string
		wantSource gtwthread.CrmContactSource
		wantIDs    []string
		wantAsked  []string
	}{
		{
			name: "unreadable binding hidden with its source", resp: candidatesResponse(), visible: map[string]bool{"2": true},
			wantSource: gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_UNSPECIFIED, wantIDs: []string{"2"}, wantAsked: []string{"1", "2"},
		},
		{
			name:    "readable binding that is not a candidate is shown",
			resp:    &threadv1.ListContactCandidatesResponse{CrmContactId: "9", CrmContactSource: selected, Candidates: candidatesResponse().GetCandidates()},
			visible: map[string]bool{"9": true, "2": true}, wantBound: "9", wantSource: gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_SEARCHED,
			wantIDs: []string{"2"}, wantAsked: []string{"1", "2", "9"},
		},
		{
			name:    "unreadable binding that is not a candidate is hidden",
			resp:    &threadv1.ListContactCandidatesResponse{CrmContactId: "9", CrmContactSource: selected, Candidates: candidatesResponse().GetCandidates()},
			visible: map[string]bool{"1": true}, wantSource: gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_UNSPECIFIED,
			wantIDs: []string{"1"}, wantAsked: []string{"1", "2", "9"},
		},
		{
			name:    "a cleared operator choice keeps its source",
			resp:    &threadv1.ListContactCandidatesResponse{CrmContactSource: selected},
			visible: map[string]bool{}, wantSource: gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_SEARCHED, wantIDs: []string{}, wantAsked: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vis := &recordingVisibility{fakeVisibility: fakeVisibility{visible: tt.visible}}
			svc := NewContactCandidates(slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeThreadReader{}, &fakeCandidateLister{resp: tt.resp}, vis)

			resp, err := svc.ListContactCandidates(operatorCtx(), &gtwthread.ListContactCandidatesRequest{ThreadId: "22222222-2222-4222-8222-222222222222"})
			if err != nil {
				t.Fatal(err)
			}

			ids := []string{}
			for _, c := range resp.GetCandidates() {
				ids = append(ids, c.GetCrmContactId())
			}

			if resp.GetCrmContactId() != tt.wantBound || resp.GetCrmContactSource() != tt.wantSource || !slices.Equal(ids, tt.wantIDs) {
				t.Fatalf("binding %q/%v, candidates %v; want %q/%v, %v", resp.GetCrmContactId(), resp.GetCrmContactSource(), ids, tt.wantBound, tt.wantSource, tt.wantIDs)
			}

			asked := slices.Sorted(slices.Values(vis.asked))
			if !slices.Equal(asked, tt.wantAsked) {
				t.Fatalf("asked the crm about %v, want %v", asked, tt.wantAsked)
			}
		})
	}
}

// recordingVisibility records the ids the read check was asked about.
type recordingVisibility struct {
	fakeVisibility

	asked []string
}

func (r *recordingVisibility) VisibleContacts(ctx context.Context, ids []string) (map[string]bool, error) {
	r.asked = append(r.asked, ids...)

	return r.fakeVisibility.VisibleContacts(ctx, ids)
}

// A domain id outside the int32 im-thread contract is rejected before any call.
func TestListContactCandidates_DomainOutOfRange(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), auth.AuthContextKey, stubIdentity{contactID: "11111111-1111-4111-8111-111111111111", domainID: math.MaxInt32 + 1, issuer: WebitelIssuer})
	threads, lister := &fakeThreadReader{}, &fakeCandidateLister{resp: candidatesResponse()}
	svc := NewContactCandidates(slog.New(slog.NewTextHandler(io.Discard, nil)), threads, lister, &fakeVisibility{})

	_, err := svc.ListContactCandidates(ctx, &gtwthread.ListContactCandidatesRequest{ThreadId: "22222222-2222-4222-8222-222222222222"})
	if errors.Code(err) != codes.InvalidArgument || len(threads.reqs) != 0 || len(lister.reqs) != 0 {
		t.Fatalf("err %v, calls %d/%d: want InvalidArgument without calls", err, len(threads.reqs), len(lister.reqs))
	}
}
