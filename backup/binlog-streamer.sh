#!/bin/bash
#
# should be called by cron at most every hour if sleepTime=0 is used
# or standalone with sleepTime > 0.  The standalone mode is useful
# in a docker container when this script is used as entrypoint.
#
# Env variables that can be used by the script:
#
# REPL_SOURCE: replication source where to connect, hostname or IP
# REPL_USER: replication user 
# REPL_PASSWORD: replication user password
# REPL_GTID: 1 if GTID is used, 0 otherwise
# REPL_SLEEP_TIME: a time in seconds 
# REPL_BINLOG_DIR: Where to store the binlog files
# REPL_BINLOG_PREFIX: The binlog file prefix
# REPL_DAYS_TO_KEEP: Then number of days to keep the binlog files
# REPL_BINLOG_COMPRESS: 1 will REPL_BINLOG_COMPRESS the completed binlog files with xz
# REPL_SLAVE_ID_default: The server ID used by the streamer, must be unique
#
# If you don't use the environment, set the corresponding default 
# values below

REPL_SOURCE_default=10.2.2.20
REPL_USER_default=repl
REPL_PASSWORD_default=replPass123
REPL_GTID_default=1
REPL_SLEEP_TIME_default=600 
REPL_BINLOG_DIR_default=/var/lib/binlogs
REPL_BINLOG_PREFIX_default=mysqld-bin
REPL_DAYS_TO_KEEP_default=30
REPL_BINLOG_COMPRESS_default=1
REPL_SLAVE_ID_default=384596821

# If unassigned, set the default values
: ${REPL_SOURCE=${REPL_SOURCE_default}}
: ${REPL_USER=${REPL_USER_default}}
: ${REPL_PASSWORD=${REPL_PASSWORD_default}}
: ${REPL_GTID=${REPL_GTID_default}}
: ${REPL_SLEEP_TIME=${REPL_SLEEP_TIME_default}}
: ${REPL_BINLOG_DIR=${REPL_BINLOG_DIR_default}}
: ${REPL_BINLOG_PREFIX=${REPL_BINLOG_PREFIX_default}}
: ${REPL_DAYS_TO_KEEP=${REPL_DAYS_TO_KEEP_default}}
: ${REPL_BINLOG_COMPRESS=${REPL_BINLOG_COMPRESS_default}}
: ${REPL_SLAVE_ID=${REPL_SLAVE_ID_default}}

mysqlbinlog56=$(which mysqlbinlog)
mysqlclient=$(which mysql)
pidfile=/var/run/binlogstream.pid

mkdir -p $REPL_BINLOG_DIR
cd $REPL_BINLOG_DIR

while true; do
  # Remove old files
  /bin/find -ctime +$REPL_DAYS_TO_KEEP | grep $REPL_BINLOG_PREFIX | /usr/bin/xargs /bin/rm -f

  # Compress files that are done, not the last one
  if [ "$REPL_BINLOG_COMPRESS" -eq 1 ]; then
    # Compression subshell, only one instance should run at a given time
    (
      flock -n 200 || exit 1 # no wait, exit subshell
      ls ${REPL_BINLOG_PREFIX}* | grep -v xz | head -n -1 | /usr/bin/xargs -r nice xz -T2
    ) 200>/var/run/binlogstream_REPL_BINLOG_COMPRESS.lock &
  fi

  /bin/kill -0 `cat $pidfile`
  if [ "$?" -eq "0" ]; then
    #echo "running"
    exit
  else
    #echo "not running"
    lastfile=`ls $REPL_BINLOG_PREFIX* | sort -n | tail -1`
    if [ -z "$lastfile" ]; then
      lastfile=`$mysqlclient -BN -u $REPL_USER --password=$REPL_PASSWORD --host=$REPL_SOURCE -e 'show master status;' 2> /dev/null | awk '{ print $1 }'`
    fi

    if [ "$REPL_GTID" -eq 1 ]; then
      $mysqlbinlog56 --stop-never-slave-server-id=$REPL_SLAVE_ID --read-from-remote-master=BINLOG-DUMP-GTIDS --host=$REPL_SOURCE \
	      --raw --stop-never  -u $REPL_USER --password=$REPL_PASSWORD $lastfile 2> /dev/null &
    else
      $mysqlbinlog56 --stop-never-slave-server-id=$REPL_SLAVE_ID --read-from-remote-master=BINLOG-DUMP-NON-GTIDS --host=$REPL_SOURCE \
	      --raw --stop-never  -u $REPL_USER --password=$REPL_PASSWORD $lastfile 2> /dev/null &
    fi
    echo -n $! > $pidfile
  fi

  if [ "$REPL_SLEEP_TIME" -gt 0]; then
    break;
  else
    sleep $REPL_SLEEP_TIME
  fi
done
