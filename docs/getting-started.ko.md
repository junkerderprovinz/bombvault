# 시작하기

이 페이지는 갓 설치한 Unraid 장비에서 첫 백업까지 안내합니다.

## 요구 사항

| 요구 사항 | 참고 |
|---|---|
| **Unraid 6.12+** | 이전 버전은 테스트되지 않았습니다. Unraid가 주 대상이지만, BombVault는 일반 Docker 호스트와 TrueNAS Scale에서도 실행됩니다([일반 Docker 호스트](#generic-docker-host) 참고). |
| **Restic 저장소 위치** | 로컬 경로(권장: 배열 또는 캐시), SMB, NFS, 또는 모든 rclone 백엔드. |
| **Docker 소켓** | 템플릿이 자동으로 마운트합니다(`/var/run/docker.sock`). |
| **Unraid 플래시** (`/boot`) | 템플릿이 전체를 자동으로 마운트합니다(`/boot`을 `/host/boot`로). 플래시 백업을 구동하고, 복원된 컨테이너가 정상적이고 편집 가능한 Unraid 앱으로 다시 나타나게 합니다. |
| **KVM VM** (선택 사용) | VM 백업은 SSH를 통해 libvirt와 통신하며 libvirt 마운트가 없습니다. **설정, 연동, 호스트 SSH**에서 구성하세요([구성](configuration.md) 참고). |
| **ZFS 데이터세트** (선택 사용) | VM 백업과 같은 SSH 연결, 호스트의 `zfs`, 그리고 접근 모드 Read/Write - Slave로 `/mnt`에 매핑된 Host Data(템플릿 기본값)가 필요합니다. [ZFS 데이터세트](zfs-datasets.md)를 참고하세요. |
| **Android 앱** (선택) | Android 10 이상이며, 버전 9.7.0 이상인 서버와 페어링합니다. [Android 앱](android.md)을 참고하세요. |

## Unraid에 설치

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

Unraid가 아니라면? BombVault는 어떤 Docker 호스트에서도 평범한 컨테이너로 돌아갑니다(TrueNAS Scale의 컨테이너 지원도, 그쪽 앱 카탈로그에 자체 항목이 생기기 전까지는 이것으로 굴러갑니다).

1. 저장소에서 바로 편집할 수 있는 [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml)을 가져옵니다.
2. `APP_KEY`(아래 참조)를 설정하고 Host Data 볼륨을 실제 데이터 루트로 지정합니다. 파일의 주석이 둘 다 안내합니다.
3. `docker compose up -d` 후 `https://<host-ip>:3443/`을 엽니다.

Unraid와 다른 점:

- **플래시 섹션이 없습니다.** 담거나 되살릴 부팅 USB가 없으므로 플래시 섹션은 여기서 할 일이 없습니다. 대신 백업 페이지의 **추가**가 **시스템 구성**을 폴더 제안으로 보여 줍니다. 저장 전에 살펴보고 손질하는 `/etc` 폴더 세트의 시작 묶음입니다.
- **Unraid 자체 알림이 없습니다.** BombVault 자신의 알림 채널(웹훅, 원격지 실패 경고 등)은 평소대로 동작합니다. 빠지는 것은 Unraid 고유 알림 시스템으로 보내는 것뿐이며, 여기에는 그런 시스템이 없기 때문입니다.
- **VM 백업은 선택 사항이며 SSH로 닿는 별도의 libvirtd 호스트가 필요합니다.** compose 파일의 주석 처리된 블록을 보십시오. 일반 Docker 호스트 자체에는 VM 관리자가 들어 있지 않습니다.
- **대시보드 위젯이 없습니다.** BombVault Widget은 Unraid 플러그인이므로 그 단계도 건너뜁니다.
- **컨테이너 데이터 찾기.** Unraid의 `appdata` 규칙이 없으므로, 컨테이너의 데이터 폴더는 `DATA_ROOT_SEGMENTS`의 구간, Docker 이름 있는 볼륨, Compose 프로젝트의 작업 디렉터리, `bombvault.data` 레이블로 찾습니다([백업 원본 자동 판별](configuration.md#backup-source-detection) 참고). 이름 있는 볼륨과 `/etc` 프리셋은 Host Data 마운트 안의 경로에만 닿으므로, Host Data는 Docker의 데이터 루트까지 포함하는 공통 상위 디렉터리로 지정하세요.
- **`PLATFORM`.** `generic` 또는 `truenas`로 설정합니다. 설정하지 않으면 BombVault는 플래시 마운트에 있는 Unraid 고유 표식으로 Unraid를 알아보고 그 밖의 경우는 모두 일반 호스트로 취급하며, Unraid 전용 단계는 시도했다 실패하는 대신 건너뜁니다.

**TrueNAS Scale**도 같은 compose 방식을 씁니다. 카탈로그 항목은 저장소에 준비되어 있지만 아직 제출하지 않았습니다. TrueNAS의 libvirtd는 자체 소켓(`/run/truenas_libvirt/libvirt-sock`)에서 대기하는데 세 개의 `LIBVIRT_*` 변수로는 이를 표현할 수 없으므로, 그곳에서 VM을 백업하려면 `LIBVIRT_URI`가 필요합니다([구성](configuration.md) 참고). 지금까지 확인된 범위는 이렇습니다. zvol 백업은 실제 TrueNAS Scale 장비에서 실행 중인 VM에 연결된 zvol을 대상으로 돌렸고, `zfs snapshot`, `zfs send`, restic, `zfs receive`를 한 바퀴 거친 결과가 바이트 단위로 일치했습니다. BombVault가 직접 수행하는 전체 복원은 아직 TrueNAS 하드웨어에서 실행해 보지 않았고, 그 zvol은 희소(sparse) 볼륨이었으므로 수 기가바이트 규모의 처리량은 검증되지 않았습니다. 그곳에서 믿고 쓰기 전에 복원을 시험해 보세요.

## 유일한 필수 설정

반드시 설정해야 하는 유일한 변수는 `APP_KEY`이며, restic 저장소 비밀번호를 파생하는 데 사용되는 32바이트 16진수 비밀 값(64개의 16진수 문자)입니다.

아무 장비에서나 하나 생성하세요:

```bash
openssl rand -hex 32
```

결과를 템플릿의 `APP_KEY` 필드(Unraid) 또는 `docker-compose.yml`의 `APP_KEY` 환경 변수(일반 Docker 호스트)에 붙여넣으세요.

!!! danger "APP_KEY를 잃어버리지 마세요"
    `APP_KEY`를 잃어버리면 암호화된 백업을 복구할 수 없게 됩니다. 서버와 분리된 안전한 곳에 보관하세요. BombVault가 실행되면 로그인 비밀번호를 설정하고 **설정, 복원**에서 **복구 키트**를 내려받으세요([오프사이트 및 복구](offsite-recovery.md#recovery-kit) 참고). 키, 모든 대상, restic 명령이 들어 있습니다.

템플릿은 또한 Docker 소켓, 플래시(`/boot`), 그리고 **Host Data** 루트(`/mnt`)를 대신 마운트합니다. 백업 *소스*와 *대상*은 모두 Host Data 아래에 있습니다. 전체 변수 참조와 오프사이트 설정은 [구성](configuration.md)을 참고하세요.

## 첫 실행

![첫 백업 뒤의 개요. 확인이 필요한 것, 각 섹션의 보호 상태, 대상, 기록.](assets/screenshots/dashboard.png)

*첫 백업 뒤의 개요. 확인이 필요한 것, 각 섹션의 보호 상태, 대상, 기록.*

1. `https://<your-unraid-ip>:3443`에서 웹 UI를 엽니다(기본으로 자체 서명된 인증서). 새로 설치하면 **BombVault에 오신 것을 환영합니다** 화면이 열리고, BombVault가 서버에서 찾은 것이 나열됩니다.
2. **시작하기**를 선택합니다. 그러면 개요에 세 단계가 있는 **첫 단계** 카드가 나타납니다.
    1. **섹션 선택**: BombVault가 백업할 것을 켭니다(컨테이너, VM, 플래시, 폴더, ZFS 데이터세트, 셀프 백업). **선택**은 섹션의 항목 목록을 열고, 거기서 **일정 일시 중지**로 하나를 뺄 수 있습니다. 여기서 켠 섹션은 매일 밤 실행되며, 이는 **설정, 일정**에서 바꿉니다.
    2. **대상 추가**: 백업이 가는 곳입니다. 시작하기에는 로컬 폴더로 충분합니다. 가장 안전한 것은 Storage Box나 Backblaze B2 같은, 현장 밖의 두 번째 대상입니다. 로컬 저장소는 기본적으로 `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`이며 첫 백업 시 생성됩니다.
    3. **첫 백업**: 그 뒤로는 매일 밤 저절로 실행됩니다.
3. **백업**을 열면 모든 항목이 한 목록에 나옵니다. **지금 모두 백업**은 켜 둔 모든 섹션에 대해 실행을 시작하고, 항목 페이지의 **지금 백업**은 그 하나만 백업합니다.

전에 BombVault를 써 보셨나요? 환영 화면의 **백업에서 복원**을 누르면 복원 페이지가 열리고, **긴급 마법사**가 잃어버린 서버의 설정, 컨테이너, VM, 폴더를 되살립니다. [오프사이트 및 복구](offsite-recovery.md#emergency)를 참고하세요.

!!! tip "선택 사항: 백업 순서 지정"
    일부 컨테이너를 항상 다른 것보다 먼저 백업해야 한다면(예: 앱을 사용하는 데이터베이스를 먼저), **설정, 일정**의 **순서** 카드를 열고 원하는 순서로 드래그하세요. 예약 실행과 전체 백업은 그 순서를 따릅니다. 순서를 지정하지 않은 항목은 가장 기한이 지난 것부터 백업됩니다.

!!! note "호스트 통합 확인"
    개요의 **호스트 통합** 카드는 모든 마운트와 CLI(Docker 소켓, libvirt, restic, qemu-img, rclone)를 검사하고 누락된 부분을 보고하므로, 컨테이너에 의존하기 전에 올바르게 연결되었는지 확인할 수 있습니다. 추가하기 전까지는 **사용자 지정**에 들어 있습니다.

## 화면 둘러보기 {#navigation}

![타일로 보는 설정과, 고른 타일의 카드. 바꾸는 즉시 기록됩니다.](assets/screenshots/settings.png)

*타일로 보는 설정과, 고른 타일의 카드. 바꾸는 즉시 기록됩니다.*

사이드바에는 다섯 개의 항목이 있습니다. **개요**, **이상 징후**(이상 징후 감지가 켜져 있는 동안), **백업**, **복원**, 그리고 **로그아웃** 아래의 **설정**입니다. 페이지의 주요 동작은 오른쪽 아래에 떠 있고, 드문 동작은 **더보기**에 있습니다. 휴대폰에서는 막대에 개요, 백업, 복원, 설정이 있고 나머지는 **더보기**에 있습니다.

- **백업**에는 모든 컨테이너, VM, 폴더 세트, 플래시, 모든 ZFS 데이터세트, 셀프 백업이 항목으로 나열됩니다. 목록 위의 타일로 종류를 고르고 **필터**, **정렬**, **검색**으로 좁힙니다. 항목마다 전용 페이지가 있으며 **백업**, **설정**, **검증** 세 탭이 있습니다.
- **복원**은 모든 것을 한 페이지에서 다룹니다. 섹션을 고르고, 항목을 검색하고, 백업을 선택합니다. 긴급 마법사와 다른 원본도 여기서 시작합니다.
- **설정**은 열네 개의 타일입니다. 일반, 모양, 일정, 대상, 복원, 알림, 검증, 보안, 인스턴스, 페어링, 연동, 앱, 시스템, 정보. `/` 또는 Ctrl+K로 모두 검색할 수 있고, 이전 버전에서 페이지가 쓰던 이름으로도 찾을 수 있습니다. **모양**에서 타일을 원하는 순서로 끌어 놓을 수 있습니다. 페이지 옆의 타일 열이 페이지를 좁게 만드는 경우, 예를 들어 너비가 약 1200픽셀인 창에서는 타일이 페이지 위의 가로로 스크롤되는 띠가 됩니다. 휴대폰도 같은 띠를 보여 줍니다.
- **대상**과 **인스턴스**는 전용 페이지가 있는 설정 타일입니다. 타일의 **사이드바에도 표시**가 이들을 사이드바에 고정합니다.

저장 버튼은 어디에도 없고 펼쳐지는 것도 없습니다. 드문 항목에는 전용 카드나 창이 있고, 설명은 (i) 말풍선에 있습니다. 업데이트 뒤에는 릴리스 노트가 저절로 열리고, 바뀐 곳에 초록 점이 표시됩니다. 클릭하면 점이 사라집니다. **설정, 정보**의 버전 카드에 있는 **새 소식**은 아직 보지 않은 것의 수를 세고, 릴리스 노트와 모든 변경의 목록을 **바로 가기**와 함께 한 창의 두 탭으로 엽니다.

## 소스에서 빌드하기 {#build-from-source}

BombVault는 JSON API와 내장 React 인터페이스를 제공하는 단일 정적 Go 바이너리입니다. 먼저 인터페이스를 빌드한 다음 바이너리를 실행하세요:

```bash
npm --prefix web ci
npm --prefix web run build     # web/dist에 기록하며, 바이너리가 이를 내장함
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # 단위 및 통합 테스트, 실제 restic 왕복 포함
golangci-lint run ./...
go run ./cmd/bombvault         # 자체 서명 인증서로 https://localhost:3443 제공
```

`go run`에도 인터페이스 빌드가 필요합니다. 저장소는 `web/dist` 아래에 빈 표식 파일만 추적하므로, `npm --prefix web run build` 없이는 바이너리에 아무것도 내장되지 않고 `500 SPA index not found`로 응답하는데, 이는 예상된 동작입니다. Docker, libvirt, Unraid는 CI에서 테스트할 수 없으므로, 풀 리퀘스트를 열기 전에 실제 호스트에서 개요의 **호스트 통합** 카드(또는 `GET /api/spike`)로 마운트, restic, VM SSH 연결을 확인하세요.

## 다음 단계

- 전체 **[기능](features.md)**을 둘러보세요.
- **[Android 앱](android.md)**으로 그룹의 모든 서버를 휴대폰에 담으세요.
- 오프사이트 대상을 하나 이상 추가하고(각 섹션은 여러 대상에 동시에 복사할 수 있음) 복구 키트를 저장하세요. 방법은 **[오프사이트 및 복구](offsite-recovery.md)**에 있습니다.
- 설정을 복제하거나 새 장비로 옮기시나요? **설정 내보내기 / 가져오기** 카드로 전체 구성을 옮기세요. [구성](configuration.md#portable-settings-export-and-import)을 참고하세요.
- 문제가 생겼나요? **[문제 해결](troubleshooting.md)**을 참고하세요.
