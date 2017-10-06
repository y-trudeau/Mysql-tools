package main 

import (
    "bytes"
    "log"
    "os"
    "strconv"
	"bufio"
	"strings"
    "container/list"
    "os/exec"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

var buf bytes.Buffer
var Logger = log.New(&buf, "shardSchema: ", log.Lshortfile)

type Task struct {
    name string
    shard Shard     // shard
    version Version   // Version to apply 
}

type MsgToWorker struct {
    msgType uint8   // message type, 1=new task, 2=status, 3=abort (only new task is implemented)
    task Task
}

type MsgFromWorker struct {
    msgType uint8 // message type, 0 = idle, 1=running, 2=done, 3=failed
    task Task
}

func main() {  
      
    Logger.Println("main started")
    
    configFile := os.Args[1] // only support one argument which is the configuration file
    
    cfg := getConfig()
    cfg.InitConfig(configFile)
    Logger.Println("Config has been initialized")
    
    // set the task prefix
    taskName := os.Hostname + ":" + strconv.Itoa(os.Getpid())

    // Create the list on ongoing task
    onGoing := list.New()
    
    // Create the channels to and from the workers
    submitMsg := make(chan MsgToWorker,5)  // do we need buffering?
    replyMsg := make(chan MsgFromWorker,5)  // do we need buffering?
    
    // For the signals
    sigs := make(chan os.Signal, 1)
    
    numWorkers := strconv.Atoi(cfg.getValue("MaxConcurrentDDL"))

    // Inspired from: https://gobyexample.com/worker-pools
    // Starting the workers
    for w := 1; w <= numWorkers; w++ {
        go worker(w, submitMsg, replyMsg)
    }    

    taskLimit := numWorkers
    newTaskLimit := numWorkers
    iteration := 0
    for {
        // just a generic loop counter
        iteration++
        
        // Let's read if the throttling file is present
        if _, err := os.Stat(cfg.getValue("throttlingFile")); !os.IsNotExist(err) {
            Logger.Println("throttlingFile exists")
            
            // Let's open it
            file := os.Open(cfg.getValue("throttlingFile"))
            Logger.Println("throttling file opened")
        
            // Let's read the first line and try to convert to int
            scanner := bufio.NewScanner(file)
            if scanner.Scan() {
                newTaskLimit, err = strconv.Atoi(scanner.Text())
                if err != nil {
                    newTaskLimit = taskLimit
                }
            }
            file.Close()
        }
            
        if newTaskLimit > numWorkers {
            Logger.Println("taskLimit set to " + strconv.Itoa(newTaskLimit) + 
                            "capping to numWorkers: " + strconv.Itoa(numWorkers))
            newTaskLimit = numWorkers
        }
        
        if newTaskLimit < 0 {
            Logger.Println("taskLimit can't be negative, setting it to 0")
            newTaskLimit = 0
        }
        
        taskLimit = newTaskLimit
        
        // Can we submit jobs?
        if onGoing.len() < taskLimit {
            
            //Yes, first we need the current highest version
            maxVersion := getMaxVersion()
            
            //then let's try to find a shard needing work
            shardToUpgrade := getShardToUpgrade(maxVersion,taskName)
            
            if shardToUpgrade != nil {
                // we have a shard!!!
                Logger.Println("Found shardId = " + strconv.Itoa(shardToUpgrade.shardId) + 
                " needing work")
                
                // What is the next version?
                nextVersion := getNextVersion(sharToUpgrade.version)
            
                newTask = Task{ name: taskName, shard: shardToUpgrade, version: nextVersion }
                
                onGoing.PushFront(newTask)
                
                // Now, build and send a message to the workers
                // this should never block since we never go beyond taskLimit
                submitMsg <- MsgToWorker{ msgType: 1, task: newTask }
                
            }
        }
        
        // listen and process messages
        gotTimeout := 0
        for gotTimeout < 1 {  // use gotTimeout to read all messages
            select {
                case rmsg := <-replyMsg: {
                    switch rmsg.msgType {
                    case 0:  { //idle  (no impleted yet)
                            Logger.Println("Received idle message")
                        }
                    case 1: { // running (no impleted yet)
                        // Update the Heartbeat field of shards
                        Logger.Println("Received running message")
                        updateShardTaskHeartbeat(rmsg.task.shard.shardId, taskName)
                        
                        }
                    case 2: {  // task done
                            // the task is done, update the shards table
                            shardUpgradeDone(rmsg.task.shard.shardId, 
                                rmsg.task.version.version, taskName)
                                
                            // and remove the task from the onGoing list
                            for e := onGoing.Front(); e != nil; e = e.Next() {
                                if e.shard.shardId == rmsg.task.shard.shardId && 
                                    e.version.version == rmsg.task.version.version {
                                      
                                    onGoing.Remove(e)
                                }
                            }
                        
                        }
                    case 3: {  // task failed
                            Logger.Println("Update of shard: " + 
                               strconv.Itoa(rmsg.task.shard.shardId) + ": to version " +
                               strconv.Itoa(rmsg.task.version.version) + 
                               "failed.  Look at the log table for more details")

                            // and remove the task from the onGoing list
                            for e := onGoing.Front(); e != nil; e = e.Next() {
                                if e.shard.shardId == rmsg.task.shard.shardId && 
                                    e.version.version == rmsg.task.version.version {
                                    onGoing.Remove(e)
                                }
                            }
                        }
                        
                    } 
                  }  
                case <-time.After(time.Second * 1): {
                    gotTimeout = 1
                }
            }
        }
    }                
}


func worker(id int, MsgIn <-chan MsgToWorker, MsgOut chan<- MsgFromWorker) {
        
    for {        
        select {
            case rmsg := <-MsgIn: {
                switch rmsg.msgType {
                    case 1:  { // new task, only type implemented so far
                        Logger.Println("Received a task:" + rmsg.task)

                        // start the work
                        switch rmsg.task.version.cmdType {
                            case "sql": {
                                
                                sqlddl := "alter table `" + rmsg.task.version.tableName + "` " + rmsg.task.version.command
                                addOpLog(rmsg.task.shard.shardId,rmsg.task.version.version,rmsg.task.name,
                                        "starting SQL command: '" + sql + "'","","")
                                        
                                dbddl, err := sql.Open("mysql", t.shard.shardDSN + "/" + t.shard.schemaName)
                                if err != nil {
                                    addOpLog(t.shard.shardId,t.version.version,t.name,
                                    "Error: shard db connection","",err)

                                    MsgOut <- MsgFromWorker{ msgType: 3, task: rmsg.task }  
                                } else {
                                    res, err := dbddl.Exec(sqlddl)
                                    if err != nil {
                                        addOpLog(t.shard.shardId,t.version.version,t.name,
                                                "Error: ddl error","",err)

                                        MsgOut <- MsgFromWorker{ msgType: 3, task: rmsg.task }
                                    } else {
                                        addOpLog(t.shard.shardId,t.version.version,t.name,
                                                "Completed OK","","")
                                        
                                        MsgOut <- MsgFromWorker{ msgType: 2, task: rmsg.task }
                                    }
                                }
                            }
                            
                            case "pt-osc": {
                                mysqlDSN, err := mysql.parseDSN(rmsg.task.shard.shardDSN)
                                if err != nil {
                                    addOpLog(rmsg.task.shard.shardId,rmsg.task.version.version,rmsg.task.name,
                                        "Error parsing the shard DSN: '" + rmsg.task.shard.shardDSN + "'","",err)
                                        
                                    MsgOut <- MsgFromWorker{ msgType: 3, task: rmsg.task }    
                                } else {
                                                                                
                                    cmdName := "pt-online-schema-change"
                                    cmdArgs := []string{"--execute","--alter","rmsg.task.version.command",
                                        "u=" + mysqlDSN.User + ",p=" + mysqlDSN.Passwd + ",D=" + 
                                            rmsg.task.shard.schemaName + ",t=" + rmsg.task.version.tableName }

                                    cmd := exec.Command(cmdName, cmdArgs...)
                                    cmdOut, err := cmd.StdoutPipe()
                                    cmdErr, err := cmd.StderrPipe()
                         
                                    var bout bytes.Buffer
                                    var berr bytes.Buffer

                                    go func() { 
                                        bout.ReadFrom(cmdOut)
                                    }()

                                    go func() { 
                                        berr.ReadFrom(cmdErr)
                                    }()

                                    addOpLog(rmsg.task.shard.shardId,rmsg.task.version.version,rmsg.task.name,
                                            "starting pt-osc command: " + cmdName + " with args: " + 
                                            strings.Join(cmdArgs," "),"","")
                                            
                                    err = cmd.Start()
                                    if err != nil {
                                        addOpLog(rmsg.task.shard.shardId,rmsg.task.version.version,rmsg.task.name,
                                            "Error: command: " + cmdName + " with args: [" + 
                                            strings.Join(cmdArgs," ") + "] failed to start","","")
                                            
                                        MsgOut <- MsgFromWorker{ msgType: 3, task: rmsg.task }
                                        
                                    } else {

                                        err = cmd.Wait()
                                        if err != nil {
                                            addOpLog(rmsg.task.shard.shardId,rmsg.task.version.version,rmsg.task.name,
                                            "Error: command: " + cmdName + " with args: [" + 
                                            strings.Join(cmdArgs," ") + "] failed",bout.String(),berr.String())
                                            
                                            MsgOut <- MsgFromWorker{ msgType: 3, task: rmsg.task }
                                            
                                        } else {
                                            addOpLog(rmsg.task.shard.shardId,rmsg.task.version.version,rmsg.task.name,
                                            "Completed OK",bout.String(),berr.String())
                                            
                                            MsgOut <- MsgFromWorker{ msgType: 2, task: rmsg.task }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }   
}

