# Ayarlar

**Ayarlar** on dört kutucuktan oluşur. Her kutucuk bir kart sayfasıdır ve her değişiklik yapıldığı anda yazılır, kontrolde kısa bir parlama görünür, sunucu reddederse sallanma. Birkaç pencerede **Kaydet** düğmesi kalır, çünkü yarım kaydetmek güvenli olmazdı, örneğin bir oturum açma ya da giriş parolası. Açıklamalar her kontrolün yanındaki (i) balonlarındadır.

Ayarların herhangi bir yerinde `/` ya da Ctrl+K'ya basarak hepsinde arama yapabilirsin, sayfaların önceki sürümlerde taşıdığı adlarla da. Kutucukların sırası **Görünüm** altında değiştirilebilir. Sayfanın yanındaki bir kutucuk sütununun onu sıkıştıracağı yerde, örneğin yaklaşık 1200 piksel genişliğinde bir pencerede ya da telefonda, kutucuklar sayfanın üstünde yana kayan bir şeride dönüşür.

![Genel kutucuğu açık Ayarlar.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Genel kutucuğu açık Ayarlar.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Bu sayfa her kutucuğu ve kartlarını listeler, her birini derinlemesine anlatan sayfaya bağlanır.

## Genel {#general}

- **Bölümler**, her bölümü açar ya da kapatır: **Konteynerler**, **VM'ler**, **Flash**, **Klasörler**, **ZFS veri kümeleri** ve **Öz yedek**. Kapalı bir bölüm, bir ögenin kendi planı dahil hiçbir zamanlamayla yedeklenmez ve ögeleri sayılmaz. **Şimdi yedekle** yine de çalışır. **Flash** yalnızca Unraid'de görünür.
- **Konteynerler**, tüm konteynerler için geçerli olanı barındırır, örneğin **Otomatik veritabanı dökümleri**. Tek bir konteyneri etkileyen her şey onun sayfasındadır. Bkz. [Veritabanı dökümleri](backups.md#database-dumps).

## Görünüm {#look}

- **Görünüm**: **Dil** (42 dil), **Tema** (**Koyu**, **Açık** veya **Sistem**) ve altında, koyu tema göründüğü sürece OLED ve AMOLED ekranlara gerçek siyah veren **OLED**, **Köşeler**, **Animasyon** (**Kapalı**, **Hafif**, **Vahşi** veya **Fırtına**), kaydetme onayları gibi başarı bildirimlerini gizleyen **Sessiz bildirimler** ve bir kutucuğu tutup yerine sürüklediğin ya da Alt ve ok tuşlarını kullandığın **Ayar sayfalarının sırası**.
- **Etiketler**: bir kontrolün ne kadarının gösterileceği, **Düğmeler**, **Kenar çubuğu**, **Sekmeler** ve **Alt çubuk** için ayrı ayrı: **Metin**, **Metin ve simge**, **Simge** ya da yalnızca simgeyi gösterip üstüne gelince sözcükleri kaydıran **Tepkisel**. **Kenar çubuğu** yalnızca masaüstü düzenini, **Alt çubuk** yalnızca telefon düzenini etkiler.
- **Renkler**: tek bir **Vurgu rengi** ya da bir listenin her satırının sekiz renkli bir paletten kendi rengini aldığı **Gökkuşağı modu**. Gökkuşağı modu şunları ekler: renklerin yavaşça sonrakine kaydığı **Disko modu**, bir satırın renginin yalnızca üstüne geldiğinde, çalıştığında ya da seçildiğinde göründüğü **Reaktif mod**, **Renk döngüsü** ve **Renk paleti**'nin kendisi. Vurgu renklerinin başındaki sihirli değnek, **Sistem rengi**, cihazının kendi rengini alır: Android uygulamasında Android 12'den itibaren Material You, Firefox ve Safari'de ise sistem rengi. Yalnızca cihazın böyle bir rengi olduğu yerde görünür.

BombVault görünümü sunucuda tutar. İlk kez gelen bir tarayıcı onu devralır. **OLED** ve **Sistem rengi** bunun istisnasıdır: ekrana bağlı oldukları için her cihaz bunları kendine saklar. Sistemin daha az hareket istediğinde bu her zaman **Animasyon**'dan önce gelir.

## Zamanlama {#schedule}

**Hafta**, **Yedeklemeler ne zaman çalışır**, **Başlangıçtan sonra kaçırılan yedeklemeleri telafi et** ve **Öncesi ve sonrası komutları** ile **Tam yedekleme**, **Bölüm başına site dışı kopyalar** ile **Bölüm başına zamanlamalar**, **Zamanlanmış ögeler**, **Yedeklemeden sonra yeniden başlatma** ile **Sıra**, **Durdurma yerine anlık görüntü**, **Yedeklemeden sonra güncellemeler** ve **Yedek öncesi boşta kalma**. Bkz. [Zamanlamalar](schedules.md).

## Hedefler {#storage-locations}

- **Kenar çubuğunda da göster**, Hedefler sayfasını kenar çubuğuna sabitler.
- **Hedef ekle** ile hedeflerinin listesi. Tıklamak bir hedefin sayfasını açar, bkz. [Bir hedefin sayfası](offsite-recovery.md#location-page).
- **Oturum açma bilgileri**: varsayılan yıldızı olan kayıtlı oturum açma bilgileri, bkz. [Oturum açma bilgileri](offsite-recovery.md#sign-ins).
- **Bölüme göre hedefler**: her satırın bir anahtar olduğu 3-2-1 akışı, bkz. [Bölüm başına varsayılanlar](offsite-recovery.md#placement-defaults).
- **Depolar**: tek tek ögeler için yapılmış depolar, bkz. [Öge başına yerleşim](offsite-recovery.md#placement).
- **Ne kadar saklanır**, saklamanın ayarlandığı hedeflere işaret eder, **Önizleme** ise bunu olmadan önce depo başına ve öge başına **Sonraki çalışmanın kaldıracakları**'nı gösterir. Hiçbir şeyi değiştirmez ve kilitlemez, bu yüzden bir yedekleme çalışırken bile yanıt verir.
- **Site dışı kopyalar**: her bölüm için **Bağlantıyı test et**, **Şimdi çoğalt** ve **Kur** ya da **Düzenle** içeren bir satır, bkz. [Site dışı kopyalar](offsite-recovery.md#off-site-copies).
- **Site dışı bant genişliği**: her kopya için **Yükleme sınırı (KiB/s)** ve **İndirme sınırı (KiB/s)**. 0 sınırsız demektir.

Plex, Jellyfin ya da Emby yayın yaparken yükleme sınırını düşüren **Yayın** kartı, Hedefler sayfasının kendisinde durur.

## Geri yükleme {#restore}

- **Geri yükleme** sayfasına bir bağlantı.
- **Geri yükleme klasörleri**: bir klasöre geri yükleme için **Varsayılan geri yükleme klasörü**, **Her bölüm için alt klasör** ve bölüm başına bir **İstisna** ile. Bkz. [Bir klasöre geri yükleme nereye gider](restore.md#folders).
- **Container registry'leri**: özel registry'ler için oturum açma bilgileri, örneğin sponsorlara özel bir `ghcr.io` image'i. Geri yükleme bunları eksik bir image'i çekmek için, yedekleme sonrası güncelleme de daha yeni birini denetlemek için kullanır. Herkese açık image'ler bunlar olmadan çekilir. **Registry ekle** herkese açık registry'leri, kendi barındırdıklarını, bulut sağlayıcısındakileri ve herhangi bir adres için **Diğer registry**'yi sunar.
- **Acil durumlar için**: giriş parolası gerektiren **Kurtarma kitini indir**. Bkz. [Kurtarma kiti](offsite-recovery.md#recovery-kit).

## Bildirimler {#notifications}

**Bildir**, özetler, **Geciken yedekleri bildir**, **Unraid bildirimleri** ve **Haftalık özet** içeren **Bildirimler**, Webhook, Apprise, Matrix, Gotify ve E-posta (SMTP) içeren **Kanallar** ve **Healthchecks**. Bkz. [Bildirimler](notifications.md).

## Doğrulama {#verify}

- **Yedekleri denetle**: bölüm başına son `restic check` sonucu, her satırda **Diğer** altında **Kilidi aç** ve **Buda** ile. **Kilidi aç** eski bir kilidi temizler ve hâlâ yenilenen bir kilide dokunmaz, **Buda** saklama kurallarını istek üzerine uygular.
- **Otomatik denetimler**: **Veri örneği (%)** ve test hedefleriyle zamanlanmış geri yükleme denetimi, bir konteyneri yalıtılmış bir kopyaya geri yükleyip başlatan **Başlatma testi**, **Zamanlanmış site dışı DR tatbikatı** ve zamanlanmış kurcalama testi olan **Silme korumasını denetle**. Bkz. [DR tatbikatları](offsite-recovery.md#dr-drills) ve [Kurcalama testi](offsite-recovery.md#tamper-test).
- **Anormallikler**: algılama, duyarlılık ve hangi bulguların mesaj göndereceği. Bkz. [Anormallikler](anomalies.md).

## Güvenlik {#security}

- **Parola**: giriş parolası, en az 12 karakter. Değiştirmek ya da silmek mevcut parolayı ve, iki adımlı doğrulama açıksa, kodu ister.
- **İki adımlı doğrulama**: parolanın üstüne bir kimlik doğrulayıcı uygulamasından kod, bir kez gösterilen sekiz kurtarma koduyla.
- **Geçiş anahtarları**: telefonundaki, dizüstündeki ya da güvenlik anahtarındaki bir anahtarla oturum açma. Gerçek bir alan adı ve tarayıcının güvendiği bir sertifika gerektirirler.
- **Android uygulaması**: **Telefonlar grubun on iki kelimesiyle eşleşebilir**, eşleşmiş bir telefonun parolasız oturum açmasını sağlar.
- **Şifreleme**: varsayılan olarak açık **Yedekleri şifrele**.

Kurtarma kiti ve anahtar emanetleri gibi bazı şeyler bir giriş parolası gerektirir. Bkz. [Güvenlik modeli](configuration.md#security-model).

## Örnekler {#instances}

- **Kenar çubuğunda da göster**, Örnekler sayfasını kenar çubuğuna sabitler.
- **Birden çok sunucu**, grubun parçalarını açar: **Örnekler**, her üye için bir kartı olan sayfa, **Alıcı** ve **Çekme**. Bkz. [Örnekler ve roller](offsite-recovery.md#roles).
- Örnekler sayfasına, açık olan parçalarla birlikte bir bağlantı.

## Eşleştirme {#pairing}

**İfade oluştur** ya da **İfade gir** içeren **Eşleştirme**, adım kartları, bu örneğin adı ve **Bilinen adresler**, dört biçimi ve **Rölenin gördüğü** olan **Röle** kartı, grubun üyeleri ve SSH üzerinden bir replika hedefi için **BombVault olmayan ZFS sunucusu**. Bkz. [Örnekleri eşleştirme](offsite-recovery.md#pairing) ve [ZFS sunucusuna gönderme](zfs-datasets.md#replica-push).

## Entegrasyonlar {#integrations}

- **MCP sunucusu**: yapay zeka asistanları için anahtarlar, **İstemci seç** ve her anahtarın günlüğü. Bkz. [MCP sunucusu](mcp.md).
- **API belirteçleri**: betikler ve panolar için adlandırılmış belirteçler. Bkz. [Belirteçler](api.md#tokens).
- **Hizmetler**: MQTT üzerinden **Home Assistant**, **Ağda bul** (`bombvault.local` olarak mDNS) ve Prometheus için **Scrape belirteci (isteğe bağlı)** ile **/metrics'i yayınla**. Bkz. [Home Assistant](api.md#home-assistant) ve [BombVault'u ağda bulma](api.md#mdns).
- **Panel widget'ı**: iframe gösteren herhangi bir pano, örneğin Homepage ya da Heimdall, için salt okunur bir belirteç ve bir adres, yalnızca canlı etkinlik günlüğüyle. Bkz. [Panel widget'ı](api.md#dashboard-widget).
- **Ana makine SSH**: VM ve ZFS veri kümesi yedekleri ile Unraid bildirimleri için BombVault'un açık anahtarı ve **Bağlantıyı test et**. Bkz. [VM yedekleme kılavuzu](vm-backup-ssh-setup.md).

## Uygulamalar {#apps}

- **Android uygulaması**: sunucunun çalıştırdığı sürümün APK'sı, bir QR koduyla, ve F-Droid sayfası. Bkz. [Android uygulaması](android.md#install).
- **ParleyPort**: Unraid şablonu ve onu başlatan Docker komutuyla röle konteyneri.
- **BombVault Widget**: Unraid Panosunda BombVault'un etkinlik günlüğünü ve sonraki çalıştırmayı gösteren bir kutucuk. Ana makine SSH bağlantısı varsa kart eklentiyi kendisi kurar ve kaldırır. Yoksa **Plugins, Install Plugin** altında kurman için `.plg` adresini verir.

## Sistem {#system}

- **Ayarları dışa / içe aktar**: tüm yapılandırman taşınabilir bir JSON dosyası olarak, kimlik bilgileriyle ya da onlarsız. İçe aktarma önce **Bu dosya şunları içerir**'i gösterir ve artık var olmayan hedeflere olan bağlantıları dışarıda bırakır. **Dışa aktarmaları age ile şifrele**, düz dışa aktarmaları, flash ZIP'ini ve kurtarma kitini senin sıraladığın **age alıcıları** için mühürler. Bkz. [Taşınabilir ayarlar](configuration.md#portable-settings-export-and-import).
- **Performans**: **CPU iş parçacığı (0 = tümü)**, `/config` altındaki restic önbelleği için **Önbellek boyutu sınırı (MB, 0 = sınırsız)** ve büyük bir yedeğin bellek yetersizliğinden öldürülmek yerine yavaşlamasını sağlayan yumuşak bir sınır olan **Yedekleme başına bellek sınırı (MiB, 0 = yok)**.

## Bilgi {#info}

- **Sürüm**: BombVault, GlimStone, restic ve rclone, **Güncellemeleri denetle** ve henüz görmediğin değişiklikleri sayıp onları listeleyen bir pencere açan **Yeni** ile.
- **Yardım**: **Kılavuz** bu belgeleri açar, **Hata bildirimi** bir GitHub issue'su için temizlenmiş bir ZIP indirir. Giriş parolası gerekir.
- **BombVault hakkında**: proje, lisansı ve yazara nasıl ulaşılacağı.
