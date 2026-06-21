# Onebox Fx Graph Removal Plan

## Current state

PR: https://github.com/temporalio/temporal/pull/10319

The branch has reached the mechanical goal: `tests/testcore/onebox.go` no longer builds its own per-service Fx graphs directly and instead starts services through `temporal.NewServerFx`. The remaining work is review and CI validation.

The public `temporal/` package must stay intentionally small. Keep public additions only when there is a clear non-test/product use case. Prefer existing config/dynamic config first, focused testhooks second, and test-side workarounds third.

## Checklist status

- Done: DLQ Fx wrapper removal is split out and merged as #10541.
- Done: onebox client extraction is split out and merged as #10575.
- Done in this branch: `FrontendServiceRefsCreated` removed. Frontend/admin/operator/history/scheduler clients are owned by `tests/testcore/clients.go`; matching now builds a production-style matching client directly in testcore.
- Done in this branch: CHASM refs hooks are typed and focused. `HistoryChasmComponentsCreated` exposes only the engine, visibility manager, and registry.
- Done in this branch: `NamespaceRegistryCreated`, `ChasmRegistryInitializer`, `MatchingRawClientCreated`, broad replication gRPC hooks, and `PersistenceExecutionManagerWrapper` were removed.
- Done in this branch: namespace availability waits, replication stream observation, history-task write observation, CHASM library registration, and matching client construction now use narrower APIs.

## Recommended PR presentation

If this all lands in one PR, do not present it primarily as a list of removed hooks. The cleaner review partition is by design boundary:

### 1. Start onebox through the production server graph

What to present:

- `tests/testcore/onebox.go` starts every service host with `temporal.NewServerFx`.
- Onebox still passes only the requested service to `ForServices`.
- Static host validation now requires `Self` only for services requested by `ForServices`.
- Each per-service config still includes frontend HTTP connection details so local system callbacks and HTTP clients do not fall back to port `0`.
- Built-in SQL/Cassandra schema validation is skipped when a custom persistence factory is supplied, because custom factories own their own schema/version compatibility contract.

Cleanliness rating: 2. This is the core architectural win, but the custom-factory schema-check rule and frontend config duplication deserve focused review.

### 2. Replace graph-captured service refs with test-owned clients

What to present:

- Frontend/admin/operator/history/scheduler clients are constructed directly from known onebox addresses.
- Matching uses `matching.NewClient` with a test static resolver, test dynamic config, metrics, TLS-aware dialing, and frontend-backed namespace ID lookup.

Cleanliness rating: 2. This removes fx graph access, but the matching client path intentionally mirrors production construction and may need maintenance if matching client dependencies change.

### 3. Replace broad customization hooks with focused observation hooks

What to present:

- Replication recording uses `ReplicationStreamMessageObserver` instead of generic gRPC interceptor/dial-option hooks.
- Task queue recording uses `HistoryTasksWrittenObserver` instead of replacing `persistence.ExecutionManager`.
- CHASM component capture remains focused on the few values tests inspect.

Cleanliness rating: 3. These hooks expose events or typed refs rather than letting tests replace arbitrary graph pieces.

### 4. Move test-specific setup to owning suites or real extension points

What to present:

- Archival setup is owned by archival tests/config.
- DLQ setup owns its queue manager and SDK client directly.
- CHASM test library registration uses `temporal.WithChasmLibraries`, a real embedded-server extension point.
- Custom archiver factory options remain public only because they are embedded-server configuration, not onebox-only graph access.

Cleanliness rating: 3. Ownership follows the suite or product extension point that actually needs the behavior.

### 5. Use observable readiness instead of internal cache refs

What to present:

- Namespace registry refs are gone.
- Namespace creation waits through public frontend APIs.
- XDC failover tests explicitly wait for cache refresh only where stale cache state is part of the behavior.

Cleanliness rating: 3. The test waits on user-visible behavior instead of registry internals.

### 6. Public API review checklist

What to present:

- Keep: `WithTestHooks`, because focused internal testhooks still need a contained injection point.
- Keep: `WithChasmLibraries`, because embedded servers need a pre-start CHASM library registration point.
- Keep: custom archiver factory options, if reviewers agree they are valid embedded-server configuration.
- Avoid: public gRPC interceptor/dial-option options for onebox-only recording.
- Avoid: onebox-only public switches such as an explicit persistence version-check disable option.

