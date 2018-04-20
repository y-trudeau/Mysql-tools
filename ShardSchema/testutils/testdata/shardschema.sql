-- MySQL dump 10.13  Distrib 5.7.19, for Linux (x86_64)
--
-- Host: 10.0.3.87    Database: shardschema
-- ------------------------------------------------------
-- Server version	5.7.18-16-log

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `oplog`
--

DROP DATABASE IF EXISTS shardschema;
CREATE DATABASE shardschema;
USE shardschema;

DROP TABLE IF EXISTS `oplog`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `oplog` (
  `shardId` int(10) unsigned NOT NULL,
  `version` int(10) unsigned NOT NULL,
  `seq` tinyint(4) unsigned NOT NULL,
  `taskName` varchar(100) DEFAULT NULL,
  `message` text,
  `output` longtext,
  `err` longtext,
  `lastUpdate` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`shardId`,`version`,`seq`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `oplog`
--

LOCK TABLES `oplog` WRITE;
/*!40000 ALTER TABLE `oplog` DISABLE KEYS */;
INSERT INTO `oplog` VALUES (1,1,1,NULL,NULL,NULL,NULL,'2017-09-21 20:28:37'),(1,1,2,NULL,NULL,NULL,NULL,'2017-09-21 20:29:01');
/*!40000 ALTER TABLE `oplog` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `shards`
--

DROP TABLE IF EXISTS `shards`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `shards` (
  `shardId` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `schemaName` varchar(64) NOT NULL,
  `shardDSN`   varchar(200) NOT NULL,
  `version`    int(11) NOT NULL DEFAULT '0',
  `taskName`   varchar(100) DEFAULT NULL,
  `lastTaskHb` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `lastUpdate` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`shardId`),
  KEY `idx_version_task` (`version`,`taskName`),
  KEY `idx_task_lasthb` (`taskName`,`lastTaskHb`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `shards`
--

LOCK TABLES `shards` WRITE;
/*!40000 ALTER TABLE `shards` DISABLE KEYS */;
INSERT INTO `shards` VALUES (1,'shard_1','user:pass@(tcp:10.2.2.1:3306)',0,NULL,'2017-09-21 18:42:56','2017-09-21 18:42:56');
/*!40000 ALTER TABLE `shards` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `versions`
--

DROP TABLE IF EXISTS `versions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `versions` (
  `version` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `command` varchar(1000) NOT NULL,
  `cmdType` enum('sql','pt-osc') NOT NULL DEFAULT 'sql',
  `lastUpdate` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `tableName` varchar(64) NOT NULL,
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `versions`
--

LOCK TABLES `versions` WRITE;
/*!40000 ALTER TABLE `versions` DISABLE KEYS */;
/*!40000 ALTER TABLE `versions` ENABLE KEYS */;
INSERT INTO `versions` VALUES
(1, "pt-online-schema-change", "pt-osc", NOW(), "t1"),
(2, "SELECT 1", "sql", NOW(), "t2");
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2017-10-06 16:03:10
