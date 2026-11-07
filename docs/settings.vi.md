# Cài đặt

**Cài đặt** gồm mười bốn ô. Mỗi ô là một trang gồm các thẻ, và mọi thay đổi được ghi ngay khi bạn thực hiện, với một nháy ngắn trên điều khiển, hoặc một cú rung khi máy chủ từ chối. Một vài cửa sổ giữ nút **Lưu** vì lưu dở dang sẽ không an toàn, chẳng hạn một thông tin đăng nhập hoặc mật khẩu đăng nhập. Phần giải thích nằm trong các bong bóng (i) bên cạnh mỗi điều khiển.

Bấm `/` hoặc Ctrl+K ở bất cứ đâu trong phần cài đặt để tìm trong tất cả, kể cả theo những tên mà các trang từng có ở các phiên bản trước. Thứ tự các ô có thể đổi trong **Giao diện**. Ở nơi một cột ô bên cạnh trang sẽ làm trang bị chật, như trong cửa sổ rộng khoảng 1200 pixel hoặc trên điện thoại, các ô trở thành một dải phía trên trang, cuộn ngang được.

![Cài đặt với ô Chung đang mở.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Cài đặt với ô Chung đang mở.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Trang này liệt kê mọi ô cùng các thẻ của nó, và liên kết tới trang giải thích sâu từng thẻ.

## Chung {#general}

- **Phần** bật hoặc tắt từng phần: **Containers**, **VMs**, **Flash**, **Thư mục**, **Tập dữ liệu ZFS** và **Tự sao lưu**. Một phần đã tắt thì không được sao lưu theo bất kỳ lịch nào, kể cả kế hoạch riêng của một mục, và các mục của nó không được tính. **Sao lưu ngay** vẫn hoạt động. **Flash** chỉ hiện trên Unraid.
- **Containers** chứa những gì áp dụng cho mọi container, chẳng hạn **Tự động kết xuất cơ sở dữ liệu**. Bất cứ thứ gì ảnh hưởng tới một container thì nằm trên trang của nó. Xem [Kết xuất cơ sở dữ liệu](backups.md#database-dumps).

## Giao diện {#look}

- **Giao diện**: **Ngôn ngữ** (42 ngôn ngữ), **Giao diện** (**Tối**, **Sáng** hoặc **Hệ thống**) với **OLED** bên dưới, cho màn hình OLED và AMOLED màu đen tuyền khi giao diện tối đang hiển thị, **Góc**, **Hoạt ảnh** (**Tắt**, **Nhẹ**, **Sôi động** hoặc **Bão**), **Thông báo im lặng**, ẩn các thông báo thành công như xác nhận đã lưu, và **Thứ tự các trang cài đặt**, nơi bạn giữ một ô rồi kéo vào vị trí, hoặc dùng Alt cùng các phím mũi tên.
- **Nhãn**: mức độ hiển thị của một điều khiển, riêng cho **Nút**, **Thanh bên**, **Thẻ** và **Thanh dưới**: **Chữ**, **Chữ và biểu tượng**, **Biểu tượng**, hoặc **Phản hồi**, chỉ hiện biểu tượng và trượt chữ ra khi bạn trỏ vào. **Thanh bên** chỉ ảnh hưởng bố cục máy tính và **Thanh dưới** chỉ ảnh hưởng bố cục điện thoại.
- **Màu sắc**: một **Màu nhấn**, hoặc **Chế độ cầu vồng**, nơi mỗi dòng của danh sách có màu riêng từ bảng tám màu. Chế độ cầu vồng thêm **Chế độ disco**, nơi các màu trượt chậm sang màu kế tiếp, **Chế độ phản ứng**, nơi màu của một dòng chỉ hiện khi bạn trỏ vào, khi nó chạy hoặc được chọn, **Luân chuyển màu** và chính **Bảng màu**. Cây đũa thần ở đầu các màu nhấn, **Màu hệ thống**, lấy màu riêng của thiết bị bạn: Material You trong ứng dụng Android từ Android 12 trở lên, và màu hệ thống trong Firefox và Safari. Nó chỉ hiện khi thiết bị có màu như vậy.

BombVault giữ giao diện trên máy chủ. Một trình duyệt lần đầu đến sẽ lấy nó. **OLED** và **Màu hệ thống** là ngoại lệ: mỗi thiết bị tự giữ chúng cho mình, vì chúng phụ thuộc vào màn hình của thiết bị. Khi hệ thống của bạn yêu cầu ít chuyển động hơn, điều đó luôn thắng **Hoạt ảnh**.

## Lịch trình {#schedule}

**Tuần**, **Sao lưu toàn bộ** với **Khi nào sao lưu chạy**, **Chạy bù các bản sao lưu bị bỏ lỡ sau khi khởi động** và **Lệnh trước và sau**, **Lịch trình theo phần** với **Bản sao ngoài site theo phần**, **Mục trong lịch trình**, **Thứ tự** với **Khởi động lại sau khi sao lưu**, **Ảnh chụp thay cho dừng**, **Cập nhật sau khi sao lưu** và **Rảnh trước khi sao lưu**. Xem [Lịch trình](schedules.md).

## Đích sao lưu {#storage-locations}

- **Hiện cả trong thanh bên** ghim trang Đích sao lưu vào thanh bên.
- Danh sách các đích sao lưu của bạn, với **Thêm đích**. Bấm vào để mở trang của một đích, xem [Trang của một đích sao lưu](offsite-recovery.md#location-page).
- **Thông tin đăng nhập**: các thông tin đăng nhập đã lưu với ngôi sao mặc định, xem [Thông tin đăng nhập](offsite-recovery.md#sign-ins).
- **Đích theo phần**: luồng 3-2-1, nơi mỗi dòng là một công tắc, xem [Mặc định theo phần](offsite-recovery.md#placement-defaults).
- **Kho**: các kho được tạo cho từng mục, xem [Vị trí theo từng mục](offsite-recovery.md#placement).
- **Giữ trong bao lâu** chỉ tới các đích sao lưu, nơi chính sách lưu giữ được đặt, và **Xem trước** hiện **Những gì lần chạy tới sẽ xóa**, theo từng kho và từng mục, trước khi nó xảy ra. Nó không thay đổi hay khóa gì cả, nên trả lời được ngay cả khi một bản sao lưu đang chạy.
- **Bản sao ngoài site**: mỗi phần một dòng với **Kiểm tra kết nối**, **Sao chép ngay** và **Thiết lập** hoặc **Chỉnh sửa**, xem [Bản sao ngoài site](offsite-recovery.md#off-site-copies).
- **Băng thông off-site**: **Giới hạn tải lên (KiB/s)** và **Giới hạn tải xuống (KiB/s)** cho mọi bản sao. 0 nghĩa là không giới hạn.

Thẻ **Phát trực tuyến**, giảm giới hạn tải lên khi Plex, Jellyfin hoặc Emby đang phát, nằm ngay trên trang Đích sao lưu.

## Khôi phục {#restore}

- Một liên kết tới trang **Khôi phục**.
- **Thư mục khôi phục**: **Thư mục khôi phục mặc định** cho một lần khôi phục vào thư mục, với **Thư mục con cho mỗi phần** và một **Ngoại lệ** cho mỗi phần. Xem [Một lần khôi phục vào thư mục đi đâu](restore.md#folders).
- **Registry container**: thông tin đăng nhập cho các registry riêng, chẳng hạn một image `ghcr.io` dành cho nhà tài trợ. Một lần khôi phục dùng chúng để kéo image còn thiếu, và một lần cập nhật sau khi sao lưu dùng để kiểm tra bản mới hơn. Image công khai được kéo mà không cần. **Thêm registry** đưa ra các registry công khai, loại tự lưu trữ, loại ở nhà cung cấp đám mây và **Registry khác** cho bất kỳ địa chỉ nào.
- **Cho trường hợp khẩn cấp**: **Tải bộ khôi phục**, cần mật khẩu đăng nhập. Xem [Bộ khôi phục](offsite-recovery.md#recovery-kit).

## Thông báo {#notifications}

**Thông báo** với **Thông báo**, các bản tóm tắt, **Báo cáo bản sao lưu bị trễ**, **Thông báo Unraid** và **Tóm tắt hằng tuần**, **Kênh** với Webhook, Apprise, Matrix, Gotify và Email (SMTP), và **Healthchecks**. Xem [Thông báo](notifications.md).

## Xác minh {#verify}

- **Kiểm tra bản sao lưu**: kết quả của `restic check` gần nhất theo từng phần, với **Mở khóa** và **Dọn bớt** dưới **Thêm** trong mỗi dòng. **Mở khóa** xóa một khóa cũ, còn **Dọn bớt** áp dụng quy tắc giữ lại theo yêu cầu.
- **Kiểm tra tự động**: kiểm tra khôi phục theo lịch với **Mẫu dữ liệu (%)** và các đích thử, **Kiểm tra khởi động**, cũng khôi phục một container vào một bản sao cô lập và khởi động nó, **Diễn tập DR ngoài site theo lịch** và **Kiểm tra bảo vệ chống xóa**, phép thử phá hoại theo lịch. Xem [Diễn tập DR](offsite-recovery.md#dr-drills) và [Phép thử phá hoại](offsite-recovery.md#tamper-test).
- **Bất thường**: phát hiện, độ nhạy và những phát hiện nào gửi tin nhắn. Xem [Bất thường](anomalies.md).

## Bảo mật {#security}

- **Mật khẩu**: mật khẩu đăng nhập, ít nhất 12 ký tự. Đổi hoặc xóa nó cần mật khẩu hiện tại và, khi bật xác thực hai yếu tố, cả mã.
- **Xác thực hai yếu tố**: một mã từ ứng dụng xác thực thêm vào mật khẩu, với tám mã khôi phục chỉ hiện một lần.
- **Khóa truy cập**: đăng nhập bằng một khóa trên điện thoại, laptop hoặc thiết bị bảo mật. Chúng cần một tên miền thật và một chứng chỉ mà trình duyệt tin cậy.
- **Ứng dụng Android**: **Điện thoại có thể ghép nối bằng mười hai từ của nhóm**, cho phép một điện thoại đã ghép nối đăng nhập mà không cần mật khẩu.
- **Mã hóa**: **Mã hóa bản sao lưu**, bật theo mặc định.

Một số việc cần mật khẩu đăng nhập, chẳng hạn bộ khôi phục và ký gửi khóa. Xem [Mô hình bảo mật](configuration.md#security-model).

## Phiên bản {#instances}

- **Hiện cả trong thanh bên** ghim trang Phiên bản vào thanh bên.
- **Nhiều máy chủ** bật các phần của một nhóm: **Phiên bản**, trang có một thẻ cho mỗi thành viên, **Bộ nhận** và **Kéo về**. Xem [Phiên bản và vai trò](offsite-recovery.md#roles).
- Một liên kết tới trang Phiên bản, với các phần đang bật.

## Ghép nối {#pairing}

**Ghép nối** với **Tạo cụm từ** hoặc **Nhập cụm từ**, các thẻ từng bước, tên của phiên bản này và **Địa chỉ đã biết**, thẻ **Relay** với bốn dạng của nó và **Những gì relay thấy**, các thành viên của nhóm, và **Máy chủ ZFS không có BombVault** cho đích bản sao qua SSH. Xem [Ghép nối các phiên bản](offsite-recovery.md#pairing) và [Đẩy tới máy chủ ZFS](zfs-datasets.md#replica-push).

## Tích hợp {#integrations}

- **Máy chủ MCP**: các khóa cho trợ lý AI, **Chọn máy khách** và nhật ký của từng khóa. Xem [Máy chủ MCP](mcp.md).
- **Mã thông báo API**: các token có tên cho script và bảng điều khiển. Xem [Token](api.md#tokens).
- **Dịch vụ**: **Home Assistant** qua MQTT, **Tìm trên mạng** (mDNS dưới dạng `bombvault.local`) và **Cung cấp /metrics** cho Prometheus, với **Token scrape (tùy chọn)**. Xem [Home Assistant](api.md#home-assistant) và [Tìm BombVault trên mạng](api.md#mdns).
- **Widget bảng điều khiển**: một token chỉ đọc và một địa chỉ cho bất kỳ bảng điều khiển nào hiển thị iframe, như Homepage hoặc Heimdall, chỉ với nhật ký hoạt động trực tiếp. Xem [Widget bảng điều khiển](api.md#dashboard-widget).
- **SSH máy chủ**: khóa công khai của BombVault và **Kiểm tra kết nối**, cho bản sao lưu VM và tập dữ liệu ZFS cùng thông báo Unraid. Xem [hướng dẫn sao lưu VM](vm-backup-ssh-setup.md).

## Ứng dụng {#apps}

- **Ứng dụng Android**: APK cho bản phát hành mà máy chủ của bạn đang chạy kèm mã QR, và mục trên F-Droid. Xem [Ứng dụng Android](android.md#install).
- **ParleyPort**: container relay với mẫu Unraid của nó và lệnh Docker để khởi động nó.
- **BombVault Widget**: một ô trên Dashboard của Unraid với nhật ký hoạt động của BombVault và lần chạy tiếp theo. Có kết nối SSH máy chủ thì thẻ tự cài và gỡ plugin. Không có thì nó đưa cho bạn địa chỉ `.plg` để cài dưới **Plugins, Install Plugin**.

## Hệ thống {#system}

- **Xuất / nhập cài đặt**: toàn bộ cấu hình của bạn dưới dạng một tệp JSON di động, có hoặc không có thông tin xác thực. Nhập sẽ hiện **Tệp này chứa** trước và bỏ qua các liên kết tới đích sao lưu đã không còn. **Mã hóa bản xuất bằng age** niêm phong các bản xuất thuần, ZIP của flash và bộ khôi phục cho **Người nhận age** mà bạn liệt kê. Xem [Cài đặt di động](configuration.md#portable-settings-export-and-import).
- **Hiệu năng**: **Luồng CPU (0 = tất cả)**, **Giới hạn kích thước bộ nhớ đệm (MB, 0 = không giới hạn)** cho bộ nhớ đệm của restic dưới `/config`, và **Giới hạn bộ nhớ mỗi bản sao lưu (MiB, 0 = không)**, một giới hạn mềm khiến một bản sao lưu lớn chạy chậm lại thay vì bị kết thúc vì hết bộ nhớ.

## Thông tin {#info}

- **Phiên bản**: BombVault, GlimStone, restic và rclone, với **Kiểm tra cập nhật** và **Mới**, đếm những thay đổi bạn chưa xem và mở một cửa sổ liệt kê chúng.
- **Trợ giúp**: **Hướng dẫn** mở tài liệu này, và **Báo lỗi** tải về một tệp ZIP đã che thông tin nhạy cảm cho một issue trên GitHub. Nó cần mật khẩu đăng nhập.
- **Giới thiệu về BombVault**: dự án, giấy phép của nó và cách liên lạc với tác giả.
