# API ve entegrasyonlar

BombVault'un betikler, panolar ve ev otomasyonu için küçük bir HTTP API'si vardır. Genel bakışın gösterdiği şeyleri okur ve bir yedekleme başlatabilir. Geri yüklemeler, yedekleri silme ve ayarlar gibi geri kalan her şey web arayüzünde kalır.

## Belirteçler {#tokens}

Giriş parolası ayarlı olmasa bile her istek bir API belirteci ister. **Ayarlar, Entegrasyonlar, API belirteçleri** altında bir tane oluştur:

1. Belirtecin nerede kullanıldığını söyleyen bir ad yaz, örneğin "Home Assistant" ya da "Uptime Kuma".
2. Belirteç yedekleme başlatabilsin istiyorsan **Yedekleme başlatmaya izin ver**'i aç. Onsuz belirteç yalnızca okuyabilir.
3. **Belirteç oluştur**'a tıkla. Belirteç bir kez gösterilir. BombVault yalnızca parmak izini tutar, bu yüzden şimdi kopyala.

Belirteci bir başlıkta gönder: `Authorization: Bearer <token>` ya da `X-API-Key: <token>`. Bir belirteç `bvapi_` ile başlar. Yalnızca API'yi açar: bir MCP anahtarı burada çalışmaz, bir belirteç de MCP için çalışmaz.

Her belirtecin adı, yedekleme başlatıp başlatamadığı, son dört karakteri, en son ne zaman ve nereden kullanıldığı ve bugünkü çağrıları olan bir kutucuğu vardır. Kutucukta onu yeniden adlandırabilir, yapabildiklerini değiştirebilir, değiştirebilir ya da iptal edebilirsin. **Günlük**, başlattığı yedekleri ve son çağrılarını gösterir. BombVault yapılandırmasını bir yedekten geri yüklemek her belirteci iptal eder, çünkü yedek, sonradan iptal ettiğin belirteçleri içerebilir.

Giriş parolası olmadan web arayüzünü açabilen herkes bir belirteç de oluşturabilir. BombVault'u herkese açık gibi görünen bir adla açarsan ve parola ayarlı değilse, o adresten belirteç oluşturulamaz, [MCP anahtarları](mcp.md#switch-on) için de aynı kural geçerlidir.

## Uç noktalar {#endpoints}

| Rota | Ne döndürür ya da yapar | Belirteç |
|---|---|---|
| `GET /api/v1/health` | Sürüm, örnek adı, bir yedeğin çalışıp çalışmadığı ve bu belirtecin neler yapabileceği | okuma |
| `GET /api/v1/status` | Bölüm başına koruma durumu: son başarılı yedek, beklenen aralık, denetimler, sonraki zamanlanmış çalıştırmalar | okuma |
| `GET /api/v1/activity` | Şu an ne çalışıyor, aşama ve yüzdeyle | okuma |
| `GET /api/v1/items` | Zamanlaması, bir yedeğin neyi durdurduğu ve son yedeğiyle her korunan öge. Tek bir bölüm için `?domain=` | okuma |
| `GET /api/v1/runs` | Çalıştırma geçmişi, en yeni önce. `limit`, `domain`, `item`, `status`, `kind`, `since` filtreleri | okuma |
| `GET /api/v1/anomalies` | Açık olanların özetiyle anormallikler. `state`, `severity`, `domain`, `limit` filtreleri | okuma |
| `GET /api/v1/anomalies/{id}` | Tek bir anormallik | okuma |
| `GET /api/v1/storage/{domain}` | Bir bölümün her deposunun boyut geçmişi, haftalık büyümesi ve boş alanı | okuma |
| `POST /api/v1/backups` | Tek bir ögeyi (`{"domain":"containers","item":"plex"}`) ya da bir bölümün tamamını (`{"domain":"vms"}`) yedekler | başlatma |
| `POST /api/v1/backups/everything` | Şimdi tümünü yedekle geçişini çalıştırır | başlatma |
| `POST /api/v1/runs/{id}/cancel` | Bu belirtecin başlattığı çalışan bir yedeği iptal eder | başlatma |

Bölümler `containers`, `vms`, `files`, `zfs`, `flash` ve `config`'tir. Zamanlar unix saniyesidir. Yanıtlar aynı adlı [MCP araçlarının](mcp.md#tools) yanıtlarıyla aynıdır, böylece ikisi de uyumlu kalır.

