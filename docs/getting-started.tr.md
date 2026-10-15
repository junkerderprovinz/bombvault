# Başlarken

Bu sayfa seni sıfırdan bir Unraid makinesinden ilk yedeğine kadar götürür.

## Gereksinimler

| Gereksinim | Notlar |
|---|---|
| **Unraid 6.12+** | Daha eski sürümler test edilmemiştir. Asıl hedef Unraid'dir, ama BombVault sade bir Docker ana makinesinde ve TrueNAS Scale üzerinde de çalışır (bkz. [Genel Docker ana makinesi](#generic-docker-host)). |
| **Restic depo konumu** | Yerel bir yol (önerilen: dizin ya da önbelleğin), SMB, NFS veya herhangi bir rclone arka ucu. |
| **Docker soketi** | Şablon tarafından otomatik olarak bağlanır (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Şablon tarafından tümüyle otomatik bağlanır (`/boot` -> `/host/boot`). Flash yedeklemesini besler ve geri yüklenen bir konteynerin normal, düzenlenebilir bir Unraid uygulaması olarak yeniden görünmesini sağlar. |
| **KVM VM'leri** (isteğe bağlı) | VM yedeklemesi libvirt ile SSH üzerinden konuşur, libvirt bağlaması yok. **Ayarlar, Entegrasyonlar, Ana makine SSH** altında kur (bkz. [Yapılandırma](configuration.md)). |
| **ZFS veri kümeleri** (isteğe bağlı) | VM yedeklemeleriyle aynı SSH bağlantısı, host'ta `zfs` ve erişim modu Read/Write - Slave (şablonun varsayılanı) olan `/mnt` olarak eşlenmiş Host Data. Bkz. [ZFS veri kümeleri](zfs-datasets.md). |
| **Android uygulaması** (isteğe bağlı) | Android 10 veya üstü, 9.7.0 veya daha yeni sürümdeki sunucularla eşleştirilmiş. Bkz. [Android uygulaması](android.md). |

## Unraid'e kurulum

En kolay yol **Community Applications**'tır.

1. Unraid'de **Apps** sekmesini aç.
2. **BombVault** araması yap.
3. **Install**'a tıkla, gerekli değişkenleri (aşağıda) ayarla ve uygula.

!!! tip "Şablonu elle kurma"
    Şablonu elle eklemeyi tercih edersen:

    1. **Docker, Add Container, Template repositories** bölümüne git ve şunu ekle:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Templates içinde **BombVault** araması yap.
    3. Gerekli değişkenleri ayarla ve **Apply**'a tıkla.

## Genel Docker ana makinesi {#generic-docker-host}

Unraid değil mi? BombVault herhangi bir Docker ana makinesinde sıradan bir kapsayıcı olarak da çalışır (TrueNAS Scale üzerindeki kapsayıcı desteği de, oradaki uygulama kataloğunda kendi kaydı olana dek buna dayanır).

1. Depodan, düzenlemeye hazır [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) dosyasını al.
2. `APP_KEY` değerini ayarla (aşağıya bak) ve Host Data birimini gerçek veri köküne yönelt. Dosyadaki yorumlar ikisini de adım adım anlatıyor.
3. `docker compose up -d`, ardından `https://<ana-makine-ip>:3443/` adresini aç.

Unraid'e göre neler değişiyor:

- **Flash bölümü yok.** Yakalanacak ya da geri yüklenecek bir açılış USB'si bulunmadığından, Flash bölümünün burada yapacağı bir şey yok. Onun yerine Yedekler sayfasındaki **Ekle**, bir klasör önerisi olarak **Sistem yapılandırması**'nı sunar: kaydetmeden önce gözden geçirip düzenlediğin, `/etc` ile başlayan bir klasör kümesi.
- **Unraid'e özgü bildirimler yok.** BombVault'un kendi bildirim kanalları (webhook, saha dışı başarısızlık uyarıları ve benzeri) her zamanki gibi çalışır; yalnızca Unraid'in kendi bildirim sistemine gönderim atlanır, çünkü burada öyle bir sistem yoktur.
- **VM yedeklemesi isteğe bağlıdır ve SSH ile erişilebilen ayrı bir libvirtd ana makinesi gerektirir.** compose dosyasındaki yorum satırına alınmış bloğa bak. Genel bir Docker ana makinesinin kendisinde VM yönetimi yoktur.
- **Dashboard widget'ı yok.** BombVault Widget bir Unraid eklentisidir, bu yüzden bu adım da atlanır.
- **Bir konteynerin verisini bulma.** Unraid'in `appdata` geleneği olmadan bir konteynerin veri klasörü `DATA_ROOT_SEGMENTS` içindeki parçalardan, Docker adlandırılmış birimlerinden, bir Compose projesinin çalışma dizininden ve `bombvault.data` etiketinden bulunur (bkz. [Yedekleme kaynaklarının belirlenmesi](configuration.md#backup-source-detection)). Adlandırılmış birimler ve `/etc` hazır ayarı yalnızca Host Data bağlamasının içindeki yollara ulaşır, bu yüzden Host Data'yı Docker'ın veri kökünü de kapsayan ortak bir üst dizine yönelt.
- **`PLATFORM`.** `generic` ya da `truenas` olarak ayarla. Ayarlanmazsa BombVault, Unraid'i flash bağlamasındaki kendi işaretinden tanır ve geri kalan her şeyi genel kabul eder; Unraid'e özgü adımlar denenip başarısız olmak yerine atlanır.

**TrueNAS Scale** aynı compose yolunu izler; bir katalog girdisi depoda hazırlanmıştır ama henüz gönderilmemiştir. Orada VM yedeklemesi `LIBVIRT_URI` gerektirir, çünkü TrueNAS'ın libvirtd'si üç `LIBVIRT_*` değişkeninin ifade edemediği kendine ait bir sokette (`/run/truenas_libvirt/libvirt-sock`) dinler (bkz. [Yapılandırma](configuration.md)). Bunun ne kadar kanıtlandığı: zvol yedeklemesi gerçek bir TrueNAS Scale makinesinde, çalışan bir VM'e bağlı bir zvol üzerinde çalıştırıldı ve `zfs snapshot`, `zfs send`, restic ve `zfs receive` onu bayt bayt gidiş dönüş aktardı. BombVault'un kendisinin yürüttüğü eksiksiz bir geri yükleme henüz TrueNAS donanımında çalıştırılmadı ve o zvol seyrekti (sparse), bu yüzden çok gigabaytlık verilerde aktarım hızı test edilmedi. Ona güvenmeden önce orada bir geri yüklemeyi dene.

## Gereken tek ayar

Ayarlaman gereken tek değişken, restic depo parolasını türetmek için kullanılan 32 baytlık bir onaltılık gizli anahtar olan (64 onaltılık karakter) `APP_KEY`'dir.

Herhangi bir makinede bir tane oluştur:

```bash
openssl rand -hex 32
```

Sonucu şablonun `APP_KEY` alanına (Unraid) veya `docker-compose.yml` içindeki `APP_KEY` ortam değişkenine (genel Docker ana makinesi) yapıştır.

!!! danger "APP_KEY'ini kaybetme"
    `APP_KEY`'i kaybetmek, şifreli yedeklerini kurtarılamaz hale getirir. Onu güvenli ve sunucudan ayrı bir yerde sakla. BombVault çalışmaya başladıktan sonra bir giriş parolası belirle ve **kurtarma kitini** **Ayarlar, Geri Yükleme** altından indir (bkz. [Site dışı ve kurtarma](offsite-recovery.md#recovery-kit)). Kitte anahtar, her hedef ve restic komutları bulunur.

Şablon ayrıca Docker soketini, flash'ı (`/boot`) ve **Host Data** kökünü (`/mnt`) senin için bağlar. Yedekleme *kaynakları* ve *hedefleri* her ikisi de Host Data altında yer alır. Tam değişken referansı ve site dışı kurulum için bkz. [Yapılandırma](configuration.md).

## İlk çalıştırma

![İlk yedekten sonraki genel bakış: ilgi bekleyenler, her bölümün koruması, hedefler ve geçmiş.](assets/screenshots/dashboard.png)

*İlk yedekten sonraki genel bakış: ilgi bekleyenler, her bölümün koruması, hedefler ve geçmiş.*

1. Web arayüzünü `https://<your-unraid-ip>:3443` adresinde aç (kutudan çıktığı gibi kendinden imzalı sertifika). Yeni bir kurulum, BombVault'un sunucuda bulduklarını listeleyen **BombVault'a hoş geldin** sayfasıyla açılır.
2. **Başlayalım**'ı seç. Genel bakış sayfası bundan sonra üç adımlı **İlk adımlar** kartını gösterir:
    1. **Bölümleri seç**: BombVault'un neyi yedekleyeceğini aç (Konteynerler, VM'ler, Flash, Klasörler, ZFS veri kümeleri, Öz yedek). **Seç**, bir bölümün ögelerinin listesini açar; orada **Zamanlamayı duraklat** biri dışarıda bırakır. Burada açtığın bölümler her gece çalışır, bunu **Ayarlar, Zamanlama** altında değiştirirsin.
    2. **Hedef ekle**: yedeklerin gideceği yer. Başlamak için yerel bir klasör yeter. En güvenlisi, bir Storage Box ya da Backblaze B2 gibi, bina dışında ikinci bir hedeftir. Yerel depolar varsayılan olarak `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` yolundadır ve ilk yedeklemede oluşturulur.
    3. **İlk yedekleme**: bundan sonra her gece kendiliğinden çalışır.
