# Başlarken

Bu sayfa sizi sıfırdan bir Unraid makinesinden ilk yedeğinize kadar götürür.

## Gereksinimler

| Gereksinim | Notlar |
|---|---|
| **Unraid 6.12+** | Daha eski sürümler test edilmemiştir. |
| **Restic depo konumu** | Yerel bir yol (önerilen: diziniz ya da önbelleğiniz), SMB, NFS veya herhangi bir rclone arka ucu. |
| **Docker soketi** | Şablon tarafından otomatik olarak bağlanır (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Şablon tarafından tümüyle otomatik bağlanır (`/boot` -> `/host/boot`). Flash yedeklemesini besler ve geri yüklenen bir konteynerin normal, düzenlenebilir bir Unraid uygulaması olarak yeniden görünmesini sağlar. |
| **KVM VM'leri** (isteğe bağlı) | VM yedeklemesi libvirt ile SSH üzerinden konuşur, libvirt bağlaması yok. Ayarlar'da kurun (bkz. [Yapılandırma](configuration.md)). |

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

## Gereken tek ayar

Ayarlamanız gereken tek değişken, restic depo parolasını türetmek için kullanılan 32 baytlık bir onaltılık gizli anahtar olan (64 onaltılık karakter) `APP_KEY`'dir.

Herhangi bir makinede bir tane oluşturun:

```bash
openssl rand -hex 32
```

Sonucu şablonun `APP_KEY` alanına yapıştırın.

!!! danger "APP_KEY'inizi kaybetmeyin"
    `APP_KEY`'i kaybetmek, şifreli yedeklerinizi kurtarılamaz hale getirir. Onu güvenli ve sunucudan ayrı bir yerde saklayın. BombVault çalışmaya başladıktan sonra, tam kurtarma paketini kaydetmek için tek tıklamayla çalışan **şifreleme anahtarı kurtarma kitini** (bkz. [Site dışı ve kurtarma](offsite-recovery.md)) kullanın.

Şablon ayrıca Docker soketini, flash'ı (`/boot`) ve **Host Data** kökünü (`/mnt`) sizin için bağlar. Yedekleme *kaynakları* ve *hedefleri* her ikisi de Host Data altında yer alır. Tam değişken referansı ve site dışı kurulum için bkz. [Yapılandırma](configuration.md).

## İlk çalıştırma

1. Web arayüzünü `https://<your-unraid-ip>:3443` adresinde açın (kutudan çıktığı gibi kendinden imzalı sertifika).
2. **Ayarlar**'da istediğiniz yedekleme etki alanlarını etkinleştirin (Konteynerler, VM'ler, Flash, Config, Dosyalar) ve bir vurgu rengi seçin.
3. **Konteynerler** sekmesinde bir konteyner seçin ve ilk geri yükleme noktanızı oluşturmak için **Yedekle**'ye tıklayın. Depo yolları varsayılan olarak `/mnt/user/bombvault/{container,vms,flash,config,files}` şeklindedir ve ilk yedeklemede oluşturulur.
4. Zamanlamayı **Ayarlar, Zamanlamalar** bölümünden kurun. Konteynerler ve VM'ler için tek tıklamalık bir *tümünü zamanlamaya ekle* seçeneği vardır.

!!! tip "İsteğe bağlı: bir yedekleme sırası seçin"
    Bazı konteynerlerin her zaman diğerlerinden önce yedeklenmesi gerekiyorsa (örneğin onu kullanan uygulamadan önce bir veritabanı), Konteynerler sayfasındaki **yedekleme sırası** panelini açın ve onları istediğiniz sıraya sürükleyin. Zamanlanan ve çoklu seçim çalışmaları buna uyar; sıralamadan bıraktığınız her şey, eskisi gibi en çok geciken önce yedeklenir.

!!! note "Host entegrasyon denetimi"
    Konteyner başladıktan sonra web arayüzünde `/spike`'ı açın. Her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir; böylece ona güvenmeden önce konteynerin doğru bağlandığını onaylayabilirsiniz.

## Basit ve Gelişmiş karşılaştırması

Varsayılan olarak arayüz yalnızca temel unsurları gösterir (yedekle, geri yükle, zamanla). Uzman denetimlerini ortaya çıkarmak için kenar çubuğundaki **Basit / Gelişmiş** anahtarını kullanın: saklama, site dışı kopya, ön/son kancalar, dosya düzeyinde geri yükleme, bildirimler, Prometheus metrikleri ve bütünlük/bakım araçları. Bu, tarayıcı başına bir tercihtir ve varsayılan olarak kapalıdır; böylece yeni gelenler temiz bir arayüz, güçlü kullanıcılar ise her şeyi elde eder.

## Sonraki adımlar

- Tüm **[Özellikler](features.md)**'e göz atın.
- Bir veya daha fazla **[Site dışı ve kurtarma](offsite-recovery.md)** kopyası ekleyin (her etki alanı aynı anda birkaç hedefe gönderebilir) ve kurtarma kitinizi kaydedin.
- Bir kurulumu klonluyor ya da yeni bir makineye mi geçiyorsunuz? Tüm yapılandırmanızı **Ayarları dışa ve içe aktar** kartıyla taşıyın. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).
- Bir sorunla mı karşılaştınız? Bkz. **[Sorun giderme](troubleshooting.md)**.
