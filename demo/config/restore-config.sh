#!/usr/bin/env bash

# Create a custom config that adds show name to episode format
CONFIG_DIR="$HOME/.title-tidy"
CONFIG_FILE="$CONFIG_DIR/config.json"
mkdir -p "$CONFIG_DIR"

# Restore backup if it existed
if [ -f "$CONFIG_FILE.backup" ]; then
    echo "Restoring original config..."
    mv "$CONFIG_FILE.backup" "$CONFIG_FILE"
else
    echo "Removing demo config (no original config existed)..."
    rm -f "$CONFIG_FILE"
    rmdir "$CONFIG_DIR" 2>/dev/null || true
fi
