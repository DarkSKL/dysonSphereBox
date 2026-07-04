# DSP Blueprint Editor

戴森球计划蓝图编辑器 & 3D 预览库

三个导出，适用于不同场景：

| 导出 | 说明 | 需要 Vue？ |
|------|------|-----------|
| `createBlueprintApp` | **完整编辑器**（侧边栏、蓝图代码、建筑信息、批量替换、主题切换） | 是 |
| `BlueprintViewer` | **纯 3D 预览器**（仅 Three.js 球体渲染，无 UI） | 否 |
| `parseBlueprint` | **蓝图解析**（字符串 → 数据对象） | 否 |

> `createBlueprintApp` 内部会自动创建一个 `BlueprintViewer`，但不对外暴露。
> 如果你需要在已有页面中嵌入 3D 预览并用代码控制加载/切换蓝图，请直接使用 `BlueprintViewer`。

---

## 安装

```bash
npm install dsp-blueprint-editor three vue@3 vue-i18n@9
```

---

## 快速开始

### 1. 完整编辑器（createBlueprintApp）

渲染完整的蓝图编辑器界面。用户可以在侧边栏粘贴/编辑蓝图代码、查看建筑信息、切换主题等。

```javascript
import { createBlueprintApp } from 'dsp-blueprint-editor';

const app = createBlueprintApp({
    container: '#app',       // CSS 选择器或 HTMLElement
    theme: 'dark',           // 'dark' | 'light' | 'auto'
    locale: 'zh',            // 'zh' | 'en'
});

// 销毁
app.destroy();
```

**浏览器 CDN 用法：**

```html
<script src="https://unpkg.com/vue@3.4/dist/vue.global.prod.js"></script>
<script src="https://unpkg.com/vue-i18n@9.9/dist/vue-i18n.global.prod.js"></script>
<script src="https://unpkg.com/dsp-blueprint-editor"></script>
<script>
    DSPBlueprintEditor.createBlueprintApp({ container: '#app', theme: 'dark', locale: 'zh' });
</script>
```

> **注意：** `createBlueprintApp` 返回 `{ app, destroy }`，不暴露内部的 `BlueprintViewer`。
> 如果需要通过代码加载/切换蓝图，可以在挂载后模拟 textarea 输入：
>
> ```javascript
> setTimeout(() => {
>     const textarea = document.querySelector('.bp-code');
>     const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set;
>     setter.call(textarea, 'BLUEPRINT:xxx');
>     textarea.dispatchEvent(new Event('change', { bubbles: true }));
> }, 300);
> ```

---

### 2. 纯 3D 预览器（BlueprintViewer）

仅渲染 Three.js 3D 球体，不包含任何 UI。适合嵌入已有页面，通过代码完全控制。

```javascript
import { BlueprintViewer } from 'dsp-blueprint-editor';

// 创建实例
const viewer = new BlueprintViewer({
    container: '#canvas',    // CSS 选择器或 HTMLElement
    theme: 'dark',           // 'dark' | 'light' | 'auto'
}, {
    onBuildingSelect: (index) => console.log('选中建筑:', index),
    onCameraChange: () => console.log('相机变化'),
});

// 加载蓝图（可多次调用来切换蓝图）
viewer.loadBlueprint('BLUEPRINT:xxx');

// 随时更换蓝图
viewer.loadBlueprint('BLUEPRINT:yyy');

// 重置视角
viewer.resetCamera();

// 导出图片
const dataUrl = viewer.toDataURL();

// 销毁
viewer.dispose();
```

**浏览器 CDN 用法：**

```html
<script src="https://unpkg.com/three@0.160.0/build/three.min.js"></script>
<script src="https://unpkg.com/dsp-blueprint-editor"></script>
<script>
    const viewer = new DSPBlueprintEditor.BlueprintViewer({ container: '#canvas' });
    viewer.loadBlueprint('BLUEPRINT:xxx');
</script>
```

> **注意：** `BlueprintViewer` 是纯 JS 类，不依赖 Vue。`loadBlueprint(str)` 可以随时调用来更换蓝图并重新渲染。

---

### 3. 蓝图解析（parseBlueprint）

将蓝图字符串解析为数据对象。