3. Her ögeyi tek listede görmek için **Yedekler**'i aç. **Şimdi tümünü yedekle**, açık olan tüm bölümler üzerinde bir çalışma başlatır; bir ögenin sayfasındaki **Şimdi yedekle** ise yalnızca onu yedekler.

BombVault'u daha önce kullandın mı? Karşılama sayfasındaki **Yedekten geri yükle**, kaybolmuş bir sunucunun ayarlarını, konteynerlerini, VM'lerini ve klasörlerini **Acil durum sihirbazı**'nın geri getirdiği Geri yükleme sayfasını açar. Bkz. [Site dışı ve kurtarma](offsite-recovery.md#emergency).

!!! tip "İsteğe bağlı: bir yedekleme sırası seç"
    Bazı konteynerler her zaman diğerlerinden önce yedeklenecekse (örneğin bir veritabanı, onu kullanan uygulamadan önce), **Ayarlar, Zamanlama** altındaki **Sıra** kartını aç ve onları istediğin sıraya sürükle. Zamanlanmış çalışmalar ve tam yedekleme bu sırayı izler; sıralamadan bıraktığın her şey, en çok gecikmiş olan önce yedeklenir.

!!! note "Host entegrasyon denetimi"
    Genel bakıştaki **Host Entegrasyonu** kartı her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir; böylece ona güvenmeden önce konteynerin doğru bağlandığını onaylayabilirsin. Sen ekleyene kadar **Özelleştir** altında bekler.

