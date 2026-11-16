#!/bin/bash
# private-isu の問題サーバーを準備する。Incus のチーム環境で、cloud-init がコンテナの初回起動時に root で流す。
# 準備が終わったら ready の印を、失敗したら理由を書いた failed の印を置く。ポータルはこの印でチーム環境の状態を決める。
set -eu

mkdir -p /var/lib/piscon
trap 'status=$?; [ "$status" -eq 0 ] || echo "user-data exited with status $status" > /var/lib/piscon/failed' EXIT

# config は、ポータルがコンテナに付けた設定値をゲスト API で読む。値が空の設定は Incus が持たないので、空として扱う。
config() {
  local body code
  body=$(mktemp)
  code=$(curl -s -o "$body" -w '%{http_code}' --unix-socket /dev/incus/sock "http://custom.socket/1.0/config/$1")
  case "$code" in
  200) cat "$body" ;;
  404) ;;
  *)
    echo "read $1: HTTP $code" >&2
    return 1
    ;;
  esac
  rm -f "$body"
}

# イメージの isucon ユーザーは sudo できるが、SSH の鍵を持たない。
keys=$(config user.piscon.ssh-public-keys)
install -d -m 700 -o isucon -g isucon /home/isucon/.ssh
printf '%s\n' "$keys" > /home/isucon/.ssh/authorized_keys
chown isucon:isucon /home/isucon/.ssh/authorized_keys
chmod 600 /home/isucon/.ssh/authorized_keys

touch /var/lib/piscon/ready
