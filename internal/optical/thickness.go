package optical

import (
	"github.com/wyw14/cry-161/internal/model"
	"math"
	"sync"
)

// layerGuard 保存单个配方层的守卫配置与判定状态。
type layerGuard struct {
	target    float64
	cfg       model.ThicknessGuard
	anomalies int // 当前连续超阈值次数
	recovered int // 离群后当前连续达标次数
	status    model.GuardStatus
}

type ThicknessWindow struct {
	mu     sync.RWMutex
	values []model.ThicknessSample
	limit  int
	layers map[int]*layerGuard
}

func NewThicknessWindow(limit int) *ThicknessWindow {
	if limit < 1 {
		limit = 1
	}
	return &ThicknessWindow{limit: limit, values: make([]model.ThicknessSample, 0, limit), layers: map[int]*layerGuard{}}
}

// ConfigureLayer 按配方层设置离群阈值、连续异常次数和恢复窗口。
func (w *ThicknessWindow) ConfigureLayer(layer int, target float64, cfg model.ThicknessGuard) {
	w.mu.Lock()
	state, ok := w.layers[layer]
	if !ok {
		state = &layerGuard{status: model.GuardMissing}
		w.layers[layer] = state
	}
	state.target = target
	state.cfg = cfg.Normalize()
	w.mu.Unlock()
}

// Assess 按层守卫判定样本状态并推进状态机；原始样本仍由 Add 完整保留。
func (w *ThicknessWindow) Assess(layer int, sample model.ThicknessSample) model.GuardStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	state, ok := w.layers[layer]
	if !ok {
		state = &layerGuard{cfg: model.ThicknessGuard{}.Normalize(), status: model.GuardMissing}
		w.layers[layer] = state
	}
	if !sample.Valid {
		// 缺失样本：不改变已锁存的状态，也不计入异常或恢复。
		return model.GuardMissing
	}
	if !state.cfg.Enabled() {
		state.status = model.GuardValid
		return state.status
	}
	if math.Abs(sample.Nanometers-state.target) > state.cfg.MaxDeviationNm {
		state.anomalies++
		state.recovered = 0
		// 已处于离群/恢复中时，任何超阈值样本立即回到离群；
		// 连续异常次数的宽限只适用于从健康状态进入离群。
		if state.status == model.GuardOutlier || state.status == model.GuardRecovering || state.anomalies >= state.cfg.ConsecutiveAnomalies {
			state.status = model.GuardOutlier
		}
		return state.status
	}
	state.anomalies = 0
	if state.status == model.GuardOutlier || state.status == model.GuardRecovering {
		state.recovered++
		if state.recovered >= state.cfg.RecoveryWindow {
			state.status = model.GuardValid
			state.recovered = 0
		} else {
			state.status = model.GuardRecovering
		}
		return state.status
	}
	state.status = model.GuardValid
	return state.status
}

// Add 保留原始样本证据，包括离群与缺失样本。
func (w *ThicknessWindow) Add(sample model.ThicknessSample) {
	w.mu.Lock()
	w.values = append(w.values, sample)
	if len(w.values) > w.limit {
		w.values = w.values[len(w.values)-w.limit:]
	}
	w.mu.Unlock()
}

func (w *ThicknessWindow) Values() []model.ThicknessSample {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]model.ThicknessSample, len(w.values))
	copy(out, w.values)
	return out
}

// Status 返回某层当前锁存状态；从未见过的层为缺失。
func (w *ThicknessWindow) Status(layer int) model.GuardStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	state, ok := w.layers[layer]
	if !ok {
		return model.GuardMissing
	}
	return state.status
}

// LayerStatuses 返回所有层当前状态的快照。
func (w *ThicknessWindow) LayerStatuses() map[int]model.GuardStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make(map[int]model.GuardStatus, len(w.layers))
	for layer, state := range w.layers {
		out[layer] = state.status
	}
	return out
}

// ReleaseReady 仅当所有已知层都处于有效状态时才允许推进放行结论。
func (w *ThicknessWindow) ReleaseReady() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, state := range w.layers {
		if state.status != model.GuardValid {
			return false
		}
	}
	return true
}
