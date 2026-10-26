# Thread source and queue (gateway)

im-gateway-service part of the design decision "source and queue in active threads".

- **Branch:** `feat/gateway-thread-source-queue`
- **Plan:** [docs/dev-task-plans/gateway-thread-source-queue.md](../dev-task-plans/gateway-thread-source-queue.md)
- **Release order:** protos -> im-thread-service -> **im-gateway-service** -> im-providers-service -> call_center. Every new field is optional, so missing headers or a missing queue behave exactly as before.

## What changed and why

Agents need to see which gate (provider) a thread came in through and which call_center queue routed it to them. That data is stored and computed in im-thread-service. The gateway has to:

1. pass the gate's name and type from the provider's headers to im-thread, so im-thread can record the thread's source,
2. forward the queue that call_center sends in `x-webitel-queue-id` / `x-webitel-queue-name-bin` headers when it adds, transfers or upgrades a member,
3. expose `source` and `queue` on the gateway `Thread`.

### Protos

Protos branch `feat/thread-search-source-queue` (commit `943524a8`, `main` merged in, not yet merged to protos `main`).

| Proto | Change |
|---|---|
| `im/api/gateway/v1/thread_member.proto` | new `message ThreadQueue { int64 id = 1; string name = 2; }` |
| `im/api/gateway/v1/thread.proto` | new `message ThreadSource { string id = 1; string name = 2; string type = 3; }`; `Thread.source = 21`, `Thread.queue = 22` (main already uses 18 `crm_contact_id` / 19 `tags` / 20 `channel`). The member/preview requests get **no** queue field: the queue comes in headers |
| `im/service/thread/v1/thread_service.proto` | `Thread.source` / `Thread.queue` renumbered from 21/22 to **22/23**, so they do not clash with `crm_contact_id = 21` on protos `main` |

These thread/v1 additions were already in the local diff: `Identity.via_ref = 5` with `message Via{id,name,type}` (`Identity.via` is now `[deprecated = true]`), `message ThreadQueue`, `AddMemberRequest.queue = 8`, `TransferRequest.queue = 7`, `UpgradePreviewToFullMemberRequest.queue = 6` and `message ThreadSource`.

### Generated code

`gen/go/thread` and `gen/go/gateway` are generated from the protos branch above (`943524a8`, which already contains protos `main`), after merging im-gateway-service `main` (WS-68 thread channel). Compared to `origin/main`, only `gen/go/thread/v1/{peer,shared,thread_preview_service,thread_service}.pb.go` and `gen/go/gateway/v1/{thread,thread_member}.pb.go` differ. `buf generate` was run by hand from `buf/` with an absolute input (`buf generate /Users/dtsnk0/Documents/projects/protos/im --template buf.gen.<x>.yaml --path .../im/<service/thread|api/gateway>`), because the relative path in the `//go:generate` lines does not resolve from a worktree. The `buf/*` files are unchanged.

> Before merge, regenerate gen/go from protos `main` once the protos PR is merged.

### Via headers -> `Identity.via_ref`

- `infra/auth/auth.go`: new constants `ViaTypeIdentificationHeader = "x-webitel-via-type"` and `ViaNameIdentificationHeader = "x-webitel-via-name-bin"`. `auth.Identifier` gains `GetViaName()` and `GetViaType()`. `auth.BinaryHeader(md, key)` returns the first value of a `-bin` header coerced with `strings.ToValidUTF8(..., "�")`, because proto3 `string` fields fail to marshal invalid UTF-8. `-bin` metadata is already base64-decoded by grpc-go.
- `infra/auth/standard/auth.go`: `Identity` gains `ViaName` and `ViaType`. `resolveProviderIdentity` reads the id, type and name (via `auth.BinaryHeader`) headers and is the only place that fills the new fields; schema, engine and user identities are unchanged.
- `internal/service/message.go`:
  - `senderIdentity(identity)` builds `threadv1.Identity{Name, ChatName, Via (deprecated, id), ViaRef}`.
  - `viaRef(id, name, type)` returns `Via{id,name,type}`, or nil when the id is empty. `callerViaRef(identity)` applies it to the caller; when the name/type headers are missing it yields an id-only `Via`.
  - `SendText`, `SendDocument` and `SendInteractive` use `senderIdentity`. `resolveContact` (recipient) sets an id-only `ViaRef: viaRef(viaID, "", "")`.
- `internal/service/message_forward.go`: the `ForwardMessages` sender identity sets `ViaRef: callerViaRef(identity)` (it keeps omitting `ChatName`, as before).
- Every call path that set `via` before now sends **both** `via` and `via_ref`. im-thread's `mapViaInfoFromProto` prefers `via_ref.id` and falls back to `via`, and older im-thread builds ignore field 5, so this is safe whichever service is deployed first. Remaining writes of `Via` carry `//nolint:staticcheck`.

### Queue forwarding

The queue travels in gRPC headers, like the gate (`via`), not in the gateway request body:

