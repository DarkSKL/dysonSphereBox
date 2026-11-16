package generator

// 文件职责：登记模式参数层的独立诊断产物。
// 使用：主产物目录初始化；边界：不同层分键，不推进工厂几何版本、不共享可变参数容器。

import "DysonSphereBox/helper/blueprint/build/workflow"

// modeArtifacts 职责：描述四个固定覆盖层的读写边界；使用：配置工具和只读属性；边界：不包含几何、验证或候选状态。
type modeArtifacts struct {
	// common 职责：隔离common层参数；使用：按顺序只读合并；边界：仅诊断产物，不授予几何证明。
	common workflow.ArtifactKey[modeValues]
	// mode 职责：隔离mode层参数；使用：按顺序只读合并；边界：仅诊断产物，不授予几何证明。
	mode workflow.ArtifactKey[modeValues]
	// submode 职责：隔离submode层参数；使用：按顺序只读合并；边界：仅诊断产物，不授予几何证明。
	submode workflow.ArtifactKey[modeValues]
	// effective 职责：隔离effective层参数；使用：按顺序只读合并；边界：仅诊断产物，不授予几何证明。
	effective workflow.ArtifactKey[modeValues]
}

// newModeArtifacts 职责：注册真实类型、克隆和容量器；使用：newArtifacts；边界：失败不返回不完整目录。
func newModeArtifacts(all *[]workflow.Artifact) (modeArtifacts, error) {
	a := modeArtifacts{}
	var err error
	if a.common, err = addArtifact(all, "configuration.common", cloneModeValues, valueBytes[modeValues], true); err != nil {
		return a, err
	}
	if a.mode, err = addArtifact(all, "configuration.mode", cloneModeValues, valueBytes[modeValues], true); err != nil {
		return a, err
	}
	if a.submode, err = addArtifact(all, "configuration.submode", cloneModeValues, valueBytes[modeValues], true); err != nil {
		return a, err
	}
	if a.effective, err = addArtifact(all, "configuration.effective", cloneModeValues, valueBytes[modeValues], true); err != nil {
		return a, err
	}
	return a, nil
}
