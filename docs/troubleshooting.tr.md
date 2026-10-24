# Sorun giderme

Kısa bir SSS. Tam VM-üzeri-SSH host tarafı sorun giderme tablosu için (izin reddedildi, host anahtarı doğrulaması, eksik şablon değişkenleri ve daha fazlası), GitHub'daki [SSH üzerinden VM yedekleme kılavuzu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'na bak.

## Bir şey doğru bağlanmamış

Genel bakışa **Özelleştir** altından **Host Entegrasyonu** kartını ekle. Host entegrasyon denetimi her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir; **Şimdi denetle** onu yeniden çalıştırır. Bir hata olduğunu varsaymadan önce buradan başla: eksik bir bağlama ya da ulaşılamayan bir host hemen görünür.

## Web arayüzüne ulaşamıyorum

BombVault, kutudan çıktığı gibi `3443` portunda HTTPS sunar (kendinden imzalı sertifika), bu yüzden `https://<your-unraid-ip>:3443` adresini aç. Kendinden imzalı sertifika uyarısını kabul et ya da BombVault'u kendi sertifikanla bir ters proxy arkasına yerleştir. `HTTP_ONLY=true` ile çalıştırırsan, bunun yerine `3000` portunda düz HTTP sunar (TLS'yi sonlandıran bir proxy arkasında kullanım için tasarlanmıştır).

## APP_KEY'imi kaybettim

