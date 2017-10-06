package main

import (
        "database/sql"
		_ "github.com/go-sql-driver/mysql"
        //"errors"
        //"strings"
        //"time"

)

var DB *sql.DB   // the configuration database handle

func InitDB(cfg *Config) {
    Logger.Println("Connection do DB")
    
    db, err := sql.Open("mysql", "skeema:skeema@tcp(10.0.3.87:3306)/shardschema")
	if err != nil {
    	Logger.Fatal(err.Error()) // Just for example purpose. You should use proper error handling instead of panic
	}
	defer db.Close()

    DB = db

	// Open doesn't open a connection. Validate DSN data:
	err = db.Ping()
	if err != nil {
    	Logger.Fatal(err.Error()) // proper error handling instead of panic in your app
	}
    Logger.Println("Ping worked ")
}

func getDbHandle() *sql.DB {
    return DB
}

//func executeSQL(sql string) {
//    debugPrint("Executing query: " + sql)
//    return DB.Query(sql)
//}
