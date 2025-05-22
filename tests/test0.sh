#!/bin/bash

curl -X POST \
    -H "Content-Type: application/json" \
    -d '{"playerId": "player1"}' \
    http://localhost:8080/join
