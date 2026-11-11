# Android uygulaması

Android uygulaması grubundaki her BombVault sunucusunu telefonuna getirir. Sunucularının bir listesiyle açılır, en üstte hepsinin etkinlik günlüğü, genel bakışın gösterdiği aynı satırlar bulunur, dokunduğun sunucunun telefon görünümünü açar. Uygulamanın kendisi hiçbir şey yedeklemez.

![Uygulamanın başlangıç ekranı: en üstte tüm sunucuların etkinlik günlüğü, ardından sunucu listesi.](assets/screenshots/android/start-dark.png#only-dark)
![Uygulamanın başlangıç ekranı: en üstte tüm sunucuların etkinlik günlüğü, ardından sunucu listesi.](assets/screenshots/android/start-light.png#only-light)

## Uygulamayı edinmek {#install}

- **Ayarlar, Uygulamalar:** Android uygulaması kartı, sunucunun çalıştırdığı sürümün APK'sını, telefondan taranabilecek bir QR koduyla sunar ve F-Droid sayfasına bağlanır.
- **APK:** her sürümün sürüm sayfasında `bombvault-android.apk` bulunur ve [bu bağlantı](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) her zaman son sürümü indirir. Android 10 ya da üstü gerekir ve Android dosyayı açtığın uygulamanın uygulama kurup kuramayacağını bir kez sorar.
- **Google Play:** uygulama herkese açılabilene kadar kapalı testte. Google Play, yeni bir geliştirici hesabından gelen bir uygulamayı ancak en az 12 test kullanıcısı onu 14 gün kurulu tuttuktan sonra listeler. Yardım etmek için [test kullanıcıları grubuna](https://groups.google.com/g/arrowloop-testers) katıl, [test sayfasını](https://play.google.com/apps/testing/bombvault.halleluja.design) aç, **Test kullanıcısı ol**'a dokun ve BombVault'u Google Play'den kur.
- **F-Droid:** [F-Droid'de BombVault](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid uygulamayı kaynaktan derler ve kendi anahtarıyla imzalar, bu yüzden yeni bir sürüm oraya birkaç gün sonra ulaşır. F-Droid uygulaması ile APK arasında geçmek için önce diğerini kaldır.

Uygulamayı 9.7.0 ve üstü sürümdeki sunucularla eşleştir. Daha eski bir sürümdeki sunucu aynı grupta olabilir, ama telefonu sıradan bir örnek olarak gösterir ve uygulama o sunucunun etkinliğini ancak oturum açtıktan sonra okur.

## QR kodla eşleştirme {#pairing}

1. Grubundaki herhangi bir sunucuda **Ayarlar, Eşleştirme**'yi aç ve **İfadeyi göster**'i seç. On iki kelime, yanında bir QR kodla görünür.
2. Uygulamada **QR kodu tara**'ya dokun ve telefonu koda tut. Kelimeleri yapıştırabilir ya da yazabilirsin de.
3. Uygulama bir şey kaydetmeden önce o grubun sunucularını listeler. **Tümünü ekle** hepsini ekler.

Telefon sonra gruba başka bir örnek gibi katılır. Her sunucuda ne çalıştığını oturum açmadan grup üzerinden okur, evdeyken doğrudan, evden uzaktayken röle üzerinden. Grubun kendisinin nasıl çalıştığı [Örnekleri eşleştirme](offsite-recovery.md#pairing) altında anlatılır.

Uygulama gruba, evde de, röle üzerinden katılır. Örneklerin **Röle yok** kullanıyorsa, kodu taramadan önce **Ayarlar, Eşleştirme** altında birini seç.

## Evden uzakta {#away}

Uygulama önce bir sunucuyu kendi adresinden dener. Bu yanıt vermediğinde, mobil ağda olduğu gibi, sunucunun arayüzünü grubun rölesi üzerinden açar. Uygulamanın adresini henüz bilmediği bir grup sunucusu da aynı şekilde açılır. Ne adres ne röle yanıt veriyorsa başlangıç ekranı bunu sunucunun kartında söyler.

Röle üzerinden sayfa, aşağıda anlatıldığı gibi, eşleşmiş bir telefonun evde ulaştığı şeye ulaşır. Parolayla oturum açma ve dosya kaydetme yalnızca sunucunun kendi adresinde çalışır. İlerleme çubukları sürekli değil iki saniyede bir güncellenir. Arayüzün dosyaları ilk ziyaretten sonra telefonda kalır, bu yüzden sonraki ziyaretler daha hızlı yüklenir. 10.0.0'dan eski bir sunucu yalnızca kendi adresinde açılır.

Uygulama bir yanıtı yalnızca sunucu onu, uygulamanın o sunucuyla ilk tanıştığında sabitlediği anahtarla imzalamışsa kabul eder. Sunucu sonradan başka bir anahtarla yanıt verirse başlangıç ekranı yeni anahtarın parmak izini gösterir ve sayfa yalnızca ona güvendikten sonra açılır.

## Sunucu değiştirmek {#switch}

Bir sunucu açıkken genel bakışının en üstünde, önünde bir geri okuyla birlikte sunucunun adı görünür. Ok uygulamanın genel bakışına, yani başlangıç ekranına götürür, orada uygulamanın bildiği her sunucu durumuyla listelenir: **Bağlandı**, **Bağlı değil**, **Oturum açma gerekiyor** ya da **Sertifikayı kontrol et**. Yanıt vermeyen bir grup sunucusu gri renkte **Etkin değil** olur ve grubun onu son ne zaman gördüğü yazar, uygulamanın henüz haber almadığı biri ise **Denetleniyor** der. Uygulama her bağlandığında grubun listesini örneklerinden birinden okur, böylece röle üzerinde hiç tanışmadığı bir örnek, örneğin telefon katıldığında kapalı olan, de listelenir. Birini açmak için orada dokun: evdeyken doğrudan, aksi halde röle üzerinden.

## Eşleşmiş bir telefonda oturum açma {#sign-in}

Grubunla eşleşmiş bir telefon, sunucularının her birini zaten oturum açmış olarak açar. Bir sayfayı yüklemeden önce o sunucudan grup üzerinden bir oturum ister ve sunucu bunu yalnızca telefon olan bir üyeye verir.

Eşleşmiş bir telefon parolanın verdiği her şeyi almaz. Oturumu, uygulamanın gösterdiği sabit bir listeye ulaşır: genel bakış, çalıştırmalar ve etkinlik günlüğü, bakılacak, başlatılacak ve geri yüklenecek yedekler, okunacak anormallikler ve bakılacak Örnekler sayfası, üyelerinin alınan depoları ve çekme kaynakları olmadan. Hedefler ve komutlar adresleri ve gizli bilgileri kaldırılmış olarak gelir. Orijinali değiştiren bir geri yükleme önce mevcut durumu yedekler. Bir klasöre geri yükleme, **Ayarlar, Geri yükleme** altında ayarlanan geri yükleme klasörünün içinde yeni bir klasöre gider ve pencere başka bir klasör sunmaz. Anahtarlar, kurtarma kiti, anahtar emanetleri, ayarlar ve yedekleri silme sunucunun kendisinde oturum açmaya bağlı kalır, geri kalan her şey o oturum açmaya ihtiyaç duyduğunu yanıtlar. Röle üzerinden aynı liste geçerlidir, parolasız bir sunucuda da.

Bunun anahtarı, **Ayarlar, Güvenlik** altındaki **Android uygulaması** kartında **Telefonlar grubun on iki kelimesiyle eşleşebilir**'dir. Bir parola sıfırlama onu kapatır ve her telefonun oturumunu sonlandırır, orada yeniden açana kadar.

## Sertifikalar {#certificates}

Kendi kendine imzalı bir sertifikaya SHA-256 parmak iziyle bir kez güvenilir. Sunucu sonradan farklı bir tane gösterirse uygulama seni uyarır ve yalnızca yeni sertifikaya güvendikten sonra açar.

## Örnekler sayfasında telefon {#instances}

Telefon, gruptaki her sunucunun Örnekler sayfasında kendi kartını alır, Android uygulaması olarak işaretlenmiş ve telefona verdiğin adı taşır. Hiçbir şey yedeklemediği için puan tablosu yoktur. **Kaldır** onu sayfadan alır ve oturumunu hemen sonlandırır, ayrıntılarında **İzleme** altındaki **Bu örneği sorgula**'yı kapatmak da öyle. Telefon kelimeleri hâlâ bilir, bu yüzden uygulaması yeniden açıldığında tekrar görünür, ama kapalı olarak, ve biri açana kadar oturum almaz. Kayıp bir telefon için kartını her sunucuda kaldır, ya da **Ayarlar, Güvenlik** altında **Telefonlar grubun on iki kelimesiyle eşleşebilir**'i kapat, ya da her örneğin **Gruptan ayrıl**'ını yapmasını sağla ve onları yeni kelimelerle yeniden eşleştir.

10.0.0'dan önce eşleşmiş bir telefon, güncellemeden sonraki ilk ziyaretinde çağrılarını imzalamaya başlar ve her sunucu telefon için aldığı ilk anahtarı tutar. Grubun kelimelerini bilen başka biri, telefon çevrimdışıyken oraya önce ulaşırsa, o sunucu telefonu o andan sonra geri çevirir ve uygulama onu artık röle üzerinden açamaz. Bunu düzeltmek için o sunucuda bir tarayıcıda oturum aç, Örnekler sayfasındaki telefonun kartında **Anahtarı unut**'u, sonra **Unutmayı onayla**'yı seç ve uygulamayı yeniden aç. Sunucu bir sonraki çağrıda telefonun kendi anahtarını alır. Telefon yine geri çevrilirse kelimeleri başkası biliyor demektir: her örnek **Gruptan ayrıl** yapsın ve onları kayıp bir telefonda olduğu gibi yeni kelimelerle yeniden eşleştir.

## Ayarlar {#settings}

Artının yanındaki dişli uygulamanın ayarlarını açar:

- dil ve telefonun Örnekler sayfasında gösterdiği ad (boş, telefonun modeli demektir),
- kendi görünümünü ayarlayana kadar listedeki ilk sunucuyu izleyen görünüm ve kendi ayarı olan animasyonlar,
- bir sorun bildirirken kopyalanacak bir rapor. Adres, ad ya da ifade içermez,
- gizlilik politikasıyla Hakkında kartı,
- uygulamadan her sunucuyu kaldıran ve gruptan ayrılan **Tüm sunucuları kaldır**. Telefonun hâlâ ulaşabildiği her sunucu onu kendi Örnekler sayfasından alır.

## İndirmeler ve yüklemeler {#files}

Dışa aktarmalar, kurtarma kitleri, flash ZIP'leri ve veritabanı dökümleri, bir tarayıcıdan olacağı gibi telefonun İndirilenler klasörüne düşer. Bir ayar içe aktarması telefonun dosya seçicisini açar. Röle üzerinden hiçbir dosya kaydedilmez. Bunun için sunucuyu kendi adresinde aç.

## Dokunmatik ekranda {#touch}

Parmağın altında hiçbir şey üzerine gelinmez, bu yüzden bir kontrol basılı tutulurken soluklaşır ve marka logolu bir düğme, parmak kalkana kadar marka renginde parlar. Bir düğmeye uzun basmak yavaş bir dokunuş sayılır ve bağlantı menüsü açmaz. Gri bir düğmeye dokunmak, başka bir yere dokunana kadar neden gri olduğunu gösterir.

## Bunun yerine tarayıcıda {#browser}

Chrome ve Edge, BombVault'un web arayüzünü kendi penceresinde bir uygulama olarak kurabilir, bilgisayarda olduğu gibi telefonda da. Hiçbir şey önbelleğe alınmaz, bu yüzden bir güncelleme hemen görünür.

## Gizlilik {#privacy}

Uygulamada hesap, reklam ve analitik yoktur ve arka planda hiçbir şey çalıştırmaz. [Gizlilik politikası](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) neyi sakladığını ve neyi nereye gönderdiğini listeler.
