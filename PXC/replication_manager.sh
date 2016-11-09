#!/bin/bash
#
# replication_manager.sh
#
# Description:  Manages master-master replication between 2 PXC clusters.
#
# Authors:  Yves Trudeau, Percona
#
# License:  GNU General Public License (GPL)
#
# (c) 2016 Percona
#
# vim: set tabstop=4 shiftwidth=4 expandtab
#
# Requires the following table in the percona schema:
#
# CREATE TABLE `replication` (
#   `host` varchar(40) NOT NULL,
#   `localIndex` int(11) DEFAULT NULL,
#   `isSlave` enum('No','Yes','Proposed','Failed') DEFAULT 'No',
#   `lastUpdate` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
#   `lastHeartbeat` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00',
#   `cluster` varchar(40) NOT NULL,
#   PRIMARY KEY (`host`,`cluster`)
# ) ENGINE=InnoDB DEFAULT CHARSET=latin1
#
# This script must be call every minutes by cron on every node that you 
# want to be a potential slave.  
# * * * * * /path/to/replication_manager.sh > /tmp/replication_manager.out
#
# The list of remote masters is defined by the variable: MASTERS_LIST  (see below)
#
# The mysql credentials needs to be in the .my.cnf file of the user under which
# the script run.  I needs SELECT, INSERT and UPDATE on percona.repliction 
# and SUPER on *.*
#
# Finally, the script requires GTID replication to be enabled.
#
DEBUG_LOG="/tmp/replication_manager.log"
if [ "${DEBUG_LOG}" -a -w "${DEBUG_LOG}" -a ! -L "${DEBUG_LOG}" ]; then
  exec 9>>"$DEBUG_LOG"
  exec 2>&9
  date >&9
  set -x
fi

#Global variables
FAILED_REPLICATION_TIMEOUT=179  # 3 times the cron interval minus 1s
MASTERS_LIST="172.29.110.132 172.29.110.133 172.29.110.134"
#MASTERS_LIST="172.29.78.132 172.29.78.133 172.29.78.134"
REPLICATION_CREDENTIALS="master_user='repl', master_password='6xF42CdmgWn', MASTER_AUTO_POSITION = 1"

# retrieve the global status and set variables
get_status_and_variables() {
    eval `mysql --connect_timeout=10 -BN -e 'show global status;show global variables;' 2> /tmp/mysql_error | tr '\t' '=' | sed -e 's/^\([^=]*\)=\(.*\)$/\1='"'"'\2'"'"'/g'`

    if [ "$(grep -c ERROR /tmp/mysql_error)" -gt 0 ]; then
        cat /tmp/mysql_error
        exit 1
    fi
}

# retrieve the slave status and set variables
get_slave_status() {
    eval `mysql --connect_timeout=10 -B -e 'show slave status\G' 2> /tmp/mysql_error | grep -v Last_IO | grep -v '\*\*\*\*' | sed -e 's/^\s*//g' -e 's/: /=/g' -e 's/\(.*\)=\(.*\)$/\1='"'"'\2'"'"'/g'`

    if [ "$(grep -c ERROR /tmp/mysql_error)" -gt 0 ]; then
        cat /tmp/mysql_error
        exit 1
    fi        
        
}

find_best_slave_candidate() {
    # we want the proposed if any, if not the lowest localIndex that has a valid lastHeartbeat
    mysql -BN -e "select host from percona.replication where cluster='$wsrep_cluster_name' and unix_timestamp(lastHeartbeat) > unix_timestamp() - $FAILED_REPLICATION_TIMEOUT order by localIndex limit 1;"
}

try_masters() {

    local masterOk=0
    for master in $MASTERS_LIST; do
        mysql -BN -e "stop slave; change master to master_host='$master', $REPLICATION_CREDENTIALS; start slave;"
        sleep 10  # Give some time for replication to settle
        get_slave_status
        stateOk=$(echo $Slave_IO_State | grep -ci connect) # anything with connect in Slave_IO_State is bad
        if [[ $Slave_IO_Running == "Yes" && $Slave_SQL_Running == "Yes" && $stateOk -eq 0 ]]; then
            masterOk=1
            break
        fi
    done
    echo $masterOk
}
setup_replication(){
    local myState
    
    # no slave are defined for the cluster
    CandidateSlave=$(find_best_slave_candidate)
    
    if [ "$CandidateSlave" == "$wsrep_node_name" ]; then
        #we are the best candidate
        myState=$(mysql -BN -e "select isSlave from percona.replication where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'")
        
        if [ "$myState" == "Proposed" ]; then
            # We are already at the proposed stated, we can setup replication
            
            masterOk=$(try_masters)
            if [ "$masterOk" -eq 1 ]; then
                # all good, let's proclaim we are the slave
                mysql -e "update percona.replication set isSlave='Yes', lastUpdate=now(), lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"
            else
                # this node failed to setup replication
                mysql -e "update percona.replication set isSlave='Failed', lastUpdate=now(), lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"
            fi
            
        elif [ "$myState" == "No" ]; then
            # update to Proposed, the use of the TRX and for update is to avoid a race condition.  The actual promotion to
            # slave will happen at the next call
            mysql -B -e "begin; select count(*) into @dummy from percona.replication where cluster = 'eadoc_dr_pxdbc_cluster' for update; select host into @hostproposed from percona.replication where cluster = '$wsrep_cluster_name' and isSlave = 'Proposed' and unix_timestamp(lastHeartbeat) > unix_timestamp() - $FAILED_REPLICATION_TIMEOUT;update percona.replication set isSlave='Proposed', localIndex=$wsrep_local_index, lastUpdate=now(), lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name' and host <> coalesce(@hostproposed,' ');commit;"    
        fi
    else
        # this node is not the best candidate for slave
        if [ "a$Master_Host" == "a" ]; then   # sanity check
            mysql -e "update percona.replication set isSlave='No', localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"    
        fi
    fi
    
}

