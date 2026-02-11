#!/bin/bash

# Configuration based on current container 'neo4j-graphdb'
CONTAINER_NAME="neo4j-graphdb"
IMAGE="docker.io/library/neo4j:5.26.0"
# Use absolute path for bind mount
BASE_PATH="$(pwd)/.gemini/graph_data/neo4j"
DATA_PATH="${BASE_PATH}/data"
LOGS_PATH="${BASE_PATH}/logs"
CONF_PATH="${BASE_PATH}/conf"

# Ensure data directories exist
mkdir -p "$DATA_PATH" "$LOGS_PATH" "$CONF_PATH"
chmod 777 "$DATA_PATH" "$LOGS_PATH" "$CONF_PATH"

# Write neo4j.conf so that it listens on all interfaces (required for podman port forwarding)
cat > "${CONF_PATH}/neo4j.conf" <<'CONF'
server.default_listen_address=0.0.0.0
dbms.security.procedures.unrestricted=apoc.*
dbms.security.procedures.allowlist=apoc.*
CONF

# Check if container exists
if podman ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Container '${CONTAINER_NAME}' already exists."
    
    # Check if it is running
    if podman ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "It is already running."
    else
        echo "Starting existing container..."
        podman start "${CONTAINER_NAME}"
    fi
else
    echo "Creating and starting new container '${CONTAINER_NAME}'..."
    # Launch with exact configuration from previous 'podman inspect'
    podman run -d \
        --user 0:0 \
        --entrypoint neo4j \
        --name "${CONTAINER_NAME}" \
        -p 7474:7474 \
        -p 7687:7687 \
        -e NEO4J_AUTH=neo4j/password \
        -e NEO4J_PLUGINS='["apoc"]' \
        -v "${DATA_PATH}:/data" \
        -v "${LOGS_PATH}:/logs" \
        -v "${CONF_PATH}/neo4j.conf:/var/lib/neo4j/conf/neo4j.conf:ro" \
        "${IMAGE}" \
        console
fi

echo "------------------------------------------------"
echo "Neo4j is running!"
echo "HTTP Interface: http://localhost:7474"
echo "Bolt Interface: bolt://localhost:7687"
echo "Username: neo4j"
echo "Password: password"
echo "------------------------------------------------"
