package api

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/openimsdk/open-im-server/v3/pkg/authverify"
	"github.com/openimsdk/open-im-server/v3/pkg/rpcli"
	"github.com/openimsdk/protocol/constant"
	"github.com/openimsdk/protocol/conversation"
	"google.golang.org/grpc"
	"net/http/httptest"
	"strings"
	"testing"
)

type repairClient struct {
	conversation.ConversationClient
	marked bool
}

func (c *repairClient) CreateGroupChatConversations(ctx context.Context, _ *conversation.CreateGroupChatConversationsReq, _ ...grpc.CallOption) (*conversation.CreateGroupChatConversationsResp, error) {
	c.marked = rpcli.IsConversationRecovery(ctx)
	keys, _ := ctx.Value(constant.RpcCustomHeader).([]string)
	if len(keys) == 0 {
		return nil, fmt.Errorf("repair flag would be stripped by RPC interceptor")
	}
	return &conversation.CreateGroupChatConversationsResp{}, nil
}

func TestConversationRepairAdminDispatchesOnlyRecoveryMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/conversation/ensure_group_conversations", strings.NewReader(`{"groupID":"fixture","userIDs":["member"]}`))
	ctx.Set(authverify.CtxAdminUserIDsKey, []string{"admin"})
	ctx.Set(constant.OpUserID, "admin")
	client := &repairClient{}
	api := ConversationApi{Client: client}
	api.EnsureGroupConversations(ctx)
	if !client.marked || !strings.Contains(recorder.Body.String(), `"errCode":0`) {
		t.Fatal("admin repair call used ordinary group-join behavior")
	}
}

func TestConversationRepairRejectsNonAdminBeforeRPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/conversation/ensure_group_conversations", strings.NewReader(`{"groupID":"fixture","userIDs":["victim"]}`))
	// A nil client would panic if permission checks happened after RPC dispatch.
	api := ConversationApi{}
	api.EnsureGroupConversations(ctx)
	if !strings.Contains(recorder.Body.String(), `"errCode":1002`) {
		t.Fatalf("unexpected rejection: %s", recorder.Body.String())
	}
}
