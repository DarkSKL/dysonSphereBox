package workflow

// 文件职责：在预算内严格加载 YAML；使用：读取方案及显式流程库；边界：拒绝别名、标签、重复键、null、多文档和未知 AST 字段。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load 职责：把有限 YAML 转为唯一 AST；使用：Compile 前；边界：不访问文件系统、不执行提供器或工具。
func Load(data []byte, l Limits) (*Document, error) {
	l, e := limits(l)
	if e != nil {
		return nil, e
	}
	if len(data) > l.ConfigBytes {
		return nil, errConfig("configuration byte budget exceeded")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var root yaml.Node
	if e = decoder.Decode(&root); e != nil {
		return nil, fmt.Errorf("YAML: %w", e)
	}
	var extra yaml.Node
	if e = decoder.Decode(&extra); e != io.EOF {
		return nil, errConfig("multiple YAML documents forbidden")
	}
	if len(root.Content) != 1 {
		return nil, errConfig("empty YAML document")
	}
	count := 0
	v, e := yamlValue(root.Content[0], 1, l, &count)
	if e != nil {
		return nil, e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	d.UseNumber()
	var doc Document
	if e = d.Decode(&doc); e != nil {
		return nil, fmt.Errorf("AST: %w", e)
	}
	if doc.SchemaVersion != 1 || doc.ID == "" {
		return nil, errConfig("schemaVersion: 1 and nonempty id required")
	}
	return &doc, nil
}

// yamlValue 职责：转换有限树并保留精确数值；使用：加载；边界：自定义标签和所有锚点拒绝。
func yamlValue(n *yaml.Node, depth int, l Limits, count *int) (any, error) {
	*count++
	if depth > l.Depth || *count > l.Nodes*16 {
		return nil, errConfig("YAML tree budget exceeded")
	}
	if n.Anchor != "" || n.Alias != nil || n.Kind == yaml.AliasNode || n.Style&yaml.TaggedStyle != 0 {
		return nil, fmt.Errorf("YAML line %d: anchors, aliases and explicit tags forbidden", n.Line)
	}
	switch n.Kind {
	case yaml.MappingNode:
		out := map[string]any{}
		for j := 0; j < len(n.Content); j += 2 {
			k := n.Content[j]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				return nil, fmt.Errorf("YAML line %d: string key required", k.Line)
			}
			if _, ok := out[k.Value]; ok {
				return nil, fmt.Errorf("YAML line %d: duplicate key %s", k.Line, k.Value)
			}
			x, e := yamlValue(n.Content[j+1], depth+1, l, count)
			if e != nil {
				return nil, e
			}
			out[k.Value] = x
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, child := range n.Content {
			x, e := yamlValue(child, depth+1, l, count)
			if e != nil {
				return nil, e
			}
			out = append(out, x)
		}
		return out, nil
	case yaml.ScalarNode:
		s := strings.ReplaceAll(n.Value, "_", "")
		switch n.Tag {
		case "!!str":
			return n.Value, nil
		case "!!bool":
			return strconv.ParseBool(s)
		case "!!int":
			z := new(big.Int)
			if _, ok := z.SetString(s, 0); !ok {
				if _, ok = z.SetString(s, 10); !ok {
					return nil, fmt.Errorf("YAML line %d: invalid integer", n.Line)
				}
			}
			return json.Number(z.String()), nil
		case "!!float":
			x, e := strconv.ParseFloat(s, 64)
			if e != nil || math.IsNaN(x) || math.IsInf(x, 0) {
				return nil, fmt.Errorf("YAML line %d: finite number required", n.Line)
			}
			text := strconv.FormatFloat(x, 'g', -1, 64)
			if !strings.ContainsAny(text, ".eE") {
				text += ".0"
			}
			return json.Number(text), nil
		default:
			return nil, fmt.Errorf("YAML line %d: unsupported tag %s (null forbidden)", n.Line, n.Tag)
		}
	}
	return nil, errConfig("unsupported YAML node")
}
