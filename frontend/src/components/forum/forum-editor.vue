<script setup lang="ts">
import {
  AlertTriangle,
  Bold,
  Code,
  Code2,
  Eye,
  FileArchive,
  FileCode2,
  Film,
  Heading2,
  Heading3,
  Image as ImageIcon,
  Italic,
  Link2,
  List,
  ListOrdered,
  Minus,
  Package,
  Plus,
  Quote,
  Redo2,
  Strikethrough,
  Trash2,
  Undo2,
} from '@lucide/vue'
import Link from '@tiptap/extension-link'
import Placeholder from '@tiptap/extension-placeholder'
import StarterKit from '@tiptap/starter-kit'
import {EditorContent, useEditor} from '@tiptap/vue-3'
import {useMessage} from 'naive-ui'
import {computed, nextTick, onBeforeUnmount, ref, watch} from 'vue'
import {BrowserOpenURL} from '../../../wailsjs/runtime/runtime'
import {
  releaseForumLocalPreviews,
  selectForumImages,
  selectForumResourceFile,
  selectForumVideo,
} from '../../services/forum'
import type {
  ForumBlueprintDraft,
  ForumDraftBindings,
  ForumModDraft,
} from '../../types/forum'
import {
  buildForumPreviewSegments,
  extractReferencedResourceSlots,
  markdownToTiptapHtml,
  tiptapDocToMarkdown,
} from '../../utils/forum-markdown'
import {formatErrorMessage, formatFileSize} from '../../utils/format'
import ForumBlueprintModal from './forum-blueprint-modal.vue'
import ForumModPickerModal from './forum-mod-picker-modal.vue'
import {ForumResourceNode} from './forum-resource-node'

