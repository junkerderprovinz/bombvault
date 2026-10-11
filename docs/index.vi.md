# BombVault

**Dữ liệu Unraid của bạn, niêm phong trong một két sắt. Thả một bản sao lưu vào. Kích nổ một lần khôi phục.**

BombVault là một ứng dụng web tự lưu trữ, thiết kế riêng cho Unraid, dùng để **sao lưu và khôi phục toàn diện sau thảm họa** cho các Docker container và KVM/libvirt VM của bạn. Nó chạy dưới dạng một Docker container đa kiến trúc duy nhất, cung cấp cho bạn giao diện web với chế độ tối, sáng hoặc theo hệ thống, và xử lý toàn bộ vòng đời: sao lưu, lên lịch, xác minh và khôi phục.

Việc khôi phục diễn ra tự động. Các container xuất hiện lại trong tab Docker của Unraid y hệt như trước, và các VM được định nghĩa lại trong VM Manager cùng với đĩa và UEFI NVRAM được gắn lại. Không cần cài đặt lại thủ công, không cấu hình lại, không rắc rối.

Được vận hành bởi [restic](https://restic.net), nên mỗi bản sao lưu đều được khử trùng lặp, tăng dần và luôn được mã hóa.

!!! note "Giữ APP_KEY của bạn an toàn"
    BombVault dẫn xuất mật khẩu kho lưu trữ restic từ một bí mật 32 byte tên là `APP_KEY`. Mất nó cùng với bộ khôi phục sẽ khiến các bản sao lưu đã mã hóa không thể khôi phục được. Hãy tạo một khóa bằng `openssl rand -hex 32` và cất giữ nó ở nơi an toàn. Xem [Cấu hình](configuration.md).

## BombVault bảo vệ những gì

Mỗi container, VM, bộ thư mục, ổ flash, mỗi tập dữ liệu ZFS và chính cài đặt của BombVault là một **mục** có trang riêng. Các mục cùng loại tạo thành một **phần**, và bạn bật phần đó trong **Cài đặt, Chung**.

| Phần | Những gì được lưu |
|---|---|
| **Containers** | Thư mục appdata cùng với định nghĩa container (image, biến môi trường, cổng, nhãn, volume). |
| **VMs** | (Các) ảnh đĩa VM, định nghĩa XML và UEFI NVRAM, được sao lưu qua SSH (không gắn kết libvirt). |
| **Flash** | Toàn bộ USB flash (`/boot`): OS, giấy phép, cấu hình mảng, share, cấu hình mạng và plugin. |
| **Tự sao lưu** | `/config` của chính BombVault: cơ sở dữ liệu cài đặt, thông tin đăng nhập của các đích sao lưu và cặp khóa SSH libvirt. |
| **Thư mục** | Bất kỳ thư mục nào trên máy chủ dưới dạng một bộ thư mục, mỗi bộ có tùy chọn mẫu loại trừ riêng. |
| **Tập dữ liệu ZFS** | Một tập dữ liệu cùng mọi tập bên dưới, đọc từ một ảnh chụp ZFS và lưu như một thư mục. Xem [Tập dữ liệu ZFS](zfs-datasets.md). |

## Khôi phục là ngôi sao

Sau khi sao chép dữ liệu trở lại từ snapshot restic, BombVault phát lại định nghĩa container đã lưu qua Docker API, nên container xuất hiện lại trong tab Docker của Unraid như thể nó vẫn luôn ở đó (cùng image, cùng cài đặt, cùng ánh xạ cổng). Các VM được định nghĩa lại XML qua SSH cùng với đĩa và UEFI NVRAM được gắn lại, ngay cả sau khi VM đã bị xóa.

Một cửa sổ khôi phục mọi loại. Mặc định nó chọn **Bên cạnh bản gốc**, đưa bản sao lưu trở lại dưới dạng một bản sao và giữ nguyên bản gốc. **Thay thế bản gốc** sao lưu trạng thái hiện tại trước, nên một lần khôi phục có thể được hoàn tác. Một thư mục hoặc một tập dữ liệu ZFS mới là các đích khác.

Khi một lần sao lưu dừng các container phụ thuộc, chúng quay trở lại theo đúng thứ tự: BombVault khởi động lại chúng theo thứ tự `depends_on` của Compose và chờ mỗi cái báo khỏe mạnh trước khi khởi động những cái phụ thuộc vào nó, nên không có gì chạy vượt lên trước một cơ sở dữ liệu hay một gateway chưa sẵn sàng. Xem [Tính năng](features.md).

## Cách nó hoạt động

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault dùng Docker socket để dừng các container trước khi sao lưu và tạo lại chúng sau khi khôi phục. Với VM, nó chạy `virsh` trên máy chủ qua SSH (`qemu+ssh://`) để tắt một domain êm ái hoặc chụp một snapshot trực tiếp. Nó không bao giờ bind-mount một đường dẫn libvirt, nên không thể cản trở VM Manager trên máy chủ.

BombVault là lớp điều phối và giao diện, không phải công cụ lưu trữ. Toàn bộ việc di chuyển dữ liệu thực tế đều đi qua restic.

## Bắt đầu nhanh

Mới đến đây? Hãy vào **[Bắt đầu](getting-started.md)** để cài đặt BombVault trên Unraid qua Community Applications và chạy bản sao lưu đầu tiên của bạn. Sau đó khám phá đầy đủ **[Tính năng](features.md)**, tinh chỉnh **[Cấu hình](configuration.md)** của bạn, và thiết lập **[Off-site & khôi phục](offsite-recovery.md)**.

Mỗi phần có thể sao chép tới nhiều đích sao lưu cùng lúc, một BombVault đã ghép nối có thể giữ các bản sao off-site của bạn trên một máy chủ từ chối xóa, và **Trình hướng dẫn khẩn cấp** đưa một máy chủ bị mất trở lại từ bộ khôi phục của nó, qua nhóm của bạn hoặc bằng khóa nhập tay. Để mang cấu hình sang một máy mới, hãy dùng thẻ **Xuất / nhập cài đặt**. Xem [Off-site & khôi phục](offsite-recovery.md) và [Cấu hình](configuration.md#portable-settings-export-and-import).

**[Ứng dụng Android](android.md)** đưa mọi máy chủ trong nhóm của bạn lên điện thoại, với nhật ký hoạt động của tất cả trên một màn hình.

## Ghi công {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** của [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) đã cho BombVault ý tưởng khởi đầu: sao lưu bằng một cú nhấp và tự động cài đặt lại các Docker container. BombVault là một bản hiện thực riêng trên Go và restic, đưa ý tưởng đó sang VM, flash và hơn thế nữa.
- **[restic](https://restic.net/)** là công cụ sao lưu nhanh, an toàn, có khử trùng lặp mà BombVault điều khiển.
- **[rclone](https://rclone.org/)** cung cấp các backend đám mây.
- Phần lớn biểu tượng trên các nút lấy từ bộ Core Solid miễn phí của **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [nguồn](https://github.com/webalys-hq/streamline-vectors)). Số còn lại lấy từ Font Awesome Free, Material Design Icons, Simple Icons và Tabler Icons, hoặc được vẽ riêng cho dự án.

## Giấy phép {#license}

Copyright (C) 2026 Junker der Provinz. BombVault là phần mềm tự do theo **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Bạn có thể chạy, nghiên cứu, chia sẻ và sửa đổi nó. Nếu bạn phân phối nó, hoặc chạy một phiên bản đã sửa đổi dưới dạng dịch vụ mạng, bạn phải công bố mã nguồn của mình theo cùng giấy phép đó và giữ nguyên các thông báo bản quyền và ghi nhận tác giả hiện có.

Tên và thương hiệu không được cấp phép. AGPL chỉ áp dụng cho mã nguồn: "BombVault", logo và thương hiệu của nó vẫn được bảo lưu, nên một bản fork phải dùng tên và thương hiệu của riêng mình và không được tự giới thiệu là BombVault.

## Liên kết

- **Mã nguồn:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Luồng hỗ trợ Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Vấn đề (Issues):** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Quyền kiểm soát máy chủ tương đương root"
    Thông qua Docker socket, BombVault có thể dừng, xóa và tạo lại các container cũng như đọc/ghi appdata, và để sao lưu VM nó đăng nhập vào máy chủ qua SSH để chạy `virsh`. Bất kỳ ai truy cập được giao diện web của nó thực chất đều có quyền root trên máy chủ. Chỉ chạy BombVault trên một mạng tin cậy, không phơi ra ngoài, và đặt mật khẩu đăng nhập trong **Cài đặt, Bảo mật**. Bộ khôi phục, bản gửi khóa và việc xóa các bản sao lưu ở một đích sao lưu đều cần mật khẩu đó. Xem [Cấu hình](configuration.md#security-model) để biết mô hình bảo mật đầy đủ.
