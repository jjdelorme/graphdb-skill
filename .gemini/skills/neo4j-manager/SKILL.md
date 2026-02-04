---
name: neo4j-manager
description: Utilities for managing Neo4j Community Edition databases. Allows listing databases and switching the active database (Stop/Start flow).
---

# Neo4j Manager Skill

This skill provides utilities to manage Neo4j databases, specifically tailored for **Neo4j Community Edition** where only one user database can be active at a time.

## Capabilities

### 1. List Databases
Displays a list of all databases in the Neo4j instance, showing their status (online/offline) and which one is default.

*   **Command:** `node .gemini/skills/neo4j-manager/scripts/list_databases.js`

### 2. Switch Database
Switches the active database.
*   **Logic:**
    1.  Checks currently active database.
    2.  Stops it (if different from target).
    3.  Starts the target database.
    4.  **Creates** the target database if it doesn't exist.

*   **Command:** `node .gemini/skills/neo4j-manager/scripts/switch_database.js <database_name>`

## Setup

1.  **Dependencies:**
    ```bash
    cd .gemini/skills/neo4j-manager
    npm install
    ```
2.  **Configuration:**
    Uses the same `.env` file as the main project (looks in project root).
    *   `NEO4J_URI`: bolt://localhost:7687
    *   `NEO4J_USER`: neo4j
    *   `NEO4J_PASSWORD`: (Required)

## Notes
*   **Community Edition Limit:** This skill is essential because Community Edition forbids `CREATE DATABASE` if another user database is already online. You must explicitly `STOP` one before `START`ing another. This skill automates that dance.
