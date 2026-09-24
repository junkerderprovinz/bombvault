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

## Một bản kết xuất cơ sở dữ liệu đã thất bại

Bản kết xuất thất bại không bao giờ làm hỏng bản sao lưu bao quanh nó; nó được ghi lại như một lần chạy thất bại của riêng mình, và lý do cho biết phải sửa gì.

- **Bị từ chối đăng nhập.** Bản kết xuất đăng nhập bằng chính các biến mật khẩu của container (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` hoặc bản `_FILE` của chúng). Hãy kiểm tra chúng trên container cơ sở dữ liệu. Một biến `_FILE` trỏ tới bí mật mà người dùng của chính container không đọc được cũng thất bại y như vậy.
- **Thiếu quyền.** Với mật khẩu root ngẫu nhiên, bản kết xuất chỉ đăng nhập được với tư cách người dùng ứng dụng nên chỉ chứa đúng cơ sở dữ liệu đó, còn MySQL 8.4 trở lên có thể từ chối hẳn. Hãy đặt cho container một mật khẩu root thật, hoặc tắt kết xuất của nó.
- **Các bảng hệ thống cần nâng cấp.** MariaDB từ chối kết xuất khi bảng hệ thống của nó đến từ phiên bản cũ hơn (lỗi 1558). Thêm biến `MARIADB_AUTO_UPGRADE=1` rồi khởi động lại container, hoặc chạy `mariadb-upgrade` một lần bên trong.
- **Không có công cụ kết xuất.** Một image rút gọn hay tự dựng mà thiếu `pg_dump`, `mysqldump` hoặc `mariadb-dump` thì không kết xuất được. Hãy dùng image chính thức, hoặc tắt kết xuất.
- **Giới hạn thời gian.** Một bản kết xuất có `DB_DUMP_MAX_HOURS` (mặc định 6), bản sao lưu bao quanh có `BACKUP_MAX_HOURS`, còn bản kết xuất ngừng tiến triển sẽ bị cắt sau `BACKUP_STALL_HOURS`. Trường hợp cuối thường do một khóa mà ứng dụng đang giữ. Hãy nâng giới hạn đã kích hoạt, hoặc kết xuất lúc ứng dụng rảnh rỗi.
- **Container đang tạm dừng hoặc đang khởi động lại.** Bản kết xuất nói chuyện với máy chủ đang chạy. Nếu container cứ khởi động lại, nhật ký của chính nó sẽ nói vì sao.
- **Không xóa được một bản kết xuất hỏng.** Bản kết xuất mà BombVault không hoàn tất được sẽ bị xóa đi. Khi lần xóa đó thất bại, bản kết xuất vẫn nằm trong danh sách với dấu hỏng, và bạn có thể xóa nó ở đó.

## Một lần nhập đã thất bại

Việc nhập sẽ dừng container, dời thư mục dữ liệu của nó sang một bên và để image tạo một thư mục trống vào chỗ đó. Nếu một bước trước khi nhập thất bại, thư mục cũ được đặt trở lại một cách tự động. Nếu chính việc nhập thất bại, container giữ thư mục mới còn thư mục cũ nằm bên cạnh với tên `<thư mục dữ liệu>.bombvault-before-import-<dấu thời gian>`; thông báo lỗi của lần chạy nêu đúng đường dẫn.

Để đặt lại bằng tay: dừng container, đổi tên thư mục dữ liệu hiện tại cho khuất lối, đổi tên thư mục được giữ về tên gốc, rồi khởi động container. Trên Unraid, trình quản lý tệp ở thẻ Shares làm được việc này.

## Sao lưu tập dữ liệu ZFS thất bại hoặc bỏ qua một tập dữ liệu {#zfs-datasets}

Mỗi sự cố có một mã lý do trong ngoặc vuông, và trang [Tập dữ liệu ZFS](zfs-datasets.md#reason-codes) liệt kê tất cả cùng cách khắc phục. Ba trường hợp hay gặp nhất:

- **`snapshot-loop`**: ảnh chụp không tới được BombVault vì Host Data không chuyển tiếp các lần gắn mới. Sửa container, đặt Access Mode của Host Data thành Read/Write - Slave rồi khởi động lại BombVault.
- **`key-not-loaded`**: tập dữ liệu mã hóa chưa nạp khóa sẽ bị bỏ qua. Nạp khóa bằng `zfs load-key` và gắn tập dữ liệu; lần sao lưu sau sẽ gồm nó.
- **`ssh-auth`**: máy chủ từ chối khóa của BombVault. Thẻ kết nối trên trang ZFS hiện lệnh cấp quyền cho khóa; chạy lệnh đó một lần trên máy chủ.

## Container cứ khởi động lại hoặc trông không khỏe mạnh

BombVault báo khỏe mạnh/không khỏe mạnh từ `/api/health` của chính nó. Một công cụ tự phục hồi (chẳng hạn Autoheal) có thể khởi động lại nó tự động nếu công cụ có bao giờ bị kẹt. Kiểm tra nhật ký container và báo cáo `/spike` để tìm nguyên nhân cơ bản.

## Vẫn bế tắc?

- Đọc đầy đủ các trang [Cấu hình](configuration.md) và [Off-site & khôi phục](offsite-recovery.md).
- Hỏi trên [luồng hỗ trợ Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Mở một [vấn đề GitHub](https://github.com/junkerderprovinz/bombvault/issues).
