package p2p

// 文件职责：检测 Windows 防火墙是否会拦截盒子的入站连接（net.firewallRule）。
// 使用：控制器每次控制链路建立（新的 libp2p host）时调用一次 FirewallInboundAllowed，结果写入 net.firewallRule 与前端诊断。
// 边界：只读检测，不添加规则（安装器添加的规则按程序路径同样能匹配）；通过 HNetCfg.FwPolicy2 COM 接口读取数值属性，不解析本地化文本；检测失败或无法确定时返回 true，避免误报 20405。

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"DysonSphereBox/helper/modDependence/headless/wincommand"
)

// firewallTimeout 职责：单次检测的 PowerShell 超时；使用：FirewallInboundAllowed；边界：超时视为检测失败（返回 true）。
const firewallTimeout = 15 * time.Second

// 防火墙 COM 常量（NET_FW_RULE_DIRECTION / NET_FW_ACTION）；配置文件类型 Domain=1、Private=2、Public=4 为位掩码。
const (
	fwDirectionIn = 1
	fwActionBlock = 0
	fwActionAllow = 1
)

// firewallScript 职责：输出当前生效配置文件与当前 exe 相关规则；使用：FirewallInboundAllowed；边界：{{EXE}} 替换为 exe 路径的 UTF-8 Base64，避免引号转义问题。
// 输出每行一条：`P <类型> <是否启用 0/1> <是否阻止全部入站 0/1>`、`R <是否启用> <方向> <动作> <配置文件位掩码>`；生效配置文件全部关闭时不枚举规则。
const firewallScript = `$ErrorActionPreference='Stop'
$exe=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('{{EXE}}'))
$p=New-Object -ComObject HNetCfg.FwPolicy2
$cur=[int]$p.CurrentProfileTypes
if($cur -eq 0){$cur=7}
$on=$false
foreach($t in 1,2,4){if($cur -band $t){$e=[bool]$p.FirewallEnabled($t);if($e){$on=$true};'P {0} {1} {2}' -f $t,[int]$e,[int][bool]$p.BlockAllInboundTraffic($t)}}
if($on){foreach($r in $p.Rules){$app=$r.ApplicationName;if($app -and [Environment]::ExpandEnvironmentVariables($app) -ieq $exe){'R {0} {1} {2} {3}' -f [int][bool]$r.Enabled,[int]$r.Direction,[int]$r.Action,[int]$r.Profiles}}}`

// firewallRun 职责：执行检测脚本并返回输出；使用：FirewallInboundAllowed；边界：测试替换为输出替身，不依赖本机真实防火墙。
var firewallRun = func(ctx context.Context, script string) ([]byte, error) {
	return wincommand.OutputContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
}

// firewallExecutable 职责：返回当前 exe 的绝对路径；使用：FirewallInboundAllowed；边界：测试可替换。
var firewallExecutable = os.Executable

// firewallProfile 是一个当前生效的防火墙配置文件。
type firewallProfile struct {
	kind     int
	enabled  bool
	blockAll bool
}

// firewallAppRule 是一条程序路径等于当前 exe 的规则。
type firewallAppRule struct {
	enabled   bool
	direction int
	action    int
	profiles  int
}

// FirewallInboundAllowed 返回入站连接是否不会被 Windows 防火墙拦截。
// 职责：当前生效的配置文件全部关闭，或每个开启的配置文件都有已启用、入站、允许、程序路径等于当前 exe 的规则（规则名不限）且没有对应的阻止规则时为 true；
// 使用：控制器每次控制链路建立时调用；边界：不缓存；非 Windows、命令失败、超时或输出无法解析时返回 true。
func FirewallInboundAllowed() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	return detectFirewall()
}

// detectFirewall 执行检测脚本并判断；与平台无关，便于用输出替身测试。
func detectFirewall() bool {
	exe, err := firewallExecutable()
	if err != nil {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	script := strings.Replace(firewallScript, "{{EXE}}", base64.StdEncoding.EncodeToString([]byte(exe)), 1)
	ctx, cancel := context.WithTimeout(context.Background(), firewallTimeout)
	defer cancel()
	output, err := firewallRun(ctx, script)
	if err != nil {
		return true
	}
	profiles, rules, err := parseFirewallOutput(string(output))
	if err != nil {
		return true
	}
	return inboundAllowed(profiles, rules)
}

// parseFirewallOutput 解析检测脚本输出；没有任何配置文件行时返回错误（视为检测失败）。
func parseFirewallOutput(output string) ([]firewallProfile, []firewallAppRule, error) {
	profiles := []firewallProfile{}
	rules := []firewallAppRule{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		values := make([]int, 0, len(fields)-1)
		for _, field := range fields[1:] {
			value, err := strconv.Atoi(field)
			if err != nil {
				return nil, nil, err
			}
			values = append(values, value)
		}
		switch {
		case fields[0] == "P" && len(values) == 3:
			profiles = append(profiles, firewallProfile{kind: values[0], enabled: values[1] != 0, blockAll: values[2] != 0})
		case fields[0] == "R" && len(values) == 4:
			rules = append(rules, firewallAppRule{enabled: values[0] != 0, direction: values[1], action: values[2], profiles: values[3]})
		default:
			return nil, nil, errors.New("unexpected firewall output line: " + scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if len(profiles) == 0 {
		return nil, nil, errors.New("no firewall profile in output")
	}
	return profiles, rules, nil
}

// inboundAllowed 按配置文件逐个判断：关闭的配置文件不拦截；开启的配置文件需要允许规则覆盖，且没有阻止规则或“阻止所有入站”。
func inboundAllowed(profiles []firewallProfile, rules []firewallAppRule) bool {
	for _, profile := range profiles {
		if !profile.enabled {
			continue
		}
		if profile.blockAll {
			return false
		}
		allowed := false
		for _, rule := range rules {
			if !rule.enabled || rule.direction != fwDirectionIn || rule.profiles&profile.kind == 0 {
				continue
			}
			if rule.action == fwActionBlock {
				return false
			}
			if rule.action == fwActionAllow {
				allowed = true
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}
