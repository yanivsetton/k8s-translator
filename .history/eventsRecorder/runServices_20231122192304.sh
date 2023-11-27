#!/bin/bash

go run ./pods.go

# Wait for all background jobs to complete
wait

echo "All Go apps have finished."