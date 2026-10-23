# Ứng dụng Android

Ứng dụng Android đưa mọi máy chủ BombVault trong nhóm của bạn lên điện thoại. Khi mở, ứng dụng hiện danh sách máy chủ, phía trên là nhật ký hoạt động của tất cả các máy, gồm đúng những dòng mà Tổng quan hiển thị, và chạm vào máy chủ nào thì giao diện điện thoại của máy chủ đó sẽ mở ra. Bản thân ứng dụng không sao lưu gì cả.

## Tải ứng dụng {#install}

- **Cài đặt, Ứng dụng:** thẻ của ứng dụng Android cung cấp APK của bản phát hành mà máy chủ của bạn đang chạy, kèm một mã QR để quét từ điện thoại.
- **APK:** mỗi bản phát hành đều có `bombvault-android.apk` trên trang phát hành của nó, và [liên kết này](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) luôn tải về bản dựng mới nhất. Cần Android 10 trở lên, và Android sẽ hỏi một lần xem ứng dụng bạn dùng để mở tệp có được phép cài đặt ứng dụng hay không.
- **Google Play:** ứng dụng đang trong giai đoạn thử nghiệm kín cho đến khi có thể phát hành công khai. Google Play chỉ đưa ứng dụng từ một tài khoản nhà phát triển mới lên danh sách sau khi có ít nhất 12 người thử nghiệm giữ ứng dụng được cài trong 14 ngày. Nếu muốn giúp, hãy tham gia [nhóm người thử nghiệm](https://groups.google.com/g/arrowloop-testers), mở [trang thử nghiệm](https://play.google.com/apps/testing/bombvault.halleluja.design), chạm vào **Trở thành người thử nghiệm** rồi cài BombVault từ Google Play.
- **F-Droid:** sẽ có sau.

Hãy ghép nối ứng dụng với các máy chủ phiên bản 9.7.0 trở lên. Một máy chủ chạy phiên bản cũ hơn vẫn có thể ở trong cùng nhóm, nhưng nó hiển thị điện thoại như một phiên bản bình thường, và ứng dụng chỉ đọc được hoạt động của máy chủ đó sau khi đăng nhập.

## Ghép nối bằng mã QR {#pairing}

1. Trên bất kỳ máy chủ nào trong nhóm, mở **Cài đặt, Ghép nối** và chọn **Hiện cụm từ**. Mười hai từ hiện ra, bên cạnh là một mã QR.
2. Trong ứng dụng, chạm vào **Quét mã QR** và hướng điện thoại vào mã. Bạn cũng có thể dán hoặc gõ các từ.
3. Ứng dụng liệt kê các máy chủ của nhóm đó trước khi lưu bất cứ thứ gì. **Thêm tất cả** sẽ thêm toàn bộ.

Sau đó điện thoại gia nhập nhóm như một phiên bản khác. Nó đọc những gì đang chạy trên từng máy chủ qua nhóm mà không cần đăng nhập: ở nhà thì kết nối trực tiếp, khi ra ngoài thì qua relay. Cách nhóm hoạt động được mô tả trong [Ghép nối các phiên bản](offsite-recovery.md#pairing).

Ứng dụng gia nhập nhóm qua relay, kể cả khi ở nhà. Nếu phiên bản của bạn đang để **Không dùng relay**, hãy chọn một relay trong **Cài đặt, Ghép nối** trước khi quét mã.

## Khi ở xa nhà {#away}

Ứng dụng thử máy chủ ở địa chỉ riêng của nó trước. Khi địa chỉ đó không trả lời, chẳng hạn trên mạng di động, ứng dụng mở giao diện của máy chủ qua relay của nhóm. Một máy chủ của nhóm mà ứng dụng chưa biết địa chỉ cũng được mở theo cách này. Nếu cả địa chỉ lẫn relay đều không trả lời, màn hình chính báo điều đó trên thẻ của máy chủ.

Qua relay, trang chạm tới những gì một điện thoại đã ghép nối chạm tới ở nhà, như mô tả bên dưới. Đăng nhập bằng mật khẩu và lưu tệp chỉ hoạt động ở địa chỉ riêng của máy chủ. Thanh tiến trình cập nhật mỗi hai giây thay vì liên tục. Các tệp của giao diện được giữ lại trên điện thoại sau lần truy cập đầu, nên những lần sau tải nhanh hơn. Máy chủ cũ hơn 10.0.0 chỉ mở được ở địa chỉ riêng của nó.

Ứng dụng chỉ nhận một phản hồi khi máy chủ đã ký nó bằng khóa mà ứng dụng đã ghim lần đầu gặp máy chủ đó. Nếu sau này máy chủ phản hồi bằng khóa khác, màn hình chính hiển thị dấu vân tay của khóa mới, và trang chỉ mở sau khi bạn tin cậy khóa đó.

## Chuyển máy chủ {#switch}

Khi một máy chủ đang mở, đầu trang Tổng quan của nó hiển thị tên máy chủ, với một mũi tên quay lại phía trước. Mũi tên dẫn về tổng quan của ứng dụng, tức màn hình chính, nơi liệt kê mọi máy chủ mà ứng dụng biết kèm trạng thái: **Đã kết nối**, **Chưa kết nối**, **Cần đăng nhập** hoặc **Kiểm tra chứng chỉ**. Một máy chủ của nhóm không phản hồi sẽ hiện **Không hoạt động**, màu xám, kèm thời điểm nhóm thấy nó lần cuối, còn máy chủ mà ứng dụng chưa nhận được tin gì sẽ hiện **Đang kiểm tra**. Mỗi lần kết nối, ứng dụng đọc danh sách của nhóm từ một trong các phiên bản của nhóm, nên danh sách có cả phiên bản mà ứng dụng chưa từng gặp qua relay, chẳng hạn một phiên bản đang tắt lúc điện thoại ghép nối. Chạm vào máy chủ khác ở đó để mở nó, trực tiếp khi ở nhà và qua relay trong các trường hợp còn lại.

## Đăng nhập trên điện thoại đã ghép nối {#sign-in}

Một điện thoại đã ghép nối với nhóm của bạn mở từng máy chủ của nhóm với trạng thái đã đăng nhập sẵn. Trước khi tải một trang, nó xin máy chủ đó một phiên qua nhóm, và máy chủ chỉ trao phiên cho một thành viên là điện thoại. Máy chủ được thêm bằng địa chỉ sẽ hỏi mật khẩu, như trong trình duyệt.

Một điện thoại đã ghép nối không nhận được mọi thứ mà mật khẩu mang lại. Phiên của nó chỉ chạm tới một danh sách cố định những gì ứng dụng hiển thị: Tổng quan, các lần chạy và nhật ký hoạt động, các bản sao lưu để xem, bắt đầu và khôi phục, các bất thường để đọc và trang Phiên bản để xem. Các đích sao lưu và lệnh được trả về với địa chỉ và bí mật đã bị lược bỏ. Một lần khôi phục thay thế bản gốc sẽ sao lưu trạng thái hiện tại trước. Khóa, bộ khôi phục, bản gửi khóa, cài đặt và việc xóa bản sao lưu vẫn cần đăng nhập ngay tại máy chủ, còn mọi thứ khác đều trả lời rằng cần đăng nhập đó. Qua relay, danh sách này vẫn áp dụng, kể cả trên máy chủ không có mật khẩu.

Công tắc cho việc này là **Điện thoại có thể ghép nối bằng mười hai từ của nhóm** trên thẻ **Ứng dụng Android** trong **Cài đặt, Bảo mật**. Việc đặt lại mật khẩu tắt nó và đăng xuất mọi điện thoại, cho đến khi bạn bật lại ở đó.

## Máy chủ ngoài nhóm {#other-servers}

- **Thêm máy chủ** nhận địa chỉ bạn dùng để mở BombVault trong trình duyệt, chẳng hạn `192.168.1.10:3443`. Nếu không có `http://` hay `https://` ở đầu, ứng dụng dùng https.
- Các máy chủ tự thông báo trên mạng cục bộ được liệt kê dưới **Trên mạng này** và mở được bằng một lần chạm. Máy chủ làm việc này khi **Tìm trên mạng** đang bật trong thẻ **Dịch vụ** dưới **Cài đặt, Tích hợp**. Máy chủ ở mạng khác hoặc nằm sau VPN sẽ không hiện ở đây.

- Chứng chỉ tự ký được tin cậy một lần theo dấu vân tay SHA-256 của nó. Nếu sau này máy chủ đưa ra một chứng chỉ khác, ứng dụng sẽ cảnh báo bạn và chỉ mở máy chủ sau khi bạn tin cậy chứng chỉ mới.

## Điện thoại trên trang Phiên bản {#instances}

Điện thoại có thẻ riêng trên trang Phiên bản của mọi máy chủ trong nhóm, được đánh dấu là ứng dụng Android và mang tên bạn đặt cho điện thoại. Thẻ này không có bảng điểm vì điện thoại không sao lưu gì. **Xóa** gỡ nó khỏi trang và kết thúc phiên của nó ngay lập tức, giống như khi tắt **Truy vấn phiên bản này** trong **Giám sát** ở phần chi tiết của nó. Điện thoại vẫn biết các từ, nên nó xuất hiện lại khi ứng dụng của nó mở ra, nhưng ở trạng thái tắt, và không nhận được phiên nào cho đến khi có người bật nó lên. Với điện thoại bị mất, hãy xóa thẻ của nó trên mọi máy chủ, hoặc tắt **Điện thoại có thể ghép nối bằng mười hai từ của nhóm** trong **Cài đặt, Bảo mật**, hoặc cho từng phiên bản chọn **Rời nhóm** rồi ghép nối lại bằng các từ mới.

Điện thoại đã ghép nối trước 10.0.0 sẽ bắt đầu ký các lệnh gọi của mình ở lần truy cập đầu tiên sau khi cập nhật, và mỗi máy chủ giữ khóa đầu tiên nhận được cho điện thoại đó. Nếu một người khác biết các từ của nhóm đến trước khi điện thoại đang ngoại tuyến, máy chủ đó sẽ từ chối điện thoại kể từ lúc ấy, và ứng dụng không còn mở được nó qua relay. Để khắc phục, hãy đăng nhập vào máy chủ đó bằng trình duyệt, chọn **Quên khóa** trên thẻ của điện thoại ở trang Phiên bản rồi chọn **Xác nhận quên**, sau đó mở lại ứng dụng. Máy chủ sẽ lấy khóa riêng của điện thoại ở lệnh gọi tiếp theo. Nếu điện thoại lại bị từ chối, nghĩa là người khác có các từ: hãy cho từng phiên bản chọn **Rời nhóm** rồi ghép nối lại bằng các từ mới, như với điện thoại bị mất.

## Cài đặt {#settings}

Biểu tượng bánh răng cạnh dấu cộng mở phần cài đặt của ứng dụng:

- ngôn ngữ, và tên mà điện thoại hiển thị trên trang Phiên bản (để trống thì dùng tên mẫu điện thoại),
- giao diện, theo máy chủ đầu tiên trong danh sách cho đến khi bạn tự chọn, còn hiệu ứng chuyển động có cài đặt riêng,
- một bản báo cáo để sao chép khi báo lỗi; nó không chứa địa chỉ, tên hay cụm từ nào,
- thẻ Giới thiệu kèm chính sách quyền riêng tư,
- **Xóa tất cả máy chủ**, xóa mọi máy chủ khỏi ứng dụng và rời nhóm. Mỗi máy chủ mà điện thoại còn kết nối được sẽ gỡ điện thoại khỏi trang Phiên bản của nó.

## Tải xuống và tải lên {#files}

Các bản xuất, bộ khôi phục, tệp ZIP của flash và bản dump cơ sở dữ liệu được lưu vào thư mục Tải xuống của điện thoại, giống như khi tải từ trình duyệt. Nhập cài đặt sẽ mở trình chọn tệp của điện thoại. Qua relay, không có tệp nào được lưu; muốn lưu, hãy mở máy chủ ở địa chỉ riêng của nó.

## Trên màn hình cảm ứng {#touch}

Ngón tay không thể di chuột qua thứ gì, nên một điều khiển sẽ tối đi khi đang được giữ, và nút có logo thương hiệu sẽ sáng lên bằng màu của thương hiệu đó cho đến khi nhấc ngón tay. Nhấn giữ một nút được tính là một lần chạm chậm và không mở menu liên kết. Chạm vào một nút bị làm mờ sẽ hiện lý do nó bị mờ, cho đến khi bạn chạm vào chỗ khác.

## Dùng trong trình duyệt {#browser}

Chrome và Edge có thể cài giao diện web của BombVault thành một ứng dụng chạy trong cửa sổ riêng, trên điện thoại cũng như trên máy tính. Không có gì được lưu vào bộ nhớ đệm, nên bản cập nhật hiện ra ngay.

## Quyền riêng tư {#privacy}

Ứng dụng không có tài khoản, không có quảng cáo, không có phân tích sử dụng, và không chạy gì ở chế độ nền. [Chính sách quyền riêng tư](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) của ứng dụng liệt kê những gì nó lưu và những gì nó gửi đi đâu.
