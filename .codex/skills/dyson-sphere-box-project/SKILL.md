---
name: dyson-sphere-box-project
description: Project-level coding and architecture rules for the DysonSphereBox Wails application. Use when editing or creating files in this repository, especially Go backend bindings, Vue 3 TypeScript frontend code, Naive UI components, Tailwind CSS styling, SCSS, Vue Router auto-routes, layouts, KeepAlive behavior, Vite configuration, Wails generated bindings, or project structure.
---

# DysonSphereBox Project

Use this skill for work inside `D:\Workspace\DysonSphere\DysonSphereBox`.

This skill is mandatory project policy. When editing this repository, follow these rules strictly unless the user explicitly asks for an exception.

## First Checks

Before editing, inspect the relevant local files instead of assuming current state:

- Root Wails files: `main.go`, `app.go`, `wails.json`, `go.mod`.
- Frontend config: `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/tailwind.config.cjs`, `frontend/postcss.config.cjs`.
- Frontend entry and shell: `frontend/src/main.ts`, `frontend/src/App.vue`, `frontend/src/router.ts`, `frontend/src/style.scss`.
- Existing directories: `frontend/src/pages`, `frontend/src/layouts`, `frontend/src/components`, `frontend/src/types`, `frontend/wailsjs`.

Read `references/project-standards.md` when implementing or reviewing non-trivial changes.

Before creating, changing, or reviewing frontend typography, UI layout, controls, theme colors, or visual states, read [界面视觉规范](references/ui-visual-standards.md). It defines the project's font roles, day/night palette, token ownership, and UI verification requirements. Use current source to verify token values and asset paths.

## Installed Stack

Use the libraries already installed in this project:

- Wails v2 for Go desktop app integration.
- Go backend with GORM for local persistence.
- SQLite is the local database. Open it through `core.GLOBAL.DB`; prefer the pure-Go SQLite driver already configured for this project.
- Use `gorm.io/datatypes` for JSON columns, especially `datatypes.JSON`.
- Vue 3 with Composition API and TypeScript.
- Vite 3 for frontend build/dev.
- Naive UI for UI components. The user may call it `Native-UI`; treat that as `naive-ui` unless the package actually changes.
- `unplugin-vue-components` with `NaiveUiResolver` for automatic Naive UI component imports.
- Vue Router 4 and `unplugin-vue-router` for file-based auto-routes in `frontend/src/pages`.
- Tailwind CSS 3 for utility styling.
- SCSS via `sass` for component-level or global styles that utilities do not express cleanly.
- PostCSS and Autoprefixer for Tailwind processing.
- `vue-tsc` for type checking.

Do not add new UI, routing, styling, database, or state libraries unless the task clearly requires capability not covered by the installed stack and the reason is stated.

## Hard Structure Rules

Separate files by responsibility. Do not put unrelated layout, page, service, type, helper, model, and API logic into one file.

