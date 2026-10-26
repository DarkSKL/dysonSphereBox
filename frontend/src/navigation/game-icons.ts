import {Icon, type IconifyIcon} from '@iconify/vue'
import cargoCrate from '@iconify-icons/game-icons/cargo-crate'
import bookAura from '@iconify-icons/game-icons/book-aura'
import cog from '@iconify-icons/game-icons/cog'
import habitatDome from '@iconify-icons/game-icons/habitat-dome'
import processor from '@iconify-icons/game-icons/processor'
import radarDish from '@iconify-icons/game-icons/radar-dish'
import ringedPlanet from '@iconify-icons/game-icons/ringed-planet'
import {defineComponent, h, type Component} from 'vue'

// =============== 科幻导航图标 ===============
// 说明：将按需导入的 Game Icons 数据适配为导航组件，尺寸和颜色沿用侧栏样式。
// 边界：直接传入本地图标数据，不使用图标名称触发在线加载；不为实心图标添加描边。
// 图标来源：https://game-icons.net/；许可：CC BY 3.0，署名随 public/game-icons-license.txt 打包。
function createNavigationIcon(name: string, icon: IconifyIcon): Component {
  return defineComponent({
    name,
    props: {
      size: {type: Number, default: 20},
    },
    setup(props) {
      return () => h(Icon, {icon, width: props.size, height: props.size})
    },
  })
}

/** 戴森球工作区的太空工业图标；仅用于导航，不参与路由和业务状态。 */
export const gameNavigationIcons = {
  planet: createNavigationIcon('GamePlanetIcon', ringedPlanet),
  home: createNavigationIcon('GameHomeIcon', habitatDome),
  headless: createNavigationIcon('GameHeadlessIcon', radarDish),
  mods: createNavigationIcon('GameModsIcon', processor),
  resources: createNavigationIcon('GameResourcesIcon', cargoCrate),
  encyclopedia: createNavigationIcon('GameEncyclopediaIcon', bookAura),
  settings: createNavigationIcon('GameSettingsIcon', cog),
} as const
