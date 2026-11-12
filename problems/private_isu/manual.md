# private-isu

この問題は [catatsuy/private-isu](https://github.com/catatsuy/private-isu) です。
問題の説明とルールは、公式の[当日マニュアル](https://github.com/catatsuy/private-isu/blob/master/manual.md)を読んでください。
公式のマニュアルと違うところは、このページに書きます。

## サーバー

チーム環境には、問題サーバーが 1 台（`Server1`）あります。

- `isucon` ユーザーで SSH できます。鍵は、チーム環境を作った時点でチームメンバーが GitHub に登録していた公開鍵です
  ```sh
  ssh isucon@<サーバーの public IP>
  ```
- ブラウザでは `http://<サーバーの public IP>/` を開くとアプリを見られます
- public IP は、チーム環境を停止して起動するたびに変わります

## ベンチマーク

ベンチマークはポータルから実行します。公式のマニュアルにある、サーバー上でベンチマーカーを動かす手順は使いません。