Cleanliness rating: 2. The public surface is small, but every exported option should be defended independently from onebox.

## Historical audit trail

### 1. `FrontendServiceRefsCreated`

Resolution: removed from `service/frontend/fx.go` and `common/testing/testhooks/hooks.go`.

What changed: frontend/admin/operator clients are now created lazily by `tests/testcore/clients.go` using direct frontend gRPC dialing. History and scheduler clients use the same direct test-client path. Matching now builds a production-style matching client directly in testcore without an fx graph capture hook.

Options considered:

- Keep `FrontendServiceRefsCreated`: rejected because it exposed unrelated frontend graph refs.
- Create all test clients directly from gRPC addresses: works for frontend/history/scheduler, but failed for matching.
- Build a production matching client from testcore without fx graph access: selected after testcore gained the static resolver and namespace lookup needed to preserve production routing behavior.

Obstacle discovered: direct matching gRPC clients failed `TestNexusMatchingTestSuite/TestDispatchNexusTaskOnNonRootPartitionNoForwarding`; they bypass the production matching routing behavior expected by Nexus matching tests.

Public `temporal/` impact: none.

### 2. CHASM service refs

Resolution: replaced broad history refs with typed CHASM-specific hooks.

What changed: `HistoryServiceRefsCreated` became `HistoryChasmComponentsCreated`, which exposes only `chasm.Engine`, `chasm.VisibilityManager`, and `*chasm.Registry`. `ChasmRegistryInitializer` was removed after CHASM library registration moved to `temporal.WithChasmLibraries`.

Options considered:

- Keep `HistoryServiceRefsCreated`: rejected because the name and shape imply arbitrary service graph access.
- Return the three CHASM values as untyped values: rejected because it preserved the same cast-heavy pattern.
- Use a typed struct: selected because it is explicit, compile-time checked, and still narrow.
- Avoid a hook entirely by moving CHASM test-library registration fully into test code: selected via the server option path.

Obstacle discovered: CHASM registration needs to happen while the production server graph is being initialized.

Public `temporal/` impact: `WithChasmLibraries`.

### 3. `PersistenceExecutionManagerWrapper`

Resolution: replaced the execution-manager wrapper with a focused task-write observer.

What changed: `TaskQueueRecorder` now observes `HistoryTasksWritten` events emitted after successful persistence writes instead of wrapping `persistence.ExecutionManager`.

Options considered:

- Leave the arbitrary wrapper hook: rejected as too broad.
- Replace with `HistoryTasksWritten`: selected because it observes the exact event tests need.
- Make task recording opt-in: preferred and tracked by #10583.

Obstacle discovered: recording must happen only after successful writes and must cover all task-emitting write paths.

Public `temporal/` impact: internal testhook only.

### 4. DLQ Fx wrapper removal

Resolution: generic `WithFxOptions` usage was removed from `tests/dlq_test.go`.

What changed: the DLQ suite now creates its own `HistoryTaskQueueManager` from the persistence test factory and dials its own system SDK client when it needs to wait on DLQ job workflows.

Options considered:

- Keep `fx.Populate` via `WithFxOptions`: rejected because it preserves generic test graph access.
- Add a DLQ-specific hook: unnecessary because the suite can get the persistence queue manager from existing test cluster state.
- Reuse the production SDK client factory: rejected for this branch because that would require another graph access path; direct SDK dialing is enough for the test.

Obstacle discovered: `parallelsuite` rejects `s.Require()`, so setup assertions must use the suite’s direct assertion methods.

Public `temporal/` impact: none.

## TODO solution evaluation

Scale: 1 = hacky / fragile, 2 = acceptable but with visible compromise, 3 = clean / narrow / maintainable.

### 1. `NamespaceRegistryCreated`

Implemented solution: removed `testhooks.NamespaceRegistryCreated`. Onebox now waits through public namespace APIs with `TestCluster.WaitForNamespaceAvailable`, and xdc failover paths wait for namespace cache refresh where stale cache state matters.

Cleanliness rating: 3. This uses the public frontend surface and keeps namespace registry internals out of testcore.

