# DysonSphereBox Project Standards

## Project Inventory

Backend:

- Wails v2 desktop application.
- Go module root contains `main.go`, `app.go`, `go.mod`, `go.sum`, and `wails.json`.
- Bound backend methods currently live on `App` and are exposed through generated frontend bindings in `frontend/wailsjs`.

Frontend:

- Vue 3 Composition API with TypeScript.
- Vite 3.
- Naive UI, used through `unplugin-vue-components` and `NaiveUiResolver`.
- Vue Router 4 with `unplugin-vue-router`.
- Tailwind CSS 3 with `tailwind.config.cjs`.
- SCSS through `sass`.
- PostCSS with `postcss.config.cjs`.
- Type checking with `vue-tsc`.

## Directory Contract

Use this structure when adding code:

```text
frontend/src/
  App.vue                 # Layout/router shell only
  main.ts                 # Vue app bootstrap only
  router.ts               # Router instance only
  style.scss              # Tailwind directives and global styles
  assets/                 # Static images/fonts used by frontend
  components/             # Reusable, page-agnostic Vue components
  layouts/                # Layout shells selected by route meta
  pages/                  # File-based routes only
  services/               # Frontend API/Wails bridge wrappers
  composables/            # Reusable Vue composition logic
  types/                  # Shared TypeScript declarations
  utils/                  # Pure utility functions
```

Create `services`, `composables`, or `utils` only when needed. Do not put service calls, heavy business logic, and page rendering in the same SFC.

## Frontend Patterns

### Vue SFCs

Use this order:

```vue
<script setup lang="ts">
</script>

<template>
</template>

<style scoped lang="scss">
</style>
```

For pure layout files with no local state, omit empty script blocks only if no TypeScript logic is needed.

### Pages

Pages belong in `frontend/src/pages` and are auto-routed.

Examples:

```text
pages/index.vue       -> /
pages/dashboard.vue   -> /dashboard
pages/users/[id].vue  -> /users/:id
```

Prefer `definePage({ meta })`:

```ts
definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: 'Dashboard',
  },
})
```

Use:

- `layout: 'main'` for normal application pages.
- `layout: 'blank'` for login, splash, full-screen tools, and isolated pages.
- `keepAlive: true` only when preserving local state is useful.

### Layouts

Layouts belong in `frontend/src/layouts`.

Layouts should contain navigation, chrome, panels, and shared page framing. They should not contain page-specific business logic.

### Components

Components belong in `frontend/src/components`.

Because `directoryAsNamespace: true` is enabled:

```text
src/components/universe/hi.vue -> <UniverseHi />
src/components/user/avatar.vue -> <UserAvatar />
```

Keep components page-agnostic. If a component only belongs to one page and is not reused, place it near the page only after creating a clear subdirectory pattern.

## Styling Rules

Visual direction:

- Prefer flat, quiet application UI.
- Reduce card usage. Do not make every section a card.
- Never place cards inside cards unless the user explicitly asks for nested framed content.
- Prefer full-width bands, bordered rows, tables, split panes, toolbars, and clear section dividers over floating card grids.
- Use extremely small radii by default: prefer `rounded-sm` or `rounded`; avoid `rounded-lg`, `rounded-xl`, `rounded-2xl`, and pill shapes unless a component truly needs them.
- Prefer subtle borders such as `border border-slate-800` over heavy shadows.
- Avoid decorative glassmorphism, large soft shadows, blob backgrounds, and over-styled panels.
- Keep the visual system flat: color, spacing, typography, and borders should carry hierarchy before elevation.

Use Tailwind CSS first for:

- Layout: `grid`, `flex`, `min-h-screen`, `max-w-*`, spacing, alignment.
- Color and typography utilities.
- Responsive states.
- Simple hover/focus transitions.

Use Naive UI for controls:

- Buttons, cards, forms, inputs, dialogs, tables, tabs, layout controls.
- Prefer `<n-button>`, `<n-card>`, `<n-input>`, `<n-space>`, and related components before custom HTML controls.
- Use `<n-card>` sparingly. Prefer it for isolated content blocks, dialogs, settings groups, and repeated items that genuinely need a frame.
- When using `<n-card>`, keep its visual style restrained and avoid placing another card inside it.

Use SCSS for:

- Complex local selectors.
- Repeated component styling.
- CSS that is clearer as a named class than a long utility list.

