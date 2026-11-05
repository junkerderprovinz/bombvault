# Tổng quan

Tổng quan là trang đầu tiên của thanh bên và là trang nên xem mỗi sáng. Nó trả lời hai câu hỏi chỉ trong một cái nhìn: mọi thứ đã được sao lưu chưa, và có gì cần bạn xử lý không. Trang bắt đầu bằng phần đầu trạng thái và bốn thẻ, còn các thẻ khác nằm chờ dưới **Tùy chỉnh**.

![Trang tổng quan sau vài ngày: phần đầu trạng thái, Cần chú ý với một cảnh báo, mức bảo vệ của từng phần, các đích sao lưu và lịch sử.](assets/screenshots/overview/home-dark.png#only-dark)
![Trang tổng quan sau vài ngày: phần đầu trạng thái, Cần chú ý với một cảnh báo, mức bảo vệ của từng phần, các đích sao lưu và lịch sử.](assets/screenshots/overview/home-light.png#only-light)

## Phần đầu trạng thái {#status-head}

Phần đầu nằm ở trên cùng của trang và luôn ở đó. Vòng tròn của nó cho biết bao nhiêu thứ đang được bảo vệ, chẳng hạn **41 trên 43 đã được bảo vệ**, và dòng bên cạnh cho biết tình hình hiện tại:

| Phần đầu hiển thị | Ý nghĩa |
|---|---|
| **Mọi thứ đã được sao lưu** | Mọi mục của mọi phần đang bật đều có một bản sao lưu còn mới. |
| **Đã sao lưu, có ghi chú** | Mọi thứ đã được sao lưu, và một số lần chạy để lại ghi chú, chẳng hạn các tệp mà chúng không đọc được. |
| **Chưa phải mọi thứ đều được bảo vệ** | Một số mục chưa có bản sao lưu còn mới, ví dụ vì đang chờ bản đầu tiên. |
| **3 việc cần bạn xử lý** | Có thứ gì đó bị lỗi hoặc đang chờ bạn quyết định. Thẻ **Cần chú ý** liệt kê chúng. |
| **Đang sao lưu** | Một bản sao lưu đang chạy. Vòng tròn chuyển thành vòng tiến trình với các trạm **Dừng**, **Đọc** và **Khởi động**, và một bản sao lưu đầy đủ đếm số mục khi chạy, chẳng hạn **Đang sao lưu: 12 trên 43**. **Xem lần chạy** mở lần chạy đó. |
| **Đang sao chép ra ngoài site** | Một lần sao chép tới một đích sao lưu đang chạy. Nó không hiện phần trăm vì `restic copy` không báo con số này. |

Bên dưới dòng đó là thời điểm lần chạy gần nhất và thời điểm lần chạy theo lịch tiếp theo bắt đầu.

## Cần chú ý {#needs-you}

Thẻ này liệt kê những gì bạn nên xem, mới nhất đứng trước, và hiện **Không có gì cần làm.** khi danh sách trống. Mỗi dòng có cách xử lý riêng:

- **Một bản sao lưu bị lỗi** nêu tên mục và lý do. **Xem lỗi** hiện đầu ra của chính restic, còn **Thử lại** chạy lại bản sao lưu.
- **Sao chép ra ngoài site không thành công** nêu tên phần và từ khi nào. **Sao chép ngay** thử lại.
- **Bất thường ở plex** dẫn đến phát hiện đó bằng **Kiểm tra**. Khi còn một phát hiện nghiêm trọng đang mở, chính sách lưu giữ giữ lại các bản sao lưu cũ của mục đó.
- Các mục **Mới** mà BombVault tìm thấy và chưa có gì sao lưu, chẳng hạn một container bạn vừa cài, đưa ra **Bắt đầu sao lưu** hoặc **Không sao lưu**.
- **Một bản sao lưu đã bỏ cuộc** vì ứng dụng của nó bận suốt thời gian chờ, xem [Khi ứng dụng đang bận](backups.md#app-busy).
- **Một bản sao ZFS bị lỗi**, nói bằng lời đơn giản, chẳng hạn máy chủ không có lệnh zfs, với đầu ra của máy chủ nằm dưới **Xem**.
- **Các bản sao lưu đã dừng: không còn mở được khóa** xuất hiện khi máy chủ này tiếp nhận khóa của một máy chủ khác và `APP_KEY` sau đó bị đổi. Xem [Xử lý sự cố](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Lưu bộ khôi phục của bạn** vẫn còn đó cho đến khi bạn tải bộ khôi phục xuống và bấm **Tôi đã cất giữ an toàn**.

## Trạng thái bảo vệ {#protection-status}

Mỗi phần đang bật có một dòng với bốn kiểm tra:

| Kiểm tra | Chuyển xanh khi | Những cách diễn đạt khác bạn có thể thấy |
|---|---|---|
| **Cục bộ** | bản sao lưu gần nhất còn mới | chưa có bản sao lưu, chưa có lịch, lần chạy gần nhất bị lỗi, bản sao lưu quá hạn, chưa phải mọi mục đều được bảo vệ, sắp đến hạn sao lưu |
| **Ngoài site** | bản sao ra ngoài cơ sở gần nhất còn mới | chưa có bản sao ngoài site, đang sao chép, bản sao quá hạn, đang chờ bạn quyết định, chưa có bản sao, bản sao ZFS bị lỗi |
| **Bảo vệ chống xóa** | một phép thử phá hoại đã chứng minh phía bên kia từ chối xóa | không có, không có và đã chấp nhận rủi ro, chưa được chứng minh, phép thử thất bại, chưa được chứng minh vì kiểm tra của nó đang tắt |
| **Khôi phục thử** | một lần khôi phục từ bản sao ngoài site đã đạt | chưa bao giờ chạy, đang chạy, thất bại, quá hạn, chỉ chạy thủ công, không thể thực hiện với phần này |

Một phần chỉ xanh khi dựa trên sự thật đã được kiểm chứng. Một bản sao trên chính máy chủ này không được tính là ngoài site, và một đích bạn đã đánh dấu **Chấp nhận khi không có bảo vệ chống xóa** hiện một vòng xám thay vì chấm hổ phách, nên màu của phần đó theo các kiểm tra còn lại.

Một phần có lịch sẽ báo **Chưa có bản sao lưu nào** và giữ trung tính cho đến khi lần chạy đầu tiên đến hạn, tính từ lần cuối kế hoạch của nó được đặt. Sau đó nó báo **Quá hạn** và cảnh báo một lần. **Quá hạn** dựa vào mục bị tụt lại xa nhất so với kế hoạch của chính nó, nên một mục bên cạnh vẫn chạy không che đi một mục đã ngừng. Một mục có container đã mất hoặc không lịch nào với tới thì không được tính. Một mục có kế hoạch hằng tuần riêng không bị xét theo kế hoạch hằng ngày của phần. Một phần không có gì để sao lưu, vì không lịch nào với tới các mục của nó, sẽ báo **Chưa lên lịch** và không bao giờ quá hạn.

Bấm vào một phần sẽ mở cửa sổ của nó với từng kiểm tra được giải thích rõ, các mục nó đang đếm và **Bắt đầu khôi phục thử** hoặc **Chạy lại khôi phục thử**. Cửa sổ cũng cho biết **Đã đặt quy tắc dọn bớt** hoặc **Chưa có quy tắc dọn bớt**, và quy tắc giữ của thư mục riêng của một phần cũng được tính như các quy tắc ngoài site của phần đó.

![Cửa sổ bảo vệ của phần Containers với bốn kiểm tra được giải thích rõ.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Cửa sổ bảo vệ của phần Containers với bốn kiểm tra được giải thích rõ.](assets/screenshots/overview/protection-window-light.png#only-light)

## Các đích sao lưu {#storage-locations}

Mỗi đích sao lưu xuất hiện ở đây như một bình chứa với mức đầy, tốc độ tăng, chẳng hạn **Đang tăng 4,2 GB/tuần**, và thời điểm nó sẽ đầy nếu giữ tốc độ đó, chẳng hạn **đầy sau khoảng 31 tuần**. S3, B2 và rest-server không báo còn bao nhiêu chỗ, nên chúng hiện **Không rõ dung lượng trống** và chỉ có tốc độ tăng. Bấm vào để tới trang của đích đó. Xem [Các đích sao lưu](offsite-recovery.md#destinations).

## Lịch sử {#history}

**Lịch sử** có hai tab. **Các lần chạy** liệt kê mọi lần chạy với kết quả, thời lượng và lý do, và bấm vào sẽ mở lần chạy đó. **Nhật ký** hiện từng dòng với thời gian của nó, đúng như BombVault ghi, và có thể thu hẹp theo một ngày. Một dòng chuyển sang màu cam khi bản sao lưu vẫn xong nhưng bỏ sót thứ gì đó, và dòng đó nêu rõ những gì bị bỏ sót.

## Thêm thẻ trong Tùy chỉnh {#customize}

**Tùy chỉnh** ở đầu trang cho phép bạn sắp xếp các thẻ. Kéo một thẻ ở thanh của nó để di chuyển, đổi kích thước ở chấm tròn ở góc dưới, hoặc ẩn nó đi. Các thẻ đã ẩn nằm chờ dưới **Thẻ đã ẩn**, và **Hiện** đưa một thẻ trở lại. Trong lúc bạn sắp xếp, các thẻ lắc nhẹ, và **Xong** kết thúc việc đó. Bố cục được lưu trong trình duyệt này.

Các thẻ sau bắt đầu ở trạng thái ẩn:

- **Bất thường**: các phát hiện đang mở, xem [Bất thường](anomalies.md).
- **Chưa được sao lưu**: mọi thứ trên máy chủ này mà không có bản sao lưu theo lịch nào bao phủ, kèm lý do, chẳng hạn **chưa bao giờ được thêm**, **không nằm trong lịch trình** hoặc **mọi đích của phần chứa nó đều đang tắt**. Thẻ đọc container thẳng từ Docker, nên nó cũng tìm ra cả container mà BombVault chưa từng thấy.
- **Lịch**: 90 ngày gần nhất, mỗi ngày một ô vuông. Bấm vào một ngày sẽ hiện các dòng của ngày đó trong nhật ký dưới **Lịch sử**.
- **Lưu trữ**: mỗi phần chiếm bao nhiêu chỗ và dữ liệu của nó được khử trùng lặp tốt đến đâu.
- **Tổng quan**: snapshot cũ nhất trong mọi phần, thời điểm bản sao lưu tiếp theo chạy và lần gần nhất diễn ra thế nào.
- **Thống kê**: có bao nhiêu container và VM, bao nhiêu lỗi, và bao nhiêu kế hoạch đang hoạt động hoặc tạm dừng.
- **Tích hợp máy chủ**: những gì BombVault chạm tới và cần trên máy chủ, chẳng hạn socket Docker, libvirt, restic và rclone. Hãy bắt đầu từ đây khi có thứ gì đó có vẻ chưa được nối.

![Chế độ Tùy chỉnh: các thẻ có thể kéo, đổi kích thước và ẩn, còn các thẻ đã ẩn nằm chờ bên dưới.](assets/screenshots/overview/customize-dark.png#only-dark)
![Chế độ Tùy chỉnh: các thẻ có thể kéo, đổi kích thước và ẩn, còn các thẻ đã ẩn nằm chờ bên dưới.](assets/screenshots/overview/customize-light.png#only-light)

## BombVault đếm như thế nào {#counting}

Mọi thẻ và phần đầu đều đếm theo cùng một cách: các mục của những phần đang bật vẫn còn được cài đặt và bạn chưa đánh dấu **Không sao lưu**. Một mục chỉ được tính là đã bảo vệ khi có lịch bao phủ nó và nó có bản sao lưu. Một bản sao lưu cũ đứng một mình thì không được tính, nên một container bạn đã đưa ra khỏi lịch sẽ hiện là chưa được bảo vệ cho đến khi bạn đưa nó trở lại hoặc đánh dấu **Không sao lưu**.

## Bước đầu tiên {#first-steps}

Trên một bản cài mới, tổng quan bắt đầu bằng thẻ **Bước đầu tiên**: **Chọn các mục**, **Thêm đích** và **Bản sao lưu đầu tiên**. Thẻ này ở lại cho đến khi bản sao lưu đầu tiên hoàn tất. [Bắt đầu nhanh](getting-started.md#first-run) hướng dẫn từng bước.
