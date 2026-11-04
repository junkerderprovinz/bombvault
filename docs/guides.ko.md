# 실전 예제

여기 있는 예제는 모두 실제 작업 하나를 처음부터 끝까지 따라가며, 이름과 숫자까지 채워 두었습니다. 각 예제는 해당 기능을 설명하는 페이지 안에 있어서 자세한 내용을 바로 옆에서 볼 수 있습니다.

## 시작하기 {#start}

- [컨테이너 첫 백업, 처음부터 끝까지](getting-started.md#example-first-backup): 설치부터 복구 키트까지.

## 저장소와 오프사이트 {#storage}

- [Backblaze B2에 오프사이트 사본 만들기](offsite-recovery.md#example-s3): 버킷, 그 버킷 전용 키, 그리고 첫 사본. Cloudflare R2도 같은 방식으로 합니다.
- [직접 운영하는 rest-server](offsite-recovery.md#example-rest-server): 삭제를 거부하는 오프사이트 사본. `docker run` 명령줄과 변조 시험도 다룹니다.
- [균형이 남기는 것](offsite-recovery.md#example-retention): 400일 동안 매일 밤 백업했을 때의 보존 결과를 백업 하나씩 따라갑니다.
- [컨테이너를 NAS로 옮길 때](offsite-recovery.md#example-own-folder): 섹션 전용 폴더의 위치가 바뀌었을 때, 이미 만든 백업은 어떻게 되는지.

## 백업과 일정 {#backups}

- [서로 맞물리는 계획 세 개](schedules.md#example): 밤마다 전체 백업, 일주일에 한 번 VM, 그리고 항목 하나를 6시간마다 백업하는 별도 계획.
- [누군가 보는 동안 Plex는 기다립니다](backups.md#example-plex): 영화 재생을 끊지 않는 예약 백업.
- [Unraid 스냅샷이 두 개인 VM](vm-backup-ssh-setup.md#example-unraid-snapshots): 스냅샷을 그대로 두는 라이브 백업, 그리고 원본 옆으로 하는 복원.
- [두 번째 Unraid 서버로 복제본 보내기](zfs-datasets.md#example-replica): 언제든 읽을 수 있는 ZFS 사본, 그리고 거기서 파일 하나를 꺼내는 세 가지 방법.

## 되찾기 {#restore}

- [파일 하나 되찾기](restore.md#example-single-file): 어젯밤 컨테이너 백업에서 파일 하나.
- [ZFS 백업에서 파일 하나](zfs-datasets.md#example-zfs-file): 데이터세트에서 같은 일을 하는 방법.
- [복구 키트로 새 서버 세우기](offsite-recovery.md#example-emergency): 고장난 서버를 새 장비에서 되살립니다. 템플릿에 예전 `APP_KEY`가 없어도 됩니다.

## 알림 받기 {#staying-informed}

- [휴대폰의 Gotify](notifications.md#example-gotify): 실패는 소리로 알리고 성공은 조용히 도착합니다.
- [Webhook으로 쓰는 ntfy](notifications.md#example-ntfy): 계정 없이 받는 메시지.

## 서버가 여러 대일 때 {#several-servers}

- [Unraid 서버 두 대 페어링](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): 서로의 사본을 보관하지만 상대의 백업은 지울 수 없는 서버 두 대.
- [Home Assistant의 카드와 알림](api.md#home-assistant-example): 대시보드에 BombVault를 올리고, 백업이 실패하면 메시지를 받습니다.
