# API ve entegrasyonlar

BombVault'un betikler, panolar ve ev otomasyonu için küçük bir HTTP API'si var. Panonun gösterdiklerini okur ve bir yedekleme başlatabilir. Geri yükleme, yedek silme ve ayarlar gibi her şey web arayüzünde kalır.

## Belirteçler {#tokens}

Her istek, giriş parolası olmasa bile bir API belirteci ister. Belirteci **Ayarlar, Sistem, API belirteçleri** altında oluştur:

1. Belirtecin nerede kullanıldığını söyleyen bir ad yaz, örneğin "Home Assistant" veya "Uptime Kuma".
2. Belirteç yedekleme başlatabilsin istiyorsan **Yedekleme başlatmaya izin ver** seçeneğini aç. Açık değilse yalnızca okuyabilir.
3. **Belirteç oluştur** düğmesine tıkla. Belirteç bir kez gösterilir. BombVault yalnızca parmak izini saklar, bu yüzden hemen kopyala.

Belirteci bir başlıkta gönder: `Authorization: Bearer <token>` ya da `X-API-Key: <token>`. Belirteç `bvapi_` ile başlar. Yalnızca API'yi açar: MCP anahtarı burada çalışmaz, belirteç de MCP için çalışmaz.

Her belirtecin bir kutucuğu vardır: adı, yedekleme başlatıp başlatamayacağı, son dört karakteri, en son ne zaman ve nereden kullanıldığı ve bugünkü çağrıları. Kutucukta adını değiştirebilir, izinlerini değiştirebilir, onu değiştirebilir veya iptal edebilirsin. **Günlük** başlattığı yedeklemeleri ve son çağrılarını gösterir. BombVault'un yapılandırmasını bir yedekten geri yüklemek tüm belirteçleri iptal eder, çünkü yedek sonradan iptal ettiğin belirteçleri içerebilir.

Giriş parolası yoksa web arayüzünü açabilen herkes belirteç de oluşturabilir. BombVault'u herkese açık görünen bir adla açarsan ve parola yoksa, o adresten belirteç oluşturulamaz; [MCP anahtarlarındaki](mcp.md#switch-on) kuralın aynısı.

## Uç noktalar {#endpoints}

| Yol | Ne döndürür veya ne yapar | Belirteç |
|---|---|---|
| `GET /api/v1/health` | Sürüm, örnek adı, yedekleme çalışıyor mu ve bu belirteç neler yapabilir | okuma |
| `GET /api/v1/status` | Alan başına koruma durumu: son başarılı yedek, beklenen aralık, denetimler, sonraki planlı çalışmalar | okuma |
| `GET /api/v1/activity` | Şu anda ne çalışıyor, aşama ve yüzdesiyle | okuma |
| `GET /api/v1/items` | Her korunan öğe, zamanlaması, bir yedeğin neyi durdurduğu ve son yedeğiyle; tek alan için `?domain=` | okuma |
| `GET /api/v1/runs` | Çalışma geçmişi, en yenisi önce; filtreler `limit`, `domain`, `item`, `status`, `kind`, `since` | okuma |
| `GET /api/v1/anomalies` | Açık olanların özetiyle anomaliler; filtreler `state`, `severity`, `domain`, `limit` | okuma |
| `GET /api/v1/anomalies/{id}` | Tek bir anomali | okuma |
| `GET /api/v1/storage/{domain}` | Bir alanın her deposu için boyut geçmişi, haftalık büyüme ve boş alan | okuma |
| `POST /api/v1/backups` | Tek bir öğeyi (`{"domain":"containers","item":"plex"}`) veya bütün bir alanı (`{"domain":"vms"}`) yedekler | başlatma |
| `POST /api/v1/backups/everything` | Backup Everything'i çalıştırır | başlatma |
| `POST /api/v1/runs/{id}/cancel` | Bu belirtecin başlattığı, süren bir yedeklemeyi iptal eder | başlatma |