const props = withDefaults(defineProps<{
  modelValue: string
  bindings: ForumDraftBindings
  mode?: 'post' | 'reply'
  placeholder?: string
  disabled?: boolean
  minHeight?: number
}>(), {
  mode: 'post',
  placeholder: '输入正文内容，或使用上方工具栏插入图片、视频与游戏资源…',
  disabled: false,
  minHeight: 220,
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void
}>()

const message = useMessage()
const editorMode = ref<'wysiwyg' | 'source'>('wysiwyg')
const showSourcePreview = ref(true)
const sourceTextareaRef = ref<HTMLTextAreaElement | null>(null)
const blueprintModalOpen = ref(false)
const modModalOpen = ref(false)
const linkModalOpen = ref(false)
const linkUrlInput = ref('')

// 标记是否由编辑器内部更新，避免循环回写重置光标。
let syncingFromEditor = false

const maxImages = computed(() => (props.mode === 'post' ? 8 : 4))
const maxBodyChars = computed(() => (props.mode === 'post' ? 10000 : 2000))
const bodyCharCount = computed(() => Array.from(props.modelValue || '').length)
const imageCount = computed(() => Object.keys(props.bindings.images).length)
const referencedSlots = computed(() => new Set(extractReferencedResourceSlots(props.modelValue || '')))
const previewSegments = computed(() => (
  editorMode.value === 'source' && showSourcePreview.value
    ? buildForumPreviewSegments(props.modelValue || '', props.bindings)
    : []
))

const blueprintMenuOptions = [
  {label: '上传蓝图文件 (.txt，最大 512 KiB)', key: 'file'},
  {label: '直接粘贴蓝图编码', key: 'inline'},
]

const modMenuOptions = [
  {label: '从模组市场选择 / 粘贴 Thunderstore 链接', key: 'thunderstore'},
  {label: '上传模组压缩包 (.zip，最大 256 MiB)', key: 'file'},
]

const boundItems = computed(() => {
  const items: Array<{slot: string; label: string; detail: string; referenced: boolean}> = []
  for (let i = 1; i <= maxImages.value; i++) {
    const slot = `image${i}`
    const file = props.bindings.images[slot]
    if (file) {
      items.push({
        slot,
        label: file.name,
        detail: `图片 · ${formatFileSize(file.size)}`,
        referenced: referencedSlots.value.has(slot),
      })
    }
  }
  if (props.mode === 'post') {
    if (props.bindings.video) {
      items.push({
        slot: 'video',
        label: props.bindings.video.name,
        detail: `MP4 视频 · ${formatFileSize(props.bindings.video.size)}`,
        referenced: referencedSlots.value.has('video'),
      })
    }
    if (props.bindings.blueprint) {
      items.push({
        slot: 'blueprint',
        label: props.bindings.blueprint.name,
        detail: props.bindings.blueprint.mode === 'file'
          ? `蓝图文件 · ${formatFileSize(props.bindings.blueprint.file?.size ?? 0)}`
          : '内联蓝图编码',
        referenced: referencedSlots.value.has('blueprint'),
      })
    }
    if (props.bindings.mod) {
      items.push({
        slot: 'mod',
        label: props.bindings.mod.label,
        detail: props.bindings.mod.mode === 'file'
          ? `模组文件 · ${formatFileSize(props.bindings.mod.file?.size ?? 0)}`
          : `Thunderstore${props.bindings.mod.version ? ` v${props.bindings.mod.version}` : ''}`,
        referenced: referencedSlots.value.has('mod'),
      })
    }
    if (props.bindings.save) {
      items.push({
        slot: 'save',
        label: props.bindings.save.name,
        detail: `主存档 · ${formatFileSize(props.bindings.save.size)}`,
        referenced: referencedSlots.value.has('save'),
      })
    }
  }
  return items
})

// =============== TipTap 实例配置 ===============
// 说明：基于 StarterKit + Link + Placeholder + 自定义 ForumResourceNode 构造所见即所得编辑器。
// 边界：不启用外部 Image 节点，所有图片与游戏资源都受控挂载为 [[resource:key]] 块节点。
const editor = useEditor({
  content: markdownToTiptapHtml(props.modelValue || ''),
  editable: !props.disabled,
  editorProps: {
    // 粘贴过滤：移除外部 <img> 与伪造的 data-forum-resource 属性，防止外链图片请求或注入未受控资源块。
    transformPastedHTML(html: string): string {
      return html
        .replace(/<img\b[^>]*>/gi, '')
        .replace(/\sdata-forum-resource(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+))?/gi, '')
    },
  },
  extensions: [
    StarterKit.configure({
      heading: {levels: [1, 2, 3, 4]},
    }),
    Link.configure({
      openOnClick: false,
      autolink: true,
      protocols: ['http', 'https'],
      HTMLAttributes: {
        rel: 'noopener noreferrer',
      },
    }),
    Placeholder.configure({
      placeholder: props.placeholder,
    }),
    ForumResourceNode.configure({
      getBindings: () => props.bindings,
      onRemoveSlot: (slot) => removeBinding(slot, false),
    }),
  ],
  onUpdate: ({editor: instance}) => {
    if (editorMode.value !== 'wysiwyg') return
    syncingFromEditor = true
    const markdown = tiptapDocToMarkdown(instance.getJSON())
    emit('update:modelValue', markdown)
    void nextTick(() => {
      syncingFromEditor = false
    })
  },
})

watch(() => props.disabled, (disabled) => {
  editor.value?.setEditable(!disabled)
})

// 外部重置草稿（如发布成功清空）时同步回 TipTap。
watch(() => props.modelValue, (nextValue) => {
  if (syncingFromEditor || editorMode.value !== 'wysiwyg') return
  const instance = editor.value
  if (!instance) return
  const currentMarkdown = tiptapDocToMarkdown(instance.getJSON())
  if (currentMarkdown === (nextValue || '')) return
  instance.commands.setContent(markdownToTiptapHtml(nextValue || ''), false)
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})

// =============== 模式切换（所见即所得 <-> Markdown 源码） ===============
function switchEditorMode(target: 'wysiwyg' | 'source'): void {
  if (editorMode.value === target) return
  editorMode.value = target
  if (target === 'wysiwyg') {
    editor.value?.commands.setContent(markdownToTiptapHtml(props.modelValue || ''), false)
    void nextTick(() => editor.value?.commands.focus('end'))
  } else {
    void nextTick(() => sourceTextareaRef.value?.focus())
  }
}

