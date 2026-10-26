# Gateway: thread source and queue

im-gateway-service part of "source and queue in active threads": read the provider gate name/type headers into `Identity.via_ref`, forward an optional call_center `queue` on AddMember / Transfer / UpgradePreviewToFullMember, and expose `Thread.source` / `Thread.queue` in the gateway API.

- **Branch:** feat/gateway-thread-source-queue
- **Plan path:** docs/dev-task-plans/gateway-thread-source-queue.md

## Acceptance criteria

- [ ] Local protos working copy (`/Users/dtsnk0/Documents/projects/protos`, branch `feat/thread-search-source-queue`) has the gateway/v1 additions: `message ThreadQueue{int64 id; string name}` (in `im/api/gateway/v1/thread_member.proto`), `message ThreadSource{string id; string name; string type}` (in `im/api/gateway/v1/thread.proto`), optional `ThreadQueue queue` on gateway `AddMemberRequest` (field 5), `TransferRequest` (field 5) and `UpgradePreviewToFullMemberRequest` (field 4), and `ThreadSource source = 20` + `ThreadQueue queue = 21` on gateway `Thread`. `GivePreviewRequest` is unchanged. Nothing in the protos repo is committed, pushed, reset or stashed.
- [ ] thread/v1 `Thread.source` / `Thread.queue` use field numbers that do not clash with `crm_contact_id = 21` on protos `main` (22/23, per Q1).
- [ ] `gen/go/thread/v1` and `gen/go/gateway/v1` are regenerated, and `git status` shows changes only under those two dirs (plus the Go sources and tests listed below). The regenerated code still contains everything the current gen has (`GetCounters`, `SetCrmContact`, `CrmContactId`, `Tags`, `RemoveTagByName`, `Snooze`, `ThreadTags`, ...). No other `gen/go/*` dir changes, and the committed `//go:generate` lines in `buf/*.go` and the `buf/buf.gen.*.yaml` templates are unchanged.
- [ ] `infra/auth/auth.go` declares `ViaTypeIdentificationHeader = "x-webitel-via-type"` and `ViaNameIdentificationHeader = "x-webitel-via-name-bin"`. For a provider call, `resolveProviderIdentity` stores the gate type and name (the name as valid UTF-8; non-ASCII names come through unchanged) on `standard.Identity`. `auth.Identifier` exposes them through `GetViaName()` / `GetViaType()`.
- [ ] `SendText`, `SendDocument` and `SendInteractive` send `From.Identity.via_ref = Via{id, name, type}` when the caller has a via id. If the name/type headers were absent, `via_ref` carries only `id`. Without a via id, `via_ref` is nil. The deprecated `Identity.via` is still set to the same gate id, for backward compatibility. `ForwardMessages` (sender identity, same via_ref rule) and `resolveContact` (recipient identity, id-only `via_ref` when the request peer has a via) also set `via_ref` next to `via`.
- [ ] `AddMember`, `Transfer` and `UpgradePreviewToFullMember` forward `queue{id,name}` to the thread/v1 request when the gateway request has a queue with `id > 0`. Otherwise the thread/v1 `queue` stays nil. `GivePreview` is not touched.
- [ ] `convertToThread` maps thread/v1 `Thread.source` → gateway `Thread.source` and `Thread.queue` → gateway `Thread.queue`, and leaves nil input as nil output. This covers Search, Get, SearchLeft and GetUpdates, which all go through `convertToThread`.
- [ ] Unit tests cover: header reading (all three headers / id only / non-ASCII name / invalid UTF-8 name); identity → `via_ref` building (full / id only / no via); queue forwarding for each of the three RPC request builders (set / nil / id 0); `convertToThread` mapping of source/queue (set / unset).
- [ ] `go build ./...` and `go test ./...` pass from the repo root.

## Codebase notes

