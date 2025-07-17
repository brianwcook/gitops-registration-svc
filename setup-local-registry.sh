#!/bin/sh
set -o errexit

# Registry settings
REG_NAME='kind-registry'
REG_PORT='5001'

# Create registry container unless it already exists
if [ "$(podman inspect -f '{{.State.Running}}' "${REG_NAME}" 2>/dev/null || true)" != 'true' ]; then
  podman run \
    -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" --network bridge --name "${REG_NAME}" \
    registry:2
fi

# Connect the registry to the cluster network if not already connected
if [ "$(podman inspect -f='{{json .NetworkSettings.Networks.kind}}' "${REG_NAME}" 2>/dev/null || echo 'null')" = 'null' ]; then
  podman network connect "kind" "${REG_NAME}" || true
fi

# Add the registry config to the nodes for localhost:5001 -> kind-registry:5000
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${REG_PORT}"
for node in $(kind get nodes --name gitops-reg); do
  podman exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOT | podman exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${REG_NAME}:5000"]
EOT
done

echo "Local registry setup complete!"
echo "Registry available at: localhost:${REG_PORT}"
echo "To use images: docker tag <image> localhost:${REG_PORT}/<image> && docker push localhost:${REG_PORT}/<image>"
