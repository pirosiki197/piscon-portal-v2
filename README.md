# piscon-portal-v2

## 開発環境

プロジェクトルートディレクトリに以下のように `.env` ファイルを用意してください。`CLIENT_ID`と`CLIENT_SECRET`はtraQのものです。

```
DB_USER=root
DB_PASSWORD=password
DB_HOST=db
DB_PORT=3306
DB_NAME=portal

ROOT_URL=http://localhost:8080

SESSION_SECRET=secret

CLIENT_ID=my-client-id
CLIENT_SECRET=my-client-secret

ADMIN_USER_ID={{ traQ user ID (UUID) }}
ADMIN_USER_NAME={{ traQ user name }}
```

traQで作成するClientは リダイレクトURL に `http://localhost:8080/api/oauth2/callback`、スコープに `openid`, `profile` を指定します。
`task up` で portal server が 8080番 ポートで起動します。`task dev` でホットリロード付きで起動します。

### AWS を使ったローカルでの動作

ポータルをローカルのdocker composeで動かし、AWSのインスタンスを操作するには、以下のような設定を行います。

#### 1. AWS CLI のインストール

AWS CLI をインストールします。

#### 2. AWS の認証情報の設定

AWS CLIを使って認証のプロファイルを設定します。`~/.aws/config` と `~/.aws/credentials` ファイルが作成されます。

#### 3. `.env` への追記

`.env` ファイルに以下のように追記します。

チーム環境は、問題バンドルの環境テンプレートから CloudFormation のスタックとして作ります。そのため、AWS を使うときは問題バンドルが必要です。サーバーの AMI とインスタンスタイプは、問題バンドルのマニフェストとテンプレートが決めます。

```dotenv
PROBLEM_BUNDLE_PATH={{ 問題バンドルのディレクトリ（ホスト上のパス） }}
PROBLEM_BUNDLE_DIR=/bundle

AWS_REGION=ap-northeast-1
AWS_SUBNET_ID={{ AWS の Subnet ID }}
AWS_SECURITY_GROUP_ID={{ AWS の Security Group ID }}

INSTANCE_MANAGER=aws

AWS_PROFILE={{ 2で用意した AWS のプロファイル名 }}
AWS_SHARED_CREDENTIALS_FILE=/app/.aws/credentials
AWS_CONFIG_FILE=/app/.aws/config
AWS_SDK_LOAD_CONFIG=true
```

#### 4. 起動

```bash
docker compose --file compose.aws.yaml up
```

### Incus を使ったローカルでの動作

AWS の代わりに、手元の [Incus](https://linuxcontainers.org/incus/) のシステムコンテナでチーム環境を作れます。
1 チーム分のチーム環境を作り、SSH で入り、ポータルからベンチを流すところまでを手元で試せます。
仕組みと user-data の書き方は [server/services/instance/incus/README.md](server/services/instance/incus/README.md) を参照してください。

Linux（WSL を含む）で、Incus と Docker Engine を同じマシンに入れて使います。
ポータルは Incus の Unix ソケットを使うので、Docker Desktop では動きません。

#### 1. Incus と Docker Engine の準備

- Incus を入れて `incus admin init --minimal` で初期化し、自分を `incus-admin` グループに入れます
- Docker Engine は、ホストのファイアウォールの FORWARD を既定で DROP にするので、Incus のコンテナから外に出られなくなります。`/etc/docker/daemon.json` に `"ip-forward-no-drop": true` を書いて Docker を再起動してください。詳しくは Incus のドキュメントの [Prevent connectivity issues with Incus and Docker](https://linuxcontainers.org/incus/docs/main/howto/network_bridge_firewalld/#prevent-connectivity-issues-with-incus-and-docker) を参照してください

#### 2. 問題のイメージを作る

問題バンドルの `environment.incus` があるのは、今は private-isu だけです。

```bash
problems/private_isu/incus/build-image.sh
```

#### 3. `.env` への追記

```dotenv
PROBLEM_BUNDLE_PATH={{ 問題バンドルのディレクトリ（ホスト上のパス） }}
PROBLEM_BUNDLE_DIR=/bundle

INSTANCE_MANAGER=incus
INCUS_SOCKET=/var/lib/incus/unix.socket
INCUS_GID={{ getent group incus-admin で表示される GID }}
```

#### 4. 起動

```bash
docker compose --file compose.incus.yaml up
```

runner は、問題のイメージから作る Incus のコンテナで動かします。ポータルが起動してから流してください。

```bash
provisioning/incus/runner.sh problems/private_isu private-isu
```

チーム環境のサーバーには、画面に表示される IP アドレスへ、Incus を動かすマシンから SSH できます。

## protobuf について

[proto/README.md](proto/README.md) を参照してください。

## ベンチマークログ

保存先と確認方法は [runner/README.md](runner/README.md) を参照してください。
