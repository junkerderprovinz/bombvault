# Máy chủ MCP

BombVault có sẵn một máy chủ cho Model Context Protocol (MCP), giao thức mà các trợ lý AI như Claude Code và Claude Desktop dùng để truy cập công cụ bên ngoài. Qua máy chủ này, một trợ lý có thể đọc tình trạng các bản sao lưu của bạn và, nếu bạn cho phép, bắt đầu một lần sao lưu hoặc hủy một lần sao lưu do chính nó bắt đầu. Máy chủ tắt cho đến khi bạn tạo một khóa: khi không có khóa nào đang hoạt động, điểm kết nối `/mcp` trả lời `404` cho mọi yêu cầu.

## Trợ lý làm được gì và không làm được gì {#tools}

| Công cụ | Chức năng | Loại |
|---|---|---|
| `get_health` | Phiên bản, tên phiên bản cài đặt, có đang sao lưu không và khóa này được phép làm gì | đọc |
| `get_status` | Trạng thái bảo vệ theo từng miền: lần sao lưu thành công gần nhất, khoảng thời gian dự kiến, các lần xác minh và kiểm tra off-site, các lần chạy theo lịch tiếp theo | đọc |
| `get_coverage` | Những gì BombVault bảo vệ và không bảo vệ, kèm lý do cho từng mục | đọc |
| `list_items` | Mọi container, VM và bộ thư mục được bảo vệ, ổ flash và cấu hình ứng dụng, kèm lịch, những gì một lần sao lưu sẽ dừng, lần sao lưu gần nhất và thời gian của nó; container cơ sở dữ liệu còn cho biết bản dump gần nhất; các dataset ZFS cũng được liệt kê, kèm kết quả lần kiểm tra gần nhất | đọc |
| `list_runs` | Lịch sử chạy, mới nhất trước, lọc được theo miền, mục, trạng thái, loại và thời gian | đọc |
| `list_restore_points` | Các điểm khôi phục của một mục trong kho chính của nó, và với container thì có cả các bản dump cơ sở dữ liệu; một dataset ZFS có một điểm khôi phục cho mỗi lần sao lưu, kèm snapshot của mọi dataset bên dưới nó | đọc |
| `get_activity` | Những gì đang chạy ngay lúc này, kèm giai đoạn và phần trăm | đọc |
| `get_storage_stats` | Lịch sử dung lượng kho chính của một miền và mức tăng mỗi tuần | đọc |
| `list_anomalies` | Những bất thường mà BombVault nhận thấy trong các bản sao lưu, lọc được theo trạng thái, mức độ nghiêm trọng và miền, kèm bản tóm tắt những mục còn mở | đọc |
| `get_anomaly` | Một trong các phát hiện đó, kèm ghi chú để lại khi xác nhận | đọc |
| `start_backup` | Sao lưu ngay một mục | bắt đầu |
| `start_domain_backup` | Sao lưu mọi mục được bảo vệ trong một miền | bắt đầu |
| `start_backup_everything` | Chạy lượt Backup Everything | bắt đầu |
| `cancel_backup` | Hủy một lần sao lưu đang chạy do khóa này bắt đầu | hủy |

Những việc sau vẫn nằm trong giao diện web: mọi kiểu khôi phục (kể cả tải xuống, lưu hoặc nhập một bản dump cơ sở dữ liệu), xóa bản sao lưu, prune, unlock, kiểm tra và diễn tập, sao chép off-site, cài đặt, thông tin đăng nhập và khóa MCP, cùng việc hủy một lần sao lưu do lịch, giao diện web hoặc khóa khác bắt đầu. Việc xác nhận một bất thường hoặc đánh dấu nó là dự kiến cũng vậy, và được thực hiện trên trang **Bất thường**. Lý do: câu trả lời của công cụ chứa tên và thông báo lỗi từ máy chủ của bạn, và bất kỳ mục nào trong đó cũng có thể chứa văn bản viết ra để điều khiển trợ lý. Một trợ lý mắc bẫy văn bản như vậy, trong trường hợp xấu nhất, chỉ có thể bắt đầu một lần sao lưu trong các giới hạn bên dưới hoặc hủy một lần sao lưu do chính nó bắt đầu.

