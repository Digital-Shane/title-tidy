#!/usr/bin/env bash

# Create a custom config that adds show name to episode format
CONFIG_DIR="$HOME/.title-tidy"
CONFIG_FILE="$CONFIG_DIR/config.json"
mkdir -p "$CONFIG_DIR"

# Backup existing config if it exists
if [ -f "$CONFIG_FILE" ]; then
    echo "Backing up existing config to $CONFIG_FILE.backup"
    mv "$CONFIG_FILE" "$CONFIG_FILE.backup"
fi
