# Off-site & khôi phục

Các bản sao lưu cục bộ bảo vệ bạn khỏi một container bị mất hay một bản cập nhật tồi. Nhân bản off-site và một bộ khôi phục đã được kiểm thử bảo vệ bạn khỏi mất cả cái máy, ransomware, hoặc một trận hỏa hoạn. Trang này bao quát việc nhân bản off-site, làm cho bản sao đó chống can thiệp, chứng minh rằng bạn có thể khôi phục, và khôi phục khi chính BombVault không còn.

## Nhân bản off-site

Giữ bản sao lưu cục bộ nhanh và thêm một hoặc nhiều bản sao off-site. Đặt một kho cho mỗi miền trên tab **Settings, Off-site**. BombVault nhân bản các snapshot mới tới đó bằng `restic copy` theo kiểu nỗ lực tối đa, nên một trục trặc off-site không bao giờ làm thất bại bản sao lưu cục bộ. Kho cục bộ vẫn là chính.

- **Nhiều đích off-site cho mỗi miền.** Mỗi miền (container, VM, flash, config và bộ tập tin) có thể nhân bản tới nhiều đích off-site cùng lúc, không chỉ một, nên bạn có thể giữ, ví dụ, một rest-server trên máy của một người bạn và một S3 bucket song song. Thêm các đích bổ sung trên Settings, Off-site, mỗi đích có kho lưu trữ riêng, lớp lưu trữ S3, cờ append-only, lưu giữ và ngân sách tăng trưởng riêng. Một thiết lập off-site đơn hiện có được chuyển sang nguyên vẹn làm đích đầu tiên, và mọi đích của một miền đều nhân bản theo lịch trình off-site của miền đó.
- **Lịch trình off-site theo từng miền** (được chỉnh cùng với mọi lịch trình khác trên Settings, Schedules): để trống để nhân bản sau mỗi lần sao lưu cục bộ, hoặc đặt một nhịp độ (ví dụ `weekly Sun 03:00`) để gửi off-site ít thường xuyên hơn tần suất bạn sao lưu cục bộ. Một nút **Replicate now** lo các lần chạy theo yêu cầu.
- **Lưu giữ off-site** nằm trên Settings, Off-site để bạn có thể giữ các bản sao off-site lâu hơn như một kho lưu trữ. Để chính sách tất cả bằng 0 để không bao giờ tự động dọn bớt các snapshot off-site.
- **Giới hạn băng thông** (Settings, Off-site) giới hạn tốc độ tải lên/tải xuống của restic để việc nhân bản không làm bão hòa WAN của bạn.
- Một **chỉ báo nhân bản** hiển thị miền nào đang nhân bản trong khi nó chạy (trên trang của nó và bảng điều khiển). Đó là một chỉ báo hoạt động, không phải một thanh phần trăm, vì `restic copy` không phơi bày tiến độ đọc được bằng máy.

!!! note "Khôi phục từ bất kỳ nơi nào"
    Mọi container, VM, bộ tập tin, flash và cấu hình ứng dụng đều liệt kê các bản sao lưu của mình như một dòng thời gian duy nhất trên tất cả những nơi một bản sao lưu nằm ở đó. Một bản sao lưu đã được sao chép sang B2 chỉ xuất hiện một lần, được đánh dấu bằng từng nơi đang giữ nó. Một lần khôi phục lấy nơi đầu tiên nó tiếp cận được, bắt đầu từ kho mà mục đó được ghi vào, và bạn có thể chọn một nơi khác cho từng hàng. Các nơi off-site chỉ được đọc khi bạn mở chúng. Xóa tại một nơi sẽ kiểm tra những nơi khác trước và cho biết đó có phải bản sao cuối cùng hay không.

## Nơi lưu trữ theo từng mục {#placement}

Mỗi thẻ container, VM và bộ tập tin có một hàng **Nơi lưu trữ** với ba phân đoạn:

