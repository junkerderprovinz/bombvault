# VM 백업

BombVault는 libvirt 경로를 하나도 마운트하지 않고 KVM/libvirt VM을 백업합니다. 호스트에서 SSH로 `virsh`를 실행하고(`qemu+ssh://`), VM 디스크는 Host Data 마운트로 읽고 쓰며(호스트의 `/mnt`는 컨테이너 안에서 `/host/user`입니다), UEFI NVRAM은 SSH로 복사합니다. libvirt에 속한 것을 바인드 마운트하지 않으므로 BombVault가 Unraid VM Manager를 방해할 일이 없습니다.

이 가이드는 설정, 백업 방식, Unraid 스냅샷이 있는 VM, TrueNAS SCALE을 다룹니다. 컨테이너 백업에는 이 중 아무것도 필요하지 않습니다.

ZFS 데이터셋 백업도 같은 키, 호스트, 사용자로 같은 SSH 연결을 씁니다. VM 백업이 연결되면 ZFS 데이터셋 항목도 연결됩니다. [ZFS 데이터셋](zfs-datasets.md)을 참고하세요.

## 설정할 항목 {#what-you-configure}

| 설정 | 위치 | 기본값 | 의미 |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | 템플릿 변수 | `host.docker.internal` | SSH로 접속하는 Unraid 호스트 |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | 템플릿 변수 | `22` | Unraid의 SSH 포트 |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | 템플릿 변수 | `root` | 호스트의 SSH 사용자 |
| 공개 키 | **설정, 연동, 호스트 SSH** | 첫 시작 때 생성 | 호스트에서 승인하세요 |

BombVault는 첫 시작 때 SSH 키 쌍을 `/config/ssh/id_ed25519`에 만들며, appdata에 보관됩니다. 호스트의 키는 첫 연결 때 `/config/ssh/known_hosts`에 고정됩니다.

템플릿은 **Host SSH: Address**를 자리 표시자 `192.168.x.x`로 채웁니다. BombVault는 이를 설정되지 않은 것으로 보고 `host.docker.internal`을 시도하며, 이는 기본 `bridge` 네트워크에서 동작합니다. 사용자 지정 네트워크에서는 호스트의 LAN IP로 바꾸세요(3단계 B).

## 1단계: Unraid 호스트에서 SSH 켜기 {#step-1}

1. Unraid에서 **Settings, Management Access**를 열고 **Use SSH**를 **Yes**로 설정합니다.
2. SSH 포트를 바꿨다면 적어 두세요. **Host SSH: Port**가 같아야 합니다.

## 2단계: BombVault의 공개 키 승인하기 {#step-2}

Unraid의 `/root`는 메모리에 있고 부팅할 때마다 지워지므로, 키를 플래시 드라이브에도 두어야 합니다. Unraid 터미널에서 다음을 실행하세요.

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 이상은 부팅할 때마다 `/boot/config/ssh/root.pubkeys`를 root의 `authorized_keys`로 복사하므로 재부팅 뒤에도 유지됩니다. BombVault가 어떤 호스트 이름이나 포트를 쓰든 모든 `root` 로그인에 적용됩니다.

## 3단계: 네트워크 {#step-3}

BombVault는 컨테이너에서 호스트의 SSH 포트로 TCP 연결을 열 수 있어야 합니다. 내 환경에 맞는 경우를 고르세요.

### A. 기본 bridge 네트워크의 BombVault {#bridge}

- **Host SSH: Address**를 `host.docker.internal`이나 템플릿 자리 표시자 그대로 두세요. 템플릿이 `--add-host=host.docker.internal:host-gateway`를 추가하며, 이것은 docker0 게이트웨이, 곧 호스트를 가리킵니다.
- **Host SSH: Port**를 내 SSH 포트로 설정하세요.

### B. 사용자 지정 네트워크의 BombVault {#custom-network}

`br0.x`, macvlan, ipvlan의 컨테이너는 `host.docker.internal`로 호스트에 닿을 수 없습니다. 172.17.0.1은 docker0이고 `br0.x`에서는 닿지 않기 때문입니다. 대신 이렇게 하세요.

