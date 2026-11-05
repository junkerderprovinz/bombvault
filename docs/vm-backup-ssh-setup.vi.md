# Sao lưu VM

BombVault sao lưu các KVM/libvirt VM mà không gắn kết bất kỳ đường dẫn libvirt nào. Nó chạy `virsh` trên máy chủ qua SSH (`qemu+ssh://`), đọc và ghi đĩa VM qua điểm gắn kết Host Data (`/mnt` trên máy chủ là `/host/user` trong container), và sao chép UEFI NVRAM qua SSH. Vì không có thứ gì thuộc về libvirt được bind-mount, BombVault không bao giờ cản trở VM Manager của Unraid.

Hướng dẫn này nói về thiết lập, các phương thức sao lưu, VM có snapshot Unraid và TrueNAS SCALE. Sao lưu container không cần bất cứ điều gì ở đây.

Sao lưu tập dữ liệu ZFS dùng cùng liên kết SSH này, với cùng khóa, máy chủ và người dùng. Khi sao lưu VM kết nối được, các mục tập dữ liệu ZFS cũng kết nối được. Xem [Tập dữ liệu ZFS](zfs-datasets.md).

## Những gì bạn cấu hình {#what-you-configure}

| Thiết lập | Ở đâu | Mặc định | Ý nghĩa |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | biến của mẫu | `host.docker.internal` | Máy chủ Unraid, được tới qua SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | biến của mẫu | `22` | Cổng SSH của Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | biến của mẫu | `root` | Người dùng SSH trên máy chủ |
| Khóa công khai | **Cài đặt, Tích hợp, SSH máy chủ** | tạo ở lần khởi động đầu | Cấp quyền cho nó trên máy chủ |

BombVault tạo cặp khóa SSH của nó ở lần khởi động đầu tiên tại `/config/ssh/id_ed25519`, lưu trong appdata. Khóa của máy chủ được ghim vào `/config/ssh/known_hosts` ở lần kết nối đầu tiên.

Mẫu điền **Host SSH: Address** bằng chỗ giữ chỗ `192.168.x.x`. BombVault coi đó là chưa đặt và thử `host.docker.internal`, thứ hoạt động trên mạng `bridge` mặc định. Trên một mạng tùy chỉnh, hãy thay nó bằng IP LAN của máy chủ (bước 3 B).

## Bước 1: Bật SSH trên máy chủ Unraid {#step-1}

1. Trong Unraid, mở **Settings, Management Access** và đặt **Use SSH** thành **Yes**.
2. Nếu bạn đã đổi cổng SSH, hãy ghi lại. **Host SSH: Port** phải khớp.

## Bước 2: Cấp quyền cho khóa công khai của BombVault {#step-2}

`/root` của Unraid nằm trong bộ nhớ và bị xóa mỗi lần khởi động, nên khóa cũng phải nằm trên ổ flash. Chạy lệnh này trong terminal của Unraid:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 trở lên chép `/boot/config/ssh/root.pubkeys` vào `authorized_keys` của root ở mỗi lần khởi động, nên cách này sống sót qua các lần khởi động lại. Nó đúng cho mọi lần đăng nhập `root`, bất kể BombVault dùng tên máy chủ hay cổng nào.

## Bước 3: Mạng {#step-3}

BombVault phải mở được một kết nối TCP từ container tới cổng SSH của máy chủ. Hãy chọn trường hợp khớp với thiết lập của bạn.

### A. BombVault trên mạng bridge mặc định {#bridge}

- Để **Host SSH: Address** là `host.docker.internal` hoặc chỗ giữ chỗ của mẫu. Mẫu thêm `--add-host=host.docker.internal:host-gateway`, trỏ tới gateway docker0, tức là máy chủ.
- Đặt **Host SSH: Port** thành cổng SSH của bạn.

### B. BombVault trên một mạng tùy chỉnh {#custom-network}

Một container trên `br0.x`, macvlan hoặc ipvlan không tới được máy chủ qua `host.docker.internal`, vì 172.17.0.1 là docker0 và không tới được từ `br0.x`. Thay vào đó:

1. Trong Unraid, đặt **Settings, Docker, Host access to custom networks** thành **Enabled**.
2. Đặt **Host SSH: Address** thành IP LAN của máy chủ Unraid, địa chỉ bạn dùng để mở giao diện web.
3. Nếu mạng của container và máy chủ nằm trên các VLAN khác nhau, hãy cho phép tuyến đường trên router hoặc tường lửa của bạn: từ VLAN của container tới IP của máy chủ ở cổng SSH, qua TCP.
4. Nếu vẫn không tới được IP LAN của máy chủ, hãy dùng IP shim của máy chủ trên subnet của container. Tìm nó trên máy chủ bằng `ip -4 addr show | grep -B2 <container-subnet>`.

### Kiểm tra máy chủ có trả lời không {#verify-reachability}

