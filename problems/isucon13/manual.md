# ISUCON13

この問題は [ISUCON13](https://github.com/isucon/isucon13) です。
問題の説明とルールは、公式のマニュアルを読んでください。

- [当日マニュアル](https://github.com/isucon/isucon13/blob/main/docs/cautionary_note.md)
- [アプリケーションマニュアル](https://github.com/isucon/isucon13/blob/main/docs/isupipe.md)

公式のマニュアルと違うところは、このページに書きます。

## サーバー

チーム環境には、問題サーバーが 3 台（`Server1`、`Server2`、`Server3`）あります。

- `isucon` ユーザーで SSH できます。鍵は、チーム環境を作った時点でチームメンバーが GitHub に登録していた公開鍵です
  ```sh
  ssh isucon@<サーバーの public IP>
  ```
- public IP は、チーム環境を停止して起動するたびに変わります。サーバー同士の通信には、変わらない private IP を使ってください
- 各サーバーの PowerDNS は、`u.isucon.local` の名前をそのサーバーの private IP で答えます。runner は private IP でベンチマークするので、公式のマニュアルにある public IP への書き換えはしないでください
- ブラウザでアプリを見るときは、手元の hosts に `<サーバーの public IP> pipe.u.isucon.local` を書いて `https://pipe.u.isucon.local/` を開きます。証明書は自己署名です

## ベンチマーク

ベンチマークはポータルから実行します。ベンチマークの対象サーバーは、ポータルでチームが選びます。
公式のマニュアルにある、ポータルやサーバー上でベンチマーカーを動かす手順は使いません。