- Route pages belong in `frontend/src/pages`.
- Shared layouts belong in `frontend/src/layouts`.
- Reusable Vue components belong in `frontend/src/components`.
- Shared TypeScript declarations belong in `frontend/src/types`.
- Frontend route setup belongs in `frontend/src/router.ts`.
- Global styles and Tailwind directives belong in `frontend/src/style.scss`.
- Wails generated files under `frontend/wailsjs` are generated output. Do not hand edit them.
- Go methods exposed to frontend must be exported methods on a bound struct, usually `App` in `app.go`.
- Backend global initialization belongs in `core`; keep it small and focused on application-wide infrastructure such as database lifecycle.
- GORM models belong under `model/<domain>`. Each domain must expose an `enter.go` with `RegisterModels() []interface{}`.
- The root `model/enter.go` is the only place that aggregates domain model groups. `core` must call `model.RegisterModels()` instead of importing domain model packages directly.
- Services belong under `service/<domain>` and are the core backend business layer. Each service domain must expose an `enter.go` that registers Wails-bound service structs.
- Helpers belong under `helper/` and only assist services with reusable implementation details. Helpers must not replace services as the owner of business workflows.
- All backend directories must also follow single-responsibility directory structure, not only `helper/<domain>`. When any directory under `apiDefinition/`, `constant/`, `core/`, `model/`, `service/`, `helper/`, or `utils/` accumulates many files or clearly contains multiple concerns, split it into concern subdirectories instead of leaving a wide flat folder. Examples: `helper/modDependence/audit/`, `helper/modDependence/installer/`, `service/modDependence/marketplace/`, `apiDefinition/modDependence/thunderstore/`, `model/modDependence/installed/`, `constant/modDependence/install/`, or `utils/path/` when those concerns grow enough to justify a package boundary.
- A backend subdirectory must have one responsibility and one reason to change. For example, `helper/modDependence/audit/` may contain only operation-log builders and audit event helpers; it must not contain installer file-copy logic, marketplace HTTP queries, BepInEx archive extraction, or headless process control. The same rule applies everywhere: `service/.../installer/` owns installer orchestration methods, `apiDefinition/.../thunderstore/` owns Thunderstore contracts, and `model/.../installed/` owns installed-record persistence models. If a concern grows again inside a subdirectory, split it one level deeper instead of creating broad catch-all files.
- Reusable utility functions belong under `utils/`. Do not hide general helpers inside `core`, `service`, `model`, or frontend code.
- Service-specific helper functions and helper objects belong under `helper/`. Do not place free helper functions in service files.
- Do not place service logic in model files, database initialization files, frontend generated files, or Wails entry files.

### Backend Responsibility Boundaries

Keep these backend root directories as the stable project architecture. Subdirectories may expand by domain or workflow, but the root responsibility must not drift:

```text
apiDefinition/   # 外部 API 契约层
constant/        # 全局常量与枚举层
core/            # 基础设施层
model/           # 数据库模型层
service/         # 核心业务层
helper/          # 业务辅助层
utils/           # 通用工具层
```

- `apiDefinition/` describes external API contracts only. Put API paths, request parameters, third-party response DTOs, and URL builders here. Do not send HTTP requests, read databases, or make business decisions here.
- API paths must be defined in `apiDefinition/` as documented constants, never inside `service/` or `helper/`. Static paths should follow the documented constant style shown by `APIURLPackageList` and `APIURLDownloadRecord`: the constant only stores a relative path, and its Chinese comment explains `职责`、`使用`、`边界`. Dynamic path parameters must be expressed in the path template as `/api/{{UserID}}/{{TestID}}`; callers should pass a struct or struct pointer to the shared `utils.ResolveAPIPath` helper, which replaces exported fields with matching placeholder names through reflection. Query strings, request timing, error policy, and HTTP execution stay in service/helper workflows, not in API constants.
- `constant/` contains stable constants, enum-like values, event names, state names, source names, and shared keys. Use it to avoid duplicated magic strings in service, helper, model, and frontend-facing DTO code.
- `core/` owns application infrastructure initialization only: database lifecycle, HTTP client construction, global runtime context, configuration units, infrastructure paths, and version metadata. Do not place Mod installation, marketplace queries, logging workflows, or other business behavior here.
- `model/` contains GORM persistence models and model registration only. Do not place third-party API responses, frontend DTOs, DTO conversion, or service behavior in model packages.
- `service/` is the core business owner. Put input validation, workflow orchestration, transaction boundaries, state transitions, calls into helper/model/core, and the decision of when to record operation logs here. Wails-bound business methods should be service methods.
- `helper/` assists services with implementation details. It may contain file operations, archive extraction, path resolution, database query helpers, DTO conversion helpers, log builders, progress event helpers, and validation helpers. Helpers must not own complete business use cases.
- `utils/` contains pure, reusable, dependency-light utilities. Prefer standard library dependencies only. Do not import `core`, `model`, `service`, or `helper` from `utils`.

Use this dependency direction unless a task explicitly redesigns the architecture:

```text
service  -> helper / model / core / apiDefinition / constant / utils
helper   -> model / core / apiDefinition / constant / utils
core     -> model / constant / utils
model    -> constant
utils    -> standard library first; no business package dependencies
constant -> no business package dependencies
```