- **No CLAUDE.md, docs/knowledge or graphify-out** in this repo. The README is empty. Commands: `go build ./...` and `go test ./...` from the repo root (`/Users/dtsnk0/.superset/worktrees/im-gateway-service/gelatinous-nest-0b53252e`). Both are green on the branch's base commit (verified during planning). Go 1.25 (`go.mod`).
- **Lint:** `.golangci.yaml` (synced from webitel/reusable-configs) enables `staticcheck`, `protogetter` (use `x.GetFoo()`, not `x.Foo`, when reading proto fields), `nlreturn`/`wsl_v5` (blank line before `return` after a block), `gci`/`gofumpt` import grouping. `golangci-lint` is installed at `/opt/homebrew/bin/golangci-lint`. The existing code does not pass lint everywhere, so lint is not a gate here, but new code should follow these rules. Once thread/v1 is regenerated, `Identity.Via` becomes `[deprecated = true]`. Every remaining write of that field needs a `//nolint:staticcheck // legacy field kept until every im-thread-service reads via_ref` comment on the same line, the way im-thread-service does it.
- **Layering:** `internal/handler/grpc/*` passes gateway protos straight to `internal/service/*` (for example `handler/grpc/thread.go:74` `AddMember` → `s.threadManager.AddMember(ctx, req)`, `handler/grpc/thread_preview.go:44-48` → `s.previewer.UpgradePreviewToFullMember(ctx, req)`). No handler changes are needed. The service layer converts gateway protos (`gen/go/gateway/v1`, package `api`, imported as `gtwthread` / `gtwpreview` / `api`) into thread/v1 protos (`gen/go/thread/v1`, imported as `threadv1`). `infra/auth` must stay free of generated-proto imports: it only imports `webitel-go-kit/pkg/errors` (the `standard` sub-package imports contact gen only for its client call).
- **Thread clients are concrete and not mockable:** `*imthread.ThreadClient`, `*imthread.ThreadPreviewClient` and `*imcontact.Client` wrap `webitel-go-kit` discovery RPC clients (`infra/client/im-thread/im_thread_manager.go:17-21`, `thread_preview.go:16-21`). Service methods that call them cannot be unit-tested without a live service. The existing tests only test pure functions (`Test_convertToThread`, `Test_readCallerID`, `Test_convertToThreadPreview`). Put new logic in pure helper functions and test those.
- **Error style:** `github.com/webitel/webitel-go-kit/pkg/errors` (`errors.InvalidArgument(...)`, `errors.New(...)`). This task adds no new error paths.
- **Proto generation:** `buf/gen_thread.go` / `buf/gen_gateway.go` contain `//go:generate buf generate ../../protos/im --template buf.gen.<x>.yaml --path ../../protos/im/...`. That relative path does not resolve from this worktree, so do **not** use `go generate`. Run `buf` (`/opt/homebrew/bin/buf`, v1.59.0) by hand **from the repo's `buf/` directory** (the templates write to `../gen/go/<x>`) with an absolute input path. A positional input overrides the template's `inputs: git_repo: ... branch: main`. Templates use remote plugins `buf.build/protocolbuffers/go:v1.34.1` + `buf.build/grpc/go:v1.5.1`, so network access to buf.build is required (it worked during planning). Before merge, gen/go must be regenerated from protos `main` once the protos PR is merged. That is out of scope here; just do not "fix" anything for it.
- **The protos checkout is behind protos `main`.** Branch `feat/thread-search-source-queue` is based on `935983c6`, while `origin/main` is `3ad8c3ba`. main has since added, among others, thread/v1 `Thread.tags = 17` and `Thread.crm_contact_id = 21`, `GetCounters`, `SetCrmContact`, tag Snooze/RemoveTagByName, `ThreadTags`, and gateway `Thread.crm_contact_id = 18` / `tags = 19`. **This repo's current gen/go already contains all of that.** Regenerating straight from the local checkout deletes those symbols and breaks the build. The planner verified this: a trial regen from the local checkout dropped `GetCounters`, `SetCrmContact` and the tag RPCs from `gen/go/thread`. A trial regen from `git archive origin/main` + the local thread/v1 diff (with source/queue renumbered to 22/23) changed only `peer.pb.go`, `shared.pb.go`, `thread_preview_service.pb.go` and `thread_service.pb.go` in `gen/go/thread/v1`, and left `gen/go/gateway` byte-identical. See Q1/Q2.
- **Release order / compatibility:** protos → im-thread-service → gateway → providers → call_center. Every new field is optional. Absent headers or absent queue must behave exactly as today.
- **im-thread-service counterpart** (branch `feat/thread-search-source-queue`, worktree `/Users/dtsnk0/.superset/worktrees/im-thread-service/cedar-chips-97d85d4d`, read-only reference):
  - `internal/handler/grpc/mapper/text_message.go:91-105` `mapViaInfoFromProto` reads `via_ref`, falls back to the deprecated `via` when `via_ref.id` is empty, and `via_ref.id` wins on conflict. So sending both is safe.
  - `internal/handler/grpc/mapper/thread.go:127-133` `convertQueueInfo` treats `q == nil || q.GetId() <= 0` as "no queue". The gateway uses the same rule.
  - Its plan's decisions: GivePreview does not change; Search returns `source`/`queue` by default, Get does not compute `queue`; `source` is returned only when a source ref row exists.

## Decisions

### Plan review (user)

- Send both `via_ref` and deprecated `via` on provider calls during the transition (as planned).
- Every call that sets `via` today also sets `via_ref`, including `ForwardMessages` and the recipient identity in `resolveContact` (id-only `via_ref` there).
- **Changed after implementation (user):** the queue comes into the gateway in the `x-webitel-queue-id` / `x-webitel-queue-name-bin` headers, not as a `queue` field on the gateway/v1 AddMember/Transfer/UpgradePreviewToFullMember requests (those fields were removed from protos). An id that is missing, non-numeric or `<= 0` is logged and dropped; the call still succeeds. Sections below that describe the request field are superseded by `docs/features/gateway-thread-source-queue.md`.

### Q1: Thread/v1 `Thread.source = 21` collides with `Thread.crm_contact_id = 21` on protos `main`. Renumber `source`/`queue` to 22/23?

- **Context:** The uncommitted local change in `/Users/dtsnk0/Documents/projects/protos/im/service/thread/v1/thread_service.proto` adds `ThreadSource source = 21; ThreadQueue queue = 22;` to `message Thread`. On `origin/main`, `Thread` already ends with `string crm_contact_id = 21;`, and this repo's `gen/go/thread/v1/thread_service.pb.go:316` already has `CrmContactId ... bytes,21`. The im-thread-service branch (`cedar-chips-97d85d4d/gen/go/thread/v1/thread_service.pb.go:382-384`) was generated with `source=21`, `queue=22` too, so it is affected the same way. The other new thread/v1 numbers do not clash with main: `Identity.via_ref=5`, `AddMemberRequest.queue=8`, `TransferRequest.queue=7`, `UpgradePreviewToFullMemberRequest.queue=6`. The gateway/v1 numbers in this plan are already chosen against main (`Thread.source=20`, `queue=21`, since main uses 18/19).
- **Options:** (a) Renumber thread/v1 to `source = 22`, `queue = 23` in the local protos working copy. This plan's subtask makes that edit, and im-thread-service must regenerate its gen/go to match. (b) The user renumbers by hand in the protos repo before implementation starts (any other numbers), and the subtask only reads them.
- **Answer:** (a). Renumber thread/v1 to `source = 22`, `queue = 23` in the local protos working copy. im-thread-service regenerates its gen/go to match, outside this task.

