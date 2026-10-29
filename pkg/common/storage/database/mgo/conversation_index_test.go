package mgo

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/openimsdk/open-im-server/v3/pkg/common/storage/database"
	"github.com/openimsdk/open-im-server/v3/pkg/common/storage/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestConversationDeletionHistory(t *testing.T) {
	uri := os.Getenv("OPENIM_CONVERSATION_INDEX_TEST_URI")
	if uri == "" {
		t.Skip("set OPENIM_CONVERSATION_INDEX_TEST_URI to an authorized local MongoDB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database("openim_v3")
	versions := db.Collection(database.ConversationVersionName)
	owner := fmt.Sprintf("conversation_recovery_regression_%d", time.Now().UnixNano())
	defer versions.DeleteMany(context.Background(), bson.M{"d_id": owner})
	store := &ConversationMgo{coll: db.Collection(database.ConversationName)}
	if deleted, err := store.ConversationWasDeleted(ctx, owner, "sg_missing"); err != nil || deleted {
		t.Fatalf("new identity incorrectly treated as deleted: deleted=%v err=%v", deleted, err)
	}
	_, err = versions.InsertOne(ctx, bson.M{"d_id": owner, "deleted": 1, "logs": []model.VersionLogElem{
		{EID: "sg_deleted", State: model.VersionStateDelete},
		{EID: "sg_active", State: model.VersionStateInsert},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		id      string
		deleted bool
	}{{"sg_deleted", true}, {"sg_active", false}, {"sg_compacted", true}} {
		if deleted, err := store.ConversationWasDeleted(ctx, owner, tt.id); err != nil || deleted != tt.deleted {
			t.Fatalf("Mongo deletion projection for %s: deleted=%v err=%v", tt.id, deleted, err)
		}
	}
}

func TestEnsureConversationIndexes(t *testing.T) {
	uri := os.Getenv("OPENIM_CONVERSATION_INDEX_TEST_URI")
	if uri == "" {
		t.Skip("set OPENIM_CONVERSATION_INDEX_TEST_URI to an authorized local MongoDB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_unique_index_%t", legacy), func(t *testing.T) {
			coll := client.Database("openim_v3").Collection(fmt.Sprintf("conversation_index_regression_%d", time.Now().UnixNano()))
			defer coll.Drop(context.Background())
			if legacy {
				_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "conversation_id", Value: 1}}, Options: options.Index().SetUnique(true)})
				if err != nil {
					t.Fatal(err)
				}
			}
			first := bson.M{"owner_user_id": "member-a", "conversation_id": "same-group", "ex": "preserve-history-owner"}
			if _, err := coll.InsertOne(ctx, first); err != nil {
				t.Fatal(err)
			}
			// Independent service processes can initialize the same collection.
			var wg sync.WaitGroup
			failures := make(chan error, 4)
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); failures <- ensureConversationIndexes(ctx, coll) }()
			}
			wg.Wait()
			close(failures)
			for err := range failures {
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := coll.InsertOne(ctx, bson.M{"owner_user_id": "member-b", "conversation_id": "same-group"}); err != nil {
				t.Fatalf("second member must own its own group conversation: %v", err)
			}
			if _, err := coll.InsertOne(ctx, first); !mongo.IsDuplicateKeyError(err) {
				t.Fatalf("same member/conversation pair must remain unique: %v", err)
			}
			if count, err := coll.CountDocuments(ctx, bson.M{"ex": "preserve-history-owner"}); err != nil || count != 1 {
				t.Fatalf("existing identity was modified: count=%d, err=%v", count, err)
			}
			if err := ensureConversationIndexes(ctx, coll); err != nil {
				t.Fatalf("repeated initialization must be safe: %v", err)
			}
		})
	}
}
