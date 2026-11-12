# ISUCON11 本選

この問題は [ISUCON11 本選](https://github.com/isucon/isucon11-final) です。
問題の説明とルールは、公式のマニュアルを読んでください。

- [当日マニュアル](https://github.com/isucon/isucon11-final/blob/main/docs/manual.md)
- [アプリケーションマニュアル](https://github.com/isucon/isucon11-final/blob/main/docs/isucholar.md)

公式のマニュアルと違うところは、このページに書きます。

## サーバー

チーム環境には、問題サーバーが 3 台（`Server1`、`Server2`、`Server3`）あります。

- `isucon` ユーザーで SSH できます。鍵は、チーム環境を作った時点でチームメンバーが GitHub に登録していた公開鍵です
  ```sh
  ssh isucon@<サーバーの public IP>
  ```
- public IP は、チーム環境を停止して起動するたびに変わります。サーバー同士の通信には、変わらない private IP を使ってください

## ベンチマーク

ベンチマークはポータルから実行します。ベンチマークの対象サーバーは、ポータルでチームが選びます。
公式のマニュアルにある、ポータルやサーバー上でベンチマーカーを動かす手順は使いません。