Başlatılan bir yedek, web arayüzünün başlattığıyla aynı yedektir: çalışan bir konteyner yedeği bitene kadar durdurulur. İstek hemen döner, `/api/v1/activity` ve `/api/v1/runs` işin nasıl gittiğini gösterir.

## Örnekler {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

BombVault'un kendi kendine imzalı sertifikasıyla `--cacert bombvault-cert.pem` (MCP kartındaki **Sertifikayı indir**'in verdiği dosya) ya da güvendiğin bir ağda `-k` ekle.

## Hatalar ve sınırlar {#errors}

Bir hata, uygun bir durumla `{"error": {"code": "...", "message": "..."}}` olarak döner:

| Durum | Kodlar | Anlamı |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Bir argüman eksik ya da yanlış |
| 401 | `no_token`, `invalid_token` | Belirteç yok ya da etkin değil |
| 403 | `not_permitted`, `forbidden_origin` | Belirteç yalnızca okuyabilir, çalıştırmayı o başlatmadı ya da istek başka bir kaynaktaki (origin) bir sayfadan geldi |
| 404 | `not_found` | Böyle bir öge, çalıştırma ya da anormallik yok |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Başka bir şey çalışıyor, bölüm kapalı ya da yapılacak bir şey yok |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Bir sınır isteği tutuyor. `Retry-After` ne zaman yeniden deneyeceğini söyler |

