package models

import "database/sql"

type OpLog struct {
	ShardId    uint32         // Id of the shard
	Version    uint32         // current schema version of the shard
	Seq        uint8          // message sequence
	TaskName   sql.NullString // identifier for current task updating the shard
	Message    sql.NullString //
	Output     sql.NullString // stdout output of the command
	Err        sql.NullString // stderr output of the command
	LastUpdate NullTime       // when was the last update to the row
}