Chạy lệnh này trong terminal của Unraid, với IP và cổng của bạn:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` nghĩa là đường đi hoạt động. `UNREACHABLE` nghĩa là phần mạng ở bước 3 cần được sửa trước mọi việc khác.

## Bước 4: Đặt các biến và thử {#step-4}

1. Trong Unraid, mở **Docker**, bấm BombVault, chọn **Edit**, đặt **Host SSH: Address** (và **Host SSH: Port** nếu không phải 22) rồi bấm **Apply**. Nếu các biến bị thiếu, hãy nhập lại mẫu, vì Unraid giữ cấu hình đã lưu của một container đang có.
2. Trong BombVault, mở **Cài đặt, Tích hợp, SSH máy chủ** và bấm **Kiểm tra kết nối**. Nó phải chuyển sang màu xanh.

Cùng lệnh gọi đó từ terminal liệt kê các VM của bạn:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Bước 5: Phương thức cho từng VM {#per-vm-method}

Mỗi VM có một **Phương thức** trong thẻ **VM** ở tab Cài đặt của nó (mở VM dưới **Bản sao lưu**):

- **Êm ái (tắt máy)** là mặc định. Nó tắt VM, sao lưu các đĩa và khởi động lại, nên bản sao lưu luôn nhất quán. Nó chờ tối đa mười phút để hệ điều hành khách tắt và không bao giờ ép tắt. Một máy khách vẫn còn chạy sau đó sẽ được để yên, và bản sao lưu đó lỗi kèm lý do. Nếu máy khách tắt trong vòng một giờ sau đó, BombVault sẽ khởi động lại nó. Nếu lần khởi động đó thất bại, lần chạy lỗi sẽ nói rõ điều đó.
- **Snapshot trực tiếp** sao lưu một VM đang chạy qua một snapshot ngoài và `blockcommit`. Nó cần đĩa nằm trên `/mnt/cache` hoặc `/mnt/diskX`, không phải `/mnt/user`. Có qemu guest agent trong VM thì snapshot là snapshot tĩnh, còn không có thì snapshot chỉ nhất quán như sau một lần sập máy (crash-consistent). Với một VM đã tắt, Trực tiếp tự chuyển về Êm ái. Khi không tạo được snapshot, bản sao lưu đó thất bại và VM vẫn tiếp tục chạy, vì một VM bạn đã chọn sao lưu trực tiếp không bao giờ bị tắt vì việc này.

## Những gì một bản sao lưu bỏ ra {#left-out}

Một bản sao lưu có thể thành công mà vẫn không chứa toàn bộ VM. Khi đó lần chạy mang một ghi chú ở mức cảnh báo, nêu tên những gì còn thiếu:

- **Các đĩa BombVault không đọc.** Một đĩa chỉ đọc, một `lun` và một đĩa `network` hoặc `volume` sẽ bị bỏ ra, và ghi chú nêu tên từng đĩa kèm thiết bị đích và loại của nó. Định nghĩa trong bản sao lưu vẫn liệt kê chúng. Ổ cdrom và ổ đĩa mềm không được nêu tên. Một thiết bị khối thông thường không phải zvol ZFS thì làm bản sao lưu thất bại.
- **Trạng thái của một TPM giả lập.** Nó không có đường dẫn trong XML của domain. Unraid VM Manager thiết lập một TPM như vậy cho VM dùng BIOS **OVMF TPM**, nên ghi chú cũng xuất hiện trên Unraid. Một máy khách giữ bí mật trong TPM của nó, như BitLocker có thể làm, có thể hỏi khóa khôi phục sau một lần khôi phục làm mất trạng thái đó, nên hãy giữ sẵn khóa ấy.

## Đĩa thin {#thin-disks}

Một lần khôi phục giữ một đĩa thin vẫn là thin, dù nó thay thế đĩa hay ghi một đĩa mới: restic ghi các đoạn số không thành lỗ trống trong một tệp mới, và BombVault trả lại các khối số không của một tệp được thay thế bằng `fallocate --dig-holes` sau khi tệp được ghi xong. Trong lúc restic thay thế một tệp, tệp đó chiếm đủ kích thước của nó, nếu pool còn chỗ. Một hệ thống tệp không hỗ trợ đục lỗ sẽ giữ đĩa ở kích thước đầy đủ, và nhật ký sẽ nói rõ điều đó. Bước kiểm tra dung lượng vẫn tính một đĩa mới theo kích thước đầy đủ.

## Một lần khôi phục thất bại {#restore-fails}

Một lần khôi phục tại chỗ tắt VM, ghi các đĩa, và chỉ thay định nghĩa cùng NVRAM sau khi các đĩa đã được ghi xong. Một lần khôi phục thất bại sẽ để VM vẫn được định nghĩa cùng NVRAM của nó. Nếu các đĩa của nó không đổi và nó đang chạy, nó sẽ chạy lại. Nếu chúng có thể đã thay đổi, nó vẫn tắt, vì khởi động từ một đĩa mới ghi một nửa sẽ ghi đè lên đĩa đó, và lần chạy sẽ nói rõ điều đó. Khôi phục lại cùng bản sao lưu đó sẽ hoàn tất công việc.

Các snapshot có hồ sơ trong libvirt (Unraid VM Manager không tạo snapshot theo cách này) sẽ chặn một lần khôi phục tại chỗ trước khi VM bị đụng tới, vì libvirt không thể xóa một VM có các snapshot đó. Hãy xóa chúng trước.

## Chỉ các khối đã đổi {#changed-blocks}

Một VM có đĩa qcow2 có thể được sao lưu bằng cách chỉ đọc những gì đã đổi. Bật **Chỉ các khối đã đổi** trong cùng thẻ, và BombVault dùng API sao lưu của libvirt thay vì phương thức ở trên:

- Nó bắt đầu một tác vụ sao lưu kiểu pull (`virsh backup-begin`) tạo một checkpoint tên `bombvault-<time>` cùng lúc đó, và đọc các đĩa qua liên kết SSH thông qua một Unix socket được chuyển tiếp. Khi guest agent trả lời, các hệ thống tệp của máy khách chỉ bị đóng băng trong lúc tác vụ bắt đầu. Một lần rã đông thất bại sẽ được thử lại trong khoảng mười lăm giây, và nếu máy khách vẫn không rã đông, BombVault gửi một thông báo.
- Lần chạy đầu đọc mọi khối đã cấp phát. Các lần sau chỉ đọc những khối mà bitmap của checkpoint đánh dấu là đã được ghi kể từ bản sao lưu trước.
- Mỗi đĩa được lưu thành một thư mục các đoạn có kích thước cố định, chứa những gì máy khách thấy. Các đoạn không đổi được lấy từ snapshot trước mà không cần đọc, nên mọi snapshot đều đầy đủ và tự khôi phục được.
- Việc khôi phục dựng lại đĩa từ các đoạn của nó và chuyển nó về qcow2 bằng `qemu-img`. Đĩa có cùng nội dung như lúc sao lưu, nhưng tệp là một tệp qcow2 mới, và chuỗi backing trở lại ở dạng đã làm phẳng.
- Phép kiểm tra trước một lần khôi phục như vậy liệt kê từng đĩa mà nó dựng lại và tính nó ở kích thước đầy đủ của máy khách, hai lần với qcow2, vì ảnh raw và bản chuyển đổi của nó nằm cạnh nhau cho đến khi tệp cũ được thay. Đó là mức lớn nhất một đĩa có thể chiếm, nên thiếu chỗ theo cách tính này chỉ làm dòng dung lượng chuyển xám thay vì chặn. Phép kiểm tra khôi phục sau bản sao lưu đầu tiên, và **Kiểm tra khôi phục**, cũng bảo đảm snapshot chứa mọi đoạn của mọi đĩa ở đúng kích thước cần có.
- Giữa các lần chạy, VM không có checkpoint nào được ghi trong libvirt. BombVault giữ định nghĩa của checkpoint `bombvault-` mới nhất dưới `/config/vm-checkpoints` và chỉ xóa bản ghi của libvirt về nó (`virsh checkpoint-delete --metadata`). Bitmap của nó vẫn nằm trong tệp qcow2 và tiếp tục đếm các lần ghi, và lần chạy sau đặt checkpoint trở lại (`virsh checkpoint-create --redefine`) trước khi bắt đầu. libvirt từ chối hủy định nghĩa một VM có checkpoint, nên đây là điều giúp **Remove VM** của Unraid và việc khôi phục hoạt động như bình thường.
- Một tác vụ sao lưu mà BombVault để lại đang chạy, vì nó dừng giữa chừng một bản sao lưu, sẽ được kết thúc khi BombVault khởi động lại, cùng với các checkpoint mà lần chạy đó để lại trong bản ghi.
- Tắt công tắc, đưa VM ra khỏi lịch hoặc xóa các bản sao lưu của nó cũng kết thúc một tác vụ như vậy, và xóa cả bitmap khi VM đang chạy. Với một VM đã tắt, libvirt không xóa được nó, và nó nằm lại trong ảnh mà không dùng đến.
- Các checkpoint của công cụ khác được để yên. Một VM có checkpoint như vậy không khôi phục được cho đến khi chúng bị xóa, và việc khôi phục dừng trước khi chạm vào VM.

Công tắc tự lùi về cách cũ. Một checkpoint thiếu hoặc hỏng, một đĩa đã đổi kích thước, hoặc snapshot mới nhất không phải loại khối đã đổi nghĩa là toàn bộ đĩa được đọc lại một lần. Một VM đã tắt, một đĩa raw hoặc zvol, không có liên kết SSH máy chủ, libvirt không có API sao lưu, hoặc quá ít chỗ trống dưới `/config` nghĩa là bản sao lưu thông thường với phương thức ở trên. Thẻ cho biết lần chạy gần nhất đã dùng cách nào và vì sao.

Yêu cầu: libvirt có API sao lưu (đã thử với libvirt 12.2 và QEMU 10.2 trên Unraid 7.3), sshd với `AllowStreamLocalForwarding yes` (mặc định, kể cả trên Unraid vốn tắt chuyển tiếp TCP), và đĩa qcow2. libvirt từ chối checkpoint trên đĩa raw, và Unraid tạo vdisk raw theo mặc định. VM tiếp tục chạy trong lúc sao lưu, như với Trực tiếp, và các đoạn đang chờ restic chiếm tối đa 4 GiB dưới `/config`.

## VM có snapshot Unraid {#unraid-snapshots}

Một snapshot do VM Manager của Unraid tạo ra biến một đĩa qcow2 thành một chuỗi: ảnh gốc ở dưới cùng, và trên đó là một tệp cho mỗi snapshot, mỗi tệp chỉ chứa những gì đã đổi kể từ tệp bên dưới. VM ghi vào tệp mới nhất. Một đĩa có hai snapshot là ba tệp.

BombVault sao lưu một VM như vậy đầy đủ:

- **Mọi tệp của chuỗi.** Nó lần theo chuỗi backing của từng đĩa đến cuối và sao lưu mọi tệp trong đó, cùng với danh sách snapshot của Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, đọc qua SSH). Một chuỗi không thể lần tới cuối, chẳng hạn vì một tệp ở giữa đã mất, sẽ làm bản sao lưu lỗi và nêu tên tệp.
- **Một bản sao lưu trực tiếp giữ nguyên các snapshot.** **Snapshot trực tiếp** đặt một lớp phủ tạm lên trên trong lúc đọc đĩa và gộp nó lại sau đó, chỉ vào ảnh ngay bên dưới. Chuỗi sau bản sao lưu giống như trước. Một VM không có snapshot hoạt động như vẫn luôn thế.
- **Một lần khôi phục đưa chuỗi trở lại.** Khôi phục tại chỗ ghi mọi tệp về chỗ cũ và đặt lại danh sách snapshot của Unraid, nên VM Manager liệt kê lại các snapshot. Khôi phục bên cạnh bản gốc ghi các tệp vào thư mục của bản sao và trỏ các đĩa được khôi phục tới các tệp được khôi phục, nên bản sao không bao giờ đọc từ ảnh của bản gốc. Danh sách snapshot của Unraid chỉ trở lại khi khôi phục tại chỗ, vì nó nêu tên các tệp ở nơi chúng được sao lưu. Không có nó, bản sao vẫn khởi động như thường, chỉ là VM Manager không liệt kê snapshot nào cho nó.
- **Chỉ các khối đã đổi** đọc những gì máy khách thấy, nên bản khôi phục của nó trở lại thành một tệp qcow2 đã làm phẳng, không có các snapshot.
- **Một lần khôi phục đưa lại đĩa của bản sao lưu bạn chọn.** Các snapshot được tạo hoặc gộp kể từ bản sao lưu đó không làm thay đổi điều này. Một bản sao lưu từ trước một snapshot sẽ định nghĩa VM trên tệp mà nó chứa, và một bản sao lưu từ trước một lần gộp sẽ đưa lớp phủ của nó trở lại và chạy trên đó. Các tệp mới hơn vẫn nằm nguyên chỗ, không được dùng và không bị thay đổi. Một bản sao lưu chứa lớp phủ nhưng không chứa tệp bên dưới nó sẽ bị từ chối trước khi VM bị đụng tới. Sau một lần khôi phục, định nghĩa của VM không liệt kê chuỗi nào, nên libvirt đọc chuỗi từ các tệp đã khôi phục.

### Ví dụ: một VM với hai snapshot {#example-unraid-snapshots}

VM `homeassistant` có một đĩa, `vdisk1.qcow2`, và hai snapshot bạn đã tạo trong VM Manager trước hai lần cập nhật. Đĩa của nó giờ là một chuỗi ba tệp. Nó chạy suốt, nên dùng **Snapshot trực tiếp**.

1. Mở **Bản sao lưu**, rồi `homeassistant`, rồi tab **Cài đặt** của nó. Trong thẻ **VM**, kiểm tra **Phương thức** là **Snapshot trực tiếp** và **Chỉ các khối đã đổi** đang tắt.
2. Bấm **Sao lưu ngay** ở phần đầu trang.
3. Khi lần chạy xong, dòng của nó trong **Lần chạy gần đây** ở tab **Xác minh** có màu xanh. VM vẫn chạy suốt thời gian đó, và bản sao lưu chứa cả ba tệp của đĩa cùng danh sách snapshot của Unraid.
4. Trong VM Manager, hai snapshot vẫn còn đó, và đĩa vẫn đọc qua cùng ba tệp.

Một tháng sau, một bản cập nhật hỏng và bạn muốn lấy lại trạng thái của hôm qua bên cạnh trạng thái hỏng, để so sánh:

1. Mở **Khôi phục**, chọn **VMs** và bấm `homeassistant`.
2. Giữ bản sao lưu được khuyên dùng từ đêm qua và chọn **Bên cạnh bản gốc** ở **Đến đâu**.
3. Bấm **Khôi phục**. Ba tệp nằm trong một thư mục đặt theo tên VM kèm `-restored`, và đĩa được khôi phục trỏ tới các tệp được khôi phục. VM đang chạy không bị thay đổi gì.

Để lùi cả VM thay vào đó, hãy chọn **Thay thế bản gốc** với **Sao lưu trạng thái hiện tại trước** đang bật. BombVault sao lưu trạng thái hỏng, tắt VM, ghi ba tệp và danh sách snapshot của Unraid trở lại, rồi khởi động lại nó.

## TrueNAS SCALE {#truenas-scale}

Các bước ở trên được viết cho Unraid, nhưng cùng liên kết `qemu+ssh://` hoạt động với bất kỳ libvirtd nào container tới được, kể cả TrueNAS SCALE, vốn lại có VM trên libvirt từ 25.04.2. Trên TrueNAS, chuỗi kết nối cần một giá trị ghi đè và khóa được đặt cho root trong phần cài đặt người dùng riêng của TrueNAS. Ba điều nữa thay đổi những gì một bản sao lưu chứa: đĩa trên zvol, trạng thái vTPM và tên mà TrueNAS đặt cho VM của nó.

