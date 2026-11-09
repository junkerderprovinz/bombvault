# Adım adım örnekler

Bu örneklerin her biri gerçek bir işi baştan sona anlatır, isimler ve sayılar da yerine konmuştur. Örnekler özelliği açıklayan sayfanın içinde durur, böylece ayrıntılar hemen yanında olur.

## Başlarken {#start}

- [İlk konteyner yedeğin, baştan sona](getting-started.md#example-first-backup): kurulumdan kurtarma kitine kadar.

## Depolama ve site dışı {#storage}

- [Backblaze B2'de site dışı kopya](offsite-recovery.md#example-s3): bir bucket, o bucket için bir anahtar ve ilk kopya. Cloudflare R2 de aynı şekilde çalışır.
- [Kendi çalıştırdığın bir rest-server](offsite-recovery.md#example-rest-server): silmeyi reddeden bir site dışı kopya, `docker run` satırı ve kurcalama testiyle.
- [Dengeli neyi saklar](offsite-recovery.md#example-retention): iki yıllık gece yedeklerinde saklama, yedek yedek.
- [Konteynerler bir NAS'a taşınıyor](offsite-recovery.md#example-own-folder): bir bölümün kendi klasörü yer değiştirir, daha önce alınmış yedeklere ne olur.

## Yedekler ve zamanlamalar {#backups}

- [Birbirine uyan üç plan](schedules.md#example): gece tam yedekleme, VM'ler haftada bir ve tek bir öge için 6 saatte bir kendi planı.
- [Plex, biri izlerken bekler](backups.md#example-plex): filmi kesmeyen zamanlanmış bir yedekleme.
- [İki Unraid anlık görüntüsü olan bir VM](vm-backup-ssh-setup.md#example-unraid-snapshots): anlık görüntüleri koruyan canlı bir yedekleme ve orijinalin yanına geri yükleme.
- [İkinci bir Unraid makinesine replika](zfs-datasets.md#example-replica): istediğin zaman okuyabileceğin bir ZFS kopyası ve ondan tek bir dosyayı almanın üç yolu.

## Verileri geri almak {#restore}

- [Tek bir dosyayı geri al](restore.md#example-single-file): dünkü gece konteyner yedeğinden tek bir dosya.
- [ZFS yedeğinden tek bir dosya](zfs-datasets.md#example-zfs-file): aynısı bir veri kümesi için.
- [Kurtarma kitinden yeni bir sunucu](offsite-recovery.md#example-emergency): ölen bir sunucu yeni bir makinede geri gelir, şablonda eski `APP_KEY` olmadan.

## Haberdar kalmak {#staying-informed}

- [Telefonunda Gotify](notifications.md#example-gotify): hatalar çalar, başarılar sessizce gelir.
- [Hesap olmadan ntfy](notifications.md#example-ntfy): sana ait bir konuya mesajlar.

## Birden çok sunucu {#several-servers}

- [İki Unraid makinesini eşleştirmek](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): birbirinin kopyalarını tutan iki sunucu, hiçbiri ötekinin yedeklerini silemez.
- [Home Assistant'ta bir kart ve bir uyarı](api.md#home-assistant-example): panonda BombVault ve yedekleme başarısız olunca bir mesaj.
