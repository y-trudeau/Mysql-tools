# Replication manager for PXC

This tool helps manage asynchronous replication between PXC clusters. The typical use case would be to manager a master-master replication link between two distincts PXC clusters but the tools supports more complex topology.  

In each cluster, any node can be the slave to another cluster and that slave can point to any of the remote nodes for its master.  The existing galera communication layer is used within a cluster for quorum and to exchange information between the nodes.  Messages are simply written to a shared table in the percona schema.  This allows a node to determine if another node in the cluster is already acting as a slave and if it is reporting correctly.  If no node in the cluster is a declaring itself a slave for a given replication link, the reporting node that has the lowest local index will propose itself as a slave and if not contested, will then start slaving.  When a given cluster needs to be slave for more than one remote cluster, it is possible to distribute the slaves across the nodes, this is the default behavior. If you don't what to distribute the slaves, you need to set the variable "DISTRIBUTE_SLAVE" to 0 in the script header. If the node that is the slave loses connection with its master, it will try to reconnect to the other potential masters, if more than one master is provided.  If it fails, it will report "Failed" so that another node in the cluster can try to become a slave.  The "Failed" state will clear up after three minutes. 

**NOTE**: Such setup can easily cause replication conflicts, make sure your schema and queries are resilient.  Compound primary keys are your friends.

## Deployment

Because of the subtle relationship between PXC galera replication and GTID replication, deploying this script in production involves more steps than one could think of a simple solution managing replication.   This solution works only with GTID based replication.  Furthermore, only Oracle based GTID implementation works. The way MariaDB works with domain-id and server-id just breaks replication all the time.  Within a PXC cluster, there must be a single GTID sequence.  If you enabled GTID after the PXC cluster is up, you'll need to shutdown MySQL on all nodes except one and force SST at restart by removing the files in the datadir. 

In the following steps, we'll assume the goal is to deploy and master-master replication links between three data-centers: DC1, DC2 and DC3.  In each DC, there are 3 PXC nodes forming distincts Galera clusters.  In DC1 the 3 nodes are DC1-1, DC1-2 and DC1-3.  The nodes in the other DCs are similarly labeled.  The goal is to have the following topology:

    DC2 <=> DC1 <=> DC3

DC1 replicates (is a slave) of DC2 and DC3.  DC2 and DC3 are slaves of DC1.  Let's start by the configuration of DC1.  The minimal MySQL configuration file common to all three DC1 nodes is:


    [mysqld]
    # General galera reqs
    default_storage_engine=InnoDB
    innodb_autoinc_lock_mode=2
    
    # Replication settings
    binlog_format=ROW
    server-id=1
    log-bin=mysql-bin
    log_slave_updates
    expire_logs_days=7
    gtid_mode = ON
    enforce_gtid_consistency=ON
    master_info_repository = TABLE
    relay_log_info_repository = TABLE
        
    # Galera configuration
    wsrep_provider=/usr/lib/galera3/libgalera_smm.so
    wsrep_cluster_address=gcomm://10.0.4.160,10.0.4.162,10.0.4.163
    wsrep_slave_threads= 2
    wsrep_log_conflicts
    wsrep_cluster_name=DC1
    pxc_strict_mode=ENFORCING
    wsrep_sst_method=xtrabackup-v2
    wsrep_sst_auth="root:root"

All nodes will have the same server-id value and the repositories are set to "TABLE" because the multi-source replication syntax will be used since a given node could end up being the slave of more than one remote cluster.  We assume the user "root@localhost" exists with the password "root".  The first step is to bootstrap the cluster on node DC1-1:

    [root@DC1-1 ~]# /etc/init.d/mysql stop 
    [root@DC1-1 ~]# /etc/init.d/mysql bootstrap-pxc

Then, on DC1-2, start MySQL after having deleted the content of the datadir in order to force a SST:

    [root@DC1-2 ~]# /etc/init.d/mysql stop
    [root@DC1-2 ~]# rm -rf /var/lib/mysql/*
    [root@DC1-2 ~]# /etc/init.d/mysql start
    
Once the SST of DC1-2 is completed, proceed on DC1-3:

    [root@DC1-3 ~]# /etc/init.d/mysql stop
    [root@DC1-3 ~]# rm -rf /var/lib/mysql/*
    [root@DC1-3 ~]# /etc/init.d/mysql start
    