Options considered:

- Keep collecting registries directly: rejected because it is graph refs access.
- Use `NamespaceCacheRefreshInterval` with a very low interval: possible but indirect and can make tests timing-sensitive.
- Add/use force-refresh-on-read dynamic config: explored, but new namespace reads already fall through to persistence; this is more useful for update/delete/replication staleness than ordinary creation.
- Wait for frontend namespace visibility only: likely enough for new namespace creation because services can read through to persistence on misses.
- New idea: the registry already does persistence read-through on cache miss (with a 1s not-found TTL); after `RegisterNamespace` returns, the namespace exists in persistence and all services will find it on the next miss (at most one TTL interval later). A focused test utility `WaitForNamespaceAvailable` that calls `DescribeNamespace` on each service's frontend gRPC address with short-interval retry would be zero-production-change and make the wait condition explicit. No registry refs, no hook, no config tweak.

Obstacle discovered: namespace cache behavior is subtle. A force-refresh option is not clearly justified for new namespace creation, and using it broadly would be a product config addition for a test-only concern.

Public `temporal/` impact: none.

### 2. Archival setup

Implemented solution: archival setup stays suite-owned instead of onebox-owned. The branch already had the archival config/factory plumbing needed by `tests/archival_test.go`, so no additional onebox change was required.

Cleanliness rating: 3. The behavior is owned by the suite that needs it and does not add a onebox-specific hook.

Options considered:

- Keep archival setup in onebox: rejected because only archival tests need it.
- Move archival setup into the archival suite: preferred; the suite controls the config it needs.
- Add public archiver factory options to `temporal/`: possible, but should only be kept if independently useful outside onebox tests.
- Use testhooks for archiver factories: possible but probably worse than suite-owned config because archiver setup is test-specific configuration, not an execution hook.

Public `temporal/` impact: `temporal.WithCustomHistoryArchiverFactory` and `temporal.WithCustomVisibilityArchiverFactory` remain justifiable as embedded-server configuration, not onebox graph access.

### 3. Replication stream recorder hooks

Implemented solution: removed generic service interceptor and client dial-option hooks. Added a focused `ReplicationStreamMessageObserver` testhook at the replication stream sender/receiver boundary, and made `ReplicationStreamRecorder` consume that focused message stream.

Cleanliness rating: 3. The hook is scoped to exactly the observed subsystem and no longer allows arbitrary service gRPC customization.

Options considered:

- Public interceptor/dial-option server options: rejected for now; this is onebox-only instrumentation.
- Keep broad `ServiceGrpcInterceptors` and `ServiceClientDialOptions`: works, but too much customization surface.
- Recorder-specific testhooks: likely best if the recorder must stay server-side and client-side.
- Test-side recorder only: insufficient if the test needs to observe server-to-server replication streams.
- New idea: the recorder already filters to exactly one method (`StreamWorkflowReplicationMessages`). Instead of per-service generic interceptor/dial-option hooks, add a focused `ReplicationStreamObserver` hook on the replication manager's stream factory or task executor. The replication manager already owns the stream lifecycle; a single hook there covers all replication traffic without generic per-service gRPC hooks. This replaces two broad hooks with one narrow hook scoped to the replication subsystem.

Public `temporal/` impact: avoid new public interceptor/dial options. Existing `WithChainedFrontendGrpcInterceptors` should not be expanded for onebox-only needs.

### 4. `TaskQueueRecorder` final shape

Implemented solution: removed `testhooks.PersistenceExecutionManagerWrapper`. Added focused `HistoryTasksWrittenObserver` / `HistoryTasksWritten` plumbing in persistence client write paths, and made `TaskQueueRecorder` a sink instead of a persistence manager wrapper.

Cleanliness rating: 3. The hook observes the event tests need after successful writes and no longer replaces the persistence implementation.

Options considered:

- Keep the typed wrapper: acceptable as an intermediate cleanup, not ideal final shape.
- Add `HistoryTasksWritten`: preferred because it observes the event tests need without replacing persistence.
- Make recorder opt-in: preferred so most tests do not pay allocation or recording overhead.
- Move recorder fully into xdc tests: possible for ownership, but the write observation point still needs to be in production/testhook plumbing.

