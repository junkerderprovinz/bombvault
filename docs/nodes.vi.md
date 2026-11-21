# Node

Node là một BombVault không có giao diện web, ví dụ trên một VPS hoặc trên một máy chủ nhỏ ở nhà bạn bè. Node sao lưu các container và thư mục trên máy chủ của chính nó, còn BombVault mà bạn dùng để thêm node, trong trang này gọi là **phiên bản chủ**, cho node biết khi nào sao lưu và gửi bản sao lưu đi đâu. Bạn chăm sóc node từ giao diện web của phiên bản chủ, ngay cạnh các mục của chính mình.

![Thẻ của node vps-falkenstein trên trang Phiên bản: Đã kết nối, phiên bản phần mềm, bản sao lưu trên máy chủ nhận, thời điểm sao lưu của từng phần, cùng Đổi đích, Thay thế và Giải phóng.](assets/screenshots/nodes/card-dark.png#only-dark)
![Thẻ của node vps-falkenstein trên trang Phiên bản: Đã kết nối, phiên bản phần mềm, bản sao lưu trên máy chủ nhận, thời điểm sao lưu của từng phần, cùng Đổi đích, Thay thế và Giải phóng.](assets/screenshots/nodes/card-light.png#only-light)

## Cách hoạt động {#how-it-works}

- **Phiên bản chủ giữ khóa của node.** Khi bạn thêm một node, phiên bản chủ tạo `APP_KEY` cho nó, giữ một bản niêm phong và trao cho bạn đúng một lần, trong tệp compose của node. Nhờ bản đó, phiên bản chủ kết nối được tới node và mở được các bản sao lưu của nó.
- **Node không cần mở cổng nào.** Phiên bản chủ và node nói chuyện qua relay bằng một kết nối riêng tư của riêng chúng, tách khỏi nhóm ghép nối của bạn. Phiên bản chủ bắt đầu và ký mọi lệnh gọi, còn node chỉ gửi cho phiên bản chủ lời chào của nó. Đích duy nhất cần mở cổng là [giữ bản sao lưu trên node](#keep-on-node).
- **Bản sao lưu không bao giờ đi qua relay.** Node gửi chúng thẳng tới nơi bạn chọn: máy chủ nhận của phiên bản chủ, một trong các đích sao lưu của phiên bản chủ, hoặc một rest-server trên chính node.
- **Node không tự lên kế hoạch gì.** Nó không chạy lịch, không gửi thông báo và không tham gia nhóm hay pháo đài nào. Phiên bản chủ bắt đầu các bản sao lưu của nó, theo dõi diễn biến và gửi cảnh báo.
- **Node sao lưu container và bộ thư mục.** VM, ổ flash, tập dữ liệu ZFS và bản tự sao lưu vẫn là việc của bản cài đầy đủ.

Kết nối dùng relay riêng của phiên bản chủ khi nhóm của phiên bản chủ có relay riêng, nếu không thì dùng relay của dự án, kể cả khi phiên bản chủ ghép nối mà không dùng relay.

## Trước khi bắt đầu {#before-you-start}

- **Một mật khẩu đăng nhập trên phiên bản chủ.** Thêm node, hiện lại tệp compose của nó và mở các mục của nó đều cần đăng nhập bằng mật khẩu, xem [Đăng nhập](configuration.md#login).
- **Docker có Compose trên máy chủ của node.**
- **Một nơi để chứa bản sao lưu**, một trong ba nơi ở [Bản sao lưu đi đâu](#placement). Máy chủ nhận là lựa chọn an toàn nhất, vì node không xóa được gì ở đó.

## Thêm một node {#add}

1. Trên phiên bản chủ, mở **Phiên bản** và chọn **Thêm node** ở góc dưới bên phải.
2. **Tên.** Gõ tên node sẽ hiển thị ở đây, chẳng hạn tên của máy chủ, rồi chọn **Tiếp**.

    ![Cửa sổ Thêm node ở bước đầu tiên, với tên vps-helsinki đã được gõ vào.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Cửa sổ Thêm node ở bước đầu tiên, với tên vps-helsinki đã được gõ vào.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Bản sao lưu.** Chọn nơi gửi bản sao lưu và điền những gì nơi đó cần. **Sao lưu hằng ngày lúc** là giờ cả hai phần bắt đầu. Phiên bản chủ đề xuất nửa giờ sau các bản sao lưu container của chính nó, để hai bên không cùng lúc tranh nhau một ổ đĩa. Sau này bạn có thể đặt kế hoạch riêng cho từng phần trên thẻ. Chọn **Tiếp**.

    ![Bước thứ hai: đã chọn máy chủ nhận, Địa chỉ cho node được điền một địa chỉ https, và Sao lưu hằng ngày lúc 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Bước thứ hai: đã chọn máy chủ nhận, Địa chỉ cho node được điền một địa chỉ https, và Sao lưu hằng ngày lúc 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Khởi động.** Cửa sổ hiện tệp compose của node với `MODE`, `APP_KEY`, `NODE_HOME` và `NODE_NAME` đã được điền. Hãy sao chép nó và cất `APP_KEY` ở nơi an toàn. Trên máy chủ của node:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Trước khi khởi động, hãy trỏ volume Host Data tới dữ liệu bạn muốn sao lưu và đặt `HOST_SOURCE_ROOT` thành cùng đường dẫn đó, như ví dụ bên dưới.

5. Cửa sổ chờ node. Khi node trả lời, phiên bản chủ cho nó biết gửi bản sao lưu đi đâu và kiểm tra rằng nó tới được nơi đó, rồi cửa sổ hiện **vps-helsinki đã trực tuyến và nhận lệnh từ phiên bản này.**

    ![Bước cuối: tệp compose, rồi node đã trực tuyến và bản sao lưu của nó được đặt trên máy chủ nhận.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Bước cuối: tệp compose, rồi node đã trực tuyến và bản sao lưu của nó được đặt trên máy chủ nhận.](assets/screenshots/nodes/add-start-light.png#only-light)

Bạn có thể đóng cửa sổ trước khi node trả lời. Khi đó thẻ của nó hiện **Đang chờ node**, và phiên bản chủ sẽ cho nó biết gửi bản sao lưu đi đâu khi nó xuất hiện.

### Ví dụ: một VPS sao lưu tới máy chủ nhận {#example-vps}

Phiên bản chủ là một máy Unraid tên `tower`, chạy [máy chủ nhận](offsite-recovery.md#receiving-server). Một proxy ngược công bố nó thành `https://backup.tower.example.org`. VPS `vps-helsinki` chạy Nextcloud và Caddy, với dữ liệu nằm dưới `/srv/appdata`.

1. Trên `tower`, vào **Phiên bản**, **Thêm node**, đặt tên `vps-helsinki`, rồi **Tiếp**.
2. Chọn **Máy chủ nhận của phiên bản này** và gõ `https://backup.tower.example.org` vào **Địa chỉ cho node**. Các container của phiên bản chủ sao lưu lúc 02:30, nên cửa sổ đề xuất 03:00. Giữ nguyên và chọn **Tiếp**.
3. Trên VPS, lưu tệp từ cửa sổ, với các volume và `HOST_SOURCE_ROOT` đổi sang `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. Chạy `docker compose up -d`. Vài giây sau, cửa sổ trên `tower` báo node đã trực tuyến và **Bản sao lưu được gửi tới https://backup.tower.example.org.**
5. Lúc 03:00 phiên bản chủ bắt đầu sao lưu cho node. Sáng hôm sau, thẻ **Node** trên Tổng quan hiện bản sao lưu gần nhất của `vps-helsinki`, và **Bản sao lưu** liệt kê các container của nó trong một phần riêng.

Cách tìm dữ liệu của một container trên một host Docker thông thường được mô tả ở [Nhận diện nguồn sao lưu](configuration.md#backup-source-detection).

## Bản sao lưu đi đâu {#placement}

Mỗi node có một đích chung cho cả hai phần của nó. **Đổi đích** trên thẻ đổi được đích khi node đã trực tuyến. Nơi mới sẽ có kho mới, còn các bản sao lưu ở nơi cũ vẫn nằm nguyên đó.

### Máy chủ nhận của phiên bản này {#to-receiver}

Node ghi vào máy chủ nhận của phiên bản chủ ở chế độ append-only: nó thêm được bản sao lưu nhưng không xóa được bản nào. Nó có một thông tin đăng nhập riêng, đặt theo tên node, và mỗi phần một kho trong thư mục riêng của nó ở đó. Phiên bản chủ thêm khóa của chính mình vào từng kho và tự dọn dẹp chúng, vì node không làm được.

Node tới máy chủ qua **Địa chỉ cho node**, mà bạn đặt ở bước thứ hai hoặc trong cửa sổ **Máy chủ nhận**. Node thường gửi từ internet, nên địa chỉ phải là một trong hai dạng sau:

- `https://`, ví dụ sau một proxy ngược, Pangolin hoặc một tunnel. Khi máy chủ đưa ra chứng chỉ riêng thay vì chứng chỉ của một tổ chức cấp phát công khai, hãy dán chứng chỉ đó ở dạng PEM vào **Chứng chỉ**.
- `http://` với một địa chỉ IP riêng, như `10.0.0.5` trên một liên kết WireGuard hoặc `100.101.102.103` trên Tailscale.

`http://` thông thường tới một địa chỉ công khai sẽ bị từ chối, vì node sẽ gửi bản sao lưu qua internet mà không mã hóa. Khi bạn đổi địa chỉ, phiên bản chủ báo cho mọi node dùng máy chủ nhận này, và node nào đang vắng mặt sẽ biết khi quay lại.

### Một đích sao lưu {#to-location}

Node ghi vào một trong các đích sao lưu của phiên bản chủ mà nó tự tới được: một dịch vụ S3 hoặc một máy chủ REST, qua `https://` hoặc ở một địa chỉ riêng. Kho của nó nằm ở `bombvault-node-<node id>-containers` và `bombvault-node-<node id>-files` tại đó.

Node đăng nhập bằng một [thông tin đăng nhập đã lưu](offsite-recovery.md#sign-ins) được tạo cho nó, và thông tin này phải khác với thông tin phiên bản chủ dùng để ghi vào đích đó. Nếu cửa sổ báo chưa có, hãy thêm một cái trước. Khác với trên máy chủ nhận, với thông tin đăng nhập này node thường xóa được những gì nằm ở đích, và cửa sổ cũng nói rõ điều đó.

### Giữ trên node {#keep-on-node}

Node tự giữ bản sao lưu của nó, sau một rest-server riêng ở chế độ append-only, và phiên bản chủ lấy một bản sao của từng phần về bộ nhớ của mình. Cách này hợp với node mà phiên bản chủ không có chỗ nào để cho. Đây là đích duy nhất cần mở cổng.

![Bước thứ hai với Giữ trên node được chọn, địa chỉ https://203.0.113.7:8443 và ghi chú về cổng cần mở.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Bước thứ hai với Giữ trên node được chọn, địa chỉ https://203.0.113.7:8443 và ghi chú về cổng cần mở.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Trong **Địa chỉ của node**, gõ nơi phiên bản chủ tới được node: địa chỉ công khai của node và một cổng còn trống, như `https://203.0.113.7:8443`, không có thư mục phía sau. Chỉ dùng `http://` với địa chỉ riêng, như ở trên.
2. Node khởi động container `bombvault-node-rest-server` trên cổng đó, với các kho nằm trong thư mục `bombvault-node` dưới Host Data. Với `https://`, node tạo một chứng chỉ cho địa chỉ đó, và phiên bản chủ ghim chứng chỉ ấy.
3. Cho phép cổng đi qua tường lửa của node, để phiên bản chủ tới được. Một cổng mà container khác trên node đang giữ sẽ bị từ chối kèm tên của container đó.

Phiên bản chủ lấy một phần về ngay khi node sao lưu xong phần đó, và để dự phòng thì lấy thêm mỗi giờ vào phút thứ 15, kể cả khi **Kéo về** đang tắt. Những gì phiên bản chủ đã lấy về ở lại với phiên bản chủ như mọi [bản sao đã kéo về](offsite-recovery.md#receiver), kể cả sau khi node không còn. Node tự dọn dẹp kho của mình khi phiên bản chủ yêu cầu, vì rest-server của nó không nhận lệnh xóa, và các bộ thư mục của node bỏ qua thư mục `bombvault-node`, nên nó không bao giờ sao lưu chính các bản sao lưu của mình.

## Khi nào node sao lưu {#schedule}

Phiên bản chủ nắm lịch của mọi node. Thẻ hiện một kế hoạch cho từng phần, như **Containers** *hằng ngày lúc 2:00* và **Thư mục** *hằng tuần (CN) lúc 4:00*. Một cú nhấp mở cùng trình chỉnh sửa như trong [Lịch trình](schedules.md#plans), và thay đổi được lưu ngay.

![Kế hoạch cho container của một node mở từ thẻ của nó, với Tắt, Hằng ngày, Hằng tuần, Mỗi N ngày và Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Kế hoạch cho container của một node mở từ thẻ của nó, với Tắt, Hằng ngày, Hằng tuần, Mỗi N ngày và Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Một node từng vắng mặt** bắt đầu các bản sao lưu bị lỡ ngay khi quay lại. Phiên bản chủ kiểm tra mỗi năm phút.
- **Một node đang bận** được hỏi lại sau năm phút.
- **Trong lúc phiên bản chủ dọn dẹp** các bản sao lưu cũ của một node, node không bắt đầu gì cho đến khi phiên bản chủ xong. Trên máy chủ nhận và trên chính node, khung giờ này mở mỗi Chủ nhật lúc 03:00 và kéo dài tối đa sáu giờ. Ở một đích sao lưu, nó theo kế hoạch sao chép off-site của phiên bản chủ cho đích đó. Phiên bản chủ giữ bản sao lưu theo quy tắc của chính nó cho phần đó trong **Giữ trong bao lâu**, hoặc theo quy tắc của đích nếu đích có quy tắc riêng.
- **Trong danh sách lượt chạy của node**, các bản sao lưu và lần khôi phục do phiên bản chủ bắt đầu hiện là do phiên bản chủ bắt đầu.

## Theo dõi các node của bạn {#watch}

Thẻ **Node** trên Tổng quan liệt kê mọi node cùng trạng thái, bản sao lưu gần nhất, lỗi và các bất thường còn mở của nó. Thẻ xuất hiện khi bạn đã có một node.

![Thẻ Node trên Tổng quan với ba node: một đã kết nối, một không hoạt động với hai lỗi, và một vẫn đang chờ node.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Thẻ Node trên Tổng quan với ba node: một đã kết nối, một không hoạt động với hai lỗi, và một vẫn đang chờ node.](assets/screenshots/nodes/overview-card-light.png#only-light)

Phiên bản chủ gửi một cảnh báo cho mỗi sự cố qua các [kênh thông báo](notifications.md) của nó, và chỉ gửi cảnh báo tiếp theo sau khi sự cố đó đã qua:

- **Một bản sao lưu thất bại**, như *BombVault: vps-helsinki: backup failed*, kèm mục và lý do. Một lần sao lưu sau đó của mục ấy thành công thì sự cố kết thúc.
- **Node ngoài tầm với**, như *BombVault: vps-helsinki is out of reach*. Phiên bản chủ chờ theo công tắc người chết ngắn nhất trên các kho của node, 26 giờ nếu bạn không đổi, hoặc 6 giờ khi node chưa có đích.

Việc theo dõi của bộ nhận trên phiên bản chủ cũng trông chừng từng kho của node, với một công tắc người chết và một lượt `restic check` hằng ngày lúc 04:00, xem [Bên nhận và bên kéo](offsite-recovery.md#receiver).

## Các mục của một node {#entries}

Trong **Bản sao lưu**, danh sách Containers và Thư mục có một phần cho mỗi node, nằm dưới các mục của chính bạn. Container của chính node không có trong đó. **Sao lưu ngay** trong menu của một mục bắt đầu sao lưu mục đó trên node, còn **Mở** mở trang của nó.

Trong phần của một node ở danh sách Thư mục, **Thêm bộ thư mục** thêm một bộ thư mục trên node, còn **Chỉnh sửa** trong menu của một bộ sẽ thay đổi bộ đó. Cả hai mở cùng cửa sổ như với các bộ của chính bạn. Trình chọn thư mục trong đó duyệt các thư mục của node, nhưng không tạo thư mục mới ở đó.

![Phần của node vps-falkenstein trong danh sách Containers: nextcloud, nextcloud-db, caddy và uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Phần của node vps-falkenstein trong danh sách Containers: nextcloud, nextcloud-db, caddy và uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Trang của một mục trên node ghi tên node, như **Trên vps-falkenstein**, và liệt kê các bản sao lưu của mục. **Khôi phục** trên một bản sẽ khôi phục mục đó trên node và thay thế những gì đang có ở đó. Container được dừng trong lúc này, như ở phiên bản chủ.

![Trang của nextcloud trên vps-falkenstein với các bản sao lưu và một nút Khôi phục bên cạnh mỗi bản.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Trang của nextcloud trên vps-falkenstein với các bản sao lưu và một nút Khôi phục bên cạnh mỗi bản.](assets/screenshots/nodes/entry-light.png#only-light)

Qua phiên bản chủ, bạn tới được một danh sách cố định các route riêng của node: container, bộ thư mục, lượt chạy và bất thường của nó, cùng việc sao lưu và khôi phục chúng. Cài đặt, khóa và bộ nhớ của node nằm ngoài tầm với, và không có gì trong danh sách đó xóa được. Xem [Node trong API](api.md#nodes).

## Thay thế và giải phóng {#replace-release}

**Thay thế** hiện lại tệp compose của node, sau khi đăng nhập bằng mật khẩu. Dùng nó để chuyển node sang một máy chủ khác hoặc dựng lại node: khởi động tệp ở máy mới và tắt máy chủ cũ. Cùng `APP_KEY` và `NODE_HOME` sẽ khiến nó lại là node cũ, với các bản sao lưu và lịch sử của nó. Phiên bản chủ giữ cho mỗi node các bộ thư mục, các container cùng những gì cần để tạo lại chúng, và các lượt chạy. Khi node khởi động với một thư mục `config` trống, phiên bản chủ nhận nó lại, cho nó biết bản sao lưu đi đâu và trả lại tất cả những thứ đó trước lần sao lưu tiếp theo, nhờ vậy node có thể khôi phục một container không còn nữa. Trên một máy chủ mới, node tự giữ bản sao lưu sẽ bắt đầu với các kho trống. Các bản sao lưu trước đó vẫn nằm trong những bản sao mà phiên bản chủ đã lấy về. Hai máy chủ có cùng `APP_KEY` được tính là một node, nên đừng bao giờ chạy cả hai.

**Giải phóng** để node ra đi. Nó thôi nhận lệnh từ phiên bản chủ và không sao lưu gì nữa. Các bản sao lưu của nó vẫn ở nguyên chỗ, và mọi bản sao phiên bản chủ đã lấy về cũng vậy. Thông tin đăng nhập của nó trên máy chủ nhận bị gỡ, và phiên bản chủ thôi theo dõi các kho của nó. Một node đang vắng mặt khi bạn giải phóng sẽ giữ thẻ ở trạng thái **Đang giải phóng** cho đến khi nó nhận được tin. Một node đã giải phóng không nhận phiên bản chủ nào nữa với cùng `APP_KEY`. Để dùng lại máy chủ, hãy thêm nó như một node mới và khởi động bằng tệp compose mới cùng một thư mục `config` trống.

## Bảo mật {#security}

Nếu phiên bản chủ bị chiếm, kẻ tấn công cũng điều khiển được các node của nó, và một node nắm Docker trên máy chủ của nó, tức là quyền root ở đó. Hãy bảo vệ phiên bản chủ bằng mật khẩu và đừng để nó lộ ra internet. Nếu một node bị chiếm, kẻ tấn công không có được nhóm ghép nối, pháo đài, node khác hay kho khác nào, và không xóa được lịch sử trên máy chủ nhận. Chi tiết nằm ở [Node trong mô hình bảo mật](configuration.md#nodes).

[Bộ khôi phục](offsite-recovery.md#recovery-kit) có một chương **Node** với `APP_KEY`, mật khẩu restic và các kho của từng node, để bạn có thể đưa một node trở lại hoặc khôi phục bản sao lưu của nó bằng restic thuần.
