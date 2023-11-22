#!/bin/bash

# Define the directory to be searched
DIRECTORY="/Users/yanivsetton/Documents/tictuk/experimental/k8s-translator-ui/eventsRecorder"

# Check if the directory exists
if [ -d "$DIRECTORY" ]; then
    # Loop through each file in the directory
    for file in "$DIRECTORY"/*; do
        # Check if it's a file and not a directory
        if [ -f "$file" ]; then
            echo "File Name: $(basename "$file")"
            echo "Content:"
            cat "$file"
            echo "" # Add an empty line for readability
        fi
    done
else
    echo "Directory does not exist."
fi