### Q2: How should gen/go be regenerated, given the local protos branch is behind protos `main`?

- **Context:** The task says to regenerate from the local checkout. Doing that removes `GetCounters`, `SetCrmContact`, `CrmContactId`, `Tags`, the tag RPCs and `ThreadTags` from gen/go. The gateway code already uses them (`internal/service/thread.go:402` `GetCounters`, `:306` `SetCrmContact`, `:780-781` `CrmContactId`/`Tags`), so `go build` would fail. The task forbids committing, pushing or resetting in the protos repo, and `git merge origin/main` would refuse anyway, because `thread_service.proto` is modified locally and changed upstream.
- **Options:** (a) The user brings the protos branch up to date with `origin/main` first (for example a WIP commit, then rebase/merge, then resolve the `Thread` hunk conflict). The dev agent then edits and regenerates straight from `/Users/dtsnk0/Documents/projects/protos/im` as the task describes. (b) The protos branch's git state stays as is. The dev agent makes the proto edits in the local working copy, then regenerates from a throwaway scratch tree: `git archive origin/main im` + `git diff` of the working copy applied with `patch`, with the conflicting `Thread` hunks re-applied by hand. The planner verified this produces only the expected gen changes. Local proto files and the generated code then use the same field numbers but different bases.
- **Answer:** (b). The protos branch's git state is not touched. Edit the local working copy and regenerate from a throwaway tree: `git archive origin/main` + the local diff, with the `Thread` hunk re-applied by hand as 22/23.

## Subtasks

### Subtask A: Protos, regeneration and gateway source/queue/via_ref wiring

**Blocked by:** Q1, Q2

#### Instructions

This is one subtask on purpose. Every Go change compiles only against the regenerated `gen/go/thread` / `gen/go/gateway`, and `message.go` depends on the new `auth.Identifier` methods, so the work cannot be split into separately buildable pieces. Do the steps in order. Paths are relative to the repo root `/Users/dtsnk0/.superset/worktrees/im-gateway-service/gelatinous-nest-0b53252e` unless absolute. `PROTOS=/Users/dtsnk0/Documents/projects/protos`.

**Part 1: proto edits in the local protos working copy.** Do not run `git commit`, `git stash`, `git reset`, `git checkout -- <file>`, `git push`, or a merge/rebase in `$PROTOS`. Only edit files.

1. (Q1 answer: (a).) In `$PROTOS/im/service/thread/v1/thread_service.proto`, `message Thread` (around lines 183-187 in the working copy), change `ThreadSource source = 21;` → `ThreadSource source = 22;` and `ThreadQueue queue = 22;` → `ThreadQueue queue = 23;`. Keep the comments. Do not add `crm_contact_id` in the local file (the local base predates it). The numbering only has to be right against main.
2. In `$PROTOS/im/api/gateway/v1/thread_member.proto`, append after `message ThreadMember { ... }`:
   ```proto

   // ThreadQueue is a call_center queue through which an agent received a thread.
   message ThreadQueue {
     // Queue id.
     int64 id = 1;
     // Queue display name.
     string name = 2;
   }
   ```
   It goes in `thread_member.proto` because both `thread.proto` and `thread_preview.proto` already import that file. Do not name anything `Via`: gateway/v1 already has `message Via` in `via.proto:82`.
3. In `$PROTOS/im/api/gateway/v1/thread.proto`:
   - Immediately above `// Thread represents a thread (aka chat or conversation) entity.` (line ~153), add:
     ```proto
     // ThreadSource is the gate (im-providers) a thread arrived through.
     message ThreadSource {
       // Gate id.
       string id = 1;
       // Gate display name.
       string name = 2;
       // Provider type, e.g. facebook | whatsapp | telegram_bot.
       string type = 3;
     }

     ```
   - At the end of `message Thread`, after `string updates_cursor = 17;`, add the lines below. Numbers 20/21 are deliberate: protos main already uses 18 (`crm_contact_id`) and 19 (`tags`) here, and this repo's gen has them.
     ```proto

       // Source gate of the thread (the external member's via). Unset when unknown.
       ThreadSource source = 20;

       // Queue through which the calling participant received the thread. Set by Search; unset when unknown.
       ThreadQueue queue = 21;
     ```
   - In `message AddMemberRequest` (line ~366), after the `role = 4` field:
     ```proto

       // Optional call_center queue through which the new member receives the thread.
       ThreadQueue queue = 5;
     ```
   - In `message TransferRequest` (line ~412), after `optional PeerIdentity initiator = 4;`:
     ```proto

       // Optional call_center queue through which the target receives the thread. Empty -> no queue.
       ThreadQueue queue = 5;
     ```
4. In `$PROTOS/im/api/gateway/v1/thread_preview.proto`, `message UpgradePreviewToFullMemberRequest` (line ~75), after the `role = 3` field:
   ```proto

     // Optional call_center queue through which the member receives the thread.
     ThreadQueue queue = 4;
   ```
   Do **not** touch `GivePreviewRequest`.
5. Sanity check: `cd $PROTOS/im && buf build` must succeed (the module in `$PROTOS/im/buf.yaml` has no lint gate here).

**Part 2: regenerate gen/go (thread + gateway only).**