- `infra/auth/auth.go`: `QueueIDIdentificationHeader = "x-webitel-queue-id"` (int64 as ASCII) and `QueueNameIdentificationHeader = "x-webitel-queue-name-bin"` (binary metadata, the display name may be non-ASCII).
- `internal/service/thread.go`: `queueFromContext(ctx, logger)` reads both headers. No id header gives nil. An id that does not parse or is `<= 0` is logged (`QUEUE_HEADER_INVALID`) and dropped, and the call still succeeds, because the queue is only shown in the UI. The name is read with `auth.BinaryHeader`.
- `AddMember`, `Transfer` (`thread.go`) and `UpgradePreviewToFullMember` (`thread_preview.go`) set `Queue: queueFromContext(ctx, logger)` directly in their request literals. `GivePreview` and `RemoveFromPreview` are not touched.

### Source/queue in responses

`convertToThread` maps `Source` via `convertThreadSource` and `Queue` via `convertThreadQueue`; a nil input gives a nil output. Search, SearchLeft, Get, Create and GetUpdates (`updates.go`, `Dialog`) all go through `convertToThread`, so all of them return the fields. Which fields are actually populated is decided by im-thread: Search returns `queue`; Get does not compute it; `source` is set only when a source ref row exists.

## Acceptance criteria

| # | Criterion | Met | Notes |
|---|---|---|---|
| 1 | gateway/v1 proto additions in the local protos working copy; `GivePreviewRequest` unchanged; nothing committed, pushed, reset or stashed in protos | Yes | Field numbers verified |
| 2 | thread/v1 `Thread.source`/`queue` = 22/23 (no clash with `crm_contact_id = 21`) | Yes | Matches the generated `thread_service.pb.go` |
| 3 | Only `gen/go/thread/v1` and `gen/go/gateway/v1` regenerated; existing symbols (`GetCounters`, `SetCrmContact`, `CrmContactId`, `Tags`, `RemoveTagByName`, `Snooze`, `ThreadTags`) kept; `buf/*` unchanged | Yes | 7 gen files changed; symbol counts match HEAD |
| 4 | Header constants; `auth.BinaryHeader` UTF-8 coercion; `GetViaName`/`GetViaType` on `auth.Identifier` | Yes | |
| 5 | `via_ref` on SendText/SendDocument/SendInteractive (full / id-only / nil), deprecated `via` kept; ForwardMessages and resolveContact also set `via_ref` | Yes | |
| 6 | Queue forwarded on AddMember/Transfer/UpgradePreviewToFullMember from the `x-webitel-queue-id` / `x-webitel-queue-name-bin` headers when the id is a positive integer; GivePreview untouched | Yes | Changed after review, at the user's request: headers instead of a request field |
| 7 | Unit tests: headers, via_ref building, queue headers (`Test_queueFromContext`: none / id only / non-ASCII and invalid UTF-8 name / 0, negative, non-numeric id), convertToThread source/queue | Yes | |
| 8 | `go build ./...` and `go test ./...` pass | Yes | |

## Open issues (from review)

- **Indirect coverage only (info):** no test calls `ForwardMessages` or `resolveContact` directly; their `via_ref` is covered only through the `senderIdentity` / `viaRef` unit tests. Queue wiring in AddMember/Transfer/UpgradePreviewToFullMember is likewise not unit-tested; only `queueFromContext` is. The thread and contact clients are concrete discovery clients and cannot be mocked, so a regression where these paths stop calling the helpers would not be caught.
- **Cross-repo follow-ups:** im-thread-service must regenerate its gen/go with thread/v1 `Thread.source = 22` / `queue = 23`; its current branch was generated with 21/22. The protos branch is up to date with `main` (`943524a8`); once its PR is merged, regenerate this repo's gen/go from protos `main` (expected to be byte-identical).

## How to use / verify

**Providers** (im-providers-service) send, next to the existing `x-webitel-provider` and `x-webitel-via`:

```
x-webitel-via-type: telegram_bot
x-webitel-via-name-bin: <gate display name, raw UTF-8; gRPC base64-encodes -bin values on the wire>
```

Old providers that send only `x-webitel-via` still work: `via_ref` is then `{id}` only.

**call_center** sends the optional queue as headers on the gateway `AddMember`, `Transfer` and `UpgradePreviewToFullMember` calls, next to its existing `x-webitel-type` / `x-webitel-schema`:

```
x-webitel-queue-id: 7
x-webitel-queue-name-bin: <queue display name, raw UTF-8; gRPC base64-encodes -bin values on the wire>
```

No `x-webitel-queue-id`, or an id that is not a positive integer, means no queue. The headers are ignored on every other RPC.

**Clients** read the new fields on gateway `Thread` (Search, Get, SearchLeft, GetUpdates `Dialog`):

```jsonc
{ "id": "...", "source": { "id": "gate-1", "name": "Support", "type": "facebook" }, "queue": { "id": 7, "name": "Sales" } }
```

**Verify locally** (from the repo root):

```sh
go build ./...
go test ./...
go test ./infra/auth/standard/ -run 'Test_providerVia|Test_IdentityViaGetters' -v
go test ./internal/service/ -run 'Test_senderIdentity|Test_viaRefFromID|Test_queueFromContext|Test_newAddMemberRequest|Test_newTransferRequest|Test_newUpgradePreviewRequest|Test_convertToThread' -v
grep -c 'GetCounters\|SetCrmContact' gen/go/thread/v1/thread_service_grpc.pb.go   # must be non-zero
```

End to end: the via headers and the queue reach im-thread only when im-thread-service is deployed from its `feat/thread-search-source-queue` branch, regenerated with 22/23. Older im-thread builds ignore the new fields.
