# runner

runner をインストールし、systemd のサービスとして起動します。
GitHub Releases から対象アーキテクチャの runner バイナリを取得してインストールします。

`runner.git_tag` に指定された GitHub Release tag のバイナリをインストールします。

`problem.bundle_dir` の問題バンドルを `/srv/runner/bundle` にコピーし、runner がそのマニフェストを読むように設定します。
コピーした後に `runner check` で問題バンドルを検証します。
コピーは既存のファイルを消さないので、問題バンドルから消したファイルは runner ホストに残ります。
