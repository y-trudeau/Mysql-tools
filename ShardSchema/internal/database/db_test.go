package database

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/models"
	tu "github.com/y-trudeau/Mysql-tools/ShardSchema/testutils"
)

func TestAddOpLog(t *testing.T) {
	db := getDB(t)
	db.Conn.Exec("DELETE FROM oplog WHERE shardId >= 100")

	err := db.AddOpLog(100, 100, "taskName", "message", "stdout", "stderr")
	tu.Ok(t, err)

	query := "SELECT `shardId`, `version`, `seq`, `taskName`, `message`, " +
		"`output`, `err`, `lastUpdate` FROM oplog WHERE shardId = 100 AND version = 100"
	o := models.OpLog{}
	err = db.Conn.QueryRow(query).Scan(&o.ShardId, &o.Version, &o.Seq, &o.TaskName, &o.Message,
		&o.Output, &o.Err, &o.LastUpdate)

	want := models.OpLog{
		ShardId:    100,
		Version:    100,
		Seq:        3,
		TaskName:   sql.NullString{String: "taskName", Valid: true},
		Message:    sql.NullString{String: "message", Valid: true},
		Output:     sql.NullString{String: "stdout", Valid: true},
		Err:        sql.NullString{String: "stderr", Valid: true},
		LastUpdate: o.LastUpdate,
	}
	tu.Equals(t, o, want)
}

func TestGetMaxVersion(t *testing.T) {
	db := getDB(t)
	var wantVersion uint32 = 2

	version, err := db.GetMaxVersion()
	tu.Ok(t, err)
	tu.Assert(t, version == wantVersion, fmt.Sprintf("invalid version. Got %d, want %d", version, wantVersion))
}

// This tests GetNextVersion and implicitly it tests GetVersion
func TestGetNextVersion(t *testing.T) {
	db := getDB(t)

	wantVersion := &models.Version{
		Version:   2,
		Command:   "SELECT 1",
		TableName: "t2",
		CmdType:   "sql",
	}

	version, err := db.GetNextVersion(1)
	tu.Ok(t, err)
	// Overwrite LastUpdate because it was written as NOW(), so we don't know the value
	wantVersion.LastUpdate = version.LastUpdate
	tu.Equals(t, version, wantVersion)
}

func TestGetShard(t *testing.T) {
	db := getDB(t)

	wantShard := &models.Shard{
		ShardId:    1,
		SchemaName: "shard_1",
		ShardDSN:   "user:pass@(tcp:10.2.2.1:3306)",
		Version:    0,
		TaskName:   sql.NullString{String: "", Valid: false},
	}

	shard, err := db.GetShard(1)
	tu.Ok(t, err)
	// Overwrite these values because they were written as NOW() so we cannot foresee the values.
	wantShard.LastTaskHb = shard.LastTaskHb
	wantShard.LastUpdate = shard.LastUpdate
	tu.Equals(t, wantShard, shard)
}

func getDB(t *testing.T) *Database {
	conn := tu.GetMySQLConnection(t)
	return NewDatabase(conn)
}
