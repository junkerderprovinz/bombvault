# BombVault

**Unraid verileriniz, bir kasada mühürlü. Bir yedek bırakın. Bir geri yüklemeyi patlatın.**

BombVault, Docker konteynerlerinizin ve KVM/libvirt VM'lerinizin **yedeklenmesi ve tam felaket kurtarması** için kendinizin barındırdığı, Unraid'e özgü bir web uygulamasıdır. Tek bir çok mimarili Docker konteyneri olarak çalışır, size sisteminizin açık/koyu tema tercihini izleyen modern bir web arayüzü sunar ve tüm yaşam döngüsünü yönetir: yedekle, zamanla, doğrula ve geri yükle.

Geri yüklemeler otomatiktir. Konteynerler, Unraid Docker sekmesinde tam olarak eskisi gibi yeniden görünür ve VM'ler, diskleri ve UEFI NVRAM'i yeniden bağlanmış olarak VM Manager'da yeniden tanımlanır. Elle yeniden kurulum yok, yeniden yapılandırma yok, dram yok.

[restic](https://restic.net) ile güçlendirilmiştir, böylece her yedek yinelenenlerden arındırılmış, artımlı ve her zaman şifrelidir.

!!! note "APP_KEY'inizi güvende tutun"
    BombVault, restic depo parolasını `APP_KEY` adında 32 baytlık bir gizli anahtardan türetir. Bunu kaybetmek, şifreli yedekleri kurtarılamaz hale getirir. `openssl rand -hex 32` ile bir tane oluşturun ve güvenli bir yerde saklayın. Bkz. [Yapılandırma](configuration.md).

## BombVault neyi korur

| Etki Alanı | Neler kaydedilir |
|---|---|
| **Docker konteynerleri** | Appdata dizini ile birlikte konteyner tanımı (image, ortam değişkenleri, portlar, etiketler, birimler). |
| **KVM / libvirt VM'leri** | VM disk imajı/imajları, XML tanımı ve UEFI NVRAM, SSH üzerinden yedeklenir (libvirt bağlaması yok). |
| **Unraid flash** | Tüm USB flash (`/boot`): işletim sistemi, lisans, dizi yapılandırması, paylaşımlar, ağ ve eklenti yapılandırması. |
| **Uygulama yapılandırması** | BombVault'un kendi `/config`'i: ayar veritabanı, site dışı kimlik bilgileri ve libvirt SSH anahtar çifti. |
| **Dosyalar ve klasörler** | **Dosya kümeleri** olarak adlandırılan, sunucudaki herhangi bir klasör, her biri isteğe bağlı küme başına hariç tutma desenleriyle. |

## Yıldız, geri yüklemedir

BombVault, restic anlık görüntüsünden veriyi geri kopyaladıktan sonra, kaydedilmiş konteyner tanımını Docker API'sine karşı yeniden oynatır; böylece konteyner, hep oradaymış gibi Unraid Docker sekmesinde yeniden görünür (aynı image, aynı ayarlar, aynı port eşlemeleri). VM'lerin XML'i SSH üzerinden yeniden tanımlanır ve diskleri ile UEFI NVRAM'i yeniden bağlanır, VM silindikten sonra bile.

Bir yedekleme bağımlı konteynerleri durdurduğunda, doğru sırayla geri gelirler: BombVault onları Compose `depends_on` sırasına göre yeniden başlatır ve kendisine bağımlı olanları başlatmadan önce her birinin sağlıklı olduğunu bildirmesini bekler; böylece hiçbir şey henüz ayakta olmayan bir veritabanının ya da ağ geçidinin önüne geçmez. Bkz. [Özellikler](features.md).

## Nasıl çalışır

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault, düzenleme ve arayüz katmanıdır, depolama motoru değildir. Tüm gerçek veri hareketi restic üzerinden gerçekleşir.

## Hızlı başlangıç

Buraya yeni mi geldiniz? BombVault'u Community Applications aracılığıyla Unraid'e kurmak ve ilk yedeğinizi çalıştırmak için **[Başlarken](getting-started.md)** sayfasına gidin. Ardından tüm **[Özellikler](features.md)**'i keşfedin, **[Yapılandırma](configuration.md)**'nızı ayarlayın ve **[Site dışı ve kurtarma](offsite-recovery.md)**'yı kurun.

Site dışı, etki alanı başına aynı anda birkaç hedefe dağıtılabilir; salt okunur bir **alıcı kontrol paneli** bu kopyaları onları alan makinede izler ve **Ayarları dışa ve içe aktar** kartıyla tüm yapılandırmanızı yeni bir makineye taşıyabilirsiniz. Bkz. [Site dışı ve kurtarma](offsite-recovery.md) ve [Yapılandırma](configuration.md#portable-settings-export-and-import).

## Bağlantılar

- **Kaynak kodu:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid destek başlığı:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Sorunlar:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Host üzerinde root eşdeğeri denetim"
    Docker soketi aracılığıyla BombVault konteynerleri durdurabilir, kaldırabilir ve yeniden oluşturabilir, appdata'yı okuyup yazabilir ve VM yedeklemesi için `virsh` çalıştırmak üzere host'a SSH ile giriş yapar. Web arayüzüne ulaşabilen herkes, host üzerinde etkin biçimde root yetkisine sahiptir. BombVault'u yalnızca güvenilen, dışarıya açık olmayan bir ağda çalıştırın ve site dışı ya da değiştirilemez yedekler kullanıldığında isteğe bağlı parola kapısını (Ayarlar, Güvenlik) etkinleştirin. Tam güvenlik modeli için bkz. [Yapılandırma](configuration.md).
