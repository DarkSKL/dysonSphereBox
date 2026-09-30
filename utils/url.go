package utils

// 文件职责：提供 HTTP 服务基础地址的校验与归一化工具。
// 使用：配置服务保存用户填写的服务器地址前调用，确保写入配置的是可直接拼接接口路径的基础地址。
// 边界：只做文本解析和格式约束，不发起网络请求、不判断服务是否可用，也不依赖业务包。

import (
	"errors"
	"net/url"
	"strings"
)

// NormalizeHTTPBaseURL 将用户输入的服务地址归一化为 HTTP 基础地址。
// 职责：校验地址协议、主机名和附加成分，并输出统一格式（小写协议与主机、无末尾斜杠）。
// 使用：service 层保存服务器连接地址等配置前调用；返回值可直接与 `/base/login` 等接口路径拼接。
// 边界：只接受 http/https；空地址、缺少主机名、包含账号信息、查询参数或锚点时返回可展示给用户的中文错误。
// 逻辑流：1. 去除首尾空白并解析 URL => 2. 校验协议与主机 => 3. 拒绝账号信息、查询参数和锚点 => 4. 去除路径末尾斜杠并返回。
func NormalizeHTTPBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("地址不能为空")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("地址格式无法识别")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("地址必须以 http:// 或 https:// 开头")
	}
	if parsed.Hostname() == "" {
		return "", errors.New("地址缺少主机名")
	}
	if parsed.User != nil {
		return "", errors.New("地址不能包含账号或密码")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", errors.New("地址不能包含查询参数或锚点")
	}

	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
	return parsed.String(), nil
}
