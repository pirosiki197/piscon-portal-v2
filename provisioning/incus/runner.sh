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
# incus launch は、標準入力が端末でなければ、そこからインスタンスの設定を読む。
incus launch "$image" "$name" </dev/null
incus exec "$name" -- mkdir -p /srv/runner/bundle
incus file push "$work/runner" "$name/usr/local/bin/runner" --uid 0 --gid 0 --mode 0755
incus file push "$work/piscon_runner.yaml" "$name/srv/runner/piscon_runner.yaml" --uid 0 --gid 0
incus file push provisioning/ansible/roles/runner/files/piscon-runner.service "$name/etc/systemd/system/piscon-runner.service" --uid 0 --gid 0
# incus file push --recursive はディレクトリごと送るので、中身だけを tar で流し込む。
tar -C "$bundle_dir" --owner=0 --group=0 -cf - . | incus exec "$name" -- tar -C /srv/runner/bundle -xf -

incus exec "$name" -- /usr/local/bin/runner check /srv/runner/bundle
incus exec "$name" -- systemctl daemon-reload
incus exec "$name" -- systemctl enable --now piscon-runner.service
echo "runner started in $name (portal: $gateway:50051)"
