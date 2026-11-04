# Bản sao lưu và các mục

Mọi thứ BombVault sao lưu đều nằm trong một danh sách dưới **Bản sao lưu**: container, VM, bộ thư mục, ổ flash, tập dữ liệu ZFS và tự sao lưu. Mỗi cái là một **mục** có trang riêng. Trang này đi qua danh sách, trang của một mục cùng ba tab của nó, và các cài đặt mà một mục có thể có.

![Danh sách Bản sao lưu với ô Containers được chọn: mỗi dòng hiện các lần chạy gần nhất dưới dạng một dải và một trạng thái.](assets/screenshots/backups/list-dark.png#only-dark)
![Danh sách Bản sao lưu với ô Containers được chọn: mỗi dòng hiện các lần chạy gần nhất dưới dạng một dải và một trạng thái.](assets/screenshots/backups/list-light.png#only-light)

## Mỗi phần lưu những gì {#what-is-saved}

| Phần | Những gì được lưu |
|---|---|
| **Containers** | Thư mục appdata cùng định nghĩa container: image, biến, cổng, nhãn và volume. Thẻ **Thư mục sao lưu** ở tab **Cài đặt** của container đánh dấu chính xác thư mục nào được bản sao lưu bao phủ, với số đường dẫn đếm trực tiếp và danh sách những gì bạn đã bỏ ra. Dữ liệu mà container giữ bên ngoài Host Data, chẳng hạn trong một ixVolume của TrueNAS, không được sao lưu, và lần chạy nêu tên các thư mục bị bỏ ra. |
| **VMs** | Các ảnh đĩa, định nghĩa XML và UEFI NVRAM, với việc tắt máy êm ái hoặc snapshot trực tiếp, qua SSH. Xem [Sao lưu VM](vm-backup-ssh-setup.md). |
| **Flash** | Chỉ Unraid. Toàn bộ USB flash (`/boot`): hệ điều hành, giấy phép, cấu hình mảng, share, mạng và cấu hình plugin. |
| **Tự sao lưu** | `/config` của chính BombVault: cơ sở dữ liệu cài đặt, thông tin đăng nhập của các đích sao lưu và cặp khóa SSH libvirt, được chụp bằng SQLite `VACUUM INTO` để cơ sở dữ liệu không bao giờ bị lấy giữa lúc đang ghi. |
| **Thư mục** | Bất kỳ thư mục nào trên máy chủ dưới dạng một bộ thư mục, chẳng hạn một share, tài liệu của bạn hoặc thư viện ảnh, với các mẫu loại trừ tùy chọn. Bộ thư mục có lịch trình, chính sách lưu giữ, bản sao ngoài site, kiểm tra và diễn tập khôi phục như mọi thứ khác. |
| **Tập dữ liệu ZFS** | Một dataset và mọi dataset bên dưới nó, đọc từ một snapshot ZFS để tất cả đến từ cùng một thời điểm, và lưu bằng restic như một thư mục. Một mục ZFS cũng có thể giữ một bản sao khớp từng khối trên một máy chủ ZFS thứ hai. Xem [Tập dữ liệu ZFS](zfs-datasets.md). |

Mọi bản sao lưu đều đi qua restic, nên nó tăng dần và được khử trùng lặp, và ngay cả đĩa VM lớn cũng không làm kho phình ra. BombVault không bao giờ sao lưu, và vì vậy không bao giờ dừng, container của chính nó. Một lần chạy có thể được dừng an toàn, vì restic ghi snapshot của nó sau cùng, nên một lần chạy bị hủy chỉ để lại dữ liệu không được tham chiếu và không có snapshot. Một khóa bị bỏ lại do khởi động lại giữa lúc chạy sẽ được xóa và lần chạy được thử lại một lần, và chỉ sau khi khóa đã mười phút không được làm mới.

## Danh sách {#list}

Các ô phía trên danh sách chọn một loại, chẳng hạn **Containers** hoặc **ZFS**, và mỗi ô đếm số mục của nó. Một ô chỉ chọn, không ẩn phần còn lại của trang.

Mỗi dòng hiện mười bốn lần chạy gần nhất của mục dưới dạng các ô vuông nhỏ và một trạng thái, chẳng hạn **Được bảo vệ**, **Không được bảo vệ**, **Đã hoãn** hoặc **Đang nhân bản**. Mọi thứ khác về nó nằm trong bong bóng thông tin bên cạnh tên, và khi con trỏ nằm trên dòng thì văn bản đó hiện ngay trong dòng. Một dấu bên cạnh tên container cho biết nó đã được tạo lại với image, cổng, biến hoặc volume khác kể từ bản sao lưu gần nhất. Dấu mất đi sau bản sao lưu kế tiếp.

Ba nút phía trên danh sách giúp thu hẹp nó:

- **Bộ lọc**: theo **Lịch trình** (**Tất cả**, **Đã lên lịch**, **Tạm dừng**), theo **Sao lưu** (**Đã sao lưu**, **Chưa từng sao lưu**) và, với **Containers và VMs**, **Đã cài đặt** hoặc **Chưa cài đặt**.
- **Sắp xếp**: theo tên, trạng thái, IP, bản sao lưu gần nhất, kích thước hoặc lịch trình. Mọi trường trừ IP đều sắp xếp được hai chiều, chẳng hạn **Kích thước, lớn nhất trước** và **Kích thước, nhỏ nhất trước**.
- **Tìm kiếm**: theo tên, image hoặc IP.

Các mục không còn được cài nhưng vẫn có bản sao lưu nằm riêng dưới **Chưa cài đặt (chỉ có bản sao lưu)**. Hãy khôi phục chúng, hoặc xóa bản sao lưu của chúng để giải phóng chỗ.

Hành động chính của trang nổi ở góc dưới bên phải: **Sao lưu tất cả ngay** bắt đầu một lượt chạy trên mọi phần đang bật. **Thêm** chứa phần còn lại:

- **Chọn** bắt đầu một lựa chọn, xem bên dưới.
- **Tìm bản sao lưu** đọc những gì các đích sao lưu đang chứa và dựng lại các mục từ đó, ví dụ sau một lần cài mới. Các bản sao ngoài site của những phần được dựng lại sẽ tạm dừng cho đến khi bạn xác nhận mặc định của chúng, xem [Sau khi dựng lại](offsite-recovery.md#after-a-rebuild).
- **Kết nối ZFS tới máy chủ này** mở phép thử SSH của phần ZFS, xem [Tập dữ liệu ZFS](zfs-datasets.md#requirements).
- **Nhân bản tất cả ngay**, khi ô ZFS được chọn, gửi bản sao của mọi mục ZFS.

![Cùng danh sách đó trên điện thoại.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Cùng danh sách đó trên điện thoại.](assets/screenshots/backups/list-phone-light.png#only-light)

## Chọn nhiều mục {#select}

Chọn các dòng như cách bạn chọn tệp trong trình quản lý tệp: Ctrl+bấm, Shift+bấm, Ctrl+A, hoặc nhấn giữ trên điện thoại. Một dòng đã chọn hiện dấu kiểm thay cho biểu tượng, và một thanh phía trên danh sách đếm chúng và đưa ra **Sao lưu đã chọn**, **Tạm dừng lịch trình** và **Tiếp tục lịch trình**, **Chọn tất cả** và **Bỏ chọn**. Mỗi dòng cũng có menu chuột phải với các hành động riêng.

Một lựa chọn được sao lưu trên máy chủ, lần lượt từng mục, nên nó vẫn tiếp tục khi bạn đóng tab. Để khôi phục nhiều mục cùng nhau, hãy dùng [trang Khôi phục](restore.md#several).

![Hai container được chọn, với thanh phía trên danh sách.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Hai container được chọn, với thanh phía trên danh sách.](assets/screenshots/backups/bulk-light.png#only-light)

## Thêm một thư mục hoặc tập dữ liệu ZFS {#add}

Container, VM và ổ flash tự xuất hiện khi phần của chúng được bật trong **Cài đặt, Chung**. Thư mục và tập dữ liệu ZFS thì bạn tự chọn, vì BombVault không thể đoán bạn muốn cái nào. **Thêm** mở **BombVault nên sao lưu thêm gì?**:

- **Một thư mục** liệt kê các share mà BombVault tìm thấy và nhận bất kỳ thư mục nào khác bên dưới Host Data. Đặt tên cho bộ thư mục, chẳng hạn `Documents`. Tên được cố định sau bản sao lưu đầu tiên, vì các snapshot được lưu dưới tên đó.
- **Một tập dữ liệu ZFS** liệt kê các dataset của máy chủ này. Chọn cái ở trên cùng của những gì bạn muốn sao lưu, xem [Các mục và dataset con](zfs-datasets.md#items-and-children).
- **Cấu hình hệ thống** xuất hiện trên một máy chủ không có Unraid mà Host Data của nó chạm tới `/etc`. Đó là một bộ thư mục `/etc` khởi đầu để bạn xem lại trước khi lưu.

## Bỏ một mục ra {#dont-back-up}

Một mục bạn cố ý bỏ ra, chẳng hạn một container thử nghiệm, có thể được đánh dấu **Không sao lưu**. Khi đó nó không còn được tính là chưa được bảo vệ, và **Bỏ loại trừ** đưa nó trở lại. **Tạm dừng lịch trình** là cách nhẹ nhàng hơn: mục rời khỏi lịch và bản sao lưu toàn bộ, mang huy hiệu xám **Lịch trình đang tạm dừng**, và **Sao lưu ngay** vẫn dùng được. Nó đặt cùng công tắc với **Trong lịch trình** trên tab Cài đặt của mục.

## Trang của một mục {#entry-page}

Bấm vào một dòng sẽ mở trang của mục. Phần đầu nêu tên mục và trạng thái của nó trong một dòng, chẳng hạn **Sao lưu lần cuối 3 giờ trước** hoặc **Đã hoãn, thử lại lúc 03:15**, và chứa các hành động: **Sao lưu ngay**, **Khôi phục**, và dưới **Thêm** là những cái hiếm dùng, như **Xuất bản sao lưu mới nhất**, **Xuất container** hoặc **Xuất máy ảo**, **Liên kết với mục cũ**, **Xóa tất cả bản sao lưu** và **Xóa mục**. Nút quay lại bên trái biểu tượng dẫn về danh sách. Khi một bản sao lưu đang chạy, **Hủy sao lưu** nằm cạnh tiến trình của nó, và một lần chạy bị hủy được ghi là đã hủy, không phải thất bại.

Trang có ba tab: **Bản sao lưu**, **Cài đặt** và **Xác minh**.

### Tab Bản sao lưu {#backups-tab}

Tab này liệt kê các snapshot của mục, mới nhất trước, với một thanh cho mỗi ngày. Một bản sao lưu nằm ở nhiều nơi chỉ xuất hiện một lần, kèm dấu của từng đích sao lưu đang giữ nó. Bấm vào một snapshot sẽ khôi phục nó, và bấm vào một vị trí bên trong snapshot sẽ khôi phục từ đúng chỗ đó.

- **Đích sao lưu** nêu tên mọi nơi có bản sao lưu của mục này và có bao nhiêu bản. Các nơi ngoài site chỉ được đọc khi bạn bấm **Kiểm tra**, vì liệt kê bộ nhớ đám mây mất một lúc. Một nơi mà mục không còn sao lưu tới được hiển thị là **Chỉ các bản sao lưu cũ hơn** và vẫn có thể khôi phục từ đó.
- Mỗi snapshot có một menu: **Thêm thẻ** hoặc **Đổi thẻ**, **So sánh với bản sao lưu khác**, liệt kê các tệp đã đến, đã đi hoặc đã đổi, **Xuất** dưới dạng tar, zip hoặc một kho restic riêng, và xóa chỉ ở một nơi.
- **Bản kết xuất cơ sở dữ liệu** của một container nằm trong danh sách riêng, với tải xuống, lưu vào thư mục và nhập.

Số snapshot được giữ lại do đích sao lưu đặt, xem [Giữ trong bao lâu](offsite-recovery.md#keep).

![Tab Bản sao lưu của một container: một thanh cho mỗi ngày, và các đích sao lưu đang giữ từng snapshot.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Tab Bản sao lưu của một container: một thanh cho mỗi ngày, và các đích sao lưu đang giữ từng snapshot.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Tab Cài đặt {#settings-tab}

Mỗi mục bắt đầu với **Lịch trình và đích sao lưu**:

- **Trong lịch trình** đưa mục vào hoặc ra khỏi các lần chạy theo lịch.
- **Kế hoạch riêng** cho riêng mục này một kế hoạch, xem [Kế hoạch riêng cho một mục](schedules.md#own-plan). Nếu không có, nó chạy theo kế hoạch của phần hoặc trong bản sao lưu toàn bộ.
- Một hàng nút, **Cục bộ** và một nút cho mỗi đích sao lưu ngoài site, cho biết bản sao lưu đi đâu. **Theo mặc định** nghĩa là mục đi tới nơi phần của nó đi, và bấm vào đó sẽ mở **Đích theo phần**. Xem [Vị trí theo từng mục](offsite-recovery.md#placement).
- **Kho** nêu tên nơi bản sao lưu được ghi (**Lưu tại**). Nơi đó được cố định từ bản sao lưu đầu tiên, còn các bản sao có thể đổi bất cứ lúc nào.

Các thẻ bên dưới phụ thuộc vào loại mục:

| Loại | Các thẻ |
|---|---|
| Container | **Thư mục sao lưu** (thư mục đã ánh xạ nào được sao lưu, với **Bỏ qua thư mục bộ nhớ đệm (CACHEDIR.TAG)**), **Mẫu loại trừ**, **Dừng các container khác**, **Lệnh trước và sau**, **Cơ sở dữ liệu**, **Khi ứng dụng đang bận**, **Cập nhật sau khi sao lưu thành công**, **Ảnh chụp thay cho dừng**, **Tên trước đây** |
| VM | **VM** với **Phương thức** (**Êm ái (tắt máy)** hoặc **Snapshot trực tiếp**) và **Chỉ các khối đã đổi**, xem [hướng dẫn sao lưu VM](vm-backup-ssh-setup.md#per-vm-method) |
| Bộ thư mục | **Thư mục** và **Mẫu loại trừ** |
| Ổ flash | **Xuất ZIP flash**, ghi mọi bản sao lưu flash ra thành một tệp `.zip` thuần vào thư mục bạn chọn |
| Tập dữ liệu ZFS | các dataset con, các container cần dừng và các lệnh quanh snapshot, **Bản nhân bản** và **Thử truy cập ảnh chụp**, xem [Tập dữ liệu ZFS](zfs-datasets.md) |

Cài đặt tự lưu khi bạn thay đổi.

![Tab Cài đặt của một container, với Kế hoạch riêng trong thẻ đầu tiên.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Tab Cài đặt của một container, với Kế hoạch riêng trong thẻ đầu tiên.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Tab Xác minh {#verify-tab}

- **Kiểm tra** hiện phép kiểm tra khôi phục chạy sau bản sao lưu đầu tiên. **Kiểm tra khôi phục** chạy lại nó trên bản sao lưu mới nhất: BombVault khôi phục một mẫu tối đa 100 tệp và 256 MiB vào một thư mục tạm và để restic đọc lại từng tệp đối chiếu với mã băm của nó.
- **Kiểm tra khởi động**, trên một container, khôi phục bản sao lưu mới nhất vào một bản sao cô lập và khởi động nó: tên bắt đầu bằng `bombvault-test-`, một mạng Docker nội bộ riêng không có cổng công bố và không có đường ra LAN, 1 CPU và 2 GiB bộ nhớ. Nó đạt khi bản sao báo khỏe, trả lời ở cổng đầu tiên được mở, hoặc, nếu cả hai đều không có, vẫn tiếp tục chạy. Bản gốc không bao giờ bị chạm tới, và bản sao, mạng và dữ liệu của nó được xóa sau đó, kể cả sau khi khởi động lại giữa lúc kiểm tra. Các container trên mạng của máy chủ, loại đặc quyền, loại có thiết bị và loại cần một container khác được hiển thị là không kiểm tra được. **Kiểm tra khởi động** trong **Cài đặt, Xác minh, Kiểm tra tự động** thử một container mỗi lần chạy, cái được thử lâu nhất trước.
- **Bất thường** đặt độ nhạy và mức thông báo riêng của mục này và hiện các phát hiện đang mở của nó.
- **Dung lượng theo thư mục** cho thấy cái gì chiếm chỗ trong bản sao lưu mới nhất và bản sao lưu gần nhất mang vào bao nhiêu, từng cấp một.
- **Lần chạy gần đây** liệt kê các lần chạy cuối với lúc bắt đầu, kết thúc và thời lượng. Bấm vào để xem chi tiết.

![Tab Xác minh của một container với Dung lượng theo thư mục đang mở.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Tab Xác minh của một container với Dung lượng theo thư mục đang mở.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Mẫu loại trừ {#excludes}

Thẻ **Mẫu loại trừ** nhận mỗi dòng một mẫu. Gõ đường dẫn như bạn thấy bên trong container, chẳng hạn `/config/Library/Application Support/Plex Media Server/Cache`, hoặc một tên trần như `.git`, khớp ở mọi độ sâu. Một bản xem trước dưới ô cho thấy mỗi dòng được hiểu thành gì, và cảnh báo khi một dòng không khớp gì hoặc sẽ loại trừ tất cả. Một dòng `/` hoặc `*`, hoặc dòng nêu cả một thư mục được sao lưu, bị bỏ qua, nên một mẫu loại trừ không bao giờ làm rỗng một bản sao lưu. Để bỏ cả một thư mục ra, hãy bỏ chọn nó dưới **Thư mục sao lưu**.

Với các image quen thuộc như Plex, Jellyfin, Sonarr, Radarr, Immich và Nextcloud, thẻ đưa ra những thư mục mà ứng dụng tự làm đầy lại, như bộ nhớ đệm, log và ảnh thu nhỏ. Không có gì bị loại trừ cho đến khi bạn bấm **Loại trừ mục đã chọn**.

Cùng thẻ đó mang các cảnh báo mà việc quét thư mục không thấy được. Chẳng hạn Immich giữ album, khuôn mặt và ngày tháng trong cơ sở dữ liệu PostgreSQL ở một container riêng, nên chỉ sao lưu tệp của container Immich thì khôi phục được ảnh mà không có gì trong số đó, và việc khôi phục trông như đã thành công. Hãy sao lưu cả container cơ sở dữ liệu.

## Kết xuất cơ sở dữ liệu {#database-dumps}

Các container PostgreSQL, MySQL và MariaDB mà BombVault nhận ra sẽ được kết xuất trước mỗi bản sao lưu, từ máy chủ đang chạy, vào một điểm khôi phục riêng bên cạnh các tệp. Điều đó bao gồm các image chính thức, PostGIS, TimescaleDB, pgvector, pgautoupgrade, các image cơ sở dữ liệu của Immich, các image MariaDB của linuxserver, yobasystems và jc21 và mysql-server của Oracle. Một container chỉ trông giống cơ sở dữ liệu cũng có cùng tùy chọn, tắt cho đến khi bạn chọn.

Bản kết xuất được truyền thẳng vào kho và không bao giờ được ghi ra đĩa. Thông tin xác thực lấy từ các biến của chính container, kể cả bí mật `*_FILE`, và không bao giờ rời khỏi nó. Thẻ **Cơ sở dữ liệu** trên tab Cài đặt của container cho biết thư mục dữ liệu được lưu khi dừng, được chép khi đang chạy, hay không được lưu, và bật hoặc tắt việc kết xuất cho container này. Nhãn `bombvault.dbdump=false` làm điều tương tự và thắng thẻ. **Tự động kết xuất cơ sở dữ liệu** trong **Cài đặt, Chung** bật hoặc tắt cho tất cả.

Một bản kết xuất lỗi không làm hỏng bản sao lưu. Nó hiện thành một lần chạy lỗi riêng kèm lý do và gửi thông báo. Bản kết xuất không bao giờ tự được nạp lại: tải một bản xuống, thuần hoặc nén, lưu vào một thư mục, hoặc nhập vào một cơ sở dữ liệu vừa khởi động chỉ với một cú bấm trên tab **Bản sao lưu**. Để lấy lại một bản kết xuất mà không cần BombVault, xem [Lấy lại một bản kết xuất cơ sở dữ liệu](offsite-recovery.md#database-dumps). Khi một bản lỗi, xem [Một bản kết xuất cơ sở dữ liệu bị lỗi](troubleshooting.md#a-database-dump-failed).

## Lệnh và các container khác {#commands}

- **Lệnh trước và sau** chạy bên trong container với `sh -c`, ví dụ để ghi một bộ nhớ đệm xuống đĩa. Một lệnh lỗi trước bản sao lưu sẽ hủy nó. Một lệnh trước bản sao lưu thoát với mã 75 thì hoãn một bản sao lưu theo lịch, xem bên dưới.
- **Dừng các container khác** nêu tên các container cần dừng trong lúc container này được sao lưu, chẳng hạn cơ sở dữ liệu của nó. Sau đó chúng khởi động lại theo thứ tự `depends_on` trong Compose, và theo mặc định mỗi container phải báo khỏe trước khi cái kế tiếp khởi động. Việc chờ và thời gian chờ của nó nằm dưới **Khởi động lại sau khi sao lưu** trong thẻ **Thứ tự** thuộc **Cài đặt, Lịch trình**.

## Khi ứng dụng đang bận {#app-busy}

Một bản sao lưu dừng container của nó, nên một bản theo lịch có thể chờ trong lúc có người dùng ứng dụng và tự thử lại. Thẻ **Khi ứng dụng đang bận** trên tab Cài đặt của container thiết lập việc này:

- **Kiểm tra hoạt động** hỏi Plex, Jellyfin hoặc Emby trước một bản sao lưu theo lịch xem có ai đang xem không. Plex được hỏi bằng token trong `Preferences.xml` của chính nó, nên không có gì để nhập. Jellyfin và Emby cần một **Khóa API**, tạo trong bảng điều khiển của máy chủ ở mục API keys.
- **Bỏ qua phiên tạm dừng sau (phút)** cho phép một bộ phim tạm dừng qua đêm thôi không được tính.
- **Sao lưu khi máy chủ không trả lời** quyết định chuyện gì xảy ra khi không hỏi được máy chủ: bật thì bản sao lưu chạy và nhật ký ghi như vậy. Tắt thì nó chờ như thể có người đang xem.
- **Chờ khi container đang gửi dữ liệu** dành cho các ứng dụng không có kiểm tra như vậy, chẳng hạn máy chủ game hoặc công cụ tải xuống. Bản sao lưu chờ khi container đã gửi nhiều hơn **Bận từ (KiB/s)** ở bất kỳ lần đọc nào trong **Yên trong (phút)** phút gần nhất.
- **Thử lại mỗi (phút)**, **Chỉ thử trong một khung giờ** với **Thử trong khoảng**, và **Chờ tối đa (giờ)** đặt cách các lần thử diễn ra.
- **Khi hết thời gian chờ** hoặc bỏ cuộc, khi đó bản sao lưu hiện dưới **Cần chú ý** với **Thử lại**, hoặc **Vẫn sao lưu**.

Một bản sao lưu đang chờ được ghi là **Đã hoãn**, màu hổ phách, và phần còn lại của lần chạy vẫn tiếp tục. Tổng quan, danh sách và mục hiển thị nó cùng lần thử tiếp theo, còn cảnh báo trễ hạn và trạng thái bảo vệ chờ nó đến hạn chót. Bạn nhận một thông báo khi nó bị hoãn và một khi nó kết thúc, không phải một thông báo mỗi lần thử. Một lần thử chờ trong lúc một bản sao lưu khác chạy, và việc chờ tiếp tục sau khi khởi động lại. Một bản sao lưu bạn tự bắt đầu luôn chạy ngay, dù bằng **Sao lưu ngay**, **Thử lại**, một lựa chọn, API hay MCP, và một bản chạy qua sẽ kết thúc việc chờ đang treo.

Tab Cài đặt của container cũng có một cách cũ hơn, đơn giản hơn: **Chờ đến khi ứng dụng rảnh** với **Chờ tối đa (giờ)**. Bản sao lưu bắt đầu ngay khi ứng dụng rảnh, điều mà thẻ **Rảnh trước khi sao lưu** trong **Cài đặt, Lịch trình** định nghĩa. Các thành viên của một stack Compose đến hạn trong cùng lần chạy chờ cùng nhau, và bản sao lưu đang chờ không giữ khóa nào, nên các container khác cứ đi tiếp.

![Thẻ Khi ứng dụng đang bận của một container Plex với kiểm tra hoạt động đã thiết lập.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Thẻ Khi ứng dụng đang bận của một container Plex với kiểm tra hoạt động đã thiết lập.](assets/screenshots/backups/app-busy-light.png#only-light)

### Ví dụ: Plex chờ trong lúc có người đang xem {#example-plex}

Bản sao lưu toàn bộ chạy mỗi đêm lúc 03:00. Không ai nên thấy bộ phim dừng vì Plex tắt để sao lưu, nhưng bản sao lưu vẫn nên xảy ra trước buổi sáng.

1. Mở **Bản sao lưu**, rồi mục `plex`, rồi tab **Cài đặt** của nó.
2. Trong **Khi ứng dụng đang bận**, đặt **Kiểm tra hoạt động** thành Plex. Không có gì để gõ.
3. Đặt **Bỏ qua phiên tạm dừng sau (phút)** thành `30`, để một bộ phim tạm dừng lúc nửa đêm thôi được tính lúc mười hai giờ rưỡi.
4. Để **Sao lưu khi máy chủ không trả lời** đang bật.
5. Đặt **Thử lại mỗi (phút)** thành `15`.
6. Bật **Chỉ thử trong một khung giờ** và đặt **Thử trong khoảng** từ `22:00` đến `06:00`.
7. Đặt **Chờ tối đa (giờ)** thành `3` và **Khi hết thời gian chờ** thành **Vẫn sao lưu**.

Chuyện gì xảy ra vào đêm có người vẫn đang xem lúc 03:00: BombVault hỏi Plex, thấy một phiên đang phát, và ghi bản sao lưu là **Đã hoãn** với lần thử tiếp theo lúc 03:15. Các container khác của lần chạy được sao lưu như thường. Khi bộ phim kết thúc lúc 04:10, lần thử lúc 04:15 thấy Plex rảnh và sao lưu nó, và việc chờ kết thúc với một thông báo. Nếu bộ phim vẫn đang chạy lúc 06:00, việc chờ sẽ đến hạn và Plex vẫn được sao lưu, nên các bản sao lưu của nó không bao giờ chậm quá ba giờ.

Với kiểm tra của riêng bạn, hãy cho **Lệnh trước sao lưu** thoát với mã 75 khi ứng dụng đang bận, ví dụ `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Thêm các triệu chứng và cách sửa nằm trong [Xử lý sự cố](troubleshooting.md#deferred-backups).

## Bản xuất thuần {#exports}

restic vẫn là bộ máy, nhưng một bản xuất cho bạn một bản sao không cần BombVault hay restic.

- **Xuất container** và **Xuất máy ảo** dưới **Thêm** trên trang của một mục ghi một bản sao đúng như lúc này, bên cạnh kho: các thư mục sao lưu của container cùng mẫu Unraid của nó, hoặc các ảnh đĩa của VM cùng định nghĩa của nó, khôi phục được bằng `virsh define`.
- **Xuất** trên một snapshot ghi một bản sao lưu đã lưu vào thư mục xuất trên máy chủ, dưới dạng tar, zip hoặc một kho restic riêng, và có thể lấy một snapshot hoặc tất cả.

Bản xuất mặc định là tệp thuần. **Mã hóa bản xuất bằng age** trong **Cài đặt, Hệ thống** niêm phong chúng, kể cả ZIP của flash, cho những người nhận bạn liệt kê, mỗi người là một khóa công khai age hoặc khóa công khai SSH. Sau này bạn giải mã chúng ở ngoài máy chủ bằng khóa riêng tương ứng. Khi bật mã hóa mà không có người nhận hợp lệ, một bản xuất sẽ lỗi với thông báo rõ ràng thay vì ghi ra tệp thuần.

## Một lần chạy cho bạn biết gì {#run-notes}

- **Thời gian.** Mỗi lần chạy ghi lúc bắt đầu, kết thúc và thời lượng, và mỗi mục có **Lần chạy gần đây** riêng.
- **Vì sao một bản sao lưu chậm.** Khi một bản sao lưu chạy, BombVault theo dõi CPU, đĩa và mạng bận đến mức nào. Khi một bản sao lưu mất lâu hơn bình thường nhiều và một thứ rõ ràng đã chạm giới hạn, lần chạy nói ra, ví dụ "Đĩa đích disk1 bận 98%". Nếu không thì nó không nói gì.
- **Tiến trình của các lần kiểm tra và prune.** Nhật ký hoạt động và thẻ **Kiểm tra bản sao lưu** cho biết restic đã đếm đến đâu, chẳng hạn *12 trên 47 pack*, kèm thời gian còn lại của bước đó khi đã đủ dữ liệu để ước tính.
- **Các tệp không đọc được.** Một bản sao lưu vẫn xong nhưng bỏ sót thứ gì đó sẽ làm dòng của nó chuyển cam và nêu rõ những gì bị bỏ sót.
