# API와 연동

BombVault에는 스크립트, 대시보드, 홈 오토메이션을 위한 작은 HTTP API가 있습니다. 개요가 보여 주는 것과 같은 내용을 읽을 수 있고 백업을 시작할 수 있습니다. 복원, 백업 삭제, 설정 같은 나머지는 모두 웹 인터페이스에서만 합니다.

## 토큰 {#tokens}

로그인 비밀번호가 없어도 모든 요청에는 API 토큰이 필요합니다. **설정, 연동, API 토큰**에서 만드세요.

1. 토큰을 쓸 곳을 알 수 있는 이름을 입력하세요. 예를 들어 "Home Assistant"나 "Uptime Kuma"입니다.
2. 토큰이 백업을 시작할 수 있어야 한다면 **백업 시작 허용**을 켜세요. 켜지 않으면 토큰은 읽기만 할 수 있습니다.
3. **토큰 만들기**를 누르세요. 토큰은 한 번만 표시됩니다. BombVault는 지문만 보관하므로 지금 복사해 두세요.

토큰은 헤더로 보냅니다. `Authorization: Bearer <token>` 또는 `X-API-Key: <token>` 중 하나입니다. 토큰은 `bvapi_`로 시작합니다. 토큰은 API만 열 수 있습니다. MCP 키는 여기서 동작하지 않고, 토큰은 MCP에서 동작하지 않습니다.

토큰마다 이름, 백업을 시작할 수 있는지, 마지막 네 글자, 마지막으로 쓰인 시각과 위치, 오늘의 호출 수가 적힌 타일이 있습니다. 타일에서 이름을 바꾸고, 권한을 바꾸고, 교체하거나 폐기할 수 있습니다. **로그**는 그 토큰이 시작한 백업과 최근 호출을 보여 줍니다. 백업에서 BombVault 설정을 복원하면 모든 토큰이 폐기됩니다. 백업에 나중에 폐기한 토큰이 들어 있을 수 있기 때문입니다.

