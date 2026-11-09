# Tính năng

Một danh mục ngắn về những gì BombVault làm. Mỗi dòng liên kết tới trang giải thích nó.

## Phạm vi sao lưu {#backup-scope}

- **Containers**: thư mục appdata và định nghĩa container, với lựa chọn thư mục cho từng container. [Bản sao lưu và các mục](backups.md#what-is-saved)
- **VMs**: ảnh đĩa, XML và UEFI NVRAM qua SSH, với tắt máy êm ái, snapshot trực tiếp hoặc chỉ các khối đã đổi. VM có snapshot Unraid và đĩa trên zvol được hỗ trợ đầy đủ. [Sao lưu VM](vm-backup-ssh-setup.md)
- **Flash**: toàn bộ USB flash của Unraid, với bản xuất zip theo lịch. [Bản sao lưu và các mục](backups.md#settings-tab)
- **Tự sao lưu**: cài đặt và thông tin đăng nhập của chính BombVault. [Trình hướng dẫn khẩn cấp](offsite-recovery.md#emergency)
- **Thư mục**: bất kỳ thư mục nào trên máy chủ dưới dạng một bộ thư mục. [Thêm một thư mục](backups.md#add)
- **Tập dữ liệu ZFS**: một cây dataset đọc từ một snapshot, khôi phục được từng tệp và có bản nhân bản tùy chọn trên một máy chủ ZFS thứ hai. [Tập dữ liệu ZFS](zfs-datasets.md)
- **Bản kết xuất cơ sở dữ liệu**: PostgreSQL, MySQL và MariaDB, được kết xuất từ máy chủ đang chạy vào một điểm khôi phục riêng. [Kết xuất cơ sở dữ liệu](backups.md#database-dumps)

## Bản sao lưu và trang của từng mục {#backups}

- **Một danh sách** với các ô, **Bộ lọc**, **Sắp xếp** và **Tìm kiếm**, và mười bốn lần chạy gần nhất của mỗi dòng dưới dạng một dải. [Danh sách](backups.md#list)
- **Chọn như trong trình quản lý tệp**, và sao lưu hoặc tạm dừng một lựa chọn trên máy chủ. [Chọn nhiều mục](backups.md#select)
- **Một trang cho mỗi mục** với **Bản sao lưu**, **Cài đặt** và **Xác minh**. [Trang của một mục](backups.md#entry-page)
- **Thẻ và so sánh** cho snapshot, và **Xuất** dưới dạng tar, zip hoặc một kho restic. [Tab Bản sao lưu](backups.md#backups-tab)
- **Mẫu loại trừ** với bản xem trước trực tiếp, và các mục loại trừ được đề xuất cho những ứng dụng quen thuộc. [Mẫu loại trừ](backups.md#excludes)
- **Lệnh trước và sau**, và **Dừng các container khác** với việc khởi động lại có chờ tình trạng khỏe theo thứ tự phụ thuộc. [Lệnh và các container khác](backups.md#commands)
- **Khi ứng dụng đang bận**: một bản sao lưu theo lịch chờ Plex, Jellyfin, Emby, kiểm tra của riêng bạn hoặc lưu lượng yên tĩnh, rồi tự thử lại. [Khi ứng dụng đang bận](backups.md#app-busy)
- **Ảnh chụp thay cho dừng**: một container trên ZFS chỉ ngừng trong vài giây. [Lịch trình](schedules.md#snapshot-instead-of-stop)
- **Bản xuất thuần**, có thể niêm phong bằng age. [Bản xuất thuần](backups.md#exports)

## Khôi phục {#restore}

- **Một trang và một cửa sổ** cho mọi loại, với bản sao lưu được khuyên dùng chọn sẵn. [Khôi phục](restore.md)
- **Bên cạnh bản gốc** theo mặc định, và **Thay thế bản gốc** sao lưu trạng thái hiện tại trước. [Cửa sổ khôi phục](restore.md#window)
- **Từng tệp** vào một thư mục hoặc trở lại chỗ cũ. [Lấy lại một tệp](restore.md#example-single-file)
- **Nhiều mục cùng lúc**, mỗi mục từ bản sao lưu được khuyên dùng hoặc từ một thời điểm. [Khôi phục nhiều mục](restore.md#several)
- **Cả stack Compose**, khởi động theo thứ tự phụ thuộc. [Khôi phục một stack](restore.md#stacks)
- **Container và VM được tạo lại** trên image mà chúng đã chạy, với GPU, giới hạn và link. [Những gì trở lại](restore.md#kinds)
- **Kiểm tra trước khi khôi phục**, và danh sách những gì nó thay đổi. [Cửa sổ khôi phục](restore.md#window)
- **Flash dưới dạng ZIP**, và từng plugin một trở lại ổ flash đang chạy. [Những gì trở lại](restore.md#kinds)
- **ZFS** vào chính tập dữ liệu với snapshot an toàn, vào một thư mục hoặc vào một tập dữ liệu mới với các thuộc tính của nó. [Tập dữ liệu ZFS](zfs-datasets.md#restore)
- **Trình hướng dẫn khẩn cấp** cho một máy chủ đã mất, với bộ khôi phục, qua nhóm hoặc bằng khóa gõ tay. [Trình hướng dẫn khẩn cấp](offsite-recovery.md#emergency)
- **Từ thư mục sao lưu cũ**, **Nhập từ plugin Appdata.Backup**, và **Tác vụ từ Duplicati hoặc Duplicacy**, giúp chuyển các tác vụ sao lưu của hai công cụ này thành bộ thư mục. [Nguồn khác](restore.md#other-sources)

## Lưu trữ và lịch trình {#storage-scheduling}

- **Đích sao lưu** thuộc mọi loại: thư mục cục bộ, SMB, WebDAV, SFTP, rest-server, S3, ổ đám mây và bất kỳ rclone remote nào. [Đích sao lưu](offsite-recovery.md#destinations)
- **Thông tin đăng nhập**: một danh sách với một mặc định. [Thông tin đăng nhập](offsite-recovery.md#sign-ins)
- **Thư mục riêng** trên máy chủ này hoặc trong một đích sao lưu. [Thư mục riêng của một phần](offsite-recovery.md#own-folder)
- **3-2-1 như một luồng** nơi mỗi dòng là một công tắc, và **vị trí theo từng mục**. [Vị trí theo từng mục](offsite-recovery.md#placement)
- **Bản sao ngoài site** tới nhiều đích cho mỗi phần, với giới hạn băng thông, nén, lớp lưu trữ lạnh và giảm tốc trong lúc Plex đang phát. [Bản sao ngoài site](offsite-recovery.md#off-site-copies)
- **Kho trực tiếp**, được liệt kê dưới **Sao lưu thẳng vào đây**. [Kho trực tiếp](offsite-recovery.md#direct-repositories)
- **Chính sách lưu giữ dưới dạng dòng thời gian**: **Ngắn**, **Cân bằng**, **Dài** hoặc **Giá trị riêng** cho mỗi đích, với quy tắc theo từng phần. [Giữ trong bao lâu](offsite-recovery.md#keep)
- **Chuyển lịch sử của một phần**, và **Xóa đích sao lưu** có hoặc không kèm bản sao lưu của nó. [Di chuyển và xóa](offsite-recovery.md#move-remove)
- **Lịch trình đọc như một câu**, dải **Tuần**, một bản sao lưu toàn bộ, kế hoạch theo phần và kế hoạch riêng cho từng mục. [Lịch trình](schedules.md)
- **Chạy bù các bản sao lưu bị bỏ lỡ**, thứ tự thủ công và cập nhật sau khi sao lưu thành công. [Lịch trình](schedules.md)

## Thông tin, xác minh và giám sát {#insight-verification-monitoring}

- **Trang tổng quan**: phần đầu trạng thái, **Cần chú ý**, **Trạng thái bảo vệ**, dự báo lưu trữ, lịch sử và các thẻ tùy chọn. [Tổng quan](overview.md)
- **Phát hiện bất thường** cho từng mục, cũng giữ lại các bản sao lưu cũ khi một nguồn sụp đổ. [Bất thường](anomalies.md)
- **Kiểm tra khôi phục** sau bản sao lưu đầu tiên, và **Kiểm tra khởi động** trong một bản sao cô lập. [Tab Xác minh](backups.md#verify-tab)
- **Kiểm tra theo lịch và khôi phục thử** từ bản sao ngoài site. [Diễn tập DR](offsite-recovery.md#dr-drills)
- **Dung lượng theo thư mục**, và vì sao một bản sao lưu chậm. [Một lần chạy cho bạn biết gì](backups.md#run-notes)
- **Thông báo**: webhook, Gotify, Matrix, email, Apprise, Unraid và Healthchecks, với bản tóm tắt hằng tuần và báo cáo trễ hạn. [Thông báo](notifications.md)
- **Prometheus**, một thiết bị Home Assistant, một HTTP API và mDNS. [API và tích hợp](api.md)
- **Báo lỗi**: một tệp ZIP đã che thông tin nhạy cảm cho issue trên GitHub. [Xử lý sự cố](troubleshooting.md#still-stuck)

## Bảo vệ chống ransomware {#ransomware-protection}

- **Bảo vệ chống xóa (append-only)**, do phía bên kia thực thi và được chứng minh bằng phép thử phá hoại. [Bảo vệ chống xóa](offsite-recovery.md#delete-protection-append-only)
- **Chấp nhận khi không có bảo vệ chống xóa** cho một nơi không thể từ chối việc xóa. [Bảo vệ chống xóa](offsite-recovery.md#delete-protection-append-only)
- **Xóa vĩnh viễn các bản sao lưu cần chính bạn đăng nhập** khi đã đặt mật khẩu đăng nhập, và điện thoại đã ghép nối không làm được. [Mô hình bảo mật](configuration.md#security-model)
- **Bộ khôi phục**, có thể niêm phong bằng age, và một **bên giữ khóa** trong nhóm của bạn. [Bộ khôi phục](offsite-recovery.md#recovery-kit), [Bên giữ khóa](offsite-recovery.md#key-keeper)

## Nhiều máy chủ {#several-servers}

- **Ghép nối bằng mười hai từ**, mã hóa đầu cuối, trực tiếp hoặc qua relay. [Ghép nối các phiên bản](offsite-recovery.md#pairing)
- **Trang Phiên bản** với nhóm hiển thị như một mạng và các vai trò **Bộ nhận**, **Bên kéo**, **Máy chủ ZFS** và **Bên giữ khóa**. [Phiên bản và vai trò](offsite-recovery.md#roles)
- **Một máy chủ nhận** được thiết lập sẵn cho bạn, ở chế độ append-only. [Máy chủ nhận](offsite-recovery.md#receiving-server)

## Trợ lý AI (MCP) {#mcp}

Một máy chủ MCP tích hợp sẵn cho phép trợ lý đọc trạng thái sao lưu, độ bao phủ, lịch sử chạy và điểm khôi phục, và với một khóa cho phép thì bắt đầu được một bản sao lưu. Khôi phục, xóa, prune và cài đặt vẫn nằm trong giao diện web. [Máy chủ MCP](mcp.md)

## Ứng dụng và công cụ đi kèm {#apps}

- **Ứng dụng Android**: mọi máy chủ trong nhóm của bạn trên điện thoại. [Ứng dụng Android](android.md)
- **BombVault Widget**: một ô trên Dashboard của Unraid. [Ứng dụng](settings.md#apps)
- **Nhật ký hoạt động nhúng được** cho các bảng điều khiển như Homepage hoặc Heimdall. [Widget bảng điều khiển](api.md#dashboard-widget)

## Khác {#other}

- **42 ngôn ngữ**, ba giao diện, và nhãn, màu sắc, góc cùng hoạt ảnh tùy bạn chọn. [Giao diện](settings.md#look)
- **Cài đặt tự lưu**, với tìm kiếm trên cả mười bốn ô. [Cài đặt](settings.md)
- **Có gì mới** sau một bản cập nhật, với chấm xanh trên những gì đã đổi. [Thông tin](settings.md#info)
- **HTTPS có sẵn**, và một healthcheck cho các công cụ tự chữa. [Cấu hình](configuration.md#network)