Reference backend subdirectory design for new or refactored code. This is an architecture example, not a mandatory file checklist. Use it to guide naming, responsibility boundaries, and future expansion; when implementing a concrete feature, keep only the subdirectories and files that the current domain genuinely needs.

```text
apiDefinition/
  modDependence/
    official_api.go
    thunderstore_api.go
    download_api.go
    official_dto.go
    thunderstore_dto.go
  system/
    update_api.go

constant/
  modDependence/
    source.go
    install_state.go
    install_stage.go
    artifact_kind.go
    event.go
  system/
    config.go
    log.go
    update.go

core/
  global.go
  database.go
  configure.go
  http_client.go
  version.go
  runtime.go

model/
  enter.go
  system/
    enter.go
    config.go
    log.go
  modDependence/
    enter.go
    dependency.go
    dependency_package.go
    installed_package.go
    installed_component.go
    installed_file.go
    remote_package_cache.go

service/
  enter.go
  system/
    enter.go
    configuration_service.go
    environment_service.go
    log_service.go
    update_service.go
    startup_service.go
    dto.go
    errors.go
  modDependence/
    enter.go
    marketplace_service.go
    installer_service.go
    bep_in_ex_service.go
    headless_service.go
    dto.go
    errors.go
    marketplace/
      provider.go
      official_provider.go
      thunderstore_provider.go
      manual_provider.go
    installer/
      remote_install.go
      legacy_install.go
      uninstall.go
      conflict.go
      session.go
      progress.go
      artifact.go
      result.go

helper/
  system/
    audit.go
    log_builder.go
    config_reflection.go
  modDependence/
    audit/
      bep_in_ex.go
      headless.go
      install.go
    store/
      package_store.go
      component_store.go
      file_store.go
      cache_store.go
      install_session_store.go
    installer/
      remote_install.go
      legacy_install.go
      uninstall.go
      conflict.go
      session.go
      progress.go
    artifact/
      resolver.go
      layout.go
      validator.go
    marketplace/
      provider.go
      official.go
      thunderstore.go
    file/
      copy.go
      backup.go
      cleanup.go
    archive/
      zip.go
      extract.go
    bepInEx/
      install.go
      uninstall.go
      status.go
      config.go
    headless/
      environment.go
      process.go
      settings.go
      network.go
    progress/
      event.go
      snapshot.go
  updater/
    gitee_source.go
    release_validator.go
    self_update.go

utils/
  file.go
  hash.go
  path.go
  slice.go
  string.go
  steam.go
  shortcut.go
  dyson_sphere.go
```

When deciding where a new type or function belongs, use this rule: service tells the business story, helper hides implementation detail, utils provides business-agnostic primitives, model describes persisted data shape, apiDefinition describes external contracts, and constant centralizes stable shared names. Do not create empty directories or placeholder files merely to match the reference design.

Use automatic imports where configured:

- Use Naive UI tags such as `<n-button>` and `<n-card>` directly in Vue templates.
- Use file-based route pages instead of manually maintaining route arrays.
- For component subdirectories, remember `directoryAsNamespace: true`: `src/components/universe/hi.vue` is used as `<UniverseHi />`.

## Coding Rules

Keep code modular and readable:

- Use `<script setup lang="ts">` for Vue SFCs.
- Use `definePage({ meta })` for route layout, title, and `keepAlive` flags.
- Use Tailwind utility classes first for layout, spacing, color, typography, and responsive behavior.
- Use SCSS for reusable selectors, complex local styling, or styles that are awkward as utilities.
- Use Naive UI components before building custom controls.
- Prefer flat UI with fewer cards, very small radii, subtle borders, and clear full-width sections. Avoid card-heavy pages and nested cards.
- Add comments only for non-obvious module boundaries, business rules, Wails bridge behavior, or complex layout decisions.
- Avoid obvious comments such as "set value" or "render button".
- Use named, focused files. Split files before they become mixed-purpose.
- Design code for reasonable reuse and ease of use. Public service methods, helper functions, utilities, DTOs, and constants should have clear names, narrow responsibilities, predictable inputs and outputs, and extension points that support likely future features without over-engineering the current task.
- Prefer general, composable helpers over one-off local implementations when the behavior is not tied to a single function. Keep the call site easy to read, and avoid hiding important business decisions behind overly generic helpers.

