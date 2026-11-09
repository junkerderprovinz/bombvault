# Thông báo

BombVault có thể báo cho bạn khi một bản sao lưu thất bại, khi nó thành công, khi một bản bị trễ, và mỗi tuần một lần về tình hình chung. Mọi thứ được đặt trong **Cài đặt, Thông báo**: những gì BombVault gửi ngoài các bản sao lưu riêng lẻ nằm trong thẻ **Thông báo**, còn tin nhắn đi đâu nằm trong thẻ **Kênh**. Mỗi kênh tự quyết định khi nào lên tiếng, và một kiểm tra Healthchecks cũng là một kênh.

![Cài đặt, Thông báo với kênh Gotify đã điền xong.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Cài đặt, Thông báo với kênh Gotify đã điền xong.](assets/screenshots/notifications/gotify-light.png#only-light)

## Điều gì gửi tin nhắn {#what}

Các công tắc trong thẻ **Thông báo** áp dụng cho mọi kênh:

- **Tóm tắt các lần chạy theo lịch**: một tin nhắn cho mỗi lần chạy theo lịch, chẳng hạn *42 trên 45 thành công*, thay vì một tin cho mỗi container hay VM. Một bản sao lưu bạn tự bắt đầu vẫn gửi tin nhắn riêng. Một máy khách bị đóng băng, một lần lưu giữ thất bại hoặc một ngân sách tăng trưởng bị vượt sẽ được gửi ngay cả khi bật tóm tắt.
- **Thông báo khi container được cập nhật**: một tin nhắn cho mỗi container mà **Cập nhật container sau khi sao lưu thành công** đã chuyển sang image mới hơn, để bạn kiểm tra nó còn chạy tốt không.
- **Báo cáo bản sao lưu bị trễ**: mỗi ngày một lần lúc 09:00, BombVault kiểm tra xem có bản sao lưu nào đang bật bị trễ không, nghĩa là cũ hơn hai lần chu kỳ lịch của nó, và gửi một tin nhắn cho mỗi trường hợp, nêu tên mục bị tụt lại. Bản sao lưu thành công kế tiếp sẽ xóa cảnh báo.
- **Tóm tắt hằng tuần**: một bản tóm tắt mỗi tuần với số lần chạy, dữ liệu sao lưu mới, những phần nào đang bị trễ, các bản sao ngoài site có còn mới không và những lỗi hàng đầu. Kế hoạch của nó là một nút như mọi kế hoạch khác, và **Tắt** sẽ tắt nó. Xem [Lịch trình](schedules.md#plans).

Kênh có báo cả khi thành công hay không là do chính kênh đó quyết định, ở mục **Thông báo**. Những bất thường nào gửi tin nhắn được đặt trong **Cài đặt, Xác minh**, xem [Bất thường](anomalies.md). Một bản sao lưu bị hoãn gửi một tin nhắn khi nó bắt đầu chờ và một tin khi việc chờ kết thúc, chứ không phải một tin cho mỗi lần thử.

## Kênh {#channels}

**Thêm kênh** ở cuối thẻ **Kênh** có **Thông báo Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **Email (SMTP)**, **Apprise**, **Webhook** và **Healthchecks**. Bạn thêm bao nhiêu tùy thích, kể cả hai kênh cùng loại, chẳng hạn một topic ntfy cho lỗi và một topic khác cho mọi thứ. Một kênh mới bắt đầu ở trạng thái tắt, nên nó được lưu trước khi bạn điền các trường.

Mỗi kênh có:

- công tắc riêng, tắt kênh mà không mất các trường của nó.
- **Thông báo**: **Không bao giờ**, **Chỉ khi thất bại** hoặc **Khi thành công và thất bại**. Một kênh mới bắt đầu với **Chỉ khi thất bại**. Một bản sao lưu thành công nhưng bỏ sót thứ gì đó cũng tới được kênh chỉ báo lỗi.
- **Tên**, không bắt buộc, hiện thay cho loại kênh, tiện khi bạn có hai kênh cùng loại.
- **Gửi thử**, gửi một tin nhắn thử qua kênh này với các giá trị đúng như trong biểu mẫu, dù đã lưu hay chưa. Dòng bên dưới kênh cho biết dịch vụ có nhận hay không, và nếu không thì vì sao.
- **Xóa kênh**, biểu tượng thùng rác bên cạnh.

Mỗi kênh gửi độc lập với giới hạn thời gian riêng, nên một máy chủ không bao giờ trả lời sẽ không giữ chân các kênh khác. Khi một dịch vụ lỗi theo kiểu mà thử lại có thể khắc phục, kênh sẽ thử tối đa ba lần. Một cảnh báo trễ hạn, vượt ngân sách tăng trưởng hoặc bất thường mà không kênh nào nhận sẽ được thử lại sau, và bản tóm tắt hằng tuần cũng vậy.

| Kênh | Các trường | Ghi chú |
|---|---|---|
| **Thông báo Unraid** | không có | Gửi tới hệ thống thông báo riêng của Unraid, nơi có thể chuyển tiếp tới Pushover, email hoặc Discord. Nó chạy qua kết nối SSH từ **Cài đặt, Tích hợp, SSH máy chủ**, nên khóa đó phải được cấp quyền. Trên TrueNAS và các máy chủ khác, nó im lặng, và kênh cho biết lý do. |
| **ntfy** | **URL chủ đề**, **Token truy cập (tùy chọn)** | Địa chỉ kết thúc bằng topic của bạn. Chỉ cần token cho topic được bảo vệ. |
| **Gotify** | **URL máy chủ**, **Token ứng dụng**, **Mức ưu tiên khi thất bại**, **Mức ưu tiên khi thành công** | Mức ưu tiên từ 0 đến 10 và bắt đầu ở 8 cho thất bại và 5 cho thành công. |
| **Discord** | **URL Webhook** | Địa chỉ của một webhook lấy từ phần cài đặt của kênh Discord. |
| **Slack** | **URL Webhook** | Địa chỉ của một incoming webhook trong ứng dụng Slack của bạn. |
| **Matrix** | **URL Homeserver**, **Mã truy cập**, **ID phòng** | Tài khoản đứng sau mã phải ở trong phòng. |
| **Email (SMTP)** | **Máy chủ SMTP**, **Cổng**, **Mã hóa**, **Tên người dùng**, **Mật khẩu**, **Địa chỉ gửi**, **Địa chỉ nhận** | **Mã hóa** là **STARTTLS**, **TLS (implicit)** hoặc **None**. |
| **Apprise** | **URL API Apprise**, **Thẻ (tùy chọn)**, **Địa chỉ Apprise (tùy chọn)** | Trỏ nó tới endpoint `/notify/<key>` của container Apprise API để tới hơn 100 dịch vụ của nó, và các thẻ chọn một tập con đích của khóa đó. Hoặc liệt kê các địa chỉ Apprise, mỗi dòng một địa chỉ, chẳng hạn `tgram://` cho Telegram hoặc `pover://` cho Pushover, và để URL API kết thúc bằng `/notify`, không có khóa. |
| **Webhook** | **URL Webhook**, **Phương thức**, **Tiêu đề HTTP**, **Nội dung** | Một yêu cầu do bạn tự định hình, xem [Webhook](#webhook). |
| **Healthchecks** | **URL ping** | Xem [Healthchecks](#healthchecks). |

Các bí mật như các token, mật khẩu SMTP và giá trị của tiêu đề webhook được lưu đã mã hóa, và máy chủ không bao giờ trả chúng về cho trang. Dưới một bí mật đã lưu, ô hiện **Đã lưu. Để trống ô này để giữ nguyên.** Khi bạn đổi nơi một kênh gửi tới, BombVault không gửi bí mật cũ tới nơi mới: ô hiện **Địa chỉ mới được lưu khi bạn nhập lại mục này.**, và địa chỉ mới được lưu khi bạn nhập nó.

Cài đặt từ một phiên bản có trước danh sách kênh được chuyển sang ở lần khởi động đầu tiên, và không mất gì cả. Mỗi kênh bạn đã điền trở thành một mục riêng với lựa chọn **Thông báo** bạn đã có. Một webhook đặt định dạng Discord, Slack, Gotify hoặc ntfy trở thành một kênh loại đó, còn một webhook Generic JSON tiếp tục gửi `title`, `message` và `ok` qua **Nội dung** của nó. Công tắc Unraid trở thành một kênh Unraid, còn URL Healthchecks cùng các ngoại lệ của nó trở thành một kênh Healthchecks.

### Webhook {#webhook}

Một kênh **Webhook** gửi một yêu cầu HTTP tới bất kỳ địa chỉ nào. **Phương thức** là GET, POST hoặc PUT. **Thêm tiêu đề** thêm một dòng dưới **Tiêu đề HTTP**, và giá trị của tiêu đề được giữ như bí mật, nên để một tiêu đề `Authorization` ở đó là an toàn. **Nội dung** được gửi đúng như bạn viết, với các biến giữ chỗ sau được điền vào:

| Biến giữ chỗ | Trở thành |
|---|---|
| `%%title%%` | tiêu đề của tin nhắn |
| `%%body%%` | chính nội dung tin nhắn |
| `%%ok%%` | `true`, hoặc `false` khi thất bại |
| `%%event%%` | điều đã xảy ra, chẳng hạn `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` hoặc `critical` |
| `%%importance%%` | mức của Unraid cho nó: `normal`, `warning` hoặc `alert` |
| `%%link%%` | một liên kết đi kèm tin nhắn, để trống khi không có |
| `%%key%%` | phần, chẳng hạn `container` hoặc `VM` |
| `%%time%%` | thời gian theo UTC, chẳng hạn `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Biến giữ chỗ cũng dùng được trong địa chỉ và trong giá trị tiêu đề, và mỗi giá trị được escape cho phù hợp với chỗ nó đứng: theo JSON khi tiêu đề `Content-Type` ghi JSON, và theo tham số truy vấn trong địa chỉ. Một webhook mới bắt đầu với tiêu đề đó và gửi `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Một tên BombVault không biết sẽ giữ nguyên như đã viết, nên lỗi gõ sẽ lộ ra trong tin nhắn. Để trống **Nội dung** nếu không muốn gửi nội dung nào.

## Healthchecks {#healthchecks}

Một dịch vụ như [Healthchecks.io](https://healthchecks.io) báo động khi BombVault ngừng báo còn sống. Thêm một kênh **Healthchecks** và dán **URL ping** của kiểm tra, và BombVault ping nó trong suốt vòng đời của mọi bản sao lưu: lúc bắt đầu, rồi khi thành công hoặc với `/fail`. Kênh này không có lựa chọn **Thông báo**, vì một kiểm tra cần các ping thành công để giữ màu xanh. Một phần không có nơi nào để sao lưu tới được tính là thất bại, nên kiểm tra của nó chuyển đỏ thay vì nhận tín hiệu thành công.

Các phần dùng cùng một URL chia sẻ một kiểm tra. Khi bản sao lưu của một phần đã thất bại, các bản sao lưu tốt của những phần khác cũng ping `/fail` và nêu tên phần đó, nên kiểm tra vẫn đỏ cho đến khi mục đó có lại một bản sao lưu tốt hoặc bạn xác nhận lỗi dưới **Cần chú ý**.

Một phần có thể ping một địa chỉ riêng: **Ngoại lệ** trong kênh Healthchecks thêm một địa chỉ, và dòng khi đó ghi **Mọi phần khác ping URL này.** **Gửi thử** ghi một mục vào nhật ký của từng kiểm tra, và trạng thái của kiểm tra vẫn giữ nguyên.

## Ví dụ: Gotify trên điện thoại của bạn {#example-gotify}

Bạn chạy Gotify tại `https://gotify.example.com` và muốn lỗi thì reo trên điện thoại còn thành công đến lặng lẽ.

1. Trong Gotify, mở **Apps**, tạo một ứng dụng tên `BombVault` và sao chép token của nó.
2. Trong BombVault, mở **Cài đặt, Thông báo** và bật **Tóm tắt các lần chạy theo lịch**, để một đêm 45 bản sao lưu gửi một tin nhắn thay vì 45.
3. Trong **Kênh**, bấm **Thêm kênh** và chọn **Gotify**.
4. Nhập `https://gotify.example.com` làm **URL máy chủ**. BombVault tự thêm `/message`.
5. Dán token vào **Token ứng dụng**.
6. Đặt **Thông báo** thành **Khi thành công và thất bại**. Để **Mức ưu tiên khi thất bại** là 8 và **Mức ưu tiên khi thành công** là 5. Ứng dụng Android của Gotify coi từ 8 trở lên là cao, nên chỉ lỗi mới phát tiếng.
7. Bật kênh và bấm **Gửi thử**. Dòng bên dưới kênh cho biết Gotify có nhận hay không, và một token bị từ chối sẽ được nêu đúng như vậy.

Lần chạy theo lịch tiếp theo gửi một bản tóm tắt ở mức ưu tiên 5, hoặc 8 khi có gì trong đó bị lỗi.

## Ví dụ: ntfy không cần tài khoản {#example-ntfy}

ntfy không cần tài khoản cho một topic công khai. Hãy chọn tên topic khó đoán, vì ai biết nó cũng đọc được.

1. Trong **Kênh**, bấm **Thêm kênh** và chọn **ntfy**.
2. Nhập địa chỉ của topic làm **URL chủ đề**, chẳng hạn `https://ntfy.sh/bombvault-tower-7f3k`, hoặc một topic trên máy chủ ntfy của riêng bạn. Để trống **Token truy cập (tùy chọn)** với topic công khai.
3. Đăng ký cùng topic đó trong ứng dụng ntfy, bật kênh và bấm **Gửi thử**.

BombVault gửi tin nhắn làm phần thân, còn tiêu đề của tin nhắn nằm trong header `Title`. Một lỗi được gửi với mức ưu tiên cao và thẻ `warning`, nên nó nổi bật trong ứng dụng, còn một VM vẫn bị đóng băng được gửi ở mức khẩn cấp.

## Khi tin nhắn không đến {#troubleshooting}

Xem [Thông báo không đến](troubleshooting.md#notifications-do-not-arrive) trong Xử lý sự cố.
