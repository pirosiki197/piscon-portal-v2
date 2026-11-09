# command

自作問題のベンチマーカーを、設定したコマンドラインで実行します。
ベンチマーカーは、結果を共通の形式で返します。

## 設定例

問題バンドルのマニフェスト（`problem.yaml`）の `bench` に、`kind: command` で指定します。
マニフェスト全体の形式は [`bundle/README.md`](../../../../bundle/README.md) を参照してください。

```yaml
bench:
  kind: command
  dir: bench # ベンチマーカーを実行するディレクトリ。問題バンドルからの相対パス
  command: ["./bench", "--target", "{{ .TargetIP }}", "--all", "{{ join .InstanceIPs \",\" }}"]
  timeout: 5m # (任意) デフォルトは10m
```

## コマンドライン

`command` の各要素は Go の `text/template` として展開します。使える値と関数は次のとおりです。

- `{{ .TargetIP }}` には、チームが選んだ対象サーバーの private IP が入ります
- `{{ .InstanceIPs }}` には、チームの稼働中のサーバーの private IP の一覧が入ります。対象サーバーもこの一覧に含まれます
- `join` は、一覧を区切り文字でつなぎます。`{{ join .InstanceIPs "," }}` のように使います

`command` の先頭の要素が相対パスなら、`dir` を基準に解決します。
runner は起動時と `runner check` の実行時に、仮の値でテンプレートを展開します。存在しない値を参照していると、エラーになります。

## 結果の返し方

- stdout に書いた内容は、参加者向けのログになります
- stderr に書いた内容は、運営向けのログになります
- fd 3 に、1 行に 1 つの JSON でレポートを書きます

```json
{"score": 1200, "finished": false}
{"score": 3400, "passed": true, "finished": true}
```

- `score` と `finished` は必須です。`passed` は `finished` が `true` のレポートで必須です
- runner は、それまでに届いた最後のレポートの `score` を途中のスコアとして表示します
- runner は、`finished` が `true` のレポートを受け取ると、以降のレポートを読みません。最終レポートは最後に書いてください

## 合否の判定

終了コードは、ベンチマーカー自身が最後まで正常に動いたかを表します。レポートは、競技の結果を表します。
整合性チェックで落ちたなどの競技上の失敗でも、ベンチマーカーが最後まで動けたなら、終了コード 0 で終わってください。

| 最後のレポート \ 終了コード | 0 | 0 以外 |
|---|---|---|
| `finished: true, passed: true` | 成功 | エラー |
| `finished: true, passed: false` | 失敗 | エラー |
| `finished: true` のレポートがない | エラー | エラー |
| JSON として読めない行、必須の値が欠けた行がある | エラー | エラー |

## タイムアウト

`timeout` を過ぎると、runner はベンチマーカーのプロセスグループに SIGKILL を送り、結果をエラーにします。
ベンチマーカーが起動した子プロセスも同じプロセスグループに入るので、一緒に停止します。
