# API と連携

BombVault には、スクリプト、ダッシュボード、ホームオートメーション向けの小さな HTTP API があります。概要に表示されるものと同じ内容を読み取れ、バックアップを開始できます。復元、バックアップの削除、設定など、それ以外はすべて Web 画面にとどまります。

## トークン {#tokens}

ログインパスワードが設定されていなくても、すべてのリクエストに API トークンが必要です。トークンは **設定、連携、API トークン** で作成します。

1. トークンをどこで使うかが分かる名前を入力します。たとえば「Home Assistant」や「Uptime Kuma」です。
2. トークンにバックアップの開始を許可したい場合は、**バックアップの開始を許可**をオンにします。オンにしないと、そのトークンは読み取りだけができます。
3. **トークンを作成**をクリックします。トークンは一度だけ表示されます。BombVault はそのフィンガープリントしか保持しないので、今すぐコピーしてください。

トークンはヘッダーで送ります。`Authorization: Bearer <token>` か `X-API-Key: <token>` のどちらかです。トークンは `bvapi_` で始まります。開けるのは API だけです。MCP キーはここでは使えず、トークンも MCP には使えません。

各トークンにはタイルがあり、名前、バックアップの開始を許可しているか、末尾 4 文字、最後に使われた日時と場所、今日の呼び出し回数が表示されます。タイルでは、名前の変更、できることの変更、トークンの置き換え、取り消しができます。**ログ**には、そのトークンが開始したバックアップと最近の呼び出しが表示されます。バックアップから BombVault の設定を復元すると、すべてのトークンが取り消されます。バックアップには、あとから取り消したトークンが含まれている場合があるからです。

