# ISUCON14

この問題は [ISUCON14](https://github.com/isucon/isucon14) です。
問題の説明とルールは、公式のマニュアルを読んでください。

- [当日マニュアル](https://github.com/isucon/isucon14/blob/main/docs/manual.md)
- [アプリケーションマニュアル](https://github.com/isucon/isucon14/blob/main/docs/ISURIDE.md)

公式のマニュアルと違うところは、このページに書きます。

## サーバー

チーム環境には、問題サーバーが 3 台（`Server1`、`Server2`、`Server3`）あります。

- `isucon` ユーザーで SSH できます。鍵は、チーム環境を作った時点でチームメンバーが GitHub に登録していた公開鍵です
  ```sh
  ssh isucon@<サーバーの public IP>
  ```
- public IP は、チーム環境を停止して起動するたびに変わります。サーバー同士の通信には、変わらない private IP を使ってください
- ブラウザでアプリを見るときは、手元の hosts に `<サーバーの public IP> isuride.xiv.isucon.net` を書いて `https://isuride.xiv.isucon.net/` を開きます。証明書は自己署名です

## ベンチマーク

ベンチマークはポータルから実行します。ベンチマークの対象サーバーは、ポータルでチームが選びます。
公式のマニュアルにある、ポータルやサーバー上でベンチマーカーを動かす手順は使いません。
