# Replication manager for PXC

This tool helps manage asynchronous replication for PXC. The existing galera communication layer is used for quorum and to exchange information between the nodes.  Messages are simply written to a shared table in the percona schema whose definition is:

    CREATE TABLE `replication` (
      `host` varchar(40) NOT NULL,
      `localIndex` int(11) DEFAULT NULL,
      `isSlave` enum('No','Yes','Proposed','Failed') DEFAULT 'No',
      `lastUpdate` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
      `lastHeartbeat` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
      `cluster` varchar(40) NOT NULL,
      PRIMARY KEY (`host`,`cluster`)
    ) ENGINE=InnoDB DEFAULT CHARSET=latin1

The table allow to determine if another node in the cluster is already acting as a slave and if it is reporting.  If no node in the cluster is a declaring itself a slave, the reporting node that has the lowest local index will propose itself as a slave and if ok, will then start slaving.  If the node that is the slave lost connection with its master, it will try to reconnect to the other potential masters, if more than one master is provided.  If it fails, it will report "Failed" so that another node in the cluster can try to become a slave.  The "Failed" state will clear up after three minutes. 

In order to use the script, perform the following tasks:

1. Setup replication between the PXC cluster and a remote master using GTIDs, the remote master can be another PXC cluster
2. Create the table percona.replication as above
3. Download the script to /usr/local/bin and make it executable by root (or another user)
4. Make sure root (or another user) has a .my.cnf file with credentials sufficient to manage replication.  SUPER on *.* and ALL on percona.replication should work.
5. Edit the scripts on each server, adjust the variable EMAIL if you want state changes to be reported, MASTERS_LIST should contains the list of the remote potential masters, REPLICATION_CREDENTIALS should be adjusted for the user used for replication.  You can also define or override these variables in the file "/usr/local/etc/replication_manager.cnf".
6. Once all is ready, start by enabling the cronjob for the script on the cluster node that is currently the acting as slave.  The cron job calls the script every minutes:

    crontab -l > /dev/null 2>/dev/null; 
    echo '* * * * * /usr/local/bin/replication_manager.sh > /tmp/replication_manager.out 2> /tmp/replication_manager.err' | crontab -

7. Wait a few minutes until you see in the percona.replication table that the host has an row inserted with isSlave set to 'Yes'.  If there is a problem, you can do "touch /tmp/replication_manager.log" to enable the script trace.  See if there is any errors.
8. Once the slave is reporting correctly, enable the cronjob on all the other cluster nodes.  They should all be listed in the table after a few minutes.

If you need to perform maintenance operation and you don't want the script to mess with what you are doing, touch the file "/tmp/replication_manager.off".  Be aware it fully disables the script only on the node it is defined.  If that node is the acting slave and another node in the cluster doesn't have the file touched, you'll end up with 2 slaves.  I recommend setting it up on all the nodes and when returning to normal, remove on the slave first and allow a few minutes for it to report before removing the file on the other nodes.

Notes: This setup has been tested between two PXC clusters, the full setup is essential a repetition of the one cluster setup apart from the creation of the percona.replication table.