- **Cục bộ** ghi mục vào kho được hiển thị dưới **Lưu tại** và không sao chép nó đi đâu cả. Dùng cho dữ liệu đã có sẵn một bản sao thứ hai, ví dụ một share nằm trên NAS.
- **Cục bộ + ngoài site** cũng ghi vào đó, đồng thời sao chép đến các đích đã đánh dấu dưới **Sao chép đến**, mỗi chip ứng với một đích off-site của miền. Bỏ đánh dấu một chip thì đích đó sẽ không nhận thêm gì mới từ mục này nữa.
- **Chỉ ngoài site** ghi mục thẳng vào nơi dưới **Gửi đến**: một kho trực tiếp bên cạnh một đích off-site, hoặc một kho từ xa bạn đã thiết lập dưới Cài đặt, Đường dẫn và lưu trữ, Kho lưu trữ.

Vị trí được cố định kể từ lần sao lưu đầu tiên của mục, vì BombVault không bao giờ di chuyển bản sao lưu giữa các kho. Các bản sao thì có thể thay đổi bất cứ lúc nào. Một đích không còn nhận mục nữa vẫn giữ các bản sao đang có và cắt bớt chúng theo mức lưu giữ riêng ở lần chạy off-site tiếp theo của miền; **Xóa tại B2** trên thẻ sẽ xóa chúng ngay lập tức. Khi một số bản sao đó không tồn tại ở nơi nào khác, xác nhận sẽ liệt kê chúng theo ngày và yêu cầu nhập tên của mục. Không thể xóa bất cứ thứ gì khỏi các đích append-only.

Dưới hàng này, thẻ cho biết mục đang đi đến đâu và thực sự có gì ở đó: có bao nhiêu địa điểm đang giữ nó, mỗi đích được thấy lần cuối khi nào, và có đáp ứng 3-2-1 hay không. Một địa điểm là máy chủ có dữ liệu gốc, mỗi đích off-site và mỗi kho được đánh dấu **Ngoài cơ sở**. BombVault kiểm tra bản sao và địa điểm; nó không kiểm tra phần "hai loại vật lưu trữ" của 3-2-1.

### Nơi lưu trữ mặc định

Cài đặt, Đường dẫn và lưu trữ, **Nơi lưu trữ mặc định** có một hàng cho mỗi miền với cùng ba phân đoạn. Các bản sao áp dụng ngay cho mọi mục không có lựa chọn riêng, và cho các thư mục dự án của các stack Compose. Vị trí áp dụng cho một mục mới ở lần sao lưu đầu tiên của nó; thay đổi nó không di chuyển bất kỳ bản sao lưu nào. Trước khi lưu, hàng này nêu tên mọi đích sẽ nhận thêm hoặc mất mục, và điều đó có nghĩa là bao nhiêu snapshot. **Áp dụng cho các mục chưa có bản sao lưu** đưa mọi mục chưa có bản sao lưu nào trở về mặc định.

Một đích off-site mới sẽ nhận mọi mục không đặt là Cục bộ. Hộp thoại thêm đích đó cho biết có bao nhiêu mục và, nếu biết, đó là bao nhiêu lịch sử, đồng thời đề nghị bỏ qua những mục đã bị loại trừ khỏi các đích khác.

### Kho trực tiếp

