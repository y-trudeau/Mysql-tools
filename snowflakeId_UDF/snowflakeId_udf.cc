/* This file implements a Snowflake ID generator for MySQL as per:
 * https://en.wikipedia.org/wiki/Snowflake_ID
 *
 * The function never returns NULL, even when you give it NULL arguments.
 *
 * To compile and install, best is to download the source tree and 
 * extract the files in the directory plugin/snowflakeId. cmake will 
 * add the rules to build it automatically.
 *
 * Once compiled and installed:
 *
 * mysql> CREATE FUNCTION snowflakeId RETURNS INTEGER SONAME 'libsnowflakeid_udf.so';
 *
 * Then:
 * mysql> select snowflakeId(@@server_id);
 * +--------------------------+
 * | snowflakeId(@@server_id) |
 * +--------------------------+
 * |      1796635423003381771 |
 * +--------------------------+
 *
 * Normally, a UDF function in prohibited to use global variables. I opted to 
 * transgress that rule using an atomic function on the sequence. This should 
 * preserve the thread safety.
 *
 * Please do not copyright this code.  This code is in the public domain.
 *
 */


#include <ctype.h>
#include <my_sys.h>
#include <mysql.h>
#include <string.h>
#include <sys/time.h>
#include <unistd.h>
#include <atomic>

std::atomic<unsigned long long> sequence{0};

/* Prototypes */

extern "C" {
bool snowflakeId_init(UDF_INIT *initid, UDF_ARGS *args, char *message);
ulonglong snowflakeId(UDF_INIT *initid, UDF_ARGS *args, char *is_null, char *error);
}

bool snowflakeId_init(UDF_INIT *initid, UDF_ARGS *args, char *message) {
  if (args->arg_count > 1) {
    strcpy(message, "SNOWFLAKE requires at most one integer argument for the machineid, you can use @@server_id");
    return true;
  }
  initid->maybe_null = 0; /* The result will never be NULL */
  return false;
}

ulonglong snowflakeId(UDF_INIT *initid [[maybe_unused]], UDF_ARGS *args,
                 char *is_null [[maybe_unused]],
                 char *error [[maybe_unused]]) {

  struct timeval now;
  ulonglong genId,ts, s;

  gettimeofday(&now,NULL);

  /* The sequence part is 12 bits, 4096 values */
  s = sequence.fetch_add(1, std::memory_order_relaxed) % 4096;

  /* Current epoch in milliseconds */
  ts = (((now.tv_sec) * 1000 + now.tv_usec/1000.0) + 0.5);
  /* Shift offset to twitter epoch (see: https://en.wikipedia.org/wiki/Snowflake_ID) */
  /* Feel free to adjust to what is suitable for your organization */
  ts -= 1288834974657; 

  genId = 1;
  if (args->arg_count == 1) {
    if (args->args[0] != NULL) {
      long long int_val;
      int_val = *((long long *)args->args[0]);

      /* The machineID part is 10 bits, 1024 values */
      genId = int_val % 1024;
    }
  }

  return ts*4*1024*1024 + genId*4*1024 + s;
}
