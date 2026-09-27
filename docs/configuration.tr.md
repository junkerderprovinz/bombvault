# Yapılandırma

Bu sayfa konteynerin ortam değişkenlerini, şablonun sağladığı bağlamaları, SSH üzerinden VM yedeklemesini ve site dışı kurulumu kapsar. Yedeklerin nereye gideceğini ortam değişkenleriyle değil, uygulamanın içinde, **Ayarlar, Depolama** sekmesinde belirlersiniz.

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
| `TRUSTED_PROXY` | Hayır | BombVault'un önündeki ters vekil sunucunun virgülle ayrılmış adresleri veya CIDR aralıkları (örneğin `192.168.20.11` ya da `10.0.0.0/8`). `X-Forwarded-For` başlığına yalnızca bu sıçramalardan inanılır ve giriş freni o zaman başarısızlıkları gerçek istemci başına sayar, vekilin arkasındaki herkesi aynı kovaya atmak yerine. Ayarlanmadığında (varsayılan) kimseye güvenilmez: koşulsuz inanılan bir başlık, herkesin kendi kovasını seçmesine izin verirdi. |
| `HOST_SOURCE_ROOT` | Hayır | **Host Data** olarak bağlanan host yolu (varsayılan `/mnt`). BombVault, Docker'ın bildirdiği bağlama kaynaklarını bu bağlamanın altındaki yollara çevirir. Yalnızca farklı bir host kökü bağladıysanız değiştirin. |
| `DATA_ROOT_SEGMENTS` | Hayır | Bir bağlama kaynağını yedekleme verisi olarak işaretleyen, virgülle ayrılmış yol segmenti adları (varsayılan `appdata`, Unraid'in `/mnt/user/appdata/<container>` kuralıyla eşleşir). Listelenen segmentlerden HERHANGİ biri host kaynağının tam bir yol segmenti olarak göründüğünde bir konteynerin bağlaması yedekleme için otomatik seçilir; örneğin `DATA_ROOT_SEGMENTS=appdata,config` bir `.../config` bağlamasını da yakalar. Bir konteynerin veri klasörünün bulunduğu diğer, her zaman etkin yöntemler için [Yedekleme kaynağı algılama](#backup-source-detection) bölümüne bakın. |
| `PLATFORM` | Hayır | Otomatik algılama yerine, BombVault'un kendisini hangi platformda çalışıyor sayacağını zorunlu kılar: `unraid`, `generic` veya `truenas` (varsayılan olarak ayarlanmamıştır: flash bağlamasının altında `dockerMan` işaretini yoklayarak Unraid'i otomatik algılar, aksi halde `generic` kullanır; tanınmayan bir değer de günlüğe kaydedilerek `generic`'e geri döner). Yalnızca Unraid'e özgü otomatik yoklamaya güvenmek yerine, genel bir Docker host'unda veya TrueNAS Scale'de bunu açıkça ayarlayın; genel compose dosyası zaten böyle yapar. appdata-fallback kuralını, örnekler arası geri yükleme hedefi varsayılanlarını ve yalnızca Unraid'e özgü bildirim/yardımcı eklenti adımlarının hiç denenip denenmeyeceğini değiştirir (bkz. `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Hayır | BombVault konteynerinin kendi adı, böylece kendisini asla yedeklemez (ve dolayısıyla durdurmaz). |
| `BACKUP_MAX_HOURS` | Hayır | Tek bir yedekleme çalışmasının, zorla iptal edilmeden önce etki alanı kilidini tutabileceği maksimum duvar saati saati (sıkışmış bir çalışmanın etki alanını sonsuza dek engelleyememesi için bir koruma). Boş (varsayılan) `48` kullanır. Çok büyük ya da yavaş bulut yedeklemeleri için artırın (sınırda iptal edilen bir çalışma `context deadline exceeded` ile başarısız olur). Sınırı tamamen devre dışı bırakmak için `0` ayarlayın. |
| `TZ` | Hayır | Zamanlayıcı için saat dilimi (örneğin `Europe/Berlin`). **Ayarlanmazsa tüm zamanlamalar UTC olarak çalışır**: 02:30 olarak ayarlanan bir zamanlama yerel saatte değil 02:30 UTC'de başlar. Unraid'de bunu asla kendiniz ayarlamazsınız: sistem kendi saat dilimini her kapsayıcıya aktarır. |

## Bağlamalar

Docker soketini, flash'ı (`/boot`) ve **Host Data** kökünü (`/mnt`) CA şablonunda gösterildiği gibi bağlayın. Yedekleme *kaynakları* ve *hedefleri* her ikisi de Host Data altında yer alır ve o **slave** olarak bağlanır, böylece konteyner başladıktan sonra bağlanan bir uzak paylaşım (örneğin `/mnt/remotes` altında) yeniden başlatma olmadan görünür hale gelir.

Yeni bir kurulum her etki alanını **Unraid** konumunda, `/mnt/user/bombvault` altında etki alanı başına bir klasörle (`container`, `vms`, `flash`, `config`, `files`) depolar; klasörler ilk yedeklemede oluşturulur. Yerel ya da uzak başka konumları **Ayarlar, Depolama** sekmesinde eklersiniz; bkz. [Depolama konumları](storage-places.md).

!!! note "Host entegrasyon denetimi"
    Konteyner başladıktan sonra web arayüzünde `/spike`'ı açın. Her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir.

## Yedekleme kaynaklarının belirlenmesi {#backup-source-detection}

Her kapsayıcı için hangi bind bağlarının ve adlandırılmış birimlerin yedekleneceğini BombVault kendisi seçer. Aşağıdakilerden herhangi biri geçerli olur olmaz bir yol alınır (sonucu kapsayıcı bazında her zaman **Yedekleme yolları** altından değiştirebilirsiniz):

- **Veri kökü parçası eşleşmesi:** bind bağının ana makinedeki kaynağı, `DATA_ROOT_SEGMENTS` parçalarından birini tam bir yol bileşeni olarak içeriyor (varsayılan olarak yalnızca `appdata`).
- **Adlandırılmış Docker birimleri** her zaman dahil edilir, çünkü atılabilir bir karşılıkları yoktur ve süzülecek bir şey kalmaz, **ama yalnızca birimin ana makinedeki gerçek depolama yolunun kendisi Host Data bağı üzerinden erişilebilir olduğunda**, tıpkı BombVault'un yedeklediği diğer her ana makine yolu gibi. Varsayılan yerel birim sürücüsü bir birimi arka planın kendi veri kökünün altına, yani değiştirilmediyse `/var/lib/docker/volumes/<ad>/_data` yoluna koyar (`docker info -f '{{.DockerRootDir}}'` ile bakabilirsiniz). Bu konum, genel `docker-compose.yml` dosyasının varsayılan olarak kullandığı tek dizinlik dar Host Data bağının kapsamında DEĞİLDİR. Erişilemeyen birim sessizce atlanır, bu bir hata değildir. Genel bir ana makinede adlandırılmış birimlerin gerçekten yedeklenmesi için Host Data'yı (ve `HOST_SOURCE_ROOT` değerini) Docker'ın veri kökünü de kapsayan ortak bir üst dizine yöneltin: ödünleşim compose dosyasının Host Data yorumunda anlatılıyor (Unraid, aynı nedenle kendi en üst düzey genel geleneği olan `/mnt` dizininin tamamını bağlayarak bunu aşar).
- **Docker Compose proje dizini:** kapsayıcı olağan `com.docker.compose.project.working_dir` etiketini taşıyorsa (`docker compose up` bunu kendiliğinden koyar), herhangi bir bind bağının veri kökü parçasıyla eşleşip eşleşmediğine bakılmaksızın o dizin de eklenir.
- **`bombvault.data` etiketiyle geçersiz kılma:** yukarıdaki iki geleneğin de yakalayamadığı bir düzen için (örneğin Compose projesi olmayan tek bir `/srv/plex/config` bağı) kapsayıcıya `bombvault.data=true` etiketini koyarak TÜM bind bağlarını dahil edin. `false` dışındaki boş olmayan her değer doğru sayılır; etiketin bulunmaması ya da `bombvault.data=false` hiçbir şeyi değiştirmez.

## Güvenlik modeli

!!! warning "Host üzerinde root eşdeğeri denetim"
    Docker soketi aracılığıyla BombVault konteynerleri durdurabilir, kaldırabilir ve yeniden oluşturabilir, appdata'yı okuyup yazabilir ve VM yedeklemesi için `virsh` çalıştırmak üzere host'a SSH ile (`qemu+ssh://`, varsayılan olarak root) giriş yapar. Web arayüzüne ulaşabilen herkes, host üzerinde etkin biçimde root yetkisine sahiptir.

- **İsteğe bağlı parola koruması** (Ayarlar, Güvenlik): giriş zorunlu kılmak için bir parola belirle, kapatmak için temizle. Güvenilen bir yerel ağda kullanım için varsayılan olarak kapalıdır. Parola, `APP_KEY` ile biberlenmiş bir değer üzerinde Argon2id ile saklanır; böylece kopyalanmış bir `/config` anahtar olmadan işe yaramaz, anahtarla da saldırısı yavaştır. Yeni bir parola en az 12 karakter olmalıdır; mevcut daha kısa bir parola değiştirilene kadar çalışmaya devam eder. Oturumlar imzalıdır (`APP_KEY`'den türetilen HMAC) ve parola değişikliği onları geçersiz kılar; girişler istemci başına dakikada beş başarısızlıkla sınırlıdır.
- **İki adımlı doğrulama** (Ayarlar): parolanın yanında bir kimlik doğrulama uygulamasından gelen zaman kodu ve açılışta bir kez verilen sekiz tek kullanımlık kurtarma kodu. Paylaşılan gizli anahtar `APP_KEY` ile şifreli saklanır ve kapatmak için güncel bir kod gerekir.
- Kapı isteğe bağlı olduğu için, ayarlanmadığında tüm arayüz ve API (site dışı kurulum, kurcalama testi rotaları ve kurtarma kiti dahil) porta ulaşabilen herkes tarafından erişilebilirdir. Site dışı, değiştirilemez yedekler ya da şifreleme kullanıldığında kapıyı etkinleştirin.
- BombVault'u yalnızca güvenilen, dışarıya açık olmayan bir ağda çalıştırın. Uzaktan erişim için onu kimlik doğrulama ve TLS ekleyen bir ters proxy arkasına yerleştirin. Yanıtlar temel güvenlik başlıklarını taşır (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Ters vekil sunucunun arkasında her istek vekilin adresini taşır; bu yüzden `TRUSTED_PROXY` olmadan fren tüm istemcileri tek kovada sayar ve bir saldırganın başarısızlıkları seni de dışarıda bırakır. Vekili `TRUSTED_PROXY` içinde belirt ki sayım yeniden istemci başına yapılsın.
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

Site dışı kopyalar depolama konumlarına gider. Konumu **Ayarlar, Depolama** sekmesinde **Konum ekle** ile ekleyin, sonra etki alanının satırında **Kopyalama hedefleri** altında işaretleyin. [Depolama konumları](storage-places.md) her bağlantı türünü, [Site dışı ve kurtarma](offsite-recovery.md) ise append-only'yi, kurcalama testini ve DR tatbikatlarını anlatır. Kısaca:

- **Bağlantı türleri:** bu Unraid'deki bir klasör ya da `/mnt/remotes` altına bağlanmış bir NAS paylaşımı, S3 depolama (Backblaze B2 ve diğer bulut sağlayıcıları ya da MinIO veya Garage gibi kendi sunucunuzda çalışan bir hizmet), rest-server, Hetzner Storage Box dahil SFTP, Nextcloud, ownCloud ve OpenCloud için WebDAV, Azure Blob ve herhangi bir rclone uzak tanımı. Backblaze B2 yalnızca anahtarı ister: BombVault bucket'ı ve S3 uç noktasını anahtardan okur.
- **Kimlik bilgileri** ait oldukları konumla birlikte şifreli saklanır. Çekme kaynaklarının kullandığı kimlik bilgisi kümeleri, Örnekler sayfasının **Çekme** sekmesindedir.
- **SSH hedefleri karşı tarafta hiçbir şey kurmayı gerektirmez.** Bir SFTP konumu yalnızca bir SSH sunucusu gerektirir. SFTP formunda gösterilen genel anahtarı (ayrıca **Ayarlar, Sistem, SSH üzerinden VM Yedeği** altında ve `/config/ssh/id_ed25519.pub` dosyasında bulunur) hedef kullanıcının `~/.ssh/authorized_keys` dosyasına ekleyin.
- **Site dışı kopya:** BombVault yeni anlık görüntüleri, etki alanının depolandığı konuma ek olarak, en iyi çaba temelinde `restic copy` ile kopyalar. Her etki alanının Ayarlar, Zamanlamalar'da kendi kopyalama zamanlaması ve satırında **Şimdi kopyala** vardır.
- **Etki alanı başına birden çok kopyalama konumu:** **Kopyalama hedefleri** altında istediğiniz kadar konum işaretleyin; her biri etki alanının zamanlamasıyla kopyalar.
- **Saklama, sınırlar, depolama sınıfı ve büyüme bütçesi konuma aittir** ve konumun ayrıntılarında ayarlanır. Bir konumun saklama kuralları oradaki her depoya uygulanır, böylece site dışı bir konum kopyaları bir arşiv olarak daha uzun tutabilir; her kuralı sıfır olan bir konum hiçbir şeyi kırpmaz.
- **Soğuk ve arşiv depolama sınıfı (S3):** bir S3 konumu için geri yüklenebilir bir katman seçin (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone uzak konumları sınıflarını rclone yapılandırmasında ayarlar.
- **Uzak bir konumda depolanan etki alanı:** bkz. [Uzak bir konumda depolanan etki alanı](offsite-recovery.md#remote-primary-repositories).

## Taşınabilir ayarlar (dışa ve içe aktarma) {#portable-settings-export-and-import}

Ayarlar sayfasındaki **Ayarları dışa ve içe aktar** kartı, tüm BombVault yapılandırmanızı (etki alanı ayarları, depolama konumları, zamanlamalar, bildirimler) başka bir örnekte içe aktarabileceğiniz taşınabilir bir JSON dosyasına yazar, böylece yeni bir makineye taşınmak ya da bir kurulumu klonlamak her şeyi elle yeniden girmek anlamına gelmez. İçe aktarma bir önizleme gösterir ve onay ister ve yedekleme verilerinize ya da geçmişinize asla dokunmaz. Önizleme dosyadaki depolama konumlarını ve kimlik bilgileriyle birlikte kimlik bilgisi kümelerini sayar; konum içermeyen eski bir dosyada BombVault konumları içe aktarılan ayarlardan oluşturur.

!!! warning "Dışa aktarma kimlik bilgileri içerebilir"
    Konumlarınızın ve bildirimlerinizin kimlik bilgilerini dosyaya dahil edip etmeyeceğinizi siz seçersiniz. Kimlik bilgileri dahilken, dışa aktarma kurtarma kitiniz kadar hassastır, bu nedenle onu güvenli bir yerde saklayın. Onlarsız, dosya yalnızca gizli olmayan ayarları tutar.
