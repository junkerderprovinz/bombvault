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

Kurtarma tanımları her deponun **içinde** yer aldığı için (`<repo>/def`, `<repo>/vm-def`), kopyalanan bir depo klasörü tamamen bağımsızdır, böylece kit ile birlikte depo, çıplak makine geri yüklemesinin ihtiyaç duyduğu her şeydir.
