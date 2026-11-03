package build

import algorithm "DysonSphereBox/helper/blueprint/build/algorithm"

// RuleProfile 职责：公开规则配置；使用：生成诊断；边界：由引擎解析。
type RuleProfile = algorithm.RuleProfile

// RuleAudit 职责：公开规则检查结果；使用：结果诊断；边界：不修改算法。
type RuleAudit = algorithm.RuleAudit

// TransportLane 职责：公开运输通道；使用：结果展示；边界：不执行寻路。
type TransportLane = algorithm.TransportLane

// BoundaryPort 职责：公开边界端口；使用：连接诊断；边界：不建立实际连接。
type BoundaryPort = algorithm.BoundaryPort

// ProductionRateAnnotation 职责：公开产能标注；使用：结果展示；边界：不重新计算产能。
type ProductionRateAnnotation = algorithm.ProductionRateAnnotation

// LogisticsCheck 职责：公开物流检查项；使用：结果诊断；边界：不执行校验。
type LogisticsCheck = algorithm.LogisticsCheck

// SearchCandidateDiagnostic 职责：公开候选诊断；使用：结果分析；边界：不参与候选选择。
type SearchCandidateDiagnostic = algorithm.SearchCandidateDiagnostic

// LayoutMetrics 职责：公开布局指标；使用：结果汇总；边界：不执行布局。
type LayoutMetrics = algorithm.LayoutMetrics

// LogisticsSummary 职责：公开物流汇总；使用：结果展示；边界：不改变生成结果。
type LogisticsSummary = algorithm.LogisticsSummary
