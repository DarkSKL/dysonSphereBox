package network

// 文件职责：从 Windows 系统命令读取网卡累计计数字节。
// 使用：InspectStatus 采样前通过 readHeadlessNetworkCounter 获取当前累计计数。
// 边界：只读取系统统计并返回统一计数 DTO，不计算速率、不维护样本、不控制网络配置。

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"DysonSphereBox/helper/modDependence/headless/wincommand"
)

// readHeadlessNetworkCounter 读取系统网卡累计计数。
// 职责：优先使用 PowerShell 网卡统计，失败时回退 netstat -e。
// 使用：InspectStatus 采样前调用。
// 边界：两个来源都失败时返回错误，不吞掉最终失败原因。
func readHeadlessNetworkCounter() (headlessNetworkCounter, error) {
	if counter, err := readPowerShellNetworkCounter(); err == nil {
		return counter, nil
	}

	return readNetstatNetworkCounter()
}

// readPowerShellNetworkCounter 通过 PowerShell 读取网卡统计。
// 职责：聚合所有网卡 ReceivedBytes 和 SentBytes。
// 使用：readHeadlessNetworkCounter 的首选路径。
// 边界：命令失败、输出为空、JSON 解析失败或计数全零时返回错误。
func readPowerShellNetworkCounter() (headlessNetworkCounter, error) {
	command := `$items = Get-NetAdapterStatistics | Select-Object ReceivedBytes,SentBytes; $items | ConvertTo-Json -Compress`
	output, err := wincommand.Output("powershell", "-NoProfile", "-Command", command)
	if err != nil {
		return headlessNetworkCounter{}, err
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" {
		return headlessNetworkCounter{}, fmt.Errorf("network statistics output is empty")
	}

	var items []headlessPowerShellNetworkCounter
	if strings.HasPrefix(trimmedOutput, "[") {
		if err := json.Unmarshal([]byte(trimmedOutput), &items); err != nil {
			return headlessNetworkCounter{}, err
		}
	} else {
		var item headlessPowerShellNetworkCounter
		if err := json.Unmarshal([]byte(trimmedOutput), &item); err != nil {
			return headlessNetworkCounter{}, err
		}
		items = append(items, item)
	}

	counter := headlessNetworkCounter{capturedAt: time.Now()}
	for _, item := range items {
		counter.receivedBytes += item.ReceivedBytes
		counter.sentBytes += item.SentBytes
	}
	if counter.receivedBytes == 0 && counter.sentBytes == 0 {
		return headlessNetworkCounter{}, fmt.Errorf("network statistics counter is empty")
	}

	return counter, nil
}

// readNetstatNetworkCounter 通过 netstat -e 读取网卡统计。
// 职责：作为 PowerShell 失败后的系统命令兜底。
// 使用：readHeadlessNetworkCounter 回退路径。
// 边界：只解析输出中的前两个数字，格式不符合预期时返回错误。
func readNetstatNetworkCounter() (headlessNetworkCounter, error) {
	output, err := wincommand.Output("netstat", "-e")
	if err != nil {
		return headlessNetworkCounter{}, err
	}

	numbers := make([]uint64, 0, 2)
	for _, field := range strings.Fields(string(output)) {
		normalized := strings.ReplaceAll(field, ",", "")
		value, err := strconv.ParseUint(normalized, 10, 64)
		if err == nil {
			numbers = append(numbers, value)
		}
	}
	if len(numbers) < 2 {
		return headlessNetworkCounter{}, fmt.Errorf("network statistics counter is unavailable")
	}

	return headlessNetworkCounter{
		receivedBytes: numbers[0],
		sentBytes:     numbers[1],
		capturedAt:    time.Now(),
	}, nil
}