At this point, the cluster DC1 is using a single GTID sequence.  To make sure GTID_PURGED is at the same, on all nodes do:

    mysql> flush logs;
    mysql> purge master logs to 'mysql-bin.000003';

where 'mysql-bin.000003' was the last file returned from *show master logs;*. At this point, we have the first cluster ready and you can  setup the clusters DC2 and DC3 similarly. Do not forget to adjust *server-id*, *wsrep_cluster_address* and *wsrep_cluster_name*.

You can start using the database and adding grants in DC1 but do not touch DC2 and DC3 yet. Add all the grants for replication between all nodes.  The following steps will assume there is this user defined:

    GRANT REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO 'repl'@'%' identified by 'replpass';
    
At this we can complete the part of the configuration stored in the database.  First, let's create the tables the replication manager need.  Let's create them on DC1-1:

    mysql> create database if not exists percona;
    mysql> CREATE TABLE `replication` (
      `host` varchar(40) NOT NULL,
      `localIndex` int(11) DEFAULT NULL,
      `isSlave` enum('No','Yes','Proposed','Failed') DEFAULT 'No',
      `lastUpdate` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
      `lastHeartbeat` timestamp NOT NULL DEFAULT '1970-01-01 00:00:00',
      `connectionName` varchar(64) NOT NULL,
      PRIMARY KEY (`connectionName`,`host`),
      KEY `idx_host` (`host`)
    ) ENGINE=InnoDB DEFAULT CHARSET=latin1;
    mysql> CREATE TABLE `cluster` (
      `cluster` varchar(31) NOT NULL,
      `masterCandidates` varchar(255) NOT NULL,
      `replCreds` varchar(255) NOT NULL,
      PRIMARY KEY (`cluster`)
    ) ENGINE=InnoDB DEFAULT CHARSET=latin1;
    mysql> CREATE TABLE `link` (
      `clusterSlave` varchar(31) NOT NULL,
      `clusterMaster` varchar(31) NOT NULL,
      PRIMARY KEY (`clusterSlave`,`clusterMaster`)
    ) ENGINE=InnoDB DEFAULT CHARSET=latin1;
    
The *replication* table will be written to by the tool, nothing needs to be inserted in that table.  The *cluster* table contains the details of each clusters.  In our case let's define our 3 clusters:

    INSERT INTO `cluster` VALUES ('DC1','10.0.4.160 10.0.4.162 10.0.4.163','master_user=\'repl\', master_password=\'replpass\'');
    INSERT INTO `cluster` VALUES ('DC2','10.0.4.164 10.0.4.165 10.0.4.166','master_user=\'repl\', master_password=\'replpass\'');
    INSERT INTO `cluster` VALUES ('DC3','10.0.4.167 10.0.4.168 10.0.4.169','master_user=\'repl\', master_password=\'replpass\'');

and the links we want:

    INSERT INTO `link` VALUES ('DC1','DC2');
    INSERT INTO `link` VALUES ('DC1','DC3');
    INSERT INTO `link` VALUES ('DC2','DC1');
    INSERT INTO `link` VALUES ('DC3','DC1');


We will not provisiong the remote clusters and start replication. On one of the DC1 node, for example DC1-1, perform a mysqldump with:

    [root@DC1-1 ~]# mysqldump -u root -p --master-data=2 --single-transaction -R -A -E > dump.sql

You can compress the file if it is too large.  Copy the backup file to one node in each remote clusters, for example to DC2-1 and DC3-1.  Restore the dump with:

    [root@DC2-1 ~]# mysql -u root -p < dump.sql
    
and:

    [root@DC3-1 ~]# mysql -u root -p < dump.sql

Now we can start configuring replication.  The first replication links have to be setup manually.  On DC2-1 do:

    mysql> change master to master_host='WAN IP of DC1-1', master_user='repl', master_password='replpass', MASTER_AUTO_POSITION = 1 for channel 'DC2-DC1';
    mysql> start slave for channel 'DC2-DC1';
    
Similarly, on DC3-1 do:

    mysql> change master to master_host='WAN IP of DC1-1', master_user='repl', master_password='replpass', MASTER_AUTO_POSITION = 1 for channel 'DC3-DC1';
    mysql> start slave for channel 'DC3-DC1';

