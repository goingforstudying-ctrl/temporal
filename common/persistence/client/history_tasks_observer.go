package client

import (
	"context"

	"go.temporal.io/server/common/persistence"
	"go.temporal.io/server/common/testing/testhooks"
)

type historyTasksWrittenObserver struct {
	persistence.ExecutionManager
	observe func(testhooks.HistoryTasksWritten)
}

func newHistoryTasksWrittenObserver(
	manager persistence.ExecutionManager,
	observe func(testhooks.HistoryTasksWritten),
) persistence.ExecutionManager {
	return &historyTasksWrittenObserver{
		ExecutionManager: manager,
		observe:          observe,
	}
}

func (o *historyTasksWrittenObserver) AddHistoryTasks(
	ctx context.Context,
	request *persistence.AddHistoryTasksRequest,
) error {
	err := o.ExecutionManager.AddHistoryTasks(ctx, request)
	if err == nil && request != nil {
		o.observe(testhooks.HistoryTasksWritten{
			ShardID:     request.ShardID,
			RangeID:     request.RangeID,
			NamespaceID: request.NamespaceID,
			WorkflowID:  request.WorkflowID,
			Tasks:       request.Tasks,
		})
	}
	return err
}

func (o *historyTasksWrittenObserver) CreateWorkflowExecution(
	ctx context.Context,
	request *persistence.CreateWorkflowExecutionRequest,
) (*persistence.CreateWorkflowExecutionResponse, error) {
	response, err := o.ExecutionManager.CreateWorkflowExecution(ctx, request)
	if err == nil && request != nil {
		o.observeWorkflowSnapshot(request.ShardID, request.RangeID, &request.NewWorkflowSnapshot)
	}
	return response, err
}

func (o *historyTasksWrittenObserver) UpdateWorkflowExecution(
	ctx context.Context,
	request *persistence.UpdateWorkflowExecutionRequest,
) (*persistence.UpdateWorkflowExecutionResponse, error) {
	response, err := o.ExecutionManager.UpdateWorkflowExecution(ctx, request)
	if err == nil && request != nil {
		o.observeWorkflowMutation(request.ShardID, request.RangeID, &request.UpdateWorkflowMutation)
		o.observeWorkflowSnapshot(request.ShardID, request.RangeID, request.NewWorkflowSnapshot)
	}
	return response, err
}

func (o *historyTasksWrittenObserver) ConflictResolveWorkflowExecution(
	ctx context.Context,
	request *persistence.ConflictResolveWorkflowExecutionRequest,
) (*persistence.ConflictResolveWorkflowExecutionResponse, error) {
	response, err := o.ExecutionManager.ConflictResolveWorkflowExecution(ctx, request)
	if err == nil && request != nil {
		o.observeWorkflowSnapshot(request.ShardID, request.RangeID, &request.ResetWorkflowSnapshot)
		o.observeWorkflowSnapshot(request.ShardID, request.RangeID, request.NewWorkflowSnapshot)
		o.observeWorkflowMutation(request.ShardID, request.RangeID, request.CurrentWorkflowMutation)
	}
	return response, err
}

func (o *historyTasksWrittenObserver) observeWorkflowSnapshot(
	shardID int32,
	rangeID int64,
	snapshot *persistence.WorkflowSnapshot,
) {
	if snapshot == nil || snapshot.ExecutionInfo == nil {
		return
	}
	o.observe(testhooks.HistoryTasksWritten{
		ShardID:     shardID,
		RangeID:     rangeID,
		NamespaceID: snapshot.ExecutionInfo.NamespaceId,
		WorkflowID:  snapshot.ExecutionInfo.WorkflowId,
		Tasks:       snapshot.Tasks,
	})
}

func (o *historyTasksWrittenObserver) observeWorkflowMutation(
	shardID int32,
	rangeID int64,
	mutation *persistence.WorkflowMutation,
) {
	if mutation == nil || mutation.ExecutionInfo == nil {
		return
	}
	o.observe(testhooks.HistoryTasksWritten{
		ShardID:     shardID,
		RangeID:     rangeID,
		NamespaceID: mutation.ExecutionInfo.NamespaceId,
		WorkflowID:  mutation.ExecutionInfo.WorkflowId,
		Tasks:       mutation.Tasks,
	})
}
