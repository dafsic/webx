#!/usr/bin/env bash
# Generate Go code + OpenAPI from proto/.
#
# We use protoc directly (instead of `buf generate`) because the project
# vendors third-party protos under proto/third_party/ rather than consuming
# them via the Buf Schema Registry. protoc's -I flag gives us fine-grained
# control over import roots without pulling network deps.
#
# Required tools on PATH (install via `go install`):
#   - protoc                        (system)
#   - protoc-gen-go
#   - protoc-gen-go-grpc
#   - protoc-gen-grpc-gateway
#   - protoc-gen-openapiv2
set -euo pipefail

cd "$(dirname "$0")/.."
PATH="${HOME}/go/bin:${HOME}/.local/bin:${PATH}"
export PATH

OUT=proto_go
rm -rf "${OUT}"
mkdir -p "${OUT}"

PROTO_INCLUDES=(
  -I proto
  -I proto/third_party
)

# Collect all *.proto under proto/<svc>/ (skip third_party).
mapfile -t FILES < <(find proto -type f -name '*.proto' ! -path 'proto/third_party/*')

if [[ ${#FILES[@]} -eq 0 ]]; then
  echo "no proto files found" >&2
  exit 1
fi

protoc "${PROTO_INCLUDES[@]}" \
  --go_out="${OUT}"           --go_opt=paths=source_relative \
  --go-grpc_out="${OUT}"      --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
  --grpc-gateway_out="${OUT}" --grpc-gateway_opt=paths=source_relative,generate_unbound_methods=true \
  --openapiv2_out="${OUT}"    --openapiv2_opt=allow_merge=true,merge_file_name=webx,json_names_for_fields=false \
  "${FILES[@]}"

echo "ok: regenerated ${OUT}/ from ${#FILES[@]} proto file(s)"

