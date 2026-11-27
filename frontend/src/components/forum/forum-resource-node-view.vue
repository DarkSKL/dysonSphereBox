<script setup lang="ts">
import {AlertTriangle, FileArchive, FileCode2, Film, Image as ImageIcon, Package, Trash2} from '@lucide/vue'
import {NodeViewWrapper, type NodeViewProps} from '@tiptap/vue-3'
import {computed} from 'vue'
import type {ForumDraftBindings} from '../../types/forum'
import {formatFileSize} from '../../utils/format'

const props = defineProps<NodeViewProps>()

const slot = computed(() => String(props.node.attrs.slot || ''))
const bindings = computed<ForumDraftBindings>(() => {
  const getter = props.extension.options?.getBindings as (() => ForumDraftBindings) | undefined
  return getter ? getter() : {images: {}, video: null, blueprint: null, mod: null, save: null}
})

const imageFile = computed(() => (/^image[1-8]$/.test(slot.value) ? bindings.value.images[slot.value] ?? null : null))
const videoFile = computed(() => (slot.value === 'video' ? bindings.value.video : null))
const blueprintDraft = computed(() => (slot.value === 'blueprint' ? bindings.value.blueprint : null))
const modDraft = computed(() => (slot.value === 'mod' ? bindings.value.mod : null))
const saveFile = computed(() => (slot.value === 'save' ? bindings.value.save : null))

const isBound = computed(() => Boolean(
  imageFile.value || videoFile.value || blueprintDraft.value || modDraft.value || saveFile.value,
))

function removeBlock(): void {
  props.deleteNode()
}

function unbindAndRemove(): void {
  const currentSlot = slot.value
  props.deleteNode()
  const onRemove = props.extension.options?.onRemoveSlot as ((slot: string) => void) | undefined
  if (onRemove && currentSlot) {
    onRemove(currentSlot)
  }
}
</script>

<template>
  <!-- ~ 编辑器内嵌资源块视图 -->
  <!-- * 说明：在 TipTap 所见即所得正文中按插入位置原位预览本地图片、MP4 或游戏资源卡 -->
  <!-- * 交互：可拖拽调整位置，点击右上角按钮仅移除本处标签或彻底移除资源绑定 -->
  <!-- * 边界：未绑定资源的标签展示琥珀警示，提示用户补齐或删除 -->
  <NodeViewWrapper
    class="forum-node-wrapper"
    :class="{'is-selected': selected, 'is-unbound': !isBound}"
    contenteditable="false"
  >
    <template v-if="imageFile">
      <div class="node-media-card">
        <div class="node-head">
          <span class="node-badge"><ImageIcon :size="13" />[[resource:{{ slot }}]]</span>
          <span class="node-meta" :title="imageFile.name">{{ imageFile.name }} · {{ formatFileSize(imageFile.size) }}</span>
          <div class="node-actions">
            <button type="button" class="node-action-btn" title="仅移除此处的标签" @click="removeBlock">移除标签</button>
            <button type="button" class="node-action-btn is-danger" title="移除图片并解除绑定" @click="unbindAndRemove"><Trash2 :size="13" /></button>
          </div>
        </div>
        <div class="node-image-frame">
          <img v-if="imageFile.previewUrl" :src="imageFile.previewUrl" :alt="imageFile.name" draggable="false" />
          <div v-else class="node-preview-fallback">图片已选中（{{ imageFile.name }}）</div>
        </div>
      </div>
    </template>

    <template v-else-if="videoFile">
      <div class="node-media-card">
        <div class="node-head">
          <span class="node-badge"><Film :size="13" />[[resource:video]]</span>
          <span class="node-meta" :title="videoFile.name">{{ videoFile.name }} · {{ formatFileSize(videoFile.size) }}</span>
          <div class="node-actions">
            <button type="button" class="node-action-btn" title="仅移除此处的标签" @click="removeBlock">移除标签</button>
            <button type="button" class="node-action-btn is-danger" title="移除视频并解除绑定" @click="unbindAndRemove"><Trash2 :size="13" /></button>
          </div>
        </div>
        <div class="node-video-frame">
          <video v-if="videoFile.previewUrl" :src="videoFile.previewUrl" controls preload="metadata" />
          <div v-else class="node-preview-fallback">视频已选中（{{ videoFile.name }}）</div>
        </div>
      </div>
    </template>

    <template v-else-if="blueprintDraft">
      <div class="node-resource-card">
        <div class="node-resource-icon"><FileCode2 :size="18" /></div>
        <div class="node-resource-body">
          <div class="node-resource-title">{{ blueprintDraft.name }}</div>
          <div class="node-resource-sub">
            <span class="node-badge">[[resource:blueprint]]</span>
            <span>{{ blueprintDraft.mode === 'file' ? `托管蓝图文件 · ${formatFileSize(blueprintDraft.file?.size ?? 0)}` : `内联蓝图编码 · ${(blueprintDraft.blueprintCode?.length ?? 0)} 字符` }}</span>
          </div>
        </div>
        <div class="node-actions">
          <button type="button" class="node-action-btn" @click="removeBlock">移除标签</button>
          <button type="button" class="node-action-btn is-danger" title="清除蓝图绑定" @click="unbindAndRemove"><Trash2 :size="13" /></button>
        </div>
      </div>
    </template>

    <template v-else-if="modDraft">
      <div class="node-resource-card">
        <div class="node-resource-icon"><Package :size="18" /></div>
        <div class="node-resource-body">
          <div class="node-resource-title">{{ modDraft.label }}</div>
          <div class="node-resource-sub">
            <span class="node-badge">[[resource:mod]]</span>
            <span v-if="modDraft.mode === 'file'">托管模组压缩包 · {{ formatFileSize(modDraft.file?.size ?? 0) }}</span>
            <span v-else>
              Thunderstore 市场
              <template v-if="modDraft.namespace && modDraft.packageName"> · {{ modDraft.namespace }}/{{ modDraft.packageName }}</template>
              <template v-if="modDraft.version"> v{{ modDraft.version }}</template>
            </span>
          </div>
        </div>
        <div class="node-actions">
          <button type="button" class="node-action-btn" @click="removeBlock">移除标签</button>
          <button type="button" class="node-action-btn is-danger" title="清除模组绑定" @click="unbindAndRemove"><Trash2 :size="13" /></button>
        </div>
      </div>
    </template>

    <template v-else-if="saveFile">
      <div class="node-resource-card">
        <div class="node-resource-icon"><FileArchive :size="18" /></div>
        <div class="node-resource-body">
          <div class="node-resource-title">{{ saveFile.name }}</div>
          <div class="node-resource-sub">
            <span class="node-badge">[[resource:save]]</span>
            <span>戴森球主存档 (.dsv) · {{ formatFileSize(saveFile.size) }}</span>
          </div>
        </div>
        <div class="node-actions">
          <button type="button" class="node-action-btn" @click="removeBlock">移除标签</button>
          <button type="button" class="node-action-btn is-danger" title="清除存档绑定" @click="unbindAndRemove"><Trash2 :size="13" /></button>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="node-resource-card is-warning">
        <div class="node-resource-icon is-warning"><AlertTriangle :size="17" /></div>
        <div class="node-resource-body">
          <div class="node-resource-title">未绑定的资源标签：[[resource:{{ slot }}]]</div>
          <div class="node-resource-sub">发布前需要通过工具栏选择对应资源，或直接删除此标签。</div>
        </div>
        <div class="node-actions">
          <button type="button" class="node-action-btn is-danger" @click="removeBlock">删除标签</button>
        </div>
      </div>
    </template>
  </NodeViewWrapper>
