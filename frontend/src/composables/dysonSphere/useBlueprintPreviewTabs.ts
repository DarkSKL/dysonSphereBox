import {ref, watch, type Ref} from 'vue'

// =============== 预览标签页命令 ===============
// 职责：集中维护下层卡片的「蓝图预览 / 算法可视化」标签与懒挂载标记，并对外暴露代码切换命令。
// 使用：页面用 tab/algorithmMounted 渲染标签栏；数据层收到候选数据时调用 showAlgorithmTab()。
// 边界：命令只改变标签状态，不创建或销毁 WebGL 场景；切回蓝图预览保留为手动触发入口。
export type PreviewTab = 'preview' | 'algorithm'

export const previewTabs: {key: PreviewTab, label: string}[] = [{key: 'preview', label: '蓝图预览'}, {key: 'algorithm', label: '算法可视化'}]

const tab = ref<PreviewTab>('preview')
const algorithmMounted = ref(false)
watch(tab, value => { if (value === 'algorithm') algorithmMounted.value = true })

/** 命令：切到蓝图预览；保持为手动触发入口，不由数据层自动调用。 */
export const showPreviewTab = (): void => { tab.value = 'preview' }
/** 命令：切到算法可视化；首次切换后保持挂载，之后只隐藏不卸载。 */
export const showAlgorithmTab = (): void => { tab.value = 'algorithm' }

export interface BlueprintPreviewTabs {
  tab: Ref<PreviewTab>
  algorithmMounted: Ref<boolean>
  previewTabs: typeof previewTabs
  showPreviewTab(): void
  showAlgorithmTab(): void
}

export const useBlueprintPreviewTabs = (): BlueprintPreviewTabs => ({tab, algorithmMounted, previewTabs, showPreviewTab, showAlgorithmTab})