6. (Q2 answer: (b).) Build a throwaway tree from protos main plus the local diff. Use a fresh `mktemp -d` dir, never inside either repo:
   ```sh
   SP=$(mktemp -d); PROTOS=/Users/dtsnk0/Documents/projects/protos
   git -C $PROTOS archive origin/main im | tar -x -C $SP
   git -C $PROTOS diff > $SP/local.patch
   cd $SP && patch -p1 --no-backup-if-mismatch -r - < $SP/local.patch
   ```
   Expect two hunks to fail, because main has extra fields at the end of `message Thread`: the `Thread` hunk in `im/service/thread/v1/thread_service.proto` and the `Thread` hunk in `im/api/gateway/v1/thread.proto`. Insert those by hand in `$SP/im/...`:
   - In `$SP/im/service/thread/v1/thread_service.proto`, after `string crm_contact_id = 21;` in `message Thread`, add `ThreadSource source = 22;` and `ThreadQueue queue = 23;` with the same comments as the local file.
   - In `$SP/im/api/gateway/v1/thread.proto`, after `repeated ChatTag tags = 19;` in `message Thread`, add `ThreadSource source = 20;` and `ThreadQueue queue = 21;` with comments.

   Then check: `grep -n "ThreadSource\|ThreadQueue\|via_ref\|message Via" -r $SP/im/service/thread/v1 $SP/im/api/gateway/v1` must show every addition from Part 1 (the thread/v1 ones were already in the local diff: `Identity.via_ref = 5` + `message Via` in `peer.proto`, `message ThreadQueue` in `shared.proto`, `AddMemberRequest.queue = 8`, `TransferRequest.queue = 7`, `UpgradePreviewToFullMemberRequest.queue = 6`, `message ThreadSource`). Also confirm `ls $SP/im/service/thread/v1/*.rej $SP/im/api/gateway/v1/*.rej` finds nothing, or delete the `.rej` files after fixing.

   If the user answered Q2 with (a) (branch already up to date with main), skip this step and use `$PROTOS/im` as `$IN` below.
7. Generate into the repo, from the repo's `buf/` dir:
   ```sh
   IN=$SP/im   # or /Users/dtsnk0/Documents/projects/protos/im for Q2 (a)
   cd /Users/dtsnk0/.superset/worktrees/im-gateway-service/gelatinous-nest-0b53252e/buf
   buf generate $IN --template buf.gen.thread.yaml  --path $IN/service/thread/
   buf generate $IN --template buf.gen.gateway.yaml --path $IN/api/gateway
   ```
8. Verify with `git status --short gen/`. Expected changes: `gen/go/thread/v1/{peer,shared,thread_preview_service,thread_service}.pb.go` and `gen/go/gateway/v1/{thread,thread_member,thread_preview}.pb.go` (a few other gateway descriptor files may change only if they import these). There must be no deletions and no changes under `gen/go/{admin,auth,contact,provider,storage}`. `grep -c "GetCounters\|SetCrmContact" gen/go/thread/v1/thread_service_grpc.pb.go` must be non-zero. If anything unexpected changed, stop and fix the input tree instead of committing it. Do not edit `buf/*.go` or `buf/*.yaml`.
9. `go build ./...` must pass at this point, before any code changes.

**Part 3: via_ref from provider headers.**

10. `infra/auth/auth.go`: in the `const` block with `ViaIdentificationHeader = "x-webitel-via"` (line 25), add:
    ```go
    // ViaTypeIdentificationHeader carries the provider type of the gate (facebook | whatsapp | telegram_bot | ...).
    ViaTypeIdentificationHeader = "x-webitel-via-type"
    // ViaNameIdentificationHeader carries the gate display name. Binary (-bin) metadata:
    // gRPC base64-encodes it on the wire, so the value read here is already the raw bytes.
    ViaNameIdentificationHeader = "x-webitel-via-name-bin"
    ```
    Extend `type Identifier interface` (line 40) with `GetViaName() string` and `GetViaType() string`, right after `GetViaPtr() *string`.
11. `infra/auth/standard/auth.go`:
    - Add fields `ViaName string` and `ViaType string` to `type Identity struct` (lines 43-51), after `Via string`.
    - Add methods after `GetViaPtr` (line ~88): `func (i *Identity) GetViaName() string { return i.ViaName }` and `func (i *Identity) GetViaType() string { return i.ViaType }`.
    - Add a pure helper under `// --- Internal Helpers ---` (next to `getHeader`):
      ```go
      // providerVia reads the gate headers im-providers-service sends with every call:
      // the gate id, its provider type and its display name. Old providers send only the id.
      // The name comes from a binary header and may hold arbitrary bytes; it is coerced to
      // valid UTF-8 because proto3 string fields reject invalid UTF-8 on marshal.
      func providerVia(md metadata.MD) (id, name, typ string) {
          id = getHeader(md, interfaces.ViaIdentificationHeader)
          name = strings.ToValidUTF8(getHeader(md, interfaces.ViaNameIdentificationHeader), "�")
          typ = getHeader(md, interfaces.ViaTypeIdentificationHeader)

          return id, name, typ
      }
      ```
      `strings` is already imported.
    - In `resolveProviderIdentity` (lines 175-210), call `viaID, viaName, viaType := providerVia(md)` once near the top, after the provider header checks. Use `viaID` for the `SearchContactRequest.Via` field (currently `getHeader(md, interfaces.ViaIdentificationHeader)` at line 193), and set `Via: viaID, ViaName: viaName, ViaType: viaType` in the returned `&Identity{...}` (line 204-209). Leave `SetIdentity` (which stores the via id under `ViaContextKey`), the schema and engine branches, and `resolveUserIdentity` unchanged.