1. Unraid에서 **Settings, Docker, Host access to custom networks**를 **Enabled**로 설정합니다.
2. **Host SSH: Address**를 Unraid 호스트의 LAN IP, 곧 웹 UI를 여는 주소로 설정합니다.
3. 컨테이너의 네트워크와 호스트가 서로 다른 VLAN에 있다면, 라우터나 방화벽에서 경로를 허용하세요. 컨테이너 VLAN에서 호스트 IP의 SSH 포트로, TCP로 허용합니다.
4. 그래도 호스트의 LAN IP에 닿지 않으면, 컨테이너 서브넷에 있는 호스트의 shim IP를 쓰세요. 호스트에서 `ip -4 addr show | grep -B2 <container-subnet>`으로 찾습니다.

### 호스트가 응답하는지 확인하기 {#verify-reachability}

Unraid 터미널에서 내 IP와 포트로 다음을 실행하세요.

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN`은 경로가 동작한다는 뜻입니다. `UNREACHABLE`은 다른 무엇보다 먼저 3단계의 네트워크를 고쳐야 한다는 뜻입니다.

## 4단계: 변수 설정과 테스트 {#step-4}

1. Unraid에서 **Docker**를 열고 BombVault를 눌러 **Edit**을 선택한 다음, **Host SSH: Address**(22가 아니면 **Host SSH: Port**도)를 설정하고 **Apply**를 누릅니다. 변수가 없다면 템플릿을 다시 가져오세요. Unraid는 기존 컨테이너의 저장된 구성을 그대로 유지하기 때문입니다.
2. BombVault에서 **설정, 연동, 호스트 SSH**를 열고 **연결 테스트**를 누릅니다. 초록색이 되어야 합니다.

터미널에서 같은 호출로 VM 목록을 볼 수 있습니다.

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## 5단계: VM별 방식 {#per-vm-method}

각 VM은 설정 탭의 **VM** 카드에 **방식**이 있습니다(**백업**에서 VM을 엽니다).

- **정상 종료 (셧다운)**가 기본입니다. VM을 종료하고, 디스크를 백업하고, 다시 시작하므로 백업이 항상 일관됩니다. 게스트가 종료될 때까지 최대 10분을 기다리고 강제로 끄지는 않습니다. 그 뒤에도 계속 실행 중인 게스트는 그대로 두고, 그 백업은 이유와 함께 실패합니다.
- **라이브 스냅샷**은 외부 스냅샷과 `blockcommit`으로 실행 중인 VM을 백업합니다. 정지된(quiesced) 스냅샷을 위해 VM에 qemu 게스트 에이전트가 필요하고, 디스크는 `/mnt/user`가 아닌 `/mnt/cache`나 `/mnt/diskX`에 있어야 합니다. 꺼진 VM에서는 라이브가 저절로 정상 종료로 바뀌고, 스냅샷을 만들 수 없을 때도 마찬가지로 바뀝니다.

## 변경된 블록만 {#changed-blocks}

qcow2 디스크가 있는 VM은 바뀐 부분만 읽어서 백업할 수 있습니다. 같은 카드에서 **변경된 블록만**을 켜면, BombVault는 위의 방식 대신 libvirt의 백업 API를 씁니다.

- 풀 모드 백업 작업(`virsh backup-begin`)을 시작하고, 같은 순간에 `bombvault-<time>`이라는 체크포인트를 만들며, 전달된 Unix 소켓을 통해 SSH 연결로 디스크를 읽습니다. 게스트 에이전트가 응답하면 작업이 시작되는 동안에만 게스트의 파일 시스템을 동결합니다. 해제에 실패하면 약 15초 동안 다시 시도하며, 그래도 게스트가 풀리지 않으면 BombVault가 알림을 보냅니다.
- 첫 실행은 할당된 모든 블록을 읽습니다. 이후 실행은 체크포인트의 비트맵이 지난 백업 이후 쓰였다고 표시한 블록만 읽습니다.
- 각 디스크는 게스트에게 보이는 내용을 담은 고정 크기 조각들의 폴더로 저장됩니다. 바뀌지 않은 조각은 읽지 않고 이전 스냅샷에서 가져오므로, 모든 스냅샷은 완전하며 단독으로 복원됩니다.
- 복원은 조각들로 디스크를 다시 만들고 `qemu-img`로 qcow2로 되돌립니다. 디스크의 내용은 백업 시점과 같지만 파일은 새 qcow2 파일이며, 백업 체인은 하나로 합쳐져 돌아옵니다.
- 이런 복원 전의 검사는 다시 만들 각 디스크를 나열하고 게스트의 전체 크기로 계산하며, qcow2는 두 번 셉니다. 원시 이미지와 변환된 사본이 옛 파일이 교체될 때까지 나란히 있기 때문입니다. 디스크 하나가 차지할 수 있는 최대치이므로, 이 계산에서 모자라도 복원을 막지 않고 공간 줄을 회색으로 둡니다. 첫 백업 뒤의 복원 확인과 **복원 확인**은 스냅샷이 모든 디스크의 모든 조각을 있어야 할 크기로 담고 있는지도 확인합니다.
- 실행과 실행 사이에 VM에는 libvirt에 기록된 체크포인트가 없습니다. BombVault는 가장 새 `bombvault-` 체크포인트의 정의를 `/config/vm-checkpoints`에 보관하고, libvirt의 기록만 제거합니다(`virsh checkpoint-delete --metadata`). 비트맵은 qcow2 파일에 남아 계속 쓰기를 셈하고, 다음 실행이 시작 전에 체크포인트를 되돌려 놓습니다(`virsh checkpoint-create --redefine`). libvirt는 체크포인트가 있는 VM의 정의 해제를 거부하는데, 이렇게 하면 Unraid의 **Remove VM**과 복원이 평소처럼 동작합니다.
- BombVault가 백업 도중 멈추어서 실행 중으로 남은 백업 작업은, BombVault가 다시 시작할 때 그 실행이 기록해 둔 체크포인트와 함께 종료됩니다.
- 스위치를 끄거나, VM을 일정에서 빼거나, 백업을 삭제해도 그런 작업이 종료되며, VM이 실행 중이면 비트맵도 삭제됩니다. 꺼진 VM에서는 libvirt가 비트맵을 삭제할 수 없어 이미지 안에 쓰이지 않은 채 남습니다.
- 다른 도구의 체크포인트는 건드리지 않습니다. 그런 것이 있는 VM은 삭제하기 전에는 복원할 수 없으며, 복원은 VM을 건드리기 전에 멈춥니다.

스위치는 저절로 대체 동작을 합니다. 체크포인트가 없거나 망가졌거나, 디스크 크기가 바뀌었거나, 가장 새 스냅샷이 변경된 블록 방식이 아니면 디스크 전체를 한 번 더 읽습니다. VM이 꺼져 있거나, 원시(raw) 또는 zvol 디스크이거나, 호스트 SSH 연결이 없거나, libvirt에 백업 API가 없거나, `/config`의 여유 공간이 너무 적으면 위 방식의 일반 백업을 합니다. 마지막 실행이 이 중 무엇을 썼고 왜 그랬는지는 카드에 나옵니다.

요구 사항은 다음과 같습니다. 백업 API가 있는 libvirt(Unraid 7.3의 libvirt 12.2와 QEMU 10.2로 테스트), `AllowStreamLocalForwarding yes`인 sshd(기본값이며, TCP 포워딩을 끄는 Unraid에서도 마찬가지입니다), qcow2 디스크입니다. libvirt는 원시 디스크의 체크포인트를 거부하며, Unraid는 기본으로 원시 vdisk를 만듭니다. 라이브와 마찬가지로 백업 중에도 VM은 계속 실행되며, restic을 기다리는 조각들은 `/config` 아래에서 최대 4 GiB를 차지합니다.

## Unraid 스냅샷이 있는 VM {#unraid-snapshots}

Unraid VM Manager가 찍은 스냅샷은 qcow2 디스크를 체인으로 바꿉니다. 맨 아래에 원본 이미지가 있고, 그 위에 스냅샷마다 파일이 하나씩 쌓이며, 각 파일은 아래 파일 이후에 바뀐 것만 담습니다. VM은 가장 새 파일에 씁니다. 스냅샷이 두 개인 디스크는 파일이 세 개입니다.

BombVault는 이런 VM을 통째로 백업합니다.

- **체인의 모든 파일.** 각 디스크의 백업 체인을 끝까지 따라가며 그 안의 모든 파일을 Unraid의 스냅샷 목록(`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, SSH로 읽음)과 함께 백업합니다. 중간의 파일 하나가 없는 경우처럼 끝까지 따라갈 수 없는 체인은 백업을 실패시키고 파일 이름을 알려 줍니다.
- **라이브 백업은 스냅샷을 유지합니다.** **라이브 스냅샷**은 디스크를 읽는 동안 임시 오버레이를 위에 얹고, 끝난 뒤 바로 아래의 이미지에만 병합합니다. 체인은 백업 전후가 같습니다. 스냅샷이 없는 VM은 예전과 똑같이 동작합니다.
- **복원은 체인을 되살립니다.** 제자리 복원은 모든 파일을 있던 자리에 다시 쓰고 Unraid의 스냅샷 목록도 되돌려 놓으므로, VM Manager가 스냅샷을 다시 나열합니다. 원본 옆 복원은 파일을 사본 폴더에 쓰고 복원된 디스크가 복원된 파일을 가리키게 하므로, 사본이 원본의 이미지를 읽는 일이 없습니다. Unraid의 스냅샷 목록은 백업 당시의 위치로 파일을 가리키므로 제자리 복원에서만 돌아옵니다. 목록이 없어도 사본은 똑같이 부팅되고, VM Manager에 스냅샷이 나열되지 않을 뿐입니다.
- **변경된 블록만**은 게스트에게 보이는 내용을 읽으므로, 복원하면 스냅샷 없는 하나로 합쳐진 qcow2 파일이 돌아옵니다.

