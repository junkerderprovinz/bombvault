# ZFS veri kümeleri

ZFS veri kümeleri, konteynerler ve VM'ler gibi **Yedekler** listesindeki ögelerdir. Bir öge, bir veri kümesi ve altındaki her veri kümesidir. Her yedek için BombVault tüm ağacın tek bir ZFS anlık görüntüsünü alır, böylece içindeki her veri kümesi aynı anda yakalanır. Sonra her veri kümesinin dosyalarını o anlık görüntüden okur ve bir klasörü sakladığı gibi restic ile saklar, anlık görüntüyü de hemen sonra kaldırır. Yedekler tekilleştirilir, hepsine göz atabilirsin ve tek tek dosyalar geri yüklenebilir.

Yedek restic'e hiçbir zaman bir akış göndermez ve hiçbir veri kümesini geri sarmaz. BombVault yalnızca kendi oluşturduğu anlık görüntüleri yok eder. İsteğe bağlı [replika](#replica) `zfs send` kullanan tek yerdir: veri kümelerini ikinci bir ZFS sunucusuna kopyalar ve yedeğe dokunmaz.

## Gereksinimler {#requirements}

- **Bu sunucuya SSH bağlantısı.** ZFS veri kümeleri, VM yedekleriyle aynı anahtarı, ana makineyi ve kullanıcıyı kullanır. VM yedekleri zaten çalışıyorsa bu da çalışır. Aksi halde [SSH üzerinden VM yedekleme kılavuzunu](vm-backup-ssh-setup.md) izle. Şablon alanları **Host SSH: Address**, **Host SSH: Port** ve **Host SSH: User** diye adlandırılır.
- **O ana makinede `zfs` komutu.** Unraid 6.12 ve üstünde ve TrueNAS SCALE'de vardır.
- **Erişim Kipi Read/Write - Slave ile `/mnt` olarak eşlenmiş Host Data.** Bu şablon varsayılanıdır. Bir veri kümesinin anlık görüntüsü, veri kümesinin `.zfs/snapshot` klasöründe ancak BombVault başladıktan sonra görünür, bu yüzden konteynerin ana makinenin sonradan yaptığı bağlamaları alması gerekir.
- **`/mnt` altında bağlı veri kümeleri.** Unraid'de havuzlar `/mnt/<pool>` konumundadır, yani bu zaten böyledir.

Bölümü **Ayarlar, Genel** altında aç (ZFS veri kümeleri). Yedekler sayfasında **Diğer** altındaki **Bu sunucuyla ZFS bağlantısı** sonra **Bu sunucuyla bağlantı**'yı açar. SSH bağlantısını test eder, bağlandığı kullanıcıyı ve ana makineyi adlandırır ve bir şey eksikse neyin eksik olduğunu söyler. Bağlantı kapalıyken Yedekler listesi bunu **Bağlantıyı göster** ile söyler ve genel bakıştaki **Host Entegrasyonu** kartı aynı sonucu gösterir.

## Ögeler ve alt veri kümeleri {#items-and-children}

Yedekler sayfasında **Ekle**'yi, sonra **Bir ZFS veri kümesi**'ni seç. Liste sunucudan gelir. Yedeklemek istediğin şeyin en üstündeki veri kümesini seç, örneğin `cache/appdata`, ve öge onu ve altındaki her veri kümesini kapsar.