## Commenting Standards

All newly written or modified comments in this repository must be written in Chinese. Comments are part of the project contract: they should explain the purpose, usage, dependencies, and boundary behavior of a module or API so that future maintainers can safely extend it.

### Go Comment Rules

Backend Go code must follow Go documentation comment conventions while keeping the content Chinese. The backend comment standard is strict: every meaningful declaration must explain `职责`、`使用`、`边界`; do not rely on names alone as documentation.

- Every Go file must include a Chinese file-level responsibility comment. The comment must cover three points: `职责`（this file/module owns what）, `使用`（which layer or caller should use the functions, structs, variables, or constants in this file and in what scenario）, and `边界`（what this file must not do, what failure/fallback behavior matters, and which responsibilities belong elsewhere). Place the file-level responsibility comment immediately after the `package` declaration or before the first related declaration.
- Each meaningful module inside a file must have a nearby Chinese comment when the file contains more than one responsibility block, such as DTO definitions, persistence conversion, filesystem cleanup, progress emission, audit log construction, or Wails event bridging. The module comment must include the same three elements: `职责`、`使用`、`边界`; it should not narrate each line.
- Every constant, variable, type, struct, interface, function, method, helper, service struct, core infrastructure object, model registration group, DTO, and package-level declaration must have a Chinese comment with `职责`、`使用`、`边界`. This applies to exported and unexported declarations.
- Every exported package-level constant, variable, type, field, function, and method must have a Go doc comment.
- The comment for an exported identifier must start with that identifier name, followed by a Chinese explanation that includes its responsibility, how callers should use it, and its boundary/fallback behavior when relevant.
- Unexported identifiers do not need to start with the identifier name, but they still need a nearby Chinese comment explaining `职责`、`使用`、`边界`. This includes small encapsulated helpers when they hide file operations, persistence behavior, path rules, cache behavior, log construction, request handling, or state transitions.
- Grouped `const` and `var` blocks must have a group-level comment explaining the module responsibility, and each constant or variable must have its own Chinese comment when it represents a distinct business value, state, path, event, source, key, or runtime object.
- Every struct and interface must have a Chinese comment explaining `职责`、`使用`、`边界`. Struct fields must also be commented when they are DTO fields, GORM model fields, configuration fields, JSON-bound fields, Wails-facing fields, or non-obvious internal state.
- Every Service method and service struct must document `职责`、`使用`、`边界`, including which frontend page or service workflow should call it, what helper/model/core dependencies it may use, and what validation or error behavior callers should expect.
- Every core package declaration must document `职责`、`使用`、`边界`, especially initialization order, global state ownership, lifecycle, fallback behavior, and what business logic is forbidden in `core`.
- Complex logic must include a short Chinese logic-flow comment. Use it when a function or module spans multiple steps such as request/response handling, remote data loading, transaction workflows, file backup/rollback, conflict recovery, progress events, or cleanup. The flow should be concise and ordered, for example `逻辑流：1. 调用远端请求 => 2. 解析响应数据 => 3. 写入本地缓存 => 4. 返回统一 DTO 给调用方`。Keep the flow at the module/function boundary; do not annotate every line.
- Logic-flow comments must describe the actual data/control path, including important failure branches when they affect behavior, such as fallback to cache, rollback files, skip shared dependencies, or return a typed error for frontend handling.
- Comments must explain what the item does, when to use it, and any important boundary behavior. Do not write empty comments such as "获取数据" when the data source, caller, fallback, side effects, or failure behavior matters.
- Comments must not be decorative or empty. A comment such as `// 获取数据`, `// 工具函数`, `// 设置变量`, or a mere English-to-Chinese restatement of the identifier is not acceptable unless it also states responsibility, usage, and boundary.
- Keep Go comments close to the declaration and let `gofmt` format the file.

Example:

```go
package modDependence

// 文件职责：维护 Thunderstore 安装会话中的文件快照与回滚文件操作。
// 使用：由远端安装器在安装取消、冲突恢复、失败清理或文件归属规则变化时调用。
// 边界：不负责下载、解压、市场查询或服务层日志决策。
// 逻辑流：1. 读取安装清单 => 2. 定位已写入文件 => 3. 按备份状态恢复或删除 => 4. 更新清单并交给上层记录结果。
```