`APP_KEY` restic depo parolasını türetir. O olmadan, kurtarma kiti olmadan ve grubunda bir anahtar emaneti olmadan şifreli yedekler kurtarılamaz. Bu yüzden genel bakış, kiti indirene kadar **Kurtarma kitini sakla** gösterir. Bkz. [Site dışı ve kurtarma](offsite-recovery.md#recovery-kit). Herhangi bir yedeğe güvenmeden önce `openssl rand -hex 32` ile bir anahtar üret ve sunucunun dışında sakla.

## Giriş parolamı unuttum

Giriş sayfasında **Parolanı mı unuttun?** seçeneğini seç. BombVault, konteyner günlüğüne 12 karakterlik bir kod yazar (Unraid'de: Docker sekmesi, BombVault simgesi, **Logs**). Onu 10 dakika içinde yeni bir parolayla birlikte yaz. Sıfırlama ayrıca iki adımlı doğrulamayı kapatır, her geçiş anahtarını kaldırır, her API belirtecini, MCP anahtarını ve OAuth oturumunu iptal eder, diğer tüm oturumları sonlandırır ve telefon eşleştirmesini kapatır. Geçiş anahtarlarını **Ayarlar, Güvenlik** altında yeniden ekle, telefon eşleştirmesini orada tekrar aç ve hâlâ ihtiyaç duyduğun yerlerde yeni belirteçler ve anahtarlar oluştur. Bkz. [Güvenlik modeli](configuration.md#login).

## Yedeklemeler durdu ve anahtarın artık açılamadığını söylüyor

Bu sunucu, Acil durum sihirbazında başka bir sunucunun yedek anahtarını devraldı ve sonra `APP_KEY` değiştirildi. Devralınan anahtar eski APP_KEY ile mühürlü olduğundan BombVault onu açamaz ve yanlış anahtarla yazmaktansa hiçbir şey yedeklemez. Genel bakıştaki **İlgi bekliyor** bunu adıyla belirtir. APP_KEY'i eski haline getir ya da Geri yükleme sayfasında **Acil durum** altında kurtarma kitini yeniden yükle.

## Bir telefon giriş sayfasına düşüyor

Eşleştirilmiş bir telefon oturumunu grup üzerinden yalnızca **Ayarlar, Güvenlik** altında **Telefonlar grubun on iki kelimesiyle eşleşebilir** açıkken alır. Parola sıfırlama bunu kapatır. Sunucudan eski bir uygulama da güncellenene kadar oturum alamaz. Adresiyle eklenen bir sunucu her zaman parola ister. Örnekler sayfasından kaldırılan bir telefon, uygulaması yeniden açıldığında kapalı olarak geri gelir; içeri almak için ayrıntılarındaki **İzleme** altında **Bu örneği sorgula**'yı aç.

## VM yedeklemesi bağlanmıyor

VM yedeklemesi, bir bağlamayla değil, SSH üzerinden libvirt ile konuşur.

- Host'ta SSH'nin etkin olduğunu ve BombVault'un genel anahtarının `/root/.ssh/authorized_keys` içinde yetkilendirildiğini onayla (Ayarlar, Entegrasyonlar, Ana makine SSH anahtarı ve bir **Bağlantıyı test et** düğmesi gösterir).
- Özel bir `br0.x` ağında, `LIBVIRT_HOST`'u Unraid LAN IP'ne ayarla (konteyner orada host'a `host.docker.internal` üzerinden ulaşamaz). **Settings, Docker, Host access to custom networks**'ni etkinleştir.
- Unraid'in SSH portunu değiştirdiysen, eşleşmesi için `LIBVIRT_SSH_PORT`'u ayarla.
- TrueNAS'ta `LIBVIRT_URI`'yi TrueNAS'ın kendi libvirt soketiyle ayarla, uygulamaya `SETPCAP` yetkisini ver ve anahtarı **Credentials, Users** altında root'a ekle (bkz. [TrueNAS SCALE](getting-started.md#truenas)).
- Tam adım adım tanılama (ulaşılabilirlik testi, VLAN yönlendirme, `Permission denied (publickey)`, `Host key verification failed`) [SSH üzerinden VM yedekleme kılavuzu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'nda yer alır.

## Bir canlı VM anlık görüntüsü çalışmadı

Canlı anlık görüntüler, VM'de qemu guest agent'ın kurulu olmasını ve diskin `/mnt/user` değil `/mnt/cache` (ya da `/mnt/diskX`) üzerinde olmasını gerektirir. Kapalı bir VM'de, canlı otomatik olarak düzgüne geri düşer. Düzgün bir yedekleme VM'yi kapatır, diskleri yedekler, ardından yeniden başlatır, böylece her zaman tutarlıdır. Konuğun kapanması için on dakikaya kadar bekler ve onu asla zorla kapatmaz; böylece güncelleme kuran bir Windows konuğu yarıda kesilmez. Bundan sonra hâlâ çalışan bir konuğa dokunulmaz ve o yedekleme nedeniyle birlikte başarısız olur.

## Bir yedekleme "repository is already locked" ile başarısız oldu

Bu genellikle konteyner bir işlemin ortasında güncellendiğinde ya da yeniden başlatıldığında geride kalan sahipsiz bir restic kilididir. BombVault kanıtlanabilir biçimde sahipsiz bir kilidi algılar, zorla temizler ve otomatik olarak bir kez yeniden dener. Sürerse **Ayarlar, Doğrulama** altındaki **Yedekleri denetle** kartını aç ve eski bir kilidi elle temizlemek için etkilenen bölümün satırında **Diğer** altından **Kilidi aç**'ı seç. Gerçek bir sorun yine de gizlenmez, ortaya çıkar. Yeniden başlatmanın ardından BombVault, böyle bir kilit on dakika boyunca yenilenmeden kalana kadar bekler. Hâlâ çalışan bir restic, örneğin aynı depoyu kullanan ikinci bir BombVault'ta, kilidini her beş dakikada bir yeniler.

## Bir yedeklemeden sonra site dışı kopyam gerçekleşmedi

Site dışı çoğaltma tasarım gereği en iyi çabayla çalışır, bu yüzden site dışında yaşanan bir aksaklık yerel yedeği asla başarısız kılmaz. Hedefin ne zaman kopyaladığına bak: **Kopyala** kartı **Her yedekten sonra** der ya da kendi planını adıyla belirtir, **Ayarlar, Zamanlama, Bölüm başına site dışı kopyalar** ise bir bölüme kendi planını verebilir. İsteğe bağlı bir çalıştırma için **Ayarlar, Hedefler** altında bölümün **Site dışı kopya** kartındaki **Şimdi çoğalt**'ı kullan ve çalışırken **Site dışı kopyalama sürüyor** diyen genel bakışın durum başlığını izle. Başarısız bir kopya **İlgi bekliyor** altında görünür.

## Bir rest-server'ın sertifikasına güvenilmiyor

restic tanımadığı bir sertifikayı reddeder ve hata onu nereye koyacağını söyler. rest-server'ın kendinden imzalı sertifikasını ya da onu imzalayan yetkilinin sertifikasını PEM dosyası olarak BombVault'un veri klasöründeki `certs` klasörüne kopyala (konteynerde `/config/certs`, Unraid'de `appdata/bombvault/certs`) ve yeniden test et. Yeniden başlatma gerekmez.

## Bir yedek, Host Data dışındaki verilerin yedeklenmediğini söylüyor {#data-outside-host-data}

Konteyner, BombVault'un Host Data üzerinden okuyamadığı bir klasörde veri tutuyor. Yedek, tanımı ve ulaşabildiği her klasörü kaydeder, etkinlik günlüğündeki satırını turuncuya boyar ve dışarıda bıraktığı klasörleri adlandırır. İki durum sık görülür: TrueNAS'ta `/mnt/.ix-apps` altındaki bir ixVolume ve `/var/lib/docker/volumes` altındaki adlandırılmış bir Docker birimi, örneğin bir compose yığınının veritabanı birimi. Konteynerin **Ayarlar** sekmesindeki **Yedeklenecek klasörler** kartı bu tür her klasörü konteyner içindeki yolu ve ana makinedeki yoluyla, kırmızıyla işaretli olarak gösterir.

Anonim bir Docker birimi bildirilmez. Docker onu, image'in tanımladığı ama kimsenin eşlemediği bir klasör için kendiliğinden oluşturur, 64 onaltılık basamaktan oluşan bir ad verir ve `/var/lib/docker/volumes` altında tutar. İçinde genellikle önbellek bulunur, bu yüzden yedek yeşil kalır ve kart onu yalnızca anonim olduğunu belirten bir notla gösterir.

- **Bu verilerin yedeklenmesini istiyorsun.** Klasörü Host Data altındaki bir ana makine yoluna, örneğin appdata paylaşımındaki bir klasöre eşle; bir sonraki yedek onu da alır.
- **Onlara ihtiyacın yok.** İki yolundan birini aynı sekmedeki **Hariç tutma desenleri** kartına bir satır olarak ekle: konteyner içindeki yolu, örneğin `/cache`, ya da uyarının adını verdiği ana makine yolunu, örneğin `/var/lib/docker/volumes/jellyfin_cache/_data`. Bir sonraki yedekten itibaren uyarı o klasörü dışarıda bırakır ve alanın altındaki önizleme nedenini söyler. Üstteki bir klasör için yazılan satır da işe yarar, düz bir ad ya da `/` yaramaz.

## Bir yedek Ertelendi gösteriyor {#deferred-backups}

Konteynerin uygulaması meşguldü, bu yüzden yedeklemesi bekliyor ve kendiliğinden yeniden denenecek. Etkinlik günlüğündeki satır nedenini ve bir sonraki denemenin ne zaman olacağını söyler; konteynerin **Ayarlar** sekmesindeki **Uygulama meşgulken** kartı da aynısını ve beklemenin ne zaman biteceğini söyler. Bir şey yapman gerekmez. Hemen yedeklemek için uygulama boşalınca **Şimdi yedekle**'yi kullan.

- **Kendi denetimin.** Uygulama meşgulken **Yedekleme öncesi komut** 75 koduyla çıksın, örneğin `[ "$(cat /tmp/players)" = 0 ] || exit 75`. 0 olmayan başka her kod yedeklemeyi eskisi gibi başarısız kılar, bu yüzden bozuk bir betik yine **İlgi bekliyor** altında görünür.
- **Plex, Jellyfin ya da Emby'ye hiç sorulmuyor.** BombVault önce sunucuya konteynerin kendi adresinden sorar, oradan yanıt gelmezse konteynerin içinde curl ya da wget çalıştırır. İkisi de olmayan bir image kendi üzerinden sorgulanamaz; günlük bunu o zaman belirtir. **Sunucu yanıt vermezse yedekle** açıksa böyle bir yedekleme yine de çalışır, kapalıysa bekler.
- **Jellyfin ya da Emby reddediyor.** **API anahtarı**'nı denetle. Sunucunun panelinde API keys altında oluşturulur.
- **Plex reddediyor.** BombVault belirteci `/config` olarak bağlanan klasördeki `Preferences.xml` dosyasından okur. Hiç oturum açılmamış bir sunucunun belirteci yoktur ve yalnızca belirteçsiz erişime izin verdiği yerde yanıt verir.
- **Duraklatılmış bir film yedeklemeyi bütün gece bekletiyor.** **Duraklatılmış oturumları şu süreden sonra yok say (dk)** değerini ayarla, örneğin 30.
- **Trafik denetimi yedeklemenin geçmesine hiç izin vermiyor.** **Şu hızdan itibaren meşgul (KiB/s)** değerini, kimse kullanmazken konteynerin gönderdiğinin üstüne çıkar. BombVault başladıktan hemen sonra denetim, konteyneri **Sessiz kalma süresi (dk)** ile belirlenen dakika boyunca izleyene dek bekler.
- **Yedekleme gece hiç çalışmıyor.** Bir aralık ayarlandığında denemeler yalnızca onun içinde olur. Son tarihine aralığın dışında ulaşan bir bekleme yine de son tarihte biter.

## Büyük bir yedekleme belleği tükendiği için sonlandırıldı

**Ayarlar, Sistem** altındaki **Performans** kartında **Yedekleme başına bellek sınırı (MiB, 0 = yok)** değerini, konteynerin kullanabileceği belleğin epey altına ayarla. restic sınıra yaklaştıkça belleği daha sık boşaltır; bu biraz hıza mal olur, ama yedekleme sonlandırılmak yerine yavaşlar.

## Bir geri yükleme başlamadan iptal oldu

Herhangi bir şey durdurulmadan ya da kaldırılmadan önce, geri yükleme bir uçuş öncesi çakışma denetimi çalıştırır: konteynerin statik IP'sinin ve yayımlanan host portlarının serbest olduğunu doğrular. Başka bir konteyner zaten birini tutuyorsa, yarım kalmış bir geri yükleme bırakmak yerine açık, uygulanabilir bir mesajla iptal eder. Çakışan portu ya da IP'yi serbest bırak, ardından yeniden dene.

Konteynerin ağı yoksa, örneğin TrueNAS uygulaması silindiği için, geri yüklemeden önceki denetim ağın adını belirtir ve geri yükleme hiçbir şeyi değiştirmeden reddeder. Uygulamayı TrueNAS'ta aynı adla yeniden kur, bu ağı yeniden oluşturur, sonra verisini geri yükle.

## Bir düz dışa aktarma, dosya yazmak yerine başarısız oldu

**Ayarlar, Sistem** altında **Dışa aktarmaları age ile şifrele** açıksa ama geçerli bir alıcı ayarlanmamışsa, bir dışa aktarma düz metin yazmak yerine açık bir hatayla başarısız olur. Geçerli bir alıcı ekle (bir age genel anahtarı ya da bir SSH genel anahtarı) veya dışa aktarmanın düz metin olmasını istiyorsan şifrelemeyi kapat. Bkz. [Özellikler](features.md).

## Bir veritabanı dökümü başarısız oldu

Başarısız bir döküm, çevresindeki yedeklemeyi asla düşürmez; kendi başına başarısız bir çalışma olarak kaydedilir ve nedeni neyin düzeltileceğini söyler.

- **Giriş reddedildi.** Döküm, konteynerin kendi parola değişkenleriyle oturum açar (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` ya da bunların `_FILE` sürümleri). Bunları veritabanı konteynerinde denetle. Konteynerin kendi kullanıcısının okuyamadığı bir sırrı gösteren `_FILE` değişkeni de aynı şekilde başarısız olur.
- **Eksik yetkiler.** Rastgele bir root parolasıyla döküm yalnızca uygulama kullanıcısı olarak girebilir, dolayısıyla yalnızca o tek veritabanını içerir; MySQL 8.4 ve sonrası ise büsbütün reddedebilir. Konteynere gerçek bir root parolası ver ya da dökümünü kapat.
- **Sistem tabloları yükseltme istiyor.** MariaDB, sistem tabloları daha eski bir sürümden geliyorsa dökümü reddeder (hata 1558). `MARIADB_AUTO_UPGRADE=1` değişkenini ekleyip konteyneri yeniden başlat ya da içinde bir kez `mariadb-upgrade` çalıştır.
- **Döküm aracı yok.** `pg_dump`, `mysqldump` veya `mariadb-dump` içermeyen ince ya da elde yapılmış bir imajın dökümü alınamaz. Resmi imajı kullan ya da dökümü kapat.
- **Bir süre sınırı.** Bir döküme `DB_DUMP_MAX_HOURS` (varsayılan 6), çevresindeki yedeklemeye `BACKUP_MAX_HOURS` tanınır; ilerlemeyi kesen bir döküm ise `BACKUP_STALL_HOURS` sonunda kesilir. Sonuncusunun ardında genellikle uygulamanın tuttuğu bir kilit vardır. Devreye giren sınırı yükselt ya da uygulama sakinken döküm al.
- **Konteyner duraklatılmış ya da yeniden başlıyor.** Döküm, çalışan sunucuyla konuşur. Konteyner sürekli yeniden başlıyorsa nedenini kendi günlüğü söyler.
- **Bozuk bir döküm kaldırılamadı.** BombVault'un tamamlayamadığı bir döküm yeniden silinir. Bu silme başarısız olduğunda döküm, bozuk olarak işaretli biçimde listede kalır ve oradan silebilirsin.

## Bir içe aktarma başarısız oldu

İçe aktarma konteyneri durdurur, veri klasörünü kenara alır ve yerine imajın boş bir klasör oluşturmasına izin verir. İçe aktarmanın kendisinden önceki bir adım başarısız olursa eski klasör kendiliğinden geri konur. İçe aktarma başarısız olursa konteyner yeni klasörle kalır, eskisi de yanında `<veri klasörü>.bombvault-before-import-<zaman damgası>` adıyla durur; çalışmanın hata iletisi tam yolu belirtir.

TrueNAS'ta olduğu gibi kendisi ayrı bir veri kümesi olan bir veri klasörünün adı değiştirilemez. İçe aktarma o zaman klasörü yerinde bırakır ve içeriğini yanındaki saklanan klasöre taşır, bu yüzden üstteki veri kümesinde onlar için yer olmalıdır. İçe aktarma bunu konteyneri durdurmadan önce denetler.

Elle geri koymak için: konteyneri durdur, mevcut veri klasörünün adını değiştirip yoldan çek, saklanan klasörü özgün adına geri döndür ve konteyneri başlat. Bir veri kümesinde klasörlerin adını değiştirmek yerine içeriği taşı. Unraid'de bunu Shares sekmesindeki dosya yöneticisi yapar.

## Bir ZFS veri kümesi yedeği başarısız oldu veya bir kümeyi atladı {#zfs-datasets}

Her sorunun köşeli parantez içinde bir neden kodu vardır ve [ZFS veri kümeleri](zfs-datasets.md#reason-codes) sayfası hepsini çözümüyle listeler. En sık üçü:

- **`snapshot-loop`**: Host Data yeni bağlamaları iletmediği için anlık görüntü BombVault'a ulaşmadı. Konteyneri düzenle, Host Data'nın Access Mode ayarını Read/Write - Slave yap ve BombVault'u yeniden başlat.
- **`key-not-loaded`**: anahtarı yüklenmemiş şifreli bir veri kümesi atlanır. Anahtarı `zfs load-key` ile yükle ve kümeyi bağla; sonraki yedek onu da alır.
- **`ssh-auth`**: sunucu BombVault'un anahtarını reddetti. Yedekler sayfasında **Bağlantıyı göster** ile açılan **Bu sunucuyla ZFS bağlantısı**, onu yetkilendiren komutu gösterir; sunucuda bir kez çalıştır.

## Bir öge "Öğreniyor N/10" durumunda kalıyor

Çoğu anormallik denetimi, bir ögenin 10 başarılı yedeğinden sonra başlar ve sayım, **Beklenen olarak işaretle**'den sonra ve ögenin seçimi değiştiğinde yeniden başlar. Zamanlanmamış bir öge öğrenmez, appdata'sı olmayan bir konteynerin de öğrenecek bir şeyi yoktur; bunu rozeti söyler. Anormallikler sayfasındaki **Öğrenme ilerlemesi**, öge başına on parçalı bir halka gösterir.

## Saklama, bir ögenin eski yedeklerini silmeyi bıraktı

Açık bir kritik anormallik onları tutuyor: ögenin kaynağı neredeyse boş, sert biçimde küçüldü ya da bir yedek verisinin çoğunu yeniden depoladı. Anormalliği ögedeki rozetten aç. Veri eksikse ya da şifrelendiyse önce bağlantılı son iyi yedekten geri yükle. Ardından anormalliği onayla ya da değişiklik seninse beklenen olarak işaretle; sonraki çalışma her zamanki gibi budar. Saklama önizlemesi böyle bir ögeyi tutulmuş olarak işaretler. Bir ZFS ögesinde yalnızca anormallikte adı geçen veri kümesi eski yedeklerini tutar, ağacın diğer veri kümeleri her zamanki gibi budanır.

## Elle budama bazı ögelerin tutulduğunu söylüyor

Aynı neden: budama, böyle bir anormalliği olan ögenin eski yedeklerine dokunmaz ve iletisinde ögeyi adıyla belirtir. Geri kalan her şey her zamanki gibi budanır.

## Geçmiş içe aktarımı bir deponun okunamadığını söylüyor

Yükseltmeden sonra BombVault, önceki yedeklerin boyutlarını her depodan bir kez okur. O sırada ulaşamadığı bir depo, örneğin kapalı olan bir site dışı hedef ya da bağlanmamış bir paylaşım, **Ayarlar, Doğrulama** altındaki **Anormallikler** kartında sayılır ve günde bir kez yeniden denenir. Ögeleri bu arada yeni yedeklerden öğrenir.

## Disk alanı uyarısı Unraid panosuyla uyuşmuyor

Unraid kullanıcı paylaşımında (`/mnt/user`) boş alan tek bir diskin değil, tüm dizinin boş alanıdır. Uzak depolar yalnızca boş alanını bildiren rclone uzakları üzerinden ölçülür; S3, B2, REST ve SFTP depolarının değeri yoktur ve **Anormallikler** kartında ölçülmemiş olarak listelenir.

## Bir yapay zekâ asistanı bağlanamıyor

[MCP sunucusu](mcp.md#troubleshooting) sayfası, MCP uç noktasının her durum kodunun ve her reddinin ne anlama geldiğini ve ne yapılacağını anlatır.

## Konteyner sürekli yeniden başlıyor ya da sağlıksız görünüyor

BombVault, kendi `/api/health`'inden sağlıklı/sağlıksız bildirir. Motor bir şekilde sıkışırsa bir otomatik onarma aracı (Autoheal gibi) onu otomatik olarak yeniden başlatabilir. Altta yatan neden için konteyner günlüğünü ve **Host Entegrasyonu** kartını denetle.

## Hâlâ takıldın mı?

- Tam [Yapılandırma](configuration.md) ve [Site dışı ve kurtarma](offsite-recovery.md) sayfalarını oku.
- [Unraid destek başlığı](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)'nda sorun.
- Bir [GitHub sorunu](https://github.com/junkerderprovinz/bombvault/issues) aç.
