# 実践例

ここに挙げる例は、どれも実際の作業を最初から最後まで、名前や数字を入れて順に説明しています。機能を説明するページの中にあるので、詳しい内容がすぐ横で確認できます。

## はじめに {#start}

- [最初のコンテナバックアップを最初から最後まで](getting-started.md#example-first-backup)：インストールからリカバリーキットまで。

## ストレージとオフサイト {#storage}

- [Backblaze B2 へのオフサイトコピー](offsite-recovery.md#example-s3)：バケット、そのバケット用のキー、最初のコピー。Cloudflare R2 でも同じ手順です。
- [自分で動かす rest-server](offsite-recovery.md#example-rest-server)：削除を拒否するオフサイトコピー。`docker run` の行と改ざんテストも載せています。
- [Balanced が残すもの](offsite-recovery.md#example-retention)：毎晩のバックアップを 400 日続けた場合の保持ルールを、バックアップごとに追います。
- [コンテナを NAS へ移す](offsite-recovery.md#example-own-folder)：セクションの専用フォルダーの場所を変えたとき、すでに作ったバックアップがどうなるか。

## バックアップとスケジュール {#backups}

- [うまくかみ合う 3 つのプラン](schedules.md#example)：夜のフルバックアップ、週 1 回の VM、1 つの項目だけ 6 時間ごとに動く専用プラン。
- [誰かが観ているあいだ Plex は待つ](backups.md#example-plex)：映画を中断しない、スケジュールされたバックアップ。
- [Unraid スナップショットが 2 つある VM](vm-backup-ssh-setup.md#example-unraid-snapshots)：スナップショットを残したままのライブバックアップと、元の隣への復元。
- [2 台目の Unraid へのレプリカ](zfs-datasets.md#example-replica)：いつでも読める ZFS のコピーと、そこから 1 つのファイルを取り出す 3 つの方法。

## データを取り戻す {#restore}

- [ファイルを 1 つ取り戻す](restore.md#example-single-file)：昨夜のコンテナバックアップから 1 つのファイルを取り出します。
- [ZFS バックアップから 1 つのファイル](zfs-datasets.md#example-zfs-file)：データセットでも同じことをします。
- [リカバリーキットから新しいサーバーへ](offsite-recovery.md#example-emergency)：テンプレートに古い `APP_KEY` がなくても、止まったサーバーを新しいマシンで復活させます。

## 状況を知らせてもらう {#staying-informed}

- [スマートフォンで Gotify](notifications.md#example-gotify)：失敗は音で、成功は静かに届きます。
- [Webhook として ntfy](notifications.md#example-ntfy)：アカウントなしでメッセージを受け取ります。

## 複数のサーバー {#several-servers}

- [2 台の Unraid をペアリングする](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end)：互いのコピーを預かり合い、どちらも相手のバックアップを削除できない 2 台のサーバー。
- [Home Assistant のカードとアラート](api.md#home-assistant-example)：BombVault をダッシュボードに表示し、バックアップが失敗したらメッセージを受け取ります。
