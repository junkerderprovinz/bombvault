# Site dışı ve kurtarma

!!! note "Site dışı kopyalar bir yeniden kurulumdan sonra bekler"
    4. adım eski ayarlar olmadan girişleri yeniden kurduğunda, o etki alanlarının site dışı çoğaltması, yerleşim varsayılanı onaylanana kadar duraklar. Bkz. [Öge başına yerleşim](#placement).

Yerel yedekler sizi kaybolmuş bir konteynerden ya da hatalı bir güncellemeden korur. Site dışı çoğaltma ve test edilmiş bir kurtarma kiti sizi tüm makineden, fidye yazılımından ya da bir yangından korur. Bu sayfa site dışına çoğaltmayı, o kopyayı kurcalamaya dayanıklı yapmayı, geri yükleyebildiğinizi kanıtlamayı ve BombVault'un kendisi kaybolduğunda kurtarmayı kapsar.

## Site dışı çoğaltma

Hızlı yerel yedeği tutun ve bir veya daha fazla site dışı kopya ekleyin. **Ayarlar, Site dışı** sayfasında etki alanı başına bir depo ayarlayın. BombVault yeni anlık görüntüleri oraya en iyi çaba temelinde `restic copy` ile çoğaltır, böylece bir site dışı aksaklık asla yerel yedeklemeyi bozmaz. Bu düzende yerel depo birincil kalır ve site dışı depo bir kopyadır, ama bir etki alanının birincil deposunun yerel olması hiç gerekmez; S3'e, rest-server'a vb. çoğaltmak yerine doğrudan oraya yedeklemek için aşağıdaki [Uzak birincil depolar](#remote-primary-repositories) bölümüne bakın.

- **Etki alanı başına birden fazla site dışı hedef.** Her etki alanı (konteynerler, VM'ler, flash, config, dosya kümeleri ve ZFS veri kümeleri) yalnızca birine değil, aynı anda birkaç site dışı hedefe çoğaltabilir, böylece örneğin bir arkadaşınızın makinesinde bir rest-server ve paralel olarak bir S3 kovası tutabilirsiniz. Ayarlar, Site dışı'nda her biri kendi deposu, S3 depolama sınıfı, yalnızca ekleme bayrağı, saklama ve büyüme bütçesiyle ek hedefler ekleyin. Mevcut tek bir site dışı kurulum, ilk hedef olarak dokunulmadan taşınır ve bir etki alanının her hedefi o etki alanının site dışı zamanlamasında çoğaltılır.
- **Etki alanı başına site dışı zamanlama** (Ayarlar, Zamanlamalar'da diğer her zamanlamanın yanında düzenlenir): her yerel yedeklemeden sonra çoğaltmak için boş bırakın ya da yerelde yedeklediğinizden daha seyrek site dışına göndermek için bir sıklık ayarlayın (örneğin `weekly Sun 03:00`). Bir **Şimdi çoğalt** düğmesi istek üzerine çalışmaları kapsar.
- **Site dışı saklama** Ayarlar, Saklama'da yer alır, böylece site dışı kopyaları bir arşiv olarak daha uzun tutabilirsiniz. Site dışı anlık görüntüleri asla otomatik kırpmamak için ilkeyi tümü sıfır bırakın.
- **Bant genişliği sınırları** (Ayarlar, Site dışı) restic yükleme/indirme hızını sınırlar, böylece çoğaltma WAN'ınızı doyurmaz.
- Bir **çoğaltma göstergesi**, çalışırken hangi etki alanının çoğaltıldığını gösterir (kendi sayfasında ve Kontrol Paneli'nde). Bu bir etkin göstergedir, bir yüzde çubuğu değil, çünkü `restic copy` makine tarafından okunabilir bir ilerleme sunmaz.

!!! note "Herhangi bir yerden geri yükleyin"
    Her konteyner, VM, dosya kümesi, flash ve uygulama yapılandırması, yedeklerini bir yedeğin bulunduğu tüm yerler boyunca tek bir zaman çizelgesi olarak listeler. B2'ye kopyalanan bir yedek yalnızca bir kez görünür, onu tutan her yerle işaretlenmiş olarak. Bir geri yükleme, ulaşabildiği ilk yeri alır, ögenin yazıldığı depoyla başlayarak, ve her satır için başka bir yer seçebilirsin. Site dışı yerler yalnızca onları açtığında okunur. Bir yerde silme, önce diğerlerini kontrol eder ve bunun son kopya olup olmadığını söyler.

## Hedefler {#destinations}

Ayarlar, Site dışı **Hedefler** ile başlar: site dışı kopyaların gittiği yerler, tüm etki alanları için bir kez kurulur. Bir hedef sonra her etki alanının ve her ögenin **Yerleşim** satırında bir düğme olarak görünür. Bir etki alanı için ilk kez işaretlendiğinde BombVault, o etki alanının deposunu hedefin altındaki bir klasörde oluşturur, örneğin `rclone:onedrive:BombVault/containers`. Flash, Öz yedek ve ZFS veri kümelerinin **Yerleşim** satırı yoktur; bu yüzden site dışı bölümleri bunun yerine hedefin adını ve ardından **hedefinden ekle** düğmesini sunar.

**Hedef ekle**, beş adımlı bir sihirbaz açar:

1. **Yedekler nereye gitsin?** Her hizmet logosuyla listelenir, dört grupta: S3 bucket'ları sunan depolama hizmetleri (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 ve diğerleri), kendi S3 sunucun (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), kendi sunucun ve paylaşımların (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, bağlanmış bir yol) ve bulut depolama (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud ve rclone'un desteklediği diğerleri). Her biri yedeklemeye ne kadar uygun olduğunu söyler: bulut sürücüleri çok sayıda istekte yavaşlar, bu yüzden ilk yedekleme ve budama orada daha uzun sürer.
2. **Oturum aç.** Alanlar hizmete göre değişir: S3 için bir erişim anahtarı, WebDAV ve SMB için kullanıcı adı ve parola, iki adımlı doğrulamanın normal parolayı engellediği yerlerde bir uygulama parolası, SFTP ve Storage Box için BombVault'un genel SSH anahtarı ya da tarayıcı üzerinden oturum açan hizmetler için bir belirteç. Bunlar için sihirbaz, tarayıcısı olan bir bilgisayarda çalıştırman gereken bir `rclone authorize` komutu gösterir; yazdırdığı belirteç alana girilir. **Bağlantıyı test et**, hiçbir şey kaydedilmeden önce oturum açmayı denetler.
3. **Bir klasör seç.** Sihirbaz hedefteki klasörleri listeler; yeni bir klasör için **Yeni klasör** vardır ve hizmet bildiriyorsa boş alan gösterilir. Boş bir klasör en güvenlisidir.
4. **Silmeye karşı koruma.** Sihirbaz hizmetin neler yapabildiğini açıkça söyler. Append-only kipindeki bir rest-server silmeyi reddeder ve kurcalama testi bunu denetler. Bir S3 bucket'ı sürümleme ve nesne kilidiyle eski sürümleri tutabilir; BombVault bunu henüz denetleyemez. Bir bulut sürücüsü silmeyi hiç reddedemez: sunucuya giren o kopyaya da girer. **Değiştirilemez (append-only)** seçeneğini yalnızca uzak taraf silmeyi gerçekten reddediyorsa aç; BombVault o zaman orada hiç budama yapmaz.
5. **Acil durum için.** Kurtarma kiti her hedefi, altındaki her etki alanının deposuyla birlikte listeler. Oturum açma bilgisi BombVault'un ayar yedeğiyle geri gelir; o olmadan yapılan yeni bir kurulumda hedefi aynı yerde yeniden kur.

S3 hizmetleri restic'in kendi S3 arka ucu üzerinden çalışır; depolama sınıfı ve nesne kilidi bu sayede uygulanabilir. Diğer tüm hizmetler BombVault'un içinde gelen rclone üzerinden çalışır ve remote'u sonra Ayarlar, Bulut erişimi altındaki rclone yapılandırmasında görünür. Bir ayar dışa aktarımı hedefleri içerir; kimlik bilgileri dahil edilirse oturum açma bilgilerini de içerir.

Grubunuzun başka bir örneğinin çalıştırdığı alıcı sunucu, sihirbazda **Grubundan** altında görünür; bkz. [Alıcı sunucu](#receiving-server).

Bir hedeften oluşturulan etki alanı hedefi, hedefin adını, konumunu, kimlik bilgilerini, depolama sınıfını ve değiştirilemez anahtarını alır. Saklaması, sıkıştırması ve büyüme bütçesi etki alanı başına kalır; konumu taşınamaz, çünkü etki alanının deposu oradadır. Her etki alanının altındaki **Yalnızca bu etki alanı için hedef ekle** yine elle yazılmış bir depo URL'si alır.

Bir etki alanının off-site alanı, yani birincil kopyası, bir hedefi de izleyebilir. Alanın altında, etki alanının henüz kopyalamadığı her hedef için bir **kullan** düğmesi bulunur. Düğme birincil kopyayı o hedefin altındaki etki alanı klasörüne taşır; kopya bundan sonra hedefin adını, kimlik bilgilerini, depolama sınıfını ve değiştirilemez anahtarını alır, saklaması ise Saklama sayfasında kalır. Alan o andan itibaren konumu kilitli gösterir. **Konum yaz** kilidi açar, elle yazılmış bir konumu kaydetmek de bağlantıyı sonlandırır.

Elle yazılmış ve deposu bir hedefin klasöründe bulunan bir etki alanı hedefi, o hedefe katılabilir. Hedef, bu tür hedefleri **Zaten bu hedefin altında** başlığı altında listeler ve **Devral** bunlardan birini hedefin altına asar. Etki alanı hedefi deposunu, snapshot'larını, saklamasını ve yerleşimini korur; hedefin adını, kimlik bilgilerini, depolama sınıfını ve değiştirilemez anahtarını alır. BombVault önce hedefin oturum açmasının depoyu açtığını doğrular ve append-only olmayan bir hedefin altına append-only bir hedef koymayı reddeder. Devralınan etki alanı birincil hedefi, o etki alanının off-site alanında kalır.

## Öge başına yerleşim {#placement}

Her konteyner, VM ve dosya kümesi kartında düğmelerden oluşan bir **Yerleşim** satırı vardır: **Yerel** ve etki alanının her site dışı hedefi için bir düğme, ardından etki alanının altında henüz hedef oluşturmadığı Hedefler. Yanan düğmeler ögenin yedeklerini alır.

- **Yerel** yanıkken öge, **Depolama konumu** altında gösterilen depoya yazılır ve yanan her diğer hedefe kopyalanır. Bir hedefi söndür, o hedef bu ögeden artık yeni bir şey almaz. Tek başına Yerel hiçbir yere kopyalamaz; bu, zaten bir ikinci kopyası olan veriler için uygundur, örneğin bir NAS üzerinde yaşayan bir paylaşım.
- **Yerel** sönükken öge, yanan ilk hedefin doğrudan deposuna yazılır ve oradan yanan diğer hedeflere kopyalanır. İlk seferde bir iletişim kutusu bu doğrudan depoyu oluşturur.
- Bir Hedefler düğmesi, etki alanının hedefini o Hedefler girdisinin altında oluşturur ve yalnızca bu öge için yakar. Diğer her öge orada kopyasız başlar.
- Bir düğme her zaman yanık kalır, çünkü bir yedeğin gidecek bir yere ihtiyacı vardır. Bir şeyi yedeklerin dışında bırakmak için onu hariç tut.

Konum, ögenin ilk yedeklemesinden itibaren sabittir, çünkü BombVault yedekleri depolar arasında asla taşımaz. Kopyalar ise her zaman değişebilir. Bir ögeyi artık almayan bir hedef, sahip olduğu kopyaları tutar ve etki alanının bir sonraki site dışı çalıştırmasında kendi saklama kuralına göre kırpar; karttaki **B2 içinde sil**, onları hemen kaldırır. Bu kopyalardan bazıları başka hiçbir yerde yoksa, onay ekranı bunları tarihe göre listeler ve ögenin adını sorar. Yalnızca ekleme hedeflerden hiçbir şey silinemez.

Satırın altında kart, ögenin nereye gittiğini ve gerçekte nerede olduğunu söyler: kaç sitenin onu tuttuğunu, her hedefin en son ne zaman görüldüğünü ve 3-2-1'in karşılanıp karşılanmadığını. Bir site, orijinal verinin bulunduğu sunucu, her site dışı hedef ve **Bina dışında** olarak işaretlenmiş her depodur. BombVault kopyaları ve siteleri kontrol eder; 3-2-1'in "iki ortam" kısmını kontrol etmez.

### Varsayılan yerleşimler

Ayarlar, Depolama, **Varsayılan yerleşimler**'de aynı düğmelerle etki alanı başına bir satır vardır. Kopyalar, kendi seçimi olmayan her ögeye ve Compose yığınlarının proje klasörlerine hemen uygulanır. Konum, yeni bir ögeye ilk yedeklemesinde uygulanır; onu değiştirmek hiçbir yedeği taşımaz. Kaydetmeden önce satır, öge kazanan ya da kaybeden her hedefi ve bunun kaç anlık görüntü anlamına geldiğini adlandırır. **Yedeksiz ögelere uygula**, henüz yedeği olmayan her ögeyi varsayılana geri döndürür.

Yeni bir site dışı hedef, Yerel'e ayarlanmamış her ögeyi alır. Onu ekleyen iletişim kutusu kaç öge olduğunu ve, biliniyorsa, bunun ne kadar geçmiş anlamına geldiğini söyler, ve diğer hedeflerden zaten hariç tutulan ögeleri dışarıda bırakmayı önerir.

### Doğrudan depolar

Bir öge için Yerel'i kapatmak, yani doğrudan deposu olmayan bir hedefin onun yuvası olması, hedefin yanında önerilen bir konumla, örneğin `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, ve hiçbir şey oluşturmayan bir bağlantı testiyle bir iletişim kutusu açar. **Oluştur ve kullan**, depoyu oluşturur ve ögeyi ona yöneltir. Doğrudan bir depo, hedefin anahtarını, depolama sınıfını, sınırlarını, append-only ayarını ve saklamasını alır ve onlarla birlikte değişir; Depolar kartı onu salt okunur gösterir. Hedef için yeni bir anahtar onu açamadığında, doğrudan depo sahip olduğu anahtarı tutar ve kayıt bunu belirtir. Doğrudan depodaki bir öge oradan yanan diğer hedeflere kopyalanır, deponun ait olduğu hedefe asla. Anlık görüntüleri `bv:direct` etiketini taşır ve diğer her budama bunları tutar, böylece hedefiyle bağlantısını kaybetmiş bir doğrudan depo yerel kurallara göre asla yaşlanmaz. B2'ye S3 uç noktası üzerinden erişilir; anahtar kimliği ve uygulama anahtarı S3 kimlik bilgileri olarak girilir. Hedefin kendi klasörüyle sınırlı bir anahtar, yanındaki klasöre erişemez, bu yüzden anahtarı bunun yerine hedefin üstündeki klasörle sınırla.

### Bina dışında

Adlandırılmış bir depo, Depolar kartında **Bina dışında** olarak işaretlenebilir. Uzak depolar işaretli başlar; aynı binadaki bir rest-server için bunu kapat. İşaret, kartlardaki siteleri ve 3-2-1'i yalnızca sayar. Hiçbir kopyayı değiştirmez.

### Bir yeniden kurulumdan sonra

Kopyalama seçimleri BombVault'un kendi ayarlarında yaşar. Geri yüklenmiş bir `/config` olmadan Yedekleri keşfet üzerinden bir yeniden kurulumdan sonra bunlar kaybolur ve her şeyi kopyalamak, dışarıda bıraktığın ögeleri tekrar B2'ye gönderir. Bu yüzden yeniden kurulan her etki alanının site dışı çoğaltması duraklar. Kontrol Paneli bunu kehribar renginde gösterir ve Varsayılan yerleşimler, bir sonraki çalıştırmanın neyi kopyalayacağının ve burada karşılığı olmayan yedeklerdeki adların bir önizlemesiyle **Varsayılanı onayla** sunar; bunları orada dışarıda bırakabilirsin. Yalnızca onay duraklamayı bitirir; bir ayar dosyası içe aktarmak kuralları ve varsayılanları geri getirir ama duraklamayı bitirmez.

## Uzak birincil depolar {#remote-primary-repositories}

Bir alanın yedekleme yolu (Ayarlar, Depolama) yerel bir klasörle sınırlı değildir: doğrudan bir restic uzak deposuna yöneltin (`s3:...`, `rest:http://host:8000/depo`, `sftp:kullanici@host:/depo`, `rclone:remote:bucket/yol`), BombVault ayrı bir yerel kopya ve çoğaltma adımı olmadan doğrudan oraya yedekler. Bu, yukarıdaki saha dışı çoğaltmadan gerçekten farklı bir biçimdir: orada yerel depo birincildir ve saha dışı depo onun elden geldiğince tutulan arşividir; burada uzak depo birincilin **kendisidir** ve o alan için ayrıca bir saha dışı çoğaltma (ya da ikinci bir uzak depo) kurmadığınız sürece tek kopyadır.

Altı yol alanının her birinin (Konteynerler, VM'ler, Flash, Öz yedek, Klasörler, ZFS veri kümeleri) hemen yanında bir **Yerel / Uzak** anahtarı vardır:

- **Yerel** alışılmış klasör tarayıcısını gösterir.
- **Uzak** onu yalın bir URL alanıyla değiştirir; yanına da, saha dışı hedeflerin kullandığı bağlantı testi ve kimlik bilgileri penceresinin aynısını bu birincil depo için ayarlanmış olarak açan bir düğme koyar. Oradan şunları elde edersiniz:
    - **Bir bağlantı testi**, gerçek yola karşı, ona güvenmeden önce.
    - **Bant genişliği sınırları** (gönderme ve alma), böylece uzak bir birincil depoya yapılan zamanlanmış yedekleme WAN hattınızı doldurmaz: saha dışı çoğaltmanın kullandığı `--limit-upload` ve `--limit-download` restic seçeneklerinin aynısı, bu kez yedeklemenin kendisine uygulanır.
    - **Yalnızca-ekleme (değiştirilemezlik) koruması**, saha dışı hedeflerin aldığı etkin kurcalama testinin aynısıyla doğrulanır (karşı tarafa gerçek bir DELETE denemesi). Açıkken BombVault deponun kendisini budamayı reddeder: arkasında ayrı bir yerel kopya bulunmadığına göre, bu makinedeki kimlik bilgileri yedeğin tek kopyasını silebilecek durumda olmamalıdır.
    - **Bir büyüme bütçesi uyarısı**, Depolama kartının zaten izlediği depo boyutu eğiliminin aynısından türetilir.

Bunların hiçbiri zorunlu değildir: elle yazılmış, kayıtlı güvenlik ayarı olmayan bir uzak yol tam da eskisi gibi yedekler (sınırsız bant genişliği, budanabilir, bütçe uyarısı yok). Güvenlik penceresi, saha dışı bir kopyanın aldığı korumaların aynısını, salt bunun için ayrı bir saha dışı hedef kurmak zorunda kalmadan istediğiniz durum içindir.

!!! note "Bulut ve REST kimlik bilgileri ortaktır"
    Uzak bir birincil depo, Ayarlar, Bulut erişimi, Paylaşılan bulut kimlik bilgileri altında yapılandırılan S3/REST kimlik bilgilerinin aynısıyla kimlik doğrular. Birincil depolar için ayrı bir kimlik bilgisi deposu yoktur.

### Host'ta bağlama olmadan SMB ve WebDAV {#smb-webdav}

Ayarlar, Bulut erişimi, rclone altında bir Windows ya da Samba paylaşımı ve bir WebDAV sunucusu (Nextcloud, ownCloud, SharePoint ya da herhangi bir başkası) için bir form vardır. Kısa bir ad, host ve paylaşım (SMB) ya da URL ve sunucu türü (WebDAV), kullanıcı ve parolayı doldurun; BombVault rclone bölümünü sizin için yazar. rclone parolayı saklanmadan önce kendisi gizler; zaten var olan bir adla bir hedef eklemek ikinci bir bölüm eklemek yerine o bölümü değiştirir.

Form, tamamlanmış konumla yanıt verir, örneğin `rclone:nas:backups`. Bunu bir Yedekleme Yolu'na ya da bir site dışı hedefe girin ve isterseniz bir alt klasör ekleyin (`rclone:nas:backups/bombvault`). Paylaşım adın bir parçası değil, yolun ilk bölümüdür.

Bu, paylaşımı Unraid'e bağlamaktan daha iyi bir yoldur: restic bir depoyu bağlanmış bir CIFS paylaşımında tutmamayı önerir ve burada hiçbir şey bağlanmaz. NFS formda yoktur, çünkü ne restic'in ne de rclone'un bir NFS arka ucu vardır; NFS için dışa aktarımı host'a bağlayın ve bir Yedekleme Yolu'nu ona yönlendirin.

## Değiştirilemez (yalnızca ekleme) site dışı

Fidye yazılımı ya da ele geçirilmiş bir host yedeklerinizi silemesin veya yeniden yazamasın diye bir site dışı depoyu yalnızca ekleme olarak işaretleyin. Karşı taraf (`--append-only` modunda çalışan bir `restic/rest-server`) bunu **uygular**. BombVault yalnızca bunu **doğrular** ve asla yalnızca bir yapılandırma iddiası üzerine yeşil göstermez.

**Rehberli site dışı kurulum** sihirbazı sizi arka uç seçiminden (rest-server / rclone / S3), yapıştırmaya hazır bir rest-server dağıtım parçacığı, bir bağlantı testi, değiştirilemez geçişi (bu, kurcalama testini hemen çalıştırır) ve bir saklama stratejisinden geçirir, böylece yalnızca ekleme site dışına yapılandırmaları elle düzenlemeden ulaşılabilir.

!!! note "`/locks/` altında başarılı bir silme beklenen davranıştır"
    Append-only, artık hiçbir şeyin silinemeyeceği anlamına gelmez. restic kendi kilitlerini alıp bırakmak zorundadır, bu yüzden `/locks/` bilerek yazılabilir ve silinebilir kalır. Anlık görüntüler ve arkasındaki veriler, yani fidye yazılımının hedefi tam olarak budur, kaldırılamaz. Uzak tarafı kendiniz denerseniz, `/locks/` altında başarılı olan bir silme doğru davranıştır ve korumada bir delik değildir.

!!! warning "Değiştirilemez depolar bu makineden asla budanmaz"
    Değiştirilemez bir site dışı, eski anlık görüntüleri kasıtlı olarak asla budamaz. Depo boyutu kontrolden çıkmadan önce uyarılmanız için ona bir **büyüme bütçesi alarmı** ayarlayın.

## Kurcalama testi

BombVault, yalnızca ekleme garantisini, site dışı depoya karşı var olmayan bir nesneyi hedefleyen gerçek bir silme girişiminde bulunarak periyodik olarak kanıtlar:

- **Reddedildi**, korunuyor demektir.
- **Kabul edildi**, korunmuyor demektir.
- **Sonuçsuz** bir sonuç (sunucuya ulaşılamıyor, kimlik doğrulama hatası) saklanan kararı asla değiştirmez.

Gerçek bir korunuyordan-korunmuyora dönüş tek bir uyarı tetikler.

## DR tatbikatları

BombVault, yedeklerinizin yalnızca mevcut değil, gerçekten geri yüklenebilir olduğuna dair iki düzeyde kanıt sunar.

- **Geri yükleme doğrulama tatbikatları (yerel).** BombVault periyodik olarak `restic check --read-data-subset` çalıştırır (sınırlı, asla diski dolduran tam bir geri yükleme değil) ve etki alanı başına bir *Geri yüklenebilir olduğu doğrulandı* rozeti gösterir. Sıklık Ayarlar, Zamanlamalar'da; rozet Ayarlar, Bütünlük'te yer alır.
- **DR tatbikatları (site dışı).** BombVault gerçek bir hedefi site dışı depodan tek kullanımlık bir korumalı alana geri yükler, onu dosya-dosya ve bayt-bayt doğrular, ardından temizler. Bu, deponun yalnızca yanıt verdiğini değil, site dışından kurtarabildiğinizi kanıtlar.

Kontrol Paneli'ndeki **fidye yazılımı koruması karnesi** bunu etki alanı başına yeşil / sarı / kırmızı bir duruşa, yaş damgalı bir kontrol listesiyle (site dışı yapılandırıldı, yalnızca ekleme doğrulandı, çoğaltma güncel, geri yükleme tatbikatı geçti, şifreleme açık, budama stratejisi ayarlandı) toplar. Her kırmızı satır düzeltmeye derin bağlantı verir ve kart yalnızca doğrulanmış gerçekler üzerine yeşile döner.

## Örnekleri eşleştirme {#pairing}

Alıcılar, çekme kaynakları, Örnekler sayfası ve Mesh site dışı, hepsi başka bir BombVault ile konuşur. Bunu tek bir eşleştirme grubunun üyeleri olarak yaparlar ve bir örnek gruba on iki kelimeyle katılır.

İlk örnekte **Ayarlar → Eşleştirme** sekmesini açın ve eşleştirme kartlarında **İfade oluştur** düğmesine basın. On iki kelime, **Kopyala** düğmesi olan bir pencerede görünür. Diğer her örnekte aynı yeri açın, **İfade gir** düğmesine basın ve kelimeleri yapıştırın ya da yazın, ya da o pencerede **Yapıştır** düğmesine basın. Listede olmayan bir kelime yazarken hemen konumuyla birlikte belirtilir, son kelime ise bir sağlama toplamı taşır, böylece yanlış yazılan ya da yer değiştiren bir kelime herhangi bir eşleştirme olmadan önce yakalanır. İfadeyi yalnızca bir örnekte oluşturun: her ikisi de ifade oluşturan iki örnek, iki ayrı grup oluşturur. Bir dakika boyunca kimse görünmezse, sekme iki çıkış yolu sunar: kelimeleri tekrar göstererek karşı tarafta girmek, ya da diğer örneğin kelimelerini girip onun grubuna tek adımda katılmak. Eşleştirme oturum açma parolası olmadan da çalışır, ama bir tane belirleyin: parola yoksa bu web arayüzünü açabilen herkes kelimeleri okuyabilir ve grup üzerinden içindeki her örneğin restic parolasını elde edebilir. Parola belirlenene kadar eşleştirme kartı bunu belirtir. Parola varsa, ifadeyi tekrar göstermek bu parolayı ister. **Gruptan ayrıl**, bir örneği gruptan tekrar çıkarır.

Kelimeleri bilen herkes gruba katılabilir, bu yüzden onlara bir parola gibi davranın.

**Üyeler birbirine nasıl ulaşır.** Her örnek, oturum açtığınız anda kendi ağ adresini tarayıcınızdan öğrenir; bu adres röle kartında **Ağındaki bu örnek** olarak gösterilir, önünde bir ters proxy veya alışılmadık bir port varsa orada düzeltebilirsiniz. Aynı ağdaysa üyeler bu adresi çoklu yayınla duyurur ve doğrudan konuşurlar; çoklu yayının Docker'ın varsayılan köprü ağı gibi bir konteyner ağını aşamadığı yerlerde ise bir örnek, yalnızca bir grup üyesinin yanıtlayabileceği imzalı bir çağrıyla kendi alt ağını tarayarak diğerlerini bulur, böylece eşleştirme rölesiz de saniyeler içinde tamamlanır. Hiçbir şey bulunmazsa, eşleştirme kartının altındaki **Bulamıyor musun?** bir adresi elle girmenizi sağlar; başka bir alt ağ veya standart olmayan bir port için. Farklı ağlardaki örnekler, aynı sekmeden seçilen bir röle üzerinden gider:

- **Proje rölesi** (varsayılan): `parleyport.halleluja.design`, KnightLoader'ın da kullandığı röle. Kurulacak hiçbir şey yok.
- **Kendi rölen**: Unraid Community Apps'ten [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) konteyneri ya da **Röle olarak çalış** açık olan, dışarıdan zaten erişilebilir örneklerinizden biri. O örnek daha sonra kendi adresinde `/relay/connect`'te yanıt verir, zaten sahip olduğu ters proxy ve sertifikanın arkasında, ve yalnızca grubunuzu içeri alır. Röleyi kullanması gereken her örneğe rölenin adresini girin.
- **Röle yok**: üyeler birbirini yalnızca aynı ağda otomatik olarak bulur, başka hiçbir yerde bulamaz.

**Rölenin gördükleri.** Üyeler arasındaki her çağrı, on iki kelimeden türetilen bir anahtar altında AES-256-GCM ile mühürlenir ve bu anahtar örneklerinizden asla çıkmaz. Röle yalnızca bağlantıları gruplayan bir özet, bir mesajın hangi örnek için olduğunu, ne kadar büyük olduğunu ve ne zaman geçtiğini öğrenir. Yerel ağdaki doğrudan bir çağrı da aynı şekilde mühürlenir ve ayrıca imzalanır, böylece hiçbir şey bir örneğin sunduğu kendinden imzalı sertifikaya bağlı değildir.

**Grup üzerinden neler geçer.** Örnekler sayfasındaki karneler, bir etki alanını şimdi kontrol etme isteği, Mesh site dışı teklifleri ve bir alıcının ya da çekme kaynağının ihtiyaç duyduğu şey: diğer örneğin depo konumları ve onun restic parolası. Yedekleme verisi asla geçmez; her zaman doğrudan restic arka uçlarına gider. APP_KEY da geçmez: restic parolası yalnızca o örneğin depolarını açar, başka hiçbir şeyi açmaz, ne saklanan sırlarını, ne oturumlarını ne de kurtarma kodlarını.

**Eşleştirmeden önceki kayıtlar.** Bir filo belirteciyle eklenen örnekler ile diğer örneğin APP_KEY'iyle kurulan alıcılar ve çekme kaynakları, güncellemeden sonra da kalır ve **Yeniden eşleştir** olarak işaretlenir. Alıcılar ve çekme kaynakları çalışmaya devam eder: ilk başlangıcında BombVault, saklanan her APP_KEY'i ondan türetilen restic parolasıyla değiştirir. İki örneği eşleştirin, ardından kaydı düzenleyip örneğini seçin. Böyle bir örnek, aynı adı taşıyan bir örnek grupta belirir belirmez eski kartını devralır.

Elle bir APP_KEY almaya devam eden tek yer [Başka bir BombVault deposundan geri yükleme](#restore-from-another-bombvault-repo)'dir; diğer örneğin kaybolduğu ve bir grup içinde yanıt veremediği durumlar için.

## Alıcı kontrol paneli (alan taraf)

![Alıcı taraf, salt okunur izlenir, bütünlük kontrolü bu makinede çalıştırılır.](assets/screenshots/receiver.png)

*Alıcı taraf, salt okunur izlenir, bütünlük kontrolü bu makinede çalıştırılır.*

Yukarıdaki her şey *gönderen* taraftır. Başka bir BombVault'tan değiştirilemez site dışı kopyalar **alan** makinede, Alıcı kontrol paneli size o depoların alan donanımda bağımsız, salt okunur izlemesini verir, böylece karşı uçtaki sessiz bir hata fark edilmeden kalmaz.

Bir **Alıcı** sekmesini ortaya çıkarmak için Ayarlar'da **Alıcı** geçişini açın. Varsayılan olarak kapalıdır; onu yalnızca gerçekten değiştirilemez site dışı yedekler alan bir makinede etkinleştirin. Ardından şunları elde etmek için alınan bir depoyu (salt okunur, gönderen örneğin restic parolasıyla açılmış; bu parola [eşleştirme grubu](#pairing) üzerinden gelir) kaydedin:

- **Kaynağa göre gruplanmış bir anlık görüntü envanteri**, böylece hangi konteynerlerin, VM'lerin ve dosya kümelerinin geldiğini tam olarak görebilirsiniz.
- Kaynak başına **Son alınan**, böylece her birinin ne kadar taze olduğunu bilirsiniz.
- Alan donanımda çalışan **bağımsız bir `restic check`**, böylece bütünlük yalnızca göndericide değil, verinin gerçekten bulunduğu yerde doğrulanır.
- **Bir ölü adam anahtarı:** bir kaynak, ayarladığınız bir pencere içinde göndermeyi durdurduğunda bir uyarı.
- **Bütünlük uyarıları:** alan tarafta bir denetim başarısız olduğunda bir uyarı.

Alıcı kesinlikle salt okunurdur. Alınan depoya asla yazmaz, böylece göndericinin dayandığı yalnızca ekleme garantisini asla bozamaz.

### Alıcı sunucu {#receiving-server}

Alan makine, diğerlerinin kopyaladığı rest-server'ı da çalıştırabilir. Alıcı sekmesinin üstündeki **Alıcı sunucuyu kur**, bir paylaşımda bir klasör ister; **Yeni klasör** ile yenisi oluşturulur. Ayrıca bir port ister (başka bir kapsayıcı kullanmıyorsa 8000). BombVault ardından şunları yapar:

1. `rest-server` adında bir kapsayıcı zaten varsa ya da portu başka bir kapsayıcı tutuyorsa reddeder;
2. `restic/rest-server` imajını çeker ve Docker soketi üzerinden yalnızca ekleme kipinde, özel depolarla ve klasörde bir oturum dosyasıyla başlatır;
3. Unraid şablonunu flash sürücüye yazar, böylece kapsayıcı Docker sekmesinde düzenlenebilir kalır; flash sürücüye ulaşılamıyorsa şablonu indirme olarak sunar;
4. sunucuya karşı kurcalama testini çalıştırır ve silme isteklerini reddedip reddetmediğini gösterir.

Grubunuzdaki örnekler sunucuyu hedef sihirbazında **Grubundan** altında, alan makinenin adıyla bulur. Her örnek sunucuyu ilk seçtiğinde kendine ait bir oturum bilgisi alır ve orada yalnızca kendi klasörüne yazar. Kart bu oturum bilgilerini listeler, **Oturum bilgisini geri al** birini kaldırır; o örneğin o ana kadar kopyaladıkları klasörde kalır. Kurulum ayrıca grubun dışındaki biri için bir oturum bilgisi oluşturur; kart bunun parolasını yalnızca bir kez gösterir.

Alan makineye yalnızca röle üzerinden ulaşan bir örnek sunucuyu kullanamaz, çünkü röle yedek taşımaz. Önce alan makinenin adresini Ayarlar, Eşleştirme altına ekleyin. BombVault kendi IP adresinde çalışıyorsa (örneğin br0 üzerinde), **Ortaklar için adres** alanını doldurun, çünkü sunucu ana makinenin adresinde dinler.

## Baştan sona örnek: iki Unraid makinesi

Yukarıda parçalar anlatılıyor. Burada gerçek değerlerle tek bir eksiksiz kurulum var, çünkü parçaları bir kez birleştirilmiş halde görmek işi çok kolaylaştırır.

İki makine: **TOWER** kapsayıcıları çalıştırır ve yedekleri gönderir, **VAULT** onları alır ve değiştirilemezliği dayatır. Kendi adlarınızı, adreslerinizi ve paylaşım yollarınızı koyun.

**1. VAULT üzerinde append-only sunucusunu kurun.** TOWER üzerindeki BombVault'ta *Ayarlar → Site dışı → Kur* bölümüne gidin, **rest-server** seçin ve tarifi oluşturun. **Unraid şablonu (XML)** sekmesini kopyalayın, VAULT üzerinde `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` olarak kaydedin, sonra *Docker → Add Container* deyip şablon listesinden **rest-server** seçin. Başlatmadan önce gösterilen `htpasswd` satırını VAULT üzerinde `/mnt/user/appdata/rest-server/.htpasswd` dosyasına yazın. Tek kullanımlık parola bir kez gösterilir ve hiç saklanmaz, şimdi kopyalayın. O satır aynı parolayı taşır, senin için bcrypt ile özetlenmiş olarak: düz metin TOWER üzerindeki REST kimlik bilgilerine, özetlenmiş satır VAULT üzerindeki `.htpasswd` dosyasına gider. Kendin bir şey özetlemek zorunda değilsin.

    OPTIONS alanındaki `--append-only` kalsın. Bütün mesele bu: onsuz VAULT yine sıradan bir paylaşıma döner.

**2. TOWER üzerinde saha dışı depoyu oraya yönlendirin.** Depo adresi, tarifin yazdırdığı kalıbı izler:

    rest:http://VAULT:8000/bombvault-containers/containers

Yolun ilk parçası htpasswd kullanıcısı, ikincisi depodur. Oluşturulan kullanıcı ve parolayı hedefin REST kimlik bilgileri olarak girin ve **bağlantı testini** çalıştırın.

**3. TOWER üzerinde „Değiştirilemez” seçeneğini açın.** Kurcalama testi hemen çalışır ve *korunuyor* demelidir. Yanıtların anlamı:

| Sonuç | Ne oldu |
| --- | --- |
| **korunuyor** | VAULT silmeyi reddetti. Geçer durum yalnızca budur. |
| **KORUNMUYOR** | VAULT bir silmeyi kabul etti. `--append-only` yok ya da kaldırılmış. |
| **belirsiz** | İkisi de değil. Genelde adres restic'in kendi kullandığı adres değildir ya da kimlik bilgileri değişmiştir. Hiçbir şey kaydedilmez ve uyarı verilmez. |

**4. VAULT üzerinde neyin geldiğini izleyin.** İki makineyi eşleştirin ([Örnekleri eşleştirme](#pairing)), *Ayarlar → Genel → Alıcı* seçeneğini açın, **Alıcı** sekmesini açın ve depoyu gönderen örnek olarak TOWER ile salt okunur olarak kaydedin.

!!! warning "Konum, kapsayıcının **içindeki** bir yoldur ve ana makine bağlama noktasına göre yazılır"
    `user/appdata/rest-server/bombvault-containers/containers` girin, `/mnt/user/appdata/…` **değil**. BombVault, ana makinenin `/mnt` dizininin başka yere bağlandığı bir kapsayıcıda çalışır; mutlak ana makine yolu orada yoktur. Yapıştırırsanız BombVault artık kullanmanız gereken göreli yolu söyler.

    Kaydettiğinizde VAULT, TOWER'ın restic parolasını grup üzerinden alır; kimsenin bir anahtar yazması gerekmez.

**5. İsterseniz karşılıklı yapın.** Aynı beş adımı ters yönde yineleyin: TOWER üzerinde VAULT'un kopyasını alan bir rest-server. Böylece her makine diğeri için değiştirilemezliği dayatır ve hiçbiri diğerinin yedeklerini silemez.

## Rehberli kurtarma

Özel bir **Kurtarma** sekmesi, sıfırdan ya da yeniden oluşturulmuş bir kurulumu felaket durumundan tek bir yerde geçirir:

1. **Önce BombVault'un kendi ayarlarını geri yükler**, böylece akışın geri kalanının ihtiyaç duyduğu yedekleme yolları, site dışı hedefler ve kimlik bilgileri önceden doldurulmuş gelir (Docker soketi üzerinden bir öz yeniden başlatma ile uygulanır, böylece canlı ayar veritabanı açık bir tanıtıcı altında asla üzerine yazılmaz).
2. **BombVault'un yedeklerinizi okuyabildiğini denetler** (şifreleme anahtarı tuzağı en başta).
3. **Mevcut deponuza yönlendirmenize** izin verir (yerel ya da site dışı).
4. İçinde saklanan konteynerleri, VM'leri, dosya kümelerini ve ZFS veri kümelerini **keşfeder**.
5. **Konteynerleri ve VM'leri tek seferde geri yükler** (durdurulmuş bırakılır, böylece onları kasıtlı olarak başlatırsınız), dosya kümelerini ve ZFS öğelerini tek tek geri yüklemeniz için listeler; ZFS öğeleri kapalı olarak geri gelir. Kurtarma kitiniz bir tık ötede.

!!! tip "Planlı geçiş ve felaket karşılaştırması"
    Rehberli kurtarma, BombVault'un kendi ayarlarını bir yedekten geri yükler. Yeni bir makineye *planlı* bir taşınma için, bunun yerine yapılandırmanızı **Ayarları dışa / içe aktar** kartıyla (taşınabilir bir JSON dosyası) doğrudan taşıyabilirsiniz. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).

### Başka bir BombVault deposundan geri yükleme {#restore-from-another-bombvault-repo}

**Kurtarma** sekmesindeki ayrı bir kart, *farklı* bir BombVault örneğinin deposunu (`/mnt` altında bağlanmış bir paylaşım ya da bir uzak URL) **o örneğin `APP_KEY`'iyle**, tek seferlik, salt okunur bir oturumda açar. Orada saklanan konteynerlere, VM'lere ve dosya kümelerine göz atın, bir anlık görüntü seçip geri yükleyin; geri yüklenen nesne normal bir yerel konteyner, VM ya da dosya kümesi olur. Diğer depoya asla hiçbir şey yazılmaz ve kendi yedekleme ayarlarınız dokunulmadan kalır (oturum bellekte yaşar ve kendiliğinden sona erer). Bir konteyneri A sunucusundan B sunucusuna taşımak, depo ayarlarınızı yeniden yönlendirmek ve sonrasında geri almak anlamına gelmez. Bu kart tek seferliktir: bir oturum açar, seçtiğinizi geri yükler ve diğer örneği unutur. Bunun yerine bu makinenin başka bir örneğin anlık görüntülerini bir zamanlamaya göre kendi deposuna çektiği kalıcı bir düzen istiyorsanız, bu **Örnekler** sayfasının **Çekme** sekmesidir.

Ağı bu sunucuda olmayan bir konteyner, örneğin sıradan bir Docker ana makinesindeki Unraid `br0` ağı, satırının altında bir ağ seçimi gösterir. BombVault onu seçtiğin ağda, diğer ağlarıyla birlikte oluşturur. Sabit IP adresi ve MAC adresi eski ağa aitti ve düşer, bu yüzden bunları yeni ağ verir.

## Şifreleme anahtarı kurtarma kiti

Bu, çalışan bir BombVault olmadığında bile felaket kurtarmayı mümkün kılan parçadır.

Tek tık, **ana anahtarı**, **türetilen restic parolasını** ve **tam depo konumlarını ve komutlarını** indirir, böylece herhangi bir makinede restic CLI ile doğrudan geri yükleyebilirsiniz. Bir Kontrol Paneli hatırlatıcısı, onu saklayana kadar dırdır eder.

!!! danger "Kurtarma kitini sunucu dışında saklayın"
    Kit, yedeklerinizin şifresini çözen gizli anahtarı içerir. Onu güvenli ve sunucudan ayrı bir yerde tutun (bir parola yöneticisi, bir kasada basılı bir kopya). Hem BombVault'u hem de `APP_KEY`'i kurtarma kiti olmadan kaybederseniz, şifreli yedekleriniz kurtarılamaz.

!!! warning "En yeni anlık görüntü her zaman geri yüklenecek olan değildir"
    restic 0.17'den beri `restic snapshots` her anlık görüntünün boyutunu gösterir. Veri kaybından sonra en yeni anlık görüntü boşaltılmış olan olabilir, bu yüzden öncekilerden çok daha küçük bir anlık görüntüyü geri yüklemeyin. Fidye yazılımından sonra olağan boyutta şifrelenmiş olan olabilir. BombVault hâlâ çalışıyorsa önce **Anormallikler** sayfasına bakın: son iyi yedeği gösterir. Geri yükleme için BombVault'un anomali verilerine gerek yoktur ve saklama duraklatması yalnızca daha fazla anlık görüntü tutar.

### Kitin mühürlenmesi

Düz dışa aktarmalar için age şifrelemesini açtıysanız (Ayarlar), kit de onunla mühürlenir ve `bombvault-recovery-kit.md.age` olarak iner. İkili değil ASCII zırhlı olduğu için yine düz metindir: bir parola yöneticisine yapıştırmak ya da yazdırmak tam olarak önceki gibi çalışır, yalnızca içerik anahtarınız olmadan okunamaz.

!!! warning "age anahtarını kitin içinde saklamayın"
    Mühürlü bir kiti açmak için age **özel** anahtarınıza ihtiyacınız vardır. Onu kitin kendisine bağlı olmayan bir yerde saklayın, yoksa kurtarmanız gereken bir değil iki şey olur. Mühürleme, kit tam olarak denetlemediğiniz bir yerde saklandığında (paylaşılan bir parola yöneticisi, bulut notları, bir ofisteki çıktı) buna değer; kendi kasanızdaki bir kit zaten kasa tarafından korunur.

    Şifreleme açıkken ve yapılandırılmış kullanılabilir bir alıcı yokken indirme doğrudan reddedilir. BombVault ana anahtarı açık metin olarak vermeye asla geri dönmez.

### Kurtarma seti elinizin altında değilse

Parola hiçbir yerde saklanmaz, `APP_KEY` değerinden **hesaplanır**. Anahtar ve bir kabuk varsa onu kendiniz de üretebilirsiniz:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Bu, sabit `bombvault:restic-repo` dizgesi üzerinde HMAC-SHA256'dır; anahtar olarak onaltılık `APP_KEY` değerinin ham baytları kullanılır ve çıktı 64 küçük harfli onaltılık karakterdir. Aynı değer sette türetilmiş restic parolası olarak durur; burası, setin sizinle aynı yerde olmadığı gün içindir.

!!! warning "Alınan bir depoda GÖNDEREN örneğin anahtarını kullanın"
    Saha dışı çoğaltmayla buraya ulaşan bir depo, onu gönderen makinede **kendi** `APP_KEY` değeriyle oluşturulmuştur. Alan makinenin anahtarından türetmek, restic'in reddettiği bir parola verir; bu tam olarak bozuk bir depo gibi görünür ama değildir. Alınan bir depoda `restic check` komutunun parolayı defalarca sormasının olağan nedeni budur.

Kurtarma tanımları her deponun **içinde** yer aldığı için (`<repo>/def`, `<repo>/vm-def`), kopyalanan bir depo klasörü tamamen bağımsızdır, böylece kit ile birlikte depo, çıplak makine geri yüklemesinin ihtiyaç duyduğu her şeydir.

## Bir veritabanı dökümünü geri alma {#database-dumps}

Bir veritabanı dökümü, konteyner deposunda kendi başına bir geri yükleme noktasıdır; `dbdump:<container>` etiketini taşır ve tek bir dosya içerir, `/dbdump/<container>.sql`. BombVault bunları **Yedekler** altında listeler, indirir ve içe aktarır; aşağıdakiler aynı adımların yalnızca restic ile hali, BombVault'un yanınızda olmadığı gün için.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Her dökümdeki `dbversion:` ve `dbname:` etiketleri, dökümün hangi sunucu sürümünden geldiğini ve hangi veritabanlarını taşıdığını söyler. Tam bir dosya `-- PostgreSQL database cluster dump complete` ya da `-- Dump completed` ile biter.

Dökümü aynı ya da daha yeni sürümlü (PostgreSQL) veya aynı ana sürümlü (MySQL ve MariaDB) bir konteynere aktarın; konteyner, kendini kurması için boş bir veri klasörüyle bir kez başlatılmış olmalı. Ana makinede veritabanı istemcisi gerekmez, konteynerde var:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Tam bir dökümden tek bir veritabanı için MySQL ve MariaDB, istemci komutunda `--one-database <name>` kabul eder. PostgreSQL dökümünde her veritabanı için bir bölüm vardır ve her biri `\connect <name>` satırıyla başlar: o bölümü kendi dosyasına kopyalayın ve veritabanını oluşturduktan sonra `-d <name>` ile aktarın.

!!! warning "Root ile alınan bir döküm sunucunun kullanıcılarını da taşır"
    Root ile alınan tam bir MySQL ya da MariaDB dökümü `mysql` sistem veritabanını içerir; içe aktarmak yeni sunucunun hesaplarını, root parolası dahil, dökümdekilerle değiştirir. PostgreSQL'de, konteynerin kendi oluşturduğu kullanıcı için gelen `role ... already exists` beklenen ve zararsız bir iletidir.