## Yolunu bulmak {#navigation}

![Kutucuklar olarak ayarlar, yanlarında seçilen kutucuğun kartları. Her değişiklik yaptığın anda yazılır.](assets/screenshots/settings.png)

*Kutucuklar olarak ayarlar, yanlarında seçilen kutucuğun kartları. Her değişiklik yaptığın anda yazılır.*

Kenar çubuğunda beş giriş var: **Genel bakış**, **Anormallikler** (anormallik algılama açıkken), **Yedekler**, **Geri yükleme** ve **Oturumu kapat**'ın altında **Ayarlar**. Bir sayfanın ana eylemi sağ altta durur, **Diğer** ise seyrek olanları tutar. Telefonda çubukta Genel bakış, Yedekler, Geri yükleme ve Ayarlar bulunur, gerisi **Diğer**'dedir.

Sayfalar ve ayarlar pencerenin tüm genişliğini kullanır. Bir pencere düğmelerini alt kısmında tutar; ileri giden düğme sağdadır. Telefonda üçten fazla düğmesi olan bir alt kısım çıkış yolunu ve ana eylemi tutar, diğerlerini **Diğer** altına koyar.

- **Yedekler**, her konteyneri, VM'yi, klasör kümesini, flash sürücüyü, her ZFS veri kümesini ve öz yedeği öge olarak listeler. Listenin üstündeki kutucuklar bir tür seçer; **Filtreler**, **Sırala** ve **Ara** daraltır. Her ögenin kendi sayfası ve üç sekmesi vardır: **Yedekler**, **Ayarlar** ve **Doğrulama**. Ögenin simgesinin solundaki bir geri düğmesi listeye götürür.
- **Geri yükleme** her şey için tek sayfadır: bir bölüm seç, bir öge ara ve bir yedek seç. Acil durum sihirbazı ve diğer kaynaklar da buradan başlar.
- **Ayarlar** on dört kutucuktur: Genel, Görünüm, Zamanlama, Hedefler, Geri Yükleme, Bildirimler, Doğrulama, Güvenlik, Örnekler, Eşleştirme, Entegrasyonlar, Uygulamalar, Sistem ve Bilgi. Hepsinde aramak için `/` ya da Ctrl+K'ye bas; sayfaların önceki sürümlerdeki adlarıyla da bulunur. Kutucukları **Görünüm** altında kendi sıranda sürükle.
- **Hedefler** ve **Örnekler**, kendi sayfaları olan ayar kutucuklarıdır. Kutucuklarındaki **Kenar çubuğunda da göster** onları kenar çubuğuna sabitler.