Chọn kho trực tiếp của một đích dưới Chỉ ngoài site sẽ mở một hộp thoại với vị trí được đề xuất bên cạnh đích đó, ví dụ `b2:bucket:containers-direct`, và một lần kiểm tra kết nối không tạo ra gì cả. **Tạo và dùng** sẽ tạo kho và trỏ mục vào đó. Một kho trực tiếp nhận khóa, lớp lưu trữ, giới hạn, cài đặt append-only và mức lưu giữ của đích, và thay đổi theo chúng; thẻ Kho lưu trữ hiển thị nó ở chế độ chỉ đọc. Khi một khóa mới của đích không thể mở được nó, kho trực tiếp giữ nguyên khóa đang có và lần lưu sẽ cho biết điều đó. Các snapshot của nó mang nhãn `bv:direct`, và mọi lần cắt tỉa khác đều giữ chúng lại, nên một kho trực tiếp đã mất liên kết với đích của nó sẽ không bao giờ già đi theo các quy tắc cục bộ. Khóa B2 chỉ giới hạn trong thư mục riêng của đích sẽ không thể tiếp cận thư mục bên cạnh nó; hãy giới hạn khóa vào thư mục phía trên đích thay vì vậy.

### Ngoài cơ sở

Một kho đã đặt tên có thể được đánh dấu **Ngoài cơ sở** trên thẻ Kho lưu trữ. Các kho từ xa bắt đầu ở trạng thái đã đánh dấu; hãy tắt nó cho một rest-server trong cùng tòa nhà. Dấu này chỉ tính vào số địa điểm và 3-2-1 trên các thẻ. Nó không thay đổi bản sao nào.

### Sau một lần xây dựng lại

Các lựa chọn sao chép sống trong cài đặt riêng của BombVault. Sau một lần xây dựng lại qua Discover mà không có `/config` được khôi phục, chúng biến mất, và việc sao chép mọi thứ sẽ gửi lại lên B2 những mục bạn đã từng bỏ qua. Vì vậy việc nhân bản off-site của mọi miền được xây dựng lại sẽ tạm dừng. Dashboard hiển thị điều này bằng màu hổ phách, và Nơi lưu trữ mặc định đưa ra **Xác nhận mặc định** với một bản xem trước những gì lần chạy tiếp theo sẽ sao chép, cùng các tên trong bản sao lưu không có mục tương ứng, mà bạn có thể bỏ qua ngay tại đó. Chỉ có xác nhận mới chấm dứt việc tạm dừng; nhập một tệp cài đặt sẽ mang quy tắc và mặc định trở lại nhưng không chấm dứt việc tạm dừng.

## Kho chính từ xa {#remote-primary-repositories}

