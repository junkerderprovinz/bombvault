# Sorun Giderme

Kısa bir SSS. SSH üzerinden VM için ana makine tarafındaki sorun giderme tablosunun tamamı (permission-denied, ana makine anahtarı doğrulaması, eksik şablon değişkenleri ve daha fazlası) için bkz. [SSH üzerinden VM yedekleme kılavuzu](vm-backup-ssh-setup.md).

## Bir şey doğru bağlanmamış {#something-is-not-wired-up-correctly}

**Özelleştir** altından genel bakışa **Host Entegrasyonu** kartını ekle. Host entegrasyon denetimi her bağlamayı ve CLI'yi (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir, **Şimdi denetle** de onu yeniden çalıştırır. Bir hata olduğunu varsaymadan önce buradan başla: eksik bir bağlama ya da ulaşılamayan bir ana makine hemen görünür.

## Web arayüzüne ulaşamıyorum {#i-cannot-reach-the-web-ui}

BombVault kutudan çıktığı gibi `3443` portunda HTTPS sunar (kendi kendine imzalı sertifika), bu yüzden `https://<your-unraid-ip>:3443` adresini aç. Kendi kendine imzalı sertifika uyarısını kabul et ya da BombVault'u kendi sertifikanla bir ters proxy'nin arkasına koy. `HTTP_ONLY=true` ile çalıştırırsan bunun yerine `3000` portunda düz HTTP sunar (TLS sonlandıran bir proxy'nin arkasında kullanım için).

## APP_KEY'imi kaybettim {#i-lost-my-app_key}

`APP_KEY`, restic deposu parolasını türetir. Onsuz, kurtarma kiti olmadan ve grubunda bir anahtar emaneti olmadan, şifreli yedekler kurtarılamaz. Bu yüzden genel bakış, onu indirene kadar **Kurtarma kitini sakla**'yı gösterir. Bkz. [Site dışı ve kurtarma](offsite-recovery.md#recovery-kit). `openssl rand -hex 32` ile bir anahtar üret ve herhangi bir yedeğe güvenmeden önce onu sunucu dışında sakla.

## Giriş parolamı unuttum {#i-forgot-the-login-password}

Giriş sayfasında **Parolanı mı unuttun?**'u seç. BombVault 12 karakterlik bir kodu konteyner günlüğüne yazar (Unraid'de: Docker sekmesi, BombVault simgesi, **Günlükler**). Onu 10 dakika içinde yeni bir parolayla yaz. Sıfırlama ayrıca iki adımlı doğrulamayı kapatır, her geçiş anahtarını kaldırır, her API belirtecini, MCP anahtarını ve OAuth oturumunu iptal eder, diğer tüm oturumları sonlandırır ve telefon eşleştirmesini kapatır. Geçiş anahtarlarını **Ayarlar, Güvenlik** altında yeniden ekle, telefon eşleştirmesini orada yeniden aç ve hâlâ gerektiği yerde yeni belirteçler ve anahtarlar oluştur. Bkz. [Güvenlik modeli](configuration.md#login).

## Yedekler durdu ve anahtarın artık açılamadığını söylüyor {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Bu sunucu Acil durum sihirbazında başka bir sunucunun yedek anahtarını devraldı ve `APP_KEY` sonradan değiştirildi. Devralınan anahtar eski APP_KEY ile mühürlüdür, bu yüzden BombVault onu açamaz ve yanlış anahtarla yazmaktansa hiçbir şey yedeklemez. Genel bakıştaki **İlgi bekliyor** bunu adlandırır. APP_KEY'i eski haline getir ya da kurtarma kitini Geri yükleme sayfasında **Acil durum** altında yeniden yükle.

## Bir telefon giriş sayfasına düşüyor {#a-phone-lands-on-the-login-page}

Eşleşmiş bir telefon oturumunu grup üzerinden yalnızca **Ayarlar, Güvenlik** altında **Telefonlar grubun on iki kelimesiyle eşleşebilir** açıkken alır. Bir parola sıfırlama onu kapatır. Sunucudan eski bir uygulama da güncellenene kadar oturum almaz. Adresiyle eklenen bir sunucu her zaman parola ister. Örnekler sayfasından kaldırılan bir telefon, uygulaması yeniden açıldığında kapalı olarak geri gelir. İçeri almak için ayrıntılarında **İzleme** altındaki **Bu örneği sorgula**'yı aç.

## VM yedekleme bağlanmıyor {#vm-backup-will-not-connect}

VM yedekleme libvirt ile SSH üzerinden konuşur, hiçbir zaman bir bağlama ile değil.

- SSH'ın ana makinede etkin olduğunu ve BombVault'un açık anahtarının `/root/.ssh/authorized_keys` içinde yetkilendirildiğini doğrula (Ayarlar, Entegrasyonlar, Ana makine SSH anahtarı ve bir **Bağlantıyı test et** düğmesi gösterir).
- Özel bir `br0.x` ağında `LIBVIRT_HOST`'u Unraid LAN IP'ne ayarla (konteyner orada ana makineye `host.docker.internal` ile ulaşamaz). **Settings, Docker, Host access to custom networks**'ü etkinleştir.
- Unraid'in SSH portunu değiştirdiysen `LIBVIRT_SSH_PORT`'u eşleşecek şekilde ayarla.
- TrueNAS'ta `LIBVIRT_URI`'yi TrueNAS'ın kendi libvirt soketiyle ayarla, uygulamaya `SETPCAP` yeteneğini ver ve anahtarı **Credentials, Users** altında root'a ekle (bkz. [TrueNAS SCALE](getting-started.md#truenas)).
- Adım adım tam tanı (erişilebilirlik testi, VLAN yönlendirmesi, `Permission denied (publickey)`, `Host key verification failed`) [SSH üzerinden VM yedekleme kılavuzunda](vm-backup-ssh-setup.md) var.

## Canlı bir VM anlık görüntüsü çalışmadı {#a-live-vm-snapshot-did-not-run}

Canlı anlık görüntüler için disk `/mnt/user` üzerinde değil `/mnt/cache` (ya da `/mnt/diskX`) üzerinde olmalıdır. VM'de qemu guest agent varsa anlık görüntü tutarlı (quiesced) alınır, yoksa çökme tutarlıdır (crash-consistent). Kapalı bir VM'de canlı kip kendiliğinden düzgün kapatmaya döner. Düzgün kapatmalı bir yedek VM'i kapatır, diskleri yedekler, sonra yeniden başlatır, bu yüzden her zaman tutarlıdır. Konuğun kapanmasını on dakikaya kadar bekler ve onu hiçbir zaman zorla kapatmaz, böylece güncelleme kuran bir Windows konuğu yarıda kesilmez. Bundan sonra hâlâ çalışan bir konuğa dokunulmaz ve o yedek nedeniyle başarısız olur.

## Bir yedek "repository is already locked" ile başarısız oldu {#a-backup-failed-with-repository-is-already-locked}

Bu genellikle konteyner bir işlemin ortasında güncellendiğinde ya da yeniden başlatıldığında geride kalan sahipsiz bir restic kilididir. BombVault kanıtlanabilir biçimde sahipsiz bir kilidi algılar, temizler ve kendiliğinden bir kez yeniden dener. Yeniden başlatmadan sonra böyle bir kilidin on dakika boyunca yenilenmeden kalmasını bekler. Hâlâ çalışan bir restic kilidini beş dakikada bir yeniler ve BombVault hâlâ yenilenen bir kilidi asla kaldırmaz, çünkü bu kilit aynı depoya yazan ikinci bir BombVault'a ait olabilir. 30 dakika boyunca yenilenmemiş bir kilit ise kimin aldığına bakılmaksızın kaldırılır. Hata sürerse **Ayarlar, Doğrulama** altındaki **Yedekleri denetle** kartını aç ve eski bir kilidi elle temizlemek için etkilenen bölümün satırında **Diğer** altında **Kilidi aç**'ı seç. **Kilidi aç** da hâlâ yenilenen bir kilide dokunmaz ve kilidin en son ne zaman yenilendiğini söyler. Gerçek bir sorun gizlenmek yerine yine yüzeye çıkar.

## Bir yedekten sonra site dışı kopyam yapılmadı {#my-off-site-copy-did-not-happen-after-a-backup}

Site dışı çoğaltma tasarımı gereği elinden geleni yapar, bu yüzden site dışında bir aksama yerel yedeği hiçbir zaman başarısız kılmaz. Hedefin ne zaman kopyaladığına bak: **Kopyala** kartı **Her yedekten sonra** der ya da kendi planını adlandırır, **Ayarlar, Zamanlama, Bölüm başına site dışı kopyalar** da bir bölüme kendi planını verebilir. İstek üzerine çalıştırmak için **Ayarlar, Hedefler** altındaki **Site dışı kopyalar** kartında bölümün satırındaki **Şimdi çoğalt**'ı kullan ve çalışırken **Site dışı kopyalama sürüyor** diyen genel bakışın durum başlığını izle. Başarısız bir kopya **İlgi bekliyor** altında görünür.

## Bir rest-server'ın sertifikasına güvenilmiyor {#a-rest-servers-certificate-is-not-trusted}

restic tanımadığı bir sertifikayı reddeder ve hata onu nereye koyacağını söyler. Rest-server'ın kendi kendine imzalı sertifikasını ya da onu imzalayan otoritenin sertifikasını PEM dosyası olarak BombVault'un veri klasörünün `certs` klasörüne kopyala (konteynerde `/config/certs`, Unraid'de `appdata/bombvault/certs`) ve yeniden dene. Yeniden başlatma gerekmez.

## Bir yedek, Host Data dışındaki verinin yedeklenmediğini söylüyor {#data-outside-host-data}

Konteyner, BombVault'un Host Data üzerinden okuyamadığı bir klasörde veri tutuyor. Yedek tanımı ve ulaşabildiği her klasörü kaydeder, etkinlik günlüğündeki satırını turuncuya çevirir ve dışarıda bıraktığı klasörleri adlandırır. İki durum yaygındır: TrueNAS'ta `/mnt/.ix-apps` altında bir ixVolume ve bir compose stack'inin veritabanı birimi gibi `/var/lib/docker/volumes` altında adlandırılmış bir Docker birimi. Konteynerin **Ayarlar** sekmesindeki **Yedeklenecek klasörler** kartı, böyle her klasörü konteyner içindeki yolu ve ana makine yoluyla, kırmızıyla işaretli olarak listeler.

Anonim bir Docker birimi bildirilmez. Docker, image'in bildirdiği ve kimsenin eşlemediği bir klasör için onu kendisi oluşturur, 64 onaltılık basamakla adlandırır ve `/var/lib/docker/volumes` altında tutar. Genellikle bir önbellek tutar, bu yüzden yedek yeşil kalır ve kart onu yalnızca anonim olduğuna dair bir notla listeler.

- **O verinin yedeklenmesini istiyorsun.** Klasörü Host Data altındaki bir ana makine yoluna eşle, örneğin appdata paylaşımındaki bir klasöre, sonraki yedek onu içerir.
- **Ona ihtiyacın yok.** İki yolundan birini aynı sekmedeki **Hariç tutma desenleri** kartına bir satır olarak ekle: konteyner içindeki yol, örneğin `/cache`, ya da uyarının adlandırdığı ana makine yolu, örneğin `/var/lib/docker/volumes/jellyfin_cache/_data`. Sonraki yedekten itibaren uyarı o klasörü dışarıda bırakır ve alanın altındaki önizleme nedenini söyler. Üstündeki bir klasör için bir satır da çalışır, çıplak bir ad ya da `/` çalışmaz.

## Bir yedek Ertelendi diyor {#deferred-backups}

Konteynerin uygulaması meşguldü, bu yüzden yedeği bekler ve kendiliğinden yeniden denenir. Etkinlik günlüğündeki satır nedenini ve sonraki denemenin ne zaman olduğunu söyler. Konteynerin **Ayarlar** sekmesindeki **Uygulama meşgulken** kartı aynısını ve beklemenin ne zaman biteceğini söyler. Yapılacak bir şey yok. Hemen yedeklemek için **Şimdi yedekle**'yi kullan: kendi başlattığın bir yedek hiç beklemez, 75 çıkış koduna da.

- **Kendi denetimin.** Uygulama meşgulken **Yedekleme öncesi komut**'un 75 koduyla çıkmasını sağla, örneğin `[ "$(cat /tmp/players)" = 0 ] || exit 75`. 0 olmayan başka herhangi bir kod yedeği eskisi gibi başarısız kılar, bu yüzden bozuk bir betik yine **İlgi bekliyor** altında görünür.
- **Plex, Jellyfin ya da Emby hiç sorulmuyor.** BombVault önce sunucuya konteynerin kendi adresinden sorar, yanıt gelmezse konteynerin içinde curl ya da wget çalıştırır. İkisi de olmayan bir image'e kendi üzerinden sorulamaz. Günlük bunu adlandırır. **Sunucu yanıt vermezse yedekle** açıkken böyle bir yedek yine de çalışır, kapalıyken bekler.
- **Jellyfin ya da Emby reddediyor.** **API anahtarı**nı kontrol et. Sunucunun panelinde API keys altında oluşturulur.
- **Plex reddediyor.** BombVault belirteci `/config` olarak bağlanan klasördeki `Preferences.xml` dosyasından okur. Hiç oturum açılmamış bir sunucuda yoktur ve yalnızca onsuz erişime izin verdiği yerde yanıt verir.
- **Duraklatılmış bir film yedeği bütün gece bekletiyor.** **Duraklatılmış oturumları şu süreden sonra yok say (dk)**'yı ayarla, örneğin 30'a.
- **Trafik denetimi yedeği hiç geçirmiyor.** **Şu hızdan itibaren meşgul (KiB/s)**'i, hiç kimse kullanmazken konteynerin gönderdiğinin üstüne çıkar. BombVault başladıktan hemen sonra denetim, konteyneri **Sessiz kalma süresi (dk)** dakika izleyene kadar bekler.
- **Yedek geceleri hiç çalışmıyor.** Bir pencere ayarlıysa denemeler yalnızca onun içinde yapılır. Pencere dışında bir son tarihe ulaşan bekleme yine de son tarihte biter.

## Büyük bir yedek bellek yetersizliğinden öldürüldü {#a-large-backup-was-killed-for-running-out-of-memory}

**Ayarlar, Sistem** altındaki **Performans** kartında **Yedekleme başına bellek sınırı (MiB, 0 = yok)**'nu konteynerin kullanabileceğinin epey altına ayarla. Sınıra yakın restic belleği daha sık serbest bırakır, bu biraz hıza mal olur, ama yedek öldürülmek yerine yavaşlar.

## Bir geri yükleme başlamadan iptal edildi {#a-restore-aborted-before-it-started}

Herhangi bir şey durdurulmadan ya da kaldırılmadan önce geri yükleme bir çakışma ön denetimi yapar: konteynerin sabit IP'sinin ve yayınlanmış ana makine portlarının boş olduğunu doğrular. Başka bir konteyner birini zaten tutuyorsa, yarım kalmış bir geri yükleme bırakmak yerine açık, uygulanabilir bir mesajla iptal eder. Çakışan portu ya da IP'yi boşalt, sonra yeniden dene.

Konteynerin ağı yoksa, örneğin TrueNAS uygulaması silindiği için, geri yükleme denetimi ağı adlandırır ve geri yükleme, bir şeyi değiştirmeden önce reddeder. Uygulamayı TrueNAS'ta aynı adla yeniden kur, bu ağı yeniden oluşturur, sonra verisini geri yükle.

## Düz bir dışa aktarma dosya yazmak yerine başarısız oldu {#a-plain-export-failed-instead-of-writing-a-file}

**Ayarlar, Sistem** altında **Dışa aktarmaları age ile şifrele** açıksa ama geçerli bir alıcı ayarlı değilse, bir dışa aktarma düz metin yazmak yerine açık bir hatayla başarısız olur. Geçerli bir alıcı ekle (bir age açık anahtarı ya da bir SSH açık anahtarı) ya da dışa aktarmanın düz metin olmasını istiyorsan şifrelemeyi kapat. Bkz. [Düz dışa aktarmalar](backups.md#exports).

## Bir veritabanı dökümü başarısız oldu {#a-database-dump-failed}

Başarısız bir döküm, çevresindeki yedeği hiçbir zaman başarısız kılmaz. Kendi başarısız çalıştırması olarak kaydedilir ve neden neyi düzeltmen gerektiğini adlandırır.

- **Giriş reddedildi.** Döküm konteynerin kendi parola değişkenleriyle (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` ya da `_FILE` sürümleri) oturum açar. Bunları veritabanı konteynerinde kontrol et. Konteynerin kendi kullanıcısının okuyamadığı bir gizli bilgiye işaret eden bir `_FILE` değişkeni de aynı şekilde başarısız olur.
- **Eksik yetkiler.** Rastgele bir root parolasıyla döküm yalnızca uygulama kullanıcısı olarak oturum açabilir, bu yüzden yalnızca o veritabanını tutar ve MySQL 8.4 ve üstü onu tümden reddedebilir. Konteynere gerçek bir root parolası ver ya da onun için dökümü kapat.
- **Sistem tablolarının yükseltilmesi gerekiyor.** MariaDB, sistem tabloları daha eski bir sürümdense dökmeyi reddeder (hata 1558). `MARIADB_AUTO_UPGRADE=1` değişkenini ekle ve konteyneri yeniden başlat ya da içinde bir kez `mariadb-upgrade` çalıştır.
- **Döküm aracı yok.** `pg_dump`, `mysqldump` ya da `mariadb-dump` olmayan, kısaltılmış ya da kendi derlenmiş bir image dökülemez. Resmi image'i kullan ya da dökümü kapat.
- **Bir zaman sınırı.** Bir döküm `DB_DUMP_MAX_HOURS` (varsayılan 6) alır, çevresindeki yedek `BACKUP_MAX_HOURS` alır ve ilerlemeyi bırakan bir döküm `BACKUP_STALL_HOURS` sonra kesilir. Sonuncusunun olağan nedeni uygulamanın tuttuğu bir kilittir. Tetiklenen sınırı yükselt ya da uygulama sakinken dök.
- **Konteyner duraklatılmış ya da yeniden başlıyor.** Döküm çalışan sunucuyla konuşur. Konteyner sürekli yeniden başlıyorsa nedenini kendi günlüğü söyler.
- **Hasarlı bir döküm kaldırılamadı.** BombVault'un bitiremediği bir döküm yeniden silinir. O silme başarısız olursa döküm hasarlı işaretiyle listede kalır ve orada silebilirsin.

## Bir içe aktarma başarısız oldu {#an-import-failed}

Bir içe aktarma konteyneri durdurur, veri klasörünü kenara taşır ve image'in yerine boş bir klasör oluşturmasına izin verir. İçe aktarmanın kendisinden önceki bir adım başarısız olursa eski klasör kendiliğinden geri konur. İçe aktarma başarısız olursa konteyner yeni klasörü tutar ve eskisi yanında `<data folder>.bombvault-before-import-<timestamp>` olarak kalır. Çalıştırmanın hata mesajı tam yolu adlandırır.

Kendi başına bir veri kümesi olan bir veri klasörü, TrueNAS'ta olduğu gibi, yeniden adlandırılamaz. İçe aktarma o zaman klasörü yerinde bırakır ve içeriğini yanındaki saklanan klasöre taşır, bu yüzden yukarıdaki veri kümesinin bunlar için yere ihtiyacı vardır. İçe aktarma bunu konteyneri durdurmadan önce kontrol eder.

Elle geri koymak için: konteyneri durdur, mevcut veri klasörünü yoldan çek, saklanan klasörü özgün adına geri çevir ve konteyneri başlat. Bir veri kümesi için klasörleri yeniden adlandırmak yerine içeriği taşı. Unraid'de dosya yöneticisi bunu Shares sekmesinden yapar.

## Bir ZFS veri kümesi yedeği başarısız oldu ya da bir veri kümesini atladı {#zfs-datasets}

Her sorun parantez içinde bir neden kodu taşır ve [ZFS veri kümeleri](zfs-datasets.md#reason-codes) sayfası hepsini çözümüyle listeler. En yaygın üçü:

- **`snapshot-loop`**: anlık görüntü BombVault'a ulaşmadı, çünkü Host Data yeni bağlamaları geçirmiyor. Konteyneri düzenle, Host Data'nın Erişim Kipini Read/Write - Slave yap ve BombVault'u yeniden başlat.
- **`key-not-loaded`**: anahtarı yüklenmemiş şifreli bir veri kümesi atlanır. Anahtarı `zfs load-key` ile yükle ve veri kümesini bağla. Sonraki yedek onu içerir.
- **`ssh-auth`**: sunucu BombVault'un anahtarını reddetti. Yedekler sayfasında **Bağlantıyı göster** ile açılan **Bu sunucuyla ZFS bağlantısı**, onu yetkilendiren komutu gösterir. Sunucuda bir kez çalıştır.

## Bir öge "Öğreniyor N/10" üzerinde kalıyor {#an-entry-stays-at-learning-n10}

Anormallik denetimlerinin çoğu bir ögenin 10 başarılı yedeğinden sonra başlar ve sayım **Beklenen olarak işaretle**'den ve ögenin seçimi değiştikten sonra yeniden başlar. Zamanlanmamış bir öge öğrenmez ve appdata'sı olmayan bir konteynerin öğrenecek bir şeyi yoktur, rozeti bunu söyler. Anormallikler sayfasındaki **Öğrenme ilerlemesi** öge başına on parçalı bir halka gösterir.

## Saklama bir ögenin eski yedeklerini silmeyi bıraktı {#retention-stopped-deleting-old-backups-of-one-entry}

Onları açık bir kritik anormallik tutuyor: ögenin kaynağı neredeyse boş, sert biçimde küçüldü ya da bir yedek verisinin çoğunu yeniden sakladı. Anormalliği ögedeki rozetten aç. Veri eksikse ya da şifrelenmişse önce bağlantılı son iyi yedekten geri yükle. Sonra anormalliği onayla ya da değişikliği sen yaptıysan beklenen olarak işaretle, sonraki çalıştırma her zamanki gibi budar. Saklama önizlemesi böyle bir ögeyi tutulan olarak işaretler. Bir ZFS ögesinde yalnızca anormallikte adı geçen veri kümesi eski yedeklerini tutar, ağacın diğer veri kümeleri her zamanki gibi budanır.

## Elle budama bazı ögelerin tutulduğunu söylüyor {#manual-prune-says-some-entries-were-kept}

Aynı neden: budama, böyle bir anormalliği olan bir ögenin eski yedeklerine dokunmaz ve mesajında ögeyi adlandırır. Geri kalan her şey her zamanki gibi budanır.

## Bir yedek "on no mounted pool or share" ile başarısız oluyor {#no-mounted-pool}

Unraid'de `/mnt` bellekte yaşar, bu yüzden orada üzerine havuz bağlanmamış bir klasör sonraki yeniden başlatmada kaybolur ve havuz geri geldiğinde gizlenir. BombVault oraya bir yedek ya da flash ZIP'i yazmayı reddeder, önceki bir yazmadan kalan bir depo o klasörde zaten duruyor olsa bile. Havuzu bağla ve yedeği yeniden çalıştır.

## Geçmiş içe aktarma bir deponun okunamadığını söylüyor {#history-import-says-a-repository-could-not-be-read}

Yükseltmeden sonra BombVault önceki yedeklerin boyutlarını her depodan bir kez okur. O sırada ulaşamadığı bir depo, örneğin kapalı olan bir site dışı hedef ya da bağlı olmayan bir paylaşım, **Ayarlar, Doğrulama** altındaki **Anormallikler** kartında sayılır ve günde bir kez yeniden denenir. Ögeleri bu arada yeni yedeklerden öğrenir.

## Disk alanı uyarısı Unraid panosuyla uyuşmuyor {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Unraid kullanıcı paylaşımında (`/mnt/user`) boş alan tek bir diskin değil tüm dizinin boş alanıdır. BombVault'un rclone üzerinden eriştiği uzak hedefler, örneğin SFTP, Hetzner Storage Box, SMB, WebDAV ya da bir bulut sürücüsü, karşı taraf boş alanını bildirdiğinde ölçülür, elle yazılmış bir `sftp:` adresi de öyle. S3, B2 ve REST depolarında rakam yoktur: hedefleri **Boyut bilinmiyor** der ve **Anormallikler** kartı onları ölçülmemiş olarak listeler.

## Bir yapay zeka asistanı bağlanamıyor {#an-ai-assistant-cannot-connect}

[MCP sunucusu sayfası](mcp.md#troubleshooting), MCP uç noktasının her durum kodunun ve her reddinin ne anlama geldiğini ve ne yapılacağını listeler.

## Konteyner sürekli yeniden başlıyor ya da sağlıksız görünüyor {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault sağlıklı/sağlıksız durumunu kendi `/api/health`'inden bildirir. Bir otomatik onarım aracı (Autoheal gibi) motor bir gün takılırsa onu kendiliğinden yeniden başlatabilir. Asıl neden için konteyner günlüğüne ve **Host Entegrasyonu** kartına bak.

## Bir hedef kaydedilmeyi reddediyor {#location-refused}

BombVault bir hedefi kaydetmeden önce denetler ve ret nedenini bir cümleyle söyler. Olağan olanlar:

- **"Bu konum başka bir depo ya da hedefin içinde duruyor ya da birini içeriyor."** Bir depo içinde bir depo ikisini de bozar. Mesaj diğer yeri adlandırır. Başka bir klasör seç ya da önce onu kaldır.
- **Senin kurduğun bir depo.** Konum **Depolar** altındaki satırlardan biridir ya da birinin içinde ya da çevresinde durur. Başka bir yer seç ya da önce o depoyu kaldır.
- **Bir bölümün kendi klasörüyle aynı yer.** Bir bölümün kendi klasörüne bir kopya, her çalıştırmada yedeklerini kendi üstüne kopyalardı, bu yüzden reddedilir. Başka bir yer seç ya da bkz. [Bir bölümün kendi klasörü](offsite-recovery.md#own-folder).
- **"Bu, bu bölümün bir site dışı hedefi değil"**, kaldırılmış bir hedefe giden bir bağlantıdan geldi. Güncel bir sürüm bunları başlangıçta ve bir içe aktarmada temizler. Hâlâ görüyorsan BombVault'u güncelle.
- `/mnt/user/backups` gibi **tam bir ana makine yolu**. BombVault bir konteynerde çalışır, bu yüzden Host Data altındaki yolu, `user/backups`, ister ve mesaj girilecek olanı adlandırır.

## Uzak bir hedef deposunun kaybolduğunu söylüyor {#repository-gone}

Yedek tutan ve birdenbire hiç depo tutmayan bir hedef kendiliğinden yeni, boş bir tane almaz, çünkü bir disk geri geldiğinde o, gerçek olanı gizlerdi. Disk bağlanmayan bir rest-server olağan nedendir, bu yüzden diski yeniden bağla ve yedeği ya da kopyayı bir kez daha çalıştır. Eski depoyu bilerek boşalttıysan ya da sildiysen ancak o zaman hedefin sayfasında **Burada yeni bir depo oluştur**'a bas. O hedefin her bölümü için bir kerede baştan başlar.

## Bir bulut oturumunun süresi doldu {#cloud-sign-in-expired}

Test ya da sonraki kopya o zaman oturumun süresinin dolduğunu söyler. Hedefin sayfasındaki **Bağlantıyı değiştir** yeniden oturum açar.

- **Box, Jottacloud ve tek kullanımlık yenileme belirteçli diğerleri:** BombVault rclone'un yenilediği belirteci, yeniden başlatmalar boyunca ve kurtarma kitinde de, tutar. Daha eski bir ayar yedeğinden geri yükledikten sonra içindeki belirtecin süresi dolmuş olabilir, bu yüzden bir kez daha oturum aç. Depo olduğu gibi kalır.
- **iCloud Drive:** Apple oturumu 30 gün sonra sonlandırır. `rclone config`'ten yeni bir güven belirteci ve çerezler gerekir.
- **OneDrive:** birden çok sürücülü bir oturum hangisinin kullanılacağını sorar, bu yüzden seçimsiz bırakılan bir hedef sen seçene kadar kullanılamaz kalır. OneDrive Business ve Google Drive için [Hedefler](offsite-recovery.md#destinations) altındaki budanan veri notuna bak.

## "Son yedeğin site dışı kopyası hâlâ çalışıyor" {#copy-still-running}

Ondan önceki yedek bitti ve kopyası hâlâ yolda, bu da o bölümün yeni bir yedeğini engeller. Durum başlığı artık **Site dışı kopyalama sürüyor** demeyene kadar bekle. Bir yedek başlatan düğmeler beklerken aynısını söyler. Yeni bir hedefe ilk kopya saatler sürebilir, bkz. [S3 örneği](offsite-recovery.md#example-s3).

## Bir ZFS replikası hedef daha yeni anlık görüntüler tuttuğu için duruyor {#replica-target-newer}

Hedef, bu sunucuyla paylaştığı en yeni olandan daha yeni anlık görüntüler tutuyor, örneğin buradaki veri kümesi geri alındığı ya da hedefteki kopya değiştirildiği için. Replika kartı ve çalıştırma geçmişi anlık görüntüyü **Hedefi şuraya geri al** altında adlandırır. Çalıştırmanın adlandırdığı her veri kümesi için hedefte o anlık görüntüye `zfs rollback -r` çalıştır ve **Şimdi çoğalt**'a bas. Bkz. [Anlık görüntüler ne kadar kalır](zfs-datasets.md#replica-retention).

## Unraid anlık görüntülü bir VM başarısız oluyor ve bir dosyayı adlandırıyor {#vm-chain}

BombVault her diskin dayanak zincirini izler ve içindeki her dosyayı yedekler. Sonuna kadar izlenemeyen bir zincir, diskin bir kısmını dışarıda bırakmak yerine yedeği başarısız kılar ve mesaj dosyayı adlandırır. Dosyanın `/mnt` altında var olduğunu ve anlık görüntünün VM Manager'da hâlâ listelendiğini kontrol et. Host Data dışındaki bir dosya hiç okunamaz. Bkz. [Unraid anlık görüntülü VM'ler](vm-backup-ssh-setup.md#unraid-snapshots).

## Site dışı kopyalar yeniden kurulumdan sonra kararını bekliyor {#copies-paused}

**Yedekleri keşfet**'ten ya da geri yüklenmiş ayarlar olmadan Acil durum sihirbazından sonra ya da bir bölüm bu veritabanından önceki kendi yedeklerini bulduğunda, o bölümün site dışı kopyaları bekler, çünkü BombVault artık hangi ögeleri dışarıda bıraktığını bilmez. **İlgi bekliyor** ve hedefin sayfası bunu sorar. Devam etmek için **Bu sunucunun kendi yedekleri olarak al**'a, ya da önce bölümün kurallarına bakmak için **Önce kuralları kontrol et**'e bas. Bir ayar dosyası içe aktarmak kararı senin yerine vermez. Bkz. [Yeniden kurulumdan sonra](offsite-recovery.md#after-a-rebuild).

## Bildirimler gelmiyor {#notifications-do-not-arrive}

**Ayarlar, Bildirimler** altında **Test gönder** ile başla. Kartın altındaki satır ne olduğunu söyler.

- **Hiçbir şey gönderilmiyor.** **Bildir**'i kontrol et. **Hiç** ile hiçbir mesaj gitmez, yalnızca bir Healthchecks URL'sinin ping'leri gider, **Yalnızca başarısızlıkta** ile de iyi bir çalıştırma sessiz kalır. **Zamanlanmış çalıştırmaları özetle** açıkken zamanlanmış bir çalıştırma sonunda tek bir özet gönderir.
- **Yeni bir adres.** Matrix, Gotify ya da e-posta sunucusunu değiştirdikten sonra alan belirteci ya da parolayı yeniden ister ve yeni adres yalnızca sen yazınca kaydedilir. BombVault eski gizli bilgiyi yeni yere göndermez.
- **Gotify**, sunucunun uygulama belirtecini reddettiğini söylüyor: onu Gotify'daki uygulamadan yeniden kopyala. **Sunucu URL'si** `/message` olmadan temel adrestir.
- **Webhook:** **Webhook biçimi**'nin servise uyduğunu kontrol et. Bir Discord adresi **Discord**, ntfy ise **ntfy** ister.
- **Unraid bildirimleri** Ana makine SSH bağlantısı üzerinden çalışır, bu yüzden önce onu **Ayarlar, Entegrasyonlar** altında test et. Başka bir platformda kapalı kalırlar ve kart bunu söyler.
- **Bildirim e-postası hiç gelmiyor.** **Şifreleme**'yi porta göre kontrol et, genellikle 587'de **STARTTLS** ya da 465'te **TLS (implicit)**.

Healthchecks pingleri bunların hiçbirine bağlı değildir, bkz. [Healthchecks](notifications.md#healthchecks).

## Android uygulaması bir sunucuyu açamıyor {#android-cannot-open}

Başlangıç ekranı nedenini sunucunun kartında söyler:

- **Bağlı değil:** ne sunucunun adresi ne de röle yanıt veriyor. **Röle yok** ile uygulama bir sunucuya yalnızca kendi adresinden ulaşır.
- **Sertifikayı kontrol et:** sunucu güvendiğinden başka bir sertifika gösteriyor. Uygulama onu yalnızca yenisine güvendikten sonra açar, bu yüzden değişikliğin senin olduğundan emin ol.
- **Oturum açma gerekiyor:** adresiyle eklenen bir sunucu parola ister. Bkz. ayrıca [Bir telefon giriş sayfasına düşüyor](#a-phone-lands-on-the-login-page).
- **Anahtar değişti:** başlangıç ekranı yeni anahtarın parmak izini gösterir ve sayfa yalnızca ona güvendikten sonra açılır. Bkz. [Evden uzakta](android.md#away).

## Başka bir kullanıcı yerel depoyu okuyamıyor {#repository-permissions}

Yerel depo, SMB üzerinden bir senkronizasyon kullanıcısı gibi diğer kullanıcılar için okunabilir kalır. 10.4.1'den önce bir budama, etiket, çekme, bölüm taşıma, silme ya da stack yedeğinin yazdığı dosyalar bir sonraki geceye kadar okunamaz kalabiliyordu. BombVault'u güncelle, hemen okunabilir olurlar. Öz yedek deposu, BombVault'un ayarlarını tuttuğu için bilerek yalnızca root tarafından okunabilir kalır.

## Hâlâ takıldın mı? {#still-stuck}

- Tam [Yapılandırma](configuration.md) ve [Site dışı ve kurtarma](offsite-recovery.md) sayfalarını oku.
- **Ayarlar, Bilgi** altındaki **Yardım** kartında **Hata bildirimi**, bir GitHub issue'su için bir ZIP indirir: ana makine denetimi, her gizli bilgisi kaldırılmış yapılandırman, son çalıştırmalar, sırada ne zamanlandığı ve konteynerin son başlamasından bu yana son günlük. İçinde ayrıca her veritabanının son dökümü, bağlamalarıyla ZFS ögeleri, açık anormallikler ve kaç MCP anahtarı olduğu da var. Parolalar, belirteçler ve rclone yapılandırması çıkarılır, paket bunu kendi manifestosunda söyler. Giriş parolası gerekir. Konteyneri yeniden başlatan bir çökme için `docker logs`'a bak.
- [Unraid destek konusunda](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/) sor.
- Bir [GitHub issue'su](https://github.com/junkerderprovinz/bombvault/issues) aç.
