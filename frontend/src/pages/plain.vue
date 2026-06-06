<script setup lang="ts">
definePage({
  meta: {
    layout: 'blank',
    title: 'Plain Page',
  },
})
</script>

<template>
  <section class="mx-auto grid min-h-[calc(100vh-4rem)] max-w-2xl place-items-center">
    <n-card title="Blank Layout">
      <div class="grid gap-4 text-left">
        <p class="text-slate-700">
          这个页面使用 blank layout，适合登录页、欢迎页、全屏工具页。
        </p>
        <RouterLink to="/">
          <n-button type="primary">Back Home</n-button>
        </RouterLink>
      </div>
    </n-card>
  </section>
</template>
