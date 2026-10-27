// 职责：把真实页面接到隔离 Go 服务；使用：本地验证 URL；边界：环境写入、剪贴板仅记录内存，不连接真实桌面或游戏资源。
import {createApp, defineComponent, h, ref, computed, KeepAlive} from 'vue'
import {createRouter, createMemoryHistory, RouterView} from 'vue-router'
import {NConfigProvider, NDialogProvider, NMessageProvider, darkTheme} from 'naive-ui'
import QuantWorkspace from '../../src/pages/dysonSphere/blueprint-generator.vue'
import {useThemeMode} from '../../src/composables/useThemeMode'
import '../../src/style.scss'

const fixture = window as Window & {go: unknown; runtime: unknown; fixture: {copied: string; saved: string; workspaceJSON: string; toggleTheme: () => void; navigate: (path: string) => Promise<unknown>}}
const {isDarkMode: dark, setThemeMode} = useThemeMode()
fixture.fixture = {copied:'',saved:'',workspaceJSON:'',toggleTheme:()=>{void setThemeMode(dark.value?'light':'dark')},navigate: async path => router.push(path)}
fixture.go = {factory:{Service:Object.fromEntries(['ListDatasets','DescribeDataset','Solve','Generate','Inspect','ReadBlueprintChunk'].map(method=>[method,async (request: unknown)=>{const response=await fetch(`http://127.0.0.1:5174/${method}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(request??null)});return response.json()}]))},system:{ConfigurationService:{GetConfig:async()=> 'light',SetConfig:async()=>{}}},environment:{EnvironmentManagerService:{QuickImportBlueprint:async (_root: string,_name: string,code: string)=>{fixture.fixture.saved=code;return {message:'隔离入库验证完成'}},GetBlueprintDetail:async()=>({code:fixture.fixture.saved,item:{name:'隔离蓝图'}})}}}
const bridge=fixture.go as {factory: {Service: Record<string, (request: unknown) => Promise<unknown>>}}
bridge.factory.Service.ExportWorkspace=async request=>{const response=await fetch('http://127.0.0.1:5174/WorkspaceRecord',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(request)});const record=await response.json();if(record.code!=='OK')throw new Error(record.code);fixture.fixture.workspaceJSON=record.data;return 'memory/方案.json'}
fixture.runtime={ClipboardSetText:async(text:string)=>{fixture.fixture.copied=text;return true}}
const router=createRouter({history:createMemoryHistory(),routes:[{path:'/dysonSphere/blueprint-generator',component:QuantWorkspace},{path:'/away',component:defineComponent({render:()=>h('div','缓存页外')})}]})
const root=defineComponent({setup(){const theme=computed(()=>dark.value?darkTheme:null);return()=>h(NConfigProvider,{theme:theme.value},()=>h(NDialogProvider,{},()=>h(NMessageProvider,{},()=>h(RouterView,{}, {default:({Component}: {Component:unknown})=>h(KeepAlive,{},()=>Component)}))))}})
await router.push('/dysonSphere/blueprint-generator')
createApp(root).use(router).mount('#app')
