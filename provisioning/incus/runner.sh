#!/bin/bash
# 問題のイメージから runner の Incus コンテナを作り、手元の runner と問題バンドルを入れて起動する。
#
# 使い方: provisioning/incus/runner.sh <問題バンドルのディレクトリ> <イメージのエイリアス>
#   リポジトリのルートで流す。runner はこのリポジトリからビルドする。
#   runner のコンテナ（piscon-runner）がすでにあれば、消して作り直す。
#   runner は、Incus のブリッジのゲートウェイのアドレスで、ポータルが公開する gRPC の 50051 番に届く。
set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 <bundle-dir> <image-alias>" >&2
  exit 2
fi
bundle_dir=$1
image=$2
name=piscon-runner

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

CGO_ENABLED=0 go build -o "$work/runner" ./cmd/runner

network=$(incus profile device get default eth0 network)
gateway=$(incus network get "$network" ipv4.address)
gateway=${gateway%/*}
cat > "$work/piscon_runner.yaml" <<EOF
portal:
  address: $gateway:50051
bundle: /srv/runner/bundle
EOF

incus delete --force "$name" >/dev/null 2>&1 || true
incus launch "$image" "$name"
incus exec "$name" -- mkdir -p /srv/runner/bundle
incus file push "$work/runner" "$name/usr/local/bin/runner" --mode 0755
incus file push "$work/piscon_runner.yaml" "$name/srv/runner/piscon_runner.yaml"
incus file push --recursive "${bundle_dir%/}/." "$name/srv/runner/bundle/"
incus file push provisioning/ansible/roles/runner/files/piscon-runner.service "$name/etc/systemd/system/piscon-runner.service"

incus exec "$name" -- /usr/local/bin/runner check /srv/runner/bundle
incus exec "$name" -- systemctl daemon-reload
incus exec "$name" -- systemctl enable --now piscon-runner.service
echo "runner started in $name (portal: $gateway:50051)"
