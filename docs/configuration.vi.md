# Cấu hình

Trang này nói về các biến môi trường của container, các điểm gắn kết mà template cung cấp, mô hình bảo mật, sao lưu VM qua SSH và thiết lập off-site. Nơi lưu các bản sao lưu được thiết lập bên trong ứng dụng dưới dạng **đích sao lưu**, không phải qua biến môi trường.

## Biến môi trường

| Biến | Bắt buộc | Mô tả |
|---|---|---|
| `APP_KEY` | **Có** | Bí mật hex 32 byte (64 ký tự hex) dùng để dẫn xuất mật khẩu kho restic. Tạo bằng `openssl rand -hex 32`. Giữ nó an toàn: đánh mất nó khiến các bản sao lưu đã mã hóa không thể khôi phục được. |
| `LIBVIRT_HOST` | Cho VM và tập dữ liệu ZFS | Máy chủ Unraid được kết nối qua SSH để sao lưu VM (mặc định `host.docker.internal`; template điền sẵn một chỗ giữ chỗ IP-LAN). Dùng IP LAN Unraid của bạn, bắt buộc trên một mạng `br0.x` tùy chỉnh. Cũng dùng cho sao lưu tập dữ liệu ZFS (trường mẫu **Host SSH: Address**); giá trị giữ chỗ `192.168.x.x` được coi là chưa đặt. |
| `LIBVIRT_SSH_PORT` | Không | Cổng SSH của máy chủ để sao lưu VM (mặc định `22`). Trường mẫu **Host SSH: Port**, cũng dùng cho tập dữ liệu ZFS. |
| `LIBVIRT_SSH_USER` | Không | Người dùng SSH trên máy chủ để sao lưu VM (mặc định `root`). Trường mẫu **Host SSH: User**, cũng dùng cho tập dữ liệu ZFS. |
| `LIBVIRT_URI` | Không | URI kết nối libvirt đầy đủ, được dùng **nguyên văn** thay vì xây dựng từ ba biến `LIBVIRT_*` phía trên (khi đó các biến này bị bỏ qua đối với chuỗi kết nối). Mặc định không đặt. Cần thiết trên TrueNAS Scale, nơi libvirtd của nó lắng nghe trên một socket không chuẩn mà dạng chuỗi dựng sẵn không thể diễn đạt được: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Xem phần TrueNAS Scale trong [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). Nếu là URI `qemu+ssh://`, mỗi biến trong `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` và `LIBVIRT_SSH_PORT` chưa đặt sẽ lấy từ đó, kể cả cho các lệnh SSH riêng của BombVault (chuyển NVRAM, tập dữ liệu ZFS). |
| `PORT` | Không | Cổng HTTP (mặc định `3000`; chỉ dùng với `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Không | Cổng HTTPS (mặc định `3443`; template công bố nó 1:1, nên WebUI trả lời tại `https://<ip>:3443`). |
| `HTTP_ONLY` | Không | Đặt `true` để tắt trình lắng nghe HTTPS tự ký và chỉ phục vụ HTTP thuần (để dùng phía sau một reverse proxy kết thúc TLS). |
| `BIND_HOST` | Không | Địa chỉ mà WebUI lắng nghe (mặc định `0.0.0.0`, mọi giao diện). Để trống trong container, vì các cổng được công bố của nó cần mọi giao diện; `127.0.0.1` phù hợp khi chạy ngoài Docker. Healthcheck hỏi cùng địa chỉ đó. |
| `TRUSTED_PROXY` | Không | Các địa chỉ hoặc dải CIDR của proxy ngược đứng trước BombVault, phân tách bằng dấu phẩy (ví dụ `192.168.20.11` hoặc `10.0.0.0/8`). Chỉ từ những chặng đó thì `X-Forwarded-For` mới được tin, và bộ hãm đăng nhập khi ấy đếm số lần thất bại theo từng máy khách thật thay vì dồn tất cả những ai ở sau proxy vào cùng một giỏ. Không đặt (mặc định) nghĩa là không tin ai: một tiêu đề được tin vô điều kiện sẽ cho phép bất kỳ ai tự chọn giỏ của mình. |
| `HOST_SOURCE_ROOT` | Không | Đường dẫn máy chủ được gắn kết làm **Host Data** (mặc định `/mnt`). BombVault dịch các nguồn bind-mount mà Docker báo cáo thành các đường dẫn dưới điểm gắn kết này. Chỉ thay đổi nếu bạn đã gắn kết một gốc máy chủ khác. |
| `DATA_ROOT_SEGMENTS` | Không | Các tên đoạn đường dẫn cách nhau bởi dấu phẩy, đánh dấu một nguồn bind-mount là dữ liệu sao lưu (mặc định `appdata`, khớp với quy ước `/mnt/user/appdata/<container>` của Unraid). Bind-mount của một container được tự động chọn để sao lưu khi BẤT KỲ đoạn nào trong danh sách xuất hiện như một đoạn đường dẫn đầy đủ trong nguồn của nó trên máy chủ, ví dụ `DATA_ROOT_SEGMENTS=appdata,config` cũng bắt luôn một bind `.../config`. Xem [Phát hiện nguồn sao lưu](#backup-source-detection) để biết những cách khác, luôn hoạt động, mà thư mục dữ liệu của một container được tìm thấy. |
| `PLATFORM` | Không | Buộc BombVault coi mình đang chạy trên nền tảng nào, thay vì tự phát hiện: `unraid`, `generic`, hoặc `truenas` (mặc định không đặt: tự phát hiện Unraid bằng cách dò tìm dấu hiệu `dockerMan` của nó dưới điểm gắn kết flash, nếu không thì dùng `generic`; một giá trị không nhận dạng được cũng quay về `generic`, được ghi log). Đặt nó rõ ràng trên một máy chủ Docker thông thường hoặc TrueNAS Scale, thay vì dựa vào việc dò tìm tự động chỉ dành cho Unraid; tệp compose thông thường đã làm đúng như vậy. Thay đổi quy ước dự phòng appdata, các đích khôi phục mặc định khi khôi phục sang một phiên bản khác, và liệu các bước thông báo/plugin đồng hành chỉ dành cho Unraid có được thử hay không (xem `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Không | Tên của chính container BombVault, để nó không bao giờ sao lưu (và do đó dừng) chính mình. |
| `BACKUP_MAX_HOURS` | Không | Số giờ đồng hồ tối đa mà một lần sao lưu đơn có thể giữ khóa phần của nó trước khi bị hủy cưỡng bức (một biện pháp bảo vệ để một lần chạy bị kẹt không thể chặn phần mãi mãi). Để trống (mặc định) dùng `48`. Tăng nó lên cho các bản sao lưu đám mây rất lớn hoặc chậm (một lần chạy bị hủy ở mức giới hạn thất bại với `context deadline exceeded`). Đặt `0` để tắt hoàn toàn giới hạn. |
| `BACKUP_STALL_HOURS` | Không | Số giờ một bản sao lưu có thể **hoàn toàn không tiến triển** trước khi bị hủy. Để trống (mặc định) dùng `2`; đặt `0` để không bao giờ hủy vì bị đình trệ. Đây là biện pháp bảo vệ tinh hơn trong hai cái và thường là cái kích hoạt: nó theo dõi xem còn điều gì đang diễn ra hay không thay vì lần chạy đã mất bao lâu, nên một bản sao lưu nhiều terabyte chậm nhưng khỏe mạnh được để yên, còn một bản bị kẹt trên một share không phản hồi sẽ bị dừng sau vài giờ thay vì vài ngày. Một cảnh báo được ghi vào nhật ký sau 30 phút im lặng, trước khi bất cứ thứ gì bị hủy. Việc quét được tính là tiến triển: restic không ghi byte nào khi đi qua một cây thư mục lớn, và giai đoạn đó được theo dõi qua tổng số tập tin và byte của nó thay vì qua số byte đã ghi. Hai biến độc lập với nhau, và `BACKUP_MAX_HOURS` vẫn giới hạn các giai đoạn sau chính bản sao lưu (lưu giữ, thống kê, bản sao off-site), nơi không có bộ đếm nào để theo dõi. |
| `DB_DUMP_MAX_HOURS` | Không | Số giờ một lần kết xuất cơ sở dữ liệu tự động được phép chạy trước khi bị dừng. Để trống (mặc định) là `6`; giá trị cho phép từ `1` đến `48`, và giới hạn này được giữ thấp hơn `BACKUP_MAX_HOURS` một giờ (bằng một nửa giá trị đó khi nó dưới hai giờ), để một bản kết xuất dài bị chính giới hạn của nó cắt và được báo cáo như vậy, thay vì kéo cả bản sao lưu theo. Bản kết xuất không còn tiến triển sẽ bị dừng sớm hơn, sau `BACKUP_STALL_HOURS`. Một bản kết xuất bị dừng chỉ thất bại cho riêng nó, còn bản sao lưu của container vẫn tiếp tục. Trên Unraid, thêm biến này vào container BombVault bằng **Add another Path, Port, Variable**. |
| `TZ` | Không | Múi giờ cho bộ lập lịch (ví dụ `Europe/Berlin`). **Nếu không đặt, mọi lịch trình sẽ chạy theo UTC**: lịch đặt lúc 02:30 sẽ bắt đầu lúc 02:30 UTC chứ không theo giờ địa phương. Trên Unraid bạn không bao giờ tự đặt giá trị này: hệ thống truyền múi giờ của chính nó vào mọi container. Nhật ký khởi động cho biết múi giờ nào đã được chọn. Một múi giờ có giờ mùa hè sẽ bỏ qua một lần chạy vào mùa xuân và chạy một lần hai lượt vào mùa thu, còn UTC thì không gặp cả hai chuyện đó nhưng lệch một giờ so với đồng hồ của bạn hai lần mỗi năm. |

## Điểm gắn kết

Gắn kết Docker socket, flash (`/boot`) và gốc **Host Data** (`/mnt`) như hiển thị trong template CA. Cả *nguồn* và *đích* sao lưu đều nằm dưới Host Data, và nó được gắn kết **slave** nên một share từ xa được gắn kết sau khi container khởi động (ví dụ dưới `/mnt/remotes`) trở nên hiển thị mà không cần khởi động lại.

Sao lưu tập dữ liệu ZFS cũng cần chế độ này: máy chủ chỉ gắn ảnh chụp của một tập dữ liệu sau khi container đã khởi động. Xem [Tập dữ liệu ZFS](zfs-datasets.md).

Các kho cục bộ mặc định nằm ở `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` và được tạo ở lần sao lưu đầu tiên. Chúng tạo thành đích sao lưu **Cục bộ**. Trên trang của nó trong **Đích sao lưu**, thẻ **Kết nối** đổi thư mục của từng phần, và cửa sổ nó mở ra có công tắc **Cục bộ / Từ xa**: đường dẫn của một phần có thể là một restic remote (`s3:...`, `rest:...`, `sftp:...`, `rclone:...`) thay vì thư mục cục bộ, sao lưu thẳng vào đó mà không có bản sao cục bộ riêng; xem [Kho chính từ xa](offsite-recovery.md#remote-primary-repositories).

!!! note "Kiểm tra tích hợp máy chủ"
    Thẻ **Tích hợp máy chủ** trên Tổng quan (thêm qua **Tùy chỉnh**) kiểm tra mọi điểm gắn kết và CLI (Docker socket, libvirt, restic, qemu-img, rclone) và báo cáo bất kỳ phần nào bị thiếu.

## Nhận diện nguồn sao lưu {#backup-source-detection}

Với mỗi container, BombVault tự động chọn bind mount và volume đặt tên nào cần sao lưu. Một đường dẫn được chọn khi bất kỳ điều nào sau đây áp dụng (bạn luôn có thể ghi đè kết quả trong thẻ **Thư mục sao lưu** ở tab Cài đặt của container):

- **Khớp một đoạn gốc dữ liệu:** nguồn trên máy chủ của bind chứa một trong các đoạn của `DATA_ROOT_SEGMENTS` như một thành phần đường dẫn trọn vẹn (mặc định chỉ `appdata`).
- **Volume Docker có tên** luôn được đưa vào, vì chúng không có bản tương đương dùng một lần nên chẳng có gì để lọc bỏ, **nhưng chỉ khi đường dẫn lưu trữ thật của volume trên máy chủ tự nó tới được qua điểm gắn Host Data**, đúng như mọi đường dẫn máy chủ khác mà BombVault sao lưu. Trình điều khiển volume cục bộ mặc định đặt volume dưới gốc dữ liệu của chính daemon, tức `/var/lib/docker/volumes/<tên>/_data` nếu bạn chưa đổi (kiểm tra bằng `docker info -f '{{.DockerRootDir}}'`). Vị trí đó KHÔNG nằm trong điểm gắn Host Data hẹp, chỉ một thư mục, mà tệp `docker-compose.yml` chung dùng theo mặc định. Volume không tới được sẽ bị bỏ qua lặng lẽ, đó không phải lỗi. Để thật sự sao lưu volume có tên trên một máy chủ thông thường, hãy trỏ Host Data (và `HOST_SOURCE_ROOT`) tới một thư mục cha chung bao trùm cả gốc dữ liệu của Docker: đánh đổi được nêu trong ghi chú Host Data của tệp compose (Unraid né chuyện này bằng cách gắn toàn bộ `/mnt`, quy ước cấp cao nhất của riêng nó, cũng vì cùng lý do).
- **Thư mục dự án Docker Compose:** nếu container mang nhãn tiêu chuẩn `com.docker.compose.project.working_dir` (do `docker compose up` tự đặt), thư mục đó cũng được thêm vào, bất kể có bind nào khớp một đoạn gốc dữ liệu hay không.
- **Ghi đè bằng nhãn `bombvault.data`:** đặt nhãn `bombvault.data=true` lên container để đưa vào TẤT CẢ bind mount của nó, dành cho bố cục mà cả hai quy ước trên đều không bắt được (ví dụ một bind `/srv/plex/config` đơn lẻ, không có dự án Compose). Mọi giá trị không rỗng khác `false` đều tính là đúng; thiếu nhãn hoặc `bombvault.data=false` thì không thay đổi gì.
- **Nhãn `bombvault.dbdump`:** đặt `bombvault.dbdump=false` trên một container để tắt việc kết xuất cơ sở dữ liệu tự động của nó (`0`, `no` và `off` cũng vậy), hoặc nêu tên engine (`postgres`, `mysql`, `mariadb`) để kết xuất một container mà BombVault tự không nhận ra. Nhãn này thắng công tắc trong thẻ **Cơ sở dữ liệu** ở tab Cài đặt của container, và trên Unraid đó là cách duy nhất.

## Mô hình bảo mật {#security-model}

!!! warning "Quyền kiểm soát máy chủ tương đương root"
    Thông qua Docker socket, BombVault có thể dừng, xóa và tạo lại các container cũng như đọc/ghi appdata, và để sao lưu VM nó đăng nhập vào máy chủ qua SSH (`qemu+ssh://`, root theo mặc định) để chạy `virsh`. Bất kỳ ai truy cập được giao diện web của nó thực chất đều có quyền root trên máy chủ.

### Đăng nhập {#login}

- **Bảo vệ bằng mật khẩu tùy chọn** (thẻ **Mật khẩu** trong **Cài đặt, Bảo mật**): đặt mật khẩu để yêu cầu đăng nhập, xóa nó để tắt. Mặc định tắt cho mạng LAN tin cậy. Mật khẩu được lưu bằng Argon2id trên một pepper gắn với `APP_KEY`, nên một `/config` bị sao chép sẽ vô giá trị nếu thiếu khóa và chậm để tấn công kể cả khi có khóa. Mật khẩu mới cần ít nhất 12 ký tự; mật khẩu cũ ngắn hơn vẫn dùng được cho đến khi đổi. Các phiên được ký (HMAC dẫn xuất từ `APP_KEY`) và đổi mật khẩu sẽ vô hiệu hóa chúng; đăng nhập bị giới hạn năm lần thất bại mỗi phút cho mỗi máy khách.
- **Một số việc cần mật khẩu.** Tải bộ khôi phục và báo cáo lỗi, gửi khóa cho bên giữ khóa, cho phép trao một bản gửi, tạo mã mở bản gửi mới, xóa các bản sao lưu ở một đích sao lưu, xuất cài đặt kèm thông tin đăng nhập, xác thực hai yếu tố và khóa truy cập đều bị từ chối cho đến khi đặt mật khẩu đăng nhập, và cần đăng nhập ngay tại phiên bản đó.
- **Xác thực hai yếu tố** (**Cài đặt, Bảo mật**): một mã thời gian từ ứng dụng xác thực bên cạnh mật khẩu, cùng tám mã khôi phục dùng một lần được trao một lần khi bật. Bí mật chia sẻ được lưu mã hóa bằng `APP_KEY`, và tắt lại yếu tố này cần một mã hiện hành.
- **Khóa truy cập** (WebAuthn) nằm trên một thẻ riêng khi đã đặt mật khẩu, dùng song song với mật khẩu chứ không bao giờ thay thế nó, nên xóa hết khóa truy cập cũng không khóa ai ở ngoài. Chúng cần một tên miền thật và một chứng chỉ mà trình duyệt tin cậy. Địa chỉ mặc định `https://<ip>:3443` đúng là thứ WebAuthn từ chối, nên thẻ nói rõ điều đó thay vì đưa ra một nút bấm sẽ thất bại.
- **Mật khẩu bị quên** được đặt lại trên trang đăng nhập. **Quên mật khẩu?** khiến BombVault ghi một mã 12 ký tự, chia ba nhóm bốn, vào nhật ký container của nó (trên Unraid: tab Docker, biểu tượng BombVault, **Logs**). Mã đó cùng một mật khẩu mới sẽ đưa bạn vào. Việc đặt lại cũng tắt xác thực hai yếu tố, gỡ mọi khóa truy cập, thu hồi mọi mã thông báo API, khóa MCP và phiên đăng nhập OAuth, kết thúc mọi phiên khác, đăng xuất mọi điện thoại đã ghép nối và tắt việc ghép nối điện thoại; trang sau đó liệt kê những gì đã mất. Mã có hiệu lực 10 phút, và đoán sai không làm tiêu hao nó. Mã thông báo widget và metrics vẫn còn vì chúng chỉ đọc.
- **Ai đọc được nhật ký container thì đặt lại được mật khẩu.** Đó là root trên máy chủ, nhưng cũng là một trình xem nhật ký như Dozzle, bất kỳ container nào được cấp Docker socket (chỉ đọc là đủ) và một công cụ chuyển tiếp nhật ký đưa đầu ra container đi nơi khác. Chỉ trao những thứ đó cho người bạn tin tưởng với mật khẩu. Người chỉ truy cập được giao diện web thì không bao giờ thấy mã.

### Điện thoại và thành viên nhóm {#phones-and-members}

- **Điện thoại đã ghép nối chạm tới một danh sách cho phép.** Một điện thoại biết mười hai từ của nhóm đăng nhập mà không cần mật khẩu, miễn là **Điện thoại có thể ghép nối bằng mười hai từ của nhóm** đang bật trong thẻ **Ứng dụng Android** dưới **Cài đặt, Bảo mật**. Phiên của nó chỉ chạm tới những gì ứng dụng hiển thị: Tổng quan, các lần chạy, các bản sao lưu để xem, bắt đầu và khôi phục, các bất thường để đọc và trang Phiên bản để xem. Các đích sao lưu và lệnh đến với nó đã bị lược bỏ địa chỉ và bí mật, và một lần khôi phục thay thế bản gốc sẽ sao lưu trạng thái hiện tại trước. Mọi thứ khác trả lời rằng cần đăng nhập ngay tại phiên bản đó: khóa, bộ khôi phục, bản gửi khóa, cài đặt, xóa bản sao lưu. Việc đặt lại mật khẩu tắt ghép nối điện thoại cho đến khi bạn bật lại.
- **Khi ra ngoài, điện thoại mở trang qua relay.** Mỗi yêu cầu của trang được niêm phong bằng khóa của nhóm, relay không thể đọc hay thay đổi, và được ký bằng khóa riêng của điện thoại, mà máy chủ ghim lại lần đầu hỏi điện thoại đó. Máy chủ chỉ chạy một yêu cầu một lần và chỉ khi nó còn mới. Máy chủ chỉ trả lời một điện thoại có trong trang Phiên bản, miễn là ghép nối điện thoại đang bật, và chỉ bằng một phiên mà nó đã cấp cho điện thoại đó, nên đăng xuất ở mọi nơi và đặt lại mật khẩu cũng chấm dứt các trang này. Máy chủ ký mọi phản hồi, và ứng dụng kiểm tra chữ ký bằng khóa đã ghim cho máy chủ đó và cảnh báo khi khóa thay đổi. Trang như vậy chạm tới cùng danh sách cho phép với một điện thoại đã ghép nối, có hay không có mật khẩu đăng nhập, cộng với trạng thái đăng nhập mà trang đọc đầu tiên. Đăng nhập và đăng xuất, đặt lại mật khẩu và tải xuống vẫn nằm ở địa chỉ riêng của máy chủ, và một phản hồi bị giới hạn ở 16 MB.
- **Các thành viên ký những gì họ yêu cầu.** Yêu cầu vai trò và câu trả lời, việc trao mật khẩu restic cho bên nhận hoặc bên kéo, đăng nhập bên nhận và bản gửi khóa được ký bằng khóa của từng thành viên, mà một phiên bản ghim lại lần đầu thấy thành viên đó. Một lệnh gọi ký bằng khóa khác bị từ chối. **Quên khóa** trong chi tiết của một phiên bản bỏ khóa đã ghim, cho trường hợp khóa đang biết ở đây không phải của chính thành viên; thành viên sẽ bị hỏi lại khóa ở lệnh gọi ký kế tiếp. Các thành viên chạy BombVault 9.x không ký và có thể bị bất kỳ ai có các từ đóng giả, như ở 9.9.
- **Mười hai từ là một mật khẩu.** Ai có chúng sẽ vào nhóm, thấy trạng thái bảo vệ của các thành viên và có thể yêu cầu vai trò, việc mà một người ở phiên bản kia vẫn phải cho phép. Không có mật khẩu đăng nhập thì bất kỳ ai mở được giao diện web đều đọc được chúng, và thẻ ghép nối nói điều đó cho đến khi đặt mật khẩu.

### Khóa {#keys}

- **APP_KEY làm hai việc.** Nó niêm phong những gì bản cài này lưu (mật khẩu, thông tin đăng nhập, mã thông báo) và là khóa của các bản sao lưu: mật khẩu restic được dẫn xuất từ nó, và định nghĩa container và VM được niêm phong bằng nó.
- **Khóa được tiếp nhận được giữ riêng.** Khi Trình hướng dẫn khẩn cấp tiếp nhận các bản sao lưu của một máy chủ khác, bản cài này tiếp tục niêm phong bằng APP_KEY của chính nó và lưu khóa của máy chủ kia làm **khóa sao lưu**, được niêm phong như mọi bí mật khác. Bộ khôi phục khi đó in cả hai. Nếu sau đó APP_KEY bị đổi, khóa sao lưu không còn mở được: BombVault ngừng sao lưu và nói điều đó trong **Cần chú ý**, thay vì ghi bằng khóa sai. Hãy tải lại bộ khôi phục trong **Khôi phục, Khẩn cấp**, hoặc đặt APP_KEY về như cũ. Xem [Trình hướng dẫn khẩn cấp](offsite-recovery.md#emergency).
- **Mã hóa** được bật mặc định (**Mã hóa bản sao lưu** trong thẻ **Mã hóa** dưới **Cài đặt, Bảo mật**). Các bản sao lưu được restic mã hóa bằng mật khẩu dẫn xuất từ khóa sao lưu.
- **Bản gửi khóa.** Một bên giữ khóa trong nhóm của bạn giữ một bản niêm phong của bộ khôi phục của máy chủ này mà chính nó không mở được. Chỉ mã mở bản gửi 20 ký tự mới mở được, và bên giữ khóa chỉ trao nó sau khi ai đó ở đó nhấn **Cho phép**. Xem [Bên giữ khóa](offsite-recovery.md#key-keeper).

### Mạng {#network}

- **Thay đổi cần JSON.** Một yêu cầu thay đổi điều gì đó phải gửi `Content-Type: application/json` và không được trình duyệt đánh dấu là liên trang (cross-site), nên một trang trên trang web khác không thể khiến trình duyệt của bạn thay đổi cài đặt trên một địa chỉ LAN. Script điều khiển API sẽ gửi header đó; mọi yêu cầu khác bị từ chối với `415`.
- Vì cổng bảo vệ là tùy chọn, khi chưa đặt thì toàn bộ giao diện và API (gồm cả đích sao lưu, các route kiểm tra can thiệp và các route khôi phục) ai tới được cổng đều truy cập được. Hãy đặt mật khẩu đăng nhập ngay khi dùng sao lưu off-site, append-only hoặc mã hóa.
- Chỉ chạy BombVault trên một mạng tin cậy, không phơi ra ngoài. Để truy cập từ xa, đặt nó phía sau một reverse proxy có thêm xác thực và TLS. Các phản hồi mang theo các tiêu đề bảo mật cơ bản (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Sau một proxy ngược, mọi yêu cầu đều mang địa chỉ của proxy, nên nếu không có `TRUSTED_PROXY` thì bộ hãm đếm tất cả máy khách vào một giỏ và những lần thất bại của kẻ tấn công cũng khóa bạn ở ngoài. Khai báo proxy trong `TRUSTED_PROXY` để việc đếm trở lại theo từng máy khách.
- Reverse proxy đặt trước BombVault phải chuyển tiếp header `Authorization` hoặc `X-API-Key` tới `/mcp` và không được đệm câu trả lời, nếu không trợ lý sẽ không kết nối được. Xem [Máy chủ MCP](mcp.md#tls).
- Endpoint MCP `/mcp` trả về `404` cho đến khi có một khóa hoặc bật đăng nhập qua OAuth, và nó đòi khóa hoặc token từ mọi máy khách ngay cả khi mật khẩu đăng nhập đang tắt; không địa chỉ nào được miễn, kể cả `localhost`. Nó không có công cụ khôi phục hay xóa, và việc khôi phục bản sao lưu cấu hình sẽ thu hồi mọi khóa. Xem [Máy chủ MCP](mcp.md#security).
- Với `HTTP_ONLY=true` cookie phiên mất cờ `Secure` của nó (bắt buộc phải vậy, để hoạt động qua HTTP thuần), nên chỉ bật mật khẩu phía sau một proxy kết thúc TLS nếu tính bảo mật là quan trọng.
- Kết nối SSH sao lưu VM tin cậy host key ở lần kết nối đầu tiên (TOFU) và ghim nó sau đó. Xác minh khóa của máy chủ ngoài luồng nếu đường dẫn container-tới-máy-chủ của bạn không tin cậy.

## Máy chủ MCP {#mcp-server}

Máy chủ MCP không cần biến môi trường nào. Bạn bật nó bằng cách tạo một khóa tại **Cài đặt, Tích hợp, Máy chủ MCP**, và nó trả lời ở `/mcp` trên cùng cổng với giao diện web (ví dụ `https://192.168.1.10:3443/mcp`). Khi không có khóa đang hoạt động, đường dẫn này trả lời `404`. Máy khách, chứng chỉ và các giới hạn được mô tả tại [Máy chủ MCP](mcp.md).

## Sao lưu VM qua SSH

BombVault sao lưu các KVM/libvirt VM **mà không gắn kết bất kỳ đường dẫn libvirt nào**. Nó chạy `virsh` trên máy chủ qua SSH (`qemu+ssh://`), nên nó không bao giờ có thể ảnh hưởng đến VM Manager của máy chủ bạn.

Gắn socket libvirt của máy chủ vào một container là cách dễ hỏng trên Unraid: VM Manager sở hữu các đường dẫn đó, và việc bật tắt "Enable VMs" có thể khiến libvirt không khởi động được. Khóa SSH cấp quyền root trên máy chủ, cùng mức tin cậy với Docker socket mà BombVault vốn đã dùng.

Thiết lập nhanh:

1. **Cài đặt, Tích hợp, SSH máy chủ:** sao chép khóa công khai được hiển thị.
2. Thêm nó vào `/root/.ssh/authorized_keys` của Unraid (cũng được lưu vào flash để nó tồn tại qua các lần khởi động lại).
3. Nhấp **Kiểm tra kết nối**.

Template thêm `--add-host=host.docker.internal:host-gateway` để container có thể tiếp cận máy chủ. Đặt `LIBVIRT_HOST` thành IP LAN Unraid của bạn nếu tên đó không phân giải được (ví dụ khi container chạy trên một mạng `br0.x` tùy chỉnh). Nếu bạn đã đổi cổng SSH của Unraid, đặt `LIBVIRT_SSH_PORT` cho khớp. **Snapshot trực tiếp** ngoài ra cần qemu guest agent trong VM và đĩa nằm trên `/mnt/cache` (không phải `/mnt/user`).

!!! important "Hướng dẫn thiết lập VM và mạng đầy đủ"
    Hướng dẫn từng bước hoàn chỉnh (bật SSH, ủy quyền khóa lâu bền, định tuyến mạng tùy chỉnh và VLAN, phương thức theo từng VM và khắc phục sự cố phía máy chủ) nằm tại [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) trên GitHub.

## Thiết lập off-site

Các bản sao off-site đi tới **đích sao lưu**. Mở **Đích sao lưu** từ ô cài đặt của nó (hoặc ghim vào thanh bên) và chọn **Thêm đích**. Xem [Off-site & khôi phục](offsite-recovery.md) để biết quy trình đầy đủ (bảo vệ chống xóa, kiểm tra can thiệp và diễn tập DR). Tóm tắt:

- **Backend:** SMB/CIFS và WebDAV qua rclone không cần gắn kết trên máy chủ, NFS (gắn export trên máy chủ và dùng như thư mục cục bộ), các backend restic gốc không cần rclone (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`), hoặc bất kỳ rclone remote nào (`rclone:<remote>:<bucket>/path`). Backblaze B2 ở đây không có backend gốc: hãy dùng điểm cuối S3 của nó (`s3:https://s3.<region>.backblazeb2.com/<bucket>/<path>`) với ID khóa và khóa ứng dụng làm thông tin xác thực S3.
- **Thông tin xác thực** được lưu mã hóa, hoặc dưới dạng thông tin xác thực đám mây dùng chung hoặc dưới dạng một bộ có tên. Cả hai là các thẻ trong **Cài đặt, Đích sao lưu**, và trang riêng của một đích chọn bộ nó dùng trong **Thông tin xác thực**.
- **Đích SSH không cần cài đặt gì ở phía bên kia.** `sftp:` chỉ cần một máy chủ SSH. Thêm khóa công khai từ **Cài đặt, Tích hợp, SSH máy chủ** (cũng nằm tại `/config/ssh/id_ed25519.pub`) vào `~/.ssh/authorized_keys` của người dùng đích.
- **Bản sao off-site:** BombVault sao chép các bản sao lưu mới bằng `restic copy` theo kiểu hết sức có thể, bên trên một kho chính (thường là cục bộ). Thẻ **Sao chép** của một đích đặt việc nó sao chép **Sau mỗi bản sao lưu** hay theo kế hoạch riêng, và **Cài đặt, Lịch trình, Bản sao ngoài site theo phần** cho một phần kế hoạch sao chép riêng. **Sao chép ngay** nằm trong thẻ **Bản sao ngoài site** của mỗi phần dưới **Cài đặt, Đích sao lưu**.
- **Nhiều đích sao lưu cho mỗi phần:** thẻ **Đích theo phần** trên trang Đích sao lưu bật hoặc tắt từng dòng từ một phần tới một đích bằng một cú nhấp, kể cả **Cục bộ**. Một phần chỉ có các đích off-site sao lưu thẳng tới chúng, và một phần có mọi dòng đều tắt thì không được sao lưu. **Quy tắc theo phần** của một đích cho một phần kho, chính sách lưu giữ hoặc nén riêng ở đó.
- **Nơi lưu trữ theo từng mục:** mỗi container, VM và bộ thư mục làm sáng **Cục bộ** và các đích nhận bản sao lưu của nó, trong thẻ **Lịch trình và đích sao lưu** ở tab Cài đặt của nó. Các mục không có lựa chọn riêng theo phần của chúng. Xem [Nơi lưu trữ theo từng mục](offsite-recovery.md#placement).
- **Lưu giữ:** mỗi đích giữ các bản sao lưu theo thẻ **Giữ trong bao lâu** của nó, và một phần có thể giữ giá trị riêng ở đó trong **Quy tắc theo phần**. Bản sao trong trường off-site của một phần giữ quy tắc của thẻ **Lưu giữ ngoài site** dưới **Cài đặt, Đích sao lưu**, có **Quy tắc lưu giữ theo nguồn** cho từng phần (để toàn bộ bằng 0 nếu không bao giờ muốn tự động cắt tỉa bản sao lưu off-site).
- **Giới hạn băng thông:** giới hạn tốc độ tải lên/tải xuống của restic trong thẻ **Băng thông off-site** dưới **Cài đặt, Đích sao lưu**, hoặc theo từng đích trong thẻ **Sao chép** của nó.
- **Ưu tiên phát trực tuyến:** trong thẻ **Phát trực tuyến** trên trang Đích sao lưu, chọn các máy chủ media (Plex, Jellyfin và Emby được chọn sẵn theo tên image), tốc độ gửi từ đó được coi là đang phát trực tuyến, giới hạn tải lên khi đang phát và bao lâu sau một luồng thì giới hạn bình thường quay lại.
- **Lớp lưu trữ lạnh và lưu trữ lâu dài (S3):** với một đích sao lưu S3, chọn một tầng đọc được khi khôi phục (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval) trên trang của nó. Các rclone remote đặt lớp của chúng trong cấu hình rclone.
- **Kho chính từ xa thay vì cục bộ:** đường dẫn riêng của một phần có thể chính là một trong các backend trên, không có bản sao cục bộ và không có bước nhân bản. Xem [Kho chính từ xa](offsite-recovery.md#remote-primary-repositories) về công tắc **Cục bộ / Từ xa** và các cài đặt băng thông, append-only và ngân sách tăng trưởng.

## Bất thường {#anomalies}

Phát hiện bất thường được thiết lập trong thẻ **Bất thường** dưới **Cài đặt, Xác minh**. Mỗi điều khiển được lưu ngay khi bạn đổi, và ba điều khiển dưới công tắc bị ẩn khi tính năng phát hiện đang tắt.

| Thiết lập | Mặc định | Tác dụng |
|---|---|---|
| **Phát hiện bất thường** | Bật | So sánh mỗi bản sao lưu với lịch sử riêng của mục. Khi tắt, không có gì mới được kiểm tra và mục **Bất thường** rời khỏi thanh bên; thẻ vẫn liên kết tới các phát hiện trước đó. |
| **Độ nhạy** | Cân bằng | Nghiêm báo cả những thay đổi nhỏ hơn, Dễ dãi chỉ báo thay đổi lớn. |
| **Gửi thông báo cho** | Chỉ các phát hiện nghiêm trọng | Mức nghiêm trọng thấp nhất gửi tin nhắn qua các kênh đã thiết lập tại Thông báo. Các lần sao lưu và dump thất bại lặp lại và các kiểm tra khôi phục theo lịch thất bại vốn đã gửi tin nhắn riêng và không bị gửi hai lần. |
| **Giữ bản sao lưu cũ khi nguồn co lại mạnh hoặc bị ghi lại** | Bật | Khi một mục có phát hiện đang mở về nguồn gần như trống, co lại mạnh hoặc phần lớn dữ liệu được lưu lại, việc lưu giữ và dọn dẹp để yên các bản sao lưu cũ của mục đó. Xác nhận phát hiện hoặc đánh dấu là đã lường trước để thả chúng ra. |

Mỗi mục có thể dùng độ nhạy và mức thông báo tối thiểu riêng. Đặt chúng trên tab **Xác minh** của mục, hoặc trên trang Bất thường dưới **Giám sát**. Với một mục ZFS, chúng áp dụng cho mọi tập dữ liệu của cây.

## Cài đặt di động (xuất và nhập) {#portable-settings-export-and-import}

Thẻ **Xuất / nhập cài đặt** dưới **Cài đặt, Hệ thống** ghi toàn bộ cấu hình BombVault của bạn (cài đặt các phần, đích sao lưu, lịch trình, lưu giữ, thông báo) ra một tệp JSON di động mà bạn có thể nhập trên phiên bản khác, nên chuyển sang máy mới hay nhân bản một thiết lập không có nghĩa là phải nhập lại mọi thứ bằng tay. Việc nhập hiển thị bản xem trước và hỏi xác nhận, và không bao giờ đụng tới dữ liệu sao lưu hay lịch sử của bạn.

!!! warning "Bản xuất có thể chứa thông tin đăng nhập"
    Bạn chọn có bao gồm thông tin đăng nhập off-site, thông báo và broker MQTT trong tệp hay không. Khi có kèm thông tin đăng nhập, bản xuất nhạy cảm như bộ khôi phục của bạn, nên hãy cất giữ nó ở nơi an toàn. Không có chúng, tệp chỉ chứa các cài đặt không bí mật.
