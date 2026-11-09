# Bildirimler

BombVault bir yedekleme başarısız olduğunda, başarılı olduğunda, geciktiğinde ve haftada bir de işlerin nasıl gittiğini sana bildirebilir. Her şey **Ayarlar, Bildirimler** altında ayarlanır: BombVault'un tek tek yedeklerin ötesinde ne gönderdiği **Bildirimler** kartında, mesajların nereye gideceği **Kanallar** kartında. Her kanal ne zaman haber vereceğine kendisi karar verir, bir Healthchecks denetimi de bir kanaldır.

![Gotify kanalı doldurulmuş Ayarlar, Bildirimler.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Gotify kanalı doldurulmuş Ayarlar, Bildirimler.](assets/screenshots/notifications/gotify-light.png#only-light)

## Neyin mesaj göndereceği {#what}

**Bildirimler** kartındaki anahtarlar tüm kanallar için geçerlidir:

- **Zamanlanmış çalıştırmaları özetle**: her konteyner ya da VM için ayrı mesaj yerine, zamanlanmış her çalıştırma için tek mesaj, örneğin *45'ten 42'si başarılı*. Elle başlattığın bir yedekleme yine kendi mesajını gönderir. Donmuş bir konuk, başarısız bir saklama ya da aşılmış bir büyüme bütçesi, özetler açıkken bile hemen gönderilir.
- **Container güncellemesinde bildir**: **Başarılı bir yedeklemeden sonra konteynerleri güncelle** seçeneğinin daha yeni bir image'e taşıdığı her konteyner için bir mesaj, böylece hâlâ çalıştığını kontrol edebilirsin.
- **Geciken yedekleri bildir**: günde bir kez 09:00'da BombVault açık bir yedeğin gecikip gecikmediğine, yani zamanlamasının iki katından eski olup olmadığına bakar ve her durum için, geride kalan ögeyi adlandıran bir mesaj gönderir. Sonraki başarılı yedek bunu temizler.
- **Haftalık özet**: çalıştırma sayıları, yeni yedek verisi, hangi bölümlerin geciktiği, site dışı kopyaların güncel olup olmadığı ve başlıca hatalarla haftada bir özet. Planı diğer planlar gibi bir düğmedir, **Kapalı** onu kapatır. Bkz. [Zamanlamalar](schedules.md#plans).

Bir kanalın başarılardan da haberdar olup olmayacağı kanalın kendisinde, **Bildir** altında ayarlanır. Hangi anormalliklerin mesaj göndereceği **Ayarlar, Doğrulama** altında ayarlanır, bkz. [Anormallikler](anomalies.md). Ertelenen bir yedek, beklemeye başlarken bir, bekleme biterken bir mesaj gönderir, her deneme için ayrı mesaj göndermez.

## Kanallar {#channels}

**Kanallar** kartının altındaki **Kanal ekle** şunları sunar: **Unraid bildirimleri**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-posta (SMTP)**, **Apprise**, **Webhook** ve **Healthchecks**. İstediğin kadar ekle, aynı türden iki tane de olur, örneğin hatalar için bir ntfy konusu ve her şey için bir başkası. Yeni bir kanal kapalı başlar, böylece alanları doldurulmadan önce kaydedilmiş olur.

Her kanalda şunlar var:

- kendi anahtarı: kanalı alanlarını kaybetmeden kapatır.
- **Bildir**: **Asla**, **Yalnızca başarısızlıkta** ya da **Başarı ve başarısızlıkta**. Yeni bir kanal **Yalnızca başarısızlıkta** ile başlar. Başarılı olan ama bir şeyi dışarıda bırakan bir yedek, yalnızca hataları bildiren bir kanala da ulaşır.
- **Ad**: isteğe bağlıdır ve türün yerine görünür, aynı türden iki kanalın varsa işine yarar.
- **Test gönder**: formda o an duran değerlerle, kaydedilmiş olsun ya da olmasın, bu kanal üzerinden bir test mesajı gönderir. Kanalın altındaki satır servisin mesajı kabul edip etmediğini, etmediyse nedenini söyler.
- **Kanalı kaldır**: yanındaki çöp kutusu.

Her kanal kendi zaman sınırıyla kendi başına gönderir, hiç yanıt vermeyen bir ana makine diğerlerini bekletmez. Bir servis yeni bir denemenin düzeltebileceği bir şekilde başarısız olursa kanal en fazla üç kez dener. Hiçbir kanalın almadığı bir gecikme, bütçe ya da anormallik uyarısı daha sonra yeniden denenir, haftalık özet de öyle.

| Kanal | Alanlar | Notlar |
|---|---|---|
| **Unraid bildirimleri** | yok | Pushover'a, e-postaya ya da Discord'a iletebilen Unraid'in kendi bildirim sistemine gönderir. **Ayarlar, Entegrasyonlar, Ana makine SSH** altındaki SSH bağlantısı üzerinden çalışır, bu yüzden o anahtarın yetkilendirilmiş olması gerekir. TrueNAS'ta ve diğer ana makinelerde sessiz kalır ve kanal nedenini söyler. |
| **ntfy** | **Konu URL'si**, **Erişim belirteci (isteğe bağlı)** | Adres konunla biter. Belirteç yalnızca korumalı bir konu için gerekir. |
| **Gotify** | **Sunucu URL'si**, **Uygulama belirteci**, **Başarısızlıkta öncelik**, **Başarıda öncelik** | Öncelikler 0 ile 10 arasındadır, varsayılan hatalar için 8, başarılar için 5'tir. |
| **Discord** | **Webhook URL'si** | Discord kanalının ayarlarındaki bir webhook'un adresi. |
| **Slack** | **Webhook URL'si** | Slack uygulamana ait bir incoming webhook'un adresi. |
| **Matrix** | **Homeserver URL'si**, **Erişim belirteci**, **Oda kimliği** | Belirtecin ait olduğu hesap odada olmalı. |
| **E-posta (SMTP)** | **SMTP sunucusu**, **Port**, **Şifreleme**, **Kullanıcı adı**, **Parola**, **Gönderen adresi**, **Alıcı adresi** | **Şifreleme** **STARTTLS**, **TLS (implicit)** ya da **None** olabilir. |
| **Apprise** | **Apprise API URL'si**, **Etiketler (isteğe bağlı)**, **Apprise adresleri (isteğe bağlı)** | 100'den fazla servisine ulaşmak için Apprise API konteynerinin `/notify/<key>` uç noktasını göster, etiketler de o anahtarın hedeflerinden bir alt kümeyi seçer. Ya da Apprise adreslerini her satıra bir tane yaz, örneğin Telegram için `tgram://` ya da Pushover için `pover://`, ve API URL'sini anahtar olmadan `/notify` ile bitir. |
| **Webhook** | **Webhook URL'si**, **Yöntem**, **Üst bilgiler**, **Gövde** | Kendin şekillendirdiğin bir istek, bkz. [Webhook](#webhook). |
| **Healthchecks** | **Ping URL'si** | Bkz. [Healthchecks](#healthchecks). |

Belirteçler, SMTP parolası ve webhook üst bilgilerinin değerleri gibi gizli bilgiler şifreli saklanır ve sunucu bunları sayfaya hiçbir zaman geri vermez. Kaydedilmiş bir gizli bilginin altında alan **Kaydedildi. Korumak için alanı boş bırak.** der. Bir kanalın nereye gönderdiğini değiştirdiğinde BombVault eski gizli bilgiyi yeni yere göndermez: alan **Yeni adres, bunu yeniden girdiğinde kaydedilir.** der ve yeni adres sen onu yazınca kaydedilir.

Kanal listesinden önceki bir sürümün ayarları ilk başlatmada aktarılır ve hiçbir şey kaybolmaz. Doldurduğun her kanal, daha önceki **Bildir** seçiminle ayrı bir girdi olur. Discord, Slack, Gotify ya da ntfy biçimine ayarlı bir webhook o türden bir kanala dönüşür, Generic JSON biçimindeki bir webhook ise **Gövde** üzerinden `title`, `message` ve `ok` göndermeye devam eder. Unraid anahtarı bir Unraid kanalına, Healthchecks URL'si de istisnalarıyla birlikte bir Healthchecks kanalına dönüşür.

### Webhook {#webhook}

Bir **Webhook** kanalı herhangi bir adrese tek bir HTTP isteği gönderir. **Yöntem** GET, POST ya da PUT olabilir. **Üst bilgi ekle**, **Üst bilgiler** altına bir satır ekler. Üst bilgi değerleri gizli bilgi olarak saklanır, bu yüzden bir `Authorization` üst bilgisi orada güvendedir. **Gövde** tam yazdığın gibi gönderilir, şu yer tutucular doldurularak:

| Yer tutucu | Neye dönüşür |
|---|---|
| `%%title%%` | mesajın başlığı |
| `%%body%%` | mesajın kendisi |
| `%%ok%%` | `true`, hata için `false` |
| `%%event%%` | ne olduğu, örneğin `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` ya da `critical` |
| `%%importance%%` | Unraid'deki karşılık gelen düzey: `normal`, `warning` ya da `alert` |
| `%%link%%` | mesaja ait bir bağlantı, yoksa boş |
| `%%key%%` | bölüm, örneğin `container` ya da `VM` |
| `%%time%%` | UTC olarak zaman, örneğin `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Yer tutucular adreste ve üst bilgi değerlerinde de çalışır ve her değer durduğu yere göre kaçışlanır: `Content-Type` üst bilgisi JSON diyorsa JSON olarak, adreste ise sorgu değeri olarak. Yeni bir webhook bu üst bilgiyle başlar ve `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}` gönderir. BombVault'un tanımadığı bir ad yazıldığı gibi kalır, böylece bir yazım hatası mesajda görünür. Gövde göndermemek için **Gövde**'yi boş bırak.

## Healthchecks {#healthchecks}

[Healthchecks.io](https://healthchecks.io) gibi bir servis, BombVault haber vermeyi bıraktığında alarm verir. Bir **Healthchecks** kanalı ekle ve denetimin **Ping URL'si**'ni yapıştır, BombVault her yedeğin tüm yaşam döngüsü boyunca ona ping atar: başlangıçta, sonra başarıda ya da `/fail` ile. Bu kanalda **Bildir** seçimi yoktur, çünkü bir denetimin yeşil kalması için başarı ping'lerine ihtiyacı vardır. Yedekleyecek hiçbir yeri olmayan bir bölüm başarısız sayılır, bu yüzden denetimi bir başarı duymak yerine kırmızıya döner.

Aynı URL'yi kullanan bölümler tek bir denetimi paylaşır. Birinin yedeği başarısız olduğunda, diğerlerinin iyi yedekleri de `/fail` ping'i atar ve onu adlandırır, böylece denetim o ögenin yeniden iyi bir yedeği olana ya da başarısızlığı **İlgi bekliyor** altında onaylayana kadar kırmızı kalır.

Bir bölüm kendi adresine ping atabilir: Healthchecks kanalındaki **İstisna** bir tane ekler, ardından satırda **Diğer tüm bölümler bu URL'ye ping atar.** yazar. **Test gönder** her denetimin günlüğüne bir kayıt yazar, denetimin durumu ise olduğu gibi kalır.

## Örnek: Telefonunda Gotify {#example-gotify}

`https://gotify.example.com` adresinde Gotify çalıştırıyorsun ve hataların telefonunda çalmasını, başarıların ise sessizce gelmesini istiyorsun.

1. Gotify'da **Uygulamalar**'ı aç, `BombVault` adında bir uygulama oluştur ve belirtecini kopyala.
2. BombVault'ta **Ayarlar, Bildirimler**'e git ve **Zamanlanmış çalıştırmaları özetle**'yi aç, böylece 45 yedeklik bir gece 45 yerine tek mesaj gönderir.
3. **Kanallar**'da **Kanal ekle**'ye bas ve **Gotify**'ı seç.
4. **Sunucu URL'si** olarak `https://gotify.example.com` gir. `/message` kısmını BombVault kendisi ekler.
5. Belirteci **Uygulama belirteci**'ne yapıştır.
6. **Bildir**'i **Başarı ve başarısızlıkta** yap. **Başarısızlıkta öncelik**'i 8, **Başarıda öncelik**'i 5 bırak. Gotify'ın Android uygulaması 8 ve üstünü yüksek sayar, bu yüzden yalnızca hatalar ses çıkarır.
7. Kanalı aç ve **Test gönder**'e bas. Kanalın altındaki satır Gotify'ın kabul edip etmediğini söyler, reddedilen bir belirteç de öyle adlandırılır.

Sonraki zamanlanmış çalıştırma 5 öncelikli tek bir özet gönderir, içinde bir şey başarısız olduysa 8 öncelikli.

## Örnek: Hesap olmadan ntfy {#example-ntfy}

Herkese açık bir konu için ntfy hesap istemez. Tahmin etmesi zor bir konu adı seç, çünkü onu bilen herkes okuyabilir.

1. **Kanallar**'da **Kanal ekle**'ye bas ve **ntfy** seçeneğini seç.
2. **Konu URL'si** olarak konunun adresini gir, örneğin `https://ntfy.sh/bombvault-tower-7f3k`, ya da kendi ntfy sunucundaki bir konunun adresini. Herkese açık bir konu için **Erişim belirteci (isteğe bağlı)** alanını boş bırak.
3. ntfy uygulamasında aynı konuya abone ol, kanalı aç ve **Test gönder**'e bas.

BombVault mesajı gövde olarak gönderir, başlığı `Title` üst bilgisinde taşır. Bir hata yüksek öncelikle ve `warning` etiketiyle gider, böylece uygulamada öne çıkar, donmuş kalan bir VM ise acil olarak gider.

## Mesajlar gelmediğinde {#troubleshooting}

Sorun Giderme içinde [Bildirimler gelmiyor](troubleshooting.md#notifications-do-not-arrive) bölümüne bak.
