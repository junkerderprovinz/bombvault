# Off-site & khôi phục

!!! note "Các bản sao off-site chờ sau một lần xây dựng lại"
    Khi Trình hướng dẫn khẩn cấp hoặc **Tìm bản sao lưu** dựng lại các mục mà không có cài đặt cũ, bản sao off-site của các phần đó tạm dừng cho đến khi mặc định của chúng được xác nhận. Xem [Sau khi dựng lại](#after-a-rebuild).

Bản sao lưu cục bộ bảo vệ bạn khỏi một container bị mất hay một bản cập nhật hỏng. Bản sao off-site và một bộ khôi phục đã được thử nghiệm bảo vệ bạn khỏi mất cả máy, ransomware hay hỏa hoạn. Trang này nói về đích sao lưu và bản sao off-site, cách làm một bản sao chống can thiệp, chứng minh rằng bạn khôi phục được, ghép nối nhiều máy chủ và lấy lại mọi thứ khi chính BombVault đã mất.

## Bản sao off-site {#off-site-copies}

Giữ bản sao lưu cục bộ nhanh và thêm một hoặc nhiều đích sao lưu ngoài cơ sở. BombVault sao chép các bản sao lưu mới tới đó bằng `restic copy` theo kiểu hết sức có thể, nên trục trặc off-site không bao giờ làm hỏng bản sao lưu cục bộ. Theo cách này, mỗi phần ghi vào thư mục riêng của nó trước và kho off-site là một bản sao. Thư mục riêng không nhất thiết phải nằm trên máy chủ này: xem [Thư mục riêng của một phần](#own-folder) để giữ nó trong một đích sao lưu như S3 hoặc một rest-server.

- **Nhiều đích sao lưu cho mỗi phần.** Mỗi phần (container, VM, flash, tự sao lưu, thư mục và tập dữ liệu ZFS) có thể sao chép tới nhiều đích off-site cùng lúc, nên bạn có thể giữ, chẳng hạn, một rest-server trên máy của bạn bè và một bucket S3 song song. Thẻ **Đích theo phần** trên trang Đích sao lưu bật hoặc tắt từng dòng từ một phần tới một đích.
- **Khi nào các bản sao chạy.** Thẻ **Sao chép** của một đích đặt **Khi nào sao chép**: **Sau mỗi bản sao lưu**, hoặc theo kế hoạch riêng thu thập các bản sao lưu và sao chép vào những giờ cố định. Dưới **Cài đặt, Lịch trình, Bản sao ngoài site theo phần**, **Ngoại lệ** cho một phần kế hoạch sao chép riêng.
- **Thẻ Bản sao ngoài site.** Dưới **Cài đặt, Đích sao lưu**, mỗi phần có bản sao off-site đều có một dòng trong **Bản sao ngoài site**, với **Kiểm tra kết nối**, **Sao chép ngay** để bắt đầu một lần sao chép theo yêu cầu, và **Thiết lập** hoặc **Chỉnh sửa**. Phần còn lại của các cài đặt sao chép của phần đó mở trong cửa sổ của nó từ đây.
- **Lưu giữ** được đặt cho từng đích sao lưu dưới **Giữ trong bao lâu**, nên bạn có thể giữ bản sao off-site lâu hơn làm kho lưu trữ. Bản sao trong trường off-site của một phần theo nó như mọi bản sao khác, và một phần có thể giữ giá trị riêng dưới **Quy tắc theo phần**. Để toàn bộ bằng 0 thì không bao giờ cắt tỉa snapshot off-site. Xem [Giữ trong bao lâu](#keep).
- **Giới hạn băng thông** (**Băng thông off-site** dưới **Cài đặt, Đích sao lưu**, hoặc thẻ **Sao chép** của một đích) giới hạn tốc độ tải lên/tải xuống của restic để các bản sao không làm nghẽn WAN của bạn. Chúng được nhập bằng KiB/s, đơn vị mà restic nhận, và mỗi ô hiện cùng tốc độ đó theo MB/s và Mbit/s khi bạn gõ: 2048 đọc là khoảng 2,0 MB/s (16,8 Mbit/s).
- **Nén** là **Tắt**, **Tự động** (mặc định của restic) hoặc **Tối đa**, đặt trên từng đích sao lưu và, cho một phần, trong quy tắc của nó ở đó. Sao lưu, sao chép và dọn dẹp ghi theo cách đó, và bộ khôi phục ghi tên nó, để restic thuần có thể tiếp tục ghi theo cùng cách.
- **Ưu tiên phát trực tuyến.** Trong lúc một máy chủ media như Plex, Jellyfin hoặc Emby đang phát, các bản sao tải lên ở mức giới hạn thấp hơn và trở lại mức bình thường vài phút sau khi luồng kết thúc. Thẻ **Phát trực tuyến** trên trang Đích sao lưu có **Làm chậm bản sao off-site khi phát trực tuyến**, **Máy chủ media** (các container có Plex, Jellyfin hoặc Emby trong tên image cho đến khi bạn chọn), **Phát trực tuyến khi trên (Mbit/s)**, **Giới hạn tải lên khi phát trực tuyến (KiB/s)** và **Trở lại bình thường sau (phút)**. BombVault đọc lưu lượng từ Docker, nên một máy chủ media trên mạng của máy chủ không đo được. REST, S3, B2, Azure, Google Cloud, Swift và rclone qua HTTP chậm lại ngay giữa lúc sao chép. SFTP và các thư mục cục bộ hoặc đã gắn dùng mức giới hạn thấp hơn ở bước sao chép kế tiếp.
- **Lưu trữ lạnh (S3).** Một đích S3 có thể dùng **Lớp lưu trữ** rẻ hơn mà restic vẫn đọc được ngay, như Standard-IA, One Zone-IA, Intelligent-Tiering hoặc Glacier Instant Retrieval. Glacier Flexible Retrieval và Deep Archive bị loại ra, vì restic không khôi phục được từ chúng. Một rclone remote đặt lớp của nó trong cấu hình rclone.
- **Hạn mức tăng trưởng.** **Hạn mức tăng trưởng (GB, 0 = tắt)** trong cửa sổ sao chép của một phần phát cảnh báo khi kho lớn hơn mức đó. Nó không bao giờ xóa gì, nên phù hợp với một bản sao append-only mà không có gì dọn từ đây.
- Khi một bản sao đang chạy, đầu trạng thái của Tổng quan ghi **Đang sao chép ra ngoài site**. Đó là một chỉ báo hoạt động, không phải thanh phần trăm, vì `restic copy` không cung cấp tiến độ đọc được bằng máy.

!!! note "Khôi phục từ bất kỳ nơi nào"
    Mỗi mục liệt kê các bản sao lưu của nó thành một dòng thời gian qua mọi nơi mà một bản sao lưu nằm, trên tab **Bản sao lưu** của nó. Một bản sao lưu được sao chép lên B2 hiện một lần, đánh dấu từng nơi giữ nó. Một lần khôi phục lấy nơi đầu tiên nó tới được, bắt đầu với kho mà mục được ghi vào, và bạn có thể chọn nơi khác cho từng bản sao lưu. Đích off-site chỉ được đọc khi bạn nhấp **Kiểm tra**, vì liệt kê lưu trữ đám mây mất thời gian. Xóa ở một nơi trước tiên kiểm tra các nơi khác và cho biết đó có phải bản sao cuối cùng không. Xem [Khôi phục](restore.md).

## Đích sao lưu {#destinations}

Đích sao lưu là nơi các bản sao lưu đi tới: một thư mục cục bộ, một share, máy chủ của riêng bạn hoặc một dịch vụ đám mây. Nó được thiết lập một lần rồi phục vụ mọi phần. Mở **Đích sao lưu** từ ô cài đặt của nó, hoặc bật **Hiện cả trong thanh bên** ở đó để ghim.

![Trang Đích sao lưu: danh sách các đích và luồng cho biết phần nào đi đâu.](assets/screenshots/storage/locations-dark.png#only-dark)
![Trang Đích sao lưu: danh sách các đích và luồng cho biết phần nào đi đâu.](assets/screenshots/storage/locations-light.png#only-light)

**Thêm đích** mở một cửa sổ gồm bốn bước:

1. **Nhà cung cấp.** Mọi dịch vụ được liệt kê cùng logo, với một ô tìm kiếm ở góc trên bên phải của cửa sổ, theo nhóm: **Cục bộ** cho một đĩa hoặc share gắn vào máy chủ này, **NAS hoặc máy chủ** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV và các loại khác), **Dịch vụ S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 và các dịch vụ khác), máy chủ S3 của riêng bạn (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Lưu trữ đám mây** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud và phần còn lại mà rclone hỗ trợ) và **Từ nhóm của bạn**, các máy chủ nhận của nhóm bạn. Mỗi dịch vụ cho biết nó hợp với việc sao lưu đến đâu: ổ đám mây chậm đi khi có nhiều yêu cầu, nên bản sao lưu đầu tiên và việc dọn dẹp ở đó lâu hơn. Với một dịch vụ mà không ô nào bao phủ, ô **Rclone** dưới **Cấu hình rclone riêng** nhận các remote dán từ một rclone.conf.
2. **Đăng nhập.** Các trường tùy theo dịch vụ: khóa truy cập cho S3, người dùng và mật khẩu cho WebDAV và SMB, mật khẩu ứng dụng nơi đăng nhập hai yếu tố chặn mật khẩu thường, khóa SSH công khai của BombVault cho SFTP và Storage Box, hoặc mã thông báo cho các dịch vụ đăng nhập qua trình duyệt. Với các dịch vụ đó, cửa sổ hiển thị một lệnh `rclone authorize` để chạy trên máy tính có trình duyệt, và mã thông báo nó in ra được dán vào trường. Nếu bạn đã có một [thông tin đăng nhập đã lưu](#sign-ins) cho dịch vụ, **Thông tin đăng nhập đã lưu** chọn nó thay cho **Khóa mới**. **Kiểm tra kết nối** kiểm tra đăng nhập trước khi lưu bất cứ gì. Sau đó cửa sổ liệt kê các thư mục trên đích sao lưu, với **Thư mục mới** để tạo một thư mục và dung lượng trống nơi dịch vụ báo cáo. Một thư mục trống là an toàn nhất. BombVault đặt mỗi phần một thư mục trong đó.
3. **Bảo vệ.** Cửa sổ nói thẳng dịch vụ làm được gì. Một rest-server ở chế độ append-only từ chối xóa, và kiểm tra can thiệp xác nhận điều đó. Một bucket S3 có thể giữ phiên bản cũ nhờ versioning và object lock, điều BombVault chưa kiểm tra được. Một ổ đám mây hoàn toàn không từ chối xóa được: ai vào được máy chủ thì cũng vào được bản sao đó. Chỉ bật **Bảo vệ chống xóa (append-only)** khi phía xa thật sự từ chối xóa. Khi đó BombVault không bao giờ dọn ở đó. **Phòng khi khẩn cấp** cho biết bộ khôi phục chứa gì cho đích này.
4. **Cho phần nào.** Chọn các phần sao lưu tới đây. Bản sao đầu tiên mang theo cả lịch sử. Các mục đã chọn đích sao lưu riêng không tự nhận đích mới, và cửa sổ nêu tên chúng.

![Thêm đích: bước đầu tiên, với mỗi nhà cung cấp là một ô.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Thêm đích: bước đầu tiên, với mỗi nhà cung cấp là một ô.](assets/screenshots/storage/add-providers-light.png#only-light)

![Thêm đích: bước đăng nhập sau khi kiểm tra thành công.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Thêm đích: bước đăng nhập sau khi kiểm tra thành công.](assets/screenshots/storage/add-signin-light.png#only-light)

Một số dịch vụ cần thêm một chút khi đăng nhập:

- **SFTP và Storage Box.** **Thêm khóa** đặt khóa công khai của BombVault lên máy chủ bằng **Mật khẩu tài khoản**, mật khẩu này chỉ dùng cho lần đăng nhập đó và không được lưu ở đâu cả. Bên dưới có lệnh để làm việc này bằng tay. Bài kiểm tra hiển thị **Khóa máy chủ** cùng dấu vân tay của nó. BombVault ghi nhớ khóa đó và từ chối máy chủ nếu sau này nó đưa ra một khóa khác.
- **Khóa S3 chỉ dành cho một bucket**, chẳng hạn token đối tượng của Cloudflare R2 hoặc khóa AWS giới hạn quyền, có thể không được phép liệt kê các bucket khác. Gõ bucket vào **Bucket**, và bài kiểm tra cùng danh sách thư mục sẽ mở đúng bucket đó.
- **Vùng và địa chỉ S3.** Vùng được đọc từ một endpoint có nêu tên nó (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage bắt đầu với vùng mặc định riêng của nó, và IDrive e2 hỏi endpoint mà bảng điều khiển của nó hiển thị. Một địa chỉ dán vào kèm bucket, như bảng điều khiển R2 hiển thị, sẽ điền máy chủ và bucket. Một bucket tạo từ trình duyệt thư mục nằm trong vùng mà biểu mẫu nêu.
- **Lớp lưu trữ.** Bộ chọn chỉ đưa ra những lớp mà dịch vụ nhận, và **Cài đặt khác** chỉ hiện những tùy chọn định hình một bucket mới.
- **Google Drive và OneDrive.** Dữ liệu đã dọn bị xóa hẳn thay vì vào thùng rác, nơi nó vẫn tính vào hạn mức. Một ổ chia sẻ của Google nằm dưới **Cài đặt khác**.
- **Máy chủ SMB không liệt kê các share của mình** nhận tên một share trong **Thư mục chia sẻ**, và bài kiểm tra mở thẳng share đó.
- **FTP** hỏi **Mã hóa**: **Không có**, **TLS tường minh (FTPES)** hoặc **TLS ngầm định (FTPS, cổng 990)**.
- **OneDrive và Zoho WorkDrive.** Khi một lần đăng nhập mở ra nhiều ổ hoặc không gian làm việc, cửa sổ hỏi các bản sao lưu sẽ đi vào cái nào.
- **rest-server dùng chứng chỉ tự ký.** Đặt chứng chỉ dưới dạng tệp PEM vào thư mục `certs` trong thư mục dữ liệu của BombVault (`/config/certs` trong container, `appdata/bombvault/certs` trên Unraid). Sao lưu, sao chép off-site và bài kiểm tra bảo vệ chống xóa đều tin cậy nó mà không cần khởi động lại.

Dịch vụ S3 chạy qua backend S3 riêng của restic, điều cho phép áp dụng lớp lưu trữ và object lock. Một rest-server được restic tới trực tiếp. Mọi dịch vụ khác chạy qua rclone mà BombVault đi kèm, và phần của nó trong cấu hình rclone hiện trong thẻ **rclone.conf** trên trang của đích, với các bí mật được ẩn. Một bản xuất cài đặt mang theo các đích sao lưu, và khi gồm cả thông tin xác thực, nó mang theo cả thông tin đăng nhập của chúng.

Lưu một đích sao lưu đã chứa bản sao lưu của container, VM hoặc thư mục, ví dụ một đích bạn đã xóa rồi thêm lại, sẽ thêm nó vào các phần đó như một bản sao chưa được đánh dấu. Không có gì được sao chép tới đó cho đến khi bạn đánh dấu, nhưng **Khôi phục** đưa ra ngay các bản sao lưu cũ.

Thư mục trên máy chủ này được gọi là **Cục bộ** ở mọi nơi: trên các trang đích sao lưu, trong luồng, trên một mục và trong Khôi phục. Khi máy chủ này có nhiều thư mục, mỗi thư mục mang tên riêng của nó. Một đích sao lưu do trường off-site của một phần tạo ra được đặt tên theo nơi của nó, tức thư mục cuối cùng trong địa chỉ hoặc, nếu không có, máy chủ của nó, và nhận thêm một số khi đã có đích khác mang tên đó. Mọi đích đều có thể đổi tên trong ô **Tên** của thẻ **Kết nối**, kể cả một thư mục bucket mà nhiều phần dùng chung và bản sao mà trường off-site của một phần mô tả, và tên bạn đặt được giữ nguyên. Khi hai đích cùng tên đứng cạnh nhau, mỗi đích kèm theo máy chủ hoặc thư mục của nó.

### Trang của một đích sao lưu {#location-page}

Mỗi đích sao lưu có một trang riêng. Nó chỉ hiện các thẻ áp dụng cho đích đó, theo thứ tự này:

![Trang của một đích sao lưu: Thư mục riêng, Kết nối và Thông tin đăng nhập.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Trang của một đích sao lưu: Thư mục riêng, Kết nối và Thông tin đăng nhập.](assets/screenshots/storage/location-page-light.png#only-light)

- **Thư mục riêng**: các phần ghi bản sao lưu của chúng vào đây trước tiên. Thông tin đăng nhập, mức bảo vệ và giới hạn của đích này áp dụng cho chúng. **Dùng làm thư mục riêng** chuyển thư mục riêng của thêm nhiều phần về đây, và **Đổi** chuyển một phần đi nơi khác. Xem [Thư mục riêng của một phần](#own-folder).
- **Sao lưu thẳng vào đây**: mỗi phần có các mục sao lưu thẳng tới nơi này thay vì thư mục riêng của chúng, và có bao nhiêu mục. Một kho trực tiếp như vậy lấy mọi cài đặt từ đích này. Cái nào không còn gì sao lưu tới thì cho biết như vậy, và **Xóa** gỡ nó khỏi danh sách. Các bản sao lưu ở nơi của nó vẫn còn. Xem [Kho trực tiếp](#direct-repositories).
- **Kết nối**: **Tên**, công tắc **Sẵn dùng** (tắt nghĩa là BombVault không còn ghi vào đây, và các mục sao lưu tới đây thất bại với thông báo rõ ràng thay vì rơi vào nơi khác), thư mục, **Nén**, **Lớp lưu trữ** và **Ngoài cơ sở**. Trên đích **Cục bộ**, nó liệt kê thư mục của từng phần dưới **Thư mục riêng ở đây**. **Đổi kết nối** kiểm tra và lưu một địa chỉ hoặc đăng nhập mới. Khi các nơi khác đăng nhập bằng cùng thông tin đăng nhập đã lưu, cửa sổ cho biết, và một thay đổi ở đây sẽ cho đích này thông tin đăng nhập riêng thay vì ghi đè cái dùng chung.
- **Thông tin đăng nhập**: nó dùng thông tin đăng nhập đã lưu nào (**Đăng nhập bằng**), những đích khác nào cũng dùng nó (**Cũng được dùng bởi**), và **Chỉnh sửa**, mở cửa sổ thông tin đăng nhập. Xem [Thông tin đăng nhập](#sign-ins).
- **rclone.conf**: phần của đích này trong cấu hình rclone, dành cho một đích chạy qua rclone.
- **Bảo vệ chống xóa**, với **Bảo vệ chống xóa (append-only)**, **Chấp nhận khi không có bảo vệ chống xóa**, **Kiểm tra bảo vệ chống xóa ngay** và **Ai dọn bớt**, cho biết BombVault dọn bản sao và các thư mục riêng ở đây hay để phía bên kia làm. Một thư mục cục bộ chỉ hiện thẻ này khi nó có mức bảo vệ mà bạn tắt được. Xem [Bảo vệ chống xóa](#delete-protection-append-only).
- **Sao chép**: **Khi nào sao chép**, cùng **Giới hạn tải lên (KiB/s)** và **Giới hạn tải xuống (KiB/s)**. Một đích chỉ chứa thư mục riêng mà không có bản sao hiển thị các giới hạn trong một thẻ tên **Băng thông**.
- **Giữ trong bao lâu**: **Ngắn**, **Cân bằng**, **Dài** hoặc **Giá trị riêng**, với dòng thời gian cho thấy quy tắc giữ những bản sao lưu nào. Xem [Giữ trong bao lâu](#keep).
- **Đã nằm dưới đích này**: các phần có địa chỉ gõ tay nằm trong một thư mục của đích này. Tiếp nhận một phần để kho ở nguyên chỗ. Thông tin đăng nhập, lớp lưu trữ và bảo vệ chống xóa khi đó đến từ đích. BombVault trước tiên kiểm tra thông tin đăng nhập của đích mở được kho, và từ chối đặt một kho append-only dưới một đích không phải append-only.
- **Quy tắc theo phần**: kho, lưu giữ hoặc nén riêng của một phần tại đích này. **Đổi quy tắc** mở chúng, và **Đặt lại theo đích** đưa một phần trở lại. Thông tin đăng nhập, băng thông và thời điểm sao chép luôn đến từ đích.
- Một bình chứa cho thấy đích đầy đến đâu, với vạch nơi nó đầy và còn bao nhiêu tuần theo mức tăng của những tuần gần đây. Một đích không báo cáo dung lượng ghi **Chưa rõ dung lượng**.

**Di chuyển** và **Xóa đích sao lưu** nằm trong các thao tác của trang, được mô tả dưới [Di chuyển và gỡ bỏ](#move-remove).

Một đích sao lưu từ xa từng chứa bản sao lưu mà bỗng không còn kho nào, chẳng hạn một rest-server có đĩa chưa được gắn, sẽ không tự nhận một kho mới, trống. Trang của nó báo điều đó ngay khi một lần sao lưu hoặc sao chép phát hiện kho đã biến mất. Hãy gắn lại đĩa. Nếu bạn cố ý làm trống kho cũ, **Tạo kho mới ở đây** bắt đầu lại từ đầu, cho mọi phần của đích và các kho của nó cùng một lúc.

Một máy chủ nhận mà phiên bản khác trong nhóm của bạn chạy hiện trong số các nhà cung cấp với tên **Từ nhóm của bạn**. Xem [Máy chủ nhận](#receiving-server).

BombVault kiểm tra các đích sao lưu của nó ở mỗi lần khởi động. Nó tự sửa những gì không cần bạn quyết định, chẳng hạn một liên kết tới bản sao hoặc kho đã mất, một bản sao trên thư mục riêng của một phần, hoặc một kho trực tiếp mà bản sao của nó đã mất, và ghi một dòng vào nhật ký cho mỗi lần sửa. Những gì không có cách sửa an toàn thì chỉ được ghi nhật ký.

![Cùng trang đó trên điện thoại.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Cùng trang đó trên điện thoại.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Giữ trong bao lâu {#keep}

Thời gian giữ bản sao lưu được đặt cho từng đích sao lưu, trong thẻ **Giữ trong bao lâu** của nó. Quy tắc bao phủ các bản sao ở đích đó và các thư mục riêng nằm ở đó, kể cả đích **Cục bộ**. Một phần có thể giữ giá trị riêng tại một đích dưới **Quy tắc theo phần**, và những giá trị đó thắng thẻ của đích cho phần ấy.

Một quy tắc giữ một bản sao lưu cho mỗi ngày, tuần, tháng và năm, lùi xa đến mức bạn đặt, và dọn phần còn lại sau mỗi bản sao lưu hoặc bản sao:

| Mẫu | Bản sao lưu mới nhất | Ngày | Tuần | Tháng | Năm |
|---|---|---|---|---|---|
| **Ngắn** | 0 | 7 | 4 | 3 | 0 |
| **Cân bằng** | 0 | 7 | 4 | 6 | 1 |
| **Dài** | 0 | 14 | 8 | 12 | 3 |

**Giá trị riêng** cho bạn tự đặt năm số đếm. Số 0 tắt số đếm đó, và một quy tắc có mọi số đếm bằng 0 giữ tất cả. Các chấm trong thẻ cho thấy quy tắc giữ những bản nào trong 40 bản sao lưu mẫu, bản mới nhất ở ngoài cùng bên phải. Mẫu này giống nhau cho mọi đích, vì hình được vẽ trước khi BombVault đọc các bản sao lưu ở đó.

**Hỏi trước khi giữ ít hơn**, bật theo mặc định, hỏi trước khi lưu một thay đổi giữ ít bản sao lưu hơn, và cho biết quy tắc mới loại bỏ bao nhiêu trong 40 bản sao lưu mẫu. Nó áp dụng cho mọi đích sao lưu. **Đặt lại theo đích** dưới **Quy tắc theo phần** cũng hỏi cùng câu đó.

Một đích có bảo vệ chống xóa không bao giờ bị dọn từ máy này. Phía bên kia của nó giữ hoặc bỏ các bản sao lưu cũ theo quy tắc riêng. Khi một mục có bất thường nghiêm trọng đang mở về dữ liệu bị mất, chính sách lưu giữ giữ các bản sao lưu cũ của mục đó cho đến khi bạn xác nhận. Xem [Bất thường](anomalies.md).

Để xem lần chạy tới thật sự sẽ xóa gì, hãy mở **Xem trước** dưới **Cài đặt, Đích sao lưu**. Nó liệt kê **Những gì lần chạy tới sẽ xóa**, theo từng kho và từng mục, kèm tên các điểm khôi phục. Nó không khóa gì và không thay đổi gì.

![Giữ trong bao lâu với Cân bằng được chọn: các chấm cho thấy bản nào trong 40 bản sao lưu mẫu được giữ.](assets/screenshots/storage/keep-dark.png#only-dark)
![Giữ trong bao lâu với Cân bằng được chọn: các chấm cho thấy bản nào trong 40 bản sao lưu mẫu được giữ.](assets/screenshots/storage/keep-light.png#only-light)

#### Ví dụ: Cân bằng giữ những gì {#example-retention}

Một container được sao lưu mỗi đêm lúc 03:00 từ ngày 1 tháng 10 năm 2025, nên vào thứ Tư 4 tháng 11 năm 2026 nó có 400 bản sao lưu. Đích sao lưu của nó đặt là **Cân bằng**: 7 ngày, 4 tuần, 6 tháng và 1 năm.

Mỗi số đếm giữ bản sao lưu mới nhất của ngày, tuần, tháng hoặc năm của nó, và một bản mà nhiều số đếm cùng chọn chỉ được giữ một lần. Tuần chạy từ thứ Hai đến Chủ nhật:

| Quy tắc | Các bản sao lưu nó chọn | Bản mới nó thêm |
|---|---|---|
| 7 ngày | 4/11, 3/11, 2/11, 1/11, 31/10, 30/10, 29/10 | cả 7 |
| 4 tuần | 4/11, 1/11, 25/10, 18/10 | 25/10, 18/10 |
| 6 tháng | 4/11, 31/10, 30/9, 31/8, 31/7, 30/6 | 30/9, 31/8, 31/7, 30/6 |
| 1 năm | 4/11 | không có |

Như vậy còn 13 bản trong 400. Bản cũ nhất là từ 30/6, nên bạn lùi lại được hơn bốn tháng một chút. Nhờ khử trùng lặp, chúng chiếm ít chỗ hơn không đáng kể so với một bản sao lưu đầy đủ cộng với những gì thay đổi ở giữa.

**Năm** đáng để xem kỹ hơn. Nó đếm các năm dương lịch có bản sao lưu, bắt đầu từ năm hiện tại, và bản sao lưu mới nhất của năm nay là của hôm nay. Vì vậy số 1 trong **Năm** không thêm gì vào các số đếm khác, và bạn cần số 2 để lùi vào năm ngoái. Cùng 400 bản sao lưu đó với các mẫu khác:

| Mẫu | Số bản được giữ | Bản cũ nhất |
|---|---|---|
| **Ngắn** | 10 | 30 tháng 9 năm 2026 |
| **Cân bằng** | 13 | 30 tháng 6 năm 2026 |
| **Dài** | 29 | 31 tháng 12 năm 2025 |

Chuyển từ **Cân bằng** sang **Ngắn** sẽ hỏi trước, nhờ **Hỏi trước khi giữ ít hơn**: "Trong 40 bản sao lưu ở hình, quy tắc mới loại bỏ 3 bản mà quy tắc hiện tại giữ." Con số này đến từ mẫu trong hình, không phải từ các bản sao lưu của bạn. Ở đây nó tình cờ khớp: 31/8, 31/7 và 30/6 sẽ bị bỏ. **Vẫn lưu** áp dụng quy tắc, và lần dọn kế tiếp xóa chúng khỏi đích đó vĩnh viễn.

Để giữ các container lâu hơn phần còn lại tại đích này, hãy bấm **Đổi quy tắc** dưới **Quy tắc theo phần**, chọn **Containers**, và đặt **Giá trị riêng** trong **Quy tắc cho Containers**, ví dụ 14 ngày, 8 tuần, 12 tháng và 2 năm. Các phần khác tiếp tục theo thẻ của đích, và **Đặt lại theo đích** đưa Containers trở lại.

### Thông tin đăng nhập {#sign-ins}

Một thông tin đăng nhập là khóa hoặc mật khẩu mà BombVault dùng cho một dịch vụ S3 hoặc một rest-server. Các thông tin đăng nhập là một danh sách, thẻ **Thông tin đăng nhập** dưới **Cài đặt, Đích sao lưu**, và mỗi dòng cho biết những đích sao lưu nào dùng nó (**Được dùng bởi**).

- **Được lưu khi bạn thêm một đích.** Thêm đích lưu các khóa bạn gõ thành một thông tin đăng nhập. Đích kế tiếp ở cùng dịch vụ có thể chọn nó dưới **Thông tin đăng nhập đã lưu** thay cho **Khóa mới**, và một thay đổi sau này áp dụng cho mọi đích dùng nó.
- **Mặc định.** Một ngôi sao đánh dấu thông tin đăng nhập mặc định. Nó được dùng ở bất cứ nơi nào không chọn thông tin đăng nhập, như một địa chỉ gõ tay, một nguồn kéo, hoặc một lần khôi phục trên bản cài mới. **Đặt làm mặc định** trao ngôi sao cho cái khác.
- **Chỉnh sửa và Kiểm tra.** **Chỉnh sửa**, ở đây hoặc trong thẻ **Thông tin đăng nhập** của một đích, mở một cửa sổ cho thông tin đăng nhập. **Kiểm tra** đăng nhập vào mọi đích dùng nó bằng những gì bạn đã gõ, trước khi lưu bất cứ gì. **Lưu** lưu nó.
- **Xóa.** Một thông tin đăng nhập chỉ xóa được khi không có đích sao lưu hay nguồn kéo nào dùng nó, và cái mặc định chỉ đi khi là cái cuối cùng. Khi đó dòng cho biết còn gì đang dùng nó.
- **Đổi kết nối của một đích** không bao giờ ghi đè một thông tin đăng nhập mà các nơi khác dùng chung. Đích đó nhận một thông tin đăng nhập riêng, và cửa sổ nói trước rằng thông tin đăng nhập của nó đang được dùng chung. Để đổi cái dùng chung ở mọi nơi, hãy sửa nó trong thẻ **Thông tin đăng nhập**.

![Thẻ Thông tin đăng nhập: mỗi thông tin đăng nhập đã lưu cùng thứ đang dùng nó, và ngôi sao ở cái mặc định.](assets/screenshots/storage/signins-dark.png#only-dark)
![Thẻ Thông tin đăng nhập: mỗi thông tin đăng nhập đã lưu cùng thứ đang dùng nó, và ngôi sao ở cái mặc định.](assets/screenshots/storage/signins-light.png#only-light)

Một thư mục trên máy chủ này không cần thông tin đăng nhập. SMB, WebDAV, SFTP, Storage Box và các ổ đám mây giữ thông tin của chúng trong cấu hình rclone, mà thẻ **rclone.conf** của đích hiển thị, nên chúng không xuất hiện trong danh sách này.

### SMB, WebDAV và các rclone remote khác {#smb-webdav}

Một share Windows hoặc Samba và một máy chủ WebDAV (Nextcloud, ownCloud, SharePoint hay bất kỳ loại nào khác) là các ô nhà cung cấp trong **Thêm đích**, dưới **NAS hoặc máy chủ**. Điền máy chủ và share (SMB) hoặc địa chỉ và loại máy chủ (WebDAV), người dùng và mật khẩu, và BombVault viết phần rclone cho bạn. rclone làm mờ mật khẩu trước khi lưu.

Với một dịch vụ mà không ô nào bao phủ, ô **Rclone** dưới **Cấu hình rclone riêng** nhận các remote dán từ một rclone.conf. Chúng được thêm vào các remote mà BombVault đã có và không bao giờ thay thế chúng, và được lưu đã mã hóa. Để sao lưu một phần tới một trong số đó, hãy nhập `rclone:<remote>:<folder>` làm kho off-site của phần ấy, ví dụ `rclone:nas:backups/bombvault`. Share hoặc bucket là đoạn đường dẫn đầu tiên, không thuộc tên.

Đây là cách tốt hơn việc gắn share trên Unraid: restic khuyên không đặt một kho trên một share CIFS đã gắn, và ở đây không có gì được gắn. NFS không có ô vì cả restic lẫn rclone đều không có backend NFS. Với NFS, hãy gắn export trên máy chủ và dùng như một thư mục cục bộ.

### Di chuyển và gỡ bỏ {#move-remove}

- **Di chuyển** đưa bản sao của một phần sang đích sao lưu khác, cùng toàn bộ lịch sử. Mọi thứ ở lại nơi cũ. BombVault không xóa gì ở đó. Việc di chuyển chạy nền và có thể lâu: khoảng 9 phút cho mỗi 100 GB giữa các đĩa cục bộ, hàng giờ qua đường truyền chậm. Tổng quan hiển thị nó khi đang chạy.
- **Xóa đích sao lưu** có hai cách. **Xóa đích sao lưu, giữ các bản sao lưu** ngừng ghi vào đó và để nguyên các bản sao lưu. Sau này bạn đọc được chúng dưới **Khôi phục, Từ thư mục sao lưu cũ**. **Xóa đích sao lưu và xóa các bản sao lưu ở đó** trước tiên đọc những gì được lưu ở đó và liệt kê: bản sao lưu nào sẽ mất, bản nào không có ở nơi nào khác, và kho nào ở lại vì bảo vệ chống xóa từ chối, các mục vẫn sao lưu thẳng vào đó, một đích hay phần khác giữ bản sao ở đó, hoặc không đọc được. Các mục sẽ mất bản sao lưu duy nhất được liệt kê riêng và được giữ lại trừ khi bạn bật **Xóa cả những mục này**. Bạn xác nhận bằng cách gõ tên của đích, và việc xóa chạy nền.
- Cách này dùng được cho mọi loại đích sao lưu: đích mà nhiều phần dùng chung, đích chỉ lập cho một phần, kho do bạn tự tạo và thư mục mà một phần ghi các bản sao lưu riêng của nó vào. Các mục từng sao lưu vào một kho đã gỡ sẽ lại theo phần của chúng, và mặc định của phần từng trỏ đến đó cũng vậy. Thư mục của một phần không thể cứ thế mà bỏ đi, vì phần nào cũng cần một thư mục. Vì vậy cửa sổ hỏi thư mục mà các phần sẽ chuyển tới, và mỗi phần giữ tên thư mục của mình bên dưới đó.
- Xóa các bản sao lưu ở một đích sao lưu cần mật khẩu đăng nhập và đăng nhập ngay tại phiên bản đó. Không có mật khẩu thì đích vẫn gỡ được, chỉ là không xóa được các bản sao lưu của nó từ đây.

### Ví dụ: một bản sao off-site trên Backblaze B2 {#example-s3}

Các container và tự sao lưu cũng nên nằm ở Backblaze B2, để một máy chủ hỏng không kéo chúng theo. Các bước tương tự dùng được cho Wasabi, Hetzner Object Storage và hầu hết dịch vụ S3 khác.

1. **Trong B2,** tạo một bucket riêng tư, chẳng hạn `tower-backups`, ở một vùng gần bạn. Dưới **Application Keys**, tạo một khóa có quyền đọc và ghi, giới hạn trong bucket đó, và ghi lại key ID cùng application key của nó. B2 chỉ hiện application key một lần. Đặt lifecycle của bucket để chỉ giữ phiên bản cuối, nếu không B2 giữ dữ liệu đã dọn dưới dạng các phiên bản ẩn và tính phí bạn.
2. **Nhà cung cấp.** Trong BombVault, mở **Đích sao lưu**, bấm **Thêm đích** và chọn **Backblaze B2** dưới **Dịch vụ S3**.
3. **Đăng nhập.** Nhập key ID làm **Access key ID** và application key làm **Secret access key**. Nhập endpoint của bucket vào **Endpoint**, chẳng hạn `s3.eu-central-003.backblazeb2.com`, và BombVault đọc vùng từ đó. Gõ `tower-backups` vào **Bucket**, vì khóa này chỉ mở một bucket.
4. **Kiểm tra kết nối.** Nó phải báo **Đã kết nối** và liệt kê các thư mục của bucket. Bấm **Thư mục mới** và đặt tên `bombvault`. BombVault đặt mỗi phần một thư mục trong đó, chẳng hạn `bombvault/containers`.
5. **Bảo vệ.** Cửa sổ giải thích rằng B2 có thể giữ phiên bản cũ nhờ object lock và BombVault không kiểm tra được điều này. Để **Bảo vệ chống xóa (append-only)** tắt trừ khi bucket thật sự từ chối xóa, để BombVault dọn được theo quy tắc giữ của đích.
6. **Cho phần nào.** Đánh dấu **Containers** và **Tự sao lưu**. Tự sao lưu mang cài đặt của bạn, nên một bản cài mới có thể đưa chúng trở lại. Cửa sổ cho biết có bao nhiêu mục và, khi biết, bản sao đầu tiên mang theo bao nhiêu lịch sử.
7. Bấm **Lưu đích**.

Bản sao đầu tiên chạy sau bản sao lưu kế tiếp. Để bắt đầu ngay, hãy mở **Cài đặt, Đích sao lưu** và bấm **Sao chép ngay** trong dòng của phần đó ở **Bản sao ngoài site**. Phần đầu trạng thái ghi **Đang sao chép ra ngoài site**. Bản sao đầu tiên mang theo toàn bộ lịch sử, nên 200 GB qua đường lên 100 Mbit/s mất khoảng bốn tiếng rưỡi. Các bản sao sau chỉ gửi những gì mới.

Để giữ đường truyền của bạn rảnh, hãy mở trang của đích và đặt **Giới hạn tải lên (KiB/s)** trong thẻ **Sao chép**, ví dụ thành `5120`, đọc là khoảng 5,0 MB/s (41,9 Mbit/s). Nhân tiện hãy xem **Giữ trong bao lâu** trên cùng trang.

Để thấy nó đã hoạt động, hãy xem **Trạng thái bảo vệ** trên trang tổng quan, nơi **Ngoài site** chuyển sang **bản sao còn mới**, và tab **Bản sao lưu** của một container, nơi các snapshot của nó giờ nêu `tower-backups` là nơi thứ hai. Rồi bấm **Tải bộ khôi phục** lần nữa dưới **Cài đặt, Khôi phục**, vì bộ khôi phục liệt kê đích mới và thông tin đăng nhập của nó.

**Cloudflare R2 hoạt động theo cùng cách,** với một token đối tượng thay cho khóa. Dán địa chỉ S3 mà bảng điều khiển R2 hiển thị, kèm bucket, và BombVault điền máy chủ và bucket.

### Ví dụ: một rest-server do bạn tự chạy {#example-rest-server}

Một rest-server ở chế độ append-only từ chối xóa, nên một bản sao trên đó vẫn còn ngay cả khi ai đó chiếm quyền máy chủ của bạn. Một máy thứ hai, ở nhà bạn bè hay trong nhà kho, là đủ. Một BombVault đã ghép nối có thể thiết lập việc này cho bạn, xem [Máy chủ nhận](#receiving-server). Để tự chạy trên bất kỳ máy chủ Docker nào:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Dòng cuối hỏi một mật khẩu và ghi người dùng vào tệp `.htpasswd` trong thư mục dữ liệu.

1. Trong BombVault, bấm **Thêm đích** và chọn **rest-server** dưới **NAS hoặc máy chủ**.
2. Nhập địa chỉ với người dùng làm thư mục của nó, chẳng hạn `http://192.168.2.20:8000/bombvault`, cùng **Tên người dùng** và **Mật khẩu**. Bấm **Kiểm tra kết nối**.
3. Trong **Bảo vệ**, bật **Bảo vệ chống xóa (append-only)**. Khi đó BombVault không bao giờ dọn ở đó và để phía bên kia làm.
4. Đánh dấu các phần và bấm **Lưu đích**.
5. Khi bản sao đầu tiên đã có, hãy mở trang của đích và bấm **Kiểm tra bảo vệ chống xóa ngay**. BombVault thử xóa một đối tượng thử. Thẻ phải ghi **Lần gần nhất: xóa bị từ chối**, và **Bảo vệ chống xóa** trên trang tổng quan chuyển sang **đã kiểm tra**. Nếu ghi **Lần gần nhất: xóa không bị từ chối**, thì `--append-only` đang thiếu.

Vì không có gì dọn bản sao này từ đây, hãy đặt **Hạn mức tăng trưởng (GB, 0 = tắt)** trong cửa sổ của phần, mở bằng **Chỉnh sửa** ở dòng của nó trong **Bản sao ngoài site**, để bạn biết trước khi đĩa đầy. Với chứng chỉ tự ký, xem ghi chú về `certs` ở trên.

## Thư mục riêng của một phần {#own-folder}

Mỗi phần có một thư mục riêng: kho mà các bản sao lưu của nó được ghi vào trước tiên. Các bản sao tới những đích sao lưu khác đi ra từ đó. Một bản cài mới giữ các thư mục riêng trên máy chủ này, dưới `/mnt/user/bombvault`, và chúng cùng nhau tạo thành đích sao lưu **Cục bộ**.

Một thư mục riêng cũng có thể nằm trong một đích sao lưu bạn đã có, như một NAS, một rest-server hoặc một bucket S3. Khi đó phần sao lưu thẳng vào đó, không có bản sao cục bộ ở giữa, và thư mục riêng theo thông tin đăng nhập, bảo vệ chống xóa, giới hạn băng thông và quy tắc giữ của đích đó. Đích liệt kê phần trong thẻ **Thư mục riêng** của nó, và **Đích theo phần** hiển thị đích là nhà của phần.

### Chuyển một thư mục riêng {#move-own-folder}

1. Mở đích sao lưu sẽ giữ thư mục riêng và chọn **Dùng làm thư mục riêng**, hoặc **Thêm phần** trong thẻ **Thư mục riêng** của nó. Để chuyển nhiều phần cùng lúc, hãy chọn **Chuyển thư mục riêng** bên cạnh **Thư mục riêng ở đây** trong thẻ **Kết nối** của đích Cục bộ.
2. Chọn các phần dưới **Phần** và nơi đến dưới **Chuyển đến**. **Nơi khác** nhận một thư mục trên máy chủ này hoặc địa chỉ của một kho từ xa gõ tay, với một thông tin đăng nhập từ các thông tin đăng nhập đã lưu của bạn.
3. Đọc **Điều gì xảy ra**. Với từng phần, nó cho biết các bản sao lưu mới đi đâu, một bản sao đã nằm ở nơi mới có trở thành thư mục riêng không, và các bản sao lưu nào được sao chép sang trước.
4. **Sao chép các bản sao lưu đã tạo** được bật theo mặc định. Khi tắt, các bản sao lưu cũ ở yên chỗ và vẫn khôi phục được từ đó.
5. Chọn **Chuyển**. Khi cần sao chép các bản sao lưu trước, BombVault sao chép chúng rồi mới chuyển thư mục riêng. Khi chúng đã có ở đó, nó kiểm tra chúng trước khi chuyển.

Sau khi chuyển, các bản sao lưu mới đi tới thư mục riêng đã chuyển, kể cả với một phần đã tắt **Cục bộ** và sao lưu thẳng tới một bản sao. Một bản sao đã nằm ở nơi mới sẽ trở thành thư mục riêng, và việc sao chép tới đó kết thúc. Khi bản sao ấy giữ một số bản sao lưu lâu hơn quy tắc giữ của phần, phần sẽ giữ bất cứ thứ gì một trong hai quy tắc giữ, nên không bản nào bị mất.

Khi việc chuyển không thể tiến hành, cửa sổ nói lý do trong một khung dưới **Điều gì xảy ra**. Khi chỉ việc sao chép các bản sao lưu cũ là vướng, tắt **Sao chép các bản sao lưu đã tạo** cho phép thư mục riêng chuyển đi mà không có chúng.

Một phần luôn cần một thư mục riêng. Xóa đích sao lưu đang giữ nó sẽ hỏi thư mục mà phần chuyển tới, xem [Di chuyển và gỡ bỏ](#move-remove). Một thư mục riêng mà các bản sao lưu mới đi vòng qua, vì phần sao lưu thẳng tới một bản sao, được đánh dấu **(tắt)** ở nơi một đích liệt kê ai dùng nó.

![Cửa sổ chuyển một thư mục riêng, với Điều gì xảy ra đã điền cho một phần.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Cửa sổ chuyển một thư mục riêng, với Điều gì xảy ra đã điền cho một phần.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Ví dụ: Containers chuyển sang một NAS {#example-own-folder}

Pool cache đang đầy, và bạn có một đích sao lưu tên `nas`, một share SMB trên máy thứ hai. Phần Containers có 61 bản sao lưu trong thư mục riêng của nó trên máy chủ này và sao chép tới Backblaze B2.

1. Mở **Đích sao lưu**, rồi `nas`, và bấm **Dùng làm thư mục riêng**.
2. Đánh dấu **Containers** dưới **Phần**.
3. Đọc **Điều gì xảy ra**. Nó cho biết các bản sao lưu mới của Containers đi tới thư mục trên `nas`, 61 bản sao lưu ở Cục bộ được sao chép sang đó trước, các bản sao vẫn tiếp tục tới Backblaze B2, và bảo vệ, giới hạn băng thông cùng cảnh báo tăng trưởng đến từ `nas`.
4. Để **Sao chép các bản sao lưu đã tạo** bật và bấm **Chuyển**.

BombVault sao chép các bản sao lưu ở chế độ nền rồi chuyển thư mục riêng. Trang `nas` liệt kê Containers dưới **Thư mục riêng**, một mục theo phần của nó nêu `nas` là nhà, và bản sao lưu kế tiếp nằm ở đó, với bản sao tới B2 đi ra từ `nas`. Thư mục cũ trên máy chủ này ở yên chỗ, và BombVault không xóa gì ở đó.

### Gõ tay một địa chỉ từ xa {#remote-primary-repositories}

Thẻ **Kết nối** của đích Cục bộ vẫn nhận một địa chỉ từ xa cho thư mục riêng của một phần: **Đổi** bên cạnh phần mở một cửa sổ có công tắc **Cục bộ / Từ xa**. **Từ xa** nhận một địa chỉ restic như `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` hoặc `rclone:remote:bucket/path`, và **Cài đặt an toàn cho kho chính từ xa** thêm một phép kiểm tra kết nối, giới hạn tải lên và tải xuống, bảo vệ append-only với cùng phép thử can thiệp như một đích off-site, và một cảnh báo hạn mức tăng trưởng.

Một địa chỉ như vậy đăng nhập bằng thông tin đăng nhập mặc định. Khi nó nằm trong một thư mục của một đích sao lưu bạn có, trang của đích đó liệt kê phần dưới **Đã nằm dưới đích này**, và tiếp nhận nó sẽ cho nó thông tin đăng nhập, lớp lưu trữ và bảo vệ chống xóa của đích. Chuyển thư mục riêng vào một đích sao lưu bằng **Dùng làm thư mục riêng** là cách đơn giản hơn, vì cửa sổ kiểm tra các bản sao lưu đã tạo trước khi thay đổi bất cứ gì.

## Nơi lưu trữ theo từng mục {#placement}

Thẻ **Lịch trình và đích sao lưu** ở tab Cài đặt của một mục có một hàng nút: **Cục bộ** và một nút cho mỗi đích sao lưu off-site của phần. Các nút sáng nhận bản sao lưu của mục.

- Khi **Cục bộ** sáng, mục được ghi vào kho hiển thị dưới **Lưu tại** và được sao chép tới mọi đích sáng khác. Làm tối một đích và nó không nhận gì mới từ mục này. Chỉ Cục bộ thì không sao chép đi đâu, hợp với dữ liệu đã có bản sao thứ hai, ví dụ một share nằm trên NAS.
- Khi **Cục bộ** tối, mục được ghi thẳng vào kho trực tiếp của đích sáng đầu tiên và được sao chép từ đó tới các đích sáng khác. Lần đầu, một hộp thoại tạo kho trực tiếp đó.
- Một nút luôn sáng, vì một bản sao lưu cần một nơi để đi. Để loại một mục khỏi sao lưu, dùng **Không sao lưu**.

BombVault không bao giờ di chuyển bản sao lưu giữa các kho. Một mục theo phần của nó sẽ đi theo khi mặc định của phần đổi: bản sao lưu kế tiếp của nó rơi vào nơi mới, còn các bản sao lưu nó đã có ở nguyên chỗ. Một vị trí được chọn trên chính mục đó được cố định từ bản sao lưu đầu tiên của nó. Thẻ **Kho** ở tab Cài đặt nêu tên nó. Các bản sao có thể đổi bất cứ lúc nào. Một đích không còn nhận một mục giữ các bản sao nó có và cắt tỉa chúng theo lưu giữ của riêng nó ở lần chạy off-site kế tiếp của phần. Không xóa được khỏi đích append-only.

Nơi mà một mục đã rời đi vẫn giữ các bản sao lưu của nó. Trên tab **Bản sao lưu** của mục và trong **Khôi phục**, nó hiển thị là **Chỉ các bản sao lưu cũ hơn**, một lần khôi phục vẫn có thể đọc từ đó, và một lần cắt tỉa ở đó bỏ qua các bản sao lưu này.

**Theo mặc định** đưa một mục có lựa chọn riêng trở về phần của nó. Nó hỏi trước, cho biết các bản sao lưu mới khi đó đi đâu, và nêu tên những gì mỗi đích thôi nhận và những gì lần chạy kế tiếp sẽ tải lên.

Dưới hàng nút, thẻ cho biết mục đi đâu và thực tế có gì ở đó: có bao nhiêu nơi giữ nó, lần cuối thấy mỗi đích là khi nào, và 3-2-1 đã đạt chưa. Một nơi là máy chủ có dữ liệu gốc, mỗi đích off-site và mỗi kho được đánh dấu **Ngoài cơ sở**. BombVault kiểm tra các bản sao và các nơi. Nó không kiểm tra phần "hai loại phương tiện" của 3-2-1.

### Mặc định theo từng phần {#placement-defaults}

Thẻ **Đích theo phần** trên trang Đích sao lưu vẽ mặc định của mọi phần dưới dạng một luồng 3-2-1. Mỗi đường từ một phần tới một đích sao lưu là một công tắc, **Cục bộ** cũng vậy, cho mọi phần, kể cả ổ flash, tập dữ liệu ZFS và tự sao lưu. Một cú nhấp vào đường hoặc chấm sẽ chuyển nó ngay, và các đường đứt nét là đang tắt. Chỉ việc tắt **Cục bộ** của một phần mới hỏi trước, trong cùng hộp thoại với thẻ mặc định, vì khi đó phần sao lưu thẳng tới một bản sao. **Hủy** để mọi thứ như cũ. **Đồ thị / Công tắc** hiện cùng nội dung dưới dạng các hàng nút. Một công tắc quyết định các bản sao lưu mới đi đâu. Các mục không có lựa chọn riêng, và thư mục dự án của các stack Compose, đi theo ngay, nên bản sao lưu kế tiếp của chúng đi tới các nơi mới. Các bản sao lưu đã lưu ở yên chỗ và vẫn khôi phục được từ đó. Flash, tập dữ liệu ZFS và tự sao lưu sao lưu vào thư mục cục bộ của chúng và sao chép ra off-site từ bản sao lưu đó. Đường của chúng tới **Cục bộ** cũng có thể tắt. Khi đó các bản sao lưu của chúng đi thẳng tới các đích off-site, và một mục ZFS có bản nhân bản mà không có đích nào chỉ gửi bản nhân bản.

Khi **Cục bộ** tắt và chỉ các đích off-site đang bật, một phần sao lưu thẳng tới chúng, và BombVault tự chọn kho ở đó. Container, VM và bộ thư mục đi vào kho trực tiếp của đích đang bật đầu tiên, mô tả bên dưới, và được sao chép từ đó tới các đích khác. Ổ flash, các tập dữ liệu ZFS và mục tự sao lưu ghi mỗi bản sao lưu vào mọi đích đang bật, và mỗi đích giữ chúng theo lưu giữ của riêng nó.

Một phần có mọi đường đều tắt hiển thị **Chưa được sao lưu**. Các mục của nó không nhận bản sao lưu nào và được tính là không được bảo vệ.

Các mục đã chọn đích sao lưu riêng trên trang của chúng giữ nguyên chúng. Luồng cho biết trong một phần có bao nhiêu mục như vậy, ví dụ **2 mục khác biệt**, và dẫn tới chúng: tới tab Cài đặt của mục khi chỉ có một, tới một danh sách cho biết mỗi mục sao lưu ở đâu khi có nhiều mục. **Theo mặc định** ở đó đưa một mục trở lại.

Một đích off-site mới nhận mọi mục không đặt là Cục bộ. Cửa sổ thêm nó cho biết có bao nhiêu mục và, nơi biết, lịch sử bao nhiêu.

### Kho trực tiếp {#direct-repositories}

Một kho trực tiếp là nơi các mục sao lưu thẳng tới một đích off-site. BombVault đặt nó bên cạnh thư mục của đích với `-direct` thêm vào tên, hoặc, khi đích nằm ở gốc bucket, remote hay người dùng rest-server của nó, trong một thư mục bên trong đặt theo tên phần, như `containers-direct`. Trong luồng trên trang Đích sao lưu, nó được tạo mà không hỏi. Tắt Cục bộ cho một mục riêng lẻ, để một đích chưa có kho trực tiếp trở thành nhà của nó, sẽ mở một hộp thoại với đích được đề xuất, ví dụ `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, và một phép kiểm tra kết nối không tạo gì. **Tạo và dùng** tạo kho và trỏ mục tới nó. Một kho trực tiếp không có cài đặt riêng. Nó theo công tắc, thông tin đăng nhập, lớp lưu trữ, giới hạn, cài đặt append-only và chính sách lưu giữ của đích, và thay đổi cùng chúng. Trang của đích liệt kê nó dưới **Sao lưu thẳng vào đây**, với mỗi phần sao lưu thẳng tới nó và có bao nhiêu mục. Cái nào không còn gì sao lưu tới thì cho biết ở đó và có thể xóa, và các bản sao lưu ở nơi của nó vẫn còn. Khi một khóa mới của đích không mở được nó, kho trực tiếp giữ khóa đang có và lần lưu cho biết như vậy. Một mục trên kho trực tiếp được sao chép từ đó tới các đích sáng khác, không bao giờ tới đích mà kho thuộc về. Các snapshot của nó mang thẻ `bv:direct`, và mọi lượt lưu giữ khác đều giữ chúng, nên một kho trực tiếp mất liên kết với đích của nó không bao giờ bị già đi theo quy tắc cục bộ. B2 được tới qua endpoint S3 của nó, với key ID và application key nhập làm khóa S3. Một khóa giới hạn trong thư mục riêng của đích không tới được thư mục bên cạnh, nên hãy giới hạn khóa ở thư mục phía trên.

### Ngoài cơ sở {#off-the-premises}

Một đích sao lưu hoặc một kho có thể được đánh dấu **Ngoài cơ sở**. Các đích từ xa bắt đầu với dấu này. Hãy tắt nó cho một rest-server trong cùng tòa nhà. Dấu này chỉ dùng để đếm các địa điểm và 3-2-1 trên các thẻ. Nó không thay đổi bản sao nào.

Một bản nhân bản ZFS là một bản sao ngoài cơ sở, nhưng không phải một bản sao lưu có lịch sử. Nó chỉ giữ các snapshot bạn đặt, còn các phiên bản cũ và từng tệp vẫn đến từ các bản sao lưu. Nó đi từ một máy chủ ZFS thẳng sang máy kia, qua SSH tới một máy chủ ZFS hoặc tới điểm nhận của một phiên bản đã ghép nối. Nhóm ghép nối chỉ mang yêu cầu. Xem [Bản nhân bản](zfs-datasets.md#replica).

### Sau khi dựng lại {#after-a-rebuild}

Các lựa chọn sao chép nằm trong cài đặt riêng của BombVault. Sau khi dựng lại bằng **Tìm bản sao lưu** hoặc trình hướng dẫn khẩn cấp mà không khôi phục cài đặt, chúng mất đi, và sao chép tất cả sẽ gửi lại những mục bạn đã loại ra tới B2. Vì vậy các bản sao off-site của mọi phần được dựng lại sẽ tạm dừng. Trang tổng quan cho biết điều đó, và thẻ **Đích theo phần** đưa ra **Xác nhận mặc định** kèm bản xem trước những gì lần chạy tới sao chép và các tên trong những bản sao lưu không có mục, mà bạn có thể loại ra ở đó. Chỉ việc xác nhận mới kết thúc việc tạm dừng. Nhập một tệp cài đặt đưa các quy tắc và mặc định trở lại nhưng không kết thúc nó.

## Bảo vệ chống xóa (append-only) {#delete-protection-append-only}

Bật bảo vệ chống xóa cho một đích sao lưu off-site để ransomware, hoặc một máy chủ bị xâm nhập, không thể xóa hay ghi lại các bản sao lưu của bạn. Phía xa (một `restic/rest-server` chạy ở chế độ `--append-only`) **thực thi** nó. BombVault chỉ **xác minh** nó và không bao giờ hiện màu xanh chỉ vì một khẳng định cấu hình. Một thư mục trên máy chủ này không thể từ chối xóa, vì ai điều khiển BombVault cũng điều khiển thư mục, nên các đích cục bộ không có công tắc này.

Công tắc thuộc về đích: một phần sao chép tới đó sẽ lấy nó từ đích, nên hãy bật nó trên trang của đích trong **Đích sao lưu**. **Kiểm tra bảo vệ chống xóa** theo lịch chỉ chạy khi có một phần sao chép tới đích đã bật công tắc. Phía bên kia phải được khởi động với `--append-only`. Với image `restic/rest-server`, trên TrueNAS cũng như trên mọi máy chủ Docker khác, giá trị này nằm trong biến `OPTIONS`.

Một số nơi hoàn toàn không thể từ chối việc xóa, ví dụ một ổ đám mây như OneDrive. Nếu bạn cố ý giữ bản sao ở đó, hãy bật **Chấp nhận khi không có bảo vệ chống xóa** trên trang của vị trí. Phần tổng quan khi đó hiển thị việc thiếu bảo vệ của một phần chỉ sao chép đến các vị trí đã chấp nhận là rủi ro đã chấp nhận, bằng vòng tròn xám thay cho chấm màu hổ phách, và màu của phần đó theo các kiểm tra còn lại. Ngoài ra không có gì thay đổi: các bản sao lưu ở đó vẫn có thể bị xóa, BombVault vẫn dọn chúng theo quy tắc của vị trí, và công tắc biến mất ngay khi bảo vệ chống xóa được bật. Một bản sao chuyển sang nơi khác sẽ mất sự chấp nhận này. Bật bảo vệ chống xóa cũng hủy sự chấp nhận này.

Cách nhanh nhất để có một bản sao append-only là một BombVault đã ghép nối với một [máy chủ nhận](#receiving-server): nó chạy rest-server cho bạn, ở chế độ append-only.

!!! note "Xóa thành công dưới `/locks/` là hành vi mong đợi"
    Append-only không có nghĩa là không còn xóa được gì nữa. restic phải tự tạo và giải phóng khóa của nó, nên `/locks/` cố ý vẫn ghi và xóa được. Các snapshot và dữ liệu phía sau chúng, tức đúng thứ mà mã độc tống tiền nhắm tới, không thể bị xóa. Nếu bạn tự kiểm tra phía xa, một thao tác xóa thành công dưới `/locks/` là hành vi đúng chứ không phải lỗ hổng bảo vệ.

!!! warning "Kho append-only không bao giờ được dọn từ máy này"
    Một đích append-only không bao giờ dọn bản sao lưu cũ từ đây. Dọn dẹp là việc của phía xa. Hãy đặt một **cảnh báo ngân sách tăng trưởng** cho nó để bạn được báo trước khi dung lượng kho vượt tầm kiểm soát.

## Kiểm tra can thiệp {#tamper-test}

BombVault định kỳ chứng minh bảo đảm append-only bằng cách thực sự thử xóa trên kho off-site, nhắm vào một đối tượng không tồn tại. **Kiểm tra bảo vệ chống xóa ngay** trên trang của một đích chạy nó theo yêu cầu, và **Kiểm tra bảo vệ chống xóa** dưới **Cài đặt, Xác minh, Kiểm tra tự động** chạy nó theo lịch.

Tắt **Kiểm tra bảo vệ chống xóa** không che giấu lỗ hổng nào: một bảo vệ chưa từng được kiểm tra khi đó hiển thị là chưa chứng minh vì việc kiểm tra đang tắt, còn một vị trí không có bảo vệ chống xóa thì trường hợp nào cũng hiển thị là không có.

Bản sao đang tắt sẽ không được kiểm tra, và đích sẽ ghi rõ điều đó. Kiểm tra kết nối vẫn chạy được vì nó chỉ đọc, nên có thể kiểm tra một bản sao trước khi bật nó.

- **Bị từ chối** nghĩa là được bảo vệ.
- **Được chấp nhận** nghĩa là không được bảo vệ.
- Một kết quả **không kết luận được** (máy chủ không tiếp cận được, lỗi xác thực) không bao giờ lật ngược phán quyết đã lưu.

Một lần lật thực sự từ được-bảo-vệ sang không-được-bảo-vệ sẽ kích một cảnh báo duy nhất.

## Diễn tập DR {#dr-drills}

BombVault cung cấp hai cấp độ bằng chứng rằng các bản sao lưu của bạn thực sự khôi phục được, không chỉ hiện diện.

- **Diễn tập xác minh khôi phục (cục bộ).** BombVault định kỳ chạy `restic check --read-data-subset` (có giới hạn, không bao giờ là một lần khôi phục đầy đủ làm đầy đĩa). Chu kỳ nằm dưới **Cài đặt, Xác minh, Kiểm tra tự động**, và thẻ **Kiểm tra bản sao lưu** trên cùng ô hiển thị kết quả.
- **Khôi phục thử (off-site).** BombVault khôi phục một đích thật từ kho off-site vào một sandbox niêm phong, xác minh từng tệp và từng byte, rồi dọn dẹp. Điều này chứng minh bạn khôi phục được từ off-site, chứ không chỉ là kho có phản hồi.

Thẻ **Trạng thái bảo vệ** trên Tổng quan gộp điều này theo từng phần thành bốn kiểm tra: một bản sao lưu hiện hành trên máy chủ này, một bản sao off-site, bảo vệ chống xóa ở đó và một lần khôi phục thử từ đó. Nhấp vào một phần hiển thị chi tiết, và thẻ chỉ xanh trên các sự thật đã xác minh. Một vị trí bạn đã chấp nhận khi không có bảo vệ chống xóa được tính ở đó là rủi ro đã chấp nhận, không phải cảnh báo.

## Ghép nối các phiên bản {#pairing}

Nhiều BombVault làm việc cùng nhau như một nhóm: chúng hiển thị trạng thái bảo vệ của nhau, đảm nhận vai trò cho nhau và có thể giữ khóa của nhau. Một phiên bản gia nhập nhóm bằng mười hai từ.

![Trang ghép nối với ba thẻ từng bước.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Trang ghép nối với ba thẻ từng bước.](assets/screenshots/offsite/pairing-light.png#only-light)

Trên phiên bản đầu tiên mở **Cài đặt, Ghép nối** và nhấn **Tạo cụm từ**. Ba thẻ bước cho thấy cách làm. Mười hai từ hiện trong một cửa sổ có nút **Sao chép**. Trên mọi phiên bản khác mở cùng chỗ đó, nhấn **Nhập cụm từ** và dán hoặc gõ chúng, hoặc nhấn **Dán** trong cửa sổ đó. Một từ không có trong danh sách được nêu tên cùng vị trí khi bạn gõ, và từ cuối mang một checksum, nên một từ gõ sai hay bị đổi chỗ bị bắt trước khi bất cứ gì được ghép nối. Chỉ tạo cụm từ trên một phiên bản: hai phiên bản cùng tạo sẽ thành hai nhóm riêng. Nếu một phút sau không ai xuất hiện, trang đưa ra hai lối thoát: hiện lại các từ để nhập chúng ở bên kia, hoặc nhập các từ của phiên bản kia và gia nhập nhóm của nó trong một bước. Ghép nối hoạt động không cần mật khẩu đăng nhập, nhưng hãy đặt một mật khẩu: không có nó, ai mở được giao diện web đều đọc được các từ và, qua nhóm, yêu cầu các vai trò trao mật khẩu restic của các phiên bản trong đó. Thẻ ghép nối nói điều đó cho đến khi đặt mật khẩu. Khi có mật khẩu, hiện lại cụm từ sẽ hỏi nó. **Rời nhóm** đưa một phiên bản ra ngoài trở lại. Thẻ **Phiên bản** trên trang ghép nối liệt kê các thành viên của nhóm. Hình ai làm gì cho ai nằm ở trang Phiên bản.

Bất kỳ ai biết các từ đó đều có thể gia nhập nhóm, nên hãy coi chúng như một mật khẩu.

**Các thành viên tìm nhau thế nào.** Mỗi phiên bản biết địa chỉ mạng của chính nó từ trình duyệt của bạn vào lúc bạn đăng nhập, được liệt kê dưới **Địa chỉ đã biết** trong thẻ **Phiên bản này** trên trang ghép nối cùng những địa chỉ phiên bản tự tìm thấy. Hãy sửa nó ở đó nếu có reverse proxy hay một cổng bất thường phía trước. Trên cùng mạng, các thành viên thông báo địa chỉ đó bằng multicast và nói chuyện trực tiếp, và nơi multicast không đi qua được mạng container, như bridge mặc định của Docker, một phiên bản thay vào đó tìm các phiên bản khác trong subnet của mình bằng một lệnh gọi có chữ ký mà chỉ thành viên nhóm mới trả lời được, nên việc ghép nối vẫn xong trong vài giây mà không cần relay. Nếu không tìm thấy gì, **Không tìm thấy phiên bản kia?** nhận một địa chỉ nhập tay, cho subnet khác hoặc cổng không chuẩn. Các phiên bản ở mạng khác nhau đi qua một relay, chọn trong một hàng bốn lựa chọn ở thẻ **Relay** trên cùng trang. Mỗi dạng kèm một hình:

- **Relay của dự án** (mặc định): `parleyport.halleluja.design`, cũng là relay mà KnightLoader dùng. Không cần thiết lập gì cả.
- **Relay riêng**: container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) từ Unraid Community Apps (cũng có dưới **Cài đặt, Ứng dụng**). Nhập địa chỉ của nó trên mọi phiên bản cần dùng nó.
- **Phiên bản BombVault**: một trong các phiên bản của bạn đã truy cập được từ bên ngoài, với **Đóng vai trò relay** được bật. Phiên bản đó khi ấy trả lời tại `/relay/connect` trên địa chỉ riêng của nó, phía sau reverse proxy và chứng chỉ nó đã có, và chỉ cho nhóm của bạn vào. Nhập địa chỉ của nó trên mọi phiên bản cần dùng nó.
- **Không dùng relay**: các thành viên tự động tìm thấy nhau trong cùng một mạng, và không ở đâu khác.

**Những gì relay thấy.** Một nút cùng tên bên cạnh bốn dạng cho biết điều này với dạng bạn đã chọn. Mọi lệnh gọi giữa các thành viên được niêm phong bằng AES-256-GCM dưới một khóa dẫn xuất từ mười hai từ, và khóa đó không bao giờ rời các phiên bản của bạn. Relay biết một hash nhóm các kết nối, thông điệp dành cho phiên bản nào, kích thước bao nhiêu và khi nào đi qua. Một lệnh gọi trực tiếp trên mạng cục bộ được niêm phong như vậy và còn được ký, nên không gì phụ thuộc vào chứng chỉ tự ký mà một phiên bản cung cấp. Các lệnh gọi trao ra thứ gì đó (câu trả lời vai trò, mật khẩu restic, đăng nhập bên nhận, bản gửi khóa) cũng được ký bằng khóa của thành viên gửi chúng, được ghim lần đầu thấy. Xem [Mô hình bảo mật](configuration.md#security-model).

**Những gì đi qua nhóm.** Trạng thái bảo vệ trên trang Phiên bản, yêu cầu kiểm tra một phần ngay, yêu cầu vai trò và câu trả lời của chúng, đề nghị lưu trữ, và những gì bên nhận hoặc bên kéo cần khi vai trò của nó được cho phép: vị trí kho của phiên bản kia và mật khẩu restic của nó. Dữ liệu sao lưu thì không bao giờ. Nó vẫn đi thẳng tới các backend restic. APP_KEY cũng không: mật khẩu restic mở các kho của phiên bản đó và không gì khác, không phải bí mật đã lưu, phiên hay mã mở bản gửi của nó.

**Các mục từ trước khi ghép nối.** Các phiên bản được thêm bằng mã thông báo fleet, cùng kho đã nhận và nguồn kéo được thiết lập bằng APP_KEY của phiên bản kia, vẫn còn sau bản cập nhật và được đánh dấu **Ghép nối lại**. Chúng vẫn hoạt động: ở lần khởi động đầu tiên BombVault thay mỗi APP_KEY đã lưu bằng mật khẩu restic dẫn xuất từ nó. Hãy ghép nối cả hai phiên bản, rồi sửa mục và chọn phiên bản của nó. Cho đến lúc đó các mục như vậy nằm trong thẻ **Không có phiên bản** trên trang Phiên bản.

Nơi duy nhất vẫn nhận APP_KEY nhập tay là [Từ thư mục sao lưu cũ](#restore-from-another-bombvault-repo), cho trường hợp phiên bản kia đã mất và không thể trả lời trong một nhóm.

## Phiên bản và vai trò {#roles}

Trang **Phiên bản** (một ô cài đặt có thể ghim vào thanh bên) hiển thị nhóm như một mạng lưới ai làm gì cho ai, một thẻ cho phiên bản này và một thẻ cho mọi thành viên khác, điện thoại Android và máy chủ ZFS. **Ghép nối** ở dưới cùng bên phải dẫn tới trang ghép nối. Phần nào bật được đặt dưới **Cài đặt, Phiên bản, Nhiều máy chủ**: **Phiên bản**, **Bộ nhận** và **Kéo về**.

![Trang Phiên bản: nhóm dưới dạng một mạng, và một thẻ cho mỗi phiên bản với các ô vai trò của nó, một ô đang Không hoạt động.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Trang Phiên bản: nhóm dưới dạng một mạng, và một thẻ cho mỗi phiên bản với các ô vai trò của nó, một ô đang Không hoạt động.](assets/screenshots/offsite/instances-light.png#only-light)

Mỗi thẻ hiển thị trạng thái bảo vệ của thành viên và các vai trò của nó dưới dạng ô. Một vai trò là điều một phiên bản làm cho phiên bản khác:

- **Bộ nhận** giữ các bản sao off-site của máy chủ này trong một rest-server ở chế độ append-only. Máy chủ này gửi sau mỗi bản sao lưu và không xóa được gì ở đó, kể cả khi ai đó chiếm quyền nó.
- **Bên kéo** kéo các bản sao lưu của máy chủ này theo lịch riêng của nó. Máy chủ này không cần đang bật.
- **Máy chủ ZFS** nhận các bản nhân bản ZFS của máy chủ này. Mỗi mục ZFS chọn nó dưới **Bản nhân bản** trong cài đặt của mình.
- **Bên giữ khóa** giữ một bản niêm phong của bộ khôi phục của máy chủ này. Xem [Bên giữ khóa](#key-keeper).

Bật một vai trò gửi một yêu cầu, và không gì bắt đầu cho đến khi ai đó trên phiên bản kia nhấn **Cho phép**. Yêu cầu hiện trên thẻ của phiên bản đó như một huy hiệu, và **Chi tiết** của nó liệt kê nó cùng **Cho phép** và **Từ chối**. Một vai trò đã cho phép có thể bị thu hồi ở đó bất cứ lúc nào. Một đường nét đứt trong mạng là một vai trò còn đang chờ. Một phiên bản chạy BombVault 9.x không trả lời được yêu cầu, nên vai trò ở đó có hiệu lực ngay khi bạn bật, và yêu cầu từ một phiên bản 9.x chờ **Cho phép** ở đây.

**Chi tiết** trên một thẻ mở phần còn lại: kho đã nhận và nguồn kéo của thành viên đó, đề nghị lưu trữ, giám sát, **Xóa** và **Quên khóa**.

Một thành viên có thể đưa ra bộ nhớ off-site của riêng nó cho một thành viên khác qua nhóm. Quản trị viên bên kia thấy lời đề nghị trong **Chi tiết** của thành viên và chấp nhận hoặc từ chối. Chấp nhận sẽ tạo một thông tin đăng nhập đã lưu và một đích sao lưu bình thường, sau cùng các kiểm tra như một đích bạn tự gõ vào. Chỉ thông tin kết nối đi theo cách này, không bao giờ là dữ liệu sao lưu. **Kiểm tra ngay** yêu cầu một thành viên xác minh kho của một phần. Không có gì trên trang này có thể bắt đầu sao lưu, khôi phục hay xóa bất cứ thứ gì trên một máy khác.

Các phiên bản đã tắt và điện thoại đã đóng ứng dụng vẫn giữ thẻ của mình ở trạng thái **Không hoạt động**, màu xám, kèm thời điểm được thấy lần cuối, và hiện mờ trong mạng cho đến khi bạn **Xóa** chúng. Ngay sau khi máy chủ này khởi động, thẻ như vậy hiện **Đang kiểm tra** cho đến khi nhóm hoạt động trở lại. **Xóa** dùng được trên thẻ như vậy bất kể phần nào đang bật, và nó cũng kết thúc các yêu cầu đang mở của thành viên, các khe nhận ZFS đang rảnh và các yêu cầu khóa đang chờ của thành viên đó. Thẻ **Phiên bản** trên trang ghép nối cũng liệt kê những thành viên này là **Không hoạt động**.

## Bên nhận và bên kéo {#receiver}

Trên máy **nhận** các bản sao append-only từ một BombVault khác, các kho đã nhận cho bạn khả năng giám sát độc lập, chỉ đọc trên phần cứng nhận, nên một lỗi âm thầm ở đầu xa không bị bỏ sót. Bật **Bộ nhận** dưới **Cài đặt, Phiên bản**, rồi mở **Chi tiết** của phiên bản gửi trên trang Phiên bản và thêm kho đã nhận (chỉ đọc, mở bằng mật khẩu restic của phiên bản gửi, mà nó nhận qua [nhóm ghép nối](#pairing)). Bạn có:

- **Bản kiểm kê theo nguồn**, để thấy chính xác container, VM và bộ thư mục nào đã đến.
- **Nhận lần cuối** cho từng nguồn, để biết mỗi nguồn mới đến đâu.
- **Một `restic check` độc lập** chạy trên phần cứng bên nhận, nên tính toàn vẹn được xác minh ngay nơi dữ liệu thực sự nằm, không chỉ trên bên gửi.
- **Một công tắc người chết:** một cảnh báo khi một nguồn ngừng gửi trong một khoảng thời gian bạn đặt.
- **Cảnh báo toàn vẹn:** một cảnh báo khi một lần kiểm tra ở phía nhận thất bại.

Bên nhận hoàn toàn chỉ đọc. Nó không bao giờ ghi vào kho đã nhận, nên không bao giờ phá được bảo đảm append-only mà bên gửi dựa vào.

Bên kéo hoạt động theo chiều ngược lại: khi bật **Kéo về**, một phiên bản đọc các bản sao lưu của một thành viên khác theo lịch của riêng nó, vào một kho riêng cho mỗi nguồn, nên chúng không bao giờ được tính là bản sao lưu của máy này. Các nguồn của nó cũng nằm trong phần chi tiết của thành viên đó: vị trí kho, loại bản sao lưu nó chứa và bao lâu thì kéo một lần. Mật khẩu restic đến qua nhóm, không bao giờ là APP_KEY, và phía xa không cần thiết lập thêm gì và không cần đang chạy. Nguồn chỉ được đọc, và mỗi bên giữ thông tin đăng nhập riêng. Một nguồn `rclone:` bị từ chối, vì rclone sẽ tới nó bằng các remote của phiên bản này.

### Máy chủ nhận {#receiving-server}

Máy nhận cũng có thể chạy rest-server mà các máy khác sao chép tới. Nó được thiết lập lần đầu bạn cho phép một yêu cầu **Bộ nhận** đòi nó: cửa sổ nói máy chủ cần một máy chủ nhận và hỏi một thư mục trên một share, với **Thư mục mới** để tạo, và một cổng (8000 trừ khi container khác dùng). BombVault sau đó:

1. từ chối nếu đã có container tên `rest-server` hoặc container khác đang giữ cổng đó,
2. kéo `restic/rest-server` và khởi động nó qua socket Docker ở chế độ append-only, với kho riêng tư và một tệp đăng nhập trong thư mục,
3. ghi mẫu Unraid của nó vào ổ flash, để container vẫn chỉnh sửa được trong tab Docker, hoặc cho tải mẫu về khi không truy cập được ổ flash,
4. chạy kiểm tra can thiệp lên nó và cho biết nó có từ chối xóa hay không.

Các phiên bản trong nhóm của bạn khi đó thấy máy chủ trong số các nhà cung cấp dưới **Từ nhóm của bạn**, đặt theo tên máy nhận, và vai trò **Bộ nhận** đưa nó ra như lựa chọn an toàn. Mỗi phiên bản nhận một đăng nhập riêng lần đầu dùng máy chủ, và chỉ ghi vào thư mục riêng của nó ở đó. Với **Bộ nhận** được bật dưới **Cài đặt, Phiên bản**, **Máy chủ nhận** trên thẻ của phiên bản này mở lại máy chủ. Cửa sổ của nó liệt kê các đăng nhập dưới **Đối tác có thông tin đăng nhập**, và **Thu hồi thông tin đăng nhập** lấy một cái đi. Những gì phiên bản đó đã sao chép vẫn nằm trong thư mục. Thiết lập cũng tạo một **Thông tin đăng nhập ngoài nhóm**, mà cửa sổ hiển thị mật khẩu một lần.

Một phiên bản chỉ tới được máy nhận qua relay thì không dùng được máy chủ, vì relay không mang bản sao lưu. Hãy thêm địa chỉ của máy nhận trên trang ghép nối trước. Khi BombVault chạy trên một địa chỉ IP riêng (ví dụ trên br0), hãy điền **Địa chỉ cho đối tác (tùy chọn)**, vì máy chủ lắng nghe trên địa chỉ của host.

## Ví dụ: ghép nối hai máy Unraid {#worked-example-two-unraid-boxes-end-to-end}

Đây là một thiết lập hoàn chỉnh từ các phần ở trên, vì chúng dễ ghép lại hơn khi bạn đã thấy chúng được ghép một lần.

Hai máy: **TOWER** chạy container và đẩy bản sao lưu. **VAULT** nhận chúng và thực thi append-only. Hãy thay bằng tên, địa chỉ và đường dẫn share của riêng bạn.

**1. Ghép nối hai máy.** Trên TOWER mở **Cài đặt, Ghép nối**, nhấn **Tạo cụm từ**, và nhập mười hai từ trên VAULT dưới **Nhập cụm từ**. Trước tiên đặt mật khẩu đăng nhập trên cả hai.

**2. Trên TOWER, nhờ VAULT làm bên nhận.** Trên trang Phiên bản, bật **Bộ nhận** trên thẻ của VAULT, chọn các phần và **Máy chủ nhận của VAULT** làm nơi lưu, rồi gửi yêu cầu.

**3. Trên VAULT, cho phép.** Trang Phiên bản của VAULT hiển thị yêu cầu trên thẻ của TOWER. Nhấn **Cho phép**. Lần đầu, VAULT hỏi một thư mục cho máy chủ nhận của nó, chẳng hạn `user/appdata/rest-server`, khởi động rest-server ở chế độ append-only và chạy kiểm tra can thiệp với nó.

**4. Trở lại TOWER.** Từ đó TOWER sao chép tới VAULT sau mỗi bản sao lưu, và đích sao lưu mới hiện trên trang Đích sao lưu của TOWER cùng bảo vệ chống xóa của nó.

Kiểm tra can thiệp trả lời những gì:

| Kết quả | Điều đã xảy ra |
| --- | --- |
| **được bảo vệ** | VAULT đã từ chối lệnh xóa. Đây là trạng thái đạt duy nhất. |
| **không được bảo vệ** | VAULT đã chấp nhận một lần xóa. `--append-only` bị thiếu hoặc đã bị gỡ. |
| **không kết luận được** | Không thuộc trường hợp nào. Thường là địa chỉ không phải địa chỉ mà chính restic dùng, hoặc thông tin đăng nhập đã đổi. Không có gì được ghi lại và không có cảnh báo nào. |

**5. Theo dõi những gì đến, nếu bạn muốn.** Bật **Bộ nhận** dưới **Cài đặt, Phiên bản** trên VAULT, mở **Chi tiết** của TOWER và thêm kho đã nhận. VAULT nhận mật khẩu restic của TOWER qua nhóm. Không ai gõ khóa.

**6. Làm cho nó hai chiều.** Lặp lại các bước theo chiều ngược lại: VAULT nhờ TOWER làm bên nhận của nó. Mỗi máy khi đó thực thi append-only cho máy kia, và không máy nào xóa được bản sao lưu của máy kia.

Một rest-server bạn tự chạy cũng dùng được: thêm nó bằng **Thêm đích** như một rest-server, với người dùng và mật khẩu từ `.htpasswd` của nó, và bật bảo vệ chống xóa. Giữ `--append-only` trong tùy chọn của nó. Thiếu nó thì phía xa lại là một share thông thường.

!!! warning "Đường dẫn cục bộ là đường dẫn bên trong container"
    Nơi BombVault hỏi một thư mục trên máy chủ này, hãy nhập nó tương đối với điểm gắn kết của host, chẳng hạn `user/appdata/rest-server/bombvault-containers`, không phải `/mnt/user/appdata/...`. BombVault chạy trong một container, nơi `/mnt` của host được gắn ở chỗ khác. Một đường dẫn tuyệt đối của host không tồn tại bên trong. Nếu bạn dán một đường dẫn như vậy, BombVault cho biết đường dẫn tương đối nên dùng thay thế.

## Trình hướng dẫn khẩn cấp {#emergency}

**Trình hướng dẫn khẩn cấp** đưa trở lại một máy chủ đã mất hoặc được thiết lập lại. Nó bắt đầu trên trang **Khôi phục** dưới **Khẩn cấp**, và từ **Khôi phục từ bản sao lưu** trên trang chào của một bản cài mới. Nó chạy qua bốn bước: **Truy cập**, **Cài đặt**, **Lựa chọn** và **Khôi phục**.

1. **Truy cập.** Cho BombVault biết khóa của các bản sao lưu cũ, theo một trong ba cách:
    - **Tải lên bộ khôi phục**: tệp bạn đã tải xuống lúc thiết lập. Nó chứa khóa và mọi đích sao lưu, và BombVault điền phần còn lại từ đó. Một bộ khôi phục niêm phong bằng age hỏi danh tính mà nó được niêm phong cho. Nó dùng một lần và không được lưu.
    - **Qua nhóm**: một phiên bản trong nhóm của bạn giữ khóa dưới dạng bản gửi. Bạn cần mười hai từ của nhóm, mã mở bản gửi và một người ở phiên bản đó. Xem [Bên giữ khóa](#key-keeper).
    - **Nhập khóa**: `APP_KEY` của máy chủ cũ, và nơi các bản sao lưu của nó nằm.

    BombVault sau đó thử khóa trên từng đích sao lưu và cho biết với mỗi đích là đọc được, không tìm thấy, bị khóa bằng khóa khác, không truy cập được hay vẫn thiếu đăng nhập. Chưa có gì được lưu, và khóa chỉ ở trong bộ nhớ một thời gian giới hạn. **Tiếp nhận khóa** làm máy chủ này từ giờ ghi bản sao lưu bằng khóa đó. **Hoàn tác việc tiếp nhận** quay về khóa riêng của nó cho đến khi bản sao lưu đầu tiên đã chạy. Một máy chủ đã có bản sao lưu riêng từ chối việc tiếp nhận và chỉ tới [Từ thư mục sao lưu cũ](#restore-from-another-bombvault-repo). **Lưu thông tin đăng nhập** lưu các đăng nhập mà bộ khôi phục mang cho các đích sao lưu từ xa, và hỏi trước khi thay các đăng nhập mà máy chủ này đã có.
2. **Cài đặt.** **Khôi phục cài đặt** khôi phục cài đặt riêng của BombVault từ Tự sao lưu mới nhất: đích sao lưu, thông tin đăng nhập, lịch trình và quy tắc. BombVault khởi động lại để áp dụng chúng, qua Docker socket, nên cơ sở dữ liệu cài đặt đang chạy không bao giờ bị ghi đè dưới một handle đang mở, và trình hướng dẫn tự tiếp tục sau đó. Đăng nhập của máy chủ này vẫn giữ: bạn tiếp tục đăng nhập bằng mật khẩu đang dùng. Những bí mật máy chủ cũ đã lưu mà không tiếp nhận được sẽ được liệt kê, để bạn nhập lại. Việc khôi phục kết thúc mọi phiên và gỡ các khóa truy cập, nên hãy thiết lập lại chúng sau đó.
3. **Lựa chọn.** BombVault xem qua các bản sao lưu của bạn và liệt kê các container, VM, bộ thư mục và tập dữ liệu ZFS nó tìm thấy. Bản sao lưu mới nhất được chọn sẵn ở mọi nơi. Những gì máy chủ này còn có bắt đầu ở trạng thái tắt, vì một lần khôi phục sẽ thay thế chúng. Bộ thư mục sau khi dựng lại không còn biết chúng đến từ đâu, nên chúng đi vào một thư mục bạn chọn.
4. **Khôi phục.** Mọi thứ đã chọn quay lại trong một lượt, để dừng nếu bạn muốn tự khởi động từng container và VM, điều an toàn nhất sau một mất mát hoàn toàn. Sau đó tải xuống một bộ khôi phục mới: bộ cũ thuộc về bản cài trước.

![Trình hướng dẫn khẩn cấp ở bước đầu tiên, với ba cách vào.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Trình hướng dẫn khẩn cấp ở bước đầu tiên, với ba cách vào.](assets/screenshots/offsite/emergency-light.png#only-light)

### Ví dụ: một máy chủ mới từ bộ khôi phục {#example-emergency}

Máy chủ TOWER đã chết cùng các đĩa của nó. Nó có 23 container và 2 VM, và đã sao chép chúng cùng tự sao lưu của nó lên Backblaze B2. Bạn có `bombvault-recovery-kit.md` từ trình quản lý mật khẩu và một máy mới đã cài Unraid. `APP_KEY` cũ của nó đã mất, và bạn không cần đến nó.

1. **Cài BombVault** trên máy mới và cho nó một `APP_KEY` mới, như trên mọi bản cài mới. Hãy thiết lập liên kết VM trong **Cài đặt, Tích hợp, SSH máy chủ** nếu bạn muốn lấy lại các VM, vì việc khôi phục VM cần nó.
2. **Bắt đầu trình hướng dẫn.** Ở **Chào mừng bạn đến với BombVault**, chọn **Khôi phục từ bản sao lưu**. Trên trang Khôi phục, chọn **Bắt đầu trình hướng dẫn khẩn cấp** dưới **Khẩn cấp**.
3. **Truy cập.** Chọn **Tải lên bộ khôi phục** và thả tệp vào. BombVault thử khóa của nó trên mọi đích sao lưu và báo, ví dụ, **Đã đọc bộ khôi phục. Khóa của nó mở được 0 trong 2 vị trí sao lưu.** Thư mục cục bộ ghi **không tìm thấy**, vì đĩa đã chết, và Backblaze B2 ghi **thiếu thông tin đăng nhập**.
4. **Lưu thông tin đăng nhập.** Bộ khôi phục mang thông tin đăng nhập của các đích từ xa. Bấm **Lưu thông tin đăng nhập**, các đích được thử lại, và Backblaze B2 giờ ghi **đọc được**.
5. **Tiếp nhận khóa.** Máy chủ này giờ ghi các bản sao lưu bằng khóa cũ, nên các bản sao lưu mới khớp với các bản cũ. Nó giữ `APP_KEY` riêng của nó trong mẫu. **Hoàn tác việc tiếp nhận** dùng được cho đến khi bản sao lưu đầu tiên chạy.
6. **Cài đặt.** Chọn **Khôi phục cài đặt**, đọc bản tự sao lưu mới nhất từ B2. BombVault khởi động lại để áp dụng, và trình hướng dẫn tự tiếp tục. Các đích sao lưu, lịch trình và quy tắc của bạn đã trở lại, và bạn tiếp tục đăng nhập bằng mật khẩu của máy chủ này. Hãy nhập lại các bí mật mà trình hướng dẫn liệt kê, chẳng hạn cài đặt thông báo.
7. **Lựa chọn.** Đặt **Các bản sao lưu nằm ở đâu?** thành các bản sao off-site, vì các bản cục bộ đã đi cùng máy chủ cũ. BombVault liệt kê 23 container và 2 VM với snapshot mới nhất được chọn sẵn, và bạn có thể chọn snapshot khác cho bất kỳ mục nào.
8. **Khôi phục.** Bật **Để dừng sau đó**, cách an toàn nhất sau một mất mát hoàn toàn, rồi bắt đầu. **Điều gì xảy ra** cho biết những gì trở lại. Dòng kết quả ghi, ví dụ, **Đã khôi phục 25, thất bại 0. Khi cần, hãy khởi động chúng ở mục Bản sao lưu.**
9. **Hoàn tất.** Khởi động các cơ sở dữ liệu của bạn trước, rồi đến các ứng dụng dùng chúng. Mở **Cài đặt, Khôi phục** và tải một bộ khôi phục mới, vì bộ cũ thuộc về bản cài trước. Kiểm tra **Cần chú ý** trên trang tổng quan, và chạy **Sao lưu tất cả ngay** một lần khi mọi thứ đã hoạt động.

Nếu tự sao lưu cũng bị thiếu, hãy chọn **Bỏ qua, tôi không có Tự sao lưu**. Khi đó các đích sao lưu chỉ đến từ bộ khôi phục, bạn thiết lập lại lịch trình và quy tắc, và các bản sao off-site tạm dừng cho đến khi bạn xác nhận mặc định của các phần, xem [Sau khi dựng lại](#after-a-rebuild).

!!! tip "Di chuyển theo kế hoạch so với thảm họa"
    Trình hướng dẫn khẩn cấp khôi phục cài đặt riêng của BombVault từ một bản sao lưu. Với một lần chuyển sang máy mới *có kế hoạch*, bạn có thể thay vào đó mang cấu hình sang trực tiếp bằng thẻ **Xuất / nhập cài đặt** (một tệp JSON di động). Xem [Cấu hình](configuration.md#portable-settings-export-and-import).

### Từ thư mục sao lưu cũ {#restore-from-another-bombvault-repo}

**Từ thư mục sao lưu cũ** dưới **Nguồn khác** trên trang Khôi phục mở kho của một phiên bản BombVault *khác* (một share đã gắn dưới `/mnt`, hoặc một địa chỉ từ xa) bằng bộ khôi phục của phiên bản đó hoặc `APP_KEY` nhập tay, trong một phiên chỉ đọc dùng một lần. Duyệt các container, VM, bộ thư mục và tập dữ liệu ZFS lưu ở đó, chọn một bản sao lưu và khôi phục nó, và đối tượng được khôi phục trở thành một container, VM hoặc bộ thư mục cục bộ bình thường. Không bao giờ có gì được ghi vào kho kia, và cài đặt sao lưu của riêng bạn không bị đụng tới (phiên sống trong bộ nhớ và tự hết hạn). Chuyển một container từ máy chủ A sang máy chủ B không có nghĩa là trỏ lại cài đặt của bạn rồi hoàn lại sau. Cửa sổ này dùng một lần: nó mở một phiên, khôi phục những gì bạn chọn và quên phiên bản kia. Nếu bạn muốn một sắp xếp lâu dài, trong đó máy này kéo bản sao lưu của phiên bản khác vào kho của chính mình theo lịch, đó là vai trò **Bên kéo** trên trang Phiên bản.

Một container có mạng không tồn tại trên máy chủ này, ví dụ mạng `br0` của Unraid trên một máy chủ Docker thông thường, sẽ hiện phần chọn mạng dưới hàng của nó. BombVault tạo container trên mạng bạn chọn, giữ nguyên các mạng khác. Địa chỉ IP cố định và địa chỉ MAC thuộc về mạng cũ nên bị bỏ, và mạng mới sẽ cấp chúng.

## Bên giữ khóa {#key-keeper}

Bên giữ khóa là một phiên bản trong nhóm của bạn giữ một bản niêm phong của bộ khôi phục của máy chủ này. Nó không tự mở được bản đó. Nếu máy chủ này bị mất, một bản cài mới gia nhập nhóm của bạn sẽ xin bản đó từ bên giữ khóa và mở nó bằng mã mở bản gửi của bạn.

![Cửa sổ mã khôi phục: hai mươi ký tự thành bốn nhóm, chỉ hiện một lần.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Cửa sổ mã khôi phục: hai mươi ký tự thành bốn nhóm, chỉ hiện một lần.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Trên trang Phiên bản, bật **Bên giữ khóa** trên thẻ của phiên bản cần giữ, và chọn **Gửi khóa**. Việc gửi khóa cần mật khẩu đăng nhập.
2. Máy chủ này hiển thị **Mã mở bản gửi** một lần: hai mươi chữ cái và chữ số trong bốn nhóm năm. Hãy ghi lại và xác nhận bằng **Tôi đã ghi lại mã**. Máy chủ này không lưu mã, chỉ lưu những gì cần để niêm phong một bản gửi cho nó.
3. Bên giữ khóa giờ giữ bản gửi, và máy chủ này làm mới nó khi bộ khôi phục đổi. **Bản gửi khóa** trên thẻ của phiên bản này liệt kê **Bản gửi của máy chủ này** cùng từng bên giữ khóa và trạng thái của nó, và **Các bản gửi máy chủ này đang giữ** cho những bên khác. **Tạo mã mới** thay mã. Mỗi bên giữ khóa chuyển sang mã mới khi nhận bản gửi mới.

Vào ngày đó bạn cần ba thứ: mười hai từ của nhóm, mã mở bản gửi, và quyền vào giao diện của bên giữ khóa, nơi ai đó phải nhấn **Cho phép**. Trong Trình hướng dẫn khẩn cấp chọn **Qua nhóm**, chọn bản gửi, xin nó và gõ mã mở bản gửi. Mã được dùng trên máy chủ mới và không gửi cho bên giữ khóa. Trên bên giữ khóa, yêu cầu hiện dưới **Bản gửi khóa** cùng ai xin, phiên bản đó đã từng được thấy ở đó chưa và đã xin bao nhiêu lần. **Cho phép** trao bản niêm phong, và việc cho phép cần mật khẩu đăng nhập. Bản cài mới có 30 phút để lấy nó sau khi Cho phép, một yêu cầu không ai trả lời hết hạn sau một ngày, và một phiên bản xin quá thường xuyên phải chờ một giờ.

Không có mã thì không ai mở được bản đó, kể cả bên giữ khóa. Nếu mất mã, hãy tạo mã mới khi máy chủ này còn chạy. Nếu máy chủ đã mất, bộ khôi phục và APP_KEY nhập tay vẫn dùng được. Bên giữ khóa có thể xóa một bản gửi bất cứ lúc nào, và chủ sở hữu khi đó phải xin lại.

## Bộ khôi phục {#recovery-kit}

Đây là mảnh khiến việc khôi phục sau thảm họa trở nên khả thi ngay cả khi không có một BombVault đang chạy.

**Tải bộ khôi phục** trong thẻ **Cho trường hợp khẩn cấp** dưới **Cài đặt, Khôi phục** cho bạn **khóa**, **mật khẩu restic dẫn xuất**, mọi **đích sao lưu** cùng kho của từng phần, các đăng nhập mà một bản cài mới cần để tới chúng, và **các lệnh restic chính xác**, để bạn khôi phục thẳng bằng restic CLI trên bất kỳ máy nào. Nó cần mật khẩu đăng nhập. Bộ khôi phục in khóa cả khi mã hóa tắt, vì định nghĩa container và VM luôn được niêm phong bằng nó. Sau một lần tiếp nhận nó in khóa sao lưu được tiếp nhận và APP_KEY của máy chủ này, thứ mà bản cài mới chỉ cần để đưa cài đặt trở lại. Một thẻ **Lưu bộ khôi phục của bạn** trên Tổng quan nhắc bạn cho đến khi bạn đã cất nó.

!!! danger "Cất giữ bộ khôi phục ngoài máy chủ"
    Bộ khôi phục chứa bí mật giải mã các bản sao lưu của bạn. Hãy cất nó ở nơi an toàn tách biệt khỏi máy chủ (trình quản lý mật khẩu, một bản in trong két). Nếu bạn mất cả BombVault lẫn `APP_KEY` mà không có bộ khôi phục và không có bản gửi khóa, các bản sao lưu đã mã hóa của bạn không thể khôi phục được.

!!! warning "Snapshot mới nhất không phải lúc nào cũng là cái nên khôi phục"
    Từ restic 0.17, `restic snapshots` hiển thị dung lượng của mỗi snapshot. Sau khi mất dữ liệu, snapshot mới nhất có thể là snapshot đã bị làm trống, nên đừng khôi phục một snapshot nhỏ hơn nhiều so với các snapshot trước. Sau ransomware, nó có thể là snapshot đã bị mã hóa với dung lượng thông thường. Nếu BombVault vẫn chạy, hãy xem trang **Bất thường** của nó trước: nó nêu tên bản sao lưu tốt gần nhất, và cửa sổ khôi phục chọn sẵn bản đó. Một lần khôi phục không cần bất kỳ dữ liệu bất thường nào của BombVault, và việc tạm dừng lưu giữ chỉ giữ thêm snapshot.

### Niêm phong bộ khôi phục {#sealing-the-kit}

Nếu bạn đã bật **Mã hóa bản xuất bằng age** dưới **Cài đặt, Hệ thống**, bộ khôi phục cũng được niêm phong bằng nó và tải xuống dưới dạng `bombvault-recovery-kit.md.age`. Nó được bọc ASCII chứ không phải nhị phân, nên vẫn là văn bản thuần: dán vào trình quản lý mật khẩu hay in ra hoạt động y như trước, chỉ là nội dung không đọc được nếu thiếu khóa của bạn.

!!! warning "Đừng cất khóa age bên trong bộ khôi phục"
    Bạn cần khóa **riêng** age để mở một bộ khôi phục đã niêm phong. Hãy cất nó ở nơi không phụ thuộc vào chính bộ khôi phục, nếu không bạn sẽ có hai thứ phải khôi phục thay vì một. Niêm phong đáng làm khi bộ khôi phục được cất ở nơi bạn không hoàn toàn kiểm soát (một trình quản lý mật khẩu dùng chung, ghi chú trên đám mây, một bản in ở văn phòng). Một bộ khôi phục trong két sắt của chính bạn đã được két sắt bảo vệ.

    Khi bật mã hóa mà không cấu hình người nhận dùng được, việc tải xuống bị từ chối thẳng. BombVault không bao giờ quay về giao khóa chính dưới dạng văn bản rõ.

### Khi không có bộ khôi phục trong tay {#if-you-do-not-have-the-kit-to-hand}

Mật khẩu không được lưu ở đâu cả, nó được **tính** từ khóa của các bản sao lưu, nên bạn có thể tự tạo lại nó chỉ với khóa và một shell:

```sh
printf 'bombvault:restic-repo' \
```

Đó là HMAC-SHA256 trên chuỗi cố định `bombvault:restic-repo`, với khóa là các byte thô của `APP_KEY` thập lục phân, in ra dưới dạng 64 ký tự thập lục phân chữ thường. Cùng giá trị đó có trong bộ khôi phục, liệt kê là mật khẩu restic dẫn xuất. Đây là cho ngày bộ khôi phục không ở gần bạn. Trên một máy chủ đã tiếp nhận bản sao lưu của máy khác, hãy dùng khóa sao lưu được tiếp nhận, tức APP_KEY cũ của máy chủ đó.

!!! warning "Với kho đã nhận, hãy dùng khóa của bản chạy GỬI"
    Một kho đến đây qua bản sao off-site được tạo bởi máy gửi nó, với khóa **của nó**. Dẫn xuất từ khóa của máy nhận tạo ra một mật khẩu mà restic sẽ từ chối, và điều đó đọc giống hệt một kho bị hỏng nhưng không phải vậy. Đây là lý do thông thường khiến `restic check` trên kho đã nhận cứ hỏi mật khẩu hết lần này đến lần khác.

Vì các định nghĩa khôi phục nằm **bên trong** mỗi kho (`<repo>/def`, `<repo>/vm-def`), một thư mục kho được sao chép hoàn toàn tự chứa, nên bộ khôi phục cộng với kho là tất cả những gì một lần khôi phục bare-metal cần.

## Lấy lại một bản kết xuất cơ sở dữ liệu {#database-dumps}

Một bản kết xuất cơ sở dữ liệu là một điểm khôi phục riêng trong kho containers, gắn thẻ `dbdump:<container>`, chứa một tệp duy nhất `/dbdump/<container>.sql`. BombVault liệt kê, tải xuống và nhập chúng trên tab **Bản sao lưu** của container. Dưới đây là các bước tương tự chỉ với restic, cho ngày BombVault không còn đó.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Các nhãn `dbversion:` và `dbname:` trên mỗi bản kết xuất cho biết nó đến từ phiên bản máy chủ nào và chứa những cơ sở dữ liệu nào. Một tệp trọn vẹn kết thúc bằng `-- PostgreSQL database cluster dump complete` hoặc `-- Dump completed`.

Hãy nhập nó vào một container cùng phiên bản hoặc mới hơn (PostgreSQL), hoặc cùng phiên bản chính (MySQL và MariaDB), đã được khởi động một lần với thư mục dữ liệu trống để nó tự khởi tạo. Máy chủ vật lý không cần trình khách cơ sở dữ liệu, container đã có sẵn:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Để lấy một cơ sở dữ liệu duy nhất từ bản kết xuất đầy đủ, MySQL và MariaDB nhận `--one-database <name>` trên lệnh trình khách. Bản kết xuất PostgreSQL có một phần cho mỗi cơ sở dữ liệu, mỗi phần mở đầu bằng dòng `\connect <name>`: chép phần đó ra tệp riêng rồi nhập bằng `-d <name>` sau khi đã tạo cơ sở dữ liệu.

!!! warning "Bản kết xuất lấy bằng root mang theo người dùng của máy chủ"
    Bản kết xuất đầy đủ của MySQL hay MariaDB lấy bằng root có chứa cơ sở dữ liệu hệ thống `mysql`, nên khi nhập, các tài khoản của máy chủ mới, kể cả mật khẩu root, sẽ bị thay bằng tài khoản trong bản kết xuất. Trên PostgreSQL, thông báo `role ... already exists` về người dùng do chính container tạo là chuyện bình thường và vô hại.
