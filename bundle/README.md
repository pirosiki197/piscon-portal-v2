# 問題バンドル

問題バンドルは、作問者が用意する問題の一式を入れたディレクトリです。
問題リポジトリのソースではなく、ビルド済みの成果物を置きます。

```text
dist/
├── problem.yaml   # マニフェスト
├── bench/         # runner ホストに配置するベンチ一式
└── manual.md      # 参加者向けのマニュアル
```

runner とポータルは、このパッケージでマニフェストを読み込み、検証します。

## マニフェスト

```yaml
name: sample-fib # 問題の名前
environment:
  imageId: ami-0123456789abcdef0 # (任意) 問題サーバーの AMI ID
bench:
  kind: command # command か adapter
  dir: bench
  command: ["./bench", "--target", "{{ .TargetIP }}"]
  timeout: 5m # (任意)
manual: manual.md
```

- パスは、問題バンドルのディレクトリからの相対パスで書きます。問題バンドルの外を指すパスは使えません
- 知らないキーがあるとエラーになります
- `bench` の書き方は、`kind` によって変わります
  - `kind: command` は、作問者のベンチマーカーを共通の形式で実行します。`dir`、`command`、`timeout` を書きます。詳しくは [`runner/benchmarker/impl/command/README.md`](../runner/benchmarker/impl/command/README.md) を参照してください
  - `kind: adapter` は、runner 内の既存の実装（過去問）でベンチを実行します。`adapter` に実装の名前（`isucon13` など）を、`options` にその実装の設定を書きます

```yaml
bench:
  kind: adapter
  adapter: isucon13
  options:
    benchmarker-path: /home/isucon/bench
    benchmarker-dir: /home/isucon
```

## 検証

`runner check <問題バンドルのディレクトリ>` は、マニフェストを検証し、ベンチマーカーを作れるかを確かめて終了します。
Ansible は、runner ホストに問題バンドルを配置した後にこのコマンドを実行します。