로그인 비밀번호가 없으면 웹 인터페이스를 열 수 있는 누구나 토큰도 만들 수 있습니다. BombVault를 공개 주소처럼 보이는 이름으로 열고 비밀번호도 없다면 그 주소에서는 토큰을 만들 수 없습니다. [MCP 키](mcp.md#switch-on)와 같은 규칙입니다.

## 엔드포인트 {#endpoints}

| 경로 | 반환하거나 하는 일 | 토큰 |
|---|---|---|
| `GET /api/v1/health` | 버전, 인스턴스 이름, 백업이 실행 중인지, 이 토큰이 할 수 있는 일 | 읽기 |
| `GET /api/v1/status` | 도메인별 보호 상태: 마지막 성공한 백업, 예상 간격, 확인, 다음 예정된 실행 | 읽기 |
| `GET /api/v1/activity` | 지금 실행 중인 것과 단계, 백분율 | 읽기 |
| `GET /api/v1/items` | 보호되는 모든 항목과 그 일정, 백업이 멈추는 것, 마지막 백업. `?domain=`으로 도메인 하나만 | 읽기 |
| `GET /api/v1/runs` | 실행 기록, 최신 순. 필터는 `limit`, `domain`, `item`, `status`, `kind`, `since` | 읽기 |
| `GET /api/v1/anomalies` | 이상 징후와 열린 것의 요약. 필터는 `state`, `severity`, `domain`, `limit` | 읽기 |
| `GET /api/v1/anomalies/{id}` | 이상 징후 하나 | 읽기 |
| `GET /api/v1/storage/{domain}` | 크기 기록, 주당 증가량, 도메인의 각 저장소의 여유 공간 | 읽기 |
| `POST /api/v1/backups` | 항목 하나(`{"domain":"containers","item":"plex"}`) 또는 도메인 전체(`{"domain":"vms"}`)를 백업 | 시작 |
| `POST /api/v1/backups/everything` | 전체 백업 패스를 실행 | 시작 |
| `POST /api/v1/runs/{id}/cancel` | 이 토큰이 시작한 실행 중인 백업을 취소 | 시작 |

도메인은 `containers`, `vms`, `files`, `zfs`, `flash`, `config`입니다. 시각은 유닉스 초입니다. 응답은 이름이 같은 [MCP 도구](mcp.md#tools)의 응답과 같으므로 둘이 보조를 맞춥니다.

시작한 백업은 웹 인터페이스가 시작하는 백업과 같습니다. 실행 중인 컨테이너는 백업이 끝날 때까지 멈춥니다. 요청은 바로 반환되고, 진행 상황은 `/api/v1/activity`와 `/api/v1/runs`에서 볼 수 있습니다.

## 예시 {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

BombVault 자체 서명 인증서를 쓰는 경우 `--cacert bombvault-cert.pem`(MCP 카드의 **인증서 내려받기**로 받는 파일)을 추가하거나, 신뢰하는 네트워크에서는 `-k`를 추가하세요.

## 오류와 제한 {#errors}

오류는 `{"error": {"code": "...", "message": "..."}}` 형태로 알맞은 상태 코드와 함께 돌아옵니다.

| 상태 | 코드 | 의미 |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | 인수가 없거나 잘못되었습니다 |
| 401 | `no_token`, `invalid_token` | 토큰이 없거나 활성 토큰이 아닙니다 |
| 403 | `not_permitted`, `forbidden_origin` | 토큰이 읽기만 할 수 있거나, 그 토큰이 시작한 실행이 아니거나, 요청이 다른 출처의 페이지에서 왔습니다 |
| 404 | `not_found` | 그런 항목, 실행, 이상 징후가 없습니다 |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | 다른 것이 실행 중이거나, 도메인이 꺼져 있거나, 할 일이 없습니다 |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | 제한이 요청을 막고 있습니다. `Retry-After`가 언제 다시 시도할지 알려 줍니다 |

시작은 [MCP를 통한 시작](mcp.md#starting-backups)과 같은 제한을 따릅니다. 토큰당 시간당 12회, 같은 항목을 두 번 시작하는 사이에 15분, 한 항목은 24시간에 최대 4회 시작, 그리고 보존 보호입니다. 마지막 세 가지는 MCP, API, Home Assistant를 통한 시작을 합쳐서 셉니다. 토큰 하나는 분당 120개의 요청을 보낼 수 있습니다. 한 주소에서 다섯 번 실패하면 1분 동안 차단됩니다.

## OpenAPI {#openapi}

BombVault는 이 경로들의 설명을 `/api/v1/openapi.json`(OpenAPI 3.1)에서 제공합니다. 토큰은 필요 없습니다. Swagger UI, Postman, 코드 생성기에 불러오세요.

## Home Assistant {#home-assistant}

BombVault는 MQTT 디스커버리를 통해 Home Assistant에 기기로 나타날 수 있습니다. Home Assistant에 MQTT 연동과 브로커가 필요하며, 예를 들면 Mosquitto 애드온입니다. 사용자 지정 컴포넌트는 필요 없습니다.

1. BombVault에서 **설정, 연동**을 열고 **서비스** 카드의 **Home Assistant**를 켜세요. 그 아래에 브로커 입력란이 나타납니다.
2. 브로커의 주소와 포트를 입력하고, 요구한다면 사용자 이름과 비밀번호도 입력하세요. 브로커가 TLS를 받는다면, 보통 포트 8883인데, **TLS 사용**을 켜세요. 브로커의 인증서는 입력한 주소에 유효해야 합니다. 주소, 포트, 사용자 이름을 바꾸면 비밀번호를 다시 입력하세요. BombVault는 저장된 비밀번호를 다른 브로커나 사용자에게 넘기지 않습니다.
3. **저장**을 누르세요. 연결이 되면 줄에 표시됩니다.

기기 이름은 BombVault이거나, 인스턴스 이름이 괄호 안에 붙은 BombVault이며, 다음 엔티티를 갖습니다.

| 엔티티 | 보여 주는 것 |
|---|---|
| Status | `ok`, `warning`, `overdue`, `failed`, `off`. 켜져 있는 도메인 중 가장 나쁜 상태. 첫 백업을 기다리는 도메인은 그 백업의 예정 시점이 지나야 계산에 들어가고, 최신 상태인 도메인이 있어야만 상태가 `ok`가 됩니다. `overdue`는 개요와 같이 도메인의 마지막 백업이 간격의 두 배보다 오래되었다는 뜻입니다 |
| Running job | 지금 실행 중인 것, 또는 `idle` |
| Open anomalies | 열린 이상 징후의 수 |
| Next scheduled backup | 다음 예정된 백업이 시작되는 시점 |
| *Domain* last backup | 도메인의 마지막 성공한 백업이 실행된 시점 |
| *Domain* last result | 마지막 백업이 어떻게 끝났는지 |
| *Domain* repository free space | 기본 저장소가 있는 곳의 여유 공간. BombVault가 읽을 수 있을 때 |
| Back up *domain* | 도메인 전체를 백업하는 버튼 |

켜져 있는 도메인마다 자기 엔티티가 생기고, 끈 도메인은 그것들을 잃습니다. 버튼은 **버튼으로 백업 시작**을 켜면 나타납니다. 새로 설치하면 꺼져 있습니다. 버튼은 [API를 통한 시작](#errors)과 같은 제한을 따릅니다. 거기에 더해 BombVault는 도메인마다 한 번에 하나의 누름만 처리하고 분당 여섯 번을 넘기지 않으며, 브로커가 유지된 메시지로 보관해 둔 누름은 무시합니다. 브로커에 게시할 수 있는 사람은 누구나 버튼을 누를 수 있으므로 브로커에 비밀번호를 설정하세요.

BombVault는 15초마다 자기 상태를 읽고, 바뀐 것이 있으면 `<prefix>/<node>/state` 아래에 JSON으로 게시합니다. prefix는 바꾸지 않으면 `bombvault`이고, node는 BombVault가 한 번 고르는 짧은 ID입니다. 디스커버리 메시지는 Home Assistant의 기본 prefix인 `homeassistant`로 갑니다. 둘 다 유지 메시지입니다. 마지막 유언(last will)은 BombVault가 알리지 않고 멈추면 기기를 사용 불가로 표시합니다. 연결을 끄면 Home Assistant에서 기기와 그 엔티티가 제거됩니다.

### 예시: 카드와 알림 {#home-assistant-example}

Home Assistant는 이름으로 엔티티 ID를 만들므로, BombVault 기기는 `sensor.bombvault_status` 같은 ID를 받습니다. 괄호 안에 인스턴스 이름이 있으면 ID에도 그것이 붙으니 먼저 **Developer Tools, States**에서 확인하세요. 대시보드용 카드입니다.

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

상태가 `failed` 또는 `overdue`가 되면 휴대폰에 알려 주는 자동화입니다.

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

**설정, 연동**의 **서비스** 카드에 있는 **/metrics 노출**은 백업 상태, 크기, 타임스탬프를 Prometheus, Grafana, Uptime Kuma용으로 웹 인터페이스 포트의 `/metrics`에서 제공합니다. 기본으로 꺼져 있습니다. **스크레이프 토큰 (선택)**을 쓰면 스크레이퍼가 베어러 토큰을 보내야 합니다. 레이블에는 비밀 정보도 경로도 들어 있지 않습니다.

## 대시보드 위젯 {#dashboard-widget}

**설정, 연동**의 **대시보드 위젯** 카드는 읽기 전용 토큰을 만들고 Homepage, Organizr, Heimdall처럼 iframe을 보여 주는 모든 대시보드를 위한 주소를 줍니다. 그 주소는 `/widget`에서 실시간 활동 로그만 있는 작은 페이지를 엽니다. 토큰은 그 로그만 허용하고 다른 것은 허용하지 않으며, **비활성화**를 누르면 바로 폐기됩니다. 삽입되는 페이지는 영어로만 나옵니다.

## 네트워크에서 BombVault 찾기 {#mdns}

BombVault는 로컬 네트워크에 mDNS로 웹 인터페이스를 알립니다. Bonjour와 Avahi의 바탕이 되는 프로토콜입니다. 그러면 브라우저는 `https://bombvault.local:3443`으로, `HTTP_ONLY`일 때는 `http://bombvault.local:3000`으로 접속할 수 있고, 서비스 브라우저는 서브타입이 `_bombvault`인 웹 서비스로 목록에 표시합니다. TXT 레코드에는 버전과 경로가 들어 있습니다. 스위치는 **설정, 연동**의 **서비스** 카드에 있는 **네트워크에서 찾기**이며 기본으로 켜져 있습니다. 같은 이름을 이미 다른 기기가 쓰고 있으면 BombVault는 `bombvault-2.local`처럼 다음 이름을 가져가며, 카드에 받은 주소가 표시됩니다. BombVault가 멈추거나 알림을 끄면 네트워크에 그렇게 알려 주므로 브라우저는 항목을 바로 지웁니다.

알림이 네트워크에 닿는지는 컨테이너가 어떻게 연결되어 있는지에 달려 있습니다.

- **bridge**, Unraid 템플릿의 기본값: 알림이 Docker의 네트워크 안에 머물러 LAN의 누구도 보지 못합니다. 이전처럼 호스트의 주소로 BombVault를 여세요.
- **br0** 또는 다른 macvlan이나 ipvlan 네트워크: 컨테이너가 LAN에 자기 주소를 가지므로 알림이 닿습니다.
- **host**: 알림이 호스트의 인터페이스로 나가며, Unraid 자체의 알림 옆에 놓입니다. Docker와 libvirt의 브리지는 건너뜁니다.

IPv4 주소만 알립니다.
