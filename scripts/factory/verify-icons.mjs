// 职责：运行量化图标模块并核对全部原版物品及机枪弹箱的真实精灵。
// 使用：在应用根目录运行 node scripts/factory/verify-icons.mjs；边界：只替代浏览器图片探测，不修改数据或联网。
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {createRequire} from 'node:module'
import {fileURLToPath} from 'node:url'
import path from 'node:path'

const root = fileURLToPath(new URL('../../', import.meta.url))
const require = createRequire(path.join(root, 'frontend/package.json'))
const ts = require('typescript')
const vue = require('vue')
const modulePath = path.join(root, 'frontend/src/services/dysonSphere/factory-icons.ts')
const source = await readFile(modulePath, 'utf8')
const code = ts.transpileModule(source, {compilerOptions: {
  target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true,
}}).outputText
const manifest = JSON.parse(await readFile(path.join(root, 'frontend/src/data/encyclopedia/manifest.json'), 'utf8'))
const spriteReady = vue.shallowRef(false)
const exports = {}
const load = specifier => {
  if (specifier === 'vue') return vue
  if (specifier.endsWith('.json')) return require(path.resolve(path.dirname(modulePath), specifier))
  if (specifier === '../encyclopedia/catalog') return {
    encyclopediaManifest: manifest,
    encyclopediaSpriteReady: spriteReady,
    loadLocalIconPack: async () => { spriteReady.value = true },
  }
  throw new Error(`不支持的图标依赖：${specifier}`)
}
new Function('require', 'exports', code)(load, exports)
assert.equal(exports.factoryIconStyle('机枪弹箱', 64), null)
exports.ensureFactoryIcons()
const itemData = JSON.parse(await readFile(path.join(root, 'frontend/src/data/encyclopedia/item.json'), 'utf8'))
const bullet = itemData.find(item => item.id === 'df-magnum-ammo-box')
const titanium = itemData.find(item => item.id === 'df-titanium-ammo-box')
assert.ok(bullet?.icon && titanium?.icon)
const bulletStyle = exports.factoryIconStyle('机枪弹箱', 64)
const titaniumStyle = exports.factoryIconStyle('钛化弹箱', 64)
assert.ok(bulletStyle && titaniumStyle)
assert.equal(bulletStyle.backgroundPosition, `${-bullet.icon.x}px ${-bullet.icon.y}px`)
assert.equal(titaniumStyle.backgroundPosition, `${-titanium.icon.x}px ${-titanium.icon.y}px`)
assert.notEqual(bulletStyle.backgroundPosition, titaniumStyle.backgroundPosition)
assert.equal(exports.factoryIconStyle('机枪弹箱', 32).backgroundPosition, `${-bullet.icon.x / 2}px ${-bullet.icon.y / 2}px`)
const vanilla = JSON.parse((await readFile(path.join(root, 'helper/quant/data/Vanilla.json'), 'utf8')).replace(/^\uFEFF/, ''))
const items = vanilla.items.filter(item => item.ID >= 1000)
assert.equal(items.length, 173)
for (const item of items) {
  const style = exports.factoryIconStyle(item.Name, 64)
  assert.ok(style, `原版物品缺少图标：${item.ID} ${item.Name}`)
  assert.equal(style.backgroundImage, `url("${manifest.sprite.path}")`)
  assert.equal(style.width, '64px')
  const [x, y] = style.backgroundPosition.split(' ').map(value => -Number.parseFloat(value))
  assert.ok(x >= 0 && y >= 0 && x + 64 <= manifest.sprite.width && y + 64 <= manifest.sprite.height)
}
assert.equal(exports.factoryIconStyle('未收录物品', 64), null)
console.log('量化图标校验通过：173 个原版物品，机枪弹箱使用铜色弹箱精灵并与钛化弹箱区分。')
