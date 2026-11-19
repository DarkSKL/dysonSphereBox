package sphereblueprint

// 文件职责：校验 DYBP 信封并选择游戏格式分支；使用：预览入口；边界：无磁盘访问，仅解压一个限量 GZip 成员。

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"DysonSphereBox/helper/blueprint"
)

// 职责：限制单次预览的内存；使用：在摘要或解压分配前检查；边界：包含空白的输入也计入文本限额。
const (
	// maxTextBytes 限制传入文本为 8 MiB。
	maxTextBytes = 8 << 20
	// maxPayloadBytes 限制解压后的结构及涂装为 16 MiB。
	maxPayloadBytes = 16 << 20
)

// Parse 职责：从原文还原球壳和轨道；使用：Wails 绑定和离线校验；边界：遇到未知版本或任何损坏均不返回半成品。
func Parse(code string) (*Blueprint, error) {
	if len(code) > maxTextBytes {
		return nil, errors.New("球壳蓝图文本超过 8 MiB")
	}
	code = strings.TrimSpace(code)
	header, body, ok := strings.Cut(code, `"`)
	if !ok || !strings.HasPrefix(header, "DYBP:") || len(header) > 4096 {
		return nil, errors.New("需要以 DYBP: 开头的戴森球蓝图")
	}
	fields := strings.Split(header[5:], ",")
	if len(fields) != 5 || fields[0] != "0" {
		return nil, errors.New("不支持的 DYBP 头部格式")
	}
	ticks, timeErr := strconv.ParseUint(fields[1], 10, 64)
	kind, typeErr := strconv.Atoi(fields[3])
	latitude, latErr := strconv.Atoi(fields[4])
	if timeErr != nil || ticks > 3155378975999999999 || typeErr != nil || kind < 1 || kind > 4 || latErr != nil || latitude < 0 || latitude > 90 {
		return nil, errors.New("球壳蓝图头部的时间、类型或纬度无效")
	}
	modern, err := modernVersion(fields[2])
	if err != nil {
		return nil, err
	}
	encoded, signature, ok := strings.Cut(body, `"`)
	if !ok || len(signature) != 32 || blueprint.MD5F([]byte(code[:len(header)+1+len(encoded)])) != signature {
		return nil, errors.New("球壳蓝图 MD5F 签名不匹配")
	}
	payload, err := decompress(encoded)
	if err != nil {
		return nil, err
	}
	r := reader{data: payload, modern: modern}
	result := Blueprint{Type: kind, GameVersion: fields[2], Layers: []Layer{}, Orbits: []Orbit{}}
	if modern {
		r.version("载荷", 0)
	}
	if kind == 3 || kind == 4 {
		result.Orbits = r.swarm()
	}
	switch kind {
	case 1:
		layer := Layer{ID: 1, Radius: 1, Rotation: []float64{0, 0, 0, 1}, Visible: true}
		r.layer(&layer)
		if len(layer.Nodes) > 0 {
			layer.Radius = positionRadius(layer.Nodes[0].Pos)
		}
		result.Layers = append(result.Layers, layer)
	case 2, 4:
		result.Layers = r.layers()
	}
	if r.err != nil {
		return nil, r.err
	}
	if len(r.data) != 0 {
		return nil, fmt.Errorf("球壳蓝图载荷末尾多出 %d 字节", len(r.data))
	}
	return &result, nil
}

// modernVersion 职责：对比四段游戏版本；使用：选择 Import 或 ImportOld；边界：超过核实的本机版本时停止解析。
func modernVersion(value string) (bool, error) {
	parts := strings.Split(value, ".")
	var version [4]uint32
	if len(parts) != len(version) {
		return false, errors.New("球壳蓝图游戏版本格式错误")
	}
	for i, part := range parts {
		number, err := strconv.ParseUint(part, 10, 31)
		if err != nil {
			return false, errors.New("球壳蓝图游戏版本格式错误")
		}
		version[i] = uint32(number)
	}
	if slices.Compare(version[:], []uint32{0, 10, 35, 29151}) > 0 {
		return false, fmt.Errorf("尚不支持游戏版本 %s 的球壳蓝图", value)
	}
	return slices.Compare(version[:], []uint32{0, 9, 24, 11286}) > 0, nil
}

// decompress 职责：解开 Base64 和 GZip；使用：签名通过后读取；边界：验证尾部校验和、解压限额和压缩数据结束位置。
func decompress(encoded string) ([]byte, error) {
	compressed, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, errors.New("球壳蓝图 Base64 编码损坏")
	}
	source := bytes.NewReader(compressed)
	stream, err := gzip.NewReader(source)
	if err != nil {
		return nil, errors.New("球壳蓝图 GZip 头部损坏")
	}
	defer stream.Close()
	stream.Multistream(false)
	raw, err := io.ReadAll(io.LimitReader(stream, maxPayloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("球壳蓝图解压失败：%w", err)
	}
	if len(raw) > maxPayloadBytes {
		return nil, errors.New("球壳蓝图解压后超过 16 MiB")
	}
	if source.Len() != 0 {
		return nil, errors.New("球壳蓝图压缩流末尾存在多余数据")
	}
	return raw, nil
}
