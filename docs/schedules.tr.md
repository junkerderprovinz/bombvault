# Zamanlamalar

**Ayarlar, Zamanlama** tüm planları barındırır: her şeyi kapsayan tam yedekleme, bir bölümün kendi planı, site dışı kopyalar, hangi ögelerin katıldığı ve hangi sırayla. Tek bir öge kendi sayfasında kendi planını alabilir. Bu sayfa bunların nasıl birbirine oturduğunu açıklar ve tipik bir kurulumu adım adım gösterir.

## Planlar cümle gibi okunur {#plans}

Her plan, cümle gibi okunan bir düğmedir, örneğin *her gün 03:00'te* ya da *2 günde bir 01:30'da*. Tıklamak onu beş kiple açar:

| Kip | Ne ayarlarsın |
|---|---|
| **Kapalı** | Bu planda hiçbir şey çalışmaz. |
| **Günlük** | **Saat** |
| **Haftalık** | **Günler** ve **Saat** |
| **Her N günde bir** | **Her** şu kadar **gün** ve **Saat**. Son zamanlanmış çalıştırmadan sayar, yani bir ögeyi elle yedeklemek planı geri itmez. |
| **Cron** | Beş alanlı bir **İfade**, örneğin 6 saatte bir için `0 */6 * * *`. **Örnekler** birkaç tane sunar, pencere de sonraki çalıştırmaları gösterir. |

