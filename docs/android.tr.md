# Android uygulaması

Android uygulaması, grubundaki tüm BombVault sunucularını telefonuna getirir. Sunucularının listesiyle açılır, listenin üstünde hepsinin etkinlik günlüğü durur (genel bakışın gösterdiği satırların aynısı) ve dokunduğun sunucunun telefon görünümünü açar. Uygulamanın kendisi hiçbir şeyi yedeklemez.

## Uygulamayı edinme {#install}

- **Ayarlar, Uygulamalar:** Android uygulamasının kartı, sunucunun çalıştırdığı sürümün APK'sını ve telefondan taranacak bir QR kodu sunar.
- **APK:** her sürümün sayfasında `bombvault-android.apk` bulunur ve [bu bağlantı](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) her zaman en son derlemeyi indirir. Android 10 veya üstü gerekir; Android, dosyayı açtığın uygulamanın uygulama yükleyip yükleyemeyeceğini bir kez sorar.
- **Google Play:** uygulama, herkese açılabilene kadar kapalı testte. Google Play, yeni bir geliştirici hesabının uygulamasını ancak en az 12 test kullanıcısı onu 14 gün boyunca yüklü tuttuktan sonra listeler. Yardım etmek istersen [test grubuna](https://groups.google.com/g/arrowloop-testers) katıl, [test sayfasını](https://play.google.com/apps/testing/bombvault.halleluja.design) aç, **Test kullanıcısı ol** düğmesine dokun ve BombVault'u Google Play'den yükle.
- **F-Droid:** kaydı daha sonra gelecek.

Uygulamayı 9.7.0 veya daha yeni sürümdeki sunucularla eşleştir. Daha eski sürümdeki bir sunucu aynı grupta olabilir, ancak telefonu sıradan bir örnek olarak gösterir ve uygulama o sunucunun etkinliğini ancak oturum açtıktan sonra okur.

## QR koduyla eşleştirme {#pairing}

1. Grubundaki herhangi bir sunucuda **Ayarlar, Eşleştirme**'yi aç ve **İfadeyi göster**'i seç. On iki kelime, yanlarında bir QR koduyla görünür.
2. Uygulamada **QR kodu tara**'ya dokun ve telefonu koda tut. Kelimeleri yapıştırabilir ya da yazabilirsin de.
3. Uygulama, bir şey kaydetmeden önce o grubun sunucularını listeler. **N tanesini de ekle** hepsini birden ekler.

Telefon bundan sonra gruba başka bir örnek gibi katılır. Her sunucuda neyin çalıştığını oturum açmadan grup üzerinden okur: evde doğrudan, evden uzaktayken röle üzerinden. Grubun kendisinin nasıl çalıştığı [Örnekleri eşleştirme](offsite-recovery.md#pairing) bölümünde anlatılıyor.

Uygulama, evde de olsa gruba röle üzerinden katılır. Gruptaki örnek **Röle yok** durumundaysa, kodu taramadan önce **Ayarlar, Eşleştirme**'den bir röle seç.

## Evden uzakta {#away}

Uygulama bir sunucuyu önce kendi adresinde dener. Adres yanıt vermezse, örneğin mobil ağda, sunucunun arayüzünü grubun rölesi üzerinden açar ve sayfanın altındaki bir satırda **Röle üzerinden açıldı** yazar. Grubun, adresini uygulamanın henüz bilmediği bir sunucusu da aynı şekilde açılır. Ne adres ne de röle yanıt verirse başlangıç ekranı bunu sunucunun kartında gösterir.

Röle üzerinden sayfa, eşleştirilmiş bir telefonun evde eriştiği her şeye erişir, aşağıda anlatıldığı gibi. Parolayla oturum açma ve dosya kaydetme yalnızca sunucunun kendi adresinde çalışır. İlerleme çubukları sürekli değil, iki saniyede bir güncellenir. Arayüzün dosyaları ilk ziyaretten sonra telefonda kalır, bu yüzden sonraki ziyaretler daha hızlı yüklenir. 10.0.0'dan eski bir sunucu yalnızca kendi adresinde açılır.

Uygulama bir yanıtı yalnızca sunucu onu, uygulamanın bu sunucuyla ilk karşılaştığında sabitlediği anahtarla imzalamışsa kabul eder. Sunucu daha sonra başka bir anahtarla yanıt verirse başlangıç ekranı yeni anahtarın parmak izini gösterir ve sayfa ancak ona güvendikten sonra açılır.

## Eşleştirilmiş telefonda oturum {#sign-in}

Grubunla eşleştirilmiş bir telefon, grubun her sunucusunu oturum açılmış olarak açar. Bir sayfayı yüklemeden önce o sunucudan grup üzerinden bir oturum ister ve sunucu oturumu yalnızca telefon olan bir üyeye verir. Adresiyle eklenen bir sunucu, tarayıcıda olduğu gibi parola ister.

Eşleştirilmiş bir telefon, bir parolanın verdiği her şeyi almaz. Oturumu, uygulamanın gösterdiklerinden oluşan sabit bir listeye erişir: genel bakış, çalışmalar ve etkinlik günlüğü, bakılacak yedekler, başlatma ve geri yükleme, okunacak anormallikler ve bakılacak Örnekler sayfası. Hedefler ve komutlar, adresleri ve gizli bilgileri çıkarılmış olarak döner. Orijinali değiştiren bir geri yükleme önce mevcut durumu yedekler. Anahtarlar, kurtarma kiti, anahtar emanetleri, ayarlar ve yedekleri silme, sunucunun kendisindeki bir oturum açmayla kalır; geri kalan her şey bu oturum açmaya ihtiyaç duyduğunu söyler. Röle üzerinden aynı liste geçerlidir, parolasız bir sunucuda da.

Bunun anahtarı, **Ayarlar, Güvenlik** altındaki **Android uygulaması** kartında **Telefonlar grubun on iki kelimesiyle eşleşebilir**'dir. Bir parola sıfırlama onu kapatır ve her telefonun oturumunu sonlandırır; sen orada tekrar açana kadar böyle kalır.

## Grup dışındaki sunucular {#other-servers}

- **Sunucu ekle**, BombVault'u tarayıcıda açtığın adresi alır, örneğin `192.168.1.10:3443`. Başında `http://` ya da `https://` yoksa uygulama https kullanır.
- Yerel ağda kendini duyuran sunucular **Bu ağda** altında listelenir ve tek dokunuşla açılır. Bunu, **Ayarlar, Entegrasyonlar** altındaki **Hizmetler** kartında **Ağda bul** açık olduğu sürece yaparlar. Başka bir ağdaki ya da bir VPN arkasındaki sunucu orada görünmez.

- Kendinden imzalı bir sertifikaya SHA-256 parmak iziyle bir kez güvenilir. Sunucu daha sonra farklı bir sertifika gösterirse uygulama seni uyarır ve sunucuyu ancak yeni sertifikaya güvendikten sonra açar.

## Örnekler sayfasında telefon {#instances}

Telefon, gruptaki her sunucunun Örnekler sayfasında kendi kartını alır; kart Android uygulaması olarak işaretlenir ve telefona verdiğin adı taşır. Hiçbir şeyi yedeklemediği için karnesi yoktur ve **Kaldır** onu sayfadan çıkarır. Kelimeleri bilen bir telefon istediği zaman yeniden eşleşebilir; bu yüzden kaybolan bir telefon için **Ayarlar, Güvenlik** altındaki **Telefonlar grubun on iki kelimesiyle eşleşebilir** anahtarını kapat ya da her örnekte **Gruptan ayrıl** seçeneğini kullan ve onları yeni kelimelerle yeniden eşleştir.

10.0.0 sürümünden önce eşleştirilmiş bir telefon, güncellemeden sonraki ilk ziyaretinde çağrılarını imzalamaya başlar ve her sunucu telefon için aldığı ilk anahtarı saklar. Telefon çevrimdışıyken grubun kelimelerini bilen başka biri önce davranırsa, o sunucu o andan itibaren telefonu geri çevirir ve uygulama onu artık röle üzerinden açamaz. Düzeltmek için o sunucuda bir tarayıcıdan oturum aç, Örnekler sayfasında telefonun kartında **Anahtarı unut** seçeneğini, ardından **Unutmayı onayla** seçeneğini seç ve uygulamayı yeniden aç. Sunucu, bir sonraki çağrıda telefonun kendi anahtarını alır. Telefon yine geri çevrilirse kelimeler başkasındadır: her örnekte **Gruptan ayrıl** seçeneğini kullan ve onları yeni kelimelerle yeniden eşleştir, tıpkı kaybolan bir telefonda olduğu gibi.

## Ayarlar {#settings}

Artı işaretinin yanındaki dişli, uygulamanın ayarlarını açar:

- dil ve telefonun Örnekler sayfasında gösterdiği ad (boş bırakılırsa telefonun modeli kullanılır),
- sen kendininkini ayarlayana kadar listedeki ilk sunucuyu izleyen görünüm, ve kendi ayarı olan animasyonlar,
- bir sorun bildirirken kopyalanacak bir rapor; içinde adres, ad ya da ifade yoktur,
- gizlilik politikasıyla birlikte Hakkında kartı,
- **Tüm sunucuları kaldır**: tüm sunucuları uygulamadan kaldırır ve gruptan ayrılır. Sunucuların kendisinde hiçbir şey değişmez.

## İndirmeler ve yüklemeler {#files}

Dışa aktarmalar, kurtarma kitleri, flash ZIP'leri ve veritabanı dökümleri, tarayıcıdan indirildiğinde olduğu gibi telefonun İndirilenler klasörüne düşer. Ayarları içe aktarmak telefonun dosya seçicisini açar. Röle üzerinden hiçbir dosya kaydedilmez; bunun için sunucuyu kendi adresinde aç.

## Dokunmatik ekranda {#touch}

Parmağın altında üzerine gelme diye bir şey yoktur, bu yüzden bir denetim basılı tutulduğu sürece soluklaşır ve marka logosu taşıyan bir düğme, parmak kalkana kadar markasının renginde yanar. Bir düğmeye uzun basmak yavaş bir dokunuş sayılır ve bağlantı menüsü açmaz.

## Bunun yerine tarayıcıda {#browser}

Chrome ve Edge, BombVault'un web arayüzünü kendi penceresinde bir uygulama olarak yükleyebilir, telefonda da bilgisayarda da. Hiçbir şey önbelleğe alınmaz, bu yüzden bir güncelleme hemen görünür.

## Gizlilik {#privacy}

Uygulamanın hesabı, reklamı ve analitiği yoktur ve arka planda hiçbir şey çalıştırmaz. [Gizlilik politikası](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md), neyi sakladığını ve neyi nereye gönderdiğini listeler.
