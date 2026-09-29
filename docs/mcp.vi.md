# Máy chủ MCP

BombVault có sẵn một máy chủ cho Model Context Protocol (MCP), giao thức mà các trợ lý AI như Claude Code và Claude Desktop dùng để tiếp cận công cụ bên ngoài. Qua đó, một trợ lý có thể đọc tình trạng các bản sao lưu của bạn và, nếu bạn cho phép, bắt đầu một bản sao lưu hoặc hủy một bản mà chính nó đã bắt đầu. Máy chủ tắt cho đến khi bạn tạo một khóa hoặc bật [đăng nhập qua OAuth](#oauth): trước đó endpoint `/mcp` trả `404` cho mọi yêu cầu.

## Trợ lý làm được gì và không làm được gì {#tools}

| Công cụ | Chức năng | Loại |
|---|---|---|
| `get_health` | Phiên bản, tên phiên bản cài đặt, có đang sao lưu không và khóa này được phép làm gì | đọc |
| `get_status` | Trạng thái bảo vệ theo từng miền: lần sao lưu thành công gần nhất, khoảng thời gian dự kiến, các lần xác minh và kiểm tra off-site, các lần chạy theo lịch tiếp theo, và với container là lần kiểm tra khởi động gần nhất | đọc |
| `get_coverage` | Những gì BombVault bảo vệ và không bảo vệ, kèm lý do cho từng mục | đọc |
| `list_items` | Mọi container, VM và bộ thư mục được bảo vệ, ổ flash và cấu hình ứng dụng, kèm lịch, những gì một lần sao lưu sẽ dừng, lần sao lưu gần nhất và thời gian của nó; container cơ sở dữ liệu còn cho biết bản dump gần nhất; các dataset ZFS cũng được liệt kê, kèm kết quả lần kiểm tra gần nhất; mỗi mục kèm lần kiểm tra khôi phục gần nhất, còn container kèm lần kiểm tra khởi động gần nhất hoặc lý do không thể kiểm tra; container được tạo lại với cài đặt khác kể từ lần sao lưu gần nhất liệt kê những gì đã đổi | đọc |
| `list_runs` | Lịch sử chạy, mới nhất trước, lọc được theo miền, mục, trạng thái, loại và thời gian; lần sao lưu chậm do một thứ kìm lại sẽ nêu tên thứ đó | đọc |
| `list_restore_points` | Các điểm khôi phục của một mục trong kho chính của nó, và với container thì có cả các bản dump cơ sở dữ liệu; một dataset ZFS có một điểm khôi phục cho mỗi lần sao lưu, kèm snapshot của mọi dataset bên dưới nó | đọc |
| `get_activity` | Những gì đang chạy ngay lúc này, kèm giai đoạn và phần trăm | đọc |
| `get_storage_stats` | Lịch sử dung lượng kho chính của một miền và mức tăng mỗi tuần, cùng dung lượng đã dùng, còn trống và tổng trên ổ đĩa hoặc nơi lưu từ xa của từng kho | đọc |
| `get_size_breakdown` | Thư mục và tệp nào chiếm chỗ trong bản sao lưu mới nhất của một container, VM hoặc bộ thư mục, và lần sao lưu gần nhất đã thêm bao nhiêu | đọc |
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
- **Bảo vệ lưu giữ.** Khi một miền giữ một số lượng điểm khôi phục cố định (chỉ "giữ N bản gần nhất", không có quy tắc theo ngày, tuần hay tháng, ở máy cục bộ hay ở đích off-site), mỗi lần sao lưu mới sẽ đẩy bản cũ nhất ra ngoài. Khi đó BombVault từ chối việc bắt đầu qua MCP cho một mục mà N-1 lần sao lưu thành công gần nhất đều được bắt đầu qua MCP. Nhờ vậy, trong tập được giữ luôn còn ít nhất một điểm khôi phục do lịch hoặc do bạn tạo. Với "giữ 1 bản gần nhất", trợ lý hoàn toàn không thể sao lưu mục đó. Lần sao lưu theo lịch tiếp theo sẽ lại tạo chỗ trống. Chỉ riêng một quy tắc theo năm được tính như "giữ 1 bản gần nhất", vì nó chỉ giữ một điểm khôi phục cho năm hiện tại.

Khi bắt đầu một miền hoặc Backup Everything, các mục bị một giới hạn giữ lại sẽ bị bỏ qua và được nêu tên trong câu trả lời. Không giới hạn nào trong số này áp dụng cho giao diện web và lịch. Hạn mức mỗi giờ nằm trong bộ nhớ, nên khởi động lại BombVault sẽ đặt nó về không.

## Bật tính năng {#switch-on}

1. Mở **Cài đặt, Hệ thống, Máy chủ MCP** và bấm nút của máy khách bạn dùng. Máy khách không có trong danh sách kết nối qua **Máy khách khác**.
2. Trong **Khóa**, giữ **Khóa mới** và tên được đề xuất, tức tên của máy khách, hoặc gõ một tên cho biết khóa dùng ở đâu, ví dụ "Claude Code trên laptop". Mỗi máy khách một khóa giúp bạn thu hồi một khóa mà không động đến các khóa khác. **Khóa hiện có** cấp cho máy khách một khóa bạn đã tạo trước đó.
3. Bật **Cho phép bắt đầu sao lưu** cho khóa cần bắt đầu được bản sao lưu; nếu không, khóa chỉ đọc được. Bạn có thể đổi sau trên ô của khóa, và thay đổi có hiệu lực từ yêu cầu tiếp theo của trợ lý mà không cần kết nối lại.
4. Bấm **Tạo khóa**. Khóa chỉ hiện một lần. BombVault chỉ giữ dấu vân của khóa và không thể hiện lại nó, nên hãy sao chép ngay. Nếu bạn đóng hộp thoại trước khi máy khách dùng khóa, thẻ vẫn tiếp tục hiện khóa cho đến khi bạn xác nhận đã sao chép.

Khi không có mật khẩu đăng nhập, chính giao diện web đã mở cho mọi người trong mạng của bạn, và ai mở được nó cũng có thể tạo khóa. Thẻ sẽ báo điều này. Nếu bạn mở BombVault bằng một tên trông như công khai (ví dụ `bombvault.example.com` sau một reverse proxy) và chưa đặt mật khẩu đăng nhập, thì không thể tạo hay thay khóa từ địa chỉ đó, để không trang web nào trên Internet có thể khiến trình duyệt của bạn tạo khóa. Hãy đặt mật khẩu đăng nhập, hoặc mở BombVault bằng địa chỉ IP hay một tên cục bộ như `tower` hoặc `tower.local`.

## Các khóa của bạn và nhật ký của chúng {#keys}

Mỗi khóa có một ô riêng trên thẻ. Ô hiển thị tên khóa, khóa được phép bắt đầu sao lưu hay chỉ đọc, bốn ký tự cuối của khóa, thời điểm tạo hoặc thay thế gần nhất, lần gần nhất một ứng dụng khách dùng nó và số lượt gọi hôm nay. Trên ô, bạn đổi tên khóa, đổi quyền, thay thế hoặc thu hồi khóa. Khóa đã thu hồi chuyển sang danh sách khóa đã thu hồi, và bạn có thể xóa hẳn nó ở đó khi không còn lần chạy nào trong lịch sử nhắc đến nó.

Cạnh tên, ô hiện logo của máy khách mà khóa được tạo cho. Khóa tạo qua **Máy khách khác**, hoặc trước khi thẻ liệt kê máy khách, sẽ hiện biểu tượng chiếc khóa thay vào đó.

**Nhật ký** trên một ô mở ra những gì khóa đó đã làm. Đầu tiên là các bản sao lưu nó đã bắt đầu, mỗi bản kèm trạng thái và liên kết tới lần chạy đó trong nhật ký hoạt động trên bảng điều khiển. Bên dưới là các lượt gọi, mới nhất trước, kèm công cụ và kết quả. Một lần từ chối có ghi lý do: khóa chỉ được đọc, cơ chế bảo vệ lưu giữ đã giữ bản sao lưu lại, một bản sao lưu khác đang chạy, mục này vừa được sao lưu qua MCP vài phút trước, hoặc khóa gửi quá nhiều yêu cầu. Một lần hủy liên kết tới lần chạy liên quan.

BombVault giữ các mục của mỗi khóa tối đa 30 ngày: 500 lần khởi chạy và hủy thành công gần nhất, cùng với đó là 200 lượt gọi khác gần nhất (đọc, từ chối và lỗi), nên một trợ lý hỏi đi hỏi lại về một bản sao lưu đang chạy hoặc thử lại mãi một lượt gọi bị từ chối không thể đẩy lần khởi chạy của nó ra khỏi nhật ký. Với mỗi lượt gọi, nó lưu công cụ, kết quả và lần chạy mà lệnh hủy nêu tên. Nó không bao giờ lưu nội dung trợ lý đã gửi, cũng không lưu khóa hay dấu vân tay của khóa. Gói chẩn đoán chỉ đếm số mục, và bản xuất cài đặt không chứa chúng.

## Kết nối máy khách {#clients}

Mỗi máy khách có một nút trên thẻ, trong nhóm **Trên máy tính này** hoặc **Trên đám mây**. Nút mở một hộp thoại ba bước: khóa; cấu hình cho máy khách đó, với địa chỉ bạn đã mở thẻ, một nút để sao chép, nơi cấu hình nằm và, với chứng chỉ riêng của BombVault, những gì máy khách cần để tin nó; và chờ lần gọi đầu tiên của máy khách. Hộp thoại theo dõi lần dùng gần nhất của khóa và chuyển sang màu xanh khi lần gọi đó đến.

Hộp thoại giữ khóa khỏi mọi dòng lệnh. Khi máy khách đọc được khóa từ biến môi trường (`BOMBVAULT_MCP_KEY`), từ lời nhắc ẩn hoặc từ tệp riêng, cấu hình chỉ nêu tên nó. Khi máy khách không có cách như vậy, khóa nằm trong tệp cấu hình hoặc cài đặt của nó, và hộp thoại nói rõ điều đó. Khi tài liệu của máy khách không nói nó xử lý chứng chỉ lạ ra sao, hộp thoại viết bước đó thành việc cần làm nếu máy khách từ chối chứng chỉ của BombVault.

| Máy khách | Thiết lập | Khóa lấy từ đâu |
|---|---|---|
| AnythingLLM | tệp cấu hình | tệp cấu hình |
| Antigravity | tệp cấu hình | biến môi trường |
| Claude Code | lệnh | tệp khóa |
| Claude Desktop | tệp cấu hình | tệp khóa |
| Cline | tệp cấu hình | tệp cấu hình |
| Codex CLI | tệp cấu hình | biến môi trường |
| Continue | tệp cấu hình | `~/.continue/.env` |
| Copilot CLI | tệp cấu hình | tệp cấu hình |
| Cursor | tệp cấu hình | biến môi trường |
| Gemini CLI | tệp cấu hình | biến môi trường |
| GitHub Copilot (VS Code) | tệp cấu hình | lời nhắc ẩn |
| Goose | tệp cấu hình | biến môi trường |
| Jan | biểu mẫu trong ứng dụng | cài đặt của ứng dụng |
| JetBrains (AI Assistant, Junie) | tệp cấu hình | tệp cấu hình |
| Kimi Code | tệp cấu hình | tệp cấu hình |
| LM Studio | tệp cấu hình | tệp cấu hình |
| Mistral Vibe | tệp cấu hình | biến môi trường |
| Msty | biểu mẫu trong ứng dụng | cài đặt của ứng dụng |
| n8n | biểu mẫu trong ứng dụng | kho thông tin đăng nhập của n8n |
| Open WebUI | biểu mẫu trong ứng dụng | cài đặt của ứng dụng |
| opencode | tệp cấu hình | biến môi trường |
| Perplexity (Mac) | biểu mẫu trong ứng dụng | tệp khóa |
| Qwen Code | tệp cấu hình | biến môi trường |
| Roo Code | tệp cấu hình | biến môi trường |
| Visual Studio | tệp cấu hình | tệp cấu hình |
| Warp | tệp cấu hình | tệp cấu hình |
| Windsurf | tệp cấu hình | biến môi trường |
| Zed | tệp cấu hình | tệp cấu hình |
| Grok | biểu mẫu, trên đám mây | máy chủ của nhà cung cấp |
| Le Chat | biểu mẫu, trên đám mây | máy chủ của nhà cung cấp |
| ChatGPT | đăng nhập qua OAuth, trên đám mây | một access token, xem [bên dưới](#oauth) |
| Claude (claude.ai) | đăng nhập qua OAuth, trên đám mây | một access token, xem [bên dưới](#oauth) |

Các phần dưới đây giải thích kỹ hơn cách thiết lập Claude Code và Claude Desktop, và liệt kê những gì mọi máy khách khác cần.

### Claude Code {#claude-code}

Claude Code kết nối tới BombVault qua `mcp-remote`, vốn cần Node.js trên máy tính đó. Trước tiên, hãy lưu khóa vào một tệp văn bản riêng, trên một dòng duy nhất:

```text
X-API-Key: <your key>
```

Sau đó điền đường dẫn của tệp này vào lệnh trong thẻ và chạy lệnh một lần trong terminal. Với chứng chỉ mà máy tính của bạn tin cậy, lệnh trông như sau:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Với chứng chỉ riêng của BombVault (xem [TLS và chứng chỉ](#tls)), lệnh còn chỉ cho Node.js tới chứng chỉ đã tải về:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Kiểm tra kết nối bằng `/mcp` bên trong Claude Code. `--scope user` giúp BombVault dùng được trong mọi dự án của bạn. Claude Code chỉ giữ đường dẫn của tệp khóa, nên khóa không xuất hiện trong lệnh và lịch sử shell, hay trong danh sách tiến trình. Hãy để tệp ở nơi chỉ bạn đọc được, và bên ngoài mọi thư mục mà bạn commit. `@latest` khiến `npx` tải một `mcp-remote` mới; nếu không có nó, một bản cũ hơn đã cài toàn cục sẽ được dùng thay vào đó, và bản này không hỗ trợ `--header-file`.

Đừng viết `${BOMBVAULT_MCP_KEY}` vào các đối số của `mcp-remote` cho Claude Code. Claude Code thay tham chiếu như vậy bằng giá trị từ môi trường của chính nó trước khi khởi động `mcp-remote`, nên khóa nằm trên dòng lệnh của tiến trình đó, nơi các chương trình và người dùng khác trên máy tính có thể đọc được.

Khi không có Node.js, và chỉ với chứng chỉ mà máy tính của bạn tin cậy, Claude Code có thể tự kết nối. Hãy đặt một tệp `.mcp.json` trong thư mục dự án:

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

Đặt `BOMBVAULT_MCP_KEY` ở nơi Claude Code khởi động, ví dụ dưới `"env"` trong `~/.claude/settings.json` hoặc trong profile của shell, bằng trình soạn thảo văn bản chứ không gõ ở dấu nhắc lệnh. Ở đây tham chiếu là an toàn, vì Claude Code không khởi động tiến trình thứ hai nào nhận khóa. Chứng chỉ riêng của BombVault không dùng được theo cách này: kết nối do chính Claude Code tạo từ chối chứng chỉ đó ngay cả khi đã đặt `NODE_EXTRA_CA_CERTS`. Đừng bao giờ commit một tệp `.mcp.json` có ghi khóa bên trong.

### Claude Desktop {#claude-desktop}

Claude Desktop kết nối tới BombVault qua `mcp-remote`, vốn cần Node.js trên máy tính đó. Trước tiên, hãy lưu khóa vào một tệp văn bản riêng, trên một dòng, như mô tả cho [Claude Code](#claude-code). Mở tệp cấu hình trong Claude Desktop qua **Settings, Developer, Edit Config**. Tệp nằm ở `%APPDATA%\Claude\claude_desktop_config.json` trên Windows và ở `~/Library/Application Support/Claude/claude_desktop_config.json` trên macOS. Thêm mục từ thẻ vào bên trong `"mcpServers"`, cạnh các máy chủ đã có, rồi khởi động lại Claude Desktop:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` chỉ có mặt vì chứng chỉ riêng của BombVault. Sau một chứng chỉ mà máy tính của bạn đã tin cậy, hãy bỏ nó đi.
- `--allow-http` chỉ được thêm cho địa chỉ `http://` thông thường.
- Trên Windows, hãy viết đường dẫn bằng dấu gạch chéo xuôi, ví dụ `C:/Users/sam/bombvault-key.txt`, vì một dấu gạch chéo ngược đơn lẻ không phải JSON hợp lệ. Giữ đường dẫn tệp khóa không có dấu cách: Claude Desktop trên Windows chuyển đường dẫn có dấu cách cho `npx` thành hai mảnh.
- Cấu hình chỉ nêu tệp khóa, nên khóa không xuất hiện trong cấu hình cũng như trong danh sách tiến trình. Hãy để tệp ở nơi chỉ bạn đọc được.

### Máy khách trên đám mây {#cloud-clients}

ChatGPT, Claude trên claude.ai, Grok và Le Chat gọi BombVault từ máy chủ của nhà cung cấp, nên BombVault phải truy cập được từ internet với chứng chỉ được tin cậy công khai, ví dụ phía sau một reverse proxy; Le Chat từ chối chứng chỉ tự ký. Đăng nhập trên proxy có thể bảo vệ giao diện web, nhưng `/mcp` phải tới được BombVault mà không qua nó: các dịch vụ này không đăng nhập được vào proxy, và BombVault tự kiểm tra khóa hoặc token của chúng. Grok và Le Chat gửi một khóa cố định, và nút của chúng thiết lập chúng như các máy khách khác. ChatGPT, và ở hầu hết tổ chức cả Claude trên claude.ai, chỉ kết nối qua đăng nhập bằng OAuth, được mô tả ngay sau đây.

### Đăng nhập qua OAuth {#oauth}

Với một máy khách không nhận được khóa, BombVault là máy chủ ủy quyền OAuth của chính nó. Máy khách tự đăng ký, đưa bạn tới một trang BombVault, và ở đó bạn đăng nhập bằng mật khẩu đăng nhập (và yếu tố thứ hai, nếu bạn đã thiết lập) rồi cho phép nó. Sau đó máy khách nhận một token chỉ dùng được cho endpoint MCP của BombVault này, và tự gia hạn token đó.

1. Đặt mật khẩu đăng nhập trong **Cài đặt, Hệ thống**. Không có mật khẩu thì BombVault không cung cấp đăng nhập nào, vì không có ai để hỏi sự đồng ý.
2. Làm cho BombVault truy cập được từ internet qua https với chứng chỉ mà trình duyệt tin cậy, thường là qua một reverse proxy. Máy khách gọi `/mcp`, `/oauth/` và `/.well-known/` từ máy chủ riêng, nên một proxy có đăng nhập riêng phải cho ba đường dẫn này đi qua tới BombVault. Trang đồng ý ở `/oauth/authorize` mở trong trình duyệt của chính bạn và có thể nằm sau đăng nhập của proxy. Hãy khai báo proxy cả trong `TRUSTED_PROXY` (xem [Cấu hình](configuration.md)). BombVault giới hạn số lần đăng ký máy khách theo từng địa chỉ, và nếu không khai báo, mọi máy khách đều có vẻ đến từ proxy.
3. Trên thẻ MCP, bật **Đăng nhập qua OAuth** và nhập **Địa chỉ công khai**: địa chỉ https không có đường dẫn, ví dụ `https://backup.example.com`. Mọi token đều gắn với địa chỉ này, nên sau khi đổi, mọi máy khách phải đăng nhập lại.
4. Bấm nút ChatGPT hoặc Claude. Hộp thoại hiển thị **URL trình kết nối**, tức địa chỉ công khai kèm `/mcp` phía sau, và chỗ nhập nó trong máy khách đó. Trong ChatGPT, bật chế độ nhà phát triển trong **Cài đặt, Ứng dụng và trình kết nối, Cài đặt nâng cao**, chọn **Tạo**, dán URL trình kết nối làm URL máy chủ MCP và chọn OAuth làm phương thức xác thực. Trên claude.ai, mở **Cài đặt, Trình kết nối, Thêm trình kết nối tùy chỉnh**, dán URL trình kết nối, để trống ID ứng dụng khách và bí mật OAuth rồi chọn **Kết nối**.
5. Máy khách mở trang đồng ý. Trang cho thấy ai đang yêu cầu, câu trả lời của bạn sẽ đưa bạn quay về đâu, và công tắc **Cho phép bắt đầu sao lưu**, mặc định tắt. Chọn **Cho phép** hoặc **Từ chối**.

Mỗi máy khách đã đăng nhập có một ô cạnh các khóa, với biểu tượng, nhật ký, **Thu hồi** và **Cho phép bắt đầu sao lưu**, và cùng giới hạn như một khóa. Thu hồi có hiệu lực ngay. Khi cùng một máy khách đăng nhập lại, quyền mới thay cho quyền cũ, và quyền mà không ai dùng trong 30 ngày sẽ hết hạn. Có thể có tối đa 10 máy khách đăng nhập cùng lúc, ngoài 10 khóa.

Trang đồng ý chỉ nhận yêu cầu từ một máy khách đã đăng ký và nêu đúng chính xác một trong các địa chỉ quay về đã đăng ký: https, hoặc một địa chỉ loopback ở cổng bất kỳ cho máy khách trên chính máy tính của bạn. Chỉ luồng mã ủy quyền với PKCE (S256) được chấp nhận, và câu trả lời của bạn gắn với phiên của bạn, nên không trang web nào khác gửi thay bạn được. Access token có hiệu lực một giờ. Refresh token được thay mỗi lần dùng, và nếu một cái xuất hiện lại sau đó, BombVault thu hồi quyền, vì có người khác đang giữ bản sao. Máy khách gửi lại lần làm mới gần nhất trong vòng 30 giây vì không nhận được câu trả lời thì sẽ được cấp token mới. BombVault không tải siêu dữ liệu máy khách từ internet, nên máy khách đăng ký qua đăng ký máy khách động.

### Máy khách khác {#other-clients}

Bất kỳ máy khách nào nói Streamable HTTP đều dùng được:

- URL: địa chỉ của giao diện web thêm `/mcp`, ví dụ `https://192.168.1.10:3443/mcp`.
- Khóa trong `Authorization: Bearer <key>` hoặc trong `X-API-Key: <key>`. Nếu gửi cả hai thì phải cùng một khóa.
- `POST` với `Content-Type: application/json` và `Accept: application/json, text/event-stream`.
- Một thông điệp JSON-RPC cho mỗi yêu cầu; yêu cầu gộp (batch) bị từ chối.
- Các phiên bản giao thức 2026-07-28, 2025-11-25, 2025-06-18 và 2025-03-26.

## TLS và chứng chỉ {#tls}

BombVault phục vụ HTTPS bằng chứng chỉ tự cấp, và ban đầu chứng chỉ này chỉ ghi `localhost`, `127.0.0.1` và `::1`. Claude Code và `mcp-remote` từ chối nó trên một địa chỉ mạng LAN. Các cách xử lý, theo thứ tự phù hợp với phần lớn các bản cài Unraid:

1. **Thêm địa chỉ trong thẻ MCP.** Khi mở thẻ qua HTTPS ở một địa chỉ mà chứng chỉ không ghi, thẻ sẽ báo và đề xuất **Thêm địa chỉ này vào chứng chỉ**. BombVault sẽ cấp lại chứng chỉ có kèm địa chỉ đó (trình duyệt sẽ cảnh báo thêm một lần, như lần đầu). Sau đó nhấn **Tải chứng chỉ**; các đoạn mã đặt `NODE_EXTRA_CA_CERTS` trỏ tới tệp đã tải, nên máy khách tin cậy đúng chứng chỉ đó. Điều đó cũng có nghĩa là mọi máy khách được thiết lập bằng tệp đã tải trước đó sẽ không kết nối được nữa ngay khi chứng chỉ được cấp lại, trên máy tính này cũng như mọi máy khác, cho đến khi nhận được tệp mới.
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

- Khi không có khóa đang hoạt động và đăng nhập qua OAuth tắt, `/mcp` trả `404`.
- Đăng nhập qua OAuth chỉ được cung cấp khi đã đặt mật khẩu đăng nhập. Token, mã và bí mật máy khách chỉ được lưu dưới dạng dấu vân tay, và một token chỉ dùng được cho địa chỉ mà nó được cấp.
- Một máy khách chỉ đăng ký được tối đa 10 lần mỗi giờ từ một địa chỉ, và BombVault giữ tối đa 100 máy khách đã đăng ký mà chưa ai đăng nhập qua, mỗi máy khách trong một ngày. Mã và refresh token sai được tính vào cùng lượt khóa như khóa sai.
- Quyền hoạt động như khóa khi khôi phục bản sao lưu cấu hình hoặc khi `APP_KEY` thay đổi: sau khi khôi phục, mọi máy khách phải đăng nhập lại.
- Không địa chỉ nào được miễn. Yêu cầu từ `localhost`, từ máy chủ Unraid, từ reverse proxy hay từ `tailscale serve` đều cần khóa như mọi yêu cầu khác, kể cả khi giao diện web không có mật khẩu đăng nhập.
- Khóa chỉ được lưu dưới dạng dấu vân tay, chỉ hiện một lần, và có thể đổi tên, thay thế, thu hồi. Tối đa 10 khóa đang hoạt động, mỗi khóa có công tắc **Cho phép bắt đầu sao lưu** riêng.
- Mỗi lần tạo, thay, đổi quyền và thu hồi đều gửi một thông báo qua các kênh thông báo của bạn, kèm địa chỉ gửi yêu cầu, trừ khi thông báo đang tắt.
- 5 khóa sai mỗi phút cho mỗi địa chỉ, sau đó là `429`. 120 yêu cầu mỗi phút và 12 lần bắt đầu sao lưu mỗi giờ cho mỗi khóa, cộng thêm thời gian chờ và bảo vệ lưu giữ nêu trên.
- Yêu cầu từ trang trình duyệt có nguồn gốc (origin) khác bị từ chối.
- Chừng nào chưa đặt mật khẩu đăng nhập thì không thể tạo khóa từ một tên máy chủ trông như công khai.
- Mọi lần sao lưu do trợ lý bắt đầu, cùng các lần chạy prune và off-site kéo theo, đều được đánh dấu "qua MCP" kèm tên khóa trong nhật ký hoạt động, trong bảng lỗi và trong thông báo sao lưu.
- Mọi lệnh gọi công cụ được ghi vào nhật ký container cùng id của khóa và bốn ký tự cuối (không bao giờ ghi tên) và được đếm trong `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Khôi phục một bản sao lưu cấu hình sẽ thu hồi mọi khóa, vì cơ sở dữ liệu được khôi phục có thể chứa các khóa bạn đã thu hồi sau khi nó được lưu. Hãy tạo khóa mới sau đó.
- Khóa ngừng hoạt động khi `APP_KEY` thay đổi (cài lại, hoặc khôi phục sang container khác). Thẻ phát hiện điều này và đánh dấu khóa, còn **Thay khóa** cấp lại cho nó một bí mật hợp lệ.
- Hãy coi khóa như mật khẩu. Máy khách không đọc được khóa từ biến môi trường, lời nhắc hay tệp khóa sẽ giữ nó dạng văn bản thường trong cấu hình hoặc cài đặt, và hộp thoại của nó nói rõ điều đó. Trên máy tính bạn ít tin tưởng hơn, hãy dùng khóa chỉ đọc.

## Những gì rời khỏi máy {#privacy}

Mọi thứ trợ lý đọc đều được gửi tới nhà cung cấp AI đứng sau nó: tên mục, lịch, lịch sử chạy kèm thông báo lỗi, id và thời điểm của các điểm khôi phục, tên các engine cơ sở dữ liệu và kích thước bản dump, hoạt động đang diễn ra, số liệu lưu trữ, độ bao phủ và trạng thái. BombVault loại bỏ đường dẫn trên máy chủ, vị trí kho, tên máy chủ, thông tin đăng nhập, lệnh hook và khóa trước khi bất cứ thứ gì rời đi.

## Khắc phục sự cố {#troubleshooting}

| Bạn thấy gì | Ý nghĩa |
|---|---|
| `404` | Không có khóa đang hoạt động và đăng nhập qua OAuth tắt, hoặc đường dẫn sai như `/api/mcp`. Endpoint là `/mcp`. |
| `401` | Thiếu khóa, gõ sai, đã thu hồi hoặc đã thay. Có thể proxy làm rơi header `Authorization` (thử `X-API-Key`). Nếu thẻ đánh dấu khóa không còn hợp lệ, `APP_KEY` đã thay đổi: hãy thay khóa. |
| `403` | Yêu cầu đến từ trang trình duyệt có nguồn gốc khác. Hãy dùng máy khách trên máy tính hoặc dòng lệnh. |
| `405` với GET | Bình thường. Điểm kết nối chỉ nhận `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Máy khách quá cũ cho Streamable HTTP. Hãy cập nhật. |
| `400` "batch requests are not accepted" | Máy khách gửi yêu cầu gộp JSON-RPC. Hãy gửi một thông điệp cho mỗi yêu cầu. |
| `429` | Quá nhiều khóa sai từ địa chỉ này, hoặc hơn 120 yêu cầu mỗi phút với một khóa. Đợi một phút và kiểm tra xem trợ lý có bị kẹt trong vòng lặp không. |
| Lỗi có "certificate", "self-signed" hoặc "unable to verify" | Máy khách không tin cậy chứng chỉ của BombVault. Xem [TLS và chứng chỉ](#tls). |
| `busy` | Một lần sao lưu khác hoặc tác vụ bảo trì đang chiếm miền đó. Thử lại khi nó xong. |
| `cooldown` | Mục này, miền này hoặc Backup Everything đã được bắt đầu qua MCP chưa đầy 15 phút trước. |
| `retention_guard` | Thêm một lần sao lưu qua MCP nữa sẽ khiến khoảng "giữ N bản gần nhất" chỉ còn các điểm khôi phục từ MCP, hoặc mục đó đã được sao lưu qua MCP 4 lần trong 24 giờ qua, tính cả các lần thất bại và bị hủy. Ở trường hợp đầu, lần sao lưu theo lịch tiếp theo sẽ tạo chỗ trống; ở trường hợp sau, mục đó được bắt đầu lại sau 24 giờ kể từ lần sao lưu cũ nhất trong số đó. Trong giao diện web, bạn có thể bắt đầu nó bất cứ lúc nào. |
| `rate_limited` | Khóa đã dùng hết 12 lần bắt đầu của giờ này. |
| `not_permitted` khi bắt đầu | Khóa chỉ được đọc. Bật **Cho phép bắt đầu sao lưu** trong thẻ; không cần kết nối lại. Khi hủy, nó có nghĩa là lần chạy đó không do khóa này bắt đầu. |
| `domain_off` | Loại sao lưu đó đang tắt trong cài đặt. |
| `not_found` | BombVault không bảo vệ mục đó. Hãy thêm nó trong giao diện web trước; MCP không bao giờ tạo cấu hình. |
| Máy khách không tìm thấy máy chủ ủy quyền | Đăng nhập qua OAuth đang tắt, chưa đặt mật khẩu đăng nhập, hoặc proxy không cho `/.well-known/` đi qua tới BombVault. |
| Trang đồng ý báo địa chỉ quay về chưa đăng ký | Máy khách gửi một địa chỉ quay về mà nó chưa đăng ký. Xóa trình kết nối trong máy khách rồi thêm lại. |
| Máy khách đã đăng nhập nhận `401` | Quyền của nó đã bị thu hồi, hết hạn sau 30 ngày không dùng, hoặc địa chỉ công khai đã đổi. Máy khách sẽ đăng nhập lại. |

Đừng đặt biến môi trường `MCPGODEBUG` trên container. Nó thay đổi cách thư viện MCP hoạt động, và một giá trị sai định dạng sẽ khiến BombVault dừng ngay khi khởi động, trước cả khi ghi được một dòng nhật ký nào.
