package model

// GuardStatus 描述厚度样本在配方层守卫下的判定状态。
type GuardStatus string

const (
	GuardValid      GuardStatus = "valid"      // 有效
	GuardOutlier    GuardStatus = "outlier"    // 离群
	GuardMissing    GuardStatus = "missing"    // 缺失
	GuardRecovering GuardStatus = "recovering" // 恢复中
)

// BlocksRelease 报告该状态是否阻止放行结论推进。
func (s GuardStatus) BlocksRelease() bool {
	return s == GuardOutlier || s == GuardMissing || s == GuardRecovering
}

// ThicknessGuard 是按配方层配置的厚度守卫参数。
type ThicknessGuard struct {
	MaxDeviationNm       float64 `json:"max_deviation_nm"`    // 离群阈值：与目标厚度允许的最大偏差（<=0 表示不启用）
	ConsecutiveAnomalies int     `json:"consecutive_anomalies"` // 连续异常次数：达到才判定离群
	RecoveryWindow       int     `json:"recovery_window"`       // 恢复窗口：离群后连续达标样本数才恢复有效
}

// Normalize 补齐计数类参数的默认值；MaxDeviationNm<=0 保持关闭守卫。
func (g ThicknessGuard) Normalize() ThicknessGuard {
	if g.ConsecutiveAnomalies < 1 {
		g.ConsecutiveAnomalies = 1
	}
	if g.RecoveryWindow < 1 {
		g.RecoveryWindow = 1
	}
	return g
}

// Enabled 报告该守卫是否启用离群判定。
func (g ThicknessGuard) Enabled() bool {
	return g.MaxDeviationNm > 0
}