12. `internal/service/message.go`: add a package-level helper, for example just above `func (m *MessageService) resolveRecipient` (line ~554):
    ```go
    // senderIdentity builds the thread/v1 identity of the calling sender. The gate is sent both
    // as via_ref{id,name,type} and as the deprecated via (id only) so im-thread versions that
    // predate via_ref keep working; im-thread prefers via_ref.id when both are set.
    func senderIdentity(identity auth.Identifier) *threadv1.Identity {
        return &threadv1.Identity{
            Name:     identity.GetName(),
            ChatName: identity.GetChatName(),
            Via:      identity.GetViaPtr(), //nolint:staticcheck // legacy field kept until every im-thread-service reads via_ref
            ViaRef:   toThreadVia(identity),
        }
    }

    // toThreadVia returns the gate the caller came through, or nil when it has no via id.
    // name/type are empty for providers that do not send the new headers yet.
    func toThreadVia(identity auth.Identifier) *threadv1.Via {
        if identity.GetVia() == "" {
            return nil
        }

        return &threadv1.Via{Id: identity.GetVia(), Name: identity.GetViaName(), Type: identity.GetViaType()}
    }
    ```
    Then replace the three `Identity: &threadv1.Identity{ Name: identity.GetName(), ChatName: identity.GetChatName(), Via: identity.GetViaPtr(), }` literals with `Identity: senderIdentity(identity),`:
    - `SendInteractive`: `converted.From = &threadv1.Peer{... Identity: ...}` at lines 120-127.
    - `SendText`: `From: &threadv1.Peer{...}` at lines 253-260.
    - `SendDocument`: `From: &threadv1.Peer{...}` at lines 308-315.

    Do not change the other `threadv1.Identity` literals (SendContact :73, SendInteractiveCallback :152, SendLocation :187, SendSystemMessage :350, EditMessage :435, DeleteMessages :465, SetReaction :502, SendTyping :533): they do not set via today. Per the user's decision, every call that sets `via` today must also set `via_ref` (both fields are sent, as in `senderIdentity`):
    - `message_forward.go:29-35` (`ForwardMessages`, sender `From` identity with `Name` + `Via: identity.GetViaPtr()`): set `ViaRef: toThreadVia(identity)` next to `Via` (keep `Via` with the `//nolint:staticcheck // legacy field kept until every im-thread-service reads via_ref` comment). Do not add `ChatName` there — keep the existing fields otherwise unchanged.
    - `resolveContact` (line ~615, `Via: via,` on the *recipient* identity; `via *string` comes from the request peer, so only the gate id is known): keep `Via: via,` with the same `//nolint:staticcheck` comment and add `ViaRef: viaRefFromID(via)`, where `viaRefFromID(via *string) *threadv1.Via` is a small package-level helper next to `toThreadVia` that returns nil when `via` is nil or empty and `&threadv1.Via{Id: *via}` otherwise (id only, no name/type).
13. `internal/service/thread_test.go`: the fake `readCallerIdentity` (lines 404-415) implements `auth.Identifier` and stops compiling after step 10. Add `func (i readCallerIdentity) GetViaName() string { return "" }` and `func (i readCallerIdentity) GetViaType() string { return "" }`, aligned with the existing one-liners.

**Part 4: queue forwarding (AddMember, Transfer, UpgradePreviewToFullMember).**

14. `internal/service/thread.go`: add next to `convertThreadTags` (line ~787):
    ```go
    // toThreadQueue forwards the optional call_center queue of a member/preview request.
    // nil or a non-positive id means "no queue" (same rule as im-thread's convertQueueInfo).
    func toThreadQueue(q *gtwthread.ThreadQueue) *threadv1.ThreadQueue {
        if q.GetId() <= 0 {
            return nil
        }

        return &threadv1.ThreadQueue{Id: q.GetId(), Name: q.GetName()}
    }
    ```
15. In the same file, pull the request construction out of the RPCs into pure builders so they can be unit-tested. Keep existing behaviour byte-for-byte and add `Queue`:
    - `AddMember` (lines 134-180): replace the `addMemberRequest := &threadv1.AddMemberRequest{...}` literal and the following `InitiatorContactId`/`SystemCall` assignments (lines 155-170) with `addMemberRequest := newAddMemberRequest(req, target.GetId(), identity, auth.IsSystemCall(ctx))`, where
      ```go
      func newAddMemberRequest(req *gtwthread.AddMemberRequest, newMemberContactID string, identity auth.Identifier, systemCall bool) *threadv1.AddMemberRequest
      ```
      sets `ThreadId: req.GetThreadId()`, `NewMemberContactId`, `Role: threadv1.ThreadRole(req.GetRole())`, `DomainId: int32(identity.GetDomainID())`, `InitiatorContactId: &initiatorContactID` (= `identity.GetContactID()`), `SystemCall: &systemCall`, `Queue: toThreadQueue(req.GetQueue())`. Move the existing explanatory comment ("Always record who acted as the message sender...") into the builder.
    - `Transfer` (lines 182-234): replace the `transferRequest := &threadv1.TransferRequest{...}` literal (lines 217-223) with `transferRequest := newTransferRequest(req, target, initiatorContactId)`, where
      ```go
      func newTransferRequest(req *gtwthread.TransferRequest, target *contact.Contact, initiatorContactID string) *threadv1.TransferRequest
      ```
      sets `ThreadId`, `NewMemberContactId: target.GetId()`, `Role: threadv1.ThreadRole(req.GetRole())`, `InitiatorContactId: initiatorContactID`, `TargetIsBot: target.GetIsBot()`, `Queue: toThreadQueue(req.GetQueue())`. `contact` is the already-imported `github.com/webitel/im-gateway-service/gen/go/contact/v1` package alias used by `fetchContact`. Leave the system-call initiator resolution above it as is.
