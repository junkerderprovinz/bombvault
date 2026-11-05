# Thông báo

BombVault có thể báo cho bạn khi một bản sao lưu thất bại, khi nó thành công, khi một bản bị trễ, và mỗi tuần một lần về tình hình chung. Mọi thứ được đặt trong **Cài đặt, Thông báo**: điều gì kích hoạt tin nhắn trong thẻ **Thông báo**, tin nhắn đi đâu trong thẻ **Kênh**, và các ping báo còn sống trong thẻ **Healthchecks**.

![Cài đặt, Thông báo với kênh Gotify đã điền xong.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Cài đặt, Thông báo với kênh Gotify đã điền xong.](assets/screenshots/notifications/gotify-light.png#only-light)

## Điều gì gửi tin nhắn {#what}

Thẻ **Thông báo** đặt quy tắc cho mọi kênh cùng lúc:

- **Thông báo**: **Không bao giờ**, **Chỉ khi thất bại** hoặc **Khi thành công và thất bại**, cho mọi kênh bên dưới. Một bản cài mới bắt đầu với **Chỉ khi thất bại**, nên một kênh bạn điền vào sẽ báo cho bạn khi một bản sao lưu bị hỏng.
- **Tóm tắt các lần chạy theo lịch**: một tin nhắn cho mỗi lần chạy theo lịch, chẳng hạn *42 trên 45 thành công*, thay vì một tin cho mỗi container hay VM. Một bản sao lưu bạn tự bắt đầu vẫn gửi tin nhắn riêng. Một máy khách bị đóng băng, một lần lưu giữ thất bại hoặc một ngân sách tăng trưởng bị vượt sẽ được gửi ngay cả khi bật tóm tắt.
- **Thông báo khi container được cập nhật**: một tin nhắn cho mỗi container mà **Cập nhật container sau khi sao lưu thành công** đã chuyển sang image mới hơn, để bạn kiểm tra nó còn chạy tốt không.
- **Báo cáo bản sao lưu bị trễ**: mỗi ngày một lần lúc 09:00, BombVault kiểm tra xem có bản sao lưu nào đang bật bị trễ không, nghĩa là cũ hơn hai lần chu kỳ lịch của nó, và gửi một tin nhắn cho mỗi trường hợp, nêu tên mục bị tụt lại. Bản sao lưu thành công kế tiếp sẽ xóa cảnh báo.
- **Thông báo Unraid**: gửi tới hệ thống thông báo riêng của Unraid, nơi có thể chuyển tiếp tới Pushover, email hoặc Discord. Nó chạy qua kết nối SSH từ **Cài đặt, Tích hợp, SSH máy chủ**, nên khóa đó phải được cấp quyền. Trên TrueNAS và các máy chủ khác, nó vẫn tắt.
- **Tóm tắt hằng tuần**: một bản tóm tắt mỗi tuần với số lần chạy, dữ liệu sao lưu mới, những phần nào đang bị trễ, các bản sao ngoài site có còn mới không và những lỗi hàng đầu. Kế hoạch của nó là một nút như mọi kế hoạch khác, và **Tắt** sẽ tắt nó. Xem [Lịch trình](schedules.md#plans).

Những bất thường nào gửi tin nhắn được đặt trong **Cài đặt, Xác minh**, xem [Bất thường](anomalies.md). Một bản sao lưu bị hoãn gửi một tin nhắn khi nó bắt đầu chờ và một tin khi việc chờ kết thúc, chứ không phải một tin cho mỗi lần thử.

**Gửi thử** ở cuối thẻ gửi một tin nhắn thử qua mọi kênh đang bật, với các giá trị đúng như trong biểu mẫu, và nêu tên từng kênh bị lỗi. Một kiểm tra Healthchecks sẽ có thêm một mục trong nhật ký của nó, và trạng thái của kiểm tra vẫn giữ nguyên.

## Kênh {#channels}

Mỗi kênh có công tắc riêng trong thẻ **Kênh**, và các trường của nó xuất hiện khi bật. Mỗi kênh gửi độc lập với giới hạn thời gian riêng, nên một máy chủ không bao giờ trả lời sẽ không giữ chân các kênh khác. Một cảnh báo trễ hạn, vượt ngân sách tăng trưởng hoặc bất thường mà không kênh nào nhận sẽ được thử lại, và bản tóm tắt hằng tuần cũng vậy. Tin nhắn của một bản sao lưu đơn lẻ hoặc một bản tóm tắt lần chạy chỉ được gửi một lần.

| Kênh | Các trường | Ghi chú |
|---|---|---|
| **Webhook** | **URL Webhook**, **Định dạng Webhook** | Các định dạng: **Generic JSON**, **Discord**, **Slack**, **Gotify** và **ntfy**. Generic JSON gửi `title`, `message` và `ok`. |
| **Apprise** | **URL API Apprise**, **Thẻ (tùy chọn)** | Trỏ nó tới endpoint `/notify/<key>` của container Apprise API để tới hơn 100 dịch vụ của nó. Các thẻ chọn một tập con đích của khóa đó. |
| **Matrix** | **URL Homeserver**, **Mã truy cập**, **ID phòng** | Mã nằm trên máy chủ và không bao giờ hiện lại. |
| **Gotify** | **URL máy chủ**, **Token ứng dụng**, **Mức ưu tiên khi thất bại**, **Mức ưu tiên khi thành công** | Mức ưu tiên từ 0 đến 10 và bắt đầu ở 8 cho thất bại và 5 cho thành công. |
| **Email (SMTP)** | **Máy chủ SMTP**, **Cổng**, **Mã hóa**, **Tên người dùng**, **Mật khẩu**, **Địa chỉ gửi**, **Địa chỉ nhận** | **Mã hóa** là **STARTTLS**, **TLS (implicit)** hoặc **Không**. |

Các bí mật như mã Matrix, token ứng dụng Gotify và mật khẩu SMTP được lưu đã mã hóa. Dưới một bí mật đã lưu, ô hiện **Đã lưu. Để trống ô này để giữ nguyên.** Khi bạn đổi địa chỉ máy chủ, BombVault không gửi bí mật cũ tới nơi mới: ô sẽ hỏi lại, và địa chỉ mới được lưu khi bạn nhập nó.

## Healthchecks {#healthchecks}

Một dịch vụ như [Healthchecks.io](https://healthchecks.io) báo động khi BombVault ngừng báo còn sống. Dán **URL ping** của kiểm tra, và BombVault ping nó trong suốt vòng đời của mọi bản sao lưu: lúc bắt đầu, rồi khi thành công hoặc với `/fail`. Nó làm vậy bất cứ khi nào có URL, bất kể **Thông báo** đặt gì, nên kiểm tra vẫn xanh khi thành công ngay cả với tin nhắn chỉ khi thất bại. Một phần không có nơi nào để sao lưu tới được tính là thất bại, nên kiểm tra của nó chuyển đỏ thay vì nhận tín hiệu thành công.

Các phần dùng cùng một URL chia sẻ một kiểm tra. Khi bản sao lưu của một phần đã thất bại, các bản sao lưu tốt của những phần khác cũng ping `/fail` và nêu tên phần đó, nên kiểm tra vẫn đỏ cho đến khi mục đó có lại một bản sao lưu tốt hoặc bạn xác nhận lỗi dưới **Cần chú ý**.

Một phần có thể ping một địa chỉ riêng: **Ngoại lệ** trong thẻ thêm một địa chỉ, và dòng ghi **Mọi phần khác ping URL này.**

## Ví dụ: Gotify trên điện thoại của bạn {#example-gotify}

Bạn chạy Gotify tại `https://gotify.example.com` và muốn lỗi thì reo trên điện thoại còn thành công đến lặng lẽ.

1. Trong Gotify, mở **Apps**, tạo một ứng dụng tên `BombVault` và sao chép token của nó.
2. Trong BombVault, mở **Cài đặt, Thông báo**. Trong **Thông báo**, đặt **Thông báo** thành **Khi thành công và thất bại** và bật **Tóm tắt các lần chạy theo lịch**, để một đêm 45 bản sao lưu gửi một tin nhắn thay vì 45.
3. Trong **Kênh**, bật **Gotify**.
4. Nhập `https://gotify.example.com` làm **URL máy chủ**. BombVault tự thêm `/message`.
5. Dán token vào **Token ứng dụng**.
6. Để **Mức ưu tiên khi thất bại** là 8 và **Mức ưu tiên khi thành công** là 5. Ứng dụng Android của Gotify coi từ 8 trở lên là cao, nên chỉ lỗi mới phát tiếng.
7. Bấm **Gửi thử**. Dòng bên dưới thẻ cho biết Gotify có nhận hay không, và một token bị từ chối sẽ được nêu đúng như vậy.

Lần chạy theo lịch tiếp theo gửi một bản tóm tắt ở mức ưu tiên 5, hoặc 8 khi có gì trong đó bị lỗi.

## Ví dụ: ntfy làm webhook {#example-ntfy}

ntfy không cần tài khoản cho một topic công khai. Hãy chọn tên topic khó đoán, vì ai biết nó cũng đọc được.

1. Trong **Kênh**, bật **Webhook**.
2. Nhập địa chỉ của topic làm **URL Webhook**, chẳng hạn `https://ntfy.sh/bombvault-tower-7f3k`, hoặc địa chỉ máy chủ ntfy của riêng bạn.
3. Đặt **Định dạng Webhook** thành **ntfy**.
4. Đăng ký cùng topic đó trong ứng dụng ntfy và bấm **Gửi thử**.

BombVault gửi tin nhắn làm phần thân với tiêu đề trong `X-Title`. Một lỗi được gửi với mức ưu tiên cao và thẻ `warning`, nên nó nổi bật trong ứng dụng.

## Khi tin nhắn không đến {#troubleshooting}

Xem [Thông báo không đến](troubleshooting.md#notifications-do-not-arrive) trong Xử lý sự cố.
