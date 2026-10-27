package quant

// 文件职责：自研两阶段单纯形；使用：小规模循环物品 LP；边界：仅连续非负变量，有迭代和上下文截止，无第三方或 cgo。

import (
	"context"
	"math"
)

// tableau 职责：保存规范型矩阵及基变量；使用：两阶段求解；边界：只在单次请求内存在。
type tableau struct {
	// d 职责：系数表；使用：枢轴；边界：最后两行为目标。
	d [][]float64
	// basic 职责：基变量；使用：还原解；边界：负数为人工变量。
	basic []int
	// nonbasic 职责：非基变量；使用：防循环选列；边界：最后一列人工变量。
	nonbasic []int
	// m 职责：约束数；使用：表尺寸；边界：不超过 200。
	m int
	// n 职责：变量数；使用：表尺寸；边界：不超过 200。
	n int
	// iterations 职责：枢轴次数；使用：退化保护；边界：超过上限报错。
	iterations int
	// ctx 职责：求解截止；使用：取消/超时；边界：不生成后台失控计算。
	ctx context.Context
}

// pivot 职责：执行一次高斯枢轴；使用：两阶段迭代；边界：调用方确保枢轴非零。
func (t *tableau) pivot(row, col int) {
	inv := 1 / t.d[row][col]
	for i := 0; i < t.m+2; i++ {
		if i == row {
			continue
		}
		for j := 0; j < t.n+2; j++ {
			if j != col {
				t.d[i][j] -= t.d[row][j] * t.d[i][col] * inv
			}
		}
	}
	for j := 0; j < t.n+2; j++ {
		if j != col {
			t.d[row][j] *= inv
		}
	}
	for i := 0; i < t.m+2; i++ {
		if i != row {
			t.d[i][col] *= -inv
		}
	}
	t.d[row][col] = inv
	t.basic[row], t.nonbasic[col] = t.nonbasic[col], t.basic[row]
}

// phase 职责：优化一个阶段；使用：人工目标或原目标；边界：无界与超时分别返回。
func (t *tableau) phase(first bool) (bool, error) {
	objective := t.m
	if first {
		objective++
	}
	for {
		if t.ctx.Err() != nil {
			return false, qerror("Q_E_TIMEOUT", "量化求解超过截止时间")
		}
		t.iterations++
		if t.iterations > 100000 {
			return false, qerror("Q_E_TIMEOUT", "LP 退化超过迭代上限")
		}
		col := -1
		for j := 0; j <= t.n; j++ {
			if !first && t.nonbasic[j] == -1 {
				continue
			}
			if col == -1 || t.d[objective][j] < t.d[objective][col]-1e-10 || math.Abs(t.d[objective][j]-t.d[objective][col]) <= 1e-10 && t.nonbasic[j] < t.nonbasic[col] {
				col = j
			}
		}
		if col < 0 || t.d[objective][col] >= -1e-10 {
			return true, nil
		}
		row := -1
		for i := 0; i < t.m; i++ {
			if t.d[i][col] <= 1e-10 {
				continue
			}
			if row == -1 {
				row = i
				continue
			}
			a, b := t.d[i][t.n+1]/t.d[i][col], t.d[row][t.n+1]/t.d[row][col]
			if a < b-1e-10 || math.Abs(a-b) <= 1e-10 && t.basic[i] < t.basic[row] {
				row = i
			}
		}
		if row == -1 {
			return false, nil
		}
		t.pivot(row, col)
	}
}

// minimize 职责：求 min c*x 且 a*x>=b、x>=0；使用：参考物品 LP 模型；边界：输入有限且小于 200 变量，失败不返回近似解。
func minimize(ctx context.Context, a [][]float64, b, c []float64) ([]float64, float64, error) {
	m, n := len(b), len(c)
	if n == 0 {
		return []float64{}, 0, nil
	}
	if n > 200 || m > 200 {
		return nil, 0, qerror("Q_E_CYCLE_TOO_LARGE", "循环/多来源变量超过 200 个")
	}
	t := tableau{m: m, n: n, basic: make([]int, m), nonbasic: make([]int, n+1), d: make([][]float64, m+2), ctx: ctx}
	for i := range t.d {
		t.d[i] = make([]float64, n+2)
	}
	row := 0
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			t.d[i][j] = -a[i][j]
		}
		t.basic[i] = n + i
		t.d[i][n] = -1
		t.d[i][n+1] = -b[i]
		if t.d[i][n+1] < t.d[row][n+1] {
			row = i
		}
	}
	for j := 0; j < n; j++ {
		t.nonbasic[j] = j
		t.d[m][j] = c[j]
	}
	t.nonbasic[n] = -1
	t.d[m+1][n] = 1
	if m > 0 && t.d[row][n+1] < -1e-10 {
		t.pivot(row, n)
		ok, err := t.phase(true)
		if err != nil {
			return nil, 0, err
		}
		if !ok || math.Abs(t.d[m+1][n+1]) > 1e-8 {
			return nil, 0, qerror("Q_E_INFEASIBLE", "选定配方无法满足物品需求")
		}
		for i := 0; i < m; i++ {
			if t.basic[i] == -1 {
				col := -1
				for j := 0; j <= n; j++ {
					if math.Abs(t.d[i][j]) > 1e-10 && (col < 0 || t.nonbasic[j] < t.nonbasic[col]) {
						col = j
					}
				}
				if col >= 0 {
					t.pivot(i, col)
				}
			}
		}
	}
	ok, err := t.phase(false)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, qerror("Q_E_INFEASIBLE", "成本目标无界，请检查配方成本")
	}
	result := make([]float64, n)
	for i := 0; i < m; i++ {
		if t.basic[i] >= 0 && t.basic[i] < n {
			result[t.basic[i]] = math.Max(0, t.d[i][n+1])
		}
	}
	return result, -t.d[m][n+1], nil
}
