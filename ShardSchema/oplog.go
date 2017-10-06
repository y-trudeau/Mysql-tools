package main

import (
        //"database/sql"            
        //"errors"
        //"fmt"
        "strconv"
        "time"
)

type OpLog struct {
    shardId  uint32     // Id of the shard
    version uint32      // current schema version of the shard
    seq uint8           // message sequence
    taskName string         // identifier for current task updating the shard
    message string 
    output string  // stdout output of the command
    err string // stderr output of the command
    lastUpdate time.Time  // when was the last update to the row
}

// Insert an Oplog entry
func addOpLog(shardId uint32, version uint32, taskName string, message string, stdout string, stderr string) {
    Logger.Println("getShard for shardId = " + strconv.Itoa(int(shardId)))
    
    db := getDbHandle()
    
    res, err := db.Exec(" insert into oplog (shardId,version,seq,taskName,message,output,err) " +
        "select $1,$2,(select coalesce(max(seq),0)+1 from oplog where shardId = 1 and version=1)," +
        "$3, $4, $5, $6;", shardId, version, taskName, message, stdout, stderr)
    
    if err != nil {
        Logger.Fatal("Can't insert a row  in the opLog table: " + err.Error())
    }
    
    count, err := res.RowsAffected()
    if count != 1 {
        Logger.Fatal("Problem with the insert, RowsAffected = " + strconv.Itoa(int(count)) +" instead of 1:" + err.Error())
    }
}

