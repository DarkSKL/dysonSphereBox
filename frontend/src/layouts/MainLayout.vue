<script setup lang="ts">
import {computed} from 'vue'
import {useRoute} from 'vue-router'

const route = useRoute()

const navItems = [
  {label: '首页', to: '/', icon: '⌂'},
  {label: 'Mod', to: '/mod', icon: '▣'},
  {label: '设置', to: '/settings', icon: '⚙'},
  {label: '关于我们', to: '/about', icon: 'i'},
]

const title = computed(() => route.meta.title || 'DysonSphereBox')
</script>

<template>
  <div class="h-screen overflow-hidden bg-[#fbfbfc] text-zinc-950">
    <aside class="fixed inset-y-0 left-0 z-20 flex w-48 flex-col border-r border-zinc-200 bg-[#f7f7f8]">
      <div class="flex h-16 items-center justify-between border-b border-zinc-200 px-4">
        <RouterLink to="/" class="min-w-0">
          <p class="truncate text-lg font-semibold text-zinc-950">戴森工坊</p>
          <p class="text-[11px] uppercase tracking-[0.18em] text-zinc-400">DSP Workshop</p>
        </RouterLink>
      </div>

      <nav class="grid gap-1 p-2">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="flex h-10 items-center gap-3 rounded border border-transparent px-3 text-sm font-medium text-zinc-700 transition hover:border-zinc-200 hover:bg-white hover:text-zinc-950"
          active-class="border-blue-200 bg-blue-50 text-blue-700 shadow-sm hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700"
        >
          <span class="grid h-4 w-4 place-items-center text-[13px] leading-none">{{ item.icon }}</span>
          <span class="truncate">{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="mt-auto border-t border-zinc-200 p-3">
        <div class="rounded border border-zinc-200 bg-white px-3 py-2">
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs text-zinc-500">运行状态</span>
            <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
          </div>
          <p class="mt-1 text-xs text-zinc-800">本地 UI 预览</p>
        </div>
      </div>
    </aside>

    <section class="flex h-screen flex-col pl-48">
      <header class="flex h-16 shrink-0 items-center justify-between border-b border-zinc-200 bg-white px-8">
        <div>
          <p class="text-sm font-semibold text-zinc-950">{{ title }}</p>
          <p class="text-xs text-zinc-500">Dyson Sphere Program Mod Manager</p>
        </div>
        <div class="flex items-center gap-2">
          <span class="rounded border border-zinc-200 bg-zinc-50 px-2 py-1 text-xs text-zinc-500">v1.0.0</span>
        </div>
      </header>

      <main class="min-h-0 flex-1 overflow-auto">
        <slot />
      </main>
    </section>
  </div>
</template>
