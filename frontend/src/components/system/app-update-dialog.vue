<script setup lang="ts">
import {CloudDownload, LogOut} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, ref} from 'vue'
import {BrowserOpenURL, Quit} from '../../../wailsjs/runtime/runtime'
import {useAppUpdateState} from '../../composables/system/useAppUpdateState'
import {formatErrorMessage} from '../../utils/format'
import {renderReleaseNotes} from '../../utils/release-notes'

const props = defineProps<{readonly show: boolean}>()
const emit = defineEmits<{(event: 'update:show', value: boolean): void}>()
const message = useMessage()
const {appUpdateInfo, isUpdatingApp, updateAppSelf} = useAppUpdateState()
const updatedTargetVersion = ref('')
const releaseNotesHtml = computed(() => renderReleaseNotes(appUpdateInfo.value?.releaseNotes?.trim() || ''))
const title = computed(() => `发现新版本 ${appUpdateInfo.value?.latestVersion || ''}`.trim())

// =============== 应用更新弹窗动作 ===============
// 说明：在标题栏版本提示打开后展示发布说明，并由弹窗底部按钮执行现有自更新流程。
// 边界：更新失败时保留弹窗和说明；成功写入后提示重启，并将主操作替换为退出应用。
const handleUpdate = async (): Promise<void> => {
  try {
    const result = await updateAppSelf()
    if (result.updated) {
      updatedTargetVersion.value = result.targetVersion || result.check.latestVersion
      message.success(result.message || '更新已完成，重启应用后生效。')
      return
    }

    message.info(result.message || '当前已是最新版本。')
  } catch (error) {
    message.error(`应用更新失败：${formatErrorMessage(error)}`)
  }
}

// =============== 发布说明链接 ===============
// 说明：正文链接在渲染时已经移除 href，只由鼠标主键、中键和 Enter 触发，并交给系统浏览器打开。
// 边界：忽略右键和没有地址的目标，避免把应用 WebView 导航到远端页面。
const openReleaseNoteLink = (event: MouseEvent | KeyboardEvent): void => {
  if (!(event.target instanceof Element)) return
  const target = event.target.closest<HTMLAnchorElement>('a[data-external-href]')
  const href = target?.dataset.externalHref
  if (!href || (event instanceof MouseEvent && event.button > 1)) return
  event.preventDefault()
  event.stopPropagation()
  void BrowserOpenURL(href)
}
</script>

<template>
  <!-- ~ 应用更新内容弹窗 -->
  <!-- * 说明：展示新版本发布说明和唯一的更新主操作，供标题栏版本提示打开 -->
  <!-- * 交互：更新期间阻止关闭；写入成功后将主操作改为退出应用以便重启生效 -->
  <!-- * 依赖：共享更新检查结果、useAppUpdateState 与 Wails BrowserOpenURL / Quit 运行时 -->
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
      <div
        v-if="releaseNotesHtml"
        class="app-update-dialog-notes"
        v-html="releaseNotesHtml"
        @click="openReleaseNoteLink"
        @auxclick.prevent="openReleaseNoteLink"
        @keydown.enter="openReleaseNoteLink"
      />
      <p v-else class="app-update-dialog-notes">该版本没有附带更新说明。</p>
      <p v-if="updatedTargetVersion" class="app-update-dialog-restart" role="status">
        已更新到 {{ updatedTargetVersion }}，退出应用后重新启动即可生效。
      </p>
    </section>

    <template #footer>
      <div class="app-update-dialog-footer">
        <n-button
          v-if="updatedTargetVersion"
          type="primary"
          @click="Quit"
        >
          <template #icon>
            <LogOut :size="15" :stroke-width="1.8" />
          </template>
          退出应用
        </n-button>
        <n-button
          v-else
          type="primary"
          :loading="isUpdatingApp"
          :disabled="!appUpdateInfo?.hasUpdate || !appUpdateInfo.assetName"
          @click="handleUpdate"
        >
          <template #icon>
            <CloudDownload :size="15" :stroke-width="1.8" />
          </template>
          立即更新
        </n-button>
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
  overflow-wrap: anywhere;

  :deep(> :first-child) { margin-top: 0; }
  :deep(> :last-child) { margin-bottom: 0; }
  :deep(p) { margin: 8px 0; }
  :deep(h1), :deep(h2), :deep(h3), :deep(h4), :deep(h5), :deep(h6) {
    margin: 14px 0 6px;
    color: var(--ds-text-primary);
    font-weight: 700;
    line-height: 1.35;
  }
  :deep(h1) { font-size: 17px; }
  :deep(h2) { font-size: 16px; }
  :deep(h3) { font-size: 14px; }
  :deep(h4), :deep(h5), :deep(h6) { font-size: 13px; }
  :deep(strong) { color: var(--ds-text-primary); font-weight: 600; }
  :deep(ul), :deep(ol) { margin: 6px 0; padding-left: 1.6em; }
  :deep(ul) { list-style: disc; }
  :deep(ol) { list-style: decimal; }
  :deep(li) { margin: 2px 0; }
  :deep(li > p) { margin: 2px 0; }
  :deep(blockquote) {
    margin: 8px 0;
    padding-left: 10px;
    border-left: 3px solid var(--ds-border-subtle);
    color: var(--ds-text-muted);
  }
  :deep(hr) { margin: 12px 0; border: 0; border-top: 1px solid var(--ds-border-subtle); }
  :deep(a[data-external-href]) {
    color: var(--ds-brand);
    text-decoration: underline;
    cursor: pointer;
  }
  :deep(a[data-external-href]:focus-visible) { outline: 2px solid var(--ds-brand); outline-offset: 2px; }
  :deep(code) {
    padding: 1px 4px;
    border-radius: 3px;
    background: var(--ds-bg-panel);
    font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
    font-size: 12px;
  }
  :deep(pre) {
    margin: 8px 0;
    padding: 10px 12px;
    overflow: auto;
    border-radius: 4px;
    background: var(--ds-bg-panel);
  }
  :deep(pre code) { padding: 0; background: transparent; }
  :deep(table) { width: 100%; margin: 10px 0; border-collapse: collapse; font-size: 12px; }
  :deep(th), :deep(td) { padding: 5px 8px; border: 1px solid var(--ds-border-subtle); text-align: left; }
  :deep(th) { background: var(--ds-bg-panel); color: var(--ds-text-primary); font-weight: 600; }
  :deep(img) { max-width: 100%; height: auto; border-radius: 4px; }
}

.app-update-dialog-restart {
  margin: 0;
  color: var(--ds-success);
  font-size: 12px;
}

.app-update-dialog-footer {
  display: flex;
  justify-content: flex-end;
}
</style>
