# Yedekler ve ögeler

BombVault'un yedeklediği her şey **Yedekler** altındaki tek bir listede durur: konteynerler, VM'ler, klasör kümeleri, flash sürücü, ZFS veri kümeleri ve öz yedek. Her biri kendi sayfası olan bir **öge**dir. Bu sayfa seni listede, öge sayfasında ve üç sekmesinde, bir ögenin sahip olabileceği ayarlarda dolaştırır.

![Konteynerler kutucuğu seçili Yedekler listesi: her satır son çalıştırmalarını bir şerit ve tek bir durum olarak gösterir.](assets/screenshots/backups/list-dark.png#only-dark)
![Konteynerler kutucuğu seçili Yedekler listesi: her satır son çalıştırmalarını bir şerit ve tek bir durum olarak gösterir.](assets/screenshots/backups/list-light.png#only-light)

## Her bölüm neyi saklar {#what-is-saved}

| Bölüm | Ne saklanır |
|---|---|
| **Konteynerler** | Appdata klasörü ve konteyner tanımı: image, değişkenler, portlar, etiketler ve birimler. Konteynerin **Ayarlar** sekmesindeki **Yedeklenecek klasörler** kartı, yedeğin tam olarak hangi klasörleri kapsadığını işaretler, yolların canlı sayısı ve dışarıda bıraktıklarının listesiyle. Bir konteynerin Host Data dışında tuttuğu veri, örneğin bir TrueNAS ixVolume'unda ya da `/opt` altındaki bir Compose projesinde, yedeklenmez ve çalıştırma dışarıda bıraktığı klasörleri söyler. Çalıştırma, Host Data içinde şu an bulunmayan bir klasörü de söyler. `/etc/localtime` ya da Docker soketi gibi sistem yolları veri sayılmaz. |
| **VM'ler** | Disk image'leri, XML tanımı ve UEFI NVRAM, düzgün kapatma ya da canlı anlık görüntüyle, SSH üzerinden. Bkz. [VM yedekleri](vm-backup-ssh-setup.md). |
| **Flash** | Yalnızca Unraid. Tüm USB flash (`/boot`): işletim sistemi, lisans, dizi yapılandırması, paylaşımlar, ağ ve eklenti yapılandırması. |
| **Öz yedek** | BombVault'un kendi `/config`'i: ayar veritabanı, hedeflerin oturum açma bilgileri, libvirt SSH anahtar çifti, ZFS sunucularının anahtarı ile sabitlenmiş ana makine anahtarları ve sertifikalar, veritabanı yazma ortasında hiç yakalanmasın diye SQLite `VACUUM INTO` ile anlık görüntüsü alınır. |
| **Klasörler** | Sunucudaki herhangi bir klasör, bir paylaşım, belgelerin ya da bir fotoğraf kitaplığı gibi, bir klasör kümesi olarak, isteğe bağlı hariç tutma desenleriyle. Klasör kümelerinin de diğer her şey gibi zamanlamaları, saklaması, site dışı kopyaları, denetimleri ve geri yükleme tatbikatları vardır. |
| **ZFS veri kümeleri** | Bir veri kümesi ve altındaki her veri kümesi, hepsi aynı andan olsun diye tek bir ZFS anlık görüntüsünden okunur ve restic ile bir klasör gibi saklanır. Bir ZFS ögesi ikinci bir ZFS sunucusunda blok blok aynı bir replika da tutabilir. Bkz. [ZFS veri kümeleri](zfs-datasets.md). |

Her yedek restic'ten geçer, bu yüzden artımlıdır ve tekilleştirilir, büyük VM diskleri bile depoyu şişirmez. BombVault kendi konteynerini hiçbir zaman yedeklemez, dolayısıyla durdurmaz da. Bir çalıştırma güvenle durdurulabilir, çünkü restic anlık görüntüsünü en son yazar, yani yarıda kesilen bir çalıştırma başvurulmayan veri bırakır ama anlık görüntü bırakmaz. Bir çalıştırmanın ortasında yeniden başlamanın bıraktığı bir kilit temizlenir ve çalıştırma bir kez daha denenir, bunu yalnızca kilit on dakika boyunca yenilenmeden kaldıktan sonra yapar.

## Liste {#list}

Listenin üstündeki kutucuklar bir tür seçer, örneğin **Konteynerler** ya da **ZFS**, ve her kutucuk ögelerini sayar. Bir kutucuk seçer, sayfanın geri kalanını gizlemez.

Her satır ögenin son on dört çalıştırmasını küçük kareler ve tek bir durum olarak gösterir, örneğin **Korumalı**, **Korumasız**, **Ertelendi** ya da **Çoğaltılıyor**. Hakkındaki geri kalan her şey adının yanındaki bilgi balonundadır ve işaretçi satırın üstünde dururken o metin satırın içinde durur. Bir konteynerin adının yanındaki bir işaret, son yedeğinden beri başka bir image, başka portlar, değişkenler ya da birimlerle yeniden oluşturulduğunu söyler. Sonraki yedekle kaybolur.

Listenin üstündeki üç düğme onu daraltır:

- **Filtreler**: **Zamanlama**'ya göre (**Tümü**, **Zamanlanmış**, **Duraklatıldı**), **Yedekleme**'ye göre (**Yedeklendi**, **Hiç yedeklenmedi**) ve **Konteynerler ve VM'ler** için **Kurulu** ya da **Kurulu değil**.
- **Sırala**: ada, duruma, IP'ye, son yedeğe, boyuta ya da zamanlamaya göre. IP dışındaki her alan iki yönde de sıralanır, örneğin **Boyut, en büyük önce** ve **Boyut, en küçük önce**.
- **Ara**: ada, image'e ya da IP'ye göre.

Artık kurulu olmayan ama hâlâ yedekleri olan ögeler **Kurulu değil (yalnızca yedekler)** altında ayrı durur. Onları geri yükle ya da yer açmak için yedeklerini sil.

Sayfanın ana eylemi sağ altta yüzer: **Şimdi tümünü yedekle**, açık olan her bölüm üzerinde bir geçiş başlatır. **Diğer** gerisini barındırır:

- **Seç** bir seçim başlatır, aşağıya bak.
- **Yedekleri keşfet**, hedeflerin tuttuklarını okur ve ögeleri bunlardan yeniden kurar, örneğin temiz bir kurulumdan sonra. Kendine ait verisi olmayan bir konteynerin anlık görüntüsü yoktur ve deponun yanında saklanan tanımdan geri gelir. Yeniden kurulan bölümlerin site dışı kopyaları, bunların yedeklerini bu sunucunun kendi yedekleri olarak alana kadar bekler, bkz. [Yeniden kurulumdan sonra](offsite-recovery.md#after-a-rebuild).
- **Bu sunucuyla ZFS bağlantısı**, ZFS bölümünün SSH testini açar, bkz. [ZFS veri kümeleri](zfs-datasets.md#requirements).
- ZFS kutucuğu seçiliyken **Şimdi tümünü çoğalt**, her ZFS ögesinin replikasını gönderir.

![Aynı liste telefonda.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Aynı liste telefonda.](assets/screenshots/backups/list-phone-light.png#only-light)

## Birden çok öge seçmek {#select}

Satırları dosya yöneticisinde dosya seçer gibi seç: Ctrl+tıklama, Shift+tıklama, Ctrl+A ya da telefonda uzun basma. Seçilen satır simgesi yerine bir onay işareti gösterir ve listenin üstündeki bir çubuk onları sayar ve **Seçilenleri yedekle**, **Zamanlamayı duraklat** ve **Zamanlamayı sürdür**, **Tümünü seç** ve **Seçimi temizle**'yi sunar. Her satırın ayrıca kendi eylemleriyle bir sağ tık menüsü vardır.

Bir seçim sunucuda, öge öge, yedeklenir, bu yüzden sekmeyi kapatsan da sürer. Birden çok ögeyi birlikte geri yüklemek için [Geri yükleme sayfasını](restore.md#several) kullan.

![Listenin üstündeki çubukla iki seçili konteyner.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Listenin üstündeki çubukla iki seçili konteyner.](assets/screenshots/backups/bulk-light.png#only-light)

## Bir klasör ya da ZFS veri kümesi eklemek {#add}

Konteynerler, VM'ler ve flash sürücü, bölümleri **Ayarlar, Genel** altında açılır açılmaz kendiliğinden görünür. Klasörleri ve ZFS veri kümelerini sen seçersin, çünkü BombVault hangilerini kastettiğini tahmin edemez. **Ekle**, **BombVault ayrıca neyi yedeklesin?**'i açar:

- **Tek klasör**, BombVault'un bulduğu paylaşımları listeler ve Host Data altındaki başka herhangi bir klasörü alır. Klasör kümesine `Documents` gibi bir ad ver. Ad ilk yedekten sonra sabitlenir, çünkü anlık görüntüler onun altında saklanır.
- **Bir ZFS veri kümesi**, bu sunucunun veri kümelerini listeler. Yedeklemek istediğin şeyin en üstündekini seç, bkz. [Ögeler ve alt veri kümeleri](zfs-datasets.md#items-and-children).
- **Sistem yapılandırması**, Host Data'sı `/etc`'ye ulaşan, Unraid olmayan bir ana makinede görünür. Kaydetmeden önce gözden geçirdiğin bir başlangıç `/etc` klasör kümesidir.

## Bir ögeyi dışarıda bırakmak {#dont-back-up}

Bilerek dışarıda bıraktığın bir öge, örneğin bir test konteyneri, **Yedeklenmesin** olarak işaretlenebilir. Bu durumda korunmuyor sayılmaz, **Hariç tutmayı kaldır** onu geri getirir. **Zamanlamayı duraklat** daha yumuşak yoldur: öge zamanlamadan ve tam yedeklemeden çıkar, gri bir **Zamanlama duraklatıldı** rozeti taşır ve **Şimdi yedekle** yine çalışır. Ögenin Ayarlar sekmesindeki **Zamanlamada** ile aynı anahtarı ayarlar.

## Bir ögenin sayfası {#entry-page}

Bir satıra tıklamak ögenin sayfasını açar. Başlık ögeyi ve durumunu tek satırda söyler, örneğin **Son yedek 3 saat önce** ya da **Ertelendi, sonraki deneme 03:15'te**, ve eylemlerini barındırır: **Şimdi yedekle**, **Geri yükle** ve **Diğer** altında nadir olanlar, **En yeni yedeği dışa aktar**, **Konteyneri dışa aktar** ya da **VM'yi dışa aktar**, **Önceki bir ögeye bağla**, **Tüm yedekleri sil** ve **Ögeyi kaldır** gibi. Simgenin solundaki bir geri düğmesi listeye götürür. Bir yedekleme çalışırken ilerlemesinin yanında **Yedeklemeyi iptal et** durur ve iptal edilen bir çalıştırma başarısız değil iptal edilmiş olarak kaydedilir.

Sayfanın üç sekmesi vardır: **Yedekler**, **Ayarlar** ve **Doğrulama**.

### Yedekler sekmesi {#backups-tab}

Bu sekme ögenin anlık görüntülerini en yeni önce, gün başına bir çubukla listeler. Birden çok yerde duran bir yedek bir kez görünür, onu tutan her hedefin işaretiyle. Bir anlık görüntüye tıklamak onu geri yükler, bir anlık görüntü içindeki bir yere tıklamak tam oradan geri yükler.

- **Hedefler**, bu ögenin yedeklerinin bulunduğu her yeri ve kaç tane olduğunu adlandırır. Site dışı yerler yalnızca **Denetim**'e bastığında okunur, çünkü bulut depolamayı listelemek biraz sürer. Ögenin artık yedeklemediği bir yer **Yalnızca eski yedekler** olarak durur ve yine de ondan geri yüklenebilir.
- Her anlık görüntünün bir menüsü vardır: **Etiket ekle** ya da **Etiketi değiştir**, gelen, giden ya da değişen dosyaları listeleyen **Başka bir yedekle karşılaştır**, tar, zip ya da kendi restic deposu olarak **Dışa aktar** ve yalnızca tek bir yerde silme.
- Bir konteynerin **Veritabanı dökümleri** kendi listesinde durur, indirme, klasöre kaydetme ve içe aktarma ile.

Kaç anlık görüntünün kaldığını hedef belirler, bkz. [Ne kadar saklanır](offsite-recovery.md#keep).

![Bir konteynerin Yedekler sekmesi: gün başına bir çubuk ve her anlık görüntüyü tutan hedefler.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Bir konteynerin Yedekler sekmesi: gün başına bir çubuk ve her anlık görüntüyü tutan hedefler.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Ayarlar sekmesi {#settings-tab}

Her öge **Zamanlama ve hedef** ile başlar:

- **Zamanlamada**, ögeyi zamanlanmış çalıştırmalara alır ya da çıkarır.
- **Kendi planı**, bu tek ögeye kendi planını verir, bkz. [Tek bir öge için kendi plan](schedules.md#own-plan). Onsuz bölümünün planında ya da tam yedeklemede çalışır.
- Bir düğme sırası, **Yerel** ve site dışı hedef başına bir düğme, yedeklerin nereye gittiğini söyler. **Varsayılanı izler**, ögenin bölümünün gittiği yere gittiği anlamına gelir ve ona tıklamak **Bölüme göre hedefler**'i açar. Bkz. [Öge başına yerleşim](offsite-recovery.md#placement).
- **Depo**, yedeklerin yazıldığı yeri adlandırır (**Hedef**). O yer ilk yedekten itibaren sabittir, kopyalar ise her zaman değişebilir.

Aşağıdaki kartlar ögenin türüne bağlıdır:

| Tür | Kartlar |
|---|---|
| Konteyner | **Yedeklenecek klasörler** (hangi eşlenmiş klasörlerin yedeklendiği, **Önbellek klasörlerini atla (CACHEDIR.TAG)** ile), **Hariç tutma desenleri**, **Diğer konteynerleri durdur**, **Öncesi ve sonrası komutları**, **Veritabanı**, **Uygulama meşgulken**, **Başarılı bir yedeklemeden sonra güncelle**, **Durdurma yerine anlık görüntü**, **Önceki adlar** |
| VM | **Yöntem** (**Düzgün (kapatma)** ya da **Canlı anlık görüntü**) ve **Yalnızca değişen bloklar** içeren **VM**, bkz. [VM yedekleme kılavuzu](vm-backup-ssh-setup.md#per-vm-method) |
| Klasör kümesi | **Klasör** ve **Hariç tutma desenleri** |
| Flash sürücü | Her flash yedeğini seçtiğin bir klasöre düz bir `.zip` olarak yazan **Flash ZIP dışa aktarma**. Tek adımda yazar, böylece bir eşitleme aracı hiçbir zaman yarım bir dosya görmez. Olması gerekenden az şey tutan bir flash yedeği son ZIP'i yerinde bırakır ve dışa aktarmanın çalıştırması bunu söyler. |
| ZFS veri kümesi | alt veri kümeleri, durdurulacak konteynerler ve anlık görüntü etrafındaki komutlar, **Replika** ve **Anlık görüntü erişimini dene**, bkz. [ZFS veri kümeleri](zfs-datasets.md) |

Ayarlar sen değiştirdikçe kendiliğinden kaydedilir.

![Bir konteynerin Ayarlar sekmesi, ilk kartında Kendi planı ile.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Bir konteynerin Ayarlar sekmesi, ilk kartında Kendi planı ile.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Doğrulama sekmesi {#verify-tab}

- **Denetimler**, ilk yedekten sonra gelen geri yükleme denetimini gösterir. **Geri yüklemeyi denetle** onu en yeni yedek üzerinde yeniden çalıştırır: BombVault 100 dosyaya ve 256 MiB'a kadar bir örneği geçici bir klasöre geri yükler ve restic'in her dosyayı karmalarına karşı geri okumasını sağlar.
- Bir konteynerde **Başlatma testi**, en yeni yedeği yalıtılmış bir kopyaya geri yükler ve başlatır: `bombvault-test-` ile başlayan bir ad, yayınlanmış port ve LAN'a yolu olmayan kendi iç Docker ağı, 1 CPU ve 2 GiB bellek. Kopya sağlıklı bildirirse, ilk açık portunda yanıt verirse ya da ikisi de yoksa çalışmaya devam ederse geçer. Orijinale hiçbir zaman dokunulmaz ve kopya, ağı ve verisi sonra, bir testin ortasında yeniden başlatmadan sonra da, kaldırılır. Ana makine ağındaki, ayrıcalıklı, aygıtlı ve başka bir konteynere ihtiyaç duyan konteynerler sınanamaz olarak gösterilir. **Ayarlar, Doğrulama, Otomatik denetimler** altındaki **Başlatma testi**, çalıştırma başına bir konteyneri sınar, en uzun süredir sınanmamış olanı.
- **Anormallikler**, bu ögenin kendi duyarlılığını ve bildirim düzeyini ayarlar ve açık bulgularını gösterir.
- **Klasöre göre boyut**, en yeni yedekte yeri neyin kapladığını ve son yedeğin bunun ne kadarını getirdiğini, her seferinde bir düzey, gösterir.
- **Son çalışmalar**, son çalıştırmaları başlangıcı, bitişi ve süresiyle listeler. Tıklamak ayrıntıları gösterir.

![Bir konteynerin Doğrulama sekmesi, Klasöre göre boyut açık.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Bir konteynerin Doğrulama sekmesi, Klasöre göre boyut açık.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Hariç tutma desenleri {#excludes}

**Hariç tutma desenleri** kartı satır başına bir desen alır. Yolları konteynerin içinde gördüğün gibi yaz, örneğin `/config/Library/Application Support/Plex Media Server/Cache`, ya da her derinlikte eşleşen `.git` gibi çıplak bir ad. Bir klasör kümesinde yolu ana makinenin bildiği gibi yaz, örneğin `/mnt/user/data/cache`. Alanın altındaki bir önizleme her satırın neyi gösterdiğini gösterir ve bir satır hiçbir şeyle eşleşmiyorsa ya da her şeyi hariç tutacaksa uyarır. `/` ya da `*` olan bir satır veya yedeklenen bir klasörün tamamını adlandıran bir satır yok sayılır, bu yüzden bir hariç tutma bir yedeği hiçbir zaman boşaltamaz. Bir klasörün tamamını dışarıda bırakmak için bunun yerine **Yedeklenecek klasörler** altında işaretini kaldır.

Plex, Jellyfin, Sonarr, Radarr, Immich ve Nextcloud gibi tanınmış image'ler için kart, uygulamanın kendi kendine yeniden doldurduğu klasörleri, örneğin önbellekleri, günlükleri ve küçük resimleri sunar. **Seçilenleri hariç tut**'a basana kadar hiçbir şey hariç tutulmaz.

Aynı kart, bir klasör taramasının göremeyeceği uyarıları da taşır. Örneğin Immich albümlerini, yüzleri ve tarihleri ayrı bir konteynerdeki bir PostgreSQL veritabanında tutar, bu yüzden tek başına Immich konteynerinin dosya yedeği, bunların hiçbiri olmadan fotoğrafları geri getirir ve geri yükleme işe yaramış gibi görünür. Veritabanı konteynerini de yedekle.

## Veritabanı dökümleri {#database-dumps}

BombVault'un tanıdığı PostgreSQL, MySQL ve MariaDB konteynerleri her yedeklemeden önce çalışan sunucudan, dosyaların yanında kendi geri yükleme noktasına dökülür. Bu, resmi image'leri, PostGIS'i, TimescaleDB'yi, pgvector'ü, pgautoupgrade'i, Immich'in veritabanı image'lerini, linuxserver, yobasystems ve jc21 MariaDB image'lerini ve Oracle'ın mysql-server'ını kapsar. Yalnızca veritabanına benzeyen bir konteyner aynı seçeneği alır, sen seçene kadar kapalı olarak.

Döküm doğrudan depoya akar ve hiçbir zaman bir diske yazılmaz. Kimlik bilgileri konteynerin kendi değişkenlerinden, `*_FILE` gizli bilgileri dahil, gelir ve ondan hiç çıkmaz. Konteynerin Ayarlar sekmesindeki **Veritabanı** kartı, veri klasörünün durdurulmuşken mi kaydedildiğini, çalışırken mi kopyalandığını yoksa kaydedilmediğini söyler ve bu konteyner için dökümü açar ya da kapatır. `bombvault.dbdump=false` etiketi aynısını yapar ve karttan önce gelir. **Ayarlar, Genel** altındaki **Otomatik veritabanı dökümleri** hepsi için açıp kapatır.

Başarısız bir döküm çevresindeki yedeği başarısız kılmaz. Nedeniyle birlikte kendi başarısız çalıştırması olarak görünür ve bir bildirim gönderir. Dökümler hiçbir zaman kendiliğinden geri yüklenmez: birini düz ya da sıkıştırılmış olarak indir, bir klasöre kaydet ya da **Yedekler** sekmesinde tek tıkla yeni başlatılmış bir veritabanına içe aktar. BombVault olmadan bir dökümü geri almak için bkz. [Bir veritabanı dökümünü geri almak](offsite-recovery.md#database-dumps). Biri başarısız olduğunda bkz. [Bir veritabanı dökümü başarısız oldu](troubleshooting.md#a-database-dump-failed).

## Komutlar ve diğer konteynerler {#commands}

- **Öncesi ve sonrası komutları** konteynerin içinde `sh -c` ile çalışır, örneğin bir önbelleği diske yazmak için. Yedek öncesi başarısız bir komut yedeği iptal eder. 75 koduyla çıkan bir yedek öncesi komut, bunun yerine zamanlanmış bir yedeği erteler, aşağıya bak.
- **Diğer konteynerleri durdur**, bu konteyner yedeklenirken durdurulacak konteynerleri, örneğin veritabanını, adlandırır. Sonra Compose `depends_on` sırasıyla yeniden başlarlar ve varsayılan olarak her biri, sonraki başlamadan önce sağlıklı bildirmelidir. Bekleme ve zaman aşımı **Ayarlar, Zamanlama** altındaki **Sıra** kartındaki **Yedeklemeden sonra yeniden başlatma**'dadır.
- Duran bir konteynerin ağ, PID ya da IPC ad alanını `--net=container:` veya Compose'un `network_mode: service:` ayarıyla paylaşan bir konteyner, listede olmasa bile onunla birlikte durur ve başlar, ve katıldığı konteynerden sonra başlar. Yeniden başlatılan bir konteyner yeni bir ağ alır, eski ağda çalışmaya devam eden bir konteynerin ise yalnızca loopback'i kalırdı. Listenin kendisi senin ayarladığın gibi kalır. Böyle bir konteyner durdurulamadıysa, çalıştırma onun yeniden başlatılana kadar ağı olmadığını söyler. Aynısı, kendini hiç durdurmayan ve listeye konamayan BombVault'un kendi konteyneri için de geçerlidir.
- Sonrasında yeniden çalışmayan bir konteyner, yedeğin kendisi başarısız olduğunda da, çalıştırmada ve bir bildirimde adıyla belirtilir.

## Uygulama meşgulken {#app-busy}

Bir yedek konteynerini durdurur, bu yüzden zamanlanmış bir yedek biri uygulamayı kullanırken bekleyip kendiliğinden yeniden deneyebilir. Bir konteynerin Ayarlar sekmesindeki **Uygulama meşgulken** kartı bunu kurar:

- **Etkinlik denetimi**, zamanlanmış bir yedekten önce Plex'e, Jellyfin'e ya da Emby'ye birinin izleyip izlemediğini sorar. Plex kendi `Preferences.xml` dosyasındaki belirteçle sorulur, yani girecek bir şey yoktur. Jellyfin ve Emby, sunucunun panelinde API keys altında oluşturulan bir **API anahtarı** alır.
- **Duraklatılmış oturumları şu süreden sonra yok say (dk)**, gece için duraklatılmış bir filmin saymayı bırakmasını sağlar.
- **Sunucu yanıt vermezse yedekle**, sunucuya sorulamadığında ne olacağına karar verir: açıksa yedek çalışır ve günlük bunu söyler. Kapalıysa birisi izliyormuş gibi bekler.
- **Konteyner veri gönderirken bekle**, bir oyun sunucusu ya da bir indirici gibi böyle bir denetimi olmayan uygulamalar içindir. Konteyner, son **Sessiz kalma süresi (dk)** dakika içindeki herhangi bir okumada **Şu hızdan itibaren meşgul (KiB/s)**'ten fazla gönderdiği sürece yedek bekler.
- **Yeniden deneme aralığı (dk)**, **Şu saatler arasında dene** ile **Yalnızca bir zaman aralığında dene** ve **En fazla bekle (saat)** denemelerin nasıl gideceğini belirler.
- **Bekleme sonunda** ya pes eder, bu yedeği **İlgi bekliyor** altında **Yeniden dene** ile gösterir, ya da **Yine de yedekle**.

Bekleyen bir yedek kehribar renkte **Ertelendi** olarak kaydedilir ve çalıştırmanın geri kalanı sürer. Genel bakış, liste ve öge onu sonraki denemeyle gösterir, gecikme uyarısı ve koruma durumu da süresi dolana kadar onu bekler. Ertelendiğinde bir, bittiğinde bir bildirim alırsın, her deneme için değil. Bir deneme başka bir yedek çalışırken bekler ve bir bekleme yeniden başlatmadan sonra devam eder. Kendi başlattığın bir yedek, **Şimdi yedekle**, **Yeniden dene**, bir seçim, API ya da MCP ile olsun, her zaman hemen çalışır ve geçen biri beklemekte olan bir beklemeyi sonlandırır.

Konteynerin Ayarlar sekmesinde eski, daha basit bir yol da vardır: **En fazla bekle (saat)** ile **Uygulama boşta kalana kadar bekle**. Yedek, uygulama boşta olur olmaz başlar, boşta olmayı **Ayarlar, Zamanlama** altındaki **Yedek öncesi boşta kalma** kartı tanımlar. Aynı çalıştırmada sırası gelen bir Compose stack'inin üyeleri birlikte bekler ve bekleyen yedek hiçbir kilit tutmaz, bu yüzden diğer konteynerler ilerler.

![Etkinlik denetimi kurulmuş bir Plex konteynerinin Uygulama meşgulken kartı.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Etkinlik denetimi kurulmuş bir Plex konteynerinin Uygulama meşgulken kartı.](assets/screenshots/backups/app-busy-light.png#only-light)

### Örnek: Plex, biri izlerken bekler {#example-plex}

Tam yedekleme her gece 03:00'te çalışır. Hiç kimse Plex yedeği için kapatıldığı için bir filmin durduğunu görmemeli, ama yedek sabaha kadar yine de alınmalı.

1. **Yedekler**'i, sonra `plex` ögesini, sonra **Ayarlar** sekmesini aç.
2. **Uygulama meşgulken** içinde **Etkinlik denetimi**'ni Plex yap. Yazacak bir şey yok.
3. **Duraklatılmış oturumları şu süreden sonra yok say (dk)**'yı `30` yap, böylece gece yarısı duraklatılan bir film yarım saat geçince saymayı bırakır.
4. **Sunucu yanıt vermezse yedekle**'yi açık bırak.
5. **Yeniden deneme aralığı (dk)**'yı `15` yap.
6. **Yalnızca bir zaman aralığında dene**'yi aç ve **Şu saatler arasında dene**'yi `22:00` ile `06:00` yap.
7. **En fazla bekle (saat)**'i `3` ve **Bekleme sonunda**'yı **Yine de yedekle** yap.

03:00'te biri hâlâ izlerken bir gecede olanlar: BombVault Plex'e sorar, oynayan bir oturum bulur ve yedeği sonraki deneme 03:15'te olacak şekilde **Ertelendi** olarak kaydeder. Çalıştırmanın diğer konteynerleri her zamanki gibi yedeklenir. Film 04:10'da bittiğinde 04:15'teki deneme Plex'i boşta bulur ve yedekler, bekleme tek bir bildirimle biter. Film 06:00'da hâlâ sürseydi bekleme sonuna gelirdi ve Plex yine de yedeklenirdi, böylece yedekleri hiçbir zaman üç saatten fazla geri kalmaz.

Kendi denetimin için, uygulama meşgulken **Yedekleme öncesi komut**'un 75 koduyla çıkmasını sağla, örneğin `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Daha fazla belirti ve çözüm [Sorun Giderme](troubleshooting.md#deferred-backups) içindedir.

## Düz dışa aktarmalar {#exports}

restic motor olarak kalır, ama bir dışa aktarma sana ne BombVault'a ne de restic'e ihtiyaç duyan bir kopya verir.

- Bir ögenin sayfasında **Diğer** altındaki **Konteyneri dışa aktar** ve **VM'yi dışa aktar**, bir kopyayı şu an olduğu haliyle, deponun yanına yazar: konteynerin yedeklenecek klasörleri ve Unraid şablonu ya da VM'in disk image'leri ve tanımı, `virsh define` ile geri yüklenebilir.
- Bir anlık görüntüdeki **Dışa aktar**, saklanmış bir yedeği sunucudaki dışa aktarma klasörüne tar, zip ya da kendi restic deposu olarak yazar ve tek bir anlık görüntüyü ya da hepsini alabilir. Bir flash yedeği ve bir öz yedek anahtarlar tutar, bu yüzden bunları dışa aktarmak bir giriş parolası ve örneğin kendisinde bir oturum açma gerektirir.

Dışa aktarmalar varsayılan olarak düz dosyalardır. **Ayarlar, Sistem** altındaki **Dışa aktarmaları age ile şifrele**, onları, flash ZIP'i dahil, her biri bir age açık anahtarı ya da bir SSH açık anahtarı olan, listelediğin alıcılar için mühürler. Onları sonra sunucu dışında eşleşen özel anahtarla çözersin. Şifreleme açıkken ve geçerli alıcı yokken, bir dışa aktarma hiçbir zaman düz dosya yazmak yerine açık bir hatayla başarısız olur.

## Bir çalıştırma sana ne söyler {#run-notes}

- **Zamanlama.** Her çalıştırma başlangıcı, bitişi ve süresini okur ve her ögenin kendi **Son çalışmalar**'ı vardır.
- **Bir yedek neden yavaştı.** Bir yedekleme çalışırken BombVault CPU'nun, disklerin ve ağın ne kadar meşgul olduğunu izler. Bir yedek olağandan çok daha uzun sürdüğünde ve tek bir şey açıkça sınırındayken çalıştırma bunu söyler, örneğin "Hedef disk disk1 %98 meşguldü". Aksi halde hiçbir şey söylemez.
- **Denetimlerin ve budamaların ilerlemesi.** Etkinlik günlüğü ve **Yedekleri denetle** kartı restic'in ne kadar saydığını gösterir, örneğin *47 paketten 12'si*, tahmin için yeterli veri olunca o adımda kalan süreyle.
- **Okunamayan dosyalar.** Tamamlanan ama bir şeyi dışarıda bırakan bir yedek satırını turuncuya çevirir ve neyin dışarıda kaldığını söyler.
