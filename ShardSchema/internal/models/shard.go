package models

import "database/sql"

type Shard struct {
	ShardId    uint32         // Id of the shard
	SchemaName string         // Name of the schema, ex shard_1234
	ShardDSN   string         // DSN of the shard, schemaName is appended
	Version    uint32         // current schema version of the shard
	TaskName   sql.NullString // identifier for current task updating the shard
	LastTaskHb NullTime       // last heartbeat of the updating task
	LastUpdate NullTime       // when was the last update to the row
}
