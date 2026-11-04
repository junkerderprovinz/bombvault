# Genel bakış

Genel bakış kenar çubuğunun ilk sayfasıdır ve her sabah bakılacak sayfadır. Bir bakışta iki soruyu yanıtlar: her şey yedeklendi mi ve senin ilgilenmen gereken bir şey var mı? Bir durum başlığı ve dört kartla başlar, daha fazla kart **Özelleştir** altında bekler.

![Birkaç gün sonra genel bakış: durum başlığı, bir uyarılı İlgi bekliyor, her bölümün koruması, hedefler ve geçmiş.](assets/screenshots/overview/home-dark.png#only-dark)
![Birkaç gün sonra genel bakış: durum başlığı, bir uyarılı İlgi bekliyor, her bölümün koruması, hedefler ve geçmiş.](assets/screenshots/overview/home-light.png#only-light)

## Durum başlığı {#status-head}

Başlık sayfanın en üstünde durur ve hep orada kalır. Halkası ne kadarının korunduğunu gösterir, örneğin **43 öğeden 41'i korunuyor**, yanındaki satır da durumu söyler:

| Başlık ne diyor | Anlamı |
|---|---|
| **Her şey yedeklendi** | Açık olan her bölümün her ögesinin güncel bir yedeği var. |
| **Yedeklendi, notlarla** | Her şey yedeklendi, bazı çalıştırmalar bir not bıraktı, örneğin okunamayan dosyalar. |
| **Henüz her şey korunmuyor** | Bazı ögelerin güncel bir yedeği yok, örneğin ilkini bekledikleri için. |
| **3 şey ilgi bekliyor** | Bir şey başarısız oldu ya da bir karar bekliyor. **İlgi bekliyor** kartı bunu listeler. |
| **Şu anda yedekleniyor** | Bir yedekleme çalışıyor. Halka, **Durdurma**, **Okuma** ve **Başlatma** istasyonlarıyla bir ilerleme halkasına dönüşür, tam yedekleme ögeleri giderken sayar, örneğin **Şu anda yedekleniyor: 43'ten 12'si**. **Çalıştırmayı göster** çalıştırmayı açar. |
| **Site dışı kopyalama sürüyor** | Bir hedefe kopyalama çalışıyor. `restic copy` yüzde bildirmediği için yüzde göstermez. |

Satırın altında son çalıştırmanın ne zaman olduğu ve sonraki planlı çalıştırmanın ne zaman başlayacağı yazar.

## İlgi bekliyor {#needs-you}

Bu kart bakman gerekenleri en yeni üstte listeler, liste boşken **Yapılacak bir şey yok.** der. Her satırın kendi çıkış yolu vardır:

- **Başarısız bir yedek**, ögeyi ve nedenini söyler. **Hataları göster** restic'in kendi çıktısını gösterir, **Yeniden dene** yedeği bir kez daha başlatır.
- **Site dışı kopyalama başarısız**, bölümü ve ne zamandan beri olduğunu söyler. **Şimdi çoğalt** yeniden dener.
- **plex'te anormallik**, **Denetle** ile bulguya götürür. Kritik bir bulgu açıkken saklama o ögenin eski yedeklerini korur.
- BombVault'un bulduğu ve henüz hiçbir şeyin yedeklemediği **Yeni** ögeler, örneğin yeni kurduğun bir konteyner, **Yedeklemeye başla** ya da **Yedeklenmesin** sunar.
- Uygulaması tüm bekleme süresince meşgul kaldığı için **pes eden bir yedek**, bkz. [Uygulama meşgulken](backups.md#app-busy).
- **Başarısız bir ZFS replikası**, sade bir dille, örneğin sunucuda zfs komutu olmadığı, sunucunun çıktısı **Görüntüle** altında.
- **Yedekler durdu: anahtar artık açılamıyor**, bu sunucu başka bir sunucunun anahtarını devraldığında ve `APP_KEY` sonradan değiştiğinde görünür. Bkz. [Sorun Giderme](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Kurtarma kitini sakla**, kiti indirip **Güvenle sakladım**'a basana kadar durur.

## Koruma durumu {#protection-status}

Açık olan her bölümün dört denetimli bir satırı vardır:

| Denetim | Şu olunca yeşile döner | Görebileceğin diğer ifadeler |
|---|---|---|
| **Yerel** | son yedek güncel | henüz yedek yok, zamanlama yok, son çalıştırma başarısız, yedek gecikmiş, her öge korunmuyor, yedek yakında gerekecek |
| **Site dışı** | tesis dışındaki son kopya güncel | site dışı kopya yok, kopyalama sürüyor, kopya gecikmiş, kopyalama duraklatıldı, henüz kopya yok, replika başarısız |
| **Silmeye karşı koruma** | bir kurcalama testi karşı tarafın silmeyi reddettiğini kanıtladı | yok, yok ve risk kabul edildi, henüz kanıtlanmadı, test başarısız, denetimi kapalı olduğu için kanıtlanmadı |
| **Deneme geri yüklemesi** | site dışı kopyadan bir geri yükleme geçti | hiç çalıştırılmadı, çalışıyor, başarısız, gecikmiş, yalnızca elle, bu bölüm için mümkün değil |

Bir bölüm yalnızca doğrulanmış gerçeklerle yeşil olur. Bu sunucudaki bir kopya site dışı sayılmaz, **Silmeye karşı koruma olmadan kabul et** olarak işaretlediğin bir hedef kehribar noktası yerine gri bir halka gösterir, böylece bölümün rengini diğer denetimleri belirler.

Zamanlanmış bir bölüm **Henüz yedek yok** der ve ilk çalıştırmasının zamanı gelene kadar nötr kalır. Sonrasında **Gecikmiş** der ve bir kez uyarır. Hiçbir zamanlama ögelerine ulaşmadığı için yedekleyecek bir şeyi olmayan bir bölüm **Zamanlanmamış** der ve hiçbir zaman gecikmiş sayılmaz.

Bir bölüme tıklamak, her denetimin ayrıntılı yazıldığı, saydığı ögelerin ve **Deneme geri yüklemesini başlat** ya da **Deneme geri yüklemesini yeniden çalıştır**'ın bulunduğu penceresini açar. Ayrıca **Budama kuralı ayarlı** ya da **Budama kuralı yok** der ve bölümün kendi klasörünün saklama kuralı da site dışı kuralları kadar sayılır.

![Konteynerler bölümünün dört denetimi ayrıntılı yazılmış koruma penceresi.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Konteynerler bölümünün dört denetimi ayrıntılı yazılmış koruma penceresi.](assets/screenshots/overview/protection-window-light.png#only-light)

## Hedefler {#storage-locations}

Her hedef burada bir kap olarak durur: doluluk düzeyi, büyüme hızı, örneğin **Büyüyor 4,2 GB/hafta**, ve bu hızla ne zaman dolacağı, örneğin **yaklaşık 31 haftada dolacak**. S3, B2 ve rest-server'lar ne kadar yer kaldığını bildirmez, bu yüzden **Boş alan bilinmiyor** ve yalnızca büyümelerini gösterirler. Tıklamak hedefin sayfasına götürür. Bkz. [Hedefler](offsite-recovery.md#destinations).

## Geçmiş {#history}

**Geçmiş**'in iki sekmesi vardır. **Çalıştırmalar** her çalıştırmayı sonucu, süresi ve nedeniyle listeler, tıklamak onu açar. **Günlük** her satırı BombVault'un yazdığı gibi saatiyle gösterir ve tek bir güne daraltılabilir. Bir yedek tamamlandığı halde bir şeyi dışarıda bıraktığında satır turuncuya döner ve neyin dışarıda kaldığını söyler.

## Özelleştir altında daha fazla kart {#customize}

Sayfanın üstündeki **Özelleştir** kartları düzenlemene izin verir. Bir kartı çubuğundan sürükleyerek taşı, sağ alt köşesindeki noktadan boyutlandır ya da gizle. Gizli kartlar **Gizli kartlar** altında bekler, **Göster** birini geri getirir. Düzenlerken kartlar hafifçe sallanır, **Bitti** bunu sonlandırır. Düzen bu tarayıcıda kaydedilir.

Bu kartlar gizli başlar:

- **Anormallikler**: açık bulgular, bkz. [Anormallikler](anomalies.md).
- **Yedeklenmedi**: bu sunucuda hiçbir zamanlanmış yedeğin kapsamadığı her şey, nedeniyle birlikte, örneğin **hiç eklenmedi**, **zamanlamaya dahil değil** ya da **bölümünün tüm hedefleri kapalı**. Konteynerleri doğrudan Docker'dan okur, bu yüzden BombVault'un hiç görmediği birini de bulur.
- **Takvim**: son 90 gün, gün başına bir kare. Bir güne tıklamak **Geçmiş** altındaki günlükte o günün satırlarını gösterir.
- **Depolama**: her bölümün ne kadar yer kapladığı ve verisinin ne kadar iyi tekilleştirildiği.
- **Genel bakış**: tüm bölümlerdeki en eski anlık görüntü, sonraki yedeğin ne zaman çalışacağı ve sonuncunun nasıl geçtiği.
- **İstatistikler**: kaç konteyner ve VM olduğu, kaç hata olduğu ve kaç planın etkin ya da duraklatılmış olduğu.
- **Host Entegrasyonu**: BombVault'un ana makinede neye ulaştığı ve neye ihtiyaç duyduğu, örneğin Docker soketi, libvirt, restic ve rclone. Bir şey bağlanmamış gibi göründüğünde buradan başla.

![Özelleştir modu: kartlar sürüklenebilir, boyutlandırılabilir ve gizlenebilir, gizliler aşağıda bekler.](assets/screenshots/overview/customize-dark.png#only-dark)
![Özelleştir modu: kartlar sürüklenebilir, boyutlandırılabilir ve gizlenebilir, gizliler aşağıda bekler.](assets/screenshots/overview/customize-light.png#only-light)

## BombVault nasıl sayar {#counting}

Her kart ve başlık aynı şekilde sayar: açık bölümlerin hâlâ kurulu olan ve senin **Yedeklenmesin** olarak işaretlemediğin ögeleri. Bir öge ancak bir zamanlama ona ulaşıyorsa ve yedeği varsa korunmuş sayılır. Tek başına eski bir yedek sayılmaz, bu yüzden zamanlamadan çıkardığın bir konteyner, onu geri koyana ya da **Yedeklenmesin** olarak işaretleyene kadar korunmuyor görünür.

## İlk adımlar {#first-steps}

Yeni bir kurulumda genel bakış **İlk adımlar** kartıyla başlar: **Bölümleri seç**, **Hedef ekle** ve **İlk yedekleme**. İlk yedekleme bitene kadar durur. [Başlarken](getting-started.md#first-run) bunu adım adım anlatır.