// =============== 插入资源标签到当前光标位置 ===============
function insertSlotTags(slots: string[]): void {
  const validSlots = slots.filter(Boolean)
  if (!validSlots.length) return

  if (editorMode.value === 'wysiwyg' && editor.value) {
    const contentNodes = validSlots.flatMap((slot) => [
      {type: 'forumResource', attrs: {slot}},
      {type: 'paragraph'},
    ])
    editor.value.chain().focus().insertContent(contentNodes).run()
    return
  }

  const current = props.modelValue || ''
  const textarea = sourceTextareaRef.value
  const tagBlock = validSlots.map((slot) => `[[resource:${slot}]]`).join('\n\n')
  if (!textarea) {
    const joined = current.trimEnd() ? `${current.trimEnd()}\n\n${tagBlock}\n` : `${tagBlock}\n`
    emit('update:modelValue', joined)
    return
  }
  const start = textarea.selectionStart ?? current.length
  const end = textarea.selectionEnd ?? current.length
  const before = current.slice(0, start)
  const after = current.slice(end)
  const prefix = before && !before.endsWith('\n\n') ? (before.endsWith('\n') ? '\n' : '\n\n') : ''
  const suffix = after && !after.startsWith('\n\n') ? (after.startsWith('\n') ? '\n' : '\n\n') : '\n'
  const next = `${before}${prefix}${tagBlock}${suffix}${after}`
  emit('update:modelValue', next)
  const nextCursor = (before + prefix + tagBlock + suffix).length
  void nextTick(() => {
    textarea.focus()
    textarea.setSelectionRange(nextCursor, nextCursor)
  })
}

function insertSlotTag(slot: string): void {
  insertSlotTags([slot])
}

// 从文档中移除指定槽位的全部标签节点或独立行。
function removeSlotTagFromBody(slot: string): void {
  if (editorMode.value === 'wysiwyg' && editor.value) {
    const {state, view} = editor.value
    const tr = state.tr
    const positions: Array<{from: number; to: number}> = []
    state.doc.descendants((node, pos) => {
      if (node.type.name === 'forumResource' && node.attrs.slot === slot) {
        positions.push({from: pos, to: pos + node.nodeSize})
      }
    })
    if (positions.length) {
      for (let i = positions.length - 1; i >= 0; i--) {
        tr.delete(positions[i].from, positions[i].to)
      }
      view.dispatch(tr)
      return
    }
  }

  const escaped = slot.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const pattern = new RegExp(`^\\s*\\[\\[resource:${escaped}\\]\\]\\s*$`, 'gm')
  const cleaned = (props.modelValue || '').replace(pattern, '').replace(/\n{3,}/g, '\n\n').trim()
  emit('update:modelValue', cleaned)
}

// 解除槽位绑定并释放受控本地预览令牌。
function removeBinding(slot: string, alsoRemoveTags = true): void {
  const tokensToRelease: string[] = []
  if (/^image[1-8]$/.test(slot)) {
    const file = props.bindings.images[slot]
    if (file?.previewToken) tokensToRelease.push(file.previewToken)
    delete props.bindings.images[slot]
  } else if (slot === 'video') {
    if (props.bindings.video?.previewToken) tokensToRelease.push(props.bindings.video.previewToken)
    props.bindings.video = null
  } else if (slot === 'blueprint') {
    props.bindings.blueprint = null
  } else if (slot === 'mod') {
    props.bindings.mod = null
  } else if (slot === 'save') {
    props.bindings.save = null
  }
  if (tokensToRelease.length) {
    void releaseForumLocalPreviews(tokensToRelease)
  }
  if (alsoRemoveTags) {
    removeSlotTagFromBody(slot)
  }
}

// =============== 资源插入操作 ===============
async function handleInsertImages(): Promise<void> {
  if (props.disabled) return
  const freeSlots: string[] = []
  for (let i = 1; i <= maxImages.value; i++) {
    const slot = `image${i}`
    if (!props.bindings.images[slot]) freeSlots.push(slot)
  }
  if (!freeSlots.length) {
    message.warning(`最多只能插入 ${maxImages.value} 张图片`)
    return
  }
  try {
    const result = await selectForumImages(freeSlots.length)
    if (result.canceled || !result.files.length) return
    const insertedSlots: string[] = []
    result.files.forEach((file, idx) => {
      const slot = freeSlots[idx]
      if (!slot) return
      props.bindings.images[slot] = file
      insertedSlots.push(slot)
    })
    insertSlotTags(insertedSlots)
  } catch (error) {
    message.error(formatErrorMessage(error))
  }
}

