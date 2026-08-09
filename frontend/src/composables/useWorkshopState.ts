import {computed, ref} from 'vue'

export type ModPackageKey = 'nebula' | 'qol'

export interface ModItem {
  name: string
  version: string
  description: string
}

export interface ModPackage {
  key: ModPackageKey
  title: string
  description: string
  fileCount: number
  mods: ModItem[]
}

const gamePath = ref('')
const downloadServer = ref('aliyun')
const autoStart = ref(false)
const checkUpdate = ref(true)
const installedPackages = ref<Record<ModPackageKey, boolean>>({
  nebula: false,
  qol: false,
})
const installingPackages = ref<Record<ModPackageKey, boolean>>({
  nebula: false,
  qol: false,
})
const packageProgress = ref<Record<ModPackageKey, number>>({
  nebula: 0,
  qol: 0,
})

export const modPackages: ModPackage[] = [
  {
    key: 'nebula',
    title: '联机模组',
    description: '安装后可创建或加入多人游戏，包含 BepInEx 与 Nebula 联机组件。',
    fileCount: 8,
    mods: [
      {name: 'BepInEx', version: 'v5.4.17', description: '模组框架底座'},
      {name: 'IlLine', version: 'v1.0.0', description: 'IL 注入辅助工具'},
      {name: 'LDBTool', version: 'v2.0.0', description: '数据修改库'},
      {name: 'CommonAPI', version: 'v1.6.0', description: '通用 API 库'},
      {name: 'DSPModSave', version: 'v1.3.0', description: '存档分离工具'},
      {name: 'NebulaMultiplayerModApi', version: 'v2.0.0', description: 'API 接口层'},
      {name: 'NebulaCompatibilityAssist', version: 'v1.0.0', description: '联机兼容补丁'},
      {name: 'NebulaMultiplayerMod', version: 'v0.9.22', description: 'Nebula 联机本体'},
    ],
  },
  {
    key: 'qol',
    title: '常用辅助模组',
    description: '量化计算、工厂定位、瓶颈分析、自动导航，纯辅助模组不修改游戏平衡。',
    fileCount: 5,
    mods: [
      {name: 'AssemblerUI', version: 'v3.0.0', description: '显示组装机 I/O 速率'},
      {name: 'BottleneckUI', version: 'v1.0.0', description: '全星球产线瓶颈检测 (Ctrl+B)'},
      {name: 'DSPCalculator', version: 'v0.5.25', description: '量化计算器 (Q)'},
      {name: 'FactoryLocator', version: 'v1.3.12', description: '工厂矿物储物仓搜索 (Ctrl+F)'},
      {name: 'CAAP_N', version: 'v1.4.5', description: '自动巡航与自动导航'},
    ],
  },
]

let timers: Partial<Record<ModPackageKey, number>> = {}

const isModEnvironmentInstalled = computed(() => {
  return installedPackages.value.nebula || installedPackages.value.qol
})

const installPackage = (key: ModPackageKey): void => {
  if (installingPackages.value[key]) {
    return
  }

  installingPackages.value[key] = true
  packageProgress.value[key] = 0

  window.clearInterval(timers[key])
  timers[key] = window.setInterval(() => {
    packageProgress.value[key] = Math.min(packageProgress.value[key] + 4, 100)

    if (packageProgress.value[key] >= 100) {
      window.clearInterval(timers[key])
      window.setTimeout(() => {
        installedPackages.value[key] = true
        installingPackages.value[key] = false
        packageProgress.value[key] = 0
      }, 350)
    }
  }, 120)
}

const uninstallPackage = (key: ModPackageKey): void => {
  installedPackages.value[key] = false
}

const resetGamePath = (): void => {
  gamePath.value = ''
}

export const useWorkshopState = () => ({
  gamePath,
  downloadServer,
  autoStart,
  checkUpdate,
  installedPackages,
  installingPackages,
  packageProgress,
  isModEnvironmentInstalled,
  installPackage,
  uninstallPackage,
  resetGamePath,
})
