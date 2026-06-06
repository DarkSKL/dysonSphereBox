<script setup lang="ts">
definePage({
  meta: {
    layout: 'main',
    title: '关于我们',
  },
})

const links = [
  {label: 'github.com/dyson-workshop/dsp-mod-manager', href: 'https://github.com/dyson-workshop/dsp-mod-manager'},
  {label: 'GitHub Issues', href: 'https://github.com/dyson-workshop/dsp-mod-manager/issues'},
  {label: 'support@dyson-workshop.com', href: 'mailto:support@dyson-workshop.com'},
]

const credits = [
  ['Mod 社区', 'Thunderstore'],
  ['联机框架', 'Nebula Multiplayer 团队'],
  ['Mod 框架', 'BepInEx 项目'],
  ['Mod 作者', 'AssemblerUI、BottleneckUI、DSPCalculator、FactoryLocator、CAAP_N'],
]
</script>

<template>
  <section class="mx-auto max-w-3xl px-8 py-8">
    <h1 class="mb-6 text-xl font-semibold text-zinc-950">关于我们</h1>

    <div class="grid gap-8">
      <section>
        <p class="text-sm text-zinc-500">版本</p>
        <p class="mt-1 text-zinc-950">v1.0.0</p>
      </section>

      <section>
        <p class="text-sm text-zinc-500">开发者</p>
        <p class="mt-1 text-zinc-950">戴森工坊团队</p>
      </section>

      <section>
        <p class="mb-3 text-sm text-zinc-500">项目主页与反馈</p>
        <div class="grid gap-3">
          <a
            v-for="link in links"
            :key="link.href"
            :href="link.href"
            target="_blank"
            class="flex items-center justify-between rounded border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700 transition hover:border-zinc-950 hover:text-zinc-950"
          >
            <span class="truncate">{{ link.label }}</span>
            <span class="ml-3 text-xs text-zinc-400">↗</span>
          </a>
        </div>
      </section>

      <section>
        <p class="mb-3 text-sm text-zinc-500">鸣谢</p>
        <div class="divide-y divide-zinc-100 border-y border-zinc-200">
          <div v-for="item in credits" :key="item[0]" class="grid grid-cols-[8rem_1fr] gap-4 py-3 text-sm">
            <span class="text-zinc-500">{{ item[0] }}</span>
            <span class="text-zinc-800">{{ item[1] }}</span>
          </div>
        </div>
      </section>

      <section class="border-t border-zinc-200 pt-6">
        <p class="mb-2 text-sm text-zinc-500">免责声明</p>
        <p class="text-sm leading-7 text-zinc-500">
          本工具仅提供 Mod 安装便利，Mod 版权归原作者所有。使用 Mod 可能影响游戏稳定性，请自行承担风险。
        </p>
      </section>
    </div>
  </section>
</template>
