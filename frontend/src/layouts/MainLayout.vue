1024<template>
  <div class="min-h-screen bg-slate-950 text-slate-100">
    <aside class="fixed inset-y-0 left-0 hidden w-60 border-r border-slate-800 bg-slate-950/95 p-5 md:block">
      <div class="mb-8 text-left">
        <p class="text-xs uppercase tracking-widest text-slate-500">DysonSphereBox</p>
        <h1 class="mt-2 text-xl font-semibold text-white">Workbench</h1>
      </div>

      <nav class="grid gap-2">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="rounded-md px-3 py-2 text-left text-sm text-slate-300 transition hover:bg-slate-900 hover:text-white"
          active-class="bg-slate-800 text-white"
        >
          {{ item.label }}
        </RouterLink>
      </nav>
    </aside>

    <section class="min-h-screen md:pl-60">
      <header class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950/90 px-5 py-4 backdrop-blur">
        <div class="mx-auto flex max-w-5xl items-center justify-between">
          <span class="text-sm text-slate-400">{{ title }}</span>
          <n-button size="small" quaternary @click="$router.back()">Back</n-button>
        </div>
      </header>

      <main class="mx-auto max-w-5xl p-5">
        <slot />
      </main>
    </section>
  </div>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useRoute} from 'vue-router'

const route = useRoute()

const navItems = [
  {label: 'Home', to: '/'},
  {label: 'Dashboard', to: '/dashboard'},
  {label: 'Plain Page', to: '/plain'},
]

const title = computed(() => route.meta.title || 'DysonSphereBox')
</script>