16. `internal/service/thread_preview.go`, `UpgradePreviewToFullMember` (lines 99-130): replace the inline `&threadv1.UpgradePreviewToFullMemberRequest{...}` (lines 113-119) with `newUpgradePreviewRequest(req, target.GetId(), identity.GetContactID(), domainID)`, where
    ```go
    func newUpgradePreviewRequest(req *gtwpreview.UpgradePreviewToFullMemberRequest, contactID, initiatorContactID string, domainID int32) *threadv1.UpgradePreviewToFullMemberRequest
    ```
    sets `ThreadId`, `ContactId`, `InitiatorContactId`, `DomainId`, `Role: threadv1.ThreadRole(req.GetRole())`, `Queue: toThreadQueue(req.GetQueue())`. `toThreadQueue` is in the same package and `gtwpreview` is the same Go package as `gtwthread`. Do not change `GivePreview` or `RemoveFromPreview`.

**Part 5: source/queue in convertToThread.**

17. `internal/service/thread.go`, `convertToThread` (lines 723-785): in the returned `&gtwthread.Thread{...}` literal, after `Tags: convertThreadTags(thr.GetTags()),`, add
    ```go
    Source: convertThreadSource(thr.GetSource()),
    Queue:  convertThreadQueue(thr.GetQueue()),
    ```
    and define next to `toThreadQueue`:
    ```go
    // convertThreadSource maps the gate the thread arrived through; nil stays nil.
    func convertThreadSource(s *threadv1.ThreadSource) *gtwthread.ThreadSource {
        if s == nil {
            return nil
        }

        return &gtwthread.ThreadSource{Id: s.GetId(), Name: s.GetName(), Type: s.GetType()}
    }

    // convertThreadQueue maps the queue through which the caller received the thread; nil stays nil.
    func convertThreadQueue(q *threadv1.ThreadQueue) *gtwthread.ThreadQueue {
        if q == nil {
            return nil
        }

        return &gtwthread.ThreadQueue{Id: q.GetId(), Name: q.GetName()}
    }
    ```
    Nothing else changes. `Search` (:474), `SearchLeft` (:526), `Get` (:559), `Create` (:119) and `updates.go:117` (GetUpdates `Dialog`) all call `convertToThread`, and the handlers pass `*gtwthread.Thread` through unchanged (`handler/grpc/mapper/message_history.go:186` `Dialog: t.Dialog`).

**Part 6: tests.**

18. Create `infra/auth/standard/auth_test.go` (`package standard`, the first test file in this package). Table-test `providerVia` with `metadata.Pairs(...)` (keys are lowercased by `Pairs`):
    - all three headers: `x-webitel-via`="gate-1", `x-webitel-via-type`="telegram_bot", `x-webitel-via-name-bin`="Support" → ("gate-1","Support","telegram_bot");
    - id only (old provider) → ("gate-1","","");
    - non-ASCII name, for example `"Підтримка 💬"` → returned unchanged;
    - invalid UTF-8 name `"bad\xffname"` → `"bad�name"`;
    - no headers → ("","","").

    Also add a tiny test that `(&Identity{Via: "g", ViaName: "n", ViaType: "t"})` returns those values from `GetVia`/`GetViaName`/`GetViaType`.
19. Create `internal/service/message_identity_test.go` (`package service`). Test `senderIdentity` with `&standard.Identity{...}` (import `github.com/webitel/im-gateway-service/infra/auth/standard`, as `thread_preview_test.go:88` already does). Compare with `proto.Equal` from `google.golang.org/protobuf/proto`, as `internal/handler/grpc/contact_test.go:144` does:
    - full: `Via:"gate-1", ViaName:"Підтримка", ViaType:"facebook", Name:"N", ChatName:"C"` → `Identity{Name:"N", ChatName:"C", Via:ptr("gate-1"), ViaRef:&threadv1.Via{Id:"gate-1",Name:"Підтримка",Type:"facebook"}}`;
    - id only → `ViaRef:&threadv1.Via{Id:"gate-1"}`, `Via:ptr("gate-1")`;
    - no via (end-user call) → `Via:nil`, `ViaRef:nil`.
20. Extend `internal/service/thread_test.go`:
    - In `Test_convertToThread` (lines 77-186), add a case whose `thr` has `Source: &threadv1.ThreadSource{Id:"gate-1",Name:"Support",Type:"facebook"}`, `Queue: &threadv1.ThreadQueue{Id:7,Name:"Sales"}`, `Members: []*threadv1.ThreadMember{}`, and whose `want` has the matching `gtwthread.ThreadSource` / `gtwthread.ThreadQueue` and `Members: []*gtwthread.ThreadMember{}`. The existing cases already cover "unset in → unset out" (their `want` has no Source/Queue), so keep them as they are. The table compares with `reflect.DeepEqual`, so build expected values as fresh literals.
    - Add `Test_toThreadQueue`: nil → nil; `&gtwthread.ThreadQueue{}` → nil; `{Id:0,Name:"x"}` → nil; `{Id:7,Name:"Sales"}` → `{Id:7,Name:"Sales"}`.
    - Add `Test_newAddMemberRequest` and `Test_newTransferRequest`, each with queue set (`Queue` forwarded) and queue nil (`GetQueue()==nil`). Also assert the existing fields: AddMember `InitiatorContactId`/`SystemCall`/`DomainId` taken from the `readCallerIdentity` fake (`GetDomainID()` returns 1); Transfer `TargetIsBot` from `&contactv1.Contact{Id:"c", IsBot:true}`.