### 예시: 스냅샷이 두 개인 VM {#example-unraid-snapshots}

VM `homeassistant`에는 디스크 `vdisk1.qcow2`가 하나 있고, 두 번의 업데이트 전에 VM Manager에서 찍은 스냅샷이 두 개 있습니다. 이제 디스크는 파일 세 개의 체인입니다. 항상 실행되므로 **라이브 스냅샷**을 씁니다.

1. **백업**을 열고 `homeassistant`, 그 **설정** 탭을 엽니다. **VM** 카드에서 **방식**이 **라이브 스냅샷**이고 **변경된 블록만**이 꺼져 있는지 확인합니다.
2. 페이지 머리글의 **지금 백업**을 누릅니다.
3. 실행이 끝나면 **검증** 탭의 **최근 실행**에 그 줄이 초록색으로 나옵니다. VM은 내내 실행 중이었고, 백업에는 디스크의 파일 세 개와 Unraid의 스냅샷 목록이 모두 들어 있습니다.
4. VM Manager에는 두 스냅샷이 그대로 있고, 디스크도 여전히 같은 파일 세 개를 거쳐 읽힙니다.

한 달 뒤 업데이트가 잘못되어, 망가진 것과 비교하려고 어제의 상태를 그 옆에 되살리고 싶다고 해 봅시다.

