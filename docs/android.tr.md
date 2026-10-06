# Android uygulaması

Android uygulaması, grubunuzdaki tüm BombVault sunucularını telefonunuza getirir. Sunucularınızın listesiyle açılır, listenin üstünde hepsinin etkinlik günlüğü durur (Kontrol Paneli'nin gösterdiği satırların aynısı) ve dokunduğunuz sunucunun telefon görünümünü açar. Uygulamanın kendisi hiçbir şeyi yedeklemez.

## Uygulamayı edinme {#install}

- **Ayarlar, Uygulamalar:** Android uygulamasının kartı, sunucunuzun çalıştırdığı sürümün APK'sını ve telefondan taranacak bir QR kodu sunar.
- **APK:** her sürümün sayfasında `bombvault-android.apk` bulunur ve [bu bağlantı](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) her zaman en son derlemeyi indirir. Android 10 veya üstü gerekir; Android, dosyayı açtığınız uygulamanın uygulama yükleyip yükleyemeyeceğini bir kez sorar.
- **Google Play:** uygulama, herkese açılabilene kadar kapalı testte. Google Play, yeni bir geliştirici hesabının uygulamasını ancak en az 12 test kullanıcısı onu 14 gün boyunca yüklü tuttuktan sonra listeler. Yardım etmek isterseniz [test grubuna](https://groups.google.com/g/arrowloop-testers) katılın, [test sayfasını](https://play.google.com/apps/testing/bombvault.halleluja.design) açın, **Test kullanıcısı ol** düğmesine dokunun ve BombVault'u Google Play'den yükleyin.
- **F-Droid:** kaydı daha sonra gelecek.

Uygulamayı 9.7.0 veya daha yeni sürümdeki sunucularla eşleştirin. Daha eski sürümdeki bir sunucu aynı grupta olabilir, ancak telefonu sıradan bir örnek olarak gösterir ve uygulama o sunucunun etkinliğini ancak oturum açtıktan sonra okur.

## QR koduyla eşleştirme {#pairing}

1. Grubunuzdaki herhangi bir sunucuda **Ayarlar, Eşleştirme**'yi açın ve **İfadeyi göster**'i seçin. On iki kelime, yanlarında bir QR koduyla görünür.
2. Uygulamada **QR kodu tara**'ya dokunun ve telefonu koda tutun. Kelimeleri yapıştırabilir ya da yazabilirsiniz de.
3. Uygulama, bir şey kaydetmeden önce o grubun sunucularını listeler. **N tanesini de ekle** hepsini birden ekler.

Telefon bundan sonra gruba başka bir örnek gibi katılır. Her sunucuda neyin çalıştığını oturum açmadan grup üzerinden okur: evde doğrudan, evden uzaktayken röle üzerinden. Grubun kendisinin nasıl çalıştığı [Örnekleri eşleştirme](offsite-recovery.md#pairing) bölümünde anlatılıyor.

!!! note "Arayüzün sunucuya yine de bir yolu olmalı"
    Sunucu listesi ve etkinlik günlüğü grup üzerinden gelir. Bir sunucunun arayüzü ise doğrudan açılır, bu yüzden telefonun sunucunun adresine evde ya da bir VPN üzerinden ulaşabilmesi gerekir.

## Eşleştirilmiş telefonda oturum {#sign-in}

Grubunuzla eşleştirilmiş bir telefon, grubun her sunucusunu oturum açılmış olarak açar. Bir sayfayı yüklemeden önce o sunucudan grup üzerinden bir oturum ister ve sunucu oturumu yalnızca telefon olan bir üyeye verir. Adresiyle eklenen bir sunucu, tarayıcıda olduğu gibi parola ister. On iki kelimeyi bilen biri grubun her yedeğini zaten açabilir, dolayısıyla eşleştirme yeni bir yetki vermez.

## Grup dışındaki sunucular {#other-servers}

- **Sunucu ekle**, BombVault'u tarayıcıda açtığınız adresi alır, örneğin `192.168.1.10:3443`. Başında `http://` ya da `https://` yoksa uygulama https kullanır.
- Yerel ağda kendini duyuran sunucular **Bu ağda** altında listelenir ve tek dokunuşla açılır. Bunu, Ayarlar, Entegrasyonlar altında **Ağda bul** açık olduğu sürece yaparlar. Başka bir ağdaki ya da bir VPN arkasındaki sunucu orada görünmez.
- Kendinden imzalı bir sertifikaya SHA-256 parmak iziyle bir kez güvenilir. Sunucu daha sonra farklı bir sertifika gösterirse uygulama sizi uyarır ve sunucuyu ancak yeni sertifikaya güvendikten sonra açar.

## Örnekler sayfasında telefon {#instances}

Telefon, gruptaki her sunucunun Örnekler sayfasında kendi kartını alır; kart Android uygulaması olarak işaretlenir ve telefona verdiğiniz adı taşır. Hiçbir şeyi yedeklemediği için karnesi yoktur ve **Kaldır** onu sayfadan çıkarır.

## Ayarlar {#settings}

Artı işaretinin yanındaki dişli, uygulamanın ayarlarını açar:

- dil ve telefonun Örnekler sayfasında gösterdiği ad (boş bırakılırsa telefonun modeli kullanılır),
- siz kendinizinkini ayarlayana kadar listedeki ilk sunucuyu izleyen görünüm, ve kendi ayarı olan animasyonlar,
- bir sorun bildirirken kopyalanacak bir rapor; içinde adres, ad ya da ifade yoktur,
- gizlilik politikasıyla birlikte Hakkında kartı,
- **Tüm sunucuları kaldır**: tüm sunucuları uygulamadan kaldırır ve gruptan ayrılır. Sunucuların kendisinde hiçbir şey değişmez.

## İndirmeler ve yüklemeler {#files}

Dışa aktarmalar, kurtarma kitleri, flash ZIP'leri ve veritabanı dökümleri, tarayıcıdan indirildiğinde olduğu gibi telefonun İndirilenler klasörüne düşer. Ayarları içe aktarmak telefonun dosya seçicisini açar.

## Dokunmatik ekranda {#touch}

Parmağın altında üzerine gelme diye bir şey yoktur, bu yüzden bir denetim basılı tutulduğu sürece soluklaşır ve marka logosu taşıyan bir düğme, parmak kalkana kadar markasının renginde yanar. Bir düğmeye uzun basmak yavaş bir dokunuş sayılır ve bağlantı menüsü açmaz.

## Bunun yerine tarayıcıda {#browser}

Chrome ve Edge, BombVault'un web arayüzünü kendi penceresinde bir uygulama olarak yükleyebilir, telefonda da bilgisayarda da. Hiçbir şey önbelleğe alınmaz, bu yüzden bir güncelleme hemen görünür.

## Gizlilik {#privacy}

Uygulamanın hesabı, reklamı ve analitiği yoktur ve arka planda hiçbir şey çalıştırmaz. [Gizlilik politikası](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md), neyi sakladığını ve neyi nereye gönderdiğini listeler.
