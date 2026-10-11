# Khắc phục sự cố

Một mục hỏi đáp ngắn. Để xem bảng khắc phục sự cố phía máy chủ đầy đủ cho VM-qua-SSH (permission-denied, xác minh host-key, thiếu biến template và hơn thế), xem [hướng dẫn Sao lưu VM qua SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) trên GitHub.

## Có gì đó chưa được kết nối đúng cách

Thêm thẻ **Tích hợp máy chủ** vào Tổng quan trong **Tùy chỉnh**. Kiểm tra tích hợp máy chủ kiểm tra mọi điểm gắn kết và CLI (Docker socket, libvirt, restic, qemu-img, rclone) và báo cáo bất kỳ phần nào bị thiếu, còn **Kiểm tra ngay** chạy lại nó. Hãy bắt đầu từ đây trước khi cho rằng có lỗi: một điểm gắn kết bị thiếu hay một máy chủ không tới được sẽ hiện ra ngay lập tức.

## Tôi không thể truy cập giao diện web

BombVault phục vụ HTTPS ngay từ đầu trên cổng `3443` (chứng chỉ tự ký), nên hãy mở `https://<your-unraid-ip>:3443`. Chấp nhận cảnh báo chứng chỉ tự ký, hoặc đặt BombVault phía sau một reverse proxy với chứng chỉ riêng của bạn. Nếu bạn chạy với `HTTP_ONLY=true`, nó phục vụ HTTP thuần trên cổng `3000` thay thế (dành cho việc dùng phía sau một proxy kết thúc TLS).

## Tôi đánh mất APP_KEY của mình

