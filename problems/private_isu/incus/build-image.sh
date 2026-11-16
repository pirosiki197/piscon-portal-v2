#!/bin/bash
# private-isu の Incus イメージを作り、エイリアスを付けて保存する。運営が手で 1 度流す。
#
# 使い方: build-image.sh [エイリアス]
#   エイリアスの既定は private-isu で、problem.yaml の environment.incus.image と揃える。
#   同じエイリアスのイメージがあれば、エイリアスを新しいイメージに付け替える。
#   PRIVATE_ISU_REF で、catatsuy/private-isu のブランチかタグを選べる（既定は master）。
set -euo pipefail

alias_name=${1:-private-isu}
ref=${PRIVATE_ISU_REF:-master}
builder=piscon-build-private-isu

cleanup() { incus delete --force "$builder" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

# cloud 版のイメージには cloud-init が入っている。チーム環境では、cloud-init が問題バンドルの user-data を流す。
# incus launch は、標準入力が端末でなければ、そこからインスタンスの設定を読む。
incus launch images:ubuntu/24.04/cloud "$builder" </dev/null
incus exec "$builder" -- cloud-init status --wait >/dev/null || true

incus exec "$builder" --env REF="$ref" -- bash -euo pipefail -c '
export DEBIAN_FRONTEND=noninteractive
apt-get -qq update
# Incus の Ubuntu のイメージは、AMI の元になる Ubuntu より入っているものが少ない。
# 公式のプロビジョニングは wget と unzip を前提にし、参加者は SSH で入る。
apt-get -qq install -y git ansible wget unzip openssh-server curl >/dev/null
git clone -q --depth 1 --branch "$REF" https://github.com/catatsuy/private-isu /root/private-isu
cd /root/private-isu/provisioning
printf "[guests]\nlocalhost ansible_connection=local\n[bench]\nlocalhost ansible_connection=local\n" > /root/inventory
# allinone は、ベンチマーカーを問題サーバーと同じマシンに入れる。runner もこのイメージから作る。
ANSIBLE_NOCOLOR=1 ansible-playbook -i /root/inventory image/ansible/playbooks.yml -e allinone=True
cd /
rm -rf /root/private-isu /root/inventory
apt-get -qq purge -y ansible >/dev/null
apt-get -qq autoremove -y >/dev/null
# 消さないと、イメージから作ったコンテナで cloud-init が user-data を流さない。
# machine-id も消し、コンテナごとに作り直させる。
cloud-init clean --logs --machine-id
# machine-id が空だと systemd は初回起動として preset を当て、preset にないユニットを有効にする。
# そのままだと、公式のプロビジョニングが有効にしていない isu-go や php-fpm まで起動するので、何も有効にさせない。
mkdir -p /etc/systemd/system-preset
echo "disable *" > /etc/systemd/system-preset/00-piscon.preset
'

incus stop "$builder"
incus publish "$builder" --alias "$alias_name" --reuse description="private-isu ($ref)"
echo "published image: $alias_name"
