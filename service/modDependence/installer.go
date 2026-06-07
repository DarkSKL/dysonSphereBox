package modDependence

// 安装器服务最核心的功能就是 联动市场，当用户点击app的市场卡片中的下载时，会自动联动卡片的下载状态以及下载整合包相关的依赖
// 然后安装器会检查目标目录是否包含插件，如果已经包含则移除，如果
/*
在已经拿到所有依赖压缩包的情况下，、
流程就是先确认游戏目录里已经有可用的 BepInEx 环境，
然后为每个 Mod 包按 Mod名 创建 游戏目录/BepInEx/plugins/ Mod名/ 目录，
把对应 zip 解压出来的全部内容复制进去，
最后保留这些插件目录供 BepInEx 在游戏启动时自动加载。
*/
// 安装完成后在本地SqlLite记录mod_dependency_packages 和mod_dependencies 代表已经安装，本地的SqlLite只记录本地已经安装依赖
// 删除时会先移除mode再移除记录
// 需要跟前端的mod市场页面进行联动

type InstallerService struct {
}
