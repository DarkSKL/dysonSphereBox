package main

// 文件职责：在独立进程中重放真实生成请求并输出资源测量报告。
// 使用：先编译本命令，再以 -request 指定人工验收包中的 request.json。
// 边界：不启动游戏，不导出半成品，不改变生成预算或几何规则。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"DysonSphereBox/helper/blueprint/build"
)

// options 职责：保存单次独立测量参数；使用：命令入口；边界：一个进程只运行一次请求。
type options struct {
	requestPath string
	reportPath  string
	cancelStage string
	cancelDelay time.Duration
	timeout     time.Duration
}

// readRequest 职责：读取原始验收请求并保留内容哈希；使用：固定输入重放；边界：拒绝未知字段、尾随文档和超过32MiB的请求。
func readRequest(path string) (build.Request, string, error) {
	var request build.Request
	file, err := os.Open(path)
	if err != nil {
		return request, "", err
	}
	defer file.Close()
	const limit = 32 << 20
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return request, "", err
	}
	if len(content) > limit {
		return request, "", fmt.Errorf("请求超过32MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, "", err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return request, "", fmt.Errorf("请求后存在额外JSON或无效内容")
	}
	if request.Mode < 1 || request.Mode > 4 {
		return request, "", fmt.Errorf("测量仅接受公开模式1到4")
	}
	if request.SurfaceLayout != nil && request.SurfaceLayout.Mode != "planar" && request.SurfaceLayout.Mode != "autoTropic" {
		return request, "", fmt.Errorf("测量仅接受旧平面兼容或自动球面")
	}
	return request, fmt.Sprintf("%x", sha256.Sum256(content)), nil
}

// writeReport 职责：写出一份可复核报告；使用：命令收尾；边界：拒绝覆盖已有文件，不把报告写入请求文件。
func writeReport(path string, report resourceReport) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(encoded, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// run 职责：验证命令参数、重放请求并提交报告；使用：独立性能验收；边界：取消未命中或生成失败以非零退出码报告。
func run(arguments []string, stdout, stderr io.Writer) int {
	var option options
	flags := flag.NewFlagSet("factory-profile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&option.requestPath, "request", "", "验收request.json路径")
	flags.StringVar(&option.reportPath, "report", "", "首次创建的报告路径")
	flags.StringVar(&option.cancelStage, "cancel-stage", "", "首次进入指定阶段后异步取消；空值为完整生成")
	flags.DurationVar(&option.cancelDelay, "cancel-delay", 10*time.Millisecond, "进入指定阶段至发出取消的延迟")
	flags.DurationVar(&option.timeout, "timeout", 5*time.Minute, "外层硬截止；引擎原有四分钟期限不变")
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if option.requestPath == "" || option.reportPath == "" || flags.NArg() != 0 || option.cancelDelay < 0 || option.timeout <= 0 {
		fmt.Fprintln(stderr, "必须提供-request与-report；延迟不可为负，截止必须为正，不接受额外参数")
		return 2
	}
	if _, err := os.Lstat(option.reportPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stderr, "报告路径已存在或不可访问，拒绝覆盖")
		return 2
	}
	request, digest, err := readRequest(option.requestPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report := measureGeneration(request, option, digest, build.Generate)
	if err := writeReport(option.reportPath, report); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "outcome=%s duration=%.3fms buildings=%d report=%s\n", report.Outcome, report.GenerationDurationMs, report.BuildingCount, option.reportPath)
	if report.VerificationError != "" {
		fmt.Fprintln(stderr, report.VerificationError)
		return 1
	}
	return 0
}

// main 职责：启动单次测量；使用：编译后的独立进程；边界：go run的编译进程不在本进程内存统计范围。
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// generateFunc 职责：约束真实生成入口签名；使用：测量与取消边界测试；边界：生产命令始终传入build.Generate。
type generateFunc func(context.Context, build.Request) (build.Result, error)