Hiçbir şeyin Kaydet düğmesi yoktur ve hiçbir şey açılıp katlanmaz: seyrek şeylerin kendi kartı ya da penceresi vardır, açıklamalar da (i) baloncuklarında durur. Bir güncellemeden sonra yeşil noktalar nelerin değiştiğini gösterir. Bir tıklama noktayı gizler; **Ayarlar, Bilgi** altındaki Sürüm kartında **Yenilikler** her değişikliği **Oraya git** ile listeler.

## Kaynaktan derleme {#build-from-source}

BombVault, bir JSON API'si ve gömülü bir React arayüzü sunan tek bir statik Go ikili dosyasıdır. Önce arayüzü derle, sonra ikili dosyayı çalıştır:

```bash
npm --prefix web ci
npm --prefix web run build     # ikili dosyanın gömdüğü web/dist'i yazar
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # gerçek bir restic gidiş dönüşüyle birim ve entegrasyon testleri
golangci-lint run ./...
go run ./cmd/bombvault         # kendinden imzalı bir sertifikayla https://localhost:3443 sunar
```

Arayüz derlemesi `go run` için de gereklidir. Depo `web/dist` altında yalnızca boş bir işaret dosyasını izler, bu yüzden `npm --prefix web run build` olmadan ikili dosya hiçbir şey gömmez ve `500 SPA index not found` yanıtını verir; bu beklenen bir durumdur. Docker, libvirt ve Unraid CI'da test edilemez, bu yüzden bir pull request açmadan önce bağlamaları, restic'i ve VM SSH bağlantısını gerçek bir host'ta genel bakışın **Host Entegrasyonu** kartıyla (ya da `GET /api/spike` ile) kontrol et.

## Sonraki adımlar

- Tüm **[Özellikler](features.md)**'e göz at.
- **[Android uygulaması](android.md)** ile grubundaki her sunucuyu telefonuna getir.
- Bina dışında bir ya da daha fazla hedef ekle (her bölüm aynı anda birkaçına kopyalayabilir) ve kurtarma kitini kaydet; bunlar **[Site dışı ve kurtarma](offsite-recovery.md)** altında anlatılıyor.

- Bir kurulumu klonluyor ya da yeni bir makineye mi geçiyorsun? Tüm yapılandırmanı **Ayarları dışa / içe aktar** kartıyla taşı. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).
- Bir sorunla mı karşılaştın? Bkz. **[Sorun giderme](troubleshooting.md)**.
