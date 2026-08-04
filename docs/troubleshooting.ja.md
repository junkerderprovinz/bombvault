# トラブルシューティング

短い FAQ です。SSH 経由の VM に関するホスト側トラブルシューティングの完全な表（permission-denied、ホスト鍵検証、テンプレート変数の欠落など）については、GitHub の [SSH 経由の VM バックアップガイド](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)を参照してください。

## 何かが正しく配線されていない

Web UI で `/spike` を開きます。ホスト統合チェックはすべてのマウントと CLI（Docker ソケット、libvirt、restic、qemu-img、rclone）をプローブし、欠けている部分を報告します。バグだと決めつける前に、まずここから始めてください: マウントの欠落や到達不能なホストはすぐに表示されます。

## Web UI に到達できない

BombVault はデフォルトでポート `3443`（自己署名証明書）で HTTPS を提供するため、`https://<your-unraid-ip>:3443` を開いてください。自己署名証明書の警告を受け入れるか、独自の証明書を持つリバースプロキシの背後に BombVault を置いてください。`HTTP_ONLY=true` で実行している場合は、代わりにポート `3000` でプレーン HTTP を提供します（TLS を終端するプロキシの背後での使用向け）。

## APP_KEY を失った

`APP_KEY` は restic リポジトリのパスワードを導出します。それ（および暗号化キー・リカバリーキット）がなければ、暗号化されたバックアップは復元できません。これが、ダッシュボードがリカバリーキットをダウンロードするようせがむ理由です。[オフサイトと復旧](offsite-recovery.md)を参照してください。`openssl rand -hex 32` で鍵を生成し、バックアップに頼る前にサーバーとは別の場所に保管してください。

## VM バックアップが接続できない

VM バックアップは、マウントではなく SSH 経由で libvirt と通信します。

- ホストで SSH が有効になっていること、そして BombVault の公開鍵が `/root/.ssh/authorized_keys` で承認されていることを確認してください（Settings, System, VM Backup over SSH に鍵と **Test connection** ボタンが表示されます）。
- カスタムの `br0.x` ネットワークでは、`LIBVIRT_HOST` を Unraid の LAN IP に設定してください（そこではコンテナは `host.docker.internal` 経由でホストに到達できません）。**Settings, Docker, Host access to custom networks** を有効にしてください。
- Unraid の SSH ポートを変更した場合は、`LIBVIRT_SSH_PORT` を一致させてください。
- 完全なステップバイステップの診断（到達性テスト、VLAN のルーティング、`Permission denied (publickey)`、`Host key verification failed`）は、[SSH 経由の VM バックアップガイド](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)にあります。

## ライブ VM スナップショットが実行されなかった

ライブスナップショットには、VM 内にインストールされた qemu ゲストエージェントと、ディスクが `/mnt/user` ではなく `/mnt/cache`（または `/mnt/diskX`）にあることが必要です。シャットオフ状態の VM では、ライブは自動でグレースフルにフォールバックします。グレースフルバックアップは VM をシャットダウンし、ディスクをバックアップしてから再起動するため、常に一貫しています。

## バックアップが "repository is already locked" で失敗した

これはたいてい、コンテナが操作の途中で更新または再起動されたときに残された、孤立した restic ロックです。BombVault は証明可能な孤立ロックを検出し、強制解除して一度だけ自動でリトライします。それでも続く場合は、影響を受けるドメインに対して **Settings, Integrity & maintenance, Unlock** を使って、古いロックを手動で削除してください。本当の問題は、隠される代わりにきちんと表面化します。

## バックアップの後にオフサイトコピーが行われなかった

オフサイト複製は設計上ベストエフォートのため、オフサイトの不調でローカルバックアップが失敗することはありません。そのドメインのオフサイトスケジュール（Settings, Schedules）を確認してください: 空欄のスケジュールは各ローカルバックアップの後に複製し、頻度を設定すると少ない頻度で送ります。オンデマンドの実行には Off-site タブの **今すぐ複製** を使い、ダッシュボードの複製インジケーターを見てください。

## 復元が始まる前に中止された

何かを停止または削除する前に、復元はプリフライトの競合チェックを実行します: コンテナの静的 IP と公開ホストポートが空いていることを検証します。別のコンテナがすでにいずれかを保持している場合、途中で終わった復元を残す代わりに、明確で実行可能なメッセージとともに中止します。競合するポートまたは IP を空けてから、リトライしてください。

## プレーンエクスポートがファイルを書き込む代わりに失敗した

age 暗号化がオン（設定）で、有効な受信者が設定されていない場合、エクスポートは平文を書き込む代わりに、明確なエラーで失敗します。有効な受信者（age 公開鍵または SSH 公開鍵）を追加するか、エクスポートを平文にするつもりなら暗号化をオフにしてください。[機能](features.md)を参照してください。

## コンテナが再起動を繰り返す、または unhealthy に見える

BombVault は自身の `/api/health` から healthy/unhealthy を報告します。auto-heal ツール（Autoheal など）は、エンジンが動かなくなった場合にそれを自動で再起動できます。根本原因については、コンテナログと `/spike` レポートを確認してください。

## それでも解決しない？

- 完全な[設定](configuration.md)と[オフサイトと復旧](offsite-recovery.md)のページを読んでください。
- [Unraid サポートスレッド](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)で質問してください。
- [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues) を開いてください。
