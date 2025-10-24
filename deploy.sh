#!/bin/bash

# This script deploys the TM25 application to a remote server.
# It uses rsync to transfer files and ssh to build the application remotely.

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
REMOTE_USER="borisk"
REMOTE_HOST="padre.rus9n.com"
REMOTE_DIR="tm25" # The destination folder on the remote server

# The full remote path for rsync
REMOTE_DEST="${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}"

echo ">>> Starting deployment to ${REMOTE_DEST}"

# --- Step 1: Sync files with rsync ---
# We use rsync to efficiently copy files.
# -avz: archive mode, verbose, compress
# --delete: delete files on the destination that are not in the source
# --exclude: specifies patterns to exclude from the transfer.
echo ">>> Syncing application files..."
rsync -avz --delete \
    --exclude 'config.toml' \
    --exclude 'data/' \
    --exclude 'tmp/' \
    --exclude '.git/' \
    --exclude 'deploy.sh' \
    --exclude 'tm25' \
    --exclude 'README.md' \
    --exclude 'LICENSE' \
    . "${REMOTE_DEST}/"

echo ">>> File sync complete."

# --- Step 2: Build the application on the remote server ---
echo ">>> Building application on remote server..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "
    set -e;
    cd ${REMOTE_DIR};
    echo '--- Running go mod tidy...';
    go mod tidy;
    echo '--- Building release executable...';
    go build -ldflags='-s -w' -o tm25 .;
    echo '--- Build complete. Executable is at ${REMOTE_DIR}/tm25';
    # Add a command here to restart your service, for example:
    # sudo systemctl restart tm25.service
"

echo ">>> Deployment finished successfully!"