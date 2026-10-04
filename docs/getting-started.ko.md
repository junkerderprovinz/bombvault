# 시작하기

이 페이지는 갓 설치한 Unraid 장비에서 첫 백업까지 안내합니다.

## 요구 사항

| 요구 사항 | 참고 |
|---|---|
| **Unraid 6.12+** | 이전 버전은 테스트되지 않았습니다. Unraid가 주 대상이지만, BombVault는 일반 Docker 호스트와 TrueNAS Scale에서도 실행됩니다([일반 Docker 호스트](#generic-docker-host) 참고). |
| **Restic 저장소 위치** | 로컬 경로(권장: 배열 또는 캐시), SMB, NFS, 또는 모든 rclone 백엔드. |
| **Docker 소켓** | 템플릿이 자동으로 마운트합니다(`/var/run/docker.sock`). |
| **Unraid 플래시** (`/boot`) | 템플릿이 전체를 자동으로 마운트합니다(`/boot`을 `/host/boot`로). 플래시 백업을 구동하고, 복원된 컨테이너가 정상적이고 편집 가능한 Unraid 앱으로 다시 나타나게 합니다. |
| **KVM VM** (선택 사용) | VM 백업은 SSH를 통해 libvirt와 통신하며 libvirt 마운트가 없습니다. 설정에서 구성하세요([구성](configuration.md) 참고). |
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

- **flash/USB 도메인이 없습니다.** 담거나 되살릴 부팅 USB가 없으므로 설정의 플래시 도메인은 여기서 할 일이 없습니다. 대신 폴더 도메인이 한 번의 클릭으로 **프리셋 추가: 호스트 시스템 구성**(저장 전에 살펴보고 손질하는 `/etc` 시작 파일 묶음)을 실용적인 일반 대응물로 제안합니다.
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
    `APP_KEY`를 잃어버리면 암호화된 백업을 복구할 수 없게 됩니다. 서버와 분리된 안전한 곳에 보관하세요. BombVault가 실행되면, 원클릭 **암호화 키 복구 키트**([오프사이트 및 복구](offsite-recovery.md) 참고)를 사용하여 전체 복구 번들을 저장하세요.

템플릿은 또한 Docker 소켓, 플래시(`/boot`), 그리고 **Host Data** 루트(`/mnt`)를 대신 마운트합니다. 백업 *소스*와 *대상*은 모두 Host Data 아래에 있습니다. 전체 변수 참조와 오프사이트 설정은 [구성](configuration.md)을 참고하세요.

## 첫 실행

![첫 백업 뒤의 대시보드. 무엇이 보호되고, 다음에 무엇이 돌고, 지금 무엇이 일어나는지.](assets/screenshots/dashboard.png)

*첫 백업 뒤의 대시보드. 무엇이 보호되고, 다음에 무엇이 돌고, 지금 무엇이 일어나는지.*

1. `https://<your-unraid-ip>:3443`에서 웹 UI를 엽니다(기본으로 자체 서명된 인증서).
2. **설정**에서 원하는 백업 도메인(컨테이너, VM, 플래시, 셀프 백업, 폴더, ZFS 데이터세트)을 활성화하고 강조 색상을 선택합니다.
3. **컨테이너** 탭에서 컨테이너를 선택하고 **지금 백업**을 클릭하여 첫 복원 지점을 만듭니다. 저장소 경로는 기본적으로 `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`이며 첫 백업 시 생성됩니다.
4. **설정, 일정**에서 예약을 구성합니다. 컨테이너와 VM에 대해 원클릭 *모두 일정에 포함*이 있습니다.

!!! tip "선택 사항: 백업 순서 지정"
    일부 컨테이너를 항상 다른 것보다 먼저 백업해야 한다면(예: 앱을 사용하는 데이터베이스를 먼저), 컨테이너 페이지에서 **백업 순서** 패널을 열고 원하는 순서로 드래그하세요. 예약 및 다중 선택 실행은 그 순서를 따릅니다. 순서를 지정하지 않은 항목은 이전과 같이 가장 기한이 지난 것부터 백업됩니다.

!!! note "호스트 통합 확인"
    컨테이너가 시작된 후 웹 UI에서 `/spike`를 엽니다. 모든 마운트와 CLI(Docker 소켓, libvirt, restic, qemu-img, rclone)를 검사하고 누락된 부분을 보고하므로, 컨테이너에 의존하기 전에 올바르게 연결되었는지 확인할 수 있습니다.

## 간단 모드 vs 고급 모드

![설정에는 저장 버튼이 없습니다. 바꾸는 즉시 기록됩니다.](assets/screenshots/settings.png)

*설정에는 저장 버튼이 없습니다. 바꾸는 즉시 기록됩니다.*

기본적으로 인터페이스는 필수 항목(백업, 복원, 예약)만 표시합니다. 사이드바의 **간단히 보기 / 고급 보기** 스위치를 사용하여 전문가용 컨트롤을 표시하세요: 보존, 오프사이트 복사, 사전/사후 훅, 파일 수준 복원, 알림, Prometheus 메트릭, 무결성/유지 관리 도구. 브라우저별 설정이며 기본적으로 꺼져 있으므로, 처음 오는 사람은 깔끔한 UI를, 파워 유저는 모든 기능을 얻습니다.

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

`go run`에도 인터페이스 빌드가 필요합니다. 저장소는 `web/dist` 아래에 빈 표식 파일만 추적하므로, `npm --prefix web run build` 없이는 바이너리에 아무것도 내장되지 않고 `500 SPA index not found`로 응답하는데, 이는 예상된 동작입니다. Docker, libvirt, Unraid는 CI에서 테스트할 수 없으므로, 풀 리퀘스트를 열기 전에 실제 호스트에서 호스트 통합 검사(`/spike`)로 마운트, restic, VM SSH 연결을 확인하세요.

## 다음 단계

- 전체 **[기능](features.md)**을 둘러보세요.
- **[Android 앱](android.md)**으로 그룹의 모든 서버를 휴대폰에 담으세요.
- 하나 이상의 **[오프사이트 및 복구](offsite-recovery.md)** 복제본을 추가하고(각 도메인은 여러 대상에 동시에 전송할 수 있음) 복구 키트를 저장하세요.
- 설정을 복제하거나 새 장비로 옮기시나요? **설정 내보내기 / 가져오기** 카드로 전체 구성을 옮기세요. [구성](configuration.md#portable-settings-export-and-import)을 참고하세요.
- 문제가 생겼나요? **[문제 해결](troubleshooting.md)**을 참고하세요.
