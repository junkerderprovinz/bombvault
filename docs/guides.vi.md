# Ví dụ thực hành

Mỗi ví dụ dưới đây đi qua một công việc thật từ đầu đến cuối, với tên và con số đã điền sẵn. Chúng nằm ngay trên trang giải thích tính năng, nên các chi tiết ở ngay bên cạnh.

## Bắt đầu {#start}

- [Bản sao lưu container đầu tiên của bạn, từ đầu đến cuối](getting-started.md#example-first-backup): từ lúc cài đặt đến bộ khôi phục.

## Lưu trữ và ngoài site {#storage}

- [Một bản sao ngoài site trên Backblaze B2](offsite-recovery.md#example-s3): một bucket, một khóa cho bucket đó và bản sao đầu tiên. Cloudflare R2 hoạt động theo cùng cách.
- [Một rest-server do bạn tự chạy](offsite-recovery.md#example-rest-server): một bản sao ngoài site từ chối việc xóa, với dòng `docker run` và phép thử phá hoại.
- [Cân bằng giữ lại những gì](offsite-recovery.md#example-retention): chính sách lưu giữ qua hai năm sao lưu hằng đêm, từng bản một.
- [Các container chuyển sang một NAS](offsite-recovery.md#example-own-folder): thư mục riêng của một phần đổi chỗ, và chuyện gì xảy ra với các bản sao lưu đã có.

## Sao lưu và lịch trình {#backups}

- [Ba kế hoạch khớp với nhau](schedules.md#example): một bản sao lưu đầy đủ vào ban đêm, VM mỗi tuần một lần và một kế hoạch riêng cho một mục cứ 6 giờ một lần.
- [Plex chờ trong lúc có người đang xem](backups.md#example-plex): một bản sao lưu theo lịch không làm gián đoạn một bộ phim.
- [Một VM với hai snapshot Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): một bản sao lưu trực tiếp giữ lại các snapshot, và một lần khôi phục bên cạnh bản gốc.
- [Một bản sao tới máy Unraid thứ hai](zfs-datasets.md#example-replica): một bản sao ZFS bạn có thể đọc bất cứ lúc nào, và ba cách lấy một tệp từ đó.

## Lấy dữ liệu trở lại {#restore}

- [Lấy lại một tệp](restore.md#example-single-file): một tệp từ bản sao lưu container đêm qua.
- [Một tệp từ bản sao lưu ZFS](zfs-datasets.md#example-zfs-file): tương tự cho một dataset.
- [Một máy chủ mới từ bộ khôi phục](offsite-recovery.md#example-emergency): một máy chủ đã hỏng được dựng lại trên một máy mới, mà không cần `APP_KEY` cũ trong mẫu.

## Luôn nắm thông tin {#staying-informed}

- [Gotify trên điện thoại của bạn](notifications.md#example-gotify): lỗi thì reo chuông, còn thành công đến lặng lẽ.
- [ntfy không cần tài khoản](notifications.md#example-ntfy): tin nhắn tới một topic của riêng bạn.

## Nhiều máy chủ {#several-servers}

- [Ghép đôi hai máy Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): hai máy chủ giữ bản sao cho nhau, và không bên nào xóa được bản sao lưu của bên kia.
- [Một thẻ và một cảnh báo trong Home Assistant](api.md#home-assistant-example): BombVault trên một bảng điều khiển, kèm tin nhắn khi một bản sao lưu bị lỗi.