async function handleInsertVideo(): Promise<void> {
  if (props.disabled || props.mode !== 'post') return
  try {
    const result = await selectForumVideo()
    if (result.canceled || !result.files.length) return
    const nextFile = result.files[0]
    if (props.bindings.video?.previewToken) {
      void releaseForumLocalPreviews([props.bindings.video.previewToken])
    }
    const hadTag = referencedSlots.value.has('video')
    props.bindings.video = nextFile
    if (!hadTag) {
      insertSlotTag('video')
    }
  } catch (error) {
    message.error(formatErrorMessage(error))
  }
}

async function handleBlueprintMenu(key: string): Promise<void> {
  if (props.disabled || props.mode !== 'post') return
  if (key === 'inline') {
    blueprintModalOpen.value = true
    return
  }
  try {
    const result = await selectForumResourceFile('blueprint')
    if (result.canceled || !result.files.length) return
    const file = result.files[0]
    const hadTag = referencedSlots.value.has('blueprint')
    props.bindings.blueprint = {
      mode: 'file',
      name: file.name,
      file,
    }
    if (!hadTag) {
      insertSlotTag('blueprint')
    }
  } catch (error) {
    message.error(formatErrorMessage(error))
  }
}

function handleConfirmInlineBlueprint(draft: ForumBlueprintDraft): void {
  const hadTag = referencedSlots.value.has('blueprint')
  props.bindings.blueprint = draft
  if (!hadTag) {
    insertSlotTag('blueprint')
  }
}

async function handleModMenu(key: string): Promise<void> {
  if (props.disabled || props.mode !== 'post') return
  if (key === 'thunderstore') {
    modModalOpen.value = true
    return
  }
  try {
    const result = await selectForumResourceFile('mod')
    if (result.canceled || !result.files.length) return
    const file = result.files[0]
    const hadTag = referencedSlots.value.has('mod')
    props.bindings.mod = {
      mode: 'file',
      label: file.name,
      file,
    }
    if (!hadTag) {
      insertSlotTag('mod')
    }
  } catch (error) {
    message.error(formatErrorMessage(error))
  }
}

function handleConfirmThunderstoreMod(draft: ForumModDraft): void {
  const hadTag = referencedSlots.value.has('mod')
  props.bindings.mod = draft
  if (!hadTag) {
    insertSlotTag('mod')
  }
}

async function handleInsertSave(): Promise<void> {
  if (props.disabled || props.mode !== 'post') return
  try {
    const result = await selectForumResourceFile('save')
    if (result.canceled || !result.files.length) return
    const file = result.files[0]
    const hadTag = referencedSlots.value.has('save')
    props.bindings.save = file
    if (!hadTag) {
      insertSlotTag('save')
    }
  } catch (error) {
    message.error(formatErrorMessage(error))
  }
}

// =============== 链接工具栏按钮 ===============
function openLinkModal(): void {
  if (!editor.value || editorMode.value !== 'wysiwyg') return
  linkUrlInput.value = String(editor.value.getAttributes('link').href || '')
  linkModalOpen.value = true
}

function applyLink(): void {
  if (!editor.value) return
  const href = linkUrlInput.value.trim()
  if (!href) {
    editor.value.chain().focus().extendMarkRange('link').unsetLink().run()
    linkModalOpen.value = false
    return
  }
  if (!/^https?:\/\//i.test(href)) {
    message.warning('仅支持 http:// 或 https:// 链接')
    return
  }
  editor.value.chain().focus().extendMarkRange('link').setLink({href}).run()
  linkModalOpen.value = false
}

function onSourceInput(event: Event): void {
  const target = event.target as HTMLTextAreaElement
  emit('update:modelValue', target.value)
}

