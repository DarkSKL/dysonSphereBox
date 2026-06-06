---
name: dyson-sphere-box-project
description: Project-level coding and architecture rules for the DysonSphereBox Wails application. Use when editing or creating files in this repository, especially Go backend bindings, Vue 3 TypeScript frontend code, Naive UI components, Tailwind CSS styling, SCSS, Vue Router auto-routes, layouts, KeepAlive behavior, Vite configuration, Wails generated bindings, or project structure.
---

# DysonSphereBox Project

Use this skill for work inside `D:\Workspace\DysonSphere\DysonSphereBox`.

## First Checks

Before editing, inspect the relevant local files instead of assuming current state:

- Root Wails files: `main.go`, `app.go`, `wails.json`, `go.mod`.
- Frontend config: `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/tailwind.config.cjs`, `frontend/postcss.config.cjs`.
- Frontend entry and shell: `frontend/src/main.ts`, `frontend/src/App.vue`, `frontend/src/router.ts`, `frontend/src/style.scss`.
- Existing directories: `frontend/src/pages`, `frontend/src/layouts`, `frontend/src/components`, `frontend/src/types`, `frontend/wailsjs`.

Read `references/project-standards.md` when implementing or reviewing non-trivial changes.

## Installed Stack

Prefer the libraries already installed in this project:

- Wails v2 for Go desktop app integration.
- Vue 3 with Composition API and TypeScript.
- Vite 3 for frontend build/dev.
- Naive UI for UI components. The user may call it `Native-UI`; treat that as `naive-ui` unless the package actually changes.
- `unplugin-vue-components` with `NaiveUiResolver` for automatic Naive UI component imports.
- Vue Router 4 and `unplugin-vue-router` for file-based auto-routes in `frontend/src/pages`.
- Tailwind CSS 3 for utility styling.
- SCSS via `sass` for component-level or global styles that utilities do not express cleanly.
- PostCSS and Autoprefixer for Tailwind processing.
- `vue-tsc` for type checking.

Do not add new UI, routing, styling, or state libraries unless the task clearly requires capability not covered by the installed stack.

## Hard Structure Rules

Separate files by responsibility. Do not put unrelated layout, page, service, type, and API logic into one file.

- Route pages belong in `frontend/src/pages`.
- Shared layouts belong in `frontend/src/layouts`.
- Reusable Vue components belong in `frontend/src/components`.
- Shared TypeScript declarations belong in `frontend/src/types`.
- Frontend route setup belongs in `frontend/src/router.ts`.
- Global styles and Tailwind directives belong in `frontend/src/style.scss`.
- Wails generated files under `frontend/wailsjs` are generated output. Do not hand edit them.
- Go methods exposed to frontend must be exported methods on a bound struct, usually `App` in `app.go`.

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

## Validation

After frontend or routing changes, run:

```powershell
cd D:\Workspace\DysonSphere\DysonSphereBox\frontend
npm run build
```

After Go backend or Wails binding changes, run the appropriate Wails generation/build flow:

```powershell
cd D:\Workspace\DysonSphere\DysonSphereBox
wails generate module
```

Use `wails dev` when runtime verification is needed.
