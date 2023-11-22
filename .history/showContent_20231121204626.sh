#!/bin/bash

# Define the directory to be searched
DIRECTORY="/Users/yanivsetton/Documents/tictuk/experimental/k8s-translator-ui/eventsRecorder"

# Define the output file
OUTPUT_FILE="/path/to/your/output_file.txt"

# Check if the directory exists
if [ -d "$DIRECTORY" ]; then
    # Empty the output file if it already exists
    > "$OUTPUT_FILE"

    # Loop through each file in the directory
    for file in "$DIRECTORY"/*; do
        # Check if it's a file and not a directory
        if [ -f "$file" ]; then
            echo "File Name: $(basename "$file")" >> "$OUTPUT_FILE"
            echo "Content:" >> "$OUTPUT_FILE"
            cat "$file" >> "$OUTPUT_FILE"
            echo "" >> "$OUTPUT_FILE" # Add an empty line for readability
        fi
    done
else
    echo "Directory does not exist."
fi