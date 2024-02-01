#!/bin/bash

set -eou pipefail

# Define the path to the test resources directory relative to the script location
TEST_RESOURCES_DIR="$(dirname "$0")/../test/resources"

# Ensure the test resources directory exists
mkdir -p "$TEST_RESOURCES_DIR"

# Function to create a bundle directory with subdirectories
create_bundle() {
    local BUNDLE_DIR="$TEST_RESOURCES_DIR/$1"
    echo "Creating $BUNDLE_DIR..."

    # Remove the bundle directory if it already exists
    if [ -d "$BUNDLE_DIR" ]; then
        echo "Removing existing directory: $BUNDLE_DIR"
        rm -rf "$BUNDLE_DIR"
    fi

    # Create the specified subdirectories and add a .gitkeep file in each
    shift
    for SUBDIR in "$@"; do
        local FULL_PATH="$BUNDLE_DIR/$SUBDIR"
        mkdir -p "$FULL_PATH"
        touch "$FULL_PATH/.gitkeep"
        echo "Created $FULL_PATH with .gitkeep"
    done

    # Add mock config.json and umoci.json files
    touch "$BUNDLE_DIR/config.json"
    touch "$BUNDLE_DIR/umoci.json"
    echo "Added mock config files to $BUNDLE_DIR"
}

# Create each bundle
create_bundle "bundle-1" "rootfs/etc" "rootfs/usr" "rootfs/var"
create_bundle "bundle-2" "rootfs/bin" "rootfs/lib" "rootfs/sbin"

echo "Mock bundle directories created successfully."
