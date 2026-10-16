Client configuration-test cleanup audit — 2026-10-15

Result: no test or fixture changes required. All five current multiplayer test files remain valid; no production behavior or pre-existing dirty work was changed. The only file created by this task is this report.

Audit: consulted client/AGENTS.md and its project skill/standards, then used CodeGraph before locating and reviewing code. CodeGraph did not index management_test.go or relay_policy_test.go, so those two files were read directly. Existing server configuration changes were consulted only to establish the removed YAML controls.

The following rg commands ran from the client module root:

```powershell
rg --files service/multiplayer helper/multiplayer apiDefinition/multiplayer -g '*_test.go' -g '*fixture*' -g '*testdata*' -g AGENTS.md
rg -n -i 'yaml|viper|mapstructure|min[-_]?client[-_]?version|observer[-_]?nat[-_]?port|reservation[-_]?ttl[-_]?seconds|max[-_]?relay[-_]?members|max[-_]?reservations|max[-_]?circuits|max[-_]?streams|max[-_]?connections|memory[-_]?bytes|buffer[-_]?size|connection[-_]?duration[-_]?seconds|connection[-_]?data[-_]?bytes' . -g '*_test.go' -g '**/testdata/**' -g '**/fixtures/**' -g '*fixture*' -g '*.yaml' -g '*.yml' -g '!frontend/node_modules/**' -g '!runtime-shots/**' -g '!.local/**'
rg --files service/multiplayer helper/multiplayer apiDefinition/multiplayer -g '*.json' -g '*.yaml' -g '*.yml' -g '**/testdata/**' -g '**/fixtures/**' -g '*fixture*'
rg -n 'RelayFallback|Capabilities|Policy|Management|management|ConfigureRelay' service/multiplayer helper/multiplayer apiDefinition/multiplayer -g '*_test.go'
```

Inventory: service/multiplayer/management_test.go; helper/multiplayer/link/link_test.go; helper/multiplayer/p2p/autonat_test.go, observed_factory_test.go, and relay_policy_test.go. The removed-field/YAML audit and fixture inventory both returned no matches (rg exit 1). No test imports/parses server YAML or asserts that configuration-file controls determine client behavior.

Preserved valid coverage:
- RelayFallback in link_test.go and relay_policy_test.go is a wire capability passed to ConfigureRelay, covering native reservations, game forwarding, revocation, replacement relay identity, and fail-closed invalid policies.
- management_test.go verifies administrative close/kick/termination, TCP/UDP/stream cleanup, suppression of reconnect, capability advertisement, and committed/stale/aborted session-policy events with admission changes.
- AutoNAT source admission and observed-address lifecycle tests exercise network trust and connection lifetime. The synthetic second address using port 4002 shares the first observer PeerID; it does not assert the removed observer-nat-port setting or a second configured observer.

Fresh validation, with Go test caching disabled:

```text
go test -count=1 ./service/multiplayer/... ./helper/multiplayer/... ./apiDefinition/multiplayer/...
ok  DysonSphereBox/service/multiplayer       2.712s
?   DysonSphereBox/helper/multiplayer/forward [no test files]
ok  DysonSphereBox/helper/multiplayer/link   0.433s
ok  DysonSphereBox/helper/multiplayer/p2p     5.243s
?   DysonSphereBox/helper/multiplayer/socket  [no test files]
?   DysonSphereBox/apiDefinition/multiplayer  [no test files]
```

Exit code: 0. Validation covers local client behavior; no public-network or desktop/WebView verification was performed. Nothing remains for the client test/fixture cleanup scope.