function handlePreviewClick(event: MouseEvent): void {
  const target = event.target as HTMLElement | null
  const anchor = target?.closest('a[data-forum-link]') as HTMLAnchorElement | null
  if (!anchor) return
  event.preventDefault()
  const href = anchor.getAttribute('data-forum-link') || anchor.getAttribute('href') || ''
  if (/^https?:\/\//i.test(href)) {
    BrowserOpenURL(href)
  }
}

function slotDisplayName(slot: string): string {
  if (slot === 'video') return props.bindings.video?.name || '视频'
  if (slot === 'blueprint') return props.bindings.blueprint?.name || '蓝图'
  if (slot === 'mod') return props.bindings.mod?.label || '模组'
  if (slot === 'save') return props.bindings.save?.name || '存档'
  return props.bindings.images[slot]?.name || slot
}
</script>

<template>
  <!-- ~ 论坛特化 TipTap 所见即所得 Markdown 编辑器 -->
  <!-- * 说明：默认所见即所得模式，内置 [[resource:...]] 块节点原位预览；可随时切换 Markdown 源码与全文实时预览 -->
  <!-- * 交互：工具栏直接触发本地文件选择、蓝图编码或模组市场选择，自动插入对应标签并维护绑定状态 -->
  <!-- * 边界：禁用外部图片直连与原始 HTML；回复模式仅开放图片（最多 4 张） -->
  <div class="forum-editor" :class="{'is-disabled': disabled}">
    <div class="editor-toolbar">
      <div class="toolbar-group" v-if="editorMode === 'wysiwyg' && editor">
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('heading', {level: 2})}" title="二级标题" @click="editor.chain().focus().toggleHeading({level: 2}).run()"><Heading2 :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('heading', {level: 3})}" title="三级标题" @click="editor.chain().focus().toggleHeading({level: 3}).run()"><Heading3 :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('bold')}" title="加粗" @click="editor.chain().focus().toggleBold().run()"><Bold :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('italic')}" title="斜体" @click="editor.chain().focus().toggleItalic().run()"><Italic :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('strike')}" title="删除线" @click="editor.chain().focus().toggleStrike().run()"><Strikethrough :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('code')}" title="行内代码" @click="editor.chain().focus().toggleCode().run()"><Code :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('codeBlock')}" title="代码块" @click="editor.chain().focus().toggleCodeBlock().run()"><Code2 :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('blockquote')}" title="引用" @click="editor.chain().focus().toggleBlockquote().run()"><Quote :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('bulletList')}" title="无序列表" @click="editor.chain().focus().toggleBulletList().run()"><List :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('orderedList')}" title="有序列表" @click="editor.chain().focus().toggleOrderedList().run()"><ListOrdered :size="15" /></button>
        <button type="button" class="tool-btn" :class="{'is-active': editor.isActive('link')}" title="插入链接" @click="openLinkModal"><Link2 :size="15" /></button>
        <button type="button" class="tool-btn" title="分割线" @click="editor.chain().focus().setHorizontalRule().run()"><Minus :size="15" /></button>
        <span class="toolbar-divider" />
        <button type="button" class="tool-btn" :disabled="!editor.can().undo()" title="撤销" @click="editor.chain().focus().undo().run()"><Undo2 :size="15" /></button>
        <button type="button" class="tool-btn" :disabled="!editor.can().redo()" title="重做" @click="editor.chain().focus().redo().run()"><Redo2 :size="15" /></button>
      </div>

      <span v-if="editorMode === 'wysiwyg'" class="toolbar-divider" />

      <div class="toolbar-group is-insert">
        <button type="button" class="tool-insert-btn" :disabled="disabled || imageCount >= maxImages" @click="handleInsertImages">
          <ImageIcon :size="14" />
          <span>图片 {{ imageCount }}/{{ maxImages }}</span>
        </button>

        <template v-if="mode === 'post'">
          <button type="button" class="tool-insert-btn" :class="{'is-bound': Boolean(bindings.video)}" :disabled="disabled" @click="handleInsertVideo">
            <Film :size="14" />
            <span>{{ bindings.video ? '替换视频' : '视频' }}</span>
          </button>

          <n-dropdown trigger="click" :options="blueprintMenuOptions" @select="handleBlueprintMenu">
            <button type="button" class="tool-insert-btn" :class="{'is-bound': Boolean(bindings.blueprint)}" :disabled="disabled">
              <FileCode2 :size="14" />
              <span>{{ bindings.blueprint ? '替换蓝图' : '蓝图' }}</span>
            </button>
          </n-dropdown>

          <n-dropdown trigger="click" :options="modMenuOptions" @select="handleModMenu">
            <button type="button" class="tool-insert-btn" :class="{'is-bound': Boolean(bindings.mod)}" :disabled="disabled">
              <Package :size="14" />
              <span>{{ bindings.mod ? '替换模组' : '模组' }}</span>
            </button>
          </n-dropdown>

          <button type="button" class="tool-insert-btn" :class="{'is-bound': Boolean(bindings.save)}" :disabled="disabled" @click="handleInsertSave">
            <FileArchive :size="14" />
            <span>{{ bindings.save ? '替换存档' : '存档' }}</span>
          </button>
        </template>
      </div>

      <div class="toolbar-spacer" />

      <div class="mode-switch">
        <button type="button" class="mode-btn" :class="{'is-active': editorMode === 'wysiwyg'}" @click="switchEditorMode('wysiwyg')">所见即所得</button>
        <button type="button" class="mode-btn" :class="{'is-active': editorMode === 'source'}" @click="switchEditorMode('source')">Markdown 源码</button>
        <button
          v-if="editorMode === 'source'"
          type="button"
          class="mode-btn"
          :class="{'is-active': showSourcePreview}"
          title="切换源码实时预览"
          @click="showSourcePreview = !showSourcePreview"
        >
          <Eye :size="13" />预览
        </button>
      </div>
    </div>

    <div v-if="editorMode === 'wysiwyg'" class="editor-wysiwyg-body" :style="{minHeight: `${minHeight}px`}">
      <EditorContent v-if="editor" :editor="editor" class="tiptap-content-host" />
    </div>

    <div v-else class="editor-source-body" :class="{'has-preview': showSourcePreview}" :style="{minHeight: `${minHeight}px`}">
      <textarea
        ref="sourceTextareaRef"
        class="source-textarea"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        spellcheck="false"
        @input="onSourceInput"
      />
      <div v-if="showSourcePreview" class="source-preview" @click="handlePreviewClick">
        <div class="source-preview-title">实时预览（支持跨资源的引用式链接）</div>
        <div v-if="!previewSegments.length" class="source-preview-empty">在此查看 Markdown 与内嵌资源实时渲染效果。</div>
        <template v-for="(seg, idx) in previewSegments" :key="idx">
          <div v-if="seg.kind === 'html'" class="preview-prose" v-html="seg.html" />
          <div v-else class="preview-slot-chip" :class="{'is-unbound': !seg.bound}">
            <template v-if="seg.bound && /^image[1-8]$/.test(seg.slot) && bindings.images[seg.slot]?.previewUrl">
              <img :src="bindings.images[seg.slot].previewUrl" :alt="slotDisplayName(seg.slot)" class="preview-inline-img" />
              <span class="preview-slot-caption">[[resource:{{ seg.slot }}]] · {{ slotDisplayName(seg.slot) }}</span>
            </template>
            <template v-else-if="seg.bound && seg.slot === 'video' && bindings.video?.previewUrl">
              <video :src="bindings.video.previewUrl" controls preload="metadata" class="preview-inline-video" />
              <span class="preview-slot-caption">[[resource:video]] · {{ slotDisplayName('video') }}</span>
            </template>
            <template v-else-if="seg.bound">
              <span class="preview-bound-tag">[[resource:{{ seg.slot }}]]</span>
              <span>{{ slotDisplayName(seg.slot) }}</span>
            </template>
            <template v-else>
              <AlertTriangle :size="14" />
              <span>未绑定资源标签：[[resource:{{ seg.slot }}]]</span>
            </template>
          </div>
        </template>
      </div>
    </div>

    <!-- ~ 已绑定资源托盘 -->
    <!-- * 说明：展示本次草稿全部已选资源及是否已在正文引用，避免误删标签导致服务端校验失败 -->
    <div v-if="boundItems.length > 0" class="binding-tray">
      <div v-for="item in boundItems" :key="item.slot" class="binding-chip" :class="{'is-unreferenced': !item.referenced}">
        <span class="chip-slot">[[resource:{{ item.slot }}]]</span>
        <span class="chip-name" :title="item.label">{{ item.label }}</span>
        <span class="chip-detail">{{ item.detail }}</span>
        <button v-if="!item.referenced" type="button" class="chip-action is-insert" @click="insertSlotTag(item.slot)">
          <Plus :size="12" />插入正文
        </button>
        <button type="button" class="chip-action is-remove" title="移除资源绑定及正文标签" @click="removeBinding(item.slot, true)">
          <Trash2 :size="12" />
        </button>
      </div>
    </div>

    <div class="editor-footer">
      <span class="footer-hint">支持在正文任意位置插入资源块；每个已选资源需保留在正文中才会随提交生效。</span>
      <span class="footer-count" :class="{'is-over': bodyCharCount > maxBodyChars}">{{ bodyCharCount }} / {{ maxBodyChars }}</span>
      <div v-if="$slots.actions" class="footer-actions">
        <slot name="actions" />
      </div>
    </div>

    <ForumBlueprintModal v-model:show="blueprintModalOpen" :initial="bindings.blueprint" @confirm="handleConfirmInlineBlueprint" />
    <ForumModPickerModal v-model:show="modModalOpen" :initial="bindings.mod" @confirm="handleConfirmThunderstoreMod" />

    <n-modal v-model:show="linkModalOpen" preset="card" title="设置链接" style="width: min(90vw, 440px);">
      <n-input v-model:value="linkUrlInput" placeholder="https://example.com" clearable @keyup.enter="applyLink" />
      <template #footer>
        <div class="link-modal-footer">
          <n-button secondary @click="linkModalOpen = false">取消</n-button>
          <n-button type="primary" @click="applyLink">确定</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped lang="scss">
