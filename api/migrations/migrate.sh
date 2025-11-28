#!/bin/bash

# Migration script for gshub-k8s database
# Usage: ./migrate.sh [host] [port] [user] [password] [database]
# Example: ./migrate.sh localhost 5432 gshub password gshub

set -e

# Configuration with defaults
DB_HOST="${1:-localhost}"
DB_PORT="${2:-5432}"
DB_USER="${3:-gshub}"
DB_PASSWORD="${4:-password}"
DB_NAME="${5:-gshub}"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Validate required parameters
if [ -z "$DB_PASSWORD" ]; then
    echo -e "${RED}Error: DB_PASSWORD is required${NC}"
    echo "Usage: $0 <host> <port> <user> <password> <database>"
    exit 1
fi

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${YELLOW}Starting database migrations...${NC}"
echo "Host: $DB_HOST"
echo "Port: $DB_PORT"
echo "User: $DB_USER"
echo "Database: $DB_NAME"
echo ""

# Find all migration files and sort them numerically
MIGRATIONS=$(find "$SCRIPT_DIR" -name "*.sql" -type f | sort)

if [ -z "$MIGRATIONS" ]; then
    echo -e "${RED}No migration files found${NC}"
    exit 1
fi

MIGRATION_COUNT=$(echo "$MIGRATIONS" | wc -l)
echo -e "${YELLOW}Found $MIGRATION_COUNT migration(s)${NC}"
echo ""

# Create migrations table if it doesn't exist
echo -e "${YELLOW}Creating migrations table if not exists...${NC}"
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" <<EOF
CREATE TABLE IF NOT EXISTS schema_migrations (
  id SERIAL PRIMARY KEY,
  version VARCHAR(255) UNIQUE NOT NULL,
  executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
EOF

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Migrations table ready${NC}"
else
    echo -e "${RED}✗ Failed to create migrations table${NC}"
    exit 1
fi

echo ""

# Execute each migration
FAILED=0
EXECUTED=0

for migration_file in $MIGRATIONS; do
    migration_name=$(basename "$migration_file")

    # Skip if it's the script itself
    if [ "$migration_name" == "migrate.sh" ]; then
        continue
    fi

    # Extract version from filename (e.g., "00001" from "00001_init.sql")
    version=$(echo "$migration_name" | sed 's/_.*\.sql//')

    # Check if migration already executed
    ALREADY_RUN=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM schema_migrations WHERE version = '$version';" 2>/dev/null || echo "0")

    if [ "$ALREADY_RUN" -gt 0 ]; then
        echo -e "${YELLOW}⊘ Skipped${NC}: $migration_name (already executed)"
        continue
    fi

    echo -n "Running: $migration_name ... "

    # Execute the migration
    if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" < "$migration_file" > /dev/null 2>&1; then
        # Record migration in the migrations table
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "INSERT INTO schema_migrations (version) VALUES ('$version');" > /dev/null 2>&1

        echo -e "${GREEN}✓ Done${NC}"
        ((EXECUTED++))
    else
        echo -e "${RED}✗ Failed${NC}"
        ((FAILED++))
    fi
done

echo ""
echo -e "${YELLOW}Migration Summary:${NC}"
echo -e "  ${GREEN}Executed: $EXECUTED${NC}"
echo -e "  ${RED}Failed: $FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All migrations completed successfully!${NC}"
    exit 0
else
    echo -e "${RED}Some migrations failed!${NC}"
    exit 1
fi
