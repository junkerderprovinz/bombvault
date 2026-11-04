# Bildirimler

BombVault bir yedekleme başarısız olduğunda, başarılı olduğunda, geciktiğinde ve haftada bir de işlerin nasıl gittiğini sana bildirebilir. Her şey **Ayarlar, Bildirimler** altında ayarlanır: ne zaman mesaj gideceği **Bildirimler** kartında, mesajların nereye gideceği **Kanallar** kartında, check-in pingleri ise **Healthchecks** kartında.

![Gotify kanalı doldurulmuş Ayarlar, Bildirimler.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Gotify kanalı doldurulmuş Ayarlar, Bildirimler.](assets/screenshots/notifications/gotify-light.png#only-light)

## Neyin mesaj göndereceği {#what}

**Bildirimler** kartı tüm kanallar için kuralları bir arada belirler:

- **Bildir**: aşağıdaki tüm kanallar için **Hiç**, **Yalnızca başarısızlıkta** ya da **Başarı ve başarısızlıkta**.
- **Zamanlanmış çalıştırmaları özetle**: her konteyner ya da VM için ayrı mesaj yerine, zamanlanmış her çalıştırma için tek mesaj, örneğin *45'ten 42'si başarılı*. Elle başlattığın bir yedekleme yine kendi mesajını gönderir. Donmuş bir konuk, başarısız bir saklama ya da aşılmış bir büyüme bütçesi, özetler açıkken bile hemen gönderilir.
- **Container güncellemesinde bildir**: **Başarılı bir yedeklemeden sonra konteynerleri güncelle** seçeneğinin daha yeni bir image'e taşıdığı her konteyner için bir mesaj, böylece hâlâ çalıştığını kontrol edebilirsin.
- **Geciken yedekleri bildir**: günde bir kez 09:00'da BombVault açık bir yedeğin gecikip gecikmediğine, yani zamanlamasının iki katından eski olup olmadığına bakar ve her durum için bir mesaj gönderir. Sonraki başarılı yedek bunu temizler.
- **Unraid bildirimleri**: Pushover'a, e-postaya ya da Discord'a iletebilen Unraid'in kendi bildirim sistemine gönderir. **Ayarlar, Entegrasyonlar, Ana makine SSH** altındaki SSH bağlantısı üzerinden çalışır, bu yüzden o anahtarın yetkilendirilmiş olması gerekir. TrueNAS'ta ve diğer ana makinelerde kapalı kalır.
- **Haftalık özet**: çalıştırma sayıları, yeni yedek verisi, site dışı kopyaların güncel olup olmadığı ve başlıca hatalarla haftada bir özet. Planı diğer planlar gibi bir düğmedir, **Kapalı** onu kapatır. Bkz. [Zamanlamalar](schedules.md#plans).

Hangi anormalliklerin mesaj göndereceği **Ayarlar, Doğrulama** altında ayarlanır, bkz. [Anormallikler](anomalies.md). Ertelenen bir yedek, beklemeye başlarken bir, bekleme biterken bir mesaj gönderir, her deneme için ayrı mesaj göndermez.

Kartın altındaki **Test gönder**, açık olan her kanaldan, formda o an duran değerlerle bir test mesajı gönderir.

## Kanallar {#channels}

**Kanallar** kartında her kanalın kendi anahtarı vardır ve alanları açıldıktan sonra görünür. Her kanal kendi zaman sınırıyla kendi başına gönderir, hiç yanıt vermeyen bir ana makine diğerlerini bekletmez ve hiçbir kanalın almadığı mesaj yeniden denenir.

| Kanal | Alanlar | Notlar |
|---|---|---|
| **Webhook** | **Webhook URL'si**, **Webhook biçimi** | Biçimler: **Generic JSON**, **Discord**, **Slack**, **Gotify** ve **ntfy**. Generic JSON `title`, `message` ve `ok` gönderir. |
| **Apprise** | **Apprise API URL'si**, **Etiketler (isteğe bağlı)** | 100'den fazla servisine ulaşmak için Apprise API konteynerinin `/notify/<key>` uç noktasını göster. Etiketler o anahtarın hedeflerinden bir alt kümeyi seçer. |
| **Matrix** | **Homeserver URL'si**, **Erişim belirteci**, **Oda kimliği** | Belirteç sunucuda kalır ve bir daha gösterilmez. |
| **Gotify** | **Sunucu URL'si**, **Uygulama belirteci**, **Başarısızlıkta öncelik**, **Başarıda öncelik** | Öncelikler 0 ile 10 arasındadır, varsayılan hatalar için 8, başarılar için 5'tir. |
| **E-posta (SMTP)** | **SMTP sunucusu**, **Port**, **Şifreleme**, **Kullanıcı adı**, **Parola**, **Gönderen adresi**, **Alıcı adresi** | **Şifreleme** **STARTTLS**, **TLS (implicit)** ya da **Yok** olabilir. |

Matrix belirteci, Gotify uygulama belirteci ve SMTP parolası gibi gizli bilgiler şifreli saklanır. Kaydedilmiş bir gizli bilginin altında alan **Kaydedildi. Korumak için alanı boş bırak.** der. Sunucunun adresini değiştirdiğinde BombVault eski gizli bilgiyi yeni yere göndermez: alan yeniden ister ve yeni adres yazdıktan sonra kaydedilir.

## Healthchecks {#healthchecks}

[Healthchecks.io](https://healthchecks.io) gibi bir servis, BombVault haber vermeyi bıraktığında alarm verir. Denetimin **Ping URL'si**'ni yapıştır, BombVault her yedeğin tüm yaşam döngüsü boyunca ona ping atar: başlangıçta, sonra başarıda ya da `/fail` ile. Bir URL ayarlı olduğu sürece, **Bildir** ne derse desin bunu yapar, böylece yalnızca başarısızlık mesajlarıyla bile denetim başarıda yeşil kalır.

Bir bölüm kendi adresine ping atabilir: karttaki **İstisna** bir tane ekler, satırda **Diğer tüm bölümler bu URL'ye ping atar.** yazar.

## Örnek: Telefonunda Gotify {#example-gotify}

`https://gotify.example.com` adresinde Gotify çalıştırıyorsun ve hataların telefonunda çalmasını, başarıların ise sessizce gelmesini istiyorsun.

1. Gotify'da **Uygulamalar**'ı aç, `BombVault` adında bir uygulama oluştur ve belirtecini kopyala.
2. BombVault'ta **Ayarlar, Bildirimler**'i aç. **Bildirimler** kartında **Bildir**'i **Başarı ve başarısızlıkta** yap ve **Zamanlanmış çalıştırmaları özetle**'yi aç, böylece 45 yedeklik bir gece 45 yerine tek mesaj gönderir.
3. **Kanallar**'da **Gotify**'ı aç.
4. **Sunucu URL'si** olarak `https://gotify.example.com` gir. `/message` kısmını BombVault kendisi ekler.
5. Belirteci **Uygulama belirteci**'ne yapıştır.
6. **Başarısızlıkta öncelik**'i 8, **Başarıda öncelik**'i 5 bırak. Gotify'ın Android uygulaması 8 ve üstünü yüksek sayar, bu yüzden yalnızca hatalar ses çıkarır.
7. **Test gönder**'e bas. Kartın altındaki satır Gotify'ın kabul edip etmediğini söyler, reddedilen bir belirteç de öyle adlandırılır.

Sonraki zamanlanmış çalıştırma 5 öncelikli tek bir özet gönderir, içinde bir şey başarısız olduysa 8 öncelikli.

## Örnek: Webhook olarak ntfy {#example-ntfy}

Herkese açık bir konu için ntfy hesap istemez. Tahmin etmesi zor bir konu adı seç, çünkü onu bilen herkes okuyabilir.

1. **Kanallar**'da **Webhook**'u aç.
2. **Webhook URL'si** olarak konunun adresini gir, örneğin `https://ntfy.sh/bombvault-tower-7f3k`, ya da kendi ntfy sunucunun adresini.
3. **Webhook biçimi**'ni **ntfy** yap.
4. ntfy uygulamasında aynı konuya abone ol ve **Test gönder**'e bas.

BombVault mesajı gövde olarak gönderir, başlığı `X-Title` içinde taşır. Bir hata yüksek öncelikle ve `warning` etiketiyle gider, böylece uygulamada öne çıkar.

## Mesajlar gelmediğinde {#troubleshooting}

Sorun Giderme içinde [Bildirimler gelmiyor](troubleshooting.md#notifications-do-not-arrive) bölümüne bak.
