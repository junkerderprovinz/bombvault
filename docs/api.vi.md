# API và tích hợp

BombVault có một API HTTP nhỏ cho script, bảng điều khiển và nhà thông minh. API đọc những gì bảng điều khiển hiển thị và có thể bắt đầu sao lưu. Mọi thứ còn lại, như khôi phục, xóa bản sao lưu và cài đặt, vẫn ở trong giao diện web.

## Mã thông báo {#tokens}

Mỗi yêu cầu cần một mã thông báo API, kể cả khi chưa đặt mật khẩu đăng nhập. Tạo mã tại **Cài đặt, Hệ thống, Mã thông báo API**:

1. Nhập một cái tên cho biết mã được dùng ở đâu, ví dụ "Home Assistant" hoặc "Uptime Kuma".
2. Bật **Cho phép bắt đầu sao lưu** nếu mã cần bắt đầu sao lưu. Nếu không, mã chỉ đọc được.
3. Bấm **Tạo mã thông báo**. Mã chỉ hiện một lần. BombVault chỉ giữ dấu vân tay của nó, nên hãy sao chép ngay.

Gửi mã trong một tiêu đề, `Authorization: Bearer <token>` hoặc `X-API-Key: <token>`. Mã bắt đầu bằng `bvapi_`. Nó chỉ mở API: khóa MCP không dùng được ở đây, và mã thông báo không dùng được cho MCP.

Mỗi mã có một ô hiển thị tên, có được bắt đầu sao lưu không, bốn ký tự cuối, thời điểm và nơi dùng gần nhất, và số lượt gọi hôm nay. Trên ô, bạn có thể đổi tên, đổi quyền, thay hoặc thu hồi mã. **Nhật ký** hiển thị các bản sao lưu mã đã bắt đầu và các lượt gọi gần đây. Khôi phục cấu hình BombVault từ bản sao lưu sẽ thu hồi mọi mã, vì bản sao lưu có thể chứa mã bạn đã thu hồi sau đó.

