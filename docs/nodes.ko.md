# 노드

노드는 웹 인터페이스가 없는 BombVault로, 예를 들어 VPS나 친구 집의 작은 서버에서 실행합니다. 노드는 자기 서버의 컨테이너와 폴더를 백업하고, 언제 백업할지와 백업을 어디로 보낼지는 노드를 추가한 BombVault가 정합니다. 이 페이지에서는 그 BombVault를 **홈 인스턴스**라고 부릅니다. 노드는 홈 인스턴스의 웹 인터페이스에서 자기 항목과 나란히 관리합니다.

![인스턴스 페이지에 있는 노드 vps-falkenstein의 카드: 연결됨, 버전, 수신 서버로 가는 백업, 섹션별 백업 시간, 그리고 보낼 곳 변경, 교체, 해제.](assets/screenshots/nodes/card-dark.png#only-dark)
![인스턴스 페이지에 있는 노드 vps-falkenstein의 카드: 연결됨, 버전, 수신 서버로 가는 백업, 섹션별 백업 시간, 그리고 보낼 곳 변경, 교체, 해제.](assets/screenshots/nodes/card-light.png#only-light)

## 작동 방식 {#how-it-works}

- **노드의 키는 홈 인스턴스가 가지고 있습니다.** 노드를 추가하면 홈 인스턴스가 노드의 `APP_KEY`를 만들어 봉인한 사본을 보관하고, 노드의 compose 파일에 담아 한 번만 건네줍니다. 홈 인스턴스는 이 사본으로 노드에 접속하고 노드의 백업을 엽니다.
- **노드에는 열린 포트가 필요 없습니다.** 홈 인스턴스와 노드는 페어링 그룹과는 별개인 전용 비공개 연결로 릴레이를 거쳐 대화합니다. 모든 호출은 홈 인스턴스가 시작하고 서명하며, 노드가 홈 인스턴스에 보내는 것은 첫인사뿐입니다. 열린 포트가 필요한 보낼 곳은 [노드에 보관](#keep-on-node) 하나뿐입니다.
- **백업은 릴레이를 거치지 않습니다.** 노드는 고른 곳으로 백업을 바로 보냅니다. 홈 인스턴스의 수신 서버, 홈 인스턴스의 대상 가운데 하나, 또는 노드 자체의 rest-server입니다.
- **노드는 스스로 아무것도 계획하지 않습니다.** 일정을 돌리지 않고, 알림을 보내지 않으며, 그룹이나 요새에 들어가지 않습니다. 홈 인스턴스가 백업을 시작하고, 진행을 지켜보고, 알림을 보냅니다.
- **노드는 컨테이너와 폴더 세트를 백업합니다.** 가상 머신, 플래시 드라이브, ZFS 데이터세트, 셀프 백업은 전체 설치에서만 다룹니다.

연결에는 홈 인스턴스의 그룹에 자체 릴레이가 있으면 그 릴레이를, 없으면 프로젝트 릴레이를 씁니다. 홈 인스턴스가 릴레이 없이 페어링하는 경우에도 마찬가지입니다.

## 시작하기 전에 {#before-you-start}

- **홈 인스턴스의 로그인 비밀번호.** 노드 추가, compose 파일 다시 보기, 노드의 항목 열기에는 비밀번호로 로그인해야 합니다. [로그인](configuration.md#login)을 참고하세요.
- **노드 서버의 Docker와 Compose.**
- **백업을 둘 곳.** [백업을 보낼 곳](#placement)에 있는 세 가지 중 하나입니다. 가장 안전한 것은 수신 서버입니다. 노드가 그곳에서는 아무것도 삭제할 수 없기 때문입니다.

## 노드 추가 {#add}

1. 홈 인스턴스에서 **인스턴스**를 열고 오른쪽 아래의 **노드 추가**를 누릅니다.
2. **이름.** 노드가 여기에 표시될 이름을 입력합니다. 서버 이름 등을 쓰면 좋습니다. **다음**을 누릅니다.

    ![노드 추가 창의 첫 단계, 이름 vps-helsinki가 입력되어 있습니다.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![노드 추가 창의 첫 단계, 이름 vps-helsinki가 입력되어 있습니다.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **백업.** 백업을 보낼 곳을 고르고, 그곳에 필요한 내용을 채웁니다. **매일 백업 시간**은 두 섹션이 모두 시작하는 시각입니다. 홈 인스턴스는 자기 컨테이너 백업 30분 뒤를 제안하므로, 둘이 같은 디스크를 동시에 쓰지 않습니다. 섹션마다 따로 정하는 계획은 나중에 카드에서 할 수 있습니다. **다음**을 누릅니다.

    ![두 번째 단계: 수신 서버를 고르고, 노드용 주소에 https 주소를 넣었으며, 매일 백업 시간은 03:00입니다.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![두 번째 단계: 수신 서버를 고르고, 노드용 주소에 https 주소를 넣었으며, 매일 백업 시간은 03:00입니다.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **시작.** 창에 `MODE`, `APP_KEY`, `NODE_HOME`, `NODE_NAME`이 채워진 노드의 compose 파일이 나옵니다. 이 파일을 복사하고, `APP_KEY`는 안전한 곳에 보관하세요. 노드의 서버에서 다음을 실행합니다.

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    시작하기 전에 Host Data 볼륨을 백업할 데이터로 지정하고, `HOST_SOURCE_ROOT`를 같은 경로로 설정하세요. 아래 예시처럼 하면 됩니다.

5. 창은 노드를 기다립니다. 노드가 응답하면 홈 인스턴스가 백업을 보낼 곳을 알려 주고 노드가 그곳에 닿는지 확인하며, 창에 **vps-helsinki 노드가 온라인이며 이 인스턴스의 지시를 받습니다.**라고 나옵니다.

    ![마지막 단계: compose 파일, 그리고 온라인이 된 노드와 수신 서버로 정해진 백업.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![마지막 단계: compose 파일, 그리고 온라인이 된 노드와 수신 서버로 정해진 백업.](assets/screenshots/nodes/add-start-light.png#only-light)

노드가 응답하기 전에 창을 닫아도 됩니다. 그러면 카드에 **노드를 기다리는 중**이라고 나오고, 노드가 나타나면 홈 인스턴스가 백업을 보낼 곳을 알려 줍니다.

### 예시: 수신 서버로 백업하는 VPS {#example-vps}

홈 인스턴스는 `tower`라는 Unraid 장비이고 [수신 서버](offsite-recovery.md#receiving-server)를 실행합니다. 리버스 프록시가 이를 `https://backup.tower.example.org`로 공개합니다. VPS `vps-helsinki`에서는 Nextcloud와 Caddy가 실행되며, 데이터는 `/srv/appdata` 아래에 있습니다.

1. `tower`에서 **인스턴스**, **노드 추가**로 가서 이름에 `vps-helsinki`를 넣고 **다음**을 누릅니다.
2. **이 인스턴스의 수신 서버**를 고르고 **노드용 주소**에 `https://backup.tower.example.org`를 입력합니다. 홈 인스턴스의 컨테이너는 02:30에 백업하므로 창은 03:00을 제안합니다. 그대로 두고 **다음**을 누릅니다.
3. VPS에서, 창의 파일을 볼륨과 `HOST_SOURCE_ROOT`만 `/srv/appdata`에 맞게 바꿔 저장합니다.

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`를 실행합니다. 몇 초 뒤 `tower`의 창에 노드가 온라인이라는 내용과 **백업이 https://backup.tower.example.org에 저장됩니다.**가 나옵니다.
5. 03:00에 홈 인스턴스가 노드의 백업을 시작합니다. 다음 날 아침에는 개요의 **노드** 카드에 `vps-helsinki`의 마지막 백업이 나오고, **백업**에는 그 컨테이너가 별도의 섹션에 나열됩니다.

일반 Docker 호스트에서 컨테이너의 데이터를 어떻게 찾는지는 [백업 원본 자동 판별](configuration.md#backup-source-detection)에 설명되어 있습니다.

## 백업을 보낼 곳 {#placement}

노드마다 두 섹션에 함께 쓰는 보낼 곳이 하나 있습니다. 노드가 온라인이면 카드의 **보낼 곳 변경**으로 바꿀 수 있습니다. 새 곳에는 새 저장소가 생기고, 이전 곳의 백업은 그대로 남습니다.

### 이 인스턴스의 수신 서버 {#to-receiver}

노드는 홈 인스턴스의 수신 서버에 append-only 모드로 씁니다. 백업을 추가할 수는 있지만 삭제할 수는 없습니다. 노드는 노드 이름을 딴 자체 로그인과, 그곳의 자기 폴더 안에 섹션마다 저장소 하나씩을 받습니다. 노드는 정리를 할 수 없으므로, 홈 인스턴스가 각 저장소에 자기 키를 추가하고 정리도 직접 합니다.

노드는 **노드용 주소**로 수신 서버에 접속합니다. 이 주소는 두 번째 단계나 **수신 서버** 창에서 정합니다. 노드는 보통 인터넷을 거쳐 보내므로, 주소는 다음 중 하나여야 합니다.

- `https://`. 예를 들어 리버스 프록시, Pangolin, 터널 뒤에 둡니다. 서버가 공인 인증 기관의 인증서가 아니라 자체 인증서를 보여 준다면, 그 인증서를 PEM 형식으로 **인증서**에 붙여 넣으세요.
- 사설 IP 주소를 쓴 `http://`. 예를 들어 WireGuard 연결의 `10.0.0.5`나 Tailscale의 `100.101.102.103`입니다.

공인 주소로 가는 일반 `http://`는 거부됩니다. 노드가 백업을 암호화하지 않은 채 인터넷으로 보내게 되기 때문입니다. 주소를 바꾸면 홈 인스턴스가 수신 서버를 쓰는 모든 노드에 알리고, 연결되어 있지 않은 노드는 돌아왔을 때 알게 됩니다.

### 대상 {#to-location}

노드는 스스로 접속할 수 있는 홈 인스턴스의 대상 가운데 하나에 씁니다. `https://`나 사설 주소로 접속하는 S3 서비스 또는 REST 서버입니다. 저장소는 그곳의 `bombvault-node-<node id>-containers`와 `bombvault-node-<node id>-files`에 있습니다.

노드는 노드용으로 만든 [저장된 자격 증명](offsite-recovery.md#sign-ins)으로 로그인합니다. 이 자격 증명은 홈 인스턴스가 그 대상에 쓸 때 쓰는 것과 달라야 합니다. 창에 자격 증명이 없다고 나오면 먼저 하나 추가하세요. 수신 서버와 달리 이 자격 증명으로는 노드가 대개 대상에 있는 것을 삭제할 수 있으며, 창에도 그렇게 나옵니다.

### 노드에 보관 {#keep-on-node}

노드는 append-only 모드의 자체 rest-server 뒤에 백업을 직접 보관하고, 홈 인스턴스가 섹션마다 사본을 자기 저장 공간으로 가져옵니다. 홈 인스턴스가 둘 곳을 마련해 줄 수 없는 노드에 알맞습니다. 열린 포트가 필요한 유일한 보낼 곳입니다.

![두 번째 단계에서 노드에 보관을 고른 모습: 주소 https://203.0.113.7:8443과 열린 포트에 대한 안내.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![두 번째 단계에서 노드에 보관을 고른 모습: 주소 https://203.0.113.7:8443과 열린 포트에 대한 안내.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. **노드 주소**에 홈 인스턴스가 노드에 접속할 곳을 입력합니다. 노드의 공인 주소와 비어 있는 포트로, 예를 들어 `https://203.0.113.7:8443`이며 뒤에 폴더는 붙이지 않습니다. `http://`는 위와 마찬가지로 사설 주소에서만 쓰세요.
2. 노드는 그 포트로 컨테이너 `bombvault-node-rest-server`를 시작하고, 저장소를 Host Data 아래의 `bombvault-node` 폴더에 둡니다. `https://`라면 그 주소용 인증서를 만들고, 홈 인스턴스가 그 인증서를 고정합니다.
3. 홈 인스턴스가 접속할 수 있도록 노드의 방화벽에서 포트를 허용하세요. 노드의 다른 컨테이너가 쓰는 포트는 그 컨테이너 이름과 함께 거부됩니다.

홈 인스턴스는 노드가 한 섹션의 백업을 마치면 바로 그 섹션을 가져오고, 대비책으로 매시 15분에도 가져옵니다. **가져오기**가 꺼져 있어도 마찬가지입니다. 홈 인스턴스가 가져온 것은 다른 [가져온 사본](offsite-recovery.md#receiver)처럼 홈 인스턴스에 남으며, 노드가 사라진 뒤에도 그대로입니다. 노드의 rest-server는 삭제를 받지 않으므로 노드는 홈 인스턴스가 요청할 때 자기 저장소를 정리합니다. 또 노드의 폴더 세트는 `bombvault-node` 폴더를 빼므로, 자기 백업을 다시 백업하는 일은 없습니다.

## 노드가 백업하는 때 {#schedule}

모든 노드의 일정은 홈 인스턴스가 정합니다. 카드에는 **컨테이너** *매일 2:00*, **폴더** *매주(일) 4:00*처럼 섹션마다 계획이 나옵니다. 누르면 [일정](schedules.md#plans)과 같은 편집기가 열리고, 바꾼 내용은 바로 저장됩니다.

![카드에서 연 노드의 컨테이너 계획: 끄기, 매일, 매주, N일마다, Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![카드에서 연 노드의 컨테이너 계획: 끄기, 매일, 매주, N일마다, Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **연결되어 있지 않던 노드**는 돌아오는 즉시 놓친 백업을 시작합니다. 홈 인스턴스는 5분마다 확인합니다.
- **바쁜 노드**에는 5분 뒤에 다시 요청합니다.
- **홈 인스턴스가 노드의 오래된 백업을 정리하는 동안**에는 끝날 때까지 노드가 아무것도 시작하지 않습니다. 수신 서버와 노드 자체에서는 이 시간이 매주 일요일 03:00에 시작해 최대 여섯 시간 이어집니다. 대상에서는 그 대상에 대한 홈 인스턴스의 오프사이트 복사 계획을 따릅니다. 홈 인스턴스는 **보존 기간**에 있는 그 섹션의 자기 규칙대로 백업을 남기며, 대상에 자체 규칙이 있으면 그 규칙을 따릅니다.
- **노드의 실행 목록**에서는 홈 인스턴스가 시작한 백업과 복원이 홈 인스턴스가 시작한 것으로 표시됩니다.

## 노드 지켜보기 {#watch}

개요의 **노드** 카드에는 모든 노드가 상태, 마지막 백업, 오류, 열린 이상 징후와 함께 나열됩니다. 노드가 생기면 이 카드가 나타납니다.

![개요의 노드 카드에 있는 노드 세 개: 하나는 연결됨, 하나는 비활성이며 오류 두 건, 하나는 아직 노드를 기다리는 중.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![개요의 노드 카드에 있는 노드 세 개: 하나는 연결됨, 하나는 비활성이며 오류 두 건, 하나는 아직 노드를 기다리는 중.](assets/screenshots/nodes/overview-card-light.png#only-light)

홈 인스턴스는 한 번의 사건마다 알림을 하나만 [알림 채널](notifications.md)로 보내고, 그 사건이 끝난 뒤에야 다음 알림을 보냅니다.

- **백업 실패.** 예를 들어 *BombVault: vps-helsinki: backup failed*이며, 항목과 이유가 함께 나옵니다. 그 항목의 다음 백업이 성공하면 사건이 끝납니다.
- **노드에 닿을 수 없음.** 예를 들어 *BombVault: vps-helsinki is out of reach*입니다. 홈 인스턴스는 노드 저장소의 데드맨 스위치 가운데 가장 짧은 시간만큼 기다립니다. 바꾸지 않았다면 26시간이고, 노드에 아직 보낼 곳이 없는 동안에는 6시간입니다.

홈 인스턴스의 수신기 감시는 노드의 각 저장소도 데드맨 스위치와 매일 04:00의 `restic check`로 지켜봅니다. [수신기와 가져오기 측](offsite-recovery.md#receiver)을 참고하세요.

## 노드의 항목 {#entries}

**백업**의 컨테이너와 폴더 목록에는 자기 항목 아래에 노드마다 섹션이 있습니다. 노드 자체의 컨테이너는 빠집니다. 항목 메뉴의 **지금 백업**은 노드에서 그 항목의 백업을 시작하고, **열기**는 항목의 페이지를 엽니다.

폴더 목록의 노드 섹션에서 **폴더 세트 추가**는 노드에 폴더 세트를 추가하고, 세트 메뉴의 **편집**은 그 세트를 바꿉니다. 둘 다 자기 세트와 같은 창을 엽니다. 그 창의 폴더 선택기는 노드의 폴더를 둘러보지만 그곳에 새 폴더를 만들지는 않습니다.

![컨테이너 목록에 있는 노드 vps-falkenstein의 섹션: nextcloud, nextcloud-db, caddy, uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![컨테이너 목록에 있는 노드 vps-falkenstein의 섹션: nextcloud, nextcloud-db, caddy, uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

노드 항목의 페이지에는 **vps-falkenstein에서**처럼 노드 이름이 나오고, 그 항목의 백업이 나열됩니다. 그중 하나에서 **복원**을 누르면 노드에서 항목을 복원하며, 지금 있는 내용은 대체됩니다. 홈 인스턴스에서와 마찬가지로 컨테이너는 그동안 멈춥니다.

![vps-falkenstein에 있는 nextcloud의 페이지: 백업과 각 백업 옆의 복원 버튼.](assets/screenshots/nodes/entry-dark.png#only-dark)
![vps-falkenstein에 있는 nextcloud의 페이지: 백업과 각 백업 옆의 복원 버튼.](assets/screenshots/nodes/entry-light.png#only-light)

홈 인스턴스를 통해서는 노드 자체 경로의 정해진 목록에만 닿습니다. 컨테이너, 폴더 세트, 실행, 이상 징후, 그리고 이들의 백업과 복원입니다. 노드의 설정, 키, 저장 공간에는 닿지 않으며, 그 목록에는 삭제하는 것이 없습니다. [API의 노드](api.md#nodes)를 참고하세요.

## 교체와 해제 {#replace-release}

**교체**는 비밀번호로 로그인한 뒤 노드의 compose 파일을 다시 보여 줍니다. 노드를 다른 서버로 옮기거나 다시 구축할 때 씁니다. 새 서버에서 그 파일을 시작하고 이전 서버는 종료하세요. 같은 `APP_KEY`와 `NODE_HOME`이 있으면 백업과 기록이 그대로인 같은 노드가 됩니다. 같은 `APP_KEY`를 가진 서버 두 대는 하나의 노드로 취급되므로, 둘을 함께 실행하지 마세요.

**해제**는 노드를 놓아줍니다. 노드는 더 이상 홈 인스턴스의 지시를 받지 않고 아무것도 백업하지 않습니다. 백업은 지금 있는 곳에 그대로 남고, 홈 인스턴스가 가져온 사본도 모두 남습니다. 수신 서버의 로그인은 사라지고, 홈 인스턴스는 노드의 저장소를 더 이상 지켜보지 않습니다. 해제할 때 연결되어 있지 않던 노드는 그 사실을 알게 될 때까지 카드가 **해제 중**으로 남습니다. 해제한 노드는 같은 `APP_KEY`로는 다시 홈 인스턴스를 가질 수 없습니다. 그 서버를 다시 쓰려면 새 노드로 추가하고, 새 compose 파일과 빈 `config` 폴더로 시작하세요.

## 보안 {#security}

홈 인스턴스를 장악당하면 공격자는 홈 인스턴스의 노드도 제어하게 되며, 노드는 자기 서버의 Docker를 쥐고 있어 그 서버에서는 root와 같습니다. 홈 인스턴스는 비밀번호로 보호하고 인터넷에 내놓지 마세요. 노드를 장악당해도 공격자는 페어링 그룹, 요새, 다른 노드, 다른 저장소를 얻지 못하며, 수신 서버의 기록을 삭제할 수도 없습니다. 자세한 내용은 [보안 모델의 노드](configuration.md#nodes)에 있습니다.

[복구 키트](offsite-recovery.md#recovery-kit)에는 **노드** 장이 있어 각 노드의 `APP_KEY`, restic 비밀번호, 저장소가 담겨 있습니다. 그래서 노드를 되살리거나 일반 restic으로 노드의 백업을 복원할 수 있습니다.
