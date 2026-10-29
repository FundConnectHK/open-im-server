// Copyright © 2023 OpenIM. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/openimsdk/open-im-server/v3/pkg/authverify"
	"github.com/openimsdk/open-im-server/v3/pkg/rpcli"
	"github.com/openimsdk/tools/apiresp"
	"google.golang.org/grpc"

	"github.com/openimsdk/protocol/conversation"
	"github.com/openimsdk/tools/a2r"
)

type ConversationApi struct {
	Client conversation.ConversationClient
}

// EnsureGroupConversations is an internal service repair operation, never a
// client privilege: membership and visibility are rechecked by the RPC.
func (o *ConversationApi) EnsureGroupConversations(c *gin.Context) {
	if err := authverify.CheckAdmin(c); err != nil {
		apiresp.GinError(c, err)
		return
	}
	a2r.Call(c, func(client conversation.ConversationClient, ctx context.Context, req *conversation.CreateGroupChatConversationsReq, options ...grpc.CallOption) (*conversation.CreateGroupChatConversationsResp, error) {
		return client.CreateGroupChatConversations(rpcli.WithConversationRecovery(ctx), req, options...)
	}, o.Client)
}

func NewConversationApi(client conversation.ConversationClient) ConversationApi {
	return ConversationApi{client}
}

func (o *ConversationApi) GetAllConversations(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetAllConversations, o.Client)
}

func (o *ConversationApi) GetSortedConversationList(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetSortedConversationList, o.Client)
}

func (o *ConversationApi) GetConversation(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetConversation, o.Client)
}

func (o *ConversationApi) GetConversations(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetConversations, o.Client)
}

func (o *ConversationApi) SetConversations(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.SetConversations, o.Client)
}

//func (o *ConversationApi) GetConversationOfflinePushUserIDs(c *gin.Context) {
//	a2r.Call(c, conversation.ConversationClient.GetConversationOfflinePushUserIDs, o.Client)
//}

func (o *ConversationApi) GetFullOwnerConversationIDs(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetFullOwnerConversationIDs, o.Client)
}

func (o *ConversationApi) GetIncrementalConversation(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetIncrementalConversation, o.Client)
}

func (o *ConversationApi) GetOwnerConversation(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetOwnerConversation, o.Client)
}

func (o *ConversationApi) GetNotNotifyConversationIDs(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetNotNotifyConversationIDs, o.Client)
}

func (o *ConversationApi) GetPinnedConversationIDs(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.GetPinnedConversationIDs, o.Client)
}

func (o *ConversationApi) UpdateConversationsByUser(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.UpdateConversationsByUser, o.Client)
}

func (o *ConversationApi) DeleteConversations(c *gin.Context) {
	a2r.Call(c, conversation.ConversationClient.DeleteConversations, o.Client)
}
