# 過去問の問題バンドル

過去の ISUCON の問題を、[問題バンドル](../bundle/README.md)にしたものです。
デプロイするときは、使う問題のディレクトリを Ansible の `local.problem.bundle_dir` に指定します。

| ディレクトリ | 問題 | サーバー |
|---|---|---|
| [`private_isu`](./private_isu) | [private-isu](https://github.com/catatsuy/private-isu) | 1 台 |
| [`isucon11-qualify`](./isucon11-qualify) | [ISUCON11 予選](https://github.com/isucon/isucon11-qualify) | 3 台 |
| [`isucon11-final`](./isucon11-final) | [ISUCON11 本選](https://github.com/isucon/isucon11-final) | 3 台 |
| [`isucon13`](./isucon13) | [ISUCON13](https://github.com/isucon/isucon13) | 3 台 |
| [`isucon14`](./isucon14) | [ISUCON14](https://github.com/isucon/isucon14) | 3 台 |

## AMI

問題サーバーには、公開されている AMI（ap-northeast-1、x86_64）を使います。
private-isu は [catatsuy/private-isu](https://github.com/catatsuy/private-isu) の AMI を、それ以外は [matsuu/aws-isucon](https://github.com/matsuu/aws-isucon) の AMI を使います。
AMI の ID は、各問題の `problem.yaml` の `environment.imageId` に書いてあります。

これらの AMI には、問題のアプリと一緒にベンチマーカーが入っています。
runner は runner ホストにあるベンチマーカーを実行するので、runner ホストも同じ AMI から作ります。
CDK でデプロイするときは、`RUNNER_AMI_ID` に問題の `environment.imageId` と同じ AMI ID を指定してください。

## チーム環境

チーム環境のサーバーは、公式の本番と同じ台数にしています。インスタンスタイプは、公式に近い `c5.large`（private-isu は `c7a.large`）です。
費用を抑えたいときは、`template.yaml` の `InstanceType` を書き換えます。

参加者は `isucon` ユーザーで SSH します。鍵には、チームメンバーが GitHub に登録した公開鍵が入ります。
