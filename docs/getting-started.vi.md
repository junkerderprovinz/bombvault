# Bắt đầu

Trang này dẫn bạn đi từ một máy Unraid mới tinh đến bản sao lưu đầu tiên của bạn.

## Yêu cầu

| Yêu cầu | Ghi chú |
|---|---|
| **Unraid 6.12+** | Các phiên bản cũ hơn chưa được kiểm thử. Unraid là mục tiêu chính, nhưng BombVault cũng chạy trên một máy chủ Docker thông thường và trên TrueNAS Scale (xem [Máy chủ Docker thông thường](#generic-docker-host)). |
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

Không dùng Unraid? BombVault cũng chạy như một container bình thường trên bất kỳ máy chủ Docker nào (đây cũng là nền cho hỗ trợ container trên TrueNAS Scale, trước khi có mục riêng trong danh mục ứng dụng ở đó).

1. Lấy tệp [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) đã sẵn sàng chỉnh sửa từ kho mã.
2. Đặt `APP_KEY` (xem bên dưới) và trỏ volume Host Data tới gốc dữ liệu thật của bạn. Các ghi chú trong tệp hướng dẫn cả hai.
3. `docker compose up -d`, rồi mở `https://<ip-máy-chủ>:3443/`.

Khác gì so với Unraid:

- **Không có phần Flash.** Không có USB khởi động để thu giữ hay khôi phục, nên phần Flash ở đây không có việc gì làm. Thay vào đó, **Thêm** trên trang Bản sao lưu đưa ra **Cấu hình hệ thống** làm gợi ý thư mục: một bộ thư mục `/etc` khởi đầu để bạn xem lại và sửa trước khi lưu.
- **Không có thông báo gốc của Unraid.** Các kênh thông báo riêng của BombVault (webhook, cảnh báo hỏng bản sao ngoại vi và những thứ tương tự) vẫn chạy bình thường; chỉ việc đẩy sang hệ thống thông báo riêng của Unraid là được bỏ qua, vì ở đây không có hệ thống đó.
- **Sao lưu VM là tùy chọn và cần một máy chủ libvirtd riêng, tới được qua SSH.** Xem khối bị chú thích trong tệp compose. Bản thân một máy chủ Docker thông thường không có sẵn trình quản lý VM.
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

- **Bản sao lưu** liệt kê mọi container, VM, bộ thư mục, ổ flash, mọi tập dữ liệu ZFS và mục tự sao lưu. Các ô phía trên danh sách chọn một loại, còn **Bộ lọc**, **Sắp xếp** và **Tìm kiếm** thu hẹp danh sách. Mỗi mục có một trang riêng với ba tab: **Bản sao lưu**, **Cài đặt** và **Xác minh**.
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
