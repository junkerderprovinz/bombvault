# Bắt đầu

Trang này dẫn bạn đi từ một máy Unraid mới tinh đến bản sao lưu đầu tiên của bạn.

## Yêu cầu

| Yêu cầu | Ghi chú |
|---|---|
| **Unraid 6.12+** | Các phiên bản cũ hơn chưa được kiểm thử. Unraid là mục tiêu chính, nhưng BombVault cũng chạy trên một máy chủ Docker thông thường và trên TrueNAS Scale (xem [Máy chủ Docker thông thường](#generic-docker-host)). |
| **Vị trí kho restic** | Một đường dẫn cục bộ (khuyến nghị: mảng hoặc cache của bạn), SMB, NFS, hoặc bất kỳ backend rclone nào. |
| **Docker socket** | Được template gắn kết tự động (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Được template gắn kết toàn bộ tự động (`/boot` tới `/host/boot`). Nó cấp năng lượng cho việc sao lưu flash và cho phép một container đã khôi phục xuất hiện lại như một ứng dụng Unraid bình thường, có thể chỉnh sửa. |
| **KVM VM** (tùy chọn tham gia) | Sao lưu VM kết nối libvirt qua SSH, không gắn kết libvirt. Hãy thiết lập trong Cài đặt (xem [Cấu hình](configuration.md)). |
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

Không dùng Unraid? BombVault cũng chạy như một container bình thường trên bất kỳ máy chủ Docker nào (đây cũng là nền cho hỗ trợ container trên TrueNAS Scale, trước khi có mục riêng trong danh mục ứng dụng ở đó).

1. Lấy tệp [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) đã sẵn sàng chỉnh sửa từ kho mã.
2. Đặt `APP_KEY` (xem bên dưới) và trỏ volume Host Data tới gốc dữ liệu thật của bạn: các ghi chú trong tệp hướng dẫn cả hai.
3. `docker compose up -d`, rồi mở `https://<ip-máy-chủ>:3443/`.

Khác gì so với Unraid:

- **Không có miền flash/USB.** Không có USB khởi động để thu giữ hay khôi phục, nên miền Flash trong phần cài đặt ở đây không có việc gì làm. Thay vào đó, miền Thư mục đưa ra gợi ý một cú nhấp **Thêm bộ định sẵn: cấu hình hệ thống máy chủ** (một bộ tệp `/etc` khởi đầu để bạn xem lại và sửa trước khi lưu), như bản tương đương chung và thiết thực.
- **Không có thông báo gốc của Unraid.** Các kênh thông báo riêng của BombVault (webhook, cảnh báo hỏng bản sao ngoại vi và những thứ tương tự) vẫn chạy bình thường; chỉ việc đẩy sang hệ thống thông báo riêng của Unraid là được bỏ qua, vì ở đây không có hệ thống đó.
- **Sao lưu máy ảo là tùy chọn và cần một máy chủ libvirtd riêng, tới được qua SSH.** Xem khối bị chú thích trong tệp compose. Bản thân một máy chủ Docker thông thường không có sẵn trình quản lý máy ảo.
- **Không có widget bảng điều khiển.** BombVault Widget là một plugin Unraid, nên bước đó cũng được bỏ qua.
- **Tìm dữ liệu của một container.** Không có quy ước `appdata` của Unraid, thư mục dữ liệu của một container được tìm từ các đoạn trong `DATA_ROOT_SEGMENTS`, volume có tên của Docker, thư mục làm việc của một dự án Compose và nhãn `bombvault.data` (xem [Nhận diện nguồn sao lưu](configuration.md#backup-source-detection)). Volume có tên và bộ định sẵn `/etc` chỉ tới được các đường dẫn bên trong điểm gắn Host Data, nên hãy trỏ Host Data tới một thư mục cha chung bao gồm cả thư mục gốc dữ liệu của Docker.
- **`PLATFORM`.** Đặt thành `generic` hoặc `truenas`. Nếu để trống, BombVault nhận ra Unraid nhờ dấu hiệu riêng của Unraid trên điểm gắn flash và coi mọi trường hợp khác là máy chủ thông thường, và các bước chỉ dành cho Unraid được bỏ qua thay vì thử rồi thất bại.

**TrueNAS Scale** đi theo cùng con đường compose; một mục danh mục đã được chuẩn bị trong kho mã nhưng chưa được gửi. Sao lưu VM ở đó cần `LIBVIRT_URI`, vì libvirtd của TrueNAS lắng nghe trên một socket riêng (`/run/truenas_libvirt/libvirt-sock`) mà ba biến `LIBVIRT_*` không thể biểu diễn (xem [Cấu hình](configuration.md)). Mức độ đã được chứng minh: bản sao lưu zvol đã được chạy trên một máy TrueNAS Scale thật, trên một zvol gắn với một VM đang chạy, và `zfs snapshot`, `zfs send`, restic và `zfs receive` đã đi trọn một vòng khớp từng byte. Một lần khôi phục đầy đủ do chính BombVault thực hiện vẫn chưa được chạy trên phần cứng TrueNAS, và zvol đó là loại thưa (sparse), nên thông lượng ở mức nhiều gigabyte chưa được kiểm thử. Hãy thử khôi phục ở đó trước khi dựa vào nó.

## Cài đặt bắt buộc duy nhất

Biến duy nhất bạn phải đặt là `APP_KEY`, một bí mật hex 32 byte (64 ký tự hex) dùng để dẫn xuất mật khẩu kho lưu trữ restic.

Tạo một khóa trên bất kỳ máy nào:

```bash
openssl rand -hex 32
```

Dán kết quả vào trường `APP_KEY` của template (Unraid), hoặc biến môi trường `APP_KEY` trong `docker-compose.yml` (máy chủ Docker thông thường).

!!! danger "Đừng đánh mất APP_KEY của bạn"
    Đánh mất `APP_KEY` khiến các bản sao lưu đã mã hóa của bạn không thể khôi phục được. Hãy cất giữ nó ở nơi an toàn và tách biệt khỏi máy chủ. Sau khi BombVault chạy, hãy dùng **bộ khôi phục khóa mã hóa** một cú nhấp của nó (xem [Off-site & khôi phục](offsite-recovery.md)) để lưu trọn gói khôi phục đầy đủ.

Template cũng gắn kết Docker socket, flash (`/boot`) và gốc **Host Data** (`/mnt`) cho bạn. Cả *nguồn* và *đích* sao lưu đều nằm dưới Host Data. Để xem tham chiếu biến đầy đủ và thiết lập off-site, xem [Cấu hình](configuration.md).

## Lần chạy đầu tiên

![Bảng điều khiển sau bản sao lưu đầu tiên: cái gì được bảo vệ, cái gì chạy tiếp, và một nhật ký trực tiếp.](assets/screenshots/dashboard.png)

*Bảng điều khiển sau bản sao lưu đầu tiên: cái gì được bảo vệ, cái gì chạy tiếp, và một nhật ký trực tiếp.*

1. Mở giao diện web tại `https://<your-unraid-ip>:3443` (chứng chỉ tự ký ngay từ đầu).
2. Trong **Cài đặt**, bật các miền sao lưu bạn muốn (Containers, VMs, Flash, Tự sao lưu, Thư mục, Tập dữ liệu ZFS) và chọn một màu nhấn.
3. Ở tab **Containers**, chọn một container và nhấp **Sao lưu ngay** để tạo điểm khôi phục đầu tiên của bạn. Các đường dẫn kho mặc định là `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` và được tạo ở lần sao lưu đầu tiên.
4. Thiết lập lập lịch từ **Cài đặt, Lịch trình**. Có một tùy chọn *Đưa tất cả vào lịch trình* một cú nhấp cho container và VM.

!!! tip "Tùy chọn: chọn một thứ tự sao lưu"
    Nếu một số container luôn cần được sao lưu trước những cái khác (ví dụ một cơ sở dữ liệu trước ứng dụng dùng nó), hãy mở bảng **Thứ tự sao lưu** trên trang Containers và kéo chúng vào trình tự bạn muốn. Các lần chạy theo lịch và chọn nhiều sau đó sẽ tuân theo thứ tự này; bất cứ cái nào bạn để không sắp xếp sẽ được sao lưu theo thứ tự quá hạn nhất trước, như trước đây.

!!! note "Kiểm tra tích hợp máy chủ"
    Mở `/spike` trong giao diện web sau khi container khởi động. Nó kiểm thử mọi điểm gắn kết và CLI (Docker socket, libvirt, restic, qemu-img, rclone) và báo cáo bất kỳ phần nào bị thiếu, để bạn có thể xác nhận container được kết nối đúng cách trước khi tin cậy nó.

## Đơn giản so với Nâng cao

![Phần cài đặt không có nút Lưu: mỗi thay đổi được ghi ngay khi bạn thực hiện.](assets/screenshots/settings.png)

*Phần cài đặt không có nút Lưu: mỗi thay đổi được ghi ngay khi bạn thực hiện.*

Theo mặc định, giao diện chỉ hiển thị những thứ thiết yếu (sao lưu, khôi phục, lên lịch). Dùng công tắc **Chế độ xem đơn giản / Chế độ xem nâng cao** trong thanh bên để hé lộ các điều khiển chuyên gia: lưu giữ, bản sao off-site, hook trước/sau, khôi phục ở cấp tập tin, thông báo, số liệu Prometheus và các công cụ toàn vẹn/bảo trì. Đây là một tùy chọn theo từng trình duyệt và tắt theo mặc định, nên người mới có giao diện gọn gàng còn người dùng chuyên sâu có đủ mọi thứ.

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

`go run` cũng cần bản dựng giao diện. Kho mã chỉ theo dõi một tệp đánh dấu rỗng dưới `web/dist`, nên nếu không chạy `npm --prefix web run build` thì tệp nhị phân không nhúng gì và trả lời `500 SPA index not found`, điều này là bình thường. Docker, libvirt và Unraid không thể kiểm thử trong CI, nên hãy kiểm tra các điểm gắn kết, restic và liên kết SSH của VM trên một máy chủ thật bằng Kiểm tra tích hợp máy chủ (`/spike`) trước khi mở một pull request.

## Bước tiếp theo

- Duyệt đầy đủ **[Tính năng](features.md)**.
- Đưa mọi máy chủ trong nhóm của bạn lên điện thoại với **[Ứng dụng Android](android.md)**.
- Thêm một hoặc nhiều bản sao **[Off-site & khôi phục](offsite-recovery.md)** (mỗi miền có thể gửi tới nhiều đích cùng lúc) và lưu bộ khôi phục của bạn.
- Nhân bản một thiết lập hay chuyển sang một máy mới? Mang toàn bộ cấu hình của bạn theo với thẻ **Xuất / nhập cài đặt**. Xem [Cấu hình](configuration.md#portable-settings-export-and-import).
- Gặp trục trặc? Xem **[Khắc phục sự cố](troubleshooting.md)**.
