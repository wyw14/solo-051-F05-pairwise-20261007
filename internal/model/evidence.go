package model

import (
	"fmt"
	"time"
)

type EvidenceStatus string

const (
	EvidenceCollecting EvidenceStatus = "collecting"
	EvidenceComplete   EvidenceStatus = "complete"
	EvidenceInvalid    EvidenceStatus = "invalid"
)

type LayerEvidence struct {
	RunID               ID             `json:"run_id"`
	LayerNumber         int            `json:"layer_number"`
	QuartzFrequency     float64        `json:"quartz_frequency"`
	OpticalTransmission float64        `json:"optical_transmission"`
	ThicknessNanometers float64        `json:"thickness_nm"`
	QuartzValid         bool           `json:"quartz_valid"`
	OpticalValid        bool           `json:"optical_valid"`
	CapturedAt          time.Time      `json:"captured_at"`
	Status              EvidenceStatus `json:"status"`
}

func BuildLayerEvidence(sample ThicknessSample, layerNumber int) LayerEvidence {
	status := EvidenceCollecting
	quartzValid := sample.Valid && sample.Frequency > 0
	opticalValid := sample.Valid && sample.Transmission >= 0 && sample.Transmission <= 1
	if quartzValid && opticalValid && sample.Nanometers >= 0 {
		status = EvidenceComplete
	}
	if !sample.Valid {
		status = EvidenceInvalid
	}
	// 离群/缺失/恢复中的样本保留原始测量值，但证据不能判定为完整，
	// 从而无法推进放行结论。
	if status == EvidenceComplete && sample.Guard.BlocksRelease() {
		status = EvidenceCollecting
	}
	return LayerEvidence{RunID: sample.RunID, LayerNumber: layerNumber, QuartzFrequency: sample.Frequency, OpticalTransmission: sample.Transmission, ThicknessNanometers: sample.Nanometers, QuartzValid: quartzValid, OpticalValid: opticalValid, CapturedAt: sample.At, Status: status}
}

func (e LayerEvidence) Validate() error {
	if e.RunID == "" {
		return fmt.Errorf("evidence run is required")
	}
	if e.LayerNumber < 1 {
		return fmt.Errorf("evidence layer is required")
	}
	if e.Status != EvidenceComplete {
		return fmt.Errorf("evidence is not complete")
	}
	if !e.QuartzValid || !e.OpticalValid {
		return fmt.Errorf("evidence sensors are invalid")
	}
	return nil
}
