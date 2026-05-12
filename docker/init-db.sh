#!/bin/bash
# PostgreSQL 初始化脚本
# 创建应用数据库和用户

set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- 创建应用用户
    CREATE USER ${APP_USER} WITH PASSWORD '${APP_PASSWORD}';
    
    -- 创建应用数据库
    CREATE DATABASE ${APP_DB} OWNER ${APP_USER};
    
    -- 授权
    GRANT ALL PRIVILEGES ON DATABASE ${APP_DB} TO ${APP_USER};
    
    -- 连接到应用数据库并授予 schema 权限
    \c ${APP_DB}
    GRANT ALL ON SCHEMA public TO ${APP_USER};

    -- 安装扩展（需要 superuser 权限）
    CREATE EXTENSION IF NOT EXISTS pg_trgm;
EOSQL

echo "Database ${APP_DB} and user ${APP_USER} created successfully."

# 创建 new-api 数据库和用户
if [ -n "${NEW_API_USER}" ] && [ -n "${NEW_API_PASSWORD}" ] && [ -n "${NEW_API_DB}" ]; then
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- 创建 new-api 用户
    CREATE USER ${NEW_API_USER} WITH PASSWORD '${NEW_API_PASSWORD}';

    -- 创建 new-api 数据库
    CREATE DATABASE ${NEW_API_DB} OWNER ${NEW_API_USER};

    -- 授权
    GRANT ALL PRIVILEGES ON DATABASE ${NEW_API_DB} TO ${NEW_API_USER};

    -- 连接到 new-api 数据库并授予 schema 权限
    \c ${NEW_API_DB}
    GRANT ALL ON SCHEMA public TO ${NEW_API_USER};
EOSQL
  echo "Database ${NEW_API_DB} and user ${NEW_API_USER} created successfully."
fi

# 创建 sub2api 数据库和用户
if [ -n "${SUB2_DB_USER}" ] && [ -n "${SUB2_DB_PASSWORD}" ] && [ -n "${SUB2_DB_NAME}" ]; then
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- 创建 sub2api 用户
    CREATE USER ${SUB2_DB_USER} WITH PASSWORD '${SUB2_DB_PASSWORD}';

    -- 创建 sub2api 数据库
    CREATE DATABASE ${SUB2_DB_NAME} OWNER ${SUB2_DB_USER};

    -- 授权
    GRANT ALL PRIVILEGES ON DATABASE ${SUB2_DB_NAME} TO ${SUB2_DB_USER};

    -- 连接到 sub2api 数据库并授予 schema 权限
    \c ${SUB2_DB_NAME}
    GRANT ALL ON SCHEMA public TO ${SUB2_DB_USER};
EOSQL
  echo "Database ${SUB2_DB_NAME} and user ${SUB2_DB_USER} created successfully."
fi