get_status_and_variables
get_slave_status

if [[ $wsrep_cluster_status == 'Primary' && ( $wsrep_local_state -eq 4 \
    || $wsrep_local_state -eq 2 ) ]]; then
    # cluster is sane for this node

    myState=`mysql -BN -e "select isSlave from percona.replication where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name';"`
    slaveDefined=`mysql -BN -e "select concat(host,'|', unix_timestamp() - unix_timestamp(lastHeartbeat)) from percona.replication where isSlave='Yes' and cluster = '$wsrep_cluster_name' order by localIndex limit 1"`
    
    if [ "a$Master_Host" == "a" ]; then 
        # This node is not currently a slave
        
        if [ "a$myState" == "a" ]; then
            # no row in percona.replication for that node, must be added
            mysql -e "insert into percona.replication (host,cluster,localIndex,isSlave,lastUpdate,lastHeartbeat) Values ('$wsrep_node_name','$wsrep_cluster_name',$wsrep_local_index,'No',now(),now())" 
            myState=No
        elif [ "$myState" == "Failed" ]; then
            # Clear the failed state after twice the normal timeout
            mysql -e "update percona.replication set isSlave='No', localIndex=$wsrep_local_index, lastUpdate=now(), lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name' and unix_timestamp(lastUpdate) < unix_timestamp() - 2*$FAILED_REPLICATION_TIMEOUT"
        fi

        if [ "a$slaveDefined" == "a" ]; then
            # no slave are defined in the cluster
            setup_replication
        else
            # There is a slave defined
            lastHeartbeat=$(echo $slaveDefined | cut -d'|' -f2)
            slaveHost=$(echo $slaveDefined | cut -d'|' -f1)
            
            if [ "$lastHeartbeat" -gt "$FAILED_REPLICATION_TIMEOUT" ]; then
                # the current slave is not reporting, 
                mysql -e "update percona.replication set isSlave='No' where cluster = '$wsrep_cluster_name' and host = '$slaveHost'"
                # the current slave is not reporting
                setup_replication
            else
                # Slave is reporting, this is the sane path for a node that isn't the slave
                mysql -e "update percona.replication set isSlave='No', localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"                
            fi
        fi
    else
        # This node is a slave
        
        if [ "a$myState" == "a" ]; then
            # no row in percona.replication for that node
            
            if [ "a$slaveDefined" == "a" ]; then
                # no row in percona.replication, likely uninitialized and we are the slave
                mysql -e "insert into percona.replication (host,cluster,localIndex,isSlave,lastUpdate,lastHeartbeat) Values ('$wsrep_node_name','$wsrep_cluster_name',$wsrep_local_index,'Yes',now(),now())" 
            else
                # That could be problematic, another slave exists let's bail-out
                mysql -e "stop slave; reset slave all; insert into percona.replication (host,cluster,localIndex,isSlave,lastUpdate,lastHeartbeat) Values ('$wsrep_node_name','$wsrep_cluster_name',$wsrep_local_index,'No',now(),now())" 
            fi
        elif [ "$myState" == "Yes" ]; then
            # myState is defined
            if [[ $Slave_IO_Running == "Yes" && $Slave_SQL_Running == "Yes" ]]; then
                #replication is going ok
                mysql -e "update percona.replication set isSlave='Yes', localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"    
        
            else
                #replication is broken
                if [ "$Slave_SQL_Running" == "No" ]; then
                    # That's bad, replication failed, let's bailout
                    mysql -e "stop slave; reset slave all; update percona.replication set isSlave='Failed', localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'" 
                elif [[ $Slave_IO_Running != "Yes" && $Slave_SQL_Running == "Yes" ]]; then
                    # Looks like we cannot reach the master, let's try to reconnect
                    
                    masterOk=$(try_masters)
                    if [ "$masterOk" -eq 1 ]; then
                        # we succeeded reconnecting
                        mysql -e "update percona.replication set isSlave='Yes', localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"         
                    else
                        # We failed
                        mysql -e "stop slave; reset slave all; update percona.replication set isSlave='Failed', localIndex=$wsrep_local_index, ,lastUpdate=now(), lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"
                    fi
                fi
            fi
        elif [ "$myState" == "No" ]; then
            # We are not defined as a slave in the cluster but we are... bailout
            mysql -e "stop slave; reset slave all; update percona.replication localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"
        elif [ "$myState" == "Failed" ]; then
            # We have failed and we are a slave, this is abnormal, fix state to 'No' and bailout
            mysql -e "stop slave; reset slave all; update percona.replication set isSlave='No', localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"            
        elif [ "$myState" == "Proposed" ]; then
            # Sanity cleanup, if the node is a slave is still at Proposed, need to be updated
            mysql -e "update percona.replication set isSlave='Yes', localIndex=$wsrep_local_index, lastHeartbeat=now() where cluster = '$wsrep_cluster_name' and host = '$wsrep_node_name'"  
        fi
    fi
else
    # cluster node is not sane for this node

    if [ "a$Master_Host" != "a" ]; then 
        # This node is currently a slave, useless to update the table since the cluster is not sane
        mysql -e "stop slave; reset slave all;"
    fi

    # Nothing else to do
fi
