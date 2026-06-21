package replication

import (
	"go.temporal.io/server/common/testing/testhooks"
	"google.golang.org/protobuf/proto"
)

const (
	adminStreamWorkflowReplicationMessagesMethod   = "/temporal.server.api.adminservice.v1.AdminService/StreamWorkflowReplicationMessages"
	historyStreamWorkflowReplicationMessagesMethod = "/temporal.server.api.historyservice.v1.HistoryService/StreamWorkflowReplicationMessages"
)

func hasReplicationStreamMessageObserver(testHooks testhooks.TestHooks) bool {
	_, ok := testhooks.Get(testHooks, testhooks.ReplicationStreamMessageObserver, testhooks.GlobalScope)
	return ok
}

func observeReplicationStreamMessage(
	testHooks testhooks.TestHooks,
	method string,
	direction testhooks.ReplicationStreamMessageDirection,
	clusterName string,
	targetAddress string,
	msg proto.Message,
) {
	if msg == nil {
		return
	}
	if hook, ok := testhooks.Get(testHooks, testhooks.ReplicationStreamMessageObserver, testhooks.GlobalScope); ok {
		hook(testhooks.ReplicationStreamMessage{
			Method:        method,
			Direction:     direction,
			ClusterName:   clusterName,
			TargetAddress: targetAddress,
			Message:       msg,
			IsStreamCall:  true,
		})
	}
}
