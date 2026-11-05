# Geri yükleme

Her geri yükleme **Geri yükleme** sayfasında başlar: bir bölüm seç, geri istediğini ara ve bir yedek seç. Tek bir pencere her türden ögeyi geri yükler ve adım adım sorar. Varsayılan olarak orijinalde hiçbir şey değişmez, çünkü yedek yanında geri gelir.

![Bölümlerin kutucuk olarak, aramanın ve Stack'ler kartının göründüğü Geri yükleme sayfası.](assets/screenshots/restore/page-dark.png#only-dark)
![Bölümlerin kutucuk olarak, aramanın ve Stack'ler kartının göründüğü Geri yükleme sayfası.](assets/screenshots/restore/page-light.png#only-light)

## Geri yükleme sayfası {#page}

- **Bölümler**, bölümleri kutucuk olarak listeler, her birinde kaç ögenin anlık görüntüsü olduğu yazar.
- **Neyi geri getirmek istiyorsun?** seçilen bölümün ögelerinde arar. Her satır en yeni anlık görüntüsünü gösterir, **Ögeyi aç** ögenin sayfasına götürür.
- **Stack'ler**, en az iki yedeklenmiş konteyneri olan Compose projelerini listeler, örneğin veritabanı ve Redis'iyle `immich`. Arama bir stack'i adıyla ya da konteynerlerinden herhangi biriyle bulur.
- **Şu an çalışıyor**, bir geri yüklemeyi, penceresini kapattıktan sonra da, çalışırken gösterir.
- **Acil durum**, kaybolmuş ya da yeniden kurulmuş bir sunucu için Acil durum sihirbazını başlatır, bkz. [Acil durum sihirbazı](offsite-recovery.md#emergency).
- **Diğer kaynaklar**, başka bir BombVault'un yedekleri için **Eski bir yedek klasöründen** ve **Appdata.Backup eklentisinden içe aktar**'ı barındırır.

![Telefonda Geri yükleme sayfası.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Telefonda Geri yükleme sayfası.](assets/screenshots/restore/page-phone-light.png#only-light)

## Geri yükleme penceresi {#window}

Bir öge üzerine tıklamak, adıyla birlikte **Geri yükle**'yi açar. Adım adım sorar ve her adım başlamadan önce değiştirilebilir:

1. **Yedek**: önerilen anlık görüntü önceden seçilidir. Bu en yenisidir, açık bir anormallikten sonra ise ondan önceki sonuncusu. Daha önceki bir geri yüklemeden önce alınan yedek, o geri yüklemenin değiştirdiği durumu tutar, bu yüzden başka bir yedek olmadıkça atlanır. Eskileri listede durur, **Karşılaştır** ikisi arasındaki farkı gösterir.
2. **Kaynak**: bir anlık görüntü birden çok yerde duruyorsa dosyaların nereden geldiği. **Yerel** en hızlısıdır.
3. **Ne**: **Tümü** ya da istediğin dosya ve klasörleri işaretlediğin **Tekil dosyalar**.
4. **Nereye**: hedef. Seçenekler ögenin türüne bağlıdır:

    | Hedef | Ne yapar |
    |---|---|
    | **Orijinalin yanına** | Varsayılan. Bir konteyner kendi portlarıyla `<name>-restored` adlı ikinci bir konteyner olarak geri gelir, bir VM'in diskleri ve NVRAM'ı `<folder>-restored` adlı bir klasöre düşer ve bir klasör kümesi `<path>-restored` olarak geri gelir. Orijinalde hiçbir şey değişmez. |
    | **Orijinali değiştir** | Şu an orada olanın üzerine yazar. Varsayılan olarak açık **Önce mevcut durumu yedekle** ile BombVault önce bugünkü durumu yedekler, böylece geri dönebilirsin. O yedek başarısız olursa geri yükleme başlamaz. Bu, sunucuda artık olmayan bir konteyner ya da VM için de çalışır: geri yüklemenin üzerine yazdığı klasörler ayrıca yedeklenir. |
    | **Bir klasöre** | Yalnızca dosyalar, göz atmak için. |
    | **ZIP olarak indir** | Flash sürücü, yeni bir bellek için. BombVault çalışan flash sürücüye hiçbir zaman yazmaz. |
    | **Yeni bir veri kümesine** ve **Veri kümesinin içine geri yükle** | ZFS veri kümeleri, bkz. [ZFS veri kümeleri](zfs-datasets.md#restore). |

5. **Ne olur**, geri yüklemenin ne yapacağını sözcüklerle söyler, bir sahne de seçtiğin hedefi gösterir.

**Geri yükle** açılmadan önce pencere deponun yanıt verdiğini, anahtarın onu açtığını, anlık görüntünün orada olduğunu ve hedefte yer olduğunu denetler. Bir denetim başarısızken düğme kilitli kalır, içindeki (i) hangisi olduğunu söyler.

Pencere ayrıca geri yüklemenin şu an orada olana göre neyi değiştirdiğini de gösterir: yeni, değiştirilen ve değişmeyen dosyalar, istenirse listeyle, ve hedefte yedekte olmayan ve yerinde kalan dosyalar. restic bunu dosyaları okumadan, boyut ve değişiklik zamanından bir deneme çalıştırması olarak hesaplar, çok büyük bir ağaç 30 saniye sonra durur ve bunu söyler. Konteynerler ve VM'ler için, geri yüklemenin yeniden oluşturduğu ayarları çalışanlarla karşılaştırır: image ve etiket, portlar, değişken adları ve birimler ya da bellek, vCPU'lar, diskler ve ağ.

Bazı şeyler bir geri yüklemeyi hiçbir şeyi değiştirmeden önce durdurabilir. Bir konteynerin sabit IP'si ve yayınlanmış portları boş olmalı, ağı da var olmalıdır. Unraid'de her yerinde geri yükleme, bağlı bir havuzda ya da paylaşımda olmayan bir klasörü reddeder, çünkü dosyalar belleğe düşer ve yeniden başlatmadan sonra kaybolurdu. Yerinde bir geri yükleme, bind mount'u yazdığı bir klasöre uzanan her diğer konteyneri adlandırır, örneğin "bu yolu nextcloud-db de kullanıyor", ama bu yalnızca bir uyarıdır.

Birkaç seçenek geçerli oldukları yerde görünür: bir konteyneri ya da VM'i yeniden çalışmadan önce incelemek için **Sonra durmuş bırak** ve yedeğin alındığı GPU'ya ya da çalışma ortamına sahip olmayan bir ana makinede **GPU ve çalışma ortamı olmadan geri yükle**.

Geri yükleme arka planda çalışır. Pencereyi kapatabilirsin, ilerleme **Şu an çalışıyor** altında da görünür. Uzun bir geri yükleme yüzde gösterir ve iptal edilebilir, iptal edilen bir geri yükleme başarısız değil iptal edilmiş olarak kaydedilir. Konteyner dururken iptal edilen ya da konteyneri kaldıramayan bir geri yükleme henüz hiçbir şeyi geri yüklememiştir, bu yüzden konteyner yeniden başlar. Birimlerini `--volumes-from` ile başka bir konteynerden alan bir konteyner onları korur ve o diğer konteyner yokken geri yükleme başlamaz.

![Tekil dosyaların işaretli olduğu Ne adımındaki geri yükleme penceresi.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Tekil dosyaların işaretli olduğu Ne adımındaki geri yükleme penceresi.](assets/screenshots/restore/window-what-light.png#only-light)

### Bir klasöre geri yükleme nereye gider {#folders}

**Ayarlar, Geri yükleme** altındaki **Geri yükleme klasörleri**, **Varsayılan geri yükleme klasörü**'nü, istersen bir **Her bölüm için alt klasör**'ü ve tek bir bölüm için **İstisna**'yı ayarlar. Seçilen dosyalar doğrudan hedef klasöre düşer: tek bir dosya yalnızca o dosya olarak geri gelir, bir klasör içeriğiyle klasör olarak, farklı klasörlerden seçilenler ise yalnızca ortak klasörün altındaki yollarını korur. BombVault, bağlı depolamada olmayan bir hedefi reddeder, çünkü Unraid'de böyle bir klasör bellekte yaşar ve yeniden başlatmadan sonra gider, ayrıca kendi veri klasörüne hiçbir zaman geri yüklemez.

## Örnek: tek bir dosyayı geri al {#example-single-file}

Biri dün gece Nextcloud'un `config.php` dosyasını düzenledi ve Nextcloud artık başlamıyor. 03:00'teki gece yedeğinde iyi dosya hâlâ var. Bütün konteyneri değil, o tek dosyayı istiyorsun.

1. **Geri yükleme**'yi aç, **Konteynerler** kutucuğunu seç ve **Neyi geri getirmek istiyorsun?** alanına `nextcloud` yaz.
2. `nextcloud`'a tıkla. Pencere, 03:00 yedeği önerilen olarak önceden seçilmiş şekilde açılır.
3. **Kaynak**'ı **Yerel**'de bırak.
4. **Ne** altında **Tekil dosyalar**'ı seç. Klasörleri `config`'e kadar aç ve `config.php`'yi işaretle.
5. **Nereye** altında **Bir klasöre**'yi seç ve bir klasör belirle, örneğin ana makinede `/mnt/user/restore` olan `user/restore`.
6. **Ne olur**'u oku: dosya hedef klasöre düşer, konteynerde hiçbir şey değişmez. **Geri yükle**'ye bas.

Pencere **Bitti: dosyalar /mnt/user/restore içinde** ile biter. Dosyayı üstündeki klasörler olmadan `/mnt/user/restore/config.php` olarak bulursun. Bozuk olanla karşılaştır, appdata klasörüne geri kopyala ve Nextcloud'u yeniden başlat.

Dosyayı doğrudan yerine koymak için 5. adımda **Orijinali değiştir**'i seç. **Önce mevcut durumu yedekle**'yi açık tut, böylece BombVault yazmadan önce bozuk durumu yedekler, onaylamak için `nextcloud` yaz ve **Geri yükle**'ye bas. Seçilen dosya yerine geri yazılırken Nextcloud durdurulur ve sonra yeniden başlatılır. Yalnızca işaretlediğin dosyalar yazılır. Yarıda başarısız olan bir geri yükleme Nextcloud'u durmuş bırakır, çünkü okuduğu dosya yarım yazılmış olabilir, ve çalıştırma bunu söyler.

![Orijinali değiştir seçili, Ne olur adımındaki geri yükleme penceresi.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Orijinali değiştir seçili, Ne olur adımındaki geri yükleme penceresi.](assets/screenshots/restore/window-replace-light.png#only-light)

## Türe göre neler geri gelir {#kinds}

**Konteynerler**, kayıtlı tanımlarından Docker API'si üzerinden yeniden oluşturulur, entrypoint, hostname, durdurma ayarları, birimler ve bağlamalar, kaynak sınırları, log sürücüsü, DNS ayarları ve eski bağlantılarıyla. Bir konteyner yedeğin alındığı andaki image'iyle geri gelir, sonradan çekilmiş daha yeni biriyle değil, eksik bir image önce **Ayarlar, Geri yükleme** altındaki **Container registry'leri**'ndeki oturum açma bilgisiyle çekilir. GPU'su ya da çalışma ortamı (`--gpus`, `--runtime=nvidia`) de geri gelir. Bu sürücüsü ya da çalışma ortamı olmayan bir ana makinede geri yükleme bunu söyler ve **GPU ve çalışma ortamı olmadan geri yükle** sunar. Eksik olan ya da geri yüklenen başlarken durmuş olan bir konteynere bağlantı dışarıda bırakılır ve çalıştırma geçmişi bunu söyler. Konteyner kaldırıldıktan sonra başarısız olan bir geri yükleme onu durmuş olarak geri koyar.

Bir yedek, konteynerin artık seçili tutmadığı klasörleri tutabilir, örneğin bir klasör kaybolmadan önce alınmış daha eski bir yedek. Olmayan ya da boş olan bir klasör geri gelir. Şu an dosya tutan bir klasör olduğu gibi kalır, çünkü işaretini bilerek kaldırmış olabilirsin, ve geri yüklemeden önceki denetim ile çalıştırma geçmişi onu adlandırır. Onu **Tekil dosyalar** ya da **Bir klasöre** ile geri al.

**VM'ler**, XML'leri SSH üzerinden yeniden tanımlanır ve diskleri ile UEFI NVRAM'ları, VM silinmiş olsa bile, yeniden bağlanır. Unraid anlık görüntülü VM'ler ve zvol üzerindeki diskler için bkz. [VM yedekleri](vm-backup-ssh-setup.md).

**Klasör kümeleri**, restic'in sakladığı şekliyle sahiplik, izinler ve zaman damgalarıyla geri gelir. Hedef yalnızca sahipliği reddettiğinde, `/mnt/user`'ın yaptığı gibi, çalıştırma bunu söyleyen bir notla biter. BombVault sonra dosyaları yedekle karşılaştırır ve gelmeyen bir dosya geri yüklemeyi başarısız kılar.

**Flash sürücü** çalışan `/boot`'a hiç dokunmaz. Geri yükleme, Unraid USB Creator'a bırakılmaya hazır `flash-<id>.zip` adlı bir indirmedir. ZIP `APP_KEY`'i tutar, bu yüzden onu indirmek bir giriş parolası ve örneğin kendisinde bir oturum açma gerektirir. Pencere ayrıca her flash yedeğindeki eklentileri sürümü ve boyutuyla listeler ve tek birini çalışan flash'a geri koyabilir: `.plg` dosyasını, `config/plugins` altındaki klasörünü ve yedeğin tuttuğu paket dosyalarını. Unraid onu sonraki açılışta ya da hemen **Plugins, Install Plugin** altında kurar.

**ZFS veri kümeleri**nin kendi hedefleri vardır ve ZFS özelliklerini korur, bkz. [Geri yükleme](zfs-datasets.md#restore).

**Öz yedek** yalnızca BombVault'u yeniden başlatan Acil durum sihirbazı üzerinden geri gelir, böylece canlı veritabanının üzerine açık bir tanıtıcı varken asla yazılmaz.

**Veritabanı dökümleri** hiçbir zaman kendiliğinden geri yüklenmez. **Veritabanı dökümleri** altındaki bir satır indirilebilir, bir klasöre kaydedilebilir ya da yeni başlatılmış bir veritabanına içe aktarılabilir. Bkz. [Veritabanı dökümleri](backups.md#database-dumps).

## Birden çok ögeyi bir kerede geri yüklemek {#several}

Geri yükleme sayfasında ögeleri Yedekler listesinde satır seçer gibi seç: Ctrl+tıklama, Shift+tıklama, Ctrl+A, uzun basma ya da **Seç**. **Geri yükle** sonra hepsi için tek bir pencere açar.

- **Hangi yedek**: **Önerilen**, her öge için kendi penceresinin önereceği yedeği alır. **Hepsi için tek an**, her öge için seçtiğin bir **An**'dan ya da daha önceki en yeni yedeğini alır, örneğin bir şeyin ters gittiği akşamdan önceki.
- **Nereye**: **Orijinalin yanına** ya da **Orijinali değiştir**, **Önce mevcut durumu yedekle** ile.

Her öge tek bir geri yüklemedeki gibi denetlenir, sonra birbiri ardına çalışırlar. Her satır kendi durumunu gösterir: **Bekliyor**, **Çalışıyor**, **Geri yüklendi**, **Başarısız** ya da **Dışarıda bırakıldı**. **Bundan sonra durdur**, çalışan geri yüklemenin bitmesine izin verir ve başka başlatmaz. Flash sürücü indirilecek bir ZIP olarak gelir, ZFS veri kümeleri ve öz yedek tek tek geri yüklenir.

![Birden çok ögeyi geri yükleme: her öge için ilerlemesiyle bir satır.](assets/screenshots/restore/several-dark.png#only-dark)
![Birden çok ögeyi geri yükleme: her öge için ilerlemesiyle bir satır.](assets/screenshots/restore/several-light.png#only-light)

## Bir stack'i geri yüklemek {#stacks}

**Stack'ler** kartındaki bir stack'e tıklamak **Stack'i geri yükle**'yi açar. Her üyeyi en son yedeğinden aynı anda geri yükler ve **Geri yükledikten sonra bağımlılık sırasına göre başlat** ile bunları `depends_on` sırasıyla başlatır: önce veritabanları ve önbellekler, sonra uygulama. Bir üyenin proje klasörü seçtiğin yerde yoksa pencere onun yerine Yerel'deki durumu sunar.

BombVault önce üyeleri ve proje klasörünü denetler ve işe yaramayacak bir geri yükleme hiçbir şeyi değiştirmez. Aksi halde çalışan üyeleri, uygulamaları veritabanlarından önce olmak üzere durdurur, sonra proje klasörünü yazar, sonra üyeleri geri yükler. Proje klasörü geri gelmezse üyeler yine de geri yüklenir ama durmuş kalır, çünkü klasör yarım yazılmış olabilir ve bir üyenin bind mount'ları genellikle orada durur. Her üyenin çalıştırması bunu söyler.

Yalnızca veritabanı dökümleri olan, dosya yedeği olmayan ve konteyneri ile adlandırılmış birimi artık bulunmayan bir veritabanı verisi olmadan geri gelir. Denetim bunu geri yüklemeden önce söyler. Veritabanı durmuş kalır, ona bağlı konteynerler de öyle, böylece hiçbir uygulama boş bir veritabanında kurulumunu çalıştırmaz. Onu başlat, bir döküm içe aktar ve diğerlerini başlat. Bkz. [Veritabanı dökümleri](backups.md#database-dumps).

## Diğer kaynaklar {#other-sources}

- **Eski bir yedek klasöründen**, artık çalışmayan başka bir BombVault'un deposunu, bir diskte ya da site dışı bir hedefte, kurtarma kiti ya da yazılmış `APP_KEY` ile açar. Oturum salt okunurdur ve yalnızca bellekte yaşar. Bkz. [Eski bir yedek klasöründen](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Appdata.Backup eklentisinden içe aktar**, eklentinin yedek klasörünü okur. Her konteyner arşivi, eklentinin yaptığı tarihle o konteynerin bir geri yükleme noktası olur. Daha önce içe aktarılan arşivler atlanır ve arşivlerin kendisi yalnızca okunur. Geri yüklemenin tanımı olsun diye konteynerin önce BombVault'ta bir yedeğe ihtiyacı vardır. Saklama içe aktarılan geri yükleme noktalarına dokunmaz, bu yüzden artık gerekmediğinde birini kendin sil.

## Bir geri yüklemeden sonra {#after}

Orijinalin yanına geri yüklenen bir kopya **Yedekler** altında hariç tutulmuş olarak listelenir, yani açık bir görev sayılmaz. Saklamak istiyorsan zamanlamaya ekle. Yedeklendiğinde çalışan bir konteyner ya da VM çalışır şekilde geri gelir, durmuş olan durmuş kalır. Unraid'de geri yüklenen bir konteyner Docker sekmesinde normal, düzenlenebilir bir uygulama olarak yeniden görünür, bir VM de diskleri ve NVRAM'ı bağlı olarak VM Manager'da.