1. **복원**을 열고 **VM**을 고른 다음 `homeassistant`를 누릅니다.
2. 어젯밤의 권장 백업을 그대로 두고 **어디로**에서 **원본 옆에**를 고릅니다.
3. **복원**을 누릅니다. 파일 세 개가 VM 이름에 `-restored`가 붙은 폴더에 놓이고, 복원된 디스크는 복원된 파일을 가리킵니다. 실행 중인 VM에는 아무 변화가 없습니다.

대신 VM 전체를 되돌리려면 **먼저 현재 상태 백업**을 켠 채 **원본 교체**를 고르세요. BombVault가 망가진 상태를 백업하고, VM을 종료하고, 파일 세 개와 Unraid의 스냅샷 목록을 다시 쓴 다음, VM을 시작합니다.

## TrueNAS SCALE {#truenas-scale}

위 단계는 Unraid 기준으로 쓰였지만, 같은 `qemu+ssh://` 연결은 컨테이너가 닿을 수 있는 어떤 libvirtd에도 동작하며, 25.04.2부터 VM이 다시 libvirt 위에서 돌아가는 TrueNAS SCALE도 포함됩니다. TrueNAS에서는 연결 문자열을 덮어써야 하고, 키는 TrueNAS 자체 사용자 설정에서 root에 넣습니다. 백업에 담기는 내용을 바꾸는 것이 세 가지 더 있습니다. zvol 위의 디스크, vTPM 상태, TrueNAS가 VM에 붙이는 이름입니다.

### 1. libvirt 소켓 {#truenas-socket}

