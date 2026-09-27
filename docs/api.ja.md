# API と連携

BombVault には、スクリプト、ダッシュボード、ホームオートメーション向けの小さな HTTP API があります。ダッシュボードに表示される内容を読み取り、バックアップを開始できます。復元、バックアップの削除、設定などそれ以外の操作は、ウェブ画面に残ります。

## トークン {#tokens}

ログインパスワードが設定されていなくても、すべてのリクエストに API トークンが必要です。**設定、システム、API トークン** で作成します。

1. トークンをどこで使うかが分かる名前を入力します。たとえば「Home Assistant」や「Uptime Kuma」です。
2. トークンでバックアップを開始したい場合は **バックアップの開始を許可** をオンにします。オフのままだと読み取りだけできます。
3. **トークンを作成** をクリックします。トークンは一度だけ表示されます。BombVault はその指紋しか保存しないので、今すぐコピーしてください。

トークンはヘッダーで送ります。`Authorization: Bearer <token>` または `X-API-Key: <token>` です。トークンは `bvapi_` で始まります。開けるのは API だけです。MCP キーはここでは使えず、トークンは MCP には使えません。

各トークンにはタイルがあり、名前、バックアップを開始できるかどうか、末尾 4 文字、最後に使われた日時と接続元、今日の呼び出し数が表示されます。タイルでは名前の変更、権限の変更、交換、失効ができます。**ログ** には、そのトークンが開始したバックアップと最近の呼び出しが表示されます。BombVault の設定をバックアップから復元すると、すべてのトークンが失効します。バックアップには、後で失効させたトークンが含まれている可能性があるためです。

ログインパスワードがない場合、ウェブ画面を開ける人は誰でもトークンを作成できます。公開されているように見える名前で BombVault を開き、パスワードもない場合、そのアドレスからはトークンを作成できません。[MCP キー](mcp.md#switch-on) と同じ規則です。

## エンドポイント {#endpoints}

| ルート | 返す内容または動作 | トークン |
|---|---|---|
| `GET /api/v1/health` | バージョン、インスタンス名、バックアップ実行中かどうか、このトークンにできること | 読み取り |
| `GET /api/v1/status` | 領域ごとの保護状態: 最後に成功したバックアップ、想定間隔、チェック、次の予定実行 | 読み取り |
| `GET /api/v1/activity` | いま実行中の処理とそのフェーズと進捗率 | 読み取り |
| `GET /api/v1/items` | 保護対象の各項目とそのスケジュール、バックアップで停止するもの、最後のバックアップ。`?domain=` で 1 領域に絞り込み | 読み取り |
| `GET /api/v1/runs` | 実行履歴 (新しい順)。フィルター `limit`、`domain`、`item`、`status`、`kind`、`since` | 読み取り |
| `GET /api/v1/anomalies` | 異常と未解決分のまとめ。フィルター `state`、`severity`、`domain`、`limit` | 読み取り |
| `GET /api/v1/anomalies/{id}` | 1 件の異常 | 読み取り |
| `GET /api/v1/storage/{domain}` | 領域の各リポジトリのサイズ履歴、週あたりの増加量、空き容量 | 読み取り |
| `POST /api/v1/backups` | 1 項目 (`{"domain":"containers","item":"plex"}`) または領域全体 (`{"domain":"vms"}`) をバックアップ | 開始 |
| `POST /api/v1/backups/everything` | Backup Everything を実行 | 開始 |
| `POST /api/v1/runs/{id}/cancel` | このトークンが開始した実行中のバックアップを中止 | 開始 |

領域は `containers`、`vms`、`files`、`zfs`、`flash`、`config` です。時刻は Unix 秒です。応答は同じ名前の [MCP ツール](mcp.md#tools) と同じなので、両者は常にそろっています。

ここで開始したバックアップは、ウェブ画面から開始したものと同じです。稼働中のコンテナはバックアップが終わるまで停止します。リクエストはすぐに戻り、進み具合は `/api/v1/activity` と `/api/v1/runs` で確認できます。

## 例 {#examples}

```sh
# バックアップの状況は?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# コンテナを 1 つ今すぐバックアップ。
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

BombVault 独自の自己署名証明書を使っている場合は、`--cacert bombvault-cert.pem` (MCP カードの **証明書をダウンロード** で得られるファイル) を付けるか、信頼できるネットワークでは `-k` を付けます。

## エラーと制限 {#errors}

エラーは `{"error": {"code": "...", "message": "..."}}` として、対応するステータスとともに返ります。

| ステータス | コード | 意味 |
|---|---|---|
| 400 | `invalid_argument`、`ambiguous` | 引数が足りないか誤っている |
| 401 | `no_token`、`invalid_token` | トークンがない、または有効でない |
| 403 | `not_permitted` | トークンが読み取り専用、またはその実行を開始したのがこのトークンではない |
| 404 | `not_found` | その項目、実行、異常が存在しない |
| 409 | `busy`、`domain_off`、`nothing_to_back_up`、`not_running` | 別の処理が実行中、領域がオフ、または何もすることがない |
| 429 | `throttled`、`rate_limited`、`cooldown`、`retention_guard` | 制限によって保留された。再試行できる時刻は `Retry-After` が示す |

開始には [MCP 経由の開始](mcp.md#starting-backups) と同じ制限がかかります。トークンごとに 1 時間 12 回、同じ項目の開始の間隔は 15 分、1 項目あたり 24 時間で最大 4 回、そして保持の保護です。後の 3 つは MCP と API 経由の開始を合わせて数えます。トークン 1 つで 1 分に 120 リクエストまで送れます。同じアドレスから 5 回失敗すると、そのアドレスは 1 分間ロックされます。

## OpenAPI {#openapi}

BombVault はこれらのルートの説明を `/api/v1/openapi.json` (OpenAPI 3.1) で提供します。トークンは不要です。Swagger UI、Postman、コードジェネレーターに読み込めます。
