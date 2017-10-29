package models

import "time"

type Version struct {
	Version    uint32
	Command    string    // alter command to run
	TableName  string    // affected table
	CmdType    string    // type of command, 'sql' or 'pt-osc'
	LastUpdate time.Time // when was the last update to the row
}
