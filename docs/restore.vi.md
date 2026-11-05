# Khôi phục

Mọi lần khôi phục đều bắt đầu từ trang **Khôi phục**: chọn một phần, tìm thứ bạn muốn lấy lại và chọn một bản sao lưu. Một cửa sổ khôi phục mọi loại mục và hỏi theo từng bước. Theo mặc định, bản gốc không bị thay đổi gì, vì bản sao lưu được đưa về bên cạnh nó.

![Trang Khôi phục với các phần dạng ô, ô tìm kiếm và thẻ Stack.](assets/screenshots/restore/page-dark.png#only-dark)
![Trang Khôi phục với các phần dạng ô, ô tìm kiếm và thẻ Stack.](assets/screenshots/restore/page-light.png#only-light)

## Trang Khôi phục {#page}

- **Phần** liệt kê các phần dưới dạng ô, mỗi ô cho biết có bao nhiêu mục có snapshot.
- **Bạn muốn khôi phục gì?** tìm trong các mục của phần đã chọn. Mỗi dòng hiện snapshot mới nhất của nó, và **Mở mục** dẫn tới trang của mục.
- **Stack** liệt kê các dự án Compose có ít nhất hai container đã được sao lưu, chẳng hạn `immich` cùng cơ sở dữ liệu và Redis của nó. Ô tìm kiếm tìm một stack theo tên hoặc theo bất kỳ container nào của nó.
- **Đang chạy** hiện một lần khôi phục khi nó đang chạy, kể cả sau khi bạn đã đóng cửa sổ của nó.
- **Khẩn cấp** khởi động trình hướng dẫn khẩn cấp cho một máy chủ đã mất hoặc được thiết lập lại, xem [Trình hướng dẫn khẩn cấp](offsite-recovery.md#emergency).
- **Nguồn khác** chứa **Từ thư mục sao lưu cũ**, cho các bản sao lưu của một BombVault khác, và **Nhập từ plugin Appdata.Backup**.

![Trang Khôi phục trên điện thoại.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Trang Khôi phục trên điện thoại.](assets/screenshots/restore/page-phone-light.png#only-light)

## Cửa sổ khôi phục {#window}

Bấm vào một mục sẽ mở **Khôi phục** với tên của nó. Cửa sổ hỏi theo từng bước, và mỗi bước có thể đổi trước khi bạn bắt đầu:

1. **Bản sao lưu**: snapshot được khuyên dùng đã được chọn sẵn. Đó là snapshot mới nhất, hoặc sau một bất thường đang mở là snapshot cuối cùng trước nó. Bản sao lưu được tạo trước một lần khôi phục trước đó chứa trạng thái mà lần khôi phục ấy đã thay thế, nên nó bị bỏ qua trừ khi không còn bản nào khác. Các snapshot cũ hơn nằm trong danh sách, và **So sánh** cho thấy hai snapshot khác nhau ở đâu.
2. **Nguồn**: nơi các tệp đến từ, khi một snapshot nằm ở nhiều chỗ. **Cục bộ** là nhanh nhất.
3. **Cái gì**: **Tất cả**, hoặc **Từng tập tin**, nơi bạn chọn các tệp và thư mục muốn lấy.
4. **Đến đâu**: đích đến. Các lựa chọn phụ thuộc vào loại mục:

    | Đích | Tác dụng |
    |---|---|
    | **Bên cạnh bản gốc** | Mặc định. Một container trở lại thành container thứ hai tên `<name>-restored` với các cổng riêng, đĩa và NVRAM của một VM nằm trong thư mục tên `<folder>-restored`, và một bộ thư mục trở lại thành `<path>-restored`. Bản gốc không bị thay đổi gì. |
    | **Thay thế bản gốc** | Ghi đè những gì đang có. Với **Sao lưu trạng thái hiện tại trước**, được bật theo mặc định, BombVault sao lưu trạng thái hôm nay trước để bạn có thể quay lại. Nếu bản sao lưu đó lỗi, việc khôi phục không bắt đầu. Điều này cũng áp dụng cho một container hoặc VM đã không còn trên máy chủ: các thư mục mà việc khôi phục ghi đè sẽ được sao lưu riêng. |
    | **Vào một thư mục** | Chỉ các tệp, để duyệt xem. |
    | **Tải xuống dạng ZIP** | Ổ flash, cho một chiếc USB mới. BombVault không bao giờ ghi vào ổ flash đang chạy. |
    | **Vào một tập dữ liệu mới** và **Khôi phục vào chính tập dữ liệu** | Tập dữ liệu ZFS, xem [Tập dữ liệu ZFS](zfs-datasets.md#restore). |

5. **Điều gì xảy ra** nói bằng lời việc khôi phục sẽ làm gì, và một hình minh họa cho thấy đích bạn đã chọn.

Trước khi **Khôi phục** được mở khóa, cửa sổ kiểm tra rằng kho có phản hồi, khóa mở được nó, snapshot có đó và đích còn chỗ. Khi một kiểm tra còn lỗi, nút vẫn bị khóa, và dấu (i) trong nó cho biết kiểm tra nào.

Cửa sổ cũng cho thấy việc khôi phục thay đổi gì so với những gì đang có: các tệp mới, được thay thế và không đổi, với danh sách khi bạn yêu cầu, và các tệp ở đích không có trong bản sao lưu và sẽ được để nguyên. restic tính điều này như một lần chạy thử từ kích thước và thời gian sửa đổi, không đọc các tệp, và một cây rất lớn sẽ dừng sau 30 giây và nói rõ như vậy. Với container và VM, nó so sánh các thiết lập mà việc khôi phục tạo lại với những thiết lập đang chạy: image và tag, cổng, tên biến và volume, hoặc bộ nhớ, vCPU, đĩa và mạng.

Một số điều có thể chặn việc khôi phục trước khi nó thay đổi bất cứ thứ gì. IP tĩnh và các cổng đã công bố của một container phải còn trống, và mạng của nó phải tồn tại. Trên Unraid, mọi lần khôi phục tại chỗ đều từ chối một thư mục không nằm trên pool hoặc share đã gắn kết, vì các tệp sẽ nằm trong bộ nhớ và mất sau khi khởi động lại. Một lần khôi phục tại chỗ nêu tên mọi container khác có bind mount chạm vào một thư mục mà nó ghi vào, chẳng hạn "đường dẫn này cũng được nextcloud-db dùng", nhưng chỉ để cảnh báo.

Một vài tùy chọn xuất hiện ở nơi chúng áp dụng: **Để dừng sau đó**, để xem một container hoặc VM trước khi nó chạy lại, và **Khôi phục không có GPU và runtime** trên một máy chủ thiếu GPU hoặc runtime mà bản sao lưu đã được tạo với.

Việc khôi phục chạy ở chế độ nền. Bạn có thể đóng cửa sổ, và tiến trình cũng hiện dưới **Đang chạy**. Một lần khôi phục dài hiện phần trăm và có thể bị hủy, và một lần khôi phục bị hủy được ghi là đã hủy, không phải thất bại. Một lần khôi phục bị hủy trong lúc container đang dừng, hoặc một lần không gỡ được container, thì chưa khôi phục gì cả, nên container được khởi động lại. Một container lấy volume từ container khác bằng `--volumes-from` sẽ giữ chúng, và việc khôi phục từ chối bắt đầu khi container kia chưa tồn tại.

![Cửa sổ khôi phục ở bước Cái gì, với các tệp riêng lẻ đã được chọn.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Cửa sổ khôi phục ở bước Cái gì, với các tệp riêng lẻ đã được chọn.](assets/screenshots/restore/window-what-light.png#only-light)

### Một lần khôi phục vào thư mục đi đâu {#folders}

**Thư mục khôi phục** trong **Cài đặt, Khôi phục** đặt **Thư mục khôi phục mặc định**, kèm **Thư mục con cho mỗi phần** nếu bạn muốn, và một **Ngoại lệ** cho một phần. Các tệp đã chọn nằm thẳng trong thư mục đích: một tệp trở lại chỉ là tệp đó, một thư mục trở lại là thư mục cùng nội dung của nó, và các lựa chọn từ nhiều thư mục khác nhau chỉ giữ đường dẫn bên dưới thư mục mà chúng chung. BombVault từ chối một đích không nằm trên bộ nhớ đã gắn, vì trên Unraid một thư mục như vậy nằm trong bộ nhớ RAM và biến mất sau khi khởi động lại, và nó không bao giờ khôi phục vào thư mục dữ liệu của chính nó.

## Ví dụ: lấy lại một tệp {#example-single-file}

Ai đó đã sửa `config.php` của Nextcloud đêm qua và Nextcloud không còn khởi động được. Bản sao lưu hằng đêm lúc 03:00 vẫn còn tệp tốt. Bạn muốn đúng tệp đó, không phải cả container.

1. Mở **Khôi phục**, chọn ô **Containers** và gõ `nextcloud` vào **Bạn muốn khôi phục gì?**.
2. Bấm `nextcloud`. Cửa sổ mở ra với bản sao lưu 03:00 được chọn sẵn làm bản khuyên dùng.
3. Để **Nguồn** là **Cục bộ**.
4. Ở **Cái gì**, chọn **Từng tập tin**. Mở các thư mục xuống tới `config` và đánh dấu `config.php`.
5. Ở **Đến đâu**, chọn **Vào một thư mục** và chọn một thư mục, ví dụ `user/restore`, tức là `/mnt/user/restore` trên máy chủ.
6. Đọc **Điều gì xảy ra**: tệp nằm trong thư mục đích, và container không bị thay đổi gì. Bấm **Khôi phục**.

Cửa sổ kết thúc với **Xong: các tệp nằm trong /mnt/user/restore**. Bạn tìm thấy tệp là `/mnt/user/restore/config.php`, không có các thư mục phía trên. Hãy so sánh nó với tệp hỏng, chép nó trở lại thư mục appdata và khởi động lại Nextcloud.

Để đặt tệp trở lại thẳng chỗ cũ, hãy chọn **Thay thế bản gốc** ở bước 5. Giữ **Sao lưu trạng thái hiện tại trước** đang bật, để BombVault sao lưu trạng thái hỏng trước khi ghi, gõ `nextcloud` để xác nhận và bấm **Khôi phục**. Nextcloud bị dừng trong lúc tệp đã chọn được ghi trở lại chỗ của nó và được khởi động lại sau đó, và chỉ những tệp bạn đã đánh dấu mới được ghi. Một lần khôi phục lỗi giữa chừng sẽ để Nextcloud ở trạng thái dừng, vì tệp nó đọc có thể mới ghi được một nửa, và lần chạy sẽ nói rõ điều đó.

![Cửa sổ khôi phục ở bước Điều gì xảy ra, với Thay thế bản gốc đã được chọn.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Cửa sổ khôi phục ở bước Điều gì xảy ra, với Thay thế bản gốc đã được chọn.](assets/screenshots/restore/window-replace-light.png#only-light)

## Những gì trở lại, theo loại {#kinds}

**Container** được tạo lại từ định nghĩa đã lưu qua Docker API, với entrypoint, hostname, thiết lập dừng, volume và mount, giới hạn tài nguyên, log driver, thiết lập DNS và các link cũ. Một container trở lại trên image mà nó chạy lúc bản sao lưu được tạo, không phải trên image mới hơn kéo về sau này, và image còn thiếu sẽ được kéo trước, với thông tin đăng nhập từ **Registry container** trong **Cài đặt, Khôi phục**. GPU hoặc runtime của nó (`--gpus`, `--runtime=nvidia`) cũng trở lại. Trên máy chủ không có driver hoặc runtime đó, việc khôi phục nói rõ và đưa ra **Khôi phục không có GPU và runtime**. Một link tới container đang thiếu, hoặc đã dừng trong lúc container được khôi phục khởi động, sẽ bị bỏ, và lịch sử chạy cho biết như vậy. Một lần khôi phục lỗi sau khi container đã bị gỡ sẽ đặt nó trở lại, ở trạng thái dừng.

Một bản sao lưu có thể chứa những thư mục mà container không còn chọn nữa, chẳng hạn một bản cũ hơn được tạo trước khi một thư mục biến mất. Một thư mục đã mất hoặc đang trống sẽ được đưa trở lại. Một thư mục hiện đang có tệp thì được giữ nguyên, vì có thể bạn đã cố ý bỏ chọn nó, và bước kiểm tra trước khi khôi phục cùng lịch sử chạy sẽ nêu tên nó. Hãy lấy lại nó bằng **Từng tập tin** hoặc **Vào một thư mục**.

**VM** có XML được định nghĩa lại qua SSH và đĩa cùng UEFI NVRAM được gắn lại, kể cả sau khi VM đã bị xóa. Xem [Sao lưu VM](vm-backup-ssh-setup.md) cho các VM có snapshot Unraid và đĩa trên zvol.

**Bộ thư mục** trở lại với quyền sở hữu, chế độ và dấu thời gian như restic lưu. Khi đích chỉ từ chối quyền sở hữu, như `/mnt/user`, lần chạy kết thúc với một ghi chú nói rõ điều đó. Khi đó BombVault so sánh các tệp với bản sao lưu, và một tệp không đến nơi sẽ làm việc khôi phục thất bại.

**Ổ flash** không bao giờ chạm vào `/boot` đang chạy. Việc khôi phục là một bản tải xuống tên `flash-<id>.zip`, sẵn sàng để thả vào Unraid USB Creator. Tệp ZIP chứa `APP_KEY`, nên việc tải nó xuống cần mật khẩu đăng nhập và đăng nhập ngay tại phiên bản. Cửa sổ cũng liệt kê các plugin trong từng bản sao lưu flash với phiên bản và kích thước của chúng, và có thể đặt riêng một plugin trở lại ổ flash đang chạy: tệp `.plg` của nó, thư mục của nó dưới `config/plugins` và các tệp gói mà bản sao lưu có. Unraid cài nó vào lần khởi động tiếp theo, hoặc ngay lập tức dưới **Plugins, Install Plugin**.

**Tập dữ liệu ZFS** có các đích riêng và giữ các thuộc tính ZFS của chúng, xem [Khôi phục](zfs-datasets.md#restore).

**Tự sao lưu** chỉ trở lại qua trình hướng dẫn khẩn cấp, vốn khởi động lại BombVault, nên cơ sở dữ liệu đang chạy không bao giờ bị ghi đè khi đang mở.

**Bản kết xuất cơ sở dữ liệu** không bao giờ tự được nạp lại. Một dòng dưới **Bản kết xuất cơ sở dữ liệu** có thể được tải xuống, lưu vào một thư mục hoặc nhập vào một cơ sở dữ liệu vừa khởi động. Xem [Kết xuất cơ sở dữ liệu](backups.md#database-dumps).

## Khôi phục nhiều mục cùng lúc {#several}

Trên trang Khôi phục, chọn các mục như cách bạn chọn các dòng trong danh sách Bản sao lưu: Ctrl+bấm, Shift+bấm, Ctrl+A, nhấn giữ, hoặc **Chọn**. **Khôi phục** sau đó mở một cửa sổ cho tất cả.

- **Bản sao lưu nào**: **Khuyên dùng** lấy cho mỗi mục bản sao lưu mà cửa sổ riêng của nó sẽ khuyên. **Một thời điểm cho tất cả** lấy cho mỗi mục bản sao lưu mới nhất của nó từ một **Thời điểm** bạn chọn trở về trước, ví dụ buổi tối trước khi có chuyện xảy ra.
- **Đến đâu**: **Bên cạnh bản gốc** hoặc **Thay thế bản gốc**, với **Sao lưu trạng thái hiện tại trước**.

Mỗi mục được kiểm tra như một lần khôi phục đơn lẻ, rồi chúng chạy lần lượt từng cái. Mỗi dòng hiện việc của nó diễn ra thế nào: **Đang chờ**, **Đang chạy**, **Đã khôi phục**, **Thất bại** hoặc **Để ngoài**. **Dừng sau mục này** để lần khôi phục đang chạy hoàn tất và không bắt đầu thêm. Ổ flash đến dưới dạng ZIP để tải xuống, còn các tập dữ liệu ZFS và tự sao lưu được khôi phục từng cái một.

![Khôi phục nhiều mục: mỗi mục một dòng với tiến trình của nó.](assets/screenshots/restore/several-dark.png#only-dark)
![Khôi phục nhiều mục: mỗi mục một dòng với tiến trình của nó.](assets/screenshots/restore/several-light.png#only-light)

## Khôi phục một stack {#stacks}

Bấm vào một stack trong thẻ **Stack** sẽ mở **Khôi phục stack**. Nó khôi phục mọi thành viên từ bản sao lưu mới nhất của chúng cùng một lúc, và với **Khởi động theo thứ tự phụ thuộc sau khi khôi phục**, nó khởi động chúng theo thứ tự `depends_on`: cơ sở dữ liệu và bộ nhớ đệm trước, rồi mới đến ứng dụng. Khi thư mục dự án của một thành viên không có ở nơi bạn đã chọn, cửa sổ đưa ra trạng thái từ Cục bộ thay thế.

Trước tiên BombVault kiểm tra các thành viên và thư mục dự án, và một lần khôi phục không thể thành công sẽ không thay đổi gì. Nếu không, nó dừng các thành viên đang chạy, ứng dụng trước cơ sở dữ liệu của chúng, rồi ghi thư mục dự án, rồi khôi phục các thành viên. Nếu thư mục dự án không trở lại được, các thành viên vẫn được khôi phục nhưng giữ ở trạng thái dừng, vì thư mục có thể mới ghi được một nửa và đó thường là nơi chứa bind mount của một thành viên. Lần chạy của mỗi thành viên sẽ nói rõ điều đó.

Một cơ sở dữ liệu chỉ có bản kết xuất cơ sở dữ liệu mà không có bản sao lưu tệp, và container cùng named volume của nó đã mất, sẽ trở lại mà không có dữ liệu. Bước kiểm tra nói điều đó trước khi khôi phục. Nó vẫn ở trạng thái dừng, các container phụ thuộc vào nó cũng vậy, để không ứng dụng nào chạy bước thiết lập trên một cơ sở dữ liệu trống. Hãy khởi động nó, nhập một bản kết xuất rồi khởi động các container còn lại. Xem [Kết xuất cơ sở dữ liệu](backups.md#database-dumps).

## Nguồn khác {#other-sources}

- **Từ thư mục sao lưu cũ** mở kho của một BombVault khác không còn chạy, trên một đĩa hoặc tại một đích sao lưu ngoài site, bằng bộ khôi phục của nó hoặc `APP_KEY` gõ vào. Phiên này chỉ đọc và chỉ tồn tại trong bộ nhớ. Xem [Từ thư mục sao lưu cũ](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Nhập từ plugin Appdata.Backup** đọc thư mục sao lưu của plugin. Mỗi tệp lưu trữ container trở thành một điểm khôi phục của container đó, có ngày là lúc plugin tạo ra nó. Các tệp lưu trữ đã nhập trước đó sẽ bị bỏ qua, và bản thân các tệp lưu trữ chỉ được đọc. Container cần một bản sao lưu trong BombVault trước, để việc khôi phục có định nghĩa của nó. Chính sách lưu giữ để yên các điểm khôi phục đã nhập, nên hãy tự xóa một điểm khi bạn không còn cần nó.

## Sau khi khôi phục {#after}

Một bản sao được khôi phục bên cạnh bản gốc được liệt kê dưới **Bản sao lưu** là đã loại trừ, nên nó không được tính là một việc đang mở. Hãy thêm nó vào lịch nếu bạn muốn giữ. Một container hoặc VM đang chạy lúc được sao lưu sẽ trở lại ở trạng thái chạy, còn cái đã dừng thì vẫn dừng. Trên Unraid, một container được khôi phục xuất hiện lại trong tab Docker như một ứng dụng bình thường, sửa được, và một VM trong VM Manager với đĩa cùng NVRAM đã được gắn.
