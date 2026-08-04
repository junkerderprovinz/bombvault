# 문제 해결

간단한 FAQ입니다. SSH를 통한 VM 백업의 전체 호스트 측 문제 해결 표(권한 거부, 호스트 키 검증, 누락된 템플릿 변수 등)는 GitHub의 [SSH를 통한 VM 백업 가이드](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)를 참고하세요.

## 무언가가 올바르게 연결되지 않았습니다

웹 UI에서 `/spike`를 엽니다. 호스트 통합 확인이 모든 마운트와 CLI(Docker 소켓, libvirt, restic, qemu-img, rclone)를 검사하고 누락된 부분을 보고합니다. 버그라고 가정하기 전에 여기서 시작하세요: 누락된 마운트나 도달할 수 없는 호스트는 즉시 나타납니다.

## 웹 UI에 접근할 수 없습니다

BombVault는 기본으로 포트 `3443`에서 HTTPS를 제공하므로(자체 서명된 인증서), `https://<your-unraid-ip>:3443`을 여세요. 자체 서명 인증서 경고를 수락하거나, BombVault를 자체 인증서를 가진 리버스 프록시 뒤에 두세요. `HTTP_ONLY=true`로 실행하면 대신 포트 `3000`에서 일반 HTTP를 제공합니다(TLS 종료 프록시 뒤에서 사용하도록 의도됨).

## APP_KEY를 잃어버렸습니다

`APP_KEY`는 restic 저장소 비밀번호를 파생합니다. 이것 없이는(그리고 암호화 키 복구 키트 없이는) 암호화된 백업을 복구할 수 없습니다. 그래서 대시보드가 복구 키트를 다운로드하라고 재촉하는 것입니다. [오프사이트 및 복구](offsite-recovery.md)를 참고하세요. `openssl rand -hex 32`로 키를 생성하고 어떤 백업에 의존하기 전에 서버 밖에 보관하세요.

## VM 백업이 연결되지 않습니다

VM 백업은 마운트가 아니라 SSH를 통해 libvirt와 통신합니다.

- 호스트에서 SSH가 활성화되어 있고 BombVault의 공개 키가 `/root/.ssh/authorized_keys`에 승인되어 있는지 확인하세요(설정, 시스템, SSH를 통한 VM 백업에 키와 **연결 테스트** 버튼이 표시됨).
- 사용자 지정 `br0.x` 네트워크에서는 `LIBVIRT_HOST`를 Unraid LAN IP로 설정하세요(거기서는 컨테이너가 `host.docker.internal`을 통해 호스트에 도달할 수 없음). **설정, Docker, 사용자 지정 네트워크에 대한 호스트 접근**을 활성화하세요.
- Unraid의 SSH 포트를 변경했다면 `LIBVIRT_SSH_PORT`를 일치하도록 설정하세요.
- 전체 단계별 진단(도달 가능성 테스트, VLAN 라우팅, `Permission denied (publickey)`, `Host key verification failed`)은 [SSH를 통한 VM 백업 가이드](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)에 있습니다.

## 라이브 VM 스냅샷이 실행되지 않았습니다

라이브 스냅샷은 VM에 설치된 qemu 게스트 에이전트와 `/mnt/user`가 아닌 `/mnt/cache`(또는 `/mnt/diskX`)에 있는 디스크가 필요합니다. 꺼진 VM에서는 라이브가 자동으로 정상 종료로 대체됩니다. 정상 종료 백업은 VM을 종료하고, 디스크를 백업한 다음, 재시작하므로 항상 일관됩니다.

## 백업이 "repository is already locked"로 실패했습니다

이는 보통 컨테이너가 작업 도중 업데이트되거나 재시작될 때 남겨진 고아 restic 잠금입니다. BombVault는 확실히 고아가 된 잠금을 감지하여 자동으로 강제 해제하고 한 번 재시도합니다. 지속되면 해당 도메인에 대해 **설정, 무결성 및 유지 관리, 잠금 해제**를 사용하여 멈춘 잠금을 손으로 제거하세요. 진짜 문제는 숨겨지지 않고 여전히 드러납니다.

## 백업 후 오프사이트 복사가 일어나지 않았습니다

오프사이트 복제는 설계상 최선 노력 방식이므로, 오프사이트 장애가 로컬 백업을 실패시키는 일은 없습니다. 해당 도메인의 오프사이트 일정(설정, 일정)을 확인하세요: 빈 일정은 매 로컬 백업 후 복제하고, 주기는 덜 자주 보냅니다. 온디맨드 실행에는 오프사이트 탭의 **지금 복제**를 사용하고, 대시보드의 복제 표시기를 지켜보세요.

## 복원이 시작되기 전에 중단되었습니다

어떤 것이 중지되거나 제거되기 전에, 복원은 사전 점검 충돌 검사를 실행합니다: 컨테이너의 고정 IP와 게시된 호스트 포트가 비어 있는지 확인합니다. 다른 컨테이너가 이미 하나를 점유하고 있으면, 중간에 끝난 복원을 남기는 대신 명확하고 실행 가능한 메시지와 함께 중단합니다. 충돌하는 포트나 IP를 비운 다음 다시 시도하세요.

## 일반 내보내기가 파일을 쓰는 대신 실패했습니다

age 암호화가 켜져 있지만(설정) 유효한 수신자가 설정되지 않으면, 내보내기는 평문을 쓰는 대신 명확한 오류와 함께 실패합니다. 유효한 수신자(age 공개 키 또는 SSH 공개 키)를 추가하거나, 내보내기를 평문으로 의도한다면 암호화를 끄세요. [기능](features.md)을 참고하세요.

## 컨테이너가 계속 재시작하거나 비정상으로 보입니다

BombVault는 자체 `/api/health`에서 정상/비정상을 보고합니다. 자가 치유 도구(예: Autoheal)가 엔진이 멈추면 자동으로 재시작할 수 있습니다. 근본 원인은 컨테이너 로그와 `/spike` 보고서를 확인하세요.

## 여전히 막혔나요?

- 전체 [구성](configuration.md)과 [오프사이트 및 복구](offsite-recovery.md) 페이지를 읽으세요.
- [Unraid 지원 스레드](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)에서 질문하세요.
- [GitHub 이슈](https://github.com/junkerderprovinz/bombvault/issues)를 여세요.
