// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package database_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"go.mau.fi/util/dbutil"
	_ "go.mau.fi/util/dbutil/litestream"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func equal[T any](t *testing.T, want, got T, msg ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s: want %#v, got %#v", fmt.Sprint(msg...), want, got)
	}
}

func TestParseRoomTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    event.Tags
	}{
		{"empty object", `{}`, event.Tags{}},
		{"no tags", `{"tags":{}}`, event.Tags{}},
		{"invalid json", `{`, event.Tags{}},
		{"numeric order", `{"tags":{"m.favourite":{"order":0.5}}}`, event.Tags{"m.favourite": {Order: "0.5"}}},
		{"no order", `{"tags":{"m.lowpriority":{}}}`, event.Tags{"m.lowpriority": {}}},
		{
			"malformed entry kept without metadata",
			`{"tags":{"m.favourite":{"order":"high"},"u.work":{"order":1}}}`,
			event.Tags{"m.favourite": {}, "u.work": {Order: "1"}},
		},
		{"non-object entry", `{"tags":{"m.favourite":true}}`, event.Tags{"m.favourite": {}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := database.ParseRoomTags(json.RawMessage(tt.content))
			if got == nil {
				t.Fatal("got nil map")
			}
			equal(t, tt.want, got)
		})
	}
}

func TestRoomTagsCheckChanges(t *testing.T) {
	fav := event.Tags{"m.favourite": {Order: "0.5"}}

	t.Run("nil means unchanged", func(t *testing.T) {
		old := &database.Room{Tags: fav}
		changed, syncable := (&database.Room{}).CheckChangesAndCopyInto(old)
		equal(t, false, changed, "changed")
		equal(t, false, syncable, "syncable")
		equal(t, fav, old.Tags)
	})
	t.Run("equal tags are not a change", func(t *testing.T) {
		old := &database.Room{Tags: fav}
		changed, _ := (&database.Room{Tags: event.Tags{"m.favourite": {Order: "0.5"}}}).CheckChangesAndCopyInto(old)
		equal(t, false, changed, "changed")
	})
	t.Run("new tags are a syncable change", func(t *testing.T) {
		old := &database.Room{}
		changed, syncable := (&database.Room{Tags: fav}).CheckChangesAndCopyInto(old)
		equal(t, true, changed, "changed")
		equal(t, true, syncable, "syncable")
		equal(t, fav, old.Tags)
	})
	t.Run("empty map clears tags", func(t *testing.T) {
		old := &database.Room{Tags: fav}
		changed, syncable := (&database.Room{Tags: event.Tags{}}).CheckChangesAndCopyInto(old)
		equal(t, true, changed, "changed")
		equal(t, true, syncable, "syncable")
		equal(t, 0, len(old.Tags), "len")
	})
}

func newTestDB(t *testing.T) *database.Database {
	t.Helper()
	rawDB, err := dbutil.NewWithDialect(filepath.Join(t.TempDir(), "hicli.db"), "sqlite3-fk-wal")
	must(t, err)
	t.Cleanup(func() { _ = rawDB.Close() })
	db := database.New(rawDB)
	must(t, db.Upgrade(context.Background()))
	return db
}

func TestRoomTagsRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	roomID := id.RoomID("!room:example.com")
	must(t, db.Room.CreateRow(ctx, roomID))

	room, err := db.Room.Get(ctx, roomID)
	must(t, err)
	equal(t, nil, room.Tags, "room without tags should scan as nil")

	fav := event.Tags{"m.favourite": {Order: "0.25"}}
	must(t, db.Room.Update(ctx, &database.Room{ID: roomID, Tags: fav}))
	room, err = db.Room.Get(ctx, roomID)
	must(t, err)
	equal(t, fav, room.Tags)

	// An update without tags must not clear them.
	must(t, db.Room.Update(ctx, &database.Room{ID: roomID, Topic: new("topic")}))
	room, err = db.Room.Get(ctx, roomID)
	must(t, err)
	equal(t, fav, room.Tags)

	// An empty map clears them.
	must(t, db.Room.Update(ctx, &database.Room{ID: roomID, Tags: event.Tags{}}))
	room, err = db.Room.Get(ctx, roomID)
	must(t, err)
	equal(t, event.Tags{}, room.Tags)
}

func TestRoomTagsMigrationBackfill(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	userID := id.UserID("@user:example.com")
	tagged := id.RoomID("!tagged:example.com")
	untagged := id.RoomID("!untagged:example.com")
	must(t, db.Room.CreateRow(ctx, tagged))
	must(t, db.Room.CreateRow(ctx, untagged))
	_, err := db.AccountData.PutRoom(ctx, userID, tagged, event.AccountDataRoomTags,
		json.RawMessage(`{"tags":{"m.lowpriority":{"order":"bad"},"u.work":{}}}`))
	must(t, err)

	// Simulate a database from before v32 and re-run the migration.
	_, err = db.Exec(ctx, "UPDATE room SET tags=NULL")
	must(t, err)
	_, err = db.Exec(ctx, "ALTER TABLE room DROP COLUMN tags")
	must(t, err)
	_, err = db.Exec(ctx, "UPDATE version SET version=31")
	must(t, err)
	must(t, db.Upgrade(ctx))

	room, err := db.Room.Get(ctx, tagged)
	must(t, err)
	equal(t, event.Tags{"m.lowpriority": {}, "u.work": {}}, room.Tags)
	room, err = db.Room.Get(ctx, untagged)
	must(t, err)
	equal(t, nil, room.Tags)
}
