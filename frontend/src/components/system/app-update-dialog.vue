<script setup lang="ts">
import {CloudDownload, LogOut} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed} from 'vue'
import {Quit} from '../../../wailsjs/runtime/runtime'
import {useAppUpdateState} from '../../composables/system/useAppUpdateState'
import {formatErrorMessage} from '../../utils/format'
import ReleaseNotes from './release-notes.vue'
import AppUpdateButton from './app-update-button.vue'
import AppUpdateProgress from './app-update-progress.vue'

const props = defineProps<{readonly show: boolean}>()
const emit = defineEmits<{(event: 'update:show', value: boolean): void}>()
const message = useMessage()
const {appUpdateInfo, isUpdatingApp, preparedTargetVersion, updateAppSelf} = useAppUpdateState()
const title = computed(() => `发现新版本 ${appUpdateInfo.value?.latestVersion || ''}`.trim())

// =============== 应用更新弹窗动作 ===============
// 说明：在标题栏版本提示打开后展示发布说明，并由弹窗底部按钮执行现有自更新流程。
// 边界：更新失败时保留弹窗和说明；准备完成后提示退出安装，并将主操作替换为退出并更新。
const handleUpdate = async (): Promise<void> => {
  try {
    const result = await updateAppSelf()
    if (result.prepared) {
      message.success(result.message || '更新包已准备好，退出后安装并自动重新打开应用。')
      return
    }

    message.info(result.message || '当前已是最新版本。')
  } catch (error) {
    message.error(`应用更新失败：${formatErrorMessage(error)}`)
  }
}
</script>

<template>
  <!-- ~ 应用更新内容弹窗 -->
  <!-- * 说明：展示新版本发布说明和唯一的更新主操作，供标题栏版本提示打开 -->
  <!-- * 交互：更新期间阻止关闭；下载完成后退出安装并自动重新打开 -->
  <!-- * 依赖：共享更新检查结果、useAppUpdateState 与 Wails Quit 运行时 -->
  <!-- * 边界：发布说明按 Markdown 渲染并清洗，链接只放行 https 且交给系统浏览器打开 -->
  <n-modal
    :show="props.show"
    preset="card"
    :title="title"
    :mask-closable="!isUpdatingApp"
    :close-on-esc="!isUpdatingApp"
    :closable="!isUpdatingApp"
    style="width: min(620px, calc(100vw - 32px));"
    @update:show="emit('update:show', $event)"
  >
    <section class="app-update-dialog-content" aria-label="更新内容">
      <header class="app-update-dialog-release">
        <span class="app-update-dialog-release-name">{{ appUpdateInfo?.releaseName || '版本更新' }}</span>
        <span v-if="appUpdateInfo?.publishedAt" class="app-update-dialog-date">{{ appUpdateInfo.publishedAt }}</span>
      </header>
      <ReleaseNotes class="app-update-dialog-notes" :source="appUpdateInfo?.releaseNotes" empty-text="该版本没有附带更新说明。" />
      <AppUpdateProgress />
      <p v-if="preparedTargetVersion" class="app-update-dialog-restart" role="status">
        {{ preparedTargetVersion }} 已下载并校验。退出后安装更新，完成后自动重新打开应用。
      </p>
    </section>

    <template #footer>
      <div class="app-update-dialog-footer">
        <n-button
          v-if="preparedTargetVersion"
          type="primary"
          @click="Quit"
        >
          <template #icon>
            <LogOut :size="15" :stroke-width="1.8" />
          </template>
          退出并更新
        </n-button>
        <AppUpdateButton
          v-else
          :disabled="!appUpdateInfo?.hasUpdate || !appUpdateInfo.assetName"
          @click="handleUpdate"
        >
          <template #icon>
            <CloudDownload :size="15" :stroke-width="1.8" />
          </template>
          立即更新
        </AppUpdateButton>
      </div>
    </template>
  </n-modal>
</template>

<style scoped lang="scss">
.app-update-dialog-content {
  min-width: 0;
}

.app-update-dialog-release {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--ds-border-subtle);
}

.app-update-dialog-release-name {
  color: var(--ds-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.app-update-dialog-date {
  color: var(--ds-text-muted);
  font-size: 12px;
}

.app-update-dialog-notes {
  max-height: min(55vh, 480px);
  margin: 0;
  padding: 12px 0;
  overflow: auto;
  color: var(--ds-text-secondary);
  font-size: 13px;
  line-height: 1.75;
}

.app-update-dialog-restart {
  margin: 0;
  color: var(--ds-success);
  font-size: 12px;
}

.app-update-dialog-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px 16px;
}

</style>
