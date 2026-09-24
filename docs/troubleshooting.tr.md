# Sorun giderme

Kısa bir SSS. Tam VM-üzeri-SSH host tarafı sorun giderme tablosu için (izin reddedildi, host anahtarı doğrulaması, eksik şablon değişkenleri ve daha fazlası), GitHub'daki [SSH üzerinden VM yedekleme kılavuzu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'na bakın.

## Bir şey doğru bağlanmamış

Web arayüzünde `/spike`'ı açın. Host entegrasyon denetimi her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir. Bir hata olduğunu varsaymadan önce buradan başlayın: eksik bir bağlama ya da ulaşılamayan bir host hemen ortaya çıkar.

## Web arayüzüne ulaşamıyorum

BombVault, kutudan çıktığı gibi `3443` portunda HTTPS sunar (kendinden imzalı sertifika), bu yüzden `https://<your-unraid-ip>:3443` adresini açın. Kendinden imzalı sertifika uyarısını kabul edin ya da BombVault'u kendi sertifikanızla bir ters proxy arkasına yerleştirin. `HTTP_ONLY=true` ile çalıştırırsanız, bunun yerine `3000` portunda düz HTTP sunar (TLS'yi sonlandıran bir proxy arkasında kullanım için tasarlanmıştır).

## APP_KEY'imi kaybettim

`APP_KEY`, restic depo parolasını türetir. O olmadan (ve şifreleme anahtarı kurtarma kiti olmadan), şifreli yedekler kurtarılamaz. Kontrol Paneli'nin kurtarma kitini indirmeniz için dırdır etmesinin nedeni budur. Bkz. [Site dışı ve kurtarma](offsite-recovery.md). `openssl rand -hex 32` ile bir anahtar oluşturun ve herhangi bir yedeğe güvenmeden önce onu sunucu dışında saklayın.

## VM yedeklemesi bağlanmıyor

VM yedeklemesi, bir bağlamayla değil, SSH üzerinden libvirt ile konuşur.

