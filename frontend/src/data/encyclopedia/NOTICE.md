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
游戏实体精灵图通过可选的 `frontend/public/encyclopedia-icons/` 本地资源包加载，该目录被 Git 排除。未准备资源包时，UI 使用打包的类别图标。
正式分发任何实体图标前，由项目维护者确认许可范围；本次不提交、推送或发布图标包。
