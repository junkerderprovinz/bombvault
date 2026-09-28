# Điểm lưu trữ

Một điểm lưu trữ là nơi BombVault giữ các bản sao lưu: một thư mục trên Unraid này, một share trên NAS, một bucket tại một nhà cung cấp đám mây, một rest-server, một tài khoản SFTP hoặc một Nextcloud. Bạn kết nối mỗi điểm lưu trữ một lần, trên **Cài đặt, Lưu trữ**, và thông tin đăng nhập, mức lưu giữ, bảo vệ và vị trí đều thuộc về chính điểm lưu trữ đó. Sau đó các miền (container, VM, flash, cấu hình của chính BombVault, bộ tập tin và tập dữ liệu ZFS) chọn từ các điểm lưu trữ: mỗi miền được lưu ở đâu và được sao chép đến đâu. Tập dữ liệu ZFS có hàng riêng trên thẻ Miền khi miền ZFS được bật.

## Thêm một điểm lưu trữ {#add-a-place}

**Thêm điểm lưu trữ** mở một cửa sổ với một ô cho mỗi nhà cung cấp, chia thành ba nhóm: lưu trữ đám mây, dịch vụ tự vận hành, và thiết bị NAS cùng máy chủ này.

1. Chọn một ô và điền vào biểu mẫu của nó. Nút hình con mắt hiển thị một bí mật bạn đã nhập.
2. **Kiểm tra kết nối** kiểm tra điểm lưu trữ và không tạo ra gì cả. Với thư mục của từng miền, nó báo lại những gì tìm thấy: trống hoặc chưa có, đã chứa một kho restic, hoặc lỗi đã làm nó dừng lại.
3. Đặt tên cho điểm lưu trữ; tên của nhà cung cấp đã được điền sẵn. Với một thiết bị bạn tự vận hành, hãy trả lời **Thiết bị nằm ở đâu?**. Các nhà cung cấp đám mây luôn ở một địa điểm khác, còn một thư mục trên Unraid này luôn ở đây.
4. **Thêm** lưu điểm lưu trữ.

