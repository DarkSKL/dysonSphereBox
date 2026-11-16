package build

// 文件职责：只汇合全部准确匹配的单项证明并组装最终报告。
// 使用：工作流最终批准步骤；边界：不代跑缺少的规则，不度量实体、不修复布局，缺任意硬规则都拒绝编码权限。

import "context"

// verifyCompleteRuleSet 职责：核验精确必需集合、来源及依赖谱系；使用：批准与编码守卫；边界：不执行实体规则，只验证封闭证据身份。
func verifyCompleteRuleSet(g GeometrySnapshot, proofs []RuleProof) (map[RuleKind]RuleProof, error) {
	profile, err := verifiedRuleProfile()
	if err != nil {
		return nil, err
	}
	required := RequiredRuleKinds()
	if len(proofs) != len(required) {
		return nil, ruleProofError("最终批准缺少必需规则或包含重复证明")
	}
	byKind := make(map[RuleKind]RuleProof, len(proofs))
	for _, proof := range proofs {
		if err := verifyRuleProof(g, proof, profile); err != nil {
			return nil, err
		}
		if _, exists := byKind[proof.Kind()]; exists {
			return nil, ruleProofError("最终批准包含重复规则证明")
		}
		byKind[proof.Kind()] = proof
	}
	for _, kind := range required {
		proof, ok := byKind[kind]
		if !ok {
			return nil, ruleProofError("最终批准缺少必需规则：" + string(kind))
		}
		for _, parent := range proof.data.parents {
			if known := byKind[parent.kind].data; known == nil || known.stamp != parent.stamp {
				return nil, ruleProofError("最终批准混用了不同检查谱系的依赖证明")
			}
		}
	}
	return byKind, nil
}

// CompleteValidation 职责：只检查全部必需同内容证明并组合已存在输出；使用：唯一最终工作流批准工具；边界：未提供的规则绝不在此补跑，报告发布不修改GeometrySnapshot。
func CompleteValidation(ctx context.Context, g GeometrySnapshot, proofs ...RuleProof) (VerifiedSnapshot, error) {
	if err := checkFinalGeometry(ctx, g); err != nil {
		return VerifiedSnapshot{}, err
	}
	byKind, err := verifyCompleteRuleSet(g, proofs)
	if err != nil {
		return VerifiedSnapshot{}, err
	}
	metrics := byKind[RuleLayoutMetrics].data.logistics
	density := byKind[RuleDensityReport].data.density
	if metrics == nil || metrics.summary == nil || density == nil {
		return VerifiedSnapshot{}, ruleProofError("最终批准缺少已计算指标或密度产物")
	}
	c := newFinalizationCloner()
	logistics := copyFinalizationPointer(c, metrics.summary, c.buildLogisticsSummary)
	// 只读取既有检查报告；机器独立证明不在物流谱系内，在此保序合并。
	for _, check := range byKind[RuleMachines].data.checks {
		found := false
		for _, known := range logistics.Checks {
			found = found || known.RuleID == check.RuleID
		}
		if !found {
			logistics.Checks = append(logistics.Checks, check)
		}
	}
	ruleCtx := g.data.rules.validationContext(ctx)
	audit := composePlanetRuleAudit(ruleCtx, byKind[RuleReferences].data.profile, g.data.request, g.data.surface, g.data.dataset.Info.GameVersion, byKind[RulePowerNetwork].data.networkCount)
	var plan *SurfacePlan
	if g.data.surface != nil {
		plan = copyFinalizationPointer(c, g.data.surface.plan, c.buildSurfacePlan)
		plan.BeltConnectionsChecked = true
		plan.StaticCollidersChecked = true
		plan.ColliderGameVersion = audit.Profile.GameVersion
		plan.CollisionCoverage = "staticBuildColliders+powerNodeSpacing+stationSpacing+beltCrossings+beltBuildingOccupancy+spraycoaterChannels+inserterSlotPoses+inserterBuildColliders"
	}
	// 验证产物可独立于原proof工作区存活，必须计入其持有的全部证明报告；几何本体由VerifiedSnapshot只计一处。
	proofBytes := int64(0)
	for _, proof := range proofs {
		proofBytes += proof.EstimatedBytes()
	}
	validation := FinalValidation{data: &finalValidationData{geometry: g.data, fingerprint: g.Fingerprint(), version: finalizationVersion, logistics: logistics, audit: copyFinalizationPointer(c, audit, c.buildRuleAudit), surfacePlan: plan, density: copyFinalizationPointer(c, density, c.buildDensityReport), warnings: copyFinalizationSlice(c, metrics.warnings, c.copyString), proofs: append([]RuleProof(nil), proofs...), bytes: c.retainedBytes(4096) + proofBytes}}
	if err := checkFinalGeometry(ctx, g); err != nil {
		return VerifiedSnapshot{}, err
	}
	return FreezeValidatedGeometry(ctx, g, validation)
}
