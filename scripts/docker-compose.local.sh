#!/bin/bash

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_DIR="${PROJECT_ROOT}/deployments/docker-compose"
COMMAND=$1

function checkResult() {
    result=$1
    if [ ${result} -ne 0 ]; then
        echo -e "Error occurred! Exiting..."
        exit ${result}
    fi
}

cd "${PROJECT_ROOT}"

COMPOSE_FILES="-f ${COMPOSE_DIR}/docker-compose.yml -f ${COMPOSE_DIR}/docker-compose.local.yml"

case $COMMAND in
    up)
        echo "Starting local containers..."
        docker compose --env-file .env $COMPOSE_FILES up -d
        checkResult $?
        ;;
    down)
        echo "Stopping local containers..."
        docker compose --env-file .env $COMPOSE_FILES down
        checkResult $?
        ;;
    *)
        echo "Restarting local containers (down then up)..."
        docker compose --env-file .env $COMPOSE_FILES down
        docker compose --env-file .env $COMPOSE_FILES up -d
        checkResult $?
        ;;
esac
