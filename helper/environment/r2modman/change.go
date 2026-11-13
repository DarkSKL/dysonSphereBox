package r2modman

// 文件职责：生成并执行 Profile 启停/移除的可回滚文件计划。
// 使用：环境服务完成所选 Profile 校验后调用。
// 边界：只移动明确归属文件，YAML 提交前检测外部变更；移除保留备份，不删除共享配置。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// 职责：描述单个有指纹证据的移动；使用：执行及中断恢复；边界：路径始终相对 Profile，禁止覆盖。
type fileMove struct {
	// From 职责：记录原始相对文件名；使用：执行和逆序恢复；边界：必须属于支持的包目录。
	From string `json:"from"`
	// To 职责：记录禁用文件名或备份位置；使用：移动目标；边界：不允许覆盖已有文件。
	To string `json:"to"`
	// Hash 职责：记录原始 SHA256；使用：提交与恢复校验；边界：变化时拒绝猜测归属。
	Hash string `json:"hash"`
}

// 职责：保存清单提交前后的字节和文件移动计划；使用：先落盘再修改文件；边界：只恢复盒子自己的计划，不推测第三方修改。
type changeJournal struct {
	// Before 职责：保存原清单字节；使用：并发检测和备份证据；边界：不覆盖第三方变更。
	Before []byte `json:"before"`
	// After 职责：保存提交后的清单字节；使用：判断中断前是否已完成提交；边界：只有完全一致才确认提交。
	After []byte `json:"after"`
	// Moves 职责：保存所有文件移动证据；使用：执行与恢复；边界：必须先完整落盘再执行移动。
	Moves []fileMove `json:"moves"`
}

// 职责：限定盒子恢复日志位置；使用：写操作和安全恢复；边界：不占用 R2 的 installation_state.yml。
const journalRelativePath = "_state/dyson-sphere-box/pending.json"

// SetEnabled 职责：同步文件 .old 后缀与清单 enabled；使用：无需联动确认的单包启停；边界：启用包含已安装依赖，联动禁用需改用 SetEnabledWithPlan，失败回滚。
func SetEnabled(path, name string, enabled bool) error {
	return changeProfile(path, name, enabled, false, nil)
}

// SetEnabledWithPlan 职责：执行已确认范围的联动禁用或启用；使用：服务接收前端确认清单后；边界：实际依赖范围变化时拒绝写入，不扩大用户确认范围。
func SetEnabledWithPlan(path, name string, enabled bool, confirmedNames []string) error {
	return changeProfile(path, name, enabled, false, confirmedNames)
}

// Remove 职责：从 Profile 清单移除包并将文件移动至备份；使用：用户确认移除后；边界：拒绝有依赖者、框架和 STATE 包，保留配置及可恢复备份。
func Remove(path, name string) error { return changeProfile(path, name, false, true, nil) }

