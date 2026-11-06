# BombVault

**Unraid verin, bir kasada mühürlü. Bir yedek bırak. Bir geri yüklemeyi patlat.**

![Genel bakış: durum başlığı, sana ihtiyaç duyanlar, her bölümün koruması, hedefler ve geçmiş.](assets/screenshots/overview/home-dark.png#only-dark)
![Genel bakış: durum başlığı, sana ihtiyaç duyanlar, her bölümün koruması, hedefler ve geçmiş.](assets/screenshots/overview/home-light.png#only-light)

BombVault, Unraid ve TrueNAS SCALE, ZimaOS ya da düz bir Linux makinesi gibi diğer Docker ana makinelerinde **yedekleme ve tam felaket kurtarma** için kendi sunucunda çalıştırdığın bir web uygulamasıdır. Docker konteynerlerini, KVM/libvirt VM'lerini, klasörleri, ZFS veri kümelerini ve Unraid flash sürücüsünü yedekler. Tek bir çok mimarili Docker konteyneri olarak çalışır, sana koyu, açık ya da sistem temalı bir web arayüzü sunar ve tüm yaşam döngüsünü yönetir: yedekle, zamanla, doğrula ve geri yükle.

Geri yüklemeler otomatiktir. Unraid'de konteynerler Docker sekmesinde eskisi gibi yeniden görünür, VM'ler de diskleri ve UEFI NVRAM'ları yeniden bağlanmış olarak VM Manager'da yeniden tanımlanır. Elle yeniden kurulum yok, yeniden yapılandırma yok, dram yok. TrueNAS'ta orijinalin yanına geri yüklenen bir konteyner, Apps ekranının listelemediği düz bir Docker konteyneri olarak çalışır.

[restic](https://restic.net) ile çalışır, bu yüzden her yedek tekilleştirilir, artımlıdır ve varsayılan olarak şifrelidir.

!!! note "APP_KEY'ini güvende tut"
    BombVault, restic deposu parolasını `APP_KEY` adlı 32 baytlık bir gizli bilgiden türetir. Kurtarma kitiyle birlikte onu kaybetmek şifreli yedekleri kurtarılamaz yapar. `openssl rand -hex 32` ile bir tane üret ve güvenli bir yerde sakla. Bkz. [Yapılandırma](configuration.md).

## BombVault neyi korur {#what-bombvault-protects}

Her konteyner, VM, klasör kümesi, flash sürücü, her ZFS veri kümesi ve BombVault'un kendi ayarları, kendi sayfası olan bir **öge**dir. Aynı türden ögeler, **Ayarlar, Genel** altında açtığın bir **bölüm** oluşturur.

| Bölüm | Ne saklanır |
|---|---|
| **Konteynerler** | Appdata dizini ve konteyner tanımı (image, ortam değişkenleri, portlar, etiketler, birimler). |
| **VM'ler** | VM disk image'leri, XML tanımı ve UEFI NVRAM, SSH üzerinden yedeklenir (libvirt bağlaması yok). |
| **Flash** | Tüm USB flash (`/boot`): işletim sistemi, lisans, dizi yapılandırması, paylaşımlar, ağ ve eklenti yapılandırması. |
| **Öz yedek** | BombVault'un kendi `/config`'i: ayar veritabanı, hedeflerinin oturum açma bilgileri, libvirt SSH anahtar çifti, ZFS sunucularının anahtarı ile sabitlenmiş ana makine anahtarları ve sertifikalar. |
| **Klasörler** | Sunucudaki herhangi bir klasör, bir klasör kümesi olarak, her birinin isteğe bağlı hariç tutma desenleri vardır. |
| **ZFS veri kümeleri** | Bir veri kümesi ve altındaki her veri kümesi, tek bir ZFS anlık görüntüsünden okunur ve bir klasör gibi saklanır. Bkz. [ZFS veri kümeleri](zfs-datasets.md). |

## Geri yükleme yıldızdır {#restore-is-the-star}

Veriyi restic anlık görüntüsünden geri kopyaladıktan sonra BombVault, kayıtlı konteyner tanımını Docker API'sine karşı yeniden oynatır, böylece konteyner Unraid Docker sekmesinde sanki hep oradaymış gibi görünür (aynı image, aynı ayarlar, aynı port eşlemeleri). VM'lerin XML'i SSH üzerinden yeniden tanımlanır ve diskleri ile UEFI NVRAM'ları, VM silinmiş olsa bile, yeniden bağlanır.

Tek bir pencere her türü geri yükler. Varsayılan olarak yedeği bir kopya olarak geri getiren ve orijinali olduğu gibi bırakan **Orijinalin yanına**'yı seçer. **Orijinali değiştir** önce mevcut durumu yedekler, böylece bir geri yükleme geri alınabilir. Bir klasör ya da yeni bir ZFS veri kümesi diğer hedeflerdir.

Bir yedek bağımlı konteynerleri durdurduğunda, doğru sırayla geri gelirler: BombVault onları Compose `depends_on` sırasıyla yeniden başlatır ve bağımlı olanları başlatmadan önce her birinin sağlıklı bildirmesini bekler, böylece henüz ayakta olmayan bir veritabanının ya da ağ geçidinin önüne hiçbir şey geçmez. Bkz. [Geri yükleme](restore.md) ve [Komutlar ve diğer konteynerler](backups.md#commands).

## Nasıl çalışır {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault, bir yedekten önce konteynerleri durdurmak ve geri yüklemeden sonra yeniden oluşturmak için Docker soketini kullanır. VM'ler için, bir domaini düzgünce kapatmak ya da canlı anlık görüntü almak üzere ana makinede SSH üzerinden (`qemu+ssh://`) `virsh` çalıştırır. Hiçbir libvirt yolunu bağlamaz, bu yüzden ana makinedeki VM Manager'ın yoluna çıkamaz.

BombVault orkestrasyon ve arayüz katmanıdır, depolama motoru değil. Tüm gerçek veri hareketi restic üzerinden geçer.

## Hızlı başlangıç {#quick-start}

Burada yeni misin? BombVault'u kurmak ve ilk yedeğini almak için **[Başlarken](getting-started.md)** sayfasına git, sonra **[Site dışı ve kurtarma](offsite-recovery.md)**'yı kur. **[Adım adım örnekler](guides.md)** yaygın işleri baştan sona anlatır, **[Ayarlar](settings.md)** sayfası her kutucuğu listeler, **[Özellikler](features.md)** de BombVault'un yaptığı her şeyin kısa kataloğudur.

Her bölüm aynı anda birden çok hedefe kopyalayabilir, eşleştirilmiş bir BombVault site dışı kopyalarını silmeyi reddeden bir sunucuda tutabilir ve **Acil durum sihirbazı** kayıp bir sunucuyu kurtarma kitinden, grubun üzerinden ya da yazılmış anahtarıyla geri getirir. Yapılandırmanı yeni bir makineye taşımak için **Ayarları dışa / içe aktar** kartını kullan. Bkz. [Site dışı ve kurtarma](offsite-recovery.md) ve [Yapılandırma](configuration.md#portable-settings-export-and-import).

**[Android uygulaması](android.md)** grubundaki her sunucuyu, hepsinin etkinlik günlüğüyle birlikte tek ekranda telefonuna getirir.

Belgeler 26 dilde, arayüz ise 42 dilde sunulur. Henüz çevirisi olmayan sayfalar İngilizce görünür.

## Burada kullanılan sözcükler {#glossary}

Birkaç sözcüğün BombVault'ta ve bu sayfalarda sabit bir anlamı vardır.

| Sözcük | Anlamı |
|---|---|
| **Öge** | BombVault'un yedeklediği tek bir şey: bir konteyner, bir VM, bir klasör kümesi, flash sürücü, altındakilerle birlikte bir ZFS veri kümesi ya da kendi ayarları. Her birinin kendi sayfası vardır. |
| **Bölüm** | Konteynerler gibi aynı türden tüm ögeler. Bölümleri **Ayarlar, Genel** altında açarsın. |
| **Hedef** | Yedeklerin gittiği bir yer: bu sunucudaki bir klasör, bir paylaşım, kendi sunucun ya da bir bulut servisi. Bkz. [Hedefler](offsite-recovery.md#destinations). |
| **Yerel** | Bölümlerin klasörlerini tutan, bu sunucudaki hedef. |
| **Kendi klasörü** | Bir bölümün yedeklerini ilk yazdığı depo. Varsayılan olarak bu sunucudadır ve bir hedefte durabilir. Bkz. [Bir bölümün kendi klasörü](offsite-recovery.md#own-folder). |
| **Kopya** | Kendi klasöründen başka bir hedefe `restic copy` ile kopyalanmış bir yedek. |
| **Doğrudan depo** | Ögelerin aralarında yerel kopya olmadan doğrudan yedeklediği, site dışı bir konumdaki depo. Bkz. [Doğrudan depolar](offsite-recovery.md#direct-repositories). |
| **Oturum açma bilgisi** | Bir S3 servisi ya da rest-server için kayıtlı anahtar veya parola. Biri varsayılandır. Bkz. [Oturum açma bilgileri](offsite-recovery.md#sign-ins). |
| **Yerleşim** | Bir ögenin yedeklerini hangi hedeflerin aldığı, ögede ya da bölümünün varsayılanı olarak ayarlanır. Bkz. [Öge başına yerleşim](offsite-recovery.md#placement). |
| **Replika** | Bir ZFS ögesinin ikinci bir ZFS sunucusundaki, `zfs send` ile yapılmış, blok blok aynı kopyası. Yedeğin yanında durur ve onun yerini almaz. Bkz. [Replika](zfs-datasets.md#replica). |
| **Rol** | Grubunun bir örneğinin başka bir örnek için yaptığı şey: Alıcı, Çeken, ZFS sunucusu ya da Anahtar bekçisi. Bkz. [Örnekler ve roller](offsite-recovery.md#roles). |

## Katkıda bulunanlar {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**, [@Darkdragon14](https://github.com/Darkdragon14) tarafından (Apache-2.0), BombVault'a başlangıç fikrini verdi: Docker konteynerlerinin tek tıkla yedeklenmesi ve otomatik yeniden kurulumu. BombVault, bu fikri VM'lere, flash'a ve daha fazlasına taşıyan, Go ve restic üzerinde ayrı bir uygulamadır.
- **[restic](https://restic.net/)**, BombVault'un yönettiği hızlı, güvenli, tekilleştiren yedekleme motorudur.
- **[rclone](https://rclone.org/)** bulut arka uçlarını sağlar.
- Düğmelerdeki simgelerin çoğu **[Streamline](https://streamlinehq.com)**'ın ücretsiz Core Solid setinden gelir ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [kaynak](https://github.com/webalys-hq/streamline-vectors)). Diğerleri Font Awesome Free, Material Design Icons, Simple Icons ve Tabler Icons'tan gelir ya da proje için çizildi.

## Lisans {#license}

Copyright (C) 2026 Junker der Provinz. BombVault, **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)) altında özgür yazılımdır. Çalıştırabilir, inceleyebilir, paylaşabilir ve değiştirebilirsin. Dağıtırsan ya da değiştirilmiş bir sürümü ağ servisi olarak çalıştırırsan, kaynağını aynı lisansla yayımlamak ve mevcut telif ve atıf bildirimlerini korumak zorundasın.

Ad ve marka lisanslanmaz. AGPL yalnızca kaynak kodu kapsar: "BombVault", logosu ve markası saklıdır, bu yüzden bir fork kendi adını ve markasını kullanmalıdır ve kendisini BombVault olarak gösteremez.

## Bağlantılar {#links}

- **Kaynak kodu:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid destek konusu:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issue'lar:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Ana makine üzerinde root'a denk denetim"
    Docker soketi aracılığıyla BombVault konteynerleri durdurabilir, kaldırabilir ve yeniden oluşturabilir, appdata'yı okuyup yazabilir, VM yedekleme için de `virsh` çalıştırmak üzere SSH ile ana makinede oturum açar. Web arayüzüne ulaşabilen herkes fiilen ana makinede root'tur. BombVault'u yalnızca güvenilir, dışa açık olmayan bir ağda çalıştır ve **Ayarlar, Güvenlik** altında bir giriş parolası ayarla. Kurtarma kiti, anahtar emanetleri, flash ZIP'i ve bir flash yedeğini ya da öz yedeği dışa aktarma bunu gerektirir. Tam güvenlik modeli için bkz. [Yapılandırma](configuration.md#security-model).
