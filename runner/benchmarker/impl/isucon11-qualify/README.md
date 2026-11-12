# isucon11-qualify

https://github.com/isucon/isucon11-qualify

## 設定例

```yaml
problem:
  name: isucon11-qualify
  options:
    benchmarker-path: /home/isucon/bench/bench
    benchmarker-dir: /home/isucon/bench # ベンチマーカーを実行するときのディレクトリ
```

## 注意

- ベンチマーカーは JIA API のモックを 4999 番で立て、ターゲットはそこにリクエストを送ります。
  JIA API の URL には、runner ホストがターゲットに通信するときの送信元 IP を使います。ターゲットからこの IP の 4999 番に届く必要があります。