```javascript
import { parseBlueprint } from 'dsp-blueprint-editor';

const data = parseBlueprint('BLUEPRINT:xxx');
console.log(data.header);      // { shortDesc, gameVersion, icons, ... }
console.log(data.buildings);   // 建筑数组
console.log(data.areas);       // 区域数组
```

---

## API 参考

### createBlueprintApp

```typescript
function createBlueprintApp(options: BlueprintAppOptions): BlueprintAppHandle
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `container` | `HTMLElement \| string` | - | 容器元素或 CSS 选择器 |
| `theme` | `'dark' \| 'light' \| 'auto'` | `'dark'` | 主题模式 |
| `locale` | `string` | `'en'` | 语言 |

返回：

| 属性 | 类型 | 说明 |
|------|------|------|
| `app` | `VueApp` | Vue 应用实例 |
| `destroy()` | `() => void` | 销毁应用并清理 DOM |

---

### BlueprintViewer

```typescript
new BlueprintViewer(options: ViewerOptions, callbacks?: ViewerEventCallbacks)
```

**构造参数 ViewerOptions：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `container` | `HTMLElement \| string` | - | 容器元素或 CSS 选择器 |
| `theme` | `'dark' \| 'light' \| 'auto'` | `'dark'` | 主题模式 |
| `width` | `number` | 容器宽度 | 渲染器宽度 |
| `height` | `number` | 容器高度 | 渲染器高度 |

**回调 ViewerEventCallbacks：**

| 回调 | 类型 | 说明 |
|------|------|------|
| `onBuildingSelect` | `(index: number \| null) => void` | 建筑选中事件 |
| `onCameraChange` | `() => void` | 相机变化事件 |

**实例方法：**

| 方法 | 返回 | 说明 |
|------|------|------|
| `loadBlueprint(str)` | `void` | 加载蓝图字符串，可多次调用切换蓝图 |
| `loadBlueprintData(data)` | `void` | 加载蓝图数据对象 |
| `getBlueprintData()` | `BlueprintData \| null` | 获取当前蓝图数据 |
| `setTheme(theme)` | `void` | 设置主题 |
| `setCustomTheme(colors)` | `void` | 自定义主题颜色 |
| `getTheme()` | `ThemeColors` | 获取当前主题颜色 |
| `selectBuilding(index \| null)` | `void` | 选中建筑 |
| `getSelectedIndex()` | `number \| null` | 获取选中索引 |
| `getBuildingInfo(index)` | `BuildingInfo \| null` | 获取建筑信息 |
| `getCamera()` | `PerspectiveCamera` | 获取相机对象 |
| `getCameraVersion()` | `number` | 获取相机版本号 |
| `resetCamera()` | `void` | 重置相机位置 |
| `setSize(w, h)` | `void` | 设置渲染器尺寸 |
| `toDataURL()` | `string` | 导出为 base64 图片 |
| `toBlob()` | `Promise<Blob>` | 导出为 Blob |
| `getCanvas()` | `HTMLCanvasElement` | 获取 canvas 元素 |
| `setCallbacks(cbs)` | `void` | 更新事件回调 |
| `dispose()` | `void` | 销毁实例，释放资源 |

---

### parseBlueprint

```typescript
function parseBlueprint(str: string): BlueprintData
```

解析蓝图字符串，返回 `BlueprintData` 对象。解析失败时抛出异常。

---

## 类型定义

```typescript
interface BlueprintAppOptions {
    container: HTMLElement | string;
    theme?: 'dark' | 'light' | 'auto';
    locale?: string;
}

interface BlueprintAppHandle {
    app: VueApp;
    destroy: () => void;
}

interface ViewerOptions {
    container: HTMLElement | string;
    theme?: 'dark' | 'light' | 'auto';
    width?: number;
    height?: number;
}

interface ViewerEventCallbacks {
    onBuildingSelect?: (index: number | null) => void;
    onCameraChange?: () => void;
}

interface ThemeColors {
    sceneBackground: number;
    planet: number;
    grid: number;
    textFill: string;
    textStroke: string;
}

interface BuildingInfo {
    index: number;
    matrix: Matrix4;
}
```

## 依赖

- `three` >= 0.160.0
- `vue` ^3.2.13（仅 createBlueprintApp 需要）
- `vue-i18n` ^9.1.0（仅 createBlueprintApp 需要）
