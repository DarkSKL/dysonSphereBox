package workflow

// 文件职责：冻结 AST 时保留空动作和显式空备用列表；使用：编译复制与指纹；边界：不把缺失字段变成 null。

import "encoding/json"

// MarshalJSON 职责：保留空 sequence 的动作身份；使用：Node 冻结；边界：其他省略字段沿用标准编码。
func (n Node) MarshalJSON() ([]byte, error) {
	type plain Node
	b, e := json.Marshal(plain(n))
	if e != nil {
		return nil, e
	}
	if n.Sequence == nil {
		return b, nil
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(b, &fields); e != nil {
		return nil, e
	}
	fields["sequence"], e = json.Marshal(n.Sequence)
	if e != nil {
		return nil, e
	}
	return json.Marshal(fields)
}

// MarshalJSON 职责：保留显式空 fallback 的分支语义；使用：候选冻结；边界：nil 仍表示没有备用。
func (c Candidates) MarshalJSON() ([]byte, error) {
	type plain Candidates
	b, e := json.Marshal(plain(c))
	if e != nil {
		return nil, e
	}
	if c.Fallback == nil {
		return b, nil
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(b, &fields); e != nil {
		return nil, e
	}
	fields["fallback"], e = json.Marshal(c.Fallback)
	if e != nil {
		return nil, e
	}
	return json.Marshal(fields)
}

// MarshalJSON 职责：把动态空组的 nil Items 规范化为空列表；使用：显式组 DTO 运输；边界：组内参数值的 null 仍由严格运输层拒绝。
func (g CandidateGroup) MarshalJSON() ([]byte, error) {
	type plain CandidateGroup
	if g.Items == nil {
		g.Items = []map[string]any{}
	}
	return json.Marshal(plain(g))
}
