<script setup lang="ts">
import {PlugZap, RotateCcw, Save} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted, ref, watch} from 'vue'
import {accountIsAuthenticated, resetAccountSession} from '../../../services/account'
import {
  DEFAULT_SERVER_ADDRESS,
  loadServerAddress,
  normalizeServerAddress,
  probeServerAddress,
  saveServerAddress,
  serverAddress,
  serverEffectiveAddress,
  type ServerProbeResult,
} from '../../../services/server'
import {formatErrorMessage} from '../../../utils/format'
import SettingsSection, {type SettingsHintTone} from './section.vue'

const message = useMessage()
const draftAddress = ref('')
const isSaving = ref(false)
const isProbing = ref(false)
const probeResult = ref<ServerProbeResult | null>(null)
// 记录最近一次已知生效地址；账号会话晚到校准地址时，用它判断草稿是否仍未被用户改动。
let lastEffectiveAddress = ''

// =============== 地址草稿状态 ===============
// 说明：输入框展示当前生效地址，编辑内容只存在草稿中；点击保存后才写入 System/ServerAddress。
// 边界：草稿为空视为恢复默认地址；格式错误时禁用保存并在提示行说明原因。
const draftValidation = computed<{address: string, error: string}>(() => {
  const value = draftAddress.value.trim()
  if (!value) {
    return {address: DEFAULT_SERVER_ADDRESS, error: ''}
  }
  try {
    return {address: normalizeServerAddress(value), error: ''}
  } catch (error) {
    return {address: '', error: formatErrorMessage(error)}
  }
})
const hasPendingChange = computed(() => (
  !draftValidation.value.error && draftValidation.value.address !== serverEffectiveAddress.value
))
const isUsingDefault = computed(() => !serverAddress.value)
const isDraftDefault = computed(() => draftValidation.value.address === DEFAULT_SERVER_ADDRESS)

// =============== 提示行 ===============
// 说明：按优先级展示格式错误、测试中、测试结论、保存影响与默认说明。
const hint = computed<{text: string, tone: SettingsHintTone}>(() => {
  if (draftValidation.value.error) return {text: draftValidation.value.error, tone: 'error'}
  if (isProbing.value) return {text: `正在连接 ${draftValidation.value.address}…`, tone: 'muted'}
  if (probeResult.value) return {text: probeResult.value.message, tone: probeResult.value.ok ? 'success' : 'error'}
  if (hasPendingChange.value && accountIsAuthenticated.value) {
    return {text: '保存后会退出当前账号，需要在新服务器上重新登录。', tone: 'warning'}
  }
  if (isUsingDefault.value) return {text: '账号等应用服务的访问地址，当前使用本机默认服务。', tone: 'muted'}
  return {text: '账号等应用服务的访问地址，当前使用自定义服务器，留空保存可恢复默认。', tone: 'muted'}
})

onMounted(async () => {
  await loadServerAddress()
  lastEffectiveAddress = serverEffectiveAddress.value
  draftAddress.value = lastEffectiveAddress
})

// 账号会话可能在后端启动完成后校准服务器地址；草稿未被用户改动时跟随更新，避免把默认地址误当成待保存修改。
watch(serverEffectiveAddress, (address) => {
  if (draftAddress.value === lastEffectiveAddress) draftAddress.value = address
  lastEffectiveAddress = address
})

// 地址变化后旧的测试结果不再代表当前草稿，直接清除
watch(draftAddress, () => {
  probeResult.value = null
})

// =============== 连接测试 ===============
// 说明：只对草稿地址做只读探测，不保存配置；探测结论展示在提示行，不弹出错误消息。
const handleProbe = async (): Promise<void> => {
  if (draftValidation.value.error || isProbing.value) {
    return
  }
  isProbing.value = true
  try {
    probeResult.value = await probeServerAddress(draftValidation.value.address)
  } finally {
    isProbing.value = false
  }
}

// =============== 地址保存 ===============
// 说明：保存后若生效地址发生变化，清除账号会话，避免旧服务签发的 token 被发往新服务。
// 边界：保存失败时保留草稿内容，并通过消息提示后端返回的原因。
const handleSave = async (): Promise<void> => {
  if (!hasPendingChange.value || isSaving.value) {
    return
  }
  isSaving.value = true
  try {
    const wasAuthenticated = accountIsAuthenticated.value
    const changed = await saveServerAddress(draftValidation.value.address)
    draftAddress.value = serverEffectiveAddress.value
    if (changed && wasAuthenticated) {
      resetAccountSession()
      message.info('服务器地址已更新，请重新登录账号。')
    } else {
      message.success('服务器地址已更新。')
    }
  } catch (error) {
    message.error(`服务器地址保存失败：${formatErrorMessage(error)}`)
  } finally {
    isSaving.value = false
  }
}

const handleRestoreDefault = (): void => {
  draftAddress.value = DEFAULT_SERVER_ADDRESS
}
</script>

<template>
  <!-- ~ 连接地址 -->
  <!-- * 说明：输入框编辑服务器基础地址，右侧为保存、测试、恢复默认；提示行展示校验与测试结论 -->
  <!-- * 交互：按 Enter 保存；清空输入等同于恢复默认；恢复默认只修改草稿，仍需保存生效 -->
  <!-- * 依赖：依赖 services/server 的地址状态、格式校验、probeServerAddress 与 saveServerAddress -->
  <!-- * 边界：格式错误时禁用保存和测试；无改动时禁用保存；草稿已是默认地址时禁用恢复默认 -->
  <SettingsSection id="settings-server" title="连接地址" :hint-tone="hint.tone">
    <n-input
      v-model:value="draftAddress"
      class="type-mono"
      :placeholder="DEFAULT_SERVER_ADDRESS"
      :status="draftValidation.error ? 'error' : undefined"
      :input-props="{spellcheck: false, autocomplete: 'off', 'aria-labelledby': 'settings-server-title', 'aria-describedby': 'settings-server-hint'}"
      clearable
      @keydown.enter="handleSave"
    />
    <n-button type="primary" secondary :disabled="!hasPendingChange" :loading="isSaving" @click="handleSave">
      <template #icon>
        <Save :size="15" :stroke-width="1.8" />
      </template>
      保存
    </n-button>
    <n-button :disabled="Boolean(draftValidation.error)" :loading="isProbing" @click="handleProbe">
      <template #icon>
        <PlugZap :size="15" :stroke-width="1.8" />
      </template>
      测试
    </n-button>
    <n-button :disabled="isDraftDefault" @click="handleRestoreDefault">
      <template #icon>
        <RotateCcw :size="15" :stroke-width="1.8" />
      </template>
      默认
    </n-button>
    <template #hint>{{ hint.text }}</template>
  </SettingsSection>
</template>
