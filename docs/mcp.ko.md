# MCP 서버

BombVault에는 Model Context Protocol(MCP) 서버가 내장되어 있습니다. MCP는 Claude Code, Claude Desktop 같은 AI 어시스턴트가 외부 도구에 접근할 때 쓰는 프로토콜입니다. 이를 통해 어시스턴트는 백업 상태를 읽을 수 있고, 허용하면 백업을 시작하거나 자신이 시작한 백업을 취소할 수 있습니다. 키를 만들기 전까지 서버는 꺼져 있습니다. 활성 키가 없으면 엔드포인트 `/mcp`는 모든 요청에 `404`로 답합니다.

## 어시스턴트가 할 수 있는 일과 없는 일 {#tools}

| 도구 | 하는 일 | 종류 |
|---|---|---|
| `get_health` | 버전, 인스턴스 이름, 백업이 실행 중인지, 이 키에 허용된 것 | 읽기 |
| `get_status` | 도메인별 보호 상태: 마지막 성공 백업, 예상 간격, 검증과 오프사이트 점검, 다음 예약 실행 | 읽기 |
| `get_coverage` | BombVault가 보호하는 것과 보호하지 않는 것, 각각의 이유 | 읽기 |
| `list_items` | 보호되는 모든 컨테이너, VM, 폴더 세트, 플래시 드라이브, 앱 설정. 일정, 백업 시 멈추는 것, 마지막 백업과 걸린 시간 포함. 데이터베이스 컨테이너는 마지막 덤프도 보여 줍니다. ZFS 데이터 세트도 마지막 점검 결과와 함께 보여 줍니다 | 읽기 |
| `list_runs` | 실행 기록, 최신순. 도메인, 항목, 상태, 종류, 시간으로 거를 수 있음 | 읽기 |
| `list_restore_points` | 한 항목의 기본 저장소에 있는 복원 지점, 컨테이너라면 데이터베이스 덤프도. ZFS 데이터 세트는 백업마다 복원 지점이 하나이고, 그 아래 모든 데이터 세트의 스냅숏이 들어 있습니다 | 읽기 |
| `get_activity` | 지금 실행 중인 것, 단계와 진행률 포함 | 읽기 |
| `get_storage_stats` | 한 도메인의 기본 저장소 크기 기록과 주간 증가량 | 읽기 |
| `list_anomalies` | BombVault가 백업에서 알아챈 이상 징후. 상태, 심각도, 도메인으로 거를 수 있고 열려 있는 것의 요약 포함 | 읽기 |
| `get_anomaly` | 그중 하나와 확인할 때 남긴 메모 | 읽기 |
| `start_backup` | 한 항목을 바로 백업합니다 | 시작 |
| `start_domain_backup` | 한 도메인의 보호 대상 항목을 모두 백업합니다 | 시작 |
| `start_backup_everything` | Backup Everything 한 바퀴를 실행합니다 | 시작 |
| `cancel_backup` | 이 키가 시작한 실행 중인 백업을 취소합니다 | 취소 |

다음은 웹 인터페이스에 남습니다: 모든 종류의 복원(데이터베이스 덤프의 다운로드, 저장, 가져오기 포함), 백업 삭제, prune, unlock, 점검과 훈련, 오프사이트 복제, 설정, 자격 증명과 MCP 키, 그리고 일정이나 웹 인터페이스 또는 다른 키가 시작한 백업의 취소. 이상 징후를 확인하거나 예상된 것으로 표시하는 일도 마찬가지로 **이상 징후** 페이지에서 합니다. 이유는 도구의 응답에 서버에서 온 이름과 오류 메시지가 들어 있고, 그중 어느 것에든 어시스턴트를 조종하려고 쓴 글이 섞여 있을 수 있기 때문입니다. 그런 글에 넘어간 어시스턴트가 할 수 있는 일은 최악의 경우에도 아래 제한 안에서 백업을 시작하거나 자신이 시작한 백업을 취소하는 것뿐입니다.

