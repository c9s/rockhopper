-- +up
-- SQL in section 'Up' is executed when this migration is applied

-- MySQL dump 10.13  Distrib 8.0.15, for osx10.13 (x86_64)
--
-- Host: localhost    Database: example_db
-- ------------------------------------------------------
-- Server version	8.0.15

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
 SET NAMES utf8mb4 ;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `legacy_items`
--
------ This table is being retroactively commented out. The service has
------ permission to CREATE TABLE but lacks permission to DROP TABLE, so we
------ must avoid re-creating it.

------/*!40101 SET @saved_cs_client     = @@character_set_client */;
------ SET character_set_client = utf8mb4 ;
------CREATE TABLE `legacy_items` (
------  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
------  `label` varchar(32) NOT NULL,
------  PRIMARY KEY (`id`)
------) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
------/*!40101 SET character_set_client = @saved_cs_client */;
------
------LOCK TABLES `legacy_items` WRITE;
------/*!40000 ALTER TABLE `legacy_items` DISABLE KEYS */;
------INSERT INTO `legacy_items` VALUES (1,'example;with;semicolons');
------/*!40000 ALTER TABLE `legacy_items` ENABLE KEYS */;
------UNLOCK TABLES;

--
-- Table structure for table `widgets`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `widgets` (
  `widget_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(64) NOT NULL,
  `price` bigint(20) unsigned NOT NULL DEFAULT 0,
  `kind` enum('alpha','beta') NOT NULL,
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`widget_id`),
  UNIQUE KEY `UNI_Name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

LOCK TABLES `widgets` WRITE;
/*!40000 ALTER TABLE `widgets` DISABLE KEYS */;
/*!40000 ALTER TABLE `widgets` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- +down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE `widgets`;
