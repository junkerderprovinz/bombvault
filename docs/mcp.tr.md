# MCP sunucusu

BombVault'ta, Claude Code ve Claude Desktop gibi yapay zeka asistanlarının dış araçlara ulaşmak için kullandığı protokol olan Model Context Protocol (MCP) için yerleşik bir sunucu vardır. Onun aracılığıyla bir asistan yedeklerinin nasıl gittiğini okuyabilir ve sen izin verirsen bir yedekleme başlatabilir ya da başlattığı birini iptal edebilir. Sen bir anahtar oluşturana ya da [OAuth ile girişi](#oauth) açana kadar kapalıdır: o zamana kadar `/mcp` uç noktası her şeye `404` yanıtlar.

## Bir asistan neler yapabilir, neler yapamaz {#tools}

| Araç | Ne yapar | Tür |
|---|---|---|
| `get_health` | Sürüm, örnek adı, bir yedeğin çalışıp çalışmadığı ve bu anahtarın neler yapabileceği | okuma |
| `get_status` | Bölüm başına koruma durumu: son başarılı yedek, beklenen aralık, doğrulama ve site dışı denetimler, sonraki zamanlanmış çalıştırmalar, boşta bir uygulamayı bekleyen yedekler ve konteynerler için en yeni başlatma testi | okuma |
| `get_coverage` | BombVault'un neyi koruduğu ve neyi korumadığı, her biri için nedenle | okuma |
| `list_items` | Zamanlaması, yedeğinin neyi durdurduğu, son yedeği ve onun ne kadar sürdüğüyle her korunan konteyner, VM, klasör kümesi, flash sürücü ve uygulama yapılandırması. Veritabanı konteynerleri son dökümlerini de taşır. ZFS veri kümeleri de son denetimlerinin sonucuyla listelenir. Her öge son geri yükleme denetimini taşır, bir konteyner de son başlatma testini ya da neden sınanamadığını. Son yedeğinden beri başka ayarlarla yeniden oluşturulmuş bir konteyner neyin değiştiğini listeler | okuma |
| `list_runs` | Çalıştırma geçmişi, en yeni önce, bölüme, ögeye, duruma, türe ve zamana göre süzülebilir. Tek bir şeyin yavaşlattığı bir yedek onu adlandırır | okuma |
| `list_restore_points` | Bir ögenin birincil deposundan geri yükleme noktaları ve bir konteyner için veritabanı dökümleri. Bir ZFS veri kümesi yedek başına, altındaki her veri kümesinin anlık görüntüsüyle, bir geri yükleme noktası alır | okuma |
| `get_activity` | Şu an ne çalışıyor, aşama ve yüzdeyle | okuma |
| `get_storage_stats` | Bir bölümün birincil deposunun boyut geçmişi ve haftalık büyümesi, ve her deposunun diskinde ya da uzak hedefinde kullanılan, boş ve toplam alan | okuma |
| `get_size_breakdown` | Bir konteynerin, VM'in ya da klasör kümesinin en yeni yedeğinde yeri hangi klasörlerin ve dosyaların kapladığı ve son yedeğin her birinden ne kadar eklediği | okuma |
| `search_files` | Tüm ögelerin, bir bölümün ya da bir ögenin yedeklerinde dosya ve klasörleri adına göre bulur (VM'lerde aramaz) ve her sonuç için onu içeren tüm yedekleri listeler | okuma |
| `list_anomalies` | BombVault'un yedeklerde fark ettiği anormallikler, duruma, önem düzeyine ve bölüme göre süzülebilir, açık olanların özetiyle | okuma |
| `get_anomaly` | Bu bulgulardan biri, onaylandığında bırakılan notla | okuma |
| `start_backup` | Bir ögeyi şimdi yedekler | başlatma |
| `start_domain_backup` | Bir bölümün korunan her ögesini yedekler | başlatma |
| `start_backup_everything` | Şimdi tümünü yedekle geçişini çalıştırır | başlatma |
| `cancel_backup` | Bu anahtarın başlattığı çalışan bir yedeği iptal eder | iptal |

Şunlar web arayüzünde kalır: her türlü geri yükleme (bir veritabanı dökümünü indirme, kaydetme ya da içe aktarma dahil), yedekleri silme, budama, kilit açma, denetimler ve tatbikatlar, site dışı çoğaltma, ayarlar, kimlik bilgileri ve MCP anahtarları ve zamanlamanın, web arayüzünün ya da başka bir anahtarın başlattığı bir yedeği iptal etme. Aynısı bir anormalliği onaylamak ya da beklenen olarak işaretlemek için de geçerlidir, bu **Anormallikler** sayfasında yapılır. Nedeni, araç çıktısının sunucundan isimler ve hata mesajları içermesi ve bunlardan herhangi birinin asistanı yönlendirmek için yazılmış metin taşıyabilmesidir. Böyle bir metne kanan bir asistan en kötü ihtimalle aşağıdaki sınırlar içinde bir yedekleme başlatabilir ya da kendi başlattığını iptal edebilir.

Bir ögenin birincil deposu uzaksa (S3, REST, SFTP, rclone), `list_restore_points` ona bağlanır, bu yüzden çağrı biraz sürebilir. Site dışı kopyalar MCP ile listelenemez. `search_files` site dışı kopyalar dahil uzak depoları yalnızca asistan isterse okur, çünkü bu daha uzun sürer ve bazı sağlayıcılar indirilen veri için ücret alır. Bir arama dakikalar sürebilir, bu yüzden ilk çağrı onu arka planda başlatır ve sonraki bir çağrı sonucu alır. Anormallik denetimlerinin neye baktığı [Anormallikler](anomalies.md) altında, bir ZFS ögesinin veri kümesi başına nasıl bir anlık görüntü tuttuğu ise [ZFS veri kümeleri](zfs-datasets.md#contents) altında anlatılır.

## Başlatılan bir yedek ne yapar {#starting-backups}

Bir asistanın yedeği, web arayüzünün başlattığıyla aynı yedektir. Çalışan bir konteyner, onunla birlikte durması ayarlı konteynerlerle birlikte, yedeği bitene kadar durdurulur. Düzgün kapatma yöntemli bir VM kapatılır ve yeniden başlatılır. Bir ZFS veri kümesi, anlık görüntüsü alınırken onun için ayarlı konteynerleri durdurur. Klasör kümeleri, flash sürücü ve yapılandırma çalışmaya devam eder. Sonra BombVault saklama ilkesini uygular ve site dışı depoya kopyalayabilir. `list_items` asistana bir ögenin neyi durdurduğunu ve son yedeğinin ne kadar sürdüğünü söyler, araç açıklamaları da bir şey başlatmadan önce sana söylemesini ister.

Bir yedek bir şeyleri durdurduğu ve eski geri yükleme noktalarını döndürdüğü için MCP üzerinden başlatmalar sınırlıdır:

- Anahtar başına saatte 12 başlatılmış yedek.
- Aynı ögenin, bölümün ya da Şimdi tümünü yedekle'nin iki MCP başlatması arasında 15 dakika.
- Aynı ögenin 24 saatte en fazla 4 MCP başlatması.
- **Saklama koruması.** Bir bölüm sabit sayıda geri yükleme noktası tuttuğunda (yalnızca "son N'i tut", günlük, haftalık ya da aylık kural olmadan, yerelde ya da site dışı hedefte), her yeni yedek en eskisini dışarı iter. BombVault o zaman, en yeni N-1 başarılı yedeğinin hepsi MCP üzerinden başlatılmış bir ögenin MCP başlatmasını reddeder. Böylece zamanlamanın ya da senin yaptığın en az bir geri yükleme noktası her zaman tutulan kümede kalır. "Son 1'i tut" ile bir asistan o ögeyi hiç yedekleyemez. Sonraki zamanlanmış yedek yeniden yer açar. Tek başına yıllık bir kural "son 1'i tut" sayılır, çünkü geçerli yıl için tek bir geri yükleme noktası tutar.

Bir bölüm ya da Şimdi tümünü yedekle başlatması, bir sınırın tuttuğu ögeleri dışarıda bırakır ve yanıtında adlandırır. Web arayüzü ve zamanlama bunların hiçbiriyle sınırlı değildir. Saatlik bütçe bellekte yaşar, bu yüzden BombVault'u yeniden başlatmak onu sıfırlar.

[API](api.md#errors) üzerinden ve [Home Assistant](api.md#home-assistant)'tan başlatmalar, öge başına aynı sınırlara ve saklama korumasına, MCP üzerinden başlatmalar gibi sayılır.

## Aç {#switch-on}

1. **Ayarlar, Entegrasyonlar, MCP sunucusu**'nu aç, **İstemci seç**'e tıkla ve penceredeki kendininkini seç. Sağ üstündeki arama onu adıyla bulur. Listelenmeyen bir istemci **Başka istemci** üzerinden bağlanır.
2. **Anahtar** altında **Yeni anahtar**'ı ve önerdiği adı, yani istemcinin adını, bırak ya da anahtarın nerede kullanıldığını söyleyen birini yaz, örneğin "Dizüstündeki Claude Code". İstemci başına bir anahtar, diğerlerine dokunmadan birini iptal etmeni sağlar. **Mevcut anahtar** istemciye daha önce yaptığın bir anahtarı verir.
3. Yedekleme başlatabilmesi gereken bir anahtar için **Yedekleme başlatmaya izin ver**'i aç. Onsuz anahtar yalnızca okuyabilir. Bunu sonra anahtarın **Ayrıntılar**'ında değiştirebilirsin ve değişiklik yeniden bağlanmadan asistanın sonraki isteğinde geçerli olur.
4. **Anahtar oluştur**'a tıkla. Anahtar bir kez gösterilir. BombVault yalnızca parmak izini tutar ve onu yeniden gösteremez, bu yüzden şimdi kopyala. İstemci anahtarı kullanmadan pencereyi kapatırsan, kopyaladığını onaylayana kadar kart onu göstermeye devam eder.

Giriş parolası olmadan web arayüzünün kendisi ağındaki herkese açıktır ve onu açabilen herkes bir anahtar da oluşturabilir. Kart bunu söyler. BombVault'u herkese açık gibi görünen bir adla açarsan (örneğin bir ters proxy ile `bombvault.example.com`) ve giriş parolası ayarlı değilse, o adresten anahtar oluşturulamaz ya da değiştirilemez, böylece internetteki hiçbir web sayfası tarayıcının bir tane oluşturmasını sağlayamaz. Bir giriş parolası ayarla ya da BombVault'u IP adresiyle veya `tower` ya da `tower.local` gibi yerel bir adla aç.

## Anahtarların ve günlükleri {#keys}

Her anahtarın **Anahtarlar ve oturum açmış istemciler** altında kendi satırı vardır. Anahtarın adını, yedekleme başlatabildiğini ya da yalnızca okuyabildiğini, anahtarın son dört karakterini, ne zaman oluşturulduğunu ya da en son değiştirildiğini ve bir istemcinin onu en son ne zaman kullandığını gösterir. **Ayrıntılar**, anahtarı yeniden adlandırdığın, iznini değiştirdiğin, değiştirdiğin ya da iptal ettiğin ve bugün kaç çağrı yaptığını gösteren bir pencere açar. İptal edilmiş bir anahtar iptal edilmişler listesine gider, geçmişteki hiçbir çalıştırma onu adlandırmayınca orada kalıcı silebilirsin.

Adının yanında satır, anahtarın yapıldığı istemcinin işaretini gösterir. **Başka istemci** üzerinden ya da kart istemcileri listelemeden önce yapılmış bir anahtar bunun yerine bir anahtar gösterir.

Bir anahtarın ayrıntılarındaki **Günlük**, o anahtarın ne yaptığını gösterir ve iptal edilmiş bir anahtar kendi **Günlük** düğmesini korur. Önce başlattığı yedekler gelir, her biri durumu ve panodaki etkinlik günlüğündeki o çalıştırmaya bir bağlantıyla. Altlarında çağrıları vardır, en yeni önce, araç ve çağrıya ne olduğuyla. Bir ret nedenini söyler: anahtar yalnızca okuyabilir, saklama koruması yedeği tuttu, başka bir yedek zaten çalışıyordu, aynı öge, bölüm ya da geçiş birkaç dakika önce web arayüzü dışında başlatıldı ya da anahtar çok fazla istek gönderdi. Bir iptal, ilgili olduğu çalıştırmaya bağlanır.

BombVault her anahtarın kayıtlarını en fazla 30 gün tutar: en yeni 500 başarılı başlatma ve iptal ve yanlarında en yeni 200 diğer çağrı (okumalar, retler ve hatalar). Çalışan bir yedeği yoklamaya devam eden ya da reddedilen bir çağrıyı yeniden deneyen bir asistan, o yedeğin başlangıcını günlükten itemez. Her çağrı için aracı, sonucu ve bir iptalin adlandırdığı çalıştırmayı saklar. Asistanın gönderdiğini hiçbir zaman saklamaz, anahtarı ya da parmak izini de. Tanılama paketi kayıtları yalnızca sayar ve ayar dışa aktarması onları dışarıda bırakır.

## Bir istemci bağla {#clients}

**İstemci seç**, her istemcinin bir kutucuk olduğu, **Bu bilgisayarda** ya da **Bulutta** altında ve sağ üstte bir aramayla bir pencere açar. Bir kutucuk istemcinin kurulumunu aynı pencerede üç adımda açar: anahtar, o istemci için yapılandırma (kartı açtığın adresle, kopyalama düğmesiyle, yapılandırmanın nerede durduğuyla ve BombVault'un kendi sertifikasıyla istemcinin ona güvenmek için neye ihtiyaç duyduğuyla) ve istemcinin ilk çağrısını bekleme. Pencere anahtarın son kullanımını izler ve o çağrı gelince yeşile döner.

Kurulum anahtarı hiçbir komut satırında tutmaz. İstemcinin onu bir ortam değişkeninden (`BOMBVAULT_MCP_KEY`), maskeli bir istemden ya da kendi dosyasından okuyabildiği yerde yapılandırma yalnızca onu adlandırır. İstemcinin böyle bir yolu olmayan yerde anahtar yapılandırma dosyasında ya da ayarlarında durur ve kurulum bunu söyler. Bir istemcinin belgelerinin tanımadığı bir sertifikaya nasıl davrandığını söylemediği yerde kurulum o adımı, istemci BombVault'un sertifikasını reddederse ne yapılacağı olarak yazar.

| İstemci | Kurulum | Anahtar nereden gelir |
|---|---|---|
| AnythingLLM | yapılandırma dosyası | yapılandırma dosyası |
| Antigravity | yapılandırma dosyası | ortam değişkeni |
| Claude Code | komut | anahtar dosyası |
| Claude Desktop | yapılandırma dosyası | anahtar dosyası |
| Cline | yapılandırma dosyası | yapılandırma dosyası |
| Codex CLI | yapılandırma dosyası | ortam değişkeni |
| Continue | yapılandırma dosyası | `~/.continue/.env` |
| Copilot CLI | yapılandırma dosyası | yapılandırma dosyası |
| Cursor | yapılandırma dosyası | ortam değişkeni |
| Gemini CLI | yapılandırma dosyası | ortam değişkeni |
| GitHub Copilot (VS Code) | yapılandırma dosyası | maskeli istem |
| Goose | yapılandırma dosyası | ortam değişkeni |
| Jan | uygulamada form | uygulamanın ayarları |
| JetBrains (AI Assistant, Junie) | yapılandırma dosyası | yapılandırma dosyası |
| Kimi Code | yapılandırma dosyası | yapılandırma dosyası |
| LM Studio | yapılandırma dosyası | yapılandırma dosyası |
| Mistral Vibe | yapılandırma dosyası | ortam değişkeni |
| Msty | uygulamada form | uygulamanın ayarları |
| n8n | uygulamada form | n8n'in kimlik bilgisi deposu |
| Open WebUI | uygulamada form | uygulamanın ayarları |
| opencode | yapılandırma dosyası | ortam değişkeni |
| Perplexity (Mac) | uygulamada form | anahtar dosyası |
| Qwen Code | yapılandırma dosyası | ortam değişkeni |
| Roo Code | yapılandırma dosyası | ortam değişkeni |
| Visual Studio | yapılandırma dosyası | yapılandırma dosyası |
| Warp | yapılandırma dosyası | yapılandırma dosyası |
| Windsurf | yapılandırma dosyası | ortam değişkeni |
| Zed | yapılandırma dosyası | yapılandırma dosyası |
| Grok | form, bulutta | sağlayıcının sunucuları |
| Le Chat | form, bulutta | sağlayıcının sunucuları |
| ChatGPT | OAuth ile giriş, bulutta | bir erişim belirteci, bkz. [aşağıda](#oauth) |
| Claude (claude.ai) | OAuth ile giriş, bulutta | bir erişim belirteci, bkz. [aşağıda](#oauth) |

Aşağıdaki bölümler Claude Code ve Claude Desktop kurulumlarını daha ayrıntılı açıklar ve başka herhangi bir istemcinin neye ihtiyaç duyduğunu listeler.

### Claude Code {#claude-code}

Claude Code, BombVault'a o bilgisayarda Node.js gerektiren `mcp-remote` üzerinden ulaşır. Önce anahtarı kendi başına bir metin dosyasına, tek satır olarak kaydet:

```text
X-API-Key: <your key>
```

Sonra karttaki komutu, o dosyanın yolunu doldurarak, bir terminalde bir kez çalıştır. Bilgisayarının güvendiği bir sertifikanın arkasında şöyle görünür:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

BombVault'un kendi sertifikasıyla ([TLS ve sertifikalar](#tls)'a bak) komut ayrıca Node.js'i indirilen sertifikaya yönlendirir:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Bağlantıyı Claude Code içinde `/mcp` ile kontrol et. `--scope user` BombVault'u tüm projelerinde kullanılabilir yapar. Claude Code yalnızca anahtar dosyasının yolunu tutar, bu yüzden anahtar ne komutta ve kabuk geçmişinde ne de işlem listesinde görünür. Dosyayı yalnızca senin okuyabileceğin bir yerde ve commit ettiğin herhangi bir klasörün dışında tut. `@latest`, `npx`'in güncel bir `mcp-remote` getirmesini sağlar. Genel olarak kurulmuş daha eski biri bunun yerine kullanılırdı ve `--header-file`'ı bilmez.

Claude Code için `mcp-remote` argümanlarına `${BOMBVAULT_MCP_KEY}` yazma. Claude Code böyle bir başvuruyu `mcp-remote`'u başlatmadan önce kendi ortamından doldurur, bu yüzden anahtar o işlemin komut satırında, bilgisayarın diğer programlarının ve kullanıcılarının okuyabileceği yerde, sonlanır.

Node.js olmadan ve yalnızca bilgisayarının güvendiği bir sertifikanın arkasında Claude Code kendi başına bağlanabilir. Proje klasörüne bir `.mcp.json` koy:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

`BOMBVAULT_MCP_KEY`'i Claude Code'un başladığı yerde ayarla, örneğin `~/.claude/settings.json` içinde `"env"` altında ya da komut isteminde yazmak yerine bir metin düzenleyicide düzenlenen kabuk profilinde. Burada başvuru güvenlidir, çünkü Claude Code onu taşıyacak ikinci bir işlem başlatmaz. BombVault'un kendi sertifikası bu şekilde çalışmaz: Claude Code'un kendi bağlantısı `NODE_EXTRA_CA_CERTS` ayarlı olsa bile onu reddeder. İçine anahtar yazılmış bir `.mcp.json`'ı asla commit etme.

### Claude Desktop {#claude-desktop}

Claude Desktop, BombVault'a o bilgisayarda Node.js gerektiren `mcp-remote` üzerinden ulaşır. Önce anahtarı kendi başına bir metin dosyasına, [Claude Code](#claude-code) için anlatıldığı gibi tek satır olarak kaydet. Yapılandırma dosyasını Claude Desktop'ta **Settings, Developer, Edit Config** ile aç. Windows'ta `%APPDATA%\Claude\claude_desktop_config.json`, macOS'ta `~/Library/Application Support/Claude/claude_desktop_config.json` konumundadır. Karttaki girdiyi, orada zaten olan sunucuların yanına `"mcpServers"` içine ekle ve Claude Desktop'ı yeniden başlat:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` yalnızca BombVault'un kendi sertifikası içindir. Bilgisayarının zaten güvendiği bir sertifikanın arkasında bırak.
- `--allow-http` yalnızca düz bir `http://` adresi için eklenir.
- Windows'ta yolları eğik çizgilerle yaz, örneğin `C:/Users/sam/bombvault-key.txt`, çünkü tek bir ters eğik çizgi JSON'da geçerli değildir. Anahtar dosyasının yolunda boşluk olmasın: Windows'ta Claude Desktop boşluklu bir yolu `npx`'e iki parça olarak verir.
- Yapılandırma yalnızca anahtar dosyasını adlandırır, bu yüzden anahtar ne onda ne de işlem listesinde görünür. Dosyayı yalnızca senin okuyabileceğin bir yerde tut.

### Buluttaki istemciler {#cloud-clients}

ChatGPT, claude.ai'daki Claude, Grok ve Le Chat BombVault'u sağlayıcılarının sunucularından çağırır, bu yüzden BombVault'a internetten, herkesçe güvenilen bir sertifikayla, örneğin bir ters proxy'nin arkasında, ulaşılabilmelidir. Le Chat kendi kendine imzalıları reddeder. Proxy'deki bir giriş web arayüzünü koruyabilir ama `/mcp` ona takılmadan BombVault'a geçmelidir: bu servisler bir proxy'de oturum açamaz ve BombVault anahtarlarını ya da belirteçlerini kendisi denetler. Grok ve Le Chat sabit bir anahtar gönderir ve düğmeleri onları diğerleri gibi kurar. ChatGPT ve çoğu kuruluşta claude.ai'daki Claude yalnızca aşağıda anlatılan OAuth ile girişle bağlanır.

### OAuth ile giriş {#oauth}

Anahtar kabul edemeyen bir istemci için BombVault kendi OAuth yetkilendirme sunucusudur. İstemci kendini kaydeder, seni bir BombVault sayfasına gönderir ve orada giriş parolanla (ve kurduysan ikinci faktörle) oturum açıp ona izin verirsin. İstemci sonra yalnızca bu BombVault'un MCP uç noktası için çalışan bir belirteç alır ve onu kendiliğinden yeniler.

1. **Ayarlar, Güvenlik** altında bir giriş parolası ayarla. Onsuz BombVault hiç oturum açma sunmaz, çünkü rıza isteyecek kimse olmazdı.
2. BombVault'a internetten https ile, tarayıcıların güvendiği bir sertifikayla, genellikle bir ters proxy aracılığıyla, ulaşılabilir yap. İstemci `/mcp`, `/oauth/` ve `/.well-known/` yollarını kendi sunucularından çağırır, bu yüzden kendi girişi olan bir proxy bu üç yolu BombVault'a geçirmelidir. `/oauth/authorize` adresindeki rıza sayfası kendi tarayıcında açılır ve proxy girişinin arkasında kalabilir. Proxy'yi `TRUSTED_PROXY` içinde de adlandır (bkz. [Yapılandırma](configuration.md)). BombVault istemci kayıtlarını adres başına sınırlar ve onsuz her istemci proxy'den geliyormuş gibi görünür.
3. MCP kartında **OAuth ile giriş**'i aç ve **Genel adres**'i gir: yolsuz https adresi, örneğin `https://backup.example.com`. Her belirteç bu adrese bağlıdır, bu yüzden bir değişiklikten sonra her istemcinin yeniden oturum açması gerekir.
4. **İstemci seç** altında ChatGPT'yi ya da Claude'u seç. Pencere genel adres artı `/mcp` olan **Bağlayıcı URL'si**'ni ve o istemcide nereye gittiğini gösterir. ChatGPT'de **Settings, Apps & Connectors, Advanced settings** altında Developer mode'u aç, **Create**'i seç, bağlayıcı URL'sini MCP sunucu URL'si olarak yapıştır ve kimlik doğrulama olarak OAuth'u seç. claude.ai'da **Settings, Connectors, Add custom connector**'ı aç, bağlayıcı URL'sini yapıştır, OAuth istemci kimliğini ve gizli bilgisini boş bırak ve **Connect**'i seç.
5. İstemci rıza sayfasını açar. Kimin istediğini, yanıtının seni nereye geri göndereceğini ve başlangıçta kapalı olan **Yedekleme başlatmaya izin ver** anahtarını gösterir. **İzin ver** ya da **Reddet**'i seç.

Oturum açan her istemci aynı listede işaretiyle kendi satırını alır. **Ayrıntılar**'ında günlüğü, **İptal et** ve **Yedekleme başlatmaya izin ver** bulunur ve bir anahtarla aynı sınırlara sahiptir. İptal hemen etkili olur. Aynı istemci yeniden oturum açtığında yeni izni eskisinin yerini alır ve 30 gün kimsenin kullanmadığı bir izin sona erer. 10 anahtarın üstüne aynı anda 10 istemciye kadar oturum açmış olabilir.

Rıza sayfası bir isteği yalnızca kayıtlı bir istemciden, kayıtlı dönüş adreslerinden birini tam olarak adlandırıyorsa kabul eder: https ya da kendi bilgisayarındaki bir istemci için herhangi bir portta bir loopback adresi. Yalnızca PKCE (S256) ile yetkilendirme kodu akışı kabul edilir ve yanıtın oturumuna bağlıdır, bu yüzden başka hiçbir web sitesi onu senin yerine gönderemez. Erişim belirteçleri bir saat sürer. Bir yenileme belirteci her kullanıldığında değiştirilir ve bundan sonra yeniden ortaya çıkan, izni iptal eder, çünkü bir kopyası başkasında demektir. Son yenilemesini 30 saniye içinde tekrarlayan bir istemci, yanıt ona ulaşmadığı için, yeni belirteçler alır. BombVault istemci üstveri belgelerini internetten getirmez, bu yüzden istemciler dinamik istemci kaydıyla kaydolur.

### Diğer istemciler {#other-clients}

Streamable HTTP konuşan her istemci çalışır:

- URL: web arayüzünün adresi artı `/mcp`, örneğin `https://192.168.1.10:3443/mcp`.
- Anahtar `Authorization: Bearer <key>` ya da `X-API-Key: <key>` içinde. İkisi de gönderilirse aynı anahtarı taşımalıdır.
- `Content-Type: application/json` ve `Accept: application/json, text/event-stream` ile `POST`.
- İstek başına bir JSON-RPC mesajı. Toplu istekler reddedilir.
- Protokol sürümleri 2026-07-28, 2025-11-25, 2025-06-18 ve 2025-03-26.

## TLS ve sertifikalar {#tls}

BombVault kendi ürettiği bir sertifikayla HTTPS sunar ve bu sertifika başlangıçta yalnızca `localhost`, `127.0.0.1` ve `::1`'i adlandırır. Claude Code ve `mcp-remote` bir LAN adresinde onu reddeder. Çözmenin yolları, çoğu Unraid kurulumuna uyan sırayla:

1. **Adresi MCP kartında ekle.** Kart HTTPS üzerinden sertifikanın adlandırmadığı bir adreste açıldığında bunu söyler ve **Bu adresi sertifikaya ekle**'yi sunar. BombVault sertifikasını o adres eklenmiş olarak yeniden düzenler (tarayıcın ilk seferdeki gibi bir kez daha uyarır). Sonra **Sertifikayı indir**'e tıkla. Parçacıklar `NODE_EXTRA_CA_CERTS`'i indirilen dosyaya ayarlar, böylece istemci tam olarak bu sertifikaya güvenir. Bu aynı zamanda, önceki bir indirmeyle kurulmuş her istemcinin, sertifika yeniden düzenlendiği anda, bu bilgisayarda ve diğer her birinde, yeni dosyayı alana kadar bağlanmayı bıraktığı anlamına gelir.
2. **Güvenilir bir sertifikalı ters proxy** (Nginx Proxy Manager, SWAG, Caddy, Traefik). İstemci o zaman proxy'nin sertifikasını görür ve fazladan bir şeye ihtiyaç duymaz, kart da BombVault'un kendi sertifikası hakkında uyarmaz.
3. **Tailscale.** Konteynerin önünde `tailscale serve` ya da Unraid Tailscale entegrasyonu sana güvenilir sertifikalı bir `ts.net` adı verir.
4. **`HTTP_ONLY=true`**, yalnızca TLS'i sonlandıran bir proxy'nin arkasında ya da tamamen güvendiğin bir ağda. Web arayüzünün tamamını düz HTTP'ye geçirir, konteyner ayarlarında bir değişiklik gerektirir ve anahtarı şifresiz gönderir.

`NODE_TLS_REJECT_UNAUTHORIZED=0`'ı asla ayarlama. O Node.js işleminin konuştuğu her şey için sertifika denetimlerini kapatır.

Bir ters proxy `Authorization` (ya da `X-API-Key`) başlığını geçirmelidir, aksi söylenmedikçe proxy'ler bunu yapar, ve `/mcp`'yi arabelleğe almamalı ya da yeniden yazmamalıdır. BombVault'un sertifikasını da denetleyen Nginx ya da Nginx Proxy Manager için bir location bloğu:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Bir proxy'nin arkasında her istek proxy'nin adresini taşır. Yanlış yapılandırılmış tek bir istemciden beş yanlış anahtar, o proxy'nin arkasındaki tüm MCP istemcilerini bir dakikalığına kilitler. İstemci başına saymak için proxy'yi `TRUSTED_PROXY` içinde adlandır (bkz. [Yapılandırma](configuration.md)).

## Güvenlik modeli {#security}

- Etkin anahtar olmadan ve OAuth ile giriş kapalıyken `/mcp` `404` yanıtlar.
- OAuth ile giriş yalnızca bir giriş parolası ayarlıyken sunulur. Belirteçler, kodlar ve istemci gizli bilgileri yalnızca parmak izi olarak saklanır ve bir belirteç yalnızca verildiği adres için çalışır.
- Bir istemci bir adresten saatte en fazla 10 kez kaydolabilir ve BombVault kimsenin oturum açmadığı en fazla 100 kayıtlı istemciyi, her birini bir gün, tutar. Yanlış kodlar ve yenileme belirteçleri, yanlış anahtarlarla aynı kilitlemeye sayılır.
- İzinler, bir yapılandırma yedeği geri yüklendiğinde ya da `APP_KEY` değiştiğinde anahtarlar gibi davranır: geri yüklemeden sonra her istemcinin yeniden oturum açması gerekir.
- Muaf adres yok. `localhost`, Unraid ana makinesi, bir ters proxy ya da `tailscale serve` kaynaklı isteklerin de diğerleri gibi bir anahtara ihtiyacı vardır, web arayüzünün giriş parolası olmadığında da.
- Anahtarlar yalnızca parmak izi olarak saklanır, bir kez gösterilir ve yeniden adlandırılabilir, değiştirilebilir ve iptal edilebilir. 10 etkin anahtara kadar, her birinin kendi **Yedekleme başlatmaya izin ver** anahtarıyla.
- Her oluşturma, değiştirme, izin değişikliği ve iptal, bildirimler kapalı değilse, geldiği adresle bildirim kanallarından bir bildirim gönderir.
- Adres başına dakikada 5 yanlış anahtar, sonra `429`. Anahtar başına dakikada 120 istek ve saatte 12 başlatılmış yedek, artı yukarıdaki bekleme süresi ve saklama koruması.
- Başka bir kaynaktaki (origin) bir tarayıcı sayfasından gelen istekler reddedilir.
- Giriş parolası ayarlı değilken herkese açık gibi görünen bir ana makine adından anahtar oluşturulamaz.
- Bir asistanın başlattığı her yedek ve neden olduğu budama ve site dışı çalıştırmalar, Etkinlik günlüğünde, hata panelinde ve yedekleme bildiriminde anahtarın adıyla "MCP üzerinden" olarak işaretlenir.
- Her araç çağrısı, anahtarın kimliği ve son dört karakteriyle (adıyla asla) konteyner günlüğüne yazılır ve `/metrics` içinde sayılır (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Bir yapılandırma yedeğini geri yüklemek her anahtarı iptal eder, çünkü geri yüklenen veritabanı kaydedildikten sonra iptal ettiğin anahtarları içerebilir. Sonrasında yeni anahtarlar oluştur.
- Bir anahtar `APP_KEY` değiştiğinde çalışmayı bırakır (yeniden kurulum ya da başka bir konteynere geri yükleme). Kart bunu algılar ve anahtarı işaretler, **Anahtarı değiştir** ona yeniden çalışan bir gizli bilgi verir.
- Bir anahtara parola gibi davran. Anahtarı bir ortam değişkeninden, bir istemden ya da bir anahtar dosyasından okuyamayan bir istemci, onu yapılandırmasında ya da ayarlarında düz metin olarak tutar ve kurulumu bunu söyler. Daha az güvendiğin bir bilgisayarda salt okunur bir anahtarı tercih et.

## Sunucudan ne çıkar {#privacy}

Bir asistanın okuduğu her şey arkasındaki yapay zeka sağlayıcısına gider: öge adları, zamanlamalar, hata mesajlarıyla çalıştırma geçmişi, geri yükleme noktası kimlikleri ve zamanları, veritabanı motoru adları ve döküm boyutları, güncel etkinlik, depolama rakamları, kapsam ve durum. Dosya araması bulduğu dosyaları da gönderir: adlarını, boyutlarını ve yedek içindeki yollarını. BombVault bir şey çıkmadan önce ana makine yollarını, depo konumlarını, ana makine adlarını, kimlik bilgilerini, kanca komutlarını ve anahtarları kaldırır.

## Sorun giderme {#troubleshooting}

| Gördüğün | Anlamı |
|---|---|
| `404` | Etkin anahtar yok ve OAuth ile giriş kapalı ya da `/api/mcp` gibi yanlış bir yol. Uç nokta `/mcp`'dir. |
| `401` | Anahtar eksik, yanlış yazılmış, iptal edilmiş ya da değiştirilmiş. Bir proxy `Authorization` başlığını düşürüyor olabilir (`X-API-Key`'i dene). Kart anahtarı artık geçerli değil diye işaretliyorsa `APP_KEY` değişmiştir: anahtarı değiştir. |
| `403` | İstek başka bir kaynaktaki (origin) bir tarayıcı sayfasından geldi. Bir masaüstü ya da komut satırı istemcisi kullan. |
| GET'te `405` | Normal. Uç nokta yalnızca `POST` alır. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | İstemci Streamable HTTP için çok eski. Güncelle. |
| `400` "batch requests are not accepted" | İstemci JSON-RPC toplu istekleri gönderiyor. İstek başına bir mesaj gönder. |
| `429` | Bu adresten çok fazla yanlış anahtar ya da tek bir anahtarla dakikada 120'den fazla istek. Bir dakika bekle ve asistanın bir döngüde takılıp takılmadığına bak. |
| "certificate", "self-signed" ya da "unable to verify" hataları | İstemci BombVault'un sertifikasına güvenmiyor. Bkz. [TLS ve sertifikalar](#tls). |
| `busy` | Başka bir yedek ya da bakım işi o bölümü tutuyor. Bittiğinde yeniden dene. |
| `cooldown` | Bu öge, bölüm ya da Şimdi tümünü yedekle web arayüzü dışında 15 dakikadan kısa süre önce başlatıldı. |
| `retention_guard` | Bir MCP yedeği daha, "son N'i tut" penceresinde yalnızca MCP yapımı geri yükleme noktaları bırakırdı ya da öge son 24 saatte zaten MCP üzerinden 4 yedek aldı, başarısız ve iptal edilenler dahil. İlk durumda sonraki zamanlanmış yedek yer açar, ikincisinde öge bu yedeklerin en eskisinden 24 saat sonra yeniden serbest kalır. Her iki durumda da onu web arayüzünde başlatabilirsin. |
| `rate_limited` | Anahtar bu saat için 12 başlatmasını kullandı. |
| Başlatmada `not_permitted` | Anahtar salt okunur. Kartta **Yedekleme başlatmaya izin ver**'i aç. Yeniden bağlanmak gerekmez. Bir iptalde bu, çalıştırmanın bu anahtarca başlatılmadığı anlamına gelir. |
| `domain_off` | O bölüm **Ayarlar, Genel** altında kapalı. |
| `not_found` | Öge BombVault tarafından korunmuyor. Önce web arayüzünde ekle. MCP hiçbir zaman yapılandırma oluşturmaz. |
| İstemci yetkilendirme sunucusunu bulamıyor | OAuth ile giriş kapalı, giriş parolası ayarlı değil ya da proxy `/.well-known/` yolunu BombVault'a geçirmiyor. |
| Rıza sayfası dönüş adresinin kayıtlı olmadığını söylüyor | İstemci kaydetmediği bir dönüş adresi gönderdi. Bağlayıcıyı istemcide kaldır ve yeniden ekle. |
| Oturum açmış bir istemci `401` alıyor | İzni iptal edildi, 30 gün kullanılmadan sona erdi ya da genel adres değişti. İstemci yeniden oturum açar. |

Konteynerde `MCPGODEBUG` ortam değişkenini ayarlama. MCP kütüphanesinin davranışını değiştirir ve hatalı biçimlendirilmiş bir değer BombVault'u tek bir günlük satırı yazmadan başlangıçta durdurur.
