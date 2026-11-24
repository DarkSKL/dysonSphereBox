package industryplan

// 文件职责：核验同轮独立星球并汇总供电缺口。使用：真实施工入口可指定并发度。
// 边界：回执按原核验顺序归并，只有全部星球均通过才可导出。

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"DysonSphereBox/helper/industryplan/search"
)

// siteLayoutCheck 保存一颗星的独立输入、实际回执和耗时，供主线程逐项提交。
type siteLayoutCheck struct {
	input   PlanetPlan
	actual  PlanetActual
	reused  bool
	elapsed time.Duration
	err     error
}

// checkLayoutBatch 职责：按优先顺序派发独立施工合同，工位空出即接下一颗星。
// 使用：核验一个固定分工。边界：并行工位可收齐能分别修正的缺电或矿源缺口；空间失败停止派新工，低算力入口保留逐星反馈。
func checkLayoutBatch(ctx context.Context, jobs []siteLayoutCheck, limit int, validate Validator, p Progress, progress func(Progress)) []siteLayoutCheck {
	var mu sync.Mutex
	next, stopped := 0, false
	work := func() {
		for {
			mu.Lock()
			if next == len(jobs) || stopped || ctx.Err() != nil {
				mu.Unlock()
				return
			}
			job := &jobs[next]
			next++
			id := job.input.PlanetID
			p.PlanetID = id
			progress(p)
			mu.Unlock()

			reported := 0
			if !job.reused {
				timer := search.ActiveTimer(ctx)
				workCtx := context.WithValue(ctx, placedMachinesKey{}, func(count int) {
					mu.Lock()
					defer mu.Unlock()
					reported += count
					p.WorkDone += count
					p.PlanetID = id
					progress(p)
				})
				job.actual, job.err = validate(workCtx, job.input)
				if job.err == nil {
					job.err = verifyPorts(job.input, job.actual)
				}
				job.elapsed = timer()
			}

			mu.Lock()
			if job.err != nil {
				if limit <= 1 || !independentSiteShortfall(job.err) {
					stopped = true
				}
			} else {
				p.Completed++
				p.WorkDone += layoutWork(job.input) - reported
				p.PlanetID = id
				progress(p)
			}
			mu.Unlock()
		}
	}
	var workers sync.WaitGroup
	// 当前调用也承担一个工位，低算力和单星核验仍然串行。
	for i := 1; i < min(limit, len(jobs)); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			work()
		}()
	}
	work()
	workers.Wait()
	return jobs[:next]
}

// siteFailure 保存一颗星的实际施工反馈。
type siteFailure struct {
	planetID int64
	err      error
}

// siteFeedbackBatch 保留排序最前的失败星，以及可在同一候选中分别调整的供电或矿源缺口。
type siteFeedbackBatch struct {
	first  error
	others []siteFailure
}

// Error 沿用首颗失败星的实际施工诊断。
func (b *siteFeedbackBatch) Error() string { return b.first.Error() }

// Unwrap 保持现有供电错误分类可用。
func (b *siteFeedbackBatch) Unwrap() error { return b.first }

// independentSiteShortfall 职责：识别能按星球分别修正的发电负荷与矿源容量；使用：派工与汇总共用同一分类；边界：能源设施放不下需要重新选址。
func independentSiteShortfall(err error) bool {
	var power *PowerLoadError
	var source *SourcePlacementError
	return errors.As(err, &power) && power.AvailableMW == nil || errors.As(err, &source) && source.ItemID > 0
}

// layoutBatchFailure 选择原顺序首个失败星；只有可逐星修正的供电与矿源缺口才能同时处理。
func layoutBatchFailure(jobs []siteLayoutCheck) (int64, error) {
	var first int64
	var failures siteFeedbackBatch
	for _, job := range jobs {
		if job.err == nil {
			continue
		}
		local := independentSiteShortfall(job.err)
		if first == 0 {
			first = job.input.PlanetID
			failures.first = fmt.Errorf("星球 %d：%w", first, job.err)
			if !local {
				return first, failures.first
			}
		} else if local {
			failures.others = append(failures.others, siteFailure{job.input.PlanetID, job.err})
		}
	}
	if len(failures.others) > 0 {
		return first, &failures
	}
	return first, failures.first
}

// surveySourceSites 核验整线当前需用的矿区和接收器，保留同一分工内各星各自测得的容量。
func surveySourceSites(ctx context.Context, sites []PlanetPlan, world World) (int64, error) {
	var jobs []siteLayoutCheck
	for _, site := range sites {
		if len(site.Mines) != 0 || site.Photons != nil || site.CanRelocate && site.Power != nil && len(site.Plan.Productions) > 0 {
			jobs = append(jobs, siteLayoutCheck{input: site})
		}
	}
	check := func(job *siteLayoutCheck) {
		if job.err = search.Checkpoint(ctx); job.err == nil {
			job.err = world.CheckSourceSites(ctx, job.input)
		}
	}
	if world.LayoutWorkers <= 1 || len(jobs) <= 1 {
		for i := range jobs {
			check(&jobs[i])
			if jobs[i].err != nil {
				return layoutBatchFailure(jobs[i : i+1])
			}
		}
		return 0, ctx.Err()
	}
	// 多星预检需要收齐本轮的全部容量。一个矿区结束即取下一份合同，结果仍按分工顺序归并。
	pending := make(chan *siteLayoutCheck, len(jobs))
	for i := range jobs {
		pending <- &jobs[i]
	}
	close(pending)
	var workers sync.WaitGroup
	for range min(world.LayoutWorkers, len(jobs)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range pending {
				check(job)
			}
		}()
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return layoutBatchFailure(jobs)
}