항목의 기본 저장소가 다른 곳(S3, REST, SFTP, rclone)에 있으면 `list_restore_points`가 그곳에 연결하므로 호출에 시간이 걸릴 수 있습니다. 오프사이트 사본은 MCP로 나열할 수 없습니다. 이상 징후 검사가 무엇을 보는지는 [기능](features.md)에, ZFS 항목이 데이터세트마다 스냅샷을 하나씩 두는 방식은 [ZFS 데이터세트](zfs-datasets.md#contents)에 설명되어 있습니다.

## 시작한 백업이 하는 일 {#starting-backups}

어시스턴트의 백업은 웹 인터페이스가 시작하는 백업과 같습니다. 실행 중인 컨테이너는 백업이 끝날 때까지 멈추며, 함께 멈추도록 설정된 컨테이너도 같이 멈춥니다. "graceful" 방식의 VM은 종료되었다가 다시 시작됩니다. ZFS 데이터 세트는 스냅숏을 찍는 동안 그 데이터 세트에 설정된 컨테이너를 멈춥니다. 폴더 세트, 플래시 드라이브, 설정은 계속 동작합니다. 그다음 BombVault는 보존 정책을 적용하고 오프사이트 저장소로 복사할 수 있습니다. `list_items`는 항목이 무엇을 멈추는지와 마지막 백업이 얼마나 걸렸는지를 어시스턴트에게 알려 주고, 도구 설명은 무언가를 시작하기 전에 이를 사용자에게 말하라고 요청합니다.

백업은 서비스를 멈추고 오래된 복원 지점을 밀어내므로, MCP를 통한 시작에는 제한이 있습니다:

- 키마다 시간당 12번의 백업 시작.
- 같은 항목, 같은 도메인, Backup Everything의 MCP 시작 사이에는 15분.
- 같은 항목의 MCP 시작은 24시간에 최대 4번.
- **보존 가드.** 도메인이 정해진 개수의 복원 지점을 보존할 때(일간, 주간, 월간 규칙 없이 "최근 N개 보존"만 있는 경우, 로컬이든 오프사이트 대상이든), 새 백업이 생길 때마다 가장 오래된 것이 밀려납니다. 이때 BombVault는 최근 N-1개의 성공한 백업이 모두 MCP로 시작된 항목에 대해 MCP 시작을 거부합니다. 그래서 보존되는 묶음에는 일정이나 사용자가 만든 복원 지점이 언제나 적어도 하나 남습니다. "최근 1개 보존"에서는 어시스턴트가 그 항목을 전혀 백업할 수 없습니다. 다음 예약 백업이 다시 자리를 만듭니다.

도메인이나 Backup Everything을 시작하면 제한에 걸린 항목은 빼고 응답에서 그 이름을 알려 줍니다. 이 제한들은 웹 인터페이스와 일정에는 적용되지 않습니다. 시간당 한도는 메모리에 있으므로 BombVault를 다시 시작하면 초기화됩니다.

## 켜기 {#switch-on}

1. **설정, 시스템, MCP 서버**를 열고 **새 키**를 누릅니다.
2. 어디에서 쓰는지 알 수 있는 이름을 키에 붙입니다. 예: "노트북의 Claude Code". 클라이언트마다 키를 하나씩 두면 다른 것을 건드리지 않고 하나만 폐기할 수 있습니다.
3. **백업 시작 허용**은 켜 두거나, 읽기만 하면 되는 키라면 끕니다. 나중에 키의 타일에서 바꿀 수 있고, 변경은 다시 연결하지 않아도 어시스턴트의 다음 요청부터 적용됩니다.
4. **키 만들기**를 누릅니다. 키는 한 번만 표시됩니다. BombVault는 지문만 저장하고 다시 보여 줄 수 없으니 지금 복사하거나, 아래에 있는 스니펫 가운데 하나를 쓰세요. 그 스니펫에는 이때 실제 키가 들어갑니다.

로그인 비밀번호가 없으면 웹 인터페이스 자체가 네트워크의 모든 사람에게 열려 있고, 열 수 있는 사람은 누구나 키도 만들 수 있습니다. 카드에도 그렇게 표시됩니다. 공개된 것처럼 보이는 이름(예: 리버스 프록시 뒤의 `bombvault.example.com`)으로 BombVault를 열었고 로그인 비밀번호가 설정되지 않았다면, 그 주소에서는 키를 만들거나 교체할 수 없습니다. 인터넷의 어떤 웹 페이지도 여러분의 브라우저가 키를 만들게 할 수 없도록 하기 위해서입니다. 로그인 비밀번호를 설정하거나, IP 주소 또는 `tower`, `tower.local` 같은 로컬 이름으로 BombVault를 여세요.

## 키와 키별 로그 {#keys}

카드에서 각 키는 자기 타일을 가집니다. 타일에는 키 이름, 백업을 시작할 수 있는지 읽기만 하는지, 키의 마지막 네 글자, 만든 시각이나 마지막으로 교체한 시각, 클라이언트가 마지막으로 사용한 시각, 오늘 호출 횟수가 나옵니다. 타일에서 키 이름을 바꾸고, 권한을 바꾸고, 키를 교체하거나 폐기합니다. 폐기한 키는 폐기된 키 목록으로 옮겨지며, 기록의 어떤 실행도 그 키를 가리키지 않게 되면 거기서 영구 삭제할 수 있습니다.

타일의 **로그**를 열면 그 키가 한 일이 보입니다. 먼저 그 키가 시작한 백업이 상태와 함께 나오고, 대시보드 활동 로그의 해당 실행으로 가는 링크가 붙습니다. 그 아래에 호출이 최신순으로 도구와 결과와 함께 나옵니다. 거부된 호출에는 이유가 붙습니다. 키가 읽기 전용이거나, 보존 보호가 백업을 막았거나, 다른 백업이 이미 실행 중이었거나, 항목이 몇 분 전에 MCP로 백업되었거나, 키가 요청을 너무 많이 보낸 경우입니다. 취소에는 해당 실행으로 가는 링크가 붙습니다.

BombVault는 키마다 최신 200건을 최대 30일 동안 보관합니다. 호출마다 도구, 결과, 취소가 가리킨 실행만 저장합니다. 어시스턴트가 보낸 내용과 키, 키의 지문은 절대 저장하지 않습니다. 진단 번들에는 건수만 들어가고, 설정 내보내기에는 빠집니다.

## 클라이언트 연결 {#clients}

카드는 연 주소에 맞는 스니펫을 바로 쓸 수 있게 보여 줍니다. 클라이언트를 고르고 스니펫을 복사하세요. 아래에서는 스니펫이 하는 일과 카드가 보여 주지 않는 형태를 설명합니다.

### Claude Code {#claude-code}

카드의 명령을 터미널에서 한 번 실행합니다. 컴퓨터가 신뢰하는 인증서라면 다음과 같습니다:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

연결은 Claude Code 안에서 `/mcp`로 확인합니다. `--scope user`는 키를 프로젝트 파일이 아닌 사용자 설정에 저장합니다.

이 명령에는 키가 들어 있어 셸이 기록에 남길 수 있습니다. 이를 피하려면 프로젝트 폴더에 `.mcp.json`을 두고 키를 환경 변수에 넣으세요. Claude Code는 파일을 읽을 때 `${BOMBVAULT_MCP_KEY}`를 값으로 바꿉니다:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

`BOMBVAULT_MCP_KEY`는 Claude Code가 시작되는 환경에 설정하세요. 예를 들어 셸 프로필에, 프롬프트에 입력하지 말고 텍스트 편집기로 적습니다. 키가 적힌 `.mcp.json`은 절대 커밋하지 마세요.

BombVault 자체 인증서를 쓰는 경우([TLS와 인증서](#tls) 참고) 카드의 명령은 대신 `mcp-remote`를 실행하고 내려받은 인증서를 Node.js에 알려 줍니다:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

작은따옴표는 셸이 변수를 펼치지 못하게 막습니다. 펼치는 일은 `mcp-remote`가 직접 합니다. 같은 형태를 `.mcp.json`에서도 쓸 수 있습니다. 아래의 Claude Desktop 항목을 쓰고 그 `env`에서 `BOMBVAULT_MCP_KEY`를 빼면 키는 환경에서 가져옵니다.

### Claude Desktop {#claude-desktop}

Claude Desktop은 `mcp-remote`를 통해 BombVault에 연결하며, 그 컴퓨터에 Node.js가 필요합니다. Claude Desktop의 **Settings, Developer, Edit Config**에서 설정 파일을 엽니다. 위치는 Windows에서는 `%APPDATA%\Claude\claude_desktop_config.json`, macOS에서는 `~/Library/Application Support/Claude/claude_desktop_config.json`입니다. 카드의 항목을 `"mcpServers"` 안, 이미 있는 서버들 옆에 추가하고 Claude Desktop을 다시 시작합니다:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS`는 BombVault 자체 인증서 때문에만 있습니다. 컴퓨터가 이미 신뢰하는 인증서 뒤에서는 빼세요.
- `--allow-http`는 일반 `http://` 주소일 때만 붙습니다.
- 헤더는 콜론 뒤에 공백 없이, 키는 `env`에 두고 `X-API-Key:${BOMBVAULT_MCP_KEY}`로 씁니다. 일부 시스템에서 `mcp-remote`는 `--header` 값을 첫 공백에서 나누므로, 공백 뒤에 쓴 키는 사라집니다.

### Claude 설정의 사용자 지정 커넥터 {#custom-connectors}

Claude 자체의 설정(claude.ai와 Claude Desktop의 커넥터 목록)에서 추가하는 커넥터는 아직 지원하지 않습니다. 이런 커넥터는 Anthropic의 클라우드에서 접속하므로 공개 HTTPS 주소가 필요하고, OAuth로 로그인합니다. 고정 키를 보낼 수 없는데 BombVault는 고정 키만 제공하며 OAuth 로그인은 없습니다. 이를 위해 BombVault를 인터넷에 내놓아도 도움이 되지 않습니다. Claude Code를 쓰거나, 위처럼 `mcp-remote`를 통해 Claude Desktop을 쓰세요.

### 다른 클라이언트 {#other-clients}

Streamable HTTP를 말하는 클라이언트라면 무엇이든 됩니다:

- URL: 웹 인터페이스 주소에 `/mcp`를 붙인 것. 예: `https://192.168.1.10:3443/mcp`.
- 키는 `Authorization: Bearer <key>` 또는 `X-API-Key: <key>`에 넣습니다. 둘 다 보내면 같은 키여야 합니다.
- `POST`에 `Content-Type: application/json`과 `Accept: application/json, text/event-stream`을 붙입니다.
- 요청당 JSON-RPC 메시지는 하나입니다. 배치는 거부됩니다.
- 프로토콜 버전은 2026-07-28, 2025-11-25, 2025-06-18, 2025-03-26입니다.

## TLS와 인증서 {#tls}

BombVault는 자신이 발급한 인증서로 HTTPS를 제공하며, 처음에 그 인증서에는 `localhost`, `127.0.0.1`, `::1`만 들어 있습니다. Claude Code와 `mcp-remote`는 LAN 주소에서 이를 거부합니다. 대부분의 Unraid 설치에 맞는 순서로 해결 방법을 적으면 다음과 같습니다:

1. **MCP 카드에서 주소 추가.** 인증서에 없는 주소로 HTTPS를 통해 카드를 열면 카드가 이를 알리고 **이 주소를 인증서에 추가**를 제안합니다. 그러면 BombVault가 그 주소를 넣어 인증서를 다시 발급합니다(브라우저는 처음처럼 한 번 더 경고합니다). 이어서 **인증서 내려받기**를 누르세요. 스니펫이 `NODE_EXTRA_CA_CERTS`를 내려받은 파일로 설정하므로 클라이언트는 바로 그 인증서를 신뢰합니다.
2. **신뢰할 수 있는 인증서를 가진 리버스 프록시**(Nginx Proxy Manager, SWAG, Caddy, Traefik). 클라이언트는 프록시의 인증서를 보므로 더 필요한 것이 없고, 카드도 BombVault 자체 인증서에 대해 경고하지 않습니다.
3. **Tailscale.** 컨테이너 앞의 `tailscale serve`나 Unraid의 Tailscale 연동으로 신뢰할 수 있는 인증서가 붙은 `ts.net` 이름을 얻습니다.
4. **`HTTP_ONLY=true`**. TLS를 종료하는 프록시 뒤나 완전히 신뢰하는 네트워크에서만 쓰세요. 웹 인터페이스 전체를 일반 HTTP로 바꾸고, 컨테이너 설정 변경이 필요하며, 키를 암호화하지 않고 보냅니다.

`NODE_TLS_REJECT_UNAUTHORIZED=0`은 절대 설정하지 마세요. 그 Node.js 프로세스가 통신하는 모든 대상에 대해 인증서 검증이 꺼집니다.

리버스 프록시는 `Authorization`(또는 `X-API-Key`) 헤더를 그대로 넘겨야 하며(따로 지시하지 않으면 프록시는 그렇게 합니다), `/mcp`를 버퍼링하거나 다시 쓰면 안 됩니다. BombVault 인증서도 검증하는 Nginx 또는 Nginx Proxy Manager용 location 블록 예시입니다:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

프록시 뒤에서는 모든 요청이 프록시의 주소를 가집니다. 그래서 잘못 설정된 클라이언트 하나가 틀린 키를 5번 보내면, 그 프록시 뒤의 모든 MCP 클라이언트가 1분 동안 막힙니다. 클라이언트별로 세려면 프록시를 `TRUSTED_PROXY`에 지정하세요([설정](configuration.md) 참고).

## 보안 모델 {#security}

- 활성 키가 없으면 `/mcp`는 `404`로 답합니다.
- 예외 주소는 없습니다. `localhost`, Unraid 호스트, 리버스 프록시, `tailscale serve`에서 오는 요청도 웹 인터페이스에 로그인 비밀번호가 없을 때조차 다른 요청과 똑같이 키가 필요합니다.
- 키는 지문으로만 저장되고 한 번만 표시되며, 이름 변경, 교체, 폐기가 가능합니다. 활성 키는 최대 10개이고 각각 **백업 시작 허용** 스위치가 있습니다.
- 생성, 교체, 권한 변경, 폐기가 있을 때마다 알림이 꺼져 있지 않으면 요청이 온 주소와 함께 알림 채널로 알림이 갑니다.
- 주소마다 1분에 틀린 키 5번이면 `429`. 키마다 1분에 120개 요청, 1시간에 12번의 백업 시작. 여기에 위의 대기 시간과 보존 가드가 더해집니다.
- 다른 출처(origin)의 브라우저 페이지에서 온 요청은 거부됩니다.
- 로그인 비밀번호가 설정되지 않은 동안에는 공개된 것처럼 보이는 호스트 이름에서 키를 만들 수 없습니다.
- 어시스턴트가 시작한 모든 백업과 그에 따른 prune 및 오프사이트 실행은 활동 로그, 오류 패널, 백업 알림에 키 이름과 함께 "MCP 경유"로 표시됩니다.
- 모든 도구 호출은 키의 ID와 마지막 네 글자(이름은 절대 아님)와 함께 컨테이너 로그에 기록되고 `/metrics`에서 집계됩니다(`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- 설정 백업을 복원하면 모든 키가 폐기됩니다. 복원한 데이터베이스에는 저장 뒤에 폐기한 키가 들어 있을 수 있기 때문입니다. 그 뒤에 새 키를 만드세요.
- `APP_KEY`가 바뀌면(재설치나 다른 컨테이너로 복원) 키는 더 이상 작동하지 않습니다. 카드가 이를 감지해 키를 표시하고, **키 교체**로 다시 유효한 비밀을 받을 수 있습니다.
- 키는 비밀번호처럼 다루세요. Claude Code와 Claude Desktop은 설정에 평문으로 저장합니다. 덜 신뢰하는 컴퓨터에서는 읽기 전용 키를 쓰는 편이 좋습니다.

## 기기 밖으로 나가는 것 {#privacy}

어시스턴트가 읽는 것은 모두 그 뒤의 AI 제공업체로 갑니다: 항목 이름, 일정, 오류 메시지를 포함한 실행 기록, 복원 지점의 ID와 시각, 데이터베이스 엔진 이름과 덤프 크기, 진행 중인 활동, 저장 공간 수치, 적용 범위와 상태. BombVault는 무엇이든 밖으로 나가기 전에 호스트 경로, 저장소 위치, 호스트 이름, 자격 증명, 훅 명령, 키를 제거합니다.

## 문제 해결 {#troubleshooting}

| 보이는 것 | 의미 |
|---|---|
| `404` | 활성 키가 없거나 `/api/mcp`처럼 경로가 틀렸습니다. 엔드포인트는 `/mcp`입니다. |
| `401` | 키가 없거나, 잘못 입력했거나, 폐기되었거나, 교체되었습니다. 프록시가 `Authorization` 헤더를 버리고 있을 수 있습니다(`X-API-Key`를 써 보세요). 카드가 키를 더 이상 유효하지 않다고 표시하면 `APP_KEY`가 바뀐 것입니다. 키를 교체하세요. |
| `403` | 다른 출처의 브라우저 페이지에서 온 요청입니다. 데스크톱이나 명령줄 클라이언트를 쓰세요. |
| GET에 `405` | 정상입니다. 엔드포인트는 `POST`만 받습니다. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | 클라이언트가 Streamable HTTP를 쓰기에는 너무 오래되었습니다. 업데이트하세요. |
| `400` "batch requests are not accepted" | 클라이언트가 JSON-RPC 배치를 보내고 있습니다. 요청마다 메시지 하나만 보내세요. |
| `429` | 이 주소에서 틀린 키가 너무 많거나, 한 키로 1분에 120개를 넘는 요청이 있었습니다. 1분 기다리고 어시스턴트가 반복에 빠지지 않았는지 확인하세요. |
| "certificate", "self-signed", "unable to verify"가 들어간 오류 | 클라이언트가 BombVault 인증서를 신뢰하지 않습니다. [TLS와 인증서](#tls)를 참고하세요. |
| `busy` | 다른 백업이나 유지보수 작업이 그 도메인을 쓰고 있습니다. 끝난 뒤 다시 시도하세요. |
| `cooldown` | 이 항목, 이 도메인 또는 Backup Everything이 15분 안에 MCP로 시작되었습니다. |
| `retention_guard` | MCP 백업을 한 번 더 하면 "최근 N개 보존" 범위에 MCP가 만든 복원 지점만 남게 됩니다. 다음 예약 백업이 자리를 만들거나, 웹 인터페이스에서 시작하세요. |
| `rate_limited` | 이 키는 이번 시간의 시작 12번을 다 썼습니다. |
| 시작 시 `not_permitted` | 읽기 전용 키입니다. 카드에서 **백업 시작 허용**을 켜세요. 다시 연결할 필요는 없습니다. 취소 시라면 그 실행을 이 키가 시작하지 않았다는 뜻입니다. |
| `domain_off` | 그 종류의 백업이 설정에서 꺼져 있습니다. |
| `not_found` | BombVault가 그 항목을 보호하지 않습니다. 먼저 웹 인터페이스에서 추가하세요. MCP는 설정을 만들지 않습니다. |

컨테이너에 환경 변수 `MCPGODEBUG`를 설정하지 마세요. MCP 라이브러리의 동작을 바꾸며, 잘못된 값이 있으면 BombVault는 로그를 한 줄도 쓰기 전에 시작 단계에서 멈춥니다.
