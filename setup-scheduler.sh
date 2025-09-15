#!/bin/bash

# Script to set up the daily Trello reset scheduler

PLIST_NAME="com.makai.trello.dailyreset.plist"
PLIST_SOURCE="/Users/macfarnsworth/Workspaces/Alkira/trello/$PLIST_NAME"
PLIST_DEST="$HOME/Library/LaunchAgents/$PLIST_NAME"

echo "Setting up Makai's daily Trello reset scheduler..."

# Copy the plist to LaunchAgents directory
echo "Copying plist to LaunchAgents..."
cp "$PLIST_SOURCE" "$PLIST_DEST"

# Load the launch agent
echo "Loading launch agent..."
launchctl load "$PLIST_DEST"

echo "✅ Scheduler set up successfully!"
echo "The daily reset will run every day at 8:00 PM"
echo ""
echo "To check status: launchctl list | grep com.makai.trello"
echo "To stop: launchctl unload $PLIST_DEST"
echo "To start again: launchctl load $PLIST_DEST"
echo ""
echo "Logs will be written to:"
echo "  Output: /Users/macfarnsworth/Workspaces/Alkira/trello/daily-reset.log"
echo "  Errors: /Users/macfarnsworth/Workspaces/Alkira/trello/daily-reset-error.log"