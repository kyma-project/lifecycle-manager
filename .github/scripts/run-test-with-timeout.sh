#!/usr/bin/env bash

# Run E2E test with timeout and capture debugging info if timeout is hit
# Usage: ./run-test-with-timeout.sh <test-name> <timeout-minutes>

set +e  # Don't exit on error

TEST_NAME="$1"
TIMEOUT_MINUTES="${2:-18}"  # Default 18 minutes (less than job timeout of 20)
TIMEOUT_SECONDS=$((TIMEOUT_MINUTES * 60))

echo "Running test '$TEST_NAME' with ${TIMEOUT_MINUTES}-minute timeout..."

# Run test in background
make -C tests/e2e "$TEST_NAME" &
TEST_PID=$!

# Wait for test with timeout
ELAPSED=0
INTERVAL=5
while kill -0 $TEST_PID 2>/dev/null; do
    sleep $INTERVAL
    ELAPSED=$((ELAPSED + INTERVAL))
    
    if [ $ELAPSED -ge $TIMEOUT_SECONDS ]; then
        echo ""
        echo "=========================================="
        echo "TEST TIMEOUT after ${TIMEOUT_MINUTES} minutes!"
        echo "=========================================="
        echo ""
        
        # Kill the test
        kill -9 $TEST_PID 2>/dev/null
        
        # Run debugging BEFORE exit
        echo "Capturing debugging information..."
        ./.github/scripts/debug/teardown.sh || true
        
        echo ""
        echo "Test timed out after ${TIMEOUT_MINUTES} minutes"
        exit 124  # Standard timeout exit code
    fi
done

# Test finished, check exit code
wait $TEST_PID
TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "Test failed with exit code: $TEST_EXIT_CODE"
fi

exit $TEST_EXIT_CODE
