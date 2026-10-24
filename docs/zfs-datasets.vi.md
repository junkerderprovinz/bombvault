# Tập dữ liệu ZFS

Tập dữ liệu ZFS là các mục trong danh sách **Bản sao lưu**, như container và VM. Một mục là một tập dữ liệu cùng mọi tập dữ liệu bên dưới nó. Với mỗi bản sao lưu, BombVault chụp một snapshot ZFS của cả cây, nên mọi tập dữ liệu trong đó được lấy cùng một thời điểm. Sau đó nó đọc tệp của từng tập dữ liệu từ snapshot đó và lưu chúng bằng restic, giống cách nó lưu một thư mục, rồi xóa snapshot ngay sau đó. Các bản sao lưu được khử trùng lặp, bạn duyệt được từng bản, và khôi phục được từng tệp.

Bản sao lưu không bao giờ gửi luồng vào restic và không bao giờ hoàn nguyên một tập dữ liệu. BombVault chỉ hủy những ảnh chụp do chính nó tạo. [Bản nhân bản](#replica) tùy chọn là nơi duy nhất dùng `zfs send`: nó sao chép các tập dữ liệu sang một máy chủ ZFS thứ hai và không động đến bản sao lưu.

## Yêu cầu {#requirements}

- **Kết nối SSH tới máy chủ này.** Tập dữ liệu ZFS dùng cùng khóa, máy chủ và người dùng với sao lưu VM. Nếu sao lưu VM đã hoạt động thì phần này cũng hoạt động. Nếu chưa, hãy làm theo [hướng dẫn sao lưu VM qua SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) trên GitHub. Các trường trong mẫu có tên **Host SSH: Address**, **Host SSH: Port** và **Host SSH: User**.
- **Lệnh `zfs` trên máy chủ đó.** Unraid 6.12 trở lên và TrueNAS SCALE đều có.
- **Host Data được ánh xạ thành `/mnt` với Access Mode Read/Write - Slave.** Đây là giá trị mặc định của mẫu. Ảnh chụp của một tập dữ liệu chỉ xuất hiện trong thư mục `.zfs/snapshot` của tập dữ liệu sau khi BombVault đã khởi động, nên container phải nhận được các lần gắn kết mà máy chủ thực hiện về sau.
- **Các tập dữ liệu được gắn kết dưới `/mnt`.** Trên Unraid, pool nằm ở `/mnt/<pool>`, nên điều này đã đúng sẵn.

Bật phần này trong **Cài đặt, Chung** (Tập dữ liệu ZFS). **Kết nối ZFS tới máy chủ này** dưới **Thêm** trên trang Bản sao lưu khi đó mở **Kết nối tới máy chủ này**. Nó kiểm tra liên kết SSH, nêu tên người dùng và host nó kết nối tới, và cho biết còn thiếu gì khi có gì đó lỗi. Khi liên kết mất, danh sách Bản sao lưu nói điều đó bằng **Xem kết nối**, và thẻ **Tích hợp máy chủ** trên Tổng quan hiển thị cùng kết quả.

## Các mục và tập dữ liệu con {#items-and-children}

Chọn **Thêm** trên trang Bản sao lưu, rồi **Một tập dữ liệu ZFS**. Danh sách lấy từ máy chủ. Chọn tập dữ liệu ở đầu những gì bạn muốn sao lưu, ví dụ `cache/appdata`, và mục bao gồm nó cùng mọi tập dữ liệu bên dưới.

- **Tập dữ liệu con mới tự gia nhập.** Một tập dữ liệu được tạo bên dưới mục về sau sẽ được sao lưu ở lần chạy kế tiếp, và lần chạy đó nêu tên nó là mới. Bản sao lưu đầu tiên đọc toàn bộ nó một lần; sau đó chỉ đọc các thay đổi.
- **Có thể loại từng tập con.** Tắt một tập con trên tab Cài đặt của mục và nó bị loại cùng mọi thứ bên dưới. Một tập con bị loại mà không còn tồn tại trên máy chủ được đánh dấu như vậy và có thể gỡ khỏi danh sách.
- **Tập con không đọc được sẽ bị bỏ qua, không bao giờ một cách âm thầm.** Lần chạy liệt kê chúng, mục hiển thị có bao nhiêu bị bỏ qua, và thẻ **Chưa được sao lưu** trên Tổng quan tính mỗi cái là chưa được bảo vệ. Lần chạy vẫn sao lưu mọi thứ khác và không thất bại vì một tập con bị bỏ qua. Lý do nằm trong [bảng mã lý do](#reason-codes): một tập dữ liệu chưa gắn, `canmount=off`, mountpoint là `legacy` hoặc không có, khóa mã hóa chưa nạp, truy cập snapshot bị tắt, hoặc một mountpoint mà BombVault không thấy.
- **Một tập dữ liệu bị bỏ qua không kéo theo các tập con của nó.** Tập dữ liệu `canmount=off` chỉ chứa các tập dữ liệu khác sẽ bị bỏ qua (hiển thị là "chỉ cấu trúc"), và các tập con đã gắn kết của nó được sao lưu. Tập dữ liệu mã hóa chưa nạp khóa sẽ bị bỏ qua cùng các tập con dùng chung khóa đó.
- **Tập con là đĩa VM hoặc dữ liệu hệ thống bắt đầu ở trạng thái tắt** trong hộp thoại thêm, kèm lý do bên cạnh công tắc. Thêm cả một pool sẽ yêu cầu xác nhận, trong đó liệt kê những gì pool chứa.

### Ổ đĩa ảo {#volumes}

Một volume (zvol) chứa một đĩa ảo thay vì tệp, và một mục ZFS không bao giờ sao lưu nó.

- Volume mà một VM dùng được sao lưu cùng VM đó.
- Volume mà không VM nào dùng (một iSCSI extent, một đĩa bạn đã tháo) **không được BombVault sao lưu**. Cửa sổ thêm và trang Bản sao lưu đếm các volume này và nói điều đó. Một phiên bản sau sẽ sao lưu chúng.

Volume bên trong cây của một mục bị bỏ qua và được nêu tên ở mỗi lần chạy.

### Bộ lưu trữ của Docker {#docker-storage}

Với trình điều khiển lưu trữ ZFS của Docker, mỗi lớp image là một tập dữ liệu có mountpoint `legacy`. Hộp thoại thêm gộp chúng thành một dòng cho mỗi tập cha. Một cây chứa hơn 20 tập dữ liệu như vậy không thể thành một mục: khi snapshot của nó còn tồn tại, Docker không xóa được các lớp image. Hãy thêm các tập dữ liệu bên dưới nó, ví dụ `appdata`.

### Các mục không bao giờ chồng lấn {#overlap}

Một tập dữ liệu chỉ thuộc về một mục. BombVault từ chối một mục mới nằm trong một mục hiện có, hoặc sẽ chứa một mục hiện có. Để gộp nhiều mục con thành một mục cha, trước tiên hãy xóa các mục con và chọn giữ lại bản sao lưu của chúng, rồi thêm mục cha. Mỗi tập dữ liệu giữ lịch sử dưới tên của chính nó, nên bản sao lưu kế tiếp tiếp tục từ chỗ các mục cũ dừng và không đọc lại mọi thứ.

## Dừng container và chạy lệnh quanh ảnh chụp {#consistency}

Snapshot của một cơ sở dữ liệu đang chạy giống như mất điện đột ngột: cơ sở dữ liệu thường sẽ tự phục hồi, nhưng nó phải làm vậy. Mỗi mục có thể làm hai việc về điều đó, và cả hai chỉ bao phủ khoảnh khắc của snapshot, không phải toàn bộ bản sao lưu.

- **Dừng các container này cho ảnh chụp.** BombVault dừng các container trong danh sách, chụp ảnh và khởi động lại chúng ngay. Các container cùng một cấp phụ thuộc dừng song song, container phụ thuộc dừng trước, nên cả khoảng thời gian thường chỉ vài giây; lần chạy cho biết nó kéo dài bao lâu. Sau đó bản sao lưu đọc ảnh chụp đã đóng băng trong khi các ứng dụng đã chạy lại. Chỉ các container đang chạy mới bị dừng.
- **Một lệnh trước và sau ảnh chụp.** Lệnh chạy bên trong container bạn chọn, ví dụ để xuất một cơ sở dữ liệu vào tập dữ liệu ngay trước khi chụp mà không dừng gì cả. Nếu lệnh trước ảnh chụp thất bại, bản sao lưu thất bại và không có ảnh chụp nào được tạo. Lệnh sau ảnh chụp thất bại sẽ hiển thị trên lần chạy nhưng không làm bản sao lưu thất bại.

Điều gì xảy ra khi có sự cố:

- Nếu không dừng được một container, BombVault khởi động các container đã dừng và bản sao lưu thất bại kèm tên container đó. Nó không bao giờ chuyển sang chụp các ứng dụng đang chạy.
- Việc dừng sẽ chờ một bản sao lưu container đang chạy kết thúc (tối đa 30 phút với lần chạy thủ công, tới giới hạn thời gian của bản sao lưu với lần chạy theo lịch), để hai bên không bao giờ dừng và khởi động cùng một container cùng lúc.
- Trước khi container đầu tiên dừng, BombVault ghi lại những container nào nó dừng. Nếu BombVault bị tắt đột ngột trong khoảng đó, nó khởi động lại các container ấy ở lần khởi động kế tiếp, gửi một thông báo, và mục hiển thị ghi chú đỏ cho mỗi container nó không khởi động được.

Các bản kết xuất cơ sở dữ liệu tự động (xem [Tính năng](features.md)) chạy cùng bản sao lưu riêng của container, không phải cùng một mục ZFS. Một cơ sở dữ liệu có container chỉ được sao lưu qua tập dữ liệu của nó thì không có bản kết xuất, nên hãy đưa cho nó một lệnh ở đây.

Một container có thể nằm trong danh sách này và đồng thời được sao lưu như một container. Dữ liệu của nó khi đó được lưu hai lần, trong hai kho, và sao lưu toàn bộ dừng nó hai lần. Mục nói điều đó.

## Khôi phục {#restore}

Mở mục và chọn **Khôi phục**, hoặc chọn nó trên trang **Khôi phục**. Cửa sổ khôi phục hỏi bản sao lưu, rồi **Tập dữ liệu**. Mặc định đó là tập dữ liệu trên cùng của mục. Dưới **Cái gì** bạn khôi phục tất cả hoặc chỉ các tệp và thư mục bạn chọn. **Đến đâu** đưa ra:

- **Khôi phục vào chính tập dữ liệu.** Tệp từ bản sao lưu được ghi vào điểm gắn kết của tập dữ liệu. Tệp cùng tên bị ghi đè, các tệp khác giữ nguyên. Tập dữ liệu không bao giờ bị quay lui hay thay thế. BombVault kiểm tra rằng tập dữ liệu đã được gắn kết, nhìn thấy được và ghi được, một lần trước khi bắt đầu và một lần nữa ngay trước khi ghi. Chỗ nào có tập con được gắn kết bên trong thì không ghi gì: tập con giữ nguyên tệp, chủ sở hữu và quyền của nó, và được khôi phục từ bản sao lưu riêng.
- **Vào một thư mục.** Chọn một thư mục dưới `/mnt`. BombVault kiểm tra thư mục nằm trên một pool hoặc share đã gắn và còn đủ dung lượng trống. Cách này hoạt động không cần liên kết SSH và cho các tập dữ liệu không còn tồn tại.
- **Vào một tập dữ liệu mới.** Đặt tên cho một tập dữ liệu chưa tồn tại; cửa sổ gợi ý `<dataset>-restored`. BombVault tạo nó với các thuộc tính ZFS được lưu trong bản sao lưu và khôi phục vào đó, xem [Khôi phục thành tập dữ liệu mới](#new-dataset).

Chọn **Mọi tập dữ liệu của bản sao lưu này** làm tập dữ liệu sẽ ghi mọi tập dữ liệu của cây vào thư mục con riêng trong thư mục bạn chọn. Các tập dữ liệu bị bỏ qua trong bản sao lưu đó được nêu tên. Để khôi phục từ kho của một BombVault khác, dùng **Từ thư mục sao lưu cũ** trên trang Khôi phục; nó luôn khôi phục vào một thư mục: mọi tập dữ liệu của một bản sao lưu, mỗi tập vào thư mục con riêng, hoặc một tập dữ liệu của cây, nguyên vẹn hoặc các tệp chọn lọc.

Danh sách container cần dừng của mục cũng được đưa ra cho việc khôi phục vào tập dữ liệu. Các container đó giữ trạng thái dừng suốt quá trình khôi phục, và sao lưu container chờ trong lúc đó.

### Ảnh chụp an toàn {#safety-snapshot}

Trước khi ghi vào một tập dữ liệu, BombVault chụp một snapshot ZFS của riêng tập dữ liệu đó, tên `bombvault-prerestore-<time>`. **Giữ trước một ảnh chụp ZFS của trạng thái hiện tại** được bật mặc định; tắt nó cần xác nhận lần hai. Nếu không chụp được snapshot, không có gì được khôi phục.

BombVault không bao giờ tự xóa snapshot an toàn. Mục liệt kê chúng cùng tuổi và dung lượng, mỗi cái có hành động **Xóa**, và cảnh báo khi cái cũ nhất quá 30 ngày, vì nó giữ dữ liệu đã xóa và đã đổi trên pool.

Để quay lại sau khi khôi phục, hãy sao chép từng tệp từ `.zfs/snapshot/bombvault-prerestore-<thời gian>` bên trong tập dữ liệu. `zfs rollback <dataset>@bombvault-prerestore-<thời gian>` chỉ dùng được khi đó vẫn là ảnh chụp mới nhất của tập dữ liệu. `zfs rollback -r` xóa mọi ảnh chụp mới hơn, kể cả ảnh chụp tự động.

### Khôi phục thành tập dữ liệu mới {#new-dataset}

BombVault lưu cùng mỗi bản sao lưu các thuộc tính ZFS đặt cục bộ của từng tập dữ liệu: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity và các thuộc tính người dùng của riêng bạn. Giá trị kế thừa và giá trị chỉ đọc được bỏ qua, vì chúng tự quay lại. Các bản sao lưu từ trước khi BombVault lưu chúng không có thuộc tính nào.

- **Vào một tập dữ liệu mới** chạy `zfs create` với mọi thuộc tính đã lưu. casesensitivity, normalization và utf8only chỉ đặt được theo cách này. Hạn mức (quota) và dung lượng đặt trước (reservation) được đặt sau khi ghi tệp, để chúng không thể từ chối các tệp đó. Điểm gắn kết được bỏ qua để bản sao không đụng bản gốc, cũng như `canmount`, `readonly` và thiết lập mã hóa, để việc khôi phục ghi được. Một tập dữ liệu mới nằm dưới tập được mã hóa sẽ nhận mã hóa của nó. Tập dữ liệu phía trên phải tồn tại. Nếu có gì thất bại sau khi tạo, tập dữ liệu mới vẫn ở trên máy chủ, vì BombVault không bao giờ hủy một tập dữ liệu.
- **Khôi phục vào chính tập dữ liệu** hiển thị các thuộc tính đã lưu bên cạnh việc khôi phục. **Đặt cả các thuộc tính này** đặt những thuộc tính mà tập dữ liệu có sẵn vẫn nhận, trước khi ghi bất kỳ tệp nào. Hạn mức (quota) và dung lượng đặt trước (reservation) được đặt sau khi ghi tệp, để chúng không thể từ chối các tệp đó. Không có công tắc này, tập dữ liệu giữ nguyên thiết lập của nó.

## Bản sao lưu gồm những gì {#contents}

Có trong bản sao lưu: tệp và thư mục của mọi tập dữ liệu được sao lưu, cùng chủ sở hữu, quyền, dấu thời gian và thuộc tính mở rộng như restic lưu, cùng các thuộc tính ZFS đặt cục bộ của từng tập dữ liệu.

Không có trong bản sao lưu:

- chủ sở hữu và quyền của chính thư mục trên cùng của mỗi tập dữ liệu (mọi thứ bên dưới đều có). Khôi phục vào chính tập dữ liệu giữ nguyên thư mục trên cùng hiện có, khôi phục vào một thư mục sẽ tạo nó với quyền `0755`;
- các ảnh chụp ZFS hiện có;
- các tập con bị bỏ qua hoặc bị loại trừ;
- ổ đĩa ảo.

Để khôi phục sang một pool mới, hãy tạo pool rồi khôi phục từng tập dữ liệu vào một tập dữ liệu mới. Việc NFSv4 ACL, theo cách TrueNAS dùng trên tập dữ liệu SMB, có trở lại như bạn mong đợi hay không vẫn chưa được kiểm chứng, nên hãy thử khôi phục với dữ liệu của chính bạn trước khi dựa vào chúng.

## Tập dữ liệu được mã hóa {#encryption}

Tập dữ liệu được mã hóa chỉ được sao lưu khi khóa của nó đã được nạp. Nếu không, nó bị bỏ qua kèm cảnh báo; hãy nạp khóa bằng `zfs load-key` và gắn kết tập dữ liệu. BombVault đọc dữ liệu đã giải mã và lưu vào kho của restic, vốn được mã hóa. Nếu bạn đã tắt mã hóa trong BombVault thì kho đó không được mã hóa.

## Bản nhân bản {#replica}

Bản nhân bản là một bản sao các tập dữ liệu của một mục trên máy chủ ZFS thứ hai. BombVault giữ nó cập nhật bằng `zfs send` và `zfs receive`. Lần chạy đầu gửi mọi thứ, sau đó chỉ các khối thay đổi được truyền. Trên máy chủ kia bạn có thể gắn bản sao ngay.

Bản nhân bản không bao giờ thay thế bản sao lưu. Phiên bản cũ hơn, từng tệp và việc kiểm tra vẫn đến từ các bản sao lưu, và bản nhân bản chỉ giữ số snapshot bạn đặt. Một bản nhân bản cập nhật được tính là bản sao ngoài cơ sở, nhưng một mục có bản nhân bản mà không có bản sao lưu vẫn ở màu cam.

**Chỉ bản nhân bản, không sao lưu** trong thẻ **Bản nhân bản** khiến cả phần ZFS gửi bản nhân bản và không tạo bản sao lưu. Thư mục riêng và các đích sao lưu off-site vẫn giữ công tắc của chúng, nên khi tắt tùy chọn này, phần đó lại sao lưu tới những nơi ấy, và thẻ của mọi mục ZFS hiển thị công tắc của phần đúng như trạng thái hiện tại. Một mục như vậy chờ bản nhân bản đầu tiên thay vì bản sao lưu, **Sao lưu ngay** được thay bằng **Nhân bản ngay**, và các container của mục dừng lại cùng các lệnh của nó chạy quanh snapshot như khi sao lưu. Các bản nhân bản của nó được tính cho lịch vài ngày một lần, cảnh báo quá hạn và độ bao phủ.

Bật nó trong thẻ **Bản nhân bản** ở tab Cài đặt của mục. Ở đó bạn chọn bản nhân bản đi đâu, khi nào chạy (**Sau mỗi bản sao lưu** hoặc **Kế hoạch riêng**) và bao nhiêu snapshot ở lại trên đích và trên máy chủ này. Thẻ liệt kê mọi tập dữ liệu và volume cùng trạng thái, và **Nhân bản ngay** bắt đầu một lần chạy. Một lần chạy nhân bản có khóa riêng, nên lần truyền đầu tiên dài không bao giờ làm chậm các bản sao lưu.

**Nhân bản tất cả ngay**, nằm cạnh các thao tác sao lưu khi ô ZFS được chọn trên trang Bản sao lưu, gửi bản nhân bản của mọi mục ZFS, lần lượt từng mục. Máy chủ chạy cả lô, nên nó vẫn tiếp tục khi bạn rời trang, và mục nào không nhân bản được sẽ được nêu tên và bỏ qua.

### Đẩy sang máy chủ ZFS {#replica-push}

Bất kỳ máy nào có ZFS và SSH đều nhận được, ví dụ một Unraid thứ hai hoặc TrueNAS. BombVault không cần chạy ở đó.

1. Dưới **Cài đặt, Ghép nối**, chọn **Kết nối qua SSH** trong **Máy chủ ZFS không có BombVault**, hoặc **Thêm máy chủ ZFS** trong thẻ **Bản nhân bản** của mục.
2. Nhập địa chỉ, người dùng và cổng. Hộp thoại hiện khóa công khai của BombVault. Thêm nó vào `~/.ssh/authorized_keys` của người dùng trên máy chủ. Trên Unraid nó nằm ở **Users, root, SSH keys**. Tùy theo **Display Settings**, Users nằm trong menu tiêu đề hoặc trong Settings. Trên TrueNAS, sửa người dùng trong **Credentials, Users**. Đây là khóa của bản sao. Nó khác với khóa của liên kết SSH cho VM và máy chủ, nên hãy thêm nó ngay cả khi khóa kia đã được cấp quyền.
3. Thử kết nối. Hộp thoại sau đó liệt kê các pool của máy chủ. Chọn một pool và đặt gốc, mặc định là `<pool>/bombvault-replica`.
4. Chọn máy chủ mới trong thẻ **Bản nhân bản** của mục. Máy chủ khi đó có thẻ riêng trên trang Phiên bản, và **Mở** dẫn tới trang của nó.

Với root thì không cần gì thêm. Một người dùng riêng cần các quyền sau trên pool của đích, hộp thoại cũng hiển thị chúng:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Trên nguồn, một người dùng cùng loại cần những quyền này trên tập dữ liệu trên cùng của mục:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

Theo chiều này, BombVault đang giữ mục cũng giữ một khóa có thể ghi trên máy chủ.

BombVault lưu khóa host mà máy chủ đưa ra ở lần kết nối đầu tiên và về sau từ chối mọi khóa khác. Nếu máy chủ được cài lại, hãy bấm **Quên khóa host** trên trang của máy chủ, lần kết nối tiếp theo sẽ lưu khóa mới.

Khi host của máy chủ này tự kết nối được tới máy chủ ZFS, nó chuyển `zfs send` thẳng vào ssh riêng của nó tới máy chủ đó, với khóa của bản sao và khóa host đã lưu, nên luồng dữ liệu không đi qua BombVault. Nếu không, luồng đi qua BombVault.

### Gửi sang phiên bản đã ghép đôi {#replica-receive}

Một BombVault đã ghép đôi có thể tự nhận bản nhân bản. Không ai được truy cập SSH vào máy chủ kia và không có khóa nào được thêm vào tệp `authorized_keys`.

1. Trong thẻ **Bản nhân bản** của mục, chọn phiên bản đã ghép nối làm đích, hoặc bật vai trò **Máy chủ ZFS** trên thẻ của phiên bản đó ở trang Phiên bản và chọn các mục ZFS ở đó. Thẻ hiển thị **Đang chờ phê duyệt** cho đến khi nó trả lời.
2. Trên phiên bản nhận, mở **Phiên bản**. Yêu cầu hiện trên thẻ của phiên bản gửi như một huy hiệu, và **Chi tiết** của nó liệt kê yêu cầu. Chọn pool và thư mục gốc trên máy chủ đó và bao nhiêu snapshot ở lại, rồi nhấn **Cho phép** hoặc **Từ chối**.
3. Sau **Cho phép**, nguồn gửi theo lịch của chính nó, như với mọi đích khác.

Phiên bản nhận chỉ chấp nhận những gì sự phê duyệt bao gồm: các tập dữ liệu của mục, vào thư mục gốc của chính nó. Nó tự chạy `zfs receive`, và nguồn không có cách nào xóa hay hoàn nguyên gì ở đó. Vì vậy bản sao sống sót khi nguồn bị ai đó chiếm quyền. Phiên bản nhận giữ chính sách lưu giữ riêng. Nguồn chỉ gợi ý một quy tắc khi nó yêu cầu.

**Revoke access** trên phiên bản nhận kết thúc sự phê duyệt bất cứ lúc nào và báo cho nguồn, nguồn khi đó hiện rằng phê duyệt đã bị rút lại và dừng gửi. Những gì phiên bản nhận đã có vẫn ở lại đó. Yêu cầu bị từ chối vẫn là bị từ chối. Muốn thêm tập dữ liệu, hoặc yêu cầu lại sau khi thu hồi, cần gửi yêu cầu mới.

Phiên bản nhận phải trả lời qua HTTPS. Nguồn không gửi gì đến một địa chỉ http thường, vì token và dữ liệu sẽ đi qua mạng mà không được mã hóa. Mỗi lần truyền cũng chừa trống một phần mười pool nhận và dừng trước khi đụng tới phần đó, nên một nguồn không thể làm đầy pool của máy chủ nhận.

### Dữ liệu nằm ở đâu {#replica-target}

Mỗi tập dữ liệu nằm tại `<root>/<server>/<pool>/<path>`. Thư mục máy chủ là tên của phiên bản nguồn, được cố định ở lần truyền đầu, nên hai máy chủ có cùng tên pool không bao giờ lẫn vào nhau. Ví dụ, `cache/appdata` của máy chủ tên `tower` kết thúc tại `backup/bombvault-replica/tower/cache/appdata`.

Sau khi BombVault được thiết lập lại, đích từ chối thư mục mà BombVault này đã ghi vào trước đó. Khi đó thẻ **Bản nhân bản** đưa ra **Tiếp quản thư mục**, và bản nhân bản tiếp theo tiếp tục từ các bản sao trong đó. Chỉ tiếp quản khi không còn BombVault nào khác nhân bản vào thư mục đó.

Bản sao trên đích ở chế độ chỉ đọc và không được gắn kết, nên nó không che phủ gì trên máy chủ đó. Các thuộc tính ZFS được mang theo, trừ điểm gắn kết, `sharenfs` và `sharesmb`. Các phần đặt trước `reservation` và `refreservation` cũng không được mang theo, nên bản sao chỉ chiếm dung lượng mà dữ liệu của nó cần.

### Những gì được đưa vào {#replica-contents}

Mọi thứ mục sao lưu đều vào, và các volume bên dưới nó cũng vậy, mà bản sao lưu bỏ qua. Một tập dữ liệu con bạn đã tắt trong mục thì ở ngoài. Mọi tập dữ liệu của một lần chạy đến từ một snapshot, như trong bản sao lưu.

### Ảnh chụp được giữ bao lâu {#replica-retention}

Ở đích, một bản nhân bản mới giữ 7 ảnh chụp hằng ngày và 3 ảnh chụp hằng tuần. Hãy chọn **Short**, **Cân bằng** hoặc **Long**, hoặc đặt **Custom values**. Chỉ các ảnh chụp tên `bombvault-replica-<14 digits>` mới bị xóa ở đó, và không bao giờ xóa ảnh chụp mới nhất mà cả hai phía cùng có.

Ở nguồn, BombVault giữ ảnh chụp nhân bản mới nhất, cùng một bookmark cho mỗi trạng thái đã gửi. Bookmark không tốn dung lượng. Lần truyền tiếp theo bắt đầu từ chúng.

Để giữ nhiều hơn cả ở nguồn, hãy đặt **Giữ trên máy chủ này** trong thẻ **Bản nhân bản**, ví dụ thành 7. Khi đó, sau mỗi lần nhân bản, chừng ấy ảnh chụp nhân bản mới nhất sẽ ở lại trên máy chủ này, bên cạnh những gì đích giữ. 0 hoặc 1 chỉ giữ ảnh chụp mới nhất. Ảnh chụp mới nhất mang một hold, để cơ sở của lần truyền tiếp theo không bị xóa nhầm. Các ảnh chụp cũ hơn không có hold. Mỗi ảnh chụp giữ lại các khối đã thay đổi sau nó, nên mỗi ảnh chụp bạn giữ đều chiếm dung lượng trong pool. Tắt bản nhân bản, chuyển nó sang đích khác hoặc xóa mục cũng sẽ xóa chúng khỏi máy chủ này.

Bạn có thể đọc hoặc sao chép ngay các tệp của một ảnh chụp đã giữ, từ thư mục `.zfs/snapshot` bên trong tập dữ liệu hoặc từ một bản clone, mà không đụng tới tập dữ liệu hay bản nhân bản:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Bản clone không chiếm dung lượng cho đến khi có gì đó trong nó thay đổi. Khi xong việc, hãy xóa nó bằng `zfs destroy cache/appdata-20261006`, vì không thể xóa một ảnh chụp đang có bản clone.

Muốn quay lại ảnh chụp nhân bản mới nhất thì ở đây chỉ cần `zfs rollback cache/appdata@<mới-nhất>`, và không phải làm gì trên đích. Hoàn nguyên cả tập dữ liệu về một ảnh chụp cũ hơn sẽ xóa mọi ảnh chụp sau nó, cả trên máy chủ này lẫn trên đích. Ảnh chụp mới nhất có hold, nên trước tiên hãy gỡ hold, rồi hoàn nguyên tập dữ liệu tại đây:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

Sau đó hoàn nguyên bản sao về cùng ảnh chụp ấy trên máy chủ ZFS, hoặc trên host của phiên bản đã ghép đôi đang nhận bản nhân bản. Nếu không, lần nhân bản tiếp theo của tập dữ liệu đó sẽ dừng lại, vì đích có các ảnh chụp mới hơn:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100
```

`zfs rollback` chỉ tác động lên một tập dữ liệu, nên hãy lặp lại cả hai lần hoàn nguyên cho từng tập dữ liệu con mà bạn cũng muốn lấy lại. Lần nhân bản tiếp theo sẽ tiếp tục từ ảnh chụp đó.

### Tập dữ liệu mã hóa trong bản nhân bản {#replica-encryption}

Một tập dữ liệu mã hóa được gửi ở dạng thô. Nó vẫn được mã hóa ở đích, và đích không bao giờ thấy khóa. Hãy giữ khóa an toàn: bạn cần nó để mở bản sao sau khi khôi phục, và bản nhân bản thiếu khóa thì không đọc được.

### Dùng bản nhân bản {#replica-use}

Mở tab **Bản sao lưu** của mục và nhấp hàng **Bản nhân bản ZFS** trong số các nơi giữ snapshot của nó. Bảng liệt kê các snapshot trên đích và hiển thị các lệnh với tên thật của bạn.

Để xem một trạng thái cũ, hãy clone một ảnh chụp trên đích. Bản clone không tốn dung lượng cho đến khi có gì thay đổi, và bản nhân bản vẫn nguyên vẹn:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Nếu nguồn hỏng, hãy biến bản sao trên đích thành một tập dữ liệu bình thường, ghi được:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

Sau đó BombVault ngừng nhân bản sang tập dữ liệu đó, cho đến khi bạn bắt đầu một lần chạy đầu tiên mới.

Để đưa một trạng thái về nguồn, nhấn **Bring back as a new dataset** trong trang. BombVault gửi ảnh chụp vào một tập dữ liệu mới cạnh bản gốc, tên là `<dataset>-bombvault-restore-` cộng với dấu thời gian. Nó không bao giờ ghi đè lên bản gốc.

## Ảnh chụp còn sót lại {#leftover-snapshots}

Ảnh chụp của một bản sao lưu có tên `<dataset>@bombvault-<14 chữ số>`, ví dụ `cache/appdata@bombvault-20260924021500` (UTC). BombVault xóa nó ngay sau khi sao lưu. Nếu việc đó thất bại, chẳng hạn vì tập dữ liệu đang bận hoặc BombVault bị dừng, BombVault sẽ xóa nó:

- trước bản sao lưu kế tiếp của mục đó,
- khi BombVault khởi động, cho mọi mục, cả khi phần này đang tắt,
- khi bạn xóa mục,
- khi bạn nhấn **Xóa ngay** trên mục, hiển thị còn lại bao nhiêu.

Chỉ những tên khớp chính xác với `bombvault-` cộng 14 chữ số mới bị xóa. Ảnh chụp an toàn, ảnh chụp của riêng bạn và ảnh chụp tự động không bao giờ bị động tới. Để xóa thủ công một ảnh chụp:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Ảnh chụp của một bản nhân bản có tên `<dataset>@bombvault-replica-<14 digits>` và không phải là ảnh chụp còn sót lại. Nó ở lại trên nguồn cho đến khi một lần chạy nhân bản mới hơn thay thế nó, hoặc chừng nào **Giữ trên máy chủ này** còn giữ nó, và ở đích chừng nào việc lưu giữ còn giữ nó. Trình dọn dẹp không bao giờ động đến nó, vì nó chỉ khớp với `bombvault-` theo sau là đúng 14 chữ số.

## Bất thường {#anomalies}

Một tập con bị làm trống hầu như không đổi tổng của một cây lớn, nên phát hiện bất thường theo dõi từng tập dữ liệu của một mục riêng: dung lượng, số tệp, dữ liệu mới và thời gian restic của nó mỗi thứ có lịch sử riêng. Lịch sử đó thuộc về tên tập dữ liệu, nên nó còn nguyên khi cây sau này được một mục khác sao lưu.

Một tập dữ liệu mà lần chạy trước đã sao lưu và lần chạy này không đọc được được tính là bị làm trống, miễn là lựa chọn của mục không đổi. Điều đó bao gồm khóa chưa nạp, tập dữ liệu chưa gắn và tập đã biến mất khỏi cây. Một tập con bạn tự loại làm đổi lựa chọn, nên lịch sử của nó bắt đầu lại. Khi một phát hiện về mất dữ liệu đang mở, lưu giữ giữ các bản sao lưu cũ của đúng tập dữ liệu đó và dọn phần còn lại của cây như thường.

Mỗi tập dữ liệu có một dòng riêng trong phần giám sát của mục, mở bằng **Giám sát** trên trang Bất thường và nằm trên tab **Xác minh** của mục. Các phát hiện đang mở hiện cạnh từng tập dữ liệu. Liên kết trong một phát hiện mở cửa sổ khôi phục trên bản sao lưu tốt gần nhất của tập dữ liệu. Việc một lần chạy có hoàn tất không được đánh giá cho cả mục, vì một lần chạy thành công hay thất bại như một thể.

Bản thân các kiểm tra được mô tả trong [Tính năng](features.md). Một trợ lý kết nối qua [máy chủ MCP](mcp.md) có thể liệt kê các điểm khôi phục của một mục ZFS, bắt đầu sao lưu của nó và đọc các phát hiện, nhưng việc xác nhận một phát hiện diễn ra trên trang Bất thường.

## Mã lý do {#reason-codes}

Trang của mục, lịch sử chạy và các thông báo nêu một vấn đề bằng một trong các mã này. Hầu hết có cách khắc phục ngay cạnh trên trang.

| Mã | Ý nghĩa | Cần làm gì |
|---|---|---|
| `ssh-missing` | Kết nối SSH chưa được thiết lập trong container này. | Thiết lập kết nối SSH như cho sao lưu VM. |
| `host-placeholder` | Host SSH: Address vẫn là giá trị mẫu, và `host.docker.internal` cũng không phản hồi. | Đặt Host SSH: Address thành IP LAN của máy chủ này. |
| `host-fallback` | Host SSH: Address vẫn là giá trị mẫu, và `host.docker.internal` hoạt động. | Không cần làm gì, hoặc đặt IP LAN. |
| `ssh-unreachable` | Không kết nối được tới máy chủ qua SSH. | Kiểm tra địa chỉ và cổng, và SSH đã được bật. |
| `ssh-auth` | Máy chủ đã từ chối khóa của BombVault. | Chạy một lần trên máy chủ lệnh hiển thị dưới **Kết nối tới máy chủ này**. |
| `zfs-not-found` | Máy chủ SSH không có lệnh `zfs`. | Trỏ Host SSH: Address tới máy sở hữu các pool. |
| `zfs-permission` | Người dùng SSH không được phép chạy lệnh zfs này. | Dùng root, hoặc xem [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` chỉ tới máy chủ hoặc người dùng khác với các trường SSH. | Làm cho chúng khớp nhau, hoặc để trống các trường SSH để cả hai lấy từ URI. |
| `zfs-error` | zfs báo một lỗi khác. | Phần chi tiết hiển thị thông báo của nó. |
| `propagation-missing` | Các lần gắn kết mới trên máy chủ không tới được container. | Đặt Access Mode của Host Data thành Read/Write - Slave và khởi động lại BombVault. |
| `invalid-name` | Tên tập dữ liệu mà BombVault không chấp nhận. | Đổi tên tập dữ liệu. |
| `name-too-long` | Một tập dữ liệu trong cây quá dài cho tên snapshot. | Đổi tên nó, hoặc thêm một tập dữ liệu bên dưới nó làm mục. |
| `invalid-exclude` | Một mẫu loại trừ hoặc một tập con bị loại không khớp với mục. | Sửa mục mà thông báo nêu tên. Để loại cả một tập dữ liệu con, hãy tắt nó thay vì viết mẫu. |
| `not-found` | Tập dữ liệu không tồn tại trên máy chủ. | Xóa mục, hoặc tạo lại tập dữ liệu. Các bản sao lưu của nó vẫn khôi phục được. |
| `not-filesystem` | Đây là ổ đĩa ảo, không phải hệ thống tệp. | Xem [Ổ đĩa ảo](#volumes). |
| `overlaps-item` | Tập dữ liệu chồng lấn với một mục hiện có. | Xem [Các mục không bao giờ chồng lấn](#overlap). |
| `docker-storage` | Cây chứa bộ lưu trữ image của Docker. | Xem [Bộ lưu trữ của Docker](#docker-storage). |
| `nothing-readable` | Không tập dữ liệu nào trong mục đọc được ngay lúc này. | Xem mã của các tập dữ liệu bị bỏ qua. |
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
| `dataset-exists` | Đã có một tập dữ liệu tên này. | Chọn tên mới, hoặc khôi phục vào chính tập dữ liệu đó. |
| `create-failed` | Không tạo được tập dữ liệu mới. | Phần chi tiết cho thấy thông báo của zfs. Kiểm tra tập dữ liệu phía trên có tồn tại không. |
| `new-dataset-not-visible` | Tập dữ liệu mới đã được tạo nhưng BombVault không thấy nó, nên chưa khôi phục gì. | Tập dữ liệu vẫn còn trên máy chủ. Gắn nó dưới đường dẫn Host Data rồi khôi phục vào đó. |
| `set-properties-failed` | Không đặt được các thuộc tính đã lưu, nên chưa khôi phục gì. | Phần chi tiết hiển thị thông báo của zfs. |
| `set-limits-failed` | Đã khôi phục các tệp, nhưng không đặt được hạn mức (quota) hoặc dung lượng đặt trước (reservation) đã lưu. | Phần chi tiết cho thấy thông báo của zfs. Hãy tự đặt hạn mức hoặc dung lượng đặt trước bằng `zfs set`. |

### Kiểm tra những gì container nhìn thấy {#mountinfo}

**Thử truy cập ảnh chụp** trên tab Cài đặt của mục chụp một snapshot thật của cây, tìm nó bên trong BombVault cho từng tập dữ liệu, rồi xóa nó. Đó là cách nhanh nhất để chứng minh toàn bộ đường đi trước lần chạy theo lịch đầu tiên.

Để tự xem, hãy chạy lệnh này trên máy chủ:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Mỗi dòng là một lần gắn kết bên trong container. Dòng của một tập dữ liệu cho biết đường dẫn của nó bên trong container (dưới `/host/user`) và tên tập dữ liệu. Trường `master:N` trên dòng đó nghĩa là lần gắn kết này nhận được các lần gắn kết mà máy chủ thực hiện về sau, và đó chính là điều quyền truy cập ảnh chụp cần. Nếu thiếu, hãy đặt Access Mode của Host Data thành Read/Write - Slave và khởi động lại BombVault.

## TrueNAS SCALE {#truenas}

- Khi `LIBVIRT_URI` được đặt (như cho sao lưu VM trên TrueNAS), BombVault lấy từ URI máy chủ, người dùng và cổng SSH cho các lệnh zfs của nó, với mỗi giá trị chưa được đặt riêng. Nếu không sao lưu VM thì không cần đặt gì: `host.docker.internal` không phân giải được trong một ứng dụng TrueNAS, nên BombVault tiếp cận máy chủ qua cổng mặc định (default gateway) của ứng dụng, với quyền root trên cổng 22. Đặt `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` và `LIBVIRT_SSH_PORT` trong `environment` của ứng dụng khi bạn muốn một địa chỉ hoặc người dùng khác.
- Thêm khóa SSH máy chủ từ **Cài đặt, Tích hợp, SSH máy chủ** cho root trong **Credentials, Users**, và bật dịch vụ SSH trong **System, Services**. Một [bản sao](#replica-push) tới TrueNAS này hoặc một TrueNAS khác cũng cần khóa của bản sao, là một khóa khác.
- Một người dùng không phải root cần quyền trên tập dữ liệu trên cùng của mục, khi đó bao gồm mọi tập dữ liệu bên dưới:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Khôi phục vào một tập dữ liệu mới còn cần quyền `create` trên tập dữ liệu phía trên, và đặt các thuộc tính đã lưu cần quyền đối với các thuộc tính đó.

  Phiên SSH không phải root trên TrueNAS không có `/usr/sbin` trong đường dẫn; khi đó BombVault gọi trực tiếp `/usr/sbin/zfs`.
- **Host Data** của ứng dụng phải là một đường dẫn máy chủ nằm trên các tập dữ liệu, ví dụ `/mnt/tank`, không phải ixVolume. Với đường dẫn máy chủ, ứng dụng chuyển các lần gắn kết mới của máy chủ tới BombVault (`rslave`), và đó chính là điều quyền truy cập ảnh chụp cần. Xem [TrueNAS SCALE](getting-started.md#truenas) cho toàn bộ phần thiết lập ứng dụng.
