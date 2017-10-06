package main

import (
        "database/sql"            
        //"errors"
        //"fmt"
        //"strings"
        "time"
)

type Shard struct {
    shardId  uint32     // Id of the shard
    schemaName string   // Name of the schema, ex shard_1234
    shardDSN string     // DSN of the shard, schemaName is appended
    version uint32      // current schema version of the shard
    taskName string         // identifier for current task updating the shard
    lastTaskHb time.Time  // last heartbeat of the updating task
    lastUpdate time.Time  // when was the last update to the row
}

// returns a Shard struc of for a given shardId
func getShard(shardId uint32) (Shard) {
    var lShardId  uint32     
    var lSchemaName string   
    var lShardDSN string     
    var lVersion uint32      
    var lTaskName string         
    var lLastTaskHb time.Time 
    var lLastUpdate time.Time  
    
    Logger.Println("getShard for shardId = " + strconv.Itoa(shardId))
    
    db := getDbHandle()
    err := db.QueryRow("select shardId, schemaName, shardDSN, version, task, lastTaskHb, lastUpdate from shards where shardId = ? ", shardId).
                Scan(&lShardId, &lSchemaName, &lShardDSN, &lVersion, &lTaskName, &lLastTaskHb, &lLastUpdate)

    if err != nil {
        return nil,err
    }
    
    return Shard{lShardId, lSchemaName, lShardDSN, lVersion, lTask, lLastTaskHb, lLastUpdate}, nil
}

// Find a shard that has a lower version and no taskName
// Should be called only by the dispatcher otherwise it needs a mutex
func getShardToUpgrade(version uint32, taskName string) (Shard) {
    var lShardId uint32
    
    Logger.Println("getShardToUpgrade with version = " + strconv.Itoa(version) + " and taskName = " + taskName)
    
    db := getDbHandle()
    err := db.QueryRow("select shardId from shards where version < ? and taskName is Null order by lastUpdate limit 1", version).
                Scan(&lShardId)
                
    switch {
        case err == sql.ErrNoRows:
            Logger.Println("Found no shards needing ddl")
            return nil,err
        case err != nil:
            Logger.Fatal("unexpected error looking for shards: " + err)
    }
    
    _, err := db.Exec("update shards set taskName = $1, lastTaskHb = now() where shardId = $2", taskName, lShardId)
    
    if err != nil {
        Logger.Fatal("Can't update the shards entry in the database: " + err)
    }
    
    return getShard(lShardId)

}

// update the lastTaskHb field for the shardId and provided the taskName matches
func updateShardTaskHeartbeat(shardId uint32, taskName string) {
    Logger.Println("updateShardTaskHeartbeat for shardId = " + strconv.Itoa(shardId) + " and taskName = " + taskName)
    
    db := getDbHandle()
    
    res, err := db.Exec("update shards set lastTaskHb = now() where taskName = $1 and shardId = $2", taskName, shardId)
    
    if err != nil {
        Logger.Fatal("Can't update lastTaskHb of the shard in the database: " + err)
    }
    
    count := res.RowsAffected()
    if count != 1 {
        Logger.Fatal("Problem with the update, count = " + strconv.Itoa(count) +" instead of 1: " + err)
    }
    
}

func shardUpgradeDone(shardId uint32, version uint32, taskName string) {
    Logger.Println("shardUpgradeDone for shardId = " + shardId + ", taskName = " + taskName + " and version = " + version)
    
    db := getDbHandle()

    res, err := db.Exec("update shards set lastTaskHb = now(), version = $3, taskName = NULL where taskName = $1 and shardId = $2", taskName, shardId, version)
    
    if err != nil {
        Logger.Fatal("Can't mark the shard as upgraded in the database: " + err)
    }
    
    count := res.RowsAffected()
    if count != 1 {
        Logger.Fatal("Problem with the update, count = " + strconv.Itoa(count) +" instead of 1: " + err)
    }
    
}

