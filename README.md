# Mysql-tools
My collection of tools for MySQL

backup/binlog-streamer.sh
-------------------------

Simple binlog streamer script handling compression and retention of the binlog files.  Needs to be called by crond every hours.

binlogRow2SQL.pl
----------------

Tool to convert binlog row format to SQL, useful to applied incompatible binlog files like from 5.7 to 5.5.  Ugly SQL but works.  Not everything works, Load data infile is not supported.


