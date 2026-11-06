# Site dışı ve kurtarma

!!! note "Yeniden kurulumdan sonra site dışı kopyalar bekler"
    Acil durum sihirbazı ya da **Yedekleri keşfet** ögeleri eski ayarlar olmadan yeniden kurduğunda, o bölümlerin site dışı kopyaları, sen onların yedeklerini bu sunucunun kendi yedekleri olarak alana kadar bekler. Bkz. [Yeniden kurulumdan sonra](#after-a-rebuild).

Yerel yedekler seni kaybolmuş bir konteynerden ya da kötü bir güncellemeden korur. Site dışı kopyalar ve denenmiş bir kurtarma kiti seni tüm makineden, fidye yazılımından ya da bir yangından korur. Bu sayfa hedefleri ve site dışı kopyaları, bir kopyayı kurcalamaya karşı korumayı, geri yükleyebildiğini kanıtlamayı, birkaç sunucuyu eşleştirmeyi ve BombVault'un kendisi gittiğinde her şeyi geri almayı kapsar.

## Site dışı kopyalar {#off-site-copies}

Hızlı yerel yedeği tut ve tesis dışında bir ya da daha fazla hedef ekle. BombVault yeni anlık görüntüleri oraya elinden geleni yaparak `restic copy` ile kopyalar, bu yüzden site dışında bir aksama yerel yedeği hiçbir zaman başarısız kılmaz. Bu düzende her bölüm önce kendi klasörüne yazar ve site dışı depo bir kopyadır. Kendi klasörün bu sunucuda olmak zorunda değil: S3 ya da bir rest-server gibi bir hedefte tutmak için bkz. [Bir bölümün kendi klasörü](#own-folder).

- **Bölüm başına birden çok hedef.** Her bölüm (konteynerler, VM'ler, flash, öz yedek, klasörler ve ZFS veri kümeleri) aynı anda birkaç site dışı hedefe kopyalayabilir, böylece örneğin bir arkadaşının makinesindeki bir rest-server'ı ve bir S3 bucket'ını paralel tutabilirsin. Hedefler sayfasındaki **Bölüme göre hedefler** kartı bir bölümden bir hedefe giden her satırı açıp kapatır.
- **Kopyalar ne zaman çalışır.** Bir hedefin **Kopyala** kartı **Ne zaman kopyalanır**'ı ayarlar: **Her yedekten sonra** ya da anlık görüntüleri toplayıp belirli zamanlarda kopyalayan kendi planında. **Ayarlar, Zamanlama, Bölüm başına site dışı kopyalar** altında **İstisna** bir bölüme kendi kopyalama planını verir.
- **Site dışı kopyalar kartı.** **Ayarlar, Hedefler** altında site dışı kopyası olan her bölümün **Site dışı kopyalar** içinde bir satırı vardır, **Bağlantıyı test et**, istek üzerine kopya başlatan **Şimdi çoğalt** ve **Kur** ya da **Düzenle** ile. Bölümün kopya ayarlarının geri kalanı oradan kendi penceresinde açılır.
- **Saklama** hedef başına **Ne kadar saklanır** altında ayarlanır, böylece site dışı kopyaları bir arşiv olarak daha uzun tutabilirsin. Bir bölümün site dışı alanındaki kopya, her diğer kopya gibi onu izler ve bir bölüm **Bölüme göre kurallar** altında kendi değerlerini tutabilir. Hepsi sıfır, site dışı anlık görüntüleri hiçbir zaman budamaz. Bkz. [Ne kadar saklanır](#keep).
- **Bant genişliği sınırları** (**Ayarlar, Hedefler** altında **Site dışı bant genişliği** ya da bir hedefin **Kopyala** kartı) kopyalar WAN'ını doldurmasın diye restic yükleme/indirme hızını sınırlar. restic'in aldığı birim olan KiB/s olarak girilirler ve her alan yazarken aynı hızı MB/s ve Mbit/s olarak gösterir: 2048 yaklaşık 2,0 MB/s (16,8 Mbit/s) okunur.
- **Sıkıştırma** **Kapalı**, **Otomatik** (restic'in varsayılanı) ya da **En yüksek**'tir, her hedefte ve bir bölüm için orada kendi kurallarında ayarlanır. Yedekler, kopyalar ve budama onunla yazar ve kurtarma kiti onu adlandırır, böylece düz restic aynı şekilde yazmaya devam edebilir.
- **Önce yayın.** Plex, Jellyfin ya da Emby gibi bir medya sunucusu yayın yaparken kopyalar daha düşük bir sınırla yüklenir ve yayın bittikten birkaç dakika sonra normale döner. Hedefler sayfasındaki **Yayın** kartında **Yayın sırasında site dışı kopyaları yavaşlat**, **Medya sunucuları** (sen seçene kadar image adında Plex, Jellyfin ya da Emby olan konteynerler), **Yayın eşiği (Mbit/s)**, **Yayın sırasında yükleme sınırı (KiB/s)** ve **Normale dönüş süresi (dk)** vardır. BombVault trafiği Docker'dan okur, bu yüzden ana makine ağındaki bir medya sunucusu ölçülemez. REST, S3, B2, Azure, Google Cloud, Swift ve HTTP üzerinden rclone bir kopyanın ortasında yavaşlar. SFTP ve yerel ya da bağlı klasörler daha düşük sınırı sonraki kopya adımında alır.
- **Soğuk depolama (S3).** Bir S3 hedefi restic'in hâlâ hemen okuyabildiği daha ucuz bir **Depolama sınıfı** kullanabilir, Standard-IA, One Zone-IA, Intelligent-Tiering ya da Glacier Instant Retrieval gibi. Glacier Flexible Retrieval ve Deep Archive dışarıda bırakılmıştır, çünkü restic onlardan geri yükleyemez. Bir rclone remote'u sınıfını rclone yapılandırmasında ayarlar.
- **Büyüme bütçesi.** Bir bölümün kopya penceresindeki **Büyüme bütçesi (GB, 0 = kapalı)**, depo o boyutu aştığında bir alarm verir. Hiçbir şeyi silmez, bu yüzden buradan hiçbir şeyin budamadığı bir append-only kopya için uygundur.
- Bir kopya çalışırken genel bakışın durum başlığı **Site dışı kopyalama sürüyor** der. Bu bir yüzde çubuğu değil, etkin bir göstergedir, çünkü `restic copy` makine tarafından okunabilir ilerleme sunmaz.

!!! note "Her yerden geri yükleme"
    Her öge yedeklerini, **Yedekler** sekmesinde, bir yedeğin durduğu tüm yerler boyunca tek bir zaman çizelgesi olarak listeler. B2'ye kopyalanmış bir yedek, onu tutan her yerin işaretiyle bir kez görünür. Bir geri yükleme ulaşabildiği ilk yeri alır, ögenin yazıldığı depodan başlayarak, ve anlık görüntü başına başka bir yer seçebilirsin. Site dışı hedefler yalnızca **Denetim**'e tıkladığında okunur, çünkü bulut depolamayı listelemek biraz sürer. Tek bir yerde silmek önce diğerlerini kontrol eder ve son kopya olup olmadığını söyler. Bkz. [Geri yükleme](restore.md).

## Hedefler {#destinations}

Bir hedef, yedeklerin gittiği bir yerdir: yerel bir klasör, bir paylaşım, kendi sunucun ya da bir bulut servisi. Bir kez kurulur ve sonra her bölüme hizmet eder. **Hedefler**'i ayar kutucuğundan aç ya da sabitlemek için orada **Kenar çubuğunda da göster**'i aç.

![Hedefler sayfası: hedeflerin listesi ve hangi bölümün nereye gittiğinin akışı.](assets/screenshots/storage/locations-dark.png#only-dark)
![Hedefler sayfası: hedeflerin listesi ve hangi bölümün nereye gittiğinin akışı.](assets/screenshots/storage/locations-light.png#only-light)

**Hedef ekle** dört adımlı bir pencere açar:

1. **Sağlayıcı.** Her servis logosuyla listelenir, pencerenin sağ üstünde bir arama vardır ve gruplar halindedir: bu sunucuya bağlı bir disk ya da paylaşım için **Yerel**, **NAS veya sunucu** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV ve diğerleri), **S3 hizmeti** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 ve diğerleri), kendi S3 sunucun (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Bulut depolama** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud ve rclone'un desteklediği geri kalanlar) ve grubunun alıcı sunucuları olan **Grubundan**. Her biri yedekler için ne kadar uygun olduğunu söyler: bulut sürücüleri çok istek altında yavaşlar, bu yüzden ilk yedek ve budama orada daha uzun sürer. Hiçbir kutucuğun kapsamadığı bir servis için **Kendi rclone yapılandırman** altındaki **Rclone** kutucuğu bir rclone.conf'tan yapıştırılan remote'ları alır.
2. **Oturum aç.** Alanlar servise bağlıdır: S3 için bir erişim anahtarı, WebDAV ve SMB için bir kullanıcı ve parola, iki adımlı girişin normalini engellediği yerde bir uygulama parolası, SFTP ve Storage Box için BombVault'un açık SSH anahtarı ya da bir tarayıcı aracılığıyla oturum açan servisler için bir belirteç. Bunlar için pencere tarayıcılı bir bilgisayarda çalıştırılacak bir `rclone authorize` komutu gösterir ve yazdırdığı belirteç alana girilir. Servis için zaten [kaydedilmiş bir oturum açma bilgin](#sign-ins) olan yerde **Kaydedilmiş oturum açma**, **Yeni anahtarlar** yerine onu seçer. **Bağlantıyı test et**, hiçbir şey kaydedilmeden önce oturum açmayı kontrol eder. Pencere sonra hedefteki klasörleri, yenisini oluşturmak için **Yeni klasör** ve servis bildiriyorsa boş alanla listeler. Boş bir klasör en güvenlisidir. BombVault içine bölüm başına bir klasör koyar.
3. **Koruma.** Pencere servisin neler yapabildiğini açıkça söyler. Append-only kipindeki bir rest-server silmeyi reddeder ve kurcalama testi bunu doğrular. Bir S3 bucket'ı sürümleme ve object lock ile eski sürümleri tutabilir, BombVault bunu henüz denetleyemez. Bir bulut sürücüsü silmeyi hiç reddedemez: sunucuya giren o kopyaya da girer. **Silmeye karşı koruma (append-only)**'yi yalnızca karşı taraf silmeyi gerçekten reddediyorsa aç. BombVault o zaman orada hiç budamaz. **Acil durum için**, kurtarma kitinin bu hedef için ne tuttuğunu söyler.
4. **Hangi bölümler için.** Buraya yedekleyen bölümleri seç. İlk kopya tüm geçmişi getirir. Kendi hedeflerini seçen ögeler yenisini kendiliğinden almaz ve pencere onları adlandırır.

![Hedef ekle: her sağlayıcının bir kutucuk olduğu ilk adım.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Hedef ekle: her sağlayıcının bir kutucuk olduğu ilk adım.](assets/screenshots/storage/add-providers-light.png#only-light)

![Hedef ekle: başarılı bir testten sonra oturum açma adımı.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Hedef ekle: başarılı bir testten sonra oturum açma adımı.](assets/screenshots/storage/add-signin-light.png#only-light)

Bazı servisler oturum açmada biraz daha fazlasını ister:

- **SFTP ve Storage Box.** **Anahtar ekle**, BombVault'un açık anahtarını **Hesap parolası** ile sunucuya koyar, parola o tek oturum açma için kullanılır ve hiçbir yerde saklanmaz. Elle yapma komutu altında gösterilir. Test, parmak iziyle **Sunucu anahtarı**'nı gösterir. BombVault onu hatırlar ve sunucu sonradan farklı bir tane gösterirse reddeder.
- Bir Cloudflare R2 nesne belirteci ya da kapsamı daraltılmış bir AWS anahtarı gibi **tek bir bucket için S3 anahtarları**, diğer bucket'ları listeleyemeyebilir. Bucket'ı **Bucket** alanına yaz, test ve klasör listesi o bucket'ı açar.
- **S3 bölgeleri ve adresleri.** Bölge, onu adlandıran bir uç noktadan okunur (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage kendi varsayılan bölgesiyle başlar ve IDrive e2 panelinin gösterdiği uç noktayı ister. R2 panelinin gösterdiği gibi bucket'ıyla yapıştırılan bir adres sunucuyu ve bucket'ı doldurur. Klasör tarayıcısından oluşturulan bir bucket formun adlandırdığı bölgeye gider.
- **Depolama sınıfı.** Seçici yalnızca servisin aldığı sınıfları sunar ve **Diğer ayarlar** yalnızca yeni bir bucket'ı şekillendiren seçenekleri gösterir.
- **Google Drive ve OneDrive.** Budanan veri, kotaya hâlâ sayılacağı çöp kutusuna ya da geri dönüşüm kutusuna gitmek yerine kalıcı silinir. Bir Google paylaşılan sürücüsü **Diğer ayarlar** altındadır.
- **Paylaşımlarını listelemeyen SMB sunucuları** **Paylaşım** altında birinin adını alır ve test o paylaşımı doğrudan açar.
- **FTP** **Şifreleme**'sini ister: **Yok**, **Açık TLS (FTPES)** ya da **Örtük TLS (FTPS, port 990)**.
- **OneDrive ve Zoho WorkDrive.** Bir oturum açma birkaç sürücü ya da çalışma alanı açtığında pencere yedeklerin hangisine gideceğini sorar.
- **Kendi kendine imzalı sertifikalı bir rest-server.** Sertifikayı PEM dosyası olarak BombVault'un veri klasörünün `certs` klasörüne koy (konteynerde `/config/certs`, Unraid'de `appdata/bombvault/certs`). Yedekler, kopyalar ve silmeye karşı koruma testi ona yeniden başlatmadan güvenir.

S3 servisleri restic'in kendi S3 arka ucundan geçer, bir depolama sınıfının ve object lock'un uygulanabilmesini sağlayan budur. Bir rest-server'a restic doğrudan ulaşır. Diğer her servis BombVault'un getirdiği rclone'dan geçer ve rclone yapılandırmasındaki bölümü gizli bilgiler gizlenmiş olarak hedefin sayfasındaki **rclone.conf** kartında görünür. Bir ayar dışa aktarması hedefleri taşır ve kimlik bilgileri dahilse oturum açma bilgilerini de.

Konteynerlerin, VM'lerin ya da klasörlerin yedeklerini zaten tutan bir hedefi kaydetmek, örneğin kaldırıp yeniden eklediğin bir tanesini, onu o bölümlere işaretlenmemiş bir kopya olarak ekler. Sen işaretleyene kadar oraya hiçbir şey kopyalanmaz, ama **Geri yükle** eski yedekleri hemen sunar.

Bu sunucudaki klasöre her yerde **Yerel** denir: hedef sayfalarında, akışta, bir ögede ve Geri yüklemede. Bu sunucuda birkaç klasör varsa her biri kendi adıyla anılır. Bir bölümün site dışı alanının kurduğu bir hedef kendi yerinden, adresinin son klasöründen ya da yoksa sunucusundan adlandırılır ve bu adla zaten bir hedef varsa bir numara alır. Her hedef **Bağlantı** kartının **Ad** alanında yeniden adlandırılabilir, birkaç bölümün paylaştığı bir bucket klasörü ve bir bölümün site dışı alanının tarif ettiği kopya da, ve verdiğin ad kalır. Aynı ada sahip iki hedefin yan yana durduğu yerde her biri sunucusunu ya da klasörünü taşır.

### Bir hedefin sayfası {#location-page}

Her hedefin kendi sayfası vardır. Yalnızca o hedef için geçerli kartları şu sırayla gösterir:

![Bir hedefin sayfası: Kendi klasörleri, Bağlantı ve Oturum açma.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Bir hedefin sayfası: Kendi klasörleri, Bağlantı ve Oturum açma.](assets/screenshots/storage/location-page-light.png#only-light)

- **Kendi klasörleri**: yedeklerini önce buraya yazan bölümler. Bu hedefin oturum açması, koruması ve sınırları onlar için geçerlidir. **Kendi klasörü olarak kullan** daha fazla bölümün kendi klasörünü buraya taşır, **Değiştir** birini yeniden uzağa taşır. Bkz. [Bir bölümün kendi klasörü](#own-folder).
- **Doğrudan buraya yedekleniyor**: ögeleri kendi klasörleri yerine doğrudan bu yere yedekleyen her bölüm ve kaç ögenin bunu yaptığı. Böyle bir doğrudan depo tüm ayarlarını bu hedeften alır. Artık hiçbir şeyin yedeklemediği biri bunu söyler ve **Kaldır** onu listeden alır. O yerdeki yedekler kalır. Bkz. [Doğrudan depolar](#direct-repositories).
- **Bağlantı**: **Ad**, **Kullanılabilir** anahtarı (kapalı, BombVault'un artık buraya yazmadığı ve buraya yedekleyen ögelerin başka bir yere düşmek yerine açık bir mesajla başarısız olduğu anlamına gelir), klasör, **Sıkıştırma**, **Depolama sınıfı** ve **Tesis dışı**. **Yerel** hedefte **Burada kendi klasörleri** altında her bölümün klasörünü listeler. **Bağlantıyı değiştir** yeni bir adresi ya da oturum açmayı test eder ve kaydeder. Başka yerler aynı kayıtlı oturum açma bilgisiyle giriş yaptığında pencere bunu söyler ve burada bir değişiklik paylaşılanı yeniden yazmak yerine bu hedefe kendi oturum açma bilgisini verir.
- **Oturum açma**: hangi kayıtlı oturum açma bilgisini kullandığı (**Şununla oturum açar**), hangi diğer hedeflerin de onu kullandığı (**Bunu da kullananlar**) ve oturum açma penceresini açan **Düzenle**. Bkz. [Oturum açma bilgileri](#sign-ins).
- **rclone.conf**: rclone üzerinden çalışan bir hedef için, bu hedefin rclone yapılandırma bölümü.
- **Silmeye karşı koruma**, **Silmeye karşı koruma (append-only)**, **Silmeye karşı koruma olmadan kabul et**, **Silmeye karşı korumayı şimdi test et** ve BombVault'un buradaki kopyayı ve kendi klasörlerini, bu hedefin **Ne kadar saklanır** kartına ya da bir bölümün **Bölüme göre kurallar** altındaki kendi değerlerine göre budayıp budamadığını ya da bunu karşı tarafa bırakıp bırakmadığını söyleyen **Kim budar** ile. Yerel bir klasör bu kartı yalnızca kapatılabilir bir koruması olduğunda gösterir. Bkz. [Silmeye karşı koruma](#delete-protection-append-only).
- **Kopyala**: **Ne zaman kopyalanır** ve **Yükleme sınırı (KiB/s)** ile **İndirme sınırı (KiB/s)**. Yalnızca kendi klasörlerini tutan ve kopya tutmayan bir hedef sınırları **Bant genişliği** adlı bir kart olarak gösterir.
- **Ne kadar saklanır**: bir kuralın hangi yedekleri tuttuğunun zaman çizelgesiyle **Kısa**, **Dengeli**, **Uzun** ya da **Özel değerler**. Bkz. [Ne kadar saklanır](#keep).
- **Zaten bu hedefin altında**: elle yazılmış adresi bu hedefin bir klasöründe duran bölümler. Birini devralmak depoyu olduğu yerde bırakır. Oturum açma, depolama sınıfı ve silmeye karşı koruma o zaman hedeften gelir. BombVault önce hedefin oturum açmasının depoyu açtığını denetler ve append-only bir depoyu append-only olmayan bir hedefin altına koymayı reddeder.
- **Bölüme göre kurallar**: bir bölümün bu hedefteki kendi deposu, saklaması ya da sıkıştırması. **Kuralları değiştir** onları açar, **Hedefin değerlerine dön** bir bölümü geri getirir. Oturum açma, bant genişliği ve kopyaların ne zaman çalışacağı her zaman hedeften gelir.
- Hedefin ne kadar dolu olduğunu, dolu olduğu işareti ve son haftaların büyümesinde kaç hafta kaldığıyla birlikte gösteren bir kap. Boyutunu bildirmeyen bir hedef **Boyut bilinmiyor** der.

**Taşı** ve **Hedefi kaldır** sayfanın eylemleri arasında durur ve [Taşıma ve kaldırma](#move-remove) altında anlatılır.

Yedek tutan ve birdenbire hiç depo tutmayan uzak bir hedef, örneğin diski bağlanmayan bir rest-server, kendiliğinden yeni, boş bir tane almaz. Sayfası, bir yedek ya da kopya deponun kaybolduğunu bulur bulmaz bunu söyler. Diski yeniden bağla. Eski depoyu bilerek boşalttıysan **Burada yeni bir depo oluştur** hedefin ve depolarının her bölümü için bir kerede baştan başlar. Bu koruma yerinde kalmış bir hedef içindir. Eklediğin bir hedef ya da bir hedefte işaretlediğin bir bölüm temiz başlar: o adreste depo yoksa ilk kopya bir tane oluşturur, daha önce kaldırılmış bir hedefin orada bir deposu olmuş olsa bile.

İki ret, neyin düzeltilmesi gerektiğini kopyanın çalıştırmasında ve hedefin sayfasında söyler. **Bu bucket sunucuda yok**, bir S3 sunucusu bunu söyler söylemez gelir ve BombVault orada hiçbir şey oluşturmaz, böylece yanlış yazılmış bir bucket adı görünür kalır. Hiç yanıt vermeyen bir sunucu başka bir durumdur: restic, sunucu geri gelir diye çağrı başına yaklaşık 15 dakika yeniden denemeye devam eder. **Burada zaten bir restic deposu var ve bu örneğin anahtarı onu açmıyor**, oradaki yedeklerin başka bir `APP_KEY`'e ait olduğu anlamına gelir.

Grubunun başka bir örneğinin çalıştırdığı bir alıcı sunucu sağlayıcılar arasında **Grubundan** olarak görünür. Bkz. [Alıcı sunucu](#receiving-server).

BombVault hedeflerini her başlangıçta denetler. Senden karar gerektirmeyeni düzeltir, örneğin kaybolmuş bir kopyaya ya da depoya bağlantıyı, bir bölümün kendi klasörüne bir kopyayı ya da kopyası kaybolmuş bir doğrudan depoyu, ve her düzeltme için günlüğe bir satır yazar. Güvenli bir çözümü olmayan yalnızca günlüğe yazılır.

![Aynı sayfa telefonda.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Aynı sayfa telefonda.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Ne kadar saklanır {#keep}

Yedeklerin ne kadar tutulacağı hedef başına, **Ne kadar saklanır** kartında ayarlanır. Kural o hedefteki kopyaları ve orada duran kendi klasörleri, **Yerel** hedef dahil, kapsar. Bir bölüm bir hedefte **Bölüme göre kurallar** altında kendi değerlerini tutabilir ve bunlar o bölüm için hedefin kartından önce gelir.

Bir kural, ayarladığın kadar geriye, gün, hafta, ay ve yıl başına bir yedek tutar ve gerisini her yedekten ya da kopyadan sonra temizler:

| Hazır ayar | Son yedekler | Gün | Hafta | Ay | Yıl |
|---|---|---|---|---|---|
| **Kısa** | 0 | 7 | 4 | 3 | 0 |
| **Dengeli** | 0 | 7 | 4 | 6 | 2 |
| **Uzun** | 0 | 14 | 8 | 12 | 3 |

**Özel değerler** beş sayıyı kendin ayarlar. 0 o sayıyı kapatır ve her sayısı 0 olan bir kural her şeyi tutar. Kartın noktaları, 40 örnek yedeğin hangilerini bir kuralın tuttuğunu gösterir, en yenisi en sağda. Örnek her hedef için aynıdır, çünkü resim BombVault oradaki yedekleri okumadan önce çizilir. Bugünden geriye doğru sayar, bu yüzden kaç noktanın kaldığı tarihe göre değişir.

Bu ana makinedeki bir klasörün ve el ile yazılmış uzak bir klasörün de kendi kartı vardır. Bu kart o klasördeki kendi klasörlerini kapsar, başkasını değil. Sen kaydedene kadar böyle bir klasör, BombVault'un onun için zaten sahip olduğu kurala göre budanır.

Varsayılan olarak açık **Daha az saklamadan önce sor**, daha az yedek tutan bir değişiklik kaydedilmeden önce sorar ve yeni kuralın 40 örnek yedeğin kaçını kaldırdığını söyler. Her hedef için geçerlidir. **Bölüme göre kurallar** altındaki **Hedefin değerlerine dön** aynı soruyu sorar.

Silmeye karşı koruması olan bir hedef bu makineden hiçbir zaman budanmaz. Karşı tarafı eski yedekleri kendi kurallarına göre tutar ya da bırakır. Bir ögede kaybolan veriyle ilgili açık bir kritik anormallik varken saklama, sen onu onaylayana kadar o ögenin eski yedeklerini tutar. Bkz. [Anormallikler](anomalies.md).

Olması gerekenden az şey tutan bir yedek yüzünden önceki yedekler hiçbir zaman eskiyip silinmez. Bu, önceki yedeklerde dosya varken hiç dosya okumayan, seçili bir klasörü kaçıran ya da bazı dosyaları okuyamayan bir yedektir. Çalıştırması, neyin eksik olduğunu söyleyen ve eski yedeklerin tutulduğunu belirten bir uyarıyla biter. Aynı mesaj bildirim kanallarına da gelir. Eksiksiz bir yedek gelene kadar hiçbir budama, ne ögenin kendi klasöründe ne de kopyalarında, o ögenin eski yedeklerine dokunmaz. İlk yedek ve hep boş olmuş seçili bir klasör eksiksiz sayılır.

Sonraki çalıştırmanın gerçekte neyi sileceğini görmek için **Ayarlar, Hedefler** altındaki **Önizleme**'yi aç. **Sonraki çalışmanın kaldıracakları**'nı bölümün kendi klasörü ve kopyalarının her biri için, depo başına ve öge başına, geri yükleme noktaları adlandırılmış olarak listeler. Kilit almaz ve hiçbir şeyi değiştirmez.

![Dengeli seçili Ne kadar saklanır: noktalar 40 örnek yedeğin hangilerinin kaldığını gösterir.](assets/screenshots/storage/keep-dark.png#only-dark)
![Dengeli seçili Ne kadar saklanır: noktalar 40 örnek yedeğin hangilerinin kaldığını gösterir.](assets/screenshots/storage/keep-light.png#only-light)

#### Örnek: Dengeli neyi saklar {#example-retention}

Bir konteyner 1 Ekim 2024'ten beri her gece 03:00'te yedekleniyor, yani 4 Kasım 2026 Çarşamba günü 765 yedeği var. Hedefi **Dengeli** olarak ayarlı: 7 gün, 4 hafta, 6 ay ve 2 yıl.

Her sayı kendi gününün, haftasının, ayının ya da yılının en yeni yedeğini tutar ve birkaç sayının seçtiği bir yedek bir kez tutulur. Haftalar Pazartesi'den Pazar'a işler:

| Kural | Seçtiği yedekler | Eklediği yeniler |
|---|---|---|
| 7 gün | 4 Kas, 3 Kas, 2 Kas, 1 Kas, 31 Eki, 30 Eki, 29 Eki | 7'si de |
| 4 hafta | 4 Kas, 1 Kas, 25 Eki, 18 Eki | 25 Eki, 18 Eki |
| 6 ay | 4 Kas, 31 Eki, 30 Eyl, 31 Ağu, 31 Tem, 30 Haz | 30 Eyl, 31 Ağu, 31 Tem, 30 Haz |
| 2 yıl | 4 Kas, 31 Ara 2025 | 31 Ara 2025 |

Bu, 765 yedekten 14'ünü bırakır. En eskisi 31 Aralık 2025'ten, yani bugün on aydan biraz fazla geriye gidebilirsin. Tekilleştirme sayesinde bir tam yedek artı arasında değişenden biraz fazla yer kaplarlar.

**Yıl** ikinci bir bakışı hak eder. İçinde yedek olan takvim yıllarını, geçerli olandan başlayarak sayar ve bu yılın en yeni yedeği bugünküdür. Yani ilk yıl diğer sayılara bir şey eklemez, ikinci yıl ise önceki yılın son yedeğini ekler. Bu yüzden bir kuralın ne kadar geriye uzandığı yıl içinde değişir. Aynı yedekler her hazır ayarda:

| Hazır ayar | Bugün tutulan yedek | Bugün en eski yedek | Yıl içinde geriye uzanma |
|---|---|---|---|
| **Kısa** | 10 | 30 Eylül 2026 | 1 ile 2 ay arası |
| **Dengeli** | 14 | 31 Aralık 2025 | ilkbahar başında yaklaşık 4 ay, 31 Aralık'ta tam bir yıl |
| **Uzun** | 30 | 31 Aralık 2024 | 1 Ocak'ta 1 yıl, 31 Aralık'ta 2 yıl |

**Dengeli**'den **Kısa**'ya geçmek önce sorar, **Daha az saklamadan önce sor** yüzünden. Soru, yeni kuralın resimdeki 40 yedekten kaçını kaldırdığını söyler. Bu sayı örnek yedeklerden gelir, yani tarihe göre değişir ve kendi yedeklerini saymaz. Bu örnekteki yedeklerden 31 Ağustos, 31 Temmuz, 30 Haziran ve 31 Aralık 2025 giderdi. **Yine de kaydet** kuralı uygular ve sonraki budama onları o hedeften kalıcı olarak siler.

Bu hedefte konteynerleri diğerlerinden daha uzun tutmak için **Bölüme göre kurallar** altında **Kuralları değiştir**'e bas, **Konteynerler**'i seç ve **Konteynerler için kurallar** içinde **Özel değerler**'i ayarla, örneğin 14 gün, 8 hafta, 12 ay ve 2 yıl. Diğer bölümler hedefin kartını izlemeye devam eder ve **Hedefin değerlerine dön** Konteynerler'i geri getirir.

### Oturum açma bilgileri {#sign-ins}

Bir oturum açma bilgisi, BombVault'un bir S3 servisi ya da rest-server için kullandığı anahtar veya paroladır. Oturum açma bilgileri tek bir listedir, **Ayarlar, Hedefler** altındaki **Oturum açma bilgileri** kartı, ve her satır hangi hedeflerin onu kullandığını söyler (**Kullanan**).

- **Bir hedef eklerken kaydedilir.** Hedef ekle yazdığın anahtarları bir oturum açma bilgisi olarak kaydeder. Aynı servisteki sonraki hedef onu **Yeni anahtarlar** yerine **Kaydedilmiş oturum açma** altında seçebilir ve onda sonradan yapılan bir değişiklik onu kullanan her hedefe uygulanır.
- **Varsayılan.** Bir yıldız varsayılan oturum açma bilgisini işaretler. El ile yazılmış bir adres ya da temiz bir kurulumda kurtarma gibi hiçbirinin seçilmediği her yerde kullanılır. Bir çekme kaynağı onu ödünç almaz. **Varsayılan yap** yıldızı başkasına verir.
- **Düzenle ve test et.** Burada ya da bir hedefin **Oturum açma** kartında **Düzenle**, oturum açma bilgisi için tek bir pencere açar. **Test**, hiçbir şey kaydedilmeden önce yazdığınla onu kullanan her hedefte oturum açar. **Kaydet** saklar.
- **Kaldırma.** Bir oturum açma bilgisi yalnızca hiçbir hedef, çekme kaynağı ya da alınan depo onu kullanmıyorsa ve hiçbir eski yedek ona ihtiyaç duymuyorsa kaldırılabilir ve varsayılan yalnızca sonuncu olarak gider. Eski yedekler, kendi klasörü bu bilgiyle oturum açan bir yerden taşınıp onları orada bıraktığında ona ihtiyaç duyar. BombVault o zaman bunun hangi yer olduğunu söyler.
- **Bir hedefin bağlantısını değiştirmek** başka yerlerin paylaştığı bir oturum açma bilgisini asla yeniden yazmaz. Hedef bunun yerine kendi oturum açma bilgisini alır ve pencere önceden onun oturum açma bilgisinin paylaşıldığını söyler. Paylaşılanı her yerde değiştirmek için **Oturum açma bilgileri** kartında düzenle.

![Oturum açma bilgileri kartı: her kayıtlı oturum açma bilgisi, onu neyin kullandığıyla ve varsayılanda yıldızla.](assets/screenshots/storage/signins-dark.png#only-dark)
![Oturum açma bilgileri kartı: her kayıtlı oturum açma bilgisi, onu neyin kullandığıyla ve varsayılanda yıldızla.](assets/screenshots/storage/signins-light.png#only-light)

Bu sunucudaki bir klasör oturum açma bilgisi istemez. SMB, WebDAV, SFTP, Storage Box ve bulut sürücüleri kendilerininkini rclone yapılandırmasında tutar, bunu hedefin **rclone.conf** kartı gösterir, bu yüzden bu listede görünmezler.

### SMB, WebDAV ve diğer rclone remote'ları {#smb-webdav}

Bir Windows ya da Samba paylaşımı ve bir WebDAV sunucusu (Nextcloud, ownCloud, SharePoint ya da başka biri), **Hedef ekle** içinde **NAS veya sunucu** altında sağlayıcı kutucuklarıdır. Ana makineyi ve paylaşımı (SMB) ya da adresi ve sunucu türünü (WebDAV), kullanıcıyı ve parolayı doldur, BombVault rclone bölümünü senin için yazar. rclone parolayı saklanmadan önce gizler.

Hiçbir kutucuğun kapsamadığı bir servis için **Kendi rclone yapılandırman** altındaki **Rclone** kutucuğu bir rclone.conf'tan yapıştırılan remote'ları alır. BombVault'un zaten sahip olduğu remote'lara eklenirler ve onların yerini asla almazlar, şifreli saklanırlar. Bir bölümü onlardan birine yedeklemek için o bölümün site dışı deposu olarak `rclone:<remote>:<folder>` gir, örneğin `rclone:nas:backups/bombvault`. Paylaşım ya da bucket ilk yol parçasıdır, adın parçası değil.

Bu, paylaşımı Unraid'e bağlamaktan daha iyi bir yoldur: restic bir depoyu bağlı bir CIFS paylaşımında tutmayı önermez ve burada hiçbir şey bağlanmaz. NFS'in kutucuğu yoktur, çünkü ne restic'in ne rclone'un NFS arka ucu var. NFS için dışa aktarmayı ana makinede bağla ve yerel klasör olarak kullan.

### Taşıma ve kaldırma {#move-remove}

- **Taşı**, bir bölümün kopyasını tüm geçmişiyle başka bir hedefe taşır. Her şey eski yerde kalır. BombVault orada hiçbir şeyi silmez. Taşıma arka planda çalışır ve uzun sürebilir: yerel diskler arasında 100 GB başına yaklaşık 9 dakika, yavaş bir hat üzerinden saatler. Genel bakış, çalışırken onu gösterir.
- **Hedefi kaldır** iki yol sunar. **Hedefi kaldır, yedekleri tut** oraya yazmayı bırakır ve yedekleri yerinde bırakır. Onları sonra **Geri yükleme, Eski bir yedek klasöründen** altında okuyabilirsin. **Hedefi kaldır ve oradaki yedekleri sil** önce orada nelerin saklandığını okur ve listeler: hangi yedekler gider, hangileri başka hiçbir yerde yok ve hangi depolar kalır, çünkü silmeye karşı koruma reddeder, ögeler hâlâ doğrudan onlara yedekler, başka bir hedef ya da bölüm kopyalarını orada tutar ya da okunamadılar. Tek yedeğini kaybedecek ögeler ayrı listelenir ve **Bunları da sil**'i açmadıkça tutulur. Hedefin adını yazarak onaylarsın ve silme arka planda çalışır.
- Bu her tür hedef için çalışır: birkaç bölümün kullandığı, bir bölüm için kurulan bir hedef, kurduğun bir depo ve bir bölümün kendi yedeklerini yazdığı klasör. Kaldırılan bir depoya yedekleyen ögeler yeniden bölümlerini izler ve oraya işaret eden bir bölüm varsayılanı da. Bir bölümün klasörü öylece gidemez, çünkü her bölümün bir tane ihtiyacı vardır. Pencere bölümlerin taşınacağı klasörü sorar ve her bölüm altında klasör adını korur.
- Bir hedefteki yedekleri silmek giriş parolası olmadan da çalışır. Parola ayarlıysa örneğin kendisinde bir oturum açma ister.

### Örnek: Backblaze B2'de site dışı kopya {#example-s3}

Konteynerler ve öz yedek ölü bir sunucu onları da götürmesin diye Backblaze B2'de de bulunmalı. Aynı adımlar Wasabi, Hetzner Object Storage ve çoğu diğer S3 servisi için de uygundur.

1. **B2'de** sana yakın bir bölgede `tower-backups` gibi özel bir bucket oluştur. **Application Keys** altında o bucket ile sınırlı, okuma ve yazma erişimli bir anahtar oluştur ve anahtar kimliğini ve uygulama anahtarını not al. B2 uygulama anahtarını bir kez gösterir. Bucket'ın yaşam döngüsünü yalnızca son sürümü tutacak şekilde ayarla, yoksa B2 budanan veriyi gizli sürümler olarak tutar ve bunlar için sana fatura keser.
2. **Sağlayıcı.** BombVault'ta **Hedefler**'i aç, **Hedef ekle**'ye bas ve **S3 hizmeti** altında **Backblaze B2**'yi seç.
3. **Oturum aç.** Anahtar kimliğini **Erişim anahtarı kimliği** olarak ve uygulama anahtarını **Gizli erişim anahtarı** olarak gir. Bucket'ın uç noktasını **Uç nokta** altına gir, örneğin `s3.eu-central-003.backblazeb2.com`, BombVault bölgeyi ondan okur. Bu anahtar yalnızca bir bucket'ı açtığı için `tower-backups`'ı **Bucket** alanına yaz.
4. **Bağlantıyı test et.** **Bağlantı kuruldu** demeli ve bucket'ın klasörlerini listelemeli. **Yeni klasör**'e bas ve adını `bombvault` koy. BombVault içine bölüm başına bir klasör koyar, örneğin `bombvault/containers`.
5. **Koruma.** Pencere B2'nin object lock ile eski sürümleri tutabildiğini ve BombVault'un bunu denetleyemediğini açıklar. Bucket gerçekten silmeyi reddetmiyorsa **Silmeye karşı koruma (append-only)**'yi kapalı bırak, böylece BombVault hedefin saklama kurallarına göre budayabilir.
6. **Hangi bölümler için.** **Konteynerler** ve **Öz yedek**'i işaretle. Öz yedek ayarlarını taşır, bu yüzden yeni bir kurulum onları geri getirebilir. Pencere kaç öge olduğunu ve bilindiği yerde ilk kopyanın ne kadar geçmiş getireceğini söyler.
7. **Hedefi kaydet**'e bas.

İlk kopya sonraki yedekten sonra çalışır. Şimdi başlatmak için **Ayarlar, Hedefler**'i aç ve **Site dışı kopyalar** içinde bölümün satırındaki **Şimdi çoğalt**'a bas. Durum başlığı **Site dışı kopyalama sürüyor** der. İlk kopya tüm geçmişi getirir, bu yüzden 100 Mbit/s'lik bir yükleme hattı üzerinden 200 GB yaklaşık dört buçuk saat sürer. Sonraki kopyalar yalnızca yeniyi gönderir.

Hattını boş tutmak için hedefin sayfasını aç ve **Kopyala** kartında **Yükleme sınırı (KiB/s)**'ni ayarla, örneğin yaklaşık 5,0 MB/s (41,9 Mbit/s) okunan `5120`. Oradayken aynı sayfada **Ne kadar saklanır**'a da bak.

İşe yaradığını görmek için genel bakıştaki **Koruma durumu**'na bak, orada **Site dışı** **kopya güncel** olur ve bir konteynerin **Yedekler** sekmesine, anlık görüntüleri artık ikinci bir yer olarak `tower-backups`'ı adlandırır. Sonra **Ayarlar, Geri yükleme** altında **Kurtarma kitini indir**'e yeniden bas, çünkü kit yeni hedefi ve oturum açmasını listeler.

**Cloudflare R2 aynı şekilde çalışır,** anahtar yerine bir nesne belirteciyle. R2 panelinin gösterdiği S3 adresini bucket'ıyla yapıştır, BombVault sunucuyu ve bucket'ı doldurur.

### Örnek: kendi çalıştırdığın bir rest-server {#example-rest-server}

Append-only kipindeki bir rest-server silmeyi reddeder, bu yüzden üzerindeki bir kopya, biri sunucunu ele geçirse bile hayatta kalır. Bir arkadaşının evinde ya da garajdaki ikinci bir makine yeter. Eşleşmiş bir BombVault bunu senin için kurabilir, bkz. [Alıcı sunucu](#receiving-server). Herhangi bir Docker ana makinesinde elle çalıştırmak için:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Son satır bir parola ister ve kullanıcıyı veri klasöründeki `.htpasswd` dosyasına yazar.

1. BombVault'ta **Hedef ekle**'ye bas ve **NAS veya sunucu** altında **rest-server**'ı seç.
2. Adresi, kullanıcısı klasör olarak gir, örneğin `http://192.168.2.20:8000/bombvault`, ve **Kullanıcı adı** ile **Parola**'yı. **Bağlantıyı test et**'e bas.
3. **Koruma** içinde **Silmeye karşı koruma (append-only)**'yi aç. BombVault o zaman orada hiç budamaz ve bunu karşı tarafa bırakır.
4. Bölümleri işaretle ve **Hedefi kaydet**'e bas.
5. İlk kopya orada olunca hedefin sayfasını aç ve **Silmeye karşı korumayı şimdi test et**'e bas. BombVault var olmayan bir nesneyi silmeyi dener. Kart **Son: silme reddedildi** demeli ve genel bakıştaki **Silmeye karşı koruma** **denetlendi** olur. **Son: silme reddedilmedi** diyorsa `--append-only` eksiktir.

Bu kopyayı buradan hiçbir şey budamadığı için, disk dolmadan önce haberin olsun diye bölümün **Site dışı kopyalar** içindeki satırında **Düzenle** ile açılan penceresinde **Büyüme bütçesi (GB, 0 = kapalı)**'ni ayarla. Kendi kendine imzalı bir sertifika için yukarıdaki `certs` notuna bak.

## Bir bölümün kendi klasörü {#own-folder}

Her bölümün bir kendi klasörü vardır: yedeklerinin önce yazıldığı depo. Diğer hedeflere kopyalar oradan çıkar. Yeni bir kurulum kendi klasörlerini bu sunucuda, `/mnt/user/bombvault` altında tutar ve birlikte **Yerel** hedefini oluştururlar.

Bir kendi klasörü, bir NAS, bir rest-server ya da bir S3 bucket'ı gibi, zaten sahip olduğun bir hedefte de durabilir. Bölüm o zaman aralarında yerel kopya olmadan doğrudan oraya yedekler ve kendi klasörü o hedefin oturum açmasını, silmeye karşı korumasını ve bant genişliği sınırlarını izler. BombVault onu her yedekten sonra hedefin **Ne kadar saklanır** kartına göre budar ve bir bölümün **Bölüme göre kurallar** altındaki kendi değerleri karttan önce gelir. Hedef bölümü **Kendi klasörleri** kartında listeler ve **Bölüme göre hedefler** hedefi bölümün evi olarak gösterir.

### Bir kendi klasörünü taşımak {#move-own-folder}

1. Kendi klasörünü tutacak hedefi aç ve **Kendi klasörü olarak kullan**'ı ya da **Kendi klasörleri** kartında **Bölüm ekle**'yi seç. Birkaç bölümü bir kerede taşımak için Yerel hedefin **Bağlantı** kartında **Burada kendi klasörleri** yanındaki **Kendi klasörlerini taşı**'yı seç.
2. **Bölümler** altında bölümleri ve **Şuraya taşı** altında yeri seç. **Başka bir yer** bu ana makinedeki bir klasörü ya da kayıtlı oturum açma bilgilerinden biriyle elle yazılan uzak bir deponun adresini alır.
3. **Ne olur**'u oku. Her bölüm için yeni yedeklerin nereye gideceğini, yeni yerde zaten olan bir kopyanın kendi klasörü olup olmayacağını ve hangi yedeklerin önce kopyalanacağını söyler.
4. **Önceden alınmış yedekleri kopyala** varsayılan olarak açıktır. Kapalıyken eski yedekler olduğu yerde kalır, ama öge artık onları listelemez. Geri yükleme sayfasındaki **Eski bir yedek klasöründen** onları yine de klasörün adresiyle ve uzak bir klasör için onun oturum açma bilgisiyle okur.
5. **Geç**'i seç. Yeni yerin **Ne kadar saklanır** kartı bölümün bugünkü kuralından daha az saklıyorsa BombVault önce **Daha az saklansın mı?** diye sorar. Orada o bölüm **için geçerli değerleri Bölüme göre kurallar altında koru** seçeneği bugünkü değerleri beraberinde taşır, **Yine de kaydet** devam eder ve **Geri** her şeyi olduğu gibi bırakır. Yedeklerin önce kopyalanması gerektiğinde BombVault onları kopyalar ve sonra kendi klasörünü değiştirir. Zaten oradalarsa değiştirmeden önce onları denetler.

Taşımadan sonra yeni yedekler taşınan kendi klasörüne gider, **Yerel**'i kapalı olan ve doğrudan bir kopyaya yedekleyen bir bölüm için de. Yeni yerde zaten duran bir kopya kendi klasörü olur ve oraya kopyalama biter. O kopyanın bölümün saklama kurallarından daha uzun yedek tuttuğu yerde bölüm iki kuraldan birinin tuttuğu her şeyi tutar, yani hiçbiri kaybolmaz.

Taşıma ilerleyemediğinde pencere nedenini **Ne olur** altındaki bir kutuda söyler. Yalnızca eski yedekleri kopyalamak engel olduğunda **Önceden alınmış yedekleri kopyala**'yı kapatmak kendi klasörünün onlarsız taşınmasına izin verir.

Bir bölümün her zaman bir kendi klasörüne ihtiyacı vardır. Onu tutan hedefi kaldırmak, bölümün taşınacağı klasörü sorar, bkz. [Taşıma ve kaldırma](#move-remove). Bölüm doğrudan bir kopyaya yedeklediği için yeni yedeklerin yanından geçtiği bir kendi klasörü, bir hedefin kimin kullandığını listelediği yerde **(kapalı)** ile işaretlenir.

![Bir kendi klasörünü taşıyan pencere, bir bölüm için doldurulmuş Ne olur ile.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Bir kendi klasörünü taşıyan pencere, bir bölüm için doldurulmuş Ne olur ile.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Örnek: Konteynerler bir NAS'a taşınıyor {#example-own-folder}

Önbellek havuzu doluyor ve ikinci bir makinedeki bir SMB paylaşımı olan `nas` adlı bir hedefin var. Konteynerler bölümünün bu sunucudaki kendi klasöründe 61 yedeği var ve Backblaze B2'ye kopyalıyor.

1. **Hedefler**'i, sonra `nas`'ı aç ve **Kendi klasörü olarak kullan**'a bas.
2. **Bölümler** altında **Konteynerler**'i işaretle.
3. **Ne olur**'u oku. Konteynerler'in yeni yedeklerinin `nas`'taki klasöre gittiğini, Yerel'deki 61 yedeğin önce oraya kopyalandığını, kopyaların Backblaze B2'ye gitmeye devam ettiğini ve korumanın, bant genişliği sınırlarının ve büyüme alarmının `nas`'tan geldiğini söyler.
4. **Önceden alınmış yedekleri kopyala**'yı açık bırak ve **Geç**'e bas.

BombVault yedekleri arka planda kopyalar ve sonra kendi klasörünü değiştirir. `nas` sayfası Konteynerler'i **Kendi klasörleri** altında listeler, bölümünü izleyen bir öge `nas`'ı evi olarak adlandırır ve sonraki yedek oraya düşer, B2'ye kopya `nas`'tan çıkar. Artık klasörü `nas`'ın **Ne kadar saklanır** kartı budar. Bu sunucudaki eski klasör olduğu yerde kalır ve BombVault orada hiçbir şeyi silmez.

### Uzak bir adresi elle yazmak {#remote-primary-repositories}

Yerel hedefin **Bağlantı** kartı hâlâ bir bölümün kendi klasörü için uzak bir adres alır: bölümün yanındaki **Değiştir**, **Yerel / Uzak** anahtarlı bir pencere açar. **Uzak**, `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` ya da `rclone:remote:bucket/path` gibi bir restic adresi alır ve **Uzak birincil depo güvenlik ayarları** bir bağlantı testi, yükleme ve indirme sınırları, bir site dışı hedefin aldığı aynı kurcalama testiyle append-only koruma ve bir büyüme bütçesi alarmı ekler.

Böyle bir adres varsayılan oturum açma bilgisiyle oturum açar. Tam olarak sahip olduğun bir hedefin bölüm için tuttuğu klasörse, farklı oturum açmadığı ya da hedef öyle değilken salt ekleme olmadığı sürece, hedefin kartı ve **Kullanılabilir** anahtarı dahil o hedefi kendiliğinden izler. O zamana kadar klasörü budayan bir kural, **Bölüme göre kurallar** altında bölümün kendi kuralı olarak kalır. O hedefin başka bir klasöründe durduğunda o hedefin sayfası bölümü **Zaten bu hedefin altında** altında listeler ve onu devralmak ona hedefin oturum açmasını, depolama sınıfını ve silmeye karşı korumasını verir. Kendi klasörü **Kendi klasörü olarak kullan** ile bir hedefe taşımak daha basit yoldur, çünkü pencere bir şey değişmeden önce önceden alınmış yedekleri denetler.

## Öge başına yerleşim {#placement}

Bir ögenin Ayarlar sekmesindeki **Zamanlama ve hedef** kartında bir düğme sırası vardır: **Yerel** ve bölümün site dışı hedefi başına bir düğme. Yanan düğmeler ögenin yedeklerini alır.

- **Yerel** yanıkken öge **Hedef** altında gösterilen depoya yazılır ve diğer her yanan hedefe kopyalanır. Bir hedefi karart, bu ögeden yeni bir şey almaz. Tek başına Yerel hiçbir yere kopyalamaz, bu da zaten ikinci bir kopyası olan veriye, örneğin bir NAS'ta yaşayan bir paylaşıma, uygundur.
- **Yerel** karanlıkken öge, ilk yanan hedefin doğrudan deposuna yazılır ve oradan diğer yanan olanlara kopyalanır. İlk seferde bir pencere o doğrudan depoyu oluşturur.
- Bir düğme yanık kalır, çünkü bir yedeğin gidecek bir yere ihtiyacı vardır. Bir ögeyi yedeklerden dışarıda bırakmak için **Yedeklenmesin**'i kullan.

BombVault yedekleri depolar arasında asla taşımaz. Bölümünü izleyen bir öge, bölümün varsayılanı değiştiğinde onunla gider: sonraki yedeği yeni yere düşer, sahip olduğu yedekler olduğu yerde kalır. Ögenin kendisinde seçilen bir hedef ilk yedekten itibaren sabit kalır. Ayarlar sekmesindeki **Depo** kartı onu adlandırır. Kopyalar her an değişebilir. Artık bir öge almayan bir hedef sahip olduğu kopyaları tutar ve bölümün sonraki site dışı çalıştırmasında onları kendi saklamasına göre budar. Append-only hedeflerden silinemez.

Bir ögenin terk ettiği yer yedeklerini tutar. Ögenin **Yedekler** sekmesinde ve **Geri yükleme**'de **Yalnızca eski yedekler** olarak durur, bir geri yükleme yine de ondan okuyabilir ve oradaki bir budama o yedeklere dokunmaz.

**Varsayılanı izle**, kendi seçimi olan bir ögeyi bölümüne geri getirir. Önce sorar, yeni yedeklerin o zaman nereye gideceğini söyler ve her hedefin neyi almayı bırakacağını ve sonraki çalıştırmanın neyi yükleyeceğini adlandırır.

Sıranın altında kart ögenin nereye gittiğini ve gerçekte orada neyin olduğunu söyler: kaç sitenin onu tuttuğu, her hedefin en son ne zaman görüldüğü ve 3-2-1'in karşılanıp karşılanmadığı. Bir site, orijinal verinin bulunduğu sunucu, her site dışı hedef ve **Tesis dışı** işaretli her depodur. BombVault kopyaları ve siteleri denetler. 3-2-1'in "iki ortam" kısmını denetlemez.

### Bölüm başına varsayılanlar {#placement-defaults}

Hedefler sayfasındaki **Bölüme göre hedefler** kartı her bölümün varsayılanını bir 3-2-1 akışı olarak çizer. Bir bölümden bir hedefe giden her satır bir anahtardır, **Yerel** dahil, her bölüm için, flash sürücü, ZFS veri kümeleri ve öz yedek de. Bir satıra ya da noktaya tıklamak onu hemen değiştirir ve kesik çizgiler kapalıdır. Yalnızca bir bölümün **Yerel**'ini kapatmak önce sorar, varsayılan kartıyla aynı pencerede, çünkü bölüm o zaman doğrudan bir kopyaya yedekler. **İptal** her şeyi olduğu gibi bırakır. **Grafik / Anahtarlar** aynısını düğme sıraları olarak gösterir. Bir anahtar yeni yedeklerin nereye gideceğini belirler. Kendi seçimi olmayan ögeler ve Compose stack'lerinin proje klasörleri hemen izler, bu yüzden sonraki yedekleri yeni yerlere gider. Zaten saklanan yedekler olduğu yerde kalır ve yine de oradan geri yüklenebilir. Flash, ZFS veri kümeleri ve öz yedek yerel klasörlerine yedeklenir ve o yedekten site dışına kopyalar. **Yerel**'e giden satırları da kapatılabilir. Yedekleri o zaman doğrudan site dışı hedeflerine gider ve replikası olan ve hiç hedefi olmayan bir ZFS ögesi yalnızca replikasını gönderir.

**Yerel** kapalı ve yalnızca site dışı hedefler açıkken bir bölüm doğrudan onlara yedekler ve BombVault oradaki depoyu kendisi seçer. Konteynerler, VM'ler ve klasör kümeleri aşağıda anlatılan, açık olan ilk hedefin doğrudan deposuna gider ve oradan diğerlerine kopyalanır. Flash sürücü, ZFS veri kümeleri ve öz yedek her yedeği açık olan her hedefe yazar ve her hedef onları kendi saklamasına göre tutar.

Her satırı kapalı olan bir bölüm **Yedeklenmedi** der. Ögeleri yedek almaz ve korunmuyor sayılır.

Sayfasında kendi hedeflerini seçen ögeler onları korur. Akış bir bölümde kaç tane olduğunu söyler, örneğin **2 öge farklı**, ve onlara götürür: bir tane varsa ögenin Ayarlar sekmesine, birkaç tane varsa her birinin nereye yedeklediği olan bir listeye. Oradaki **Varsayılanı izle** bir ögeyi geri getirir.

Yeni bir site dışı hedef, Yerel'e ayarlı olmayan her ögeyi alır. Onu ekleyen pencere kaç öge olduğunu ve bilindiği yerde ne kadar geçmiş olduğunu söyler.

### Doğrudan depolar {#direct-repositories}

Doğrudan depo, ögelerin doğrudan bir site dışı hedefe yedeklediği yerdir. BombVault onu hedefin klasörünün yanına adına `-direct` eklenerek koyar ya da hedef bucket'ının, remote'unun ya da bir rest-server kullanıcısının kökünde duruyorsa, içinde bölümün adıyla anılan bir klasöre, örneğin `containers-direct`. Hedefler sayfasındaki akışta soru sorulmadan oluşturulur. Tek bir öge için Yerel'i kapatmak, doğrudan deposu olmayan bir hedefin evi olması için, önerilen konumla, örneğin `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, ve hiçbir şey oluşturmayan bir bağlantı testiyle bir pencere açar. **Oluştur ve kullan** depoyu oluşturur ve ögeyi ona yönlendirir. Doğrudan deponun kendi ayarı yoktur. Hedefinin anahtarını, oturum açmasını, depolama sınıfını, sınırlarını, append-only ayarını ve saklamasını izler ve onlarla birlikte değişir. Hedefin sayfası onu **Doğrudan buraya yedekleniyor** altında, doğrudan ona yedekleyen her bölüm ve kaç ögenin yedeklediğiyle listeler. Artık hiçbir şeyin yedeklemediği biri orada bunu söyler ve kaldırılabilir, o yerdeki yedekler kalır. Hedef için yeni bir anahtar onu açamadığında doğrudan depo sahip olduğu anahtarı tutar ve kaydetme bunu söyler. Bir doğrudan depodaki bir öge oradan diğer açık hedeflere kopyalanır, deponun ait olduğu hedefe asla. Anlık görüntüleri `bv:direct` etiketini taşır ve her diğer saklama geçişi onları tutar, bu yüzden hedefiyle bağlantısını kaybetmiş bir doğrudan depo hiçbir zaman yerel kurallara göre eskimez. B2'ye S3 uç noktasıyla, anahtar kimliği ve uygulama anahtarı S3 anahtarları olarak girilerek ulaşılır. Hedefin kendi klasörüyle sınırlı bir anahtar yanındaki klasöre ulaşamaz, bu yüzden anahtarı bunun yerine üstteki klasörle sınırla.

### Tesis dışı {#off-the-premises}

Bir hedef ya da depo **Tesis dışı** olarak işaretlenebilir. Uzak olanlar işaretli başlar. Aynı binadaki bir rest-server için kapat. İşaret yalnızca kartlardaki siteleri ve 3-2-1'i sayar. Hiçbir kopyayı değiştirmez.

Bir ZFS replikası tesis dışı bir kopyadır, ama geçmişi olan bir yedek değildir. Yalnızca ayarladığın anlık görüntüleri tutar ve eski sürümler ile tek tek dosyalar yine yedeklerden gelir. Bir ZFS sunucusundan doğrudan diğerine, bir ZFS sunucusuna SSH üzerinden ya da eşleşmiş bir örneğin alma uç noktasına gider. Eşleştirme grubu yalnızca isteği taşır. Bkz. [Replika](zfs-datasets.md#replica).

### Yeniden kurulumdan sonra {#after-a-rebuild}

Kopya seçimleri BombVault'un kendi ayarlarında yaşar. **Yedekleri keşfet** ya da geri yüklenmiş ayarlar olmadan Acil durum sihirbazı ile bir yeniden kurulumdan sonra kaybolurlar ve her şeyi kopyalamak dışarıda bıraktığın ögeleri B2'ye yeniden gönderirdi. Aynısı, bu veritabanından hiç site dışına kopyalamamış bir bölüm bir hedefte kendi yedeklerini ya da kendi klasöründe daha eskilerini bulduğunda da geçerlidir, örneğin aynı `APP_KEY` ile önceki bir kurulumun yedeklerinin üzerine bir hedef eklendikten sonra. O bölümün site dışı kopyaları o zaman senin kararını bekler.

Genel bakıştaki **İlgi bekliyor** ve bölümün kopyaladığı her hedefin sayfası, yedeklerin bu sunucunun kendi yedekleri olup olmadığını sorar. **Bu sunucunun kendi yedekleri olarak al** bölümün varsayılanını onaylar ve kopyaları sonraki geçişten itibaren yeniden çalışır. **Önce kuralları kontrol et**, bölümün **Bölüme göre hedefler** içindeki satırına götürür. Orada her ögenin nereye gittiğini görürsün ve kaybolan bir kuralı ögenin sayfasında yeniden ayarlayabilirsin. Kopyalama orada sen **Bu sunucunun kendi yedekleri olarak al**'a basana kadar beklemeye devam eder. Bu düğme önce sonraki çalıştırmanın neyi kopyalayacağını ve yedeklerde ögesi olmayan, dışarıda bırakabileceğin adları gösterir. Bekleyen her ögenin en yeni yedeği bunu, karara bir bağlantıyla, kendi çalıştırmasında söyler ve bildirimler açıksa bir bildirim gider. Bir ayar dosyası içe aktarmak kuralları ve varsayılanları geri getirir ama kararı senin yerine vermez.

![Yeniden kurulumdan sonra İlgi bekliyor: klasör kümeleri bu veritabanından önceye ait yedekler buldu ve site dışı kopyaları kararı bekliyor.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Yeniden kurulumdan sonra İlgi bekliyor: klasör kümeleri bu veritabanından önceye ait yedekler buldu ve site dışı kopyaları kararı bekliyor.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Silmeye karşı koruma (append-only) {#delete-protection-append-only}

Fidye yazılımı ya da ele geçirilmiş bir ana makine yedeklerini silemesin ya da yeniden yazamasın diye bir site dışı hedef için silmeye karşı korumayı aç. Bunu karşı taraf (`--append-only` kipinde çalışan bir `restic/rest-server`) **uygular**. BombVault yalnızca **doğrular** ve yalnızca bir yapılandırma iddiasına bakarak asla yeşil göstermez. Bu sunucudaki bir klasör silmeyi reddedemez, çünkü BombVault'u denetleyen klasörü de denetler, bu yüzden yerel hedefler anahtarı sunmaz.

Anahtar hedefe aittir: oraya kopyalayan bir bölüm onu hedeften alır, bu yüzden onu **Hedefler** altındaki hedefin sayfasından aç. Zamanlanmış **Silme korumasını denetle** yalnızca bir bölüm anahtarı açık bir hedefe kopyaladığında çalışır. Karşı tarafın `--append-only` ile başlatılması gerekir. `restic/rest-server` image'inde, TrueNAS'ta da, diğer her Docker ana makinesinde olduğu gibi, bu `OPTIONS` değişkenine girer.

Bazı yerler silmeyi hiç reddedemez, örneğin OneDrive gibi bir bulut sürücüsü. Kopyaları orada bilerek tutuyorsan hedefin sayfasında **Silmeye karşı koruma olmadan kabul et**'i aç. Genel bakış o zaman yalnızca kabul edilmiş hedeflere kopyalayan bir bölümün eksik silmeye karşı korumasını kabul edilmiş bir risk olarak, kehribar nokta yerine gri bir halkayla gösterir ve bölümün rengini diğer denetimleri belirler. Başka bir şey değişmez: oradaki yedekler hâlâ silinebilir, BombVault onları hâlâ hedefin saklama kurallarına göre budar ve silmeye karşı koruma açılınca anahtar kaybolur. Başka bir yere taşınan bir kopya kabulü kaybeder. Silmeye karşı korumayı açmak da onu temizler.

Append-only bir kopyaya en hızlı yol, bir [alıcı sunucusu](#receiving-server) olan eşleşmiş bir BombVault'tur: rest-server'ı senin için append-only kipinde çalıştırır.

!!! note "`/locks/` altında başarılı bir silme beklenen bir şeydir"
    Append-only, hiçbir şeyin hiçbir zaman kaldırılamayacağı anlamına gelmez. restic kendi kilitlerini alıp bırakmak zorundadır, bu yüzden `/locks/` tasarım gereği yazılabilir ve silinebilir kalır. Fidye yazılımının peşine düşeceği anlık görüntüler ve arkasındaki veri kaldırılamaz. Karşı tarafı kendin yoklarsan `/locks/` altında başarılı bir silme doğru davranıştır, korumada bir delik değil.

!!! warning "Append-only depolar bu makineden hiç budanmaz"
    Append-only bir hedef buradan eski anlık görüntüleri hiçbir zaman budamaz. Budama karşı tarafın işidir. Depo boyutu kontrolden çıkmadan önce uyarılman için ona bir **büyüme bütçesi alarmı** ayarla.

## Kurcalama testi {#tamper-test}

BombVault, site dışı depoya var olmayan bir nesneyi hedefleyen bir silme girişimi yaparak append-only garantisini periyodik olarak kanıtlar. Bir hedefin sayfasındaki **Silmeye karşı korumayı şimdi test et** onu istek üzerine, **Ayarlar, Doğrulama, Otomatik denetimler** altındaki **Silme korumasını denetle** ise bir zamanlamayla çalıştırır.

**Silme korumasını denetle**'yi kapatmak bir açığı gizlemez. Hiç test edilmemiş, silmeye karşı korumalı bir hedef o zaman "kanıtlanmadı, denetimi kapalı" okunur, silmeye karşı koruması olmayan bir hedef de her iki durumda "yok" okunur.

Kapalı bir kopya test edilmez ve hedef bunu söyler. Bağlantı testi yine de üzerinde çalışır, çünkü yalnızca okur, böylece bir kopya açılmadan önce denetlenebilir.

- **Reddedildi** korumalı demektir.
- **Kabul edildi** korumasız demektir.
- **Kesin olmayan** bir sonuç (sunucuya ulaşılamıyor, kimlik doğrulama hatası) kayıtlı hükmü asla çevirmez.

Gerçek bir korumalıdan korumasıza geçiş tek bir uyarı tetikler.

## DR tatbikatları {#dr-drills}

BombVault, yedeklerinin yalnızca var olmadığını, gerçekten geri yüklenebilir olduğunu kanıtlamak için iki düzey sunar.

- **Geri yükleme doğrulama tatbikatları (yerel).** BombVault periyodik olarak `restic check --read-data-subset` çalıştırır (sınırlı, asla bir diski dolduran tam bir geri yükleme değil). Sıklık **Ayarlar, Doğrulama, Otomatik denetimler** altındadır ve aynı kutucuktaki **Yedekleri denetle** kartı sonucu gösterir.
- **Deneme geri yüklemesi (site dışı).** BombVault gerçek bir hedefi site dışı depodan kapalı bir kum havuzuna geri yükler, dosya dosya ve bayt bayt doğrular, sonra temizler. Bu, deponun yalnızca yanıt verdiğini değil, site dışından kurtarabildiğini kanıtlar.

Genel bakıştaki **Koruma durumu** kartı bunu bölüm başına dört denetimde toplar: bu sunucuda güncel bir yedek, site dışında bir kopya, orada silmeye karşı koruma ve oradan bir deneme geri yüklemesi. Bir bölüme tıklamak ayrıntıları gösterir ve kart yalnızca doğrulanmış gerçeklerle yeşile döner. Silmeye karşı koruma olmadan kabul ettiğin bir hedef orada bir uyarı değil, kabul edilmiş bir risk sayılır.

## Örnekleri eşleştirme {#pairing}

Birkaç BombVault bir grup olarak birlikte çalışır: birbirlerinin koruma durumunu gösterirler, birbirleri için roller üstlenirler ve birbirlerinin anahtarını tutabilirler. Bir örnek gruba on iki kelimeyle katılır.

![Üç adım kartlı eşleştirme sayfası.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Üç adım kartlı eşleştirme sayfası.](assets/screenshots/offsite/pairing-light.png#only-light)

İlk örnekte **Ayarlar, Eşleştirme**'yi aç ve **İfade oluştur**'a bas. Üç adım kartı nasıl gittiğini gösterir. On iki kelime bir **Kopyala** düğmesiyle bir pencerede görünür. Her diğer örnekte aynı yeri aç, **İfade gir**'e bas ve yapıştır ya da yaz ya da o penceredeki **Yapıştır**'a bas. Listede olmayan bir kelime, yazarken konumuyla adlandırılır ve son kelime bir sağlama toplamı taşır, bu yüzden yanlış yazılmış ya da yer değiştirmiş bir kelime hiçbir şey eşleşmeden yakalanır. İfadeyi yalnızca bir örnekte oluştur: ikisi de bir tane oluşturan iki örnek iki ayrı grup oluşturur. Bir dakika kimse görünmezse sayfa iki çıkış yolu sunar: kelimeleri orada girmek için yeniden göster ya da diğer örneğin kelimelerini gir ve onun grubuna tek adımda katıl. Eşleştirme giriş parolası olmadan çalışır, ama bir tane ayarla: onsuz web arayüzünü açabilen herkes kelimeleri okuyabilir ve grup aracılığıyla içindeki örneklerin restic parolasını veren rolleri isteyebilir. Eşleştirme kartı bir parola ayarlanana kadar bunu söyler. Bir parolayla ifadeyi yeniden göstermek onu ister. **Gruptan ayrıl** bir örneği yeniden çıkarır. Eşleştirme sayfasındaki **Örnekler** kartı grubun üyelerini listeler. Kimin kimin için ne yaptığının resmi Örnekler sayfasındadır.

Kelimeleri bilen herkes gruba katılabilir, bu yüzden onlara bir parola gibi davran.

**Üyeler birbirine nasıl ulaşır.** Her örnek ağdaki kendi adresini, oturum açtığın anda tarayıcından öğrenir, eşleştirme sayfasındaki **Bu örnek** kartında **Bilinen adresler** altında, örneğin kendisinin bulduklarıyla birlikte listelenir. Önünde bir ters proxy ya da olağandışı bir port varsa orada düzelt. Aynı ağda üyeler o adresi multicast ile duyurur ve doğrudan konuşur, multicast'in Docker'ın varsayılan bridge'i gibi bir konteyner ağını geçemediği yerde bir örnek bunun yerine kendi alt ağında diğerlerini yalnızca bir grup üyesinin yanıtlayabileceği imzalı bir çağrıyla arar, böylece eşleştirme röle olmadan da saniyeler içinde biter. Hiçbir şey çıkmazsa, başka bir alt ağ ya da standart olmayan bir port için **Bulamıyor musun?** elle bir adres alır. Farklı ağlardaki örnekler, aynı sayfadaki **Röle** kartında dörtlü tek bir sıradan seçilen bir röle üzerinden gider. Her biçimin bir resmi vardır:

- **Proje rölesi** (varsayılan): `parleyport.halleluja.design`, KnightLoader'ın da kullandığı röle. Kurulacak bir şey yok.
- **Kendi rölen**: Unraid Community Apps'ten [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) konteyneri (**Ayarlar, Uygulamalar** altında da). Adresini onu kullanması gereken her örnekte gir.
- **BombVault örneği**: dışarıdan zaten erişilebilir olan, **Röle olarak çalış** açık örneklerinden biri. O örnek o zaman kendi adresinde `/relay/connect` üzerinde, zaten sahip olduğu ters proxy ve sertifikanın arkasında yanıt verir ve yalnızca grubunu içeri alır. Adresini onu kullanması gereken her örnekte gir.
- **Röle yok**: üyeler aynı ağda birbirini kendiliğinden bulur, başka hiçbir yerde değil.

**Rölenin gördüğü.** Dört biçimin yanındaki aynı adlı bir düğme bunu seçtiğin biçim için gösterir. Üyeler arasındaki her çağrı on iki kelimeden türetilen bir anahtarla AES-256-GCM ile mühürlenir ve o anahtar örneklerinden hiç çıkmaz. Röle bağlantıları gruplayan bir karmayı, bir mesajın hangi örnek için olduğunu, ne kadar büyük olduğunu ve ne zaman geçtiğini öğrenir. Yerel ağdaki doğrudan bir çağrı aynı şekilde mühürlenir ve ayrıca imzalanır, bu yüzden hiçbir şey bir örneğin sunduğu kendi kendine imzalı sertifikaya bağlı değildir. Bir şey veren çağrılar (rol yanıtları, bir restic parolası, alıcı girişleri, anahtar emanetleri) ayrıca onları gönderen üyenin, ilk görüldüğünde sabitlenen bir anahtarıyla imzalanır. Bkz. [Güvenlik modeli](configuration.md#security-model).

**Grup üzerinden ne yol alır.** Örnekler sayfasındaki koruma durumu, bir bölümü şimdi denetleme isteği, rol istekleri ve yanıtları, depolama teklifleri ve bir alıcının ya da çekenin rolü izin verildikten sonra ihtiyaç duyduğu şey: diğer örneğin depo konumları ve restic parolası. Yedek verisi asla. O hâlâ doğrudan restic arka uçlarına gider. APP_KEY de değil: restic parolası o örneğin depolarını açar ve başka hiçbir şeyi, saklanan gizli bilgilerini, oturumlarını ya da kurtarma kodlarını değil.

**Eşleştirmeden önceki girdiler.** Bir filo belirteciyle eklenen örnekler ve diğer örneğin APP_KEY'iyle kurulan alınan depolar ve çekme kaynakları güncellemeden sonra kalır ve **Yeniden eşleştir** olarak işaretlenir. Çalışmaya devam ederler: ilk başlangıçta BombVault her saklanan APP_KEY'i ondan türetilen restic parolasıyla değiştirir. Her iki örneği de eşleştir, sonra girdiyi düzenle ve örneğini seç. O zamana kadar bu girdiler Örnekler sayfasındaki **Örneksiz** kartında durur.

Hâlâ elle bir APP_KEY alan tek yer, diğer örneğin gittiği ve bir grupta yanıt veremediği durum için [Eski bir yedek klasöründen](#restore-from-another-bombvault-repo)'dir.

## Örnekler ve roller {#roles}

**Örnekler** sayfası (kenar çubuğuna sabitlenebilen bir ayar kutucuğu) grubu kimin kimin için ne yaptığının bir ağı olarak, bu örnek için bir kart ve her diğer üye, Android telefonları ve ZFS sunucuları için bir kartla gösterir. Sağ alttaki **Eşleştirme** eşleştirme sayfasına götürür. Hangi parçaların açık olduğu **Ayarlar, Örnekler, Birden çok sunucu** altında ayarlanır: **Örnekler**, **Alıcı** ve **Çekme**.

![Örnekler sayfası: ağ olarak grup ve rol kutucuklarıyla örnek başına bir kart, biri Etkin değil.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Örnekler sayfası: ağ olarak grup ve rol kutucuklarıyla örnek başına bir kart, biri Etkin değil.](assets/screenshots/offsite/instances-light.png#only-light)

Her kart üyenin koruma durumunu ve rollerini kutucuk olarak gösterir. Bir rol, bir örneğin başka bir örnek için yaptığı şeydir:

- **Alıcı**, bu sunucunun site dışı kopyalarını append-only kipindeki bir rest-server'da tutar. Bu sunucu her yedekten sonra gönderir ve biri onu ele geçirse bile orada hiçbir şeyi silemez.
- **Çeken**, bu sunucunun yedeklerini kendi zamanlamasında çeker. Bunun için bu sunucunun uyanık olması gerekmez. Çeken örnekte isteğe izin verebilmeden önce **Çekme** açık olmalıdır.
- **ZFS sunucusu**, bu sunucunun ZFS replikalarını alır. Her ZFS ögesi onu ayarlarındaki **Replika** altında seçer.
- **Anahtar bekçisi**, bu sunucunun kurtarma kitinin mühürlü bir kopyasını tutar. Bkz. [Anahtar bekçisi](#key-keeper).

Bir rolü açmak bir istek gönderir ve diğer örnekte biri **İzin ver**'e basana kadar hiçbir şey başlamaz. İstek o örneğin kartında bir rozet olarak görünür ve **Ayrıntılar**'ı onu **İzin ver** ve **Reddet** ile listeler. İzin verilen bir rol orada her an iptal edilebilir. Ağdaki kesik bir çizgi hâlâ bekleyen bir roldür. BombVault 9.x üzerindeki bir örnek bir isteğe yanıt veremez, bu yüzden bir rol orada açar açmaz etkindir ve bir 9.x örneğinden gelen bir istek burada **İzin ver**'i bekler.

Bir karttaki **Ayrıntılar** gerisini açar: o üyenin alınan depoları ve çekme kaynakları, depolama teklifleri, izleme, **Kaldır** ve **Anahtarı unut**.

Bir üye kendi site dışı depolamasını grup üzerinden başka bir üyeye sunabilir. Diğer yönetici teklifi üyenin **Ayrıntılar**'ında görür ve kabul eder ya da reddeder. Kabul etmek, yazdığın bir hedefle aynı denetimlerden sonra kayıtlı bir oturum açma bilgisi ve sıradan bir hedef oluşturur. Hedef silmeye karşı koruma açık olarak gelir, çünkü teklifin gösterdiği tarif salt ekleme yapan bir rest-server'dır, bu yüzden orada **Ne kadar saklanır** geçerli değildir. Her teklifin kendi oturum açması ve kendi klasörü vardır, bu yüzden bir bölümü aynı üyeye ya da başka birine yeniden sunmak ilk ortağın kopyalarına dokunmaz: `.htpasswd` dosyasına yeni satırı ekle ve sunucuyu çalışır bırak. Bu yolla yalnızca bağlantı ayrıntıları gider, yedek verisi asla. **Şimdi denetle**, bir üyeden bir bölümün deposunu doğrulamasını ister. Sayfadaki hiçbir şey başka bir makinede bir yedek, geri yükleme başlatamaz ya da bir şey silemez.

Kapatılmış örnekler ve uygulaması kapalı telefonlar kartlarını gri olarak **Etkin değil** diye, en son ne zaman görüldükleriyle korurlar ve sen onları **Kaldır**'a kadar ağda soluk dururlar. Bu sunucu başlar başlamaz böyle bir kart grup dönene kadar **Denetleniyor** der. **Kaldır** hangi parçalar açık olursa olsun böyle bir kartta çalışır ve ayrıca üyenin açık isteklerini, boşta ZFS alma yuvalarını ve bekleyen anahtar isteklerini de sonlandırır. Eşleştirme sayfasındaki **Örnekler** kartı bu üyeleri de **Etkin değil** olarak listeler.

## Alıcı ve çeken {#receiver}

Başka bir BombVault'tan append-only kopyaları **alan** makinede, alınan depolar alıcı donanımda bağımsız, salt okunur izleme sağlar, böylece uzak uçtaki sessiz bir hata gözden kaçmaz. **Ayarlar, Örnekler** altında **Alıcı**'yı aç, sonra Örnekler sayfasında gönderen örneğin **Ayrıntılar**'ını aç ve alınan depoyu ekle (salt okunur, gönderen örneğin [eşleştirme grubu](#pairing) üzerinden aldığı restic parolasıyla açılır). Şunları elde edersin:

- **Kaynağa göre bir envanter**, hangi konteynerlerin, VM'lerin ve klasör kümelerinin indiğini tam olarak görmen için.
- Kaynak başına **Son alınan**, her birinin ne kadar taze olduğunu bilmen için.
- Alıcı donanımda çalışan **bağımsız bir `restic check`**, bütünlüğün yalnızca gönderende değil, verinin gerçekten durduğu yerde doğrulanması için.
- **Bir ölü adam anahtarı:** bir kaynak senin ayarladığın bir pencere içinde göndermeyi bıraktığında bir uyarı. Günde bir kez, 09:15'te bakar. Denetim zamanlaması ne derse desin, depo hiç okunamadığında ve onu ekledikten sonra tam bir pencere boyunca hiçbir şey gelmediğinde de bir uyarı alırsın. Gönderenin ilk kopyasından önce eklediğin bir depo o zamana kadar sorun sayılmaz.
- **Bütünlük uyarıları:** alıcı tarafta bir denetim başarısız olduğunda bir uyarı. Depoyu açamayan bir denetim bütünlük hakkında bir şey söylemez, bu yüzden bir hüküm bırakmaz ve ertesi gün aynı saatte yeniden dener.

Başka bir sunucudaki bir depo o sunucunun oturum açmasını ister: onu bir [oturum açma bilgisi](#sign-ins) olarak kaydet ve iletişim kutusunda **Kimlik bilgileri** altında seç.

Alıcı kesinlikle salt okunurdur. Alınan depoya asla yazmaz, bu yüzden göndericinin dayandığı append-only garantisini asla bozamaz.

Çeken tersine çalışır: **Çekme** açıkken bir örnek başka bir üyenin yedeklerini kendi zamanlamasında, her kaynak için kendi deposuna okur, böylece bu makinenin kendi yedekleri sayılmazlar. Kaynakları da o üyenin ayrıntılarında durur: depo konumu, hangi tür yedeği tuttuğu ve ne sıklıkla çekileceği. restic parolası grup üzerinden gelir, APP_KEY asla, ve karşı tarafın kurması gereken başka bir şey yoktur ve çalışıyor olması gerekmez. Kaynak yalnızca okunur ve her taraf kendi oturum açmasını korur. Bir çekme, zamanlamasının söylediği gün ve saatte çalışır ve yeni bir kaynak da o saati bekler. **Şimdi çek** beklemez. Bu makine kapalı olduğu için kaçırılan bir çekme, makine geri gelince çalışır. Başarısız olan bir çekme, haftalık bir zamanlamada da, ertesi gün aynı saatte yeniden dener. Başarısız bir çekme etkinlik günlüğünde görünür ve bir çekme yeniden başarılı olana kadar bir uyarı gönderir. Bir `rclone:` kaynağı reddedilir, çünkü rclone ona bu örneğin remote'larıyla ulaşırdı.

Çekilen bir kopya yalnızca büyür. BombVault ona ekler, ama onu ne budar ne de denetler, bu yüzden kaynağın çoktan unuttuğu şeyler sen klasörü silene kadar burada kalır ve Hedefler sayfası onu saymaz. Bölümün kendi klasörünün içinde `pulled/<source id>` altında durur. O klasör taşındığında kopya olduğu yerde kalır ve sonraki çekmeler her şeyi yeniden kopyalamak yerine orada devam eder. Onu silersen sonraki çekme, bölümün o zamanki kendi klasöründe baştan başlar.

### Alıcı sunucu {#receiving-server}

Alıcı makine diğerlerinin kopyaladığı rest-server'ı da çalıştırabilir. Onu isteyen bir **Alıcı** isteğine ilk izin verdiğinde kurulur: pencere sunucunun bir alıcı sunucuya ihtiyaç duyduğunu söyler ve bir paylaşımda bir klasör, oluşturmak için **Yeni klasör** ile, ve bir port (başka bir konteyner kullanmıyorsa 8000) ister. BombVault sonra:

1. `rest-server` adlı bir konteyner zaten varsa ya da portu başka bir konteyner tutuyorsa reddeder,
2. `restic/rest-server`'ı çeker ve Docker soketi üzerinden append-only kipinde, özel depolar ve klasörde bir giriş dosyasıyla başlatır,
3. Unraid şablonunu flash sürücüye yazar, böylece konteyner Docker sekmesinde düzenlenebilir kalır ya da flash sürücüye ulaşılamıyorsa şablonu indirme olarak sunar,
4. üzerinde kurcalama testini çalıştırır ve silmeyi reddedip reddetmediğini gösterir.

Grubundaki örnekler sunucuyu o zaman sağlayıcılar arasında **Grubundan** altında alıcı makinenin adıyla bulur ve **Alıcı** rolü onu güvenli seçim olarak sunar. Her örnek sunucuyu ilk kullandığında kendi girişini alır ve orada yalnızca kendi klasörüne yazar. **Ayarlar, Örnekler** altında **Alıcı** açıkken, bu örneğin kartındaki **Alıcı sunucu** sunucuyu yeniden açar. Penceresi girişleri **Oturum bilgisi olan ortaklar** altında listeler ve **Girişi iptal et** birini alır. O örneğin zaten kopyaladıkları klasörde kalır. Kurulum ayrıca parolasını pencerenin bir kez gösterdiği bir **Grup dışı oturum bilgisi** oluşturur.

Alıcı makineye yalnızca röle üzerinden ulaşan bir örnek sunucuyu kullanamaz, çünkü röle yedek taşımaz. Önce alıcı makinenin adresini eşleştirme sayfasına ekle. BombVault kendi IP adresinde (örneğin br0'da) çalışıyorsa **Ortaklar için adres**'i doldur, çünkü sunucu ana makinenin adresini dinler.

## Örnek: iki Unraid makinesini eşleştirmek {#worked-example-two-unraid-boxes-end-to-end}

Bu, yukarıdaki parçaların tek bir eksiksiz kurulumudur, çünkü bir araya getirilmiş halde görüldükten sonra bir araya getirmek daha kolaydır.

İki makine: **TOWER** konteynerleri çalıştırır ve yedekleri gönderir. **VAULT** onları alır ve append-only'yi uygular. Kendi adlarını, adreslerini ve paylaşım yollarını koy.

**1. İki makineyi eşleştir.** TOWER'da **Ayarlar, Eşleştirme**'yi aç, **İfade oluştur**'a bas ve on iki kelimeyi VAULT'ta **İfade gir** altına gir. Önce her ikisinde de bir giriş parolası ayarla.

**2. TOWER'da VAULT'tan alıcı olmasını iste.** Örnekler sayfasında VAULT'un kartında **Alıcı**'yı aç, bölümleri ve yer olarak **VAULT'un alıcı sunucusu**'nu seç ve isteği gönder.

**3. VAULT'ta izin ver.** VAULT'un Örnekler sayfası isteği TOWER'ın kartında gösterir. **İzin ver**'e bas. İlk seferde VAULT alıcı sunucusu için `user/appdata/rest-server` gibi bir klasör ister, rest-server'ı append-only kipinde başlatır ve üzerinde kurcalama testini çalıştırır.

**4. Yine TOWER'da.** Bundan sonra TOWER her yedekten sonra VAULT'a kopyalar ve yeni hedef TOWER'ın Hedefler sayfasında silmeye karşı korumasıyla görünür.

Kurcalama testinin yanıtladığı:

| Sonuç | Ne oldu |
| --- | --- |
| **korumalı** | VAULT silmeyi reddetti. Bu tek geçen durumdur. |
| **korumasız** | VAULT bir silmeyi kabul etti. `--append-only` eksik ya da kaldırılmış. |
| **kesin olmayan** | İkisi de değil. Genellikle URL restic'in kendisinin kullandığı değil ya da kimlik bilgileri değişti. Hiçbir şey kaydedilmez ve uyarı tetiklenmez. |

**5. İstersen gelenleri izle.** VAULT'ta **Ayarlar, Örnekler** altında **Alıcı**'yı aç, TOWER'ın **Ayrıntılar**'ını aç ve alınan depoyu ekle: iletişim kutusunun listelediği hedeflerden birine tıkla. VAULT, TOWER'ın restic parolasını grup üzerinden alır ve hedef VAULT'un kendi alıcı sunucusunda durduğu için klasörü doğrudan okur. Kimse bir anahtar ya da bir oturum açma bilgisi yazmaz.

**6. Karşılıklı yap.** Adımları diğer yönde tekrarla: VAULT, TOWER'dan alıcısı olmasını ister. Her makine o zaman diğeri için append-only'yi uygular ve hiçbiri ötekinin yedeklerini silemez.

Elle çalıştırdığın bir rest-server da çalışır: onu **Hedef ekle** ile bir rest-server olarak, `.htpasswd` dosyasındaki kullanıcı ve parolayla ekle ve silmeye karşı korumayı aç. Seçeneklerinde `--append-only`'yi koru. Onsuz karşı taraf yeniden sıradan bir paylaşımdır.

!!! warning "Yerel bir yol, konteynerin içindeki bir yoldur"
    BombVault bu sunucuda bir klasör istediğinde onu ana makine bağlamasına göre göreli gir, örneğin `user/appdata/rest-server/bombvault-containers`, `/mnt/user/appdata/...` değil. BombVault ana makinenin `/mnt`'sinin başka yere bağlandığı bir konteynerde çalışır. Mutlak bir ana makine yolu içinde yoktur. Birini yapıştırırsan BombVault bunun yerine kullanılacak göreli yolu söyler.

## Acil durum sihirbazı {#emergency}

**Acil durum sihirbazı** kaybolmuş ya da yeniden kurulmuş bir sunucuyu geri getirir. **Geri yükleme** sayfasında **Acil durum** altında ve yeni bir kurulumun karşılama sayfasındaki **Yedekten geri yükle**'den başlar. Dört adımda çalışır: **Erişim**, **Ayarlar**, **Seçim** ve **Geri yükleme**.

1. **Erişim.** BombVault'a eski yedeklerin anahtarını üç yoldan biriyle göster:
    - **Kurtarma kitini yükle**: kurulum sırasında indirdiğin dosya. İçinde anahtar ve her hedef vardır ve BombVault geri kalanını ondan doldurur. age ile mühürlenmiş bir kit, mühürlendiği kimliği ister. Bir kez kullanılır ve saklanmaz.
    - **Grup üzerinden**: grubundaki bir örnek anahtarı bir emanet olarak tutar. Grubun on iki kelimesine, teslim koduna ve o örnekte birine ihtiyacın var. Bkz. [Anahtar bekçisi](#key-keeper).
    - **Anahtarı yaz**: eski sunucunun `APP_KEY`'i ve yedeklerinin nerede olduğu.

    BombVault sonra anahtarı her hedefte dener ve her biri için okunabilir mi, bulunamadı mı, başka bir anahtarla kilitli mi, ulaşılamıyor mu ya da oturum açması hâlâ eksik mi olduğunu söyler. Henüz hiçbir şey kaydedilmedi ve anahtar sınırlı bir süre bellekte kalır. **Anahtarı devral**, bu sunucunun bundan sonra yedeklerini o anahtarla yazmasını sağlar. **Devralmayı geri al**, ilk yedek çalışana kadar kendi anahtarına döner. Kendi yedekleri olan bir sunucu devralmayı reddeder ve bunun yerine [Eski bir yedek klasöründen](#restore-from-another-bombvault-repo)'e işaret eder. **Oturum bilgilerini kaydet**, kitin uzak hedefler için taşıdığı oturum açma bilgilerini saklar ve bu sunucunun zaten sahip olduğu oturum açma bilgilerini değiştirmeden önce sorar.
2. **Ayarlar.** **Ayarları geri getir**, BombVault'un kendi ayarlarını en yeni öz yedekten geri yükler: hedefler, oturum açma bilgileri, zamanlamalar ve kurallar. BombVault bunları uygulamak için Docker soketi üzerinden yeniden başlar, böylece canlı ayar veritabanının üzerine açık bir tutamaç varken asla yazılmaz ve sihirbaz sonra kendi kendine sürer. Bu sunucunun girişi kalır: şu an kullandığın parolayla oturum açmaya devam edersin. Eski sunucunun sakladığı ve devralınamayan gizli bilgiler listelenir, böylece onları yeniden girebilirsin. Geri yükleme her oturumu sonlandırır ve geçiş anahtarlarını kaldırır, bu yüzden sonra onları yeniden kur.
3. **Seçim.** BombVault yedeklerine bakar ve bulduğu konteynerleri, VM'leri, klasör kümelerini ve ZFS veri kümelerini listeler. En yeni anlık görüntü her yerde önceden seçilidir. Bu sunucunun hâlâ sahip olduğu şey kapalı başlar, çünkü bir geri yükleme onun yerini alırdı. Klasör kümeleri yeniden kurulumdan sonra nereden geldiklerini bilmez, bu yüzden seçtiğin bir klasöre giderler.
4. **Geri yükleme.** Seçilen her şey tek seferde geri gelir, her konteyneri ve VM'i kendin başlatmak istiyorsan durmuş halde, ki bu toplam bir kayıptan sonra en güvenlisidir. Sonra yeni bir kurtarma kiti indir: eskisi önceki kuruluma aittir.

![Acil durum sihirbazı ilk adımında, üç giriş yoluyla.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Acil durum sihirbazı ilk adımında, üç giriş yoluyla.](assets/screenshots/offsite/emergency-light.png#only-light)

### Örnek: kurtarma kitinden yeni bir sunucu {#example-emergency}

TOWER sunucusu disklerinin yanı sıra öldü. 23 konteyner ve 2 VM tutuyordu ve bunları ve öz yedeğini Backblaze B2'ye kopyalıyordu. Parola yöneticinden `bombvault-recovery-kit.md` dosyan ve üzerinde Unraid olan yeni bir makinen var. Eski `APP_KEY` gitti ve ona ihtiyacın yok.

1. Yeni makineye **BombVault'u kur** ve ona her yeni kurulumda olduğu gibi yeni bir `APP_KEY` ver. VM'leri geri istiyorsan VM bağlantısını **Ayarlar, Entegrasyonlar, Ana makine SSH** altında kur, çünkü bir VM geri yüklemesi buna ihtiyaç duyar.
2. **Sihirbazı başlat.** **BombVault'a hoş geldin** üzerinde **Yedekten geri yükle**'yi seç. Geri yükleme sayfasında **Acil durum** altında **Acil durum sihirbazını başlat**'ı seç.
3. **Erişim.** **Kurtarma kitini yükle**'yi seç ve dosyayı bırak. BombVault anahtarını her hedefte dener ve örneğin **Kurtarma kiti okundu. Anahtarı 2 hedeften 0'ını açıyor.** der. Yerel klasör, disk öldüğü için **bulunamadı** der ve Backblaze B2 **kimlik bilgisi yok** der.
4. **Oturum bilgilerini kaydet.** Kit her hedefin oturum açma bilgisini taşır. **Oturum bilgilerini kaydet**'e bas, hedefler yeniden denenir ve Backblaze B2 artık **okunabilir** der. 10.5'ten önce yazılmış bir kit yalnızca varsayılan oturum açma bilgisini tutar. Bir hedef hâlâ **kimlik bilgisi yok** diyorsa, oturum açma bilgisini **Eksik kimlik bilgisini gir** altında gir ve **Hedefleri yeniden dene**'ye bas.
5. **Anahtarı devral.** Bu sunucu şimdi yedeklerini eski anahtarla yazar, bu yüzden yeni yedekler eskilere uyar. Şablondaki kendi `APP_KEY`'ini korur. **Devralmayı geri al** ilk yedek çalışana kadar işler.
6. **Ayarlar.** Adım, anahtarın öz yedeği bulduğu yerde, burada B2'de, adresi doldurulmuş olarak açılır. Oradan en yeni öz yedeği okuyan **Ayarları geri getir**'i seç. BombVault onu uygulamak için yeniden başlar ve sihirbaz kendi kendine sürer. Hedeflerin, zamanlamaların ve kuralların geri geldi ve bu sunucunun parolasıyla oturum açmaya devam edersin. Sihirbazın listelediği gizli bilgileri, örneğin bildirim ayarlarını, yeniden gir.
7. **Seçim.** Yerelde hiçbir şey kalmadığında **Yedekler nerede?** site dışı kopyalarda açılır. BombVault her hedefi okur, böylece ölü bir hedef diğerlerini gizlemez, 23 konteyneri ve 2 VM'i listeler ve her birini en yeni anlık görüntüsünü tutan hedeften geri yükler.
8. **Geri yükleme.** Toplam bir kayıptan sonra en güvenlisi olan **Sonra durmuş bırak**'ı aç ve başlat. **Ne olur** neyin geri geldiğini söyler. Sonuç satırı örneğin **25 geri yüklendi, 0 başarısız. Gerektiğinde Yedekler altında başlat.** diye okunur.
9. **Bitir.** Önce veritabanlarını, sonra onları kullanan uygulamaları başlat. **Ayarlar, Geri yükleme**'yi aç ve yeni bir kurtarma kiti indir, çünkü eskisi önceki kuruluma aittir. Genel bakışta **İlgi bekliyor**'a bak ve her şey çalıştığında **Şimdi tümünü yedekle**'yi çalıştır.

Öz yedek de eksikse adım **Atla, bende Öz yedek yok** üzerinde açılır ve bunu söyler. Hedefler o zaman yalnızca kitten gelir, zamanlamaları ve kuralları yeniden kurarsın ve sen onların yedeklerini bu sunucunun kendi yedekleri olarak alana kadar site dışı kopyalar bekler, bkz. [Yeniden kurulumdan sonra](#after-a-rebuild). **Seçim** yine de site dışı kopyaların tuttuklarını listeler. Klasör kümeleri seçtiğin bir klasöre gider. Konteynerler ve VM'ler **Yalnızca dosyalar geri gelebilir** altında bir klasöre düz dosyalar olarak geri gelir, çünkü tanımları yerel deponun yanında ve öz yedekte durur, site dışı bir kopya ise ikisini de tutmaz.

!!! tip "Planlı taşıma ile felaket"
    Acil durum sihirbazı BombVault'un kendi ayarlarını bir yedekten geri yükler. Yeni bir makineye *planlı* bir taşıma için bunun yerine yapılandırmanı doğrudan **Ayarları dışa / içe aktar** kartıyla (taşınabilir bir JSON dosyası) taşıyabilirsin. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).

### Eski bir yedek klasöründen {#restore-from-another-bombvault-repo}

Geri yükleme sayfasında **Diğer kaynaklar** altındaki **Eski bir yedek klasöründen**, *başka* bir BombVault örneğinin deposunu (`/mnt` altına bağlı bir paylaşım ya da uzak bir adres) o örneğin kurtarma kiti ya da yazılmış `APP_KEY` ile, tek seferlik, salt okunur bir oturumda açar. Orada saklanan konteynerlere, VM'lere, klasör kümelerine ve ZFS veri kümelerine göz at, bir anlık görüntü seç ve geri yükle, geri yüklenen nesne normal bir yerel konteyner, VM ya da klasör kümesi olur. Diğer depoya hiçbir zaman bir şey yazılmaz ve kendi yedekleme ayarların dokunulmadan kalır (oturum bellekte yaşar ve kendiliğinden sona erer). Bir konteyneri A sunucusundan B sunucusuna taşımak, ayarlarını yeniden yönlendirmek ve sonra geri almak anlamına gelmez. Bu pencere tek seferliktir: bir oturum açar, seçtiğini geri yükler ve diğer örneği unutur. Bunun yerine sürekli bir düzen istiyorsan, yani bu makinenin başka bir örneğin anlık görüntülerini kendi deposuna bir zamanlamayla çekmesini, bu Örnekler sayfasındaki **Çeken** rolüdür.

### Bu sunucunun çektikleri {#restore-what-was-fetched}

Bu sunucunun çektiği her kaynağın, Geri yükleme sayfasında **Diğer kaynaklar** altında kendi **… kaynağından alındı** satırı vardır. Çekilen bir kopya kaynağın anahtarıyla değil, bu sunucunun kendi anahtarıyla kilitlidir, bu yüzden hiçbir şey sorulmaz: kaynağın anahtarı onu açmaz. Klasör kümeleri ve ZFS veri kümeleri seçtiğin bir klasöre gider. Konteynerler ve VM'ler bir klasöre düz dosyalar olarak geri gelir, çünkü bir çekme anlık görüntüleri taşır, tanımları değil. Onları yeniden oluşturmak için Acil durum sihirbazında kaynağın kurtarma kitini ve öz yedeğini kullan.

Ağı bu sunucuda olmayan bir konteyner, örneğin düz bir Docker ana makinesinde bir Unraid `br0` ağı, satırının altında bir ağ seçici gösterir. BombVault onu seçtiğin ağda, diğer ağlarıyla birlikte oluşturur. Sabit IP ve MAC adresi eski ağa aitti ve bırakılır, yani yeni ağ onları atar.

## Anahtar bekçisi {#key-keeper}

Anahtar bekçisi, bu sunucunun kurtarma kitinin mühürlü bir kopyasını tutan grubundaki bir örnektir. Kopyayı kendisi açamaz. Bu sunucu bir gün kaybolursa, grubuna katılan yeni bir kurulum bekçiden kopyayı ister ve teslim koduyla açar.

![Teslim kodu penceresi: dört grupta yirmi karakter, bir kez gösterilir.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Teslim kodu penceresi: dört grupta yirmi karakter, bir kez gösterilir.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Örnekler sayfasında onu tutacak örneğin kartında **Anahtar bekçisi**'ni aç ve **Emanet et**'i seç. Emanet etmek bir giriş parolası ister.
2. Bu sunucu **Teslim kodu**'nu bir kez gösterir: beşerli dört grupta yirmi harf ve rakam. Yaz ve **Kodu not ettim** ile onayla. Bu sunucu kodu saklamaz, yalnızca onun için bir emanet mühürlemek için gerekeni.
3. Bekçi artık emaneti tutuyor ve kit değiştiğinde bu sunucu onu yeniler. Bu örneğin kartındaki **Anahtar emaneti**, her bekçide **Bu sunucunun emaneti**'ni durumuyla ve diğerleri için **Bu sunucunun sakladığı emanetler**'i listeler. **Yeni kod oluştur** kodu değiştirir. Her bekçi yeni emaneti aldığında yeniye geçer.

O gün üç şeye ihtiyacın var: grubunun on iki kelimesi, teslim kodu ve birinin **İzin ver**'e basması gereken bekçinin arayüzüne erişim. Acil durum sihirbazında **Grup üzerinden**'i seç, emaneti seç, iste ve teslim kodunu yaz. Kod yeni sunucuda kullanılır ve bekçiye gönderilmez. Bekçide istek, **Anahtar emaneti** altında kimin istediği, o örneğin orada daha önce görülüp görülmediği ve ne sıklıkla istediğiyle görünür. **İzin ver** mühürlü kopyayı verir ve izin vermek bir giriş parolası ister. Yeni kurulumun İzin ver'den sonra onu almak için 30 dakikası vardır, kimsenin yanıtlamadığı bir isteğin süresi bir gün sonra dolar ve çok sık isteyen bir örnek bir saat beklemek zorundadır.

Kod olmadan kopyayı hiç kimse açamaz, bekçi dahil. Kod kaybolduysa bu sunucu hâlâ çalışırken yeni bir tane oluştur. Sunucu zaten gittiyse kurtarma kiti ve yazılmış APP_KEY yine çalışır. Bekçi bir emaneti her an silebilir ve sahibinin o zaman yeniden istemesi gerekir.

## Kurtarma kiti {#recovery-kit}

Bu, çalışan hiçbir BombVault olmadığında bile felaket kurtarmayı mümkün kılan parçadır.

**Ayarlar, Geri yükleme** altındaki **Acil durumlar için** kartında **Kurtarma kitini indir**, sana **anahtarı**, **türetilmiş restic parolasını**, her bölümün deposuyla her **hedefi**, yeni bir kurulumun onlara ulaşması için gereken oturum açma bilgilerini ve **tam restic komutlarını** verir, böylece herhangi bir makinede restic CLI ile doğrudan geri yükleyebilirsin. Bir giriş parolası ister. Kit anahtarı şifreleme kapalıyken de basar, çünkü konteyner ve VM tanımları her zaman onunla mühürlenir. Bir devralmadan sonra devralınan yedek anahtarını ve bu sunucunun APP_KEY'ini basar, ikincisine yeni bir kurulum yalnızca ayarları geri getirmek için ihtiyaç duyar. Genel bakıştaki **Kurtarma kitini sakla** kartı, onu saklayana kadar sana hatırlatır.

!!! danger "Kurtarma kitini sunucu dışında sakla"
    Kit yedeklerini çözen gizli bilgiyi içerir. Onu sunucudan ayrı güvenli bir yerde sakla (bir parola yöneticisi, bir kasada basılı bir kopya). Kurtarma kiti ve anahtar emaneti olmadan hem BombVault'u hem `APP_KEY`'i kaybedersen şifreli yedeklerin kurtarılamaz.

!!! warning "En yeni anlık görüntü her zaman geri yüklenecek olan değildir"
    restic 0.17'den beri `restic snapshots` her anlık görüntünün boyutunu gösterir. Veri kaybından sonra en yeni anlık görüntü boşaltılmış olan olabilir, bu yüzden öncekilerden çok daha küçük bir anlık görüntüyü geri yükleme. Fidye yazılımından sonra olağan boyutta şifrelenmiş olan olabilir. BombVault hâlâ çalışıyorsa önce **Anormallikler** sayfasına bak: son iyi yedeği adlandırır ve geri yükleme penceresi onu önceden seçer. Bir geri yükleme BombVault'un hiçbir anormallik verisine ihtiyaç duymaz ve saklama duraklaması yalnızca daha fazla anlık görüntü tutar.

### Kiti mühürlemek {#sealing-the-kit}

**Ayarlar, Sistem** altında **Dışa aktarmaları age ile şifrele**'yi açtıysan kit de onunla mühürlenir ve `bombvault-recovery-kit.md.age` olarak iner. İkili değil ASCII zırhlıdır, bu yüzden hâlâ düz metindir: bir parola yöneticisine yapıştırmak ya da yazdırmak eskisi gibi çalışır, içerik anahtarın olmadan okunamaz.

!!! warning "age anahtarını kitin içinde saklama"
    Mühürlü bir kiti açmak için age **özel** anahtarına ihtiyacın var. Onu kitin kendisine bağlı olmayan bir yerde tut, yoksa kurtarman gereken bir yerine iki şeyin olur. Mühürleme, kit tam olarak denetlemediğin bir yerde saklandığında (paylaşılan bir parola yöneticisi, bulut notları, bir ofisteki çıktı) buna değer. Kendi kasandaki bir kit zaten kasa tarafından korunur.

    Şifreleme açıkken ve kullanılabilir bir alıcı yapılandırılmamışken indirme tümden reddedilir. BombVault ana anahtarı hiçbir zaman açık olarak vermeye düşmez.

### Kit elinde değilse {#if-you-do-not-have-the-kit-to-hand}

Parola hiçbir yerde saklanmaz, yedeklerin anahtarından **hesaplanır**, bu yüzden onu yalnızca anahtar ve bir kabukla kendin yeniden üretebilirsin:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Bu, sabit `bombvault:restic-repo` dizesi üzerinde, onaltılık `APP_KEY`'in ham baytlarıyla anahtarlanmış HMAC-SHA256'dır ve 64 küçük harfli onaltılık karakter olarak basılır. Aynı değer kitte türetilmiş restic parolası olarak listelenir. Bu, kitin sen yokken bir yerde olduğu gün içindir. Başka bir sunucunun yedeklerini devralmış bir sunucuda, devralınan yedek anahtarını, yani o sunucunun eski APP_KEY'ini kullan.

!!! warning "Alınan bir depo için GÖNDEREN örneğin anahtarını kullan"
    Buraya bir site dışı kopyayla gelen bir depo, onu gönderen makine tarafından **onun** anahtarıyla oluşturuldu. Alıcı makinenin anahtarından türetmek restic'in reddedeceği bir parola üretir, bu da tam olarak bozuk bir depo gibi görünür ve değildir. Alınan bir depoda `restic check`'in tekrar tekrar parola istemesinin olağan nedeni budur.

Kurtarma tanımları her deponun **içinde** durduğundan (`<repo>/def`, `<repo>/vm-def`), kopyalanmış bir depo klasörü tamamen kendi kendine yeterlidir, bu yüzden kit artı depo, çıplak donanım geri yüklemesinin ihtiyaç duyduğu her şeydir. Site dışı bir kopya ve çekilen bir kopya anlık görüntüleri tanımlar olmadan tutar: dosyaları geri verirler, bir konteyner ya da VM ise öz yedekten yeniden oluşturulur.

## Bir veritabanı dökümünü geri almak {#database-dumps}

Bir veritabanı dökümü, konteynerler deposunda `dbdump:<container>` etiketli, tek dosya olan `/dbdump/<container>.sql` dosyasını tutan kendi geri yükleme noktasıdır. BombVault bunları konteynerin **Yedekler** sekmesinde listeler, indirir ve içe aktarır. Aşağıda aynı adımlar, BombVault'un olmadığı gün için, yalnızca restic ile.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Her dökümdeki `dbversion:` ve `dbname:` etiketleri hangi sunucu sürümünden geldiğini ve hangi veritabanlarını tuttuğunu söyler. Eksiksiz bir dosya `-- PostgreSQL database cluster dump complete` ya da `-- Dump completed` ile biter.

Onu aynı ya da daha yeni sürümdeki (PostgreSQL) ya da aynı ana sürümdeki (MySQL ve MariaDB) bir konteynere, başlayabilsin diye boş bir veri klasörüyle bir kez başlatıldıktan sonra içe aktar. Ana makinenin bir veritabanı istemcisine ihtiyacı yoktur, konteynerde var:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Tam bir dökümden tek bir veritabanı için MySQL ve MariaDB istemci komutunda `--one-database <name>` alır. Bir PostgreSQL dökümünde veritabanı başına bir bölüm vardır, her biri bir `\connect <name>` satırıyla başlar: o bölümü kendi dosyasına kopyala ve veritabanını oluşturduktan sonra `-d <name>` ile içe aktar.

!!! warning "Bir root dökümü sunucunun kullanıcılarını taşır"
    root olarak alınan tam bir MySQL ya da MariaDB dökümü `mysql` sistem veritabanını içerir, bu yüzden içe aktarmak yeni sunucunun hesaplarını, root'un parolası dahil, dökümdekilerle değiştirir. PostgreSQL'de konteynerin oluşturduğu kullanıcı için `role ... already exists` beklenen ve zararsızdır.