```go
// EnvironmentService 提供前端可调用的运行环境检测能力。
// 使用：由设置页和启动页调用，用于定位戴森球计划安装目录、校验游戏启动文件是否存在。
// 边界：找不到路径时返回空字符串而不是猜测无效目录。
type EnvironmentService struct{}

// LoadOrInitDysonSphereExe 加载或初始化戴森球计划安装目录。
// 使用：前端点击“自动获取”时调用；优先读取已保存配置，再从 Steam 与快捷方式查找。
// 返回：找到时返回包含 DSPGAME.exe 的目录；未找到时返回空字符串。
func (model *EnvironmentService) LoadOrInitDysonSphereExe() (dirPath string, err error) {
	// ...
}
```

### Frontend Script Comment Rules

Vue and TypeScript frontend code must use modular Chinese comments for major script sections. Use separator comments when a file has multiple responsibilities such as state, computed values, lifecycle, event handlers, API calls, and formatting helpers.

- Use `// =============== 模块名称 ===============` to mark script modules.
- Each module should include a short Chinese comment explaining what it owns and how other code should use it.
- Add function-level Chinese comments for user actions, API bridges, data normalization, and failure handling.
- Avoid commenting every line. Comment the module boundary and non-obvious decisions.

Example:

```ts
// =============== 游戏路径状态 ===============
// 说明：维护输入框路径、已保存路径快照与保存按钮显隐状态。
// 使用：用户选择或自动获取路径后，只更新输入框；点击确认后才写入配置。
const savedGamePath = ref(gamePath.value)
const hasPendingGamePathChange = computed(() => gamePath.value.trim() !== savedGamePath.value.trim())

// =============== 游戏路径保存 ===============
// 说明：将输入框路径同步到后端 Game/StartupFilePath 配置。
// 边界：保存失败时保留输入内容，并通过弹窗提示错误。
const confirmGamePath = async (): Promise<void> => {
	// ...
}
```

### Vue Template Comment Rules

Vue page and component templates must use Chinese HTML comments before meaningful UI modules. Use this pattern for page-level sections, complex controls, modal dialogs, conditionally rendered states, and repeated lists:

```vue
<!-- ~ 功能模块 -->
<!-- * 说明：展示当前沟通绑定的简历摘要卡片 -->
<!-- * 交互：首屏优先展示骨架，详情返回后展示简历名称与摘要 -->
<!-- * 依赖：依赖 GET_RESUME_DETAIL 与 ResumeId 路由参数 -->
<!-- * 边界：简历详情失败时兜底显示基础标题，不阻塞消息列表渲染 -->
```

Apply the same structure with content that matches the current UI:

- `说明` describes what this UI module shows or controls.
- `交互` describes user actions, loading states, modal behavior, or navigation.
- `依赖` names important reactive state, backend APIs, route params, props, or generated Wails bindings.
- `边界` explains empty states, failure states, disabled behavior, permission constraints, or fallback rendering.

Keep template comments directly above the relevant block. Do not add comments for tiny one-line static text unless it is part of a larger functional module.

## Backend Rules

Follow these rules strictly when changing Go backend code:

