# Working in DysonSphereBox

## Scope and authoritative guidance

This directory is the Go module and Wails application root (`develop/client` in the surrounding workspace). Run root commands here, not in a parent DysonSphereBox directory. The application is a Windows desktop companion for Dyson Sphere Program: BepInEx/mod installation, Nebula headless servers, blueprint tools, local resource management, configuration, logs, and self-updates.

Read [.codex/skills/dyson-sphere-box-project/SKILL.md](.codex/skills/dyson-sphere-box-project/SKILL.md) before changing code; its [project standards](.codex/skills/dyson-sphere-box-project/references/project-standards.md) contain detailed architecture and UI policy. Their absolute workspace paths and some early inventory descriptions are stale. In particular, business bindings now live in registered service structs, not primarily on `App`. Use current source/configuration to establish implementation details.

## Commands and verification

The current `go.mod` declares **Go 1.24.11**, with Wails **v2.12.0**. The README's Go 1.23 requirement is outdated. Development requires Node/npm, Wails CLI v2, and Windows WebView2. Wails uses npm even though pnpm workspace and lock files are also present.

| Working directory | Command | Purpose |
| --- | --- | --- |
| Application root | `wails doctor` | Check desktop development prerequisites |
| `frontend` | `npm install` | Install dependencies; also configured as Wails' install command |
| Application root | `wails dev` | Run the desktop host and Vite watcher together |
| `frontend` | `npm run dev` | Run Vite alone; does not supply the desktop Go/runtime bridge |
| `frontend` | `npm run build` | Run `vue-tsc --noEmit`, then Vite production build |
| Application root | `go build ./...` | Compile backend packages |
| Application root | `wails generate module` | Regenerate bindings after bound method/signature changes |
| Application root | `wails build` | Build packaged desktop application; build assets live under `build/` |
| `frontend` | `npm run preview` | Preview built frontend assets |

`main.go` embeds `all:frontend/dist` and `app.db`. On a clean checkout, build the frontend before a standalone Go build; `frontend/dist` is ignored and must exist for Go embedding. For binding changes, regenerate the bridge and run the frontend build against the new types as well as the Go build.

No Go test files, frontend test suite, test/lint package scripts, or CI workflow were found. The established checks are frontend typecheck/build, Go compilation, and relevant desktop verification through `wails dev`. Do not report builds as automated behavioral tests. No Markdown validation command is configured.

The README mentions `deploy/build.bat` and `script/install-headless.*`, but these scripts are absent in this checkout; `deploy/` is ignored. Do not assume those release commands are available. `core.Version` is the build-time version injection target (`-X DysonSphereBox/core.Version=...`); its current source default is `v0.0.0`, despite a comment referring to a `dev` default. Release assets and `checksums.txt` are described in the README's update/release sections.

## Backend architecture and wiring

The principal call path is:

```text
Vue page/composable -> generated Wails binding -> bound service method
                    -> helper / model / core -> filesystem, SQLite, HTTP, processes
Go progress event -> Wails runtime listener -> Vue state
```

- `main.go` embeds assets, configures the frameless desktop window/file drops, and binds `App` plus `service.RegisterServices()`.
- `app.go` owns Wails lifecycle/runtime glue and the game directory dialog. Startup sets the runtime context, initializes the database/configuration/HTTP client, repairs legacy scoped BepInEx files, recovers V2 installs, and recovers environment operations. Shutdown closes the database. Preserve this initialization order.
- `service/enter.go` aggregates `system`, `modDependence`, and `environment` service groups. A new bound service needs registration in its domain's `enter.go` and aggregation here if it introduces a domain. Constructors/registration must not require the database before startup.
- `model/enter.go` similarly aggregates domain models for `AutoMigrate`. New persistence models must be registered in their domain and, for new domains, the root registry. Merely declaring a model does not migrate it.

### Responsibility boundaries

