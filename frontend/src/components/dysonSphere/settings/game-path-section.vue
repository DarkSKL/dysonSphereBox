<script setup lang="ts">
import {FolderOpen, Save, ScanSearch} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted, ref, watch} from 'vue'
import {
  detectGameDirectory,
  gameDirectory,
  gameDirectoryLoaded,
  gameDirectoryStored,
  loadGameDirectory,
  saveGameDirectory,
  selectGameDirectory,
} from '../../../services/dysonSphere/game'
import {formatErrorMessage} from '../../../utils/format'
import SettingsSection, {type SettingsHintTone} from '../../system/settings/section.vue'

type GameDirState = 'loading' | 'ready' | 'invalid' | 'empty'

const message = useMessage()
const draftPath = ref('')
const loadError = ref('')
const isSaving = ref(false)
const isSelecting = ref(false)
const isDetecting = ref(false)
const isBusy = computed(() => isSaving.value || isSelecting.value || isDetecting.value)

// =============== 目录状态归纳 ===============
// 说明：区分加载中、有效、已保存但失效（例如游戏被移动或卸载）、未配置四种状态。
const state = computed<GameDirState>(() => {
  if (!gameDirectoryLoaded.value) return 'loading'
  if (gameDirectory.value) return 'ready'
  return gameDirectoryStored.value ? 'invalid' : 'empty'
})
const hasPendingChange = computed(() => {
  const value = draftPath.value.trim()
  return Boolean(value) && value !== gameDirectory.value
})

// =============== 提示行 ===============
// 说明：说明目录是否可用；手动编辑未保存时提示保存方式。
const hint = computed<{text: string, tone: SettingsHintTone}>(() => {
  if (loadError.value) return {text: `读取游戏路径失败：${loadError.value}`, tone: 'error'}
  if (hasPendingChange.value) return {text: '按 Enter 或点击保存，路径可以是游戏目录或 DSPGAME.exe。', tone: 'muted'}
  switch (state.value) {
    case 'ready':
      return {text: '已检测到 DSPGAME.exe，Mod 安装与本地资源管理都基于该目录。', tone: 'success'}
    case 'invalid':
      return {text: '已保存的目录中找不到 DSPGAME.exe，游戏可能已被移动或卸载，请重新选择。', tone: 'warning'}
    case 'empty':
      return {text: '尚未配置游戏路径，可手动选择，或从 Steam 库自动获取。', tone: 'warning'}
    default:
      return {text: '正在读取游戏路径…', tone: 'muted'}
  }
})

const syncDraft = (): void => {
  draftPath.value = gameDirectory.value || gameDirectoryStored.value
}

onMounted(async () => {
  try {
    await loadGameDirectory()
  } catch (error) {
    loadError.value = formatErrorMessage(error)
  }
  syncDraft()
})

// 其它入口（目录选择器、自动获取）更新目录后同步输入框
watch(gameDirectory, syncDraft)

// =============== 目录保存 ===============
// 说明：手动填写的路径可以是游戏目录或 DSPGAME.exe 路径，由后端归一化后保存。
// 边界：后端找不到 DSPGAME.exe 时拒绝保存，保留草稿供用户修改。
const handleSave = async (): Promise<void> => {
  if (!hasPendingChange.value || isBusy.value) {
    return
  }
  isSaving.value = true
  try {
    const path = await saveGameDirectory(draftPath.value)
    draftPath.value = path
    message.success('游戏路径已保存。')
  } catch (error) {
    message.error(`游戏路径保存失败：${formatErrorMessage(error)}`)
  } finally {
    isSaving.value = false
  }
}

// =============== 目录选择与自动获取 ===============
// 说明：目录选择器选中有效目录时后端直接保存；自动获取优先沿用已保存目录，否则从 Steam 库定位。
// 边界：取消选择不提示；选中目录不含 DSPGAME.exe 时不保存，只提示原因。
const handleSelect = async (): Promise<void> => {
  if (isBusy.value) return
  isSelecting.value = true
  try {
    const result = await selectGameDirectory()
    if (result.status === 'saved') {
      message.success('游戏路径已保存。')
    } else if (result.status === 'invalid') {
      message.warning('所选目录中没有找到 DSPGAME.exe，请选择戴森球计划的安装目录。')
    }
  } catch (error) {
    message.error(`打开目录选择器失败：${formatErrorMessage(error)}`)
  } finally {
    isSelecting.value = false
  }
}

const handleDetect = async (): Promise<void> => {
  if (isBusy.value) return
  isDetecting.value = true
  try {
    await detectGameDirectory()
    message.success('已自动获取游戏路径。')
  } catch (error) {
    message.error(`自动获取失败：${formatErrorMessage(error)}`)
  } finally {
    isDetecting.value = false
  }
}
</script>

<template>
  <!-- ~ 游戏路径 -->
  <!-- * 说明：输入框编辑游戏目录，右侧为选择、自动获取；手动编辑后出现保存按钮 -->
  <!-- * 交互：按 Enter 保存；选择有效目录后立即保存；自动获取从 Steam 库定位 -->
  <!-- * 依赖：依赖 services/dysonSphere/game 的目录状态与读写方法 -->
  <!-- * 边界：任一操作进行中时禁用其它操作，避免并发写入 -->
  <SettingsSection id="game-settings-path" title="游戏路径" :hint-tone="hint.tone">
    <n-input
      v-model:value="draftPath"
      placeholder="例如 D:\SteamLibrary\steamapps\common\Dyson Sphere Program"
      :status="state === 'invalid' && !hasPendingChange ? 'warning' : undefined"
      :input-props="{spellcheck: false, autocomplete: 'off', 'aria-labelledby': 'game-settings-path-title', 'aria-describedby': 'game-settings-path-hint'}"
      :disabled="state === 'loading'"
      @keydown.enter="handleSave"
    />
    <n-button
      v-if="hasPendingChange"
      type="primary"
      secondary
      :disabled="isBusy && !isSaving"
      :loading="isSaving"
      @click="handleSave"
    >
      <template #icon>
        <Save :size="15" :stroke-width="1.8" />
      </template>
      保存
    </n-button>
    <n-button :disabled="isBusy && !isSelecting" :loading="isSelecting" @click="handleSelect">
      <template #icon>
        <FolderOpen :size="15" :stroke-width="1.8" />
      </template>
      选择
    </n-button>
    <n-button :disabled="isBusy && !isDetecting" :loading="isDetecting" @click="handleDetect">
      <template #icon>
        <ScanSearch :size="15" :stroke-width="1.8" />
      </template>
      自动获取
    </n-button>
    <template #hint>{{ hint.text }}</template>
  </SettingsSection>
</template>
