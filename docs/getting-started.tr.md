# Başlarken

Bu sayfa sizi sıfırdan bir Unraid makinesinden ilk yedeğinize kadar götürür.

## Gereksinimler

| Gereksinim | Notlar |
|---|---|
| **Unraid 6.12+** | Daha eski sürümler test edilmemiştir. Asıl hedef Unraid'dir, ama BombVault sade bir Docker ana makinesinde ve TrueNAS Scale üzerinde de çalışır (bkz. [Genel Docker ana makinesi](#generic-docker-host)). |
| **Restic depo konumu** | Yerel bir yol (önerilen: diziniz ya da önbelleğiniz), SMB, NFS veya herhangi bir rclone arka ucu. |
| **Docker soketi** | Şablon tarafından otomatik olarak bağlanır (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Şablon tarafından tümüyle otomatik bağlanır (`/boot` -> `/host/boot`). Flash yedeklemesini besler ve geri yüklenen bir konteynerin normal, düzenlenebilir bir Unraid uygulaması olarak yeniden görünmesini sağlar. |
| **KVM VM'leri** (isteğe bağlı) | VM yedeklemesi libvirt ile SSH üzerinden konuşur, libvirt bağlaması yok. Ayarlar'da kurun (bkz. [Yapılandırma](configuration.md)). |
| **ZFS veri kümeleri** (isteğe bağlı) | VM yedeklemeleriyle aynı SSH bağlantısı, host'ta `zfs` ve erişim modu Read/Write - Slave (şablonun varsayılanı) olan `/mnt` olarak eşlenmiş Host Data. Bkz. [ZFS veri kümeleri](zfs-datasets.md). |
| **Android uygulaması** (isteğe bağlı) | Android 10 veya üstü, 9.7.0 veya daha yeni sürümdeki sunucularla eşleştirilmiş. Bkz. [Android uygulaması](android.md). |

## Unraid'e kurulum

En kolay yol **Community Applications**'tır.

1. Unraid'de **Apps** sekmesini açın.
2. **BombVault** araması yapın.
3. **Install**'a tıklayın, gerekli değişkenleri (aşağıda) ayarlayın ve uygulayın.

!!! tip "Şablonu elle kurma"
    Şablonu elle eklemeyi tercih ederseniz:

    1. **Docker, Add Container, Template repositories** bölümüne gidin ve şunu ekleyin:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Templates içinde **BombVault** araması yapın.
    3. Gerekli değişkenleri ayarlayın ve **Apply**'a tıklayın.

## Genel Docker ana makinesi {#generic-docker-host}

Unraid değil mi? BombVault herhangi bir Docker ana makinesinde sıradan bir kapsayıcı olarak da çalışır (TrueNAS Scale üzerindeki kapsayıcı desteği de, oradaki uygulama kataloğunda kendi kaydı olana dek buna dayanır).

1. Depodan, düzenlemeye hazır [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) dosyasını alın.
2. `APP_KEY` değerini ayarlayın (aşağıya bakın) ve Host Data birimini gerçek veri kökünüze yöneltin: dosyadaki yorumlar ikisini de adım adım anlatıyor.
3. `docker compose up -d`, ardından `https://<ana-makine-ip>:3443/` adresini açın.

Unraid'e göre neler değişiyor:

- **flash/USB alanı yok.** Yakalanacak ya da geri yüklenecek bir açılış USB'si bulunmadığından, ayarlardaki Flash alanının burada yapacağı bir şey yok. Onun yerine Klasörler alanı, pratik bir genel karşılık olarak tek tıklamalık **Hazır ayar ekle: ana makine sistem yapılandırması** önerisini sunuyor (kaydetmeden önce gözden geçirip düzenlediğiniz bir başlangıç `/etc` dosya kümesi).
- **Unraid'e özgü bildirimler yok.** BombVault'un kendi bildirim kanalları (webhook, saha dışı başarısızlık uyarıları ve benzeri) her zamanki gibi çalışır; yalnızca Unraid'in kendi bildirim sistemine gönderim atlanır, çünkü burada öyle bir sistem yoktur.
- **Sanal makine yedeklemesi isteğe bağlıdır ve SSH ile erişilebilen ayrı bir libvirtd ana makinesi gerektirir.** compose dosyasındaki yorum satırına alınmış bloğa bakın. Genel bir Docker ana makinesinin kendisinde sanal makine yönetimi yoktur.
- **Dashboard widget'ı yok.** BombVault Widget bir Unraid eklentisidir, bu yüzden bu adım da atlanır.
- **Bir konteynerin verisini bulma.** Unraid'in `appdata` geleneği olmadan bir konteynerin veri klasörü `DATA_ROOT_SEGMENTS` içindeki parçalardan, Docker adlandırılmış birimlerinden, bir Compose projesinin çalışma dizininden ve `bombvault.data` etiketinden bulunur (bkz. [Yedekleme kaynaklarının belirlenmesi](configuration.md#backup-source-detection)). Adlandırılmış birimler ve `/etc` hazır ayarı yalnızca Host Data bağlamasının içindeki yollara ulaşır, bu yüzden Host Data'yı Docker'ın veri kökünü de kapsayan ortak bir üst dizine yöneltin.
- **`PLATFORM`.** `generic` ya da `truenas` olarak ayarlayın. Ayarlanmazsa BombVault, Unraid'i flash bağlamasındaki kendi işaretinden tanır ve geri kalan her şeyi genel kabul eder; Unraid'e özgü adımlar denenip başarısız olmak yerine atlanır.

**TrueNAS Scale** aynı compose yolunu izler; bir katalog girdisi depoda hazırlanmıştır ama henüz gönderilmemiştir. Orada VM yedeklemesi `LIBVIRT_URI` gerektirir, çünkü TrueNAS'ın libvirtd'si üç `LIBVIRT_*` değişkeninin ifade edemediği kendine ait bir sokette (`/run/truenas_libvirt/libvirt-sock`) dinler (bkz. [Yapılandırma](configuration.md)). Bunun ne kadar kanıtlandığı: zvol yedeklemesi gerçek bir TrueNAS Scale makinesinde, çalışan bir VM'e bağlı bir zvol üzerinde çalıştırıldı ve `zfs snapshot`, `zfs send`, restic ve `zfs receive` onu bayt bayt gidiş dönüş aktardı. BombVault'un kendisinin yürüttüğü eksiksiz bir geri yükleme henüz TrueNAS donanımında çalıştırılmadı ve o zvol seyrekti (sparse), bu yüzden çok gigabaytlık verilerde aktarım hızı test edilmedi. Ona güvenmeden önce orada bir geri yüklemeyi deneyin.

## Gereken tek ayar

Ayarlamanız gereken tek değişken, restic depo parolasını türetmek için kullanılan 32 baytlık bir onaltılık gizli anahtar olan (64 onaltılık karakter) `APP_KEY`'dir.

Herhangi bir makinede bir tane oluşturun:

```bash
openssl rand -hex 32
```

Sonucu şablonun `APP_KEY` alanına (Unraid) veya `docker-compose.yml` içindeki `APP_KEY` ortam değişkenine (genel Docker ana makinesi) yapıştırın.

!!! danger "APP_KEY'inizi kaybetmeyin"
    `APP_KEY`'i kaybetmek, şifreli yedeklerinizi kurtarılamaz hale getirir. Onu güvenli ve sunucudan ayrı bir yerde saklayın. BombVault çalışmaya başladıktan sonra, tam kurtarma paketini kaydetmek için tek tıklamayla çalışan **şifreleme anahtarı kurtarma kitini** (bkz. [Site dışı ve kurtarma](offsite-recovery.md)) kullanın.

Şablon ayrıca Docker soketini, flash'ı (`/boot`) ve **Host Data** kökünü (`/mnt`) sizin için bağlar. Yedekleme *kaynakları* ve *hedefleri* her ikisi de Host Data altında yer alır. Tam değişken referansı ve site dışı kurulum için bkz. [Yapılandırma](configuration.md).

## İlk çalıştırma

![İlk yedekten sonraki panel: neyin korunduğu, sırada ne olduğu ve canlı bir günlük.](assets/screenshots/dashboard.png)

*İlk yedekten sonraki panel: neyin korunduğu, sırada ne olduğu ve canlı bir günlük.*

1. Web arayüzünü `https://<your-unraid-ip>:3443` adresinde açın (kutudan çıktığı gibi kendinden imzalı sertifika).
2. **Ayarlar**'da istediğiniz yedekleme etki alanlarını etkinleştirin (Konteynerler, VM'ler, Flash, Öz yedek, Klasörler, ZFS veri kümeleri) ve bir vurgu rengi seçin.
3. **Konteynerler** sekmesinde bir konteyner seçin ve ilk geri yükleme noktanızı oluşturmak için **Şimdi yedekle**'ye tıklayın. Depo yolları varsayılan olarak `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` şeklindedir ve ilk yedeklemede oluşturulur.
4. Zamanlamayı **Ayarlar, Zamanlamalar** bölümünden kurun. Konteynerler ve VM'ler için tek tıklamalık bir *Tümünü zamanlamaya ekle* seçeneği vardır.

