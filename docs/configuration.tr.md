# Yapılandırma

Bu sayfa konteynerin ortam değişkenlerini, şablonun sağladığı bağlamaları, SSH üzerinden VM yedeklemesini ve site dışı kurulumu kapsar. Yedekleme **depo yolları** ortam değişkenleriyle değil, uygulamanın içinde yapılandırılır (Ayarlar, Yedekleme yolları).

## Ortam değişkenleri

| Değişken | Gerekli | Açıklama |
|---|---|---|
| `APP_KEY` | **Evet** | restic depo parolasını türetmek için kullanılan 32 baytlık onaltılık gizli anahtar (64 onaltılık karakter). `openssl rand -hex 32` ile oluşturun. Bunu güvende tutun: kaybetmek şifreli yedekleri kurtarılamaz hale getirir. |
| `LIBVIRT_HOST` | VM'ler için | VM yedeklemesi için SSH üzerinden ulaşılan Unraid host'u (varsayılan `host.docker.internal`; şablon bir LAN-IP yer tutucusunu önceden doldurur). Unraid LAN IP'nizi kullanın, özel bir `br0.x` ağında gereklidir. |
| `LIBVIRT_SSH_PORT` | Hayır | VM yedeklemesi için host SSH portu (varsayılan `22`). |
| `LIBVIRT_SSH_USER` | Hayır | VM yedeklemesi için host'taki SSH kullanıcısı (varsayılan `root`). |
| `LIBVIRT_URI` | Hayır | Tam libvirt bağlantı URI'si; yukarıdaki üç `LIBVIRT_*` değişkeninden bir tane oluşturmak yerine **harfiyen** kullanılır (bu durumda söz konusu değişkenler bağlantı dizesi için yok sayılır). Varsayılan olarak ayarlanmamıştır. libvirtd'i standart olmayan, oluşturulan dize biçiminin ifade edemediği bir soket üzerinden dinleyen TrueNAS Scale'de gereklidir: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. TrueNAS Scale bölümü GitHub'daki [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) adresinde yer alır. |
| `PORT` | Hayır | HTTP portu (varsayılan `3000`; yalnızca `HTTP_ONLY=true` ile kullanılır). |
| `HTTPS_PORT` | Hayır | HTTPS portu (varsayılan `3443`; şablon onu 1:1 yayımlar, böylece WebUI `https://<ip>:3443` üzerinde yanıt verir). |
| `HTTP_ONLY` | Hayır | Kendinden imzalı HTTPS dinleyicisini devre dışı bırakmak ve yalnızca düz HTTP sunmak için `true` ayarlayın (TLS'yi sonlandıran bir ters proxy arkasında kullanım için). |
| `HOST_SOURCE_ROOT` | Hayır | **Host Data** olarak bağlanan host yolu (varsayılan `/mnt`). BombVault, Docker'ın bildirdiği bağlama kaynaklarını bu bağlamanın altındaki yollara çevirir. Yalnızca farklı bir host kökü bağladıysanız değiştirin. |
| `DATA_ROOT_SEGMENTS` | Hayır | Bir bağlama kaynağını yedekleme verisi olarak işaretleyen, virgülle ayrılmış yol segmenti adları (varsayılan `appdata`, Unraid'in `/mnt/user/appdata/<container>` kuralıyla eşleşir). Listelenen segmentlerden HERHANGİ biri host kaynağının tam bir yol segmenti olarak göründüğünde bir konteynerin bağlaması yedekleme için otomatik seçilir; örneğin `DATA_ROOT_SEGMENTS=appdata,config` bir `.../config` bağlamasını da yakalar. Bir konteynerin veri klasörünün bulunduğu diğer, her zaman etkin yöntemler için [Yedekleme kaynağı algılama](#backup-source-detection) bölümüne bakın. |
| `PLATFORM` | Hayır | Otomatik algılama yerine, BombVault'un kendisini hangi platformda çalışıyor sayacağını zorunlu kılar: `unraid`, `generic` veya `truenas` (varsayılan olarak ayarlanmamıştır: flash bağlamasının altında `dockerMan` işaretini yoklayarak Unraid'i otomatik algılar, aksi halde `generic` kullanır; tanınmayan bir değer de günlüğe kaydedilerek `generic`'e geri döner). Yalnızca Unraid'e özgü otomatik yoklamaya güvenmek yerine, genel bir Docker host'unda veya TrueNAS Scale'de bunu açıkça ayarlayın; genel compose dosyası zaten böyle yapar. appdata-fallback kuralını, örnekler arası geri yükleme hedefi varsayılanlarını ve yalnızca Unraid'e özgü bildirim/yardımcı eklenti adımlarının hiç denenip denenmeyeceğini değiştirir (bkz. `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Hayır | BombVault konteynerinin kendi adı, böylece kendisini asla yedeklemez (ve dolayısıyla durdurmaz) (varsayılan `BombVault`; köprü ağında ana bilgisayar adı aracılığıyla otomatik algılanır). |
| `BACKUP_MAX_HOURS` | Hayır | Tek bir yedekleme çalışmasının, zorla iptal edilmeden önce etki alanı kilidini tutabileceği maksimum duvar saati saati (sıkışmış bir çalışmanın etki alanını sonsuza dek engelleyememesi için bir koruma). Boş (varsayılan) `48` kullanır. Çok büyük ya da yavaş bulut yedeklemeleri için artırın (sınırda iptal edilen bir çalışma `context deadline exceeded` ile başarısız olur). Sınırı tamamen devre dışı bırakmak için `0` ayarlayın. |
| `TZ` | Hayır | Zamanlayıcı için saat dilimi (örneğin `Europe/Berlin`). |

## Bağlamalar

Docker soketini, flash'ı (`/boot`) ve **Host Data** kökünü (`/mnt`) CA şablonunda gösterildiği gibi bağlayın. Yedekleme *kaynakları* ve *hedefleri* her ikisi de Host Data altında yer alır ve o **rslave** olarak bağlanır, böylece konteyner başladıktan sonra bağlanan bir uzak paylaşım (örneğin `/mnt/remotes` altında) yeniden başlatma olmadan görünür hale gelir.

Yedekleme depo yolları varsayılan olarak `/mnt/user/bombvault/{container,vms,flash,config,files}` şeklindedir, ilk yedeklemede oluşturulur. Konumu istediğiniz zaman **Ayarlar, Yedekleme yolları**'nda değiştirin.

!!! note "Host entegrasyon denetimi"
    Konteyner başladıktan sonra web arayüzünde `/spike`'ı açın. Her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir.

## Güvenlik modeli

!!! warning "Host üzerinde root eşdeğeri denetim"
    Docker soketi aracılığıyla BombVault konteynerleri durdurabilir, kaldırabilir ve yeniden oluşturabilir, appdata'yı okuyup yazabilir ve VM yedeklemesi için `virsh` çalıştırmak üzere host'a SSH ile (`qemu+ssh://`, varsayılan olarak root) giriş yapar. Web arayüzüne ulaşabilen herkes, host üzerinde etkin biçimde root yetkisine sahiptir.

- **İsteğe bağlı parola koruması** (Ayarlar, Güvenlik): giriş yapmayı gerektirmek için bir parola ayarlayın, devre dışı bırakmak için temizleyin. Güvenilen LAN kullanımı için varsayılan olarak kapalıdır. Oturumlar imzalıdır (`APP_KEY`'den türetilen HMAC) ve parolayı değiştirmek onları geçersiz kılar; girişler hız sınırlıdır.
- Kapı isteğe bağlı olduğu için, ayarlanmadığında tüm arayüz ve API (site dışı kurulum, kurcalama testi rotaları ve kurtarma kiti dahil) porta ulaşabilen herkes tarafından erişilebilirdir. Site dışı, değiştirilemez yedekler ya da şifreleme kullanıldığında kapıyı etkinleştirin.
- BombVault'u yalnızca güvenilen, dışarıya açık olmayan bir ağda çalıştırın. Uzaktan erişim için onu kimlik doğrulama ve TLS ekleyen bir ters proxy arkasına yerleştirin. Yanıtlar temel güvenlik başlıklarını taşır (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- `HTTP_ONLY=true` ile oturum çerezi `Secure` bayrağını kaybeder (düz HTTP üzerinde çalışması için buna zorunludur), bu nedenle gizlilik önemliyse parolayı yalnızca TLS'yi sonlandıran bir proxy arkasında etkinleştirin.
- VM yedekleme SSH bağlantısı, ilk bağlantıda host anahtarına güvenir (TOFU) ve sonrasında onu sabitler. Konteynerden host'a giden yolunuz güvenilir değilse host'un anahtarını bant dışı doğrulayın.
- Şifreleme etkinleştirildiğinde (Ayarlar; varsayılan olarak açık) yedekler restic tarafından şifrelenir, anahtar `APP_KEY`'den türetilir.

## SSH üzerinden VM yedeklemesi

BombVault, KVM/libvirt VM'lerini **herhangi bir libvirt yolunu bağlamadan** yedekler. `virsh`'i host'ta SSH üzerinden (`qemu+ssh://`) çalıştırır, böylece host VM Manager'ınızı asla etkileyemez.

Hızlı kurulum:

1. **Ayarlar, Sistem, SSH üzerinden VM Yedeği:** gösterilen genel anahtarı kopyalayın.
2. Onu Unraid'in `/root/.ssh/authorized_keys` dosyasına ekleyin (yeniden başlatmalarda kalıcı olması için flash'a da yazılır).
3. **Bağlantıyı test et**'e tıklayın.

Şablon, konteynerin host'a ulaşabilmesi için `--add-host=host.docker.internal:host-gateway` ekler. O ad çözümlenmezse (örneğin konteyner özel bir `br0.x` ağında çalıştığında) `LIBVIRT_HOST`'u Unraid LAN IP'nize ayarlayın. Unraid'in SSH portunu değiştirdiyseniz, eşleşmesi için `LIBVIRT_SSH_PORT`'u ayarlayın. **Canlı anlık görüntüler** ayrıca VM'de qemu guest agent ve diskin `/mnt/cache` üzerinde (`/mnt/user` değil) olmasını gerektirir.

!!! important "Tam VM kurulumu ve ağ kılavuzu"
    Tam adım adım kılavuz (SSH etkinleştirme, kalıcı anahtar yetkilendirme, özel ağ ve VLAN yönlendirme, VM başına yöntem ve host tarafı sorun giderme) GitHub'daki [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) adresinde yer alır.

## Site dışı kurulum

**Ayarlar, Site dışı** sekmesinde bir site dışı kopya kurun. Tam iş akışı için (değiştirilemez/yalnızca ekleme, kurcalama testi ve DR tatbikatları) bkz. [Site dışı ve kurtarma](offsite-recovery.md). Kısaca:

- **Arka uçlar:** SMB/CIFS ve NFS (paylaşımı bağlayın ve ona bir Yedekleme Yolu ayarlayın), rclone olmadan yerel restic arka uçları (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) ya da herhangi bir rclone uzak konumu (`rclone:<remote>:<bucket>/path`).
- **Bulut kimlik bilgileri** Ayarlar, Site dışı, Bulut kimlik bilgileri altında şifreli saklanır.
- **SSH hedefleri karşı tarafta hiçbir şey kurmayı gerektirmez.** `sftp:` yalnızca bir SSH sunucusu gerektirir. **Ayarlar, Sistem, SSH üzerinden VM Yedeği** bölümündeki genel anahtarı (ayrıca `/config/ssh/id_ed25519.pub` konumunda) hedef kullanıcının `~/.ssh/authorized_keys` dosyasına ekleyin.
- **Site dışı kopya:** BombVault yeni anlık görüntüleri en iyi çaba temelinde `restic copy` ile çoğaltır. Yerel depo birincil kalır. Her etki alanının kendi site dışı zamanlaması ve ayrıca bir **Şimdi çoğalt** düğmesi vardır.
- **Etki alanı başına birden fazla site dışı hedef:** her etki alanı aynı anda birkaç site dışı hedefe çoğaltabilir. Ayarlar, Site dışı'nda her biri kendi deposu, S3 depolama sınıfı, yalnızca ekleme bayrağı, saklama ve büyüme bütçesiyle ek hedefler ekleyin; hepsi o etki alanının site dışı zamanlamasında çoğaltılır. Mevcut tek bir site dışı kurulum ilk hedef olarak taşınır.
- **Kaynak başına saklama:** yerel ilke Ayarlar, Yollar ve Depolama'da yer alır; site dışı ilke Ayarlar, Site dışı'nda (site dışı anlık görüntüleri asla otomatik kırpmamak için tümünü sıfır bırakın).
- **Bant genişliği sınırları:** Ayarlar, Site dışı altında restic yükleme/indirme hızını sınırlayın.
- **Soğuk ve arşiv depolama sınıfı (S3):** yerel bir S3 site dışı deposu için geri yüklenebilir bir katman seçin (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone uzak konumları sınıflarını rclone yapılandırmasında ayarlar.

## Taşınabilir ayarlar (dışa ve içe aktarma) {#portable-settings-export-and-import}

Ayarlar sayfasındaki **Ayarları dışa ve içe aktar** kartı, tüm BombVault yapılandırmanızı (etki alanı ayarları, site dışı hedefler, zamanlamalar, saklama, bildirimler) başka bir örnekte içe aktarabileceğiniz taşınabilir bir JSON dosyasına yazar, böylece yeni bir makineye taşınmak ya da bir kurulumu klonlamak her şeyi elle yeniden girmek anlamına gelmez. İçe aktarma bir önizleme gösterir ve onay ister ve yedekleme verilerinize ya da geçmişinize asla dokunmaz.

!!! warning "Dışa aktarma kimlik bilgileri içerebilir"
    Site dışı ve bildirim kimlik bilgilerini dosyaya dahil edip etmeyeceğinizi siz seçersiniz. Kimlik bilgileri dahilken, dışa aktarma kurtarma kitiniz kadar hassastır, bu nedenle onu güvenli bir yerde saklayın. Onlarsız, dosya yalnızca gizli olmayan ayarları tutar.
