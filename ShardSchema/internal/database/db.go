package database

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
	"github.com/y-trudeau/Mysql-tools/ShardSchema/internal/models"
)

// Database is the Database Abstraction Layer. It holds all the DB related methods
type Database struct {
	Conn *sql.DB
}

// NewDatabase initliazes the DB connection using the parameters from the config file
func NewDatabase(conn *sql.DB) *Database {
	return &Database{Conn: conn}
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
	query := "SELECT shardId, schemaName, shardDSN, version, taskName, lastTaskHb, lastUpdate " +
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
