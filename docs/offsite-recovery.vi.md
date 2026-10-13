# Off-site & khôi phục

!!! note "Các bản sao off-site chờ sau một lần xây dựng lại"
    Khi Trình hướng dẫn khẩn cấp hoặc **Tìm bản sao lưu** dựng lại các mục mà không có cài đặt cũ, bản sao off-site của các phần đó tạm dừng cho đến khi mặc định của chúng được xác nhận. Xem [Sau khi dựng lại](#after-a-rebuild).

Bản sao lưu cục bộ bảo vệ bạn khỏi một container bị mất hay một bản cập nhật hỏng. Bản sao off-site và một bộ khôi phục đã được thử nghiệm bảo vệ bạn khỏi mất cả máy, ransomware hay hỏa hoạn. Trang này nói về đích sao lưu và bản sao off-site, cách làm một bản sao chống can thiệp, chứng minh rằng bạn khôi phục được, ghép nối nhiều máy chủ và lấy lại mọi thứ khi chính BombVault đã mất.

## Bản sao off-site {#off-site-copies}

Giữ bản sao lưu cục bộ nhanh và thêm một hoặc nhiều đích sao lưu ngoài cơ sở. BombVault sao chép các bản sao lưu mới tới đó bằng `restic copy` theo kiểu hết sức có thể, nên trục trặc off-site không bao giờ làm hỏng bản sao lưu cục bộ. Theo cách này kho cục bộ vẫn là kho chính còn kho off-site là một bản sao, nhưng kho chính của một phần hoàn toàn không cần là cục bộ; xem [Kho chính từ xa](#remote-primary-repositories) bên dưới để sao lưu thẳng vào S3, một rest-server và những thứ tương tự thay vì sao chép tới đó.

- **Nhiều đích sao lưu cho mỗi phần.** Mỗi phần (container, VM, flash, tự sao lưu, thư mục và tập dữ liệu ZFS) có thể sao chép tới nhiều đích off-site cùng lúc, nên bạn có thể giữ, chẳng hạn, một rest-server trên máy của bạn bè và một bucket S3 song song. Thẻ **Đích theo phần** trên trang Đích sao lưu bật hoặc tắt từng dòng từ một phần tới một đích.
- **Khi nào các bản sao chạy.** Thẻ **Sao chép** của một đích đặt **Khi nào sao chép**: **Sau mỗi bản sao lưu**, hoặc theo kế hoạch riêng thu thập các bản sao lưu và sao chép vào những giờ cố định. Dưới **Cài đặt, Lịch trình, Bản sao ngoài site theo phần**, **Ngoại lệ** cho một phần kế hoạch sao chép riêng. **Sao chép ngay** trong thẻ **Bản sao ngoài site** của một phần dưới **Cài đặt, Đích sao lưu** bắt đầu một lần sao chép theo yêu cầu.
- **Lưu giữ** được đặt cho từng đích sao lưu dưới **Giữ trong bao lâu**, nên bạn có thể giữ bản sao off-site lâu hơn làm kho lưu trữ. Bản sao trong trường off-site của một phần theo thẻ **Lưu giữ ngoài site** dưới **Cài đặt, Đích sao lưu**; để toàn bộ bằng 0 nếu không bao giờ muốn tự động cắt tỉa bản sao lưu off-site.
- **Giới hạn băng thông** (**Băng thông off-site** dưới **Cài đặt, Đích sao lưu**, hoặc thẻ **Sao chép** của một đích) giới hạn tốc độ tải lên/tải xuống của restic để các bản sao không làm nghẽn WAN của bạn.
- Khi một bản sao đang chạy, đầu trạng thái của Tổng quan ghi **Đang sao chép ra ngoài site**. Đó là một chỉ báo hoạt động, không phải thanh phần trăm, vì `restic copy` không cung cấp tiến độ đọc được bằng máy.

!!! note "Khôi phục từ bất kỳ nơi nào"
    Mỗi mục liệt kê các bản sao lưu của nó thành một dòng thời gian qua mọi nơi mà một bản sao lưu nằm, trên tab **Bản sao lưu** của nó. Một bản sao lưu được sao chép lên B2 hiện một lần, đánh dấu từng nơi giữ nó. Một lần khôi phục lấy nơi đầu tiên nó tới được, bắt đầu với kho mà mục được ghi vào, và bạn có thể chọn nơi khác cho từng bản sao lưu. Đích off-site chỉ được đọc khi bạn nhấp **Kiểm tra**, vì liệt kê lưu trữ đám mây mất thời gian. Xóa ở một nơi trước tiên kiểm tra các nơi khác và cho biết đó có phải bản sao cuối cùng không.

## Đích sao lưu {#destinations}

Đích sao lưu là nơi các bản sao lưu đi tới: một thư mục cục bộ, một share, máy chủ của riêng bạn hoặc một dịch vụ đám mây. Nó được thiết lập một lần rồi phục vụ mọi phần. Mở **Đích sao lưu** từ ô cài đặt của nó, hoặc bật **Hiện cả trong thanh bên** ở đó để ghim.

**Thêm đích** mở một cửa sổ gồm bốn bước:

1. **Nhà cung cấp.** Mọi dịch vụ được liệt kê cùng logo, với một ô tìm kiếm phía trên, theo nhóm: dịch vụ lưu trữ có bucket S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 và các dịch vụ khác), máy chủ S3 của riêng bạn (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), máy chủ và share của riêng bạn (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, một đường dẫn đã gắn), lưu trữ đám mây (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud và phần còn lại mà rclone hỗ trợ) và các máy chủ nhận của nhóm bạn. Mỗi dịch vụ cho biết nó hợp với việc sao lưu đến đâu: ổ đám mây chậm đi khi có nhiều yêu cầu, nên bản sao lưu đầu tiên và việc dọn dẹp ở đó lâu hơn.
2. **Đăng nhập.** Các trường tùy theo dịch vụ: khóa truy cập cho S3, người dùng và mật khẩu cho WebDAV và SMB, mật khẩu ứng dụng nơi đăng nhập hai yếu tố chặn mật khẩu thường, khóa SSH công khai của BombVault cho SFTP và Storage Box, hoặc mã thông báo cho các dịch vụ đăng nhập qua trình duyệt. Với các dịch vụ đó, cửa sổ hiển thị một lệnh `rclone authorize` để chạy trên máy tính có trình duyệt; mã thông báo nó in ra được dán vào trường. **Kiểm tra** kiểm tra đăng nhập trước khi lưu bất cứ gì. Sau đó cửa sổ liệt kê các thư mục trên đích sao lưu, với **Thư mục mới** để tạo một thư mục và dung lượng trống nơi dịch vụ báo cáo. Một thư mục trống là an toàn nhất; BombVault đặt mỗi phần một thư mục trong đó.
3. **Bảo vệ.** Cửa sổ nói thẳng dịch vụ làm được gì. Một rest-server ở chế độ append-only từ chối xóa, và kiểm tra can thiệp xác nhận điều đó. Một bucket S3 có thể giữ phiên bản cũ nhờ versioning và object lock, điều BombVault chưa kiểm tra được. Một ổ đám mây hoàn toàn không từ chối xóa được: ai vào được máy chủ thì cũng vào được bản sao đó. Chỉ bật **Bảo vệ chống xóa (append-only)** khi phía xa thật sự từ chối xóa; khi đó BombVault không bao giờ dọn ở đó. **Phòng khi khẩn cấp** cho biết bộ khôi phục chứa gì cho đích này.
4. **Cho phần nào.** Chọn các phần sao lưu tới đây. Bản sao đầu tiên mang theo cả lịch sử. Các mục đã chọn đích sao lưu riêng không tự nhận đích mới, và cửa sổ nêu tên chúng.

Dịch vụ S3 chạy qua backend S3 riêng của restic, điều cho phép áp dụng lớp lưu trữ và object lock. Mọi dịch vụ khác chạy qua rclone mà BombVault đi kèm, và remote của nó khi đó hiện trong thẻ **Ngoại vi (rclone)** dưới **Cài đặt, Đích sao lưu**. Một bản xuất cài đặt mang theo các đích sao lưu; khi gồm cả thông tin xác thực, nó mang theo cả đăng nhập của chúng.

Mỗi đích sao lưu có một trang riêng:

- **Kết nối** với tên của nó, công tắc **Sẵn dùng** (tắt nghĩa là BombVault không còn ghi vào đây, và các mục sao lưu tới đây thất bại với thông báo rõ ràng thay vì rơi vào nơi khác), thư mục, nén, lớp lưu trữ và **Ngoài cơ sở**. **Đổi kết nối** kiểm tra và lưu một địa chỉ hoặc đăng nhập mới.
- **Thông tin xác thực**: bộ dùng chung hoặc bộ riêng.
- **Bảo vệ chống xóa**, với **Kiểm tra bảo vệ chống xóa ngay** và ai dọn bản sao này.
- **Sao chép**: khi nào sao chép, cùng giới hạn tải lên và tải xuống.
- **Giữ trong bao lâu**: **Ngắn**, **Cân bằng**, **Dài** hoặc **Giá trị riêng**, với dòng thời gian cho thấy quy tắc giữ những bản sao lưu nào. **Hỏi trước khi giữ ít hơn** hỏi trước khi một quy tắc ngắn hơn xóa các bản sao lưu mà bản duy nhất nằm ở đây.
- **Quy tắc theo phần**: kho, lưu giữ hoặc nén riêng của một phần tại đích này. **Đổi quy tắc** mở chúng, và **Đặt lại theo đích** đưa một phần trở lại.
- **Đã nằm dưới đích này**: các phần có địa chỉ gõ tay nằm trong một thư mục của đích này. Tiếp nhận một phần để kho ở nguyên chỗ; đăng nhập, lớp lưu trữ và bảo vệ chống xóa khi đó đến từ đích. BombVault trước tiên kiểm tra đăng nhập của đích mở được kho, và từ chối đặt một kho append-only dưới một đích không phải append-only.
- Một bình chứa cho thấy đích đầy đến đâu, với vạch nơi nó đầy và còn bao nhiêu tuần theo mức tăng của những tuần gần đây. Một đích không báo cáo dung lượng ghi **Chưa rõ dung lượng**.

Một máy chủ nhận mà phiên bản khác trong nhóm của bạn chạy hiện trong số các nhà cung cấp với tên **Từ nhóm của bạn**; xem [Máy chủ nhận](#receiving-server).

### Di chuyển và gỡ bỏ {#move-remove}

- **Di chuyển** đưa bản sao của một phần sang đích sao lưu khác, cùng toàn bộ lịch sử. Mọi thứ ở lại nơi cũ; BombVault không xóa gì ở đó. Việc di chuyển chạy nền và có thể lâu: khoảng 9 phút cho mỗi 100 GB giữa các đĩa cục bộ, hàng giờ qua đường truyền chậm. Tổng quan hiển thị nó khi đang chạy.
- **Xóa đích sao lưu** có hai cách. **Xóa đích sao lưu, giữ các bản sao lưu** ngừng ghi vào đó và để nguyên các bản sao lưu; sau này bạn đọc được chúng dưới **Khôi phục, Từ thư mục sao lưu cũ**. **Xóa đích sao lưu và xóa các bản sao lưu ở đó** trước tiên đọc những gì được lưu ở đó và liệt kê: bản sao lưu nào sẽ mất, bản nào không có ở nơi nào khác, và kho nào ở lại vì bảo vệ chống xóa từ chối, các mục vẫn sao lưu thẳng vào đó, một đích hay phần khác giữ bản sao ở đó, hoặc không đọc được. Các mục sẽ mất bản sao lưu duy nhất được liệt kê riêng và được giữ lại trừ khi bạn đánh dấu **Xóa cả những mục này**. Bạn xác nhận bằng cách gõ tên của đích, và việc xóa chạy nền.
- Xóa các bản sao lưu ở một đích sao lưu cần mật khẩu đăng nhập và đăng nhập ngay tại phiên bản đó. Không có mật khẩu thì đích vẫn gỡ được, chỉ là không xóa được các bản sao lưu của nó từ đây.

## Nơi lưu trữ theo từng mục {#placement}

Thẻ **Lịch trình và đích sao lưu** ở tab Cài đặt của một mục có một hàng nút: **Cục bộ** và một nút cho mỗi đích sao lưu off-site của phần. Các nút sáng nhận bản sao lưu của mục.

- Khi **Cục bộ** sáng, mục được ghi vào kho hiển thị dưới **Lưu tại** và được sao chép tới mọi đích sáng khác. Làm tối một đích và nó không nhận gì mới từ mục này. Chỉ Cục bộ thì không sao chép đi đâu, hợp với dữ liệu đã có bản sao thứ hai, ví dụ một share nằm trên NAS.
- Khi **Cục bộ** tối, mục được ghi thẳng vào kho trực tiếp của đích sáng đầu tiên và được sao chép từ đó tới các đích sáng khác. Lần đầu, một hộp thoại tạo kho trực tiếp đó.
- Một nút luôn sáng, vì một bản sao lưu cần một nơi để đi. Để loại một mục khỏi sao lưu, dùng **Không sao lưu**.

Vị trí được cố định từ bản sao lưu đầu tiên của mục, vì BombVault không bao giờ di chuyển bản sao lưu giữa các kho; thẻ **Kho** ở tab Cài đặt nêu tên nó. Các bản sao có thể đổi bất cứ lúc nào. Một đích không còn nhận một mục giữ các bản sao nó có và cắt tỉa chúng theo lưu giữ của riêng nó ở lần chạy off-site kế tiếp của phần. Không xóa được khỏi đích append-only.

Dưới hàng nút, thẻ cho biết mục đi đâu và thực tế có gì ở đó: có bao nhiêu nơi giữ nó, lần cuối thấy mỗi đích là khi nào, và 3-2-1 đã đạt chưa. Một nơi là máy chủ có dữ liệu gốc, mỗi đích off-site và mỗi kho được đánh dấu **Ngoài cơ sở**. BombVault kiểm tra các bản sao và các nơi; nó không kiểm tra phần "hai loại phương tiện" của 3-2-1.

### Mặc định theo từng phần {#placement-defaults}

Thẻ **Đích theo phần** trên trang Đích sao lưu là mặc định của mọi phần. Trong biểu đồ mỗi đường là một bản sao, và nhấp vào một đường hay một điểm sẽ bật hay tắt nó; đường nét đứt là đang tắt. **Biểu đồ / Công tắc** hiển thị cùng thứ dưới dạng các hàng nút. Các bản sao áp dụng ngay cho mọi mục không có lựa chọn riêng và cho các thư mục dự án của Compose stack. Đích áp dụng cho một mục mới ở bản sao lưu đầu tiên của nó; đổi nó không di chuyển bản sao lưu nào. Trước khi lưu, BombVault nêu tên mọi đích được thêm hay mất mục và điều đó có nghĩa là bao nhiêu bản sao lưu.

Các mục sao lưu ở nơi khác phần của chúng nằm trong thẻ **Lựa chọn riêng**, mỗi mục có **Theo mặc định**. Các bản sao lưu mới khi đó đi tới các đích của phần, và những bản trước ở nguyên chỗ.

Một đích off-site mới nhận mọi mục không đặt là Cục bộ. Cửa sổ thêm nó cho biết có bao nhiêu mục và, nơi biết, lịch sử bao nhiêu.

### Kho trực tiếp

Tắt Cục bộ cho một mục, để một đích không có kho trực tiếp trở thành nhà của nó, mở một hộp thoại với một vị trí gợi ý bên cạnh, ví dụ `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, và một lần kiểm tra kết nối không tạo gì cả. **Tạo và dùng** tạo kho và trỏ mục vào đó. Một kho trực tiếp lấy khóa, lớp lưu trữ, giới hạn, thiết lập append-only và lưu giữ của đích, và đổi theo chúng; thẻ Kho hiển thị nó ở chế độ chỉ đọc. Khi khóa mới của đích không mở được nó, kho trực tiếp giữ khóa nó đang có và lần lưu nói điều đó. Một mục trên kho trực tiếp được sao chép từ đó tới các đích sáng khác, không bao giờ tới đích mà kho thuộc về. Bản sao lưu của nó mang thẻ `bv:direct`, và mọi lượt lưu giữ khác để yên chúng, nên một kho trực tiếp mất liên kết với đích của nó không bao giờ bị già đi theo quy tắc cục bộ. B2 được truy cập qua điểm cuối S3 của nó, với ID khóa và khóa ứng dụng nhập làm thông tin xác thực S3; một khóa giới hạn ở thư mục riêng của đích không tới được thư mục bên cạnh, nên hãy giới hạn khóa ở thư mục bên trên.

### Ngoài cơ sở

Một đích sao lưu hoặc một kho có tên có thể được đánh dấu **Ngoài cơ sở**. Các đích từ xa bắt đầu với dấu này; hãy tắt nó cho một rest-server trong cùng tòa nhà. Dấu này chỉ ảnh hưởng đến việc đếm các nơi và 3-2-1 trên các thẻ. Nó không đổi bản sao nào.

Một bản nhân bản ZFS là một bản sao ngoài cơ sở, nhưng không phải một bản sao lưu có lịch sử. Nó chỉ giữ các snapshot bạn đặt, còn các phiên bản cũ và từng tệp vẫn đến từ các bản sao lưu. Nó đi từ một máy chủ ZFS thẳng sang máy kia, qua SSH tới một máy chủ ZFS hoặc tới điểm nhận của một phiên bản đã ghép nối. Nhóm ghép nối chỉ mang yêu cầu. Xem [Bản nhân bản](zfs-datasets.md#replica).

### Sau khi dựng lại {#after-a-rebuild}

Lựa chọn sao chép nằm trong cài đặt riêng của BombVault. Sau khi dựng lại qua **Tìm bản sao lưu** hoặc Trình hướng dẫn khẩn cấp mà không khôi phục cài đặt, chúng đã mất, và sao chép mọi thứ sẽ lại gửi lên B2 những mục bạn đã loại. Vì vậy bản sao off-site của mọi phần được dựng lại tạm dừng. Tổng quan nói điều đó, và thẻ **Đích theo phần** đưa ra **Xác nhận mặc định** cùng bản xem trước những gì lần chạy kế tiếp sẽ sao chép và các tên trong các bản sao lưu không có mục, mà bạn có thể loại ở đó. Chỉ việc xác nhận mới kết thúc tạm dừng; nhập một tệp cài đặt đưa quy tắc và mặc định trở lại nhưng không kết thúc nó.

## Kho chính từ xa {#remote-primary-repositories}

Đường dẫn riêng của một phần không bị giới hạn ở thư mục cục bộ: trỏ nó thẳng vào một restic remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) và BombVault sao lưu thẳng vào đó, không có bản sao cục bộ riêng và không có bước sao chép. Đây là một hình thức khác với các bản sao off-site ở trên: ở đó kho cục bộ là kho chính và kho off-site là bản lưu trữ hết sức có thể của nó; ở đây kho từ xa **chính là** kho chính, và là bản sao duy nhất trừ khi bạn cũng sao chép phần đó sang một đích sao lưu khác.

Đường dẫn của mỗi phần (Containers, VMs, Flash, Tự sao lưu, Thư mục, Tập dữ liệu ZFS) được đặt trên trang của đích sao lưu **Thư mục cục bộ**: **Đổi** cạnh phần trong thẻ **Kết nối** của nó mở một cửa sổ có công tắc **Cục bộ / Từ xa**:

- **Cục bộ** hiển thị trình duyệt thư mục quen thuộc.
- **Từ xa** thay nó bằng một trường URL thuần và các thiết lập kết nối mà một đích off-site dùng, được cấu hình cho kho chính này. Từ đó bạn có:
    - **Một lần kiểm tra kết nối** với đường dẫn thật, trước khi bạn trông cậy vào nó.
    - **Giới hạn băng thông** (tải lên/tải xuống) để một bản sao lưu theo lịch tới kho chính từ xa không làm nghẽn WAN của bạn: cùng các cờ restic `--limit-upload`/`--limit-download` mà bản sao off-site dùng, áp dụng cho chính bản sao lưu.
    - **Bảo vệ append-only**, được xác minh bằng cùng kiểm tra can thiệp chủ động (một lần thăm dò DELETE thật tới phía xa) mà các đích off-site nhận. Khi bật, BombVault từ chối tự dọn kho, vì không có bản sao cục bộ riêng phía sau nó và thông tin xác thực trên máy này không được phép xóa bản sao duy nhất của bản sao lưu.
    - **Cảnh báo ngân sách tăng trưởng**, lấy từ chính xu hướng kích thước kho mà thẻ Lưu trữ vốn đã theo dõi.

Không cái nào trong số này là bắt buộc: một đường dẫn từ xa gõ tay không có thiết lập an toàn đã lưu sao lưu không giới hạn, dọn được và không có cảnh báo ngân sách. Các thiết lập an toàn có đó khi bạn muốn những bảo vệ mà một bản sao off-site nhận được, mà không cần một đích sao lưu riêng chỉ để có chúng.

!!! note "Thông tin đăng nhập đám mây và REST dùng chung"
    Một kho chính từ xa xác thực bằng cùng thông tin xác thực đám mây dùng chung dưới **Cài đặt, Đích sao lưu**; không có kho thông tin xác thực riêng cho kho chính.

### SMB và WebDAV không cần gắn kết trên máy chủ {#smb-webdav}

Thẻ **Ngoại vi (rclone)** dưới **Cài đặt, Đích sao lưu** có một biểu mẫu cho share Windows hoặc Samba và cho máy chủ WebDAV (Nextcloud, ownCloud, SharePoint hay bất kỳ máy chủ nào khác). Điền một tên ngắn, host và share (SMB) hoặc URL và loại máy chủ (WebDAV), người dùng và mật khẩu, và BombVault viết phần rclone cho bạn. rclone tự làm rối mật khẩu trước khi lưu; thêm với một tên đã tồn tại sẽ thay phần đó chứ không thêm phần thứ hai. **Thêm đích** cung cấp SMB và WebDAV làm nhà cung cấp với cùng các trường.

Biểu mẫu trả lời bằng vị trí hoàn chỉnh, ví dụ `rclone:nas:backups`. Dùng nó làm đường dẫn của một phần hoặc một đích sao lưu và thêm thư mục con nếu muốn (`rclone:nas:backups/bombvault`). Share là đoạn đường dẫn đầu tiên, không phải một phần của tên.

Đây là cách tốt hơn gắn share vào Unraid: restic khuyên không giữ kho trên một share CIFS đã gắn, và ở đây không có gì được gắn. NFS không có trong biểu mẫu vì cả restic lẫn rclone đều không có backend NFS; với NFS, hãy gắn export trên máy chủ và dùng như thư mục cục bộ.

## Bảo vệ chống xóa (append-only)

Bật bảo vệ chống xóa cho một đích sao lưu off-site để ransomware, hoặc một máy chủ bị xâm nhập, không thể xóa hay ghi lại các bản sao lưu của bạn. Phía xa (một `restic/rest-server` chạy ở chế độ `--append-only`) **thực thi** nó. BombVault chỉ **xác minh** nó và không bao giờ hiện màu xanh chỉ vì một khẳng định cấu hình. Một thư mục trên máy chủ này không thể từ chối xóa, vì ai điều khiển BombVault cũng điều khiển thư mục, nên các đích cục bộ không có công tắc này.

Cách nhanh nhất để có một bản sao append-only là một BombVault đã ghép nối với một [máy chủ nhận](#receiving-server): nó chạy rest-server cho bạn, ở chế độ append-only.

!!! note "Xóa thành công dưới `/locks/` là hành vi mong đợi"
    Append-only không có nghĩa là không còn xóa được gì nữa. restic phải tự tạo và giải phóng khóa của nó, nên `/locks/` cố ý vẫn ghi và xóa được. Các snapshot và dữ liệu phía sau chúng, tức đúng thứ mà mã độc tống tiền nhắm tới, không thể bị xóa. Nếu bạn tự kiểm tra phía xa, một thao tác xóa thành công dưới `/locks/` là hành vi đúng chứ không phải lỗ hổng bảo vệ.

!!! warning "Kho append-only không bao giờ được dọn từ máy này"
    Một đích append-only không bao giờ dọn bản sao lưu cũ từ đây; dọn dẹp là việc của phía xa. Hãy đặt một **cảnh báo ngân sách tăng trưởng** cho nó để bạn được báo trước khi dung lượng kho vượt tầm kiểm soát.

## Kiểm tra can thiệp

BombVault định kỳ chứng minh bảo đảm append-only bằng cách thực sự thử xóa trên kho off-site, nhắm vào một đối tượng không tồn tại. **Kiểm tra bảo vệ chống xóa ngay** trên trang của một đích chạy nó theo yêu cầu, và **Kiểm tra bảo vệ chống xóa** dưới **Cài đặt, Xác minh, Kiểm tra tự động** chạy nó theo lịch.

- **Bị từ chối** nghĩa là được bảo vệ.
- **Được chấp nhận** nghĩa là không được bảo vệ.
- Một kết quả **không kết luận được** (máy chủ không tiếp cận được, lỗi xác thực) không bao giờ lật ngược phán quyết đã lưu.

Một lần lật thực sự từ được-bảo-vệ sang không-được-bảo-vệ sẽ kích một cảnh báo duy nhất.

## Diễn tập DR

BombVault cung cấp hai cấp độ bằng chứng rằng các bản sao lưu của bạn thực sự khôi phục được, không chỉ hiện diện.

- **Diễn tập xác minh khôi phục (cục bộ).** BombVault định kỳ chạy `restic check --read-data-subset` (có giới hạn, không bao giờ là một lần khôi phục đầy đủ làm đầy đĩa). Chu kỳ nằm dưới **Cài đặt, Xác minh, Kiểm tra tự động**, và thẻ **Kiểm tra bản sao lưu** trên cùng ô hiển thị kết quả.
- **Khôi phục thử (off-site).** BombVault khôi phục một đích thật từ kho off-site vào một sandbox niêm phong, xác minh từng tệp và từng byte, rồi dọn dẹp. Điều này chứng minh bạn khôi phục được từ off-site, chứ không chỉ là kho có phản hồi.

Thẻ **Trạng thái bảo vệ** trên Tổng quan gộp điều này theo từng phần thành bốn kiểm tra: một bản sao lưu hiện hành trên máy chủ này, một bản sao off-site, bảo vệ chống xóa ở đó và một lần khôi phục thử từ đó. Nhấp vào một phần hiển thị chi tiết, và thẻ chỉ xanh trên các sự thật đã xác minh.

## Ghép nối các phiên bản {#pairing}

Nhiều BombVault làm việc cùng nhau như một nhóm: chúng hiển thị trạng thái bảo vệ của nhau, đảm nhận vai trò cho nhau và có thể giữ khóa của nhau. Một phiên bản gia nhập nhóm bằng mười hai từ.

Trên phiên bản đầu tiên mở **Cài đặt, Ghép nối** và nhấn **Tạo cụm từ**. Ba thẻ bước cho thấy cách làm. Mười hai từ hiện trong một cửa sổ có nút **Sao chép**. Trên mọi phiên bản khác mở cùng chỗ đó, nhấn **Nhập cụm từ** và dán hoặc gõ chúng, hoặc nhấn **Dán** trong cửa sổ đó. Một từ không có trong danh sách được nêu tên cùng vị trí khi bạn gõ, và từ cuối mang một checksum, nên một từ gõ sai hay bị đổi chỗ bị bắt trước khi bất cứ gì được ghép nối. Chỉ tạo cụm từ trên một phiên bản: hai phiên bản cùng tạo sẽ thành hai nhóm riêng. Nếu một phút sau không ai xuất hiện, trang đưa ra hai lối thoát: hiện lại các từ để nhập chúng ở bên kia, hoặc nhập các từ của phiên bản kia và gia nhập nhóm của nó trong một bước. Ghép nối hoạt động không cần mật khẩu đăng nhập, nhưng hãy đặt một mật khẩu: không có nó, ai mở được giao diện web đều đọc được các từ và, qua nhóm, yêu cầu các vai trò trao mật khẩu restic của các phiên bản trong đó. Thẻ ghép nối nói điều đó cho đến khi đặt mật khẩu. Khi có mật khẩu, hiện lại cụm từ sẽ hỏi nó. **Rời nhóm** đưa một phiên bản ra ngoài trở lại.

Bất kỳ ai biết các từ đó đều có thể gia nhập nhóm, nên hãy coi chúng như một mật khẩu.

**Các thành viên tìm nhau thế nào.** Mỗi phiên bản biết địa chỉ mạng của chính nó từ trình duyệt của bạn vào lúc bạn đăng nhập, được liệt kê dưới **Địa chỉ đã biết** trên trang ghép nối cùng những địa chỉ phiên bản tự tìm thấy; hãy sửa nó ở đó nếu có reverse proxy hay một cổng bất thường phía trước. Trên cùng mạng, các thành viên thông báo địa chỉ đó bằng multicast và nói chuyện trực tiếp, và nơi multicast không đi qua được mạng container, như bridge mặc định của Docker, một phiên bản thay vào đó tìm các phiên bản khác trong subnet của mình bằng một lệnh gọi có chữ ký mà chỉ thành viên nhóm mới trả lời được, nên việc ghép nối vẫn xong trong vài giây mà không cần relay. Nếu không tìm thấy gì, **Không tìm thấy phiên bản kia?** nhận một địa chỉ nhập tay, cho subnet khác hoặc cổng không chuẩn. Các phiên bản ở mạng khác nhau đi qua một relay, chọn trên cùng trang; mỗi trong bốn dạng của nó kèm một hình:

- **Relay của dự án** (mặc định): `parleyport.halleluja.design`, cũng là relay mà KnightLoader dùng. Không cần thiết lập gì cả.
- **Relay riêng**: container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) từ Unraid Community Apps (cũng có dưới **Cài đặt, Ứng dụng**), hoặc một trong các phiên bản của bạn đã truy cập được từ bên ngoài với **Đóng vai trò relay** được bật. Phiên bản đó khi ấy trả lời tại `/relay/connect` trên địa chỉ riêng của nó, phía sau reverse proxy và chứng chỉ nó đã có, và chỉ cho nhóm của bạn vào. Nhập địa chỉ của relay trên mọi phiên bản cần dùng nó.
- **Không dùng relay**: các thành viên tự động tìm thấy nhau trong cùng một mạng, và không ở đâu khác.

**Những gì relay thấy.** Mọi lệnh gọi giữa các thành viên được niêm phong bằng AES-256-GCM dưới một khóa dẫn xuất từ mười hai từ, và khóa đó không bao giờ rời các phiên bản của bạn. Relay biết một hash nhóm các kết nối, thông điệp dành cho phiên bản nào, kích thước bao nhiêu và khi nào đi qua. Một lệnh gọi trực tiếp trên mạng cục bộ được niêm phong như vậy và còn được ký, nên không gì phụ thuộc vào chứng chỉ tự ký mà một phiên bản cung cấp. Các lệnh gọi trao ra thứ gì đó (câu trả lời vai trò, mật khẩu restic, đăng nhập bên nhận, bản gửi khóa) cũng được ký bằng khóa của thành viên gửi chúng, được ghim lần đầu thấy; xem [Mô hình bảo mật](configuration.md#security-model).

**Những gì đi qua nhóm.** Trạng thái bảo vệ trên trang Phiên bản, yêu cầu kiểm tra một phần ngay, yêu cầu vai trò và câu trả lời của chúng, đề nghị lưu trữ, và những gì bên nhận hoặc bên kéo cần khi vai trò của nó được cho phép: vị trí kho của phiên bản kia và mật khẩu restic của nó. Dữ liệu sao lưu thì không bao giờ; nó vẫn đi thẳng tới các backend restic. APP_KEY cũng không: mật khẩu restic mở các kho của phiên bản đó và không gì khác, không phải bí mật đã lưu, phiên hay mã mở bản gửi của nó.

**Các mục từ trước khi ghép nối.** Các phiên bản được thêm bằng mã thông báo fleet, cùng kho đã nhận và nguồn kéo được thiết lập bằng APP_KEY của phiên bản kia, vẫn còn sau bản cập nhật và được đánh dấu **Ghép nối lại**. Chúng vẫn hoạt động: ở lần khởi động đầu tiên BombVault thay mỗi APP_KEY đã lưu bằng mật khẩu restic dẫn xuất từ nó. Hãy ghép nối cả hai phiên bản, rồi sửa mục và chọn phiên bản của nó. Cho đến lúc đó các mục như vậy nằm trong thẻ **Không có phiên bản** trên trang Phiên bản.

Nơi duy nhất vẫn nhận APP_KEY nhập tay là [Từ thư mục sao lưu cũ](#restore-from-another-bombvault-repo), cho trường hợp phiên bản kia đã mất và không thể trả lời trong một nhóm.

## Phiên bản và vai trò {#roles}

Trang **Phiên bản** (một ô cài đặt có thể ghim vào thanh bên) hiển thị nhóm như một mạng lưới ai làm gì cho ai, một thẻ cho phiên bản này và một thẻ cho mọi thành viên khác, điện thoại Android và máy chủ ZFS. **Ghép nối** nổi ở dưới cùng bên phải. Phần nào bật được đặt dưới **Cài đặt, Phiên bản, Nhiều máy chủ**: **Phiên bản**, **Bộ nhận** và **Kéo về**.

Mỗi thẻ hiển thị trạng thái bảo vệ của thành viên và các vai trò của nó dưới dạng ô. Một vai trò là điều một phiên bản làm cho phiên bản khác:

- **Bộ nhận** giữ các bản sao off-site của máy chủ này trong một rest-server ở chế độ append-only. Máy chủ này gửi sau mỗi bản sao lưu và không xóa được gì ở đó, kể cả khi ai đó chiếm quyền nó.
- **Bên kéo** kéo các bản sao lưu của máy chủ này theo lịch riêng của nó. Máy chủ này không cần đang bật.
- **Máy chủ ZFS** nhận các bản nhân bản ZFS của máy chủ này. Mỗi mục ZFS chọn nó dưới **Bản nhân bản** trong cài đặt của mình.
- **Bên giữ khóa** giữ một bản niêm phong của bộ khôi phục của máy chủ này. Xem [Bên giữ khóa](#key-keeper).

Bật một vai trò gửi một yêu cầu, và không gì bắt đầu cho đến khi ai đó trên phiên bản kia nhấn **Cho phép**. Yêu cầu hiện trên thẻ của phiên bản đó như một huy hiệu, và **Chi tiết** của nó liệt kê nó cùng **Cho phép** và **Từ chối**; một vai trò đã cho phép có thể bị thu hồi ở đó bất cứ lúc nào. Một đường nét đứt trong mạng là một vai trò còn đang chờ. Một phiên bản chạy BombVault 9.x không trả lời được yêu cầu, nên vai trò ở đó có hiệu lực ngay khi bạn bật, và yêu cầu từ một phiên bản 9.x chờ **Cho phép** ở đây.

**Chi tiết** trên một thẻ mở phần còn lại: kho đã nhận và nguồn kéo của thành viên đó, đề nghị lưu trữ, giám sát, **Xóa** và **Quên khóa**.

## Bên nhận và bên kéo {#receiver}

![Một bản sao off-site đã nhận trong chi tiết của một phiên bản, được theo dõi chỉ đọc, với kiểm tra tính toàn vẹn chạy trên phần cứng này.](assets/screenshots/receiver.png)

*Một bản sao off-site đã nhận trong chi tiết của một phiên bản, được theo dõi chỉ đọc, với kiểm tra tính toàn vẹn chạy trên phần cứng này.*

Trên máy **nhận** các bản sao append-only từ một BombVault khác, các kho đã nhận cho bạn khả năng giám sát độc lập, chỉ đọc trên phần cứng nhận, nên một lỗi âm thầm ở đầu xa không bị bỏ sót. Bật **Bộ nhận** dưới **Cài đặt, Phiên bản**, rồi mở **Chi tiết** của phiên bản gửi trên trang Phiên bản và thêm kho đã nhận (chỉ đọc, mở bằng mật khẩu restic của phiên bản gửi, mà nó nhận qua [nhóm ghép nối](#pairing)). Bạn có:

- **Bản kiểm kê theo nguồn**, để thấy chính xác container, VM và bộ thư mục nào đã đến.
- **Nhận lần cuối** cho từng nguồn, để biết mỗi nguồn mới đến đâu.
- **Một `restic check` độc lập** chạy trên phần cứng bên nhận, nên tính toàn vẹn được xác minh ngay nơi dữ liệu thực sự nằm, không chỉ trên bên gửi.
- **Một công tắc người chết:** một cảnh báo khi một nguồn ngừng gửi trong một khoảng thời gian bạn đặt.
- **Cảnh báo toàn vẹn:** một cảnh báo khi một lần kiểm tra ở phía nhận thất bại.

Bên nhận hoàn toàn chỉ đọc. Nó không bao giờ ghi vào kho đã nhận, nên không bao giờ phá được bảo đảm append-only mà bên gửi dựa vào.

Bên kéo làm ngược lại: khi **Kéo về** được bật, một phiên bản đọc các bản sao lưu của thành viên khác vào kho của chính nó theo lịch riêng. Các nguồn của nó cũng nằm trong chi tiết của thành viên đó.

### Máy chủ nhận {#receiving-server}

Máy nhận cũng có thể chạy rest-server mà các máy khác sao chép tới. Nó được thiết lập lần đầu bạn cho phép một yêu cầu **Bộ nhận** đòi nó: cửa sổ nói máy chủ cần một máy chủ nhận và hỏi một thư mục trên một share, với **Thư mục mới** để tạo, và một cổng (8000 trừ khi container khác dùng). BombVault sau đó:

1. từ chối nếu đã có container tên `rest-server` hoặc container khác đang giữ cổng đó;
2. kéo `restic/rest-server` và khởi động nó qua socket Docker ở chế độ append-only, với kho riêng tư và một tệp đăng nhập trong thư mục;
3. ghi mẫu Unraid của nó vào ổ flash, để container vẫn chỉnh sửa được trong tab Docker, hoặc cho tải mẫu về khi không truy cập được ổ flash;
4. chạy kiểm tra can thiệp lên nó và cho biết nó có từ chối xóa hay không.

Các phiên bản trong nhóm của bạn khi đó thấy máy chủ trong số các nhà cung cấp dưới **Từ nhóm của bạn**, đặt theo tên máy nhận, và vai trò **Bộ nhận** đưa nó ra như lựa chọn an toàn. Mỗi phiên bản nhận một đăng nhập riêng lần đầu dùng máy chủ, và chỉ ghi vào thư mục riêng của nó ở đó. Với **Bộ nhận** được bật dưới **Cài đặt, Phiên bản**, **Máy chủ nhận** trên thẻ của phiên bản này mở lại máy chủ. Cửa sổ của nó liệt kê các đăng nhập dưới **Đối tác có thông tin đăng nhập**, và **Thu hồi thông tin đăng nhập** lấy một cái đi; những gì phiên bản đó đã sao chép vẫn nằm trong thư mục. Thiết lập cũng tạo một **Thông tin đăng nhập ngoài nhóm**, mà cửa sổ hiển thị mật khẩu một lần.

Một phiên bản chỉ tới được máy nhận qua relay thì không dùng được máy chủ, vì relay không mang bản sao lưu. Hãy thêm địa chỉ của máy nhận trên trang ghép nối trước. Khi BombVault chạy trên một địa chỉ IP riêng (ví dụ trên br0), hãy điền **Địa chỉ cho đối tác (tùy chọn)**, vì máy chủ lắng nghe trên địa chỉ của host.

## Ví dụ hoàn chỉnh: hai máy Unraid, từ đầu đến cuối

Mọi thứ ở trên mô tả các bộ phận. Đây là một thiết lập hoàn chỉnh, vì các bộ phận dễ ghép hơn khi bạn đã thấy chúng được ghép một lần.

Hai máy: **TOWER** chạy container và đẩy bản sao lưu; **VAULT** nhận chúng và thực thi append-only. Hãy thay bằng tên, địa chỉ và đường dẫn share của riêng bạn.

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

**5. Theo dõi những gì đến, nếu bạn muốn.** Bật **Bộ nhận** dưới **Cài đặt, Phiên bản** trên VAULT, mở **Chi tiết** của TOWER và thêm kho đã nhận. VAULT nhận mật khẩu restic của TOWER qua nhóm; không ai gõ khóa.

**6. Làm cho nó hai chiều.** Lặp lại các bước theo chiều ngược lại: VAULT nhờ TOWER làm bên nhận của nó. Mỗi máy khi đó thực thi append-only cho máy kia, và không máy nào xóa được bản sao lưu của máy kia.

Một rest-server bạn tự chạy cũng dùng được: thêm nó bằng **Thêm đích** như một rest-server, với người dùng và mật khẩu từ `.htpasswd` của nó, và bật bảo vệ chống xóa. Giữ `--append-only` trong tùy chọn của nó; thiếu nó thì phía xa lại là một share thông thường.

!!! warning "Đường dẫn cục bộ là đường dẫn bên trong container"
    Nơi BombVault hỏi một thư mục trên máy chủ này, hãy nhập nó tương đối với điểm gắn kết của host, chẳng hạn `user/appdata/rest-server/bombvault-containers`, không phải `/mnt/user/appdata/...`. BombVault chạy trong một container, nơi `/mnt` của host được gắn ở chỗ khác; một đường dẫn tuyệt đối của host không tồn tại bên trong. Nếu bạn dán một đường dẫn như vậy, BombVault cho biết đường dẫn tương đối nên dùng thay thế.

## Trình hướng dẫn khẩn cấp {#emergency}

**Trình hướng dẫn khẩn cấp** đưa trở lại một máy chủ đã mất hoặc được thiết lập lại. Nó bắt đầu trên trang **Khôi phục** dưới **Khẩn cấp**, và từ **Khôi phục từ bản sao lưu** trên trang chào của một bản cài mới. Nó chạy qua bốn bước: **Truy cập**, **Cài đặt**, **Lựa chọn** và **Khôi phục**.

1. **Truy cập.** Cho BombVault biết khóa của các bản sao lưu cũ, theo một trong ba cách:
    - **Tải lên bộ khôi phục**: tệp bạn đã tải xuống lúc thiết lập. Nó chứa khóa và mọi đích sao lưu, và BombVault điền phần còn lại từ đó. Một bộ khôi phục niêm phong bằng age hỏi danh tính mà nó được niêm phong cho; nó dùng một lần và không được lưu.
    - **Qua nhóm**: một phiên bản trong nhóm của bạn giữ khóa dưới dạng bản gửi. Bạn cần mười hai từ của nhóm, mã mở bản gửi và một người ở phiên bản đó. Xem [Bên giữ khóa](#key-keeper).
    - **Nhập khóa**: `APP_KEY` của máy chủ cũ, và nơi các bản sao lưu của nó nằm.

    BombVault sau đó thử khóa trên từng đích sao lưu và cho biết với mỗi đích là đọc được, không tìm thấy, bị khóa bằng khóa khác, không truy cập được hay vẫn thiếu đăng nhập. Chưa có gì được lưu, và khóa chỉ ở trong bộ nhớ một thời gian giới hạn. **Tiếp nhận khóa** làm máy chủ này từ giờ ghi bản sao lưu bằng khóa đó; **Hoàn tác việc tiếp nhận** quay về khóa riêng của nó cho đến khi bản sao lưu đầu tiên đã chạy. Một máy chủ đã có bản sao lưu riêng từ chối việc tiếp nhận và chỉ tới [Từ thư mục sao lưu cũ](#restore-from-another-bombvault-repo). **Lưu thông tin đăng nhập** lưu các đăng nhập mà bộ khôi phục mang cho các đích sao lưu từ xa, và hỏi trước khi thay các đăng nhập mà máy chủ này đã có.
2. **Cài đặt.** **Khôi phục cài đặt** khôi phục cài đặt riêng của BombVault từ Tự sao lưu mới nhất: đích sao lưu, thông tin xác thực, lịch trình và quy tắc. BombVault khởi động lại để áp dụng chúng, qua Docker socket, nên cơ sở dữ liệu cài đặt đang chạy không bao giờ bị ghi đè dưới một handle đang mở, và trình hướng dẫn tự tiếp tục sau đó. Đăng nhập của máy chủ này vẫn giữ: bạn tiếp tục đăng nhập bằng mật khẩu đang dùng. Những bí mật máy chủ cũ đã lưu mà không tiếp nhận được sẽ được liệt kê, để bạn nhập lại.
3. **Lựa chọn.** BombVault xem qua các bản sao lưu của bạn và liệt kê các container, VM, bộ thư mục và tập dữ liệu ZFS nó tìm thấy. Bản sao lưu mới nhất được chọn sẵn ở mọi nơi. Những gì máy chủ này còn có bắt đầu ở trạng thái tắt, vì một lần khôi phục sẽ thay thế chúng. Bộ thư mục sau khi dựng lại không còn biết chúng đến từ đâu, nên chúng đi vào một thư mục bạn chọn.
4. **Khôi phục.** Mọi thứ đã chọn quay lại trong một lượt, để dừng nếu bạn muốn tự khởi động từng container và VM, điều an toàn nhất sau một mất mát hoàn toàn. Sau đó tải xuống một bộ khôi phục mới: bộ cũ thuộc về bản cài trước.

!!! tip "Di chuyển theo kế hoạch so với thảm họa"
    Trình hướng dẫn khẩn cấp khôi phục cài đặt riêng của BombVault từ một bản sao lưu. Với một lần chuyển sang máy mới *có kế hoạch*, bạn có thể thay vào đó mang cấu hình sang trực tiếp bằng thẻ **Xuất / nhập cài đặt** (một tệp JSON di động). Xem [Cấu hình](configuration.md#portable-settings-export-and-import).

### Từ thư mục sao lưu cũ {#restore-from-another-bombvault-repo}

**Từ thư mục sao lưu cũ** dưới **Nguồn khác** trên trang Khôi phục mở kho của một phiên bản BombVault *khác* (một share đã gắn dưới `/mnt`, hoặc một địa chỉ từ xa) bằng bộ khôi phục của phiên bản đó hoặc `APP_KEY` nhập tay, trong một phiên chỉ đọc dùng một lần. Duyệt các container, VM, bộ thư mục và tập dữ liệu ZFS lưu ở đó, chọn một bản sao lưu và khôi phục nó, và đối tượng được khôi phục trở thành một container, VM hoặc bộ thư mục cục bộ bình thường. Không bao giờ có gì được ghi vào kho kia, và cài đặt sao lưu của riêng bạn không bị đụng tới (phiên sống trong bộ nhớ và tự hết hạn). Chuyển một container từ máy chủ A sang máy chủ B không có nghĩa là trỏ lại cài đặt của bạn rồi hoàn lại sau. Cửa sổ này dùng một lần: nó mở một phiên, khôi phục những gì bạn chọn và quên phiên bản kia. Nếu bạn muốn một sắp xếp lâu dài, trong đó máy này kéo bản sao lưu của phiên bản khác vào kho của chính mình theo lịch, đó là vai trò **Bên kéo** trên trang Phiên bản.

Một container có mạng không tồn tại trên máy chủ này, ví dụ mạng `br0` của Unraid trên một máy chủ Docker thông thường, sẽ hiện phần chọn mạng dưới hàng của nó. BombVault tạo container trên mạng bạn chọn, giữ nguyên các mạng khác. Địa chỉ IP cố định và địa chỉ MAC thuộc về mạng cũ nên bị bỏ, và mạng mới sẽ cấp chúng.

## Bên giữ khóa {#key-keeper}

Bên giữ khóa là một phiên bản trong nhóm của bạn giữ một bản niêm phong của bộ khôi phục của máy chủ này. Nó không tự mở được bản đó. Nếu máy chủ này bị mất, một bản cài mới gia nhập nhóm của bạn sẽ xin bản đó từ bên giữ khóa và mở nó bằng mã mở bản gửi của bạn.

1. Trên trang Phiên bản, bật **Bên giữ khóa** trên thẻ của phiên bản cần giữ, và chọn **Gửi khóa**. Việc gửi khóa cần mật khẩu đăng nhập.
2. Máy chủ này hiển thị **Mã mở bản gửi** một lần: hai mươi chữ cái và chữ số trong bốn nhóm năm. Hãy ghi lại và xác nhận bằng **Tôi đã ghi lại mã**. Máy chủ này không lưu mã, chỉ lưu những gì cần để niêm phong một bản gửi cho nó.
3. Bên giữ khóa giờ giữ bản gửi, và máy chủ này làm mới nó khi bộ khôi phục đổi. **Bản gửi khóa** trên thẻ của phiên bản này liệt kê **Bản gửi của máy chủ này** cùng từng bên giữ khóa và trạng thái của nó, và **Các bản gửi máy chủ này đang giữ** cho những bên khác. **Tạo mã mới** thay mã; mỗi bên giữ khóa chuyển sang mã mới khi nhận bản gửi mới.

Vào ngày đó bạn cần ba thứ: mười hai từ của nhóm, mã mở bản gửi, và quyền vào giao diện của bên giữ khóa, nơi ai đó phải nhấn **Cho phép**. Trong Trình hướng dẫn khẩn cấp chọn **Qua nhóm**, chọn bản gửi, xin nó và gõ mã mở bản gửi. Mã được dùng trên máy chủ mới và không gửi cho bên giữ khóa. Trên bên giữ khóa, yêu cầu hiện dưới **Bản gửi khóa** cùng ai xin, phiên bản đó đã từng được thấy ở đó chưa và đã xin bao nhiêu lần; **Cho phép** trao bản niêm phong, và việc cho phép cần mật khẩu đăng nhập. Bản cài mới có 30 phút để lấy nó sau khi Cho phép, một yêu cầu không ai trả lời hết hạn sau một ngày, và một phiên bản xin quá thường xuyên phải chờ một giờ.

Không có mã thì không ai mở được bản đó, kể cả bên giữ khóa. Nếu mất mã, hãy tạo mã mới khi máy chủ này còn chạy; nếu máy chủ đã mất, bộ khôi phục và APP_KEY nhập tay vẫn dùng được. Bên giữ khóa có thể xóa một bản gửi bất cứ lúc nào, và chủ sở hữu khi đó phải xin lại.

## Bộ khôi phục {#recovery-kit}

Đây là mảnh khiến việc khôi phục sau thảm họa trở nên khả thi ngay cả khi không có một BombVault đang chạy.

**Tải bộ khôi phục** trong thẻ **Cho trường hợp khẩn cấp** dưới **Cài đặt, Khôi phục** cho bạn **khóa**, **mật khẩu restic dẫn xuất**, mọi **đích sao lưu** cùng kho của từng phần, các đăng nhập mà một bản cài mới cần để tới chúng, và **các lệnh restic chính xác**, để bạn khôi phục thẳng bằng restic CLI trên bất kỳ máy nào. Nó cần mật khẩu đăng nhập. Bộ khôi phục in khóa cả khi mã hóa tắt, vì định nghĩa container và VM luôn được niêm phong bằng nó. Sau một lần tiếp nhận nó in khóa sao lưu được tiếp nhận và APP_KEY của máy chủ này, thứ mà bản cài mới chỉ cần để đưa cài đặt trở lại. Một thẻ **Lưu bộ khôi phục của bạn** trên Tổng quan nhắc bạn cho đến khi bạn đã cất nó.

!!! danger "Cất giữ bộ khôi phục ngoài máy chủ"
    Bộ khôi phục chứa bí mật giải mã các bản sao lưu của bạn. Hãy cất nó ở nơi an toàn tách biệt khỏi máy chủ (trình quản lý mật khẩu, một bản in trong két). Nếu bạn mất cả BombVault lẫn `APP_KEY` mà không có bộ khôi phục và không có bản gửi khóa, các bản sao lưu đã mã hóa của bạn không thể khôi phục được.

!!! warning "Snapshot mới nhất không phải lúc nào cũng là cái nên khôi phục"
    Từ restic 0.17, `restic snapshots` hiển thị dung lượng của mỗi snapshot. Sau khi mất dữ liệu, snapshot mới nhất có thể là snapshot đã bị làm trống, nên đừng khôi phục một snapshot nhỏ hơn nhiều so với các snapshot trước. Sau ransomware, nó có thể là snapshot đã bị mã hóa với dung lượng thông thường. Nếu BombVault vẫn chạy, hãy xem trang **Bất thường** của nó trước: nó nêu tên bản sao lưu tốt gần nhất, và cửa sổ khôi phục chọn sẵn bản đó. Một lần khôi phục không cần bất kỳ dữ liệu bất thường nào của BombVault, và việc tạm dừng lưu giữ chỉ giữ thêm snapshot.

### Niêm phong bộ khôi phục

Nếu bạn đã bật **Mã hóa bản xuất bằng age** dưới **Cài đặt, Hệ thống**, bộ khôi phục cũng được niêm phong bằng nó và tải xuống dưới dạng `bombvault-recovery-kit.md.age`. Nó được bọc ASCII chứ không phải nhị phân, nên vẫn là văn bản thuần: dán vào trình quản lý mật khẩu hay in ra hoạt động y như trước, chỉ là nội dung không đọc được nếu thiếu khóa của bạn.

!!! warning "Đừng cất khóa age bên trong bộ khôi phục"
    Bạn cần khóa **riêng** age để mở một bộ khôi phục đã niêm phong. Hãy cất nó ở nơi không phụ thuộc vào chính bộ khôi phục, nếu không bạn sẽ có hai thứ phải khôi phục thay vì một. Niêm phong đáng làm khi bộ khôi phục được cất ở nơi bạn không hoàn toàn kiểm soát (một trình quản lý mật khẩu dùng chung, ghi chú trên đám mây, một bản in ở văn phòng); một bộ khôi phục trong két sắt của chính bạn đã được két sắt bảo vệ.

    Khi bật mã hóa mà không cấu hình người nhận dùng được, việc tải xuống bị từ chối thẳng. BombVault không bao giờ quay về giao khóa chính dưới dạng văn bản rõ.

### Khi không có bộ khôi phục trong tay

Mật khẩu không được lưu ở đâu cả, nó được **tính** từ khóa của các bản sao lưu, nên bạn có thể tự tạo lại nó chỉ với khóa và một shell:

```sh
printf 'bombvault:restic-repo' \
```

Đó là HMAC-SHA256 trên chuỗi cố định `bombvault:restic-repo`, với khóa là các byte thô của `APP_KEY` thập lục phân, in ra dưới dạng 64 ký tự thập lục phân chữ thường. Cùng giá trị đó có trong bộ khôi phục, liệt kê là mật khẩu restic dẫn xuất; đây là cho ngày bộ khôi phục không ở gần bạn. Trên một máy chủ đã tiếp nhận bản sao lưu của máy khác, hãy dùng khóa sao lưu được tiếp nhận, tức APP_KEY cũ của máy chủ đó.

!!! warning "Với kho đã nhận, hãy dùng khóa của bản chạy GỬI"
    Một kho đến đây qua bản sao off-site được tạo bởi máy gửi nó, với khóa **của nó**. Dẫn xuất từ khóa của máy nhận tạo ra một mật khẩu mà restic sẽ từ chối, và điều đó đọc giống hệt một kho bị hỏng nhưng không phải vậy. Đây là lý do thông thường khiến `restic check` trên kho đã nhận cứ hỏi mật khẩu hết lần này đến lần khác.

Vì các định nghĩa khôi phục nằm **bên trong** mỗi kho (`<repo>/def`, `<repo>/vm-def`), một thư mục kho được sao chép hoàn toàn tự chứa, nên bộ khôi phục cộng với kho là tất cả những gì một lần khôi phục bare-metal cần.

## Lấy lại một bản kết xuất cơ sở dữ liệu {#database-dumps}

Một bản kết xuất cơ sở dữ liệu là một điểm khôi phục riêng trong kho containers, gắn thẻ `dbdump:<container>`, chứa một tệp duy nhất `/dbdump/<container>.sql`. BombVault liệt kê, tải xuống và nhập chúng trên tab **Bản sao lưu** của container; dưới đây là các bước tương tự chỉ với restic, cho ngày BombVault không còn đó.

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
