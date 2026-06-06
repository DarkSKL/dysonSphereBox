<script setup lang="ts">
import {ref} from 'vue'
import {useWorkshopState} from '../composables/useWorkshopState'

definePage({
  meta: {
    layout: 'main',
    title: '设置',
  },
})

const {
  gamePath,
  downloadServer,
  autoStart,
  checkUpdate,
  isGamePathValid,
  resetGamePath,
} = useWorkshopState()

const clearConfirmVisible = ref(false)
const serverOptions = [
  {label: '国内 CDN (阿里云) - 推荐', value: 'aliyun'},
  {label: 'GitHub Releases', value: 'github'},
  {label: 'Thunderstore', value: 'thunderstore'},
]
</script>

<template>
  <section class="mx-auto max-w-3xl px-8 py-8">
    <h1 class="mb-6 text-xl font-semibold text-zinc-950">设置</h1>

    <div class="space-y-8">
      <section>
        <label class="mb-2 block text-sm font-semibold text-zinc-950">游戏路径</label>
        <div class="flex gap-2">
          <n-input v-model:value="gamePath" placeholder="请选择游戏安装目录" />
          <n-button secondary @click="resetGamePath">选择</n-button>
        </div>
        <p v-if="!isGamePathValid" class="mt-2 text-sm text-red-600">未在该目录下找到 DSPGAME.exe</p>
        <p v-else class="mt-2 text-xs text-zinc-500">已自动检测到游戏路径</p>
      </section>

      <section>
        <label class="mb-2 block text-sm font-semibold text-zinc-950">下载服务器</label>
        <n-select v-model:value="downloadServer" :options="serverOptions" />
        <p class="mt-2 text-xs text-zinc-500">国内服务器下载速度更快，无需加速器。</p>
      </section>

      <section class="flex items-center justify-between gap-6 border-y border-zinc-200 py-5">
        <div>
          <p class="text-sm font-semibold text-zinc-950">开机自启</p>
          <p class="mt-1 text-sm text-zinc-500">Windows 启动时自动运行戴森工坊。</p>
        </div>
        <n-switch v-model:value="autoStart" />
      </section>

      <section class="flex items-center justify-between gap-6 border-b border-zinc-200 pb-5">
        <div>
          <p class="text-sm font-semibold text-zinc-950">启动时检查 Mod 更新</p>
          <p class="mt-1 text-sm text-zinc-500">每次启动时自动检查已安装 Mod 的更新。</p>
        </div>
        <n-switch v-model:value="checkUpdate" />
      </section>

      <section>
        <p class="mb-2 text-sm font-semibold text-zinc-950">清除缓存</p>
        <n-button type="error" secondary @click="clearConfirmVisible = true">清除缓存文件</n-button>
        <p class="mt-2 text-xs text-zinc-500">清除下载缓存，不会删除已安装的 Mod。</p>
      </section>

      <div class="border-t border-zinc-200 pt-4 text-xs text-zinc-500">数据存储路径：%AppData%/dsp-workshop/</div>
    </div>

    <n-modal v-model:show="clearConfirmVisible" preset="dialog" title="确认清除缓存" positive-text="确认清除" negative-text="取消">
      <p class="text-sm text-zinc-500">将清除所有下载缓存文件，已安装的 Mod 不会受影响。</p>
    </n-modal>
  </section>
</template>
