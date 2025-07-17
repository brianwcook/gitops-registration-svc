#!/bin/bash

set -euo pipefail

# Check dependencies
if ! command -v kubectl &>/dev/null || ! command -v yq &>/dev/null; then
  echo "This script requires both 'kubectl' and 'yq' (v4+)."
  exit 1
fi

NAMESPACE=$1
OUTPUT_FILE=$2
shift 2
RESOURCE_TYPES=("$@")

if [ -z "$NAMESPACE" ] || [ -z "$OUTPUT_FILE" ] || [ ${#RESOURCE_TYPES[@]} -eq 0 ]; then
  echo "Usage: $0 <namespace> <output-file.yaml> <resource1> <resource2> ..."
  exit 1
fi

echo "# Exporting resources from namespace: $NAMESPACE"
echo "# Types: ${RESOURCE_TYPES[*]}"
TEMP_DIR=$(mktemp -d)

for TYPE in "${RESOURCE_TYPES[@]}"; do
  echo "Exporting $TYPE..."
  RESOURCES=$(kubectl get "$TYPE" -n "$NAMESPACE" -o name || true)
  for RES in $RESOURCES; do
    FILE="$TEMP_DIR/$(echo "$RES" | tr '/' '_').yaml"
    kubectl get "$RES" -n "$NAMESPACE" -o yaml > "$FILE"
    
    # Strip non-GitOps fields
    yq eval '
      del(
        .metadata.creationTimestamp,
        .metadata.resourceVersion,
        .metadata.uid,
        .metadata.managedFields,
        .metadata.annotations."kubectl.kubernetes.io/last-applied-configuration",
        .status
      )
    ' -i "$FILE"
  done
done

# Merge into one file
echo "# Merging and writing to: $OUTPUT_FILE"
yq eval-all 'select(fileIndex >= 0)' "$TEMP_DIR"/*.yaml > "$OUTPUT_FILE"

# Clean up
rm -rf "$TEMP_DIR"

echo "✅ Done. Clean YAML written to: $OUTPUT_FILE"