- Host'ta SSH'nin etkin olduğunu ve BombVault'un genel anahtarının `/root/.ssh/authorized_keys` içinde yetkilendirildiğini onaylayın (Ayarlar, Sistem, SSH üzerinden VM Yedeği anahtarı ve bir **Bağlantıyı test et** düğmesi gösterir).
- Özel bir `br0.x` ağında, `LIBVIRT_HOST`'u Unraid LAN IP'nize ayarlayın (konteyner orada host'a `host.docker.internal` üzerinden ulaşamaz). **Ayarlar, Docker, Özel ağlara host erişimi**'ni etkinleştirin.
- Unraid'in SSH portunu değiştirdiyseniz, eşleşmesi için `LIBVIRT_SSH_PORT`'u ayarlayın.
- Tam adım adım tanılama (ulaşılabilirlik testi, VLAN yönlendirme, `Permission denied (publickey)`, `Host key verification failed`) [SSH üzerinden VM yedekleme kılavuzu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'nda yer alır.

## Bir canlı VM anlık görüntüsü çalışmadı

Canlı anlık görüntüler, VM'de qemu guest agent'ın kurulu olmasını ve diskin `/mnt/user` değil `/mnt/cache` (ya da `/mnt/diskX`) üzerinde olmasını gerektirir. Kapalı bir VM'de, canlı otomatik olarak düzgüne geri düşer. Düzgün bir yedekleme VM'yi kapatır, diskleri yedekler, ardından yeniden başlatır, böylece her zaman tutarlıdır.

## Bir yedekleme "repository is already locked" ile başarısız oldu

Bu genellikle, konteyner işlem ortasında güncellendiğinde ya da yeniden başlatıldığında geride kalan öksüz bir restic kilididir. BombVault, kanıtlanabilir biçimde öksüz bir kilidi algılar, zorla temizler ve otomatik olarak bir kez yeniden dener. Sürerse, eski bir kilidi elle temizlemek için etkilenen etki alanı için **Ayarlar, Bütünlük ve bakım, Kilidi aç**'ı kullanın. Gerçek bir sorun gizlenmek yerine yine de ortaya çıkar.

## Bir yedeklemeden sonra site dışı kopyam gerçekleşmedi

Site dışı çoğaltma tasarım gereği en iyi çabadır, böylece bir site dışı aksaklık asla yerel yedeklemeyi bozmaz. O etki alanı için site dışı zamanlamayı denetleyin (Ayarlar, Zamanlamalar): boş bir zamanlama her yerel yedeklemeden sonra çoğaltır, bir sıklık ise daha seyrek gönderir. İstek üzerine bir çalışma için Site dışı sekmesindeki **Şimdi çoğalt**'ı kullanın ve Kontrol Paneli'ndeki çoğaltma göstergesini izleyin.

## Bir geri yükleme başlamadan iptal oldu

Herhangi bir şey durdurulmadan ya da kaldırılmadan önce, geri yükleme bir uçuş öncesi çakışma denetimi çalıştırır: konteynerin statik IP'sinin ve yayımlanan host portlarının serbest olduğunu doğrular. Başka bir konteyner zaten birini tutuyorsa, yarım kalmış bir geri yükleme bırakmak yerine açık, uygulanabilir bir mesajla iptal eder. Çakışan portu ya da IP'yi serbest bırakın, ardından yeniden deneyin.

## Bir düz dışa aktarma, dosya yazmak yerine başarısız oldu

age şifrelemesi açıksa (Ayarlar) ama geçerli bir alıcı ayarlanmamışsa, bir dışa aktarma düz metin yazmak yerine açık bir hatayla başarısız olur. Geçerli bir alıcı ekleyin (bir age genel anahtarı ya da bir SSH genel anahtarı) ya da dışa aktarmanın düz metin olmasını istiyorsanız şifrelemeyi kapatın. Bkz. [Özellikler](features.md).

## Bir veritabanı dökümü başarısız oldu

Başarısız bir döküm, çevresindeki yedeklemeyi asla düşürmez; kendi başına başarısız bir çalışma olarak kaydedilir ve nedeni neyin düzeltileceğini söyler.

- **Giriş reddedildi.** Döküm, konteynerin kendi parola değişkenleriyle oturum açar (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` ya da bunların `_FILE` sürümleri). Bunları veritabanı konteynerinde denetleyin. Konteynerin kendi kullanıcısının okuyamadığı bir sırrı gösteren `_FILE` değişkeni de aynı şekilde başarısız olur.
- **Eksik yetkiler.** Rastgele bir root parolasıyla döküm yalnızca uygulama kullanıcısı olarak girebilir, dolayısıyla yalnızca o tek veritabanını içerir; MySQL 8.4 ve sonrası ise büsbütün reddedebilir. Konteynere gerçek bir root parolası verin ya da dökümünü kapatın.
- **Sistem tabloları yükseltme istiyor.** MariaDB, sistem tabloları daha eski bir sürümden geliyorsa dökümü reddeder (hata 1558). `MARIADB_AUTO_UPGRADE=1` değişkenini ekleyip konteyneri yeniden başlatın ya da içinde bir kez `mariadb-upgrade` çalıştırın.
- **Döküm aracı yok.** `pg_dump`, `mysqldump` veya `mariadb-dump` içermeyen ince ya da elde yapılmış bir imajın dökümü alınamaz. Resmi imajı kullanın ya da dökümü kapatın.
- **Bir süre sınırı.** Bir döküme `DB_DUMP_MAX_HOURS` (varsayılan 6), çevresindeki yedeklemeye `BACKUP_MAX_HOURS` tanınır; ilerlemeyi kesen bir döküm ise `BACKUP_STALL_HOURS` sonunda kesilir. Sonuncusunun ardında genellikle uygulamanın tuttuğu bir kilit vardır. Devreye giren sınırı yükseltin ya da uygulama sakinken döküm alın.
- **Konteyner duraklatılmış ya da yeniden başlıyor.** Döküm, çalışan sunucuyla konuşur. Konteyner sürekli yeniden başlıyorsa nedenini kendi günlüğü söyler.
- **Bozuk bir döküm kaldırılamadı.** BombVault'un tamamlayamadığı bir döküm yeniden silinir. Bu silme başarısız olduğunda döküm, bozuk olarak işaretli biçimde listede kalır ve oradan silebilirsiniz.

## Bir içe aktarma başarısız oldu

İçe aktarma konteyneri durdurur, veri klasörünü kenara alır ve yerine imajın boş bir klasör oluşturmasına izin verir. İçe aktarmanın kendisinden önceki bir adım başarısız olursa eski klasör kendiliğinden geri konur. İçe aktarma başarısız olursa konteyner yeni klasörle kalır, eskisi de yanında `<veri klasörü>.bombvault-before-import-<zaman damgası>` adıyla durur; çalışmanın hata iletisi tam yolu belirtir.

Elle geri koymak için: konteyneri durdurun, mevcut veri klasörünün adını değiştirip yoldan çekin, saklanan klasörü özgün adına geri döndürün ve konteyneri başlatın. Unraid'de bunu Shares sekmesindeki dosya yöneticisi yapar.

## Bir ZFS veri kümesi yedeği başarısız oldu veya bir kümeyi atladı {#zfs-datasets}

Her sorunun köşeli parantez içinde bir neden kodu vardır ve [ZFS veri kümeleri](zfs-datasets.md#reason-codes) sayfası hepsini çözümüyle listeler. En sık üçü:

- **`snapshot-loop`**: Host Data yeni bağlamaları iletmediği için anlık görüntü BombVault'a ulaşmadı. Konteyneri düzenleyin, Host Data'nın Access Mode ayarını Read/Write - Slave yapın ve BombVault'u yeniden başlatın.
- **`key-not-loaded`**: anahtarı yüklenmemiş şifreli bir veri kümesi atlanır. Anahtarı `zfs load-key` ile yükleyin ve kümeyi bağlayın; sonraki yedek onu da alır.
- **`ssh-auth`**: sunucu BombVault'un anahtarını reddetti. ZFS sayfasındaki bağlantı kartı, anahtarı yetkilendiren komutu gösterir; sunucuda bir kez çalıştırın.

## Bir öğe "Öğreniyor N/10" durumunda kalıyor

Anomali denetimlerinin çoğu bir öğenin 10 başarılı yedeğinden sonra başlar ve sayım **Beklenen olarak işaretle** sonrasında ve öğenin seçimi değiştikten sonra yeniden başlar. Zamanlaması olmayan bir öğe öğrenmez, appdata'sı olmayan bir konteynerin de öğrenecek bir şeyi yoktur; rozeti de bunu söyler.

## Saklama, bir öğenin eski yedeklerini silmeyi bıraktı

Açık bir kritik anomali onları tutuyor: öğenin kaynağı neredeyse boş, çok küçülmüş ya da bir yedek verilerin çoğunu yeniden kaydetmiş. Anomaliyi öğedeki rozetten açın. Veri eksikse ya da şifrelenmişse önce bağlantısı verilen son iyi yedekten geri yükleyin. Ardından anomaliyi onaylayın ya da değişiklik sizden geldiyse beklenen olarak işaretleyin; sonraki çalıştırma her zamanki gibi temizler. Saklama önizlemesi böyle bir öğeyi tutulmuş olarak işaretler.

## Elle temizlik bazı öğelerin tutulduğunu söylüyor

Aynı neden: temizlik, böyle bir anomalisi olan öğenin eski yedeklerine dokunmaz ve öğeyi mesajında adlandırır. Geri kalan her şey her zamanki gibi temizlenir.

## Geçmiş içe aktarımı bir deponun okunamadığını söylüyor

Güncellemeden sonra BombVault önceki yedeklerin boyutlarını her depodan bir kez okur. O sırada erişilemeyen bir depo, örneğin çalışmayan bir uzak hedef ya da bağlanmamış bir paylaşım, **Ayarlar, Bütünlük** altındaki **Anormallikler** kartında listelenir ve günde bir kez yeniden denenir. Bu arada öğeleri yeni yedeklerden öğrenir.

## Disk alanı uyarısı Unraid panosuyla uyuşmuyor

Unraid kullanıcı paylaşımında (`/mnt/user`) boş alan tek bir diskin değil, tüm dizinin boş alanıdır. Uzak depolar yalnızca boş alanını bildiren rclone uzakları üzerinden ölçülür; S3, B2, REST ve SFTP depolarının değeri yoktur ve **Anormallikler** kartında ölçülmemiş olarak listelenir.

## Bir yapay zekâ asistanı bağlanamıyor

[MCP sunucusu](mcp.md#troubleshooting) sayfası, MCP uç noktasının her durum kodunun ve her reddinin ne anlama geldiğini ve ne yapılacağını anlatır.

## Konteyner sürekli yeniden başlıyor ya da sağlıksız görünüyor

BombVault, kendi `/api/health`'inden sağlıklı/sağlıksız bildirir. Motor bir şekilde sıkışırsa bir otomatik onarma aracı (Autoheal gibi) onu otomatik olarak yeniden başlatabilir. Altta yatan neden için konteyner günlüğünü ve `/spike` raporunu denetleyin.

## Hâlâ takıldınız mı?

- Tam [Yapılandırma](configuration.md) ve [Site dışı ve kurtarma](offsite-recovery.md) sayfalarını okuyun.
- [Unraid destek başlığı](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)'nda sorun.
- Bir [GitHub sorunu](https://github.com/junkerderprovinz/bombvault/issues) açın.