.forum-editor {
  display: flex;
  flex-direction: column;
  border-radius: 4px;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel);
  overflow: hidden;

  &.is-disabled {
    opacity: 0.72;
    pointer-events: none;
  }
}

.editor-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 7px 10px;
  border-bottom: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel-elevated);
}

.toolbar-group {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 2px;
}

.toolbar-divider {
  width: 1px;
  height: 18px;
  margin: 0 4px;
  background: var(--ds-border-default);
}

.toolbar-spacer {
  flex: 1;
}

.tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-secondary);
  cursor: pointer;

  &:hover:not(:disabled) {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &.is-active {
    background: var(--ds-active-bg);
    color: var(--ds-brand);
  }

  &:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }
}

.tool-insert-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 8px;
  border-radius: 4px;
  border: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-primary);
  font-size: 12px;
  cursor: pointer;

  &:hover:not(:disabled) {
    border-color: var(--ds-border-default);
    background: var(--ds-hover-bg);
  }

  &.is-bound {
    border-color: var(--ds-brand);
    color: var(--ds-brand);
  }

  &:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
}

.mode-switch {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 5px;
  background: var(--ds-active-bg);
}

.mode-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: 24px;
  padding: 0 8px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 12px;
  cursor: pointer;

  &.is-active {
    background: var(--ds-bg-panel-elevated);
    color: var(--ds-text-primary);
    font-weight: 600;
  }
}