!!! tip "İsteğe bağlı: bir yedekleme sırası seçin"
    Bazı konteynerlerin her zaman diğerlerinden önce yedeklenmesi gerekiyorsa (örneğin onu kullanan uygulamadan önce bir veritabanı), Konteynerler sayfasındaki **Yedekleme sırası** panelini açın ve onları istediğiniz sıraya sürükleyin. Zamanlanan ve çoklu seçim çalışmaları buna uyar; sıralamadan bıraktığınız her şey, eskisi gibi en çok geciken önce yedeklenir.

!!! note "Host Entegrasyon Denetimi"
    Konteyner başladıktan sonra web arayüzünde `/spike`'ı açın. Her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir; böylece ona güvenmeden önce konteynerin doğru bağlandığını onaylayabilirsiniz.

## Basit ve Gelişmiş karşılaştırması

![Ayarlarda Kaydet düğmesi yoktur: her değişiklik yaptığınız anda yazılır.](assets/screenshots/settings.png)

*Ayarlarda Kaydet düğmesi yoktur: her değişiklik yaptığınız anda yazılır.*

Varsayılan olarak arayüz yalnızca temel unsurları gösterir (yedekle, geri yükle, zamanla). Uzman denetimlerini ortaya çıkarmak için kenar çubuğundaki **Basit görünüm / Gelişmiş görünüm** anahtarını kullanın: saklama, site dışı kopya, ön/son kancalar, dosya düzeyinde geri yükleme, bildirimler, Prometheus metrikleri ve bütünlük/bakım araçları. Bu, tarayıcı başına bir tercihtir ve varsayılan olarak kapalıdır; böylece yeni gelenler temiz bir arayüz, güçlü kullanıcılar ise her şeyi elde eder.

