import {Node, mergeAttributes} from '@tiptap/core'
import {VueNodeViewRenderer} from '@tiptap/vue-3'
import type {ForumDraftBindings} from '../../types/forum'
import ForumResourceNodeView from './forum-resource-node-view.vue'

// =============== TipTap 论坛资源块自定义节点 ===============
// 说明：将独立行 [[resource:key]] 映射为 TipTap 块级原子节点，在所见即所得模式下直接渲染本地图片、MP4 播放器或游戏资源卡。
// 边界：不保存二进制内容，只保存槽位键 slot；真实文件元数据从编辑器的响应式 bindings 动态读取。

export interface ForumResourceNodeOptions {
  getBindings: () => ForumDraftBindings
  onRemoveSlot: (slot: string) => void
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    forumResource: {
      insertForumResource: (slot: string) => ReturnType
    }
  }
}

export const ForumResourceNode = Node.create<ForumResourceNodeOptions>({
  name: 'forumResource',
  group: 'block',
  atom: true,
  selectable: true,
  draggable: true,

  addOptions() {
    return {
      getBindings: () => ({images: {}, video: null, blueprint: null, mod: null, save: null}),
      onRemoveSlot: () => {},
    }
  },

  addAttributes() {
    return {
      slot: {
        default: '',
        parseHTML: (element) => element.getAttribute('data-slot') || '',
        renderHTML: (attributes) => ({'data-slot': attributes.slot || ''}),
      },
    }
  },

  parseHTML() {
    return [{tag: 'div[data-forum-resource]'}]
  },

  renderHTML({HTMLAttributes}) {
    return ['div', mergeAttributes({'data-forum-resource': ''}, HTMLAttributes)]
  },

  addCommands() {
    return {
      insertForumResource:
        (slot: string) =>
        ({commands}) => {
          return commands.insertContent([
            {type: this.name, attrs: {slot}},
            {type: 'paragraph'},
          ])
        },
    }
  },

  addNodeView() {
    return VueNodeViewRenderer(ForumResourceNodeView)
  },
})
