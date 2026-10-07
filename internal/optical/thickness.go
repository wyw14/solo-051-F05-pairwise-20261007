package optical

import (
	"math"
	"sync"

	"github.com/wyw14/cry-161/internal/model"
)

// SampleStatus 是窗口对样本的判定结果
type SampleStatus string

const (
	SampleValid      SampleStatus = "valid"      // 有效
	SampleOutlier    SampleStatus = "outlier"    // 离群
	SampleMissing    SampleStatus = "missing"    // 缺失
	SampleRecovering SampleStatus = "recovering" // 恢复中
)

// LayerGuard 是按配方层配置的离群监控策略
type LayerGuard struct {
	TargetNm             float64 `json:"target_nm"`
	ThresholdNm          float64 `json:"threshold_nm"`
	ConsecutiveAnomalies int     `json:"consecutive_anomalies"`
	RecoveryWindow       int     `json:"recovery_window"`
}

// GuardFromLayer 从配方层提取离群监控策略
func GuardFromLayer(layer model.Layer) LayerGuard {
	return LayerGuard{TargetNm: layer.TargetNanometers, ThresholdNm: layer.OutlierThresholdNm, ConsecutiveAnomalies: layer.ConsecutiveAnomalies, RecoveryWindow: layer.RecoveryWindow}
}

// Enabled 报告是否启用离群判定；阈值非正表示不启用
func (g LayerGuard) Enabled() bool { return g.ThresholdNm > 0 }

// AssessedSample 保留原始样本及其判定状态，离群样本的原始证据不丢失
type AssessedSample struct {
	Sample model.ThicknessSample `json:"sample"`
	Layer  int                   `json:"layer"`
	Status SampleStatus          `json:"status"`
}

type layerState struct {
	guard             LayerGuard
	consecutive       int
	recoveryRemaining int
}

type ThicknessWindow struct {
	mu     sync.RWMutex
	values []AssessedSample
	limit  int
	layers map[int]*layerState
}

func NewThicknessWindow(limit int) *ThicknessWindow {
	if limit < 1 {
		limit = 1
	}
	return &ThicknessWindow{limit: limit, values: make([]AssessedSample, 0, limit), layers: map[int]*layerState{}}
}

// ConfigureLayer 按配方层设置离群策略；配置未变化时保留该层的连续计数与恢复进度
func (w *ThicknessWindow) ConfigureLayer(layer int, guard LayerGuard) {
	w.mu.Lock()
	defer w.mu.Unlock()
	state, ok := w.layers[layer]
	if ok && state.guard == guard {
		return
	}
	w.layers[layer] = &layerState{guard: guard}
}

// Add 判定并保存样本，返回带状态的条目；离群样本同样保留在窗口中
func (w *ThicknessWindow) Add(sample model.ThicknessSample, layer int) AssessedSample {
	w.mu.Lock()
	entry := AssessedSample{Sample: sample, Layer: layer, Status: w.assessLocked(sample, layer)}
	w.values = append(w.values, entry)
	if len(w.values) > w.limit {
		w.values = w.values[len(w.values)-w.limit:]
	}
	w.mu.Unlock()
	return entry
}

func (w *ThicknessWindow) assessLocked(sample model.ThicknessSample, layer int) SampleStatus {
	if !sample.Valid {
		return SampleMissing
	}
	state, ok := w.layers[layer]
	if !ok || !state.guard.Enabled() {
		return SampleValid
	}
	trigger := state.guard.ConsecutiveAnomalies
	if trigger < 1 {
		trigger = 1
	}
	if math.Abs(sample.Nanometers-state.guard.TargetNm) > state.guard.ThresholdNm {
		state.consecutive++
		if state.guard.RecoveryWindow > 0 && (state.recoveryRemaining > 0 || state.consecutive >= trigger) {
			// 连续异常达到阈值进入恢复中；恢复中再现离群则重新开始恢复窗口
			state.recoveryRemaining = state.guard.RecoveryWindow
		}
		return SampleOutlier
	}
	state.consecutive = 0
	if state.recoveryRemaining > 0 {
		state.recoveryRemaining--
		if state.recoveryRemaining > 0 {
			return SampleRecovering
		}
	}
	return SampleValid
}

// Assessed 返回窗口内全部带状态的条目
func (w *ThicknessWindow) Assessed() []AssessedSample {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]AssessedSample, len(w.values))
	copy(out, w.values)
	return out
}

// AssessedForRun 返回某次运行的全部判定条目
func (w *ThicknessWindow) AssessedForRun(runID model.ID) []AssessedSample {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]AssessedSample, 0, len(w.values))
	for _, entry := range w.values {
		if entry.Sample.RunID == runID {
			out = append(out, entry)
		}
	}
	return out
}

func (w *ThicknessWindow) Values() []model.ThicknessSample {
	assessed := w.Assessed()
	out := make([]model.ThicknessSample, len(assessed))
	for index, entry := range assessed {
		out[index] = entry.Sample
	}
	return out
}
