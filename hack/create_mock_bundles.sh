#!/bin/bash

set -eou pipefail

# Define the path to the test resources directory relative to the script location
TEST_RESOURCES_DIR="$(dirname "$0")/../test/resources"

# Ensure the test resources directory exists
mkdir -p "$TEST_RESOURCES_DIR"

# Function to create a mock_unpacked_dir directory with subdirectories
create_mock_unpacked_dir() {
    local MOCK_UNPACKED_DIR="$TEST_RESOURCES_DIR/$1"
    echo "Creating $MOCK_UNPACKED_DIR..."

    # Remove the mock_unpacked_dir directory if it already exists
    if [ -d "$MOCK_UNPACKED_DIR" ]; then
        echo "Removing existing directory: $MOCK_UNPACKED_DIR"
        rm -rf "$MOCK_UNPACKED_DIR"
    fi

    mkdir -p "$MOCK_UNPACKED_DIR"

    # Shift to process subdirectories
    shift
    for SUBDIR in "$@"; do
        local FULL_PATH="$MOCK_UNPACKED_DIR/$SUBDIR"
        mkdir -p "$FULL_PATH"
        touch "$FULL_PATH/.gitkeep"
        echo "Created $FULL_PATH with .gitkeep"
    done
}

# Create the mock_unpacked_dir directories and subdirectories
create_mock_unpacked_dir "mock_unpacked_dir-1" "etc" "usr" "usr/lib64"
create_mock_unpacked_dir "mock_unpacked_dir-2" "bin" "lib" "sbin"

# After ensuring the directories are created, copy the binaries for mock_unpacked_dir-1
echo "Copying binaries to mock_unpacked_dir-1..."
cp "$TEST_RESOURCES_DIR/fips_compliant_app" "$TEST_RESOURCES_DIR/mock_unpacked_dir-1/usr/fips_compliant_app"
cp "$TEST_RESOURCES_DIR/libcrypto.so" "$TEST_RESOURCES_DIR/mock_unpacked_dir-1/usr/lib64/libcrypto.so"
cp "$TEST_RESOURCES_DIR/libcrypto.so" "$TEST_RESOURCES_DIR/mock_unpacked_dir-1/usr/lib64/libcrypto.so.1.1"
echo "Copied binaries to mock_unpacked_dir-1"

# Define symlink path
SYMLINK_PATH="$TEST_RESOURCES_DIR/mock_unpacked_dir-1/usr/lib64/libcrypto.so.1.1"

# Check if the symlink or file already exists
if [ -e "$SYMLINK_PATH" ] || [ -L "$SYMLINK_PATH" ]; then
    echo "Existing symlink or file found at $SYMLINK_PATH. Removing..."
    rm -f "$SYMLINK_PATH"
else
    echo "No existing symlink or file found at $SYMLINK_PATH."
fi

# Now attempt to create the symlink
echo "Creating symlink for libcrypto.so.1.1 to libcrypto.so in mock_unpacked_dir-1/usr/lib64..."
ln -s "$TEST_RESOURCES_DIR/mock_unpacked_dir-1/usr/lib64/libcrypto.so" "$SYMLINK_PATH"
if [ $? -eq 0 ]; then
    echo "Symlink created successfully."
else
    echo "Failed to create symlink. Investigating..."
    # Check if the target file exists
    if [ ! -e "$TEST_RESOURCES_DIR/mock_unpacked_dir-1/usr/lib64/libcrypto.so" ]; then
        echo "Target file does not exist: $TEST_RESOURCES_DIR/mock_unpacked_dir-1/usr/lib64/libcrypto.so"
    else
        echo "Target file exists. Other issue preventing symlink creation."
    fi
fi

# Add mock config.json and umoci.json files to both mock_unpacked_dirs
for MOCK_UNPACKED_DIR in "mock_unpacked_dir-1" "mock_unpacked_dir-2"; do
    touch "$TEST_RESOURCES_DIR/$MOCK_UNPACKED_DIR/config.json"
    touch "$TEST_RESOURCES_DIR/$MOCK_UNPACKED_DIR/umoci.json"
    echo "Added mock config files to $TEST_RESOURCES_DIR/$MOCK_UNPACKED_DIR"
done

echo "Mock mock_unpacked_dir directories created successfully."