For the other direction, we'll use DC1-1 for both:

    mysql> change master to master_host='WAN IP of DC2-1', master_user='repl', master_password='replpass', MASTER_AUTO_POSITION = 1 for channel 'DC1-DC2';
    mysql> start slave for channel 'DC1-DC2';
    mysql> change master to master_host='WAN IP of DC3-1', master_user='repl', master_password='replpass', MASTER_AUTO_POSITION = 1 for channel 'DC1-DC3';
    mysql> start slave for channel 'DC1-DC3';


Now, we have all the clusters linked in a master to master way.  You can try some writes and look at the GTID_EXECUTED sequence on all nodes, it should be very similar with 3 UUID sequences, one per cluster.  It is time to pull in the *replication_manager.sh* script.  On each node, perform the following steps:

    # cd /usr/local/bin
    # wget https://github.com/y-trudeau/Mysql-tools/raw/multi-source/PXC/replication_manager.sh
    # chmod u+x replication_manager.sh

When executed for the first time, the replication manager will detect the current replication links and insert rows in the *percona.replication* table.  In order to avoid problems, we'll start by the nodes that are already slaves.  On these nodes (DC1-1, DC2-1 and DC3-1), execute the script manually once (remember you need the mysql credentials in /root/.my.cnf):

    # /usr/local/bin/replication_manager.sh
    
The replication state should be unchanged and the *percona.replication* table should have the following rows:

    mysql> select * from percona.replication;
    +-------+------------+---------+---------------------+---------------------+----------------+
    | host  | localIndex | isSlave | lastUpdate          | lastHeartbeat       | connectionName |
    +-------+------------+---------+---------------------+---------------------+----------------+
    | DC1-1 |          1 | Yes     | 2017-06-30 13:03:01 | 2017-06-30 13:03:01 | DC1-DC2        |
    | DC1-1 |          1 | Yes     | 2017-06-30 13:03:01 | 2017-06-30 13:03:01 | DC1-DC3        |
    | DC2-1 |          1 | Yes     | 2017-06-30 13:03:01 | 2017-06-30 13:03:01 | DC2-DC1        |
    | DC3-1 |          1 | Yes     | 2017-06-30 13:03:01 | 2017-06-30 13:03:01 | DC3-DC1        |
    +-------+------------+---------+---------------------+---------------------+----------------+
    12 rows in set (0.00 sec)

That is the sane behavior.  If you don't get this, go to the *Debugging* section below.  On these same nodes, enable the cron job:

    * * * * * /usr/local/bin/replication_manager.sh 

Let a least one minute pass then proceed with the other nodes.  You can try a manual run first, see if the script added a line to the replication table for the host, likely with isSlave = No, and then add the cron jobs.  In my test setup, the end result is:

    mysql> select * from percona.replication;
    +-------+------------+---------+---------------------+---------------------+----------------+
    | host  | localIndex | isSlave | lastUpdate          | lastHeartbeat       | connectionName |
    +-------+------------+---------+---------------------+---------------------+----------------+
    | DC1-1 |          1 | Yes     | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC1-DC2        |
    | DC1-2 |          2 | No      | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC1-DC2        |
    | DC1-3 |          0 | No      | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC1-DC2        |
    | DC1-1 |          1 | Yes     | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC1-DC3        |
    | DC1-2 |          2 | No      | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC1-DC3        |
    | DC1-3 |          0 | No      | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC1-DC3        |
    | DC2-1 |          1 | Yes     | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC2-DC1        |
    | DC2-2 |          0 | No      | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC2-DC1        |
    | DC2-3 |          2 | No      | 2017-06-19 15:58:01 | 2017-06-19 15:58:01 | DC2-DC1        |
    | DC3-1 |          1 | Yes     | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC3-DC1        |
    | DC3-2 |          2 | No      | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC3-DC1        |
    | DC3-3 |          0 | No      | 2017-06-30 13:13:01 | 2017-06-30 13:13:01 | DC3-DC1        |
    +-------+------------+---------+---------------------+---------------------+----------------+


## Debugging

The script outputs its trace (bash -x) to the file "/tmp/replication_manager.log" if present.  If there is an error during the manual invocation or something unexpected is happening, touch the file, run the script manually and look at the file content for hints.  If you think there is a bug, I invite you to fill an issue on github:

    https://github.com/y-trudeau/Mysql-tools/issues/new