// 职责：编排单次有日志保护的本地修改；使用：启停及移除共同入口；边界：外部改变清单或文件时拒绝提交，回滚错误向上返回。
// 逻辑流：1. 锁定 Profile 并恢复上次操作 => 2. 解析依赖和文件计划 => 3. 保存备份及恢复日志 => 4. 移动文件 => 5. 原子提交 YAML => 6. 清除恢复日志。
func changeProfile(path, name string, enabled, remove bool, confirmedNames []string) error {
	root, err := ValidateProfile(path)
	if err != nil {
		return err
	}
	unlock, err := acquireProfileLock(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err := recoverLocked(root); err != nil {
		return err
	}
	doc, err := readManifest(root)
	if err != nil {
		return err
	}
	indices, err := affectedMods(doc.mods, name, enabled, remove)
	if err != nil {
		return err
	}
	if !remove {
		for _, mod := range doc.mods {
			if mod.Name == name && mod.Enabled == enabled {
				return nil
			}
		}
	}
	if !enabled && !remove && (len(indices) > 1 || len(confirmedNames) > 0) {
		confirmed := map[string]bool{}
		for _, name := range confirmedNames {
			confirmed[name] = true
		}
		if len(confirmed) != len(indices) || len(confirmedNames) != len(indices) {
			return fmt.Errorf("请确认完整的联动禁用范围；依赖关系已变化时请先同步")
		}
		for _, index := range indices {
			if !confirmed[doc.mods[index].Name] {
				return fmt.Errorf("联动禁用范围已变化，请先同步后重新确认")
			}
		}
	}
	journal := changeJournal{Before: doc.raw, Moves: []fileMove{}}
	backupRelative := fmt.Sprintf("_state/dyson-sphere-box/backups/%d", time.Now().UnixNano())
	for _, index := range indices {
		files, err := packageFiles(root, doc.mods[index])
		if err != nil {
			return fmt.Errorf("%s: %w", doc.mods[index].DisplayName, err)
		}
		for _, relative := range files {
			target := relative + ".old"
			if enabled {
				target = relative[:len(relative)-len(".old")]
			}
			if remove {
				target = filepath.ToSlash(filepath.Join(backupRelative, filepath.FromSlash(relative)))
			}
			source, err := safeProfilePath(root, relative)
			if err != nil {
				return err
			}
			destination, err := safeProfilePath(root, target)
			if err != nil {
				return err
			}
			if _, err := os.Lstat(destination); !os.IsNotExist(err) {
				return fmt.Errorf("目标已存在或不可访问，不覆盖: %s", target)
			}
			hash, err := fileHash(source)
			if err != nil {
				return err
			}
			journal.Moves = append(journal.Moves, fileMove{From: relative, To: target, Hash: hash})
		}
		node := doc.node.Content[0].Content[index]
		if !remove {
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == "enabled" {
					node.Content[i+1].SetString(fmt.Sprint(enabled))
					node.Content[i+1].Tag = "!!bool"
				}
			}
		}
	}
	if remove {
		index := indices[0]
		sequence := doc.node.Content[0]
		sequence.Content = append(sequence.Content[:index], sequence.Content[index+1:]...)
	}
	journal.After, err = yaml.Marshal(&doc.node)
	if err != nil {
		return err
	}
	backup, err := safeProfilePath(root, filepath.ToSlash(filepath.Join(backupRelative, "mods.yml")))
	if err != nil {
		return err
	}
	if err := writeAtomic(backup, doc.raw, 0600); err != nil {
		return err
	}
	journalPath, err := safeProfilePath(root, journalRelativePath)
	if err != nil {
		return err
	}
	content, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	if err := writeAtomic(journalPath, content, 0600); err != nil {
		return err
	}
	if err := applyMoves(root, journal); err != nil {
		return errors.Join(err, recoverLocked(root))
	}
	return recoverLocked(root)
}

// 职责：复核快照后移动文件并提交清单；使用：恢复日志落盘后调用；边界：不覆盖目标，清单外部变化时交回上层回滚。
func applyMoves(root string, journal changeJournal) error {
	manifest := filepath.Join(root, "mods.yml")
	current, err := os.ReadFile(manifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, journal.Before) {
		return fmt.Errorf("mods.yml 已被 r2modman 修改，请刷新后重试")
	}
	for _, move := range journal.Moves {
		source, err := safeProfilePath(root, move.From)
		if err != nil {
			return err
		}
		target, err := safeProfilePath(root, move.To)
		if err != nil {
			return err
		}
		hash, err := fileHash(source)
		if err != nil {
			return err
		}
		if hash != move.Hash {
			return fmt.Errorf("模组文件已被外部修改: %s", move.From)
		}
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("目标冲突: %s", move.To)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.Rename(source, target); err != nil {
			return fmt.Errorf("移动模组文件失败: %w", err)
		}
	}
	current, err = os.ReadFile(manifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, journal.Before) {
		return fmt.Errorf("mods.yml 在操作期间发生变化，未覆盖外部修改")
	}
	info, err := os.Stat(manifest)
	if err != nil {
		return err
	}
	return writeAtomic(manifest, journal.After, info.Mode().Perm())
}

// 职责：计算文件指纹；使用：计划、提交与恢复复核；边界：读取错误阻止继续，不修改文件。
func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// 职责：先写同目录临时文件再替换完整文件；使用：清单、备份与恢复日志提交；边界：写入或关闭失败不替换旧文件，临时文件始终清理。
func writeAtomic(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".dsbox-write-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
