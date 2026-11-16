package generator

// 文件职责：从内置或明确指定目录读取并冻结生产方案族。
// 使用：每个新任务及Schema导出；边界：运行中任务持有原计划，不热修改，不搜索任意外部引用，不回退旧算法。

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// schemeFiles 职责：提供随程序发布的完整默认配置；使用：未指定外部目录时；边界：只有YAML属于可执行方案。
//
//go:embed schemes/*.yaml
var schemeFiles embed.FS

// 文件预算职责：约束配置装载总量；使用：内置及外部目录共同边界；边界：不允许用多个文件绕开每文件上限。
const maximumSchemeBytes = 1 << 20
const maximumSchemeTotalBytes = 4 << 20
const maximumSchemeFiles = 64

// schemeSource 职责：保存一次任务装载到的不可变文件字节；使用：编译与指纹；边界：不是执行中持续读取的文件句柄。
type schemeSource struct {
	name string
	data []byte
}

// configurationCache 职责：只缓存最近一份编译计划；使用：相同配置任务复用；边界：内容变更产生新计划，旧运行持有旧指针，缓存不会无限增长。
var configurationCache struct {
	sync.Mutex
	fingerprint string
	value       *runtime
}

// readSchemeSources 职责：以明确上限读取一个目录的所有方案；使用：任务启动；边界：拒绝软链接和非普通文件，不递归扫描用户目录。
func readSchemeSources(source fs.FS) ([]schemeSource, string, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, "", err
	}
	files := []schemeSource{}
	total := 0
	hash := sha256.New()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		if len(files) >= maximumSchemeFiles {
			return nil, "", fmt.Errorf("workflow directory exceeds %d files", maximumSchemeFiles)
		}
		if !entry.Type().IsRegular() {
			return nil, "", fmt.Errorf("workflow source %s must be a regular file", entry.Name())
		}
		file, err := source.Open(entry.Name())
		if err != nil {
			return nil, "", err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maximumSchemeBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, "", readErr
		}
		if closeErr != nil {
			return nil, "", closeErr
		}
		if len(data) > maximumSchemeBytes {
			return nil, "", fmt.Errorf("workflow source %s exceeds 1 MiB", entry.Name())
		}
		total += len(data)
		if total > maximumSchemeTotalBytes {
			return nil, "", fmt.Errorf("workflow directory exceeds 4 MiB")
		}
		files = append(files, schemeSource{entry.Name(), data})
		_, _ = hash.Write([]byte(entry.Name()))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(data)
		_, _ = hash.Write([]byte{0})
	}
	if len(files) == 0 {
		return nil, "", fmt.Errorf("workflow directory contains no YAML schemes")
	}
	return files, hex.EncodeToString(hash.Sum(nil)), nil
}

// loadRuntime 职责：读取明确配置源并缓存其完整编译快照；使用：每次新生成；边界：外部配置错误直接失败，不静默采用内置方案。
func loadRuntime() (*runtime, error) {
	var source fs.FS
	configured := strings.TrimSpace(os.Getenv("DSBOX_BLUEPRINT_WORKFLOWS"))
	if configured == "" {
		source, _ = fs.Sub(schemeFiles, "schemes")
	} else {
		if !filepath.IsAbs(configured) {
			exe, err := os.Executable()
			if err != nil {
				return nil, err
			}
			configured = filepath.Join(filepath.Dir(exe), configured)
		}
		source = os.DirFS(filepath.Clean(configured))
	}
	files, fingerprint, err := readSchemeSources(source)
	if err != nil {
		return nil, err
	}
	configurationCache.Lock()
	defer configurationCache.Unlock()
	if configurationCache.value != nil && configurationCache.fingerprint == fingerprint {
		return configurationCache.value, nil
	}
	r, err := compileSources(files)
	if err != nil {
		return nil, err
	}
	configurationCache.fingerprint, configurationCache.value = fingerprint, r
	return r, nil
}

// compileSources 职责：编译冻结文件与白名单工具契约；使用：装载器和配置隔离回归；边界：不执行工具，要求唯一factory入口。
func compileSources(files []schemeSource) (*runtime, error) {
	a, err := newArtifacts()
	if err != nil {
		return nil, err
	}
	registry, err := newRegistry(a)
	if err != nil {
		return nil, err
	}
	limits := workflow.Limits{Candidates: 4096, StateBytes: 1 << 30, Parallel: 10, Steps: 65536, Nodes: 16384, CallDepth: 32, Depth: 48}
	var main *workflow.Document
	libraries := []*workflow.Document{}
	for _, file := range files {
		doc, err := workflow.Load(file.data, limits)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.name, err)
		}
		if doc.ID == "factory" {
			if main != nil {
				return nil, fmt.Errorf("duplicate factory entry")
			}
			main = doc
		} else {
			libraries = append(libraries, doc)
		}
	}
	if main == nil {
		return nil, fmt.Errorf("factory scheme missing")
	}
	plan, err := workflow.Compile(main, registry, workflow.CompileOptions{Initial: []workflow.Artifact{a.request.Ref()}, Libraries: libraries, Limits: limits})
	if err != nil {
		return nil, err
	}
	return &runtime{keys: a, plan: plan, registry: registry}, nil
}

// ConfigurationSchema 职责：从真实注册工具导出可执行配置Schema；使用：编辑器、CLI和配置回归；边界：不复制一份手写工具参数定义。
func ConfigurationSchema() ([]byte, error) {
	a, err := newArtifacts()
	if err != nil {
		return nil, err
	}
	registry, err := newRegistry(a)
	if err != nil {
		return nil, err
	}
	return workflow.GenerateSchema(registry)
}
