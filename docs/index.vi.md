# BombVault

**Dữ liệu Unraid của bạn, niêm phong trong một két sắt. Thả một bản sao lưu vào. Kích nổ một lần khôi phục.**

BombVault là một ứng dụng web tự lưu trữ, thiết kế riêng cho Unraid, dùng để **sao lưu và khôi phục toàn diện sau thảm họa** cho các Docker container và KVM/libvirt VM của bạn. Nó chạy dưới dạng một Docker container đa kiến trúc duy nhất, cung cấp một giao diện web tối hiện đại, và xử lý toàn bộ vòng đời: sao lưu, lên lịch, xác minh và khôi phục.

Việc khôi phục diễn ra tự động. Các container xuất hiện lại trong tab Docker của Unraid y hệt như trước, và các VM được định nghĩa lại trong VM Manager cùng với đĩa và UEFI NVRAM được gắn lại. Không cần cài đặt lại thủ công, không cấu hình lại, không rắc rối.

Được vận hành bởi [restic](https://restic.net), nên mỗi bản sao lưu đều được khử trùng lặp, tăng dần và luôn được mã hóa.

!!! note "Giữ APP_KEY của bạn an toàn"
    BombVault dẫn xuất mật khẩu kho lưu trữ restic từ một bí mật 32 byte tên là `APP_KEY`. Đánh mất nó khiến các bản sao lưu đã mã hóa không thể khôi phục được. Hãy tạo một khóa bằng `openssl rand -hex 32` và cất giữ nó ở nơi an toàn. Xem [Cấu hình](configuration.md).

## BombVault bảo vệ những gì

| Miền | Những gì được lưu |
|---|---|
| **Docker container** | Thư mục appdata cùng với định nghĩa container (image, biến môi trường, cổng, nhãn, volume). |
| **KVM / libvirt VM** | (Các) ảnh đĩa VM, định nghĩa XML và UEFI NVRAM, được sao lưu qua SSH (không gắn kết libvirt). |
| **Unraid flash** | Toàn bộ USB flash (`/boot`): OS, giấy phép, cấu hình mảng, share, cấu hình mạng và plugin. |
| **Cấu hình ứng dụng** | `/config` của chính BombVault: cơ sở dữ liệu cài đặt, thông tin đăng nhập off-site và cặp khóa SSH libvirt. |
| **Tập tin & thư mục** | Các **bộ tập tin** có tên, bất kỳ thư mục nào trên máy chủ, mỗi bộ có tùy chọn mẫu loại trừ riêng. |

## Khôi phục là ngôi sao

Sau khi sao chép dữ liệu trở lại từ snapshot restic, BombVault phát lại định nghĩa container đã lưu qua Docker API, nên container xuất hiện lại trong tab Docker của Unraid như thể nó vẫn luôn ở đó (cùng image, cùng cài đặt, cùng ánh xạ cổng). Các VM được định nghĩa lại XML qua SSH cùng với đĩa và UEFI NVRAM được gắn lại, ngay cả sau khi VM đã bị xóa.

Khi một lần sao lưu dừng các container phụ thuộc, chúng quay trở lại theo đúng thứ tự: BombVault khởi động lại chúng theo thứ tự `depends_on` của Compose và chờ mỗi cái báo khỏe mạnh trước khi khởi động những cái phụ thuộc vào nó, nên không có gì chạy vượt lên trước một cơ sở dữ liệu hay một gateway chưa sẵn sàng. Xem [Tính năng](features.md).

## Cách nó hoạt động

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

BombVault là lớp điều phối và giao diện, không phải công cụ lưu trữ. Toàn bộ việc di chuyển dữ liệu thực tế đều đi qua restic.

## Bắt đầu nhanh

Mới đến đây? Hãy vào **[Bắt đầu](getting-started.md)** để cài đặt BombVault trên Unraid qua Community Applications và chạy bản sao lưu đầu tiên của bạn. Sau đó khám phá đầy đủ **[Tính năng](features.md)**, tinh chỉnh **[Cấu hình](configuration.md)** của bạn, và thiết lập **[Off-site & khôi phục](offsite-recovery.md)**.

Off-site có thể phân phối tới nhiều đích cho mỗi miền cùng lúc, một **bảng điều khiển bên nhận** chỉ đọc giám sát các bản sao đó trên máy nhận chúng, và bạn có thể mang toàn bộ cấu hình của mình sang một máy mới bằng thẻ **Xuất và nhập cài đặt**. Xem [Off-site & khôi phục](offsite-recovery.md) và [Cấu hình](configuration.md#portable-settings-export-and-import).

## Liên kết

- **Mã nguồn:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Luồng hỗ trợ Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Vấn đề (Issues):** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Quyền kiểm soát máy chủ tương đương root"
    Thông qua Docker socket, BombVault có thể dừng, xóa và tạo lại các container cũng như đọc/ghi appdata, và để sao lưu VM nó đăng nhập vào máy chủ qua SSH để chạy `virsh`. Bất kỳ ai truy cập được giao diện web của nó thực chất đều có quyền root trên máy chủ. Chỉ chạy BombVault trên một mạng tin cậy, không phơi ra ngoài, và bật cổng bảo vệ mật khẩu tùy chọn (Cài đặt, Bảo mật) một khi bạn dùng đến sao lưu off-site hoặc bất biến. Xem [Cấu hình](configuration.md) để biết mô hình bảo mật đầy đủ.
