// =============== 游戏目录与 BepInEx ===============
// 说明：集中封装游戏设置页使用的 Wails 绑定：游戏目录读写、目录选择、Steam 识别与 BepInEx 状态/安装/卸载。
// 使用：游戏设置页各分区调用；configuredGameDir 为模块级共享状态，BepInEx 分区据此刷新。
// 边界：游戏目录校验以后端 DSPGAME.exe 检测为准；前端不直接访问文件系统。

import {computed, ref} from 'vue'
import {SelectDysonSphereDirectory} from '../../../wailsjs/go/main/App'
import {GetLatestVersion, GetStatus, Install, Uninstall} from '../../../wailsjs/go/modDependence/BepInExService'
import {contract} from '../../../wailsjs/go/models'
import {GetConfig, SetConfig} from '../../../wailsjs/go/system/ConfigurationService'
import {CheckDysonSphereExe, LoadOrInitDysonSphereExe} from '../../../wailsjs/go/system/EnvironmentService'

const GAME_CONFIG_GROUP = 'Game'
const GAME_CONFIG_KEY = 'StartupFilePath'

export type BepInExStatus = contract.BepInExStatus
export type BepInExPackage = contract.BepInExPackage

export type GameDirSelectResult =
  | {readonly status: 'cancelled'}
  | {readonly status: 'saved', readonly path: string}
  | {readonly status: 'invalid', readonly path: string}

interface GameBridgeWindow extends Window {
  go?: Record<string, unknown>
}

// 已保存且有效的游戏目录；空字符串表示未配置或已失效
const configuredGameDir = ref('')
// 数据库中保存的原始值，目录失效时仍用于回显
const storedGameDir = ref('')
const isGameDirLoaded = ref(false)

const assertDesktopBridge = (): void => {
  if (!(window as GameBridgeWindow).go) {
    throw new Error('游戏设置需要在戴森球盒子桌面应用中运行（wails dev 或打包应用）。')
  }
}

// =============== 游戏目录 ===============
// 说明：读取配置后再做一次 DSPGAME.exe 检测，区分“未配置”和“已保存但目录已失效”。
export async function loadGameDirectory(): Promise<void> {
  assertDesktopBridge()
  try {
    const stored = (await GetConfig(GAME_CONFIG_GROUP, GAME_CONFIG_KEY)).trim()
    storedGameDir.value = stored
    configuredGameDir.value = stored && await CheckDysonSphereExe(stored) ? stored : ''
  } finally {
    isGameDirLoaded.value = true
  }
}

// saveGameDirectory 保存手动填写的路径；后端会归一化为游戏目录，找不到 DSPGAME.exe 时拒绝保存。
export async function saveGameDirectory(raw: string): Promise<string> {
  assertDesktopBridge()
  await SetConfig(GAME_CONFIG_GROUP, GAME_CONFIG_KEY, raw.trim())
  await loadGameDirectory()
  return configuredGameDir.value
}

// selectGameDirectory 打开系统目录选择器；有效目录由后端直接保存，无效目录只返回原路径不保存。
export async function selectGameDirectory(): Promise<GameDirSelectResult> {
  assertDesktopBridge()
  const path = await SelectDysonSphereDirectory()
  if (!path) {
    return {status: 'cancelled'}
  }
  if (!await CheckDysonSphereExe(path)) {
    return {status: 'invalid', path}
  }
  storedGameDir.value = path
  configuredGameDir.value = path
  return {status: 'saved', path}
}

// detectGameDirectory 优先沿用已保存的有效目录，否则从 Steam 库定位并保存。
export async function detectGameDirectory(): Promise<string> {
  assertDesktopBridge()
  const path = await LoadOrInitDysonSphereExe()
  if (!path) {
    throw new Error('未能在 Steam 库中找到戴森球计划，请手动选择游戏目录。')
  }
  storedGameDir.value = path
  configuredGameDir.value = path
  return path
}

// =============== BepInEx ===============
// 说明：全部操作作用于已保存的游戏目录；调用前应确认 configuredGameDir 非空。
export function getBepInExStatus(): Promise<BepInExStatus> {
  assertDesktopBridge()
  return GetStatus()
}

// getLatestBepInExPackage 查询 Thunderstore 上可安装的 BepInEx 版本。
export function getLatestBepInExPackage(): Promise<BepInExPackage> {
  assertDesktopBridge()
  return GetLatestVersion()
}

// installBepInEx 下载并安装 BepInEx；已安装时后端只确认或修复 Doorstop 配置。
export function installBepInEx(): Promise<contract.BepInExInstallResult> {
  assertDesktopBridge()
  return Install()
}

// uninstallBepInEx 删除游戏目录下的 BepInEx 目录与 Doorstop 注入文件。
export function uninstallBepInEx(): Promise<contract.BepInExUninstallResult> {
  assertDesktopBridge()
  return Uninstall()
}

export const gameDirectory = configuredGameDir
export const gameDirectoryStored = storedGameDir
export const gameDirectoryLoaded = isGameDirLoaded
export const hasGameDirectory = computed(() => Boolean(configuredGameDir.value))
