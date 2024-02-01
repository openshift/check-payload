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

    mkdir -p "$BUNDLE_DIR"

    # Shift to process subdirectories
    shift
    for SUBDIR in "$@"; do
        local FULL_PATH="$BUNDLE_DIR/$SUBDIR"
        mkdir -p "$FULL_PATH"
        touch "$FULL_PATH/.gitkeep"
        echo "Created $FULL_PATH with .gitkeep"
    done
}

# Create the bundle directories and subdirectories
create_bundle "bundle-1" "etc" "usr" "usr/lib64"
create_bundle "bundle-2" "bin" "lib" "sbin"

# After ensuring the directories are created, copy the binaries for bundle-1
echo "Copying binaries to bundle-1..."
cp "$TEST_RESOURCES_DIR/fips_compliant_app" "$TEST_RESOURCES_DIR/bundle-1/usr/fips_compliant_app"
cp "$TEST_RESOURCES_DIR/libcrypto.so" "$TEST_RESOURCES_DIR/bundle-1/usr/lib64/libcrypto.so"
cp "$TEST_RESOURCES_DIR/libcrypto.so" "$TEST_RESOURCES_DIR/bundle-1/usr/lib64/libcrypto.so.1.1"
echo "Copied binaries to bundle-1"

# Add mock config.json and umoci.json files to both bundles
for BUNDLE in "bundle-1" "bundle-2"; do
    touch "$TEST_RESOURCES_DIR/$BUNDLE/config.json"
    touch "$TEST_RESOURCES_DIR/$BUNDLE/umoci.json"
    echo "Added mock config files to $TEST_RESOURCES_DIR/$BUNDLE"
done

echo "Mock bundle directories created successfully."