Đường dẫn sao lưu của một miền (Cài đặt, Đường dẫn và lưu trữ) không giới hạn ở thư mục cục bộ: trỏ thẳng nó tới một kho restic từ xa (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:nguoidung@host:/repo`, `rclone:remote:bucket/duong-dan`) và BombVault sao lưu thẳng tới đó, không cần bản sao cục bộ riêng và không có bước nhân bản. Đây thực sự là một hình thái khác với nhân bản ngoại vi ở trên: ở đó kho cục bộ là kho chính còn kho ngoại vi là bản lưu trữ của nó trong khả năng có thể; ở đây kho từ xa **chính là** kho chính, và là bản duy nhất chừng nào bạn chưa cấu hình thêm nhân bản ngoại vi (hoặc một kho từ xa thứ hai) cho miền đó.

Mỗi trong năm ô đường dẫn (Container, Máy ảo, Flash, Cấu hình, Tệp) đều có ngay bên cạnh một công tắc **Cục bộ / Từ xa**:

- **Cục bộ** hiển thị trình duyệt thư mục quen thuộc.
- **Từ xa** đổi nó thành một ô URL đơn giản, kèm một nút mở đúng hộp thoại kiểm tra kết nối và thông tin đăng nhập mà các đích ngoại vi vẫn dùng, chỉ khác là được cấu hình cho kho chính này. Từ đó bạn có:
    - **Một lần kiểm tra kết nối** với đường dẫn thật, trước khi bạn trông cậy vào nó.
    - **Giới hạn băng thông** (tải lên và tải xuống) để một bản sao lưu theo lịch tới kho chính từ xa không làm nghẽn đường WAN của bạn: đúng những tùy chọn restic `--limit-upload` và `--limit-download` mà nhân bản ngoại vi dùng, nay áp lên chính việc sao lưu.
    - **Bảo vệ chỉ-ghi-thêm (bất biến)**, được xác minh bằng đúng bài kiểm tra can thiệp chủ động (một phép thử DELETE thật tới đầu bên kia) mà các đích ngoại vi nhận được. Khi bật, BombVault từ chối tự cắt tỉa kho: vì phía sau không có bản sao cục bộ riêng, thông tin đăng nhập trên máy này không được phép xóa bản sao lưu duy nhất.
    - **Cảnh báo ngân sách tăng trưởng**, lấy từ chính xu hướng kích thước kho mà thẻ Lưu trữ vốn đã theo dõi.

Không điều nào trong số này là bắt buộc: một đường dẫn từ xa gõ tay, không lưu thiết lập an toàn nào, vẫn sao lưu y như trước (băng thông không giới hạn, cắt tỉa được, không cảnh báo ngân sách). Hộp thoại an toàn có ở đó cho lúc bạn muốn đúng những lớp bảo vệ mà một bản sao ngoại vi nhận được, mà không phải tạo riêng một đích ngoại vi chỉ để có chúng.

!!! note "Thông tin đăng nhập đám mây và REST dùng chung"
    Kho chính từ xa xác thực bằng đúng thông tin đăng nhập S3/REST đã cấu hình ở Cài đặt, Ngoại vi, Thông tin đăng nhập đám mây. Không có kho thông tin đăng nhập riêng cho các kho chính.

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

- **Diễn tập xác minh khôi phục (cục bộ).** BombVault định kỳ chạy `restic check --read-data-subset` (có giới hạn, không bao giờ là một lần khôi phục toàn bộ làm đầy đĩa) và hiển thị một huy hiệu *xác minh khôi phục được lần cuối* cho mỗi miền. Nhịp độ nằm trên Settings, Schedules; huy hiệu trên Settings, Integrity.
- **Diễn tập DR (off-site).** BombVault khôi phục một đích thực từ kho off-site vào một hộp cát dùng một lần, xác minh nó từng tập tin và từng byte, rồi dọn dẹp. Điều này chứng minh bạn có thể khôi phục từ off-site, không chỉ là kho phản hồi.

**Bảng điểm bảo vệ chống ransomware** trên bảng điều khiển gom điều này thành một thế phòng thủ xanh / hổ phách / đỏ cho mỗi miền, với một danh sách kiểm tra có đóng dấu tuổi (off-site đã cấu hình, append-only đã xác minh, nhân bản hiện thời, diễn tập khôi phục đã qua, mã hóa đã bật, chiến lược dọn bớt đã đặt). Mỗi hàng đỏ liên kết sâu tới bản sửa, và thẻ chỉ bao giờ chuyển xanh dựa trên các sự thật đã xác minh.

## Bảng điều khiển bên nhận (phía nhận)

![Phía nhận, được theo dõi ở chế độ chỉ đọc, với kiểm tra toàn vẹn chạy trên máy này.](assets/screenshots/receiver.png)

*Phía nhận, được theo dõi ở chế độ chỉ đọc, với kiểm tra toàn vẹn chạy trên máy này.*

Mọi thứ ở trên là phía *gửi*. Trên máy **nhận** các bản sao off-site bất biến từ một BombVault khác, bảng điều khiển bên nhận cho bạn giám sát độc lập, chỉ đọc các kho đó trên phần cứng bên nhận, nên một lần thất bại âm thầm ở đầu xa không bị bỏ qua.

Bật công tắc **Receiver** trong Settings để hé lộ một tab **Receiver**. Nó mặc định tắt; chỉ bật nó trên một máy thực sự nhận các bản sao lưu off-site bất biến. Sau đó đăng ký một kho đã nhận (chỉ đọc, mở bằng khóa của phiên bản gửi) để có được:

- **Một kho snapshot được gom theo nguồn**, nên bạn có thể thấy chính xác những container, VM và bộ tập tin nào đã đến.
- **Nhận lần cuối** cho mỗi nguồn, nên bạn biết mỗi cái mới đến mức nào.
- **Một `restic check` độc lập** chạy trên phần cứng bên nhận, nên tính toàn vẹn được xác minh ngay nơi dữ liệu thực sự nằm, không chỉ trên bên gửi.
- **Một công tắc người chết:** một cảnh báo khi một nguồn ngừng gửi trong một khoảng thời gian bạn đặt.
- **Cảnh báo toàn vẹn:** một cảnh báo khi một lần kiểm tra ở phía nhận thất bại.

Bên nhận nghiêm ngặt chỉ đọc. Nó không bao giờ ghi vào kho đã nhận, nên nó không bao giờ có thể phá vỡ bảo đảm append-only mà bên gửi dựa vào.

## Ví dụ hoàn chỉnh: hai máy Unraid, từ đầu đến cuối

Phần trên mô tả các bộ phận. Đây là một thiết lập hoàn chỉnh với giá trị thật, vì các bộ phận dễ lắp hơn nhiều khi ta đã thấy chúng lắp xong một lần.

Hai máy: **TOWER** chạy các container và gửi bản sao lưu, **VAULT** nhận chúng và cưỡng chế tính bất biến. Hãy thay bằng tên, địa chỉ và đường dẫn chia sẻ của bạn.

**1. Trên VAULT, dựng máy chủ chỉ-ghi-thêm.** Trong BombVault trên TOWER, vào *Cài đặt → Ngoại vi → thiết lập có hướng dẫn*, chọn **rest-server** và tạo công thức. Sao chép thẻ **Mẫu Unraid (XML)**, lưu trên VAULT thành `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, rồi *Docker → Add Container* và chọn **rest-server** trong danh sách mẫu. Trước khi khởi động, ghi dòng `htpasswd` hiển thị vào `/mnt/user/appdata/rest-server/.htpasswd` trên VAULT. Mật khẩu dùng một lần chỉ hiện một lần và không bao giờ được lưu, hãy sao chép ngay. Dòng đó mang chính mật khẩu ấy, đã được băm bằng bcrypt sẵn cho bạn: văn bản rõ đi vào thông tin đăng nhập REST trên TOWER, dòng đã băm đi vào `.htpasswd` trên VAULT. Bạn không phải tự băm gì cả.

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

**4. Trên VAULT, xem những gì tới nơi.** Bật *Cài đặt → Bộ nhận*, mở thẻ **Bộ nhận** và đăng ký kho ở chế độ chỉ đọc.

!!! warning "Vị trí là đường dẫn **bên trong** container, viết tương đối so với điểm gắn của máy chủ"
    Nhập `user/appdata/rest-server/bombvault-containers/containers`, **không phải** `/mnt/user/appdata/…`. BombVault chạy trong container, nơi `/mnt` của máy chủ được gắn ở chỗ khác; đường dẫn tuyệt đối của máy chủ không tồn tại bên trong. Nếu bạn dán vào, BombVault nay sẽ cho biết đường dẫn tương đối cần dùng.

    **APP_KEY bên gửi** là khóa của TOWER, không phải của VAULT. Bạn tìm thấy nó trên TOWER tại *Cài đặt → Hệ thống*.

**5. Nếu muốn, hãy làm hai chiều.** Lặp lại đúng năm bước theo chiều ngược lại: một rest-server trên TOWER nhận bản sao của VAULT. Khi đó mỗi máy cưỡng chế tính bất biến cho máy kia, và không máy nào xóa được bản sao lưu của máy kia.

## Khôi phục có hướng dẫn

Một tab **Recovery** chuyên biệt dẫn một bản cài đặt mới hoặc được dựng lại đi qua tình huống thảm họa, ở một nơi:

1. **Khôi phục cài đặt của chính BombVault trước**, nên các đường dẫn sao lưu, đích off-site và thông tin đăng nhập mà phần còn lại của quy trình cần được điền sẵn (áp dụng qua một lần tự khởi động lại thông qua Docker socket, nên cơ sở dữ liệu cài đặt đang chạy không bao giờ bị ghi đè dưới một handle đang mở).
2. **Kiểm tra BombVault có thể đọc các bản sao lưu của bạn** (điểm mắc kẹt về khóa mã hóa ngay từ đầu).
3. Cho bạn **trỏ tới kho hiện có của bạn** (cục bộ hoặc off-site).
4. **Khám phá** các container, VM và bộ tập tin được lưu trong đó.
5. **Khôi phục tất cả chúng** (để nguyên trạng thái dừng, nên bạn khởi động chúng một cách có chủ đích), với bộ khôi phục của bạn chỉ cách một cú nhấp.

!!! note "Các bản sao off-site chờ sau một lần xây dựng lại"
    Khi bước 4 xây dựng lại các mục nhập mà không có cài đặt cũ, việc nhân bản off-site của các miền đó tạm dừng cho đến khi nơi lưu trữ mặc định được xác nhận. Xem [Nơi lưu trữ theo từng mục](#placement).

!!! tip "Di chuyển theo kế hoạch so với thảm họa"
    Khôi phục có hướng dẫn khôi phục cài đặt của chính BombVault từ một bản sao lưu. Với một lần chuyển *theo kế hoạch* sang một máy mới, thay vào đó bạn có thể mang cấu hình của mình theo trực tiếp bằng thẻ **Xuất và nhập cài đặt** (một tệp JSON di động). Xem [Cấu hình](configuration.md#portable-settings-export-and-import).

### Khôi phục từ một kho BombVault khác

Một thẻ riêng trên tab **Recovery** mở kho của một phiên bản BombVault *khác* (một share được gắn kết dưới `/mnt`, hoặc một URL từ xa) bằng **`APP_KEY` của phiên bản đó**, trong một phiên chỉ đọc, dùng một lần. Duyệt các container, VM và bộ tập tin được lưu ở đó, chọn một snapshot và khôi phục nó, và đối tượng đã khôi phục trở thành một container, VM hay bộ tập tin cục bộ bình thường. Không có gì bao giờ được ghi vào kho kia, và các cài đặt sao lưu của chính bạn giữ nguyên không bị đụng (phiên sống trong bộ nhớ và tự hết hạn). Chuyển một container từ máy chủ A sang máy chủ B không còn có nghĩa là trỏ lại cài đặt kho của bạn rồi hoàn nguyên chúng sau đó. Liên kết máy-chủ-với-máy-chủ trực tiếp được rõ ràng nằm ngoài phạm vi; đây là một lần kéo một phát có chủ đích.

## Bộ khôi phục khóa mã hóa

Đây là mảnh khiến việc khôi phục sau thảm họa trở nên khả thi ngay cả khi không có một BombVault đang chạy.

Một cú nhấp tải xuống **khóa chính**, **mật khẩu restic dẫn xuất**, và **các vị trí kho cùng lệnh chính xác**, nên bạn có thể khôi phục thẳng bằng restic CLI trên bất kỳ máy nào. Một lời nhắc trên bảng điều khiển sẽ nhắc nhở cho đến khi bạn đã cất giữ nó.

!!! danger "Cất giữ bộ khôi phục ngoài máy chủ"
    Bộ khôi phục chứa bí mật giải mã các bản sao lưu của bạn. Giữ nó ở nơi an toàn và tách biệt khỏi máy chủ (một trình quản lý mật khẩu, một bản in trong két sắt). Nếu bạn mất cả BombVault và `APP_KEY` mà không có bộ khôi phục, các bản sao lưu đã mã hóa của bạn không thể khôi phục được.

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
