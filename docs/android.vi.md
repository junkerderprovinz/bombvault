# Ứng dụng Android

Ứng dụng Android đưa mọi máy chủ BombVault trong nhóm của bạn lên điện thoại. Khi mở, ứng dụng hiện danh sách máy chủ, phía trên là nhật ký hoạt động của tất cả các máy, gồm đúng những dòng mà Bảng điều khiển hiển thị, và chạm vào máy chủ nào thì giao diện điện thoại của máy chủ đó sẽ mở ra. Bản thân ứng dụng không sao lưu gì cả.

## Tải ứng dụng {#install}

- **APK:** mỗi bản phát hành đều có `bombvault-android.apk` trên trang phát hành của nó, và [liên kết này](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) luôn tải về bản dựng mới nhất. Cần Android 10 trở lên, và Android sẽ hỏi một lần xem ứng dụng bạn dùng để mở tệp có được phép cài đặt ứng dụng hay không.
- **Google Play:** ứng dụng đang trong giai đoạn thử nghiệm kín cho đến khi có thể phát hành công khai. Google Play chỉ đưa ứng dụng từ một tài khoản nhà phát triển mới lên danh sách sau khi có ít nhất 12 người thử nghiệm giữ ứng dụng được cài trong 14 ngày. Nếu muốn giúp, hãy tham gia [nhóm người thử nghiệm](https://groups.google.com/g/arrowloop-testers), mở [trang thử nghiệm](https://play.google.com/apps/testing/bombvault.halleluja.design), chạm vào **Trở thành người thử nghiệm** rồi cài BombVault từ Google Play.
- **F-Droid:** sẽ có sau.

Hãy ghép nối ứng dụng với các máy chủ phiên bản 9.7.0 trở lên. Một máy chủ chạy phiên bản cũ hơn vẫn có thể ở trong cùng nhóm, nhưng nó hiển thị điện thoại như một phiên bản bình thường, và ứng dụng chỉ đọc được hoạt động của máy chủ đó sau khi đăng nhập.

## Ghép nối bằng mã QR {#pairing}

1. Trên bất kỳ máy chủ nào trong nhóm, mở **Cài đặt, Ghép nối** và chọn **Hiện cụm từ**. Mười hai từ hiện ra, bên cạnh là một mã QR.
2. Trong ứng dụng, chạm vào **Quét mã QR** và hướng điện thoại vào mã. Bạn cũng có thể dán hoặc gõ các từ.
3. Ứng dụng liệt kê các máy chủ của nhóm đó trước khi lưu bất cứ thứ gì. **Thêm tất cả** sẽ thêm toàn bộ.

Sau đó điện thoại gia nhập nhóm như một phiên bản khác. Nó đọc những gì đang chạy trên từng máy chủ qua nhóm mà không cần đăng nhập: ở nhà thì kết nối trực tiếp, khi ra ngoài thì qua relay. Cách nhóm hoạt động được mô tả trong [Ghép nối các phiên bản](offsite-recovery.md#pairing).

!!! note "Giao diện vẫn cần đường tới máy chủ"
    Danh sách máy chủ và nhật ký hoạt động đến qua nhóm. Giao diện của một máy chủ thì mở trực tiếp, nên điện thoại phải tới được địa chỉ của máy chủ, ở nhà hoặc qua VPN.

## Đăng nhập trên điện thoại đã ghép nối {#sign-in}

Một điện thoại đã ghép nối với nhóm của bạn mở từng máy chủ của nhóm ở trạng thái đã đăng nhập sẵn. Trước khi tải một trang, nó xin máy chủ đó một phiên qua nhóm, và máy chủ chỉ cấp phiên cho thành viên là điện thoại. Máy chủ được thêm bằng địa chỉ sẽ hỏi mật khẩu, giống như trong trình duyệt. Ai có mười hai từ thì vốn đã mở được mọi bản sao lưu của nhóm, nên việc ghép nối không cấp thêm quyền gì mới.

## Máy chủ ngoài nhóm {#other-servers}

- **Thêm máy chủ** nhận địa chỉ bạn dùng để mở BombVault trong trình duyệt, chẳng hạn `192.168.1.10:3443`. Nếu không có `http://` hay `https://` ở đầu, ứng dụng dùng https.
- Các máy chủ tự thông báo trên mạng cục bộ được liệt kê dưới **Trên mạng này** và mở được bằng một lần chạm. Máy chủ làm việc này khi **Tìm trên mạng** đang bật trong Cài đặt, Tích hợp. Máy chủ ở mạng khác hoặc nằm sau VPN sẽ không hiện ở đây.
- Chứng chỉ tự ký được tin cậy một lần theo dấu vân tay SHA-256 của nó. Nếu sau này máy chủ đưa ra một chứng chỉ khác, ứng dụng sẽ cảnh báo bạn và chỉ mở máy chủ sau khi bạn tin cậy chứng chỉ mới.

## Điện thoại trên trang Phiên bản {#instances}

Điện thoại có thẻ riêng trên trang Phiên bản của mọi máy chủ trong nhóm, được đánh dấu là ứng dụng Android và mang tên bạn đặt cho điện thoại. Thẻ này không có bảng điểm vì điện thoại không sao lưu gì, và **Xóa** sẽ gỡ nó khỏi trang.

## Cài đặt {#settings}

Biểu tượng bánh răng cạnh dấu cộng mở phần cài đặt của ứng dụng:

- ngôn ngữ, và tên mà điện thoại hiển thị trên trang Phiên bản (để trống thì dùng tên mẫu điện thoại),
- giao diện, theo máy chủ đầu tiên trong danh sách cho đến khi bạn tự chọn, còn hiệu ứng chuyển động có cài đặt riêng,
- một bản báo cáo để sao chép khi báo lỗi; nó không chứa địa chỉ, tên hay cụm từ nào,
- thẻ Giới thiệu kèm chính sách quyền riêng tư,
- **Xóa tất cả máy chủ**, xóa mọi máy chủ khỏi ứng dụng và rời nhóm. Trên chính các máy chủ thì không có gì thay đổi.

## Tải xuống và tải lên {#files}

Các bản xuất, bộ khôi phục, tệp ZIP của flash và bản dump cơ sở dữ liệu được lưu vào thư mục Tải xuống của điện thoại, giống như khi tải từ trình duyệt. Nhập cài đặt sẽ mở trình chọn tệp của điện thoại.

## Trên màn hình cảm ứng {#touch}

Ngón tay không thể di chuột qua thứ gì, nên một điều khiển sẽ tối đi khi đang được giữ, và nút có logo thương hiệu sẽ sáng lên bằng màu của thương hiệu đó cho đến khi nhấc ngón tay. Nhấn giữ một nút được tính là một lần chạm chậm và không mở menu liên kết.

## Dùng trong trình duyệt {#browser}

Chrome và Edge có thể cài giao diện web của BombVault thành một ứng dụng chạy trong cửa sổ riêng, trên điện thoại cũng như trên máy tính. Không có gì được lưu vào bộ nhớ đệm, nên bản cập nhật hiện ra ngay.

## Quyền riêng tư {#privacy}

Ứng dụng không có tài khoản, không có quảng cáo, không có phân tích sử dụng, và không chạy gì ở chế độ nền. [Chính sách quyền riêng tư](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) của ứng dụng liệt kê những gì nó lưu và những gì nó gửi đi đâu.
