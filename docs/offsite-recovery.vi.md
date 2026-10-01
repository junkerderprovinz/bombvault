# Off-site & khôi phục

Các bản sao lưu cục bộ bảo vệ bạn khỏi một container bị mất hay một bản cập nhật tồi. Nhân bản off-site và một bộ khôi phục đã được kiểm thử bảo vệ bạn khỏi mất cả cái máy, ransomware, hoặc một trận hỏa hoạn. Trang này bao quát việc nhân bản off-site, làm cho bản sao đó chống can thiệp, chứng minh rằng bạn có thể khôi phục, và khôi phục khi chính BombVault không còn.

## Nhân bản off-site

Giữ bản sao lưu cục bộ nhanh và thêm một hoặc nhiều bản sao off-site. Đặt một kho cho mỗi miền trên trang **Cài đặt, Ngoài site**. BombVault nhân bản các snapshot mới tới đó bằng `restic copy` theo kiểu nỗ lực tối đa, nên một trục trặc off-site không bao giờ làm thất bại bản sao lưu cục bộ. Ở dạng này kho cục bộ vẫn là chính và kho off-site là một bản sao, nhưng kho chính của một miền hoàn toàn không cần phải là cục bộ; xem [Kho chính từ xa](#remote-primary-repositories) bên dưới để sao lưu thẳng tới S3/rest-server/v.v. thay vì nhân bản tới đó.

- **Nhiều đích off-site cho mỗi miền.** Mỗi miền (container, VM, flash, config, bộ tập tin và tập dữ liệu ZFS) có thể nhân bản tới nhiều đích off-site cùng lúc, không chỉ một, nên bạn có thể giữ, ví dụ, một rest-server trên máy của một người bạn và một S3 bucket song song. Thêm các đích bổ sung trên Cài đặt, Ngoài site, mỗi đích có kho lưu trữ riêng, lớp lưu trữ S3, cờ append-only, lưu giữ và ngân sách tăng trưởng riêng. Một thiết lập off-site đơn hiện có được chuyển sang nguyên vẹn làm đích đầu tiên, và mọi đích của một miền đều nhân bản theo lịch trình off-site của miền đó.
- **Lịch trình off-site theo từng miền** (được chỉnh cùng với mọi lịch trình khác trên Cài đặt, Lịch trình): để trống để nhân bản sau mỗi lần sao lưu cục bộ, hoặc đặt một nhịp độ (ví dụ `weekly Sun 03:00`) để gửi off-site ít thường xuyên hơn tần suất bạn sao lưu cục bộ. Một nút **Sao chép ngay** lo các lần chạy theo yêu cầu.
- **Lưu giữ off-site** nằm trên Cài đặt, Lưu giữ để bạn có thể giữ các bản sao off-site lâu hơn như một kho lưu trữ. Để chính sách tất cả bằng 0 để không bao giờ tự động dọn bớt các snapshot off-site.
- **Giới hạn băng thông** (Cài đặt, Ngoài site) giới hạn tốc độ tải lên/tải xuống của restic để việc nhân bản không làm bão hòa WAN của bạn.
- Một **chỉ báo nhân bản** hiển thị miền nào đang nhân bản trong khi nó chạy (trên trang của nó và bảng điều khiển). Đó là một chỉ báo hoạt động, không phải một thanh phần trăm, vì `restic copy` không phơi bày tiến độ đọc được bằng máy.

!!! note "Khôi phục thẳng từ off-site"
    Mọi trình duyệt sao lưu đều có công tắc **Local / Off-site**, nên nếu một kho cục bộ bị mất hay hỏng, bạn có thể liệt kê và khôi phục trực tiếp từ bản sao off-site. Việc xóa là theo từng nguồn: xóa một bản sao lưu chỉ ảnh hưởng đến bản sao bạn đang xem.

## Kho chính từ xa {#remote-primary-repositories}

Đường dẫn sao lưu của một miền (Cài đặt, Lưu trữ) không giới hạn ở thư mục cục bộ: trỏ thẳng nó tới một kho restic từ xa (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:nguoidung@host:/repo`, `rclone:remote:bucket/duong-dan`) và BombVault sao lưu thẳng tới đó, không cần bản sao cục bộ riêng và không có bước nhân bản. Đây thực sự là một hình thái khác với nhân bản ngoại vi ở trên: ở đó kho cục bộ là kho chính còn kho ngoại vi là bản lưu trữ của nó trong khả năng có thể; ở đây kho từ xa **chính là** kho chính, và là bản duy nhất chừng nào bạn chưa cấu hình thêm nhân bản ngoại vi (hoặc một kho từ xa thứ hai) cho miền đó.

Mỗi trong sáu ô đường dẫn (Container, Máy ảo, Flash, Cấu hình, Tệp, Tập dữ liệu ZFS) đều có ngay bên cạnh một công tắc **Cục bộ / Từ xa**:

- **Cục bộ** hiển thị trình duyệt thư mục quen thuộc.
- **Từ xa** đổi nó thành một ô URL đơn giản, kèm một nút mở đúng hộp thoại kiểm tra kết nối và thông tin đăng nhập mà các đích ngoại vi vẫn dùng, chỉ khác là được cấu hình cho kho chính này. Từ đó bạn có:
    - **Một lần kiểm tra kết nối** với đường dẫn thật, trước khi bạn trông cậy vào nó.
    - **Giới hạn băng thông** (tải lên và tải xuống) để một bản sao lưu theo lịch tới kho chính từ xa không làm nghẽn đường WAN của bạn: đúng những tùy chọn restic `--limit-upload` và `--limit-download` mà nhân bản ngoại vi dùng, nay áp lên chính việc sao lưu.
    - **Bảo vệ chỉ-ghi-thêm (bất biến)**, được xác minh bằng đúng bài kiểm tra can thiệp chủ động (một phép thử DELETE thật tới đầu bên kia) mà các đích ngoại vi nhận được. Khi bật, BombVault từ chối tự cắt tỉa kho: vì phía sau không có bản sao cục bộ riêng, thông tin đăng nhập trên máy này không được phép xóa bản sao lưu duy nhất.
    - **Cảnh báo ngân sách tăng trưởng**, lấy từ chính xu hướng kích thước kho mà thẻ Lưu trữ vốn đã theo dõi.

Không điều nào trong số này là bắt buộc: một đường dẫn từ xa gõ tay, không lưu thiết lập an toàn nào, vẫn sao lưu y như trước (băng thông không giới hạn, cắt tỉa được, không cảnh báo ngân sách). Hộp thoại an toàn có ở đó cho lúc bạn muốn đúng những lớp bảo vệ mà một bản sao ngoại vi nhận được, mà không phải tạo riêng một đích ngoại vi chỉ để có chúng.

!!! note "Thông tin đăng nhập đám mây và REST dùng chung"
    Kho chính từ xa xác thực bằng đúng thông tin đăng nhập S3/REST đã cấu hình ở Cài đặt, Truy cập đám mây, Thông tin đăng nhập đám mây dùng chung. Không có kho thông tin đăng nhập riêng cho các kho chính.

### SMB và WebDAV không cần gắn kết trên máy chủ {#smb-webdav}

Cài đặt, Truy cập đám mây, rclone có một biểu mẫu cho share Windows hoặc Samba và cho máy chủ WebDAV (Nextcloud, ownCloud, SharePoint hoặc bất kỳ loại nào khác). Điền một tên ngắn, máy chủ và share (SMB) hoặc URL và loại máy chủ (WebDAV), người dùng và mật khẩu, rồi BombVault sẽ viết phần cấu hình rclone cho bạn. rclone tự làm rối mật khẩu trước khi nó được lưu; thêm một đích có tên đã tồn tại sẽ thay thế phần đó thay vì thêm một phần thứ hai.

Biểu mẫu trả về vị trí hoàn chỉnh, ví dụ `rclone:nas:backups`. Đặt nó vào một Backup Path hoặc một đích off-site và thêm một thư mục con nếu muốn (`rclone:nas:backups/bombvault`). Share là đoạn đường dẫn đầu tiên, không phải một phần của tên.

Đây là cách tốt hơn so với gắn kết share trên Unraid: restic khuyên không nên đặt kho trên một share CIFS được gắn kết, và ở đây không có gì được gắn kết. NFS không có trong biểu mẫu vì cả restic lẫn rclone đều không có backend NFS; với NFS, hãy gắn kết export trên máy chủ và trỏ một Backup Path tới đó.

## Off-site bất biến (append-only)

Đánh dấu một kho off-site là append-only để ransomware, hoặc một máy chủ bị xâm nhập, không thể xóa hay ghi lại các bản sao lưu của bạn. Phía bên kia (một `restic/rest-server` chạy ở chế độ `--append-only`) **thực thi** điều đó. BombVault chỉ luôn **xác minh** nó và không bao giờ hiển thị xanh chỉ dựa trên một tuyên bố cấu hình.

Trình hướng dẫn **thiết lập off-site có hướng dẫn** dẫn bạn từ lựa chọn backend (rest-server / rclone / S3) qua một đoạn triển khai rest-server sẵn sàng để dán, một lần kiểm tra kết nối, công tắc bất biến (chạy ngay lập tức bài kiểm tra can thiệp) và một chiến lược lưu giữ, nên off-site append-only là điều có thể đạt được mà không cần chỉnh sửa cấu hình bằng tay.

!!! note "Xóa thành công dưới `/locks/` là hành vi mong đợi"
    Append-only không có nghĩa là không còn xóa được gì nữa. restic phải tự tạo và giải phóng khóa của nó, nên `/locks/` cố ý vẫn ghi và xóa được. Các snapshot và dữ liệu phía sau chúng, tức đúng thứ mà mã độc tống tiền nhắm tới, không thể bị xóa. Nếu bạn tự kiểm tra phía xa, một thao tác xóa thành công dưới `/locks/` là hành vi đúng chứ không phải lỗ hổng bảo vệ.

!!! warning "Các kho bất biến không bao giờ được dọn bớt từ máy này"
    Một off-site bất biến cố ý không bao giờ dọn bớt các snapshot cũ. Đặt một **cảnh báo ngân sách tăng trưởng** cho nó để bạn được cảnh báo trước khi kích thước kho vượt tầm kiểm soát.

## Kiểm tra can thiệp

BombVault định kỳ chứng minh bảo đảm append-only bằng cách thực sự thử một thao tác xóa nhắm vào kho off-site, nhắm vào một đối tượng không tồn tại:

- **Bị từ chối** nghĩa là được bảo vệ.
- **Được chấp nhận** nghĩa là không được bảo vệ.
- Một kết quả **không kết luận được** (máy chủ không tiếp cận được, lỗi xác thực) không bao giờ lật ngược phán quyết đã lưu.

Một lần lật thực sự từ được-bảo-vệ sang không-được-bảo-vệ sẽ kích một cảnh báo duy nhất.

## Diễn tập DR

BombVault cung cấp hai cấp độ bằng chứng rằng các bản sao lưu của bạn thực sự khôi phục được, không chỉ hiện diện.

- **Diễn tập xác minh khôi phục (cục bộ).** BombVault định kỳ chạy `restic check --read-data-subset` (có giới hạn, không bao giờ là một lần khôi phục toàn bộ làm đầy đĩa) và hiển thị một huy hiệu *xác minh khôi phục được lần cuối* cho mỗi miền. Nhịp độ nằm trên Cài đặt, Lịch trình; huy hiệu trên Cài đặt, Toàn vẹn.
- **Diễn tập DR (off-site).** BombVault khôi phục một đích thực từ kho off-site vào một hộp cát dùng một lần, xác minh nó từng tập tin và từng byte, rồi dọn dẹp. Điều này chứng minh bạn có thể khôi phục từ off-site, không chỉ là kho phản hồi.

**Bảng điểm bảo vệ chống ransomware** trên bảng điều khiển gom điều này thành một thế phòng thủ xanh / hổ phách / đỏ cho mỗi miền, với một danh sách kiểm tra có đóng dấu tuổi (off-site đã cấu hình, append-only đã xác minh, nhân bản hiện thời, diễn tập khôi phục đã qua, mã hóa đã bật, chiến lược dọn bớt đã đặt). Mỗi hàng đỏ liên kết sâu tới bản sửa, và thẻ chỉ bao giờ chuyển xanh dựa trên các sự thật đã xác minh.

## Ghép nối các phiên bản {#pairing}

Bộ nhận, nguồn Kéo về, trang Phiên bản và Mesh off-site đều nói chuyện với một BombVault khác. Chúng làm vậy với tư cách thành viên của một nhóm ghép nối, và một phiên bản gia nhập nhóm bằng mười hai từ.

Trên phiên bản đầu tiên, mở **Cài đặt → Ghép nối** rồi nhấn **Tạo cụm từ** trong các thẻ ghép nối. Mười hai từ hiện ra trong một cửa sổ có nút **Sao chép**. Trên mọi phiên bản khác, mở cùng chỗ đó, nhấn **Nhập cụm từ** rồi dán hoặc gõ các từ vào, hoặc nhấn **Dán** trong cửa sổ đó. Một từ không có trong danh sách sẽ được nêu tên cùng vị trí của nó ngay khi bạn gõ, và từ cuối cùng mang một checksum, nên một từ gõ sai hay bị đảo chỗ sẽ bị phát hiện trước khi việc ghép nối xảy ra. Chỉ tạo cụm từ trên một phiên bản duy nhất: hai phiên bản mà cả hai đều tạo cụm từ sẽ tạo thành hai nhóm riêng biệt. Nếu không ai xuất hiện trong một phút, tab sẽ đưa ra hai cách thoát: hiện lại các từ để nhập chúng ở phía bên kia, hoặc nhập các từ của phiên bản kia và tham gia nhóm của nó trong một bước. Việc ghép nối vẫn hoạt động khi không có mật khẩu đăng nhập, nhưng hãy đặt một mật khẩu: nếu không, bất kỳ ai mở được giao diện web này đều có thể đọc các từ đó và, thông qua nhóm, lấy được mật khẩu restic của mọi phiên bản trong nhóm. Thẻ ghép nối sẽ nhắc điều này cho đến khi mật khẩu được đặt. Khi đã có mật khẩu, việc hiện lại cụm từ sẽ hỏi mật khẩu đó. **Rời nhóm** đưa một phiên bản ra khỏi nhóm trở lại.

Bất kỳ ai biết các từ đó đều có thể gia nhập nhóm, nên hãy coi chúng như một mật khẩu.

**Các thành viên đến được với nhau như thế nào.** Ngay khi bạn đăng nhập, mỗi phiên bản sẽ nhận biết địa chỉ mạng của chính nó từ trình duyệt của bạn, được hiển thị trên thẻ relay là **Phiên bản này trên mạng của bạn**; hãy sửa lại ở đó nếu có một reverse proxy hay một cổng khác thường đứng phía trước. Trong cùng một mạng, các thành viên thông báo địa chỉ đó bằng multicast và nói chuyện trực tiếp với nhau, còn ở nơi multicast không thể vượt qua một mạng container, chẳng hạn mạng bridge mặc định của Docker, một phiên bản sẽ quét mạng con của chính nó để tìm những phiên bản còn lại bằng một lệnh gọi có chữ ký mà chỉ thành viên trong nhóm mới trả lời được, nhờ đó việc ghép nối vẫn hoàn tất trong vài giây mà không cần relay. Nếu không tìm thấy gì, **Không tìm thấy?** bên dưới thẻ ghép nối sẽ nhận một địa chỉ nhập tay, dùng cho mạng con khác hoặc cổng không chuẩn. Các phiên bản ở mạng khác nhau đi qua một relay, được chọn trên cùng tab đó:

- **Relay của dự án** (mặc định): `relay.halleluja.design`, cũng là relay mà KnightLoader dùng. Không cần thiết lập gì cả.
- **Relay riêng**: container **BombVault Relay** từ Unraid Community Apps, hoặc một trong các phiên bản của bạn vốn đã truy cập được từ bên ngoài với **Làm relay** được bật. Phiên bản đó khi ấy sẽ trả lời tại `/relay/connect` trên địa chỉ của chính nó, phía sau reverse proxy và chứng chỉ mà nó đã có sẵn, và chỉ cho nhóm của bạn vào. Nhập địa chỉ của relay trên mọi phiên bản cần dùng nó.
- **Không dùng relay**: các thành viên tự động tìm thấy nhau trong cùng một mạng, và không ở đâu khác.

**Relay thấy được gì.** Mọi cuộc gọi giữa các thành viên được niêm phong bằng AES-256-GCM dưới một khóa suy ra từ mười hai từ, và khóa đó không bao giờ rời khỏi các phiên bản của bạn. Relay chỉ biết một hash gom nhóm các kết nối, phiên bản nào là đích của một thông điệp, nó lớn bao nhiêu và khi nào nó đi qua. Một cuộc gọi trực tiếp trên mạng cục bộ cũng được niêm phong theo cùng cách và còn được ký nữa, nên không có gì phụ thuộc vào chứng chỉ tự ký mà một phiên bản đang dùng.

**Những gì đi qua nhóm.** Các bảng điểm trên trang Phiên bản, một yêu cầu kiểm tra một miền ngay bây giờ, các đề nghị lưu trữ ngoài site của Mesh, và những gì một bộ nhận hay nguồn Kéo về cần: vị trí kho của phiên bản kia và mật khẩu restic của nó. Dữ liệu sao lưu thì không bao giờ; nó vẫn luôn đi thẳng tới các restic backend. APP_KEY cũng không: mật khẩu restic chỉ mở kho của phiên bản đó và không gì khác, không phải các bí mật đã lưu, phiên làm việc hay mã khôi phục của nó.

**Các mục có từ trước khi ghép nối.** Các phiên bản được thêm bằng mã thông báo fleet, cùng các bộ nhận và nguồn Kéo về được thiết lập bằng APP_KEY của phiên bản kia, vẫn còn sau bản cập nhật và được đánh dấu **Ghép nối lại**. Bộ nhận và nguồn Kéo về vẫn tiếp tục hoạt động: ở lần khởi động đầu tiên, BombVault thay mỗi APP_KEY đã lưu bằng mật khẩu restic suy ra từ nó. Hãy ghép nối cả hai phiên bản, rồi sửa mục đó và chọn phiên bản của nó. Một phiên bản như vậy sẽ nhận lại thẻ cũ của nó ngay khi một phiên bản cùng tên xuất hiện trong nhóm.

Nơi duy nhất vẫn còn nhận APP_KEY nhập bằng tay là [Khôi phục từ một kho BombVault khác](#restore-from-another-bombvault-repo), cho trường hợp phiên bản kia đã biến mất và không thể trả lời trong một nhóm.

## Bảng điều khiển bên nhận (phía nhận)

![Phía nhận, được theo dõi ở chế độ chỉ đọc, với kiểm tra toàn vẹn chạy trên máy này.](assets/screenshots/receiver.png)

*Phía nhận, được theo dõi ở chế độ chỉ đọc, với kiểm tra toàn vẹn chạy trên máy này.*

Mọi thứ ở trên là phía *gửi*. Trên máy **nhận** các bản sao off-site bất biến từ một BombVault khác, bảng điều khiển bên nhận cho bạn giám sát độc lập, chỉ đọc các kho đó trên phần cứng bên nhận, nên một lần thất bại âm thầm ở đầu xa không bị bỏ qua.

Bật công tắc **Bộ nhận** trong Cài đặt để hé lộ một tab **Bộ nhận**. Nó mặc định tắt; chỉ bật nó trên một máy thực sự nhận các bản sao lưu off-site bất biến. Sau đó đăng ký một kho đã nhận (chỉ đọc, mở bằng mật khẩu restic của phiên bản gửi, đến qua [nhóm ghép nối](#pairing)) để có được:

- **Một kho snapshot được gom theo nguồn**, nên bạn có thể thấy chính xác những container, VM và bộ tập tin nào đã đến.
- **Nhận lần cuối** cho mỗi nguồn, nên bạn biết mỗi cái mới đến mức nào.
- **Một `restic check` độc lập** chạy trên phần cứng bên nhận, nên tính toàn vẹn được xác minh ngay nơi dữ liệu thực sự nằm, không chỉ trên bên gửi.
- **Một công tắc người chết:** một cảnh báo khi một nguồn ngừng gửi trong một khoảng thời gian bạn đặt.
- **Cảnh báo toàn vẹn:** một cảnh báo khi một lần kiểm tra ở phía nhận thất bại.

Bên nhận nghiêm ngặt chỉ đọc. Nó không bao giờ ghi vào kho đã nhận, nên nó không bao giờ có thể phá vỡ bảo đảm append-only mà bên gửi dựa vào.

## Ví dụ hoàn chỉnh: hai máy Unraid, từ đầu đến cuối

Phần trên mô tả các bộ phận. Đây là một thiết lập hoàn chỉnh với giá trị thật, vì các bộ phận dễ lắp hơn nhiều khi ta đã thấy chúng lắp xong một lần.

Hai máy: **TOWER** chạy các container và gửi bản sao lưu, **VAULT** nhận chúng và cưỡng chế tính bất biến. Hãy thay bằng tên, địa chỉ và đường dẫn chia sẻ của bạn.

**1. Trên VAULT, dựng máy chủ chỉ-ghi-thêm.** Trong BombVault trên TOWER, vào *Cài đặt → Ngoài site → thiết lập có hướng dẫn*, chọn **rest-server** và tạo công thức. Sao chép thẻ **Mẫu Unraid (XML)**, lưu trên VAULT thành `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, rồi *Docker → Add Container* và chọn **rest-server** trong danh sách mẫu. Trước khi khởi động, ghi dòng `htpasswd` hiển thị vào `/mnt/user/appdata/rest-server/.htpasswd` trên VAULT. Mật khẩu dùng một lần chỉ hiện một lần và không bao giờ được lưu, hãy sao chép ngay. Dòng đó mang chính mật khẩu ấy, đã được băm bằng bcrypt sẵn cho bạn: văn bản rõ đi vào thông tin đăng nhập REST trên TOWER, dòng đã băm đi vào `.htpasswd` trên VAULT. Bạn không phải tự băm gì cả.

    Giữ nguyên `--append-only` trong ô OPTIONS. Đó chính là điểm mấu chốt: thiếu nó, VAULT lại chỉ là một thư mục chia sẻ thông thường.

**2. Trên TOWER, trỏ kho ngoại vi tới đó.** Địa chỉ kho theo đúng mẫu mà công thức in ra:

    rest:http://VAULT:8000/bombvault-containers/containers

Đoạn đầu của đường dẫn là người dùng htpasswd, đoạn thứ hai là kho. Nhập người dùng và mật khẩu đã tạo làm thông tin đăng nhập REST của đích, rồi chạy **kiểm tra kết nối**.

**3. Trên TOWER, bật „Bất biến”.** Kiểm tra can thiệp chạy ngay và phải báo *được bảo vệ*. Ý nghĩa các kết quả:

| Kết quả | Điều đã xảy ra |
| --- | --- |
| **được bảo vệ** | VAULT đã từ chối lệnh xóa. Đây là trạng thái đạt duy nhất. |
| **KHÔNG được bảo vệ** | VAULT đã chấp nhận một lệnh xóa. Thiếu `--append-only` hoặc nó đã bị bỏ đi. |
| **không kết luận được** | Không thuộc trường hợp nào. Thường là địa chỉ không phải địa chỉ mà chính restic dùng, hoặc thông tin đăng nhập đã đổi. Không có gì được ghi lại và không có cảnh báo nào. |

**4. Trên VAULT, xem những gì tới nơi.** Ghép nối hai máy ([Ghép nối các phiên bản](#pairing)), bật *Cài đặt → Ghép nối → Bộ nhận*, mở thẻ **Bộ nhận** và đăng ký kho ở chế độ chỉ đọc với TOWER là phiên bản gửi.

!!! warning "Vị trí là đường dẫn **bên trong** container, viết tương đối so với điểm gắn của máy chủ"
    Nhập `user/appdata/rest-server/bombvault-containers/containers`, **không phải** `/mnt/user/appdata/…`. BombVault chạy trong container, nơi `/mnt` của máy chủ được gắn ở chỗ khác; đường dẫn tuyệt đối của máy chủ không tồn tại bên trong. Nếu bạn dán vào, BombVault nay sẽ cho biết đường dẫn tương đối cần dùng.

    VAULT nhận mật khẩu restic của TOWER qua nhóm khi bạn lưu; không ai phải gõ khóa nào cả.

**5. Nếu muốn, hãy làm hai chiều.** Lặp lại đúng năm bước theo chiều ngược lại: một rest-server trên TOWER nhận bản sao của VAULT. Khi đó mỗi máy cưỡng chế tính bất biến cho máy kia, và không máy nào xóa được bản sao lưu của máy kia.

## Khôi phục có hướng dẫn

Một tab **Recovery** chuyên biệt dẫn một bản cài đặt mới hoặc được dựng lại đi qua tình huống thảm họa, ở một nơi:

1. **Khôi phục cài đặt của chính BombVault trước**, nên các đường dẫn sao lưu, đích off-site và thông tin đăng nhập mà phần còn lại của quy trình cần được điền sẵn (áp dụng qua một lần tự khởi động lại thông qua Docker socket, nên cơ sở dữ liệu cài đặt đang chạy không bao giờ bị ghi đè dưới một handle đang mở).
2. **Kiểm tra BombVault có thể đọc các bản sao lưu của bạn** (điểm mắc kẹt về khóa mã hóa ngay từ đầu).
3. Cho bạn **trỏ tới kho hiện có của bạn** (cục bộ hoặc off-site).
4. **Khám phá** các container, VM, bộ tập tin và tập dữ liệu ZFS được lưu trong đó.
5. **Khôi phục các container và VM cùng một lúc** (để nguyên trạng thái dừng, nên bạn khởi động chúng một cách có chủ đích) và liệt kê các bộ tập tin và mục ZFS để khôi phục từng cái một; các mục ZFS trở lại ở trạng thái tắt. Bộ khôi phục của bạn chỉ cách một cú nhấp.

!!! tip "Di chuyển theo kế hoạch so với thảm họa"
    Khôi phục có hướng dẫn khôi phục cài đặt của chính BombVault từ một bản sao lưu. Với một lần chuyển *theo kế hoạch* sang một máy mới, thay vào đó bạn có thể mang cấu hình của mình theo trực tiếp bằng thẻ **Xuất / nhập cài đặt** (một tệp JSON di động). Xem [Cấu hình](configuration.md#portable-settings-export-and-import).

### Khôi phục từ một kho BombVault khác {#restore-from-another-bombvault-repo}

Một thẻ riêng trên tab **Recovery** mở kho của một phiên bản BombVault *khác* (một share được gắn kết dưới `/mnt`, hoặc một URL từ xa) bằng **`APP_KEY` của phiên bản đó**, trong một phiên chỉ đọc, dùng một lần. Duyệt các container, VM và bộ tập tin được lưu ở đó, chọn một snapshot và khôi phục nó, và đối tượng đã khôi phục trở thành một container, VM hay bộ tập tin cục bộ bình thường. Không có gì bao giờ được ghi vào kho kia, và các cài đặt sao lưu của chính bạn giữ nguyên không bị đụng (phiên sống trong bộ nhớ và tự hết hạn). Chuyển một container từ máy chủ A sang máy chủ B không còn có nghĩa là trỏ lại cài đặt kho của bạn rồi hoàn nguyên chúng sau đó. Thẻ này chỉ dùng một lần: nó mở một phiên, khôi phục những gì bạn chọn, rồi quên phiên bản kia. Nếu bạn muốn một sắp xếp lâu dài thay vào đó, trong đó máy này lấy các snapshot của một phiên bản khác về kho của chính nó theo lịch, thì đó là tab **Kéo về** của trang **Phiên bản**.

## Bộ khôi phục khóa mã hóa

Đây là mảnh khiến việc khôi phục sau thảm họa trở nên khả thi ngay cả khi không có một BombVault đang chạy.

Một cú nhấp tải xuống **khóa chính**, **mật khẩu restic dẫn xuất**, và **các vị trí kho cùng lệnh chính xác**, nên bạn có thể khôi phục thẳng bằng restic CLI trên bất kỳ máy nào. Một lời nhắc trên bảng điều khiển sẽ nhắc nhở cho đến khi bạn đã cất giữ nó.

!!! danger "Cất giữ bộ khôi phục ngoài máy chủ"
    Bộ khôi phục chứa bí mật giải mã các bản sao lưu của bạn. Giữ nó ở nơi an toàn và tách biệt khỏi máy chủ (một trình quản lý mật khẩu, một bản in trong két sắt). Nếu bạn mất cả BombVault và `APP_KEY` mà không có bộ khôi phục, các bản sao lưu đã mã hóa của bạn không thể khôi phục được.

!!! warning "Snapshot mới nhất không phải lúc nào cũng là cái nên khôi phục"
    Từ restic 0.17, `restic snapshots` hiển thị kích thước của mỗi snapshot. Sau khi mất dữ liệu, snapshot mới nhất có thể là cái đã bị làm trống, vì vậy đừng khôi phục một snapshot nhỏ hơn nhiều so với các snapshot trước nó. Sau ransomware, đó có thể là cái đã bị mã hóa với kích thước bình thường. Nếu BombVault vẫn chạy, hãy xem trang **Bất thường** trước: trang này nêu bản sao lưu tốt cuối cùng. Việc khôi phục không cần dữ liệu bất thường nào của BombVault, và việc tạm dừng lưu giữ chỉ giữ lại nhiều snapshot hơn.

### Niêm phong bộ khôi phục

Nếu bạn đã bật mã hóa age cho các bản xuất thô (Cài đặt), bộ khôi phục cũng được niêm phong bằng nó và được tải xuống dưới dạng `bombvault-recovery-kit.md.age`. Nó ở dạng ASCII-armored chứ không phải nhị phân, nên vẫn là văn bản thô: dán vào trình quản lý mật khẩu hay in ra vẫn hoạt động y như trước, chỉ là nội dung không đọc được nếu không có khóa của bạn.

!!! warning "Đừng cất khóa age bên trong bộ khôi phục"
    Bạn cần khóa **riêng** age để mở một bộ khôi phục đã niêm phong. Hãy cất nó ở nơi không phụ thuộc vào chính bộ khôi phục, nếu không bạn sẽ có hai thứ phải khôi phục thay vì một. Niêm phong đáng làm khi bộ khôi phục được cất ở nơi bạn không hoàn toàn kiểm soát (một trình quản lý mật khẩu dùng chung, ghi chú trên đám mây, một bản in ở văn phòng); một bộ khôi phục trong két sắt của chính bạn đã được két sắt bảo vệ.

    Khi bật mã hóa mà không cấu hình người nhận dùng được, việc tải xuống bị từ chối thẳng. BombVault không bao giờ quay về giao khóa chính dưới dạng văn bản rõ.

### Khi không có bộ khôi phục trong tay

Mật khẩu không được lưu ở đâu cả, nó được **tính** từ `APP_KEY`. Chỉ cần khóa và một shell là bạn tự tạo lại được:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Đó là HMAC-SHA256 trên chuỗi cố định `bombvault:restic-repo`, khóa là các byte thô của `APP_KEY` dạng thập lục phân, in ra 64 ký tự thập lục phân chữ thường. Cùng giá trị đó nằm trong bộ khôi phục, ghi là mật khẩu restic được suy ra; mục này dành cho ngày bộ khôi phục ở nơi khác chứ không ở chỗ bạn.

!!! warning "Với kho đã nhận, hãy dùng khóa của bản chạy GỬI"
    Kho đến đây qua nhân bản ngoại vi được tạo bởi máy đã gửi nó, bằng `APP_KEY` của **chính máy đó**. Suy ra từ khóa của máy nhận sẽ cho một mật khẩu mà restic từ chối, trông y hệt một kho bị hỏng mà thực ra không hỏng. Đây là lý do thường gặp khiến `restic check` trên kho đã nhận cứ hỏi mật khẩu mãi.

Vì các định nghĩa khôi phục nằm **bên trong** mỗi kho (`<repo>/def`, `<repo>/vm-def`), một thư mục kho được sao chép hoàn toàn tự chứa, nên bộ khôi phục cộng với kho là tất cả những gì một lần khôi phục bare-metal cần.

## Lấy lại một bản kết xuất cơ sở dữ liệu {#database-dumps}

Bản kết xuất cơ sở dữ liệu là một điểm khôi phục riêng trong kho của các container, mang nhãn `dbdump:<container>` và chỉ chứa một tệp duy nhất, `/dbdump/<container>.sql`. BombVault liệt kê, tải về và nhập chúng ở mục **Sao lưu**; dưới đây là cùng các bước ấy chỉ với restic, dành cho ngày không có BombVault.

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
