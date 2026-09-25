# Tập dữ liệu ZFS

Trang **ZFS** sao lưu các tập dữ liệu ZFS. Một mục là một tập dữ liệu cùng với mọi tập dữ liệu nằm dưới nó. Với mỗi lần sao lưu, BombVault chụp một ảnh chụp ZFS duy nhất của cả cây, nên mọi tập dữ liệu trong đó được ghi lại ở cùng một thời điểm. Sau đó nó đọc tệp của từng tập dữ liệu từ ảnh chụp đó, lưu bằng restic giống như lưu một thư mục, và xóa ảnh chụp ngay sau đó. Các bản sao lưu được khử trùng lặp, bạn có thể duyệt từng bản, và có thể khôi phục từng tệp riêng lẻ.

BombVault không bao giờ dùng `zfs send` cho tập dữ liệu, không bao giờ quay lui một tập dữ liệu và không bao giờ hủy tập dữ liệu nào.

## Yêu cầu {#requirements}

- **Kết nối SSH tới máy chủ này.** Tập dữ liệu ZFS dùng cùng khóa, máy chủ và người dùng với sao lưu VM. Nếu sao lưu VM đã hoạt động thì phần này cũng hoạt động. Nếu chưa, hãy làm theo [hướng dẫn sao lưu VM qua SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) trên GitHub. Các trường trong mẫu có tên **Host SSH: Address**, **Host SSH: Port** và **Host SSH: User**.
- **Lệnh `zfs` trên máy chủ đó.** Unraid 6.12 trở lên và TrueNAS SCALE đều có.
- **Host Data được ánh xạ thành `/mnt` với Access Mode Read/Write - Slave.** Đây là giá trị mặc định của mẫu. Ảnh chụp của một tập dữ liệu chỉ xuất hiện trong thư mục `.zfs/snapshot` của tập dữ liệu sau khi BombVault đã khởi động, nên container phải nhận được các lần gắn kết mà máy chủ thực hiện về sau.
- **Các tập dữ liệu được gắn kết dưới `/mnt`.** Trên Unraid, pool nằm ở `/mnt/<pool>`, nên điều này đã đúng sẵn.

Bật miền trong **Cài đặt, Chung** (Tập dữ liệu ZFS). Khi đó trang ZFS hiển thị thẻ **Kết nối tới máy chủ này**. Thẻ này kiểm tra kết nối SSH, cho biết người dùng và máy chủ mà nó kết nối tới, và nói còn thiếu gì khi có gì đó thiếu. Mục kiểm tra tích hợp máy chủ (`/spike`) hiển thị cùng kết quả.

## Mục và tập dữ liệu con {#items-and-children}

Mở **Thêm tập dữ liệu** trên trang ZFS. Danh sách lấy từ máy chủ. Chọn tập dữ liệu nằm cao nhất trong những gì bạn muốn sao lưu, ví dụ `cache/appdata`, và mục sẽ bao gồm nó cùng mọi tập dữ liệu dưới nó.

