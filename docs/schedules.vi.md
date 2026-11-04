# Lịch trình

**Cài đặt, Lịch trình** chứa mọi kế hoạch: bản sao lưu đầy đủ bao phủ mọi thứ, kế hoạch riêng cho một phần, các bản sao ngoài site, mục nào tham gia và theo thứ tự nào. Một mục đơn lẻ có thể có kế hoạch riêng trên trang của nó. Trang này giải thích cách chúng khớp với nhau và đi qua một cách thiết lập điển hình.

## Kế hoạch đọc như một câu {#plans}

Mỗi kế hoạch là một nút đọc như một câu, chẳng hạn *hằng ngày lúc 03:00* hoặc *mỗi 2 ngày lúc 01:30*. Bấm vào để mở nó với năm chế độ:

| Chế độ | Những gì bạn đặt |
|---|---|
| **Tắt** | Không có gì chạy theo kế hoạch này. |
| **Hằng ngày** | **Giờ** |
| **Hằng tuần** | **Ngày** và **Giờ** |
| **Mỗi N ngày** | **Mỗi** bao nhiêu **ngày**, và **Giờ**. Nó đếm từ lần chạy theo lịch gần nhất, nên việc sao lưu một mục bằng tay không làm lùi kế hoạch. |
| **Cron** | Một **Biểu thức** gồm năm trường, chẳng hạn `0 */6 * * *` cho mỗi 6 giờ. **Ví dụ** đưa ra vài mẫu, và cửa sổ hiện các lần chạy tiếp theo. |

