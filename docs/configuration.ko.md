# 구성

이 페이지는 컨테이너의 환경 변수, 템플릿이 제공하는 마운트, SSH를 통한 VM 백업, 오프사이트 설정을 다룹니다. 백업 **저장소 경로**는 환경 변수가 아니라 앱 내부(설정, 백업 경로)에서 구성합니다.

## 환경 변수

| 변수 | 필수 | 설명 |
|---|---|---|
| `APP_KEY` | **예** | restic 저장소 비밀번호를 파생하는 데 사용되는 32바이트 16진수 비밀 값(64개의 16진수 문자). `openssl rand -hex 32`로 생성하세요. 안전하게 보관하세요: 잃어버리면 암호화된 백업을 복구할 수 없게 됩니다. |
| `LIBVIRT_HOST` | VM용 | VM 백업을 위해 SSH로 도달하는 Unraid 호스트(기본값 `host.docker.internal`; 템플릿이 LAN IP 자리 표시자를 미리 채웁니다). Unraid LAN IP를 사용하세요. 사용자 지정 `br0.x` 네트워크에서는 필수입니다. |
| `LIBVIRT_SSH_PORT` | 아니요 | VM 백업용 호스트 SSH 포트(기본값 `22`). |
| `LIBVIRT_SSH_USER` | 아니요 | VM 백업용 호스트의 SSH 사용자(기본값 `root`). |
| `LIBVIRT_URI` | 아니요 | 전체 libvirt 연결 URI입니다. 위의 세 `LIBVIRT_*` 변수로 조합하는 대신 이 값을 **그대로** 사용합니다(이 경우 위 변수들은 연결 문자열 생성에 쓰이지 않습니다). 기본값은 설정되지 않음입니다. TrueNAS Scale에서 필요합니다. 이곳의 libvirtd는 조합 방식으로는 표현할 수 없는 비표준 소켓에서 대기합니다. 예: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. TrueNAS Scale 섹션은 [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)를 참고하세요. |
| `PORT` | 아니요 | HTTP 포트(기본값 `3000`; `HTTP_ONLY=true`에서만 사용됨). |
| `HTTPS_PORT` | 아니요 | HTTPS 포트(기본값 `3443`; 템플릿이 1:1로 게시하므로 WebUI가 `https://<ip>:3443`에서 응답함). |
| `HTTP_ONLY` | 아니요 | 자체 서명 HTTPS 리스너를 비활성화하고 일반 HTTP만 제공하려면 `true`로 설정(TLS 종료 리버스 프록시 뒤에서 사용). |
| `HOST_SOURCE_ROOT` | 아니요 | **Host Data**로 마운트되는 호스트 경로(기본값 `/mnt`). BombVault는 Docker가 보고하는 바인드 마운트 소스를 이 마운트 아래의 경로로 변환합니다. 다른 호스트 루트를 마운트한 경우에만 변경하세요. |
| `DATA_ROOT_SEGMENTS` | 아니요 | 바인드 마운트 소스를 백업 데이터로 표시하는, 쉼표로 구분한 경로 세그먼트 이름 목록입니다(기본값 `appdata`, Unraid의 `/mnt/user/appdata/<container>` 규칙과 일치). 나열된 세그먼트 중 하나라도 호스트 소스의 전체 경로 세그먼트로 나타나면 해당 컨테이너의 바인드 마운트가 백업 대상으로 자동 선택됩니다. 예를 들어 `DATA_ROOT_SEGMENTS=appdata,config`는 `.../config` 바인드도 함께 포함시킵니다. 컨테이너의 데이터 폴더를 찾는 그 밖의 상시 활성 방식은 [백업 소스 감지](#backup-source-detection)를 참고하세요. |
| `PLATFORM` | 아니요 | 자동 감지 대신 BombVault가 자신이 실행 중이라고 판단할 플랫폼을 강제로 지정합니다: `unraid`, `generic`, `truenas` 중 하나입니다(기본값은 설정되지 않음입니다. 플래시 마운트 아래의 `dockerMan` 표시를 찾아 Unraid를 자동 감지하고, 찾지 못하면 `generic`으로 처리합니다. 인식되지 않는 값도 `generic`으로 대체되며 이때 로그가 남습니다). 일반 Docker 호스트나 TrueNAS Scale에서는 Unraid 전용 자동 감지에 의존하지 말고 명시적으로 설정하세요. 일반용 compose 파일이 이렇게 되어 있습니다. 이 값은 appdata 대체 규칙, 인스턴스 간 복원 대상 기본값, Unraid 전용 알림/컴패니언 플러그인 단계 시도 여부를 바꿉니다(`internal/platform` 참고). |
| `BOMBVAULT_SELF_CONTAINER` | 아니요 | BombVault 컨테이너 자체의 이름으로, 자기 자신을 절대 백업(따라서 중지)하지 않도록 합니다(기본값 `BombVault`; 브리지 네트워킹에서 호스트 이름으로 자동 감지). |
| `BACKUP_MAX_HOURS` | 아니요 | 단일 백업 실행이 강제 취소되기 전에 도메인 잠금을 유지할 수 있는 최대 실제 경과 시간(멈춘 실행이 도메인을 영원히 차단하지 못하게 하는 보호 장치). 비어 있으면(기본값) `48`을 사용합니다. 매우 크거나 느린 클라우드 백업에는 이를 높이세요(상한에서 취소된 실행은 `context deadline exceeded`로 실패함). 상한을 완전히 비활성화하려면 `0`으로 설정하세요. |
| `TZ` | 아니요 | 스케줄러용 시간대(예: `Europe/Berlin`). **설정하지 않으면 모든 일정이 UTC로 실행됩니다**: 02:30으로 설정한 일정은 현지 시간이 아니라 02:30 UTC에 시작됩니다. Unraid에서는 직접 설정하지 않습니다. 시스템이 자체 시간대를 모든 컨테이너에 전달합니다. |

## 마운트

CA 템플릿에 표시된 대로 Docker 소켓, 플래시(`/boot`), **Host Data** 루트(`/mnt`)를 마운트하세요. 백업 *소스*와 *대상*은 모두 Host Data 아래에 있으며, **rslave**로 마운트되므로 컨테이너 시작 후에 마운트되는 원격 공유(예: `/mnt/remotes` 아래)가 재시작 없이 보이게 됩니다.

백업 저장소 경로는 기본적으로 `/mnt/user/bombvault/{container,vms,flash,config,files}`이며 첫 백업 시 생성됩니다. **설정, 백업 경로**에서 언제든지 위치를 변경할 수 있습니다.

!!! note "호스트 통합 확인"
    컨테이너가 시작된 후 웹 UI에서 `/spike`를 엽니다. 모든 마운트와 CLI(Docker 소켓, libvirt, restic, qemu-img, rclone)를 검사하고 누락된 부분을 보고합니다.

## 보안 모델

!!! warning "호스트에 대한 root에 준하는 제어 권한"
    Docker 소켓을 통해 BombVault는 컨테이너를 중지, 제거, 재생성할 수 있고 appdata를 읽고 쓸 수 있으며, VM 백업을 위해 SSH(`qemu+ssh://`, 기본 root)를 통해 호스트에 로그인하여 `virsh`를 실행합니다. 그 웹 UI에 접근할 수 있는 사람은 사실상 호스트에 대한 root 권한을 가진 것과 같습니다.

- **선택적 비밀번호 보호**(설정, 보안): 로그인을 요구하려면 비밀번호를 설정하고, 비활성화하려면 지우세요. 신뢰할 수 있는 LAN 사용을 위해 기본적으로 꺼져 있습니다. 세션은 서명되며(`APP_KEY`에서 파생된 HMAC) 비밀번호를 변경하면 무효화됩니다. 로그인은 속도 제한이 적용됩니다.
- 게이트는 선택 사용이므로, 설정하지 않으면 전체 UI와 API(오프사이트 설정, 변조 테스트 경로, 복구 키트 포함)에 포트에 접근할 수 있는 누구나 도달할 수 있습니다. 오프사이트, 불변 백업 또는 암호화를 사용하게 되면 게이트를 활성화하세요.
- BombVault는 신뢰할 수 있고 외부에 노출되지 않은 네트워크에서만 실행하세요. 원격 접근에는 인증과 TLS를 추가하는 리버스 프록시 뒤에 두세요. 응답에는 기본 보안 헤더(CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`)가 포함됩니다.
- `HTTP_ONLY=true`에서는 세션 쿠키가 `Secure` 플래그를 잃으므로(일반 HTTP에서 작동하려면 그래야 함), 기밀성이 중요하다면 TLS 종료 프록시 뒤에서만 비밀번호를 활성화하세요.
- VM 백업 SSH 연결은 첫 접속 시 호스트 키를 신뢰하고(TOFU) 이후 이를 고정합니다. 컨테이너에서 호스트로의 경로가 신뢰할 수 없다면 호스트의 키를 대역 외로 확인하세요.
- 암호화가 활성화되면(설정; 기본 켜짐) 백업은 restic에 의해 암호화되며, 키는 `APP_KEY`에서 파생됩니다.

## SSH를 통한 VM 백업

BombVault는 **어떤 libvirt 경로도 마운트하지 않고** KVM/libvirt VM을 백업합니다. SSH(`qemu+ssh://`)를 통해 호스트에서 `virsh`를 실행하므로 호스트 VM Manager에 절대 영향을 줄 수 없습니다.

빠른 설정:

1. **설정, 시스템, SSH를 통한 VM 백업:** 표시된 공개 키를 복사합니다.
2. Unraid의 `/root/.ssh/authorized_keys`에 추가합니다(재부팅 후에도 유지되도록 플래시에도 저장됨).
3. **연결 테스트**를 클릭합니다.

템플릿은 컨테이너가 호스트에 도달할 수 있도록 `--add-host=host.docker.internal:host-gateway`를 추가합니다. 그 이름이 확인되지 않으면(예: 컨테이너가 사용자 지정 `br0.x` 네트워크에서 실행될 때) `LIBVIRT_HOST`를 Unraid LAN IP로 설정하세요. Unraid의 SSH 포트를 변경했다면 `LIBVIRT_SSH_PORT`를 일치하도록 설정하세요. **라이브 스냅샷**은 추가로 VM에 qemu 게스트 에이전트가 있어야 하고 디스크가 `/mnt/cache`(`/mnt/user`가 아님)에 있어야 합니다.

!!! important "전체 VM 설정 및 네트워킹 가이드"
    전체 단계별 가이드(SSH 활성화, 영구 키 승인, 사용자 지정 네트워크 및 VLAN 라우팅, VM별 방식과 호스트 측 문제 해결)는 GitHub의 [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)에 있습니다.

## 오프사이트 설정

**설정, 오프사이트** 탭에서 오프사이트 복제본을 구성하세요. 전체 워크플로(불변/append-only, 변조 테스트, DR 리허설)는 [오프사이트 및 복구](offsite-recovery.md)를 참고하세요. 요약하면:

- **백엔드:** SMB/CIFS와 NFS(공유를 마운트하고 백업 경로를 그곳으로 지정), rclone 없는 네이티브 restic 백엔드(`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), 또는 모든 rclone 원격(`rclone:<remote>:<bucket>/path`).
- **클라우드 자격 증명**은 설정, 오프사이트, 클라우드 자격 증명 아래에 암호화되어 저장됩니다.
- **SSH 대상은 상대편에 아무것도 설치할 필요가 없습니다.** `sftp:`는 SSH 서버만 필요합니다. **설정, 시스템, SSH를 통한 VM 백업**의 공개 키(`/config/ssh/id_ed25519.pub`에도 있음)를 대상 사용자의 `~/.ssh/authorized_keys`에 추가하세요.
- **오프사이트 복사:** BombVault는 새 스냅샷을 `restic copy`로 최선 노력 방식으로 복제합니다. 로컬 저장소가 주 저장소로 유지됩니다. 각 도메인은 자체 오프사이트 일정을 가지며, **지금 복제** 버튼도 있습니다.
- **도메인별 여러 오프사이트 대상:** 각 도메인은 여러 오프사이트 대상에 동시에 복제할 수 있습니다. 설정, 오프사이트에서 추가 대상을 더하세요. 각각 자체 저장소, S3 스토리지 클래스, append-only 플래그, 보존, 증가 예산을 가지며, 모두 그 도메인의 오프사이트 일정에 따라 복제됩니다. 기존의 단일 오프사이트 설정은 첫 번째 대상으로 이어집니다.
- **소스별 보존:** 로컬 정책은 설정, 경로 및 스토리지에, 오프사이트 정책은 설정, 오프사이트에 있습니다(오프사이트 스냅샷을 자동으로 정리하지 않으려면 모두 0으로 두세요).
- **대역폭 제한:** 설정, 오프사이트에서 restic 업로드/다운로드 속도를 제한하세요.
- **콜드 및 아카이브 스토리지 클래스(S3):** 네이티브 S3 오프사이트 저장소의 경우 복원 가능한 계층(Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval)을 선택하세요. rclone 원격은 rclone 구성에서 클래스를 설정합니다.

## 이식 가능한 설정(내보내기 및 가져오기) {#portable-settings-export-and-import}

설정 페이지의 **설정 내보내기 및 가져오기** 카드는 전체 BombVault 구성(도메인 설정, 오프사이트 대상, 일정, 보존, 알림)을 다른 인스턴스에서 가져올 수 있는 이식 가능한 JSON 파일로 기록하므로, 새 장비로 옮기거나 설정을 복제할 때 모든 것을 손으로 다시 입력할 필요가 없습니다. 가져오기는 미리 보기를 표시하고 확인을 요청하며, 백업 데이터나 기록을 절대 건드리지 않습니다.

!!! warning "내보내기에 자격 증명이 포함될 수 있습니다"
    오프사이트 및 알림 자격 증명을 파일에 포함할지 선택할 수 있습니다. 자격 증명을 포함하면 내보내기가 복구 키트만큼 민감해지므로 안전한 곳에 보관하세요. 포함하지 않으면 파일에는 비밀이 아닌 설정만 담깁니다.
