package main

import (
	"bufio"
	"bytes"
	"container/list"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/config"
	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/database"
	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/models"
)

var buf bytes.Buffer
var Logger = log.New(&buf, "shardSchema: ", log.Lshortfile)

type Task struct {
	name    string
	shard   *models.Shard   // shard
	version *models.Version // Version to apply
}

func (t *Task) String() string {
	return fmt.Sprintf("%s, %v", t.name, t.version)
}

type MsgToWorker struct {
	msgType uint8 // message type, 1=new task, 2=status, 3=abort (only new task is implemented)
	task    Task
}

type MsgFromWorker struct {
	msgType uint8 // message type, 0 = idle, 1=running, 2=done, 3=failed
	task    Task
}

func main() {
	Logger.Println("main started")

	if len(os.Args) < 2 {
		fmt.Println("must indicate the config file name")
		os.Exit(1)
	}
	configFile := os.Args[1] // only support one argument which is the configuration file

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Printf("cannot load config: %s", err)
		os.Exit(1)
	}

	conn, err := getDBConnection(cfg)
	if err != nil {
		log.Printf("cannot connect to the db: %s", err)
		os.Exit(1)
	}
	db := database.NewDatabase(conn)

	// set the task prefix
	hostname, _ := os.Hostname()
	taskName := fmt.Sprintf("%s:%06d", hostname, os.Getpid())

	// Create the list on ongoing task
	onGoing := list.New()

	// Create the channels to and from the workers
	submitMsg := make(chan MsgToWorker, 5)  // do we need buffering?
	replyMsg := make(chan MsgFromWorker, 5) // do we need buffering?

	// For the signals
	//sigs := make(chan os.Signal, 1)

	numWorkers := cfg.MaxConcurrentDDL

	// Inspired from: https://gobyexample.com/worker-pools
	// Starting the workers
	for w := 1; w <= numWorkers; w++ {
		go worker(db, w, submitMsg, replyMsg)
	}

	taskLimit := numWorkers
	newTaskLimit := numWorkers
	iteration := 0
	for {
		// just a generic loop counter
		iteration++

		// Let's read if the throttling file is present
		if _, err := os.Stat(cfg.ThrottlingFile); !os.IsNotExist(err) {
			Logger.Println("throttlingFile exists")

			// Let's open it
			file, err := os.Open(cfg.ThrottlingFile)
			if err != nil {
				break
			}

			Logger.Println("throttling file opened")

			// Let's read the first line and try to convert to int
			scanner := bufio.NewScanner(file)
			if scanner.Scan() {
				newTaskLimit, err = strconv.Atoi(scanner.Text())
				if err != nil {
					taskLimit = newTaskLimit
				}
			}
			file.Close()
		}

		if newTaskLimit > numWorkers {
			Logger.Printf("taskLimit set to %d. capping to numWorkers: %d\n", newTaskLimit, numWorkers)
			newTaskLimit = numWorkers
		}

		if newTaskLimit < 0 {
			Logger.Println("taskLimit can't be negative, setting it to 0")
			newTaskLimit = 0
		}

		taskLimit = newTaskLimit

		// Can we submit jobs?
		if onGoing.Len() < taskLimit {

			//Yes, first we need the current highest version
			maxVersion, _ := db.GetMaxVersion()

			//then let's try to find a shard needing work
			shardToUpgrade, _ := db.GetShardToUpgrade(maxVersion, taskName)

			if shardToUpgrade != nil {
				// we have a shard!!!
				Logger.Printf("Found shardId = %d needing work\n", shardToUpgrade.ShardId)

				// What is the next version?
				nextVersion, err := db.GetNextVersion(shardToUpgrade.Version)
				if err != nil {
					// TODO handle the error
				}

				newTask := Task{name: taskName, shard: shardToUpgrade, version: nextVersion}

				onGoing.PushFront(newTask)

				// Now, build and send a message to the workers
				// this should never block since we never go beyond taskLimit
				submitMsg <- MsgToWorker{msgType: 1, task: newTask}

			}
		}

		// listen and process messages
		gotTimeout := 0
		for gotTimeout < 1 { // use gotTimeout to read all messages
			select {
			case rmsg := <-replyMsg:
				{
					switch rmsg.msgType {
					case 0:
						{ //idle  (no impleted yet)
							Logger.Println("Received idle message")
						}
					case 1:
						{ // running (no impleted yet)
							// Update the Heartbeat field of shards
							Logger.Println("Received running message")
							db.UpdateShardTaskHeartbeat(rmsg.task.shard.ShardId, taskName)

						}
					case 2:
						{ // task done
							// the task is done, update the shards table
							db.ShardUpgradeDone(rmsg.task.shard.ShardId,
								rmsg.task.version.Version, taskName)

							// and remove the task from the onGoing list
							for e := onGoing.Front(); e != nil; e = e.Next() {
								if e.Value.(models.Shard).ShardId == rmsg.task.shard.ShardId &&
									e.Value.(models.Shard).Version == rmsg.task.version.Version {
									onGoing.Remove(e)
								}
							}

						}
					case 3:
						{ // task failed
							Logger.Printf("Update of shard: %d to version %d failed. Look at the log table for more details",
								rmsg.task.shard.ShardId, rmsg.task.version.Version)

							// and remove the task from the onGoing list
							for e := onGoing.Front(); e != nil; e = e.Next() {
								if e.Value.(models.Shard).ShardId == rmsg.task.shard.ShardId &&
									e.Value.(models.Shard).Version == rmsg.task.version.Version {
									onGoing.Remove(e)
								}
							}
						}

					}
				}
			case <-time.After(time.Second * 1):
				{
					gotTimeout = 1
				}
			}
		}
	}
}

