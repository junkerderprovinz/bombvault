# BombVault

**당신의 Unraid 데이터를 금고에 봉인하세요. 백업을 넣고, 복원을 폭파하세요.**

BombVault는 Docker 컨테이너와 KVM/libvirt VM의 **백업과 전체 재해 복구**를 위한 셀프 호스팅, Unraid 네이티브 웹 앱입니다. 단일 멀티 아치 Docker 컨테이너로 실행되며, 현대적인 다크 웹 UI를 제공하고, 전체 수명 주기(백업, 예약, 검증, 복원)를 처리합니다.

복원은 자동입니다. 컨테이너는 이전과 똑같이 Unraid Docker 탭에 다시 나타나고, VM은 디스크와 UEFI NVRAM이 다시 연결된 상태로 VM Manager에 다시 정의됩니다. 수동 재설치도, 재구성도, 소동도 없습니다.

[restic](https://restic.net)로 구동되므로 모든 백업은 중복 제거되고 증분이며 항상 암호화됩니다.

!!! note "APP_KEY를 안전하게 보관하세요"
    BombVault는 `APP_KEY`라는 이름의 32바이트 비밀 값에서 restic 저장소 비밀번호를 파생합니다. 이를 잃어버리면 암호화된 백업을 복구할 수 없게 됩니다. `openssl rand -hex 32`로 하나 생성하여 안전한 곳에 보관하세요. [구성](configuration.md)을 참고하세요.

## BombVault가 보호하는 대상

| 도메인 | 저장되는 항목 |
|---|---|
| **Docker 컨테이너** | Appdata 디렉터리와 컨테이너 정의(이미지, 환경 변수, 포트, 레이블, 볼륨). |
| **KVM / libvirt VM** | VM 디스크 이미지, XML 정의, UEFI NVRAM. SSH를 통해 백업됩니다(libvirt 마운트 없음). |
| **Unraid 플래시** | USB 플래시 전체(`/boot`): OS, 라이선스, 배열 구성, 공유, 네트워크 및 플러그인 구성. |
| **앱 구성** | BombVault 자체 `/config`: 설정 데이터베이스, 오프사이트 자격 증명, libvirt SSH 키 쌍. |
| **파일 및 폴더** | 이름이 지정된 **파일 세트**. 서버의 모든 폴더로, 각각 선택적으로 세트별 제외 패턴을 가질 수 있습니다. |

## 복원이 주인공

restic 스냅샷에서 데이터를 다시 복사한 후, BombVault는 저장된 컨테이너 정의를 Docker API에 대해 재생하므로 컨테이너가 언제나 그 자리에 있었던 것처럼 Unraid Docker 탭에 다시 나타납니다(같은 이미지, 같은 설정, 같은 포트 매핑). VM은 SSH를 통해 XML이 다시 정의되고 디스크와 UEFI NVRAM이 다시 연결되며, VM이 삭제된 후에도 마찬가지입니다.

백업이 종속 컨테이너를 중지할 때, 이들은 올바른 순서로 돌아옵니다. BombVault는 Compose의 `depends_on` 순서대로 재시작하고 각 컨테이너가 정상 상태를 보고할 때까지 기다린 후에 그에 의존하는 컨테이너를 시작합니다. 따라서 아직 가동되지 않은 데이터베이스나 게이트웨이보다 먼저 앞서 나가는 것은 없습니다. [기능](features.md)을 참고하세요.

## 작동 방식

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault는 오케스트레이션 및 UI 계층이지 스토리지 엔진이 아닙니다. 실제 모든 데이터 이동은 restic을 거칩니다.

## 빠른 시작

여기가 처음이신가요? **[시작하기](getting-started.md)**로 이동하여 Community Applications를 통해 Unraid에 BombVault를 설치하고 첫 백업을 실행하세요. 그런 다음 전체 **[기능](features.md)**을 살펴보고, **[구성](configuration.md)**을 조정하고, **[오프사이트 및 복구](offsite-recovery.md)**를 설정하세요.

오프사이트는 도메인별로 여러 대상에 동시에 분산될 수 있으며, 읽기 전용 **수신자 대시보드**가 그 사본을 받는 쪽 장비에서 모니터링하고, **설정 내보내기 및 가져오기** 카드로 전체 구성을 새 장비로 옮길 수 있습니다. [오프사이트 및 복구](offsite-recovery.md)와 [구성](configuration.md#portable-settings-export-and-import)을 참고하세요.

## 링크

- **소스 코드:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid 지원 스레드:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **이슈:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "호스트에 대한 root에 준하는 제어 권한"
    Docker 소켓을 통해 BombVault는 컨테이너를 중지, 제거, 재생성할 수 있고 appdata를 읽고 쓸 수 있으며, VM 백업을 위해 SSH를 통해 호스트에 로그인하여 `virsh`를 실행합니다. 그 웹 UI에 접근할 수 있는 사람은 사실상 호스트에 대한 root 권한을 가진 것과 같습니다. BombVault는 신뢰할 수 있고 외부에 노출되지 않은 네트워크에서만 실행하고, 오프사이트 또는 불변 백업을 사용하게 되면 선택적 비밀번호 게이트(설정, 보안)를 활성화하세요. 전체 보안 모델은 [구성](configuration.md)을 참고하세요.
