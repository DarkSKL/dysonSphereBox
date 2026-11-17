package build

// 文件职责：对最终记录及规则输入完全一致的候选复用成功终检；使用：所有模式共同出口；边界：不替代依赖builder身份的物料、流量、机器和缓存链校验。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
)

// finalRuleProof 职责：保存不可变规则审计及原校验的标记副作用；使用：单请求缓存；边界：不保存builder指针或失败。
type finalRuleProof struct {
	audit                         RuleAudit
	beltChecked, collidersChecked bool
	coverage                      string
	colliderGameVersion           string
}

// finalRuleFingerprint 职责：纳入完整记录、请求、Area与坐标系以及密铺规则语义；使用：终检前精确状态标识；边界：不忽略顺序、槽位、过滤、经纬锚点和已验证状态。
func finalRuleFingerprint(ctx context.Context, req Request, encoding *surfaceEncoding, records []blueprint.Building) ([32]byte, error) {
	type surfaceState struct {
		Root, Planar, Packed bool
		Areas                []blueprint.Area
		LatitudeOffsets      []int
		LongitudeScales      []float64
		Plan                 *SurfacePlan
	}
	var state *surfaceState
	if encoding != nil {
		state = &surfaceState{encoding.rootCoordinates, encoding.planar, encoding.packed, encoding.areas, encoding.latitudeOffsets, encoding.longitudeScales, encoding.plan}
	}
	// 最终规则只读取已编码记录和上面的坐标信息；byBuilding、重定位源记录及寻锚候选仅用于此前的装配/编码。
	// 私有区域许可也是规则输入；缺省请求仍编码原Request以保持旧key兼容。
	var requestIdentity any = req
	if l := req.SurfaceLayout; l != nil && (l.offsetWindow || l.adjacentDepth != 0 || l.nativeBand != nil || l.polarRoot) {
		requestIdentity = inputRequestIdentity(req)
	}
	hash := sha256.New()
	err := json.NewEncoder(hash).Encode(struct {
		Request         any
		Surface         *surfaceState
		Records         []blueprint.Building
		DenseCollisions bool
	}{requestIdentity, state, records, usesDenseBeltCollisions(ctx)})
	var key [32]byte
	copy(key[:], hash.Sum(nil))
	return key, err
}

// copyRuleAudit 职责：隔离缓存内部切片；使用：写入和返回审计；边界：调用方增补说明不改变后续命中结果。
func copyRuleAudit(a RuleAudit) RuleAudit {
	a.Checks = append([]LogisticsCheck(nil), a.Checks...)
	a.Unverified = append([]string(nil), a.Unverified...)
	return a
}

// reuseFinalPlanetRules 职责：将无副作用规则工作合并，同时重放既有检查标记；使用：最终规则入口；边界：非法JSON数值仍由原规则返回原错误，取消不返回缓存成功。
func reuseFinalPlanetRules(ctx context.Context, req Request, encoding *surfaceEncoding, records []blueprint.Building) (*RuleAudit, error) {
	registry, _ := ctx.Value(candidateRegistryKey{}).(*candidateRegistry)
	if registry == nil {
		return checkFinalPlanetRules(ctx, req, encoding, records)
	}
	key, err := finalRuleFingerprint(ctx, req, encoding, records)
	if err != nil {
		return checkFinalPlanetRules(ctx, req, encoding, records)
	}
	proof, err := registry.rules.Do(ctx, key, func() (finalRuleProof, error) {
		audit, err := checkFinalPlanetRules(ctx, req, encoding, records)
		if err != nil {
			return finalRuleProof{}, err
		}
		proof := finalRuleProof{audit: copyRuleAudit(*audit)}
		if encoding != nil && encoding.plan != nil {
			proof.beltChecked = encoding.plan.BeltConnectionsChecked
			proof.collidersChecked = encoding.plan.StaticCollidersChecked
			proof.coverage = encoding.plan.CollisionCoverage
			proof.colliderGameVersion = encoding.plan.ColliderGameVersion
		}
		return proof, nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, previewContextError(err)
		}
		return nil, err
	}
	if encoding != nil && encoding.plan != nil {
		encoding.plan.BeltConnectionsChecked = proof.beltChecked
		encoding.plan.StaticCollidersChecked = proof.collidersChecked
		encoding.plan.CollisionCoverage = proof.coverage
		encoding.plan.ColliderGameVersion = proof.colliderGameVersion
	}
	audit := copyRuleAudit(proof.audit)
	return &audit, nil
}
