package main

import (
        //"database/sql"
        //"errors"
        //"fmt"
        //"strings"
        "time"
)


type Version struct {
    version  uint32
    command string      // alter command to run
    tableName string    // affected table
    cmdType string      // type of command, 'sql' or 'pt-osc'
    lastUpdate time.Time  // when was the last update to the row
}


// This function returns the highest current version
func getMaxVersion() uint32 {
    var version uint32
    
    db := getDbHandle()
    err := db.QueryRow("select coalesce(max(version),0) from versions").Scan(&version)
    
    if err != nil {
        Logger.Fatal("Did not get a max version value, really strange: " + err)
    }
    return version
}

// get the Version object of the following version
func getNextVersion(version uint32) (Version,err) {
    var lVersion uint32
    
    Logger.Println("getNextVersion for version = " + strconv.Itoa(version))
    
    db := getDbHandle()
    err := db.QueryRow("select version from versions where version > ? order by version limit 1;", version).
                Scan(&lVersion)
                
    if err != nil {
        return nil,err
    }
    
    return getVersion(lVersion)

}
    
// Returns a Version struct of a given version
func getVersion(version uint32) (Version,err) {
    var lVersion uint32
    var lCommand string      // alter command to run
    var lTableName string    // affected table
    var lCmdType string      // type of command, 'sql' or 'pt-osc'
    var lLastUpdate time.Time  // when was the last update to the row
    
    Logger.Println("getVersion for version = " + strconv.Itoa(version))
    
    db := getDbHandle()
    err := db.QueryRow("select version, command, tableName, cmdType, lastUpdate from versions where version = ? ", version).
                Scan(&lVersion,&lCommand, &lTableName, &lCmdType, &lLastUpdate)

    if err != nil {
        return nil,err
    }
    
    return Version{lVersion, lCommand, lTableName, lCmdType, lLastUpdate}, nil

}
