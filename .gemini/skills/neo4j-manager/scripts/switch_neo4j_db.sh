#!/bin/bash

# Neo4j Database Switcher for Community Edition
# Usage: ./switch_neo4j_db.sh <database_name>

DB_NAME=$1

if [ -z "$DB_NAME" ]; then
  echo "Usage: $0 <database_name>"
  echo "Example: $0 neo4j"
  echo "Example: $0 new-graph"
  exit 1
fi

NEO4J_CONF="/etc/neo4j/neo4j.conf"
# Alternative location if using tarball install
# NEO4J_CONF="/path/to/neo4j/conf/neo4j.conf"

if [ ! -f "$NEO4J_CONF" ]; then
  echo "Error: neo4j.conf not found at $NEO4J_CONF"
  echo "Please edit this script to point to the correct config location."
  exit 1
fi

echo "Switching default database to '$DB_NAME'..."

# Update the default database settings
# Uses sed to replace the lines. Backs up config to neo4j.conf.bak
sudo sed -i.bak "s/^server\.default_database=.*/server.default_database=$DB_NAME/" "$NEO4J_CONF"
sudo sed -i "s/^dbms\.default_database=.*/dbms.default_database=$DB_NAME/" "$NEO4J_CONF"
sudo sed -i "s/^initial\.dbms\.default_database=.*/initial.dbms.default_database=$DB_NAME/" "$NEO4J_CONF"

if [ $? -eq 0 ]; then
  echo "Configuration updated."
else
  echo "Failed to update configuration. Check sudo permissions."
  exit 1
fi

echo "Restarting Neo4j service..."
sudo systemctl restart neo4j

if [ $? -eq 0 ]; then
  echo "✅ Successfully switched to '$DB_NAME' and restarted Neo4j."
else
  echo "❌ Failed to restart Neo4j service."
  exit 1
fi
