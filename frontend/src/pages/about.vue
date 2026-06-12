<script setup lang="ts">
import {Copy, ExternalLink, GitFork, MessageCircle} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {onMounted} from 'vue'
import {useAppVersion} from '../composables/useAppVersion'

definePage({
  meta: {
    layout: 'main',
    title: '关于我们',
  },
})

const message = useMessage()

// =============== 页面链接 ===============
// 说明：维护项目主页与问题反馈入口，模板只负责渲染外链。
const links = [
  {
    label: 'https://gitee.com/Dark-Skl/dyson-sphere-box.git',
    href: 'https://gitee.com/Dark-Skl/dyson-sphere-box.git',
    icon: GitFork,
  },
  {
    label: 'Gitee Issues',
    href: 'https://gitee.com/Dark-Skl/dyson-sphere-box/issues',
    icon: GitFork,
  },
]

// =============== 联系方式 ===============
// 说明：维护可复制的联系渠道，点击联系人时统一复制 copyValue。
const contacts = [
  {
    label: '微信号：DarkSKL',
    copyValue: 'DarkSKL',
    successText: '微信号已复制',
    icon: MessageCircle,
    iconText: '',
  },
  {
    label: 'QQ 群：779182569',
    copyValue: '779182569',
    successText: 'QQ 群号已复制',
    icon: null,
    iconText: 'QQ',
  },
]

const credits = [
  ['模组社区', 'Thunderstore'],
  ['联机框架', 'Nebula Multiplayer 团队'],
  ['模组框架', 'BepInEx 项目'],
  ['模组作者', 'AssemblerUI、BottleneckUI、DSPCalculator、FactoryLocator、CAAP_N'],
  ['蓝图', 'B站 UP：神不在的黄昏'],
]

const {
  appVersion,
  loadAppVersion,
} = useAppVersion()

// =============== 复制反馈 ===============
// 说明：优先使用浏览器剪贴板 API，失败时用输入框选中方案兜底。
const copyTextToClipboard = async (text: string): Promise<void> => {
  if (navigator.clipboard !== undefined) {
    await navigator.clipboard.writeText(text)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()

  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)

  if (!copied) {
    throw new Error('copy command failed')
  }
}

// 说明：联系人点击入口，复制成功或失败都给出明确的用户反馈。
const copyContact = async (contact: (typeof contacts)[number]): Promise<void> => {
  try {
    await copyTextToClipboard(contact.copyValue)
    message.success(contact.successText)
  } catch (error) {
    message.error(`复制失败，请手动复制：${contact.copyValue}`)
  }
}

onMounted(() => {
  void loadAppVersion()
})
</script>

<template>
  <section class="mx-auto max-w-3xl px-8 py-8">
    <h1 class="mb-6 text-xl font-semibold text-zinc-950">关于我们</h1>

    <div class="grid gap-8">
      <section>
        <p class="text-sm text-zinc-500">版本</p>
        <p class="mt-1 text-zinc-950">{{ appVersion }}</p>
      </section>

      <section>
        <p class="text-sm text-zinc-500">开发者</p>
        <p class="mt-1 text-zinc-950">DarkSKL</p>
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
            <span class="flex min-w-0 items-center gap-2">
              <component :is="link.icon" :size="16" :stroke-width="1.8" class="shrink-0" />
              <span class="truncate">{{ link.label }}</span>
            </span>
            <ExternalLink :size="14" :stroke-width="1.8" class="ml-3 shrink-0 text-zinc-400" />
          </a>
        </div>
      </section>

      <section>
        <p class="mb-3 text-sm text-zinc-500">联系方式</p>
        <div class="grid gap-3">
          <!-- ~ 联系方式复制 -->
          <!-- * 说明：展示可复制的微信号与 QQ 群号 -->
          <!-- * 交互：点击任一联系方式后复制对应内容，并通过消息提示反馈结果 -->
          <!-- * 依赖：依赖 contacts 数据与浏览器剪贴板能力 -->
          <!-- * 边界：剪贴板 API 不可用时使用输入框选中复制兜底，失败时提示用户手动复制 -->
          <button
            v-for="contact in contacts"
            :key="contact.label"
            type="button"
            class="flex items-center justify-between rounded border border-zinc-200 bg-white px-3 py-2 text-left text-sm text-zinc-700 transition hover:border-zinc-950 hover:text-zinc-950"
            @click="copyContact(contact)"
          >
            <span class="flex min-w-0 items-center gap-2">
              <n-icon v-if="contact.iconText" :size="16" class="shrink-0">
                <span class="text-[10px] font-semibold leading-none">{{ contact.iconText }}</span>
              </n-icon>
              <component
                :is="contact.icon"
                v-else
                :size="16"
                :stroke-width="1.8"
                class="shrink-0"
              />
              <span class="truncate">{{ contact.label }}</span>
            </span>
            <Copy :size="14" :stroke-width="1.8" class="ml-3 shrink-0 text-zinc-400" />
          </button>
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
          本工具仅提供模组安装便利，模组版权归原作者所有。使用模组可能影响游戏稳定性，请自行承担风险。
        </p>
      </section>
    </div>
  </section>
</template>
