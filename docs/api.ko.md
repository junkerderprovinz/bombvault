# API와 연동

BombVault에는 스크립트, 대시보드, 홈 자동화를 위한 작은 HTTP API가 있습니다. 대시보드에 보이는 내용을 읽고 백업을 시작할 수 있습니다. 복원, 백업 삭제, 설정 같은 나머지는 웹 화면에 남습니다.

## 토큰 {#tokens}

로그인 비밀번호가 없어도 모든 요청에는 API 토큰이 필요합니다. **설정, 시스템, API 토큰**에서 만듭니다.

1. 토큰을 어디에 쓰는지 알 수 있는 이름을 입력합니다. 예를 들어 "Home Assistant"나 "Uptime Kuma"입니다.
2. 토큰이 백업을 시작해야 하면 **백업 시작 허용**을 켭니다. 끄면 읽기만 할 수 있습니다.
3. **토큰 만들기**를 누릅니다. 토큰은 한 번만 표시됩니다. BombVault는 지문만 보관하므로 지금 복사하세요.

토큰은 헤더로 보냅니다. `Authorization: Bearer <token>` 또는 `X-API-Key: <token>`입니다. 토큰은 `bvapi_`로 시작합니다. 토큰은 API만 엽니다. MCP 키는 여기서 통하지 않고, 토큰은 MCP에 통하지 않습니다.

토큰마다 타일이 있어 이름, 백업 시작 가능 여부, 마지막 네 글자, 마지막 사용 시각과 위치, 오늘의 호출 수를 보여 줍니다. 타일에서 이름을 바꾸고, 권한을 바꾸고, 교체하거나 취소할 수 있습니다. **로그**는 그 토큰이 시작한 백업과 최근 호출을 보여 줍니다. BombVault 설정을 백업에서 복원하면 모든 토큰이 취소됩니다. 백업에 나중에 취소한 토큰이 들어 있을 수 있기 때문입니다.

로그인 비밀번호가 없으면 웹 화면을 열 수 있는 사람은 누구나 토큰도 만들 수 있습니다. 공개된 것처럼 보이는 이름으로 BombVault를 열었고 비밀번호도 없다면, 그 주소에서는 토큰을 만들 수 없습니다. [MCP 키](mcp.md#switch-on)와 같은 규칙입니다.

## 엔드포인트 {#endpoints}

| 경로 | 반환하거나 하는 일 | 토큰 |
|---|---|---|
| `GET /api/v1/health` | 버전, 인스턴스 이름, 백업 실행 여부, 이 토큰이 할 수 있는 일 | 읽기 |
| `GET /api/v1/status` | 영역별 보호 상태: 마지막 성공 백업, 예상 간격, 점검, 다음 예약 실행 | 읽기 |
| `GET /api/v1/activity` | 지금 실행 중인 작업과 단계, 진행률 | 읽기 |
| `GET /api/v1/items` | 보호 중인 모든 항목과 일정, 백업이 멈추는 것, 마지막 백업. `?domain=`으로 한 영역만 | 읽기 |
| `GET /api/v1/runs` | 실행 기록(최신순). 필터 `limit`, `domain`, `item`, `status`, `kind`, `since` | 읽기 |
| `GET /api/v1/anomalies` | 이상 징후와 열린 항목 요약. 필터 `state`, `severity`, `domain`, `limit` | 읽기 |
| `GET /api/v1/anomalies/{id}` | 이상 징후 하나 | 읽기 |
| `GET /api/v1/storage/{domain}` | 영역의 각 저장소에 대한 크기 기록, 주간 증가량, 여유 공간 | 읽기 |
| `POST /api/v1/backups` | 항목 하나(`{"domain":"containers","item":"plex"}`) 또는 영역 전체(`{"domain":"vms"}`)를 백업 | 시작 |
| `POST /api/v1/backups/everything` | Backup Everything 실행 | 시작 |
| `POST /api/v1/runs/{id}/cancel` | 이 토큰이 시작한 실행 중인 백업 취소 | 시작 |

영역은 `containers`, `vms`, `files`, `zfs`, `flash`, `config`입니다. 시간은 Unix 초입니다. 응답은 같은 이름의 [MCP 도구](mcp.md#tools)와 같아서 둘이 어긋나지 않습니다.

여기서 시작한 백업은 웹 화면에서 시작한 백업과 같습니다. 실행 중인 컨테이너는 백업이 끝날 때까지 멈춥니다. 요청은 바로 돌아오고, 진행 상황은 `/api/v1/activity`와 `/api/v1/runs`에서 볼 수 있습니다.

## 예시 {#examples}

```sh
# 백업 상태는?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# 컨테이너 하나를 지금 백업.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

BombVault 자체 서명 인증서를 쓴다면 `--cacert bombvault-cert.pem`(MCP 카드의 **인증서 내려받기**로 받는 파일)을 붙이거나, 믿을 수 있는 네트워크에서는 `-k`를 붙입니다.

## 오류와 제한 {#errors}

오류는 `{"error": {"code": "...", "message": "..."}}` 형태로, 맞는 상태 코드와 함께 돌아옵니다.

| 상태 | 코드 | 의미 |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | 인수가 없거나 틀림 |
| 401 | `no_token`, `invalid_token` | 토큰이 없거나 활성 토큰이 아님 |
| 403 | `not_permitted` | 토큰이 읽기 전용이거나 그 실행을 시작한 토큰이 아님 |
| 404 | `not_found` | 그런 항목, 실행, 이상 징후가 없음 |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | 다른 작업이 실행 중이거나, 영역이 꺼져 있거나, 할 일이 없음 |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | 제한 때문에 요청이 보류됨. 다시 시도할 시점은 `Retry-After`가 알려 줌 |

시작에는 [MCP를 통한 시작](mcp.md#starting-backups)과 같은 제한이 적용됩니다. 토큰당 시간당 12회, 같은 항목의 두 시작 사이 15분, 한 항목당 24시간에 최대 4회, 그리고 보존 보호입니다. 뒤의 세 가지는 MCP와 API를 통한 시작을 함께 셉니다. 토큰 하나는 분당 120번 요청할 수 있습니다. 한 주소에서 다섯 번 실패하면 그 주소는 1분 동안 잠깁니다.

## OpenAPI {#openapi}

BombVault는 이 경로들의 설명을 `/api/v1/openapi.json`(OpenAPI 3.1)에서 제공합니다. 토큰은 필요 없습니다. Swagger UI, Postman, 코드 생성기에 불러오세요.