Nếu kho chính của một mục nằm ở nơi khác (S3, REST, SFTP, rclone), `list_restore_points` sẽ kết nối tới đó và lệnh gọi có thể mất một lúc. Không thể liệt kê bản sao off-site qua MCP. Những gì các bước kiểm tra bất thường xem xét được mô tả trong [Tính năng](features.md), và cách một mục ZFS giữ một snapshot cho mỗi tập dữ liệu được mô tả trong [Tập dữ liệu ZFS](zfs-datasets.md#contents).

## Một lần sao lưu được bắt đầu sẽ làm gì {#starting-backups}

Lần sao lưu do trợ lý bắt đầu giống hệt lần sao lưu do giao diện web bắt đầu. Container đang chạy sẽ dừng cho đến khi sao lưu xong, cùng với các container được đặt để dừng theo nó. VM dùng phương thức "graceful" sẽ được tắt rồi khởi động lại. Một dataset ZFS dừng các container được đặt cho nó trong lúc chụp snapshot. Bộ thư mục, ổ flash và cấu hình vẫn tiếp tục chạy. Sau đó BombVault áp dụng chính sách lưu giữ và có thể sao chép sang kho off-site. `list_items` cho trợ lý biết một mục sẽ dừng những gì và lần sao lưu gần nhất mất bao lâu, còn mô tả của công cụ yêu cầu trợ lý báo cho bạn trước khi bắt đầu bất cứ việc gì.

Vì sao lưu làm dừng dịch vụ và đẩy các điểm khôi phục cũ ra ngoài, việc bắt đầu qua MCP có giới hạn:

- 12 lần sao lưu được bắt đầu mỗi giờ cho mỗi khóa.
- 15 phút giữa hai lần bắt đầu qua MCP của cùng một mục, cùng một miền hoặc Backup Everything.
- Tối đa 4 lần bắt đầu qua MCP cho cùng một mục trong 24 giờ.
- **Bảo vệ lưu giữ.** Khi một miền giữ một số lượng điểm khôi phục cố định (chỉ "giữ N bản gần nhất", không có quy tắc theo ngày, tuần hay tháng, ở máy cục bộ hay ở đích off-site), mỗi lần sao lưu mới sẽ đẩy bản cũ nhất ra ngoài. Khi đó BombVault từ chối việc bắt đầu qua MCP cho một mục mà N-1 lần sao lưu thành công gần nhất đều được bắt đầu qua MCP. Nhờ vậy, trong tập được giữ luôn còn ít nhất một điểm khôi phục do lịch hoặc do bạn tạo. Với "giữ 1 bản gần nhất", trợ lý hoàn toàn không thể sao lưu mục đó. Lần sao lưu theo lịch tiếp theo sẽ lại tạo chỗ trống.

Khi bắt đầu một miền hoặc Backup Everything, các mục bị một giới hạn giữ lại sẽ bị bỏ qua và được nêu tên trong câu trả lời. Không giới hạn nào trong số này áp dụng cho giao diện web và lịch. Hạn mức mỗi giờ nằm trong bộ nhớ, nên khởi động lại BombVault sẽ đặt nó về không.

## Bật tính năng {#switch-on}

1. Mở **Cài đặt, Hệ thống, Máy chủ MCP** và nhấn **Khóa mới**.
2. Đặt cho khóa một tên cho biết nơi dùng nó, ví dụ "Claude Code trên laptop". Mỗi máy khách một khóa thì bạn có thể thu hồi một khóa mà không động đến các khóa khác.
3. Để bật **Cho phép bắt đầu sao lưu**, hoặc tắt nó cho khóa chỉ cần đọc. Bạn có thể đổi sau ở dòng của khóa, và thay đổi có hiệu lực từ yêu cầu tiếp theo của trợ lý mà không cần kết nối lại.
4. Nhấn **Tạo khóa**. Khóa chỉ hiện một lần. BombVault chỉ giữ dấu vân tay của khóa và không thể hiện lại, nên hãy sao chép ngay hoặc dùng một trong các đoạn mã bên dưới, lúc đó chứa khóa thật.

Khi không có mật khẩu đăng nhập, chính giao diện web đã mở cho mọi người trong mạng của bạn, và ai mở được nó cũng có thể tạo khóa. Thẻ sẽ báo điều này. Nếu bạn mở BombVault bằng một tên trông như công khai (ví dụ `bombvault.example.com` sau một reverse proxy) và chưa đặt mật khẩu đăng nhập, thì không thể tạo hay thay khóa từ địa chỉ đó, để không trang web nào trên Internet có thể khiến trình duyệt của bạn tạo khóa. Hãy đặt mật khẩu đăng nhập, hoặc mở BombVault bằng địa chỉ IP hay một tên cục bộ như `tower` hoặc `tower.local`.

## Kết nối máy khách {#clients}

Thẻ hiển thị sẵn các đoạn mã cho địa chỉ bạn dùng để mở nó: chọn máy khách và sao chép đoạn mã. Phần còn lại giải thích các đoạn mã làm gì và đưa ra những dạng mà thẻ không hiển thị.

### Claude Code {#claude-code}

Chạy lệnh trong thẻ một lần trong terminal. Với chứng chỉ mà máy tính của bạn tin cậy, lệnh trông như sau:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Kiểm tra kết nối bằng `/mcp` bên trong Claude Code. `--scope user` lưu khóa trong cấu hình người dùng của bạn chứ không phải trong tệp dự án.

Lệnh này chứa khóa, và shell của bạn có thể lưu nó trong lịch sử. Để tránh điều đó, hãy đặt một tệp `.mcp.json` trong thư mục dự án và giữ khóa trong một biến môi trường. Claude Code thay `${BOMBVAULT_MCP_KEY}` bằng giá trị khi đọc tệp:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

Đặt `BOMBVAULT_MCP_KEY` ở nơi Claude Code khởi động, ví dụ trong profile của shell, bằng trình soạn thảo văn bản chứ không gõ ở dấu nhắc lệnh. Đừng bao giờ commit một tệp `.mcp.json` có ghi khóa bên trong.

Với chứng chỉ riêng của BombVault (xem [TLS và chứng chỉ](#tls)), lệnh trong thẻ sẽ chạy `mcp-remote` thay vào đó và chỉ cho Node.js tới chứng chỉ đã tải về:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Dấu nháy đơn ngăn shell khai triển biến; việc đó do chính `mcp-remote` làm. Dạng này cũng dùng được trong `.mcp.json`: dùng mục của Claude Desktop bên dưới và bỏ `BOMBVAULT_MCP_KEY` khỏi `env` của nó, khi đó khóa lấy từ môi trường của bạn.

### Claude Desktop {#claude-desktop}

Claude Desktop kết nối tới BombVault qua `mcp-remote`, vốn cần Node.js trên máy tính đó. Mở tệp cấu hình trong Claude Desktop qua **Settings, Developer, Edit Config**. Tệp nằm ở `%APPDATA%\Claude\claude_desktop_config.json` trên Windows và ở `~/Library/Application Support/Claude/claude_desktop_config.json` trên macOS. Thêm mục từ thẻ vào bên trong `"mcpServers"`, cạnh các máy chủ đã có, rồi khởi động lại Claude Desktop:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` chỉ có mặt vì chứng chỉ riêng của BombVault. Sau một chứng chỉ mà máy tính của bạn đã tin cậy, hãy bỏ nó đi.
- `--allow-http` chỉ được thêm cho địa chỉ `http://` thông thường.
- Header được viết là `X-API-Key:${BOMBVAULT_MCP_KEY}`, không có khoảng trắng sau dấu hai chấm và khóa nằm trong `env`. Trên một số hệ thống, `mcp-remote` tách giá trị của `--header` tại khoảng trắng đầu tiên, và khóa viết sau khoảng trắng sẽ bị mất.

### Connector tùy chỉnh trong cài đặt của Claude {#custom-connectors}

Các connector bạn thêm trong cài đặt của chính Claude (trên claude.ai và trong danh sách connector của Claude Desktop) hiện chưa được hỗ trợ. Các connector này được gọi từ đám mây của Anthropic, nên cần một địa chỉ HTTPS công khai, và chúng đăng nhập qua OAuth. Chúng không gửi được khóa cố định, còn BombVault chỉ cung cấp khóa cố định, không có đăng nhập OAuth. Đưa BombVault lên Internet vì chúng cũng không giúp được gì. Hãy dùng Claude Code, hoặc Claude Desktop qua `mcp-remote` như trên.

### Máy khách khác {#other-clients}

Bất kỳ máy khách nào nói Streamable HTTP đều dùng được:

- URL: địa chỉ của giao diện web thêm `/mcp`, ví dụ `https://192.168.1.10:3443/mcp`.
- Khóa trong `Authorization: Bearer <key>` hoặc trong `X-API-Key: <key>`. Nếu gửi cả hai thì phải cùng một khóa.
- `POST` với `Content-Type: application/json` và `Accept: application/json, text/event-stream`.
- Một thông điệp JSON-RPC cho mỗi yêu cầu; yêu cầu gộp (batch) bị từ chối.
- Các phiên bản giao thức 2026-07-28, 2025-11-25, 2025-06-18 và 2025-03-26.

## TLS và chứng chỉ {#tls}

BombVault phục vụ HTTPS bằng chứng chỉ tự cấp, và ban đầu chứng chỉ này chỉ ghi `localhost`, `127.0.0.1` và `::1`. Claude Code và `mcp-remote` từ chối nó trên một địa chỉ mạng LAN. Các cách xử lý, theo thứ tự phù hợp với phần lớn các bản cài Unraid:

1. **Thêm địa chỉ trong thẻ MCP.** Khi mở thẻ qua HTTPS ở một địa chỉ mà chứng chỉ không ghi, thẻ sẽ báo và đề xuất **Thêm địa chỉ này vào chứng chỉ**. BombVault sẽ cấp lại chứng chỉ có kèm địa chỉ đó (trình duyệt sẽ cảnh báo thêm một lần, như lần đầu). Sau đó nhấn **Tải chứng chỉ**; các đoạn mã đặt `NODE_EXTRA_CA_CERTS` trỏ tới tệp đã tải, nên máy khách tin cậy đúng chứng chỉ đó.
2. **Một reverse proxy với chứng chỉ đáng tin cậy** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Khi đó máy khách thấy chứng chỉ của proxy và không cần gì thêm, và thẻ cũng không cảnh báo về chứng chỉ của BombVault.
3. **Tailscale.** `tailscale serve` đặt trước container, hoặc tích hợp Tailscale của Unraid, cho bạn một tên `ts.net` với chứng chỉ đáng tin cậy.
4. **`HTTP_ONLY=true`**, chỉ dùng sau một proxy kết thúc TLS hoặc trong mạng bạn hoàn toàn tin cậy. Nó chuyển toàn bộ giao diện web sang HTTP thường, cần sửa cài đặt container và gửi khóa không mã hóa.

Đừng bao giờ đặt `NODE_TLS_REJECT_UNAUTHORIZED=0`. Việc này tắt kiểm tra chứng chỉ cho mọi thứ mà tiến trình Node.js đó giao tiếp.

Reverse proxy phải chuyển tiếp header `Authorization` (hoặc `X-API-Key`), điều mà proxy vẫn làm trừ khi được cấu hình khác, và không được đệm hay viết lại `/mcp`. Một khối location cho Nginx hoặc Nginx Proxy Manager có kiểm tra cả chứng chỉ của BombVault:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Sau một proxy, mọi yêu cầu đều mang địa chỉ của proxy. Khi đó năm khóa sai từ một máy khách cấu hình sai sẽ chặn mọi máy khách MCP sau proxy đó trong một phút. Hãy khai báo proxy trong `TRUSTED_PROXY` (xem [Cấu hình](configuration.md)) để đếm riêng theo từng máy khách.

## Mô hình bảo mật {#security}

- Khi không có khóa đang hoạt động, `/mcp` trả lời `404`.
- Không địa chỉ nào được miễn. Yêu cầu từ `localhost`, từ máy chủ Unraid, từ reverse proxy hay từ `tailscale serve` đều cần khóa như mọi yêu cầu khác, kể cả khi giao diện web không có mật khẩu đăng nhập.
- Khóa chỉ được lưu dưới dạng dấu vân tay, chỉ hiện một lần, và có thể đổi tên, thay thế, thu hồi. Tối đa 10 khóa đang hoạt động, mỗi khóa có công tắc **Được phép bắt đầu sao lưu** riêng.
- Mỗi lần tạo, thay, đổi quyền và thu hồi đều gửi một thông báo qua các kênh thông báo của bạn, kèm địa chỉ gửi yêu cầu, trừ khi thông báo đang tắt.
- 5 khóa sai mỗi phút cho mỗi địa chỉ, sau đó là `429`. 120 yêu cầu mỗi phút và 12 lần bắt đầu sao lưu mỗi giờ cho mỗi khóa, cộng thêm thời gian chờ và bảo vệ lưu giữ nêu trên.
- Yêu cầu từ trang trình duyệt có nguồn gốc (origin) khác bị từ chối.
- Chừng nào chưa đặt mật khẩu đăng nhập thì không thể tạo khóa từ một tên máy chủ trông như công khai.
- Mọi lần sao lưu do trợ lý bắt đầu, cùng các lần chạy prune và off-site kéo theo, đều được đánh dấu "qua MCP" kèm tên khóa trong nhật ký hoạt động, trong bảng lỗi và trong thông báo sao lưu.
- Mọi lệnh gọi công cụ được ghi vào nhật ký container cùng id của khóa và bốn ký tự cuối (không bao giờ ghi tên) và được đếm trong `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Khôi phục một bản sao lưu cấu hình sẽ thu hồi mọi khóa, vì cơ sở dữ liệu được khôi phục có thể chứa các khóa bạn đã thu hồi sau khi nó được lưu. Hãy tạo khóa mới sau đó.
- Khóa ngừng hoạt động khi `APP_KEY` thay đổi (cài lại, hoặc khôi phục sang container khác). Thẻ phát hiện điều này và đánh dấu khóa, còn **Thay khóa** cấp lại cho nó một bí mật hợp lệ.
- Hãy đối xử với khóa như mật khẩu. Claude Code và Claude Desktop lưu khóa dạng văn bản thường trong cấu hình của chúng. Trên máy tính bạn ít tin cậy hơn, nên dùng khóa chỉ đọc.

## Những gì rời khỏi máy {#privacy}

Mọi thứ trợ lý đọc đều được gửi tới nhà cung cấp AI đứng sau nó: tên mục, lịch, lịch sử chạy kèm thông báo lỗi, id và thời điểm của các điểm khôi phục, tên các engine cơ sở dữ liệu và kích thước bản dump, hoạt động đang diễn ra, số liệu lưu trữ, độ bao phủ và trạng thái. BombVault loại bỏ đường dẫn trên máy chủ, vị trí kho, tên máy chủ, thông tin đăng nhập, lệnh hook và khóa trước khi bất cứ thứ gì rời đi.

## Khắc phục sự cố {#troubleshooting}

| Bạn thấy gì | Ý nghĩa |
|---|---|
| `404` | Không có khóa đang hoạt động, hoặc sai đường dẫn như `/api/mcp`. Điểm kết nối là `/mcp`. |
| `401` | Thiếu khóa, gõ sai, đã thu hồi hoặc đã thay. Có thể proxy làm rơi header `Authorization` (thử `X-API-Key`). Nếu thẻ đánh dấu khóa không còn hợp lệ, `APP_KEY` đã thay đổi: hãy thay khóa. |
| `403` | Yêu cầu đến từ trang trình duyệt có nguồn gốc khác. Hãy dùng máy khách trên máy tính hoặc dòng lệnh. |
| `405` với GET | Bình thường. Điểm kết nối chỉ nhận `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Máy khách quá cũ cho Streamable HTTP. Hãy cập nhật. |
| `400` "batch requests are not accepted" | Máy khách gửi yêu cầu gộp JSON-RPC. Hãy gửi một thông điệp cho mỗi yêu cầu. |
| `429` | Quá nhiều khóa sai từ địa chỉ này, hoặc hơn 120 yêu cầu mỗi phút với một khóa. Đợi một phút và kiểm tra xem trợ lý có bị kẹt trong vòng lặp không. |
| Lỗi có "certificate", "self-signed" hoặc "unable to verify" | Máy khách không tin cậy chứng chỉ của BombVault. Xem [TLS và chứng chỉ](#tls). |
| `busy` | Một lần sao lưu khác hoặc tác vụ bảo trì đang chiếm miền đó. Thử lại khi nó xong. |
| `cooldown` | Mục này, miền này hoặc Backup Everything đã được bắt đầu qua MCP chưa đầy 15 phút trước. |
| `retention_guard` | Thêm một lần sao lưu qua MCP nữa sẽ khiến khoảng "giữ N bản gần nhất" chỉ còn các điểm khôi phục từ MCP. Lần sao lưu theo lịch tiếp theo sẽ tạo chỗ trống, hoặc hãy bắt đầu nó trong giao diện web. |
| `rate_limited` | Khóa đã dùng hết 12 lần bắt đầu của giờ này. |
| `not_permitted` khi bắt đầu | Khóa chỉ được đọc. Bật **Được phép bắt đầu sao lưu** trong thẻ; không cần kết nối lại. Khi hủy, nó có nghĩa là lần chạy đó không do khóa này bắt đầu. |
| `domain_off` | Loại sao lưu đó đang tắt trong cài đặt. |
| `not_found` | BombVault không bảo vệ mục đó. Hãy thêm nó trong giao diện web trước; MCP không bao giờ tạo cấu hình. |

Đừng đặt biến môi trường `MCPGODEBUG` trên container. Nó thay đổi cách thư viện MCP hoạt động, và một giá trị sai định dạng sẽ khiến BombVault dừng ngay khi khởi động, trước cả khi ghi được một dòng nhật ký nào.
