# ZFS veri kümeleri

**ZFS** sayfası ZFS veri kümelerinin yedeğini alır. Bir öğe, bir veri kümesi ile altındaki tüm veri kümeleridir. BombVault her yedek için tüm ağacın tek bir ZFS anlık görüntüsünü alır, böylece içindeki her veri kümesi aynı anda yakalanır. Ardından her veri kümesinin dosyalarını bu anlık görüntüden okur, bir klasörü nasıl saklıyorsa öyle restic ile saklar ve anlık görüntüyü hemen sonra kaldırır. Yedekler tekilleştirilmiştir, her birine göz atabilirsiniz ve tek tek dosyalar geri yüklenebilir.

BombVault veri kümeleri için hiçbir zaman `zfs send` kullanmaz, hiçbir veri kümesini geri sarmaz ve hiçbirini yok etmez.

## Gereksinimler {#requirements}

- **Bu sunucuya SSH bağlantısı.** ZFS veri kümeleri VM yedekleriyle aynı anahtarı, ana makineyi ve kullanıcıyı kullanır. VM yedekleri zaten çalışıyorsa bu da çalışır. Aksi halde GitHub'daki [SSH üzerinden VM yedeği kılavuzunu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) izleyin. Şablon alanlarının adı **Host SSH: Address**, **Host SSH: Port** ve **Host SSH: User**'dır.
- **O ana makinede `zfs` komutu.** Unraid 6.12 ve sonrası ile TrueNAS SCALE'de vardır.
- **Host Data, Read/Write - Slave Access Mode ile `/mnt` olarak eşlenmiş olmalı.** Bu, şablonun varsayılanıdır. Bir veri kümesinin anlık görüntüsü, veri kümesinin `.zfs/snapshot` klasöründe ancak BombVault başladıktan sonra görünür; bu yüzden konteynerin, ana makinenin sonradan yaptığı bağlamaları alması gerekir.
- **Veri kümeleri `/mnt` altına bağlanmış olmalı.** Unraid'de havuzlar `/mnt/<pool>` altında bulunduğundan bu zaten böyledir.

Etki alanını **Ayarlar, Genel** altında açın (ZFS veri kümeleri). ZFS sayfası o zaman **Bu sunucuyla bağlantı** kartını gösterir. Kart SSH bağlantısını test eder, bağlandığı kullanıcıyı ve ana makineyi belirtir ve bir şey eksikse neyin eksik olduğunu söyler. Ana makine entegrasyonu denetimi (`/spike`) aynı sonucu gösterir.

## Öğeler ve alt veri kümeleri {#items-and-children}

ZFS sayfasında **Veri kümesi ekle**'yi açın. Liste sunucudan gelir. Yedeklemek istediğiniz şeyin en üstündeki veri kümesini seçin, örneğin `cache/appdata`; öğe onu ve altındaki her veri kümesini kapsar.

