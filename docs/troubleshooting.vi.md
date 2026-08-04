# Khắc phục sự cố

Một mục hỏi đáp ngắn. Để xem bảng khắc phục sự cố phía máy chủ đầy đủ cho VM-qua-SSH (permission-denied, xác minh host-key, thiếu biến template và hơn thế), xem [hướng dẫn Sao lưu VM qua SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) trên GitHub.

## Có gì đó chưa được kết nối đúng cách

Mở `/spike` trong giao diện web. Kiểm tra tích hợp máy chủ kiểm thử mọi điểm gắn kết và CLI (Docker socket, libvirt, restic, qemu-img, rclone) và báo cáo bất kỳ phần nào bị thiếu. Bắt đầu ở đây trước khi cho rằng có một lỗi: một điểm gắn kết bị thiếu hoặc một máy chủ không tiếp cận được sẽ hiện ra ngay lập tức.

## Tôi không thể truy cập giao diện web

BombVault phục vụ HTTPS ngay từ đầu trên cổng `3443` (chứng chỉ tự ký), nên hãy mở `https://<your-unraid-ip>:3443`. Chấp nhận cảnh báo chứng chỉ tự ký, hoặc đặt BombVault phía sau một reverse proxy với chứng chỉ riêng của bạn. Nếu bạn chạy với `HTTP_ONLY=true`, nó phục vụ HTTP thuần trên cổng `3000` thay thế (dành cho việc dùng phía sau một proxy kết thúc TLS).

## Tôi đánh mất APP_KEY của mình

`APP_KEY` dẫn xuất mật khẩu kho lưu trữ restic. Không có nó (và không có bộ khôi phục khóa mã hóa), các bản sao lưu đã mã hóa không thể khôi phục được. Đây là lý do bảng điều khiển nhắc nhở bạn tải xuống bộ khôi phục. Xem [Off-site & khôi phục](offsite-recovery.md). Tạo một khóa bằng `openssl rand -hex 32` và cất giữ nó ngoài máy chủ trước khi bạn tin cậy bất kỳ bản sao lưu nào.

## Sao lưu VM không kết nối được

Sao lưu VM kết nối libvirt qua SSH, không bao giờ qua một điểm gắn kết.

- Xác nhận SSH được bật trên máy chủ và khóa công khai của BombVault được ủy quyền trong `/root/.ssh/authorized_keys` (Settings, System, VM Backup over SSH hiển thị khóa và một nút **Test connection**).
- Trên một mạng `br0.x` tùy chỉnh, đặt `LIBVIRT_HOST` thành IP LAN Unraid của bạn (container không thể tiếp cận máy chủ qua `host.docker.internal` ở đó). Bật **Settings, Docker, Host access to custom networks**.
- Nếu bạn đã đổi cổng SSH của Unraid, đặt `LIBVIRT_SSH_PORT` cho khớp.
- Chẩn đoán từng bước đầy đủ (kiểm tra khả năng tiếp cận, định tuyến VLAN, `Permission denied (publickey)`, `Host key verification failed`) nằm trong [hướng dẫn Sao lưu VM qua SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Một snapshot VM trực tiếp đã không chạy

Snapshot trực tiếp cần qemu guest agent được cài đặt trong VM và đĩa nằm trên `/mnt/cache` (hoặc `/mnt/diskX`), không phải `/mnt/user`. Trên một VM đã tắt, trực tiếp tự động rơi về êm ái. Một lần sao lưu êm ái tắt VM xuống, sao lưu các đĩa, rồi khởi động lại nó, nên nó luôn nhất quán.

## Một lần sao lưu thất bại với "repository is already locked"

Đây thường là một khóa restic mồ côi bị bỏ lại khi container được cập nhật hoặc khởi động lại giữa chừng thao tác. BombVault phát hiện một khóa mồ côi được chứng minh, xóa cưỡng bức nó và thử lại một lần, tự động. Nếu nó cứ dai dẳng, dùng **Settings, Integrity & maintenance, Unlock** cho miền bị ảnh hưởng để xóa một khóa bị kẹt bằng tay. Một vấn đề thực sự vẫn hiện ra thay vì bị ẩn đi.

## Bản sao off-site của tôi đã không diễn ra sau một lần sao lưu

Nhân bản off-site theo thiết kế là nỗ lực tối đa, nên một trục trặc off-site không bao giờ làm thất bại bản sao lưu cục bộ. Kiểm tra lịch trình off-site cho miền đó (Settings, Schedules): một lịch trình trống sẽ nhân bản sau mỗi lần sao lưu cục bộ, trong khi một nhịp độ sẽ gửi ít thường xuyên hơn. Dùng **Replicate now** trên tab Off-site cho một lần chạy theo yêu cầu, và theo dõi chỉ báo nhân bản trên bảng điều khiển.

## Một lần khôi phục đã hủy trước khi nó bắt đầu

Trước khi bất cứ thứ gì bị dừng hoặc xóa, việc khôi phục chạy một lần kiểm tra xung đột trước khi chạy: nó xác minh rằng IP tĩnh của container và các cổng máy chủ được công bố đều còn trống. Nếu một container khác đã giữ một trong số đó, nó hủy bỏ với một thông báo rõ ràng, khả thi thay vì để lại một lần khôi phục dở dang. Giải phóng cổng hoặc IP xung đột, rồi thử lại.

## Một bản xuất thô đã thất bại thay vì ghi một tệp

Nếu bật mã hóa age (Settings) nhưng không đặt người nhận hợp lệ nào, một bản xuất sẽ thất bại với một lỗi rõ ràng thay vì ghi văn bản thô. Thêm một người nhận hợp lệ (một khóa công khai age hoặc một khóa công khai SSH), hoặc tắt mã hóa nếu bạn có ý định bản xuất là văn bản thô. Xem [Tính năng](features.md).

## Container cứ khởi động lại hoặc trông không khỏe mạnh

BombVault báo khỏe mạnh/không khỏe mạnh từ `/api/health` của chính nó. Một công cụ tự phục hồi (chẳng hạn Autoheal) có thể khởi động lại nó tự động nếu công cụ có bao giờ bị kẹt. Kiểm tra nhật ký container và báo cáo `/spike` để tìm nguyên nhân cơ bản.

## Vẫn bế tắc?

- Đọc đầy đủ các trang [Cấu hình](configuration.md) và [Off-site & khôi phục](offsite-recovery.md).
- Hỏi trên [luồng hỗ trợ Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Mở một [vấn đề GitHub](https://github.com/junkerderprovinz/bombvault/issues).