Başlatmalar [MCP üzerinden başlatmalar](mcp.md#starting-backups) ile aynı sınırlara tabidir: belirteç başına saatte 12, aynı ögenin iki başlatması arasında 15 dakika, 24 saatte aynı ögenin en fazla 4 başlatması ve saklama koruması. Son üçü MCP, API ve Home Assistant üzerinden başlatmaları birlikte sayar. Bir belirteç dakikada 120 istek yapabilir. Tek bir adresten beş başarısız deneme onu bir dakikalığına kilitler.

## OpenAPI {#openapi}

BombVault bu rotaların bir tanımını `/api/v1/openapi.json` adresinde sunar (OpenAPI 3.1). Belirteç gerekmez. Swagger UI'ye, Postman'a ya da bir kod üreticisine yükle.

## Home Assistant {#home-assistant}

BombVault, MQTT keşfi aracılığıyla Home Assistant'ta bir cihaz olarak görünebilir. Home Assistant'ın MQTT entegrasyonuna ve bir broker'a ihtiyacı var, örneğin Mosquitto eklentisi. Özel bir bileşen gerekmez.

1. BombVault'ta **Ayarlar, Entegrasyonlar**'ı aç ve **Hizmetler** kartında **Home Assistant**'ı aç. Broker alanları altında görünür.
2. Broker'ın adresini ve portunu, istiyorsa kullanıcı adını ve parolasını gir. Broker TLS alıyorsa, genellikle 8883 portunda, **TLS kullan**'ı aç. Sertifikası girdiğin adres için geçerli olmalı. Adresi, portu ya da kullanıcı adını değiştirirsen parolayı yeniden gir: BombVault kayıtlı parolayı başka bir broker'a ya da kullanıcıya iletmez.
3. **Kaydet**'e tıkla. Satır bağlantı kurulduğunda bunu gösterir.

Cihaz BombVault ya da örnek adı parantez içinde olan BombVault olarak adlandırılır ve şu varlıklara sahiptir:

| Varlık | Ne gösterir |
|---|---|
| Status | `ok`, `warning`, `failed` ya da `off`, açık bölümlerin en kötüsü |
| Running job | Şu an ne çalışıyor, ya da `idle` |
| Open anomalies | Kaç anormallik açık |
| Next scheduled backup | Sonraki zamanlanmış yedeğin ne zaman başlayacağı |
| *Bölüm* last backup | Bölümün son başarılı yedeğinin ne zaman çalıştığı |
| *Bölüm* last result | Son yedeğinin nasıl bittiği |
| *Bölüm* repository free space | BombVault okuyabiliyorsa, birincil deposunun bulunduğu yerdeki boş alan |
| Back up *bölüm* | Tüm bölümü yedekleyen bir düğme |

Açık olan her bölüm kendi varlıklarını alır, kapattığın bir bölüm onları kaybeder. Düğmeler **Düğmeler yedekleme başlatır**'ı açtığında görünür. Yeni bir kurulumda bu kapalıdır. [API üzerinden başlatmalar](#errors) ile aynı sınırlara tabidirler. Bunun üstüne BombVault bölüm başına aynı anda bir basışı ele alır, dakikada altıdan fazlasını almaz ve broker'ın retained mesaj olarak sakladığı bir basışı yok sayar. Broker'a yayın yapabilen herkes onlara basabilir, bu yüzden broker'a bir parola ver.

BombVault durumunu 15 saniyede bir okur ve bir şey değiştiğinde `<prefix>/<node>/state` altında JSON olarak yayınlar. Öneki değiştirmezsen `bombvault`'tur, düğüm ise BombVault'un bir kez seçtiği kısa bir kimliktir. Keşif mesajları Home Assistant'ın varsayılan öneki `homeassistant`'a gider. İkisi de retained'dir. Bir last will, BombVault haber vermeden durursa cihazı kullanılamaz işaretler. Bağlantıyı kapatmak cihazı ve varlıklarını Home Assistant'tan kaldırır.

### Örnek: bir kart ve bir uyarı {#home-assistant-example}

Home Assistant varlık kimliklerini adlardan üretir, bu yüzden BombVault cihazı `sensor.bombvault_status` gibi kimlikler alır. Parantez içinde bir örnek adı varsa kimlikler onu da taşır, bu yüzden önce **Developer Tools, States** altında kontrol et. Pano için bir kart:

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

Durum `failed` olduğunda telefonuna haber veren bir otomasyon:

```yaml
alias: BombVault backup failed
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to: failed
action:
  - service: notify.mobile_app_my_phone
    data:
      message: A BombVault backup failed. Open Needs you on the overview.
```

## Prometheus {#metrics}

**Ayarlar, Entegrasyonlar** altındaki **Hizmetler** kartında **/metrics'i yayınla**, yedekleme durumunu, boyutları ve zaman damgalarını Prometheus, Grafana ya da Uptime Kuma için web arayüzünün portundaki `/metrics` adresinde sunar. Varsayılan olarak kapalıdır. **Scrape belirteci (isteğe bağlı)**, toplayıcının bir bearer belirteci göndermesini sağlar. Etiketlerde gizli bilgi ve yol yoktur.

## Panel widget'ı {#dashboard-widget}

**Ayarlar, Entegrasyonlar** altındaki **Panel widget'ı** kartı salt okunur bir belirteç üretir ve Homepage, Organizr ya da Heimdall gibi iframe gösteren herhangi bir pano için bir adres verir. Adres, yalnızca canlı etkinlik günlüğünü gösteren küçük bir `/widget` sayfası açar. Belirteç o günlüğe erişim verir, başka hiçbir şeye vermez, **Devre dışı bırak** onu hemen iptal eder. Gömülü sayfa yalnızca İngilizcedir.

## BombVault'u ağda bulma {#mdns}

BombVault web arayüzünü yerel ağda, Bonjour ve Avahi'nin arkasındaki protokol olan mDNS ile duyurur. Bir tarayıcı ona `https://bombvault.local:3443` olarak, `HTTP_ONLY` ile ise `http://bombvault.local:3000` olarak ulaşır, servis tarayıcıları da onu `_bombvault` alt türüyle bir web servisi olarak listeler. TXT kayıtları sürümü ve yolu taşır. Anahtar **Ayarlar, Entegrasyonlar** altındaki **Hizmetler** kartında **Ağda bul**'dur ve varsayılan olarak açıktır. Ad başka bir cihaz tarafından zaten kullanılıyorsa BombVault `bombvault-2.local` gibi bir ad alır ve kart aldığı adresi gösterir. BombVault durduğunda ya da duyuruyu kapattığında ağa haber verir, böylece tarayıcılar kaydı hemen bırakır.

Duyurunun ağına ulaşıp ulaşmadığı konteynerin nasıl bağlandığına bağlıdır:

- **bridge**, Unraid şablonundaki varsayılan: duyuru Docker'ın ağının içinde kalır ve LAN'daki kimse görmez. BombVault'u eskisi gibi ana makinenin adresiyle aç.
- **br0** ya da başka bir macvlan veya ipvlan ağı: konteynerin LAN'da kendi adresi vardır ve duyuru ona ulaşır.
- **host**: duyuru ana makinenin arayüzlerinden, Unraid'in kendi duyurusunun yanında çıkar. Docker ve libvirt köprülerini atlar.

Yalnızca IPv4 adresleri duyurulur.