Alanlar `containers`, `vms`, `files`, `zfs`, `flash` ve `config`. Zamanlar Unix saniyesidir. Yanıtlar aynı adlı [MCP araçlarınınkiyle](mcp.md#tools) aynıdır, böylece ikisi birlikte kalır.

Buradan başlatılan yedek, web arayüzünün başlattığıyla aynıdır: çalışan bir konteyner yedeği bitene kadar durdurulur. İstek hemen döner; `/api/v1/activity` ve `/api/v1/runs` nasıl gittiğini gösterir.

## Örnekler {#examples}

```sh
# Yedekler ne durumda?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Bir konteyneri şimdi yedekle.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

BombVault'un kendi imzaladığı sertifikayla `--cacert bombvault-cert.pem` ekle (dosyayı MCP kartındaki **Sertifikayı indir** verir) ya da güvendiğin bir ağda `-k` kullan.

## Hatalar ve sınırlar {#errors}

Hata, uygun durum koduyla `{"error": {"code": "...", "message": "..."}}` olarak döner:

| Durum | Kodlar | Anlamı |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Bir bağımsız değişken eksik veya yanlış |
| 401 | `no_token`, `invalid_token` | Belirteç yok ya da etkin değil |
| 403 | `not_permitted`, `forbidden_origin` | Belirteç yalnızca okuyabilir ya da çalışmayı o başlatmadı ya da istek başka bir kaynaktan (origin) gelen bir sayfadan geldi |
| 404 | `not_found` | Böyle bir öğe, çalışma veya anomali yok |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Başka bir şey çalışıyor, alan kapalı ya da yapılacak bir şey yok |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Bir sınır isteği bekletiyor; `Retry-After` ne zaman yeniden deneneceğini söyler |

Başlatmalar [MCP üzerinden başlatmalarla](mcp.md#starting-backups) aynı sınırlara tabidir: belirteç başına saatte 12, aynı öğenin iki başlatması arasında 15 dakika, bir öğe için 24 saatte en çok 4 başlatma ve saklama koruması. Son üçü MCP ve API üzerinden ve Home Assistant'tan başlatmaları birlikte sayar. Bir belirteç dakikada 120 istek yapabilir. Bir adresten beş başarısız deneme o adresi bir dakika kilitler.

## OpenAPI {#openapi}

BombVault bu yolların açıklamasını `/api/v1/openapi.json` adresinde sunar (OpenAPI 3.1). Belirteç gerekmez. Swagger UI, Postman veya bir kod üreticisine yükle.

## Home Assistant {#home-assistant}

BombVault, MQTT keşfi sayesinde Home Assistant'ta bir cihaz olarak görünebilir. Home Assistant'ın bunun için MQTT entegrasyonuna ve bir aracıya, örneğin Mosquitto eklentisine ihtiyacı vardır. Ayrı bir bileşen gerekmez.

1. BombVault'ta **Ayarlar, Sistem, Home Assistant** bölümünü aç.
2. Aracının adresini ve bağlantı noktasını, isterse kullanıcı adını ve parolayı gir. Aracı TLS kullanıyorsa, genellikle 8883 numaralı bağlantı noktasında, **TLS kullan** seçeneğini aç; sertifikası girdiğin adres için geçerli olmalı. Adresi, bağlantı noktasını ya da kullanıcı adını değiştirirsen parolayı yeniden gir: BombVault kayıtlı parolayı başka bir aracıya ya da kullanıcıya iletmez.
3. **Home Assistant'a bağlan** seçeneğini aç ve **Kaydet** düğmesine tıkla. Kart, bağlantı kurulduğunda bunu gösterir.

Cihazın adı BombVault'tur ya da parantez içinde örnek adıyla BombVault'tur ve şu varlıkları vardır:

| Varlık | Gösterdiği |
|---|---|
| Status | `ok`, `warning`, `failed` veya `off`, açık alanların en kötüsü |
| Running job | Şu anda ne çalışıyor, ya da `idle` |
| Open anomalies | Kaç anomali açık |
| Next scheduled backup | Sonraki planlı yedeğin ne zaman başladığı |
| *Alan* last backup | Alanın son başarılı yedeğinin ne zaman çalıştığı |
| *Alan* last result | Son yedeğinin nasıl bittiği |
| *Alan* repository free space | Ana deposunun bulunduğu yerdeki boş alan, BombVault okuyabiliyorsa |
| Back up *alan* | Tüm alanı yedekleyen bir düğme |

Varlık adları İngilizcedir, çünkü Home Assistant onları BombVault'un gönderdiği gibi alır. Açık her alan kendi varlıklarını alır, kapattığın bir alan onları kaybeder. Düğmeler, **Düğmeler yedekleme başlatır** seçeneğini açtığında görünür; yeni kurulumda bu kapalıdır. Düğmeler [API üzerinden başlatmalarla](#errors) aynı sınırlara tabidir. Bunun üstüne BombVault her alan için aynı anda tek bir basışı ve dakikada en fazla altı basışı kabul eder, aracının retained ileti olarak sakladığı bir basışı da yok sayar. Aracıya yayın yapabilen herkes onlara basabilir; bu yüzden aracıya bir parola ver.

BombVault durumunu her 15 saniyede bir okur ve bir şey değiştiğinde `<önek>/<düğüm>/state` altında JSON olarak yayımlar. Önek, sen değiştirmedikçe `bombvault`'tur; düğüm ise BombVault'un bir kez seçtiği kısa bir kimliktir. Keşif iletileri Home Assistant'ın varsayılan öneki `homeassistant`'a gider. İkisi de saklanır (retained). Son vasiyet (last will), BombVault haber vermeden durursa cihazı kullanılamaz olarak işaretler. Bağlantıyı kapatırsan BombVault cihazı ve varlıklarını Home Assistant'tan kaldırır.

## BombVault'u ağda bulmak {#mdns}

BombVault, web arayüzünü yerel ağda Bonjour ve Avahi'nin ardındaki protokol olan mDNS ile duyurur. Böylece bir tarayıcı ona `https://bombvault.local:3443` adresinden, `HTTP_ONLY` ile `http://bombvault.local:3000` adresinden ulaşır ve hizmet tarayıcıları onu `_bombvault` alt türüyle bir web hizmeti olarak listeler. TXT kayıtları sürümü ve yolu taşır. Anahtar **Ayarlar, Sistem, Ağda bul** altındadır ve varsayılan olarak açıktır. Ad başka bir cihaz tarafından kullanılıyorsa BombVault `bombvault-2.local` gibi bir sonraki adı alır ve kart aldığı adresi gösterir. BombVault durduğunda ya da duyuruyu kapattığında bunu ağa bildirir, tarayıcılar da kaydı hemen kaldırır.

Duyurunun ağına ulaşıp ulaşmadığı konteynerin nasıl bağlandığına bağlıdır:

- **bridge**, Unraid şablonundaki varsayılan: duyuru Docker ağının içinde kalır ve yerel ağda kimse görmez. BombVault'u eskisi gibi ana makinenin adresinden aç.
- **br0** ya da başka bir macvlan veya ipvlan ağı: konteynerin yerel ağda kendi adresi vardır ve duyuru oraya ulaşır.
- **host**: duyuru, Unraid'in kendi duyurusunun yanında ana makinenin arayüzlerinden çıkar. Docker ve libvirt köprüleri dışarıda kalır.

Yalnızca IPv4 adresleri duyurulur.