func worker(db *database.Database, id int, MsgIn <-chan MsgToWorker, MsgOut chan<- MsgFromWorker) {

	for {
		select {
		case rmsg := <-MsgIn:
			{
				switch rmsg.msgType {
				case 1:
					{ // new task, only type implemented so far
						Logger.Printf("Received a task: %+v\n", rmsg.task)

						// start the work
						switch rmsg.task.version.CmdType {
						case "sql":
							{

								sqlddl := "alter table `" + rmsg.task.version.TableName + "` " + rmsg.task.version.Command
								db.AddOpLog(rmsg.task.shard.ShardId, rmsg.task.version.Version, rmsg.task.name,
									"starting SQL command: '"+sqlddl+"'", "", "")

								//dbddl, err := sql.Open("mysql", t.shard.shardDSN+"/"+t.shard.schemaName)
								//if err != nil {
								//	addOpLog(t.shard.shardId, t.version.version, t.name,
								//		"Error: shard db connection", "", err)

								//	MsgOut <- MsgFromWorker{msgType: 3, task: rmsg.task}
								//} else {
								//	res, err := dbddl.Exec(sqlddl)
								//	if err != nil {
								//		addOpLog(t.shard.shardId, t.version.version, t.name,
								//			"Error: ddl error", "", err)

								//		MsgOut <- MsgFromWorker{msgType: 3, task: rmsg.task}
								//	} else {
								//		addOpLog(t.shard.shardId, t.version.version, t.name,
								//			"Completed OK", "", "")

								//		MsgOut <- MsgFromWorker{msgType: 2, task: rmsg.task}
								//	}
								//}
							}

						case "pt-osc":
							{
								mysqlDSN, err := mysql.ParseDSN(rmsg.task.shard.ShardDSN)
								if err != nil {
									db.AddOpLog(rmsg.task.shard.ShardId, rmsg.task.version.Version, rmsg.task.name,
										"Error parsing the shard DSN: '"+rmsg.task.shard.ShardDSN+"'", "", err.Error())

									MsgOut <- MsgFromWorker{msgType: 3, task: rmsg.task}
								} else {

									cmdName := "pt-online-schema-change"
									cmdArgs := []string{"--execute", "--alter", "rmsg.task.version.command",
										"u=" + mysqlDSN.User + ",p=" + mysqlDSN.Passwd + ",D=" +
											rmsg.task.shard.SchemaName + ",t=" + rmsg.task.version.TableName}

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

									db.AddOpLog(rmsg.task.shard.ShardId, rmsg.task.version.Version, rmsg.task.name,
										"starting pt-osc command: "+cmdName+" with args: "+
											strings.Join(cmdArgs, " "), "", "")

									err = cmd.Start()
									if err != nil {
										db.AddOpLog(rmsg.task.shard.ShardId, rmsg.task.version.Version, rmsg.task.name,
											"Error: command: "+cmdName+" with args: ["+
												strings.Join(cmdArgs, " ")+"] failed to start", "", "")

										MsgOut <- MsgFromWorker{msgType: 3, task: rmsg.task}

									} else {

										err = cmd.Wait()
										if err != nil {
											db.AddOpLog(rmsg.task.shard.ShardId, rmsg.task.version.Version, rmsg.task.name,
												"Error: command: "+cmdName+" with args: ["+
													strings.Join(cmdArgs, " ")+"] failed", bout.String(), berr.String())

											MsgOut <- MsgFromWorker{msgType: 3, task: rmsg.task}

										} else {
											db.AddOpLog(rmsg.task.shard.ShardId, rmsg.task.version.Version, rmsg.task.name,
												"Completed OK", bout.String(), berr.String())

											MsgOut <- MsgFromWorker{msgType: 2, task: rmsg.task}
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

func getDBConnection(cfg *config.Config) (*sql.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cannot initialize the DB connection (nil config)")
	}

	db, err := sql.Open("mysql", buildDSN(cfg))
	if err != nil {
		// logging fatal errors is not a good idea because some testing frameworks will hide this
		// and make it harder to debug. The best is to return an error or nil if there is no error
		// and make the logging in the caller.
		// I am using an external "errors" package that wraps the error along with a custom message.
		// This way you don't lose the original error value in case you need to get some extra info
		// from it.
		return nil, errors.Wrap(err, "cannot open db connection to ")
	}
	// This would close the connection at the end of this function so it would
	// be closed when you want to use it.
	// defer db.Close()

	// Open doesn't really open a connection. Validate DSN data:
	err = db.Ping()
	if err != nil {
		return nil, errors.Wrap(err, "db connection is not active")
	}
	return db, nil
}

func buildDSN(cfg *config.Config) string {
	dsnCfg := mysql.NewConfig() // Load defaults from mysql pkg

	dsnCfg.Addr = cfg.Host
	dsnCfg.User = cfg.User
	dsnCfg.Passwd = cfg.Password
	dsnCfg.DBName = cfg.DBName
	dsnCfg.ParseTime = true
	//dsnCfg.InterpolateParams = true
	dsnCfg.Net = "tcp"
	if cfg.Host == "localhost" {
		dsnCfg.Net = "unix"
	}

	return dsnCfg.FormatDSN()
}