## Kaynaktan derleme {#build-from-source}

BombVault, bir JSON API'si ve gömülü bir React arayüzü sunan tek bir statik Go ikili dosyasıdır. Önce arayüzü derleyin, sonra ikili dosyayı çalıştırın:

```bash
npm --prefix web ci
npm --prefix web run build     # ikili dosyanın gömdüğü web/dist'i yazar
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # gerçek bir restic gidiş dönüşüyle birim ve entegrasyon testleri
golangci-lint run ./...
go run ./cmd/bombvault         # kendinden imzalı bir sertifikayla https://localhost:3443 sunar
```

Arayüz derlemesi `go run` için de gereklidir. Depo `web/dist` altında yalnızca boş bir işaret dosyasını izler, bu yüzden `npm --prefix web run build` olmadan ikili dosya hiçbir şey gömmez ve `500 SPA index not found` yanıtını verir; bu beklenen bir durumdur. Docker, libvirt ve Unraid CI'da test edilemez, bu yüzden bir pull request açmadan önce bağlamaları, restic'i ve VM SSH bağlantısını gerçek bir host üzerinde Host Entegrasyon Denetimi (`/spike`) ile kontrol edin.

## Sonraki adımlar

- Tüm **[Özellikler](features.md)**'e göz atın.
- **[Android uygulaması](android.md)** ile grubunuzdaki her sunucuyu telefonunuza getirin.
- Bir veya daha fazla **[Site dışı ve kurtarma](offsite-recovery.md)** kopyası ekleyin (her etki alanı aynı anda birkaç hedefe gönderebilir) ve kurtarma kitinizi kaydedin.
- Bir kurulumu klonluyor ya da yeni bir makineye mi geçiyorsunuz? Tüm yapılandırmanızı **Ayarları dışa / içe aktar** kartıyla taşıyın. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).
- Bir sorunla mı karşılaştınız? Bkz. **[Sorun giderme](troubleshooting.md)**.
