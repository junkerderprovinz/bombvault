# Site dışı ve kurtarma

Yerel yedekler sizi kaybolmuş bir konteynerden ya da hatalı bir güncellemeden korur. Site dışı çoğaltma ve test edilmiş bir kurtarma kiti sizi tüm makineden, fidye yazılımından ya da bir yangından korur. Bu sayfa site dışına çoğaltmayı, o kopyayı kurcalamaya dayanıklı yapmayı, geri yükleyebildiğinizi kanıtlamayı ve BombVault'un kendisi kaybolduğunda kurtarmayı kapsar.

## Site dışı çoğaltma

Hızlı yerel yedeği tutun ve onu bir ya da daha fazla başka konuma kopyalayın. Bir etki alanının kopyalandığı konumları **Ayarlar, Depolama** altındaki **Alanlar** kartında, konum başına bir çiple seçersiniz (bkz. [Depolama konumları](storage-places.md#domains)). BombVault yeni anlık görüntüleri oraya en iyi çaba temelinde `restic copy` ile kopyalar, böylece başarısız bir kopya asla yerel yedeklemeyi bozmaz. Bir etki alanının depolandığı konumun yerel olması gerekmez; bkz. [Uzak bir konumda depolanan etki alanı](#remote-primary-repositories).

- **Etki alanı başına birden çok kopyalama konumu.** Bir etki alanı aynı anda birkaç konuma kopyalanabilir, örneğin bir arkadaşınızın evindeki bir rest-server'a ve bir B2 bucket'ına. Saklama, depolama sınıfı, append-only, sınırlar ve büyüme bütçesi konuma aittir, böylece her kopya vardığı konumun kurallarına uyar.
- **Etki alanı başına kopyalama zamanlaması** (Ayarlar, Zamanlamalar'da diğer her zamanlamanın yanında düzenlenir): her yerel yedeklemeden sonra kopyalamak için boş bırakın ya da yedeklediğinizden daha seyrek kopyalamak için bir sıklık ayarlayın (örneğin `weekly Sun 03:00`). Etki alanının satırındaki **Şimdi kopyala** kopyalamayı istek üzerine çalıştırır.
- **Konum başına saklama.** Her konum kendi kurallarını tutar, böylece site dışı bir konum kopyaları bir arşiv olarak daha uzun tutabilir. Her kuralı sıfır olan bir konum hiçbir şeyi kırpmaz.
- **Bant genişliği sınırları** konum başına restic yükleme ve indirme hızını sınırlar, böylece kopyalama WAN'ınızı doyurmaz.
- Bir **çoğaltma göstergesi**, çalışırken hangi etki alanının kopyalandığını gösterir (kendi sayfasında ve Kontrol Paneli'nde). Bu bir etkin göstergedir, bir yüzde çubuğu değil, çünkü `restic copy` makine tarafından okunabilir bir ilerleme sunmaz.

!!! note "Herhangi bir yerden geri yükleyin"
    Her konteyner, VM, dosya kümesi, flash ve uygulama yapılandırması, yedeklerini bir yedeğin bulunduğu tüm yerler boyunca tek bir zaman çizelgesi olarak listeler. B2'ye kopyalanan bir yedek yalnızca bir kez görünür, onu tutan her yerle işaretlenmiş olarak. Bir geri yükleme, ulaşabildiği ilk yeri alır, ögenin yazıldığı depoyla başlayarak, ve her satır için başka bir yer seçebilirsin. Site dışı yerler yalnızca onları açtığında okunur. Bir yerde silme, önce diğerlerini kontrol eder ve bunun son kopya olup olmadığını söyler.

## Öge başına yerleşim {#placement}

Her konteyner, VM ve dosya kümesi kartında üç segmentli bir **Yerleşim** satırı vardır:

- **Yerel**, ögeyi **Depolama konumu** altında gösterilen depoya yazar ve hiçbir yere kopyalamaz. Zaten bir ikinci kopyası olan veriler için kullan, örneğin bir NAS üzerinde yaşayan bir paylaşım.
- **Yerel + site dışı**, oraya da yazar ve **Kopyalama hedefi** altında işaretlenen hedeflere kopyalar, etki alanının her site dışı hedefi için bir çip. Bir çipin işaretini kaldır, o hedef bu ögeden artık yeni bir şey almaz.
- **Yalnızca site dışı**, ögeyi doğrudan **Gönderim hedefi** altındaki konuma yazar; bu, etki alanının ana konumu dışındaki herhangi bir konum olabilir. Etki alanı o konuma zaten kopyalanıyorsa öge, kopyaların yanında doğrudan bir depo alır; aksi halde BombVault orada etki alanı için bir depo oluşturur.

Konum, ögenin ilk yedeklemesinden itibaren sabittir, çünkü BombVault yedekleri depolar arasında asla taşımaz. Kopyalar ise her zaman değişebilir. Bir ögeyi artık almayan bir hedef, sahip olduğu kopyaları tutar ve etki alanının bir sonraki site dışı çalıştırmasında kendi saklama kuralına göre kırpar; karttaki **B2 içinde sil**, onları hemen kaldırır. Bu kopyalardan bazıları başka hiçbir yerde yoksa, onay ekranı bunları tarihe göre listeler ve ögenin adını sorar. Yalnızca ekleme hedeflerden hiçbir şey silinemez.

Satırın altında kart, ögenin nereye gittiğini ve gerçekte nerede olduğunu söyler: kaç sitenin onu tuttuğunu, her hedefin en son ne zaman görüldüğünü ve 3-2-1'in karşılanıp karşılanmadığını. Bir site, orijinal verinin bulunduğu sunucu ve başka bir yerde duran her konumdur (bkz. [Bina dışında](#off-the-premises-mark)). BombVault kopyaları ve siteleri kontrol eder; 3-2-1'in "iki ortam" kısmını kontrol etmez.

### Etki alanı başına varsayılanlar

Ayarlar, Depolama altındaki **Alanlar** kartında etki alanı başına bir satır vardır. **Kopyalama hedefleri**, kendi seçimi olmayan her ögeye ve Compose yığınlarının proje klasörlerine hemen uygulanır. Etki alanının yedekleri olduğunda **Depolama konumu** yeni bir ögeye ilk yedeklemesinde uygulanır ve onu değiştirmek hiçbir yedeği taşımaz. Kaydetmeden önce satır, öge kazanan ya da kaybeden her konumu ve bunun kaç anlık görüntü anlamına geldiğini adlandırır; soru ayrıca **Yedeksiz ögelere uygula** anahtarını taşır, bu anahtar henüz yedeği olmayan her ögeyi de yeni varsayılana alır. **İstisnalar**, kendi seçimi olan ögeleri listeler.

Yeni bir konumu **Kopyalama hedefleri** altında işaretlemek, o konumun Yerel'e ayarlanmamış her ögeyi almasını sağlar. Onay, kaç öge olduğunu ve, biliniyorsa, bunun ne kadar geçmiş anlamına geldiğini söyler.

### Doğrudan depolar

Etki alanının zaten kopyalandığı bir konumu Yalnızca site dışı altında seçmek bir kez sorar, sonra kopyaların yanında doğrudan bir depo oluşturur, örneğin `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, ve ögeyi ona yöneltir. Konumu olmayan bir kopyalama hedefi için bu seçim, önerilen bir adres ve hiçbir şey oluşturmayan bir bağlantı testiyle bir iletişim kutusu açar; **Oluştur ve kullan** depoyu oluşturur. Doğrudan bir depo, konumun anahtarını, depolama sınıfını, sınırlarını, append-only ayarını ve saklamasını alır ve onlarla birlikte değişir. Konum için yeni bir anahtar onu açamadığında, doğrudan depo sahip olduğu anahtarı tutar ve kayıt bunu belirtir. Anlık görüntüleri `bv:direct` etiketini taşır ve diğer her budama bunları tutar, böylece konumuyla bağlantısını kaybetmiş bir doğrudan depo yerel kurallara göre asla yaşlanmaz. Tek bir klasörle sınırlı bir B2 anahtarı yalnızca etki alanının klasörünü değil, konumun adresini de kapsamalıdır; aksi halde yanındaki klasöre erişilemez.

### Bina dışında {#off-the-premises-mark}

Bir kopya, yalnızca konumu başka bir yerdeyse ayrı bir site sayılır. Bir bulut konumu her zaman sayılır, bu Unraid'deki bir klasör ise asla sayılmaz; bir NAS, bir rest-server ya da bir SFTP sunucusu için konumun ayrıntılarında **Cihaz nerede?** sorusu **Burada, bu binada** ya da **Başka bir yerde** ile yanıtlanır. Yanıt, kartlarda ve Kontrol Paneli'nde yalnızca siteleri ve 3-2-1'i sayar. Hiçbir kopyayı değiştirmez.

### Bir yeniden kurulumdan sonra

Kopyalama seçimleri BombVault'un kendi ayarlarında yaşar. Geri yüklenmiş bir `/config` olmadan Discover üzerinden bir yeniden kurulumdan sonra bunlar kaybolur ve her şeyi kopyalamak, dışarıda bıraktığın ögeleri tekrar B2'ye gönderir. Bu yüzden yeniden kurulan her etki alanının site dışı çoğaltması duraklar. Kontrol Paneli bunu kehribar renginde gösterir ve etki alanının Alanlar kartındaki satırı, bir sonraki çalıştırmanın neyi kopyalayacağının ve burada karşılığı olmayan yedeklerdeki adların bir önizlemesiyle **Varsayılanı onayla** sunar; bunları orada dışarıda bırakabilirsin. Yalnızca onay duraklamayı bitirir; bir ayar dosyası içe aktarmak kuralları ve varsayılanları geri getirir ama duraklamayı bitirmez.

## Uzak bir konumda depolanan etki alanı {#remote-primary-repositories}

Bir etki alanının yerel olarak depolanması gerekmez. Yedekleme yolunda henüz yedek yokken Alanlar kartında **Depolama konumu** altında uzak bir konum seçin; etki alanı yerel bir kopya ve kopyalama adımı olmadan doğrudan oraya yedekler. Uzak depo o zaman, etki alanı başka bir konuma da kopyalanmadıkça tek kopyadır. Her uzak konum aynı güvencelerle gelir:

- **Bir bağlantı testi**, herhangi bir şey yazılmadan önce.
- **Bant genişliği sınırları**, yedeklemenin kendisi için; bir kopyanın kullandığı `--limit-upload` ve `--limit-download` seçeneklerinin aynısı.
- **Append-only koruması**, aynı etkin kurcalama testiyle doğrulanır. Açıkken BombVault depoyu asla budamaz, çünkü bu makinedeki kimlik bilgileri yedeğin tek kopyasını silebilecek durumda olmamalıdır.
- **Bir büyüme bütçesi**, Depolama kartının izlediği boyut eğiliminin aynısından örneklenir.

Uzak bir konumda depolanan bir etki alanı da, yerel bir etki alanı gibi, kopyalarının kaynağıdır; bkz. [Farklı kimlik bilgilerine sahip konumlar arasında kopyalar](storage-places.md#different-credentials).

!!! note "Kimlik bilgileri konuma aittir"
    Uzak bir konum kendi kimlik bilgilerini tutar. Ortak bulut kimlik bilgileriyle kurulmuş bir konum, erişimi konumun ayrıntılarında değiştirilene kadar onları kullanmaya devam eder.

### Ana makinede bağlama olmadan SMB ve WebDAV {#smb-webdav}

**Konum ekle** penceresinin rclone formunda bir Windows veya Samba paylaşımı ve bir WebDAV sunucusu (Nextcloud, ownCloud, SharePoint ya da başka herhangi biri) için bir form bulunur. Kısa bir ad, ana makineyi ve paylaşımı (SMB) ya da URL'yi ve sunucu türünü (WebDAV), kullanıcıyı ve parolayı girin; BombVault rclone bölümünü sizin için yazar. rclone parolayı kaydedilmeden önce kendisi gizler; zaten var olan bir adla hedef eklemek, ikinci bir bölüm eklemek yerine o bölümü değiştirir.

Yeni uzak tanım ardından formun uzak tanımlar listesinde görünür ve onu orada konum için seçersiniz. Paylaşım adın bir parçası değil, yolun ilk bölümüdür.

Bu, paylaşımı Unraid'e bağlamaktan daha iyi bir yoldur: restic bir depoyu bağlanmış bir CIFS paylaşımında tutmayı önermez ve burada hiçbir şey bağlanmaz. NFS formda yoktur, çünkü ne restic'in ne de rclone'un bir NFS arka ucu vardır; NFS için dışa aktarımı ana makineye bağlayın ve **Başka bir paylaşım** ile konum olarak ekleyin.

## Değiştirilemez (yalnızca ekleme) site dışı

Fidye yazılımı ya da ele geçirilmiş bir host yedeklerinizi silemesin veya yeniden yazamasın diye bir site dışı depoyu yalnızca ekleme olarak işaretleyin. Karşı taraf (`--append-only` modunda çalışan bir `restic/rest-server`) bunu **uygular**. BombVault yalnızca bunu **doğrular** ve asla yalnızca bir yapılandırma iddiası üzerine yeşil göstermez.

**Konum ekle** penceresi, bu BombVault için tek kullanıcılı, append-only modunda çalışan bir rest-server için yapıştırmaya hazır bir tarif taşır. **Append-only** açık bir rest-server konumunda, konumun ayrıntılarındaki **Append-only'yi test et**, konumdaki her etki alanı yolu, açık kopya ve depo için kurcalama testini çalıştırır ve konum için tek bir yanıt verir; böylece yalnızca ekleme site dışına yapılandırmaları elle düzenlemeden ulaşılabilir.

!!! note "`/locks/` altında başarılı bir silme beklenen davranıştır"
    Append-only, artık hiçbir şeyin silinemeyeceği anlamına gelmez. restic kendi kilitlerini alıp bırakmak zorundadır, bu yüzden `/locks/` bilerek yazılabilir ve silinebilir kalır. Anlık görüntüler ve arkasındaki veriler, yani fidye yazılımının hedefi tam olarak budur, kaldırılamaz. Uzak tarafı kendin denersen, `/locks/` altında başarılı olan bir silme doğru davranıştır ve korumada bir delik değildir.

!!! warning "Değiştirilemez depolar bu makineden asla budanmaz"
    Değiştirilemez bir site dışı, eski anlık görüntüleri kasıtlı olarak asla budamaz. Depo boyutu kontrolden çıkmadan önce uyarılmanız için ona bir **büyüme bütçesi alarmı** ayarlayın.

## Kurcalama testi

BombVault, yalnızca ekleme garantisini, site dışı depoya karşı var olmayan bir nesneyi hedefleyen gerçek bir silme girişiminde bulunarak periyodik olarak kanıtlar:

- **Reddedildi**, korunuyor demektir.
- **Kabul edildi**, korunmuyor demektir.
- **Sonuçsuz** bir sonuç (sunucuya ulaşılamıyor, kimlik doğrulama hatası) saklanan kararı asla değiştirmez.

Gerçek bir korunuyordan-korunmuyora dönüş tek bir uyarı tetikler.

Bir konumda **Append-only'yi test et**, oradaki her etki alanı yolunu, açık kopyayı ve depoyu kendi kimlik bilgileriyle yoklar ve sonuçları tek bir yanıtta birleştirir: silmeyi kabul eden tek bir depo bile tüm konumu *silme kabul edildi* yapar.

## DR tatbikatları

BombVault, yedeklerinizin yalnızca mevcut değil, gerçekten geri yüklenebilir olduğuna dair iki düzeyde kanıt sunar.

- **Geri yükleme doğrulama tatbikatları (yerel).** BombVault periyodik olarak `restic check --read-data-subset` çalıştırır (sınırlı, asla diski dolduran tam bir geri yükleme değil) ve etki alanı başına bir *son doğrulanan geri yüklenebilir* rozeti gösterir. Sıklık Ayarlar, Zamanlamalar'da; rozet Ayarlar, Bütünlük'te yer alır.
- **DR tatbikatları (site dışı).** BombVault gerçek bir hedefi site dışı depodan tek kullanımlık bir korumalı alana geri yükler, onu dosya-dosya ve bayt-bayt doğrular, ardından temizler. Bu, deponun yalnızca yanıt verdiğini değil, site dışından kurtarabildiğinizi kanıtlar. Yalnızca başka bir yerdeki konumlarda tatbikat yapılır, çünkü aynı binadaki bir kopya binanın kaybı hakkında hiçbir şey kanıtlamaz. Bunlardan birkaçına kopyalanan bir etki alanı, her zamanlanmış çalıştırmada sırayla bunlardan biriyle tatbikat yapar ve Kontrol Paneli son tatbikatın konumunu gösterir.

Kontrol Paneli'ndeki **fidye yazılımı koruması karnesi** bunu etki alanı başına yeşil / sarı / kırmızı bir duruşa, yaş damgalı bir kontrol listesiyle (site dışı yapılandırıldı, yalnızca ekleme doğrulandı, çoğaltma güncel, geri yükleme tatbikatı geçti, şifreleme açık, budama stratejisi ayarlandı) toplar. Her kırmızı satır düzeltmeye derin bağlantı verir ve kart yalnızca doğrulanmış gerçekler üzerine yeşile döner.

## Alıcı kontrol paneli (alan taraf)

![Alıcı taraf, salt okunur izlenir, bütünlük kontrolü bu makinede çalıştırılır.](assets/screenshots/receiver.png)

*Alıcı taraf, salt okunur izlenir, bütünlük kontrolü bu makinede çalıştırılır.*

Yukarıdaki her şey *gönderen* taraftır. Başka bir BombVault'tan değiştirilemez site dışı kopyalar **alan** makinede, Alıcı kontrol paneli size o depoların alan donanımda bağımsız, salt okunur izlemesini verir, böylece karşı uçtaki sessiz bir hata fark edilmeden kalmaz.

Bir **Alıcı** sekmesini ortaya çıkarmak için Ayarlar'da **Alıcı** geçişini açın. Varsayılan olarak kapalıdır; onu yalnızca gerçekten değiştirilemez site dışı yedekler alan bir makinede etkinleştirin. Ardından şunları elde etmek için alınan bir depoyu (salt okunur, gönderen örneğin anahtarıyla açılmış) kaydedin:

- **Kaynağa göre gruplanmış bir anlık görüntü envanteri**, böylece hangi konteynerlerin, VM'lerin ve dosya kümelerinin geldiğini tam olarak görebilirsiniz.
- Kaynak başına **Son alınan**, böylece her birinin ne kadar taze olduğunu bilirsiniz.
- Alan donanımda çalışan **bağımsız bir `restic check`**, böylece bütünlük yalnızca göndericide değil, verinin gerçekten bulunduğu yerde doğrulanır.
- **Bir ölü adam anahtarı:** bir kaynak, ayarladığınız bir pencere içinde göndermeyi durdurduğunda bir uyarı.
- **Bütünlük uyarıları:** alan tarafta bir denetim başarısız olduğunda bir uyarı.

Alıcı kesinlikle salt okunurdur. Alınan depoya asla yazmaz, böylece göndericinin dayandığı yalnızca ekleme garantisini asla bozamaz.

## Baştan sona örnek: iki Unraid makinesi

Yukarıda parçalar anlatılıyor. Burada gerçek değerlerle tek bir eksiksiz kurulum var, çünkü parçaları bir kez birleştirilmiş halde görmek işi çok kolaylaştırır.

İki makine: **TOWER** kapsayıcıları çalıştırır ve yedekleri gönderir, **VAULT** onları alır ve değiştirilemezliği dayatır. Kendi adlarınızı, adreslerinizi ve paylaşım yollarınızı koyun.

**1. VAULT üzerinde append-only sunucusunu kurun.** TOWER üzerindeki BombVault'ta *Ayarlar → Depolama* sekmesini açın, **Konum ekle**'ye tıklayın, **rest-server** seçin ve **Tarifi göster**'e tıklayın. **Unraid şablonu** bloğunu kopyalayın, VAULT üzerinde `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` olarak kaydedin, sonra *Docker → Add Container* deyip şablon listesinden **rest-server** seçin. Başlatmadan önce gösterilen `htpasswd` satırını VAULT üzerinde `/mnt/user/appdata/rest-server/.htpasswd` dosyasına yazın. Parola bir kez gösterilir ve hiç saklanmaz; tarif onu ve kullanıcıyı TOWER'daki forma zaten yazmıştır, bu yüzden o pencereyi açık bırakın. `htpasswd` satırı aynı parolayı sizin için bcrypt ile özetlenmiş olarak taşır, yani kendiniz bir şey özetlemek zorunda değilsiniz.

    OPTIONS alanındaki `--append-only` kalsın. Onsuz VAULT yine sıradan bir paylaşımdan ibarettir.

**2. TOWER üzerinde konumu ekleyin.** Tarifin doldurduğu kullanıcı ve parolanın yanına VAULT'un adresini, `http://VAULT:8000`, girin, sonra **Bağlantıyı test et**'e tıklayın. BombVault adresi bunlardan oluşturur:

    rest:http://VAULT:8000/tower

Yolun ilk parçası htpasswd kullanıcısıdır, burada `tower`; her etki alanı klasörünü onun altında alır, örneğin `rest:http://VAULT:8000/tower/container`. **Cihaz nerede?** sorusunu **Başka bir yerde** ile yanıtlayın, **Ekle**'ye tıklayın ve oraya gitmesi gereken etki alanları için konumu **Kopyalama hedefleri** altında işaretleyin.

**3. TOWER üzerinde Append-only'yi açın.** Bunu konumun ayrıntılarında **Koruma** altında yapın, sonra **Append-only'yi test et**'e tıklayın. Test, konumdaki her etki alanı yolunu, kopyayı ve depoyu yoklar ve konum için tek bir yanıt verir; bu yanıt *silme reddedildi* olmalıdır. Yanıtların anlamı:

| Sonuç | Ne oldu |
| --- | --- |
| **silme reddedildi** | VAULT silmeyi reddetti. Geçer durum yalnızca budur. |
| **silme kabul edildi** | VAULT bir silmeyi kabul etti. `--append-only` yok ya da kaldırılmış. |
| sonuç yerine bir ileti | Test çalışamadı. Genelde adres restic'in kendi kullandığı adres değildir ya da kimlik bilgileri değişmiştir. Hiçbir şey kaydedilmez ve uyarı verilmez. |

**4. VAULT üzerinde neyin geldiğini izleyin.** *Ayarlar → Alıcı* seçeneğini açın, **Alıcı** sekmesini açın ve depoyu salt okunur olarak kaydedin.

!!! warning "Konum, kapsayıcının **içindeki** bir yoldur ve ana makine bağlama noktasına göre yazılır"
    `user/appdata/rest-server/tower/container` girin, `/mnt/user/appdata/…` **değil**. BombVault, ana makinenin `/mnt` dizininin başka yere bağlandığı bir kapsayıcıda çalışır; mutlak ana makine yolu orada yoktur. Yapıştırırsanız BombVault kullanmanız gereken göreli yolu söyler.

    **Gönderen APP_KEY**, VAULT'un değil TOWER'ın anahtarıdır. TOWER üzerinde *Ayarlar → Sistem* altında bulunur.

**5. İsterseniz karşılıklı yapın.** Aynı beş adımı ters yönde yineleyin: TOWER üzerinde VAULT'un kopyasını alan bir rest-server. Böylece her makine diğeri için değiştirilemezliği dayatır ve hiçbiri diğerinin yedeklerini silemez.

## Rehberli kurtarma

Özel bir **Kurtarma** sekmesi, sıfırdan ya da yeniden oluşturulmuş bir kurulumu felaket durumundan tek bir yerde geçirir:

1. **BombVault'un yedeklerinizi okuyabildiğini denetler** (şifreleme anahtarı tuzağı en başta).
2. **BombVault'un kendi ayarlarını geri yükler**, böylece akışın geri kalanının ihtiyaç duyduğu yedekleme yolları, site dışı hedefler ve kimlik bilgileri önceden doldurulmuş gelir. Ayar yedeğini, Öz yedek satırının **Depolama konumu** altında belirttiği konumdan ya da Öz yedeğin **Kopyalama hedefleri** altındaki kopyasından okur ve o konumu adresiyle gösterir; başka bir konumdan okumak için önce 3. adımdaki Öz yedek satırını değiştirin. Geri yükleme Docker soketi üzerinden bir öz yeniden başlatma ile uygulanır, böylece canlı ayar veritabanı açık bir tanıtıcı altında asla üzerine yazılmaz.
3. **Mevcut yedeklerinizi bağlar**, bunu **Alanlar** kartının satırları üzerinden yapar: her etki alanının satırında, yedeklerinin bulunduğu konumu **Depolama konumu** altında, kopyalarını tutan konumları da **Kopyalama hedefleri** altında seçin. Henüz hiçbir satırın sunmadığı bir konum, örneğin bir paylaşım, bir sunucu ya da bir bulut bucket'ı, Ayarlar, Depolama'dakiyle aynı pencere olan **Konum ekle** ile bağlanır. Ardından **Bağlan ve önizle** yedeklerin okunabildiğini denetler.
4. İçinde saklanan konteynerleri, VM'leri, dosya kümelerini ve ZFS veri kümelerini **keşfeder**.
5. **Konteynerleri ve VM'leri tek seferde geri yükler** (durdurulmuş bırakılır, böylece onları kasıtlı olarak başlatırsınız), dosya kümelerini ve ZFS öğelerini tek tek geri yüklemeniz için listeler; ZFS öğeleri kapalı olarak geri gelir. Kurtarma kitiniz bir tık ötede.

!!! note "Site dışı kopyalar bir yeniden kurulumdan sonra bekler"
    4. adım eski ayarlar olmadan girişleri yeniden kurduğunda, o etki alanlarının site dışı çoğaltması, yerleşim varsayılanı onaylanana kadar duraklar. Bkz. [Öge başına yerleşim](#placement).

!!! tip "Planlı geçiş ve felaket karşılaştırması"
    Rehberli kurtarma, BombVault'un kendi ayarlarını bir yedekten geri yükler. Yeni bir makineye *planlı* bir taşınma için, bunun yerine yapılandırmanızı **Ayarları dışa ve içe aktar** kartıyla (taşınabilir bir JSON dosyası) doğrudan taşıyabilirsiniz. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).

### Başka bir BombVault deposundan geri yükleme

**Kurtarma** sekmesindeki ayrı bir kart, *farklı* bir BombVault örneğinin deposunu (`/mnt` altında bağlanmış bir paylaşım ya da bir uzak URL) **o örneğin `APP_KEY`'iyle**, tek seferlik, salt okunur bir oturumda açar. Orada saklanan konteynerlere, VM'lere ve dosya kümelerine göz atın, bir anlık görüntü seçip geri yükleyin; geri yüklenen nesne normal bir yerel konteyner, VM ya da dosya kümesi olur. Diğer depoya asla hiçbir şey yazılmaz ve kendi yedekleme ayarlarınız dokunulmadan kalır (oturum bellekte yaşar ve kendiliğinden sona erer). Bir konteyneri A sunucusundan B sunucusuna taşımak artık depo ayarlarınızı yeniden yönlendirmek ve sonrasında geri almak anlamına gelmez. Canlı sunucudan sunucuya federasyon açıkça kapsam dışıdır; bu kasıtlı bir tek atışlık çekmedir.

## Şifreleme anahtarı kurtarma kiti

Bu, çalışan bir BombVault olmadığında bile felaket kurtarmayı mümkün kılan parçadır.

Tek tık, **ana anahtarı**, **türetilen restic parolasını** ve **tam depo konumlarını ve komutlarını** indirir, böylece herhangi bir makinede restic CLI ile doğrudan geri yükleyebilirsiniz. Bir Kontrol Paneli hatırlatıcısı, onu saklayana kadar dırdır eder.

!!! danger "Kurtarma kitini sunucu dışında saklayın"
    Kit, yedeklerinizin şifresini çözen gizli anahtarı içerir. Onu güvenli ve sunucudan ayrı bir yerde tutun (bir parola yöneticisi, bir kasada basılı bir kopya). Hem BombVault'u hem de `APP_KEY`'i kurtarma kiti olmadan kaybederseniz, şifreli yedekleriniz kurtarılamaz.

!!! warning "En yeni anlık görüntü her zaman geri yüklenecek olan değildir"
    restic 0.17'den beri `restic snapshots` her anlık görüntünün boyutunu gösterir. Veri kaybından sonra en yeni anlık görüntü boşaltılmış olan olabilir, bu yüzden öncekilerden çok daha küçük bir anlık görüntüyü geri yüklemeyin. Fidye yazılımından sonra olağan boyutta şifrelenmiş olan olabilir. BombVault hâlâ çalışıyorsa önce **Anormallikler** sayfasına bakın: son iyi yedeği gösterir. Geri yükleme için BombVault'un anomali verilerine gerek yoktur ve saklama duraklatması yalnızca daha fazla anlık görüntü tutar.

### Kitin mühürlenmesi

Düz dışa aktarmalar için age şifrelemesini açtıysanız (Ayarlar), kurtarma kiti de onunla mühürlenir ve `bombvault-recovery-kit.md.age` olarak iner. İkili değil ASCII armor biçimindedir, bu yüzden hâlâ düz metindir: bir parola yöneticisine yapıştırmak ya da yazdırmak tam önceki gibi çalışır, yalnızca içerik anahtarınız olmadan okunamaz.

!!! warning "age anahtarını kitin içinde saklamayın"
    Mühürlü bir kiti açmak için age **özel** anahtarınız gerekir. Onu kitin kendisine bağlı olmayan bir yerde tutun, yoksa kurtarmanız gereken bir yerine iki şey olur. Mühürleme, kit tam olarak denetleyemediğiniz bir yerde saklandığında işe yarar (paylaşılan bir parola yöneticisi, buluttaki notlar, bir ofisteki çıktı); kendi kasanızdaki bir kiti zaten kasa korur.

    Şifreleme açıkken kullanılabilir bir alıcı ayarlanmamışsa indirme doğrudan reddedilir. BombVault asla bunun yerine ana anahtarı düz metin olarak vermeye başvurmaz.

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