### 1. Socket libvirt {#truenas-socket}

libvirtd của TrueNAS không lắng nghe ở nơi chuỗi kết nối dựng từ các trường **Host SSH** mong đợi. Hãy đặt `LIBVIRT_URI` thay vào đó, với tham số `?socket=` thêm vào:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault dùng `LIBVIRT_URI` đúng như được viết, và không cần `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` và `LIBVIRT_SSH_PORT`. Với mỗi biến chưa đặt, BombVault lấy máy chủ, người dùng và cổng từ URI cho cả các lệnh SSH riêng của nó, nên việc sao chép NVRAM và sao lưu tập dữ liệu ZFS tới cùng một máy với `virsh`. Biến do bạn tự đặt luôn thắng, và nhật ký khởi động nêu đích mà nó lấy từ URI. Trên TrueNAS, một người dùng không phải root cũng cần `zfs allow <user> snapshot,destroy,mount <dataset>` cho sao lưu tập dữ liệu ZFS, xem [Tập dữ liệu ZFS](zfs-datasets.md#truenas).

Hãy đặt địa chỉ LAN của TrueNAS vào URI. Khi cả `LIBVIRT_URI` lẫn `LIBVIRT_HOST` đều chưa đặt, BombVault lấy gateway mặc định của ứng dụng làm máy chủ, vì `host.docker.internal` không phân giải được trong một ứng dụng TrueNAS, và nhật ký khởi động nêu địa chỉ mà nó đã chọn. Như vậy là đủ cho sao lưu tập dữ liệu ZFS. Sao lưu VM vẫn cần URI cho socket.

Ứng dụng cần khả năng `SETPCAP` cho sao lưu VM, bên cạnh `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` và `SETUID`. Không có nó, libvirt dừng với "cannot apply process capabilities". Đoạn YAML dưới [TrueNAS SCALE](getting-started.md#truenas) đặt tất cả, cùng với `SYS_CHROOT` mà ảnh chụp thay cho dừng cần.

Trên TrueNAS SCALE 25.10.0, đường dẫn socket đã được kiểm tra trên phần cứng thật: `qemu:///system` mặc định lỗi với `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, và cùng lệnh gọi với `?socket=/run/truenas_libvirt/libvirt-sock` liệt kê được các domain.

### 2. Khóa của BombVault trên TrueNAS {#truenas-key}

Bước 2 ở trên ghi vào ổ flash của Unraid và không áp dụng ở đây. Trên TrueNAS:

1. Bật dịch vụ SSH trong **System, Services**.
2. Trong **Credentials, Users**, sửa **root** và dán khóa công khai của BombVault từ **Cài đặt, Tích hợp, SSH máy chủ** làm khóa công khai SSH của nó. TrueNAS giữ nó qua các lần khởi động lại và cập nhật.

Các bản cài mới của TrueNAS 24.10 trở lên không đặt mật khẩu cho root, và đăng nhập bằng khóa cũng không cần. Nếu bạn không muốn dùng root, hãy cấp quyền cho khóa trên một tài khoản quản trị có sudo và đặt tên của nó vào chỗ `root` trong `LIBVIRT_URI`. Khi đó sao lưu tập dữ liệu ZFS cần `zfs allow` như mô tả ở trên.

Một bản nhân bản ZFS tới máy chủ thứ hai đăng nhập bằng khóa riêng, mà hộp thoại bản nhân bản hiển thị. Khóa SSH máy chủ không cấp quyền cho bản nhân bản, và khóa bản nhân bản không cấp quyền cho sao lưu VM.

!!! warning "TrueNAS không thấy các thay đổi làm bằng virsh"
    Middleware của TrueNAS, lớp đứng sau các màn hình VM của nó, được định để là cách duy nhất thay đổi VM của nó. Một lệnh `virsh` qua liên kết SSH này, đúng là thứ mà sao lưu và khôi phục VM của BombVault chạy, đi vòng qua nó, và sau này TrueNAS có thể đối chiếu hoặc ghi đè trạng thái mà nó không tạo ra. Điều này đến từ hành vi mà TrueNAS ghi trong tài liệu và chưa từng thấy trên một máy thật. Đây là cùng loại rủi ro như việc VM Manager của Unraid đối chiếu các thay đổi làm bên ngoài nó, chỉ là mạnh hơn trên TrueNAS.

### 3. Đĩa trên zvol {#truenas-zvols}

Đĩa VM của TrueNAS thường là zvol, tức thiết bị khối ZFS tại `/dev/zvol/<pool>/<dataset>`, không phải tệp qcow2. restic không sao lưu được một thiết bị khối theo đường dẫn của nó như cách nó sao lưu một tệp, nên BombVault nhận ra một đĩa zvol trong XML của domain (`<source dev="...">` thay vì `<source file="...">`) và xử lý nó qua cùng liên kết SSH. Container BombVault không cần công cụ ZFS nào.

Một bản sao lưu zvol diễn ra như sau:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` cố định một thời điểm, kể cả khi VM đang chạy.
2. `zfs clone -o volmode=dev -o readonly=on` làm snapshot đó đọc được như một thiết bị. `volmode=dev` giữ cho máy chủ không quét các phân vùng của máy khách.
3. `dd` đọc thiết bị, và ảnh raw đi vào restic như một tệp, `/vm-disks/<dataset>.img`, gắn thẻ `vm:<name>:zvol:<dev>`. restic cắt nó thành các khối theo nội dung, nên các khối không đổi kể từ bản sao lưu trước không bị lưu lại.
4. Bản clone và snapshot bị hủy sau đó, dù bản sao lưu có thành công hay không. Nếu BombVault dừng giữa một bản sao lưu, bản sao lưu kế tiếp của zvol đó trước hết xóa snapshot `bombvault-vm-<time>` và bản clone `<dataset>-bombvault-vm-<time>` của nó. Nó để yên mọi snapshot khác, và giữ lại cái mà thứ gì đó khác đã được clone từ đó. Nhật ký nêu những gì đã xóa hoặc giữ.

BombVault không dùng `zfs send` cho việc này. Mỗi bản ghi của một luồng send mang GUID của snapshot và một checksum chạy liên tục, nên restic sẽ không tìm thấy gì của bản sao lưu trước để tái dùng và lưu lại cả đĩa mỗi lần. Đo trên TrueNAS SCALE 25.10 với một zvol 512 MiB chứa 200 MiB dữ liệu: bản sao lưu đầu thêm 211 MB, bản thứ hai không có gì đổi thêm 52 KB, và bản sau khi đổi 1 MiB thêm 3,6 MB. Cùng zvol đó qua `zfs send` thêm 214 MB ở mỗi lần chạy.

Mỗi lần chạy cũng ghi một snapshot nhỏ gắn thẻ `vm:<name>` với XML của domain, sau khi mọi đĩa đã vào. Một VM mà mọi đĩa đều là zvol không có snapshot tệp, và đây là snapshot liệt kê lần chạy dưới VM và làm nó khôi phục được.

Một lần khôi phục ghi từng zvol trở lại tại chỗ, như một đĩa tệp được ghi trở lại tệp của nó, nên VM được khôi phục khởi động từ dữ liệu được khôi phục:

1. Trước khi chạm vào bất cứ thứ gì, BombVault kiểm tra lần chạy có chứa ảnh của mọi đĩa zvol. Một lần chạy thiếu một ảnh sẽ bị từ chối và VM được để nguyên.
2. Một zvol đang có được chụp snapshot trước, `<dataset>@bombvault-before-restore-<time>`, với thời gian theo UTC như mọi tên snapshot BombVault tạo, nên có thể lệch vài giờ so với đồng hồ địa phương. Nhật ký nêu tên nó, và `zfs rollback` về nó sẽ hoàn tác việc khôi phục khi nó còn là snapshot mới nhất của zvol. Khi đã có snapshot mới hơn, chẳng hạn của lần khôi phục kế tiếp, `zfs rollback -r` làm được việc đó và hủy các snapshot mới hơn. Nó ở lại cho đến khi bạn xóa bằng `zfs destroy` sau khi VM được khôi phục chạy tốt.
3. Một zvol không còn tồn tại được tạo ở kích thước nó có khi được sao lưu, và một zvol đang có với kích thước khác được đặt lại về kích thước đó.
4. `dd` ghi ảnh vào `/dev/zvol/<dataset>`. Khi bật nén, mặc định của TrueNAS, các phần trống của ảnh trở lại thành khoảng trống và không chiếm chỗ.

**Một lần khôi phục sang phiên bản khác cần một pool đích.** Nó chuyển đĩa tệp vào một thư mục đích, nhưng một thư mục không nêu pool ZFS nào, nên nếu thiếu, BombVault từ chối việc khôi phục trước khi nó bắt đầu. Hãy truyền pool qua tham số `zvolPool` của `POST /api/foreign/restore`: mỗi zvol giữ đường dẫn của nó bên dưới pool (`tank/vms/win10/disk0` sang `flashpool` thành `flashpool/vms/win10/disk0`), và XML của domain trỏ tới đó. Cửa sổ **Từ thư mục sao lưu cũ** trên trang Khôi phục không có trường cho việc này, nên chỉ làm được bằng một lệnh gọi API trực tiếp. Một lần khôi phục trên cùng phiên bản ghi từng zvol trở lại chính nó.

**Xuất dưới dạng tệp bỏ qua zvol.** Một VM mà mọi đĩa đều là zvol bị từ chối với lý do đó, nên hãy khôi phục nó từ một bản sao lưu. Với VM cũng có đĩa tệp, bản xuất chứa các đĩa tệp và một ghi chú nêu tên các zvol đã bỏ ra.

Điều này đã được thử trên TrueNAS SCALE 25.10 với ZFS 2.3.4: ba bản sao lưu của một zvol thử nghiệm, một lần khôi phục tại chỗ sau khi zvol bị ghi đè và mở rộng, và một lần khôi phục lên zvol đã bị xóa. Mỗi lần khôi phục đọc lại với cùng SHA-256 như nguồn, zvol có lại kích thước cũ, và không còn snapshot hay clone nào bị bỏ lại. Một VM nguyên vẹn mà đĩa duy nhất là zvol đã được sao lưu khi đang chạy, với Êm ái và với Trực tiếp, và khi đã tắt, và một lần khôi phục cả VM (hủy định nghĩa, ghi các zvol, định nghĩa, khởi động) cũng chạy ở đó: zvol đọc lại với checksum của nguồn và VM khởi động được.

Đĩa tệp, như Unraid dùng, không bị ảnh hưởng bởi bất cứ điều nào trong số này.

### 4. Trạng thái vTPM {#truenas-vtpm}

Một VM có thể có TPM ảo, thứ mà Secure Boot và máy khách Windows 11 cần. Trạng thái của nó nằm ngoài đĩa và NVRAM, nên một bản sao lưu chỉ có hai thứ đó sẽ đưa máy khách trở lại với một TPM mới, trống.

- **Một TPM passthrough**, một chip thật được chuyển cho máy khách, nêu đường dẫn của nó trong XML của domain. BombVault lưu trạng thái của nó cạnh NVRAM qua SSH và ghi trở lại trước `virsh define`, theo kiểu hết sức có thể như NVRAM: một lần đọc hoặc ghi lỗi được ghi nhật ký và không làm hỏng việc sao lưu hay khôi phục.
- **Một vTPM giả lập**, loại TrueNAS thiết lập cho máy khách Windows 11 và Secure Boot, không công bố đường dẫn trạng thái trong XML của domain, nên trạng thái của nó không được sao lưu. BombVault không đoán đường dẫn, và bản sao lưu kết thúc với một ghi chú rằng trạng thái TPM của VM không có trong đó. Hãy giữ sẵn khóa khôi phục của máy khách đó, chẳng hạn khóa BitLocker, trước khi bạn khôi phục nó.
- **Bạn không thể tự gắn TPM vào một VM TrueNAS.** API thiết bị VM của TrueNAS SCALE 25.10 chỉ nhận `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` và `USB`, và từ chối `TPM` là loại không xác định.

Việc xử lý TPM chỉ được kiểm thử đơn vị và chưa chạy trên một máy khách TrueNAS thật có vTPM. Một VM không có TPM được sao lưu và khôi phục y như trước.

NVRAM được lấy như thường lệ. TrueNAS giữ nó theo từng VM tại `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, và XML của domain nêu đường dẫn đó.

### 5. Tên VM trên TrueNAS {#truenas-names}

Trên Unraid, tên libvirt của một VM là tên bạn đã chọn. TrueNAS tự đặt tên libvirt, và đã đổi cách đặt hai lần:

| Phiên bản TrueNAS | Tên libvirt | Tên của bạn nằm ở đâu |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, chẳng hạn `1_debian` | trong tên libvirt, sau `{id}_` |
| 26 | UUID của VM, chẳng hạn `550e8400-e29b-41d4-a716-446655440000` | chỉ trong `<title>` của XML domain |

Trên một máy chủ mà BombVault nhận ra là TrueNAS, hoặc `PLATFORM=truenas` nêu tên, danh sách Bản sao lưu hiện tên của bạn: `debian` thay cho `1_debian` trên 25.10, và `<title>` thay cho UUID trên 26, hoặc UUID khi không có tiêu đề. Đó chỉ là những gì danh sách hiện. Mọi lệnh `virsh` và cài đặt theo từng VM vẫn dùng tên libvirt, nên sao lưu và khôi phục luôn tác động đúng VM. Trên TrueNAS 26, điều đó có nghĩa là một VM mà TrueNAS tạo lại với UUID mới sẽ hiện như một mục mới không có cài đặt của nó, dù tên trông giống nhau. Trên Unraid không có gì đổi, kể cả với một VM bạn đặt tên `10_Windows`.

Cách đặt tên của 25.10 đã được thấy trên một máy TrueNAS SCALE 25.10 thật. Cách đặt tên của TrueNAS 26 đến từ tài liệu của TrueNAS và chỉ được kiểm thử đơn vị.

## Xử lý sự cố {#troubleshooting}

| Triệu chứng | Nguyên nhân và cách sửa |
|---|---|
| **Kiểm tra kết nối** bị treo, rồi lỗi | Không tới được máy chủ. Kiểm tra bước 3 (mạng, VLAN, tường lửa) và **Host SSH: Address** cùng **Port**. |
| `Permission denied (publickey)` dù khóa đã có trong `authorized_keys` | `StrictModes` của sshd từ chối tệp khóa vì một thư mục cha có chủ sở hữu hoặc chế độ sai. Điều này thường gặp khi `/root/.ssh` là một liên kết tới ổ flash FAT (`/boot/config/ssh/...`), vốn ghi được cho mọi người. Nhật ký máy chủ hiện `Authentication refused: bad ownership or modes for directory ...`. Thêm `StrictModes no` vào cấu hình sshd của máy chủ trong phần chung, trước mọi khối `Match`, rồi chạy `/etc/rc.d/rc.sshd restart`. Unraid ghi `/etc/ssh/sshd_config` mới ở mỗi lần khởi động, nên hãy giữ nó như mô tả bên dưới. |
| `Permission denied (publickey)` và khóa thật sự bị thiếu | Lặp lại bước 2, và kiểm tra SSH đã bật. |
| `Host key verification failed` | Chạy `docker exec BombVault rm -f /config/ssh/known_hosts` rồi thử lại. |
| Phép kiểm tra ở bước 3 báo `UNREACHABLE` | Container không tới được cổng SSH của máy chủ. Xem bước 3: truy cập máy chủ tới mạng tùy chỉnh, định tuyến VLAN, hoặc IP shim hay IP LAN. |
| Các biến bị thiếu dưới **Edit** | Nhập lại mẫu. Một container đang có giữ cấu hình đã lưu cũ của nó. |
| Một VM có snapshot Unraid bị lỗi và nêu tên một tệp | Không lần được chuỗi backing của đĩa tới cuối. Kiểm tra trong VM Manager rằng mọi snapshot của đĩa đó vẫn còn, và tệp được nêu tồn tại bên dưới `/mnt`. |

Thêm các triệu chứng về VM nằm trong [Xử lý sự cố](troubleshooting.md#vm-backup-will-not-connect).

## Giữ qua các lần khởi động lại {#persistence}

Unraid ghi `/etc/ssh/sshd_config` mới ở mỗi lần khởi động, nên các thay đổi phía máy chủ phải được làm lại khi khởi động:

- **Cổng SSH** được giữ trong `/boot/config/ident.cfg`, đặt qua giao diện web của Unraid.
- **`StrictModes no`**, nếu bạn cần nó ở trên, không được giữ. Hãy áp dụng nó lúc khởi động từ `/boot/config/go`:

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

- Khóa SSH của riêng BombVault và `~/.ssh/config` nằm trong `/config` (appdata) và được tạo lại ở mỗi lần khởi động, nên chúng sống sót qua các lần khởi động lại container và máy chủ.

## Bảo mật {#security}

Khóa được cấp quyền cho phép dùng root trên máy chủ, cùng mức tin cậy với Docker socket mà BombVault vốn đã dùng. Hãy giữ BombVault trên một mạng tin cậy. Khóa là ed25519, được lưu với chế độ `0600` trong appdata, và khóa của máy chủ được ghim. Mọi lệnh trên máy chủ được truyền dưới dạng các đối số riêng, không bao giờ qua một shell từ xa, và kết nối SSH dùng `BatchMode` cùng thời gian chờ kết nối.
