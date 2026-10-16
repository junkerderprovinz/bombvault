# Bắt đầu

Trang này dẫn bạn đi từ một máy Unraid mới tinh đến bản sao lưu đầu tiên của bạn.

## Yêu cầu

| Yêu cầu | Ghi chú |
|---|---|
| **Unraid 6.12+** | Các phiên bản cũ hơn chưa được kiểm thử. Unraid là mục tiêu chính, nhưng BombVault cũng chạy trên một máy chủ Docker thông thường và trên TrueNAS Scale (xem [Máy chủ Docker thông thường](#generic-docker-host) và [TrueNAS SCALE](#truenas)). |
| **Vị trí kho restic** | Một đường dẫn cục bộ (khuyến nghị: mảng hoặc cache của bạn), SMB, NFS, hoặc bất kỳ backend rclone nào. |
| **Docker socket** | Được template gắn kết tự động (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Được template gắn kết toàn bộ tự động (`/boot` tới `/host/boot`). Nó cấp năng lượng cho việc sao lưu flash và cho phép một container đã khôi phục xuất hiện lại như một ứng dụng Unraid bình thường, có thể chỉnh sửa. |
| **KVM VM** (tùy chọn tham gia) | Sao lưu VM kết nối libvirt qua SSH, không gắn kết libvirt. Hãy thiết lập tại **Cài đặt, Tích hợp, SSH máy chủ** (xem [Cấu hình](configuration.md)). |
| **Tập dữ liệu ZFS** (tùy chọn tham gia) | Cùng liên kết SSH như sao lưu VM, `zfs` trên máy chủ, và Host Data được ánh xạ thành `/mnt` với chế độ truy cập Read/Write - Slave, là mặc định của template. Xem [Tập dữ liệu ZFS](zfs-datasets.md). |
| **Ứng dụng Android** (không bắt buộc) | Android 10 trở lên, ghép nối với các máy chủ phiên bản 9.7.0 trở lên. Xem [Ứng dụng Android](android.md). |

## Cài đặt trên Unraid

Cách dễ nhất là qua **Community Applications**.

1. Mở tab **Apps** trong Unraid.
2. Tìm kiếm **BombVault**.
3. Nhấp **Install**, đặt các biến bắt buộc (bên dưới), rồi áp dụng.

!!! tip "Cài template thủ công"
    Nếu bạn thích thêm template bằng tay:

    1. Vào **Docker, Add Container, Template repositories** và thêm:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Tìm kiếm **BombVault** trong Templates.
    3. Đặt các biến bắt buộc và nhấp **Apply**.

## Máy chủ Docker thông thường {#generic-docker-host}

Không dùng Unraid? BombVault cũng chạy như một container bình thường trên bất kỳ máy chủ Docker nào. TrueNAS SCALE có các bước riêng trong [TrueNAS SCALE](#truenas).

1. Lấy tệp [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) đã sẵn sàng chỉnh sửa từ kho mã.
2. Đặt `APP_KEY` (xem bên dưới) và trỏ volume Host Data tới gốc dữ liệu thật của bạn. Các ghi chú trong tệp hướng dẫn cả hai.
3. `docker compose up -d`, rồi mở `https://<ip-máy-chủ>:3443/`.

Khác gì so với Unraid:

- **Không có phần Flash.** Không có USB khởi động để thu giữ hay khôi phục, nên phần Flash ở đây không có việc gì làm. Thay vào đó, khi Host Data tới được `/etc` của máy chủ, **Thêm** trên trang Bản sao lưu đưa ra **Cấu hình hệ thống** làm gợi ý thư mục: một bộ thư mục `/etc` khởi đầu để bạn xem lại và sửa trước khi lưu.
- **Không có thông báo gốc của Unraid.** Các kênh thông báo riêng của BombVault (webhook, cảnh báo hỏng bản sao ngoại vi và những thứ tương tự) vẫn chạy bình thường; chỉ việc đẩy sang hệ thống thông báo riêng của Unraid là được bỏ qua, vì ở đây không có hệ thống đó.
- **Sao lưu VM là tùy chọn và cần một máy chủ libvirtd riêng, tới được qua SSH.** Xem khối bị chú thích trong tệp compose. Bản thân một máy chủ Docker thông thường không có sẵn trình quản lý VM.
- **Không có widget bảng điều khiển.** BombVault Widget là một plugin Unraid, nên bước đó cũng được bỏ qua.
- **Tìm dữ liệu của một container.** Không có quy ước `appdata` của Unraid, thư mục dữ liệu của một container được tìm từ các đoạn trong `DATA_ROOT_SEGMENTS`, volume có tên của Docker, thư mục làm việc của một dự án Compose và nhãn `bombvault.data` (xem [Nhận diện nguồn sao lưu](configuration.md#backup-source-detection)). Volume có tên và bộ định sẵn `/etc` chỉ tới được các đường dẫn bên trong điểm gắn Host Data, nên hãy trỏ Host Data tới một thư mục cha chung bao gồm cả thư mục gốc dữ liệu của Docker.
- **`PLATFORM`.** Đặt thành `generic` hoặc `truenas`. Nếu để trống, BombVault nhận ra Unraid nhờ dấu hiệu riêng của Unraid trên điểm gắn flash và coi mọi trường hợp khác là máy chủ thông thường, và các bước chỉ dành cho Unraid được bỏ qua thay vì thử rồi thất bại.

### TrueNAS SCALE {#truenas}

BombVault chưa có trong danh mục ứng dụng của TrueNAS, nên hãy cài nó như một ứng dụng tùy chỉnh: **Apps, Discover Apps**, rồi chọn **Install via YAML** trong menu bên cạnh **Custom App**. YAML dưới đây đặt những thứ giống như mục danh mục trong thư mục `truenas-apps` của kho mã. Thay `tank` bằng pool của bạn và điền `APP_KEY` như phần tiếp theo mô tả.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # Cho sao lưu VM:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Sau đó mở `https://<truenas-ip>:30528/`. Những gì liệt kê trong [Khác gì so với Unraid](#generic-docker-host) cũng áp dụng ở đây, và trên TrueNAS còn có:

- **Capabilities.** BombVault cần cả bảy capability ở trên. Sao lưu VM sẽ thất bại nếu thiếu `SETPCAP`, và nếu thiếu `SYS_CHROOT` thì mọi container đều bị dừng suốt cả lần sao lưu thay vì được đọc từ ảnh chụp.
- **Host Data** là một đường dẫn máy chủ trên một tập dữ liệu, ở đây là `/mnt/tank`, được gắn kết tại cùng đường dẫn bên trong ứng dụng. Vì vậy `HOST_SOURCE_ROOT` và `HOST_MOUNT_ROOT` đều nêu đường dẫn này. `rslave` chuyển tiếp các lần gắn kết mà máy chủ tạo sau đó, điều mà sao lưu tập dữ liệu ZFS cần.
- **Dữ liệu trong ixVolume không được sao lưu.** Một ứng dụng dùng ixVolume làm nơi lưu trữ sẽ giữ dữ liệu dưới `/mnt/.ix-apps`, bên ngoài Host Data. BombVault chỉ lưu định nghĩa của ứng dụng, và lần chạy nêu tên các thư mục đã bị bỏ ra. Với mọi ứng dụng mà bạn muốn sao lưu dữ liệu, hãy chọn kiểu lưu trữ **Host Path** trên một tập dữ liệu nằm dưới Host Data.
- **Sao lưu VM và tập dữ liệu ZFS** kết nối tới TrueNAS qua SSH với quyền root. Bật dịch vụ SSH trong **System, Services**, và thêm khóa công khai từ **Cài đặt, Tích hợp, SSH máy chủ** cho người dùng root trong **Credentials, Users**. Sao lưu VM còn cần `LIBVIRT_URI` như ở trên, vì libvirtd của TrueNAS lắng nghe trên một socket riêng. Khi cả `LIBVIRT_URI` lẫn `LIBVIRT_HOST` đều chưa được đặt, BombVault dùng cổng mặc định (default gateway) của ứng dụng làm máy chủ, vì `host.docker.internal` không phân giải được trong một ứng dụng TrueNAS. [Hướng dẫn sao lưu VM](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md#truenas-scale) trình bày chi tiết, kể cả VM trên zvol.
- **Bản sao ZFS dùng một khóa khác.** Một bản sao trên máy chủ ZFS thứ hai đăng nhập bằng khóa mà hộp thoại bản sao hiển thị, không phải khóa SSH máy chủ. Hãy thêm khóa đó trên máy chủ nhận.
- **Một ứng dụng đã xóa được đưa lại qua hai bước.** Trước tiên cài lại nó trong TrueNAS với cùng tên, việc này tạo lại mạng của nó, rồi khôi phục dữ liệu của nó trong BombVault. Cho tới lúc đó, kiểm tra trước khi khôi phục báo mạng bị thiếu, và việc khôi phục từ chối trước khi thay đổi bất cứ thứ gì.
- **Bên cạnh bản gốc** tạo một container Docker thông thường. Nó chạy được, nhưng màn hình Apps của TrueNAS không liệt kê nó.
- **Bản kết xuất cơ sở dữ liệu** hoạt động trên các điểm gắn kết tập dữ liệu. Khi nhập vào một thư mục dữ liệu vốn là một tập dữ liệu riêng, nội dung cũ được chuyển vào một thư mục bên cạnh, nên tập dữ liệu phía trên cần đủ chỗ cho chúng.

## Cài đặt bắt buộc duy nhất

Biến duy nhất bạn phải đặt là `APP_KEY`, một bí mật hex 32 byte (64 ký tự hex) dùng để dẫn xuất mật khẩu kho lưu trữ restic.

Tạo một khóa trên bất kỳ máy nào:

```bash
openssl rand -hex 32
```

Dán kết quả vào trường `APP_KEY` của template (Unraid), hoặc biến môi trường `APP_KEY` trong `docker-compose.yml` (máy chủ Docker thông thường).

!!! danger "Đừng đánh mất APP_KEY của bạn"
    Đánh mất `APP_KEY` khiến các bản sao lưu đã mã hóa của bạn không thể khôi phục được. Hãy cất giữ nó ở nơi an toàn và tách biệt khỏi máy chủ. Khi BombVault đã chạy, hãy đặt mật khẩu đăng nhập và tải **bộ khôi phục** trong **Cài đặt, Khôi phục** (xem [Off-site & khôi phục](offsite-recovery.md#recovery-kit)). Bộ này chứa khóa, mọi đích sao lưu và các lệnh restic.

Template cũng gắn kết Docker socket, flash (`/boot`) và gốc **Host Data** (`/mnt`) cho bạn. Cả *nguồn* và *đích* sao lưu đều nằm dưới Host Data. Để xem tham chiếu biến đầy đủ và thiết lập off-site, xem [Cấu hình](configuration.md).

## Lần chạy đầu tiên

![Tổng quan sau bản sao lưu đầu tiên: những gì cần bạn xử lý, mức bảo vệ của từng phần, các đích sao lưu và lịch sử.](assets/screenshots/dashboard.png)

*Tổng quan sau bản sao lưu đầu tiên: những gì cần bạn xử lý, mức bảo vệ của từng phần, các đích sao lưu và lịch sử.*

1. Mở giao diện web tại `https://<your-unraid-ip>:3443` (chứng chỉ tự ký ngay từ đầu). Một bản cài mới mở ra trang **Chào mừng bạn đến với BombVault**, liệt kê những gì BombVault đã tìm thấy trên máy chủ.
2. Chọn **Bắt đầu thôi**. Trang Tổng quan khi đó hiện thẻ **Bước đầu tiên** với ba bước:
    1. **Chọn các mục**: bật những gì BombVault sao lưu (Containers, VMs, Flash, Thư mục, Tập dữ liệu ZFS, Tự sao lưu). **Chọn** mở danh sách các mục của một phần, nơi **Tạm dừng lịch trình** loại một mục ra. Các phần bạn bật ở đây chạy mỗi đêm, và bạn đổi điều đó trong **Cài đặt, Lịch trình**.
    2. **Thêm đích**: nơi lưu các bản sao lưu. Một thư mục cục bộ là đủ để bắt đầu. An toàn nhất là một đích thứ hai ngoài cơ sở, chẳng hạn Storage Box hoặc Backblaze B2. Các kho cục bộ mặc định nằm ở `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` và được tạo ở lần sao lưu đầu tiên.
    3. **Bản sao lưu đầu tiên**: sau đó nó tự chạy mỗi đêm.
3. Mở **Bản sao lưu** để xem mọi mục trong một danh sách. **Sao lưu tất cả ngay** bắt đầu một lượt chạy trên mọi phần đã bật, còn **Sao lưu ngay** trên trang của một mục chỉ sao lưu riêng mục đó.

Đã dùng BombVault trước đây? **Khôi phục từ bản sao lưu** trên trang chào mở trang Khôi phục, nơi **Trình hướng dẫn khẩn cấp** đưa cài đặt, container, VM và thư mục của một máy chủ bị mất trở lại. Xem [Off-site & khôi phục](offsite-recovery.md#emergency).

!!! tip "Tùy chọn: chọn một thứ tự sao lưu"
    Nếu một số container luôn cần được sao lưu trước những cái khác (ví dụ một cơ sở dữ liệu trước ứng dụng dùng nó), hãy mở thẻ **Thứ tự** trong **Cài đặt, Lịch trình** và kéo chúng vào trình tự bạn muốn. Các lần chạy theo lịch và lần sao lưu toàn bộ sau đó sẽ tuân theo thứ tự này; bất cứ cái nào bạn để không xếp thứ tự thì được sao lưu theo kiểu quá hạn lâu nhất trước.

!!! note "Kiểm tra tích hợp máy chủ"
    Thẻ **Tích hợp máy chủ** trên Tổng quan kiểm tra mọi điểm gắn kết và CLI (Docker socket, libvirt, restic, qemu-img, rclone) và báo cáo bất kỳ phần nào bị thiếu, để bạn có thể xác nhận container được kết nối đúng cách trước khi tin cậy nó. Thẻ này nằm chờ trong **Tùy chỉnh** cho đến khi bạn thêm nó.

## Tìm đường {#navigation}

![Cài đặt dưới dạng các ô, với các thẻ của ô được chọn bên cạnh. Mỗi thay đổi được ghi ngay khi bạn thực hiện.](assets/screenshots/settings.png)

*Cài đặt dưới dạng các ô, với các thẻ của ô được chọn bên cạnh. Mỗi thay đổi được ghi ngay khi bạn thực hiện.*

Thanh bên có năm mục: **Tổng quan**, **Bất thường** (khi bật phát hiện bất thường), **Bản sao lưu**, **Khôi phục** và, bên dưới **Đăng xuất**, **Cài đặt**. Hành động chính của một trang nổi ở dưới cùng bên phải, còn **Thêm** chứa những hành động hiếm dùng. Trên điện thoại, thanh chứa Tổng quan, Bản sao lưu, Khôi phục và Cài đặt, còn **Thêm** có phần còn lại.

Các trang và phần cài đặt dùng toàn bộ chiều rộng cửa sổ. Một cửa sổ giữ các nút của nó ở chân, nút đi tiếp nằm bên phải. Trên điện thoại, một chân có hơn ba nút giữ lại lối thoát và hành động chính, còn các nút khác nằm dưới **Thêm**.

- **Bản sao lưu** liệt kê mọi container, VM, bộ thư mục, ổ flash, mọi tập dữ liệu ZFS và mục tự sao lưu. Các ô phía trên danh sách chọn một loại, còn **Bộ lọc**, **Sắp xếp** và **Tìm kiếm** thu hẹp danh sách. Mỗi mục có một trang riêng với ba tab: **Bản sao lưu**, **Cài đặt** và **Xác minh**. Một nút quay lại ở bên trái biểu tượng của mục dẫn về danh sách.
- **Khôi phục** là một trang cho mọi thứ: chọn một phần, tìm một mục và chọn một bản sao lưu. Trình hướng dẫn khẩn cấp và các nguồn khác cũng bắt đầu từ đây.
- **Cài đặt** gồm mười bốn ô: Chung, Giao diện, Lịch trình, Đích sao lưu, Khôi phục, Thông báo, Xác minh, Bảo mật, Phiên bản, Ghép nối, Tích hợp, Ứng dụng, Hệ thống và Thông tin. Nhấn `/` hoặc Ctrl+K để tìm trong tất cả, kể cả theo tên mà các trang từng có ở các phiên bản trước. Kéo các ô vào thứ tự riêng của bạn trong **Giao diện**. Khi một cột ô bên cạnh trang sẽ làm trang quá hẹp, như trong cửa sổ rộng khoảng 1200 pixel, các ô trở thành một dải phía trên trang và cuộn ngang được. Điện thoại hiển thị cùng dải đó.
- **Đích sao lưu** và **Phiên bản** là các ô cài đặt có trang riêng. **Hiện cả trong thanh bên** trên ô của chúng ghim chúng vào thanh bên.

Không có gì có nút Lưu và không có gì bung ra: những thứ hiếm dùng có thẻ riêng hoặc một cửa sổ, và phần giải thích nằm trong các bong bóng (i). Sau một bản cập nhật, ghi chú phát hành tự mở, và các chấm xanh đánh dấu những gì đã đổi. Một cú nhấp sẽ ẩn chấm. **Mới** trong thẻ Phiên bản dưới **Cài đặt, Thông tin** đếm những gì bạn chưa xem và mở ghi chú phát hành cùng danh sách mọi thay đổi kèm **Đi tới đó** thành hai tab của một cửa sổ.

## Dựng từ mã nguồn {#build-from-source}

BombVault là một tệp nhị phân Go tĩnh duy nhất phục vụ một JSON API và một giao diện React được nhúng sẵn. Hãy dựng giao diện trước, rồi chạy tệp nhị phân:

```bash
npm --prefix web ci
npm --prefix web run build     # ghi ra web/dist, thứ mà tệp nhị phân nhúng vào
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # kiểm thử đơn vị và tích hợp, có một vòng restic thật
golangci-lint run ./...
go run ./cmd/bombvault         # phục vụ https://localhost:3443 với chứng chỉ tự ký
```

`go run` cũng cần bản dựng giao diện. Kho mã chỉ theo dõi một tệp đánh dấu rỗng dưới `web/dist`, nên nếu không chạy `npm --prefix web run build` thì tệp nhị phân không nhúng gì và trả lời `500 SPA index not found`, điều này là bình thường. Docker, libvirt và Unraid không thể kiểm thử trong CI, nên trước khi mở pull request, hãy kiểm tra các điểm gắn kết, restic và liên kết SSH của VM trên một máy chủ thật bằng thẻ **Tích hợp máy chủ** của Tổng quan (hoặc `GET /api/spike`).

## Bước tiếp theo

- Duyệt đầy đủ **[Tính năng](features.md)**.
- Đưa mọi máy chủ trong nhóm của bạn lên điện thoại với **[Ứng dụng Android](android.md)**.
- Thêm một hoặc nhiều đích sao lưu ngoài cơ sở (mỗi phần có thể sao chép tới nhiều đích cùng lúc) và lưu bộ khôi phục của bạn, như mô tả trong **[Off-site & khôi phục](offsite-recovery.md)**.

- Nhân bản một thiết lập hay chuyển sang một máy mới? Mang toàn bộ cấu hình của bạn theo với thẻ **Xuất / nhập cài đặt**. Xem [Cấu hình](configuration.md#portable-settings-export-and-import).
- Gặp trục trặc? Xem **[Khắc phục sự cố](troubleshooting.md)**.
