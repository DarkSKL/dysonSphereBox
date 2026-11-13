package r2modman

// 文件职责：收束盒子自身 Profile 操作的中断日志。
// 使用：加载已托管 Profile 或开始下一次写入时调用。
// 边界：只按已验证指纹恢复已移动文件，不覆盖外部清单或其他来源文件，不处理 R2 的安装恢复。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Recover 职责：恢复盒子留下的中断操作；使用：服务加载托管目标前；边界：没有盒子恢复日志时完全只读，有其他盒子进程正在写入时拒绝抢占。
func Recover(path string) error {
	root, err := ValidateProfile(path)
	if err != nil {
		return err
	}
	journalPath, err := safeProfilePath(root, journalRelativePath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	unlock, err := acquireProfileLock(root)
	if err != nil {
		return err
	}
	defer unlock()
	return recoverLocked(root)
}

// 职责：在持锁状态下确认提交或逆序恢复文件；使用：启动恢复及失败回滚；边界：先验证所有移动证据才回滚，文件有外部变化时保留日志，始终保留外部清单修改。
func recoverLocked(root string) error {
	path, err := safeProfilePath(root, journalRelativePath)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var journal changeJournal
	if err := json.Unmarshal(content, &journal); err != nil {
		return fmt.Errorf("盒子 Profile 恢复日志损坏: %w", err)
	}
	if len(journal.Before) == 0 || len(journal.After) == 0 {
		return fmt.Errorf("盒子 Profile 恢复日志缺少清单证据")
	}
	current, err := os.ReadFile(filepath.Join(root, "mods.yml"))
	if err != nil {
		return err
	}
	if bytes.Equal(current, journal.After) {
		return os.Remove(path)
	}
	// 外部改动清单时也仅恢复本次移动的文件，不覆盖外部清单；依赖原始指纹验证避免猜测文件状态。
	reverse := []fileMove{}
	for index := len(journal.Moves) - 1; index >= 0; index-- {
		move := journal.Moves[index]
		if err := validateJournalMove(root, move); err != nil {
			return err
		}
		source, err := safeProfilePath(root, move.From)
		if err != nil {
			return err
		}
		target, err := safeProfilePath(root, move.To)
		if err != nil {
			return err
		}
		_, sourceErr := os.Lstat(source)
		_, targetErr := os.Lstat(target)
		if sourceErr == nil && os.IsNotExist(targetErr) {
			hash, err := fileHash(source)
			if err != nil || hash != move.Hash {
				return fmt.Errorf("源文件已变化，保留恢复日志: %s", move.From)
			}
			continue
		}
		if os.IsNotExist(sourceErr) && targetErr == nil {
			hash, err := fileHash(target)
			if err != nil || hash != move.Hash {
				return fmt.Errorf("目标文件已变化，保留恢复日志: %s", move.To)
			}
			reverse = append(reverse, fileMove{From: move.To, To: move.From, Hash: move.Hash})
			continue
		}
		return fmt.Errorf("移动证据冲突，保留恢复日志: %s", move.From)
	}
	for _, move := range reverse {
		source, err := safeProfilePath(root, move.From)
		if err != nil {
			return err
		}
		target, err := safeProfilePath(root, move.To)
		if err != nil {
			return err
		}
		hash, err := fileHash(source)
		if err != nil || hash != move.Hash {
			return fmt.Errorf("回滚期间文件变化: %s", move.From)
		}
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("回滚目标冲突: %s", move.To)
		}
		if err := os.Rename(source, target); err != nil {
			return fmt.Errorf("回滚失败，保留恢复日志: %w", err)
		}
	}
	return os.Remove(path)
}

// 职责：限制日志中的移动仍属于同一 SUBDIR 包及盒子备份；使用：恢复任何文件之前；边界：无效日志不能借恢复触及配置、框架或其他目录。
func validateJournalMove(root string, move fileMove) error {
	from := filepath.ToSlash(filepath.Clean(filepath.FromSlash(move.From)))
	parts := splitPackagePath(from)
	if len(parts) == 0 || len(move.Hash) != 64 {
		return fmt.Errorf("盒子恢复日志含非法源路径")
	}
	if _, err := safeProfilePath(root, move.From); err != nil {
		return err
	}
	if _, err := safeProfilePath(root, move.To); err != nil {
		return err
	}
	if strings.EqualFold(move.To, move.From+".old") || strings.EqualFold(move.From, move.To+".old") {
		return nil
	}
	to := filepath.ToSlash(filepath.Clean(filepath.FromSlash(move.To)))
	prefix := "_state/dyson-sphere-box/backups/"
	if len(to) > len(prefix) && to[:len(prefix)] == prefix {
		rest := to[len(prefix):]
		for index, char := range rest {
			if char == '/' && rest[index+1:] == from {
				return nil
			}
		}
	}
	return fmt.Errorf("盒子恢复日志含非法目标路径")
}

// 职责：验证移动源符合固定包目录布局；使用：恢复日志验证；边界：禁止空包名、框架名和目录穿越片段。
func splitPackagePath(relative string) []string {
	parts := strings.Split(relative, "/")
	if len(parts) < 4 || parts[0] != "BepInEx" || (parts[1] != "plugins" && parts[1] != "patchers" && parts[1] != "monomod") || !validPackageName(parts[2]) {
		return nil
	}
	if strings.Contains(strings.ToLower(parts[2]), "bepinex") {
		return nil
	}
	return parts
}
