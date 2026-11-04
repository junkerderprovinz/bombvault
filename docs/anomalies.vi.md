# Bất thường

Ransomware và một ứng dụng hỏng trông giống nhau trong một bản sao lưu: đột nhiên phần lớn dữ liệu là mới, hoặc phần lớn đã biến mất. BombVault so sánh mọi bản sao lưu với lịch sử riêng của mục đó và đưa ra một phát hiện khi có điều gì đó trông không ổn. Khi một phát hiện về dữ liệu bị mất còn đang mở, nó cũng giữ lại các bản sao lưu cũ của mục đó, để bản sao lưu tốt gần nhất không bị hết hạn. Mục **Bất thường** trong thanh bên hiện khi tính năng phát hiện đang bật.

![Trang Bất thường với một phát hiện nghiêm trọng đang mở.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Trang Bất thường với một phát hiện nghiêm trọng đang mở.](assets/screenshots/anomalies/page-light.png#only-light)

## Trang này {#page}

Các ô **Nghiêm trọng**, **Cảnh báo** và **Ghi chú** chọn một mức độ, còn **Đang mở** và **Đã đóng** phía trên danh sách chuyển giữa những gì đang mở và những gì đã đóng trong 30 ngày qua. Một phát hiện nghiêm trọng nằm trên cùng một bề mặt với các phát hiện khác, và mức độ của nó hiện ở huy hiệu cùng viền màu. Mỗi phát hiện đưa ra cách khắc phục trước, và phần chi tiết cho thấy giá trị dưới dạng một đường cong trên khoảng giá trị thông thường.

Khi không có gì đang mở, **Tiến trình học** hiện một vòng tròn mười phần cho mỗi mục, mỗi phần ứng với một bản sao lưu mà BombVault đã học từ đó. **Giám sát** mở danh sách các mục cùng cài đặt riêng của chúng.

## Những gì được kiểm tra {#checks}

Mọi bản sao lưu của một container, VM, bộ thư mục, bản dump cơ sở dữ liệu, ổ flash và tự sao lưu đều được so sánh với lịch sử riêng của mục đó. Các kiểm tra xem xét:

- dữ liệu mới mà một lần chạy đã lưu, so với những lượng lớn nhất thường thấy của các bản sao lưu gần đây và so với tốc độ thông thường mỗi giờ,
- một bản sao lưu đã lưu lại phần lớn dữ liệu một lần nữa, bao gồm cả các tệp được đổi tên và ghi lại,
- dung lượng nguồn và số tệp mà restic báo cho từng mục và từng bản dump cơ sở dữ liệu,
- thời gian sao lưu của chính restic,
- chuỗi lỗi liên tiếp và lỗi ngắt quãng,
- các kiểm tra khôi phục không còn đạt,
- dung lượng trống của các kho cục bộ, SFTP và rclone, dự báo từ mức tăng của kho.

Một mục học từ 10 bản sao lưu đầu tiên. Nguồn gần như trống, việc ghi lại phần lớn dữ liệu và các lỗi được kiểm tra ngay từ đầu.

Một phát hiện về dung lượng hoặc số tệp của nguồn được gán vào bản sao lưu đầu tiên mà nó xuất hiện. **So sánh với bản sao lưu trước** liệt kê các thư mục có tệp biến mất, mới đến hoặc thay đổi, kèm ghi chú khi gần như toàn bộ nằm trong một chỉ mục tìm kiếm, một bộ nhớ đệm hoặc ảnh thu nhỏ mà ứng dụng tự tạo lại.

Các mục ZFS được theo dõi theo từng dataset, xem [Tập dữ liệu ZFS](zfs-datasets.md#anomalies). Một trợ lý có thể đọc các phát hiện đang mở qua [máy chủ MCP](mcp.md#tools).

Các kiểm tra có giới hạn. Kho S3, B2 và REST không có con số dung lượng trống, trên user share của Unraid dung lượng trống là của cả mảng, và các bản sao lưu tạo trước restic 0.17 không có lịch sử dung lượng.

## Đóng một phát hiện {#closing}

- **Cảnh báo** tự đóng khi nguyên nhân đã hết.
- Các phát hiện **Nghiêm trọng** về dữ liệu bị mất và đĩa đầy vẫn ở đó cho đến khi bạn xử lý.
- **Xác nhận** đánh dấu một phát hiện là đã xem và đóng nó. Nó không được nêu lại cho đến khi nguyên nhân đã biến mất ít nhất một lần.
- **Đánh dấu là đã lường trước** nghĩa là thay đổi đó do bạn. Mức mới trở thành bình thường sau 10 bản sao lưu, nhưng kiểm tra nguồn gần như trống không bao giờ bị tắt.
- Một lựa chọn bị thay đổi, chẳng hạn một thư mục bạn đã chọn hoặc bỏ chọn, tự bắt đầu lại lịch sử của mục đó.

Thông báo được gửi một lần cho mỗi đợt, và các lỗi cùng kiểm tra khôi phục vốn đã có thông báo thì không bị báo hai lần.

## Các bản sao lưu cũ được giữ trong thời gian đó {#retention-hold}

Khi một nguồn gần như trống, đã co lại mạnh hoặc có phần lớn dữ liệu bị lưu lại, chính sách lưu giữ và prune để yên các bản sao lưu cũ của mục đó, và phát hiện liên kết tới bản sao lưu tốt gần nhất, mà cửa sổ khôi phục sẽ chọn sẵn. Trước khi bạn đóng một phát hiện như vậy, BombVault cho biết lần dọn dẹp tiếp theo sẽ xóa các bản sao lưu cũ theo quy tắc giữ lại. Vì vậy nếu dữ liệu bị mất hoặc bị mã hóa, hãy khôi phục trước và đóng phát hiện sau.

## Cài đặt {#settings}

Thẻ **Bất thường** dưới **Cài đặt, Xác minh** chứa các cài đặt chung. Mỗi điều khiển được lưu ngay khi bạn thay đổi, và ba điều khiển dưới công tắc bị ẩn khi tính năng phát hiện đang tắt.

| Cài đặt | Mặc định | Tác dụng |
|---|---|---|
| **Phát hiện bất thường** | Bật | So sánh mọi bản sao lưu với lịch sử riêng của mục. Khi tắt, không còn gì mới được kiểm tra và mục **Bất thường** rời khỏi thanh bên. Thẻ vẫn liên kết tới các phát hiện trước đó. |
| **Độ nhạy** | Cân bằng | Nghiêm báo cả những thay đổi nhỏ hơn, Dễ dãi chỉ báo những thay đổi lớn. |
| **Gửi thông báo cho** | Chỉ các phát hiện nghiêm trọng | Mức độ thấp nhất sẽ gửi tin nhắn qua các kênh được thiết lập trong [Thông báo](notifications.md). Các lỗi sao lưu và dump lặp lại cùng các kiểm tra khôi phục theo lịch bị lỗi vốn đã gửi tin nhắn riêng nên không bị gửi hai lần. |
| **Giữ bản sao lưu cũ khi nguồn co lại mạnh hoặc bị ghi lại** | Bật | Khi một mục có phát hiện đang mở về nguồn gần như trống, co lại mạnh hoặc phần lớn dữ liệu bị lưu lại, chính sách lưu giữ và prune để yên các bản sao lưu cũ của mục đó. Hãy xác nhận phát hiện hoặc đánh dấu là đã lường trước để cho chúng được xóa. |

Mỗi mục có thể dùng độ nhạy và mức thông báo tối thiểu riêng. Đặt chúng ở tab **Xác minh** của mục, hoặc trên trang **Bất thường** dưới **Giám sát**. Với một mục ZFS, chúng áp dụng cho mọi dataset trong cây của nó.
