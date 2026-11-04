# 시작하기

이 페이지는 갓 설치한 Unraid 장비에서 첫 백업까지 안내합니다.

## 요구 사항 {#requirements}

| 요구 사항 | 참고 |
|---|---|
| **Unraid 6.12+** | 이전 버전은 테스트되지 않았습니다. Unraid가 주 대상이지만, BombVault는 일반 Docker 호스트와 TrueNAS Scale에서도 실행됩니다([일반 Docker 호스트](#generic-docker-host) 및 [TrueNAS SCALE](#truenas) 참고). |
| **Restic 저장소 위치** | 로컬 경로(권장: 배열 또는 캐시), SMB, NFS, 또는 모든 rclone 백엔드. |
| **Docker 소켓** | 템플릿이 자동으로 마운트합니다(`/var/run/docker.sock`). |
| **Unraid 플래시** (`/boot`) | 템플릿이 전체를 자동으로 마운트합니다(`/boot`을 `/host/boot`로). 플래시 백업을 구동하고, 복원된 컨테이너가 정상적이고 편집 가능한 Unraid 앱으로 다시 나타나게 합니다. |
| **KVM VM** (선택 사용) | VM 백업은 SSH를 통해 libvirt와 통신하며 libvirt 마운트가 없습니다. **설정, 연동, 호스트 SSH**에서 구성하세요([구성](configuration.md) 참고). |
| **ZFS 데이터세트** (선택 사용) | VM 백업과 같은 SSH 연결, 호스트의 `zfs`, 그리고 접근 모드 Read/Write - Slave로 `/mnt`에 매핑된 Host Data(템플릿 기본값)가 필요합니다. [ZFS 데이터세트](zfs-datasets.md)를 참고하세요. |
| **Android 앱** (선택) | Android 10 이상이며, 버전 9.7.0 이상인 서버와 페어링합니다. [Android 앱](android.md)을 참고하세요. |

## Unraid에 설치 {#install-on-unraid}

가장 쉬운 방법은 **Community Applications**입니다.

1. Unraid에서 **Apps** 탭을 엽니다.
2. **BombVault**를 검색합니다.
3. **Install**을 클릭하고, 필수 변수(아래)를 설정한 후 적용합니다.

!!! tip "수동 템플릿 설치"
    템플릿을 직접 추가하려면:

    1. **Docker, Add Container, Template repositories**로 이동하여 다음을 추가합니다:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Templates에서 **BombVault**를 검색합니다.
    3. 필수 변수를 설정하고 **Apply**를 클릭합니다.

## 일반 Docker 호스트 {#generic-docker-host}

Unraid가 아니라면? BombVault는 어떤 Docker 호스트에서도 평범한 컨테이너로 돌아갑니다. TrueNAS SCALE용 절차는 [TrueNAS SCALE](#truenas)에 따로 있습니다.

1. 저장소에서 바로 편집할 수 있는 [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml)을 가져옵니다.
2. `APP_KEY`(아래 참조)를 설정하고 Host Data 볼륨을 실제 데이터 루트로 지정합니다. 파일의 주석이 둘 다 안내합니다.
3. `docker compose up -d` 후 `https://<host-ip>:3443/`을 엽니다.

Unraid와 다른 점:

- **플래시 섹션이 없습니다.** 담거나 되살릴 부팅 USB가 없으므로 플래시 섹션은 여기서 할 일이 없습니다. 대신 Host Data가 호스트의 `/etc`까지 닿으면 백업 페이지의 **추가**가 **시스템 구성**을 폴더 제안으로 보여 줍니다. 저장 전에 살펴보고 손질하는 `/etc` 폴더 세트의 시작 묶음입니다.
- **Unraid 자체 알림이 없습니다.** BombVault 자신의 알림 채널(웹훅, 오프사이트 실패 경고 등)은 평소대로 동작합니다. 빠지는 것은 Unraid 고유 알림 시스템으로 보내는 것뿐이며, 여기에는 그런 시스템이 없기 때문입니다.
- **VM 백업은 선택 사항이며 SSH로 닿는 별도의 libvirtd 호스트가 필요합니다.** compose 파일의 주석 처리된 블록을 보세요. 일반 Docker 호스트 자체에는 VM 관리자가 들어 있지 않습니다.
- **대시보드 위젯이 없습니다.** BombVault Widget은 Unraid 플러그인이므로 그 단계도 건너뜁니다.
- **컨테이너 데이터 찾기.** Unraid의 `appdata` 규칙이 없으므로, 컨테이너의 데이터 폴더는 `DATA_ROOT_SEGMENTS`의 구간, Docker 이름 있는 볼륨, Compose 프로젝트의 작업 디렉터리, `bombvault.data` 레이블로 찾습니다([백업 원본 자동 판별](configuration.md#backup-source-detection) 참고). 이름 있는 볼륨과 `/etc` 프리셋은 Host Data 마운트 안의 경로에만 닿으므로, Host Data는 Docker의 데이터 루트까지 포함하는 공통 상위 디렉터리로 지정하세요.
- **`PLATFORM`.** `generic` 또는 `truenas`로 설정합니다. 설정하지 않으면 BombVault는 플래시 마운트에 있는 Unraid 고유 표식으로 Unraid를 알아보고 그 밖의 경우는 모두 일반 호스트로 취급하며, Unraid 전용 단계는 시도했다 실패하는 대신 건너뜁니다.

### TrueNAS SCALE {#truenas}

BombVault는 아직 TrueNAS 앱 카탈로그에 없으므로 사용자 지정 앱으로 설치합니다. **Apps, Discover Apps**를 연 다음, **Custom App** 옆 메뉴에서 **Install via YAML**을 고릅니다. 아래 YAML은 저장소의 `truenas-apps` 폴더에 있는 카탈로그 항목과 같은 내용을 설정합니다. `tank`를 자신의 풀로 바꾸고, `APP_KEY`는 다음 섹션의 설명대로 채우세요.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

그런 다음 `https://<truenas-ip>:30528/`을 엽니다. [Unraid와 다른 점](#generic-docker-host)에 나온 내용은 여기에도 해당하며, TrueNAS에서는 다음이 더 있습니다.

- **기능(capabilities).** BombVault에는 위의 일곱 가지가 필요합니다. `SETPCAP`이 없으면 VM 백업이 실패하고, `SYS_CHROOT`가 없으면 모든 컨테이너가 스냅샷에서 읽히지 않고 백업 내내 중지됩니다.
- **Host Data**는 데이터세트 위의 호스트 경로(여기서는 `/mnt/tank`)이며, 앱 안에서도 같은 경로에 마운트됩니다. 그래서 `HOST_SOURCE_ROOT`와 `HOST_MOUNT_ROOT` 모두 이 경로를 가리킵니다. `rslave`는 호스트가 나중에 만드는 마운트를 전달하며, ZFS 데이터세트 백업에는 이것이 필요합니다.
- **ixVolume 안의 데이터는 백업되지 않습니다.** 저장소가 ixVolume인 앱은 데이터를 Host Data 바깥인 `/mnt/.ix-apps` 아래에 둡니다. BombVault는 앱의 정의만 저장하고, 실행 기록에 제외한 폴더가 표시됩니다. 데이터를 백업하려는 모든 앱에는 Host Data 아래 데이터세트의 **Host Path** 저장소를 고르세요.
- **VM 및 ZFS 데이터세트 백업**은 root로 SSH를 통해 TrueNAS에 접속합니다. **System, Services**에서 SSH 서비스를 켜고, **설정, 연동, 호스트 SSH**의 공개 키를 **Credentials, Users**에서 root 사용자에 추가하세요. TrueNAS의 libvirtd는 자체 소켓에서 대기하므로 VM 백업에는 위와 같이 `LIBVIRT_URI`도 필요합니다. `LIBVIRT_URI`와 `LIBVIRT_HOST`가 모두 설정되지 않으면, TrueNAS 앱에서는 `host.docker.internal`이 확인되지 않으므로 BombVault는 앱의 기본 게이트웨이를 호스트로 사용합니다. zvol 위의 VM을 포함한 자세한 내용은 [VM 백업 가이드](vm-backup-ssh-setup.md#truenas-scale)에 있습니다.
- **ZFS 복제본은 다른 키를 씁니다.** 두 번째 ZFS 서버의 복제본은 호스트 SSH 키가 아니라 복제본 대화 상자에 표시되는 키로 로그인합니다. 그 키를 받는 쪽 서버에 추가하세요.
- **삭제한 앱은 두 단계로 되살립니다.** 먼저 TrueNAS에서 같은 이름으로 앱을 다시 설치해 네트워크를 다시 만든 다음, BombVault에서 데이터를 복원합니다. 그 전까지는 복원 확인이 없는 네트워크를 보고하고, 복원은 아무것도 바꾸기 전에 거부됩니다.
- **원본 옆에**는 일반 Docker 컨테이너를 만듭니다. 실행은 되지만 TrueNAS Apps 화면에는 나타나지 않습니다.
- **데이터베이스 덤프**는 데이터세트 마운트에서도 동작합니다. 그 자체가 데이터세트인 데이터 폴더로 가져오면 예전 내용이 옆 폴더로 옮겨지므로, 상위 데이터세트에 그만큼의 공간이 필요합니다.

### ZimaOS, CasaOS 및 기타 호스트 {#other-hosts}

- **ZimaOS와 CasaOS:** [커뮤니티 스토어](https://github.com/junkerderprovinz/zimaos-apps#install)를 추가하고 거기서 BombVault를 설치하세요. 그곳에서는 `PLATFORM=generic`으로, Host Data는 `/DATA`로 실행됩니다. ZimaOS는 앱 데이터를 `/DATA/AppData/<app>`에 두는데, `DATA_ROOT_SEGMENTS`가 대소문자와 관계없이 `appdata`와 일치하므로 BombVault가 저절로 찾습니다.
- **MOS:** [MOS Hub](https://mos-official.net/apps.html?search=BombVault)에서 BombVault를 설치하세요. 그곳에서는 `PLATFORM=generic`으로, Host Data는 `/mnt/cache`로 실행됩니다.
- **Synology:** Container Manager에서 compose 파일로 프로젝트를 만드세요(DSM 7.2 이상).
- **Proxmox:** Docker가 있는 VM이나 LXC 컨테이너에서 실행하세요.

주석을 뺀 일반 compose 파일은 짧습니다. `./data`를 컨테이너 데이터가 있는 호스트 폴더로 바꾸고, `HOST_SOURCE_ROOT`를 같은 경로로 설정한 다음, `APP_KEY`를 채우세요.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Docker 이름 있는 볼륨도 백업하려면, Unraid 템플릿이 `/mnt` 전체를 마운트하는 것처럼, 데이터와 Docker의 데이터 루트(`docker info -f '{{.DockerRootDir}}'`, 보통 `/var/lib/docker`)를 모두 포함하는 폴더를 마운트하세요.

## 유일한 필수 설정 {#the-one-required-setting}

반드시 설정해야 하는 유일한 변수는 `APP_KEY`이며, restic 저장소 비밀번호를 파생하는 데 사용되는 32바이트 16진수 비밀 값(64개의 16진수 문자)입니다.

아무 장비에서나 하나 생성하세요.

```bash
openssl rand -hex 32
```

결과를 템플릿의 `APP_KEY` 필드(Unraid) 또는 `docker-compose.yml`의 `APP_KEY` 환경 변수(일반 Docker 호스트)에 붙여넣으세요.

!!! danger "APP_KEY를 잃어버리지 마세요"
    `APP_KEY`를 잃어버리면 암호화된 백업을 복구할 수 없게 됩니다. 서버와 분리된 안전한 곳에 보관하세요. BombVault가 실행되면 로그인 비밀번호를 설정하고 **설정, 복원**에서 **복구 키트**를 내려받으세요([오프사이트 및 복구](offsite-recovery.md#recovery-kit) 참고). 키, 모든 대상, restic 명령이 들어 있습니다.

템플릿은 또한 Docker 소켓, 플래시(`/boot`), 그리고 **Host Data** 루트(`/mnt`)를 대신 마운트합니다. 백업 *원본*과 *대상*은 모두 Host Data 아래에 있습니다. 전체 변수 참조와 오프사이트 설정은 [구성](configuration.md)을 참고하세요.

## 첫 실행 {#first-run}

![새로 설치한 뒤의 환영 페이지. BombVault가 서버에서 찾은 것이 나옵니다.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![새로 설치한 뒤의 환영 페이지. BombVault가 서버에서 찾은 것이 나옵니다.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. `https://<your-unraid-ip>:3443`에서 웹 UI를 엽니다(기본으로 자체 서명된 인증서). 새로 설치하면 **BombVault에 오신 것을 환영합니다** 화면이 열리고, BombVault가 서버에서 찾은 것이 나열됩니다.
2. **시작하기**를 선택합니다. 그러면 개요에 세 단계가 있는 **첫 단계** 카드가 나타납니다.
    1. **섹션 선택**: BombVault가 백업할 것을 켭니다(컨테이너, VM, 플래시, 폴더, ZFS 데이터세트, 셀프 백업). **선택**은 섹션의 항목 목록을 열고, 거기서 **일정 일시 중지**로 하나를 뺄 수 있습니다. 여기서 켠 섹션은 매일 밤 실행되며, 이는 **설정, 일정**에서 바꿉니다.
    2. **대상 추가**: 백업을 둘 두 번째 장소입니다. 모든 섹션에는 이미 이 서버에 폴더가 있으며, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`이고 첫 백업 때 만들어집니다. 가장 안전한 것은 Storage Box나 Backblaze B2 같은, 현장 밖의 두 번째 대상입니다.
    3. **첫 백업**: 그 뒤로는 매일 밤 저절로 실행됩니다. 이 카드는 첫 백업이 끝날 때까지 남아 있습니다.
3. **백업**을 열면 모든 항목이 한 목록에 나옵니다. **지금 모두 백업**은 켜 둔 모든 섹션에 대해 실행을 시작하고, 항목 페이지의 **지금 백업**은 그 하나만 백업합니다.

![세 단계가 있는 첫 단계 카드. 두 번째 단계가 열려 있습니다.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![세 단계가 있는 첫 단계 카드. 두 번째 단계가 열려 있습니다.](assets/screenshots/getting-started/first-steps-light.png#only-light)

전에 BombVault를 써 보셨나요? 환영 화면의 **백업에서 복원**을 누르면 복원 페이지가 열리고, **긴급 마법사**가 잃어버린 서버의 설정, 컨테이너, VM, 폴더를 되살립니다. [오프사이트 및 복구](offsite-recovery.md#emergency)를 참고하세요.

!!! tip "선택 사항: 백업 순서 지정"
    일부 컨테이너를 항상 다른 것보다 먼저 백업해야 한다면(예: 앱을 사용하는 데이터베이스를 먼저), **설정, 일정**의 **순서** 카드를 열고 원하는 순서로 드래그하세요. 예약 실행과 전체 백업은 그 순서를 따릅니다. 순서를 지정하지 않은 항목은 가장 기한이 지난 것부터 백업됩니다.

## 예시: 첫 컨테이너 백업, 처음부터 끝까지 {#example-first-backup}

`plex`를 비롯한 컨테이너 몇 개가 있는 갓 설치한 Unraid 장비에서 첫 백업이 끝나고 그것을 어디서 찾는지 알게 될 때까지 따라가 봅니다. 첫 백업에 걸리는 시간을 빼면 10분쯤 걸립니다.

1. **설치.** Unraid **Apps** 탭에서 **BombVault**를 검색하고 **Install**을 누릅니다.
2. **키 설정.** 컴퓨터에서 `openssl rand -hex 32`를 실행하고 64자를 템플릿의 `APP_KEY` 필드에 붙여넣습니다. 다른 무엇보다 먼저, 지금 비밀번호 관리자에 사본을 보관하세요. **Apply**를 누릅니다.
3. **열기.** `https://<your-unraid-ip>:3443`으로 가서 자체 서명된 인증서를 수락합니다. **BombVault에 오신 것을 환영합니다**에 컨테이너 12개, VM 2개, 플래시 드라이브처럼 찾은 것이 나열됩니다.
4. **시작하기.** 개요가 **첫 단계** 카드와 함께 열립니다.
5. **섹션 선택.** **컨테이너**는 처음부터 켜져 있습니다. **플래시**처럼 더 백업할 것을 켜세요. **선택**은 찾은 컨테이너의 목록을 열고, 거기서 **일정 일시 중지**로 신경 쓰지 않는 테스트 컨테이너 같은 것을 뺄 수 있습니다. 켜 둔 섹션은 전체 백업에서 매일 밤 실행됩니다.
6. **대상 추가.** 모든 섹션에는 이미 이 서버의 `/mnt/user/bombvault` 아래에 전용 폴더가 있으므로 첫 백업이 갈 곳은 있습니다. 이 단계는 두 번째 장소를 더하는 것이며, 가장 안전한 곳은 현장 밖입니다. 지금 [S3 예시](offsite-recovery.md#example-s3)로 해도 되고, 첫 백업이 끝난 뒤에 와서 해도 됩니다. 카드는 그때까지 남아 있습니다.
7. **첫 백업.** **백업**을 열고 **지금 모두 백업**을 누르거나 밤까지 기다립니다. 상태 머리글의 링이 진행 링으로 바뀌고 항목 수를 셉니다. 예를 들어 **지금 백업 중: 12개 중 3개**입니다. 각 컨테이너는 데이터를 읽는 동안 중지되었다가 바로 다시 시작됩니다. 데이터가 ZFS에 있는 컨테이너는 스냅샷을 찍는 몇 초 동안만 내려갑니다. 첫 백업은 모든 것을 읽으므로 가장 오래 걸립니다. 이후에는 바뀐 것만 저장합니다.
8. **확인.** 링에 **모든 항목이 백업됨**이라고 나오면 개요의 **기록**을 엽니다. 모든 컨테이너가 **실행**에 초록 줄을 가지고 있습니다. **백업**을 열고 `plex`를 클릭해 **백업** 탭을 보세요. 첫 스냅샷이 있고, 그것을 가진 장소로 **로컬**이 나옵니다. 몇 분 뒤 **검증** 탭에 복원 확인이 나타나는데, 백업의 표본을 복원해 다시 읽어 본 결과입니다.
9. **키 보호.** **설정, 보안**을 열고 로그인 비밀번호를 설정합니다. 그런 다음 **설정, 복원**을 열고 **긴급 상황용**에서 **복구 키트 다운로드**를 누릅니다. 파일을 서버 밖에, 2단계에서 보관한 `APP_KEY` 사본 옆에 두세요. 개요의 **복구 키트를 저장하세요** 알림은 **안전하게 보관했습니다**를 누르면 사라집니다.

이제부터 컨테이너는 매일 밤 백업됩니다. 이어서 할 만한 일을 순서대로 적으면, 서버가 죽어도 백업이 함께 사라지지 않도록 [오프사이트 대상](offsite-recovery.md#example-s3)을 만들고, 실패가 알려지도록 [알림](notifications.md)을 설정하고, 03:00이 맞지 않는다면 [일정](schedules.md)을 살펴보는 것입니다.

![첫 백업이 실행되는 동안의 상태 머리글. 진행 링과 그 단계들.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![첫 백업이 실행되는 동안의 상태 머리글. 진행 링과 그 단계들.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "호스트 통합 확인"
    개요의 **호스트 통합** 카드는 모든 마운트와 CLI(Docker 소켓, libvirt, restic, qemu-img, rclone)를 검사하고 누락된 부분을 보고하므로, 컨테이너에 의존하기 전에 올바르게 연결되었는지 확인할 수 있습니다. 추가하기 전까지는 **사용자 지정**에 들어 있습니다.

## 화면 둘러보기 {#navigation}

![타일로 보는 설정과, 고른 타일의 카드. 바꾸는 즉시 기록됩니다.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![타일로 보는 설정과, 고른 타일의 카드. 바꾸는 즉시 기록됩니다.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

사이드바에는 다섯 개의 항목이 있습니다. **개요**, **이상 징후**(이상 징후 감지가 켜져 있는 동안), **백업**, **복원**, 그리고 **로그아웃** 아래의 **설정**입니다. 페이지의 주요 동작은 오른쪽 아래에 떠 있고, 드문 동작은 **더보기**에 있습니다. 휴대폰에서는 막대에 개요, 백업, 복원, 설정이 있고 나머지는 **더보기**에 있습니다.

페이지와 설정은 창의 전체 너비를 씁니다. 창은 버튼을 아래쪽에 두고, 앞으로 진행하는 버튼은 오른쪽에 있습니다. 휴대폰에서는 버튼이 세 개를 넘는 아래쪽 영역이 닫는 버튼과 주요 동작만 남기고 나머지를 **더보기** 아래에 둡니다.

![휴대폰에서 더보기를 연 아래쪽 막대.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![휴대폰에서 더보기를 연 아래쪽 막대.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **개요**는 모든 것이 백업되었는지, 무엇에 손이 필요한지 보여 줍니다. [개요](overview.md)를 참고하세요.
- **백업**에는 모든 컨테이너, VM, 폴더 세트, 플래시 드라이브, 모든 ZFS 데이터세트, 셀프 백업이 항목으로 나열됩니다. 목록 위의 타일로 종류를 고르고 **필터**, **정렬**, **검색**으로 좁힙니다. 항목마다 전용 페이지가 있으며 **백업**, **설정**, **검증** 세 탭이 있습니다. 항목 아이콘 왼쪽의 뒤로 가기 버튼으로 목록에 돌아갑니다. [백업과 항목](backups.md)을 참고하세요.
- **복원**은 모든 것을 한 페이지에서 다룹니다. 섹션을 고르고, 항목을 검색하고, 백업을 선택합니다. 긴급 마법사와 다른 원본도 여기서 시작합니다. [복원](restore.md)을 참고하세요.
- **설정**은 열네 개의 타일입니다. 일반, 모양, 일정, 대상, 복원, 알림, 검증, 보안, 인스턴스, 페어링, 연동, 앱, 시스템, 정보. [설정](settings.md)을 참고하세요. `/` 또는 Ctrl+K로 모두 검색할 수 있고, 이전 버전에서 페이지가 쓰던 이름으로도 찾을 수 있습니다. **모양**에서 타일을 원하는 순서로 끌어 놓을 수 있습니다. 페이지 옆의 타일 열이 페이지를 좁게 만드는 경우, 예를 들어 너비가 약 1200픽셀인 창에서는 타일이 페이지 위의 가로로 스크롤되는 띠가 됩니다. 휴대폰도 같은 띠를 보여 줍니다.
- **대상**과 **인스턴스**는 전용 페이지가 있는 설정 타일입니다. 타일의 **사이드바에도 표시**가 이들을 사이드바에 고정합니다.

저장 버튼은 어디에도 없고 펼쳐지는 것도 없습니다. 드문 항목에는 전용 카드나 창이 있고, 설명은 (i) 말풍선에 있습니다. 업데이트 뒤에는 직전 버전 이후의 릴리스마다 카드가 하나씩 있는 창이 저절로 열리고, 바뀐 곳에 초록 점이 표시됩니다. 클릭하면 점이 사라집니다. **설정, 정보**의 버전 카드에 있는 **새 소식**은 아직 보지 않은 것의 수를 세고, 이 창을 다시 열어 각 릴리스의 노트와 그 변경을 **바로 가기**와 함께 보여 줍니다.

## 소스에서 빌드하기 {#build-from-source}

BombVault는 JSON API와 내장 React 인터페이스를 제공하는 단일 정적 Go 바이너리입니다. 먼저 인터페이스를 빌드한 다음 바이너리를 실행하세요.

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

`go run`에도 인터페이스 빌드가 필요합니다. 저장소는 `web/dist` 아래에 빈 표식 파일만 추적하므로, `npm --prefix web run build` 없이는 바이너리에 아무것도 내장되지 않고 `500 SPA index not found`로 응답하는데, 이는 예상된 동작입니다. Docker, libvirt, Unraid는 CI에서 테스트할 수 없으므로, 풀 리퀘스트를 열기 전에 실제 호스트에서 개요의 **호스트 통합** 카드(또는 `GET /api/spike`)로 마운트, restic, VM SSH 연결을 확인하세요.

## 다음 단계 {#next-steps}

- 현장 밖의 대상을 하나 이상 추가하고(각 섹션은 여러 대상에 동시에 복사할 수 있음) 복구 키트를 저장하세요. 방법은 **[오프사이트 및 복구](offsite-recovery.md)**에 있습니다. [S3 예시](offsite-recovery.md#example-s3)가 하나를 처음부터 안내합니다.
- **[알림](notifications.md)**으로 실패가 꼭 전해지게 하세요.
- **[일정](schedules.md)**에서 언제 실행되는지 확인하세요.
- **[Android 앱](android.md)**으로 그룹의 모든 서버를 휴대폰에 담으세요.
- **[실전 예시](guides.md)**는 자주 하는 작업을 안내하고, **[기능](features.md)**은 BombVault가 하는 모든 일을 나열합니다.
- 설정을 복제하거나 새 장비로 옮기시나요? **설정 내보내기 / 가져오기** 카드로 전체 구성을 옮기세요. [구성](configuration.md#portable-settings-export-and-import)을 참고하세요.
- 문제가 생겼나요? **[문제 해결](troubleshooting.md)**을 참고하세요.
