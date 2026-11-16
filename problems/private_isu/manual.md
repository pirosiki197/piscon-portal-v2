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

## 手元の Incus で動かす場合

ポータルを Incus で動かすと、問題サーバーは手元の Incus のコンテナになります。

- サーバーには、Incus を動かすマシンから SSH します。public IP と private IP には、どちらもコンテナのアドレスが表示されます
- サーバーは CPU 2 コアとメモリ 4GiB に制限されています。ただし、`free` などに表示されるのはホスト全体のメモリです
- `/var/lib/piscon/ready` は消さないでください。消すとベンチと停止・起動ができなくなります。消してしまったら `sudo touch /var/lib/piscon/ready` で戻せます
