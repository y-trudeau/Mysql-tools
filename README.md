# Mysql-tools
My collection of tools for MySQL

backup/binlog-streamer.sh
-------------------------

Simple binlog streamer script handling compression and retention of the binlog files.  Needs to be called by crond every hours.

binlogRow2SQL.pl
----------------

Tool to convert binlog row format to SQL, useful to applied incompatible binlog files like from 5.7 to 5.5.  Ugly SQL but works.  Not everything works, Load data infile is not supported.

PXC/replication_manager.sh
--------------------------

The replication manager is a tool to help manage the MySQL asynchronous replication links between galera based clusters.  The script elects one slave per replication link defined and use the galera cluster to ensure quorum.  It has been tested mostly with Percona XtraDB cluster but it seems to also work fine with MariaDB 10.1.14+. It allows for arbitrarily complex replication topologies but the usual constraints regarding master to master topologies apply, your writes must be replication compatible. GTIDs are required.

PXC/slave_manager.sh
--------------------

A simplified replication manager script handling the replication link of an isolated slave having a cluster as master.

ShardSchema
-----------

A tool to manage a large number of similar schema, still in development. Progress is currently very slow because of shifting priorities and lack of time.
