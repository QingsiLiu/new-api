package controller

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

const funnelMaintenanceBatchSize = 500

type funnelMaintenanceHandler struct{}

func (funnelMaintenanceHandler) Type() string { return model.SystemTaskTypeFunnelMaintenance }

func (funnelMaintenanceHandler) Enabled() bool {
	config, err := service.LoadGeiliFunnelConfig()
	return err == nil && config.Enabled
}

func (funnelMaintenanceHandler) Interval() time.Duration { return 24 * time.Hour }

func (funnelMaintenanceHandler) NewPayload() any { return nil }

func (funnelMaintenanceHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := model.RunFunnelMaintenance(ctx, common.GetTimestamp(), funnelMaintenanceBatchSize)
	if err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, nil, err)
		return
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, result, nil)
}
