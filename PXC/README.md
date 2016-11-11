# Replication manager for PXC

This tool helps manage asynchronous replication between two separated PXC Clusters. The existing galera communication layer is used for quorum and to exchange information between the nodes.  Messages are simply written to a shared table in the percona schema whose definition is::

    CREATE TABLE `replication` (
      `host` varchar(40) NOT NULL,
      `localIndex` int(11) DEFAULT NULL,
      `isSlave` enum('No','Yes','Proposed','Failed') DEFAULT 'No',
      `lastUpdate` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
      `lastHeartbeat` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
      `cluster` varchar(40) NOT NULL,
      PRIMARY KEY (`host`,`cluster`)
    ) ENGINE=InnoDB DEFAULT CHARSET=latin1

The table allow to determine if another node is already a slave and if it is reporting.  If no node in the same cluster is a slave, the reporting node that has the lowest local index will propose itself and if ok, will then start slaving.  If a node is a slave and lost connection to its master, it will try to reconnect to the other potential master of the remote cluster.  If it fails, it will report "Failed" so that another node can try to become a slave.  The "Failed" state will clear up after three minutes. 

In order to use the script, perform the following tasks:

1. Setup master-master between the 2 PXC clusters with GTIDs
2. Create the table percona.replicatoin
3. Download the script to /usr/local/bin and make it executable by root (or another user)
4. Make sure root (or another user) has a .my.cnf file with credentials sufficient to manage replication.  SUPER and ALL on percona.replication should work.
5. Edit the scripts on each server, adjust the variable EMAIL if you want state changes to be reported, MASTERS_LIST should contains the list of the remote potential master, REPLICATION_CREDENTIALS should be adjusted for the user used for replication.
6. Once all is ready, start by enabling the cronjob for the script on the nodes that are the acting slaves.  The cron job must call the script every minutes.

cronjob -l > /dev/null 2>/dev/null; echo '* * * * * /usr/local/bin/replication_manager.sh > /tmp/replication_manager.out 2> /tmp/replication_manager.err' | crontab -

7. Wait a few minutes until you see in the percona.replication table that these hosts have isSlave set to 'Yes'.  If there is a problem, you can do "touch /tmp/replication_manager.log" to enable the script trace.  See if you see any errors.

8. Once the slaves are reporting correctly, enable the cronjob on all the other servers.  They should all be listed in the table after a few minutes.


