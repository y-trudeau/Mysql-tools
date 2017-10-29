package database

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/config"
	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/models"
	tu "github.com/y-trudeau/Mysql-tools/ShardSchema/testutils"
)

func TestConnect(t *testing.T) {
	cfg := &config.Config{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "",
	}

	db, err := NewMySQLConnection(cfg)
	tu.Ok(t, err)
	tu.Assert(t, db.Conn != nil, "db handle is nil")
}

func TestAddOpLog(t *testing.T) {
	db := getDBHandle(t)
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
	db := getDBHandle(t)
	var wantVersion uint32 = 2

	version, err := db.GetMaxVersion()
	tu.Ok(t, err)
	tu.Assert(t, version == wantVersion, fmt.Sprintf("invalid version. Got %d, want %d", version))
}

// This tests GetNextVersion and implicitly it tests GetVersion
func TestGetNextVersion(t *testing.T) {
	db := getDBHandle(t)

	wantVersion := &models.Version{
		Version:    2,
		Command:    "SELECT 1",
		TableName:  "t2",
		CmdType:    "sql",
		LastUpdate: time.Date(2017, 10, 28, 22, 41, 13, 0, time.UTC),
	}

	version, err := db.GetNextVersion(1)
	tu.Ok(t, err)
	tu.Equals(t, version, wantVersion)
}

func getDBHandle(t *testing.T) *Database {
	cfg := &config.Config{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "",
		DBName:   "shardschema",
	}

	db, err := NewMySQLConnection(cfg)
	if err != nil {
		fmt.Printf("cannot connect to the db: %s", err)
		t.Fail()
	}
	return db
}
