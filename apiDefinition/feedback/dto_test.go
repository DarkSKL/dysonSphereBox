package feedback

// 文件职责：验证服务端星位 JSON 到桌面 Wails JSON 的兼容性。
// 使用：更改响应 DTO 或地图范围时运行本包测试。
// 边界：只校验传输字段，星位分配由服务端负责。

import (
	"encoding/json"
	"maps"
	"testing"
)

// TestOptionalPreviewFields 职责：核对新旧 HTTP 回执在 Wails JSON 中的展示字段。
// 使用：修改星图、单星或作者列表 DTO 时运行。边界：缺失与空值均保留给页面回退。
func TestOptionalPreviewFields(t *testing.T) {
	for _, tc := range []struct {
		name, field, value string
		newDTO             func() any
	}{
		{"map", "previewVersion", "18446744073709551615", func() any { return &StarPoint{} }},
		{"preview", "displayName", "匿名", func() any { return &PreviewResult{} }},
		{"mine", "displayName", "玩家昵称", func() any { return &MyStar{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, input := range []string{`{}`, `{"` + tc.field + `":""}`, `{"` + tc.field + `":null}`} {
				fields := roundTripFields(t, input, tc.newDTO())
				if value, exists := fields[tc.field]; exists {
					t.Fatalf("%s should be absent for %s, got %v", tc.field, input, value)
				}
			}
			input := `{"` + tc.field + `":"` + tc.value + `"}`
			if fields := roundTripFields(t, input, tc.newDTO()); fields[tc.field] != tc.value {
				t.Fatalf("%s = %v, want %s", tc.field, fields[tc.field], tc.value)
			}
		})
	}
}

// TestStarPositionJSON 验证四类响应都传递有符号高度，并为旧响应补出盘面高度 0。
func TestStarPositionJSON(t *testing.T) {
	for _, tt := range []struct {
		name, serverJSON string
		wantZ            float64
	}{
		{"legacy", `{"x":-24,"y":53}`, 0},
		{"plane", `{"x":-24,"y":53,"z":0}`, 0},
		{"above", `{"x":-24,"y":53,"z":10000}`, 10000},
		{"below", `{"x":-24,"y":53,"z":-10000}`, -10000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for name, dto := range map[string]any{
				"MapPoint":  &StarPoint{},
				"Detail":    &StarDetail{},
				"MineItem":  &MyStar{},
				"WriteStar": &WriteStarResult{},
			} {
				t.Run(name, func(t *testing.T) {
					got := roundTripFields(t, tt.serverJSON, dto)
					if got["x"] != -24.0 || got["y"] != 53.0 || got["z"] != tt.wantZ {
						t.Fatalf("x/y/z = %v/%v/%v, want -24/53/%v", got["x"], got["y"], got["z"], tt.wantZ)
					}
				})
			}
		})
	}
}

// TestMapBoundsJSON 验证请求和快照边界都只传递四个盘面范围值，保留小数精度。
func TestMapBoundsJSON(t *testing.T) {
	const bounds = `{"minX":-8.5,"minY":-4,"maxX":16.25,"maxY":9,"minZ":-2,"maxZ":2}`
	want := map[string]any{"minX": -8.5, "minY": -4.0, "maxX": 16.25, "maxY": 9.0}
	for name, dto := range map[string]any{"MapRequest": &MapRequest{}, "Bounds": &Bounds{}} {
		t.Run(name, func(t *testing.T) {
			if got := roundTripFields(t, bounds, dto); !maps.Equal(got, want) {
				t.Fatalf("bounds = %v, want %v", got, want)
			}
		})
	}
}

// roundTripFields 用指定 Go DTO 解码输入，再检查桌面桥接编码出的 JSON 字段及其类型。
func roundTripFields(t *testing.T, input string, dto any) map[string]any {
	t.Helper()
	if err := json.Unmarshal([]byte(input), dto); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}
