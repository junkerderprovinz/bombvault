# Khắc phục sự cố

Một mục hỏi đáp ngắn. Để xem bảng khắc phục sự cố phía máy chủ đầy đủ cho VM-qua-SSH (permission-denied, xác minh host-key, thiếu biến template và hơn thế), xem [hướng dẫn Sao lưu VM qua SSH](vm-backup-ssh-setup.md).

## Có gì đó chưa được kết nối đúng cách {#something-is-not-wired-up-correctly}

Thêm thẻ **Tích hợp máy chủ** vào Tổng quan trong **Tùy chỉnh**. Kiểm tra tích hợp máy chủ kiểm tra mọi điểm gắn kết và CLI (Docker socket, libvirt, restic, qemu-img, rclone) và báo cáo bất kỳ phần nào bị thiếu, còn **Kiểm tra ngay** chạy lại nó. Hãy bắt đầu từ đây trước khi cho rằng có lỗi: một điểm gắn kết bị thiếu hay một máy chủ không tới được sẽ hiện ra ngay lập tức.

## Tôi không thể truy cập giao diện web {#i-cannot-reach-the-web-ui}

BombVault phục vụ HTTPS ngay từ đầu trên cổng `3443` (chứng chỉ tự ký), nên hãy mở `https://<your-unraid-ip>:3443`. Chấp nhận cảnh báo chứng chỉ tự ký, hoặc đặt BombVault phía sau một reverse proxy với chứng chỉ riêng của bạn. Nếu bạn chạy với `HTTP_ONLY=true`, nó phục vụ HTTP thuần trên cổng `3000` thay thế (dành cho việc dùng phía sau một proxy kết thúc TLS).

## Tôi đánh mất APP_KEY của mình {#i-lost-my-app_key}

