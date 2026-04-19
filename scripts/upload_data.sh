#!/usr/bin/env bash
# Upload a StoryPlotter JSON export onto the Coolify host into the
# persistent volume mounted by the MCP service.
#
# Assumes the Coolify service is configured to mount
# /opt/storyplotter-mcp/data → /data inside the container, and that
# you have SSH access (e.g. via Tailscale) as the coolify user.
#
# Usage:
#   ./scripts/upload_data.sh <local-backup.json> [ssh-target]
#
# ssh-target defaults to $COOLIFY_SSH (e.g. coolify-prod or user@host).

set -euo pipefail

src="${1:?usage: upload_data.sh <local-backup.json> [ssh-target]}"
target="${2:-${COOLIFY_SSH:-}}"

if [ -z "$target" ]; then
  echo "set COOLIFY_SSH or pass an ssh target as arg 2" >&2
  exit 1
fi

remote_dir="/opt/storyplotter-mcp/data"
remote_file="$remote_dir/storyplotter.json"

echo ">> ensuring $remote_dir exists on $target"
ssh "$target" "sudo mkdir -p $remote_dir && sudo chown -R \$USER $remote_dir"

echo ">> uploading $(basename "$src") → $target:$remote_file"
rsync -avz --progress "$src" "$target:$remote_file"

echo ">> verifying"
ssh "$target" "ls -la $remote_file && head -c 200 $remote_file && echo"

echo
echo "done. Restart the MCP service in Coolify so it reloads the file."
