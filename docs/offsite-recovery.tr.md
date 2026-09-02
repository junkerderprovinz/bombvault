# Site dışı ve kurtarma

Yerel yedekler sizi kaybolmuş bir konteynerden ya da hatalı bir güncellemeden korur. Site dışı çoğaltma ve test edilmiş bir kurtarma kiti sizi tüm makineden, fidye yazılımından ya da bir yangından korur. Bu sayfa site dışına çoğaltmayı, o kopyayı kurcalamaya dayanıklı yapmayı, geri yükleyebildiğinizi kanıtlamayı ve BombVault'un kendisi kaybolduğunda kurtarmayı kapsar.

## Site dışı çoğaltma

Hızlı yerel yedeği tutun ve bir veya daha fazla site dışı kopya ekleyin. **Ayarlar, Site dışı** sekmesinde etki alanı başına bir depo ayarlayın. BombVault yeni anlık görüntüleri oraya en iyi çaba temelinde `restic copy` ile çoğaltır, böylece bir site dışı aksaklık asla yerel yedeklemeyi bozmaz. Yerel depo birincil kalır.

- **Etki alanı başına birden fazla site dışı hedef.** Her etki alanı (konteynerler, VM'ler, flash, config ve dosya kümeleri) yalnızca birine değil, aynı anda birkaç site dışı hedefe çoğaltabilir, böylece örneğin bir arkadaşınızın makinesinde bir rest-server ve paralel olarak bir S3 kovası tutabilirsiniz. Ayarlar, Site dışı'nda her biri kendi deposu, S3 depolama sınıfı, yalnızca ekleme bayrağı, saklama ve büyüme bütçesiyle ek hedefler ekleyin. Mevcut tek bir site dışı kurulum, ilk hedef olarak dokunulmadan taşınır ve bir etki alanının her hedefi o etki alanının site dışı zamanlamasında çoğaltılır.
- **Etki alanı başına site dışı zamanlama** (Ayarlar, Zamanlamalar'da diğer her zamanlamanın yanında düzenlenir): her yerel yedeklemeden sonra çoğaltmak için boş bırakın ya da yerelde yedeklediğinizden daha seyrek site dışına göndermek için bir sıklık ayarlayın (örneğin `weekly Sun 03:00`). Bir **Şimdi çoğalt** düğmesi istek üzerine çalışmaları kapsar.
- **Site dışı saklama** Ayarlar, Site dışı'nda yer alır, böylece site dışı kopyaları bir arşiv olarak daha uzun tutabilirsiniz. Site dışı anlık görüntüleri asla otomatik kırpmamak için ilkeyi tümü sıfır bırakın.
- **Bant genişliği sınırları** (Ayarlar, Site dışı) restic yükleme/indirme hızını sınırlar, böylece çoğaltma WAN'ınızı doyurmaz.
- Bir **çoğaltma göstergesi**, çalışırken hangi etki alanının çoğaltıldığını gösterir (kendi sayfasında ve Kontrol Paneli'nde). Bu bir etkin göstergedir, bir yüzde çubuğu değil, çünkü `restic copy` makine tarafından okunabilir bir ilerleme sunmaz.

!!! note "Doğrudan site dışından geri yükleyin"
    Her yedekleme tarayıcısında bir **Yerel / Site dışı** anahtarı vardır, böylece bir yerel depo kaybolur ya da bozulursa doğrudan site dışı kopyadan listeleyip geri yükleyebilirsiniz. Silme kaynağa özeldir: bir yedeği kaldırmak yalnızca görüntülediğiniz kopyayı etkiler.

## Uzak birincil depolar {#remote-primary-repositories}

Bir alanın yedekleme yolu (Ayarlar, Yollar ve depolama) yerel bir klasörle sınırlı değildir: doğrudan bir restic uzak deposuna yöneltin (`s3:...`, `rest:http://host:8000/depo`, `b2:...`, `sftp:kullanici@host:/depo`, `rclone:remote:bucket/yol`), BombVault ayrı bir yerel kopya ve çoğaltma adımı olmadan doğrudan oraya yedekler. Bu, yukarıdaki saha dışı çoğaltmadan gerçekten farklı bir biçimdir: orada yerel depo birincildir ve saha dışı depo onun elden geldiğince tutulan arşividir; burada uzak depo birincilin **kendisidir** ve o alan için ayrıca bir saha dışı çoğaltma (ya da ikinci bir uzak depo) kurmadığınız sürece tek kopyadır.

Beş yol alanının her birinin (Kapsayıcılar, Sanal makineler, Flash, Yapılandırma, Dosyalar) hemen yanında bir **Yerel / Uzak** anahtarı vardır:

- **Yerel** alışılmış klasör tarayıcısını gösterir.
- **Uzak** onu yalın bir URL alanıyla değiştirir; yanına da, saha dışı hedeflerin kullandığı bağlantı testi ve kimlik bilgileri penceresinin aynısını bu birincil depo için ayarlanmış olarak açan bir düğme koyar. Oradan şunları elde edersiniz:
    - **Bir bağlantı testi**, gerçek yola karşı, ona güvenmeden önce.
    - **Bant genişliği sınırları** (gönderme ve alma), böylece uzak bir birincil depoya yapılan zamanlanmış yedekleme WAN hattınızı doldurmaz: saha dışı çoğaltmanın kullandığı `--limit-upload` ve `--limit-download` restic seçeneklerinin aynısı, bu kez yedeklemenin kendisine uygulanır.
    - **Yalnızca-ekleme (değiştirilemezlik) koruması**, saha dışı hedeflerin aldığı etkin kurcalama testinin aynısıyla doğrulanır (karşı tarafa gerçek bir DELETE denemesi). Açıkken BombVault deponun kendisini budamayı reddeder: arkasında ayrı bir yerel kopya bulunmadığına göre, bu makinedeki kimlik bilgileri yedeğin tek kopyasını silebilecek durumda olmamalıdır.
    - **Bir büyüme bütçesi uyarısı**, Depolama kartının zaten izlediği depo boyutu eğiliminin aynısından türetilir.

Bunların hiçbiri zorunlu değildir: elle yazılmış, kayıtlı güvenlik ayarı olmayan bir uzak yol tam da eskisi gibi yedekler (sınırsız bant genişliği, budanabilir, bütçe uyarısı yok). Güvenlik penceresi, saha dışı bir kopyanın aldığı korumaların aynısını, salt bunun için ayrı bir saha dışı hedef kurmak zorunda kalmadan istediğiniz durum içindir.

!!! note "Bulut ve REST kimlik bilgileri ortaktır"
    Uzak bir birincil depo, Ayarlar, Saha dışı, Bulut kimlik bilgileri altında yapılandırılan S3/REST kimlik bilgilerinin aynısıyla kimlik doğrular. Birincil depolar için ayrı bir kimlik bilgisi deposu yoktur.

## Değiştirilemez (yalnızca ekleme) site dışı

Fidye yazılımı ya da ele geçirilmiş bir host yedeklerinizi silemesin veya yeniden yazamasın diye bir site dışı depoyu yalnızca ekleme olarak işaretleyin. Karşı taraf (`--append-only` modunda çalışan bir `restic/rest-server`) bunu **uygular**. BombVault yalnızca bunu **doğrular** ve asla yalnızca bir yapılandırma iddiası üzerine yeşil göstermez.

**Rehberli site dışı kurulum** sihirbazı sizi arka uç seçiminden (rest-server / rclone / S3), yapıştırmaya hazır bir rest-server dağıtım parçacığı, bir bağlantı testi, değiştirilemez geçişi (bu, kurcalama testini hemen çalıştırır) ve bir saklama stratejisinden geçirir, böylece yalnızca ekleme site dışına yapılandırmaları elle düzenlemeden ulaşılabilir.

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

- **Geri yükleme doğrulama tatbikatları (yerel).** BombVault periyodik olarak `restic check --read-data-subset` çalıştırır (sınırlı, asla diski dolduran tam bir geri yükleme değil) ve etki alanı başına bir *son doğrulanan geri yüklenebilir* rozeti gösterir. Sıklık Ayarlar, Zamanlamalar'da; rozet Ayarlar, Bütünlük'te yer alır.
- **DR tatbikatları (site dışı).** BombVault gerçek bir hedefi site dışı depodan tek kullanımlık bir korumalı alana geri yükler, onu dosya-dosya ve bayt-bayt doğrular, ardından temizler. Bu, deponun yalnızca yanıt verdiğini değil, site dışından kurtarabildiğinizi kanıtlar.

Kontrol Paneli'ndeki **fidye yazılımı koruması karnesi** bunu etki alanı başına yeşil / sarı / kırmızı bir duruşa, yaş damgalı bir kontrol listesiyle (site dışı yapılandırıldı, yalnızca ekleme doğrulandı, çoğaltma güncel, geri yükleme tatbikatı geçti, şifreleme açık, budama stratejisi ayarlandı) toplar. Her kırmızı satır düzeltmeye derin bağlantı verir ve kart yalnızca doğrulanmış gerçekler üzerine yeşile döner.

## Alıcı kontrol paneli (alan taraf)

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

**1. VAULT üzerinde append-only sunucusunu kurun.** TOWER üzerindeki BombVault'ta *Ayarlar → Saha dışı → rehberli kurulum* bölümüne gidin, **rest-server** seçin ve tarifi oluşturun. **Unraid şablonu (XML)** sekmesini kopyalayın, VAULT üzerinde `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` olarak kaydedin, sonra *Docker → Add Container* deyip şablon listesinden **rest-server** seçin. Başlatmadan önce gösterilen `htpasswd` satırını VAULT üzerinde `/mnt/user/appdata/rest-server/.htpasswd` dosyasına yazın. Tek kullanımlık parola bir kez gösterilir ve hiç saklanmaz, şimdi kopyalayın.

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

**4. VAULT üzerinde neyin geldiğini izleyin.** *Ayarlar → Alıcı* seçeneğini açın, **Alıcı** sekmesini açın ve depoyu salt okunur olarak kaydedin.

!!! warning "Konum, kapsayıcının **içindeki** bir yoldur ve ana makine bağlama noktasına göre yazılır"
    `user/appdata/rest-server/bombvault-containers/containers` girin, `/mnt/user/appdata/…` **değil**. BombVault, ana makinenin `/mnt` dizininin başka yere bağlandığı bir kapsayıcıda çalışır; mutlak ana makine yolu orada yoktur. Yapıştırırsanız BombVault artık kullanmanız gereken göreli yolu söyler.

    **Gönderen APP_KEY**, VAULT'un değil TOWER'ın anahtarıdır. TOWER üzerinde *Ayarlar → Sistem* altında bulunur.

**5. İsterseniz karşılıklı yapın.** Aynı beş adımı ters yönde yineleyin: TOWER üzerinde VAULT'un kopyasını alan bir rest-server. Böylece her makine diğeri için değiştirilemezliği dayatır ve hiçbiri diğerinin yedeklerini silemez.

## Rehberli kurtarma

Özel bir **Kurtarma** sekmesi, sıfırdan ya da yeniden oluşturulmuş bir kurulumu felaket durumundan tek bir yerde geçirir:

1. **Önce BombVault'un kendi ayarlarını geri yükler**, böylece akışın geri kalanının ihtiyaç duyduğu yedekleme yolları, site dışı hedefler ve kimlik bilgileri önceden doldurulmuş gelir (Docker soketi üzerinden bir öz yeniden başlatma ile uygulanır, böylece canlı ayar veritabanı açık bir tanıtıcı altında asla üzerine yazılmaz).
2. **BombVault'un yedeklerinizi okuyabildiğini denetler** (şifreleme anahtarı tuzağı en başta).
3. **Mevcut deponuza yönlendirmenize** izin verir (yerel ya da site dışı).
4. İçinde saklanan konteynerleri, VM'leri ve dosya kümelerini **keşfeder**.
5. **Hepsini geri yükler** (durdurulmuş bırakılır, böylece onları kasıtlı olarak başlatırsınız), kurtarma kitiniz bir tık ötede.

!!! tip "Planlı geçiş ve felaket karşılaştırması"
    Rehberli kurtarma, BombVault'un kendi ayarlarını bir yedekten geri yükler. Yeni bir makineye *planlı* bir taşınma için, bunun yerine yapılandırmanızı **Ayarları dışa ve içe aktar** kartıyla (taşınabilir bir JSON dosyası) doğrudan taşıyabilirsiniz. Bkz. [Yapılandırma](configuration.md#portable-settings-export-and-import).

### Başka bir BombVault deposundan geri yükleme

**Kurtarma** sekmesindeki ayrı bir kart, *farklı* bir BombVault örneğinin deposunu (`/mnt` altında bağlanmış bir paylaşım ya da bir uzak URL) **o örneğin `APP_KEY`'iyle**, tek seferlik, salt okunur bir oturumda açar. Orada saklanan konteynerlere, VM'lere ve dosya kümelerine göz atın, bir anlık görüntü seçip geri yükleyin; geri yüklenen nesne normal bir yerel konteyner, VM ya da dosya kümesi olur. Diğer depoya asla hiçbir şey yazılmaz ve kendi yedekleme ayarlarınız dokunulmadan kalır (oturum bellekte yaşar ve kendiliğinden sona erer). Bir konteyneri A sunucusundan B sunucusuna taşımak artık depo ayarlarınızı yeniden yönlendirmek ve sonrasında geri almak anlamına gelmez. Canlı sunucudan sunucuya federasyon açıkça kapsam dışıdır; bu kasıtlı bir tek atışlık çekmedir.

## Şifreleme anahtarı kurtarma kiti

Bu, çalışan bir BombVault olmadığında bile felaket kurtarmayı mümkün kılan parçadır.

Tek tık, **ana anahtarı**, **türetilen restic parolasını** ve **tam depo konumlarını ve komutlarını** indirir, böylece herhangi bir makinede restic CLI ile doğrudan geri yükleyebilirsiniz. Bir Kontrol Paneli hatırlatıcısı, onu saklayana kadar dırdır eder.

!!! danger "Kurtarma kitini sunucu dışında saklayın"
    Kit, yedeklerinizin şifresini çözen gizli anahtarı içerir. Onu güvenli ve sunucudan ayrı bir yerde tutun (bir parola yöneticisi, bir kasada basılı bir kopya). Hem BombVault'u hem de `APP_KEY`'i kurtarma kiti olmadan kaybederseniz, şifreli yedekleriniz kurtarılamaz.

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
