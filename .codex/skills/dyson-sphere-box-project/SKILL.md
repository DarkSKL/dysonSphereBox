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
- Services belong under `service/<domain>`. Each service domain must expose an `enter.go` that registers Wails-bound service structs.
- Reusable utility functions belong under `utils/`. Do not hide general helpers inside `core`, `service`, `model`, or frontend code.
- Service-specific helper objects belong under `helper/`. Do not place free helper functions in service files.
- Do not place service logic in model files, database initialization files, frontend generated files, or Wails entry files.

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

## Commenting Standards

All newly written or modified comments in this repository must be written in Chinese. Comments are part of the project contract: they should explain the purpose, usage, dependencies, and boundary behavior of a module or API so that future maintainers can safely extend it.

### Go Comment Rules

Backend Go code must follow Go documentation comment conventions while keeping the content Chinese:

- Every exported package-level constant, variable, type, field whose meaning is not obvious, function, and method must have a Go doc comment.
- The comment for an exported identifier must start with that identifier name, followed by a Chinese explanation.
- Important unexported functions, methods, variables, and structs must also have Chinese comments when they encode business rules, filesystem behavior, Wails bridge behavior, persistence assumptions, or non-obvious edge cases.
- Struct fields used by frontend DTOs, GORM models, configuration units, or JSON bindings must be commented when the field name alone cannot fully explain its role.
- Comments must explain what the item does, when to use it, and any important boundary behavior. Do not write empty comments such as "获取数据" when the data source, fallback, or failure behavior matters.
- Keep Go comments close to the declaration and let `gofmt` format the file.

Example:

```go
// EnvironmentService 提供前端可调用的运行环境检测能力。
// 用途：定位戴森球计划安装目录、校验游戏启动文件是否存在。
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
- Before adding any helper or utility function, inspect `utils/` first with `rg` or `rg --files utils`. Reuse existing helpers when they match the need.
- If no suitable helper exists, create a focused utility in `utils/<topic>.go` under package `utils`. Keep utilities generic and dependency-light; do not import service, model, or core packages from `utils`.

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

- Service files may contain only service structs and methods on those service structs.
- Do not define non-method helper functions in service files.
- Keep methods cohesive by domain; split files by workflow when a service grows.
- Validate input at the service boundary and return Go errors instead of panicking.
- If service logic needs reusable helper behavior, first inspect `helper/`. Reuse an existing helper object or helper function when available.
- If no helper exists, create a focused helper under `helper/<domain>` or `helper/<topic>.go`, then call it from the service method.
- Helper packages may encapsulate workflow support, parsing, filesystem checks, or other service-adjacent behavior, but must not become Wails-bound service layers.
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
