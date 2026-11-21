# Düğümler

Düğüm, web arayüzü olmayan bir BombVault'tur, örneğin bir VPS'te ya da bir arkadaşının evindeki küçük bir sunucuda. Kendi sunucusundaki konteynerleri ve klasörleri yedekler. Ne zaman yedekleyeceğini ve yedeklerin nereye gideceğini ise onu eklediğin BombVault söyler, bu sayfada ona **ana örnek** diyoruz. Düğümü ana örneğin web arayüzünden, kendi ögelerinin yanında yönetirsin.

![Örnekler sayfasında vps-falkenstein düğümünün kartı: Bağlı, sürümü, alıcı sunucudaki yedekleri, her bölümün ne zaman yedeklendiği ve Hedefi değiştir, Değiştir ile Serbest bırak.](assets/screenshots/nodes/card-dark.png#only-dark)
![Örnekler sayfasında vps-falkenstein düğümünün kartı: Bağlı, sürümü, alıcı sunucudaki yedekleri, her bölümün ne zaman yedeklendiği ve Hedefi değiştir, Değiştir ile Serbest bırak.](assets/screenshots/nodes/card-light.png#only-light)

## Nasıl çalışır {#how-it-works}

- **Düğümün anahtarını ana örnek tutar.** Bir düğüm eklediğinde ana örnek onun `APP_KEY` anahtarını üretir, mühürlü bir kopyasını saklar ve anahtarı sana bir kez, düğümün compose dosyasının içinde verir. Ana örnek bu kopyayla düğüme ulaşır ve yedeklerini açar.
- **Düğümün açık bir porta ihtiyacı yoktur.** Ana örnek ve düğüm, eşleştirme grubundan ayrı, kendilerine ait özel bir bağlantıyla röle üzerinden konuşur. Her çağrıyı ana örnek başlatır ve imzalar, düğüm ise ondan selamı dışında hiçbir şey istemez. Açık port gerektiren tek hedef [yedekleri düğümde tutmaktır](#keep-on-node).
- **Yedekler asla röleden geçmez.** Düğüm onları doğrudan seçtiğin yere gönderir: ana örneğin alıcı sunucusuna, ana örneğin hedeflerinden birine ya da düğümün kendi üzerindeki bir rest-server'a.
- **Düğüm kendi başına hiçbir şey planlamaz.** Zamanlama çalıştırmaz, bildirim göndermez, hiçbir gruba ya da kaleye katılmaz. Yedeklerini ana örnek başlatır, nasıl gittiğini izler ve uyarıları gönderir.
- **Düğüm konteynerleri ve klasör kümelerini yedekler.** VM'ler, flash sürücü, ZFS veri kümeleri ve öz yedek tam kurulumlarda kalır.

Bağlantı, grubunda varsa ana örneğin kendi rölesini, yoksa projenin rölesini kullanır, ana örnek röle olmadan eşleşse bile.

## Başlamadan önce {#before-you-start}

- **Ana örnekte bir giriş parolası.** Düğüm eklemek, compose dosyasını yeniden göstermek ve ögelerini açmak parolayla oturum açmayı gerektirir, bkz. [Giriş](configuration.md#login).
- **Düğümün sunucusunda Compose ile Docker.**
- **Yedekler için bir yer**, [Yedekler nereye gider](#placement) altındaki üç seçenekten biri. En güvenli seçim alıcı sunucudur, çünkü düğüm orada hiçbir şey silemez.

## Düğüm eklemek {#add}

1. Ana örnekte **Örnekler** sayfasını aç ve sağ alttan **Düğüm ekle**'yi seç.
2. **Ad.** Düğümün burada hangi adla görüneceğini yaz, örneğin sunucunun adı, ve **İleri**'yi seç.

    ![Düğüm ekle penceresinin ilk adımı, vps-helsinki adı yazılmış.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Düğüm ekle penceresinin ilk adımı, vps-helsinki adı yazılmış.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Yedekler.** Yedeklerin nereye gideceğini seç ve o yerin istediklerini doldur. **Her gün yedekleme saati** iki bölümün de başladığı saattir. Ana örnek, ikisi aynı anda aynı disklere uzanmasın diye kendi konteyner yedeklerinden yarım saat sonrasını önerir. Daha sonra kartta her bölüme kendi planını verebilirsin. **İleri**'yi seç.

    ![İkinci adım: alıcı sunucu seçili, Düğümler için adres bir https adresiyle dolu ve Her gün yedekleme saati 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![İkinci adım: alıcı sunucu seçili, Düğümler için adres bir https adresiyle dolu ve Her gün yedekleme saati 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Başlat.** Pencere, `MODE`, `APP_KEY`, `NODE_HOME` ve `NODE_NAME` doldurulmuş olarak düğümün compose dosyasını gösterir. Dosyayı kopyala ve `APP_KEY` anahtarını güvenli bir yerde sakla. Düğümün sunucusunda:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Başlatmadan önce Host Data birimini yedeklemek istediğin verilere yönlendir ve `HOST_SOURCE_ROOT` değerini, aşağıdaki örnekte olduğu gibi aynı yola ayarla.

5. Pencere düğümü bekler. Düğüm yanıt verince ana örnek ona yedeklerinin nereye gideceğini söyler ve oraya ulaşıp ulaşmadığını denetler. Pencerede o zaman **vps-helsinki çevrimiçi ve bu örnekten komut alıyor.** yazar.

    ![Son adım: compose dosyası, ardından çevrimiçi düğüm ve alıcı sunucuya yerleştirilmiş yedekleri.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Son adım: compose dosyası, ardından çevrimiçi düğüm ve alıcı sunucuya yerleştirilmiş yedekleri.](assets/screenshots/nodes/add-start-light.png#only-light)

Pencereyi düğüm yanıt vermeden kapatabilirsin. Kartında o zaman **Düğüm bekleniyor** yazar ve ana örnek, düğüm göründüğü anda ona yedeklerinin nereye gideceğini söyler.

### Örnek: alıcı sunucuya yedekleyen bir VPS {#example-vps}

Ana örnek, [alıcı sunucuyu](offsite-recovery.md#receiving-server) çalıştıran `tower` adlı bir Unraid makinesidir. Bir ters proxy onu `https://backup.tower.example.org` olarak yayınlar. `vps-helsinki` VPS'i Nextcloud ve Caddy çalıştırır, verileri `/srv/appdata` altındadır.

1. `tower` üzerinde: **Örnekler**, **Düğüm ekle**, ad `vps-helsinki`, **İleri**.
2. **Bu örneğin alıcı sunucusu**'nu seç ve **Düğümler için adres** alanına `https://backup.tower.example.org` yaz. Ana örneğin konteynerleri 02:30'da yedeklenir, bu yüzden pencere 03:00'ü önerir. Bunu bırak ve **İleri**'yi seç.
3. VPS'te, birimleri ve `HOST_SOURCE_ROOT` değerini `/srv/appdata` için değiştirerek penceredeki dosyayı kaydet:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Birkaç saniye sonra `tower` üzerindeki pencere düğümün çevrimiçi olduğunu ve **Yedekler https://backup.tower.example.org hedefine gidiyor.** yazar.
5. Saat 03:00'te ana örnek düğümün yedeklerini başlatır. Ertesi sabah genel bakıştaki **Düğümler** kartı `vps-helsinki` düğümünün son yedeğini gösterir, **Yedekler** de konteynerlerini ayrı bir bölümde listeler.

Bir konteynerin verilerinin düz bir Docker ana makinesinde nasıl bulunduğu [Yedek kaynağı algılama](configuration.md#backup-source-detection) altında anlatılır.

## Yedekler nereye gider {#placement}

Her düğümün iki bölümü için tek bir hedefi vardır. Kartındaki **Hedefi değiştir**, düğüm çevrimiçiyken hedefi değiştirir. Yeni yer yeni depolar alır, eski yerdeki yedekler olduğu yerde kalır.

### Bu örneğin alıcı sunucusu {#to-receiver}

Düğüm, ana örneğin alıcı sunucusuna append-only kipte yazar: yedek ekleyebilir ama hiçbirini silemez. Orada düğümün adını taşıyan kendi girişini ve kendi klasöründe bölüm başına bir depo alır. Ana örnek her depoya kendi anahtarını ekler ve onları kendisi toparlar, çünkü düğüm bunu yapamaz.

Düğüm sunucuya, ikinci adımda ya da **Alıcı sunucu** penceresinde ayarladığın **Düğümler için adres** üzerinden ulaşır. Düğüm genellikle internetten gönderir, bu yüzden adres şunlardan biri olmalıdır:

- `https://`, örneğin bir ters proxy, Pangolin ya da bir tünel arkasında. Sunucu herkese açık bir sertifika yetkilisinin sertifikası yerine kendi sertifikasını gösteriyorsa, onu PEM biçiminde **Sertifika** alanına yapıştır.
- Özel bir IP adresiyle `http://`, örneğin bir WireGuard bağlantısında `10.0.0.5` ya da Tailscale'de `100.101.102.103`.

Herkese açık bir adrese düz `http://` reddedilir, çünkü düğüm yedeklerini internet üzerinden şifresiz gönderirdi. Adresi değiştirdiğinde ana örnek bunu alıcıdaki her düğüme bildirir, o sırada uzakta olan bir düğüm de döndüğünde öğrenir.

### Bir hedef {#to-location}

Düğüm, ana örneğin kendisinin de ulaşabildiği hedeflerinden birine yazar: `https://` üzerinden ya da özel bir adreste bir S3 hizmetine veya bir REST sunucusuna. Depoları orada `bombvault-node-<node id>-containers` ve `bombvault-node-<node id>-files` altında durur.

Düğüm, kendisi için oluşturulmuş [kayıtlı bir oturum açma bilgisiyle](offsite-recovery.md#sign-ins) oturum açar, ve bu bilgi ana örneğin hedefe yazarken kullandığından farklı olmalıdır. Pencere hiç olmadığını söylüyorsa önce bir tane ekle. Bu oturum açma bilgisiyle düğüm, alıcı sunucunun aksine, hedefteki şeyleri genellikle silebilir ve pencere bunu belirtir.

### Düğümde tut {#keep-on-node}

Düğüm yedeklerini, append-only kipte kendi rest-server'ının arkasında kendisi tutar, ana örnek de her bölümün bir kopyasını kendi deposuna çeker. Bu, ana örneğin yer sunamadığı bir düğüme uyar. Açık port gerektiren tek hedef budur.

![Düğümde tut seçili ikinci adım, https://203.0.113.7:8443 adresi ve açık porta dair not.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Düğümde tut seçili ikinci adım, https://203.0.113.7:8443 adresi ve açık porta dair not.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. **Düğümün adresi** alanına ana örneğin düğüme nereden ulaşacağını yaz: düğümün herkese açık adresi ve boş bir port, örneğin `https://203.0.113.7:8443`, sonunda klasör olmadan. `http://` yalnızca özel bir adresle kullan, yukarıdaki gibi.
2. Düğüm bu portta `bombvault-node-rest-server` konteynerini başlatır, depoları Host Data altındaki `bombvault-node` klasöründe durur. `https://` ile o adres için bir sertifika üretir, ana örnek de onu sabitler.
3. Ana örnek ulaşabilsin diye portu düğümün güvenlik duvarından geçir. Düğümdeki başka bir konteynerin tuttuğu bir port, o konteynerin adıyla birlikte reddedilir.

Ana örnek bir bölümü, düğümdeki yedekleri biter bitmez çeker, yedek olarak da her saat başını çeyrek geçe, **Çekme** kapalıyken bile. Ana örneğin çektikleri, düğüm gittikten sonra bile her [çekilen kopya](offsite-recovery.md#receiver) gibi ana örnekte kalır. Rest-server'ı silme kabul etmediği için düğüm kendi depolarını ana örnek istediğinde kendisi toparlar, klasör kümeleri de `bombvault-node` klasörünü dışarıda bırakır, böylece kendi yedeklerini asla yedeklemez.

## Düğümler ne zaman yedekler {#schedule}

Her düğümün zamanlaması ana örneğe aittir. Kart her bölüm için bir plan gösterir, örneğin **Konteynerler** *her gün 2:00'de* ve **Klasörler** *haftalık (Paz) 4:00'te*. Tıklamak, [Zamanlamalar](schedules.md#plans) altındakiyle aynı düzenleyiciyi açar ve değişiklik hemen kaydedilir.

![Kartından açılan, bir düğümün konteyner planı: Kapalı, Günlük, Haftalık, Her N günde bir ve Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Kartından açılan, bir düğümün konteyner planı: Kapalı, Günlük, Haftalık, Her N günde bir ve Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Uzakta olan bir düğüm** kaçırdığı yedekleri döner dönmez başlatır. Ana örnek beş dakikada bir bakar.
- **Meşgul bir düğüme** beş dakika sonra yeniden sorulur.
- **Ana örnek bir düğümün eski yedeklerini toparlarken**, düğüm ana örnek işini bitirene kadar hiçbir şey başlatmaz. Alıcı sunucuda ve düğümün kendisinde bu pencere her pazar 03:00'te açılır ve en fazla altı saat sürer. Bir hedefte, ana örneğin o hedef için tuttuğu site dışı kopya planını izler. Ana örnek yedekleri, bölümün **Ne kadar saklanır** altındaki kendi kuralına ya da hedefin bir kuralı varsa onun kuralına göre saklar.
- **Düğümün çalışma listesinde**, ana örneğin başlattığı yedekler ve geri yüklemeler ana örnek tarafından başlatılmış olarak görünür.

## Düğümleri izlemek {#watch}

Genel bakıştaki **Düğümler** kartı her düğümü durumu, son yedeği, hataları ve açık anormallikleriyle listeler. Bir düğümün olduğunda görünür.

![Genel bakışta üç düğümlü Düğümler kartı: biri bağlı, biri iki hatayla etkin değil, biri hâlâ düğümü bekliyor.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Genel bakışta üç düğümlü Düğümler kartı: biri bağlı, biri iki hatayla etkin değil, biri hâlâ düğümü bekliyor.](assets/screenshots/nodes/overview-card-light.png#only-light)

Ana örnek, [bildirim kanalları](notifications.md) üzerinden her olay için bir uyarı gönderir, bir sonrakini de ancak o olay bittikten sonra:

- **Bir yedek başarısız oldu**, örneğin *BombVault: vps-helsinki: backup failed*, öge ve nedeniyle. O ögenin sonraki başarılı bir yedeği olayı bitirir.
- **Düğüme ulaşılamıyor**, örneğin *BombVault: vps-helsinki is out of reach*. Ana örnek, düğümün depolarındaki en kısa ölü adam anahtarı kadar bekler, değiştirmezsen 26 saat, düğümün henüz hedefi yokken 6 saat.

Ana örneğin alıcı izlemesi, düğümün her deposuna da bir ölü adam anahtarı ve 04:00'te günlük bir `restic check` ile göz kulak olur, bkz. [Alıcı ve çeken](offsite-recovery.md#receiver).

## Bir düğümün ögeleri {#entries}

**Yedekler** altında Konteynerler ve Klasörler listeleri, kendi ögelerinin altında her düğüm için bir bölüm içerir. Düğümün kendi konteyneri listede yer almaz. Bir ögenin menüsündeki **Şimdi yedekle** onun yedeğini düğümde başlatır, **Aç** da sayfasını açar.

Klasörler listesinde bir düğümün bölümünde **Klasör seti ekle** düğüme bir klasör seti ekler, bir setin menüsündeki **Düzenle** de onu değiştirir. İkisi de kendi setlerin için açılan pencerenin aynısını açar. Penceredeki klasör seçici düğümün klasörlerine göz atar, ama orada yeni klasör oluşturmaz.

![Konteynerler listesinde vps-falkenstein düğümünün bölümü: nextcloud, nextcloud-db, caddy ve uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Konteynerler listesinde vps-falkenstein düğümünün bölümü: nextcloud, nextcloud-db, caddy ve uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Bir düğüm ögesinin sayfası, örneğin **vps-falkenstein üzerinde** diyerek düğümü gösterir ve yedeklerini listeler. Bunlardan birinin yanındaki **Geri yükle**, ögeyi düğümde geri yükler ve şu an orada olanın yerine koyar. Bunun için konteyner, ana örnekte olduğu gibi durdurulur.

![vps-falkenstein üzerindeki nextcloud sayfası, yedekleri ve her birinin yanında bir Geri yükle düğmesi.](assets/screenshots/nodes/entry-dark.png#only-dark)
![vps-falkenstein üzerindeki nextcloud sayfası, yedekleri ve her birinin yanında bir Geri yükle düğmesi.](assets/screenshots/nodes/entry-light.png#only-light)

Ana örnek üzerinden düğümün kendi rotalarından oluşan sabit bir listeye ulaşırsın: konteynerleri, klasör kümeleri, çalışmaları ve anormallikleri ile bunları yedeklemek ve geri yüklemek. Düğümün ayarları, anahtarları ve deposu erişim dışında kalır, o listedeki hiçbir şey de silme yapmaz. Bkz. [API'de düğümler](api.md#nodes).

## Değiştirmek ve serbest bırakmak {#replace-release}

**Değiştir**, parolayla oturum açtıktan sonra düğümün compose dosyasını yeniden gösterir. Düğümü başka bir sunucuya taşımak ya da yeniden kurmak için kullan: dosyayı orada başlat ve eski sunucuyu kapat. Aynı `APP_KEY` ve `NODE_HOME` onu yeniden aynı düğüm yapar, yedekleri ve geçmişiyle birlikte. Aynı `APP_KEY` ile çalışan iki sunucu tek düğüm sayılır, bu yüzden ikisini asla birlikte çalıştırma.

**Serbest bırak** bir düğümü bırakır. Düğüm ana örnekten komut almayı bırakır ve artık hiçbir şey yedeklemez. Yedekleri olduğu yerde kalır, ana örneğin çektiği her kopya da öyle. Alıcı sunucudaki girişi kaldırılır ve ana örnek depolarını izlemeyi bırakır. Serbest bıraktığında uzakta olan bir düğüm, bunu öğrenene kadar kartını **Serbest bırakılıyor** olarak korur. Serbest bırakılmış bir düğüm aynı `APP_KEY` ile bir daha hiçbir ana örneği kabul etmez. Sunucuyu yeniden kullanmak için onu yeni bir düğüm olarak ekle ve yeni compose dosyası ile boş bir `config` klasörüyle başlat.

## Güvenlik {#security}

Ana örnek ele geçirilirse saldırgan onun düğümlerini de yönetir, düğüm de kendi sunucusunun Docker'ını elinde tutar, yani orada root'tur. Ana örneği bir parolayla koru ve internetten uzak tut. Bir düğüm ele geçirilirse saldırgan ne eşleştirme grubunu, ne kaleyi, ne başka bir düğümü ne de başka bir depoyu alır ve alıcı sunucuda geçmişi silemez. Ayrıntılar [Güvenlik modelinde düğümler](configuration.md#nodes) altındadır.

[Kurtarma kitinde](offsite-recovery.md#recovery-kit) her düğümün `APP_KEY` anahtarını, restic parolasını ve depolarını içeren bir **Düğümler** bölümü vardır, böylece bir düğümü geri getirebilir ya da yedeklerini düz restic ile geri yükleyebilirsin.
