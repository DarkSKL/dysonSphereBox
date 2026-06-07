<script setup lang="ts">
import {ref} from 'vue'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: '缓存测试',
  },
})

const count = ref(0)
</script>

<template>
  <section class="grid gap-4">
    <n-card title="页面缓存测试">
      <div class="grid gap-4 text-left">
        <p class="text-slate-700">
          这个页面开启了 KeepAlive。点击增加计数后切到其他页面再回来，数值会保留。
        </p>
        <div class="flex items-center gap-3">
          <n-button type="primary" @click="count++">增加计数</n-button>
          <span class="text-lg font-semibold text-sky-500">{{ count }}</span>
        </div>
      </div>
    </n-card>
  </section>
</template>
