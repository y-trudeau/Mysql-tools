#!/bin/bash
#
# should be called by cron at every hour

mysqlbinlog56=$(which mysqlbinlog)
mysqlclient=$(which mysql)
pidfile=/var/run/binlogstream.pid
binlogdir=/home/binlog_stream/binlogs
binlogprefix=mysqld-bin
mysqluser=percona
mysqlpass=P3rc0na!
mysqlmaster=10.2.2.20
daystokeep=10  # will in fact be one day more if compression is used
compress=1
streamer_slave_id=384596821

# Remove old files
/bin/find $binlogdir -ctime +$daystokeep | grep $binlogprefix | /usr/bin/xargs /bin/rm -f

# Compress files that are done, not the last one
if [ "$compress" -eq 1 ]; then
(
   flock -n 200 || exit 1 # no wait
   ls ${binlogdir}/${binlogprefix}.* | grep -v xz | head -n -1 | /usr/bin/xargs -r nice xz -T6
) 200>/var/run/binlogstream_compress.lock
fi

/bin/kill -0 `cat $pidfile`
if [ "$?" -eq "0" ]; then
   #echo "running"
   exit
else
   #echo "not running"
   cd $binlogdir
   lastfile=`ls $binlogprefix* | sort -n | tail -1`
   if [ -z "$lastfile" ]; then
      lastfile=`$mysqlclient -BN -u $mysqluser --password=$mysqluser --host=$mysqlmaster -e 'show master status;' 2> /dev/null | awk '{ print $1 }
'`
   fi
   $mysqlbinlog56 --stop-never-slave-server-id=$streamer_slave_id --read-from-remote-master=BINLOG-DUMP-NON-GTIDS --host=$mysqlmaster \
	   --raw --stop-never  -u $mysqluser --password=$mysqlpass $lastfile 2> /dev/null &
   echo -n $! > $pidfile
fi