Khi chưa có mật khẩu đăng nhập, ai mở được giao diện web cũng tạo được mã. Nếu bạn mở BombVault bằng một tên trông như công khai và chưa có mật khẩu, không thể tạo mã từ địa chỉ đó, giống như với [khóa MCP](mcp.md#switch-on).

## Điểm cuối {#endpoints}

| Tuyến | Trả về hoặc làm gì | Mã |
|---|---|---|
| `GET /api/v1/health` | Phiên bản, tên phiên bản chạy, có bản sao lưu đang chạy không và mã này được làm gì | đọc |
| `GET /api/v1/status` | Trạng thái bảo vệ theo miền: bản sao lưu thành công gần nhất, khoảng thời gian dự kiến, kiểm tra, lần chạy theo lịch tiếp theo | đọc |
| `GET /api/v1/activity` | Những gì đang chạy lúc này, kèm giai đoạn và phần trăm | đọc |
| `GET /api/v1/items` | Mọi mục được bảo vệ với lịch, những gì một bản sao lưu dừng và bản sao lưu gần nhất; `?domain=` cho một miền | đọc |
| `GET /api/v1/runs` | Lịch sử chạy, mới nhất trước; bộ lọc `limit`, `domain`, `item`, `status`, `kind`, `since` | đọc |
| `GET /api/v1/anomalies` | Bất thường kèm tóm tắt những mục còn mở; bộ lọc `state`, `severity`, `domain`, `limit` | đọc |
| `GET /api/v1/anomalies/{id}` | Một bất thường | đọc |
| `GET /api/v1/storage/{domain}` | Lịch sử dung lượng, mức tăng mỗi tuần và dung lượng trống của từng kho trong một miền | đọc |
| `POST /api/v1/backups` | Sao lưu một mục (`{"domain":"containers","item":"plex"}`) hoặc cả miền (`{"domain":"vms"}`) | bắt đầu |
| `POST /api/v1/backups/everything` | Chạy Backup Everything | bắt đầu |
| `POST /api/v1/runs/{id}/cancel` | Hủy một bản sao lưu đang chạy do mã này bắt đầu | bắt đầu |

Các miền là `containers`, `vms`, `files`, `zfs`, `flash` và `config`. Thời gian tính bằng giây Unix. Câu trả lời giống với [công cụ MCP](mcp.md#tools) cùng tên, nên hai bên luôn khớp nhau.

Bản sao lưu bắt đầu từ đây giống bản sao lưu mà giao diện web bắt đầu: container đang chạy sẽ dừng cho tới khi bản sao lưu xong. Yêu cầu trả về ngay, và `/api/v1/activity` cùng `/api/v1/runs` cho thấy tiến trình.

## Ví dụ {#examples}

```sh
# Các bản sao lưu thế nào rồi?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Sao lưu một container ngay bây giờ.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Với chứng chỉ tự ký của BombVault, thêm `--cacert bombvault-cert.pem` (tệp lấy từ **Tải chứng chỉ** trên thẻ MCP) hoặc `-k` trên mạng bạn tin cậy.

## Lỗi và giới hạn {#errors}

Lỗi trả về dưới dạng `{"error": {"code": "...", "message": "..."}}` kèm trạng thái tương ứng:

| Trạng thái | Mã lỗi | Ý nghĩa |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Thiếu tham số hoặc tham số sai |
| 401 | `no_token`, `invalid_token` | Không có mã, hoặc mã không còn hoạt động |
| 403 | `not_permitted` | Mã chỉ được đọc, hoặc không phải mã đã bắt đầu lần chạy đó |
| 404 | `not_found` | Không có mục, lần chạy hoặc bất thường đó |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Đã có việc khác đang chạy, miền đang tắt, hoặc không có gì để làm |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Một giới hạn giữ yêu cầu lại; `Retry-After` cho biết khi nào thử lại |

Việc bắt đầu tuân theo cùng giới hạn như [bắt đầu qua MCP](mcp.md#starting-backups): 12 lần mỗi giờ cho mỗi mã, cách nhau 15 phút giữa hai lần bắt đầu cùng một mục, tối đa 4 lần cho một mục trong 24 giờ, và cơ chế bảo vệ lưu giữ. Ba giới hạn sau tính chung các lần bắt đầu qua MCP, qua API và từ Home Assistant. Một mã được gửi 120 yêu cầu mỗi phút. Năm lần thử thất bại từ một địa chỉ sẽ khóa địa chỉ đó trong một phút.

## OpenAPI {#openapi}

BombVault cung cấp mô tả các tuyến này tại `/api/v1/openapi.json` (OpenAPI 3.1). Không cần mã thông báo. Hãy nạp nó vào Swagger UI, Postman hoặc một trình tạo mã.

## Home Assistant {#home-assistant}

BombVault có thể xuất hiện trong Home Assistant như một thiết bị, nhờ cơ chế khám phá MQTT. Home Assistant cần tích hợp MQTT của nó và một broker, ví dụ tiện ích bổ sung Mosquitto. Không cần thành phần riêng nào.

1. Trong BombVault, mở **Cài đặt, Hệ thống, Home Assistant**.
2. Nhập địa chỉ và cổng của broker, cùng tên người dùng và mật khẩu nếu broker yêu cầu. Bật **Dùng TLS** nếu broker dùng TLS, thường ở cổng 8883; chứng chỉ của broker phải hợp lệ cho địa chỉ bạn nhập.
3. Bật **Kết nối với Home Assistant** và bấm **Lưu**. Thẻ sẽ cho biết khi kết nối đã thông.

Thiết bị có tên BombVault, hoặc BombVault kèm tên phiên bản chạy trong ngoặc, và có các thực thể sau:

| Thực thể | Hiển thị gì |
|---|---|
| Status | `ok`, `warning`, `failed` hoặc `off`, trạng thái tệ nhất trong các miền đang bật |
| Running job | Việc đang chạy, hoặc `idle` |
| Open anomalies | Số bất thường còn mở |
| Next scheduled backup | Khi nào bản sao lưu theo lịch tiếp theo bắt đầu |
| *Miền* last backup | Khi nào bản sao lưu thành công gần nhất của miền chạy |
| *Miền* last result | Bản sao lưu gần nhất kết thúc ra sao |
| *Miền* repository free space | Dung lượng trống nơi đặt kho chính của miền, nếu BombVault đọc được |
| Back up *miền* | Nút sao lưu cả miền |

Tên thực thể bằng tiếng Anh, vì Home Assistant dùng đúng tên BombVault gửi. Mỗi miền đang bật có thực thể riêng, và miền bạn tắt sẽ mất chúng. Các nút tuân theo cùng giới hạn như [bắt đầu qua API](#errors). Ai công bố được lên broker đều bấm được, nên hãy đặt mật khẩu cho broker hoặc tắt **Nút bấm bắt đầu sao lưu**.

BombVault đọc trạng thái của mình mỗi 15 giây và công bố khi có thay đổi, dạng JSON tại `<tiền tố>/<nút>/state`. Tiền tố là `bombvault` nếu bạn không đổi, còn nút là một mã ngắn BombVault chọn một lần. Các thông điệp khám phá đi tới tiền tố mặc định của Home Assistant là `homeassistant`. Cả hai đều được giữ lại (retained). Thông điệp di chúc (last will) đánh dấu thiết bị là không khả dụng nếu BombVault dừng mà không báo trước. Khi tắt liên kết, BombVault gỡ thiết bị và các thực thể của nó khỏi Home Assistant.