21. Extend `internal/service/thread_preview_test.go` with `Test_newUpgradePreviewRequest` (queue set / nil).
22. Run `go build ./...` and `go test ./...` from the repo root. Optionally run `golangci-lint run ./infra/auth/... ./internal/service/...` and fix only issues in lines you added.

#### Files

- `/Users/dtsnk0/Documents/projects/protos/im/service/thread/v1/thread_service.proto` — **edit** — `message Thread`: renumber the uncommitted `ThreadSource source = 21` / `ThreadQueue queue = 22` to 22/23 (Q1). Leave everything else as is (`ThreadSource` message, `AddMemberRequest.queue = 8`, `TransferRequest.queue = 7` are already present).
- `/Users/dtsnk0/Documents/projects/protos/im/service/thread/v1/peer.proto` — **read** — already has `Identity.via` `[deprecated = true]`, `Via via_ref = 5`, `message Via {string id=1; string name=2; string type=3;}`. Generated as `threadv1.Via`, `Identity.ViaRef`.
- `/Users/dtsnk0/Documents/projects/protos/im/service/thread/v1/shared.proto` — **read** — already has `message ThreadQueue { int64 id = 1; string name = 2; }` → `threadv1.ThreadQueue`.
- `/Users/dtsnk0/Documents/projects/protos/im/service/thread/v1/thread_preview_service.proto` — **read** — already has `UpgradePreviewToFullMemberRequest.queue = 6`.
- `/Users/dtsnk0/Documents/projects/protos/im/api/gateway/v1/thread_member.proto` — **edit** — append `message ThreadQueue {int64 id = 1; string name = 2;}` after `message ThreadMember` (file is 27 lines, imports `contact.proto`, `thread_permission.proto`).
- `/Users/dtsnk0/Documents/projects/protos/im/api/gateway/v1/thread.proto` — **edit** — add `message ThreadSource` above `message Thread` (line 154). Add `source = 20`, `queue = 21` at the end of `Thread` (after `updates_cursor = 17`), `ThreadQueue queue = 5` to `AddMemberRequest` (line 366; fields 1,3,4) and `ThreadQueue queue = 5` to `TransferRequest` (line 412; fields 1-4). The file's comment style is `// Sentence.` above each field, with a blank line between fields in `Thread`.
- `/Users/dtsnk0/Documents/projects/protos/im/api/gateway/v1/thread_preview.proto` — **edit** — `UpgradePreviewToFullMemberRequest` (line 75; fields 1-3): add `ThreadQueue queue = 4`. `GivePreviewRequest` (line 49) is NOT changed.
- `buf/buf.gen.thread.yaml` — **read** — output `../gen/go/thread`, `go_package` override `.../gen/go/thread/v1;thread`. Run `buf generate` from `buf/` so the relative `out` resolves into the repo. Do not edit.
- `buf/buf.gen.gateway.yaml` — **read** — output `../gen/go/gateway`, package `api`. Do not edit.
- `buf/gen_thread.go`, `buf/gen_gateway.go` — **read** — committed `//go:generate` lines with `../../protos/im`. Do not edit and do not run `go generate`.
- `gen/go/thread/v1/*.pb.go` — **edit (generated)** — regenerated by buf only. Expected diffs: `peer.pb.go`, `shared.pb.go`, `thread_preview_service.pb.go`, `thread_service.pb.go`.
- `gen/go/gateway/v1/*.pb.go` — **edit (generated)** — regenerated by buf only. Expected diffs: `thread.pb.go`, `thread_member.pb.go`, `thread_preview.pb.go`.
- `infra/auth/auth.go` — **edit** — header constants in the `const` block at lines 16-26 (`ViaIdentificationHeader = "x-webitel-via"`). `type Identifier interface` (lines 40-49): add `GetViaName() string`, `GetViaType() string`.
- `infra/auth/standard/auth.go` — **edit** — `type Identity struct` (43-51): add `ViaName`, `ViaType`. Add getters after `GetViaPtr` (81-88). New helper `providerVia` next to `getHeader` (310-316). `resolveProviderIdentity` (175-210) fills the new fields. `var _ interfaces.Identifier = (*Identity)(nil)` at line 41 enforces the interface.
- `infra/auth/standard/auth_test.go` — **create** — tests for `providerVia` and the getters (package `standard`). Mirror the table style of `infra/auth/auth_test.go`.
- `internal/service/message.go` — **edit** — add `senderIdentity` / `toThreadVia` / `viaRefFromID`. Use `senderIdentity` in `SendInteractive` (identity literal at 122-126), `SendText` (255-259) and `SendDocument` (310-314). In `resolveContact` (~615) keep `Via: via,` (with `//nolint:staticcheck`) and add `ViaRef: viaRefFromID(via)`; cover `viaRefFromID` (nil / empty / set) in a unit test.
- `internal/service/message_forward.go` — **edit** — add `ViaRef: toThreadVia(identity)` to the `ForwardMessages` sender identity (line ~33) and a `//nolint:staticcheck` comment on the kept `Via: identity.GetViaPtr(),`.
- `internal/service/message_identity_test.go` — **create** — `senderIdentity` tests (full / id only / none).
- `internal/service/thread.go` — **edit** — `AddMember` (134-180) → `newAddMemberRequest`. `Transfer` (182-234) → `newTransferRequest`. New `toThreadQueue`, `convertThreadSource`, `convertThreadQueue` near `convertThreadTags` (787). `convertToThread` return literal (768-784) gets `Source`/`Queue`.
- `internal/service/thread_preview.go` — **edit** — `UpgradePreviewToFullMember` (99-130) → `newUpgradePreviewRequest` with `Queue`. `GivePreview` (37-68) is untouched.
- `internal/service/thread_test.go` — **edit** — add `GetViaName`/`GetViaType` to `readCallerIdentity` (404-415). New `Test_convertToThread` case. New `Test_toThreadQueue`, `Test_newAddMemberRequest`, `Test_newTransferRequest`. The imports already include `contactv1`, `gtwthread`, `threadv1`, `auth`.
- `internal/service/thread_preview_test.go` — **edit** — add `Test_newUpgradePreviewRequest`. The file already imports `standard`, `auth`, `gtwpreview`, `threadv1`.
- `internal/service/updates.go` — **read** — line 117 `t.Dialog = convertToThread(t.RawDialog, byID)`: GetUpdates gets source/queue for free.
- `internal/handler/grpc/thread.go`, `internal/handler/grpc/thread_preview.go` — **read** — pass gateway requests through unchanged (`AddMember` :74, `Transfer` :127, `UpgradePreviewToFullMember` :44). No edits.

