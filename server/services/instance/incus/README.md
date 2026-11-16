# Incus のチーム環境

`INSTANCE_MANAGER=incus` のとき、ポータルはチーム環境を手元の Incus のシステムコンテナで作ります。
テンプレートの `AWS::EC2::Instance` 1 つにつき 1 台のコンテナを、問題バンドルの `environment.incus.image` のイメージから作ります。

- コンテナの名前は `<チーム環境の名前>-<サーバーの名前>`（例: `piscon-<UUID>-Server1`）です。名前が 63 文字に収まるよう、サーバーの名前は英数字 19 文字までにしてください
- 各コンテナには `environment.incus.limits` の CPU とメモリの上限を付けます
- コンテナの中の `free` や `/proc/meminfo` には、上限ではなくホストのメモリが表示されます。上限は cgroup で効くので、超えると OOM killer が動きます

## user-data の書き方

`environment.incus.userData` のファイルは、cloud-init がコンテナの初回起動時に root で流します。全サーバーが同じファイルを使います。
イメージは cloud-init を含むもの（`images:ubuntu/24.04/cloud` など）から作ってください。

ポータルは、各コンテナに次の設定値を付けます。user-data は、コンテナの中のゲスト API でこれを読みます。

| 設定値 | 中身 |
|---|---|
| `user.piscon.environment` | チーム環境の名前 |
| `user.piscon.server` | サーバーの名前（テンプレートの論理 ID） |
| `user.piscon.team-id` | チームの ID |
| `user.piscon.ssh-public-keys` | チームメンバーの公開鍵を改行で区切ったもの。鍵が 1 つもなければ設定値ごとない |

```sh
curl -s --unix-socket /dev/incus/sock http://custom.socket/1.0/config/user.piscon.server
```

user-data は次のことを守ってください。

- 準備が終わったら `/var/lib/piscon/ready` を作る。ポータルは、全サーバーにこのファイルがそろったときにチーム環境を ready にする
- 失敗したら、理由を書いた `/var/lib/piscon/failed` を作る。ポータルはチーム環境を failed にし、ファイルの中身を理由として表示する
- サーバーごとに準備を変えるときは、`user.piscon.server` の値で分岐する

最初のコンテナを作ってから 15 分たっても全サーバーがそろわなければ、チーム環境は failed になります。

例は [`problems/private_isu/incus/user-data.sh`](../../../../problems/private_isu/incus/user-data.sh) を参照してください。

## チーム環境の状態

ポータルは状態を DB に持たず、読むたびにコンテナと印のファイルから決めます。上の行から順に当てはめます。

| コンテナと印 | 状態 |
|---|---|
| コンテナが 1 つもない | deleted |
| 削除を依頼してから 5 分を過ぎても残っている | deleteFailed |
| 削除を依頼した | deleting |
| どれかのサーバーに `failed` がある | failed |
| 全サーバーのコンテナがあり、全サーバーに `ready` がある | ready |
| 最初のコンテナを作ってから 15 分を過ぎた | failed |
| それ以外 | creating |

参加者が `/var/lib/piscon/ready` を消すと、チーム環境が ready でなくなり、ベンチと停止・起動ができなくなります。
`sudo touch /var/lib/piscon/ready` で戻せます。

削除では、コンテナを ephemeral にしてから強制停止し、止まったコンテナを Incus に消させます。
ポータルが作ったのに DB に記録のないコンテナ（作成の途中でポータルが落ちたときなど）は、`incus list piscon-` で探して `incus delete --force` で消してください。