ログインパスワードがないと、Web 画面を開ける人なら誰でもトークンを作成できます。公開されているように見える名前で BombVault を開いていて、パスワードが設定されていない場合、そのアドレスからはトークンを作成できません。これは [MCP キー](mcp.md#switch-on)と同じ規則です。

## エンドポイント {#endpoints}

| ルート | 返すもの、または動作 | トークン |
|---|---|---|
| `GET /api/v1/health` | バージョン、インスタンス名、バックアップが実行中かどうか、このトークンにできること | 読み取り |
| `GET /api/v1/status` | ドメインごとの保護ステータス：直近の成功したバックアップ、想定される間隔、チェック、次回のスケジュール実行 | 読み取り |
| `GET /api/v1/activity` | 今実行中のもの。フェーズと進捗率つき | 読み取り |
| `GET /api/v1/items` | 保護されているすべての項目と、そのスケジュール、バックアップが停止するもの、直近のバックアップ。`?domain=` で 1 つのドメインに絞れます | 読み取り |
| `GET /api/v1/runs` | 実行履歴を新しい順に。フィルターは `limit`、`domain`、`item`、`status`、`kind`、`since` | 読み取り |
| `GET /api/v1/anomalies` | 異常と、未処理のものの概要。フィルターは `state`、`severity`、`domain`、`limit` | 読み取り |
| `GET /api/v1/anomalies/{id}` | 1 件の異常 | 読み取り |
| `GET /api/v1/storage/{domain}` | ドメインの各リポジトリのサイズ履歴、週あたりの増加、空き容量 | 読み取り |
| `POST /api/v1/backups` | 1 つの項目（`{"domain":"containers","item":"plex"}`）またはドメイン全体（`{"domain":"vms"}`）をバックアップ | 開始 |
| `POST /api/v1/backups/everything` | 「すべてバックアップ」のパスを実行 | 開始 |
| `POST /api/v1/runs/{id}/cancel` | このトークンが開始した実行中のバックアップを中止 | 開始 |

ドメインは `containers`、`vms`、`files`、`zfs`、`flash`、`config` です。時刻は UNIX 秒です。応答は、同じ名前の [MCP ツール](mcp.md#tools)と同じなので、両者は歩調が合っています。

開始されたバックアップは、Web 画面が開始するバックアップと同じものです。実行中のコンテナは、バックアップが終わるまで停止します。リクエストはすぐに返り、進み具合は `/api/v1/activity` と `/api/v1/runs` で確認できます。

## 例 {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

BombVault 自身の自己署名証明書を使う場合は、`--cacert bombvault-cert.pem`（MCP カードの**証明書をダウンロード**で得られるファイル）を付けるか、信頼できるネットワークでは `-k` を付けます。

## エラーと制限 {#errors}

エラーは `{"error": {"code": "...", "message": "..."}}` の形で、対応するステータスとともに返ります。

| ステータス | コード | 意味 |
|---|---|---|
| 400 | `invalid_argument`、`ambiguous` | 引数が足りないか、誤っている |
| 401 | `no_token`、`invalid_token` | トークンがない、または有効でない |
| 403 | `not_permitted`、`forbidden_origin` | トークンが読み取り専用、そのトークンが開始した実行ではない、または別のオリジンのページからのリクエスト |
| 404 | `not_found` | そのような項目、実行、異常はない |
| 409 | `busy`、`domain_off`、`nothing_to_back_up`、`not_running` | ほかのものが実行中、ドメインがオフ、またはやることがない |
| 429 | `throttled`、`rate_limited`、`cooldown`、`retention_guard` | 制限によりリクエストが止められている。`Retry-After` で、いつ再試行できるかが分かる |

開始には、[MCP 経由の開始](mcp.md#starting-backups)と同じ制限があります。トークンごとに 1 時間に 12 回、同じ項目の 2 回の開始のあいだは 15 分、1 つの項目は 24 時間に最大 4 回、そして保持ガードです。後ろの 3 つは、MCP、API、Home Assistant からの開始をまとめて数えます。トークンは 1 分に 120 回のリクエストができます。同じアドレスから 5 回失敗すると、1 分間ロックアウトされます。

## OpenAPI {#openapi}

BombVault は、これらのルートの記述を `/api/v1/openapi.json`（OpenAPI 3.1）で提供します。トークンは不要です。Swagger UI、Postman、コードジェネレーターに読み込めます。

## Home Assistant {#home-assistant}

BombVault は、MQTT ディスカバリーを通じて、Home Assistant にデバイスとして現れます。Home Assistant には MQTT 連携とブローカーが必要です。たとえば Mosquitto アドオンです。カスタムコンポーネントは使いません。

1. BombVault で **設定、連携** を開き、**サービス**カードの **Home Assistant** をオンにします。ブローカー用のフィールドがその下に現れます。
2. ブローカーのアドレスとポート、そして必要ならユーザー名とパスワードを入力します。ブローカーが TLS に対応しているなら、**TLS を使う**をオンにします。通常はポート 8883 です。証明書は、入力したアドレスに対して有効である必要があります。アドレス、ポート、ユーザー名を変更したときは、パスワードをもう一度入力してください。BombVault は、保存済みのパスワードを別のブローカーやユーザーには渡しません。
3. **保存**をクリックします。接続が確立すると、その行に表示されます。

デバイスの名前は BombVault、またはインスタンス名を括弧に入れた BombVault で、次のエンティティを持ちます。

| エンティティ | 表示するもの |
|---|---|
| Status | `ok`、`warning`、`overdue`、`failed`、`off` のどれか。オンになっているドメインのうち最も悪いもの。最初のバックアップを待っているドメインは、そのバックアップの期限が来てから初めて数に入ります。ステータスが `ok` になるのは、最新の状態のドメインがある場合だけです。`overdue` は概要と同じく、ドメインの最後のバックアップが間隔の 2 倍より古いことを示します |
| Running job | 今実行中のもの、または `idle` |
| Open anomalies | 未処理の異常の数 |
| Next scheduled backup | 次のスケジュールされたバックアップが始まる時刻 |
| *ドメイン* last backup | そのドメインの直近の成功したバックアップが実行された時刻 |
| *ドメイン* last result | 直近のバックアップの終わり方 |
| *ドメイン* repository free space | プライマリリポジトリがある場所の空き容量（BombVault が読み取れる場合） |
| Back up *ドメイン* | ドメイン全体をバックアップするボタン |

オンになっているドメインごとに専用のエンティティができ、オフにしたドメインのエンティティは消えます。ボタンは、**ボタンでバックアップを開始**をオンにすると現れます。新規インストールではオフです。[API 経由の開始](#errors)と同じ制限に従います。それに加えて、BombVault はドメインごとに一度に 1 回の押下だけを扱い、1 分に 6 回までとし、ブローカーが保持メッセージとして残していた押下は無視します。ブローカーに発行できる人なら誰でもボタンを押せるので、ブローカーにはパスワードを設定してください。

BombVault は 15 秒ごとに状態を読み取り、変化があったときに、`<prefix>/<node>/state` の下に JSON として発行します。プレフィックスは変更しなければ `bombvault`、ノードは BombVault が一度だけ選ぶ短い ID です。ディスカバリーメッセージは、Home Assistant の標準プレフィックス `homeassistant` に送られます。どちらも保持されます。BombVault が何も言わずに停止した場合は、遺言メッセージ（last will）によってデバイスが利用不可になります。連携をオフにすると、デバイスとそのエンティティが Home Assistant から削除されます。

### 例：カードとアラート {#home-assistant-example}

Home Assistant は名前からエンティティ ID を作るので、BombVault のデバイスには `sensor.bombvault_status` のような ID が付きます。インスタンス名が括弧に入っている場合は、ID にもそれが含まれるので、先に **Developer Tools, States** で確認してください。ダッシュボード用のカードは次のとおりです。

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

ステータスが `failed` または `overdue` に変わったときにスマートフォンへ知らせるオートメーションです。

```yaml
alias: BombVault needs you
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to:
      - failed
      - overdue
action:
  - service: notify.mobile_app_my_phone
    data:
      message: "BombVault status: {{ trigger.to_state.state }}. Open the overview."
```

## Prometheus {#metrics}

**設定、連携** の**サービス**カードにある **/metrics を公開**は、Prometheus、Grafana、Uptime Kuma 向けに、バックアップの状態、サイズ、タイムスタンプを、Web 画面のポートの `/metrics` で提供します。標準ではオフです。**スクレイプトークン（任意）**を設定すると、スクレイパーはベアラートークンを送るようになります。ラベルにはシークレットもパスも含まれません。

## ダッシュボードウィジェット {#dashboard-widget}

**設定、連携** の**ダッシュボードウィジェット**カードは、読み取り専用のトークンを生成し、Homepage、Organizr、Heimdall など iframe を表示するダッシュボード向けのアドレスを渡します。アドレスを開くと、`/widget` に、ライブのアクティビティログだけを表示する小さなページが開きます。トークンが許可するのはそのログだけで、**無効化**ですぐに取り消せます。埋め込まれるページは英語のみです。

## ネットワーク上で BombVault を見つける {#mdns}

BombVault は、Bonjour や Avahi の基盤となるプロトコルである mDNS で、Web 画面をローカルネットワーク上に知らせます。ブラウザーからは `https://bombvault.local:3443` で、`HTTP_ONLY` の場合は `http://bombvault.local:3000` で届き、サービスブラウザーにはサブタイプ `_bombvault` の Web サービスとして一覧されます。TXT レコードには、バージョンとパスが入っています。スイッチは **設定、連携** の**サービス**カードにある**ネットワークで見つける**で、標準ではオンです。同じ名前をすでにほかの機器が使っている場合、BombVault は `bombvault-2.local` のように別の名前を取り、カードには取得したアドレスが表示されます。BombVault が停止したときや、通知をオフにしたときは、ネットワークにそれを伝えるので、ブラウザーは項目をすぐに削除します。

通知があなたのネットワークに届くかどうかは、コンテナの接続方法によります。

- **bridge**（Unraid テンプレートの標準）：通知は Docker のネットワークの中にとどまり、LAN の誰にも見えません。これまでどおり、ホストのアドレスで BombVault を開いてください。
- **br0** またはほかの macvlan や ipvlan ネットワーク：コンテナが LAN 上に専用のアドレスを持ち、通知が届きます。
- **host**：通知は、Unraid 自身のものの隣で、ホストのインターフェースから出ます。Docker と libvirt のブリッジは除かれます。

通知されるのは IPv4 アドレスだけです。