.editor-wysiwyg-body {
  padding: 12px 14px;
  overflow-y: auto;
  max-height: 480px;
}

.tiptap-content-host {
  :deep(.ProseMirror) {
    outline: none;
    min-height: 180px;
    font-size: 14px;
    line-height: 1.75;
    color: var(--ds-text-primary);
    word-break: break-word;

    p.is-editor-empty:first-child::before {
      content: attr(data-placeholder);
      float: left;
      color: var(--ds-text-muted);
      pointer-events: none;
      height: 0;
    }

    h1, h2, h3, h4 {
      margin: 14px 0 8px;
      font-weight: 700;
      line-height: 1.4;
    }

    h2 { font-size: 17px; }
    h3 { font-size: 15px; }

    p { margin: 0 0 8px; }
    ul { list-style: disc; padding-left: 22px; margin: 6px 0 10px; }
    ol { list-style: decimal; padding-left: 22px; margin: 6px 0 10px; }

    blockquote {
      margin: 8px 0;
      padding: 6px 12px;
      border-left: 3px solid var(--ds-border-strong);
      background: var(--ds-hover-bg);
      color: var(--ds-text-secondary);
    }

    code {
      padding: 1px 5px;
      border-radius: 4px;
      background: var(--ds-active-bg);
      font-family: var(--ds-font-mono);
      font-size: 12.5px;
    }

    pre {
      margin: 8px 0;
      padding: 10px 12px;
      border-radius: 6px;
      border: 1px solid var(--ds-border-subtle);
      background: var(--ds-bg-panel);
      font-family: var(--ds-font-mono);
      overflow-x: auto;

      code {
        padding: 0;
        background: transparent;
      }
    }

    a {
      color: var(--ds-brand);
      text-decoration: underline;
    }

    hr {
      margin: 12px 0;
      border: none;
      border-top: 1px solid var(--ds-border-default);
    }
  }
}