Do not duplicate Tailwind config as `.js` and `.cjs`. This project uses `tailwind.config.cjs` and `postcss.config.cjs`.

## Wails Backend Rules

Expose backend functions as exported methods on a bound Go struct:

```go
func (a *App) ReadBoxName(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("id cannot be empty")
	}

	return "Box " + id, nil
}
```

After adding or changing exposed methods, regenerate frontend bindings:

```powershell
wails generate module
```

Do not hand edit:

```text
frontend/wailsjs/go/**
frontend/wailsjs/runtime/**
```

Create frontend wrappers in `frontend/src/services` if a backend method is used from multiple pages or needs conversion/error handling.

## Backend Service And Helper Boundaries

Backend service packages are the core business layer:

- Put business orchestration in `service/<domain>` methods.
- Keep state changes, transaction decisions, important operation flow, and service-level validation in service methods.
- Keep service files limited to service structs and methods on those structs.
- Do not add package-level free helper functions to service files.

All backend package directories should remain single-responsibility:

- Apply directory-level single responsibility across `apiDefinition/`, `constant/`, `core/`, `model/`, `service/`, `helper/`, and `utils/`, not only inside helpers.
- When any backend directory accumulates many files or clearly contains multiple concerns, split it into concern subdirectories instead of leaving a wide flat folder.
- Examples: `helper/modDependence/audit/` for log/audit helpers, `helper/modDependence/installer/` for installer implementation details, `service/modDependence/marketplace/` for marketplace service workflows, `apiDefinition/modDependence/thunderstore/` for Thunderstore contracts, `model/modDependence/installed/` for installed-record models, `constant/modDependence/install/` for install state/stage constants, or `utils/path/` when generic path utilities grow into multiple files.
- A subdirectory must have one responsibility and one reason to change. If it grows into multiple concerns, split it one level deeper rather than adding broad catch-all files.

API definitions:

- Put every external API path, request DTO, response DTO, and API-only URL builder under `apiDefinition/`; do not define API paths in `service/` or `helper/`.
- Define static API paths as documented constants. Use the `APIURLPackageList` / `APIURLDownloadRecord` style: each constant stores only a relative path and has Chinese `职责`、`使用`、`边界` comments.
- Define dynamic path parameters in templates such as `/api/{{UserID}}/{{TestID}}`. The placeholder name must match an exported field name on the parameter struct passed by the caller.
- Use `utils.ResolveAPIPath(pathTemplate, params)` to replace dynamic path parameters before sending a request. The utility owns reflection-based field lookup and path escaping; query strings and HTTP request behavior remain the caller's responsibility.

Helper packages assist services:

- Put reusable support code in `helper/<domain>` or `helper/<topic>.go`.
- Do not keep unrelated helper workflows flat in one domain directory after their concerns are clear. For example, `helper/modDependence/audit/` should contain only audit/log event helpers; installer file copy, marketplace HTTP queries, BepInEx package extraction, and headless process control belong in their own concern directories.
- Use helpers for DTO conversion, log entry construction, parsing, filesystem checks, reusable validation, and small implementation details.
- Do not move complete service workflows into helpers.
- Avoid helper names that represent full use cases when the service should own the flow, such as `InstallPackageWithLog`, `UpdateAppSelfWithLog`, or `LoadOrInitGamePath`.

Important operation logging:

- Service workflows that change persisted state, runtime state, files, network configuration, process state, installation state, or user-visible system behavior should record system logs.
- Build log entries through helper-layer logging functions, but keep the business decision to log in the service workflow.
- Keep the log schema generic: query by `namespace`, `module`, `event`, `level`, and `type`; write user-facing state descriptions into `content`; put structured details in `metadata`.
- Do not add workflow-specific log columns such as `subject`, `stateBefore`, or `stateAfter` unless the user explicitly redesigns the log model.

## TypeScript Rules

Use strict, explicit TypeScript:

- Avoid `any`; use `unknown`, generics, or explicit interfaces.
- Define shared route meta declarations in `frontend/src/types`.
- Define props and emitted events explicitly when a component receives inputs.
- Keep exported functions and services typed with explicit return types.
- Keep generated declaration files such as `components.d.ts` and `typed-router.d.ts` generated.

## Comments