`APP_KEY` dẫn xuất mật khẩu kho lưu trữ restic. Không có nó, không có bộ khôi phục và không có bản gửi khóa trong nhóm của bạn, các bản sao lưu đã mã hóa không thể khôi phục được. Đó là lý do Tổng quan hiện **Lưu bộ khôi phục của bạn** cho đến khi bạn tải nó về. Xem [Off-site & khôi phục](offsite-recovery.md#recovery-kit). Hãy tạo một khóa bằng `openssl rand -hex 32` và cất nó bên ngoài máy chủ trước khi bạn tin cậy bất kỳ bản sao lưu nào.

## Tôi quên mật khẩu đăng nhập {#i-forgot-the-login-password}

Chọn **Quên mật khẩu?** trên trang đăng nhập. BombVault ghi một mã gồm 12 ký tự vào nhật ký container của nó (trên Unraid: tab Docker, biểu tượng BombVault, **Logs**). Hãy nhập mã đó cùng một mật khẩu mới trong vòng 10 phút. Việc đặt lại cũng tắt xác thực hai yếu tố, gỡ mọi khóa truy cập, thu hồi mọi mã thông báo API, khóa MCP và phiên đăng nhập OAuth, kết thúc mọi phiên khác và tắt việc ghép nối điện thoại. Hãy thêm lại khóa truy cập trong **Cài đặt, Bảo mật**, bật lại việc ghép nối điện thoại ở đó, rồi tạo mã thông báo và khóa mới ở nơi bạn vẫn cần. Xem [Mô hình bảo mật](configuration.md#login).

## Sao lưu dừng lại và báo không thể mở khóa nữa {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Máy chủ này đã tiếp nhận khóa sao lưu của một máy chủ khác trong Trình hướng dẫn khẩn cấp, và sau đó `APP_KEY` bị đổi. Khóa được tiếp nhận được niêm phong bằng APP_KEY cũ, nên BombVault không mở được nó và không sao lưu gì thay vì ghi bằng khóa sai. **Cần chú ý** trên Tổng quan nêu tên nó. Hãy đặt APP_KEY về giá trị cũ, hoặc tải lại bộ khôi phục trên trang Khôi phục trong **Khẩn cấp**.

## Điện thoại bị đưa tới trang đăng nhập {#a-phone-lands-on-the-login-page}

Một điện thoại đã ghép nối chỉ nhận phiên qua nhóm khi **Điện thoại có thể ghép nối bằng mười hai từ của nhóm** đang bật trong **Cài đặt, Bảo mật**. Việc đặt lại mật khẩu sẽ tắt nó. Một ứng dụng cũ hơn máy chủ cũng không nhận được phiên cho đến khi được cập nhật. Máy chủ được thêm bằng địa chỉ luôn hỏi mật khẩu. Một điện thoại đã bị xóa khỏi trang Phiên bản sẽ quay lại ở trạng thái tắt khi ứng dụng của nó mở lại. Để cho nó vào, hãy bật **Truy vấn phiên bản này** trong **Giám sát** ở phần chi tiết của nó.

## Sao lưu VM không kết nối được {#vm-backup-will-not-connect}

Sao lưu VM kết nối libvirt qua SSH, không bao giờ qua một điểm gắn kết.

- Xác nhận SSH được bật trên máy chủ và khóa công khai của BombVault được ủy quyền trong `/root/.ssh/authorized_keys` (Cài đặt, Tích hợp, SSH máy chủ hiển thị khóa và một nút **Kiểm tra kết nối**).
- Trên một mạng `br0.x` tùy chỉnh, đặt `LIBVIRT_HOST` thành IP LAN Unraid của bạn (container không thể tiếp cận máy chủ qua `host.docker.internal` ở đó). Bật **Settings, Docker, Host access to custom networks**.
- Nếu bạn đã đổi cổng SSH của Unraid, đặt `LIBVIRT_SSH_PORT` cho khớp.
- Trên TrueNAS, đặt `LIBVIRT_URI` với socket libvirt riêng của TrueNAS, cấp capability `SETPCAP` cho ứng dụng và thêm khóa cho root trong **Credentials, Users** (xem [TrueNAS SCALE](getting-started.md#truenas)).
- Chẩn đoán từng bước đầy đủ (kiểm tra khả năng tiếp cận, định tuyến VLAN, `Permission denied (publickey)`, `Host key verification failed`) nằm trong [hướng dẫn Sao lưu VM qua SSH](vm-backup-ssh-setup.md).

## Một snapshot VM trực tiếp đã không chạy {#a-live-vm-snapshot-did-not-run}

Snapshot trực tiếp cần qemu guest agent được cài đặt trong VM và đĩa nằm trên `/mnt/cache` (hoặc `/mnt/diskX`), không phải `/mnt/user`. Trên một VM đã tắt, trực tiếp tự động rơi về êm ái. Một lần sao lưu êm ái tắt VM xuống, sao lưu các đĩa, rồi khởi động lại nó, nên nó luôn nhất quán. Nó chờ tối đa mười phút để máy khách tắt và không bao giờ buộc tắt máy khách, nên một máy khách Windows đang cài bản cập nhật không bị cắt ngang giữa chừng. Một máy khách vẫn còn chạy sau thời gian đó sẽ được để yên, và lần sao lưu đó thất bại kèm lý do.

## Một lần sao lưu thất bại với "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Đây thường là một khóa restic mồ côi còn sót lại khi container được cập nhật hoặc khởi động lại giữa chừng một thao tác. BombVault phát hiện khóa mồ côi đã được chứng minh, tự động xóa cưỡng bức và thử lại một lần. Nếu vẫn còn, hãy mở thẻ **Kiểm tra bản sao lưu** trong **Cài đặt, Xác minh** và chọn **Mở khóa** dưới **Thêm** trong hàng của phần bị ảnh hưởng để tự tay xóa khóa cũ. Một sự cố thật vẫn sẽ hiện ra chứ không bị che giấu. Sau khi khởi động lại, BombVault chờ cho đến khi khóa như vậy đã mười phút không được làm mới. Một restic vẫn đang chạy, ví dụ trong một BombVault thứ hai trên cùng kho lưu trữ, làm mới khóa của nó mỗi năm phút.

## Bản sao off-site của tôi đã không diễn ra sau một lần sao lưu {#my-off-site-copy-did-not-happen-after-a-backup}

Nhân bản off-site vốn được thiết kế theo kiểu hết sức có thể, nên một trục trặc ở off-site không bao giờ làm hỏng bản sao lưu cục bộ. Hãy kiểm tra khi nào đích sao lưu sao chép: thẻ **Sao chép** của nó ghi **Sau mỗi bản sao lưu** hoặc nêu một kế hoạch riêng, và **Cài đặt, Lịch trình, Bản sao ngoài site theo phần** có thể cho một phần kế hoạch riêng. Hãy dùng **Sao chép ngay** trong dòng của phần đó ở thẻ **Bản sao ngoài site** dưới **Cài đặt, Đích sao lưu** để chạy theo yêu cầu, và theo dõi phần đầu trạng thái của Tổng quan, nơi ghi **Đang sao chép ra ngoài site** khi đang chạy. Một bản sao thất bại hiện dưới **Cần chú ý**.

## Chứng chỉ của rest-server không được tin cậy {#a-rest-servers-certificate-is-not-trusted}

restic từ chối một chứng chỉ mà nó không biết, và thông báo lỗi cho biết nên đặt chứng chỉ ở đâu. Sao chép chứng chỉ tự ký của rest-server, hoặc chứng chỉ của tổ chức đã ký nó, dưới dạng tệp PEM vào thư mục `certs` trong thư mục dữ liệu của BombVault (`/config/certs` trong container, `appdata/bombvault/certs` trên Unraid) rồi kiểm tra lại. Không cần khởi động lại.

## Bản sao lưu báo rằng dữ liệu ngoài Host Data chưa được sao lưu {#data-outside-host-data}

Container giữ dữ liệu trong một thư mục mà BombVault không đọc được qua Host Data. Bản sao lưu lưu định nghĩa và mọi thư mục nó với tới, tô cam dòng của nó trong nhật ký hoạt động và nêu tên các thư mục đã bỏ qua. Có hai trường hợp thường gặp: một ixVolume trên TrueNAS dưới `/mnt/.ix-apps`, và một volume Docker có tên dưới `/var/lib/docker/volumes`, chẳng hạn volume cơ sở dữ liệu của một stack compose. Thẻ **Thư mục sao lưu** trên tab **Cài đặt** của container hiển thị từng thư mục như vậy với đường dẫn bên trong container và đường dẫn trên host, được đánh dấu màu đỏ.

Volume Docker ẩn danh không bị báo. Docker tự tạo nó cho một thư mục do image khai báo nhưng chưa ai ánh xạ, đặt tên bằng 64 chữ số thập lục phân và giữ nó dưới `/var/lib/docker/volumes`. Bên trong thường là bộ nhớ đệm, nên bản sao lưu vẫn xanh và thẻ chỉ liệt kê nó kèm ghi chú rằng nó ẩn danh.

- **Bạn muốn sao lưu dữ liệu đó.** Ánh xạ thư mục tới một đường dẫn trên host bên dưới Host Data, ví dụ một thư mục trong share appdata của bạn, và lần sao lưu tiếp theo sẽ bao gồm nó.
- **Bạn không cần nó.** Thêm một trong hai đường dẫn của nó thành một dòng trong thẻ **Mẫu loại trừ** trên cùng tab: đường dẫn bên trong container, như `/cache`, hoặc đường dẫn trên host mà cảnh báo nêu, như `/var/lib/docker/volumes/jellyfin_cache/_data`. Từ lần sao lưu tiếp theo, cảnh báo sẽ bỏ thư mục đó ra, và bản xem trước bên dưới ô nhập cho biết lý do. Một dòng cho thư mục cha cũng có tác dụng, một tên đơn giản hoặc `/` thì không.

## Một bản sao lưu báo Đã hoãn {#deferred-backups}

Ứng dụng của container đang bận, nên bản sao lưu của nó chờ và được tự động thử lại. Dòng trong nhật ký hoạt động cho biết lý do và khi nào có lần thử tiếp theo. Thẻ **Khi ứng dụng đang bận** trên tab **Cài đặt** của container cũng cho biết như vậy và khi nào việc chờ kết thúc. Bạn không cần làm gì. Để sao lưu ngay, hãy dùng **Sao lưu ngay**: một lần sao lưu do bạn tự bắt đầu không bao giờ chờ, kể cả với mã thoát 75.

- **Kiểm tra của riêng bạn.** Cho **Lệnh trước sao lưu** thoát với mã 75 khi ứng dụng đang bận, ví dụ `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Mọi mã khác không phải 0 vẫn làm bản sao lưu thất bại như trước, nên một script hỏng vẫn hiện dưới **Cần chú ý**.
- **Plex, Jellyfin hoặc Emby không bao giờ được hỏi.** BombVault trước tiên hỏi máy chủ ở chính địa chỉ của container và, khi ở đó không có trả lời, chạy curl hoặc wget bên trong container. Một image không có cả hai thì không thể được hỏi thông qua chính nó. Khi đó nhật ký sẽ nêu điều này. Khi bật **Sao lưu khi máy chủ không trả lời**, bản sao lưu như vậy vẫn chạy, khi tắt thì nó chờ.
- **Jellyfin hoặc Emby từ chối.** Kiểm tra **Khóa API**. Khóa được tạo trong bảng điều khiển của máy chủ, ở mục API keys.
- **Plex từ chối.** BombVault đọc token từ `Preferences.xml` trong thư mục được gắn vào `/config`. Một máy chủ chưa từng được đăng nhập thì không có token và chỉ trả lời ở nơi nó cho phép truy cập không cần token.
- **Một bộ phim tạm dừng giữ bản sao lưu chờ cả đêm.** Đặt **Bỏ qua phiên tạm dừng sau (phút)**, ví dụ 30.
- **Kiểm tra lưu lượng không bao giờ cho bản sao lưu chạy.** Nâng **Bận từ (KiB/s)** lên trên mức container gửi khi không ai dùng nó. Ngay sau khi BombVault khởi động, kiểm tra sẽ chờ cho đến khi đã theo dõi container đủ số phút trong **Yên trong (phút)**.
- **Bản sao lưu không bao giờ chạy vào ban đêm.** Khi đã đặt khoảng thời gian, các lần thử chỉ diễn ra trong đó. Một lần chờ chạm hạn chót bên ngoài khoảng đó vẫn kết thúc đúng hạn chót.

## Một bản sao lưu lớn bị dừng vì hết bộ nhớ {#a-large-backup-was-killed-for-running-out-of-memory}

Đặt **Giới hạn bộ nhớ mỗi bản sao lưu (MiB, 0 = không)** trong thẻ **Hiệu năng** dưới **Cài đặt, Hệ thống** ở mức thấp hơn hẳn lượng bộ nhớ mà container được phép dùng. Khi gần tới giới hạn, restic giải phóng bộ nhớ thường xuyên hơn, việc này làm chậm đi một chút, nên bản sao lưu chạy chậm lại thay vì bị dừng.

## Một lần khôi phục đã hủy trước khi nó bắt đầu {#a-restore-aborted-before-it-started}

Trước khi bất cứ thứ gì bị dừng hoặc xóa, việc khôi phục chạy một lần kiểm tra xung đột trước khi chạy: nó xác minh rằng IP tĩnh của container và các cổng máy chủ được công bố đều còn trống. Nếu một container khác đã giữ một trong số đó, nó hủy bỏ với một thông báo rõ ràng, khả thi thay vì để lại một lần khôi phục dở dang. Giải phóng cổng hoặc IP xung đột, rồi thử lại.

Nếu mạng của container không tồn tại, chẳng hạn vì ứng dụng TrueNAS của nó đã bị xóa, kiểm tra trước khi khôi phục nêu tên mạng đó và việc khôi phục từ chối trước khi thay đổi bất cứ thứ gì. Cài lại ứng dụng trong TrueNAS với cùng tên, việc này tạo lại mạng, rồi khôi phục dữ liệu của nó.

## Một bản xuất thô đã thất bại thay vì ghi một tệp {#a-plain-export-failed-instead-of-writing-a-file}

Nếu **Mã hóa bản xuất bằng age** đang bật trong **Cài đặt, Hệ thống** nhưng chưa đặt người nhận hợp lệ nào, một lần xuất sẽ thất bại với lỗi rõ ràng thay vì ghi văn bản thuần. Hãy thêm một người nhận hợp lệ (khóa công khai age hoặc khóa công khai SSH), hoặc tắt mã hóa nếu bạn muốn bản xuất là văn bản thuần. Xem [Bản xuất thuần](backups.md#exports).

## Một bản kết xuất cơ sở dữ liệu đã thất bại {#a-database-dump-failed}

Bản kết xuất thất bại không bao giờ làm hỏng bản sao lưu bao quanh nó. Nó được ghi lại như một lần chạy thất bại của riêng mình, và lý do cho biết phải sửa gì.

- **Bị từ chối đăng nhập.** Bản kết xuất đăng nhập bằng chính các biến mật khẩu của container (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` hoặc bản `_FILE` của chúng). Hãy kiểm tra chúng trên container cơ sở dữ liệu. Một biến `_FILE` trỏ tới bí mật mà người dùng của chính container không đọc được cũng thất bại y như vậy.
- **Thiếu quyền.** Với mật khẩu root ngẫu nhiên, bản kết xuất chỉ đăng nhập được với tư cách người dùng ứng dụng nên chỉ chứa đúng cơ sở dữ liệu đó, còn MySQL 8.4 trở lên có thể từ chối hẳn. Hãy đặt cho container một mật khẩu root thật, hoặc tắt kết xuất của nó.
- **Các bảng hệ thống cần nâng cấp.** MariaDB từ chối kết xuất khi bảng hệ thống của nó đến từ phiên bản cũ hơn (lỗi 1558). Thêm biến `MARIADB_AUTO_UPGRADE=1` rồi khởi động lại container, hoặc chạy `mariadb-upgrade` một lần bên trong.
- **Không có công cụ kết xuất.** Một image rút gọn hay tự dựng mà thiếu `pg_dump`, `mysqldump` hoặc `mariadb-dump` thì không kết xuất được. Hãy dùng image chính thức, hoặc tắt kết xuất.
- **Giới hạn thời gian.** Một bản kết xuất có `DB_DUMP_MAX_HOURS` (mặc định 6), bản sao lưu bao quanh có `BACKUP_MAX_HOURS`, còn bản kết xuất ngừng tiến triển sẽ bị cắt sau `BACKUP_STALL_HOURS`. Trường hợp cuối thường do một khóa mà ứng dụng đang giữ. Hãy nâng giới hạn đã kích hoạt, hoặc kết xuất lúc ứng dụng rảnh rỗi.
- **Container đang tạm dừng hoặc đang khởi động lại.** Bản kết xuất nói chuyện với máy chủ đang chạy. Nếu container cứ khởi động lại, nhật ký của chính nó sẽ nói vì sao.
- **Không xóa được một bản kết xuất hỏng.** Bản kết xuất mà BombVault không hoàn tất được sẽ bị xóa đi. Khi lần xóa đó thất bại, bản kết xuất vẫn nằm trong danh sách với dấu hỏng, và bạn có thể xóa nó ở đó.

## Một lần nhập đã thất bại {#an-import-failed}

Việc nhập sẽ dừng container, dời thư mục dữ liệu của nó sang một bên và để image tạo một thư mục trống vào chỗ đó. Nếu một bước trước khi nhập thất bại, thư mục cũ được đặt trở lại một cách tự động. Nếu chính việc nhập thất bại, container giữ thư mục mới còn thư mục cũ nằm bên cạnh với tên `<thư mục dữ liệu>.bombvault-before-import-<dấu thời gian>`. Thông báo lỗi của lần chạy nêu đúng đường dẫn.

Một thư mục dữ liệu vốn là một tập dữ liệu riêng, như trên TrueNAS, không thể đổi tên. Khi đó việc nhập để thư mục nguyên chỗ và chuyển nội dung của nó vào thư mục được giữ bên cạnh, nên tập dữ liệu phía trên cần đủ chỗ cho chúng. Việc nhập kiểm tra điều này trước khi dừng container.

Để đặt lại bằng tay: dừng container, đổi tên thư mục dữ liệu hiện tại cho khuất lối, đổi tên thư mục được giữ về tên gốc, rồi khởi động container. Với một tập dữ liệu, hãy chuyển nội dung thay vì đổi tên các thư mục. Trên Unraid, trình quản lý tệp ở thẻ Shares làm được việc này.

## Sao lưu tập dữ liệu ZFS thất bại hoặc bỏ qua một tập dữ liệu {#zfs-datasets}

Mỗi sự cố có một mã lý do trong ngoặc vuông, và trang [Tập dữ liệu ZFS](zfs-datasets.md#reason-codes) liệt kê tất cả cùng cách khắc phục. Ba trường hợp hay gặp nhất:

- **`snapshot-loop`**: ảnh chụp không tới được BombVault vì Host Data không chuyển tiếp các lần gắn mới. Sửa container, đặt Access Mode của Host Data thành Read/Write - Slave rồi khởi động lại BombVault.
- **`key-not-loaded`**: tập dữ liệu mã hóa chưa nạp khóa sẽ bị bỏ qua. Nạp khóa bằng `zfs load-key` và gắn tập dữ liệu. Lần sao lưu sau sẽ gồm nó.
- **`ssh-auth`**: máy chủ đã từ chối khóa của BombVault. **Kết nối ZFS tới máy chủ này**, mở bằng **Xem kết nối** trên trang Bản sao lưu, hiện lệnh cấp quyền cho nó. Hãy chạy lệnh đó một lần trên máy chủ.

## Một mục đứng yên ở "Đang học N/10" {#an-entry-stays-at-learning-n10}

Hầu hết các kiểm tra bất thường bắt đầu sau 10 lần sao lưu thành công của một mục, và bộ đếm bắt đầu lại sau **Đánh dấu là đã lường trước** và sau khi lựa chọn của mục thay đổi. Một mục không có lịch thì không học, và một container không có appdata thì không có gì để học, điều mà huy hiệu của nó cho biết. **Tiến trình học** trên trang Bất thường hiện một vòng mười phần cho mỗi mục.

## Chính sách lưu giữ không còn xóa bản sao lưu cũ của một mục {#retention-stopped-deleting-old-backups-of-one-entry}

Một bất thường nghiêm trọng đang mở đang giữ chúng: nguồn của mục gần như trống, co lại mạnh, hoặc một bản sao lưu lại lưu phần lớn dữ liệu của nó. Hãy mở bất thường từ huy hiệu trên mục. Nếu dữ liệu bị thiếu hoặc đã bị mã hóa, trước tiên hãy khôi phục từ bản sao lưu tốt gần nhất được liên kết. Sau đó xác nhận bất thường, hoặc đánh dấu là đã lường trước nếu thay đổi là của bạn, và lần chạy kế tiếp sẽ dọn như thường. Bản xem trước lưu giữ đánh dấu một mục như vậy là được giữ lại. Với một mục ZFS, chỉ tập dữ liệu được nêu trong bất thường giữ các bản sao lưu cũ, còn các tập dữ liệu khác của cây được dọn như thường.

## Dọn dẹp thủ công báo rằng một số mục đã được giữ lại {#manual-prune-says-some-entries-were-kept}

Cùng nguyên nhân: dọn dẹp để yên các bản sao lưu cũ của một mục có bất thường như vậy và nêu tên mục trong thông báo của nó. Mọi thứ khác được dọn như thường.

## Nhập lịch sử báo rằng không đọc được một kho lưu trữ {#history-import-says-a-repository-could-not-be-read}

Sau khi nâng cấp, BombVault đọc dung lượng của các bản sao lưu trước đó từ mỗi kho lưu trữ một lần. Một kho mà lúc đó nó không tới được, chẳng hạn một đích off-site đang tắt hoặc một share chưa được gắn kết, được đếm trong thẻ **Bất thường** dưới **Cài đặt, Xác minh** và được thử lại mỗi ngày một lần. Trong lúc đó các mục của nó học từ những bản sao lưu mới.

## Cảnh báo dung lượng đĩa không khớp với bảng điều khiển Unraid {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Trên chia sẻ người dùng của Unraid (`/mnt/user`), dung lượng trống là của cả mảng, không phải của một đĩa. Các đích sao lưu từ xa mà BombVault tới qua rclone, như SFTP, Hetzner Storage Box, SMB, WebDAV hay một ổ đám mây, được đo khi phía bên kia báo dung lượng trống, và một địa chỉ `sftp:` gõ tay cũng vậy. Kho S3, B2 và REST không có số liệu: đích sao lưu của chúng ghi **Chưa rõ dung lượng**, và thẻ **Bất thường** liệt kê chúng là chưa đo.

## Trợ lý AI không kết nối được {#an-ai-assistant-cannot-connect}

Trang [Máy chủ MCP](mcp.md#troubleshooting) giải thích ý nghĩa của từng mã trạng thái và từng lần từ chối của điểm kết nối MCP, cùng cách xử lý.

## Container cứ khởi động lại hoặc trông không khỏe mạnh {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault báo cáo khỏe/không khỏe từ `/api/health` của chính nó. Một công cụ tự chữa (như Autoheal) có thể tự khởi động lại nó nếu bộ máy bị treo. Hãy kiểm tra nhật ký container và thẻ **Tích hợp máy chủ** để tìm nguyên nhân gốc.

## Một đích sao lưu từ chối lưu {#location-refused}

BombVault kiểm tra một đích trước khi lưu, và lời từ chối nói lý do trong một câu. Những trường hợp thường gặp:

- **"This location lies inside another repository or target, or contains one."** Một kho nằm trong một kho khác sẽ làm hỏng cả hai. Thông báo nêu tên nơi kia. Hãy chọn thư mục khác, hoặc xóa nơi đó trước.
- **Nó là một kho bạn đã thiết lập.** Đích này là một trong các dòng dưới **Kho lưu trữ**, hoặc nằm trong hay bao quanh một kho. Hãy chọn nơi khác, hoặc xóa kho đó trước.
- **Cùng nơi với thư mục riêng của một phần.** Một bản sao tới thư mục riêng của một phần sẽ sao chép các bản sao lưu của nó lên chính chúng ở mỗi lần chạy, nên bị từ chối. Hãy chọn nơi khác, hoặc xem [Thư mục riêng của một phần](offsite-recovery.md#own-folder).
- **"That is not an off-site target of this domain"** đến từ một liên kết tới đích đã bị xóa. Phiên bản hiện tại xóa chúng khi khởi động và khi nhập. Nếu bạn vẫn thấy, hãy cập nhật BombVault.
- **Một đường dẫn đầy đủ trên máy chủ** như `/mnt/user/backups`. BombVault chạy trong một container, nên nó cần đường dẫn bên dưới Host Data, `user/backups`, và thông báo nêu đường dẫn cần nhập.

## Một đích sao lưu từ xa báo kho của nó đã mất {#repository-gone}

Một đích từng chứa bản sao lưu mà đột nhiên không còn kho nào sẽ không tự có một kho mới, trống, vì kho đó sẽ che kho thật khi đĩa trở lại. Một rest-server có đĩa không được gắn là nguyên nhân thường gặp, vậy hãy gắn lại đĩa và chạy lại bản sao lưu hoặc bản sao một lần nữa. Chỉ khi bạn cố ý làm trống hoặc xóa kho cũ, hãy bấm **Tạo kho mới ở đây** trên trang của đích. Nó bắt đầu lại cho mọi phần của đích đó cùng lúc.

## Một đăng nhập đám mây đã hết hạn {#cloud-sign-in-expired}

Khi đó phép thử hoặc lần sao chép tiếp theo báo rằng đăng nhập đã hết hạn. **Đổi kết nối** trên trang của đích sẽ đăng nhập lại.

- **Box, Jottacloud và các dịch vụ khác dùng token làm mới một lần:** BombVault giữ token mà rclone đã làm mới, kể cả qua các lần khởi động lại và trong bộ khôi phục. Sau khi khôi phục từ một bản sao lưu cài đặt cũ hơn, token trong đó có thể đã hết hạn, nên hãy đăng nhập thêm một lần. Kho vẫn giữ nguyên.
- **iCloud Drive:** Apple kết thúc đăng nhập sau 30 ngày. Nó cần một token tin cậy và cookie mới từ `rclone config`.
- **OneDrive:** một đăng nhập có nhiều ổ sẽ hỏi dùng ổ nào, nên một đích không có lựa chọn vẫn không dùng được cho đến khi bạn chọn. Với OneDrive Business và Google Drive, hãy xem ghi chú về dữ liệu bị dọn dưới [Các đích sao lưu](offsite-recovery.md#destinations).

## "The off-site copy of the last backup is still running" {#copy-still-running}

Bản sao lưu trước đã xong, và bản sao của nó vẫn đang trên đường, điều này chặn một bản sao lưu mới của phần đó. Hãy chờ đến khi phần đầu trạng thái không còn ghi **Đang sao chép ra ngoài site**. Các nút bắt đầu một bản sao lưu cũng nói như vậy trong lúc chờ. Lần sao chép đầu tiên tới một đích mới có thể mất hàng giờ, xem [ví dụ S3](offsite-recovery.md#example-s3).

## Một bản nhân bản ZFS dừng vì đích có snapshot mới hơn {#replica-target-newer}

Đích giữ các snapshot mới hơn snapshot mới nhất mà nó chung với máy chủ này, ví dụ vì tập dữ liệu ở đây đã bị lùi lại hoặc bản sao trên đích đã bị thay đổi. Thẻ Bản nhân bản và lịch sử chạy nêu tên snapshot dưới **Đưa đích về**. Hãy chạy `zfs rollback -r` về snapshot đó trên đích, cho từng tập dữ liệu mà lần chạy nêu, rồi bấm **Nhân bản ngay**. Xem [Snapshot được giữ bao lâu](zfs-datasets.md#replica-retention).

## Một VM có snapshot Unraid bị lỗi và nêu tên một tệp {#vm-chain}

BombVault lần theo chuỗi backing của từng đĩa và sao lưu mọi tệp trong đó. Một chuỗi không thể theo tới cuối sẽ làm bản sao lưu lỗi thay vì bỏ sót một phần của đĩa, và thông báo nêu tên tệp. Hãy kiểm tra tệp có tồn tại bên dưới `/mnt` không và snapshot còn được liệt kê trong VM Manager không. Một tệp bên ngoài Host Data hoàn toàn không đọc được. Xem [VM có snapshot Unraid](vm-backup-ssh-setup.md#unraid-snapshots).

## Bản sao ngoài site bị tạm dừng sau khi dựng lại {#copies-paused}

Sau **Tìm bản sao lưu** hoặc trình hướng dẫn khẩn cấp mà không khôi phục cài đặt, các bản sao ngoài site của mọi phần được dựng lại phải chờ, vì BombVault không còn biết bạn đã loại mục nào ra. Trang tổng quan cho biết điều đó. Hãy bấm **Xác nhận mặc định** trong **Đích theo phần**, xem bản xem trước và bỏ ra những gì nên ở ngoài. Nhập một tệp cài đặt không kết thúc việc tạm dừng. Xem [Sau khi dựng lại](offsite-recovery.md#after-a-rebuild).

## Thông báo không đến {#notifications-do-not-arrive}

Bắt đầu với **Gửi thử** dưới **Cài đặt, Thông báo**. Dòng bên dưới thẻ cho biết chuyện gì đã xảy ra.

- **Không có gì được gửi cả.** Hãy kiểm tra **Thông báo**. Với **Không bao giờ** thì không gì đi ra, và với **Chỉ khi thất bại** thì một lần chạy tốt vẫn im lặng. Khi bật **Tóm tắt các lần chạy theo lịch**, một lần chạy theo lịch gửi một bản tóm tắt vào cuối.
- **Một địa chỉ mới.** Sau khi bạn đổi máy chủ của Matrix, Gotify hay email, ô sẽ hỏi lại token hoặc mật khẩu, và địa chỉ mới chỉ được lưu khi bạn gõ nó. BombVault không gửi bí mật cũ tới nơi mới.
- **Gotify** báo máy chủ từ chối token ứng dụng: hãy sao chép lại từ ứng dụng trong Gotify. **URL máy chủ** là địa chỉ gốc, không có `/message`.
- **Webhook:** hãy kiểm tra **Định dạng Webhook** có khớp với dịch vụ không. Địa chỉ Discord cần **Discord**, và ntfy cần **ntfy**.
- **Thông báo Unraid** chạy qua kết nối SSH máy chủ, nên hãy thử cái đó trước dưới **Cài đặt, Tích hợp**. Trên nền tảng khác chúng vẫn tắt, và thẻ ghi như vậy.
- **Thư thông báo không bao giờ đến.** Hãy kiểm tra **Mã hóa** so với cổng, thường là **STARTTLS** ở 587 hoặc **TLS (implicit)** ở 465.

Các ping Healthchecks không phụ thuộc vào bất cứ điều nào ở trên, xem [Healthchecks](notifications.md#healthchecks).

## Ứng dụng Android không mở được một máy chủ {#android-cannot-open}

Màn hình bắt đầu cho biết lý do trên thẻ của máy chủ:

- **Chưa kết nối:** cả địa chỉ của máy chủ lẫn relay đều không trả lời. Với **Không dùng relay**, ứng dụng chỉ tới được máy chủ ở địa chỉ riêng của nó.
- **Kiểm tra chứng chỉ:** máy chủ trình ra một chứng chỉ khác với cái bạn đã tin cậy. Ứng dụng chỉ mở nó sau khi bạn tin cậy chứng chỉ mới, nên hãy chắc rằng thay đổi đó là của bạn.
- **Cần đăng nhập:** một máy chủ được thêm bằng địa chỉ sẽ hỏi mật khẩu. Xem thêm [Điện thoại bị đưa tới trang đăng nhập](#a-phone-lands-on-the-login-page).
- **Khóa của máy chủ đã thay đổi:** màn hình bắt đầu hiện dấu vân tay của khóa mới, và trang chỉ mở sau khi bạn tin cậy nó. Xem [Khi ở xa nhà](android.md#away).

## Người dùng khác không đọc được kho cục bộ {#repository-permissions}

Kho cục bộ vẫn đọc được với những người dùng khác, chẳng hạn một người dùng đồng bộ qua SMB. Trước 10.4.1, các tệp mà prune, thẻ, pull, di chuyển phần, xóa hoặc sao lưu stack ghi ra có thể vẫn không đọc được cho đến đêm kế tiếp. Hãy cập nhật BombVault, và chúng đọc được ngay. Kho tự sao lưu vẫn chỉ root đọc được, có chủ ý, vì nó chứa cài đặt của BombVault.

## Vẫn bế tắc? {#still-stuck}

- Đọc đầy đủ các trang [Cấu hình](configuration.md) và [Off-site & khôi phục](offsite-recovery.md).
- **Báo lỗi** trong thẻ **Trợ giúp** dưới **Cài đặt, Thông tin** tải xuống một tệp ZIP cho issue trên GitHub: kiểm tra máy chủ, cấu hình của bạn với mọi bí mật đã được gỡ, các lần chạy gần đây, những gì được lên lịch tiếp theo và nhật ký gần đây kể từ lần container khởi động gần nhất. Nó cũng chứa bản kết xuất gần nhất của từng cơ sở dữ liệu, các mục ZFS với điểm gắn kết của chúng, các bất thường đang mở và số khóa MCP đang có. Mật khẩu, token và cấu hình rclone đã được gỡ, và gói tự ghi điều đó trong manifest của nó. Nó cần mật khẩu đăng nhập. Với sự cố làm container khởi động lại, hãy xem `docker logs`.
- Hỏi trên [luồng hỗ trợ Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Mở một [vấn đề GitHub](https://github.com/junkerderprovinz/bombvault/issues).
