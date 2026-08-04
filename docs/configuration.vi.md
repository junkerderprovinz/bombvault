# Cấu hình

Trang này bao quát các biến môi trường của container, các điểm gắn kết mà template cung cấp, sao lưu VM qua SSH, và thiết lập off-site. Các **đường dẫn kho** sao lưu được cấu hình bên trong ứng dụng (Settings, Backup paths), không phải qua biến môi trường.

## Biến môi trường

| Biến | Bắt buộc | Mô tả |
|---|---|---|
| `APP_KEY` | **Có** | Bí mật hex 32 byte (64 ký tự hex) dùng để dẫn xuất mật khẩu kho restic. Tạo bằng `openssl rand -hex 32`. Giữ nó an toàn: đánh mất nó khiến các bản sao lưu đã mã hóa không thể khôi phục được. |
| `LIBVIRT_HOST` | Cho VM | Máy chủ Unraid được kết nối qua SSH để sao lưu VM (mặc định `host.docker.internal`; template điền sẵn một chỗ giữ chỗ IP-LAN). Dùng IP LAN Unraid của bạn, bắt buộc trên một mạng `br0.x` tùy chỉnh. |
| `LIBVIRT_SSH_PORT` | Không | Cổng SSH của máy chủ để sao lưu VM (mặc định `22`). |
| `LIBVIRT_SSH_USER` | Không | Người dùng SSH trên máy chủ để sao lưu VM (mặc định `root`). |
| `PORT` | Không | Cổng HTTP (mặc định `3000`; chỉ dùng với `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Không | Cổng HTTPS (mặc định `3443`; template công bố nó 1:1, nên WebUI trả lời tại `https://<ip>:3443`). |
| `HTTP_ONLY` | Không | Đặt `true` để tắt trình lắng nghe HTTPS tự ký và chỉ phục vụ HTTP thuần (để dùng phía sau một reverse proxy kết thúc TLS). |
| `HOST_SOURCE_ROOT` | Không | Đường dẫn máy chủ được gắn kết làm **Host Data** (mặc định `/mnt`). BombVault dịch các nguồn bind-mount mà Docker báo cáo thành các đường dẫn dưới điểm gắn kết này. Chỉ thay đổi nếu bạn đã gắn kết một gốc máy chủ khác. |
| `BOMBVAULT_SELF_CONTAINER` | Không | Tên của chính container BombVault, để nó không bao giờ sao lưu (và do đó dừng) chính mình (mặc định `BombVault`; tự phát hiện qua hostname trên mạng bridge). |
| `BACKUP_MAX_HOURS` | Không | Số giờ đồng hồ tối đa mà một lần sao lưu đơn có thể giữ khóa miền của nó trước khi bị hủy cưỡng bức (một biện pháp bảo vệ để một lần chạy bị kẹt không thể chặn miền mãi mãi). Để trống (mặc định) dùng `48`. Tăng nó lên cho các bản sao lưu đám mây rất lớn hoặc chậm (một lần chạy bị hủy ở mức giới hạn thất bại với `context deadline exceeded`). Đặt `0` để tắt hoàn toàn giới hạn. |
| `TZ` | Không | Múi giờ cho bộ lập lịch (ví dụ `Europe/Berlin`). |

## Điểm gắn kết

Gắn kết Docker socket, flash (`/boot`) và gốc **Host Data** (`/mnt`) như hiển thị trong template CA. Cả *nguồn* và *đích* sao lưu đều nằm dưới Host Data, và nó được gắn kết **rslave** nên một share từ xa được gắn kết sau khi container khởi động (ví dụ dưới `/mnt/remotes`) trở nên hiển thị mà không cần khởi động lại.

Các đường dẫn kho sao lưu mặc định là `/mnt/user/bombvault/{container,vms,flash,config,files}`, được tạo ở lần sao lưu đầu tiên. Thay đổi vị trí bất cứ lúc nào trong **Settings, Backup paths**.

!!! note "Kiểm tra tích hợp máy chủ"
    Mở `/spike` trong giao diện web sau khi container khởi động. Nó kiểm thử mọi điểm gắn kết và CLI (Docker socket, libvirt, restic, qemu-img, rclone) và báo cáo bất kỳ phần nào bị thiếu.

## Mô hình bảo mật

!!! warning "Quyền kiểm soát máy chủ tương đương root"
    Thông qua Docker socket, BombVault có thể dừng, xóa và tạo lại các container cũng như đọc/ghi appdata, và để sao lưu VM nó đăng nhập vào máy chủ qua SSH (`qemu+ssh://`, root theo mặc định) để chạy `virsh`. Bất kỳ ai truy cập được giao diện web của nó thực chất đều có quyền root trên máy chủ.

- **Bảo vệ mật khẩu tùy chọn** (Settings, Security): đặt một mật khẩu để yêu cầu đăng nhập, xóa nó để tắt. Mặc định tắt cho việc dùng trên LAN tin cậy. Các phiên được ký (HMAC dẫn xuất từ `APP_KEY`) và đổi mật khẩu sẽ vô hiệu hóa chúng; các lần đăng nhập bị giới hạn tần suất.
- Vì cổng bảo vệ là tùy chọn tham gia, khi chưa đặt thì toàn bộ giao diện và API (bao gồm thiết lập off-site, các tuyến kiểm tra can thiệp và bộ khôi phục) đều có thể truy cập bởi bất kỳ ai truy cập được cổng. Bật cổng bảo vệ một khi bạn dùng đến off-site, sao lưu bất biến hoặc mã hóa.
- Chỉ chạy BombVault trên một mạng tin cậy, không phơi ra ngoài. Để truy cập từ xa, đặt nó phía sau một reverse proxy có thêm xác thực và TLS. Các phản hồi mang theo các tiêu đề bảo mật cơ bản (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Với `HTTP_ONLY=true` cookie phiên mất cờ `Secure` của nó (bắt buộc phải vậy, để hoạt động qua HTTP thuần), nên chỉ bật mật khẩu phía sau một proxy kết thúc TLS nếu tính bảo mật là quan trọng.
- Kết nối SSH sao lưu VM tin cậy host key ở lần kết nối đầu tiên (TOFU) và ghim nó sau đó. Xác minh khóa của máy chủ ngoài luồng nếu đường dẫn container-tới-máy-chủ của bạn không tin cậy.
- Các bản sao lưu được mã hóa bởi restic khi bật mã hóa (Settings; mặc định bật), với khóa dẫn xuất từ `APP_KEY`.

## Sao lưu VM qua SSH

BombVault sao lưu các KVM/libvirt VM **mà không gắn kết bất kỳ đường dẫn libvirt nào**. Nó chạy `virsh` trên máy chủ qua SSH (`qemu+ssh://`), nên nó không bao giờ có thể ảnh hưởng đến VM Manager của máy chủ bạn.

Thiết lập nhanh:

1. **Settings, System, VM Backup over SSH:** sao chép khóa công khai được hiển thị.
2. Thêm nó vào `/root/.ssh/authorized_keys` của Unraid (cũng được lưu vào flash để nó tồn tại qua các lần khởi động lại).
3. Nhấp **Test connection**.

Template thêm `--add-host=host.docker.internal:host-gateway` để container có thể tiếp cận máy chủ. Đặt `LIBVIRT_HOST` thành IP LAN Unraid của bạn nếu tên đó không phân giải được (ví dụ khi container chạy trên một mạng `br0.x` tùy chỉnh). Nếu bạn đã đổi cổng SSH của Unraid, đặt `LIBVIRT_SSH_PORT` cho khớp. **Snapshot trực tiếp** ngoài ra cần qemu guest agent trong VM và đĩa nằm trên `/mnt/cache` (không phải `/mnt/user`).

!!! important "Hướng dẫn thiết lập VM và mạng đầy đủ"
    Hướng dẫn từng bước hoàn chỉnh (bật SSH, ủy quyền khóa lâu bền, định tuyến mạng tùy chỉnh và VLAN, phương thức theo từng VM và khắc phục sự cố phía máy chủ) nằm tại [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) trên GitHub.

## Thiết lập off-site

Thiết lập một bản sao off-site trên tab **Settings, Off-site**. Xem [Off-site & khôi phục](offsite-recovery.md) để biết quy trình đầy đủ (bất biến/append-only, kiểm tra can thiệp và diễn tập DR). Tóm lại:

- **Backend:** SMB/CIFS và NFS (gắn kết share và trỏ một Backup Path tới đó), các backend restic gốc không cần rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), hoặc bất kỳ remote rclone nào (`rclone:<remote>:<bucket>/path`).
- **Thông tin đăng nhập đám mây** được lưu mã hóa dưới Settings, Off-site, Cloud credentials.
- **Đích SSH không cần cài đặt gì ở phía bên kia.** `sftp:` chỉ cần một máy chủ SSH. Thêm khóa công khai từ **Settings, System, VM Backup over SSH** (cũng nằm tại `/config/ssh/id_ed25519.pub`) vào `~/.ssh/authorized_keys` của người dùng đích.
- **Bản sao off-site:** BombVault nhân bản các snapshot mới bằng `restic copy` theo kiểu nỗ lực tối đa. Kho cục bộ vẫn là chính. Mỗi miền có lịch trình off-site riêng, cùng với một nút **Replicate now**.
- **Nhiều đích off-site cho mỗi miền:** mỗi miền có thể nhân bản tới nhiều đích off-site cùng lúc. Thêm các đích bổ sung trên Settings, Off-site, mỗi đích có kho lưu trữ riêng, lớp lưu trữ S3, cờ append-only, lưu giữ và ngân sách tăng trưởng riêng; tất cả chúng nhân bản theo lịch trình off-site của miền đó. Một thiết lập off-site đơn hiện có được chuyển sang làm đích đầu tiên.
- **Lưu giữ theo từng nguồn:** chính sách cục bộ nằm trên Settings, Paths & Storage; chính sách off-site trên Settings, Off-site (để tất cả bằng 0 để không bao giờ tự động dọn bớt các snapshot off-site).
- **Giới hạn băng thông:** giới hạn tốc độ tải lên/tải xuống của restic dưới Settings, Off-site.
- **Lớp lưu trữ nguội và lưu trữ dài hạn (S3):** với một kho off-site S3 gốc, chọn một tầng có thể đọc để khôi phục (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Các remote rclone đặt lớp của chúng trong cấu hình rclone.

## Cài đặt di động (xuất và nhập) {#portable-settings-export-and-import}

Thẻ **Xuất và nhập cài đặt** trên trang Settings ghi toàn bộ cấu hình BombVault của bạn (cài đặt miền, đích off-site, lịch trình, lưu giữ, thông báo) ra một tệp JSON di động mà bạn có thể nhập trên một phiên bản khác, nên chuyển sang một máy mới hay nhân bản một thiết lập không có nghĩa là nhập lại mọi thứ bằng tay. Việc nhập hiển thị một bản xem trước và hỏi xác nhận, và nó không bao giờ đụng đến dữ liệu hay lịch sử sao lưu của bạn.

!!! warning "Bản xuất có thể chứa thông tin đăng nhập"
    Bạn chọn có bao gồm thông tin đăng nhập off-site và thông báo trong tệp hay không. Khi có kèm thông tin đăng nhập, bản xuất nhạy cảm như bộ khôi phục của bạn, nên hãy cất giữ nó ở nơi an toàn. Không có chúng, tệp chỉ chứa các cài đặt không bí mật.
