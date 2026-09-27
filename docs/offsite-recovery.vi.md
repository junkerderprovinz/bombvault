# Off-site & khôi phục

Các bản sao lưu cục bộ bảo vệ bạn khỏi một container bị mất hay một bản cập nhật tồi. Nhân bản off-site và một bộ khôi phục đã được kiểm thử bảo vệ bạn khỏi mất cả cái máy, ransomware, hoặc một trận hỏa hoạn. Trang này bao quát việc nhân bản off-site, làm cho bản sao đó chống can thiệp, chứng minh rằng bạn có thể khôi phục, và khôi phục khi chính BombVault không còn.

## Nhân bản off-site

Giữ bản sao lưu cục bộ nhanh và sao chép nó đến một hoặc nhiều điểm lưu trữ khác. Bạn chọn các điểm lưu trữ mà một miền được sao chép đến trên thẻ **Miền** dưới **Cài đặt, Lưu trữ**, mỗi chip ứng với một điểm lưu trữ (xem [Điểm lưu trữ](storage-places.md#domains)). BombVault sao chép các snapshot mới tới đó bằng `restic copy` theo kiểu nỗ lực tối đa, nên một lần sao chép thất bại không bao giờ làm thất bại bản sao lưu cục bộ. Điểm lưu trữ nơi một miền được lưu không nhất thiết phải là cục bộ; xem [Một miền được lưu ở điểm lưu trữ từ xa](#remote-primary-repositories).

- **Nhiều điểm sao chép cho mỗi miền.** Một miền có thể được sao chép đến nhiều điểm lưu trữ cùng lúc, ví dụ một rest-server ở nhà một người bạn và một bucket B2. Mức lưu giữ, lớp lưu trữ, append-only, giới hạn và ngân sách tăng trưởng thuộc về điểm lưu trữ, nên mỗi bản sao tuân theo quy tắc của điểm lưu trữ mà nó được gửi tới.
- **Lịch sao chép theo từng miền** (được chỉnh cùng với mọi lịch trình khác trên Cài đặt, Lịch trình): để trống để sao chép sau mỗi lần sao lưu cục bộ, hoặc đặt một nhịp độ (ví dụ `weekly Sun 03:00`) để sao chép ít thường xuyên hơn tần suất bạn sao lưu. **Sao chép ngay** trên hàng của miền chạy nó theo yêu cầu.
- **Lưu giữ theo từng điểm lưu trữ.** Mỗi điểm lưu trữ giữ quy tắc riêng của nó, nên một điểm lưu trữ off-site có thể giữ các bản sao lâu hơn để lưu trữ dài hạn. Một điểm lưu trữ có mọi quy tắc bằng 0 thì không bao giờ cắt bớt.
- **Giới hạn băng thông** theo từng điểm lưu trữ giới hạn tốc độ tải lên và tải xuống của restic để việc sao chép không làm bão hòa WAN của bạn.
- Một **chỉ báo nhân bản** hiển thị miền nào đang sao chép trong khi việc sao chép chạy (trên trang của nó và bảng điều khiển). Đó là một chỉ báo hoạt động, không phải một thanh phần trăm, vì `restic copy` không phơi bày tiến độ đọc được bằng máy.

!!! note "Khôi phục từ bất kỳ nơi nào"
    Mọi container, VM, bộ tập tin, flash và cấu hình ứng dụng đều liệt kê các bản sao lưu của mình như một dòng thời gian duy nhất trên tất cả những nơi một bản sao lưu nằm ở đó. Một bản sao lưu đã được sao chép sang B2 chỉ xuất hiện một lần, được đánh dấu bằng từng nơi đang giữ nó. Một lần khôi phục lấy nơi đầu tiên nó tiếp cận được, bắt đầu từ kho mà mục đó được ghi vào, và bạn có thể chọn một nơi khác cho từng hàng. Các nơi off-site chỉ được đọc khi bạn mở chúng. Xóa tại một nơi sẽ kiểm tra những nơi khác trước và cho biết đó có phải bản sao cuối cùng hay không.

## Nơi lưu trữ theo từng mục {#placement}

Mỗi thẻ container, VM và bộ tập tin có một hàng **Nơi lưu trữ** với ba phân đoạn:

- **Cục bộ** ghi mục vào kho được hiển thị dưới **Lưu tại** và không sao chép nó đi đâu cả. Dùng cho dữ liệu đã có sẵn một bản sao thứ hai, ví dụ một share nằm trên NAS.
- **Cục bộ + ngoài site** cũng ghi vào đó, đồng thời sao chép đến các đích đã đánh dấu dưới **Sao chép đến**, mỗi chip ứng với một đích off-site của miền. Bỏ đánh dấu một chip thì đích đó sẽ không nhận thêm gì mới từ mục này nữa.
- **Chỉ ngoài site** ghi mục thẳng vào điểm lưu trữ dưới **Gửi đến**, bất kỳ điểm lưu trữ nào khác ngoài nơi lưu chính của miền. Khi miền đã được sao chép đến điểm lưu trữ đó, mục nhận một kho trực tiếp bên cạnh các bản sao; nếu không, BombVault tạo một kho cho miền tại đó.

Vị trí được cố định kể từ lần sao lưu đầu tiên của mục, vì BombVault không bao giờ di chuyển bản sao lưu giữa các kho. Các bản sao thì có thể thay đổi bất cứ lúc nào. Một đích không còn nhận mục nữa vẫn giữ các bản sao đang có và cắt bớt chúng theo mức lưu giữ riêng ở lần chạy off-site tiếp theo của miền; **Xóa tại B2** trên thẻ sẽ xóa chúng ngay lập tức. Khi một số bản sao đó không tồn tại ở nơi nào khác, xác nhận sẽ liệt kê chúng theo ngày và yêu cầu nhập tên của mục. Không thể xóa bất cứ thứ gì khỏi các đích append-only.

Dưới hàng này, thẻ cho biết mục đang đi đến đâu và thực sự có gì ở đó: có bao nhiêu địa điểm đang giữ nó, mỗi đích được thấy lần cuối khi nào, và có đáp ứng 3-2-1 hay không. Một địa điểm là máy chủ có dữ liệu gốc và mỗi điểm lưu trữ ở một địa điểm khác (xem [Ngoài cơ sở](#off-the-premises-mark)). BombVault kiểm tra bản sao và địa điểm; nó không kiểm tra phần "hai loại vật lưu trữ" của 3-2-1.

### Mặc định theo từng miền

Thẻ **Miền** dưới Cài đặt, Lưu trữ có một hàng cho mỗi miền. **Sao chép đến** áp dụng ngay cho mọi mục không có lựa chọn riêng, và cho các thư mục dự án của các stack Compose. Khi một miền đã có bản sao lưu, **Lưu tại** áp dụng cho một mục mới ở lần sao lưu đầu tiên của nó, và thay đổi nó không di chuyển bất kỳ bản sao lưu nào. Trước khi lưu, hàng này nêu tên mọi điểm lưu trữ sẽ nhận thêm hoặc mất mục và điều đó có nghĩa là bao nhiêu snapshot, và câu hỏi xác nhận kèm công tắc **Áp dụng cho các mục chưa có bản sao lưu**, công tắc này cũng đưa mọi mục chưa có bản sao lưu nào sang mặc định mới. **Ngoại lệ** liệt kê các mục có lựa chọn riêng.

Đánh dấu một điểm lưu trữ mới dưới **Sao chép đến** sẽ khiến nó nhận mọi mục không đặt là Cục bộ. Lời xác nhận cho biết có bao nhiêu mục và, nếu biết, đó là bao nhiêu lịch sử.

### Kho trực tiếp

Chọn dưới Chỉ ngoài site một điểm lưu trữ mà miền đã được sao chép đến sẽ hỏi một lần, rồi tạo một kho trực tiếp bên cạnh các bản sao, ví dụ `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, và trỏ mục vào đó. Với một đích sao chép không có điểm lưu trữ, lựa chọn này mở một hộp thoại với một địa chỉ được đề xuất và một lần kiểm tra kết nối không tạo ra gì cả, và **Tạo và dùng** sẽ tạo kho. Một kho trực tiếp nhận khóa, lớp lưu trữ, giới hạn, cài đặt append-only và mức lưu giữ của điểm lưu trữ, và thay đổi theo chúng. Khi một khóa mới của điểm lưu trữ không thể mở được nó, kho trực tiếp giữ nguyên khóa đang có và lần lưu sẽ cho biết điều đó. Các snapshot của nó mang nhãn `bv:direct`, và mọi lần cắt tỉa khác đều giữ chúng lại, nên một kho trực tiếp đã mất liên kết với điểm lưu trữ của nó sẽ không bao giờ già đi theo các quy tắc cục bộ. Một khóa B2 bị giới hạn trong một thư mục phải bao trùm địa chỉ của điểm lưu trữ, không chỉ thư mục của miền, nếu không thư mục bên cạnh sẽ nằm ngoài tầm với.

### Ngoài cơ sở {#off-the-premises-mark}

Một bản sao chỉ được tính là một địa điểm riêng khi điểm lưu trữ của nó ở một địa điểm khác. Một điểm lưu trữ đám mây luôn được tính còn một thư mục trên Unraid này thì không bao giờ; với một NAS, một rest-server hoặc một máy chủ SFTP, hãy trả lời **Thiết bị nằm ở đâu?** trong phần chi tiết của điểm lưu trữ bằng **Ở đây, trong nhà** hoặc **Ở một địa điểm khác**. Câu trả lời chỉ dùng để đếm địa điểm và 3-2-1 trên các thẻ và bảng điều khiển. Nó không thay đổi bản sao nào.

### Sau một lần xây dựng lại

Các lựa chọn sao chép sống trong cài đặt riêng của BombVault. Sau một lần xây dựng lại qua Discover mà không có `/config` được khôi phục, chúng biến mất, và việc sao chép mọi thứ sẽ gửi lại lên B2 những mục bạn đã từng bỏ qua. Vì vậy việc nhân bản off-site của mọi miền được xây dựng lại sẽ tạm dừng. Dashboard hiển thị điều này bằng màu hổ phách, và hàng của miền trên thẻ Miền đưa ra **Xác nhận mặc định** với một bản xem trước những gì lần chạy tiếp theo sẽ sao chép, cùng các tên trong bản sao lưu không có mục tương ứng, mà bạn có thể bỏ qua ngay tại đó. Chỉ có xác nhận mới chấm dứt việc tạm dừng; nhập một tệp cài đặt sẽ mang quy tắc và mặc định trở lại nhưng không chấm dứt việc tạm dừng.

## Một miền được lưu ở điểm lưu trữ từ xa {#remote-primary-repositories}

Một miền không nhất thiết phải được lưu cục bộ. Khi vị trí sao lưu của nó chưa chứa bản sao lưu nào, hãy chọn một điểm lưu trữ từ xa dưới **Lưu tại** trên thẻ Miền và miền sẽ sao lưu thẳng tới đó, không có bản sao cục bộ và không có bước sao chép. Khi đó kho từ xa là bản duy nhất, trừ khi miền còn được sao chép đến một điểm lưu trữ khác. Mọi điểm lưu trữ từ xa đều có cùng các biện pháp bảo vệ:

- **Một lần kiểm tra kết nối** trước khi bất cứ thứ gì được ghi.
- **Giới hạn băng thông** cho chính việc sao lưu, đúng những tùy chọn `--limit-upload` và `--limit-download` mà một lần sao chép dùng.
- **Bảo vệ append-only**, được xác minh bằng đúng bài kiểm tra can thiệp chủ động. Khi bật, BombVault không bao giờ cắt tỉa kho, vì thông tin đăng nhập trên máy này không được phép xóa bản sao lưu duy nhất.
- **Ngân sách tăng trưởng**, lấy từ chính xu hướng kích thước mà thẻ Lưu trữ theo dõi.

Một miền được lưu ở điểm lưu trữ từ xa là nguồn của các bản sao của nó, giống như một miền cục bộ; xem [Sao chép giữa các điểm lưu trữ có thông tin đăng nhập khác nhau](storage-places.md#different-credentials).

!!! note "Thông tin đăng nhập thuộc về điểm lưu trữ"
    Một điểm lưu trữ từ xa giữ thông tin đăng nhập riêng của nó. Một điểm lưu trữ được thiết lập bằng thông tin đăng nhập đám mây dùng chung sẽ tiếp tục dùng chúng cho đến khi quyền truy cập của nó được thay đổi trong phần chi tiết.

## Off-site bất biến (append-only)

Đánh dấu một kho off-site là append-only để ransomware, hoặc một máy chủ bị xâm nhập, không thể xóa hay ghi lại các bản sao lưu của bạn. Phía bên kia (một `restic/rest-server` chạy ở chế độ `--append-only`) **thực thi** điều đó. BombVault chỉ luôn **xác minh** nó và không bao giờ hiển thị xanh chỉ dựa trên một tuyên bố cấu hình.

Cửa sổ **Thêm điểm lưu trữ** kèm một công thức sẵn sàng để dán cho một rest-server ở chế độ append-only, với một người dùng cho BombVault này. Tại một điểm lưu trữ rest-server có bật **Append-only**, **Kiểm tra append-only** trong phần chi tiết của điểm lưu trữ chạy bài kiểm tra can thiệp với mọi đường dẫn miền, bản sao đang bật và kho tại điểm lưu trữ, rồi đưa ra một câu trả lời cho cả điểm lưu trữ, nên off-site append-only là điều có thể đạt được mà không cần chỉnh sửa cấu hình bằng tay.

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

Tại một điểm lưu trữ, **Kiểm tra append-only** dò từng đường dẫn miền, bản sao đang bật và kho ở đó bằng thông tin đăng nhập riêng của nó rồi gộp các kết luận thành một câu trả lời: chỉ cần một kho chấp nhận việc xóa là cả điểm lưu trữ thành *xóa được chấp nhận*.

## Diễn tập DR

BombVault cung cấp hai cấp độ bằng chứng rằng các bản sao lưu của bạn thực sự khôi phục được, không chỉ hiện diện.

- **Diễn tập xác minh khôi phục (cục bộ).** BombVault định kỳ chạy `restic check --read-data-subset` (có giới hạn, không bao giờ là một lần khôi phục toàn bộ làm đầy đĩa) và hiển thị một huy hiệu *xác minh khôi phục được lần cuối* cho mỗi miền. Nhịp độ nằm trên Settings, Schedules; huy hiệu trên Settings, Integrity.
- **Diễn tập DR (off-site).** BombVault khôi phục một đích thực từ kho off-site vào một hộp cát dùng một lần, xác minh nó từng tập tin và từng byte, rồi dọn dẹp. Điều này chứng minh bạn có thể khôi phục từ off-site, không chỉ là kho phản hồi. Chỉ các điểm lưu trữ ở địa điểm khác mới được diễn tập, vì một bản sao trong cùng ngôi nhà không chứng minh được gì khi mất cả ngôi nhà. Một miền được sao chép tới nhiều điểm như vậy sẽ lần lượt được diễn tập với một điểm mỗi lần chạy theo lịch, và bảng điều khiển ghi tên điểm lưu trữ của lần diễn tập gần nhất.

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

**1. Trên VAULT, dựng máy chủ chỉ-ghi-thêm.** Trong BombVault trên TOWER, mở *Cài đặt → Lưu trữ*, nhấp **Thêm điểm lưu trữ**, chọn **rest-server** và nhấp **Hiện công thức**. Sao chép khối **Mẫu Unraid**, lưu trên VAULT thành `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, rồi *Docker → Add Container* và chọn **rest-server** trong danh sách mẫu. Trước khi khởi động, ghi dòng `htpasswd` hiển thị vào `/mnt/user/appdata/rest-server/.htpasswd` trên VAULT. Mật khẩu chỉ hiện một lần và không bao giờ được lưu; công thức đã điền nó cùng người dùng vào biểu mẫu trên TOWER, nên hãy để cửa sổ đó mở. Dòng `htpasswd` mang chính mật khẩu ấy, đã được băm bằng bcrypt sẵn cho bạn, nên bạn không phải tự băm gì cả.

    Giữ nguyên `--append-only` trong ô OPTIONS. Thiếu nó, VAULT lại chỉ là một thư mục chia sẻ thông thường.

**2. Trên TOWER, thêm điểm lưu trữ.** Nhập địa chỉ của VAULT, `http://VAULT:8000`, bên cạnh người dùng và mật khẩu mà công thức đã điền, rồi nhấp **Kiểm tra kết nối**. BombVault dựng địa chỉ từ các thông tin đó:

    rest:http://VAULT:8000/tower

Đoạn đầu của đường dẫn là người dùng htpasswd, ở đây là `tower`, và mỗi miền có thư mục riêng bên dưới, ví dụ `rest:http://VAULT:8000/tower/container`. Trả lời **Thiết bị nằm ở đâu?** bằng **Ở một địa điểm khác**, nhấp **Thêm**, rồi đánh dấu điểm lưu trữ dưới **Sao chép đến** cho các miền cần gửi đến đó.

**3. Trên TOWER, bật Append-only** dưới **Bảo vệ** trong phần chi tiết của điểm lưu trữ, rồi nhấp **Kiểm tra append-only**. Bài kiểm tra dò mọi đường dẫn miền, bản sao và kho tại điểm lưu trữ và đưa ra một câu trả lời cho điểm lưu trữ, câu trả lời đó phải là *xóa bị từ chối*. Ý nghĩa các kết quả:

| Kết quả | Điều đã xảy ra |
| --- | --- |
| **xóa bị từ chối** | VAULT đã từ chối lệnh xóa. Đây là trạng thái đạt duy nhất. |
| **xóa được chấp nhận** | VAULT đã chấp nhận một lệnh xóa. Thiếu `--append-only` hoặc nó đã bị bỏ đi. |
| một thông báo thay vì kết quả | Bài kiểm tra không chạy được. Thường là địa chỉ không phải địa chỉ mà chính restic dùng, hoặc thông tin đăng nhập đã đổi. Không có gì được ghi lại và không có cảnh báo nào. |

**4. Trên VAULT, xem những gì tới nơi.** Bật *Cài đặt → Bộ nhận*, mở thẻ **Bộ nhận** và đăng ký kho ở chế độ chỉ đọc.

!!! warning "Vị trí là đường dẫn **bên trong** container, viết tương đối so với điểm gắn của máy chủ"
    Nhập `user/appdata/rest-server/tower/container`, **không phải** `/mnt/user/appdata/…`. BombVault chạy trong container, nơi `/mnt` của máy chủ được gắn ở chỗ khác; đường dẫn tuyệt đối của máy chủ không tồn tại bên trong. Nếu bạn dán vào, BombVault sẽ cho biết đường dẫn tương đối cần dùng.

    **APP_KEY bên gửi** là khóa của TOWER, không phải của VAULT. Bạn tìm thấy nó trên TOWER tại *Cài đặt → Hệ thống*.

**5. Nếu muốn, hãy làm hai chiều.** Lặp lại đúng năm bước theo chiều ngược lại: một rest-server trên TOWER nhận bản sao của VAULT. Khi đó mỗi máy cưỡng chế tính bất biến cho máy kia, và không máy nào xóa được bản sao lưu của máy kia.

## Khôi phục có hướng dẫn

Một tab **Recovery** chuyên biệt dẫn một bản cài đặt mới hoặc được dựng lại đi qua tình huống thảm họa, ở một nơi:

1. **Kiểm tra BombVault có thể đọc các bản sao lưu của bạn** (điểm mắc kẹt về khóa mã hóa ngay từ đầu).
2. **Khôi phục cài đặt của chính BombVault**, nên các đường dẫn sao lưu, đích off-site và thông tin đăng nhập mà phần còn lại của quy trình cần được điền sẵn. Nó đọc bản sao lưu cài đặt từ điểm lưu trữ mà hàng Tự sao lưu ghi ở **Lưu tại**, hoặc từ bản sao của Tự sao lưu ở **Sao chép đến**, và hiển thị điểm lưu trữ đó kèm địa chỉ; để đọc từ một điểm lưu trữ khác, hãy đổi hàng Tự sao lưu ở bước 3 trước. Việc khôi phục được áp dụng qua một lần tự khởi động lại thông qua Docker socket, nên cơ sở dữ liệu cài đặt đang chạy không bao giờ bị ghi đè dưới một handle đang mở.
3. **Gắn các bản sao lưu hiện có của bạn** qua các hàng của thẻ **Miền**: trên hàng của mỗi miền, chọn điểm lưu trữ chứa các bản sao lưu của nó ở **Lưu tại** và các điểm lưu trữ giữ bản sao của nó ở **Sao chép đến**. Một điểm lưu trữ chưa có hàng nào đưa ra, chẳng hạn một thư mục chia sẻ, một máy chủ hay một bucket đám mây, được kết nối bằng **Thêm điểm lưu trữ**, cùng cửa sổ như trên Cài đặt, Lưu trữ. Sau đó **Kết nối và xem trước** kiểm tra rằng các bản sao lưu đọc được.
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
