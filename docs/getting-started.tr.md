# Başlarken

Bu sayfa seni sıfırdan bir Unraid makinesinden ilk yedeğine kadar götürür.

## Gereksinimler {#requirements}

| Gereksinim | Notlar |
|---|---|
| **Unraid 6.12+** | Daha eski sürümler denenmedi. Ana hedef Unraid, ama BombVault düz bir Docker ana makinesinde ve TrueNAS Scale'de de çalışır (bkz. [Genel Docker ana makinesi](#generic-docker-host) ve [TrueNAS SCALE](#truenas)). |
| **Restic deposu konumu** | Yerel bir yol (önerilen: dizin ya da önbellek), SMB, NFS ya da herhangi bir rclone arka ucu. |
| **Docker soketi** | Şablon tarafından kendiliğinden bağlanır (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Şablon tarafından bütün olarak kendiliğinden bağlanır (`/boot` içinde `/host/boot`). Flash yedeğine güç verir ve geri yüklenen bir konteynerin normal, düzenlenebilir bir Unraid uygulaması olarak yeniden görünmesini sağlar. |
| **KVM VM'leri** (isteğe bağlı) | VM yedekleme libvirt ile SSH üzerinden konuşur, libvirt bağlaması yoktur. Bunu **Ayarlar, Entegrasyonlar, Ana makine SSH** altında kur (bkz. [Yapılandırma](configuration.md)). |
| **ZFS veri kümeleri** (isteğe bağlı) | VM yedekleriyle aynı SSH bağlantısı, ana makinede `zfs` ve şablon varsayılanı olan Read/Write - Slave erişim kipiyle `/mnt` olarak eşlenmiş Host Data. Bkz. [ZFS veri kümeleri](zfs-datasets.md). |
| **Android uygulaması** (isteğe bağlı) | Android 10 ya da üstü, 9.7.0 ve üstü sürümdeki sunucularla eşleştirilmiş. Bkz. [Android uygulaması](android.md). |

## Unraid'e kurulum {#install-on-unraid}

En kolay yol **Community Applications**'tır.

1. Unraid'de **Apps** sekmesini aç.
2. **BombVault**'u ara.
3. **Install**'a tıkla, gerekli değişkenleri (aşağıda) ayarla ve uygula.

!!! tip "Şablonu elle kurma"
    Şablonu elle eklemeyi tercih edersen:

    1. **Docker, Add Container, Template repositories**'e git ve şunu ekle:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Şablonlarda **BombVault**'u ara.
    3. Gerekli değişkenleri ayarla ve **Apply**'a tıkla.

## Genel Docker ana makinesi {#generic-docker-host}

Unraid'de değil misin? BombVault herhangi bir Docker ana makinesinde düz bir konteyner olarak da çalışır. TrueNAS SCALE'in kendi adımları [TrueNAS SCALE](#truenas) altındadır.

1. Depodan hazır düzenlenecek [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) dosyasını al.
2. `APP_KEY`'i ayarla (aşağıya bak) ve Host Data birimini gerçek veri köküne yönlendir. Dosyadaki yorumlar ikisini de anlatır.
3. `docker compose up -d` çalıştır, sonra `https://<host-ip>:3443/` adresini aç.

Unraid'den farkı:

- **Flash bölümü yok.** Yedeklenecek ya da geri yüklenecek bir önyükleme USB'si yoktur, bu yüzden Flash bölümünün burada yapacak bir işi yoktur. Bunun yerine Host Data ana makinenin `/etc` klasörüne ulaştığında, Yedekler sayfasındaki **Ekle** bir klasör önerisi olarak **Sistem yapılandırması** sunar: kaydetmeden önce gözden geçirip düzenlediğin bir başlangıç `/etc` klasör kümesi.
- **Unraid'e özgü bildirimler yok.** BombVault'un kendi uygulama içi bildirim kanalları (webhook, site dışı hata uyarıları vb.) normal çalışır. Yalnızca Unraid'in yerel bildirim sistemine gönderme atlanır, çünkü gönderilecek böyle bir sistem yoktur.
- **VM yedekleme isteğe bağlıdır ve SSH ile ulaşılabilen ayrı bir libvirtd ana makinesi gerektirir.** Compose dosyasındaki yorum satırı yapılmış bloğa bak. Genel bir Docker ana makinesinde yerleşik bir VM yöneticisi yoktur.
- **Panoda kutucuk yok.** TollRoll bir Unraid eklentisidir, bu yüzden o adım da atlanır.
- **Bir konteynerin verisini bulmak.** Unraid'in `appdata` geleneği olmadan, konteynerin veri klasörü `DATA_ROOT_SEGMENTS` içindeki parçalardan, Docker adlandırılmış birimlerinden, bir Compose projesinin çalışma dizininden ve `bombvault.data` etiketinden bulunur (bkz. [Yedek kaynağı algılama](configuration.md#backup-source-detection)). Adlandırılmış birimler ve `/etc` ön ayarı yalnızca Host Data bağlamasının içindeki yollara ulaşır, bu yüzden Host Data'yı Docker'ın veri kökünü de kapsayan ortak bir üst klasöre yönlendir.
- **`PLATFORM`.** `generic` ya da `truenas` olarak ayarla. Ayarlanmazsa BombVault Unraid'i flash bağlamasındaki kendi işaretinden algılar ve geri kalan her şeyi genel sayar, yalnızca Unraid'e özgü adımlar denenip başarısız olmak yerine atlanır.

### TrueNAS SCALE {#truenas}

BombVault henüz TrueNAS uygulama kataloğunda değil, bu yüzden özel bir uygulama olarak kur: **Apps, Discover Apps**, sonra **Custom App**'in yanındaki menüde **Install via YAML**. Aşağıdaki YAML, deponun `truenas-apps` klasöründeki katalog girdisiyle aynı şeyleri ayarlar. `tank`'ı kendi havuzunla değiştir ve `APP_KEY`'i sonraki bölümde anlatıldığı gibi doldur.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Sonra `https://<truenas-ip>:30528/` adresini aç. [Unraid'den farkı](#generic-docker-host) altında listelenenler burada da geçerlidir, TrueNAS'ta ayrıca şunlar:

- **Yetenekler (capabilities).** BombVault'un yukarıdaki yedisine ihtiyacı var. `SETPCAP` olmadan VM yedekleri başarısız olur, `SYS_CHROOT` olmadan da her konteyner bir anlık görüntüden okunmak yerine tüm yedekleme boyunca durdurulur.
- **Host Data**, bir veri kümesi üzerindeki ana makine yoludur, burada `/mnt/tank`, uygulamanın içinde aynı yola bağlanır. Bu yüzden `HOST_SOURCE_ROOT` ve `HOST_MOUNT_ROOT` ikisi de onu adlandırır. `rslave`, ana makinenin sonradan yaptığı bağlamaları geçirir, bunlar ZFS veri kümesi yedeklerine gerekir.
- **Bir ixVolume'daki veri yedeklenmez.** Depolaması ixVolume olan bir uygulama verisini Host Data dışında, `/mnt/.ix-apps` altında tutar. BombVault yalnızca uygulamanın tanımını saklar ve çalıştırma dışarıda bıraktığı klasörleri adlandırır. Verisinin yedeklenmesini istediğin her uygulama için Host Data altındaki bir veri kümesinde **Host Path** depolamasını seç.
- **VM ve ZFS veri kümesi yedekleri** TrueNAS'a root olarak SSH ile ulaşır. **System, Services** altında SSH servisini aç ve **Ayarlar, Entegrasyonlar, Ana makine SSH** altındaki açık anahtarı **Credentials, Users** altında root kullanıcısına ekle. VM yedekleri ayrıca yukarıdaki gibi `LIBVIRT_URI` ister, çünkü TrueNAS'ın libvirtd'si kendi soketini dinler. `LIBVIRT_URI` de `LIBVIRT_HOST` da ayarlı değilse BombVault uygulamanın varsayılan ağ geçidini ana makine olarak kullanır, çünkü `host.docker.internal` bir TrueNAS uygulamasında çözümlenmez. Ayrıntılar, zvol üzerindeki VM'ler dahil, [VM yedekleme kılavuzunda](vm-backup-ssh-setup.md#truenas-scale).
- **Bir ZFS replikası farklı bir anahtar kullanır.** İkinci bir ZFS sunucusundaki replika, Host SSH anahtarıyla değil replika penceresinin gösterdiği anahtarla oturum açar. Onu alıcı sunucuya ekle.
- **Silinmiş bir uygulama iki adımda geri gelir.** Önce onu TrueNAS'ta aynı adla yeniden kur, bu ağını yeniden oluşturur, sonra verisini BombVault'ta geri yükle. O zamana kadar geri yükleme denetimi eksik ağı bildirir ve geri yükleme, bir şeyi değiştirmeden önce reddeder.
- **Orijinalin yanına**, düz bir Docker konteyneri oluşturur. Çalışır, ama TrueNAS Apps ekranı onu listelemez.
- **Veritabanı dökümleri** veri kümesi bağlamalarında çalışır. Kendi başına bir veri kümesi olan bir veri klasörüne içe aktarma, eski içeriği yanındaki bir klasöre taşır, bu yüzden yukarıdaki veri kümesinin bunlar için yere ihtiyacı vardır.

### ZimaOS, CasaOS ve diğer ana makineler {#other-hosts}

- **ZimaOS ve CasaOS:** [topluluk mağazasını](https://github.com/junkerderprovinz/zimaos-apps#install) ekle ve BombVault'u ondan kur. Orada `PLATFORM=generic` olarak, Host Data `/DATA` üzerinde çalışır. ZimaOS uygulama verilerini `/DATA/AppData/<app>` içinde tutar, `DATA_ROOT_SEGMENTS` `appdata` ile büyük küçük harf fark etmeksizin eşleştiği için BombVault bunu kendisi bulur.
- **MOS:** BombVault'u [MOS Hub](https://mos-official.net/apps.html?search=BombVault)'dan kur. Orada `PLATFORM=generic` olarak, Host Data `/mnt/cache` üzerinde çalışır.
- **Synology:** Container Manager'da compose dosyasından bir proje oluştur (DSM 7.2 ya da daha yeni).
- **Proxmox:** Docker olan bir VM'de ya da LXC konteynerinde çalıştır.

Yorumlar olmadan genel compose dosyası kısadır. `./data`'yı konteynerlerinin verisini tutan ana makine klasörüyle değiştir, `HOST_SOURCE_ROOT`'u aynı yola ayarla ve `APP_KEY`'i doldur:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Docker adlandırılmış birimlerini de yedeklemek için, Unraid şablonunun `/mnt`'nin tamamını bağlaması gibi, hem verini hem de Docker'ın veri kökünü (`docker info -f '{{.DockerRootDir}}'`, genellikle `/var/lib/docker`) tutan bir klasörü bağla.

## Gerekli tek ayar {#the-one-required-setting}

Ayarlaman gereken tek değişken `APP_KEY`'dir, restic deposu parolasını türetmek için kullanılan 32 baytlık onaltılık bir gizli bilgi (64 onaltılık karakter).

Herhangi bir makinede bir tane üret:

```bash
openssl rand -hex 32
```

Sonucu şablonun `APP_KEY` alanına (Unraid) ya da `docker-compose.yml` içindeki `APP_KEY` ortam değişkenine (genel Docker ana makinesi) yapıştır.

!!! danger "APP_KEY'ini kaybetme"
    `APP_KEY`'i kaybetmek şifreli yedeklerini kurtarılamaz yapar. Onu sunucudan ayrı güvenli bir yerde sakla. BombVault çalışmaya başlayınca bir giriş parolası ayarla ve **Ayarlar, Geri yükleme** altından **kurtarma kitini** indir (bkz. [Site dışı ve kurtarma](offsite-recovery.md#recovery-kit)). İçinde anahtar, her hedef ve restic komutları bulunur.

Şablon ayrıca Docker soketini, flash'ı (`/boot`) ve **Host Data** kökünü (`/mnt`) senin için bağlar. Yedek *kaynakları* ve *hedefleri* ikisi de Host Data altında durur. Değişkenlerin tam başvurusu ve site dışı kurulum için bkz. [Yapılandırma](configuration.md).

## İlk çalıştırma {#first-run}

![Yeni bir kurulumun karşılama sayfası, BombVault'un sunucuda bulduklarıyla.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![Yeni bir kurulumun karşılama sayfası, BombVault'un sunucuda bulduklarıyla.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Web arayüzünü `https://<your-unraid-ip>:3443` adresinde aç (kutudan çıktığı gibi kendi kendine imzalı sertifika). Yeni bir kurulum, BombVault'un sunucuda bulduklarını listeleyen **BombVault'a hoş geldin** ile açılır.
2. **Başlayalım**'ı seç. Genel bakış sonra üç adımlı **İlk adımlar** kartını gösterir:
    1. **Bölümleri seç**: BombVault'un neyi yedekleyeceğini aç (Konteynerler, VM'ler, Flash, Klasörler, ZFS veri kümeleri, Öz yedek). **Seç**, bir bölümün ögelerinin listesini açar, orada **Zamanlamayı duraklat** birini dışarıda bırakır. Burada açtığın bölümler her gece çalışır, bunu **Ayarlar, Zamanlama** altında değiştirirsin.
    2. **Hedef ekle**: yedekler için ikinci bir yer. Her bölümün bu sunucuda, ilk yedekte oluşturulan `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` altında zaten bir klasörü var. En güvenlisi, Storage Box ya da Backblaze B2 gibi tesis dışında ikinci bir hedef.
    3. **İlk yedekleme**: ondan sonra her gece kendiliğinden çalışır. Kart, ilk yedekleme bitene kadar durur.
3. Tüm ögeleri tek bir listede görmek için **Yedekler**'i aç. **Şimdi tümünü yedekle**, açık olan tüm bölümler üzerinde bir geçiş başlatır, bir ögenin sayfasındaki **Şimdi yedekle** ise yalnızca onu yedekler.

![Üç adımlı İlk adımlar kartı, ikincisi açık.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![Üç adımlı İlk adımlar kartı, ikincisi açık.](assets/screenshots/getting-started/first-steps-light.png#only-light)

BombVault'u daha önce kullandın mı? Karşılama sayfasındaki **Yedekten geri yükle**, kayıp bir sunucunun ayarlarını, konteynerlerini, VM'lerini ve klasörlerini **Acil durum sihirbazı**'nın geri getirdiği Geri yükleme sayfasını açar. Bkz. [Site dışı ve kurtarma](offsite-recovery.md#emergency).

!!! tip "İsteğe bağlı: bir yedekleme sırası seç"
    Bazı konteynerler her zaman diğerlerinden önce yedeklenmeliyse (örneğin bir veritabanı, onu kullanan uygulamadan önce), **Ayarlar, Zamanlama** altındaki **Sıra** kartını aç ve onları istediğin sıraya sürükle. Zamanlanmış çalıştırmalar ve tam yedekleme bundan sonra onu izler. Sıralamadığın her şey en çok gecikmiş olandan başlayarak yedeklenir.

## Örnek: ilk konteyner yedeğin, baştan sona {#example-first-backup}

Bu örnek, aralarında `plex`'in de olduğu birkaç konteynerli yeni bir Unraid makinesinde ilk yedek bitip nerede bulacağını bilene kadar yürür. Yaklaşık on dakika ve ilk yedeğin süresi kadar sürer.

1. **Kur.** Unraid **Apps** sekmesinde **BombVault**'u ara ve **Install**'a bas.
2. **Anahtarı ayarla.** Bir bilgisayarda `openssl rand -hex 32` çalıştır ve 64 karakteri şablonun `APP_KEY` alanına yapıştır. Başka her şeyden önce, şimdi, bir kopyasını parola yöneticine koy. **Apply**'a bas.
3. **Aç.** `https://<your-unraid-ip>:3443` adresine git ve kendi kendine imzalı sertifikayı kabul et. **BombVault'a hoş geldin**, bulduklarını listeler, örneğin 12 konteyner, 2 VM ve flash sürücü.
4. **Başlayalım.** Genel bakış **İlk adımlar** kartıyla açılır.
5. **Bölümleri seç.** **Konteynerler** baştan açıktır. İstediğin diğerlerini aç, örneğin **Flash**. **Seç**, bulduğu konteynerlerin listesini açar, orada **Zamanlamayı duraklat** birini, örneğin umursamadığın bir test konteynerini, dışarıda bırakır. Açık bölümler her gece tam yedeklemede çalışır.
6. **Hedef ekle.** Her bölümün bu sunucuda `/mnt/user/bombvault` altında kendi klasörü zaten var, yani ilk yedeğin gidecek bir yeri var. Bu adım ikinci bir yer ekler ve en güvenlisi tesis dışında olandır. Bunu şimdi [S3 örneğiyle](offsite-recovery.md#example-s3) yapabilir ya da ilk yedek bittikten sonra dönebilirsin. Kart o zamana kadar durur.
7. **İlk yedekleme.** **Yedekler**'i aç ve **Şimdi tümünü yedekle**'ye bas ya da geceyi bekle. Durum başlığındaki halka bir ilerleme halkasına döner ve ögeleri sayar, örneğin **Şu anda yedekleniyor: 12'den 3'ü**. Her konteyner verisi okunurken durdurulur ve hemen sonra yeniden başlatılır. Verisi ZFS üzerinde olan bir konteyner yalnızca bir anlık görüntünün birkaç saniyesi kadar kapalı kalır. İlk yedek her şeyi okur, bu yüzden en uzun sürendir. Sonrakiler yalnızca değişeni saklar.
8. **Kontrol et.** Halka **Her şey yedeklendi** dediğinde genel bakışta **Geçmiş**'i aç: her konteynerin **Çalıştırmalar**'da yeşil bir satırı vardır. **Yedekler**'i aç, `plex`'e tıkla ve **Yedekler** sekmesine bak: ilk anlık görüntü orada, onu tutan yer **Yerel** olarak. Birkaç dakika sonra **Doğrulama** sekmesi, yedekten bir örneği geri yükleyip geri okuyan geri yükleme denetimini gösterir.
9. **Anahtarı koru.** **Ayarlar, Güvenlik**'i aç ve bir giriş parolası ayarla. Sonra **Ayarlar, Geri yükleme**'yi aç ve **Acil durumlar için** içinde **Kurtarma kitini indir**'e bas. Dosyayı sunucudan uzakta, 2. adımdaki `APP_KEY` kopyasının yanında sakla. Genel bakıştaki **Kurtarma kitini sakla** hatırlatması **Güvenle sakladım**'a basınca kaybolur.

Bundan sonra konteynerler her gece yedeklenir. Sırasıyla iyi sonraki adımlar: ölü bir sunucu yedekleri de alıp götürmesin diye bir [site dışı hedef](offsite-recovery.md#example-s3), sonra bir hata sana ulaşsın diye [bildirimler](notifications.md), ve 03:00 sana uymuyorsa [Zamanlamalar](schedules.md)'a bir bakış.

![İlk yedek çalışırken durum başlığı, ilerleme halkası ve istasyonlarıyla.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![İlk yedek çalışırken durum başlığı, ilerleme halkası ve istasyonlarıyla.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Host entegrasyon denetimi"
    Genel bakıştaki **Host Entegrasyonu** kartı her bağlamayı ve CLI'yi (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir, böylece konteynerin doğru bağlandığını güvenmeden önce doğrulayabilirsin. Sen ekleyene kadar **Özelleştir** altında bekler.

## Yolunu bulmak {#navigation}

![Kutucuklar olarak Ayarlar, yanlarında seçilen kutucuğun kartları. Her değişiklik yapıldığı anda yazılır.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Kutucuklar olarak Ayarlar, yanlarında seçilen kutucuğun kartları. Her değişiklik yapıldığı anda yazılır.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Kenar çubuğunda beş giriş var: **Genel bakış**, **Anormallikler** (anormallik algılama açıkken), **Yedekler**, **Geri yükleme** ve **Oturumu kapat**'ın altında **Ayarlar**. Bir sayfanın ana eylemi sağ altta yüzer, **Diğer** nadir olanları barındırır. Telefonda çubuk Genel bakış, Yedekler, Geri yükleme ve Ayarlar'ı tutar, **Diğer**'de geri kalanlar vardır.

Sayfalar ve ayarlar pencerenin tüm genişliğini kullanır. Bir pencere düğmelerini alt kısmında tutar, ileriye götüren sağdadır. Telefonda üçten fazla düğmesi olan bir alt kısım çıkışı ve ana eylemi tutar, diğerlerini **Diğer** altına koyar.

![Telefonda, Diğer açılmış alt çubuk.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Telefonda, Diğer açılmış alt çubuk.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Genel bakış**, her şeyin yedeklenip yedeklenmediğini ve neyin sana ihtiyaç duyduğunu gösterir. Bkz. [Genel bakış](overview.md).
- **Yedekler**, her konteyneri, VM'i, klasör kümesini, flash sürücüyü, her ZFS veri kümesini ve öz yedeği öge olarak listeler. Listenin üstündeki kutucuklar bir tür seçer, **Filtreler**, **Sırala** ve **Ara** daraltır. Her ögenin üç sekmeli kendi sayfası vardır: **Yedekler**, **Ayarlar** ve **Doğrulama**. Ögenin simgesinin solundaki bir geri düğmesi listeye götürür. Bkz. [Yedekler ve ögeler](backups.md).
- **Geri yükleme**, her şey için tek sayfadır: bir bölüm seç, bir öge ara ve bir yedek seç. Acil durum sihirbazı ve diğer kaynaklar da buradan başlar. Bkz. [Geri yükleme](restore.md).
- **Ayarlar** on dört kutucuktur: Genel, Görünüm, Zamanlama, Hedefler, Geri yükleme, Bildirimler, Doğrulama, Güvenlik, Örnekler, Eşleştirme, Entegrasyonlar, Uygulamalar, Sistem ve Bilgi. Bkz. [Ayarlar](settings.md). Hepsinde arama yapmak için `/` ya da Ctrl+K'ya bas, sayfaların önceki sürümlerdeki adlarıyla da. Kutucukları **Görünüm** altında kendi sırana sürükle. Sayfanın yanındaki bir kutucuk sütununun onu sıkıştıracağı yerde, örneğin yaklaşık 1200 piksel genişliğinde bir pencerede, kutucuklar sayfanın üstünde yana kayan bir şeride dönüşür. Telefon aynı şeridi gösterir.
- **Hedefler** ve **Örnekler**, kendi sayfaları olan ayar kutucuklarıdır. Kutucuklarındaki **Kenar çubuğunda da göster** onları kenar çubuğuna sabitler.

Hiçbir şeyde Kaydet düğmesi ve katlanan bölüm yoktur: nadir şeylerin kendi kartı ya da penceresi vardır, açıklamalar (i) balonlarındadır. Bir güncellemeden sonra son sürümünden bu yana her sürüm için bir kartlı bir pencere kendiliğinden açılır ve yeşil noktalar neyin değiştiğini işaretler. Bir tıklama noktayı gizler. **Ayarlar, Bilgi** altındaki Sürüm kartındaki **Yeni**, henüz görmediklerini sayar ve her sürümün notları ve **Oraya git** ile değişiklikleriyle o pencereyi yeniden açar.

## Kaynaktan derlemek {#build-from-source}

BombVault, bir JSON API'si ve gömülü bir React arayüzü sunan tek bir statik Go ikilisidir. Önce arayüzü derle, sonra ikiliyi çalıştır:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Arayüz derlemesi `go run` için de gerekir. Depo `web/dist` altında yalnızca boş bir işaret izler, bu yüzden `npm --prefix web run build` olmadan ikili hiçbir şey gömmez ve `500 SPA index not found` yanıtlar, bu beklenen bir şeydir. Docker, libvirt ve Unraid CI'da denenemez, bu yüzden bir pull request açmadan önce bağlamaları, restic'i ve VM SSH bağlantısını gerçek bir ana makinede genel bakışın **Host Entegrasyonu** kartıyla (ya da `GET /api/spike`) kontrol et.

## Sonraki adımlar {#next-steps}

- Tesis dışında bir ya da daha fazla hedef ekle (her bölüm aynı anda birçoğuna kopyalayabilir) ve kurtarma kitini **[Site dışı ve kurtarma](offsite-recovery.md)** altında anlatıldığı gibi sakla. [S3 örneği](offsite-recovery.md#example-s3) birini adım adım gösterir.
- Bir hatanın sana ulaştığından **[Bildirimler](notifications.md)** ile emin ol.
- İşlerin ne zaman çalıştığına **[Zamanlamalar](schedules.md)** altında bak.
- Grubundaki her sunucuyu **[Android uygulaması](android.md)** ile telefonuna taşı.
- **[Adım adım örnekler](guides.md)** yaygın işleri anlatır, **[Özellikler](features.md)** ise BombVault'un yaptığı her şeyi listeler.
- Bir kurulumu klonluyor ya da yeni bir makineye mi taşınıyorsun? Tüm yapılandırmanı **Ayarları dışa / içe aktar** kartıyla taşı. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).
- Bir sorunla mı karşılaştın? Bkz. **[Sorun Giderme](troubleshooting.md)**.