Public `temporal/` impact: internal testhook only.

### 5. `ChasmRegistryInitializer`

Implemented solution: removed `testhooks.ChasmRegistryInitializer`. Added `temporal.WithChasmLibraries(...chasm.Library)` and pass `chasmtests.Library` through onebox server options.

Cleanliness rating: 3. This is a real embedded-server extension point and registers libraries before startup without exposing registry internals.

Options considered:

- Keep typed `ChasmRegistryInitializer`: simple and narrow, but still startup-time test behavior in the production graph.
- Use `HistoryChasmComponentsCreated` to register after creation: likely too late if services need libraries registered during startup.
- Make CHASM test registry setup non-fx/test-owned: attractive, but needs a way to provide the preconfigured registry to the production graph without adding public options.
- New idea: `temporal.WithChasmLibraries(...chasm.Library)` as a public production option, not just a test concern. Production CHASM modules register their libraries via `fx.Invoke` inside each service's fx module, which is not accessible to users embedding Temporal via `temporal.NewServer`. Anyone writing a custom CHASM state machine for an embedded deployment would need this same registration point. Exposing it as a supported option would be independently justified, eliminate the testhook entirely, and give embedded users a proper extension point without requiring them to fork service fx modules.

Public `temporal/` impact: adds `WithChasmLibraries`, which is broader than onebox but justified for embedded users that need to register custom CHASM libraries.

### 6. `MatchingRawClientCreated`

Implemented solution: removed `testhooks.MatchingRawClientCreated`. `tests/testcore/clients.go` now constructs a production-style matching client directly with `matching.NewClient`, a static test resolver over known matching hosts, namespace ID lookup through frontend, test dynamic config, metrics, and TLS-aware gRPC dialing.

Cleanliness rating: 2. This removes graph access and preserves production routing behavior, but it duplicates enough matching-client construction in testcore that it should be watched if matching client dependencies change.

Options considered:

- Direct matching gRPC client: rejected after the Nexus matching regression.
- Recreate production matching raw client in testcore: not currently practical without duplicating client bean and namespace routing setup.
- Keep old frontend refs hook: rejected as too broad.
- Capture only the production matching raw client: selected as the narrowest working path.
- New idea: the matching raw client construction in `clientfactory.go` needs a membership resolver, RPC factory, metrics handler, dynamic config, logger, and a namespace-ID-to-name function. These are all available in testcore via other already-collected state (service addresses, membership ports, onebox logger, etc.). Testcore could construct a static membership resolver from known service addresses and call the existing `matching.NewClient` directly without graph access. This needs more investigation to confirm the full dependency list is available without the production fx graph, but if viable it removes the hook entirely rather than just narrowing it.

Public `temporal/` impact: internal testhook only.

### 7. Public `temporal/` package diff

Implemented solution:

- Keep `WithTestHooks` for focused internal testhooks.
- Keep `WithChasmLibraries` as a non-test-specific embedded-server extension point.
- Keep custom archiver factory options as embedded-server configuration.
- Relax static-host validation to require self-addresses only for services requested by `ForServices`.
- Skip built-in persistence schema/version checks when a custom persistence factory is supplied, because custom factories own their own compatibility contract.

Cleanliness rating: 2. The public surface is still small and the onebox-specific skip option was avoided, but `WithTestHooks` remains intentionally test-only and the custom-factory schema-check rule deserves reviewer attention.

Options considered:

- Add public options for every onebox customization: rejected; this would turn test-only needs into supported product API.
- Keep only `WithTestHooks`: preferred because it gives test-only branches a contained extension point.
- Use existing config/dynamic config everywhere possible: preferred before testhooks.
- Reach into fx graphs from onebox: rejected as the pattern this PR is trying to remove.

Obstacle discovered: onebox's custom persistence factories exercise `NewServerFx` through a path where the built-in config-backed SQL/Cassandra schema checker is the wrong owner. The implemented rule keeps production defaults strict while letting custom factories define their own schema contract.

## Are we close?

Yes. Onebox now starts services through `temporal.NewServerFx`, broad graph-access hooks are removed, and the remaining compromises are explicit in the ratings above. The root PR should stay draft until CI validates the final branch state.