| Directory | Responsibility |
| --- | --- |
| `service/` | Public business API, input validation, workflow/transaction decisions, audit decisions |
| `helper/` | Domain implementation details: queries, conversion, filesystem/archive operations, progress, logging builders |
| `core/` | Database lifecycle, configuration accessors, shared HTTP clients, runtime context, version |
| `model/` | GORM persistence schema and registration |
| `apiDefinition/` | External request/response contracts, relative API paths, URL construction |
| `constant/` | Shared states, source names, stages, event names and other stable values |
| `utils/` | Reusable low-dependency utilities; no imports from core/model/service/helper |

Project policy keeps new service files focused on structs and methods; reusable free functions belong in an existing suitable helper or utility package. Search those packages before creating parallel helpers. Split growing backend concerns into focused subpackages. The installer currently delegates substantial workflows to the `installer/v2` helpers; inspect that implementation rather than assuming the policy describes every historical file exactly. DTOs currently occur in service, helper/contract, and API packages; persistence models should not become a dumping ground for frontend or external DTOs.

External path templates belong in `apiDefinition/`; dynamic placeholders such as `{{UserID}}` are resolved with `utils.ResolveAPIPath` using matching exported struct fields. Keep HTTP execution and query/error policy in the calling workflow.

## Persistence, paths, and configuration traps

- Root `app.db` is an **embedded template**, not the live application database. The live file is `dyson-sphere-box.db` beside the executable, created from the template only if absent. Working-directory changes do not relocate it (`core/database_path.go`). Do not modify the template to alter a running application's settings.
- The environment library is also portable: `appData` beside the executable, not Windows `%APPDATA%` (`helper/environment/path.go`). Distinguish library copies from real game Mod, blueprint, and save directories.
- `core.InitDatabase` uses the existing pure-Go SQLite/GORM driver, one open/idle connection, DELETE journal mode, a compatibility migration, and registered-model `AutoMigrate`. Reuse `core.GLOBAL.DB`; preserve transaction handles when querying inside transactions. Complex data migrations are separate from ordinary schema registration.
- Configuration is stored by group/key through `ConfigUnit`; `Configure` holds accessors, not cached values. Important keys include `Game/StartupFilePath`, `Game/ServerArchive`, and `System/AppTheme`.
- Despite its name, `Game/StartupFilePath` stores a normalized **game directory**. `ConfigurationService.SetConfig` accepts a directory or executable path only when `dysonsphere.NormalizeDir` finds `DSPGAME.exe`. The directory picker returns an empty string on cancellation and can return an invalid raw path without persisting it; a nonempty picker result alone does not prove a valid saved configuration.
- The frontend never calls the application server directly. Account requests go Vue → Wails `AccountService` → `helper/system/account` → `core.GLOBAL.ServerClient` → server. `GLOBAL.ServerClient` is built from `System/ServerAddress` (empty means `constant/system.DefaultServerAddress`, `http://127.0.0.1:8888`); saving that key validates a replacement client before persisting, swaps `GLOBAL.ServerClient`, and clears the account session if the effective address changed. The account token lives in the `Account/Token` config, owned by the backend and never returned to the frontend. The old `System/BaseURL` key is retired. There is no `/api` proxy in Vite or the Wails asset server.
- Environment root resolution carries `NeedConfirm`, confidence, source, and existence separately. A guessed/default path is not authorization to create or overwrite resources. Recovery only cleans known `.dsbox` temporary artifacts and designated temp directories; preserve its distinction from user files.
- State-changing workflows record operation logs through helpers. Log dimensions are `namespace`, `module`, `event`, `level`, and `type`; readable text goes in `content`, structured details in `metadata`. Do not add workflow-specific log columns for details already representable there.

## Mod installation and environment flows

`service/modDependence/marketplace.go` exposes marketplace operations; `installer.go` exposes installation, uninstall, cancellation, conflict continuation, installed identity lists, and progress snapshots. BepInEx and Nebula have separate service/helper areas. `service/environment/` manages the local Mod/blueprint/save library and synchronization operations; inspect its manager/operation methods alongside `helper/environment/` when changing disk behavior.

Installation uses source-aware package keys (`<source>:...`) across DTOs, installed-state lookups, UI maps, and uninstall calls; the numeric-ID legacy installer and the private-server `official` market have been removed. Supported source values are `thunderstore` (fetched directly from thunderstore.io, never through `GLOBAL.ServerClient`) and `manual`; the manual entry is not ordinary remote pagination.

