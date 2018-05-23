#!/bin/bash
#
# slave_manager.sh
#
# Description:  Manages a slave of a PXC cluster.
#
# Authors:  Yves Trudeau, Percona
#
# License:  GNU General Public License (GPL)
#
# (c) 2018 Percona
#
# vim: set tabstop=4 shiftwidth=4 expandtab
#
#
# The script requires the use of GTIDs (Oracle implementation)
#
# This script must be call every minutes by cron on every node that you 
# want to be a potential slave.  
# * * * * * /path/to/slave_manager.sh > /tmp/slave_manager.out
#
# The local credentials must be in the ~/.my.cnf file of the user under which the
# cron from will be defined.
#
# You must set the variable masterCandidates to the list of the PXC cluster nodes
# that can act as masters.
#
# You must set the variable replCreds to an actual database user that can connect
# from the slave with the replication slave privilege
#
# If you need to debug the script, just do "touch /tmp/slave_manager.log" and the file
# will start to accumulate the bash trace
#
# You can suspend the execution of the script with "touch /tmp/slave_manager.off". To 
# resume normal operation, do "rm -f /tmp/slave_manager.off"
#
# The script can send emails when there is a change to the address defined in the EMAIL 
# variable.
#
# WARNING: although simple, as of May 2018 this script is untested
#
DEBUG_LOG="/tmp/slave_manager.log"
if [ "${DEBUG_LOG}" -a -w "${DEBUG_LOG}" -a ! -L "${DEBUG_LOG}" ]; then
	exec 9>>"$DEBUG_LOG"
	exec 2>&9
	date >&9
	set -x
fi

masterCandidates="10.1.1.10 10.1.1.11 10.1.1.12"
replCreds="master_user='repl', master_password='replpass'"

if [ -f /tmp/slave_manager.off ]; then
	echo "/tmp/slave_manager.off exists, exiting now"
	exit
fi

MYSQL="`which mysql` --connect_timeout=10 -B"

# retrieve the slave status and set variables
get_slave_status() {
	eval `$MYSQL -e "show slave status\G" 2> /tmp/mysql_error | grep -v Last \
		| grep -v '\*\*\*\*' \| sed -e ':a' -e 'N' -e '$!ba' -e 's/,\n/, /g' \
		| sed -e 's/^\s*//g' -e 's/: /=/g' -e 's/\(.*\)=\(.*\)$/\1='"'"'\2'"'"'/g'`

	if [ "$(grep -c ERROR /tmp/mysql_error)" -gt 0 ]; then
		cat /tmp/mysql_error
		exit 1
	fi        
}

send_email() {
Mailer=$(which mail)
if [[ ${#EMAIL} -gt 0 && ${#Mailer} -gt 0 ]]; then
	echo "$1" | $Mailer -s "$2" $EMAIL
fi
}

is_replication_ok() {
	local stateOk
	
	get_slave_status
	stateOk=$(echo $Slave_IO_State | grep -ci connect) # anything with connect in Slave_IO_State is bad
	if [[ $Slave_IO_Running == "Yes" && $Slave_SQL_Running == "Yes" && $stateOk -eq 0 ]]; then
		echo 1
	else
		echo 0
	fi
}

setup_replication() {
	local masterOk=0
	
	for master in $masterCandidates; do
		$MYSQL -N -e "stop slave;"
		
		$MYSQL -N -e "
			change master to master_host='${master}', ${replCreds}, MASTER_AUTO_POSITION = 1; 
			start slave;"
		
		sleep 10  # Give some time for replication to settle
		
		if [ "$(is_replication_ok)" -eq 1 ]; then
			send_email "Slave $hostname master is now $master" "New slave"  
			masterOk=1
			break
		fi
	done
	echo $masterOk
}

get_slave_status 

if [ "$(is_replication_ok)" -eq 1 ]; then
	# This node is not currently a slave or replication is having issues
	
	# Let's try to setup replication
	if [ "$(setup_replication)" -eq 1 ]; then
		exit 0
	else
		exit 1
	fi
fi
