# ZFS veri kümeleri

ZFS veri kümeleri, konteynerler ve VM'ler gibi **Yedekler** listesindeki ögelerdir. Bir öge, bir veri kümesi ile altındaki her veri kümesidir. Her yedek için BombVault tüm ağacın tek bir ZFS anlık görüntüsünü alır; böylece içindeki her veri kümesi aynı anda yakalanır. Sonra her veri kümesinin dosyalarını o anlık görüntüden okur ve onları restic ile, bir klasörü sakladığı gibi saklar; anlık görüntüyü hemen ardından kaldırır. Yedekler yinelenenlerden arındırılır, her birine göz atabilirsin ve tek tek dosyalar geri yüklenebilir.

Yedekleme restic'e hiçbir zaman bir akış göndermez ve hiçbir veri kümesini geri sarmaz. BombVault yalnızca kendi oluşturduğu anlık görüntüleri yok eder. İsteğe bağlı [replika](#replica) `zfs send` kullanan tek yerdir: veri kümelerini ikinci bir ZFS sunucusuna kopyalar ve yedeğe dokunmaz.

## Gereksinimler {#requirements}

- **Bu sunucuya SSH bağlantısı.** ZFS veri kümeleri VM yedekleriyle aynı anahtarı, ana makineyi ve kullanıcıyı kullanır. VM yedekleri zaten çalışıyorsa bu da çalışır. Aksi halde GitHub'daki [SSH üzerinden VM yedeği kılavuzunu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) izle. Şablon alanlarının adı **Host SSH: Address**, **Host SSH: Port** ve **Host SSH: User**'dır.
- **O ana makinede `zfs` komutu.** Unraid 6.12 ve sonrası ile TrueNAS SCALE'de vardır.
- **Host Data, Read/Write - Slave Access Mode ile `/mnt` olarak eşlenmiş olmalı.** Bu, şablonun varsayılanıdır. Bir veri kümesinin anlık görüntüsü, veri kümesinin `.zfs/snapshot` klasöründe ancak BombVault başladıktan sonra görünür; bu yüzden konteynerin, ana makinenin sonradan yaptığı bağlamaları alması gerekir.
- **Veri kümeleri `/mnt` altına bağlanmış olmalı.** Unraid'de havuzlar `/mnt/<pool>` altında bulunduğundan bu zaten böyledir.

Bölümü **Ayarlar, Genel** altında aç (ZFS veri kümeleri). Yedekler sayfasında **Diğer** altındaki **Bu sunucuyla ZFS bağlantısı** sonra **Bu sunucuyla bağlantı**'yı açar. SSH bağlantısını sınar, bağlandığı kullanıcıyı ve host'u adlandırır ve bir şey eksikse neyin eksik olduğunu söyler. Bağlantı kesikken Yedekler listesi bunu **Bağlantıyı göster** ile söyler ve genel bakıştaki **Host Entegrasyonu** kartı aynı sonucu gösterir.

## Ögeler ve alt veri kümeleri {#items-and-children}

Yedekler sayfasında **Ekle**'yi, sonra **Bir ZFS veri kümesi**'ni seç. Liste sunucudan gelir. Yedeklenmesini istediğin şeyin en üstündeki veri kümesini seç, örneğin `cache/appdata`; öge onu ve altındaki her veri kümesini kapsar.

