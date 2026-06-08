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

Helper packages assist services:

- Put reusable support code in `helper/<domain>` or `helper/<topic>.go`.
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

Use comments sparingly and only when they add structural knowledge:

- Module purpose when filename does not make intent obvious.
- Wails bridge behavior.
- Route/layout decisions.
- Non-obvious caching or KeepAlive behavior.
- Complex styling reason that Tailwind classes alone do not communicate.

Avoid comments that narrate obvious code.

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