Mọi giờ đều theo múi giờ của máy chủ. Trên Unraid đó là múi giờ của chính hệ thống. Ở nơi khác, hãy đặt `TZ`, nếu không mọi kế hoạch chạy theo UTC, xem [Biến môi trường](configuration.md#environment-variables).

## Tuần {#week}

Thẻ **Tuần** vẽ mọi lần chạy đã lên kế hoạch từ thứ Hai đến Chủ nhật, mỗi ngày một dòng: sao lưu, sao chép ngoài site và kiểm tra. Trỏ vào một nét để đọc cái gì bắt đầu ở đó. Ở chỗ ba kế hoạch bắt đầu trong vòng hai giờ, khoảng đó được đánh dấu, để bạn có thể dàn chúng ra.

![Cài đặt, Lịch trình: dải Tuần ở trên cùng, rồi bản sao lưu đầy đủ và các kế hoạch theo phần.](assets/screenshots/schedule/week-dark.png#only-dark)
![Cài đặt, Lịch trình: dải Tuần ở trên cùng, rồi bản sao lưu đầy đủ và các kế hoạch theo phần.](assets/screenshots/schedule/week-light.png#only-light)

## Sao lưu toàn bộ {#full-backup}

**Sao lưu toàn bộ** là kế hoạch mặc định cho mọi thứ. **Khi nào sao lưu chạy** đặt kế hoạch đó: một lượt sao lưu lần lượt từng phần, containers, VMs, flash, thư mục, tập dữ liệu ZFS và tự sao lưu ở cuối. Dòng bên dưới cho biết nó gồm những phần nào. **Tắt** sẽ tắt nó, và khi đó chỉ những gì có kế hoạch riêng mới chạy.

- **Chạy bù các bản sao lưu bị bỏ lỡ sau khi khởi động**: khi máy chủ tắt vào lúc đã định, BombVault chạy bản sao lưu đó khoảng hai phút sau khi nó khởi động.
- **Lệnh trước và sau**: **Lệnh trước khi sao lưu toàn bộ** chạy trên máy chủ với `sh -c` trước khi lượt sao lưu bắt đầu, ví dụ một script dừng một dịch vụ. Nếu nó lỗi, bản sao lưu toàn bộ vẫn chạy. **Lệnh sau khi sao lưu toàn bộ** chạy một lần sau khi mọi phần đã được thử, kể cả khi một trong số đó lỗi, thích hợp cho một ping xác nhận cả máy chủ đã được bảo vệ. Lệnh cho từng container nằm trên trang của nó.

**Sao lưu tất cả ngay** trên trang Bản sao lưu bắt đầu cùng lượt đó bằng tay, trên mọi phần đang bật.

## Lịch trình theo phần {#plans-per-section}

Một phần có thể rời khỏi bản sao lưu toàn bộ và chạy vào giờ riêng. Trong **Lịch trình theo phần**, **Ngoại lệ** thêm một phần với kế hoạch riêng, còn **Xóa ngoại lệ** đưa nó trở lại. Dòng trong thẻ cho biết các phần còn lại làm gì, chẳng hạn **Mọi phần khác đều chạy trong sao lưu toàn bộ.**

Một phần có kế hoạch riêng sẽ không được sao lưu lại trong bản sao lưu toàn bộ, nên không có gì bị sao lưu hai lần. Điều này cũng đúng cho một mục đơn lẻ có kế hoạch riêng.

**Bản sao ngoài site theo phần** hoạt động theo cách tương tự cho các bản sao. Theo mặc định, mỗi phần sao chép tới các đích ngoài site của nó ngay sau mỗi bản sao lưu. **Ngoại lệ** cho một phần một kế hoạch sao chép riêng, ví dụ để các VM chỉ sao chép tới Backblaze B2 vào tối Chủ nhật. Thẻ **Sao chép** của chính một đích sao lưu cũng có thể đặt **Khi nào sao chép**, xem [Bản sao ngoài site](offsite-recovery.md#off-site-copies).

## Mục trong lịch trình {#scheduled-entries}

**Mục trong lịch trình** cho biết mỗi phần có bao nhiêu mục tham gia, chẳng hạn **38 trên 41 đã lên lịch**. **Mục** mở danh sách với công tắc **Trong lịch trình** cho từng mục. Một mục bị tắt chỉ được sao lưu khi bạn tự bắt đầu bằng tay. Nó được tính là chưa được bảo vệ trừ khi bạn đánh dấu **Không sao lưu**.

## Thứ tự {#order}

**Thứ tự** đặt trình tự mà các lần chạy theo lịch và bản sao lưu toàn bộ sao lưu container và VM. Kéo một mục vào vị trí hoặc dùng các mũi tên, và mỗi lần di chuyển được lưu ngay. Các mục bạn để không xếp thứ tự sẽ được sao lưu theo thứ tự trễ nhất trước. Một bản sao lưu đơn lẻ làm bằng tay không bị ảnh hưởng.

**Khởi động lại sau khi sao lưu** trong cùng thẻ áp dụng cho các container mà **Dừng các container khác** đã tắt trong lúc sao lưu. Chúng khởi động lại theo thứ tự phụ thuộc, và với **Chờ các phụ thuộc khỏe mạnh trước khi khởi động cái tiếp theo**, mỗi container phải báo khỏe hoặc đang chạy trước khi cái kế tiếp khởi động. **Thời gian chờ tình trạng theo từng container (giây)** giới hạn thời gian chờ, mặc định 120, trong khoảng từ 5 đến 3600.

## Ảnh chụp thay cho dừng {#snapshot-instead-of-stop}

**Chỉ dừng container trên ZFS để chụp ảnh** được bật theo mặc định. Một container có dữ liệu nằm trên ZFS chỉ bị dừng trong lúc BombVault chụp ảnh dữ liệu đó, thường là vài giây, và bản sao lưu sau đó được đọc từ ảnh chụp. Nó cần kết nối SSH tới máy chủ mà các tập dữ liệu ZFS dùng. Một container có một phần dữ liệu ở nơi khác thì vẫn bị dừng suốt bản sao lưu như trước, và lần chạy của nó cho biết lý do, còn container không có dữ liệu nào trên ZFS thì bị dừng mà không có ghi chú. Nó hoạt động trên một user share của Unraid nằm trên một pool ZFS duy nhất. Trên TrueNAS, ứng dụng cũng cần khả năng `SYS_CHROOT`, xem [TrueNAS SCALE](getting-started.md#truenas). Một container đơn lẻ có thể được đặt khác đi trên trang của nó.

Bản sao lưu nằm dưới cùng các đường dẫn và thẻ như bản được tạo khi container bị dừng, nên việc khôi phục và kiểm tra đối xử với cả hai như nhau. Lệnh trước khi sao lưu và bản dump cơ sở dữ liệu vẫn chạy trước, các container khác cần dừng sẽ tắt và bật lại cùng với nó, và lệnh sau khi sao lưu chạy ngay khi container lên lại. Một lần cập nhật sau khi sao lưu vẫn chờ đến khi bản sao lưu xong. Các lần chạy của nó hiện **Ảnh chụp** và số giây container ngừng hoạt động. Nếu BombVault dừng giữa chừng một bản sao lưu container nào đó, nó sẽ khởi động lại các container mà nó đã giữ tắt vào lần khởi động tiếp theo.

## Cập nhật sau khi sao lưu {#updates}

- **Cập nhật container sau khi sao lưu thành công** chỉ kéo một image mới hơn sau khi bản sao lưu trước đó đã xong, nên luôn có một điểm khôi phục mới trước. Tùy chọn này tắt theo mặc định, và một container đơn lẻ có thể được đặt khác đi trên trang của nó ở **Cập nhật sau khi sao lưu thành công**.
- **Xóa image cũ sau khi cập nhật** xóa image mà container đã bỏ lại. Một image nền mà các container khác dùng chung thì không bao giờ bị xóa.
- **Làm mới trạng thái cập nhật của Unraid sau khi cập nhật một container** xóa biểu ngữ cập nhật của Unraid cho container đó.

Tin nhắn cho từng container được cập nhật được đặt trong **Cài đặt, Thông báo** với **Thông báo khi container được cập nhật**.

## Rảnh trước khi sao lưu {#idle}

Thẻ **Rảnh trước khi sao lưu** đặt khi nào một ứng dụng được coi là rảnh đối với container có bản sao lưu theo lịch đang chờ nó: **CPU dưới (% một lõi)**, **Lưu lượng dưới (Mbit/s)** và **Trong ít nhất (phút)**. Một máy chủ media được coi là rảnh khi nó không phát, và với container trên mạng của máy chủ thì chỉ CPU được tính. Việc chờ được đặt trên từng container, xem [Khi ứng dụng đang bận](backups.md#app-busy).

## Kế hoạch riêng cho một mục {#own-plan}

Mỗi mục có **Kế hoạch riêng** trong thẻ **Lịch trình và đích sao lưu** của tab Cài đặt. Khi có kế hoạch ở đó, mục chạy vào giờ riêng và rời khỏi bản sao lưu toàn bộ cùng kế hoạch của phần. Khi không có, nó đi theo chúng.

## Ví dụ: ba kế hoạch khớp với nhau {#example}

Một máy Unraid điển hình: phần lớn container có thể sao lưu vào ban đêm, các VM lớn và thay đổi chậm, còn Immich nhận ảnh mới suốt cả ngày.

1. **Sao lưu toàn bộ lúc 03:00.** Mở **Cài đặt, Lịch trình**. Trong **Sao lưu toàn bộ**, đặt **Khi nào sao lưu chạy** thành **Hằng ngày** lúc `03:00`. Bật **Chạy bù các bản sao lưu bị bỏ lỡ sau khi khởi động**, để một đêm máy chủ tắt vẫn được bù lại.
2. **VM mỗi tuần một lần.** Trong **Lịch trình theo phần**, bấm **Ngoại lệ**, chọn **VMs** và đặt kế hoạch của nó thành **Hằng tuần**, **Ngày** Chủ nhật, **Giờ** `04:00`. Thẻ bây giờ cho biết mọi phần khác đều chạy trong sao lưu toàn bộ.
3. **VM cũng sao chép ra ngoài site vào Chủ nhật.** Trong **Bản sao ngoài site theo phần**, bấm **Ngoại lệ**, chọn **VMs** và đặt **Hằng tuần**, Chủ nhật lúc `06:00`, sau khi bản sao lưu của chúng đã có thời gian hoàn tất.
4. **Immich mỗi 6 giờ.** Mở **Bản sao lưu**, rồi `immich`, rồi tab **Cài đặt** của nó. Đặt **Kế hoạch riêng** thành **Cron** với `0 */6 * * *`.

Bây giờ những gì chạy:

| Khi nào | Cái gì |
|---|---|
| Mỗi ngày lúc 03:00 | Containers trừ `immich`, flash, thư mục, tập dữ liệu ZFS và tự sao lưu, lần lượt từng phần |
| Chủ nhật lúc 04:00 | VMs |
| Chủ nhật lúc 06:00 | Bản sao ngoài site của các VM |
| 00:00, 06:00, 12:00 và 18:00 | `immich` riêng |

Thẻ **Tuần** hiện dòng 03:00 vào mỗi ngày, các nét 04:00 và 06:00 vào Chủ nhật, và bốn nét mỗi ngày cho `immich`. `immich` không bị sao lưu lần thứ hai lúc 03:00, vì một mục có kế hoạch riêng sẽ rời khỏi bản sao lưu toàn bộ. Các bản sao của những phần khác vẫn chạy ngay sau mỗi bản sao lưu.

Khi sau này bạn xóa kế hoạch riêng của `immich`, nó lại tham gia lượt 03:00 cùng phần của nó.