- **Tập dữ liệu con mới tự được thêm vào.** Một tập dữ liệu được tạo sau này dưới mục sẽ được sao lưu ở lần chạy kế tiếp, và lần chạy đó ghi nó là mới. Bản sao lưu đầu tiên đọc toàn bộ nó một lần; sau đó chỉ đọc phần thay đổi.
- **Bạn có thể loại trừ từng tập con.** Tắt một tập con trong cài đặt của mục thì nó bị loại trừ cùng mọi thứ bên dưới. Một tập con đã loại trừ mà không còn trên máy chủ sẽ được đánh dấu như vậy và có thể xóa khỏi danh sách.
- **Tập con không đọc được sẽ bị bỏ qua, nhưng không bao giờ bỏ qua âm thầm.** Lần chạy liệt kê chúng, mục cho biết đã bỏ qua bao nhiêu, và thẻ độ bao phủ trên bảng điều khiển tính mỗi tập là chưa được bảo vệ. Lần chạy vẫn sao lưu mọi thứ còn lại và không thất bại vì một tập con bị bỏ qua. Lý do nằm trong [bảng mã lý do](#reason-codes): tập dữ liệu chưa được gắn kết, có `canmount=off`, có điểm gắn kết `legacy` hoặc không có điểm gắn kết, khóa mã hóa chưa được nạp, quyền truy cập ảnh chụp bị tắt, hoặc điểm gắn kết mà BombVault không thấy.
- **Một tập dữ liệu bị bỏ qua không kéo theo các tập con của nó.** Tập dữ liệu `canmount=off` chỉ chứa các tập dữ liệu khác sẽ bị bỏ qua (hiển thị là "chỉ cấu trúc"), và các tập con đã gắn kết của nó được sao lưu. Tập dữ liệu mã hóa chưa nạp khóa sẽ bị bỏ qua cùng các tập con dùng chung khóa đó.
- **Tập con là đĩa VM hoặc dữ liệu hệ thống bắt đầu ở trạng thái tắt** trong hộp thoại thêm, kèm lý do bên cạnh công tắc. Thêm cả một pool sẽ yêu cầu xác nhận, trong đó liệt kê những gì pool chứa.

### Ổ đĩa ảo {#volumes}

Ổ đĩa ảo (zvol) chứa một đĩa ảo thay vì tệp, và trang ZFS không bao giờ sao lưu nó.

- Ổ đĩa ảo mà một VM dùng được sao lưu cùng VM đó trên trang **VMs**.
- Ổ đĩa ảo không VM nào dùng (một iSCSI extent, một đĩa bạn đã tháo) **không được BombVault sao lưu**. Hộp thoại thêm và trang ZFS đếm các ổ đĩa ảo này và cho biết điều đó. Một phiên bản sau sẽ sao lưu chúng.

Ổ đĩa ảo trong cây của một mục bị bỏ qua và được nêu tên ở mỗi lần chạy.

### Bộ lưu trữ của Docker {#docker-storage}

Với trình điều khiển lưu trữ ZFS của Docker, mỗi lớp của image là một tập dữ liệu có điểm gắn kết `legacy`. Hộp thoại thêm gộp chúng thành một dòng cho mỗi tập cha. Một cây chứa hơn 20 tập dữ liệu như vậy không thể trở thành mục: khi còn ảnh chụp của nó, Docker không thể xóa các lớp image. Thay vào đó hãy thêm các tập dữ liệu dưới nó, ví dụ `appdata`.

### Các mục không bao giờ chồng lên nhau {#overlap}

Một tập dữ liệu chỉ có thể thuộc về một mục. BombVault từ chối mục mới nằm bên trong một mục hiện có, hoặc sẽ chứa một mục hiện có. Để gộp nhiều mục con thành một mục cha, trước tiên hãy xóa các mục con và chọn giữ lại bản sao lưu của chúng, rồi thêm mục cha. Mỗi tập dữ liệu giữ lịch sử dưới tên riêng của nó, nên lần sao lưu kế tiếp tiếp tục từ nơi các mục cũ dừng lại và không đọc lại toàn bộ.

## Dừng container và chạy lệnh quanh ảnh chụp {#consistency}

Ảnh chụp của một cơ sở dữ liệu đang chạy giống như bị mất điện đột ngột: cơ sở dữ liệu thường sẽ phục hồi, nhưng nó phải làm việc đó. Mỗi mục có thể làm hai việc cho chuyện này, và cả hai chỉ áp dụng cho thời điểm chụp, không phải toàn bộ bản sao lưu.

- **Dừng các container này cho ảnh chụp.** BombVault dừng các container trong danh sách, chụp ảnh và khởi động lại chúng ngay. Các container cùng một cấp phụ thuộc dừng song song, container phụ thuộc dừng trước, nên cả khoảng thời gian thường chỉ vài giây; lần chạy cho biết nó kéo dài bao lâu. Sau đó bản sao lưu đọc ảnh chụp đã đóng băng trong khi các ứng dụng đã chạy lại. Chỉ các container đang chạy mới bị dừng.
- **Một lệnh trước và sau ảnh chụp.** Lệnh chạy bên trong container bạn chọn, ví dụ để xuất một cơ sở dữ liệu vào tập dữ liệu ngay trước khi chụp mà không dừng gì cả. Nếu lệnh trước ảnh chụp thất bại, bản sao lưu thất bại và không có ảnh chụp nào được tạo. Lệnh sau ảnh chụp thất bại sẽ hiển thị trên lần chạy nhưng không làm bản sao lưu thất bại.

Điều gì xảy ra khi có sự cố:

- Nếu không dừng được một container, BombVault khởi động các container đã dừng và bản sao lưu thất bại kèm tên container đó. Nó không bao giờ chuyển sang chụp các ứng dụng đang chạy.
- Việc dừng sẽ chờ một bản sao lưu container đang chạy kết thúc (tối đa 30 phút với lần chạy thủ công, tới giới hạn thời gian của bản sao lưu với lần chạy theo lịch), để hai bên không bao giờ dừng và khởi động cùng một container cùng lúc.
- Trước khi dừng container đầu tiên, BombVault ghi lại các container nó sẽ dừng. Nếu BombVault bị tắt ngang trong khoảng thời gian đó, lần khởi động tiếp theo nó sẽ khởi động lại các container đó, gửi thông báo, và mục sẽ hiện ghi chú màu đỏ cho mỗi container nó không khởi động được.

Việc xuất cơ sở dữ liệu tự động (xem [Tính năng](features.md)) chạy cùng bản sao lưu riêng của container trên trang **Containers**, không chạy cùng mục ZFS. Một cơ sở dữ liệu mà container chỉ được sao lưu qua tập dữ liệu của nó sẽ không có bản xuất, vì vậy hãy đặt cho nó một lệnh ở đây.

Một container có thể nằm trong danh sách này và trên trang **Containers** cùng lúc. Khi đó dữ liệu của nó được lưu hai lần, trong hai kho, và **Sao lưu toàn bộ** sẽ dừng nó hai lần. Mục sẽ cho biết điều đó.

## Khôi phục {#restore}

Mở **Bản sao lưu** trên mục, chọn bản sao lưu, rồi chọn tập dữ liệu. Mặc định là tập dữ liệu trên cùng của mục.

- **Khôi phục vào chính tập dữ liệu.** Tệp từ bản sao lưu được ghi vào điểm gắn kết của tập dữ liệu. Tệp cùng tên bị ghi đè, các tệp khác giữ nguyên. Tập dữ liệu không bao giờ bị quay lui hay thay thế. BombVault kiểm tra rằng tập dữ liệu đã được gắn kết, nhìn thấy được và ghi được, một lần trước khi bắt đầu và một lần nữa ngay trước khi ghi. Chỗ nào có tập con được gắn kết bên trong thì không ghi gì: tập con giữ nguyên tệp, chủ sở hữu và quyền của nó, và được khôi phục từ bản sao lưu riêng.
- **Khôi phục vào một thư mục.** Chọn một thư mục dưới `/mnt`. BombVault kiểm tra rằng thư mục nằm trên một pool hoặc share đã gắn kết và còn đủ dung lượng trống. Cách này hoạt động mà không cần kết nối SSH và cho cả tập dữ liệu không còn tồn tại.
- **Chọn tệp** (nâng cao): chỉ ghi lại vào tập dữ liệu những tệp và thư mục bạn chọn.
- **Mọi tập dữ liệu của bản sao lưu này** (nâng cao): mỗi tập dữ liệu của cây vào thư mục con riêng trong thư mục bạn chọn. Các tập dữ liệu bị bỏ qua trong bản sao lưu đó sẽ được nêu tên.
- **Từ máy chủ khác:** trang **Khôi phục** khôi phục từ kho của một BombVault khác, luôn vào một thư mục: mọi tập dữ liệu của một bản sao lưu, mỗi tập vào thư mục con riêng, hoặc một tập dữ liệu của cây, toàn bộ hoặc các tệp đã chọn.

Danh sách container cần dừng của mục cũng được đưa ra khi khôi phục vào chính tập dữ liệu. Các container đó vẫn dừng trong suốt quá trình khôi phục, và trong lúc đó các bản sao lưu container phải chờ.

### Ảnh chụp an toàn {#safety-snapshot}

Trước khi ghi vào một tập dữ liệu, BombVault chụp một ảnh chụp ZFS chỉ của tập dữ liệu đó, tên là `bombvault-prerestore-<thời gian>`. Tính năng này bật theo mặc định; muốn tắt cần xác nhận lần hai. Nếu không chụp được, sẽ không có gì được khôi phục.

BombVault không bao giờ tự xóa ảnh chụp an toàn. Mục liệt kê chúng kèm tuổi và kích thước, mỗi ảnh có thao tác **Xóa**, và cảnh báo khi ảnh cũ nhất đã quá 30 ngày, vì nó giữ lại dữ liệu đã xóa và đã thay đổi trên pool.

Để quay lại sau khi khôi phục, hãy sao chép từng tệp từ `.zfs/snapshot/bombvault-prerestore-<thời gian>` bên trong tập dữ liệu. `zfs rollback <dataset>@bombvault-prerestore-<thời gian>` chỉ dùng được khi đó vẫn là ảnh chụp mới nhất của tập dữ liệu. `zfs rollback -r` xóa mọi ảnh chụp mới hơn, kể cả ảnh chụp tự động.

### Khôi phục thành tập dữ liệu mới {#new-dataset}

BombVault không tạo tập dữ liệu. Hãy tạo nó trên máy chủ với các thuộc tính bạn muốn, rồi khôi phục vào thư mục là điểm gắn kết của nó:

```
zfs create -o compression=lz4 cache/appdata-restored
```

và trong BombVault hãy khôi phục vào thư mục `cache/appdata-restored` dưới `/mnt`.

## Bản sao lưu gồm những gì {#contents}

Có trong bản sao lưu: tệp và thư mục của mọi tập dữ liệu được sao lưu, cùng chủ sở hữu, quyền, dấu thời gian và thuộc tính mở rộng như restic lưu.

Không có trong bản sao lưu:

- các thuộc tính ZFS của tập dữ liệu (compression, recordsize, quota, mountpoint và các thuộc tính khác);
- chủ sở hữu và quyền của chính thư mục trên cùng của mỗi tập dữ liệu (mọi thứ bên dưới đều có). Khôi phục vào chính tập dữ liệu giữ nguyên thư mục trên cùng hiện có, khôi phục vào một thư mục sẽ tạo nó với quyền `0755`;
- các ảnh chụp ZFS hiện có;
- các tập con bị bỏ qua hoặc bị loại trừ;
- ổ đĩa ảo.

Để khôi phục sang một pool mới, hãy tạo các tập dữ liệu với thuộc tính mong muốn trước. Việc NFSv4 ACL, theo cách TrueNAS dùng trên tập dữ liệu SMB, có trở lại như bạn mong đợi hay không vẫn chưa được kiểm chứng, nên hãy thử khôi phục với dữ liệu của chính bạn trước khi dựa vào chúng.

## Tập dữ liệu được mã hóa {#encryption}

Tập dữ liệu được mã hóa chỉ được sao lưu khi khóa của nó đã được nạp. Nếu không, nó bị bỏ qua kèm cảnh báo; hãy nạp khóa bằng `zfs load-key` và gắn kết tập dữ liệu. BombVault đọc dữ liệu đã giải mã và lưu vào kho của restic, vốn được mã hóa. Nếu bạn đã tắt mã hóa trong BombVault thì kho đó không được mã hóa.

## Ảnh chụp còn sót lại {#leftover-snapshots}

Ảnh chụp của một bản sao lưu có tên `<dataset>@bombvault-<14 chữ số>`, ví dụ `cache/appdata@bombvault-20260924021500` (UTC). BombVault xóa nó ngay sau khi sao lưu. Nếu việc đó thất bại, chẳng hạn vì tập dữ liệu đang bận hoặc BombVault bị dừng, BombVault sẽ xóa nó:

- trước lần sao lưu kế tiếp của mục đó,
- khi BombVault khởi động, cho mọi mục, kể cả khi miền đang tắt,
- khi bạn xóa mục,
- khi bạn bấm **Xóa ngay** trên mục, nơi cũng cho biết còn lại bao nhiêu.

Chỉ những tên khớp chính xác với `bombvault-` cộng 14 chữ số mới bị xóa. Ảnh chụp an toàn, ảnh chụp của riêng bạn và ảnh chụp tự động không bao giờ bị động tới. Để xóa thủ công một ảnh chụp:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Bất thường {#anomalies}

Một tập con bị làm trống hầu như không làm thay đổi tổng của một cây lớn, nên việc phát hiện bất thường theo dõi riêng từng tập dữ liệu của một mục: kích thước, số tệp, dữ liệu mới và thời gian restic đều có lịch sử riêng. Lịch sử đó gắn với tên của tập dữ liệu, nên vẫn còn khi sau này một mục khác sao lưu cây đó.

Một tập dữ liệu mà lần chạy trước đã sao lưu và lần chạy này không đọc được sẽ được tính là đã bị làm trống, miễn là lựa chọn của mục không thay đổi. Điều này bao gồm khóa chưa được nạp, tập dữ liệu chưa gắn kết và tập dữ liệu đã biến mất khỏi cây. Một tập con do chính bạn loại trừ sẽ làm thay đổi lựa chọn, nên lịch sử của nó bắt đầu lại từ đầu. Khi một phát hiện về dữ liệu bị mất còn mở, chính sách giữ lại sẽ giữ các bản sao lưu cũ của riêng tập dữ liệu đó và dọn phần còn lại của cây như bình thường.

Trong tab **Mục** của trang **Bất thường**, mỗi tập dữ liệu có một dòng riêng dưới mục của nó, và cây của mục trên trang này hiển thị các phát hiện đang mở bên cạnh từng tập dữ liệu. Liên kết trong một phát hiện mở bảng khôi phục của mục tại bản sao lưu tốt cuối cùng của tập dữ liệu. Việc một lần chạy có hoàn tất hay không được đánh giá cho cả mục, vì một lần chạy thành công hay thất bại như một khối.

Bản thân các kiểm tra được mô tả trong [Tính năng](features.md). Trợ lý kết nối qua [máy chủ MCP](mcp.md) có thể liệt kê các điểm khôi phục của một mục ZFS, bắt đầu sao lưu nó và đọc các phát hiện, nhưng việc xác nhận một phát hiện được thực hiện trên trang **Bất thường**.

## Mã lý do {#reason-codes}

Trang, lịch sử chạy và thông báo nêu một vấn đề bằng một trong các mã sau. Phần lớn mã cũng có cách khắc phục hiển thị ngay bên cạnh trên trang.

| Mã | Ý nghĩa | Cần làm gì |
|---|---|---|
| `ssh-missing` | Kết nối SSH chưa được thiết lập trong container này. | Thiết lập kết nối SSH như cho sao lưu VM. |
| `host-placeholder` | Host SSH: Address vẫn là giá trị mẫu, và `host.docker.internal` cũng không phản hồi. | Đặt Host SSH: Address thành IP LAN của máy chủ này. |
| `host-fallback` | Host SSH: Address vẫn là giá trị mẫu, và `host.docker.internal` hoạt động. | Không cần làm gì, hoặc đặt IP LAN. |
| `ssh-unreachable` | Không kết nối được tới máy chủ qua SSH. | Kiểm tra địa chỉ và cổng, và SSH đã được bật. |
| `ssh-auth` | Máy chủ từ chối khóa của BombVault. | Chạy một lần trên máy chủ lệnh hiển thị trên thẻ kết nối. |
| `zfs-not-found` | Máy chủ SSH không có lệnh `zfs`. | Trỏ Host SSH: Address tới máy sở hữu các pool. |
| `zfs-permission` | Người dùng SSH không được phép chạy lệnh zfs này. | Dùng root, hoặc xem [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` chỉ tới máy chủ hoặc người dùng khác với các trường SSH. | Làm cho chúng khớp nhau, hoặc để trống các trường SSH để cả hai lấy từ URI. |
| `zfs-error` | zfs báo một lỗi khác. | Phần chi tiết hiển thị thông báo của nó. |
| `propagation-missing` | Các lần gắn kết mới trên máy chủ không tới được container. | Đặt Access Mode của Host Data thành Read/Write - Slave và khởi động lại BombVault. |
| `invalid-name` | Tên tập dữ liệu mà BombVault không chấp nhận. | Đổi tên tập dữ liệu. |
| `name-too-long` | Một tập dữ liệu trong cây quá dài để làm tên ảnh chụp. | Đổi tên nó, hoặc thêm một tập dữ liệu bên dưới làm mục. |
| `invalid-exclude` | Một mẫu loại trừ hoặc một tập con bị loại trừ không khớp với mục. | Sửa mục mà thông báo nêu. Để loại trừ cả một tập con, hãy tắt nó thay vì viết mẫu. |
| `not-found` | Tập dữ liệu không tồn tại trên máy chủ. | Xóa mục, hoặc tạo lại tập dữ liệu. Các bản sao lưu của nó vẫn khôi phục được. |
| `not-filesystem` | Đây là ổ đĩa ảo, không phải hệ thống tệp. | Xem [Ổ đĩa ảo](#volumes). |
| `overlaps-item` | Tập dữ liệu chồng lên một mục hiện có. | Xem [Các mục không bao giờ chồng lên nhau](#overlap). |
| `docker-storage` | Cây chứa bộ lưu trữ image của Docker. | Xem [Bộ lưu trữ của Docker](#docker-storage). |
| `nothing-readable` | Hiện không đọc được tập dữ liệu nào trong mục. | Xem mã của các tập dữ liệu bị bỏ qua. |
| `snapshot-failed` | Không tạo được ảnh chụp. | Phần chi tiết hiển thị thông báo của zfs. |
| `containers-busy` | Một bản sao lưu container vẫn đang chạy khi các container cần dừng. | Bắt đầu lại sau. Các lần chạy theo lịch sẽ tự chờ. |
| `consistency-stop-failed` | Không dừng được một container, nên không có ảnh chụp nào được tạo. | Kiểm tra container, hoặc gỡ nó khỏi danh sách. |
| `pre-snapshot-failed` | Lệnh trước ảnh chụp đã thất bại. | Chi tiết lần chạy hiển thị đầu ra của lệnh. |
| `container-unknown` | Một container trong danh sách không tồn tại. | Gỡ nó khỏi danh sách. |
| `container-is-self` | BombVault không thể dừng container của chính nó. | Gỡ nó khỏi danh sách. |
| `leftover-snapshots` | Vẫn còn ảnh chụp mà BombVault không xóa được trên máy chủ. | Bấm **Xóa ngay**, xem [Ảnh chụp còn sót lại](#leftover-snapshots). |
| `zvol` | Một ổ đĩa ảo trong cây, đã bỏ qua. | Xem [Ổ đĩa ảo](#volumes). |
| `canmount-off` | Không bao giờ được gắn kết (`canmount=off`), đã bỏ qua. | Nếu có dữ liệu, hãy gắn kết nó hoặc chuyển dữ liệu sang một tập con. |
| `legacy-mount` | Điểm gắn kết legacy, đã bỏ qua. | Đặt cho nó một điểm gắn kết dưới `/mnt`. |
| `no-mountpoint` | Không có điểm gắn kết, đã bỏ qua. | Đặt cho nó một điểm gắn kết dưới `/mnt`. |
| `not-mounted` | Chưa gắn kết trên máy chủ, đã bỏ qua. | Gắn kết bằng `zfs mount`, hoặc đặt `canmount=on`. |
| `key-not-loaded` | Được mã hóa và khóa chưa được nạp, đã bỏ qua. | `zfs load-key`, rồi gắn kết. |
| `snapdir-disabled` | Quyền truy cập ảnh chụp bị tắt, đã bỏ qua. | `zfs set snapdir=hidden <dataset>`. Thư mục `.zfs` vẫn bị ẩn. |
| `not-visible` | BombVault không thấy điểm gắn kết của tập dữ liệu. | Chuyển điểm gắn kết xuống dưới đường dẫn Host Data, hoặc ánh xạ nó vào container tại cùng đường dẫn với Read/Write - Slave. |
| `shfs-only` | Tập dữ liệu chỉ thấy được qua `/mnt/user`, vốn ẩn các ảnh chụp. | Ánh xạ `/mnt`, không phải `/mnt/user`, làm Host Data. |
| `snapshot-not-visible` | Ảnh chụp đã được tạo nhưng không xuất hiện bên trong BombVault. | Chạy **Thử truy cập ảnh chụp**; xem bên dưới. |
| `snapshot-loop` | Ảnh chụp không tới được BombVault vì Host Data không chuyển tiếp các lần gắn kết mới. | Đặt Access Mode của Host Data thành Read/Write - Slave và khởi động lại BombVault. |
| `backup-failed` | restic thất bại với tập dữ liệu này. | Chi tiết lần chạy cho biết lý do. |
| `not-reached` | Lần chạy kết thúc trước khi tới tập dữ liệu này. | Chạy lại bản sao lưu. |
| `gone` | Tập dữ liệu không còn trên máy chủ. | Không cần làm gì. Các bản sao lưu của nó vẫn khôi phục được. |
| `read-only-mount` | BombVault chỉ đọc được tập dữ liệu, nên không thể khôi phục vào đó. | Đặt ánh xạ thành Read/Write - Slave, hoặc khôi phục vào một thư mục. |
| `destination-not-mounted` | Thư mục không nằm trên pool hoặc share đã gắn kết. | Chọn một thư mục trên pool hoặc share. |
| `not-enough-space` | Không đủ dung lượng trống tại đích. | Giải phóng dung lượng hoặc chọn thư mục khác. |
| `safety-snapshot-failed` | Không chụp được ảnh chụp an toàn, nên không có gì được khôi phục. | Phần chi tiết hiển thị thông báo của zfs. |
| `safety-name-too-long` | Tên tập dữ liệu quá dài cho một ảnh chụp an toàn. | Tắt ảnh chụp an toàn, hoặc khôi phục vào một thư mục. |

### Kiểm tra những gì container nhìn thấy {#mountinfo}

**Thử truy cập ảnh chụp** trên một mục sẽ chụp một ảnh thật của cây, tìm nó bên trong BombVault cho từng tập dữ liệu, rồi xóa nó đi. Đây là cách nhanh nhất để chứng minh toàn bộ đường đi trước lần chạy theo lịch đầu tiên.

Để tự xem, hãy chạy lệnh này trên máy chủ:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Mỗi dòng là một lần gắn kết bên trong container. Dòng của một tập dữ liệu cho biết đường dẫn của nó bên trong container (dưới `/host/user`) và tên tập dữ liệu. Trường `master:N` trên dòng đó nghĩa là lần gắn kết này nhận được các lần gắn kết mà máy chủ thực hiện về sau, và đó chính là điều quyền truy cập ảnh chụp cần. Nếu thiếu, hãy đặt Access Mode của Host Data thành Read/Write - Slave và khởi động lại BombVault.

## TrueNAS SCALE {#truenas}

- Khi `LIBVIRT_URI` được đặt (như cho sao lưu VM trên TrueNAS), BombVault lấy từ URI máy chủ, người dùng và cổng SSH cho các lệnh zfs của nó, với mỗi giá trị chưa được đặt riêng. Nếu không sao lưu VM, hãy đặt `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` và `LIBVIRT_SSH_PORT` thay thế. Thêm các biến trong **Additional Environment Variables**.
- Người dùng khác root cần quyền trên tập dữ liệu trên cùng của mục, quyền này sau đó áp dụng cho mọi tập dữ liệu bên dưới:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Phiên SSH không phải root trên TrueNAS không có `/usr/sbin` trong đường dẫn; khi đó BombVault gọi trực tiếp `/usr/sbin/zfs`.
- **Host Data** của ứng dụng phải là một đường dẫn máy chủ nằm trên các tập dữ liệu, ví dụ `/mnt/tank`, không phải ixVolume. Với đường dẫn máy chủ, ứng dụng chuyển các lần gắn kết mới của máy chủ tới BombVault (`rslave`), và đó chính là điều quyền truy cập ảnh chụp cần.