Tüm saatler sunucunun saat dilimindedir. Unraid'de bu sistemin kendi saat dilimidir. Başka yerlerde `TZ` ayarla, yoksa her plan UTC'de çalışır, bkz. [Ortam değişkenleri](configuration.md#environment-variables).

## Hafta {#week}

**Hafta** kartı planlanmış her çalıştırmayı Pazartesi'den Pazar'a, gün başına bir satır olarak çizer: yedekler, site dışı kopyalar ve denetimler. Neyin orada başladığını okumak için bir çizgiye gel. İki saat içinde üç planın başladığı yerde aralık işaretlenir, böylece onları açabilirsin.

![Ayarlar, Zamanlama: üstte Hafta şeridi, ardından tam yedekleme ve bölüm başına planlar.](assets/screenshots/schedule/week-dark.png#only-dark)
![Ayarlar, Zamanlama: üstte Hafta şeridi, ardından tam yedekleme ve bölüm başına planlar.](assets/screenshots/schedule/week-light.png#only-light)

## Tam yedekleme {#full-backup}

**Tam yedekleme** her şey için varsayılan plandır. Onu **Yedeklemeler ne zaman çalışır** ayarlar: her bölümü sırayla yedekleyen tek bir geçiş, konteynerler, VM'ler, flash, klasörler, ZFS veri kümeleri ve en son öz yedek. Altındaki satır hangi bölümleri içerdiğini söyler. **Kapalı** onu kapatır, o zaman yalnızca kendi planı olanlar çalışır.

- **Başlangıçtan sonra kaçırılan yedeklemeleri telafi et**: sunucu planlanan zamanda kapalıysa BombVault o yedeği açıldıktan yaklaşık iki dakika sonra çalıştırır.
- **Öncesi ve sonrası komutları**: **Tam yedekleme öncesi komut** geçiş başlamadan önce sunucuda `sh -c` ile çalışır, örneğin bir servisi durduran bir betik. Başarısız olursa tam yedekleme yine de çalışır. **Tam yedekleme sonrası komut** her bölüm denendikten sonra, biri başarısız olsa bile, bir kez çalışır, bu, tüm sunucunun korunduğunu doğrulayan bir ping için uygundur. Tek bir konteynerin komutları kendi sayfasındadır.

Yedekler sayfasındaki **Şimdi tümünü yedekle**, aynı geçişi elle, açık olan tüm bölümler üzerinde başlatır.

## Bölüm başına planlar {#plans-per-section}

Bir bölüm tam yedeklemeden ayrılıp kendi saatinde çalışabilir. **Bölüm başına zamanlamalar** içinde **İstisna** kendi planı olan bir bölüm ekler, **İstisnayı kaldır** onu geri gönderir. Karttaki satır diğerlerinin ne yaptığını söyler, örneğin **Diğer tüm bölümler tam yedeklemede çalışır.**

Kendi planı olan bir bölüm tam yedeklemede yeniden yedeklenmez, yani hiçbir şey iki kez yedeklenmez. Aynısı kendi planı olan tek bir öge için de geçerlidir.

**Bölüm başına site dışı kopyalar** kopyalar için aynı şekilde çalışır. Varsayılan olarak her bölüm, her yedeklemeden hemen sonra site dışı hedeflerine kopyalar. **İstisna** bir bölüme kendi kopyalama planını verir, örneğin VM'lerin Backblaze B2'ye yalnızca Pazar geceleri kopyalanması için. Bir hedefin kendi **Kopyala** kartı da **Ne zaman kopyalanır**'ı ayarlayabilir, bkz. [Site dışı kopyalar](offsite-recovery.md#off-site-copies).

## Zamanlanmış ögeler {#scheduled-entries}

**Zamanlanmış ögeler**, her bölüm için kaç ögenin katıldığını söyler, örneğin **41'den 38'i zamanlanmış**. **Ögeler**, her öge için bir **Zamanlamada** anahtarı olan listeyi açar. Kapalı olan bir öge yalnızca sen elle başlattığında yedeklenir. **Yedeklenmesin** olarak işaretlemediysen korunmuyor sayılır.

## Sıra {#order}

**Sıra**, zamanlanmış çalıştırmaların ve tam yedeklemenin konteynerleri ve VM'leri hangi sırayla yedekleyeceğini belirler. Bir ögeyi yerine sürükle ya da oklarla taşı, her hareket hemen kaydedilir. Sıralamadığın ögeler en çok gecikmiş olandan başlayarak yedeklenir. Elle alınan tek bir yedek bundan etkilenmez.

Aynı karttaki **Yedeklemeden sonra yeniden başlatma**, bir yedekleme sırasında **Diğer konteynerleri durdur**'un kapattığı konteynerleri kapsar. Bağımlılık sırasına göre yeniden başlarlar ve **Sonrakini başlatmadan önce bağımlılıkların sağlıklı olmasını bekle** ile her biri, sonraki başlamadan önce sağlıklı ya da çalışıyor bildirmelidir. **Konteyner başına sağlık zaman aşımı (saniye)** beklemeyi sınırlar, varsayılan 120, 5 ile 3600 arasında.

## Durdurma yerine anlık görüntü {#snapshot-instead-of-stop}

**ZFS üzerindeki kapsayıcıları yalnızca anlık görüntü için durdur** varsayılan olarak açıktır. Verisi ZFS üzerinde olan bir konteyner yalnızca BombVault o verinin anlık görüntüsünü alırken, genellikle birkaç saniye, durdurulur, sonra yedek anlık görüntüden okunur. Bunun için ZFS veri kümelerinin kullandığı, ana makineye giden SSH bağlantısı gerekir. Verisinin bir kısmı başka yerde olan bir konteyner eskisi gibi tüm yedekleme boyunca durdurulur ve çalıştırması nedenini söyler, verisinin hiçbiri ZFS'te olmayan bir konteyner not düşülmeden durdurulur. Tek bir ZFS havuzunda duran bir Unraid kullanıcı paylaşımında çalışır. TrueNAS'ta uygulamanın ayrıca `SYS_CHROOT` yeteneğine ihtiyacı vardır, bkz. [TrueNAS SCALE](getting-started.md#truenas). Tek bir konteyner kendi sayfasında farklı ayarlanabilir.

Yedek, konteyner durdurulmuşken alınanla aynı yollara ve etiketlere düşer, bu yüzden geri yüklemeler ve denetimler ikisine de aynı davranır. Yedek öncesi komut ve veritabanı dökümü yine ilk çalışır, durdurulacak diğer konteynerler onunla birlikte kapanıp açılır ve yedek sonrası komut konteyner tekrar ayağa kalkar kalkmaz çalışır. Yedeklemeden sonraki güncelleme yine yedeğin bitmesini bekler. Çalıştırmalarında **Anlık görüntü** ve konteynerin kaç saniye kapalı kaldığı görünür. BombVault herhangi bir konteyner yedeği sırasında ortada durursa, bir sonraki açılışında kapalı tuttuğu konteynerleri yeniden başlatır.

## Yedeklemeden sonra güncellemeler {#updates}

- **Başarılı bir yedeklemeden sonra konteynerleri güncelle**, yeni bir image'i yalnızca öncesindeki yedek bittikten sonra çeker, böylece taze bir geri yükleme noktası her zaman önce vardır. Varsayılan olarak kapalıdır, tek bir konteyner kendi sayfasında **Başarılı bir yedeklemeden sonra güncelle** altında farklı ayarlanabilir.
- **Güncellemeden sonra eski image'i kaldır**, konteynerin geride bıraktığı image'i siler. Başka konteynerlerin paylaştığı bir temel image hiçbir zaman silinmez.
- **Bir konteyneri güncelledikten sonra Unraid'in güncelleme durumunu yenile**, Unraid'in o konteyner için güncelleme bannerını temizler.

Güncellenen her konteyner için bir mesaj, **Ayarlar, Bildirimler** altında **Container güncellemesinde bildir** ile ayarlanır.

## Yedek öncesi boşta kalma {#idle}

**Yedek öncesi boşta kalma** kartı, zamanlanmış yedeği onu bekleyen bir konteyner için bir uygulamanın ne zaman boşta sayılacağını belirler: **CPU şunun altında (bir çekirdeğin %)**, **Trafik şunun altında (Mbit/s)** ve **En az (dk)**. Bir medya sunucusu yayın yapmadığında boşta sayılır, ana makine ağındaki bir konteyner için ise yalnızca CPU sayılır. Bekleme her konteynerde ayarlanır, bkz. [Uygulama meşgulken](backups.md#app-busy).

## Tek bir öge için kendi plan {#own-plan}

Her ögenin Ayarlar sekmesindeki **Zamanlama ve hedef** kartında **Kendi planı** vardır. Orada bir plan varsa öge kendi saatlerinde çalışır ve tam yedeklemeyi ve bölümünün planını bırakır. Yoksa onları izler.

## Örnek: birbirine uyan üç plan {#example}

Tipik bir Unraid makinesi: konteynerlerin çoğu geceleri yedeklenebilir, VM'ler büyüktür ve yavaş değişir, Immich ise gün boyu yeni fotoğraf alır.

1. **03:00'te tam yedekleme.** **Ayarlar, Zamanlama**'yı aç. **Tam yedekleme** içinde **Yedeklemeler ne zaman çalışır**'ı `03:00`'te **Günlük** yap. Sunucunun kapalı olduğu bir gece telafi edilsin diye **Başlangıçtan sonra kaçırılan yedeklemeleri telafi et**'i aç.
2. **VM'ler haftada bir.** **Bölüm başına zamanlamalar** içinde **İstisna**'ya bas, **VM'ler**'i seç ve planını **Haftalık**, **Günler** Pazar, **Saat** `04:00` yap. Kart artık diğer tüm bölümlerin tam yedeklemede çalıştığını söyler.
3. **VM'ler Pazar günü site dışına da kopyalanır.** **Bölüm başına site dışı kopyalar** içinde **İstisna**'ya bas, **VM'ler**'i seç ve yedekleri bitecek zaman bulsun diye **Haftalık**, Pazar `06:00` yap.
4. **Immich 6 saatte bir.** **Yedekler**'i, sonra `immich`'i, sonra **Ayarlar** sekmesini aç. **Kendi planı**'nı `0 */6 * * *` ile **Cron** yap.

Şimdi ne çalışıyor:

| Ne zaman | Ne |
|---|---|
| Her gün 03:00 | `immich` dışındaki konteynerler, flash, klasörler, ZFS veri kümeleri ve öz yedek, bölüm bölüm |
| Pazar 04:00 | VM'ler |
| Pazar 06:00 | VM'lerin site dışı kopyası |
| 00:00, 06:00, 12:00 ve 18:00 | `immich` kendi başına |

**Hafta** kartı her güne 03:00 satırını, Pazar günü 04:00 ve 06:00 çizgilerini ve `immich` için günde dört çizgiyi gösterir. `immich` 03:00'te ikinci kez yedeklenmez, çünkü kendi planı olan bir öge tam yedeklemeyi bırakır. Diğer bölümlerin kopyaları yine her yedeklemenin hemen ardından çalışır.

Sonra `immich`'in kendi planını kaldırırsan, bölümüyle birlikte yeniden 03:00 çalıştırmasına katılır.
