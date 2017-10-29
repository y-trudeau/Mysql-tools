package models

import (
	"database/sql"
	"database/sql/driver"
	"time"
)

type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

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

type Shard struct {
	ShardId    uint32    // Id of the shard
	SchemaName string    // Name of the schema, ex shard_1234
	ShardDSN   string    // DSN of the shard, schemaName is appended
	Version    uint32    // current schema version of the shard
	TaskName   string    // identifier for current task updating the shard
	LastTaskHb time.Time // last heartbeat of the updating task
	LastUpdate time.Time // when was the last update to the row
}

type Version struct {
	Version    uint32
	Command    string    // alter command to run
	TableName  string    // affected table
	CmdType    string    // type of command, 'sql' or 'pt-osc'
	LastUpdate time.Time // when was the last update to the row
}
