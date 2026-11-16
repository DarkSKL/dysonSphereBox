package workflow

// 文件职责：检查整个冻结方案库的声明图；使用：Compile；边界：未被入口调用的递归、未知工具和过量声明也不能隐藏。

import (
	"fmt"
	"strings"
)

// literalParameters 职责：限定命名参数为字面运输值；使用：所有方案库；边界：禁止参数表自身构造循环引用。
func literalParameters(v any) error {
	if _, _, is, e := reference(v); e != nil {
		return e
	} else if is {
		return errConfig("parameters must be literals")
	}
	switch x := v.(type) {
	case map[string]any:
		for _, child := range x {
			if e := literalParameters(child); e != nil {
				return e
			}
		}
	case []any:
		for _, child := range x {
			if e := literalParameters(child); e != nil {
				return e
			}
		}
	}
	return nil
}

// resolveSubflow 职责：解析显式库及私有引用；使用：声明图；边界：不查找任何外部文件。
func (c *compiler) resolveSubflow(owner *Document, call string) (string, error) {
	id := call
	if parts := strings.SplitN(call, "/", 2); len(parts) == 2 {
		owner = c.docs[parts[0]]
		id = parts[1]
	}
	if owner == nil {
		return "", errConfig("unknown flow library " + call)
	}
	if _, ok := owner.Subflows[id]; !ok {
		return "", errConfig("unknown subflow " + call)
	}
	return owner.ID + "/" + id, nil
}

// validateDeclarations 职责：在有限扫描内检查所有调用图；使用：入口展开前；边界：图复用不等于允许递归，展开预算仍单独核对。
func (c *compiler) validateDeclarations() error {
	graph := map[string][]string{}
	declared := 0
	var scan func([]Node, *Document, string, int) error
	scan = func(nodes []Node, owner *Document, key string, depth int) error {
		if depth > c.plan.limits.Depth {
			return errConfig("declaration depth budget exceeded")
		}
		ids := map[string]bool{}
		for _, n := range nodes {
			declared++
			if declared > c.plan.limits.Nodes {
				return errConfig("declared node budget exceeded")
			}
			if n.ID == "" || ids[n.ID] {
				return errConfig("empty or duplicate declared node id")
			}
			ids[n.ID] = true
			actions := 0
			for _, yes := range []bool{n.Tool != "", n.Sequence != nil, n.Switch != nil, n.Call != "", n.Regions != nil, n.Candidates != nil, n.Fail != ""} {
				if yes {
					actions++
				}
			}
			if actions != 1 {
				return errConfig("declared node must have one action")
			}
			if n.Tool != "" && c.plan.registry.tools[n.Tool] == nil {
				return errConfig("unknown declared tool " + n.Tool)
			}
			if n.Call != "" {
				target, e := c.resolveSubflow(owner, n.Call)
				if e != nil {
					return e
				}
				graph[key] = append(graph[key], target)
			}
			children := [][]Node{n.Sequence}
			if n.Switch != nil {
				if n.Switch.Otherwise == nil {
					return errConfig("switch requires otherwise")
				}
				for _, branch := range n.Switch.Cases {
					children = append(children, branch)
				}
				children = append(children, n.Switch.Otherwise)
			}
			if n.Regions != nil {
				children = append(children, n.Regions.Flow)
			}
			if n.Candidates != nil {
				children = append(children, n.Candidates.Flow, n.Candidates.Fallback)
			}
			for _, child := range children {
				if child != nil {
					if e := scan(child, owner, key, depth+1); e != nil {
						return e
					}
				}
			}
		}
		return nil
	}
	for _, owner := range c.docs {
		if strings.Contains(owner.ID, "/") {
			return errConfig("document id cannot contain slash")
		}
		if e := literalParameters(owner.Parameters); e != nil {
			return e
		}
		if e := scan(owner.Flow, owner, owner.ID+"/#entry", 0); e != nil {
			return e
		}
		for id, sub := range owner.Subflows {
			if id == "" || strings.Contains(id, "/") {
				return errConfig("invalid private subflow id")
			}
			key := owner.ID + "/" + id
			if _, ok := graph[key]; !ok {
				graph[key] = nil
			}
			if e := scan(sub.Flow, owner, key, 0); e != nil {
				return e
			}
		}
	}
	colors := map[string]int{}
	longest := map[string]int{}
	var visit func(string) (int, error)
	visit = func(key string) (int, error) {
		if colors[key] == 1 {
			return 0, errConfig("recursive subflow " + key)
		}
		if colors[key] == 2 {
			return longest[key], nil
		}
		colors[key] = 1
		depth := 0
		for _, next := range graph[key] {
			child, e := visit(next)
			if e != nil {
				return 0, e
			}
			if child+1 > depth {
				depth = child + 1
			}
		}
		if depth > c.plan.limits.CallDepth {
			return 0, fmt.Errorf("%s: subflow depth budget exceeded", key)
		}
		colors[key] = 2
		longest[key] = depth
		return depth, nil
	}
	for key := range graph {
		if _, e := visit(key); e != nil {
			return e
		}
	}
	return nil
}