</template>

<style scoped lang="scss">
.forum-node-wrapper {
  margin: 10px 0;
  border-radius: 6px;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel-elevated);
  transition: border-color 0.15s ease;
  user-select: none;

  &.is-selected {
    border-color: var(--ds-brand);
    box-shadow: 0 0 0 1px var(--ds-brand);
  }

  &.is-unbound {
    border-color: var(--ds-warning-border);
    background: var(--ds-warning-bg);
  }
}

.node-media-card {
  display: flex;
  flex-direction: column;
}

.node-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--ds-border-subtle);
  font-size: 12px;
}

.node-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--ds-active-bg);
  color: var(--ds-text-primary);
  font-family: var(--ds-font-mono);
  font-size: 11px;
  white-space: nowrap;
}

.node-meta {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ds-text-muted);
}

.node-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.node-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: 24px;
  padding: 0 7px;
  border-radius: 4px;
  border: 1px solid var(--ds-border-subtle);
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 11px;
  cursor: pointer;

  &:hover {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &.is-danger:hover {
    border-color: var(--ds-error-border);
    background: var(--ds-error-bg);
    color: var(--ds-error);
  }
}

.node-image-frame {
  padding: 10px;
  display: flex;
  justify-content: center;
  background: var(--ds-bg-panel);

  img {
    max-width: 100%;
    max-height: 280px;
    border-radius: 4px;
    object-fit: contain;
  }
}

.node-video-frame {
  padding: 10px;
  background: var(--ds-bg-panel);

  video {
    width: 100%;
    max-height: 300px;
    border-radius: 4px;
    background: #000;
  }
}

.node-preview-fallback {
  padding: 16px;
  font-size: 12px;
  color: var(--ds-text-muted);
}

.node-resource-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;

  &.is-warning {
    color: var(--ds-warning);
  }
}

.node-resource-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 6px;
  background: var(--ds-active-bg);
  color: var(--ds-text-primary);
  flex-shrink: 0;

  &.is-warning {
    background: var(--ds-warning-bg);
    color: var(--ds-warning);
  }
}

.node-resource-body {
  flex: 1;
  min-width: 0;
}

.node-resource-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ds-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-resource-sub {
  margin-top: 3px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  font-size: 12px;
  color: var(--ds-text-muted);
}
</style>