.editor-source-body {
  display: grid;
  grid-template-columns: 1fr;
  max-height: 480px;

  &.has-preview {
    grid-template-columns: 1fr 1fr;
  }
}

.source-textarea {
  width: 100%;
  min-height: 220px;
  padding: 12px 14px;
  border: none;
  outline: none;
  resize: vertical;
  background: transparent;
  color: var(--ds-text-primary);
  font-family: var(--ds-font-mono);
  font-size: 13px;
  line-height: 1.7;
}

.source-preview {
  padding: 10px 14px;
  border-left: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel);
  overflow-y: auto;
  font-size: 13.5px;
  line-height: 1.7;
}

.source-preview-title {
  margin-bottom: 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--ds-text-muted);
}

.source-preview-empty {
  color: var(--ds-text-muted);
  font-size: 12px;
}

.preview-prose {
  :deep(p) { margin: 0 0 8px; }
  :deep(pre) {
    padding: 8px 10px;
    border-radius: 4px;
    background: var(--ds-bg-panel-elevated);
    overflow-x: auto;
  }
  :deep(a) { color: var(--ds-brand); text-decoration: underline; }
}

.preview-slot-chip {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 8px 0;
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel-elevated);
  font-size: 12px;

  &.is-unbound {
    flex-direction: row;
    align-items: center;
    border-color: var(--ds-warning-border);
    background: var(--ds-warning-bg);
    color: var(--ds-warning);
  }
}

.preview-inline-img {
  max-width: 100%;
  max-height: 180px;
  object-fit: contain;
  border-radius: 4px;
}

.preview-inline-video {
  width: 100%;
  max-height: 200px;
  border-radius: 4px;
  background: #000;
}

.preview-slot-caption,
.preview-bound-tag {
  font-family: var(--ds-font-mono);
  font-size: 11px;
  color: var(--ds-text-muted);
}

.binding-tray {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 10px;
  border-top: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel);
}

.binding-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 3px 8px;
  border-radius: 4px;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel-elevated);
  font-size: 12px;

  &.is-unreferenced {
    border-color: var(--ds-warning-border);
    background: var(--ds-warning-bg);
  }
}

.chip-slot {
  font-family: var(--ds-font-mono);
  font-size: 11px;
  color: var(--ds-brand);
}

.chip-name {
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ds-text-primary);
}

.chip-detail {
  font-size: 11px;
  color: var(--ds-text-muted);
}

.chip-action {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 5px;
  border: none;
  border-radius: 3px;
  background: transparent;
  font-size: 11px;
  cursor: pointer;

  &.is-insert {
    color: var(--ds-warning);
    &:hover { background: var(--ds-hover-bg); }
  }

  &.is-remove {
    color: var(--ds-text-muted);
    &:hover { color: var(--ds-error); background: var(--ds-error-bg); }
  }
}

.editor-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 10px;
  border-top: 1px solid var(--ds-border-subtle);
  font-size: 11.5px;
  color: var(--ds-text-muted);
}

.footer-count {
  margin-left: auto;
  white-space: nowrap;
  font-family: var(--ds-font-mono);

  &.is-over {
    color: var(--ds-error);
    font-weight: 600;
  }
}

.footer-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  margin-left: auto;
}

.link-modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
