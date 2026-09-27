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
| 403 | `not_permitted` | Belirteç yalnızca okuyabilir ya da çalışmayı o başlatmadı |
| 404 | `not_found` | Böyle bir öğe, çalışma veya anomali yok |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Başka bir şey çalışıyor, alan kapalı ya da yapılacak bir şey yok |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Bir sınır isteği bekletiyor; `Retry-After` ne zaman yeniden deneneceğini söyler |

Başlatmalar [MCP üzerinden başlatmalarla](mcp.md#starting-backups) aynı sınırlara tabidir: belirteç başına saatte 12, aynı öğenin iki başlatması arasında 15 dakika, bir öğe için 24 saatte en çok 4 başlatma ve saklama koruması. Son üçü MCP ve API üzerinden başlatmaları birlikte sayar. Bir belirteç dakikada 120 istek yapabilir. Bir adresten beş başarısız deneme o adresi bir dakika kilitler.

## OpenAPI {#openapi}

BombVault bu yolların açıklamasını `/api/v1/openapi.json` adresinde sunar (OpenAPI 3.1). Belirteç gerekmez. Swagger UI, Postman veya bir kod üreticisine yükle.