#### Context

- **Why `infra/auth` exposes strings, not a proto `Via`:** `infra/auth` has no thread-gen import today, and `auth.Identifier` is a transport-neutral interface. The thread/v1 `Via` is built in the service layer (`toThreadVia`), next to where the existing `threadv1.Identity` literals live.
- **`-bin` header semantics:** grpc-go base64-decodes `*-bin` metadata on receive, so `md.Get("x-webitel-via-name-bin")[0]` is the raw UTF-8 bytes as a Go string. In a unit test, `metadata.Pairs("x-webitel-via-name-bin", "Підтримка")` simulates exactly that (no base64 in the test). The UTF-8 coercion matters because protobuf-go `proto.Marshal` returns an error for invalid UTF-8 in a proto3 `string` field. Without it, a bad byte from a provider would fail every `SendText` from that gate.
- **Header absent → empty strings.** `getHeader` returns "" for missing keys, so old providers produce `ViaName == ""`, `ViaType == ""` and `via_ref = {id}` only. Schema/engine/user identities never get the new fields: only `resolveProviderIdentity` sets them, and only the provider path sets `Via` today.
- **Sending both `via` and `via_ref`:** this is the planner's decision, not an open question. im-thread-service's `mapViaInfoFromProto` treats `via_ref.id` as authoritative and falls back to `via`. An im-thread-service build that predates via_ref ignores field 5 and keeps using `via`. Sending both is therefore safe in every rollout order. Do not drop `Via` from `senderIdentity`.
- **`via_ref` is nil when there is no via id.** A name/type without an id is meaningless for the thread (`source_ref` is keyed by gate id).
- **Queue "empty" rule:** `q.GetId() <= 0` → nil. This is nil-safe because proto getters handle a nil receiver, and it matches im-thread's `convertQueueInfo`, so a `{id:0,name:"x"}` queue is dropped in both services consistently. `Thread.queue` in responses is mapped as-is (nil → nil), with no id filtering.
- **Request builders:** the only reason to extract `newAddMemberRequest` / `newTransferRequest` / `newUpgradePreviewRequest` is testability, because the services hold concrete discovery clients (see Codebase notes). Keep `fetchContact` / `resolveContact` calls and the `auth.IsSystemCall` / initiator logic in the RPC methods. Pass their results into the builders. While moving fields into the builders, switch to getter style (`req.GetRole()` instead of `req.Role`), as `protogetter` requires.
- **Field numbering is decided against protos `main`, not the stale local base.** Gateway `Thread` uses 20/21 because main has `crm_contact_id = 18` and `tags = 19`, and this repo's gen already has them (`gen/go/gateway/v1/thread.pb.go:290-292`). The local working copy of `thread.proto` will show a gap after `updates_cursor = 17`. That is expected and resolves when the protos branch is brought up to date. Thread/v1 `Thread` uses 22/23 (Q1).
- **Regeneration trial results from planning:** generating thread/v1 from the stale local checkout dropped `GetCounters`, `SetCrmContact`, `RemoveTagByName`, `Snooze`, `Unsnooze`, `ThreadTags` and others from `gen/go/thread`. Generating from `git archive origin/main` + the local thread/v1 patch + the manual `Thread` hunk (22/23) changed only the 4 expected thread files, and gateway gen came out byte-identical to the committed one. `patch` reports `1 out of 4 hunks failed` for `thread_service.proto`, which is the `Thread` hunk.
- **Do not touch:** `GivePreview` (gateway or thread), `SetIdentity`/`ViaContextKey`/`GetViaFromContext` (unused elsewhere, still id only), the other `threadv1.Identity` literals in `message.go`, `buf/*`, and any gen dir other than thread/gateway.
- **Test conventions:** tests are in-package (`package service`, `package standard`). They use the standard `testing` package with table-driven `t.Run`, compare plain structs with `reflect.DeepEqual` (`Test_convertToThread`) and proto messages with `proto.Equal`. There is no testify in `internal/service` tests. Keep test data dummy (`gate-1`, `Support`, `Sales`): a recent commit replaced real-looking contact data in tests.
