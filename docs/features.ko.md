# 기능

BombVault가 하는 일을 짧게 모은 목록입니다. 각 줄은 그것을 설명하는 페이지로 연결됩니다.

## 백업 범위 {#backup-scope}

- **컨테이너**: appdata 폴더와 컨테이너 정의이며, 컨테이너마다 폴더를 고를 수 있습니다. [백업과 항목](backups.md#what-is-saved)
- **VM**: SSH를 통한 디스크 이미지, XML, UEFI NVRAM이며, 정상 종료, 라이브 스냅샷, 변경된 블록만 복사하는 방식이 있습니다. Unraid 스냅샷이 있는 VM과 zvol 위의 디스크도 전부 다룹니다. [VM 백업](vm-backup-ssh-setup.md)
- **플래시**: Unraid USB 플래시 전체이며, 예약된 zip 내보내기가 있습니다. [백업과 항목](backups.md#settings-tab)
- **셀프 백업**: BombVault 자체의 설정과 로그인 정보. [긴급 마법사](offsite-recovery.md#emergency)
- **폴더**: 서버의 어떤 폴더든 폴더 세트로. [폴더 추가](backups.md#add)
- **ZFS 데이터세트**: 스냅샷 하나에서 읽는 데이터세트 트리이며, 개별 파일 복원과 두 번째 ZFS 서버에 대한 선택적 복제본이 있습니다. [ZFS 데이터세트](zfs-datasets.md)
- **데이터베이스 덤프**: PostgreSQL, MySQL, MariaDB를 실행 중인 서버에서 덤프해 전용 복원 지점으로 만듭니다. [데이터베이스 덤프](backups.md#database-dumps)

## 백업과 항목 페이지 {#backups}

- **하나의 목록**: 타일, **필터**, **정렬**, **검색**이 있고, 각 줄에는 최근 실행 열네 번이 띠로 표시됩니다. [목록](backups.md#list)
- **파일 관리자처럼 선택**하고, 선택한 항목을 서버에서 백업하거나 일시 중지합니다. [여러 항목 선택](backups.md#select)
- **항목마다 페이지**가 있으며 **백업**, **설정**, **검증**이 있습니다. [항목의 페이지](backups.md#entry-page)
- 스냅샷의 **태그와 비교**, 그리고 tar, zip, restic 저장소로의 **내보내기**. [백업 탭](backups.md#backups-tab)
- 실시간 미리 보기가 있는 **제외 패턴**과 잘 알려진 앱을 위한 권장 제외 설정. [제외 패턴](backups.md#excludes)
- **전후 명령**, 그리고 의존성 순서로 상태를 확인하며 재시작하는 **다른 컨테이너 중지**. [명령과 다른 컨테이너](backups.md#commands)
- **앱이 사용 중일 때**: 예약된 백업이 Plex, Jellyfin, Emby, 직접 만든 확인, 또는 조용해진 트래픽을 기다렸다가 저절로 다시 시도합니다. [앱이 사용 중일 때](backups.md#app-busy)
- **중지 대신 스냅샷**: ZFS의 컨테이너는 몇 초만 멈춥니다. [일정](schedules.md#snapshot-instead-of-stop)
- age로 봉인할 수도 있는 **일반 내보내기**. [일반 내보내기](backups.md#exports)

## 복원 {#restore}

- 모든 종류를 위한 **페이지 하나와 창 하나**이며, 권장 백업이 미리 선택됩니다. [복원](restore.md)
- 기본은 **원본 옆에**이고, **원본 교체**는 먼저 현재 상태를 백업합니다. [복원 창](restore.md#window)
- 폴더로, 또는 제자리로 되돌리는 **개별 파일**. [파일 하나 되찾기](restore.md#example-single-file)
- **여러 항목 한꺼번에**, 각각 권장 백업에서 또는 한 시점에서. [여러 항목 한꺼번에 복원](restore.md#several)
- 의존성 순서로 시작하는 **Compose 스택 전체**. [스택 복원](restore.md#stacks)
- **컨테이너와 VM을 다시 만들며**, 실행하던 이미지로, GPU, 제한, 링크도 함께. [무엇이 돌아오는가](restore.md#kinds)
- **복원 전 확인**과 무엇이 바뀌는지의 목록. [복원 창](restore.md#window)
- **ZIP으로 플래시**, 그리고 실행 중인 플래시에 플러그인을 한 번에 하나씩 되돌리기. [무엇이 돌아오는가](restore.md#kinds)
- 안전 스냅샷과 함께 데이터세트 안으로, 폴더로, 또는 속성을 유지한 채 새 데이터세트로 **ZFS** 복원. [ZFS 데이터세트](zfs-datasets.md#restore)
- 복구 키트, 그룹을 통해, 또는 직접 입력한 키로 하는 잃어버린 서버를 위한 **긴급 마법사**. [긴급 마법사](offsite-recovery.md#emergency)
- **이전 백업 폴더에서**, 그리고 **Appdata.Backup 플러그인에서 가져오기**. [다른 출처](restore.md#other-sources)

## 저장소와 일정 {#storage-scheduling}

- 모든 종류의 **대상**: 로컬 폴더, SMB, WebDAV, SFTP, rest-server, S3, 클라우드 드라이브, 모든 rclone 리모트. [대상](offsite-recovery.md#destinations)
- **자격 증명**: 기본값이 있는 하나의 목록. [자격 증명](offsite-recovery.md#sign-ins)
- 이 서버나 대상 안의 **전용 폴더**. [섹션 전용 폴더](offsite-recovery.md#own-folder)
- 모든 줄이 스위치인 **흐름으로서의 3-2-1**과 **항목별 배치**. [항목별 배치](offsite-recovery.md#placement)
- 섹션마다 여러 대상으로의 **오프사이트 복사**이며, 대역폭 제한, 압축, 콜드 스토리지 클래스, Plex 스트리밍 중 속도 제한이 있습니다. [오프사이트 복사](offsite-recovery.md#off-site-copies)
- **여기로 바로 백업**에 나열되는 **직접 저장소**. [직접 저장소](offsite-recovery.md#direct-repositories)
- **타임라인으로서의 보존**: 대상마다 **짧게**, **균형**, **길게**, **사용자 지정 값**이며 섹션별 규칙이 있습니다. [보존 기간](offsite-recovery.md#keep)
- **섹션의 기록 옮기기**, 그리고 백업을 함께 지우거나 남기는 **대상 제거**. [이동과 제거](offsite-recovery.md#move-remove)
- **문장처럼 읽히는 일정**, **주** 띠, 전체 백업, 섹션별 계획, 항목별 전용 계획. [일정](schedules.md)
- **놓친 백업 따라잡기**, 수동 순서, 백업 성공 후 업데이트. [일정](schedules.md)

## 통찰, 검증, 모니터링 {#insight-verification-monitoring}

- **개요**: 상태 머리글, **조치 필요**, **보호 상태**, 저장 공간 예측, 기록, 선택 카드. [개요](overview.md)
- 소스가 무너졌을 때 오래된 백업도 지켜 주는 항목별 **이상 징후 감지**. [이상 징후](anomalies.md)
- 첫 백업 뒤의 **복원 검사**와 격리된 사본에서의 **시작 테스트**. [검증 탭](backups.md#verify-tab)
- 오프사이트 복사본으로 하는 **예약된 검사와 테스트 복원**. [DR 훈련](offsite-recovery.md#dr-drills)
- **폴더별 크기**와 백업이 느렸던 이유. [실행이 알려 주는 것](backups.md#run-notes)
- **알림**: webhook, Gotify, Matrix, 이메일, Apprise, Unraid, Healthchecks이며 주간 요약과 지연 보고가 있습니다. [알림](notifications.md)
- **Prometheus**, Home Assistant 기기, HTTP API, mDNS. [API와 연동](api.md)
- **버그 신고**: GitHub 이슈용으로 민감한 정보를 가린 ZIP. [문제 해결](troubleshooting.md#still-stuck)

## 랜섬웨어 방어 {#ransomware-protection}

- 상대편이 강제하고 변조 테스트로 증명하는 **삭제 방지 (append-only)**. [삭제 방지](offsite-recovery.md#delete-protection-append-only)
- 삭제를 거부할 수 없는 곳을 위한 **삭제 방지 없이 수락**. [삭제 방지](offsite-recovery.md#delete-protection-append-only)
- **로그인 비밀번호를 설정했다면 백업의 영구 삭제에는 본인 로그인이 필요합니다.** 페어링한 휴대폰으로는 삭제할 수 없습니다. [보안 모델](configuration.md#security-model)
- age로 봉인할 수 있는 **복구 키트**와 그룹 안의 **키 보관 측**. [복구 키트](offsite-recovery.md#recovery-kit), [키 보관 측](offsite-recovery.md#key-keeper)

## 여러 서버 {#several-servers}

- 직접 또는 릴레이를 통한, 종단 간 암호화된 **열두 단어 페어링**. [인스턴스 페어링](offsite-recovery.md#pairing)
- 그룹이 그물로 보이고 **수신기**, **가져오기 측**, **ZFS 서버**, **키 보관 측** 역할이 있는 **인스턴스 페이지**. [인스턴스와 역할](offsite-recovery.md#roles)
- append-only 모드로 대신 설정해 주는 **수신 서버**. [수신 서버](offsite-recovery.md#receiving-server)

## AI 어시스턴트 (MCP) {#mcp}

내장 MCP 서버를 통해 어시스턴트는 백업 상태, 적용 범위, 실행 기록, 복원 지점을 읽을 수 있으며, 허용하는 키가 있으면 백업을 시작할 수 있습니다. 복원, 삭제, 정리, 설정은 웹 인터페이스에서만 합니다. [MCP 서버](mcp.md)

## 앱과 동반 도구 {#apps}

- **Android 앱**: 그룹의 모든 서버를 휴대폰에서. [Android 앱](android.md)
- **BombVault Widget**: Unraid 대시보드의 타일. [앱](settings.md#apps)
- Homepage나 Heimdall 같은 대시보드를 위한 **삽입 가능한 활동 로그**. [대시보드 위젯](api.md#dashboard-widget)

## 기타 {#other}

- **42개 언어**, 세 가지 테마, 원하는 대로 고르는 레이블, 색상, 모서리, 애니메이션. [모양](settings.md#look)
- **저절로 저장되는 설정**과 열네 개의 모든 타일을 대상으로 하는 검색. [설정](settings.md)
- 업데이트 뒤의 **새 소식**이며, 바뀐 것에 초록색 점이 붙습니다. [정보](settings.md#info)
- **처음부터 HTTPS**, 그리고 자동 복구 도구를 위한 상태 확인. [구성](configuration.md#network)
