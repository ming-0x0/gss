-- ============================================
-- LOCAL DEVELOPMENT ONLY
-- ============================================

-- 1. Create database
CREATE DATABASE IF NOT EXISTS `gss`
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

-- 2. Create user
CREATE USER IF NOT EXISTS 'gss'@'%' IDENTIFIED BY 'gss';

-- 3. Grant all privileges 
GRANT ALL PRIVILEGES ON `gss`.* TO 'gss'@'%';

FLUSH PRIVILEGES;
