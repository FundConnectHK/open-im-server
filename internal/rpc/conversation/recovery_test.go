package conversation

import (
	"context"
	"github.com/openimsdk/open-im-server/v3/pkg/authverify"
	"github.com/openimsdk/open-im-server/v3/pkg/common/storage/controller"
	"github.com/openimsdk/open-im-server/v3/pkg/common/storage/database"
	"github.com/openimsdk/open-im-server/v3/pkg/common/storage/model"
	"github.com/openimsdk/open-im-server/v3/pkg/rpcli"
	"github.com/openimsdk/protocol/constant"
	pbconversation "github.com/openimsdk/protocol/conversation"
	"github.com/openimsdk/protocol/group"
	"github.com/openimsdk/protocol/msg"
	"github.com/openimsdk/protocol/sdkws"
	"google.golang.org/grpc"
	"reflect"
	"testing"
)

func TestConversationRecoveryPreservesExplicitDeletion(t *testing.T) {
	for _, tt := range []struct {
		name    string
		version *model.VersionLog
		deleted bool
	}{
		{"new member", nil, false},
		{"deleted", &model.VersionLog{Logs: []model.VersionLogElem{{EID: "sg_group", State: model.VersionStateDelete}}}, true},
		{"active", &model.VersionLog{Logs: []model.VersionLogElem{{EID: "sg_group", State: model.VersionStateInsert}}}, false},
		{"compacted deletion history", &model.VersionLog{Deleted: 1}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.version.WasDeleted("sg_group") != tt.deleted {
				t.Fatal("conversation deletion intent changed")
			}
		})
	}
}

type recoveryGroups struct {
	group.GroupClient
	members   []string
	dismissed bool
}

func (g recoveryGroups) GetGroupsInfo(context.Context, *group.GetGroupsInfoReq, ...grpc.CallOption) (*group.GetGroupsInfoResp, error) {
	status := int32(0)
	if g.dismissed {
		status = constant.GroupStatusDismissed
	}
	return &group.GetGroupsInfoResp{GroupInfos: []*sdkws.GroupInfo{{GroupID: "fixture", Status: status}}}, nil
}
func (g recoveryGroups) GetGroupMemberUserIDs(context.Context, *group.GetGroupMemberUserIDsReq, ...grpc.CallOption) (*group.GetGroupMemberUserIDsResp, error) {
	return &group.GetGroupMemberUserIDsResp{UserIDs: g.members}, nil
}

type recoveryRows struct {
	controller.ConversationDatabase
	rows   map[string]*model.Conversation
	writes int
}

func (d *recoveryRows) FindConversations(_ context.Context, user string, _ []string) ([]*model.Conversation, error) {
	if row := d.rows[user]; row != nil {
		return []*model.Conversation{row}, nil
	}
	return nil, nil
}
func (d *recoveryRows) CreateGroupChatConversation(_ context.Context, _ string, users []string, row *model.Conversation) error {
	for _, user := range users {
		copy := *row
		copy.OwnerUserID = user
		d.rows[user] = &copy
		d.writes++
	}
	return nil
}
func (d *recoveryRows) CreateConversation(_ context.Context, rows []*model.Conversation) error {
	for _, row := range rows {
		copy := *row
		d.rows[row.OwnerUserID] = &copy
		d.writes++
	}
	return nil
}

func recoveryContext() context.Context {
	return authverify.WithTempAdmin(rpcli.WithConversationRecovery(context.Background()))
}

type recoveryMessages struct {
	msg.MsgClient
	reset *msg.SetUserConversationMaxSeqReq
}

func (m *recoveryMessages) SetUserConversationMaxSeq(_ context.Context, req *msg.SetUserConversationMaxSeqReq, _ ...grpc.CallOption) (*msg.SetUserConversationMaxSeqResp, error) {
	m.reset = req
	return &msg.SetUserConversationMaxSeqResp{}, nil
}

type recoveryBounds struct{ database.SeqUser }

func (recoveryBounds) GetUserMinSeq(context.Context, string, string) (int64, error) { return 7, nil }
func (recoveryBounds) GetUserMaxSeq(context.Context, string, string) (int64, error) { return 17, nil }

func recoveryServer() (*conversationServer, *recoveryRows) {
	rows := &recoveryRows{rows: map[string]*model.Conversation{"existing": {OwnerUserID: "existing", ConversationID: "sg_fixture", GroupID: "fixture", MaxSeq: 11, MinSeq: 3, IsPinned: true, RecvMsgOpt: 2, Ex: "preserve"}}}
	server := &conversationServer{config: &Config{}, conversationDatabase: rows, seqUser: recoveryBounds{}, groupClient: &rpcli.GroupClient{GroupClient: recoveryGroups{members: []string{"existing", "missing", "deleted"}}}, wasDeleted: func(_ context.Context, user, _ string) (bool, error) { return user == "deleted", nil }}
	return server, rows
}
func TestConversationRecoveryPreservesRowsAndBounds(t *testing.T) {
	server, rows := recoveryServer()
	old := *rows.rows["existing"]
	req := &pbconversation.CreateGroupChatConversationsReq{GroupID: "fixture", UserIDs: []string{"existing", "missing", "deleted"}}
	for i := 0; i < 2; i++ {
		if _, err := server.CreateGroupChatConversations(recoveryContext(), req); err != nil {
			t.Fatal(err)
		}
	}
	if rows.writes != 1 || !reflect.DeepEqual(old, *rows.rows["existing"]) {
		t.Fatal("existing preferences or visibility changed")
	}
	if row := rows.rows["missing"]; row == nil || row.MinSeq != 7 || row.MaxSeq != 17 {
		t.Fatal("member history boundaries were broadened")
	}
	if rows.rows["deleted"] != nil {
		t.Fatal("deliberate deletion was restored")
	}
}
func TestConversationRecoveryRejectsNonMemberAndDismissedGroup(t *testing.T) {
	for _, dismissed := range []bool{false, true} {
		server, rows := recoveryServer()
		server.groupClient = &rpcli.GroupClient{GroupClient: recoveryGroups{members: []string{"existing"}, dismissed: dismissed}}
		if _, err := server.CreateGroupChatConversations(recoveryContext(), &pbconversation.CreateGroupChatConversationsReq{GroupID: "fixture", UserIDs: []string{"outsider"}}); err == nil || rows.writes != 0 {
			t.Fatal("unauthorized group row created")
		}
	}
}

func TestConversationRecoveryCannotBeEnabledByNonAdmin(t *testing.T) {
	server, rows := recoveryServer()
	ctx := rpcli.WithConversationRecovery(context.Background())
	if _, err := server.CreateGroupChatConversations(ctx, &pbconversation.CreateGroupChatConversationsReq{GroupID: "fixture", UserIDs: []string{"missing"}}); err == nil || rows.writes != 0 {
		t.Fatal("unprivileged caller enabled repair mode")
	}
}

func TestConversationNormalRejoinStillRecreatesDeletedConversation(t *testing.T) {
	server, rows := recoveryServer()
	messages := &recoveryMessages{}
	server.msgClient = &rpcli.MsgClient{MsgClient: messages}
	req := &pbconversation.CreateGroupChatConversationsReq{GroupID: "fixture", UserIDs: []string{"deleted"}}
	if _, err := server.CreateGroupChatConversations(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if rows.rows["deleted"] == nil || rows.writes != 1 || messages.reset == nil || messages.reset.MaxSeq != 0 || !reflect.DeepEqual(messages.reset.OwnerUserID, []string{"deleted"}) {
		t.Fatal("normal join stopped restoring the entrant's active conversation")
	}
}
