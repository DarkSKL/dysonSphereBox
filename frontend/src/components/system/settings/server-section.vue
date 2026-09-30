<script setup lang="ts">
import {CircleAlert, CircleCheck, LoaderCircle, PlugZap, RotateCcw, Save} from '@lucide/vue'
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
import SettingsSection from './section.vue'

const message = useMessage()
const draftAddress = ref('')
const isSaving = ref(false)
const isProbing = ref(false)
const probeResult = ref<ServerProbeResult | null>(null)

// =============== 地址草稿状态 ===============
// 说明：输入框展示当前生效地址，编辑内容只存在草稿中；点击保存后才写入 System/ServerAddress。
// 边界：草稿为空视为恢复默认地址；格式错误时禁用保存并在输入框下方提示原因。
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

onMounted(async () => {
  await loadServerAddress()
  draftAddress.value = serverEffectiveAddress.value
})

// 地址变化后旧的测试结果不再代表当前草稿，直接清除
watch(draftAddress, () => {
  probeResult.value = null
})

// =============== 连接测试 ===============
// 说明：只对草稿地址做只读探测，不保存配置；探测失败以状态行提示，不弹出错误消息。
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
  <SettingsSection
    id="settings-server"
    title="服务器连接地址"
    description="账号登录、注册等应用服务通过该地址访问。未自定义时使用本机默认服务。"
  >
    <!-- ~ 地址输入与操作 -->
    <!-- * 说明：左侧编辑服务器基础地址，右侧紧跟保存、测试连接、恢复默认三个图标按钮 -->
    <!-- * 交互：按 Enter 保存；清空输入等同于恢复默认；图标按钮悬停显示文字提示，恢复默认只修改草稿，仍需保存生效 -->
    <!-- * 依赖：依赖 services/server 的地址状态、格式校验、probeServerAddress 与 saveServerAddress -->
    <!-- * 边界：格式错误时禁用保存和测试；无改动时禁用保存；草稿已是默认地址时禁用恢复默认 -->
    <label class="server-label" for="settings-server-input">
      服务器地址
      <span v-if="isUsingDefault" class="server-badge">默认</span>
      <span v-else class="server-badge is-custom">自定义</span>
    </label>
    <div class="server-row">
      <n-input
        id="settings-server-input"
        v-model:value="draftAddress"
        class="server-input"
        :placeholder="DEFAULT_SERVER_ADDRESS"
        :status="draftValidation.error ? 'error' : undefined"
        :input-props="{spellcheck: false, autocomplete: 'off', 'aria-describedby': 'settings-server-hint'}"
        clearable
        @keydown.enter="handleSave"
      />
      <n-tooltip :show-arrow="false">
        <template #trigger>
          <n-button
            type="primary"
            aria-label="保存"
            :disabled="!hasPendingChange"
            :loading="isSaving"
            @click="handleSave"
          >
            <template #icon>
              <Save :size="16" :stroke-width="1.8" />
            </template>
          </n-button>
        </template>
        保存
      </n-tooltip>
      <n-tooltip :show-arrow="false">
        <template #trigger>
          <n-button
            aria-label="测试连接"
            :disabled="Boolean(draftValidation.error)"
            :loading="isProbing"
            @click="handleProbe"
          >
            <template #icon>
              <PlugZap :size="16" :stroke-width="1.8" />
            </template>
          </n-button>
        </template>
        测试连接
      </n-tooltip>
      <n-tooltip :show-arrow="false">
        <template #trigger>
          <n-button aria-label="恢复默认" :disabled="isDraftDefault" @click="handleRestoreDefault">
            <template #icon>
              <RotateCcw :size="16" :stroke-width="1.8" />
            </template>
          </n-button>
        </template>
        恢复默认
      </n-tooltip>
    </div>
    <p
      id="settings-server-hint"
      class="server-hint"
      :class="{'is-error': draftValidation.error}"
      :role="draftValidation.error ? 'alert' : undefined"
    >
      <template v-if="draftValidation.error">{{ draftValidation.error }}</template>
      <template v-else-if="hasPendingChange && accountIsAuthenticated">保存后会退出当前账号，需要在新服务器上重新登录。</template>
      <template v-else>示例：http://127.0.0.1:8888 或 https://api.example.com</template>
    </p>

    <!-- ~ 连接测试结果 -->
    <!-- * 说明：展示最近一次测试的结论与耗时 -->
    <!-- * 交互：修改草稿后自动清除，避免旧结论误导 -->
    <!-- * 依赖：依赖 probeResult 与 isProbing -->
    <!-- * 边界：未测试时不渲染该区域 -->
    <p v-if="isProbing" class="server-probe" aria-live="polite">
      <LoaderCircle class="server-probe-spin" :size="15" :stroke-width="1.8" aria-hidden="true" />
      正在连接 {{ draftValidation.address }}…
    </p>
    <p
      v-else-if="probeResult"
      class="server-probe"
      :class="probeResult.ok ? 'is-ok' : 'is-fail'"
      aria-live="polite"
    >
      <CircleCheck v-if="probeResult.ok" :size="15" :stroke-width="1.8" aria-hidden="true" />
      <CircleAlert v-else :size="15" :stroke-width="1.8" aria-hidden="true" />
      {{ probeResult.message }}
    </p>
  </SettingsSection>
</template>

<style scoped lang="scss">
.server-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  color: var(--ds-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.server-badge {
  padding: 1px 6px;
  border: 1px solid var(--ds-border-default);
  border-radius: 2px;
  color: var(--ds-text-muted);
  font-size: 11px;
  font-weight: 600;
}

.server-badge.is-custom {
  border-color: var(--ds-border-strong);
  color: var(--ds-brand);
}

.server-hint {
  min-height: 18px;
  margin: 8px 0 0;
  color: var(--ds-text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.server-hint.is-error {
  color: var(--ds-error);
}

// 输入框与图标按钮同行；输入框限制宽度，地址文本通常不超过 40 个字符
.server-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.server-input {
  flex: 0 1 380px;
  min-width: 0;
}

.server-probe {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 8px 0 0;
  color: var(--ds-text-secondary);
  font-size: 12px;
}

.server-probe.is-ok {
  color: var(--ds-success);
}

.server-probe.is-fail {
  color: var(--ds-error);
}

.server-probe-spin {
  animation: server-probe-spin 0.9s linear infinite;
}

@keyframes server-probe-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .server-probe-spin {
    animation: none;
  }
}
</style>
