package database

import (
	"database/sql"
	"fmt"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/config"
	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/models"
)

// Database is the Database Abstraction Layer. It holds all the DB related methods
type Database struct {
	Conn *sql.DB
}

// NewMySQLConnection initliazes the DB connection using the parameters from the config file
func NewMySQLConnection(cfg *config.Config) (*Database, error) {
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
	return &Database{
		Conn: db,
	}, nil
}

// AddOpLog inserts an Oplog entry
func (d *Database) AddOpLog(shardID uint32, version uint32, taskName string, message string, stdout string, stderr string) error {
	res, err := d.Conn.Exec("INSERT INTO oplog (shardId, version, seq, taskName, message, output, err) "+
		"SELECT ?, ?, (SELECT COALESCE(MAX(seq),0)+1 FROM oplog WHERE shardId = 1 AND version=1),"+
		"?, ?, ?, ?", shardID, version, taskName, message, stdout, stderr)

	if err != nil {
		return errors.Wrap(err, "Can't insert a row  in the opLog table")
	}

	count, err := res.RowsAffected()
	if count != 1 {
		return errors.Wrap(err, fmt.Sprintf("Problem with the insert, RowsAffected = %d instead of 1", count))
	}
	return nil
}

// GetMaxVersion returns the highest current version
func (d *Database) GetMaxVersion() (uint32, error) {
	var version uint32

	query := "SELECT COALESCE(MAX(version),0) FROM versions"
	if err := d.Conn.QueryRow(query).Scan(&version); err != nil {
		return 0, errors.Wrap(err, "Did not get a max version value, really strange")
	}
	return version, nil
}

// GetNextVersion gets the Version object of the following version
func (d *Database) GetNextVersion(version uint32) (*models.Version, error) {
	var lVersion uint32

	query := "SELECT version FROM versions WHERE version > ? ORDER BY version LIMIT 1"
	if err := d.Conn.QueryRow(query, version).Scan(&lVersion); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("cannot get next version of %d", version))
	}

	return d.GetVersion(lVersion)
}

// GetVersion returns a Version struct of a given version
func (d *Database) GetVersion(version uint32) (*models.Version, error) {
	//Logger.Println("getVersion for version = " + strconv.Itoa(version))
	v := &models.Version{}
	query := "SELECT `version`, `command`, `tableName`, `cmdType`, `lastUpdate` FROM `versions` WHERE `version` = ?"
	if err := d.Conn.QueryRow(query, version).
		Scan(&v.Version, &v.Command, &v.TableName, &v.CmdType, &v.LastUpdate); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("cannot get version %d from the db", version))
	}

	return v, nil
}

// GetShard returns a Shard struc of for a given shardId
func (d *Database) GetShard(shardID uint32) (*models.Shard, error) {
	query := "SELECT shardId, schemaName, shardDSN, version, task, lastTaskHb, lastUpdate " +
		"FROM shards WHERE shardId = ?"
	s := &models.Shard{}
	err := d.Conn.QueryRow(query, shardID).Scan(&s.ShardId, &s.SchemaName, &s.ShardDSN,
		&s.Version, &s.TaskName, &s.LastTaskHb, &s.LastUpdate)

	if err != nil {
		return nil, err
	}

	return s, nil
}

// GetShardToUpgrade finds a shard that has a lower version and no taskName
// Should be called only by the dispatcher otherwise it needs a mutex
func (d *Database) GetShardToUpgrade(version uint32, taskName string) (*models.Shard, error) {
	var shardID uint32

	query := "SELECT shardId FROM shards WHERE version < ? AND taskName IS NULL ORDER BY lastUpdate LIMIT 1"
	err := d.Conn.QueryRow(query, version).Scan(&shardID)

	switch {
	case err == sql.ErrNoRows:
		//Logger.Println("Found no shards needing ddl")
		return nil, nil
	case err != nil:
		return nil, errors.Wrap(err, "unexpected error looking for shards")
	}

	updateQuery := "UPDATE shards SET taskName = ?, lastTaskHb = NOW() WHERE shardId = ?"
	_, err = d.Conn.Exec(updateQuery, taskName, shardID)
	if err != nil {
		return nil, errors.Wrap(err, "can't update the shards entry in the database")
	}

	return d.GetShard(shardID)
}

// UpdateShardTaskHeartbeat updates the lastTaskHb field for the shardId and provided the taskName matches
func (d *Database) UpdateShardTaskHeartbeat(shardID uint32, taskName string) error {
	query := "UPDATE shards SET lastTaskHb = NOW() WHERE taskName = ? AND shardId = ?"
	res, err := d.Conn.Exec(query, taskName, shardID)

	if err != nil {
		return errors.Wrap(err, "Can't update lastTaskHb of the shard in the database")
	}

	if count, err := res.RowsAffected(); err == nil && count != 1 {
		return fmt.Errorf("problem with the update, count = %d instead of 1", count)
	}
	return nil
}

// ShardUpgradeDone updates the shards object
func (d *Database) ShardUpgradeDone(shardID uint32, version uint32, taskName string) error {
	query := "UPDATE shards SET lastTaskHb = NOW(), VERSION = ?, taskNAme = NULL WHERE taskName = ? AND shardId = ?"
	res, err := d.Conn.Exec(query, taskName, shardID, version)

	if err != nil {
		return errors.Wrap(err, "can't mark the shard as upgraded in the database")
	}

	count, err := res.RowsAffected()
	if err == nil && count != 1 {
		return fmt.Errorf("problem with the update, count = %d instead of 1", count)
	}
	return nil
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