Use comments to preserve structural knowledge while following Go documentation conventions. Backend comments are mandatory for every meaningful declaration and should consistently explain three things: `职责`（what this file/module/identifier owns）, `使用`（who should call or depend on it, and in what scenario）, and `边界`（what it must not do, what fallback/failure behavior matters, and which responsibility belongs elsewhere）.

- Every Go file must include a Chinese file-level responsibility comment covering `职责`、`使用`、`边界`.
- When a file contains multiple meaningful blocks, add Chinese module comments near each block, such as DTO definitions, persistence conversion, filesystem cleanup, progress emission, audit log construction, Wails event bridging, or platform-specific behavior. Module comments must cover `职责`、`使用`、`边界`.
- Every constant, variable, type, struct, interface, function, method, helper, service struct, core infrastructure object, model registration group, DTO, and package-level declaration must have a Chinese comment with `职责`、`使用`、`边界`. This applies to exported and unexported declarations.
- Exported Go identifiers must keep normal Go doc style: the comment starts with the identifier name and explains responsibility, caller usage, and boundary/fallback behavior in Chinese.
- Unexported identifiers do not need to start with the identifier name, but they still need a nearby Chinese comment explaining `职责`、`使用`、`边界`. This includes small encapsulated helpers when they hide file operations, persistence behavior, path rules, cache behavior, log construction, request handling, or state transitions.
- Grouped `const` and `var` blocks must have a group-level comment explaining the module responsibility, and each constant or variable must have its own Chinese comment when it represents a distinct business value, state, path, event, source, key, or runtime object.
- Every struct and interface must have a Chinese comment explaining `职责`、`使用`、`边界`. Struct fields must also be commented when they are DTO fields, GORM model fields, configuration fields, JSON-bound fields, Wails-facing fields, or non-obvious internal state.
- Every Service method and service struct must document `职责`、`使用`、`边界`, including which frontend page or service workflow should call it, what helper/model/core dependencies it may use, and what validation or error behavior callers should expect.
- Every core package declaration must document `职责`、`使用`、`边界`, especially initialization order, global state ownership, lifecycle, fallback behavior, and what business logic is forbidden in `core`.
- Complex logic must include a short Chinese logic-flow comment. Use it for request/response handling, remote data loading, transaction workflows, file backup/rollback, conflict recovery, progress events, cleanup, and any function or module whose behavior is easier to understand as ordered steps.
- Logic-flow comments should use concise ordered steps, for example `逻辑流：1. 调用远端请求 => 2. 解析响应数据 => 3. 写入本地缓存 => 4. 返回统一 DTO 给调用方`。Include important failure branches when they define behavior, such as fallback to cache, rollback files, skip shared dependencies, or returning a typed error for frontend handling.
- Keep logic-flow comments at the file/module/function boundary. Do not add line-by-line narration or duplicate what the code already says.
- Route/layout decisions, non-obvious caching or KeepAlive behavior, and complex styling reasons should also be commented in Chinese when they affect future maintenance.
- Comments must not be decorative or empty. A comment such as `// 获取数据`, `// 工具函数`, `// 设置变量`, or a mere English-to-Chinese restatement of the identifier is not acceptable unless it also states responsibility, usage, and boundary.

Avoid comments that narrate obvious code. A good comment explains ownership and boundaries; a weak comment merely repeats the next statement.

Go examples:

```go
package modDependence

// 文件职责：维护 Thunderstore 安装会话中的文件快照与回滚文件操作。
// 使用：由远端安装器在安装取消、冲突恢复、失败清理或文件归属规则变化时调用。
// 边界：不负责下载、解压、市场查询或服务层日志决策。
// 逻辑流：1. 读取安装清单 => 2. 定位已写入文件 => 3. 按备份状态恢复或删除 => 4. 更新清单并交给上层记录结果。
```

## Verification Checklist

Before finishing frontend work:

- `npm run build` passes in `frontend`.
- Tailwind utilities used in templates are visible in output or runtime.
- Auto-generated `components.d.ts` and `typed-router.d.ts` are refreshed when relevant.
- New pages are under `src/pages` and include appropriate route meta.
- New shared code is placed in `components`, `layouts`, `services`, `composables`, `types`, or `utils`, not mixed into random files.

Before finishing backend/Wails work:

- Exposed methods are exported and attached to a bound struct.
- `wails generate module` is run when bindings change.
- Frontend calls generated bindings through a service wrapper when reused.