- `core/database.go` owns database lifecycle only: resolve runtime database path, copy the embedded `app.db` template when needed, open GORM, set SQLite pragmas, call `AutoMigrate`, and close the connection. Do not put business queries or model definitions there.
- Keep `core/global.go` as a small global holder. It may expose `GLOBAL.DB` and infrastructure paths, but it must not contain initialization logic or business state.
- Use `core.GLOBAL.DB` for database operations after startup. Do not create ad-hoc database connections in services.
- Use GORM APIs for normal CRUD (`Create`, `First`, `Find`, `Save`, `Updates`, `Delete`, `Transaction`). Use raw SQL only when GORM cannot express the operation clearly.
- For SQLite JSON fields, use `datatypes.JSON` from `gorm.io/datatypes`; do not store JSON as untyped strings unless the field is intentionally plain text.
- For frontend-facing Wails methods, return DTOs when GORM models contain `time.Time`, internal fields, or database-only tags. Avoid exposing persistence models directly when it would generate weak TypeScript types.
- Run `wails generate module` after adding, removing, renaming, or changing exported methods on bound services.
- Before adding a generic utility function, inspect `utils/` first with `rg` or `rg --files utils`. Reuse existing utilities when they match the need instead of writing a new local utility inside the current file.
- If no suitable generic utility exists, create a focused utility in `utils/<topic>.go` under package `utils`. Keep utilities generic and dependency-light; do not import service, model, or core packages from `utils`.
- Before adding a service-specific helper function, inspect `helper/` first with `rg` or `rg --files helper`. Reuse or extend an existing helper when it already owns the same concern instead of creating a parallel helper with overlapping behavior.
- Create a new helper only when no existing helper fits the concern cleanly. Helpers may support service methods with DTO conversion, log event construction, parsing, filesystem checks, reusable validation, or small persistence helpers, but the service method remains the business workflow owner.

### Model Registration Pattern

Every model domain package must have this shape:

```go
package someDomain

type ModelGroup struct {
	SomeModel
}

func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.SomeModel,
	}
}
```

The root `model/enter.go` must aggregate every domain explicitly:

```go
type ModelGroup struct {
	SystemModelGroup system.ModelGroup
	SomeModelGroup   someDomain.ModelGroup
}

func RegisterModels() []interface{} {
	models := make([]interface{}, 0)
	models = append(models, ModelGroupApp.SystemModelGroup.RegisterModels()...)
	models = append(models, ModelGroupApp.SomeModelGroup.RegisterModels()...)
	return models
}
```

When adding a new model:

- Create or update `model/<domain>/<model>.go`.
- Add the model to `model/<domain>/enter.go`.
- Add the domain group to `model/enter.go` if it is a new domain.
- Let `core/database.go` keep calling only `model.RegisterModels()`.

### Service Pattern

Each service domain should follow the existing `service/<domain>/enter.go` pattern:

- Service is the core business layer. Put orchestration, business decisions, state changes, transaction boundaries, and important operation flow in service methods.
- Helper is only an auxiliary layer for services. Use helper functions to remove repetitive or low-level implementation details, not to hide or replace the service workflow.
- Service files may contain only service structs and methods on those service structs.
- Do not define non-method helper functions in service files.
- Keep methods cohesive by domain; split files by workflow when a service grows.
- Validate input at the service boundary and return Go errors instead of panicking.
- If service logic needs reusable auxiliary behavior, first inspect `helper/`. Reuse an existing helper object or helper function when available.
- If no helper exists, create a focused helper under `helper/<domain>` or `helper/<topic>.go`, then call it from the service method.
- Helper packages may encapsulate DTO conversion, log entry construction, parsing, filesystem checks, reusable validation, and other service-adjacent implementation details.
- Do not move core business workflows into helper packages. Avoid helper APIs named like complete use cases such as `InstallPackageWithLog`, `UpdateAppSelfWithLog`, or `LoadOrInitGamePath` when the service should own that workflow.
- Every service workflow that changes persisted state, runtime state, files, network configuration, process state, installation state, or user-visible system behavior should record an important operation log through helper-layer logging functions.
- Keep the system log schema generic: use `namespace`, `module`, `event`, `level`, and `type` as query dimensions; put user-readable state changes in `content`; put structured details such as port, PID, package ID, path, version, or source in `metadata`.
- Do not add workflow-specific log columns such as `subject`, `stateBefore`, or `stateAfter` unless the user explicitly redesigns the log model.
- Do not hand edit `frontend/wailsjs`; regenerate it with Wails.

## Validation

After frontend or routing changes, run:

```powershell
cd D:\Workspace\DysonSphere\DysonSphereBox\frontend
npm run build
```

After Go backend or Wails binding changes, run the appropriate Wails generation/build flow:

```powershell
cd D:\Workspace\DysonSphere\DysonSphereBox
go build ./...
wails generate module
```

Use `wails dev` when runtime verification is needed.