Một điểm lưu trữ mới chưa được miền nào dùng. Hãy chọn nó dưới **Lưu tại** hoặc **Sao chép đến** trên [thẻ Miền](#domains), hoặc trên thẻ của một mục chỉ cho riêng mục đó.

## Thư mục {#folders}

Một điểm lưu trữ giữ một thư mục cho mỗi miền: `container`, `vms`, `flash`, `config`, `files` và `zfs`, cũng là các tên mà vị trí sao lưu mặc định dùng. Các thư mục được liệt kê trong phần chi tiết của điểm lưu trữ và có thể đổi tên ở đó (xem [Đổi địa chỉ](#addresses)). Một miền không có thư mục tại một điểm lưu trữ thì không thể chọn điểm lưu trữ đó.

Khi một miền dùng một điểm lưu trữ cho cả hai vai trò, vai trò thứ hai được thêm một hậu tố còn vai trò thứ nhất giữ nguyên thư mục của nó. Một điểm lưu trữ đã nhận các bản sao của một miền sẽ lưu những mục được gửi thẳng tới nó trong `<folder>-direct`; một điểm lưu trữ đã lưu một miền sẽ nhận các bản sao của miền đó trong `<folder>-copies`.

Một số điểm lưu trữ tự chúng là một kho restic: một địa chỉ đã chứa sẵn một kho khi điểm lưu trữ được thêm, một kho có tên từ một thiết lập có sẵn, hoặc một đích sao chép nằm ở gốc của một bucket. Một điểm lưu trữ như vậy không có thư mục, mọi miền dùng chung một kho duy nhất của nó, và nó không nhận vai trò thứ hai. Để lưu thêm tại cùng nhà cung cấp, hãy kết nối một bucket hoặc thư mục khác thành một điểm lưu trữ riêng.

## Chi tiết điểm lưu trữ {#details}

Mỗi điểm lưu trữ là một hàng hiển thị nhà cung cấp, việc nó đang được dùng vào, và lần kiểm tra hoặc sao chép gần nhất. **Kiểm tra** kiểm tra mọi địa chỉ tại điểm lưu trữ, và **Chi tiết** mở phần cài đặt của nó. Mọi thay đổi trong phần chi tiết được lưu ngay khi bạn thực hiện.

- **Chung**: tên, công tắc bật và tắt điểm lưu trữ, địa chỉ và, với một thiết bị bạn tự vận hành, **Thiết bị nằm ở đâu?** (xem [Ngoài cơ sở](#off-the-premises)).
- **Lưu giữ**: giữ gần nhất, hằng ngày, hằng tuần và hằng tháng, cho mọi kho tại điểm lưu trữ. Một điểm lưu trữ mới bắt đầu với các quy tắc mặc định; một điểm lưu trữ có mọi quy tắc bằng 0 thì không bao giờ cắt bớt.
- **Bảo vệ**: công tắc **Append-only**. Phía bên kia phải tự thực thi append-only; khi bật công tắc, BombVault không bao giờ dọn bớt hay xóa gì ở đó. Tại một rest-server có bật append-only, **Kiểm tra append-only** chạy bài kiểm tra can thiệp với mọi đường dẫn miền, bản sao đang bật và kho tại điểm lưu trữ, và hiển thị một câu trả lời cho cả điểm lưu trữ, *xóa bị từ chối* hoặc *xóa được chấp nhận* (xem [Off-site & khôi phục](offsite-recovery.md)). Chỉ các điểm lưu trữ từ xa mới có mục này, vì không gì trên máy này có thể ngăn một kho cục bộ bị xóa.
- **Truy cập**: thông tin đăng nhập và, với S3, lớp lưu trữ. Một điểm lưu trữ dùng thông tin đăng nhập chung sẽ nhận một bộ riêng ở lần thay đổi đầu tiên. Một kho trực tiếp tại điểm lưu trữ mà thông tin đăng nhập mới không mở được sẽ giữ bộ cũ, và phản hồi sẽ cho biết điều đó. Các điểm lưu trữ dạng thư mục, SFTP và rclone không có mục này.
- **Giới hạn**: tốc độ tải lên và tải xuống, cùng ngân sách tăng trưởng.
- **Thư mục**: một công tắc cho mỗi miền, kèm tên thư mục của miền đó. Một miền bị tắt ở đây thì không thể chọn điểm lưu trữ này.

Giảm mức lưu giữ sẽ hỏi trước và cho biết có bao nhiêu mục bị ảnh hưởng; tắt append-only sẽ hỏi trước và cho biết có bao nhiêu kho tại điểm lưu trữ mất nó. Tắt một điểm lưu trữ sẽ tắt mọi kho tại đó; một điểm lưu trữ đang lưu một miền thì không thể tắt.

## Thẻ Miền {#domains}

Thẻ có một hàng cho mỗi miền, với lịch trình, nơi miền được lưu, nơi miền được sao chép đến và các ngoại lệ của nó.

- **Lưu tại**: khi vị trí sao lưu của miền chưa chứa bản sao lưu nào, điểm lưu trữ được chọn trở thành nơi lưu chính của miền và vị trí được chuyển sang đó. Khi vị trí đã có bản sao lưu, lựa chọn cho container, VM và bộ tập tin trở thành mặc định cho các mục mới, và các mục này nhận nó ở lần sao lưu đầu tiên; các mục đã có bản sao lưu vẫn ở nguyên chỗ, vì BombVault không bao giờ di chuyển một bản sao lưu. Với flash và cấu hình của chính BombVault, nơi lưu chính được chuyển đi, còn các bản sao lưu đã ghi vẫn ở điểm lưu trữ cũ.
- **Sao chép đến**: một chip cho mỗi điểm lưu trữ có thể nhận bản sao của miền. Đánh dấu một chip sẽ biến điểm lưu trữ đó thành đích sao chép của miền; ở lần đầu, BombVault cho biết trước lần chạy đầu tiên sẽ gửi bao nhiêu mục, bao nhiêu snapshot và bao nhiêu dữ liệu. Bỏ đánh dấu sẽ dừng các bản sao mới: các bản sao đã có ở đó vẫn được giữ và già đi theo mức lưu giữ của điểm lưu trữ, còn các mục có lựa chọn riêng vẫn tiếp tục sao chép đến đó. Bỏ đánh dấu chip cuối cùng sẽ dừng mọi bản sao, kể cả đến các điểm lưu trữ được thêm sau này, cho đến khi một chip được đánh dấu lại. Một điểm lưu trữ đã tắt hiển thị như một chip mờ và không thể được chọn.
- **Ngoại lệ**: các mục có lựa chọn riêng, dưới dạng một danh sách có liên kết tới thẻ của chúng.
- **Sao chép ngay** chạy việc sao chép của miền ngay lập tức.

Một miền bị tạm dừng sau một lần xây dựng lại qua Discover sẽ hiển thị việc tạm dừng trên hàng của nó, kèm **Xác nhận mặc định** (xem [Nơi lưu trữ theo từng mục](offsite-recovery.md#placement)).

## Đổi địa chỉ {#addresses}

Thư mục của một miền có thể được đổi trong phần chi tiết của điểm lưu trữ, và địa chỉ của một điểm lưu trữ cục bộ cũng vậy, ví dụ sau khi một kho được chuyển sang ổ đĩa khác bằng tay. BombVault kiểm tra mọi địa chỉ mà thay đổi ảnh hưởng tới và chấp nhận nó khi mỗi địa chỉ mới đều trống và không có gì được lưu ở địa chỉ cũ, hoặc khi mỗi địa chỉ mới chứa cùng kho restic với địa chỉ cũ. Mọi trường hợp khác đều bị từ chối, kèm số bản sao lưu vẫn còn ở địa chỉ cũ. Một điểm lưu trữ từ xa giữ nguyên địa chỉ của nó; để sao lưu đến nơi khác, hãy kết nối nơi đó thành một điểm lưu trữ riêng.

BombVault dựng danh sách điểm lưu trữ từ cơ sở dữ liệu của chính nó và không bao giờ liệt kê một kho từ xa để điền vào danh sách đó; việc kiểm tra chỉ chạy khi bạn thay đổi điều gì đó.

## Gỡ bỏ một điểm lưu trữ {#remove}

Một điểm lưu trữ chỉ có thể được gỡ bỏ khi không có gì dùng nó: không miền nào được lưu ở đó, không mặc định nào trỏ tới nó, không mục nào được lưu ở đó, và không kho trực tiếp nào tại đó chứa mục. Nếu không, lời từ chối sẽ liệt kê những gì đang giữ nó. Gỡ bỏ nó cũng gỡ theo các đích sao chép của nó, và cả thông tin đăng nhập riêng của nó trừ khi một nguồn kéo về hoặc một điểm lưu trữ khác đang dùng chúng. Không có gì bị xóa trên chính bộ lưu trữ, và lời xác nhận cho biết có bao nhiêu bản sao vẫn còn lại ở đó.

## Không có điểm lưu trữ {#without-a-place}

Một địa chỉ không khớp với dạng một điểm lưu trữ cộng một thư mục vẫn tiếp tục hoạt động và được liệt kê dưới **Không có điểm lưu trữ**, kèm địa chỉ của nó. Các địa chỉ `b2:`, `gs:` và `swift:` gốc thuộc nhóm này. **Gán vào điểm lưu trữ** gắn một hàng như vậy vào một điểm lưu trữ, sau cùng bài kiểm tra như khi [đổi địa chỉ](#addresses). Một đích sao chép không có điểm lưu trữ cũng được nêu tên trên hàng của miền nó thuộc về, cạnh các chip, và vẫn tiếp tục sao chép. Một hàng từ xa ở đó có công tắc **Append-only** riêng, và tắt nó sẽ hỏi trước kèm số mục đang giữ bản sao lưu ở địa chỉ đó. Một kho trực tiếp đi theo công tắc của đích của nó.

## Ngoài cơ sở {#off-the-premises}

**Thiết bị nằm ở đâu?** có hai câu trả lời: **Ở đây, trong nhà** và **Ở một địa điểm khác**. Một bản sao chỉ được tính là một địa điểm riêng, cho dòng 3-2-1 trên các thẻ và cho các kiểm tra off-site của bảng điều khiển, khi điểm lưu trữ của nó ở một địa điểm khác. Một ổ đĩa thứ hai hay một NAS trong cùng ngôi nhà là một bản sao thứ hai, không phải một địa điểm thứ hai. Câu trả lời không thay đổi bản sao nào. Các nhà cung cấp đám mây luôn ở một địa điểm khác và một thư mục trên Unraid này luôn ở đây, nên biểu mẫu không hỏi với chúng; với mọi điểm lưu trữ khác, hãy đổi câu trả lời trong phần chi tiết của nó. Một điểm lưu trữ ở địa điểm khác mang dấu **Địa điểm khác** trên hàng của nó.

## Các loại kết nối

### Thư mục trên Unraid này hoặc NAS {#kind-local}

Địa chỉ là một đường dẫn dưới `/mnt`, viết không kèm `/mnt`, ví dụ `user/bombvault`, và thư mục của mỗi miền nằm bên dưới nó: `user/bombvault/container`.

- **Thư mục trên Unraid này** chọn từ các share, ổ đĩa và pool.
- **Synology**, **QNAP**, **TrueNAS**, **Một Unraid khác** và **Chia sẻ khác** chọn từ `/mnt/remotes`. Hãy gắn kết share trên Unraid trước, ví dụ bằng plugin Unassigned Devices. Host Data phải được gắn kết ở chế độ Read/Write - Slave, nếu không một share được gắn kết sau khi BombVault khởi động sẽ không hiển thị cho đến khi khởi động lại (xem [Cấu hình](configuration.md)).

Trình chọn thư mục tạo một thư mục ngay tại vị trí đang mở bằng **Thư mục mới**. Bài kiểm tra xác nhận rằng thư mục trống hoặc chưa tồn tại và BombVault có thể ghi vào đó.

### S3 {#kind-s3}

Địa chỉ có dạng `s3:https://<endpoint>/<bucket>/<path>`, ví dụ `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** chỉ cần ID khóa và khóa ứng dụng. BombVault hỏi B2 xem khóa bị giới hạn ở bucket, điểm cuối S3 và thư mục nào, rồi dựng địa chỉ từ đó. Một khóa được phép truy cập mọi bucket sẽ đưa ra các bucket của nó để bạn chọn.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** và **Vultr** hỏi khóa và, khi nhà cung cấp cần, hỏi vùng, ID tài khoản hoặc điểm cuối. BombVault điền sẵn điểm cuối và liệt kê các bucket khi khóa được phép liệt kê chúng; nếu không, hãy nhập tên bucket.
- **Google Cloud Storage** đi qua giao diện S3 của nó với một khóa HMAC, được tạo trong phần cài đặt Cloud Storage tại mục Interoperability. Tệp tài khoản dịch vụ (service account) không dùng được ở đây.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** và **Dịch vụ S3 khác** nhận địa chỉ của dịch vụ và một khóa.

Lớp lưu trữ được đặt trong phần chi tiết của điểm lưu trữ, giới hạn ở các tầng mà một lần khôi phục có thể đọc mà không cần rã đông.

### rest-server {#kind-rest}

Địa chỉ có dạng `rest:<url>/<user>`, ví dụ `rest:https://nas.lan:8000/tower`. Biểu mẫu hỏi địa chỉ máy chủ, một người dùng và một mật khẩu. Với `--private-repos`, một người dùng chỉ được truy cập các đường dẫn bắt đầu bằng tên của chính họ, nên BombVault đặt người dùng lên đầu trừ khi bạn nhập một đường dẫn khác. Khi máy chủ từ chối một đường dẫn nằm ngoài phần của người dùng, thông báo lỗi sẽ nói rõ điều đó.

Biểu mẫu rest-server kèm một công thức sẵn sàng để dán cho một rest-server ở chế độ append-only với một người dùng cho BombVault này. **Hiện công thức** tạo một mật khẩu, chỉ hiện một lần, và đưa ra một dòng `docker run`, một tệp compose và một mẫu Unraid, mỗi thứ kèm dòng `htpasswd` cần đặt lên máy chủ; người dùng và mật khẩu được điền thẳng vào biểu mẫu.

**Một BombVault khác** liệt kê, phía trên các ô của chính nó, những đề nghị đang mở mà các phiên bản khác đã gửi qua trang Đội. Chấp nhận một đề nghị sẽ thêm một điểm lưu trữ chỉ giữ bản sao của miền được đề nghị, vì một đề nghị mang theo một người dùng cho đúng miền đó. Chấp nhận trên trang Đội cũng thêm cùng điểm lưu trữ ấy.

### SFTP {#kind-sftp}

Địa chỉ có dạng `sftp://<user>@<host>:<port>/<path>`, ví dụ `sftp://bv@backup.lan:22/bombvault`. Biểu mẫu hỏi máy chủ, cổng và người dùng, và hiển thị khóa công khai của BombVault. Thêm khóa đó vào `~/.ssh/authorized_keys` của người dùng trên máy chủ; không cần cài đặt gì khác ở đó. BombVault chấp nhận host key của máy chủ ở lần liên lạc đầu tiên và kiểm tra nó từ đó về sau.

**Hetzner Storage Box** điền sẵn `<user>.your-storagebox.de` và cổng 23. Cài khóa lên Storage Box bằng lệnh riêng của Hetzner, lệnh này hỏi mật khẩu của Storage Box một lần:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Biểu mẫu hỏi địa chỉ máy chủ, người dùng và một mật khẩu ứng dụng. Tạo mật khẩu ứng dụng trong phần cài đặt bảo mật của tài khoản, và nhập ID người dùng thay vì địa chỉ e-mail. BombVault dựng đường dẫn WebDAV mà sản phẩm dùng và chuyển kết nối cho restic qua các biến môi trường của rclone, với mật khẩu ở dạng đã làm rối (obscure) của rclone. Địa chỉ có dạng `rclone:bvp<id>:<path>`, trong đó `bvp<id>` là một remote chỉ tồn tại trong môi trường đó; không có gì được ghi vào cấu hình rclone.

### Azure Blob {#kind-azure}

Địa chỉ có dạng `azure:<container>:/<path>`. Biểu mẫu hỏi tài khoản lưu trữ và khóa truy cập của nó; sau **Kiểm tra kết nối**, nó liệt kê các container của tài khoản để bạn chọn, hoặc bạn nhập tên một container. BombVault truyền tài khoản và khóa cho restic dưới dạng `AZURE_ACCOUNT_NAME` và `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Địa chỉ có dạng `rclone:<remote>:<path>`. Biểu mẫu liệt kê các remote trong cấu hình rclone của BombVault để bạn chọn. Để thay thế cấu hình đó, hãy dán toàn bộ một tệp `rclone.conf` vào **Cấu hình rclone** và nhấp **Lưu cấu hình**. Cấu hình được lưu ngay và dùng cho mọi điểm lưu trữ rclone, dù sau đó cửa sổ có thêm điểm lưu trữ hay không.

## Sao chép giữa các điểm lưu trữ có thông tin đăng nhập khác nhau {#different-credentials}

Một miền được lưu ở một điểm lưu trữ từ xa là nguồn của các bản sao của nó. `restic copy` chạy với một môi trường duy nhất, và BombVault thêm thông tin đăng nhập của nguồn vào thông tin đăng nhập của đích khi hai bên không đặt cùng một biến với các giá trị khác nhau. Một điểm lưu trữ Nextcloud và một điểm lưu trữ B2 dùng các biến khác nhau, nên một miền được lưu trong Nextcloud có thể được sao chép sang B2. Hai tài khoản S3 hoặc hai người dùng rest-server sẽ cần cùng các biến với giá trị khác nhau; restic không thể nhận cả hai, và chip trên thẻ Miền sẽ báo rằng thông tin đăng nhập không khớp.