- **Yeni alt veri kümeleri kendiliğinden katılır.** Ögenin altında sonradan oluşturulan bir veri kümesi sonraki çalıştırmada yedeklenir ve o çalıştırma onu yeni olarak adlandırır. İlk yedeği onu bir kez tam okur. Sonrasında yalnızca değişiklikler okunur.
- **Tek tek alt ögeleri dışarıda bırakabilirsin.** Bir alt ögeyi ögenin Ayarlar sekmesinde kapat, altındaki her şeyle birlikte dışarıda kalır. Sunucuda artık var olmayan dışarıda bırakılmış bir alt veri kümesi öyle işaretlenir ve listeden kaldırılabilir.
- **Okunamayan alt veri kümeleri atlanır, ama sessizce değil.** Çalıştırma onları listeler, öge kaç tanesinin atlandığını gösterir ve genel bakıştaki **Yedeklenmedi** kartı her birini korunmuyor sayar. Çalıştırma yine de geri kalan her şeyi yedekler ve atlanan bir alt veri kümesi yüzünden başarısız olmaz. Nedenler [neden kodları tablosundadır](#reason-codes): bağlı olmayan bir veri kümesi, `canmount=off`, `legacy` ya da bağlama noktasının olmaması, yüklenmemiş bir şifreleme anahtarı, kapalı anlık görüntü erişimi ya da BombVault'un göremediği bir bağlama noktası.
- **Atlanan bir veri kümesi alt öğelerini beraberinde götürmez.** Yalnızca başka veri kümelerini tutan bir `canmount=off` veri kümesi atlanır ("yalnızca yapı" olarak gösterilir) ve bağlı alt öğeleri yedeklenir. Anahtarı yüklü olmayan şifreli bir veri kümesi, aynı anahtarı paylaşan alt öğeleriyle birlikte atlanır.
- **VM diski ya da sistem verisi olan alt veri kümeleri kapalı başlar**, ekleme penceresinde, anahtarın yanında nedeniyle birlikte. Bir havuzun tamamını eklemek, içerdiğini listeleyen bir onay ister.

### Birimler {#volumes}

Bir birim (zvol) dosyalar yerine sanal bir disk tutar ve bir ZFS ögesi onu hiçbir zaman yedeklemez.

- Bir VM'in kullandığı bir birim o VM ile yedeklenir.
- Hiçbir VM'in kullanmadığı bir birim (bir iSCSI extent, ayırdığın bir disk) **BombVault tarafından yedeklenmez**. Ekleme penceresi ve Yedekler sayfası bu birimleri sayar ve bunu söyler. Sonraki bir sürüm onları yedekleyecek.

Bir ögenin ağacındaki birimler atlanır ve her çalıştırmada adlandırılır.

### Docker'ın depolaması {#docker-storage}

Docker'ın ZFS depolama sürücüsüyle her image katmanı `legacy` bağlama noktalı bir veri kümesidir. Ekleme penceresi bunları üst öğe başına tek satıra katlar. 20'den fazla böyle veri kümesi tutan bir ağaç öge olamaz: bir anlık görüntüsü varken Docker image katmanlarını kaldıramaz. Bunun yerine altındaki veri kümelerini ekle, örneğin `appdata`.

### Ögeler hiç çakışmaz {#overlap}

Bir veri kümesi yalnızca bir ögeye ait olabilir. BombVault mevcut bir ögenin içinde duran ya da birini içerecek yeni bir ögeyi reddeder. Birkaç alt ögeyi tek bir üst ögede birleştirmek için önce alt ögeleri sil ve yedeklerini tutmayı seç, sonra üst ögeyi ekle. Her veri kümesi geçmişini kendi adı altında tutar, bu yüzden sonraki yedek eski ögelerin bıraktığı yerden devam eder ve her şeyi yeniden okumaz.

## Konteynerleri durdurma ve anlık görüntü etrafındaki komutlar {#consistency}

Çalışan bir veritabanının anlık görüntüsü ani bir elektrik kesintisi gibidir: veritabanı genellikle toparlanır, ama toparlanması gerekir. Her öge bunun için iki şey yapabilir ve ikisi de tüm yedeği değil yalnızca anlık görüntü anını kapsar.

- **Anlık görüntü için bu konteynerleri durdur.** BombVault listelenen konteynerleri durdurur, anlık görüntüyü alır ve hemen yeniden başlatır. Bir bağımlılık düzeyindeki konteynerler paralel durur, bağımlı olanlar önce, bu yüzden tüm pencere genellikle birkaç saniyedir. Çalıştırma ne kadar sürdüğünü gösterir. Yedek sonra uygulamalar zaten yeniden çalışırken dondurulmuş anlık görüntüyü okur. Yalnızca çalışan konteynerler durdurulur.
- **Anlık görüntüden önce ve sonra bir komut.** Seçtiğin bir konteynerin içinde çalışır, örneğin hiçbir şeyi durdurmadan anlık görüntüden hemen önce bir veritabanını veri kümesine dökmek için. Anlık görüntüden önceki komut başarısız olursa yedek başarısız olur ve anlık görüntü alınmaz. Anlık görüntüden sonraki başarısız bir komut çalıştırmada gösterilir ama yedeği başarısız kılmaz.

Bir şeyler ters gittiğinde ne olur:

- Bir konteyner durdurulamazsa BombVault zaten durdurduklarını başlatır ve yedek konteyneri adlandırarak başarısız olur. Çalışan uygulamaların anlık görüntüsüne hiçbir zaman düşmez.
- Durdurma, çalışan bir konteyner yedeğinin bitmesini bekler (elle çalıştırma için 30 dakikaya kadar, zamanlanmış için yedekleme zaman sınırına kadar), böylece ikisi aynı konteyneri aynı anda hiç durdurmaz ve başlatmaz.
- İlk konteyner durmadan önce BombVault hangilerini durdurduğunu yazar. BombVault pencerenin içinde öldürülürse, sonraki başlangıcında o konteynerleri yeniden başlatır, bir bildirim gönderir ve öge, başlatamadığı herhangi bir konteyner için kırmızı bir not gösterir.

Otomatik [veritabanı dökümleri](backups.md#database-dumps) bir ZFS ögesiyle değil, bir konteynerin kendi yedeğiyle çalışır. Konteyneri yalnızca veri kümesi aracılığıyla yedeklenen bir veritabanı döküm almaz, bu yüzden ona burada bir komut ver.

Bir konteyner bu listede olabilir ve aynı zamanda bir konteyner olarak yedeklenebilir. Verisi o zaman iki depoda iki kez saklanır ve tam yedekleme onu iki kez durdurur. Öge bunu söyler.

## Geri yükleme {#restore}

Ögeyi aç ve **Geri yükle**'yi seç ya da **Geri yükleme** sayfasında seç. Geri yükleme penceresi yedeği, sonra **Veri kümesi**'ni sorar. Varsayılan olarak ögenin en üst veri kümesidir. **Ne** altında her şeyi ya da yalnızca seçtiğin dosya ve klasörleri geri yüklersin. **Nereye** şunları sunar:

- **Veri kümesinin içine geri yükle.** Yedekteki dosyalar veri kümesinin bağlama noktasına yazılır. Aynı ada sahip dosyaların üzerine yazılır, diğerleri kalır. Veri kümesi hiçbir zaman geri sarılmaz ya da değiştirilmez. BombVault veri kümesinin bağlı, görünür ve yazılabilir olduğunu başlamadan önce bir kez, yazmadan hemen önce yeniden kontrol eder. İçinde bir alt veri kümesinin bağlı olduğu yerde hiçbir şey yazılmaz: alt veri kümesi dosyalarını, sahibini ve izinlerini korur ve kendi yedeğinden geri yüklenir.
- **Bir klasöre.** `/mnt` altında bir klasör seç. BombVault klasörün bağlı bir havuzda ya da paylaşımda olduğunu ve yeterli boş alan olduğunu kontrol eder. Bu SSH bağlantısı olmadan ve artık var olmayan veri kümeleri için çalışır.
- **Yeni bir veri kümesine.** Henüz var olmayan bir veri kümesini adlandır. Pencere `<dataset>-restored` önerir. BombVault onu yedekte saklanan ZFS özellikleriyle oluşturur ve içine geri yükler, bkz. [Yeni veri kümesi olarak geri yükleme](#new-dataset).

Veri kümesi olarak **Bu yedeğin bütün veri kümeleri**'ni seçmek, ağacın her veri kümesini seçtiğin klasörün kendi alt klasörüne yazar. O yedekte atlanan veri kümeleri adlandırılır. Başka bir BombVault'un deposundan geri yüklemek için Geri yükleme sayfasındaki **Eski bir yedek klasöründen**'i kullan. Her zaman bir klasöre geri yükler: bir yedeğin tüm veri kümeleri, her biri kendi alt klasörüne, ya da ağacın tek bir veri kümesi, bütün ya da seçilen dosyalar.

Ögenin durdurulacak konteynerler listesi veri kümesinin içine geri yükleme için de sunulur. O konteynerler tüm geri yükleme boyunca durmuş kalır ve konteyner yedekleri bu arada bekler.

### Güvenlik anlık görüntüsü {#safety-snapshot}

Bir veri kümesine yazmadan önce BombVault yalnızca o veri kümesinin `bombvault-prerestore-<time>` adlı bir ZFS anlık görüntüsünü alır. **Önce şimdiki durumun ZFS anlık görüntüsünü sakla** varsayılan olarak açıktır. Kapatmak ikinci bir onay ister. Anlık görüntü alınamazsa hiçbir şey geri yüklenmez.

BombVault bir güvenlik anlık görüntüsünü kendiliğinden hiçbir zaman silmez. Öge onları yaşları ve boyutlarıyla, her biri bir **Sil** eylemiyle listeler ve en eskisi 30 günden eskiyse uyarır, çünkü havuzda silinmiş ve değişmiş veriyi tutar.

Bir geri yüklemeden sonra geri dönmek için veri kümesinin içindeki `.zfs/snapshot/bombvault-prerestore-<time>` konumundan tek tek dosyaları kopyala. `zfs rollback <dataset>@bombvault-prerestore-<time>` yalnızca o veri kümesinin en yeni anlık görüntüsü olduğu sürece çalışır. `zfs rollback -r` daha yeni her anlık görüntüyü, otomatik olanlar dahil, siler.

### Yeni veri kümesi olarak geri yükleme {#new-dataset}

BombVault her yedekle her veri kümesinin yerel olarak ayarlanmış ZFS özelliklerini saklar: sıkıştırma, kayıt boyutu, kota, rezervasyon, atime, xattr, acltype, büyük küçük harf duyarlılığı ve kendi kullanıcı özelliklerin. Devralınan ve salt okunur değerler dışarıda bırakılır, çünkü kendiliğinden geri gelirler. BombVault bunları saklamaya başlamadan önceki yedeklerde yoktur.

- **Yeni bir veri kümesine**, her saklanan özellikle `zfs create` çalıştırır. Büyük küçük harf duyarlılığı, normalleştirme ve utf8only yalnızca bu şekilde ayarlanabilir. Kotalar ve rezervasyonlar dosyalardan sonra ayarlanır, bu yüzden onları reddedemez. Bağlama noktası, kopya orijinalle çakışmasın diye dışarıda bırakılır, `canmount`, `readonly` ve şifreleme ayarları da geri yükleme yazabilsin diye. Şifreli bir veri kümesinin altındaki yeni bir veri kümesi onun şifrelemesini devralır. Yeni olanın üstündeki veri kümesi var olmalıdır. Yeni veri kümesi oluşturulduktan sonra bir şey başarısız olursa sunucuda kalır, çünkü BombVault hiçbir zaman bir veri kümesini yok etmez.
- **Veri kümesinin içine geri yükle**, saklanan özellikleri geri yüklemenin yanında listeler. **Bu özellikleri de ayarla**, var olan bir veri kümesinin hâlâ kabul edebildiklerini, herhangi bir dosya yazılmadan önce ayarlar. Kotalar ve rezervasyonlar dosyalardan sonra ayarlanır, bu yüzden onları reddedemez. O anahtar olmadan veri kümesi ayarlarını korur.

## Yedekte ne var {#contents}

Yedekte: her yedeklenen veri kümesinin dosyaları ve klasörleri, restic'in sakladığı şekliyle sahiplikleri, kipleri, zaman damgaları ve genişletilmiş öznitelikleriyle, ve her veri kümesinin yerel olarak ayarlanmış ZFS özellikleri.

Yedekte olmayanlar:

- her veri kümesinin en üst klasörünün kendisinin sahibi ve kipi (altındaki her şey dahildir). Veri kümesinin içine geri yükleme mevcut en üst klasörü olduğu gibi bırakır, bir klasöre geri yükleme onu `0755` kipiyle oluşturur,
- mevcut ZFS anlık görüntüleri,
- atlanan ya da dışarıda bırakılan alt veri kümeleri,
- birimler.

Yeni bir havuza geri yüklemek için havuzu oluştur ve her veri kümesini yeni bir veri kümesine geri yükle. TrueNAS'ın SMB veri kümelerinde kullandığı gibi NFSv4 ACL'lerinin beklediğin şekilde geri gelip gelmediği henüz doğrulanmadı, bu yüzden onlara güvenmeden önce bir geri yüklemeyi kendi verinde dene.

## Şifreli veri kümeleri {#encryption}

Şifreli bir veri kümesi yalnızca anahtarı yüklüyken yedeklenir. Aksi halde bir uyarıyla atlanır. Anahtarı `zfs load-key` ile yükle ve veri kümesini bağla. BombVault veriyi çözülmüş okur ve şifreli olan restic deposuna saklar. BombVault'ta şifrelemeyi kapattıysan o depo şifreli değildir.

## Replika {#replica}

Bir replika, bir ögenin veri kümelerinin ikinci bir ZFS sunucusundaki kopyasıdır. BombVault onu `zfs send` ve `zfs receive` ile güncel tutar. İlk çalıştırma her şeyi gönderir, sonrasında yalnızca değişen bloklar yol alır. Diğer sunucuda kopyayı hemen bağlayabilirsin.

Bir replika yedeğin yerini hiçbir zaman almaz. Eski sürümler, tek tek dosyalar ve denetim yine yedeklerden gelir ve replika yalnızca senin ayarladığın kadar anlık görüntü tutar. Güncel bir replika tesis dışı bir kopya sayılır, ama bir yedek değil.

Replika yedeği beklemez. **Her yedekten sonra**, ögenin yedek çalıştırmasından sonra demektir, o çalıştırma başarılı olsa, başarısız olsa ya da yapacak bir şey bulamasa da. ZFS bölümünün açık bir hedefi yoksa ve ögenin kendi deposu yoksa, ögenin bir yedek çalıştırması yalnızca replikayı gönderir ve Replika kartı bunu **Ne zaman**'ın yanındaki balonda söyler. Yedekler listesi, ögenin başlığı ve çalıştırma geçmişi replikayı gösterir: çalışırken **Çoğaltılıyor**, bittiğinde saatle **Çoğaltıldı**.

Replika kartındaki **Yalnızca replika, yedek yok**, tüm ZFS bölümünün replika göndermesini ve yedek almamasını sağlar. Kendi klasörü ve site dışı hedefler anahtarlarını korur, bu yüzden kapatmak onlara yeniden yedekler ve her ZFS ögesinin kartı bölümün anahtarını olduğu gibi gösterir. Böyle bir öge bir yedek yerine ilk replikasını bekler, **Şimdi yedekle** **Şimdi çoğalt** olarak sunulur ve ögenin konteynerleri durur ve komutları bir yedekteki gibi anlık görüntü etrafında çalışır. Replikaları birkaç günde bir zamanlama, gecikme uyarısı ve kapsam için sayılır.

Onu ögenin Ayarlar sekmesindeki **Replika** kartında aç. Kart her veri kümesini ve birimi durumuyla listeler ve **Şimdi çoğalt** bir çalıştırma başlatır. Bir replika çalıştırmasının kendi kilidi vardır, bu yüzden uzun bir ilk aktarım yedekleri hiçbir zaman geciktirmez.

| Alan | Neyi ayarlar |
|---|---|
| **Çoğaltılacak yer** | Hedef: bir ZFS sunucusu (**SSH üzerinden** işaretli) ya da **eşleşmiş bir örnek**. **ZFS sunucusu ekle** yenisini kurar. |
| **Ne zaman** | **Her yedekten sonra**, yani ögenin yedek çalıştırmasından sonra, ya da **Kendi planı**. |
| **Hedefte sakla** | ZFS sunucusunda kaç anlık görüntü kalır: **Kısa**, **Dengeli**, **Uzun** ya da **Özel değerler**. Eşleşmiş bir örnek bunu kendisi belirler. |
| **Bu sunucuda sakla** | En yeni replika anlık görüntülerinin kaçı bu sunucuda da kalır. 0 ya da 1 yalnızca sonuncuyu tutar. |
| **Yalnızca replika, yedek yok** | Tüm ZFS bölümü replika gönderir ve yedek almaz. |

![Bir ZFS ögesinin Replika kartı.](assets/screenshots/zfs/replica-dark.png#only-dark)
![Bir ZFS ögesinin Replika kartı.](assets/screenshots/zfs/replica-light.png#only-light)

Yedekler sayfasında ZFS kutucuğu seçiliyken yedekleme eylemlerinin yanındaki **Şimdi tümünü çoğalt**, her ZFS ögesinin replikasını birbiri ardına gönderir. Toplu işi sunucu yürütür, bu yüzden sayfadan ayrıldığında da sürer ve çoğaltamayan bir öge adlandırılır ve atlanır.

### Bir ZFS sunucusuna gönderme {#replica-push}

ZFS ve SSH olan her makine alabilir, örneğin ikinci bir Unraid ya da bir TrueNAS. BombVault'un orada çalışması gerekmez.

1. **Ayarlar, Eşleştirme** altında **BombVault olmayan ZFS sunucusu** içinde **SSH ile bağlan**'ı ya da ögenin **Replika** kartında **ZFS sunucusu ekle**'yi seç.
2. Adresi, kullanıcıyı ve portu gir. Pencere BombVault'un açık anahtarını gösterir. Onu sunucudaki kullanıcının `~/.ssh/authorized_keys` dosyasına ekle. Unraid'de bu **Users, root, SSH keys** altındadır. **Display Settings** ayarına bağlı olarak Users başlık menüsünde ya da Settings altında bulunur. TrueNAS'ta kullanıcıyı **Credentials, Users** altında düzenle. Bu replika anahtarıdır. VM ve ana makine SSH bağlantısının anahtarından farklıdır, bu yüzden o zaten yetkilendirilmiş olsa bile bunu ekle.
3. Bağlantıyı test et. Pencere sonra sunucunun havuzlarını listeler. Birini seç ve varsayılanı `<pool>/bombvault-replica` olan kökü ayarla.
4. Yeni sunucuyu ögenin **Replika** kartında seç. Sunucunun o zaman Örnekler sayfasında kendi kartı olur ve **Aç** sayfasına götürür.

root ile başka bir şey gerekmez. Kendi kullanıcısı olan biri, pencerenin de gösterdiği, hedefin havuzunda şu izinlere ihtiyaç duyar:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Kaynakta aynı türden bir kullanıcının ögenin en üst veri kümesinde şunlara ihtiyacı vardır:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

Bu yönde, ögeyi tutan BombVault sunucuda yazabilen bir anahtarı da tutar.

BombVault, sunucunun ilk bağlantıda gösterdiği ana makine anahtarını sabitler ve sonradan farklı olanı reddeder. Sunucu yeniden kurulduysa sunucunun sayfasındaki **Ana makine anahtarını unut**'a bas, sonraki bağlantı yeni anahtarı sabitler.

Bu sunucunun ana makinesi ZFS sunucusuna kendisi ulaşabiliyorsa, `zfs send`'i replika anahtarı ve sabitlenmiş ana makine anahtarıyla doğrudan kendi ssh'ine yönlendirir, böylece akış BombVault'tan geçmez. Aksi halde akış BombVault üzerinden gider.

### Eşleşmiş bir örneğe gönderme {#replica-receive}

Eşleşmiş bir BombVault replikayı kendisi alabilir. Kimse diğer ana makineye SSH erişimi almaz ve hiçbir anahtar bir `authorized_keys` dosyasına girmez.

1. Ögenin **Replika** kartında eşleşmiş örneği hedef olarak seç ya da Örnekler sayfasında o örneğin kartında **ZFS sunucusu** rolünü aç ve oradan ZFS ögelerini seç. Kart, yanıt verene kadar **Onay bekliyor** gösterir.
2. Alıcı örnekte **Örnekler**'i aç. İstek, gönderen örneğin kartında bir rozet olarak görünür ve **Ayrıntılar**'ı onu listeler. O sunucuda havuzu ve kökü ve kaç anlık görüntünün kalacağını seç, sonra **İzin ver** ya da **Reddet**'e bas.
3. **İzin ver**'den sonra kaynak, diğer her hedefte olduğu gibi kendi planıyla gönderir.

Alıcı örnek yalnızca onayın kapsadığını kabul eder: ögenin veri kümeleri, kendi köküne. `zfs receive`'i kendisi çalıştırır ve kaynağın orada hiçbir şeyi silmesinin ya da geri sarmasının yolu yoktur. Böylece kopya, birinin ele geçirdiği bir kaynaktan sağ çıkar. Alıcı örnek kendi saklamasını tutar. Kaynak yalnızca sorarken bir kural önerir.

Alıcı örnekteki **Erişimi iptal et** onayı her an sonlandırır ve kaynağa haber verir, kaynak o zaman onayın geri çekildiğini gösterir ve durur. Alıcı örneğin zaten tuttukları orada kalır. Reddedilen bir istek reddedilmiş kalır. Daha fazla veri kümesi ya da bir iptalden sonra bir istek yeniden sorar.

Alıcı örneğin HTTPS üzerinden yanıt vermesi gerekir. Kaynak düz bir http adresine hiçbir şey göndermez, çünkü belirteç ve veri ağda şifresiz yol alırdı. Her aktarım ayrıca alıcı havuzun onda birini boş bırakır ve içine girmeden önce durur, böylece tek bir kaynak alıcı sunucunun havuzunu dolduramaz.

### Verinin düştüğü yer {#replica-target}

Her veri kümesi `<root>/<server>/<pool>/<path>` konumuna düşer. Sunucu klasörü, ilk aktarımda sabitlenen kaynak örneğin adıdır, böylece aynı havuz adına sahip iki sunucu birbirinin yoluna hiç çıkmaz. Örneğin `tower` adlı bir sunucunun `cache/appdata` veri kümesi `backup/bombvault-replica/tower/cache/appdata` konumunda biter.

BombVault yeniden kurulduktan sonra hedef, bu BombVault'un daha önce doldurduğu klasörü reddeder. Replika kartı o zaman **Klasörü devral**'ı sunar ve sonraki replika içindeki kopyalardan devam eder. Onu yalnızca başka hiçbir BombVault hâlâ o klasöre replika yapmıyorsa devral.

Hedefteki kopya salt okunurdur. Varsayılan olarak bağlı değildir, bu yüzden o sunucuda hiçbir şeyi hiçbir zaman örtmez. SSH ile bağlanan bir ZFS sunucusunun sayfasında bir **Sunucuda salt okunur olarak bağla** anahtarı vardır: açıkken BombVault her çalıştırmadan sonra kopyaları orada salt okunur bağlar, böylece dosyalarını hedefte okuyabilir ve kopyalayabilirsin. Şifreli kopyalar anahtarları yüklenene kadar kilitli kalır ve kapatmak hiçbir şeyi ayırmaz. Replikayı alan eşleşmiş bir örnek onu bağlanmamış tutar. ZFS özellikleri, bağlama noktası, `sharenfs` ve `sharesmb` hariç, yanında yol alır. `reservation` ve `refreservation` rezervasyonları da geride kalır, bu yüzden kopya yalnızca verisinin ihtiyaç duyduğu yeri alır.

### Neler giriyor {#replica-contents}

Ögenin yedeklediği her şey girer, altındaki yedeğin atladığı birimler de. Ögede kapattığın bir alt veri kümesi dışarıda kalır. Bir çalıştırmanın tüm veri kümeleri, yedekteki gibi, tek bir anlık görüntüden gelir.

### Anlık görüntüler ne kadar kalır {#replica-retention}

Hedefte yeni bir replika 7 günlük ve 3 haftalık anlık görüntü tutar. Bunun yerine **Kısa**, **Dengeli** ya da **Uzun**'u seç ya da **Özel değerler**'i ayarla. Orada yalnızca `bombvault-replica-<14 digits>` adlı anlık görüntüler kaldırılır ve iki tarafın paylaştığı en yeni olan hiçbir zaman.

Kaynakta BombVault son replika anlık görüntüsünü ve gönderdiği her durum için bir yer imi tutar. Yer imleri yer kaplamaz. Sonraki aktarım onlardan başlar.

Kaynakta da daha fazlasını tutmak için Replika kartında **Bu sunucuda sakla**'yı ayarla, örneğin 7'ye. O kadar en yeni replika anlık görüntüsü, hedefin tuttuğunun yanında, her replikadan sonra bu sunucuda kalır. 0 ya da 1 yalnızca sonuncuyu tutar. Sonuncunun bir tutması (hold) vardır, böylece sonraki aktarımın tabanı yanlışlıkla yok edilmez. Eskilerde yoktur. Her anlık görüntü kendinden sonra değişen blokları tutar, bu yüzden tuttuğun her biri havuzda yer kaplar. Replikayı kapatmak ya da başka bir hedefe geçmek veya ögeyi silmek onları bu sunucudan da kaldırır.

Tutulan bir anlık görüntünün dosyaları, veri kümesine ya da replikaya dokunmadan, veri kümesinin içindeki `.zfs/snapshot` klasöründen ya da bir klondan hemen okunabilir veya kopyalanabilir:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Bir klon, içinde bir şey değişene kadar yer kaplamaz. İşin bitince onu `zfs destroy cache/appdata-20261006` ile yok et, çünkü klonu olan bir anlık görüntü kaldırılamaz.

Son replika anlık görüntüsüne geri dönmek için burada yalnızca `zfs rollback cache/appdata@<latest>` gerekir, hedefte hiçbir şey değil. Tüm veri kümesini daha eski bir anlık görüntüye geri sarmak, ondan sonraki her anlık görüntüyü, bu sunucuda ve hedefte, yok eder. Sonuncusu tutulur, bu yüzden önce onun tutmasını kaldır, sonra veri kümesini burada geri sar:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

Hedefteki kopyanın da geri dönmesi gerekir, ama her zaman aynı anlık görüntüye değil, çünkü kendi saklama kuralları onu bırakmış olabilir. Replikayı bir kez çalıştır. Hedef daha yeni anlık görüntüler tuttuğu için o veri kümesinde durur ve Replika kartı ile çalıştırma geçmişi hedefin geri sarılacağı anlık görüntüyü adlandırır: iki tarafın hâlâ paylaştığı en yeni olanı. Kopyayı ZFS sunucusunda ya da replikayı alan eşleşmiş örneğin ana makinesinde ona geri sar, örneğin:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` tek bir veri kümesi üzerinde çalışır, bu yüzden geri istediğin her alt veri kümesi için geri sarmaları tekrarla. Ondan sonra **Şimdi çoğalt** paylaşılan anlık görüntüden devam eder.

### Replikadaki şifreli veri kümeleri {#replica-encryption}

Şifreli bir veri kümesi ham gönderilir. Hedefte şifreli kalır ve hedef anahtarı hiçbir zaman görmez. Anahtarı güvende tut: bir geri yüklemeden sonra kopyayı açmak için ona ihtiyacın var ve onsuz bir replika okunamaz.

### Replikayı kullanmak {#replica-use}

Ögenin **Yedekler** sekmesini aç ve anlık görüntülerini tutan yerler arasında **ZFS replikası** satırına tıkla. Sayfa hedefteki anlık görüntüleri listeler ve komutları gerçek adlarınla gösterir.

Eski bir duruma bakmak için hedefte bir anlık görüntüyü klonla. Bir klon bir şey değişene kadar yer kaplamaz ve replika dokunulmadan kalır:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Kaynak çökerse kopyayı hedefte normal, yazılabilir bir veri kümesine çevir:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault sonra yeni bir ilk çalıştırma başlatana kadar o veri kümesine replika yapmayı bırakır.

Bir durumu kaynağa geri almak için sayfadaki **Yeni veri kümesi olarak geri getir**'e bas. BombVault anlık görüntüyü orijinalin yanında, `<dataset>-bombvault-restore-` artı bir zaman damgası adlı yeni bir veri kümesine gönderir. Orijinalin üzerine hiçbir zaman yazmaz.

### Örnek: ikinci bir Unraid makinesine replika {#example-replica}

`tower` sunucusu `cache/appdata` veri kümesini bir ZFS ögesi olarak yedekliyor. `192.168.1.20` adresindeki, BombVault olmayan ikinci bir Unraid makinesinin `backup` adlı bir havuzu var ve istediğin zaman okuyabileceğin bir kopya tutmalı.

1. **Yedekler**'i, sonra `cache/appdata`'yı, sonra **Ayarlar** sekmesini aç. **Replika** kartında **Çoğaltılacak yer** altında **ZFS sunucusu ekle**'yi seç.
2. `192.168.1.20`'yi, `root` kullanıcısını ve `22` portunu gir. Pencere replika anahtarını gösterir. İkinci makinede onu **Users, root, SSH keys** altına yapıştır ve **Apply**'a bas. Bu anahtar Host SSH anahtarından farklıdır, bu yüzden o zaten orada olsa bile bunu ekle.
3. Bağlantıyı test et. `backup` havuzunu seç ve önerdiği kökü, `backup/bombvault-replica`, olduğu gibi bırak.
4. Karta döndüğünde yeni sunucu **Çoğaltılacak yer** altında seçilidir. **Ne zaman**'ı **Her yedekten sonra**'da bırak, **Hedefte sakla**'yı 7 günlük ve 3 haftalık anlık görüntüsünde bırak ve **Bu sunucuda sakla**'yı `7` yap.
5. **Şimdi çoğalt**'a bas. İlk çalıştırma her şeyi gönderir, bu yüzden 1 Gbit/s'lik bir hat üzerinden 300 GB en az 40 dakika sürer. Kart ve Yedekler listesi **Çoğaltılıyor**, sonra saatle **Çoğaltıldı** der. Sonraki çalıştırmalar yalnızca değişen blokları gönderir.

Veri kümeleri şimdi ikinci makinede `backup/bombvault-replica/tower/cache/appdata` konumunda, salt okunur durur. Genel bakışta ZFS bölümünün **Site dışı** değeri replikayı tesis dışı bir kopya sayar.

Geçen Salı'dan tek bir dosyayı almak için üç yolun var:

- **Bu sunucuda**, çünkü **Bu sunucuda sakla** yedi anlık görüntü tutuyor:

    ```
    cp /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261103014100/plex/Preferences.xml /mnt/user/restore/
    ```

- **İkinci makinede**, hedefteki bir anlık görüntünün klonuyla. **ZFS replikası** satırına tıkladığında ögenin **Yedekler** sekmesi komutları gerçek adlarınla gösterir. İşin bitince klonu `zfs destroy` ile kaldır.
- **Bir bağlamayla**, ZFS sunucusunun sayfasında **Sunucuda salt okunur olarak bağla**'yı açarak. Sonraki çalıştırmadan sonra kopyalar ikinci makinede salt okunur bağlı durur.

Replikanın tuttuğundan eski sürümler, sonraki örneğin gösterdiği gibi, yine yedeklerden gelir.

### Örnek: bir ZFS yedeğinden tek bir dosya {#example-zfs-file}

1. **Geri yükleme**'yi aç, **ZFS** kutucuğunu seç ve `cache/appdata`'ya tıkla.
2. Yedeği seç ve **Veri kümesi** altında dosyayı tutan veri kümesini seç.
3. **Ne** altında **Tekil dosyalar**'ı seç ve dosyayı işaretle.
4. **Nereye** altında **Bir klasöre**'yi seç ve `/mnt` altında `user/restore` gibi bir klasör seç. Bu SSH bağlantısı gerektirmez ve artık var olmayan bir veri kümesi için de çalışır.
5. **Geri yükle**'ye bas. Dosya üstündeki yol olmadan doğrudan o klasöre düşer.

## Artakalan anlık görüntüler {#leftover-snapshots}

Bir yedeğin anlık görüntüsü `<dataset>@bombvault-<14 digits>` diye adlandırılır, örneğin `cache/appdata@bombvault-20260924021500` (UTC). BombVault onu yedekten hemen sonra kaldırır. Bu başarısız olursa, örneğin veri kümesi meşgul olduğu ya da BombVault durdurulduğu için, BombVault onu kaldırır:

- o ögenin sonraki yedeğinden önce,
- BombVault başladığında her öge için, alan kapalıyken de,
- ögeyi sildiğinde,
- ögede kaç tane kaldığını gösteren **Şimdi kaldır**'a bastığında.

Yalnızca tam olarak `bombvault-` artı 14 basamakla eşleşen adlar kaldırılır. Güvenlik anlık görüntülerine, kendi anlık görüntülerine ve otomatik anlık görüntülere hiçbir zaman dokunulmaz. Birini elle kaldırmak için:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Bir replikanın anlık görüntüsü `<dataset>@bombvault-replica-<14 digits>` diye adlandırılır ve artakalan değildir. Daha yeni bir replika çalıştırması onun yerini alana ya da **Bu sunucuda sakla** onu tuttuğu sürece kaynakta, saklama tuttuğu sürece hedefte durur. Temizleyici ona hiçbir zaman dokunmaz, çünkü yalnızca `bombvault-` ardından tam olarak 14 basamakla eşleşir.

## Anormallikler {#anomalies}

Boşaltılmış bir alt veri kümesi büyük bir ağacın toplamını pek değiştirmez, bu yüzden anormallik algılama bir ögenin her veri kümesini kendi başına izler: boyutunun, dosya sayısının, yeni verisinin ve restic süresinin her birinin kendi geçmişi vardır. O geçmiş veri kümesinin adına aittir, bu yüzden ağaç sonradan başka bir ögeyle yedeklense de kalır.

Önceki çalıştırmanın yedeklediği ve bu çalıştırmanın okuyamadığı bir veri kümesi, ögenin seçimi değişmediği sürece boşaltılmış sayılır. Bu, yüklenmemiş bir anahtarı, bağlı olmayan bir veri kümesini ve ağaçtan kaybolmuş birini kapsar. Kendin hariç tuttuğun bir alt veri kümesi seçimi değiştirir, bu yüzden geçmişi baştan başlar. Kaybolan veriyle ilgili bir bulgu açıkken saklama yalnızca o veri kümesinin eski yedeklerini tutar ve ağacın geri kalanını her zamanki gibi budar.

Her veri kümesinin ögenin izlemesinde kendi satırı vardır, bu **Anormallikler** sayfasında **İzleme** ile açılır ve ögenin **Doğrulama** sekmesinde durur. Açık bulgular her veri kümesinin yanında görünür. Bir bulgudaki bağlantı, geri yükleme penceresini veri kümesinin son iyi yedeğinde açar. Bir çalıştırmanın bitip bitmediği tüm öge için değerlendirilir, çünkü bir çalıştırma bir bütün olarak başarılı ya da başarısız olur.

Denetimlerin kendileri [Anormallikler](anomalies.md) altında anlatılır. [MCP sunucusu](mcp.md) üzerinden bağlanan bir asistan bir ZFS ögesinin geri yükleme noktalarını listeleyebilir, yedeğini başlatabilir ve bulguları okuyabilir, ama bir bulguyu onaylamak **Anormallikler** sayfasında yapılır.

## Neden kodları {#reason-codes}

Ögenin sayfası, çalıştırma geçmişi ve bildirimler bir sorunu bu kodlardan biriyle adlandırır. Çoğunun çözümü sayfada yanında da vardır.

| Kod | Anlamı | Ne yapmalı |
|---|---|---|
| `ssh-missing` | Bu konteynerde SSH bağlantısı kurulmamış. | SSH bağlantısını VM yedekleri için olduğu gibi kur. |
| `host-placeholder` | Host SSH: Address hâlâ yer tutucu ve `host.docker.internal` de yanıt vermedi. | Host SSH: Address'i bu sunucunun LAN IP'sine ayarla. |
| `host-fallback` | Host SSH: Address hâlâ yer tutucu ve `host.docker.internal` çalışıyor. | Hiçbir şey ya da LAN IP'sini ayarla. |
| `ssh-unreachable` | Sunucuya SSH ile ulaşılamıyor. | Adresi, portu ve SSH'ın açık olduğunu kontrol et. |
| `ssh-auth` | Sunucu BombVault'un anahtarını reddetti. | **Bu sunucuyla bağlantı** altında gösterilen komutu sunucuda bir kez çalıştır. |
| `zfs-not-found` | SSH ana makinesinde `zfs` komutu yok. | Host SSH: Address'i havuzların sahibi makineye yönlendir. |
| `zfs-permission` | SSH kullanıcısı bu zfs komutunu çalıştıramaz. | root kullan ya da bkz. [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` SSH alanlarından farklı bir ana makine ya da kullanıcı adlandırıyor. | Aynı yap ya da ikisi de URI'den gelsin diye SSH alanlarını temizle. |
| `zfs-error` | zfs başka bir hata bildirdi. | Ayrıntılar mesajını gösterir. |
| `propagation-missing` | Ana makinedeki yeni bağlamalar konteynere ulaşmıyor. | Host Data'nın Erişim Kipini Read/Write - Slave yap ve BombVault'u yeniden başlat. |
| `invalid-name` | BombVault'un kabul etmediği bir veri kümesi adı. | Veri kümesini yeniden adlandır. |
| `name-too-long` | Ağaçtaki bir veri kümesi bir anlık görüntü adı için çok uzun. | Yeniden adlandır ya da altındaki bir veri kümesini öge olarak ekle. |
| `invalid-exclude` | Bir hariç tutma deseni ya da dışarıda bırakılan bir alt öge ögeye uymuyor. | Mesajın adlandırdığı ögeyi düzelt. Bir alt veri kümesinin tamamını dışarıda bırakmak için desen yazmak yerine onu kapat. |
| `not-found` | Veri kümesi sunucuda yok. | Ögeyi kaldır ya da veri kümesini yeniden oluştur. Yedekleri geri yüklenebilir kalır. |
| `not-filesystem` | Bu bir dosya sistemi değil, bir birim. | Bkz. [Birimler](#volumes). |
| `overlaps-item` | Veri kümesi mevcut bir ögeyle çakışıyor. | Bkz. [Ögeler hiç çakışmaz](#overlap). |
| `docker-storage` | Ağaç Docker'ın image depolamasını tutuyor. | Bkz. [Docker'ın depolaması](#docker-storage). |
| `nothing-readable` | Ögedeki hiçbir veri kümesi şu an okunamıyor. | Atlanan veri kümelerinin kodlarına bak. |
| `snapshot-failed` | Anlık görüntü oluşturulamadı. | Ayrıntılar zfs'nin mesajını gösterir. |
| `containers-busy` | Konteynerlerin durması gerektiğinde bir konteyner yedeği hâlâ çalışıyordu. | Sonra yeniden başla. Zamanlanmış çalıştırmalar kendiliğinden bekler. |
| `consistency-stop-failed` | Bir konteyner durdurulamadı, bu yüzden anlık görüntü alınmadı. | Konteyneri kontrol et ya da listeden çıkar. |
| `pre-snapshot-failed` | Anlık görüntüden önceki komut başarısız oldu. | Çalıştırma ayrıntıları çıktısını gösterir. |
| `container-unknown` | Listelenen bir konteyner yok. | Listeden kaldır. |
| `container-is-self` | BombVault kendi konteynerini durduramaz. | Listeden kaldır. |
| `leftover-snapshots` | BombVault'un kaldıramadığı anlık görüntüler hâlâ sunucuda. | **Şimdi kaldır**'a bas, bkz. [Artakalan anlık görüntüler](#leftover-snapshots). |
| `zvol` | Ağaçta bir birim, atlandı. | Bkz. [Birimler](#volumes). |
| `canmount-off` | Hiç bağlanmaz (`canmount=off`), atlandı. | Veri tutuyorsa bağla ya da veriyi bir alt veri kümesine taşı. |
| `legacy-mount` | Legacy bağlama noktası, atlandı. | Ona `/mnt` altında bir bağlama noktası ver. |
| `no-mountpoint` | Bağlama noktası yok, atlandı. | Ona `/mnt` altında bir bağlama noktası ver. |
| `not-mounted` | Sunucuda bağlı değil, atlandı. | `zfs mount` ile bağla ya da `canmount=on` yap. |
| `key-not-loaded` | Şifreli ve anahtar yüklü değil, atlandı. | `zfs load-key`, sonra bağla. |
| `snapdir-disabled` | Anlık görüntü erişimi kapalı, atlandı. | `zfs set snapdir=hidden <dataset>`. `.zfs` klasörü gizli kalır. |
| `not-visible` | BombVault veri kümesinin bağlama noktasını göremiyor. | Bağlama noktasını Host Data yolunun altına taşı ya da konteynere aynı yolda Read/Write - Slave ile eşle. |
| `shfs-only` | Veri kümesi yalnızca anlık görüntüleri gizleyen `/mnt/user` üzerinden görünür. | Host Data olarak `/mnt/user` değil `/mnt` eşle. |
| `snapshot-not-visible` | Anlık görüntü oluşturuldu ama BombVault içinde görünmedi. | **Anlık görüntü erişimini dene**'yi çalıştır. Aşağıya bak. |
| `snapshot-loop` | Anlık görüntü BombVault'a ulaşmadı, çünkü Host Data yeni bağlamaları geçirmiyor. | Host Data'nın Erişim Kipini Read/Write - Slave yap ve BombVault'u yeniden başlat. |
| `backup-failed` | restic bu veri kümesi için başarısız oldu. | Çalıştırma ayrıntıları nedenini gösterir. |
| `not-reached` | Çalıştırma bu veri kümesine gelmeden bitti. | Yedeği yeniden çalıştır. |
| `gone` | Veri kümesi artık sunucuda değil. | Hiçbir şey. Yedekleri geri yüklenebilir kalır. |
| `read-only-mount` | BombVault veri kümesini yalnızca okuyabilir, bu yüzden ona geri yükleyemez. | Eşlemeyi Read/Write - Slave yap ya da bir klasöre geri yükle. |
| `destination-not-mounted` | Klasör bağlı bir havuzda ya da paylaşımda değil. | Bir havuzda ya da paylaşımda bir klasör seç. |
| `not-enough-space` | Hedefte yeterli boş alan yok. | Yer aç ya da başka bir klasör seç. |
| `safety-snapshot-failed` | Güvenlik anlık görüntüsü alınamadı, bu yüzden hiçbir şey geri yüklenmedi. | Ayrıntılar zfs'nin mesajını gösterir. |
| `safety-name-too-long` | Veri kümesi adı bir güvenlik anlık görüntüsü için çok uzun. | Güvenlik anlık görüntüsünü kapat ya da bir klasöre geri yükle. |
| `dataset-exists` | Bu adda bir veri kümesi zaten var. | Yeni bir ad seç ya da veri kümesinin kendisine geri yükle. |
| `create-failed` | Yeni veri kümesi oluşturulamadı. | Ayrıntılar zfs'nin mesajını gösterir. Üstündeki veri kümesinin var olduğunu kontrol et. |
| `new-dataset-not-visible` | Yeni veri kümesi oluşturuldu ama BombVault onu göremiyor, bu yüzden hiçbir şey geri yüklenmedi. | Veri kümesi sunucuda kalır. Onu Host Data yolunun altına bağla ve ona geri yükle. |
| `set-properties-failed` | Saklanan özellikler ayarlanamadı, bu yüzden hiçbir şey geri yüklenmedi. | Ayrıntılar zfs'nin mesajını gösterir. |
| `set-limits-failed` | Dosyalar geri yüklendi, ama saklanan kota ya da rezervasyon ayarlanamadı. | Ayrıntılar zfs'nin mesajını gösterir. Kotayı ya da rezervasyonu `zfs set` ile kendin ayarla. |

### Konteynerin ne gördüğünü kontrol etmek {#mountinfo}

Ögenin Ayarlar sekmesindeki **Anlık görüntü erişimini dene**, ağacının gerçek bir anlık görüntüsünü alır, her veri kümesi için onu BombVault içinde arar ve yeniden kaldırır. İlk zamanlanmış çalıştırmadan önce tüm yolu kanıtlamanın en hızlı yoludur.

Kendin bakmak için bunu sunucuda çalıştır:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Her satır konteynerin içindeki bir bağlamadır. Bir veri kümesinin satırı konteyner içindeki yolunu (`/host/user` altında) ve veri kümesi adını gösterir. O satırdaki bir `master:N` alanı, bağlamanın ana makinenin sonradan yaptığı bağlamaları aldığı anlamına gelir, anlık görüntü erişiminin ihtiyaç duyduğu budur. Yoksa Host Data'nın Erişim Kipini Read/Write - Slave yap ve BombVault'u yeniden başlat.

## TrueNAS SCALE {#truenas}

- `LIBVIRT_URI` ayarlıyken (TrueNAS'ta VM yedekleri için olduğu gibi), BombVault zfs komutları için SSH ana makinesini, kullanıcıyı ve portu, tek başına ayarlı olmayan her biri için, URI'den alır. VM yedekleri olmadan hiçbir şey ayarlanmak zorunda değildir: `host.docker.internal` bir TrueNAS uygulamasında çözümlenmez, bu yüzden BombVault ana makineye uygulamanın varsayılan ağ geçidi üzerinden, 22 portunda root olarak ulaşır. Başka bir adres ya da kullanıcı istediğinde `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` ve `LIBVIRT_SSH_PORT`'u uygulamanın `environment` bölümünde ayarla.
- **Ayarlar, Entegrasyonlar, Ana makine SSH** altındaki Host SSH anahtarını **Credentials, Users** altında root'a ekle ve **System, Services** altında SSH servisini aç. Bu ya da başka bir TrueNAS'a bir [replika](#replica-push) ayrıca farklı olan replika anahtarına da ihtiyaç duyar.
- Root dışında bir kullanıcı, ögenin en üst veri kümesi üzerinde izne ihtiyaç duyar, bu da altındaki her veri kümesini kapsar:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Yeni bir veri kümesine geri yüklemek ayrıca üstündeki veri kümesinde `create` ister ve saklanan özellikleri ayarlamak bu özellikler için izin ister.

  TrueNAS'ta root olmayan bir SSH oturumunun yolunda `/usr/sbin` yoktur. BombVault o zaman `/usr/sbin/zfs`'yi doğrudan çağırır.
- Uygulamanın **Host Data**'sı bir ixVolume değil, veri kümelerinin üstünde bir ana makine yolu olmalıdır, örneğin `/mnt/tank`. Bir ana makine yolu ile uygulama ana makinenin yeni bağlamalarını BombVault'a geçirir (`rslave`), anlık görüntü erişimi buna ihtiyaç duyar. Uygulamanın tüm kurulumu için bkz. [TrueNAS SCALE](getting-started.md#truenas).
