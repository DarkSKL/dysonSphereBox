package generator

// 文件职责：从主工作流冻结任务预算，在创建上下文和调度候选之前应用。

import (
	"fmt"
	"time"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// executionPolicy 控制单任务的截止时间与自动并发；手动并发及父上下文仍然优先。
type executionPolicy struct {
	// LargePlanMachines 是启用大规模任务并发值的首个机器数。
	LargePlanMachines int `json:"largePlanMachines" min:"1"`
	// LargePlanParallel 是自动模式达到阈值时使用的计算槽数。
	LargePlanParallel int `json:"largePlanParallel" min:"1" max:"10"`
	// BaseSeconds 是第一档任务的总时限。
	BaseSeconds int `json:"baseSeconds" min:"1" max:"86400"`
	// MachinesPerBand 是首档及每个后续档位的机器数。
	MachinesPerBand int `json:"machinesPerBand" min:"1"`
	// ExtraSecondsPerBand 是每越过一个档位增加的预算，零表示固定时限。
	ExtraSecondsPerBand int `json:"extraSecondsPerBand" min:"0" max:"86400"`
	// MaxSeconds 限制所有档位的任务总时限。
	MaxSeconds int `json:"maxSeconds" min:"1" max:"86400"`
}

// loadExecutionPolicy 在方案编译时绑定显式字段，并为旧方案补齐历史默认值。
func loadExecutionPolicy(main *workflow.Document) (executionPolicy, error) {
	defaults := executionPolicy{
		LargePlanMachines: 256, LargePlanParallel: 1,
		BaseSeconds: 240, MachinesPerBand: 2048, ExtraSecondsPerBand: 90, MaxSeconds: 600,
	}
	policy, err := workflow.BindDocumentParameter(main, "execution", defaults)
	if err == nil && policy.MaxSeconds < policy.BaseSeconds {
		return policy, fmt.Errorf("factory.parameters.execution.maxSeconds 不能小于 baseSeconds")
	}
	return policy, err
}

// workBudget 先用剩余预算限定可相乘的档数，极大机器数也不会溢出。
func (p executionPolicy) workBudget(machines int) time.Duration {
	seconds := p.BaseSeconds
	if machines > p.MachinesPerBand && p.ExtraSecondsPerBand > 0 {
		bands := (machines - 1) / p.MachinesPerBand
		if bands > (p.MaxSeconds-seconds)/p.ExtraSecondsPerBand {
			seconds = p.MaxSeconds
		} else {
			seconds += bands * p.ExtraSecondsPerBand
		}
	}
	return time.Duration(seconds) * time.Second
}

// parallel 仅在请求选择自动并发且达到大厂规模时覆盖引擎自动值。
func (p executionPolicy) parallel(requested, machines int) int {
	if requested == 0 && machines >= p.LargePlanMachines {
		return p.LargePlanParallel
	}
	return requested
}
