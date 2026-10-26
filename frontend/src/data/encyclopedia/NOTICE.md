# 图鉴元数据来源

- 数据源：FactorioLab，https://github.com/factoriolab/factoriolab
- 固定提交：`d3a6e0eeb98a415a85ed544ee66ff7b8cde1810f`
- 上游游戏版本：DSP `0.10.29.21950`
- 上游路径：`public/data/dsp/data.json`、`public/data/dsp/i18n/zh.json`
- 元数据转换脚本：`scripts/encyclopedia/import-data.mjs`，由本项目独立实现。
- 上游 MIT 文本：同目录 `LICENSE-FactorioLab.txt`。原文件 SHA-256 记录在 `manifest.json`。
- 游戏、图标、名称及商标的权利归 Youthcat Studio、Gamera Games 和其他原权利人所有；本记录不代表取得游戏资产的再分发授权。

分类布局参考 https://wiki.biligame.com/dsp/首页；没有复制 Wiki 正文、脚本或边框图片。离线阅读摘要由本模块编写。

本地 Game Icons 图标遵循 CC BY 3.0，署名位于 `frontend/public/game-icons-license.txt`。
游戏实体精灵图和 BWIKI 分类图标保存在 `frontend/public/encyclopedia-icons/`，随源码提交并随程序打包，运行时从本地加载。目录内附来源说明和 FactorioLab 仓库许可证文本；文件缺失或损坏时使用打包的类别图标回退。
来源记录和随附仓库许可证不代表对游戏素材、名称或商标取得额外授权。