- **Yeni alt veri kümeleri kendiliğinden katılır.** Öğenin altında sonradan oluşturulan bir veri kümesi sonraki çalıştırmada yedeklenir ve o çalıştırma onu yeni olarak belirtir. İlk yedeği onu bir kez tamamen okur; sonrasında yalnızca değişiklikler okunur.
- **Tek tek alt veri kümelerini dışarıda bırakabilirsiniz.** Öğenin ayarlarında birini kapatın; altındaki her şeyle birlikte dışarıda kalır. Sunucuda artık bulunmayan dışarıda bırakılmış bir alt veri kümesi böyle işaretlenir ve listeden kaldırılabilir.
- **Okunamayan alt veri kümeleri atlanır, ama asla sessizce değil.** Çalıştırma onları listeler, öğe kaç tanesinin atlandığını gösterir ve kontrol panelindeki kapsam kartı her birini korunmayan olarak sayar. Çalıştırma yine de geri kalan her şeyi yedekler ve atlanan bir alt veri kümesi yüzünden başarısız olmaz. Nedenler [neden kodları tablosunda](#reason-codes) bulunur: bağlanmamış bir veri kümesi, `canmount=off`, `legacy` bir bağlama noktası ya da hiç bağlama noktası olmaması, yüklenmemiş bir şifreleme anahtarı, kapatılmış anlık görüntü erişimi veya BombVault'un göremediği bir bağlama noktası.
- **Atlanan bir veri kümesi alt veri kümelerini de beraberinde götürmez.** Yalnızca başka veri kümeleri içeren `canmount=off` bir veri kümesi atlanır ("yalnızca yapı" olarak gösterilir) ve bağlı alt veri kümeleri yedeklenir. Anahtarı yüklenmemiş şifreli bir veri kümesi, anahtarını paylaşan alt veri kümeleriyle birlikte atlanır.
- **VM diski veya sistem verisi olan alt veri kümeleri ekleme penceresinde kapalı başlar**, nedeni anahtarın yanında yazar. Bütün bir havuzu eklemek, içinde ne olduğunu listeleyen bir onay ister.

### Birimler {#volumes}

Bir birim (zvol) dosyalar yerine sanal bir disk içerir ve ZFS sayfası hiçbir zaman birim yedeklemez.

- Bir VM'nin kullandığı birim, o VM ile birlikte **VM'ler** sayfasında yedeklenir.
- Hiçbir VM'nin kullanmadığı bir birim (bir iSCSI extent, ayırdığınız bir disk) **BombVault tarafından yedeklenmez**. Ekleme penceresi ve ZFS sayfası bu birimleri sayar ve bunu belirtir. Daha sonraki bir sürüm onları yedekleyecek.

Bir öğenin ağacındaki birimler her çalıştırmada atlanır ve adlandırılır.

### Docker'ın depolaması {#docker-storage}

Docker'ın ZFS depolama sürücüsüyle her imaj katmanı, `legacy` bağlama noktasına sahip bir veri kümesidir. Ekleme penceresi bunları üst öğe başına tek satırda toplar. Bu tür 20'den fazla veri kümesi içeren bir ağaç öğe olamaz: onun bir anlık görüntüsü var olduğu sürece Docker imaj katmanlarını kaldıramaz. Bunun yerine altındaki veri kümelerini ekleyin, örneğin `appdata`.

### Öğeler asla çakışmaz {#overlap}

Bir veri kümesi yalnızca bir öğeye ait olabilir. BombVault, mevcut bir öğenin içinde kalan veya birini içerecek yeni bir öğeyi reddeder. Birkaç alt öğeyi tek bir üst öğede birleştirmek için önce alt öğeleri yedeklerini tutmayı seçerek silin, sonra üst öğeyi ekleyin. Her veri kümesi geçmişini kendi adı altında tutar, bu yüzden bir sonraki yedek eski öğelerin kaldığı yerden devam eder ve her şeyi yeniden okumaz.

## Anlık görüntü etrafında konteynerleri durdurmak ve komut çalıştırmak {#consistency}

Çalışan bir veritabanının anlık görüntüsü ani bir elektrik kesintisi gibidir: veritabanı genellikle toparlanır, ama toparlanmak zorundadır. Her öğe bu konuda iki şey yapabilir ve ikisi de tüm yedeği değil, yalnızca anlık görüntü anını kapsar.

- **Anlık görüntü için bu kapsayıcıları durdur.** BombVault listelenen konteynerleri durdurur, anlık görüntüyü alır ve onları hemen yeniden başlatır. Aynı bağımlılık düzeyindeki konteynerler paralel olarak durur, önce bağımlı olanlar; bu yüzden tüm pencere genellikle birkaç saniye sürer, çalıştırma ne kadar sürdüğünü gösterir. Yedek, uygulamalar zaten yeniden çalışırken dondurulmuş anlık görüntüyü okur. Yalnızca çalışmakta olan konteynerler durdurulur.
- **Anlık görüntüden önce ve sonra bir komut.** Seçtiğiniz bir konteynerin içinde çalışır; örneğin hiçbir şeyi durdurmadan, anlık görüntüden hemen önce bir veritabanını veri kümesine dökmek için. Anlık görüntüden önceki komut başarısız olursa yedek başarısız olur ve anlık görüntü alınmaz. Anlık görüntüden sonraki başarısız bir komut çalıştırmada gösterilir, ama yedeği başarısız kılmaz.

Bir şeyler ters giderse ne olur:

- Bir konteyner durdurulamazsa BombVault zaten durdurduklarını başlatır ve yedek, konteynerin adını vererek başarısız olur. Hiçbir zaman çalışan uygulamaların anlık görüntüsüne geri dönmez.
- Durdurma, devam eden bir konteyner yedeğinin bitmesini bekler (el ile çalıştırmada en fazla 30 dakika, zamanlanmış olanda yedeğin süre sınırına kadar); böylece ikisi aynı konteyneri asla aynı anda durdurup başlatmaz.
- İlk konteyner durmadan önce BombVault hangilerini durduracağını not eder. BombVault pencerenin içinde öldürülürse, bir sonraki başlatılışında o konteynerleri yeniden başlatır, bir bildirim gönderir ve öğe başlatamadığı her konteyner için kırmızı bir not gösterir.

Otomatik veritabanı dökümleri (bkz. [Özellikler](features.md)) bir ZFS öğesiyle değil, bir konteynerin **Konteynerler** sayfasındaki kendi yedeğiyle çalışır. Konteyneri yalnızca veri kümesi üzerinden yedeklenen bir veritabanı döküm almaz; bu yüzden ona burada bir komut verin.

Bir konteyner aynı anda hem bu listede hem de **Konteynerler** sayfasında olabilir. O zaman verileri iki depoda, iki kez saklanır ve **Tam yedekleme** onu iki kez durdurur. Öğe bunu belirtir.

## Geri yükleme {#restore}

Öğede **Yedekler**'i açın, yedeği ve ardından veri kümesini seçin. Varsayılan olarak bu, öğenin en üstteki veri kümesidir.

- **Veri kümesinin içine geri yükle.** Yedekteki dosyalar veri kümesinin bağlama noktasına yazılır. Aynı adlı dosyaların üzerine yazılır, diğerleri kalır. Veri kümesi hiçbir zaman geri sarılmaz veya değiştirilmez. BombVault veri kümesinin bağlı, görünür ve yazılabilir olduğunu başlamadan önce bir kez, yazmadan hemen önce tekrar denetler. İçine bir alt veri kümesi bağlanmış olan yerlere hiçbir şey yazılmaz: alt veri kümesi dosyalarını, sahibini ve izinlerini korur ve kendi yedeğinden geri yüklenir.
- **Bir klasöre geri yükle.** `/mnt` altında bir klasör seçin. BombVault klasörün bağlı bir havuzda veya paylaşımda olduğunu ve yeterli boş alan bulunduğunu denetler. Bu, SSH bağlantısı olmadan ve artık var olmayan veri kümeleri için de çalışır.
- **Dosya seç** (gelişmiş): yalnızca seçtiğiniz dosya ve klasörleri veri kümesine geri yazın.
- **Bu yedeğin bütün veri kümeleri** (gelişmiş): ağacın her veri kümesini seçtiğiniz klasörün kendi alt klasörüne. O yedekte atlanan veri kümeleri adlandırılır.
- **Başka bir sunucudan:** **Kurtarma** sayfası başka bir BombVault'un deposundan, her zaman bir klasöre geri yükler: bir yedeğin tüm veri kümeleri, her biri kendi alt klasörüne, ya da ağacın bir veri kümesi, tamamı veya seçilmiş dosyalar.

Öğenin durdurulacak konteynerler listesi, veri kümesinin içine geri yüklemede de sunulur. Bu konteynerler tüm geri yükleme boyunca durdurulmuş kalır ve bu arada konteyner yedekleri bekler.

### Güvenlik anlık görüntüsü {#safety-snapshot}

BombVault bir veri kümesine yazmadan önce yalnızca o veri kümesinin `bombvault-prerestore-<zaman>` adlı bir ZFS anlık görüntüsünü alır. Varsayılan olarak açıktır; kapatmak ikinci bir onay gerektirir. Anlık görüntü alınamazsa hiçbir şey geri yüklenmez.

BombVault bir güvenlik anlık görüntüsünü hiçbir zaman kendiliğinden silmez. Öğe bunları yaşları ve boyutlarıyla listeler, her birinde **Sil** eylemi bulunur ve en eskisi 30 günden eskiyse uyarır, çünkü havuzda silinmiş ve değiştirilmiş verileri tutar.

Bir geri yüklemeden sonra geri dönmek için veri kümesinin içindeki `.zfs/snapshot/bombvault-prerestore-<zaman>` konumundan tek tek dosyaları kopyalayın. `zfs rollback <dataset>@bombvault-prerestore-<zaman>` yalnızca bu, o veri kümesinin en yeni anlık görüntüsü olduğu sürece çalışır. `zfs rollback -r` otomatik olanlar dahil tüm daha yeni anlık görüntüleri siler.

### Yeni bir veri kümesi olarak geri yükleme {#new-dataset}

BombVault veri kümesi oluşturmaz. Onu istediğiniz özelliklerle sunucuda oluşturun, ardından bağlama noktası olan bir klasöre geri yükleyin:

```
zfs create -o compression=lz4 cache/appdata-restored
```

ve BombVault'ta `/mnt` altındaki `cache/appdata-restored` klasörüne geri yükleyin.

## Yedekte ne var {#contents}

Yedekte: yedeklenen her veri kümesinin dosya ve klasörleri, restic'in sakladığı şekliyle sahiplikleri, izinleri, zaman damgaları ve genişletilmiş öznitelikleriyle.

Yedekte olmayanlar:

- veri kümelerinin ZFS özellikleri (compression, recordsize, quota, mountpoint ve diğerleri);
- her veri kümesinin en üst klasörünün kendi sahibi ve izinleri (altındaki her şey dahildir). Veri kümesinin içine geri yükleme mevcut en üst klasörü olduğu gibi bırakır, bir klasöre geri yükleme onu `0755` izinleriyle oluşturur;
- mevcut ZFS anlık görüntüleri;
- atlanan veya dışarıda bırakılan alt veri kümeleri;
- birimler.

Yeni bir havuza geri yüklemek için önce veri kümelerini istediğiniz özelliklerle oluşturun. TrueNAS'ın SMB veri kümelerinde kullandığı NFSv4 ACL'lerinin beklediğiniz gibi geri gelip gelmediği henüz doğrulanmadı; bu yüzden onlara güvenmeden önce kendi verilerinizle bir geri yüklemeyi deneyin.

## Şifreli veri kümeleri {#encryption}

Şifreli bir veri kümesi yalnızca anahtarı yüklüyken yedeklenir. Aksi halde bir uyarıyla atlanır; anahtarı `zfs load-key` ile yükleyin ve veri kümesini bağlayın. BombVault verileri şifresi çözülmüş olarak okur ve şifreli olan restic deposunda saklar. BombVault'ta şifrelemeyi kapattıysanız o depo şifreli değildir.

## Kalan anlık görüntüler {#leftover-snapshots}

Bir yedeğin anlık görüntüsünün adı `<dataset>@bombvault-<14 rakam>` biçimindedir, örneğin `cache/appdata@bombvault-20260924021500` (UTC). BombVault onu yedekten hemen sonra kaldırır. Bu başarısız olursa, örneğin veri kümesi meşgul olduğu ya da BombVault durdurulduğu için, BombVault onu şu durumlarda kaldırır:

- o öğenin bir sonraki yedeğinden önce,
- BombVault başladığında, her öğe için, etki alanı kapalıyken de,
- öğeyi sildiğinizde,
- öğede **Şimdi kaldır**'a bastığınızda; orada kaç tane kaldığı da görünür.

Yalnızca tam olarak `bombvault-` artı 14 rakamdan oluşan adlar kaldırılır. Güvenlik anlık görüntülerine, kendi anlık görüntülerinize ve otomatik anlık görüntülere asla dokunulmaz. Birini elle kaldırmak için:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anormallikler {#anomalies}

Boşaltılmış bir alt veri kümesi büyük bir ağacın toplamını neredeyse hiç değiştirmez; bu yüzden anormallik algılama bir öğenin her veri kümesini ayrı ayrı izler: boyutu, dosya sayısı, yeni verisi ve restic süresi her birinin kendi geçmişine sahiptir. Bu geçmiş veri kümesinin adına aittir, bu yüzden ağaç daha sonra başka bir öğe tarafından yedeklendiğinde de kalır.

Önceki çalıştırmanın yedeklediği ve bu çalıştırmanın okuyamadığı bir veri kümesi, öğenin seçimi değişmediği sürece boşaltılmış sayılır. Bu, yüklenmemiş bir anahtarı, bağlanmamış bir veri kümesini ve ağaçtan kaybolmuş bir veri kümesini kapsar. Kendiniz hariç tuttuğunuz bir alt veri kümesi seçimi değiştirir, bu yüzden onun geçmişi baştan başlar. Kayıp verilerle ilgili bir bulgu açık olduğu sürece saklama yalnızca o veri kümesinin eski yedeklerini tutar ve ağacın geri kalanını her zamanki gibi budar.

**Anormallikler** sayfasının **Ögeler** sekmesinde her veri kümesinin, öğesinin altında kendi satırı vardır ve bu sayfadaki öğe ağacı her veri kümesinin yanında açık bulguları gösterir. Bir bulgudaki bağlantı, öğenin geri yükleme panelini veri kümesinin son iyi yedeğinde açar. Bir çalıştırmanın bitip bitmediği tüm öğe için değerlendirilir, çünkü bir çalıştırma bir bütün olarak başarılı ya da başarısız olur.

Denetimlerin kendisi [Özellikler](features.md) altında anlatılır. [MCP sunucusu](mcp.md) üzerinden bağlanan bir asistan bir ZFS öğesinin geri yükleme noktalarını listeleyebilir, yedeğini başlatabilir ve bulguları okuyabilir, ama bir bulgunun onaylanması **Anormallikler** sayfasında yapılır.

## Neden kodları {#reason-codes}

Sayfa, çalıştırma geçmişi ve bildirimler bir sorunu bu kodlardan biriyle adlandırır. Çoğunun çözümü sayfada da yanında görünür.

| Kod | Anlamı | Ne yapmalı |
|---|---|---|
| `ssh-missing` | Bu konteynerde SSH bağlantısı kurulmamış. | SSH bağlantısını VM yedeklerindeki gibi kurun. |
| `host-placeholder` | Host SSH: Address hâlâ örnek değer ve `host.docker.internal` de yanıt vermedi. | Host SSH: Address'i bu sunucunun LAN IP'sine ayarlayın. |
| `host-fallback` | Host SSH: Address hâlâ örnek değer ve `host.docker.internal` çalışıyor. | Hiçbir şey, ya da LAN IP'sini ayarlayın. |
| `ssh-unreachable` | Sunucuya SSH ile ulaşılamıyor. | Adresi ve portu, ayrıca SSH'nin açık olduğunu denetleyin. |
| `ssh-auth` | Sunucu BombVault'un anahtarını reddetti. | Bağlantı kartında gösterilen komutu sunucuda bir kez çalıştırın. |
| `zfs-not-found` | SSH ana makinesinde `zfs` komutu yok. | Host SSH: Address'i havuzların sahibi olan makineye yönlendirin. |
| `zfs-permission` | SSH kullanıcısı bu zfs komutunu çalıştıramıyor. | root kullanın ya da bkz. [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI`, SSH alanlarından farklı bir ana makine veya kullanıcı belirtiyor. | Bunları uyumlu hale getirin ya da ikisinin de URI'den gelmesi için SSH alanlarını boşaltın. |
| `zfs-error` | zfs başka bir hata bildirdi. | Ayrıntılar onun iletisini gösterir. |
| `propagation-missing` | Ana makinedeki yeni bağlamalar konteynere ulaşmıyor. | Host Data'nın Access Mode'unu Read/Write - Slave yapın ve BombVault'u yeniden başlatın. |
| `invalid-name` | BombVault'un kabul etmediği bir veri kümesi adı. | Veri kümesini yeniden adlandırın. |
| `name-too-long` | Ağaçtaki bir veri kümesi bir anlık görüntü adı için fazla uzun. | Onu yeniden adlandırın ya da altındaki bir veri kümesini öğe olarak ekleyin. |
| `invalid-exclude` | Bir hariç tutma deseni veya dışarıda bırakılmış bir alt veri kümesi öğeye uymuyor. | İletinin belirttiği girdiyi düzeltin. Bütün bir alt veri kümesini dışarıda bırakmak için desen yazmak yerine onu kapatın. |
| `not-found` | Veri kümesi sunucuda yok. | Öğeyi kaldırın ya da veri kümesini yeniden oluşturun. Yedekleri geri yüklenebilir kalır. |
| `not-filesystem` | Bu bir birimdir, dosya sistemi değil. | Bkz. [Birimler](#volumes). |
| `overlaps-item` | Veri kümesi mevcut bir öğeyle çakışıyor. | Bkz. [Öğeler asla çakışmaz](#overlap). |
| `docker-storage` | Ağaç Docker'ın imaj depolamasını içeriyor. | Bkz. [Docker'ın depolaması](#docker-storage). |
| `nothing-readable` | Öğedeki hiçbir veri kümesi şu anda okunamıyor. | Atlanan veri kümelerinin kodlarına bakın. |
| `snapshot-failed` | Anlık görüntü oluşturulamadı. | Ayrıntılar zfs'nin iletisini gösterir. |
| `containers-busy` | Konteynerlerin durması gerektiğinde bir konteyner yedeği hâlâ çalışıyordu. | Daha sonra yeniden başlatın. Zamanlanmış çalıştırmalar kendiliğinden bekler. |
| `consistency-stop-failed` | Bir konteyner durdurulamadı, bu yüzden anlık görüntü alınmadı. | Konteyneri denetleyin ya da listeden çıkarın. |
| `pre-snapshot-failed` | Anlık görüntüden önceki komut başarısız oldu. | Çalıştırma ayrıntıları çıktısını gösterir. |
| `container-unknown` | Listelenen bir konteyner yok. | Onu listeden çıkarın. |
| `container-is-self` | BombVault kendi konteynerini durduramaz. | Onu listeden çıkarın. |
| `leftover-snapshots` | BombVault'un kaldıramadığı anlık görüntüler hâlâ sunucuda. | **Şimdi kaldır**'a basın, bkz. [Kalan anlık görüntüler](#leftover-snapshots). |
| `zvol` | Ağaçta bir birim, atlandı. | Bkz. [Birimler](#volumes). |
| `canmount-off` | Hiç bağlanmıyor (`canmount=off`), atlandı. | Veri içeriyorsa bağlayın ya da verileri bir alt veri kümesine taşıyın. |
| `legacy-mount` | Legacy bağlama noktası, atlandı. | Ona `/mnt` altında bir bağlama noktası verin. |
| `no-mountpoint` | Bağlama noktası yok, atlandı. | Ona `/mnt` altında bir bağlama noktası verin. |
| `not-mounted` | Sunucuda bağlı değil, atlandı. | `zfs mount` ile bağlayın ya da `canmount=on` ayarlayın. |
| `key-not-loaded` | Şifreli ve anahtar yüklü değil, atlandı. | `zfs load-key`, ardından bağlayın. |
| `snapdir-disabled` | Anlık görüntü erişimi kapalı, atlandı. | `zfs set snapdir=hidden <dataset>`. `.zfs` klasörü gizli kalır. |
| `not-visible` | BombVault veri kümesinin bağlama noktasını göremiyor. | Bağlama noktasını Host Data yolunun altına taşıyın ya da Read/Write - Slave ile aynı yolda konteynere eşleyin. |
| `shfs-only` | Veri kümesi yalnızca anlık görüntüleri gizleyen `/mnt/user` üzerinden görünüyor. | Host Data olarak `/mnt/user` değil, `/mnt` eşleyin. |
| `snapshot-not-visible` | Anlık görüntü oluşturuldu ama BombVault içinde görünmedi. | **Anlık görüntü erişimini dene**'yi çalıştırın; aşağıya bakın. |
| `snapshot-loop` | Host Data yeni bağlamaları geçirmediği için anlık görüntü BombVault'a ulaşmadı. | Host Data'nın Access Mode'unu Read/Write - Slave yapın ve BombVault'u yeniden başlatın. |
| `backup-failed` | restic bu veri kümesi için başarısız oldu. | Çalıştırma ayrıntıları nedenini gösterir. |
| `not-reached` | Çalıştırma bu veri kümesinden önce bitti. | Yedeği yeniden çalıştırın. |
| `gone` | Veri kümesi artık sunucuda değil. | Hiçbir şey. Yedekleri geri yüklenebilir kalır. |
| `read-only-mount` | BombVault veri kümesini yalnızca okuyabiliyor, bu yüzden içine geri yükleyemiyor. | Eşlemeyi Read/Write - Slave yapın ya da bir klasöre geri yükleyin. |
| `destination-not-mounted` | Klasör bağlı bir havuzda veya paylaşımda değil. | Bir havuzda veya paylaşımda bir klasör seçin. |
| `not-enough-space` | Hedefte yeterli boş alan yok. | Yer açın ya da başka bir klasör seçin. |
| `safety-snapshot-failed` | Güvenlik anlık görüntüsü alınamadı, bu yüzden hiçbir şey geri yüklenmedi. | Ayrıntılar zfs'nin iletisini gösterir. |
| `safety-name-too-long` | Veri kümesinin adı bir güvenlik anlık görüntüsü için fazla uzun. | Güvenlik anlık görüntüsünü kapatın ya da bir klasöre geri yükleyin. |

### Konteynerin ne gördüğünü denetlemek {#mountinfo}

Bir öğedeki **Anlık görüntü erişimini dene**, ağacının gerçek bir anlık görüntüsünü alır, onu her veri kümesi için BombVault içinde arar ve yeniden kaldırır. İlk zamanlanmış çalıştırmadan önce tüm yolu kanıtlamanın en hızlı yoludur.

Kendiniz bakmak için sunucuda şunu çalıştırın:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Her satır konteynerin içindeki bir bağlamadır. Bir veri kümesinin satırı, onun konteyner içindeki yolunu (`/host/user` altında) ve veri kümesinin adını gösterir. O satırdaki bir `master:N` alanı, bağlamanın ana makinenin sonradan yaptığı bağlamaları aldığı anlamına gelir; anlık görüntü erişiminin ihtiyaç duyduğu da budur. Eksikse Host Data'nın Access Mode'unu Read/Write - Slave yapın ve BombVault'u yeniden başlatın.

## TrueNAS SCALE {#truenas}

- `LIBVIRT_URI` ayarlandığında (TrueNAS'taki VM yedeklerinde olduğu gibi) BombVault, zfs komutları için SSH ana makinesi, kullanıcı ve porttan ayrıca ayarlanmamış olanları URI'den alır. VM yedekleri olmadan bunun yerine `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` ve `LIBVIRT_SSH_PORT` ayarlayın. Değişkenleri **Additional Environment Variables** altında ekleyin.
- root dışındaki bir kullanıcının, öğenin en üstteki veri kümesinde izne ihtiyacı vardır; bu izin altındaki her veri kümesini de kapsar:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  TrueNAS'ta root olmayan bir SSH oturumunun yolunda `/usr/sbin` yoktur; BombVault o durumda doğrudan `/usr/sbin/zfs` çağırır.
- Uygulamanın **Host Data** değeri veri kümelerinin üstünde bir ana makine yolu olmalıdır, örneğin `/mnt/tank`; ixVolume olmamalıdır. Bir ana makine yoluyla uygulama, ana makinenin yeni bağlamalarını BombVault'a iletir (`rslave`); anlık görüntü erişiminin ihtiyaç duyduğu da budur.
