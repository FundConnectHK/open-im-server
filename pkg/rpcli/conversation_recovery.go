package rpcli

import (
	"context"

	"github.com/openimsdk/protocol/constant"
)

const conversationRecoveryKey = "openim-conversation-recovery"

// WithConversationRecovery marks the internal admin repair call. RpcCustomHeader
// is required because the OpenIM interceptor rebuilds outgoing gRPC metadata.
func WithConversationRecovery(ctx context.Context) context.Context {
	keys, _ := ctx.Value(constant.RpcCustomHeader).([]string)
	copyKeys := append([]string(nil), keys...)
	for _, key := range copyKeys {
		if key == conversationRecoveryKey {
			return context.WithValue(ctx, conversationRecoveryKey, []string{"1"})
		}
	}
	ctx = context.WithValue(ctx, constant.RpcCustomHeader, append(copyKeys, conversationRecoveryKey))
	return context.WithValue(ctx, conversationRecoveryKey, []string{"1"})
}

func IsConversationRecovery(ctx context.Context) bool {
	values, _ := ctx.Value(conversationRecoveryKey).([]string)
	return len(values) == 1 && values[0] == "1"
}
