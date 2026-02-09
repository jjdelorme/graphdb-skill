#!/bin/bash

# Configuration based on current container 'neo4j-graphdb'
CONTAINER_NAME="neo4j-graphdb"
IMAGE="docker.io/library/neo4j:5.26.0"
VOLUME_NAME="neo4j_data"

# Check if the volume exists (optional, but good for info)
if ! podman volume exists "$VOLUME_NAME"; then
    echo "Warning: Volume '$VOLUME_NAME' does not exist. A new one will be created."
fi

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
    podman run -d 
        --name "${CONTAINER_NAME}" 
        -p 7474:7474 
        -p 7687:7687 
        -e NEO4J_AUTH=neo4j/password 
        -e NEO4J_PLUGINS='["apoc"]' 
        -e NEO4J_dbms_security_procedures_unrestricted=apoc.* 
        -e NEO4J_dbms_security_procedures_allowlist=apoc.* 
        -v "${VOLUME_NAME}:/data" 
        "${IMAGE}"
fi

echo "------------------------------------------------"
echo "Neo4j is running!"
echo "HTTP Interface: http://localhost:7474"
echo "Bolt Interface: bolt://localhost:7687"
echo "Username: neo4j"
echo "Password: password"
echo "------------------------------------------------"