The V2 progress event is `modDependence:installProgress` (defined under `constant/modDependence/install/`). `useModPageState.ts` subscribes on mount, restores snapshots on mount/activation, and disposes listeners/observers/timers before unmount. Snapshots recover visible state after navigation/reload; they do not automatically restart an install. Cancellation and conflicts are special states, not ordinary failed installations. Uninstall behavior relies on tracked files and shared dependency ownership, not indiscriminate deletion of plugin directories.

## Frontend contracts

- Routes are generated from `frontend/src/pages` by `unplugin-vue-router`; `router.ts` uses hash history. Add page metadata with `definePage({ meta: { layout, title, keepAlive } })`. `App.vue` chooses `main`/`blank` layouts.
- `App.vue` uses the **RouterView slot's route** for both KeepAlive decisions and `route.fullPath` component keys. Preserve that pairing to avoid cached/uncached component identity problems. Cached pages need activation handling when external state may have changed.
- `frontend/wailsjs/**` is generated; never hand-edit it. `frontend/src/components.d.ts` and `typed-router.d.ts` are also generated by Vite plugins.
- Naive UI template tags auto-import through `NaiveUiResolver`. Component folders form namespaces (`components/universe/hi.vue` becomes `<UniverseHi />`). Script-side APIs still have explicit imports.
- Reusable bridge wrappers live in `frontend/src/services`; page orchestration/state lives in `composables`; pages render that state. Some composables also call generated bindings directly. Shared state such as theme/workshop state uses module-level refs; inspect scope before making it per-component.
- Follow Composition API, `<script setup lang="ts">`, strict TypeScript, explicit DTO/prop/event types, and existing file-local formatting. Page/component files commonly use lowercase kebab names, layouts PascalCase, and composables `use...` names. Formatting varies, so avoid unrelated rewrites.
- Use the installed Naive UI/Tailwind/SCSS stack. Project policy favors flat sections, small radii, subtle borders, and few cards. Global style tokens are in `style.scss`, Tailwind/PostCSS configuration uses `.cjs`, and Naive theme overrides are in `App.vue`.

### Blueprint and theme integration

`useThemeMode.ts` shares theme state, persists `System/AppTheme`, and updates `html[data-theme]` plus `color-scheme`. The resource detail preview in `components/blueprint/viewer-panel.vue` loads the bundled UMD script from `packages/dsp-blueprint-editor` via Vite `?url`. Its loader handles Wails/WebView `exports`/`module` globals.

The third-party preview library mutates global theme state and injects styles. Preserve the wrapper's theme snapshot/restore, stylesheet cleanup, and viewer lifecycle handling; removing them can change the entire application's day/night theme. This local viewer is separate from the full tool page in `pages/blueprint-generator.vue`.

### Tutorials and UI changes

Driver.js tutorial definitions live in `tutorials/registry.ts`, selectors in `tutorials/selectors.ts`, and lifecycle/state coordination in `usePageTutorial.ts`. Tutorial completion/skipping is persisted through `TutorialService` by page key **and tutorial version**. UI changes must keep selectors consistent with their targets. Read `docs/首次使用引导教程开发指导.md` before extending the tutorial system.

## Existing project conventions and further context

The project skill specifies Chinese code comments: backend responsibility/usage/boundary documentation (`职责`, `使用`, `边界`), identifier-first Go documentation comments, and Chinese frontend section/template documentation. Consult the skill for the full convention when code comments are in scope. User-facing application text is predominantly Chinese.

Read domain documentation selectively:

- `docs/环境管理方案.md`: environment management design; compare proposed stages with current service implementation.
- `docs/戴森球计划蓝图解析方案.md`: blueprint parsing context; implementation begins in `helper/environment/blueprint_parser.go`.
- `frontend/src/packages/dsp-blueprint-editor/lib-README.md`: bundled viewer/editor API.
- `README.md`: product capabilities and release/update context, with the version/path caveats above.

`runtime-shots/` contains screenshots and a large Edge profile; exclude it from normal source searches. Generated build output and local runtime data are not substitutes for source. Keep changes focused on the requested feature and review generated-file diffs after builds or binding generation.
