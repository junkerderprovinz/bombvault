# BombVault

**Unraid 데이터를 금고에 봉인하세요. 백업을 넣고, 복원을 폭파하세요.**

![개요: 상태 머리글, 확인이 필요한 것, 각 섹션의 보호 상태, 대상, 기록.](assets/screenshots/overview/home-dark.png#only-dark)
![개요: 상태 머리글, 확인이 필요한 것, 각 섹션의 보호 상태, 대상, 기록.](assets/screenshots/overview/home-light.png#only-light)

BombVault는 Unraid와 TrueNAS SCALE, ZimaOS, 일반 Linux 장비 같은 다른 Docker 호스트에서 **백업과 전체 재해 복구**를 맡는 셀프 호스팅 웹 앱입니다. Docker 컨테이너, KVM/libvirt VM, 폴더, ZFS 데이터세트, Unraid 플래시 드라이브를 백업합니다. 단일 멀티 아치 Docker 컨테이너로 실행되고, 다크, 라이트, 시스템 테마를 고를 수 있는 웹 화면을 제공하며, 백업, 예약, 검증, 복원까지 전체 수명 주기를 처리합니다.

복원은 자동입니다. Unraid에서는 컨테이너가 이전과 똑같이 Docker 탭에 다시 나타나고, VM은 디스크와 UEFI NVRAM이 다시 연결된 상태로 VM Manager에 다시 정의됩니다. 수동 재설치도, 재구성도, 소동도 없습니다. TrueNAS에서 원본 옆에 복원한 컨테이너는 Apps 화면에 나타나지 않는 일반 Docker 컨테이너로 실행됩니다.

[restic](https://restic.net)로 구동되므로 모든 백업은 중복 제거되고 증분이며 기본적으로 암호화됩니다.

!!! note "APP_KEY를 안전하게 보관하세요"
    BombVault는 `APP_KEY`라는 이름의 32바이트 비밀 값에서 restic 저장소 비밀번호를 파생합니다. 이를 복구 키트와 함께 잃어버리면 암호화된 백업을 복구할 수 없게 됩니다. `openssl rand -hex 32`로 하나 생성하여 안전한 곳에 보관하세요. [구성](configuration.md)을 참고하세요.

## BombVault가 보호하는 대상 {#what-bombvault-protects}

컨테이너, VM, 폴더 세트, 플래시 드라이브, 모든 ZFS 데이터세트, BombVault 자체 설정은 각각 전용 페이지가 있는 **항목**입니다. 같은 종류의 항목이 모여 **섹션**이 되며, **설정, 일반**에서 켭니다.

| 섹션 | 저장되는 항목 |
|---|---|
| **컨테이너** | Appdata 디렉터리와 컨테이너 정의(이미지, 환경 변수, 포트, 레이블, 볼륨). |
| **가상 머신** | VM 디스크 이미지, XML 정의, UEFI NVRAM. SSH를 통해 백업됩니다(libvirt 마운트 없음). |
| **플래시** | USB 플래시 전체(`/boot`): OS, 라이선스, 배열 구성, 공유, 네트워크 및 플러그인 구성. |
| **셀프 백업** | BombVault 자체 `/config`: 설정 데이터베이스, 대상의 로그인 정보, libvirt SSH 키 쌍, ZFS 서버의 키와 고정된 호스트 키, 인증서. |
| **폴더** | 서버의 모든 폴더를 폴더 세트로 저장합니다. 각각 선택적으로 제외 패턴을 가질 수 있습니다. |
| **ZFS 데이터세트** | 데이터세트와 그 아래의 모든 데이터세트를 하나의 ZFS 스냅샷에서 읽어 폴더처럼 저장합니다. [ZFS 데이터세트](zfs-datasets.md)를 참고하세요. |

## 복원이 주인공 {#restore-is-the-star}

restic 스냅샷에서 데이터를 다시 복사한 후, BombVault는 저장된 컨테이너 정의를 Docker API에 대해 재생하므로 컨테이너가 언제나 그 자리에 있었던 것처럼 Unraid Docker 탭에 다시 나타납니다(같은 이미지, 같은 설정, 같은 포트 매핑). VM은 SSH를 통해 XML이 다시 정의되고 디스크와 UEFI NVRAM이 다시 연결되며, VM이 삭제된 후에도 마찬가지입니다.

창 하나로 모든 종류를 복원합니다. 기본으로 **원본 옆에**가 선택되어 있어 백업을 사본으로 되돌리고 원본은 그대로 둡니다. **원본 교체**는 먼저 현재 상태를 백업하므로 복원을 되돌릴 수 있습니다. 그 밖의 복원 위치는 폴더와 새 ZFS 데이터세트입니다.

백업이 종속 컨테이너를 중지할 때, 이들은 올바른 순서로 돌아옵니다. BombVault는 Compose의 `depends_on` 순서대로 재시작하고 각 컨테이너가 정상 상태를 보고할 때까지 기다린 후에 그에 의존하는 컨테이너를 시작합니다. 따라서 아직 가동되지 않은 데이터베이스나 게이트웨이보다 먼저 앞서 나가는 것은 없습니다. [복원](restore.md)과 [명령과 다른 컨테이너](backups.md#commands)를 참고하세요.

## 작동 방식 {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault는 Docker 소켓을 사용해 백업 전에 컨테이너를 중지하고 복원 후에 다시 생성합니다. VM의 경우 SSH(`qemu+ssh://`)를 통해 호스트에서 `virsh`를 실행해 도메인을 정상 종료하거나 라이브 스냅샷을 만듭니다. libvirt 경로를 바인드 마운트하는 일은 없으므로 호스트의 VM Manager에 방해가 되지 않습니다.

BombVault는 오케스트레이션 및 UI 계층이지 스토리지 엔진이 아닙니다. 실제 모든 데이터 이동은 restic을 거칩니다.

## 빠른 시작 {#quick-start}

여기가 처음이신가요? **[시작하기](getting-started.md)**로 이동하여 BombVault를 설치하고 첫 백업을 실행한 다음, **[오프사이트 및 복구](offsite-recovery.md)**를 설정하세요. **[실전 예시](guides.md)**는 자주 하는 작업을 처음부터 끝까지 안내하고, **[설정](settings.md)** 페이지는 모든 타일을 나열하며, **[기능](features.md)**은 BombVault가 하는 모든 일의 짧은 목록입니다.

각 섹션은 여러 대상에 동시에 복사할 수 있고, 페어링한 BombVault는 삭제를 거부하는 서버에 오프사이트 사본을 보관할 수 있습니다. **긴급 마법사**는 복구 키트로 잃어버린 서버를 되살리며, 그룹을 통하거나 직접 입력한 키를 사용합니다. 구성을 새 장비로 옮기려면 **설정 내보내기 / 가져오기** 카드를 사용하세요. [오프사이트 및 복구](offsite-recovery.md)와 [구성](configuration.md#portable-settings-export-and-import)을 참고하세요.

**[Android 앱](android.md)**은 그룹의 모든 서버를 휴대폰에 담고, 모든 서버의 활동 로그를 한 화면에 보여 줍니다.

문서는 26개 언어로, 화면은 42개 언어로 제공됩니다. 아직 번역이 없는 페이지는 영어로 표시됩니다.

## 이 문서에서 쓰는 용어 {#glossary}

몇몇 단어는 BombVault와 이 페이지들에서 정해진 뜻으로 쓰입니다.

| 용어 | 뜻 |
|---|---|
| **항목** | BombVault가 백업하는 것 하나. 컨테이너, VM, 폴더 세트, 플래시 드라이브, 아래 데이터세트를 포함한 ZFS 데이터세트, 또는 자체 설정입니다. 각각 전용 페이지가 있습니다. |
| **섹션** | 컨테이너처럼 같은 종류의 모든 항목. 섹션은 **설정, 일반**에서 켭니다. |
| **대상** | 백업이 가는 곳. 이 서버의 폴더, 공유, 직접 운영하는 서버, 클라우드 서비스입니다. [대상](offsite-recovery.md#destinations)을 참고하세요. |
| **로컬** | 섹션들의 폴더가 있는, 이 서버의 대상. |
| **전용 폴더** | 섹션이 백업을 가장 먼저 기록하는 저장소. 기본적으로 이 서버에 있으며 다른 대상에 둘 수도 있습니다. [섹션의 전용 폴더](offsite-recovery.md#own-folder)를 참고하세요. |
| **복사** | `restic copy`로 전용 폴더에서 다른 대상으로 복사한 백업. |
| **직접 저장소** | 항목이 로컬 사본을 거치지 않고 바로 백업하는 오프사이트 위치의 저장소. [직접 저장소](offsite-recovery.md#direct-repositories)를 참고하세요. |
| **로그인 정보** | S3 서비스나 rest-server에 저장해 둔 키 또는 비밀번호. 그중 하나가 기본값입니다. [로그인 정보](offsite-recovery.md#sign-ins)를 참고하세요. |
| **배치** | 항목의 백업이 가는 대상. 항목에서 정하거나 섹션의 기본값으로 정합니다. [항목별 배치](offsite-recovery.md#placement)를 참고하세요. |
| **복제본** | `zfs send`로 만든, 두 번째 ZFS 서버에 있는 ZFS 항목의 블록 단위 동일 사본. 백업 옆에 두는 것이며 백업을 대신하지 않습니다. [복제본](zfs-datasets.md#replica)을 참고하세요. |
| **역할** | 그룹의 한 인스턴스가 다른 인스턴스를 위해 하는 일: 수신기, 가져오기 측, ZFS 서버, 키 보관 측. [인스턴스와 역할](offsite-recovery.md#roles)을 참고하세요. |

## 크레딧 {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**는 [@Darkdragon14](https://github.com/Darkdragon14)(Apache-2.0)의 프로젝트로, Docker 컨테이너를 클릭 한 번으로 백업하고 자동으로 다시 설치한다는 BombVault의 출발 아이디어를 주었습니다. BombVault는 Go와 restic 위에서 따로 구현한 것이며 이 아이디어를 VM, 플래시 등으로 넓혔습니다.
- **[restic](https://restic.net/)**은 BombVault가 구동하는 빠르고 안전한 중복 제거 백업 엔진입니다.
- **[rclone](https://rclone.org/)**은 클라우드 백엔드를 제공합니다.
- 버튼의 아이콘 대부분은 **[Streamline](https://streamlinehq.com)**의 무료 Core Solid 세트([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [소스](https://github.com/webalys-hq/streamline-vectors))에서 가져왔습니다. 나머지는 Font Awesome Free, Material Design Icons, Simple Icons, Tabler Icons에서 가져왔거나 이 프로젝트를 위해 그렸습니다.

## 라이선스 {#license}

Copyright (C) 2026 Junker der Provinz. BombVault는 **GNU Affero General Public License v3.0**([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE))에 따른 자유 소프트웨어입니다. 실행하고, 연구하고, 공유하고, 수정할 수 있습니다. 배포하거나 수정한 버전을 네트워크 서비스로 운영하는 경우에는 같은 라이선스로 소스를 공개하고 기존 저작권 및 출처 표시를 유지해야 합니다.

이름과 브랜딩은 라이선스 대상이 아닙니다. AGPL은 소스 코드에만 적용됩니다. "BombVault"와 로고, 브랜딩은 별도로 보호되므로, 포크는 자체 이름과 브랜딩을 써야 하며 BombVault인 것처럼 내세울 수 없습니다.

## 링크 {#links}

- **소스 코드:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid 지원 스레드:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **이슈:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "호스트에 대한 root 수준의 제어"
    Docker 소켓을 통해 BombVault는 컨테이너를 중지, 제거, 재생성하고 appdata를 읽고 쓸 수 있으며, VM 백업을 위해 호스트에 SSH로 로그인해 `virsh`를 실행합니다. 웹 UI에 접근할 수 있는 사람은 사실상 호스트의 root 권한을 가집니다. BombVault는 신뢰할 수 있고 외부에 노출되지 않은 네트워크에서만 실행하고, **설정, 보안**에서 로그인 비밀번호를 설정하세요. 복구 키트, 키 보관, 플래시 ZIP, 플래시 백업이나 셀프 백업 내보내기에는 비밀번호가 필요합니다. 전체 보안 모델은 [구성](configuration.md#security-model)을 참고하세요.
