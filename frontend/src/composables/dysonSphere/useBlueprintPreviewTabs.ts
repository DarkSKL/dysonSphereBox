import {ref, watch, type Ref} from 'vue'

// =============== 预览标签页命令 ===============
// 职责：集中维护下层卡片的「蓝图预览 / 平面预览器」标签与懒挂载标记，并对外暴露代码切换命令。
// 使用：页面用 tab/algorithmMounted 渲染标签栏，用户点击走 selectTab()；数据层收到候选差量时调用 showAlgorithmTab()，
//       算法舞台播放收敛后再调用 showPreviewTab() 切回蓝图预览。
// 边界：命令只改变标签状态，不创建或销毁 WebGL 场景；用户手动切过标签后，数据层本轮不再自动改写。
export type PreviewTab = 'preview' | 'algorithm'

export const previewTabs: {key: PreviewTab, label: string}[] = [{key: 'preview', label: '蓝图预览'}, {key: 'algorithm', label: '平面预览器（内测版）'}]

export const tab = ref<PreviewTab>('preview')
const algorithmMounted = ref(false)
watch(tab, value => { if (value === 'algorithm') algorithmMounted.value = true })
// 用户意图：手动点过标签后，本轮生成内数据层不再自动改写标签，避免把用户切走的画面夺回来
export const userSwitched = ref(false)

/** 命令：切到蓝图预览；数据层在算法播放结束后使用。 */
export const showPreviewTab = (): void => { tab.value = 'preview' }
/** 命令：切到平面预览器；首次切换后保持挂载，之后只隐藏不卸载。 */
export const showAlgorithmTab = (): void => { tab.value = 'algorithm' }
/** 命令：用户手动切换标签，同时记录用户意图。 */
export const selectTab = (key: PreviewTab): void => { userSwitched.value = true; tab.value = key }
/** 命令：新一轮生成开始时清除用户意图，让数据层重新接管自动切换。 */
export const resetTabIntent = (): void => { userSwitched.value = false }

export interface BlueprintPreviewTabs {
  tab: Ref<PreviewTab>
  algorithmMounted: Ref<boolean>
  previewTabs: typeof previewTabs
  userSwitched: Ref<boolean>
  showPreviewTab(): void
  showAlgorithmTab(): void
  selectTab(key: PreviewTab): void
  resetTabIntent(): void
}

export const useBlueprintPreviewTabs = (): BlueprintPreviewTabs => ({
  tab,
  algorithmMounted,
  previewTabs,
  userSwitched,
  showPreviewTab,
  showAlgorithmTab,
  selectTab,
  resetTabIntent,
})