`APP_KEY` dẫn xuất mật khẩu kho lưu trữ restic. Không có nó, không có bộ khôi phục và không có bản gửi khóa trong nhóm của bạn, các bản sao lưu đã mã hóa không thể khôi phục được. Đó là lý do Tổng quan hiện **Lưu bộ khôi phục của bạn** cho đến khi bạn tải nó về. Xem [Off-site & khôi phục](offsite-recovery.md#recovery-kit). Hãy tạo một khóa bằng `openssl rand -hex 32` và cất nó bên ngoài máy chủ trước khi bạn tin cậy bất kỳ bản sao lưu nào.

## Tôi quên mật khẩu đăng nhập

Chọn **Quên mật khẩu?** trên trang đăng nhập. BombVault ghi một mã gồm 12 ký tự vào nhật ký container của nó (trên Unraid: tab Docker, biểu tượng BombVault, **Logs**). Hãy nhập mã đó cùng một mật khẩu mới trong vòng 10 phút. Việc đặt lại cũng tắt xác thực hai yếu tố, gỡ mọi khóa truy cập, thu hồi mọi mã thông báo API, khóa MCP và phiên đăng nhập OAuth, kết thúc mọi phiên khác và tắt việc ghép nối điện thoại. Hãy thêm lại khóa truy cập trong **Cài đặt, Bảo mật**, bật lại việc ghép nối điện thoại ở đó, rồi tạo mã thông báo và khóa mới ở nơi bạn vẫn cần. Xem [Mô hình bảo mật](configuration.md#login).

## Sao lưu dừng lại và báo không thể mở khóa nữa

Máy chủ này đã tiếp nhận khóa sao lưu của một máy chủ khác trong Trình hướng dẫn khẩn cấp, và sau đó `APP_KEY` bị đổi. Khóa được tiếp nhận được niêm phong bằng APP_KEY cũ, nên BombVault không mở được nó và không sao lưu gì thay vì ghi bằng khóa sai. **Cần chú ý** trên Tổng quan nêu tên nó. Hãy đặt APP_KEY về giá trị cũ, hoặc tải lại bộ khôi phục trên trang Khôi phục trong **Khẩn cấp**.

## Điện thoại bị đưa tới trang đăng nhập

Một điện thoại đã ghép nối chỉ nhận phiên qua nhóm khi **Điện thoại có thể ghép nối bằng mười hai từ của nhóm** đang bật trong **Cài đặt, Bảo mật**. Việc đặt lại mật khẩu sẽ tắt nó. Một ứng dụng cũ hơn máy chủ cũng không nhận được phiên cho đến khi được cập nhật. Máy chủ được thêm bằng địa chỉ luôn hỏi mật khẩu.

## Sao lưu VM không kết nối được

Sao lưu VM kết nối libvirt qua SSH, không bao giờ qua một điểm gắn kết.

- Xác nhận SSH được bật trên máy chủ và khóa công khai của BombVault được ủy quyền trong `/root/.ssh/authorized_keys` (Cài đặt, Tích hợp, SSH máy chủ hiển thị khóa và một nút **Kiểm tra kết nối**).
- Trên một mạng `br0.x` tùy chỉnh, đặt `LIBVIRT_HOST` thành IP LAN Unraid của bạn (container không thể tiếp cận máy chủ qua `host.docker.internal` ở đó). Bật **Settings, Docker, Host access to custom networks**.
- Nếu bạn đã đổi cổng SSH của Unraid, đặt `LIBVIRT_SSH_PORT` cho khớp.
- Chẩn đoán từng bước đầy đủ (kiểm tra khả năng tiếp cận, định tuyến VLAN, `Permission denied (publickey)`, `Host key verification failed`) nằm trong [hướng dẫn Sao lưu VM qua SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Một snapshot VM trực tiếp đã không chạy

Snapshot trực tiếp cần qemu guest agent được cài đặt trong VM và đĩa nằm trên `/mnt/cache` (hoặc `/mnt/diskX`), không phải `/mnt/user`. Trên một VM đã tắt, trực tiếp tự động rơi về êm ái. Một lần sao lưu êm ái tắt VM xuống, sao lưu các đĩa, rồi khởi động lại nó, nên nó luôn nhất quán.

## Một lần sao lưu thất bại với "repository is already locked"

Đây thường là một khóa restic mồ côi còn sót lại khi container được cập nhật hoặc khởi động lại giữa chừng một thao tác. BombVault phát hiện khóa mồ côi đã được chứng minh, tự động xóa cưỡng bức và thử lại một lần. Nếu vẫn còn, hãy mở thẻ **Kiểm tra bản sao lưu** trong **Cài đặt, Xác minh** và chọn **Mở khóa** dưới **Thêm** trong hàng của phần bị ảnh hưởng để tự tay xóa khóa cũ. Một sự cố thật vẫn sẽ hiện ra chứ không bị che giấu. Sau khi khởi động lại, BombVault chờ cho đến khi khóa như vậy đã mười phút không được làm mới. Một restic vẫn đang chạy, ví dụ trong một BombVault thứ hai trên cùng kho lưu trữ, làm mới khóa của nó mỗi năm phút.

## Bản sao off-site của tôi đã không diễn ra sau một lần sao lưu

Nhân bản off-site vốn được thiết kế theo kiểu hết sức có thể, nên một trục trặc ở off-site không bao giờ làm hỏng bản sao lưu cục bộ. Hãy kiểm tra khi nào đích sao lưu sao chép: thẻ **Sao chép** của nó ghi **Sau mỗi bản sao lưu** hoặc nêu một kế hoạch riêng, và **Cài đặt, Lịch trình, Bản sao ngoài site theo phần** có thể cho một phần kế hoạch riêng. Hãy dùng **Sao chép ngay** trong thẻ **Bản sao ngoài site** của phần đó dưới **Cài đặt, Đích sao lưu** để chạy theo yêu cầu, và theo dõi phần đầu trạng thái của Tổng quan, nơi ghi **Đang sao chép ra ngoài site** khi đang chạy. Một bản sao thất bại hiện dưới **Cần chú ý**.

## Một lần khôi phục đã hủy trước khi nó bắt đầu

Trước khi bất cứ thứ gì bị dừng hoặc xóa, việc khôi phục chạy một lần kiểm tra xung đột trước khi chạy: nó xác minh rằng IP tĩnh của container và các cổng máy chủ được công bố đều còn trống. Nếu một container khác đã giữ một trong số đó, nó hủy bỏ với một thông báo rõ ràng, khả thi thay vì để lại một lần khôi phục dở dang. Giải phóng cổng hoặc IP xung đột, rồi thử lại.

## Một bản xuất thô đã thất bại thay vì ghi một tệp

Nếu **Mã hóa bản xuất bằng age** đang bật trong **Cài đặt, Hệ thống** nhưng chưa đặt người nhận hợp lệ nào, một lần xuất sẽ thất bại với lỗi rõ ràng thay vì ghi văn bản thuần. Hãy thêm một người nhận hợp lệ (khóa công khai age hoặc khóa công khai SSH), hoặc tắt mã hóa nếu bạn muốn bản xuất là văn bản thuần. Xem [Tính năng](features.md).

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
- **`ssh-auth`**: máy chủ đã từ chối khóa của BombVault. **Kết nối ZFS tới máy chủ này**, mở bằng **Xem kết nối** trên trang Bản sao lưu, hiện lệnh cấp quyền cho nó; hãy chạy lệnh đó một lần trên máy chủ.

## Một mục đứng yên ở "Đang học N/10"

Hầu hết các kiểm tra bất thường bắt đầu sau 10 lần sao lưu thành công của một mục, và bộ đếm bắt đầu lại sau **Đánh dấu là đã lường trước** và sau khi lựa chọn của mục thay đổi. Một mục không có lịch thì không học, và một container không có appdata thì không có gì để học, điều mà huy hiệu của nó cho biết. **Tiến trình học** trên trang Bất thường hiện một vòng mười phần cho mỗi mục.

## Chính sách lưu giữ không còn xóa bản sao lưu cũ của một mục

Một bất thường nghiêm trọng đang mở đang giữ chúng: nguồn của mục gần như trống, co lại mạnh, hoặc một bản sao lưu lại lưu phần lớn dữ liệu của nó. Hãy mở bất thường từ huy hiệu trên mục. Nếu dữ liệu bị thiếu hoặc đã bị mã hóa, trước tiên hãy khôi phục từ bản sao lưu tốt gần nhất được liên kết. Sau đó xác nhận bất thường, hoặc đánh dấu là đã lường trước nếu thay đổi là của bạn, và lần chạy kế tiếp sẽ dọn như thường. Bản xem trước lưu giữ đánh dấu một mục như vậy là được giữ lại. Với một mục ZFS, chỉ tập dữ liệu được nêu trong bất thường giữ các bản sao lưu cũ, còn các tập dữ liệu khác của cây được dọn như thường.

## Dọn dẹp thủ công báo rằng một số mục đã được giữ lại

Cùng nguyên nhân: dọn dẹp để yên các bản sao lưu cũ của một mục có bất thường như vậy và nêu tên mục trong thông báo của nó. Mọi thứ khác được dọn như thường.

## Nhập lịch sử báo rằng không đọc được một kho lưu trữ

Sau khi nâng cấp, BombVault đọc dung lượng của các bản sao lưu trước đó từ mỗi kho lưu trữ một lần. Một kho mà lúc đó nó không tới được, chẳng hạn một đích off-site đang tắt hoặc một share chưa được gắn kết, được đếm trong thẻ **Bất thường** dưới **Cài đặt, Xác minh** và được thử lại mỗi ngày một lần. Trong lúc đó các mục của nó học từ những bản sao lưu mới.

## Cảnh báo dung lượng đĩa không khớp với bảng điều khiển Unraid

Trên chia sẻ người dùng của Unraid (`/mnt/user`), dung lượng trống là của cả mảng, không phải của một đĩa. Kho từ xa chỉ được đo qua các remote rclone có báo dung lượng trống; kho S3, B2, REST và SFTP không có số liệu và được liệt kê là chưa đo trong thẻ **Bất thường**.

## Trợ lý AI không kết nối được

Trang [Máy chủ MCP](mcp.md#troubleshooting) giải thích ý nghĩa của từng mã trạng thái và từng lần từ chối của điểm kết nối MCP, cùng cách xử lý.

## Container cứ khởi động lại hoặc trông không khỏe mạnh

BombVault báo cáo khỏe/không khỏe từ `/api/health` của chính nó. Một công cụ tự chữa (như Autoheal) có thể tự khởi động lại nó nếu bộ máy bị treo. Hãy kiểm tra nhật ký container và thẻ **Tích hợp máy chủ** để tìm nguyên nhân gốc.

## Vẫn bế tắc?

- Đọc đầy đủ các trang [Cấu hình](configuration.md) và [Off-site & khôi phục](offsite-recovery.md).
- Hỏi trên [luồng hỗ trợ Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Mở một [vấn đề GitHub](https://github.com/junkerderprovinz/bombvault/issues).
