#!/usr/bin/env bash
# Prints a JSON array of the services that need building, e.g. ["portal","bills"].
#
# A service is any services/<name>/ that has a Dockerfile. Usage:
#   changed-services.sh <base-commit>      services changed since <base-commit>
#   changed-services.sh                    every service (also used when the base
#                                          is missing, e.g. a new branch or force-push)
set -euo pipefail

base="${1:-}"

all=$(for d in services/*/; do
  n=$(basename "$d")
  [ -f "$d/Dockerfile" ] && echo "$n"
done)

if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ] \
   || ! git cat-file -e "$base" 2>/dev/null; then
  selected="$all"
else
  changed=$(git diff --name-only "$base" HEAD)
  # Shared code, the workspace, docker ignore rules or the workflows affect every service.
  if echo "$changed" | grep -qE '^(shared/|go\.work|\.dockerignore|\.github/)'; then
    selected="$all"
  else
    selected=$(for n in $all; do
      echo "$changed" | grep -q "^services/$n/" && echo "$n"
    done || true)
  fi
fi

echo "$selected" | sed '/^$/d' | jq -R . | jq -sc .