- **Yeni alt veri kümeleri kendiliğinden katılır.** Ögenin altında sonradan oluşturulan bir veri kümesi sonraki çalışmada yedeklenir ve o çalışma onu yeni olarak adlandırır. İlk yedeği onu bir kez tamamen okur; ondan sonra yalnızca değişiklikler okunur.
- **Tek tek alt kümeler dışarıda bırakılabilir.** Bir alt kümeyi ögenin Ayarlar sekmesinde kapat; altındaki her şeyle birlikte dışarıda kalır. Sunucuda artık var olmayan, dışarıda bırakılmış bir alt küme öyle işaretlenir ve listeden kaldırılabilir.
- **Okunamayan alt kümeler atlanır, asla sessizce değil.** Çalışma onları listeler, öge kaç tanesinin atlandığını gösterir ve genel bakıştaki **Yedeklenmedi** kartı her birini korumasız sayar. Çalışma geri kalan her şeyi yine yedekler ve atlanan bir alt küme yüzünden başarısız olmaz. Nedenler [neden kodları tablosunda](#reason-codes) yer alır: bağlanmamış bir veri kümesi, `canmount=off`, `legacy` ya da hiç bağlama noktası olmaması, yüklenmemiş bir şifreleme anahtarı, kapatılmış anlık görüntü erişimi ya da BombVault'un göremediği bir bağlama noktası.
- **Atlanan bir veri kümesi alt veri kümelerini de beraberinde götürmez.** Yalnızca başka veri kümeleri içeren `canmount=off` bir veri kümesi atlanır ("yalnızca yapı" olarak gösterilir) ve bağlı alt veri kümeleri yedeklenir. Anahtarı yüklenmemiş şifreli bir veri kümesi, anahtarını paylaşan alt veri kümeleriyle birlikte atlanır.
- **VM diski veya sistem verisi olan alt veri kümeleri ekleme penceresinde kapalı başlar**, nedeni anahtarın yanında yazar. Bütün bir havuzu eklemek, içinde ne olduğunu listeleyen bir onay ister.

### Birimler {#volumes}

Bir birim (zvol) dosyalar yerine sanal bir disk tutar ve bir ZFS ögesi onu asla yedeklemez.

- Bir VM'in kullandığı birim, o VM ile yedeklenir.
- Hiçbir VM'in kullanmadığı bir birim (bir iSCSI extent, ayırdığın bir disk) **BombVault tarafından yedeklenmez**. Ekleme penceresi ve Yedekler sayfası bu birimleri sayar ve bunu söyler. Sonraki bir sürüm onları yedekleyecek.

Bir ögenin ağacındaki birimler her çalışmada atlanır ve adlandırılır.

### Docker'ın depolaması {#docker-storage}

Docker'ın ZFS depolama sürücüsüyle her image katmanı, `legacy` bağlama noktalı bir veri kümesidir. Ekleme penceresi bunları ebeveyn başına tek satırda toplar. 20'den fazla böyle veri kümesi tutan bir ağaç öge olamaz: onun bir anlık görüntüsü varken Docker image katmanlarını kaldıramaz. Bunun yerine altındaki veri kümelerini ekle, örneğin `appdata`.

### Ögeler asla çakışmaz {#overlap}

Bir veri kümesi yalnızca bir ögeye ait olabilir. BombVault, var olan bir ögenin içinde kalan ya da birini kapsayacak yeni bir ögeyi reddeder. Birkaç alt ögeyi tek bir ebeveyn ögede birleştirmek için önce alt ögeleri sil ve yedeklerini korumayı seç, sonra ebeveyni ekle. Her veri kümesi geçmişini kendi adı altında tutar; böylece sonraki yedek eski ögelerin bıraktığı yerden devam eder ve her şeyi yeniden okumaz.

## Anlık görüntü etrafında konteynerleri durdurmak ve komut çalıştırmak {#consistency}

Çalışan bir veritabanının anlık görüntüsü ani bir elektrik kesintisi gibidir: veritabanı genellikle toparlanır, ama toparlanması gerekir. Her öge bunun için iki şey yapabilir ve ikisi de yalnızca tüm yedeği değil, anlık görüntünün anını kapsar.

- **Anlık görüntü için bu kapsayıcıları durdur.** BombVault listelenen konteynerleri durdurur, anlık görüntüyü alır ve onları hemen yeniden başlatır. Aynı bağımlılık düzeyindeki konteynerler paralel olarak durur, önce bağımlı olanlar; bu yüzden tüm pencere genellikle birkaç saniye sürer, çalıştırma ne kadar sürdüğünü gösterir. Yedek, uygulamalar zaten yeniden çalışırken dondurulmuş anlık görüntüyü okur. Yalnızca çalışmakta olan konteynerler durdurulur.
- **Anlık görüntüden önce ve sonra bir komut.** Seçtiğin bir konteynerin içinde çalışır; örneğin hiçbir şeyi durdurmadan, anlık görüntüden hemen önce bir veritabanını veri kümesine dökmek için. Anlık görüntüden önceki komut başarısız olursa yedek başarısız olur ve anlık görüntü alınmaz. Anlık görüntüden sonraki başarısız bir komut çalıştırmada gösterilir, ama yedeği başarısız kılmaz.

Bir şeyler ters giderse ne olur:

- Bir konteyner durdurulamazsa BombVault zaten durdurduklarını başlatır ve yedek, konteynerin adını vererek başarısız olur. Hiçbir zaman çalışan uygulamaların anlık görüntüsüne geri dönmez.
- Durdurma, devam eden bir konteyner yedeğinin bitmesini bekler (el ile çalıştırmada en fazla 30 dakika, zamanlanmış olanda yedeğin süre sınırına kadar); böylece ikisi aynı konteyneri asla aynı anda durdurup başlatmaz.
- İlk konteyner durmadan önce BombVault hangilerini durdurduğunu not eder. BombVault pencerenin içinde öldürülürse, o konteynerleri bir sonraki başlayışında yeniden başlatır, bir bildirim gönderir ve öge, başlatamadığı her konteyner için kırmızı bir not gösterir.

Otomatik veritabanı dökümleri (bkz. [Özellikler](features.md)) bir konteynerin kendi yedeğiyle çalışır, bir ZFS ögesiyle değil. Konteyneri yalnızca veri kümesi üzerinden yedeklenen bir veritabanı döküm almaz; bu yüzden ona burada bir komut ver.

Bir konteyner bu listede olabilir ve aynı zamanda bir konteyner olarak yedeklenebilir. Verisi o zaman iki depoda iki kez saklanır ve tam yedekleme onu iki kez durdurur. Öge bunu söyler.

## Geri yükleme {#restore}

Ögeyi aç ve **Geri yükle**'yi seç ya da **Geri yükleme** sayfasında seç. Geri yükleme penceresi yedeği, sonra **Veri kümesi**'ni sorar. Varsayılan olarak bu, ögenin en üstteki veri kümesidir. **Ne** altında her şeyi ya da yalnızca seçtiğin dosya ve klasörleri geri yüklersin. **Nereye** şunları sunar:

- **Veri kümesinin içine geri yükle.** Yedekteki dosyalar veri kümesinin bağlama noktasına yazılır. Aynı adlı dosyaların üzerine yazılır, diğerleri kalır. Veri kümesi hiçbir zaman geri sarılmaz veya değiştirilmez. BombVault veri kümesinin bağlı, görünür ve yazılabilir olduğunu başlamadan önce bir kez, yazmadan hemen önce tekrar denetler. İçine bir alt veri kümesi bağlanmış olan yerlere hiçbir şey yazılmaz: alt veri kümesi dosyalarını, sahibini ve izinlerini korur ve kendi yedeğinden geri yüklenir.
- **Bir klasöre.** `/mnt` altında bir klasör seç. BombVault klasörün bağlanmış bir havuzda ya da paylaşımda olduğunu ve yeterli boş alan bulunduğunu denetler. Bu, SSH bağlantısı olmadan ve artık var olmayan veri kümeleri için çalışır.
- **Yeni bir veri kümesine.** Henüz var olmayan bir veri kümesi adlandır; pencere `<dataset>-restored` önerir. BombVault onu yedekte saklanan ZFS özellikleriyle oluşturur ve içine geri yükler, bkz. [Yeni veri kümesi olarak geri yükleme](#new-dataset).

Veri kümesi olarak **Bu yedeğin bütün veri kümeleri**'ni seçmek, ağacın her veri kümesini seçtiğin klasörün kendi alt klasörüne yazar. O yedekte atlanan veri kümeleri adlandırılır. Başka bir BombVault'un deposundan geri yüklemek için Geri yükleme sayfasındaki **Eski bir yedek klasöründen**'i kullan; o her zaman bir klasöre geri yükler: bir yedeğin tüm veri kümeleri, her biri kendi alt klasörüne, ya da ağacın tek bir veri kümesi, bütünüyle ya da seçilen dosyalar.

Ögenin durdurulacak konteynerler listesi, veri kümesine geri yükleme için de sunulur. O konteynerler tüm geri yükleme boyunca durmuş kalır ve bu sürede konteyner yedekleri bekler.

### Güvenlik anlık görüntüsü {#safety-snapshot}

Bir veri kümesine yazmadan önce BombVault yalnızca o veri kümesinin `bombvault-prerestore-<time>` adlı bir ZFS anlık görüntüsünü alır. **Önce şimdiki durumun ZFS anlık görüntüsünü sakla** varsayılan olarak açıktır; kapatmak ikinci bir onay ister. Anlık görüntü alınamazsa hiçbir şey geri yüklenmez.

BombVault bir güvenlik anlık görüntüsünü kendi başına asla silmez. Öge onları yaşları ve boyutlarıyla listeler, her biri bir **Sil** eylemiyle, ve en eskisi 30 günden eskiyse uyarır, çünkü silinmiş ve değişmiş veriyi havuzda tutar.

Bir geri yüklemeden sonra geri dönmek için veri kümesinin içindeki `.zfs/snapshot/bombvault-prerestore-<zaman>` konumundan tek tek dosyaları kopyala. `zfs rollback <dataset>@bombvault-prerestore-<zaman>` yalnızca bu, o veri kümesinin en yeni anlık görüntüsü olduğu sürece çalışır. `zfs rollback -r` otomatik olanlar dahil tüm daha yeni anlık görüntüleri siler.

### Yeni bir veri kümesi olarak geri yükleme {#new-dataset}

BombVault her yedekle birlikte her veri kümesinin yerel olarak ayarlanmış ZFS özelliklerini saklar: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity ve kendi kullanıcı özelliklerin. Devralınan ve salt okunur değerler dışarıda bırakılır, çünkü kendiliğinden geri gelirler. BombVault bunları saklamaya başlamadan önceki yedeklerde hiç yoktur.

- **Yeni bir veri kümesine** her saklanan özellikle `zfs create` çalıştırır. casesensitivity, normalization ve utf8only yalnızca bu şekilde ayarlanabilir. Kotalar ve rezervasyonlar dosyalardan sonra ayarlanır, böylece dosyaları reddedemezler. Kopya orijinalle çakışmasın diye bağlama noktası dışarıda bırakılır; geri yükleme yazabilsin diye `canmount`, `readonly` ve şifreleme de. Şifreli bir veri kümesinin altındaki yeni veri kümesi onun şifrelemesini alır. Üstteki veri kümesi mevcut olmalıdır. Oluşturulduktan sonra bir şey başarısız olursa yeni veri kümesi sunucuda kalır, çünkü BombVault asla bir veri kümesini yok etmez.
- **Veri kümesinin içine geri yükle** saklanan özellikleri geri yüklemenin yanında gösterir. **Bu özellikleri de ayarla**, var olan bir veri kümesinin hâlâ kabul ettiği özellikleri herhangi bir dosya yazılmadan önce ayarlar. Kotalar ve rezervasyonlar dosyalardan sonra ayarlanır, böylece dosyaları reddedemezler. Bu anahtar olmadan veri kümesi ayarlarını korur.

## Yedekte ne var {#contents}

Yedekte: yedeklenen her veri kümesinin dosya ve klasörleri, restic'in sakladığı şekliyle sahiplikleri, izinleri, zaman damgaları ve genişletilmiş öznitelikleriyle ve her veri kümesinin yerel olarak ayarlanmış ZFS özellikleri.

Yedekte olmayanlar:

- her veri kümesinin en üst klasörünün kendi sahibi ve izinleri (altındaki her şey dahildir). Veri kümesinin içine geri yükleme mevcut en üst klasörü olduğu gibi bırakır, bir klasöre geri yükleme onu `0755` izinleriyle oluşturur;
- mevcut ZFS anlık görüntüleri;
- atlanan veya dışarıda bırakılan alt veri kümeleri;
- birimler.

Yeni bir havuza geri yüklemek için havuzu oluştur ve her veri kümesini yeni bir veri kümesine geri yükle. TrueNAS'ın SMB veri kümelerinde kullandığı NFSv4 ACL'lerinin beklediğin gibi geri gelip gelmediği henüz doğrulanmadı; bu yüzden onlara güvenmeden önce kendi verilerinle bir geri yüklemeyi dene.

## Şifreli veri kümeleri {#encryption}

Şifreli bir veri kümesi yalnızca anahtarı yüklüyken yedeklenir. Aksi halde bir uyarıyla atlanır; anahtarı `zfs load-key` ile yükle ve veri kümesini bağla. BombVault verileri şifresi çözülmüş olarak okur ve şifreli olan restic deposunda saklar. BombVault'ta şifrelemeyi kapattıysan o depo şifreli değildir.

## Replika {#replica}

Replika, bir ögenin veri kümelerinin ikinci bir ZFS sunucusundaki kopyasıdır. BombVault onu `zfs send` ve `zfs receive` ile güncel tutar. İlk çalışma her şeyi gönderir, sonra yalnızca değişen bloklar gider. Öbür sunucuda kopyayı hemen bağlayabilirsin.

Bir replika yedeğin yerini asla almaz. Eski sürümler, tek tek dosyalar ve denetim yine yedeklerden gelir ve replika yalnızca belirlediğin kadar anlık görüntü tutar. Güncel bir replika bina dışında bir kopya sayılır, ama replikası olan ve yedeği olmayan bir öge turuncu kalır.

Replika kartındaki **Yalnızca replika, yedek yok**, tüm ZFS bölümünün replika göndermesini ve yedek almamasını sağlar. Kendi klasörü ve bina dışındaki hedefler anahtarlarını korur; bu yüzden kapatıldığında bölüm yeniden oraya yedekler ve her ZFS ögesinin kartı bölümün anahtarını olduğu gibi gösterir. Böyle bir öge yedek yerine ilk replikasını bekler, **Şimdi yedekle** onda **Şimdi çoğalt** olarak sunulur ve ögenin konteynerleri, bir yedeklemedeki gibi anlık görüntünün çevresinde durur, komutları da çalışır. Replikaları birkaç günde bir çalışan bir zamanlama, gecikme uyarısı ve kapsam için sayılır.

Bunu ögenin Ayarlar sekmesindeki **Replika** kartında aç. Orada replikanın nereye gideceğini, ne zaman çalışacağını (**Her yedekten sonra** ya da **Kendi planı**) ve hedefte ve bu sunucuda kaç anlık görüntünün kalacağını seçersin. Kart her veri kümesini ve birimi durumuyla listeler ve **Şimdi çoğalt** bir çalışma başlatır. Bir replika çalışmasının kendi kilidi vardır; bu yüzden uzun bir ilk aktarım yedekleri asla geciktirmez.

Yedekler sayfasında ZFS kutucuğu seçiliyken yedekleme eylemlerinin yanında duran **Şimdi tümünü çoğalt**, her ZFS ögesinin replikasını birbiri ardına gönderir. Toplu işi sunucu çalıştırır; bu yüzden sayfadan ayrılsan da sürer ve çoğaltılamayan bir öge adıyla belirtilip atlanır.

### Bir ZFS sunucusuna gönderme {#replica-push}

ZFS ve SSH olan her makine alabilir, örneğin ikinci bir Unraid veya bir TrueNAS. BombVault'un orada çalışması gerekmez.

1. **Ayarlar, Eşleştirme** altında **BombVault olmayan ZFS sunucusu** içinde **SSH ile bağlan**'ı ya da ögenin **Replika** kartında **ZFS sunucusu ekle**'yi seç.
2. Adresi, kullanıcıyı ve portu gir. Pencere BombVault'un genel anahtarını gösterir. Onu sunucuda kullanıcının `~/.ssh/authorized_keys` dosyasına ekle. Unraid'de bu **Users, root, SSH keys** altındadır. **Display Settings** ayarına göre Users üst menüde ya da Settings altında yer alır. TrueNAS'ta kullanıcıyı **Credentials, Users** altında düzenle. Bu, kopya anahtarıdır. VM'ler ve ana makine için kullanılan SSH bağlantısının anahtarından farklıdır, o zaten yetkilendirilmiş olsa bile bunu da ekleyin.
3. Bağlantıyı sına. Pencere ardından sunucunun havuzlarını listeler. Birini seç ve varsayılanı `<pool>/bombvault-replica` olan kökü belirle.
4. Yeni sunucuyu ögenin **Replika** kartında seç. Sunucunun o zaman Örnekler sayfasında kendi kartı olur ve **Aç** onun sayfasına götürür.

root ile başka bir şey gerekmez. Kendi kullanıcısı olan biri için hedefin havuzunda şu izinler gerekir, pencere bunları da gösterir:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Kaynakta aynı türden bir kullanıcı ögenin en üstteki veri kümesinde şunlara ihtiyaç duyar:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

Bu yönde ögeyi tutan BombVault, sunucuya yazabilen bir anahtarı da tutar.

BombVault, sunucunun ilk bağlantıda gösterdiği ana makine anahtarını kaydeder ve sonra başka her anahtarı reddeder. Sunucu yeniden kurulduysa sunucunun sayfasında **Ana makine anahtarını unut** düğmesine bas; sonraki bağlantı yeni anahtarı kaydeder.

Bu sunucunun ana makinesi ZFS sunucusuna kendisi ulaşabiliyorsa `zfs send` çıktısını, replika anahtarı ve kaydedilmiş ana makine anahtarıyla doğrudan sunucuya giden kendi ssh'ına aktarır; böylece akış BombVault'un içinden geçmez. Aksi halde akış BombVault üzerinden geçer.

### Eşleştirilmiş bir örneğe gönderme {#replica-receive}

Eşleştirilmiş bir BombVault replikayı kendisi alabilir. Kimse diğer ana makineye SSH erişimi kazanmaz ve hiçbir anahtar `authorized_keys` dosyasına girmez.

1. Ögenin **Replika** kartında hedef olarak eşleştirilmiş örneği seç ya da o örneğin Örnekler sayfasındaki kartında **ZFS sunucusu** rolünü aç ve ZFS ögelerini orada seç. Kart, o yanıt verene kadar **Onay bekliyor** gösterir.
2. Alan örnekte **Örnekler**'i aç. İstek, gönderen örneğin kartında bir rozet olarak görünür ve **Ayrıntılar** onu listeler. O sunucuda havuzu ve kökü ve kaç anlık görüntünün kalacağını seç, sonra **İzin ver**'e ya da **Reddet**'e bas.
3. **İzin ver**'den sonra kaynak, başka herhangi bir hedefte olduğu gibi kendi planına göre gönderir.

Alan örnek yalnızca onayın kapsadığını kabul eder: ögenin veri kümeleri, kendi kökünün içine. `zfs receive`'i kendisi çalıştırır ve kaynağın orada bir şeyi silme ya da geri sarma yolu yoktur. Böylece kopya, birinin ele geçirdiği bir kaynaktan sağ çıkar. Alan örnek kendi saklamasını tutar. Kaynak, sorduğunda yalnızca bir kural önerir.

Alan örnekteki **Revoke access** onayı istediğin zaman sona erdirir ve kaynağa bildirir. Kaynak onayın geri çekildiğini gösterir ve durur. Alan örneğin zaten elinde olanlar orada kalır. Reddedilen bir istek reddedilmiş kalır. Daha fazla veri kümesi için ya da bir iptalden sonra yeniden istek gerekir.

Alan örnek HTTPS üzerinden yanıt vermelidir. Kaynak düz bir http adresine hiçbir şey göndermez, çünkü belirteç ve veriler ağdan şifrelenmeden geçerdi. Her aktarım ayrıca alan havuzun onda birini boş bırakır ve o kısma dokunmadan durur; böylece tek bir kaynak alan sunucunun havuzunu dolduramaz.

### Verinin indiği yer {#replica-target}

Her veri kümesi `<root>/<server>/<pool>/<path>` konumuna iner. Sunucu klasörü kaynak örneğin adıdır ve ilk aktarımda sabitlenir, bu yüzden havuz adı aynı olan iki sunucu birbirine karışmaz. Örneğin `tower` adlı bir sunucunun `cache/appdata` veri kümesi `backup/bombvault-replica/tower/cache/appdata` konumunda biter.

BombVault yeniden kurulduktan sonra hedef, bu BombVault'un daha önce doldurduğu klasörü reddeder. Replika kartı o zaman **Klasörü devral**'ı sunar ve sonraki replika içindeki kopyalardan devam eder. Klasörü yalnızca başka hiçbir BombVault artık o klasöre çoğaltma yapmıyorsa devral.

Hedefteki kopya salt okunurdur ve bağlanmaz, bu yüzden o sunucuda hiçbir şeyi örtmez. ZFS özellikleri, bağlama noktası ile `sharenfs` ve `sharesmb` dışında birlikte gider. `reservation` ve `refreservation` ayırmaları da gitmez, bu yüzden kopya yalnızca verilerinin ihtiyaç duyduğu yeri kaplar.

### Neler dahil {#replica-contents}

Ögenin yedeklediği her şey girer ve altındaki birimler de, ki yedek onları atlar. Ögede kapattığın bir alt veri kümesi dışarıda kalır. Bir çalışmanın tüm veri kümeleri, yedekte olduğu gibi tek bir anlık görüntüden gelir.

### Anlık görüntülerin ne kadar kaldığı {#replica-retention}

Hedefte yeni bir replika 7 günlük ve 3 haftalık anlık görüntü tutar. Bunun yerine **Short**, **Dengeli** veya **Long**'u seç ya da **Custom values**'ı ayarla. Orada yalnızca `bombvault-replica-<14 digits>` adlı anlık görüntüler kaldırılır ve iki tarafın da paylaştığı en yeni olan asla kaldırılmaz.

Kaynakta BombVault en son replika anlık görüntüsünü ve gönderdiği her durum için bir yer imi tutar. Yer imleri alan harcamaz. Sonraki aktarım onlardan başlar.

Kaynakta da daha fazlasını tutmak için Replika kartında **Bu sunucuda sakla**'yı ayarla, örneğin 7'ye. O zaman her replikadan sonra en yeni replika anlık görüntülerinden o kadarı, hedefin tuttuklarına ek olarak bu sunucuda kalır ve birine geri dönmek için burada bir `zfs rollback` yeter, aktarılacak bir şey yoktur. 0 veya 1 yalnızca en sonuncuyu tutar. En sonuncunun üzerinde bir hold vardır, böylece sonraki aktarımın tabanı yanlışlıkla kaldırılmaz. Eskilerinde hold yoktur, bu yüzden onlara elle geri sarabilir ya da onları elle kaldırabilirsin. Her anlık görüntü kendisinden sonra değişen blokları tutar, bu yüzden tuttuğun her biri havuzda yer kaplar. Replikayı kapatmak, başka bir hedefe geçirmek ya da ögeyi silmek onları da bu sunucudan kaldırır.

En son replika anlık görüntüsüne geri dönmek için veri kümesini geri sar:

```
zfs rollback cache/appdata@bombvault-replica-20261006014100
```

`zfs rollback` tek bir veri kümesinde çalışır, bu yüzden geri istediğin her alt veri kümesini de geri sar. Daha eski bir anlık görüntüye dönmek daha yenilerini kaldırır, bu yüzden önce `zfs release -r bombvault-replica cache/appdata@<son>` ile en sonuncudaki hold'u kaldır, sonra `zfs rollback -r` kullan. Hedefteki kopyada daha yeni anlık görüntüler hâlâ durur ve o veri kümesinin sonraki replikası, oradaki kopyayı `zfs rollback -r` ile aynı anlık görüntüye geri sarana kadar başarısız olur.

### Replikada şifreli veri kümeleri {#replica-encryption}

Şifreli bir veri kümesi ham olarak gönderilir. Hedefte şifreli kalır ve hedef anahtarı hiçbir zaman görmez. Anahtarı güvende tut: bir geri yüklemeden sonra kopyayı açmak için ona ihtiyacın vardır ve anahtarsız bir replika okunamaz.

### Replikayı kullanma {#replica-use}

Ögenin **Yedekler** sekmesini aç ve anlık görüntülerini tutan yerler arasında **ZFS replikası** satırına tıkla. Sayfa hedefteki anlık görüntüleri listeler ve komutları gerçek adlarınla gösterir.

Eski bir duruma bakmak için hedefte bir anlık görüntüyü klonla. Bir klon, bir şey değişene kadar yer kaplamaz ve replika olduğu gibi kalır:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Kaynak çökerse kopyayı hedefte normal, yazılabilir bir veri kümesine çevir:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault bundan sonra yeni bir ilk çalıştırma başlatana kadar o veri kümesine replikayı durdurur.

Bir durumu kaynağa geri getirmek için sayfada **Bring back as a new dataset**'e bas. BombVault anlık görüntüyü orijinalin yanında, adı `<dataset>-bombvault-restore-` artı bir zaman damgası olan yeni bir veri kümesine gönderir. Orijinalin üzerine asla yazmaz.

## Kalan anlık görüntüler {#leftover-snapshots}

Bir yedeğin anlık görüntüsünün adı `<dataset>@bombvault-<14 rakam>` biçimindedir, örneğin `cache/appdata@bombvault-20260924021500` (UTC). BombVault onu yedekten hemen sonra kaldırır. Bu başarısız olursa, örneğin veri kümesi meşgul olduğu ya da BombVault durdurulduğu için, BombVault onu şu durumlarda kaldırır:

- ögenin sonraki yedeğinden önce,
- BombVault başladığında, her öge için, bölüm kapalıyken de,
- ögeyi sildiğinde,
- ögede **Şimdi kaldır**'a bastığında; bu, kaç tanesinin kaldığını gösterir.

Yalnızca tam olarak `bombvault-` artı 14 rakamdan oluşan adlar kaldırılır. Güvenlik anlık görüntülerine, kendi anlık görüntülerine ve otomatik anlık görüntülere asla dokunulmaz. Birini elle kaldırmak için:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Bir replikanın anlık görüntüsü `<dataset>@bombvault-replica-<14 digits>` olarak adlandırılır ve kalıntı değildir. Kaynakta daha yeni bir replika çalıştırması onun yerini alana kadar ya da **Bu sunucuda sakla** onu tuttuğu sürece, hedefte ise saklama onu tuttuğu sürece kalır. Temizleyici ona hiçbir zaman dokunmaz, çünkü yalnızca `bombvault-` ve ardından tam olarak 14 rakam olan adlarla eşleşir.

## Anormallikler {#anomalies}

Büyük bir ağaçta boşaltılmış bir alt küme toplamı pek değiştirmez; bu yüzden anormallik algılama bir ögenin her veri kümesini ayrı ayrı izler: boyutunun, dosya sayısının, yeni verisinin ve restic süresinin her birinin kendi geçmişi vardır. Bu geçmiş veri kümesinin adına aittir; bu yüzden ağaç sonradan başka bir ögeyle yedeklense de kalır.

Önceki çalışmanın yedeklediği ve bu çalışmanın okuyamadığı bir veri kümesi, ögenin seçimi değişmediği sürece boşaltılmış sayılır. Bu, yüklenmemiş bir anahtarı, bağlanmamış bir veri kümesini ve ağaçtan kaybolmuş birini kapsar. Kendi dışarıda bıraktığın bir alt küme seçimi değiştirir, bu yüzden geçmişi baştan başlar. Kayıp veri hakkında bir bulgu açıkken saklama, yalnızca o veri kümesinin eski yedeklerini tutar ve ağacın geri kalanını her zamanki gibi budar.

Her veri kümesinin ögenin izlemesinde kendi satırı vardır; bu izleme **Anormallikler** sayfasında **İzleme** ile açılır ve ögenin **Doğrulama** sekmesinde durur. Açık bulgular her veri kümesinin yanında görünür. Bir bulgudaki bağlantı, geri yükleme penceresini veri kümesinin son iyi yedeğinde açar. Bir çalışmanın bitip bitmediği tüm öge için değerlendirilir, çünkü bir çalışma bütünüyle başarılı olur ya da başarısız olur.

Denetimlerin kendisi [Özellikler](features.md) altında anlatılır. [MCP sunucusu](mcp.md) üzerinden bağlanan bir asistan bir ZFS ögesinin geri yükleme noktalarını listeleyebilir, yedeğini başlatabilir ve bulguları okuyabilir, ama bir bulguyu onaylamak **Anormallikler** sayfasında yapılır.

## Neden kodları {#reason-codes}

Ögenin sayfası, çalışma geçmişi ve bildirimler bir sorunu bu kodlardan biriyle adlandırır. Çoğunun düzeltmesi sayfada yanında da durur.

| Kod | Anlamı | Ne yapmalı |
|---|---|---|
| `ssh-missing` | Bu konteynerde SSH bağlantısı kurulmamış. | SSH bağlantısını VM yedeklerindeki gibi kur. |
| `host-placeholder` | Host SSH: Address hâlâ örnek değer ve `host.docker.internal` de yanıt vermedi. | Host SSH: Address'i bu sunucunun LAN IP'sine ayarla. |
| `host-fallback` | Host SSH: Address hâlâ örnek değer ve `host.docker.internal` çalışıyor. | Hiçbir şey, ya da LAN IP'sini ayarla. |
| `ssh-unreachable` | Sunucuya SSH ile ulaşılamıyor. | Adresi ve portu, ayrıca SSH'nin açık olduğunu denetle. |
| `ssh-auth` | Sunucu BombVault'un anahtarını reddetti. | **Bu sunucuyla bağlantı** altında gösterilen komutu sunucuda bir kez çalıştır. |
| `zfs-not-found` | SSH ana makinesinde `zfs` komutu yok. | Host SSH: Address'i havuzların sahibi olan makineye yönlendir. |
| `zfs-permission` | SSH kullanıcısı bu zfs komutunu çalıştıramıyor. | root kullan ya da bkz. [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI`, SSH alanlarından farklı bir ana makine veya kullanıcı belirtiyor. | Bunları uyumlu hale getir ya da ikisinin de URI'den gelmesi için SSH alanlarını boşalt. |
| `zfs-error` | zfs başka bir hata bildirdi. | Ayrıntılar onun iletisini gösterir. |
| `propagation-missing` | Ana makinedeki yeni bağlamalar konteynere ulaşmıyor. | Host Data'nın Access Mode'unu Read/Write - Slave yap ve BombVault'u yeniden başlat. |
| `invalid-name` | BombVault'un kabul etmediği bir veri kümesi adı. | Veri kümesini yeniden adlandır. |
| `name-too-long` | Ağaçtaki bir veri kümesi bir anlık görüntü adı için fazla uzun. | Onu yeniden adlandır ya da ögenin altında bir veri kümesi ekle. |
| `invalid-exclude` | Bir hariç tutma deseni ya da dışarıda bırakılmış bir alt küme ögeye uymuyor. | İletinin adlandırdığı ögeyi düzelt. Bir alt veri kümesini tamamen dışarıda bırakmak için desen yazmak yerine onu kapat. |
| `not-found` | Veri kümesi sunucuda yok. | Ögeyi kaldır ya da veri kümesini yeniden oluştur. Yedekleri geri yüklenebilir kalır. |
| `not-filesystem` | Bu bir birimdir, dosya sistemi değil. | Bkz. [Birimler](#volumes). |
| `overlaps-item` | Veri kümesi var olan bir ögeyle çakışıyor. | Bkz. [Ögeler asla çakışmaz](#overlap). |
| `docker-storage` | Ağaç Docker'ın imaj depolamasını içeriyor. | Bkz. [Docker'ın depolaması](#docker-storage). |
| `nothing-readable` | Ögedeki hiçbir veri kümesi şu an okunamıyor. | Atlanan veri kümelerinin kodlarına bak. |
| `snapshot-failed` | Anlık görüntü oluşturulamadı. | Ayrıntılar zfs'nin iletisini gösterir. |
| `containers-busy` | Konteynerlerin durması gerektiğinde bir konteyner yedeği hâlâ çalışıyordu. | Daha sonra yeniden başlat. Zamanlanmış çalıştırmalar kendiliğinden bekler. |
| `consistency-stop-failed` | Bir konteyner durdurulamadı, bu yüzden anlık görüntü alınmadı. | Konteyneri denetle ya da listeden çıkar. |
| `pre-snapshot-failed` | Anlık görüntüden önceki komut başarısız oldu. | Çalıştırma ayrıntıları çıktısını gösterir. |
| `container-unknown` | Listelenen bir konteyner yok. | Onu listeden çıkar. |
| `container-is-self` | BombVault kendi konteynerini durduramaz. | Onu listeden çıkar. |
| `leftover-snapshots` | BombVault'un kaldıramadığı anlık görüntüler hâlâ sunucuda. | **Şimdi kaldır**'a bas, bkz. [Kalan anlık görüntüler](#leftover-snapshots). |
| `zvol` | Ağaçta bir birim, atlandı. | Bkz. [Birimler](#volumes). |
| `canmount-off` | Hiç bağlanmıyor (`canmount=off`), atlandı. | Veri içeriyorsa bağla ya da verileri bir alt veri kümesine taşı. |
| `legacy-mount` | Legacy bağlama noktası, atlandı. | Ona `/mnt` altında bir bağlama noktası ver. |
| `no-mountpoint` | Bağlama noktası yok, atlandı. | Ona `/mnt` altında bir bağlama noktası ver. |
| `not-mounted` | Sunucuda bağlı değil, atlandı. | `zfs mount` ile bağla ya da `canmount=on` ayarla. |
| `key-not-loaded` | Şifreli ve anahtar yüklü değil, atlandı. | `zfs load-key`, ardından bağla. |
| `snapdir-disabled` | Anlık görüntü erişimi kapalı, atlandı. | `zfs set snapdir=hidden <dataset>`. `.zfs` klasörü gizli kalır. |
| `not-visible` | BombVault veri kümesinin bağlama noktasını göremiyor. | Bağlama noktasını Host Data yolunun altına taşı ya da Read/Write - Slave ile aynı yolda konteynere eşle. |
| `shfs-only` | Veri kümesi yalnızca anlık görüntüleri gizleyen `/mnt/user` üzerinden görünüyor. | Host Data olarak `/mnt/user` değil, `/mnt` eşle. |
| `snapshot-not-visible` | Anlık görüntü oluşturuldu ama BombVault içinde görünmedi. | **Anlık görüntü erişimini dene**'yi çalıştır; aşağıya bak. |
| `snapshot-loop` | Host Data yeni bağlamaları geçirmediği için anlık görüntü BombVault'a ulaşmadı. | Host Data'nın Access Mode'unu Read/Write - Slave yap ve BombVault'u yeniden başlat. |
| `backup-failed` | restic bu veri kümesi için başarısız oldu. | Çalıştırma ayrıntıları nedenini gösterir. |
| `not-reached` | Çalıştırma bu veri kümesinden önce bitti. | Yedeği yeniden çalıştır. |
| `gone` | Veri kümesi artık sunucuda değil. | Hiçbir şey. Yedekleri geri yüklenebilir kalır. |
| `read-only-mount` | BombVault veri kümesini yalnızca okuyabiliyor, bu yüzden içine geri yükleyemiyor. | Eşlemeyi Read/Write - Slave yap ya da bir klasöre geri yükle. |
| `destination-not-mounted` | Klasör bağlı bir havuzda veya paylaşımda değil. | Bir havuzda veya paylaşımda bir klasör seç. |
| `not-enough-space` | Hedefte yeterli boş alan yok. | Yer aç ya da başka bir klasör seç. |
| `safety-snapshot-failed` | Güvenlik anlık görüntüsü alınamadı, bu yüzden hiçbir şey geri yüklenmedi. | Ayrıntılar zfs'nin iletisini gösterir. |
| `safety-name-too-long` | Veri kümesinin adı bir güvenlik anlık görüntüsü için fazla uzun. | Güvenlik anlık görüntüsünü kapat ya da bir klasöre geri yükle. |
| `dataset-exists` | Bu adda bir veri kümesi zaten var. | Yeni bir ad seç ya da veri kümesinin kendisine geri yükle. |
| `create-failed` | Yeni veri kümesi oluşturulamadı. | Ayrıntılar zfs'in mesajını gösterir. Üstteki veri kümesinin var olduğunu kontrol et. |
| `new-dataset-not-visible` | Yeni veri kümesi oluşturuldu ama BombVault onu göremiyor, bu yüzden hiçbir şey geri yüklenmedi. | Veri kümesi sunucuda kalır. Onu Host Data yolunun altına bağla ve içine geri yükle. |
| `set-properties-failed` | Saklanan özellikler ayarlanamadı, bu yüzden hiçbir şey geri yüklenmedi. | Ayrıntılar zfs'nin iletisini gösterir. |
| `set-limits-failed` | Dosyalar geri yüklendi, ancak saklanan kota ya da rezervasyon ayarlanamadı. | Ayrıntılar zfs'in mesajını gösterir. Kotayı ya da rezervasyonu `zfs set` ile kendin ayarla. |

### Konteynerin ne gördüğünü denetlemek {#mountinfo}

Ögenin Ayarlar sekmesindeki **Anlık görüntü erişimini dene**, ağacının gerçek bir anlık görüntüsünü alır, her veri kümesi için onu BombVault içinde arar ve yeniden kaldırır. İlk zamanlanmış çalışmadan önce tüm yolu kanıtlamanın en hızlı yoludur.

Kendin bakmak için sunucuda şunu çalıştır:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Her satır konteynerin içindeki bir bağlamadır. Bir veri kümesinin satırı, onun konteyner içindeki yolunu (`/host/user` altında) ve veri kümesinin adını gösterir. O satırdaki bir `master:N` alanı, bağlamanın ana makinenin sonradan yaptığı bağlamaları aldığı anlamına gelir; anlık görüntü erişiminin ihtiyaç duyduğu da budur. Eksikse Host Data'nın Access Mode'unu Read/Write - Slave yap ve BombVault'u yeniden başlat.

## TrueNAS SCALE {#truenas}

- `LIBVIRT_URI` ayarlandığında (TrueNAS'taki VM yedeklerinde olduğu gibi) BombVault, zfs komutları için SSH ana makinesi, kullanıcı ve porttan ayrıca ayarlanmamış olanları URI'den alır. VM yedekleri olmadan hiçbir şey ayarlaman gerekmez: bir TrueNAS uygulamasında `host.docker.internal` çözümlenmez, bu yüzden BombVault ana makineye uygulamanın varsayılan ağ geçidi üzerinden, root olarak 22 numaralı porttan ulaşır. Başka bir adres ya da kullanıcı istiyorsan `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` ve `LIBVIRT_SSH_PORT` değerlerini uygulamanın `environment` bölümünde ayarla.
- **Ayarlar, Entegrasyonlar, Ana makine SSH** içindeki Ana makine SSH anahtarını **Credentials, Users** altında root'a ekle ve **System, Services** altında SSH hizmetini aç. Bu ya da başka bir TrueNAS'a giden bir [replika](#replica-push) ayrıca replika anahtarına ihtiyaç duyar, o farklı bir anahtardır.
- Root dışında bir kullanıcı ögenin en üstteki veri kümesi üzerinde izne ihtiyaç duyar; bu, altındaki her veri kümesini kapsar:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Yeni bir veri kümesine geri yükleme ayrıca üstteki veri kümesinde `create` izni gerektirir; saklanan özellikleri ayarlamak da o özellikler için izin gerektirir.

  TrueNAS'ta root olmayan bir SSH oturumunun yolunda `/usr/sbin` yoktur; BombVault o durumda doğrudan `/usr/sbin/zfs` çağırır.
- Uygulamanın **Host Data** değeri veri kümelerinin üstünde bir ana makine yolu olmalıdır, örneğin `/mnt/tank`; ixVolume olmamalıdır. Bir ana makine yoluyla uygulama, ana makinenin yeni bağlamalarını BombVault'a iletir (`rslave`); anlık görüntü erişiminin ihtiyaç duyduğu da budur. Uygulamanın tüm kurulumu için bkz. [TrueNAS SCALE](getting-started.md#truenas).