TrueNAS의 libvirtd는 **Host SSH** 입력란으로 만든 연결 문자열이 기대하는 곳에서 수신하지 않습니다. 대신 추가 `?socket=` 매개변수를 넣어 `LIBVIRT_URI`를 설정하세요.

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault는 `LIBVIRT_URI`를 쓴 그대로 사용하며, `LIBVIRT_HOST`, `LIBVIRT_SSH_USER`, `LIBVIRT_SSH_PORT`는 필요 없습니다. 설정되지 않은 각 변수에 대해, BombVault는 자신의 SSH 명령에도 URI의 호스트, 사용자, 포트를 쓰므로, NVRAM 복사와 ZFS 데이터셋 백업이 `virsh`와 같은 기기에 닿습니다. 직접 설정한 변수가 항상 우선하며, 부팅 로그에 URI에서 가져온 대상이 나옵니다. TrueNAS에서 root가 아닌 사용자는 ZFS 데이터셋 백업을 위해 `zfs allow <user> snapshot,destroy,mount <dataset>`도 필요합니다. [ZFS 데이터셋](zfs-datasets.md#truenas)을 참고하세요.

URI에 TrueNAS의 LAN 주소를 넣으세요. `LIBVIRT_URI`도 `LIBVIRT_HOST`도 설정하지 않으면 TrueNAS 앱에서는 `host.docker.internal`이 해석되지 않으므로 BombVault가 앱의 기본 게이트웨이를 호스트로 삼고, 부팅 로그에 고른 주소가 나옵니다. ZFS 데이터셋 백업에는 그것으로 충분합니다. VM 백업에는 소켓 때문에 여전히 URI가 필요합니다.

앱에는 VM 백업을 위해 `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, `SETUID`와 함께 `SETPCAP` 기능이 필요합니다. 없으면 libvirt가 "cannot apply process capabilities"라며 멈춥니다. [TrueNAS SCALE](getting-started.md#truenas)의 YAML은 이 모두와, 중지하는 대신 스냅샷을 찍는 데 필요한 `SYS_CHROOT`를 함께 설정합니다.

TrueNAS SCALE 25.10.0에서 실제 하드웨어로 소켓 경로를 확인했습니다. 기본 `qemu:///system`은 `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`로 실패하고, 같은 호출에 `?socket=/run/truenas_libvirt/libvirt-sock`을 붙이면 도메인이 나열됩니다.

### 2. TrueNAS에서 BombVault의 키 {#truenas-key}

위의 2단계는 Unraid의 플래시 드라이브에 쓰므로 여기에는 해당하지 않습니다. TrueNAS에서는 이렇게 합니다.

1. **System, Services**에서 SSH 서비스를 켭니다.
2. **Credentials, Users**에서 **root**를 편집하고, **설정, 연동, 호스트 SSH**의 BombVault 공개 키를 SSH 공개 키로 붙여 넣습니다. TrueNAS는 재부팅과 업데이트 뒤에도 이를 유지합니다.

TrueNAS 24.10 이상을 새로 설치하면 root에 비밀번호가 없고, 키 로그인에는 비밀번호가 필요 없습니다. root를 쓰고 싶지 않다면 sudo가 있는 관리자 계정에 키를 승인하고, `LIBVIRT_URI`의 `root` 자리에 그 이름을 넣으세요. 그러면 ZFS 데이터셋 백업에는 위에서 설명한 대로 `zfs allow`가 필요합니다.

두 번째 서버로의 ZFS 복제는 복제 창에 표시되는 자체 키로 로그인합니다. 호스트 SSH 키는 복제를 승인하지 않고, 복제 키는 VM 백업을 승인하지 않습니다.

!!! warning "TrueNAS는 virsh로 한 변경을 알지 못합니다"
    TrueNAS의 미들웨어, 곧 VM 화면 뒤의 계층은 VM을 바꾸는 유일한 경로가 되도록 만들어졌습니다. 이 SSH 연결로 보내는 `virsh` 명령, 곧 BombVault의 VM 백업과 복원이 실행하는 것은 이를 우회하며, TrueNAS가 나중에 자신이 만들지 않은 상태를 조정하거나 덮어쓸 수 있습니다. 이는 TrueNAS의 문서화된 동작에서 나온 것이며 실제 기기에서 확인된 적은 없습니다. Unraid의 VM Manager가 밖에서 한 변경을 조정하는 것과 같은 종류의 위험이며, TrueNAS에서 더 날카로울 뿐입니다.

### 3. zvol 위의 디스크 {#truenas-zvols}

TrueNAS VM 디스크는 qcow2 파일이 아니라 `/dev/zvol/<pool>/<dataset>`의 ZFS 블록 장치인 zvol인 경우가 많습니다. restic은 파일을 백업하듯 경로로 블록 장치를 백업할 수 없으므로, BombVault는 도메인 XML에서 zvol 디스크(`<source file="...">` 대신 `<source dev="...">`)를 알아보고 같은 SSH 연결로 처리합니다. BombVault 컨테이너에는 ZFS 도구가 필요 없습니다.

zvol 백업은 다음과 같이 진행됩니다.

1. `zfs snapshot <dataset>@bombvault-vm-<time>`이 VM이 실행 중일 때도 한 시점을 고정합니다.
2. `zfs clone -o volmode=dev -o readonly=on`이 그 스냅샷을 장치로 읽을 수 있게 만듭니다. `volmode=dev`는 호스트가 게스트의 파티션을 검색하지 못하게 합니다.
3. `dd`가 장치를 읽고, 원시 이미지가 `vm:<name>:zvol:<dev>` 태그가 붙은 파일 하나, `/vm-disks/<dataset>.img`로 restic에 들어갑니다. restic이 내용에 따라 덩어리로 자르므로, 지난 백업 이후 바뀌지 않은 블록은 다시 저장되지 않습니다.
4. 백업이 성공했든 아니든 클론과 스냅샷은 그 뒤에 삭제됩니다. BombVault가 백업 도중 멈췄다면, 그 zvol의 다음 백업이 먼저 `bombvault-vm-<time>` 스냅샷과 그 `<dataset>-bombvault-vm-<time>` 클론을 제거합니다. 다른 스냅샷은 건드리지 않으며, 다른 무언가가 복제해 간 것은 남겨 둡니다. 로그에 제거하거나 남긴 것이 나옵니다.

BombVault는 이를 위해 `zfs send`를 쓰지 않습니다. 전송 스트림의 모든 레코드에는 스냅샷의 GUID와 누적 체크섬이 들어 있어, restic이 지난 백업에서 재사용할 것을 하나도 찾지 못하고 매번 디스크 전체를 다시 저장하게 됩니다. 데이터 200 MiB가 든 512 MiB zvol로 TrueNAS SCALE 25.10에서 측정했습니다. 첫 백업은 211 MB를 추가했고, 아무것도 바뀌지 않은 두 번째는 52 KB, 1 MiB가 바뀐 뒤의 백업은 3.6 MB를 추가했습니다. 같은 zvol을 `zfs send`로 하면 매 실행마다 214 MB가 추가되었습니다.

각 실행은 모든 디스크가 들어간 뒤에 도메인 XML이 담긴 작은 스냅샷도 `vm:<name>` 태그로 씁니다. 디스크가 모두 zvol인 VM에는 파일 스냅샷이 없으며, 이 스냅샷이 실행을 VM 아래에 나열하고 복원할 수 있게 해 줍니다.

복원은 파일 디스크를 그 파일에 다시 쓰듯 각 zvol을 제자리에 다시 쓰므로, 복원된 VM이 복원된 데이터로 부팅합니다.

1. 무엇이든 건드리기 전에, BombVault는 실행에 모든 zvol 디스크의 이미지가 있는지 확인합니다. 하나라도 빠진 실행은 거부되고 VM은 그대로 남습니다.
2. 이미 있는 zvol은 먼저 `<dataset>@bombvault-before-restore-<time>` 스냅샷을 받습니다. 시각은 BombVault가 만드는 모든 스냅샷 이름처럼 UTC이므로 현지 시계와 몇 시간 차이가 날 수 있습니다. 로그에 이름이 나오며, 그것이 zvol의 가장 새 스냅샷인 동안에는 `zfs rollback`으로 복원을 되돌릴 수 있습니다. 다음 복원의 것처럼 더 새로운 스냅샷이 생기면 `zfs rollback -r`로 되돌릴 수 있지만 더 새로운 스냅샷은 삭제됩니다. 복원된 VM이 잘 동작하는 것을 확인한 뒤 `zfs destroy`로 지울 때까지 남아 있습니다.
3. 더 이상 없는 zvol은 백업 당시의 크기로 만들어지고, 크기가 다른 기존 zvol은 그 크기로 되돌립니다.
4. `dd`가 이미지를 `/dev/zvol/<dataset>`에 씁니다. TrueNAS 기본값인 압축이 켜져 있으면 이미지의 빈 부분은 구멍으로 돌아와 공간을 차지하지 않습니다.

**다른 인스턴스로의 복원에는 대상 풀이 필요합니다.** 파일 디스크는 대상 폴더로 옮기지만 폴더는 ZFS 풀을 가리키지 않으므로, 풀이 없으면 BombVault는 시작하기 전에 복원을 거부합니다. 풀은 `POST /api/foreign/restore`의 `zvolPool` 매개변수로 전달하세요. 각 zvol은 풀 아래의 경로를 유지하며(`tank/vms/win10/disk0`을 `flashpool`로 보내면 `flashpool/vms/win10/disk0`이 됩니다), 도메인 XML이 그것을 가리킵니다. 복원 페이지의 **이전 백업 폴더에서** 창에는 이를 위한 입력란이 없으므로, 직접 API를 호출해야만 쓸 수 있습니다. 같은 인스턴스의 복원은 각 zvol을 자기 자신에게 다시 씁니다.

**파일로 내보내기는 zvol을 빼놓습니다.** 디스크가 모두 zvol인 VM은 그 이유와 함께 거부되므로, 대신 백업에서 복원하세요. 파일 디스크도 있는 VM의 내보내기에는 파일 디스크와, 빼놓은 zvol의 이름을 적은 메모가 들어갑니다.

TrueNAS SCALE 25.10과 ZFS 2.3.4에서 시험했습니다. 임시 zvol의 백업 세 번, 덮어쓰고 키운 뒤의 제자리 복원, 삭제된 zvol로의 복원입니다. 각 복원은 원본과 같은 SHA-256으로 읽혔고, zvol은 옛 크기로 돌아왔으며, 스냅샷이나 클론이 남지 않았습니다. 유일한 디스크가 zvol인 VM 전체를 실행 중에 정상 종료와 라이브로, 그리고 꺼진 상태로 백업했고, VM 전체 복원(정의 해제, zvol 쓰기, 정의, 부팅)도 거기서 실행했습니다. zvol은 원본의 체크섬으로 읽혔고 VM이 시작되었습니다.

Unraid가 쓰는 파일 디스크에는 이 중 아무것도 영향을 주지 않습니다.

### 4. vTPM 상태 {#truenas-vtpm}

VM에는 Secure Boot와 Windows 11 게스트에 필요한 가상 TPM이 있을 수 있습니다. 그 상태는 디스크와 NVRAM 밖에 있으므로, 이 둘만 백업하면 게스트가 새 빈 TPM과 함께 돌아옵니다.

- **패스스루 TPM**, 곧 게스트에 넘긴 실제 칩은 도메인 XML에 경로가 나옵니다. BombVault는 그 상태를 SSH로 NVRAM 옆에 저장하고 `virsh define` 전에 다시 쓰며, NVRAM처럼 최선을 다하는 방식입니다. 읽기나 쓰기가 실패하면 기록만 하고 백업이나 복원을 실패시키지 않습니다.
- **에뮬레이션된 vTPM**, 곧 TrueNAS가 Windows 11과 Secure Boot 게스트를 위해 설정하는 종류는 도메인 XML에 상태 경로를 내놓지 않습니다. 그러면 BombVault는 경로를 추측하는 대신 "no TPM path found"라고 알립니다. 그런 게스트를 복원하기 전에 BitLocker 키 같은 복구 키를 가까이에 두세요.
- **TrueNAS VM에는 TPM을 직접 붙일 수 없습니다.** TrueNAS SCALE 25.10의 VM 장치 API는 `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK`, `USB`만 받고 `TPM`은 알 수 없는 종류로 거부합니다.

TPM 처리는 단위 테스트만 거쳤고 vTPM이 있는 실제 TrueNAS 게스트에서는 실행된 적이 없습니다. Unraid처럼 TPM이 없는 VM은 예전과 똑같이 백업되고 복원됩니다.

NVRAM은 평소처럼 가져옵니다. TrueNAS는 이를 VM마다 `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`에 보관하며, 도메인 XML에 그 경로가 나옵니다.

### 5. TrueNAS의 VM 이름 {#truenas-names}

Unraid에서 VM의 libvirt 이름은 내가 정한 이름입니다. TrueNAS는 libvirt 이름을 만들어 내며, 그 방식을 두 번 바꿨습니다.

| TrueNAS 버전 | libvirt 이름 | 내 이름이 있는 곳 |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, 예를 들어 `1_debian` | libvirt 이름의 `{id}_` 뒤 |
| 26 | VM의 UUID, 예를 들어 `550e8400-e29b-41d4-a716-446655440000` | 도메인 XML의 `<title>`에만 |

BombVault가 TrueNAS로 감지했거나 `PLATFORM=truenas`로 지정한 호스트에서는 백업 목록에 내 이름이 나옵니다. 25.10에서는 `1_debian` 대신 `debian`, 26에서는 UUID 대신 `<title>`(제목이 없으면 UUID)입니다. 이것은 목록에 보이는 것일 뿐입니다. 모든 `virsh` 호출과 VM별 설정은 여전히 libvirt 이름을 쓰므로, 백업과 복원은 항상 맞는 VM에 작용합니다. TrueNAS 26에서는 TrueNAS가 새 UUID로 다시 만든 VM이 이름이 같아 보여도 설정 없는 새 항목으로 나타난다는 뜻입니다. Unraid에서는 `10_Windows`라고 이름 붙인 VM을 포함해 아무것도 달라지지 않습니다.

25.10 방식은 실제 TrueNAS SCALE 25.10 기기에서 확인했습니다. TrueNAS 26 방식은 TrueNAS 문서에서 가져왔으며 단위 테스트만 거쳤습니다.

## 문제 해결 {#troubleshooting}

| 증상 | 원인과 해결 |
|---|---|
| **연결 테스트**가 멈추었다가 실패함 | 호스트에 닿을 수 없습니다. 3단계(네트워크, VLAN, 방화벽)와 **Host SSH: Address**, **Port**를 확인하세요. |
| 키가 `authorized_keys`에 있는데도 `Permission denied (publickey)` | 상위 폴더의 소유자나 모드가 잘못되어 sshd의 `StrictModes`가 키 파일을 거부합니다. `/root/.ssh`가 모두가 쓸 수 있는 FAT 플래시 드라이브(`/boot/config/ssh/...`)로의 링크일 때 흔합니다. 호스트 로그에 `Authentication refused: bad ownership or modes for directory ...`가 나옵니다. 호스트 sshd 설정의 전역 섹션에서 `Match` 블록 앞에 `StrictModes no`를 추가한 다음 `/etc/rc.d/rc.sshd restart`를 실행하세요. Unraid는 부팅할 때마다 `/etc/ssh/sshd_config`를 새로 쓰므로 아래 설명대로 유지되게 하세요. |
| `Permission denied (publickey)`이고 키가 정말 없음 | 2단계를 반복하고 SSH가 켜져 있는지 확인하세요. |
| `Host key verification failed` | `docker exec BombVault rm -f /config/ssh/known_hosts`를 실행하고 다시 테스트하세요. |
| 3단계의 확인에서 `UNREACHABLE`이 나옴 | 컨테이너가 호스트의 SSH 포트에 닿지 못합니다. 3단계를 보세요. 사용자 지정 네트워크의 호스트 접근, VLAN 라우팅, shim 또는 LAN IP입니다. |
| **Edit**에서 변수가 보이지 않음 | 템플릿을 다시 가져오세요. 기존 컨테이너는 옛 저장 구성을 유지합니다. |
| Unraid 스냅샷이 있는 VM이 실패하고 파일 이름을 알려 줌 | 디스크의 백업 체인을 끝까지 따라갈 수 없습니다. 그 디스크의 모든 스냅샷이 VM Manager에 아직 있는지, 알려 준 파일이 `/mnt` 아래에 있는지 확인하세요. |

VM에 관한 다른 증상은 [문제 해결](troubleshooting.md#vm-backup-will-not-connect)에 있습니다.

## 재부팅 뒤에도 유지하기 {#persistence}

Unraid는 부팅할 때마다 `/etc/ssh/sshd_config`를 새로 쓰므로, 호스트 쪽 변경은 부팅할 때 다시 적용해야 합니다.

- **SSH 포트**는 Unraid 웹 UI에서 설정하며 `/boot/config/ident.cfg`에 보관됩니다.
- 위에서 필요했다면 **`StrictModes no`**는 유지되지 않습니다. `/boot/config/go`에서 부팅 때 적용하세요.

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- BombVault 자체의 SSH 키와 `~/.ssh/config`는 `/config`(appdata)에 있고 시작할 때마다 다시 만들어지므로, 컨테이너와 호스트를 재시작해도 유지됩니다.

## 보안 {#security}

승인된 키는 호스트의 root 권한을 주며, 이는 BombVault가 이미 쓰는 Docker 소켓과 같은 수준의 신뢰입니다. BombVault는 신뢰할 수 있는 네트워크에 두세요. 키는 ed25519이고 appdata에 모드 `0600`으로 저장되며, 호스트의 키는 고정됩니다. 모든 호스트 명령은 원격 셸을 거치지 않고 별도의 인수로 전달되며, SSH 연결은 `BatchMode`와 연결 제한 시간을 씁니다.
