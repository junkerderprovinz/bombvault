# Depolama konumları

Depolama konumu, BombVault'un yedekleri tuttuğu bir yerdir: bu Unraid'deki bir klasör, bir NAS'taki bir paylaşım, bir bulut sağlayıcısındaki bir bucket, bir rest-server, bir SFTP hesabı ya da bir Nextcloud. Her konumu **Ayarlar, Depolama** sekmesinde bir kez bağlarsınız; kimlik bilgileri, saklama kuralları, koruması ve bulunduğu yer o konuma aittir. Etki alanları (konteynerler, VM'ler, flash, BombVault'un kendi yapılandırması, dosya kümeleri ve ZFS veri kümeleri) ardından bu konumlar arasından seçim yapar: her etki alanının nerede depolanacağını ve nereye kopyalanacağını. ZFS etki alanı açık olduğu sürece ZFS veri kümelerinin Alanlar kartında kendi satırı vardır.

## Konum ekleme {#add-a-place}

**Konum ekle**, sağlayıcı başına bir kutucuk içeren bir pencere açar. Kutucuklar üç gruptadır: bulut depolama, kendi sunucunuzda çalışan hizmetler, NAS cihazları ve bu sunucu.

1. Bir kutucuk seçin ve formunu doldurun. Göz düğmesi yazdığınız gizli bilgiyi gösterir.
2. **Bağlantıyı test et** konumu denetler ve hiçbir şey oluşturmaz. Her etki alanının klasörü için ne bulduğunu bildirir: boş ya da henüz yok, zaten bir restic deposu içeriyor ya da onu durduran hata.
3. Konuma bir ad verin; sağlayıcının adı önceden doldurulur. Kendiniz çalıştırdığınız bir cihaz için **Cihaz nerede?** sorusunu yanıtlayın. Bulut sağlayıcıları her zaman başka bir yerdedir, bu Unraid'deki bir klasör ise her zaman buradadır.
4. **Ekle** konumu kaydeder.

Yeni bir konumu henüz hiçbir etki alanı kullanmaz. Onu [Alanlar kartında](#domains) **Depolama konumu** ya da **Kopyalama hedefleri** altında seçin, ya da yalnızca tek bir öge için o ögenin kartında.

## Klasörler {#folders}

Bir konum her etki alanı için bir klasör tutar: `container`, `vms`, `flash`, `config`, `files` ve `zfs`, yani varsayılan yedekleme yollarının kullandığı adlar. Klasörler konumun ayrıntılarında listelenir ve orada yeniden adlandırılabilir (bkz. [Adres değiştirme](#addresses)). Bir konumda klasörü olmayan bir etki alanı o konumu seçemez.

Bir etki alanı bir konumu iki rolde birden kullandığında ikinci rol bir sonek alır, ilk rol klasörünü korur. Bir etki alanının kopyalarını zaten alan bir konum, doğrudan ona gönderilen ögeleri `<folder>-direct` içinde depolar; bir etki alanını zaten depolayan bir konum ise o etki alanının kopyalarını `<folder>-copies` içinde alır.

Bazı konumlar kendileri bir restic deposudur: konum eklendiğinde zaten bir depo içeren bir adres, mevcut bir kurulumdan gelen adlandırılmış bir depo ya da bir bucket'ın kökündeki bir kopyalama hedefi. Böyle bir konumun klasörü yoktur, her etki alanı onun tek deposunu paylaşır ve konum ikinci bir rol üstlenmez. Aynı sağlayıcıda daha fazlasını depolamak için başka bir bucket'ı ya da klasörü ayrı bir konum olarak bağlayın.

## Konum ayrıntıları {#details}

Her konum bir satırdır; satırda sağlayıcısı, ne için kullanıldığı ve son testi ya da son kopyası görünür. **Test et** konumdaki her adresi denetler, **Ayrıntılar** ise konumun ayarlarını açar. Ayrıntılardaki her değişiklik yapıldığı anda kaydedilir.

- **Genel**: ad, konumu açıp kapatan anahtar, adres ve kendiniz çalıştırdığınız bir cihaz için **Cihaz nerede?** (bkz. [Bina dışında](#off-the-premises)).
- **Saklama**: son-tut, günlük, haftalık ve aylık; konumdaki her depo için. Yeni bir konum varsayılan kurallarla başlar; her kuralı sıfır olan bir konum hiçbir şeyi kırpmaz.
- **Koruma**: **Append-only** anahtarı. Append-only'yi karşı tarafın uygulaması gerekir; anahtar açıkken BombVault orada asla budama ya da silme yapmaz. Append-only açık bir rest-server'da **Append-only'yi test et**, konumdaki her etki alanı yolu, açık kopya ve depo için kurcalama testini çalıştırır ve tüm konum için tek bir yanıt gösterir: *silme reddedildi* ya da *silme kabul edildi* (bkz. [Site dışı ve kurtarma](offsite-recovery.md)). Bu bölüm yalnızca uzak konumlarda vardır, çünkü bu makinedeki hiçbir şey yerel bir deponun silinmesini engelleyemez.
- **Erişim**: kimlik bilgileri ve S3 için depolama sınıfı. Ortak kimlik bilgilerini kullanan bir konum, ilk değişiklikte kendine ait bir küme alır. Konumdaki doğrudan bir depoyu yeni kimlik bilgileri açamıyorsa o depo eskilerini korur ve yanıt bunu belirtir. Klasör, SFTP ve rclone konumlarında bu bölüm yoktur.
- **Sınırlar**: yükleme ve indirme hızı ile büyüme bütçesi.
- **Klasörler**: her etki alanı için, klasörünün adıyla birlikte bir anahtar. Burada kapatılan bir etki alanı bu konumu seçemez.

Saklamayı düşürmek önce sorar ve bunun kaç ögeyi etkilediğini söyler; append-only'yi kapatmak önce sorar ve konumdaki kaç deponun bu korumayı kaybettiğini söyler. Bir konumu kapatmak oradaki her depoyu kapatır; bir etki alanının depolandığı konum kapatılamaz.

## Alanlar kartı {#domains}

Kartta her etki alanı için bir satır vardır: zamanlaması, nerede depolandığı, nereye kopyalandığı ve istisnaları.

- **Depolama konumu**: etki alanının yedekleme yolunda henüz yedek yokken seçilen konum etki alanının ana konumu olur ve yedekleme yolu oraya taşınır. Yolda yedekler bulunduğunda ise konteynerler, VM'ler ve dosya kümeleri için seçim yeni ögelerin varsayılanı olur ve yeni ögeler onu ilk yedeklemelerinde alır. Zaten yedeği olan ögeler olduğu yerde kalır, çünkü BombVault bir yedeği asla taşımaz. Flash ve BombVault'un kendi yapılandırması için ana konum taşınır, zaten yazılmış yedekler eski konumda kalır.
- **Kopyalama hedefleri**: etki alanının kopyalarını alabilen her konum için bir çip. Bir çipi işaretlemek, konumu etki alanının bir kopyalama hedefi yapar; ilk seferde BombVault, ilk çalıştırmanın kaç öge ve anlık görüntü ile ne kadar veri göndereceğini önceden söyler. İşareti kaldırmak yeni kopyaları durdurur: oradaki kopyalar kalır ve konumun saklama kurallarına göre yaşlanır, kendi seçimi olan ögeler ise oraya kopyalamaya devam eder. Son çipin işaretini kaldırmak, yeniden bir çip işaretlenene kadar tüm kopyaları durdurur, sonradan eklenen konumlara gidecek olanları da. Kapatılmış bir konum soluk bir çip olarak görünür ve seçilemez.
- **İstisnalar**: kendi seçimi olan ögeler, kartlarına bağlantılarla bir liste olarak.
- **Şimdi kopyala** etki alanının kopyalarını hemen çalıştırır.

Discover üzerinden bir yeniden kurulumdan sonra duraklatılan bir etki alanı, duraklamayı satırında **Varsayılanı onayla** ile birlikte gösterir (bkz. [Öge başına yerleşim](offsite-recovery.md#placement)).

## Adres değiştirme {#addresses}

Bir etki alanının klasörü konumun ayrıntılarında değiştirilebilir; yerel bir konumun adresi de, örneğin bir depo elle başka bir diske taşındıktan sonra. BombVault değişikliğin etkilediği her adresi test eder ve değişikliği iki durumda kabul eder: her yeni adres boşsa ve eski adreste hiçbir şey depolanmamışsa, ya da her yeni adres eskisiyle aynı restic deposunu içeriyorsa. Bunun dışındaki her şey, eski adreste hâlâ duran yedeklerin sayısıyla birlikte reddedilir. Uzak bir konum adresini korur; başka bir yere yedeklemek için orayı ayrı bir konum olarak bağlayın.

BombVault konum listesini kendi veritabanından oluşturur ve listeyi doldurmak için hiçbir uzak depoyu listelemez; test yalnızca bir şeyi değiştirdiğinizde çalışır.

## Konum kaldırma {#remove}

Bir konum ancak hiçbir şey onu kullanmıyorken kaldırılabilir: orada hiçbir etki alanı depolanmıyorsa, hiçbir varsayılan onu göstermiyorsa, orada hiçbir öge depolanmıyorsa ve oradaki hiçbir doğrudan depo öge tutmuyorsa. Aksi halde ret yanıtı konumu neyin tuttuğunu listeler. Kaldırma, konumun kopyalama hedeflerini ve, bir çekme kaynağı ya da başka bir konum kullanmıyorsa, kendi kimlik bilgilerini de birlikte kaldırır. Depolamanın kendisinde hiçbir şey silinmez ve onay, orada kaç kopyanın kaldığını söyler.

## Konumu olmayanlar {#without-a-place}

Konum artı klasör biçimine uymayan bir adres çalışmaya devam eder ve adresiyle birlikte **Konumu olmayanlar** altında listelenir. restic'in kendi `b2:`, `gs:` ve `swift:` adresleri bunlar arasındadır. **Konuma ata**, böyle bir satırı [adres değiştirme](#addresses) ile aynı testten sonra bir konuma bağlar. Konumu olmayan bir kopyalama hedefi ayrıca etki alanının satırında, çiplerin yanında adıyla gösterilir ve kopyalamaya devam eder. Oradaki uzak bir satırın kendi **Append-only** anahtarı vardır ve onu kapatmak, o adreste yedek tutan öge sayısıyla önce sorar. Doğrudan bir depo, hedefinin anahtarını izler.

## Bina dışında {#off-the-premises}

**Cihaz nerede?** sorusunun iki yanıtı vardır: **Burada, bu binada** ve **Başka bir yerde**. Bir kopya, kartlardaki 3-2-1 satırı ve Kontrol Paneli'nin site dışı denetimleri için, yalnızca konumu başka bir yerdeyse ayrı bir site sayılır. Aynı binadaki ikinci bir disk ya da NAS ikinci bir kopyadır, ikinci bir site değil. Yanıt hiçbir kopyayı değiştirmez. Bulut sağlayıcıları her zaman başka bir yerdedir, bu Unraid'deki bir klasör ise her zaman buradadır, bu yüzden form bunlar için sormaz; diğer her konum için yanıtı konumun ayrıntılarında değiştirin. Başka bir yerdeki bir konum, satırında **Başka yerde** işaretini taşır.

## Bağlantı türleri

### Bu Unraid'deki bir klasör ya da NAS {#kind-local}

Adres `/mnt` altında bir yoldur ve `/mnt` olmadan yazılır, örneğin `user/bombvault`; her etki alanının klasörü onun altında durur: `user/bombvault/container`.

- **Bu Unraid'de bir klasör** paylaşımlar, diskler ve havuzlar arasından seçer.
- **Synology**, **QNAP**, **TrueNAS**, **Başka bir Unraid** ve **Başka bir paylaşım** `/mnt/remotes` içinden seçer. Paylaşımı önce Unraid'e bağlayın, örneğin Unassigned Devices eklentisiyle. Host Data, Read/Write - Slave olarak bağlanmış olmalıdır; aksi halde BombVault başladıktan sonra bağlanan bir paylaşım bir yeniden başlatmaya kadar görünmez kalır (bkz. [Yapılandırma](configuration.md)).

Klasör seçici, bulunduğu yerde **Yeni klasör** ile bir klasör oluşturur. Test, klasörün boş olduğunu ya da henüz bulunmadığını ve BombVault'un oraya yazabildiğini denetler.

### S3 {#kind-s3}

Adres `s3:https://<endpoint>/<bucket>/<path>` biçimindedir, örneğin `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** yalnızca anahtar kimliğini ve uygulama anahtarını ister. BombVault, anahtarın hangi bucket, S3 uç noktası ve klasörle sınırlı olduğunu B2'ye sorar ve adresi bunlardan oluşturur. Her bucket'a erişebilen bir anahtarda bucket'lar seçim için listelenir.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** ve **Vultr** anahtarı ve, sağlayıcı gerektiriyorsa, bölgeyi, hesap kimliğini ya da uç noktayı ister. BombVault uç noktayı doldurur ve anahtarın izni varsa bucket'ları listeler; yoksa bucket'ın adını yazın.
- **Google Cloud Storage**, S3 arayüzü üzerinden, Cloud Storage ayarlarında Interoperability altında oluşturulan bir HMAC anahtarıyla bağlanır. Bir hizmet hesabı dosyası burada çalışmaz.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** ve **Başka bir S3 hizmeti** hizmetin adresini ve bir anahtarı alır.

Depolama sınıfı konumun ayrıntılarında ayarlanır ve bir geri yüklemenin önce çözülme beklemeden okuyabildiği katmanlarla sınırlıdır.

### rest-server {#kind-rest}

Adres `rest:<url>/<user>` biçimindedir, örneğin `rest:https://nas.lan:8000/tower`. Form sunucunun adresini, bir kullanıcıyı ve bir parolayı ister. `--private-repos` ile bir kullanıcı yalnızca kendi adıyla başlayan yollara erişebilir, bu yüzden siz başka bir yol yazmadıkça BombVault kullanıcıyı yolun başına koyar. Sunucu, kullanıcının kendi yolu dışındaki bir yolu reddettiğinde hata bunu söyler.

rest-server formu, bu BombVault için tek kullanıcılı, append-only modunda çalışan bir rest-server için yapıştırmaya hazır bir tarif taşır. **Tarifi göster** yalnızca bir kez gösterilen bir parola üretir ve bir `docker run` satırı, bir compose dosyası ve bir Unraid şablonu verir; her birinde sunucuya konacak `htpasswd` satırı da vardır. Kullanıcı ve parola doğrudan forma yazılır.

**Başka bir BombVault**, kendi form alanlarının üstünde diğer örneklerin Fleet üzerinden gönderdiği açık teklifleri listeler. Birini kabul etmek, yalnızca teklif edilen etki alanının kopyalarını tutan bir konum ekler, çünkü bir teklif yalnızca o etki alanı için bir kullanıcı taşır. Fleet sayfasında kabul etmek de aynı konumu ekler.

### SFTP {#kind-sftp}

Adres `sftp://<user>@<host>:<port>/<path>` biçimindedir, örneğin `sftp://bv@backup.lan:22/bombvault`. Form host, port ve kullanıcıyı ister ve BombVault'un genel anahtarını gösterir. Bu anahtarı sunucuda kullanıcının `~/.ssh/authorized_keys` dosyasına ekleyin; orada başka hiçbir şeyin kurulması gerekmez. BombVault sunucunun host anahtarını ilk temasta kabul eder ve o andan itibaren denetler.

**Hetzner Storage Box** `<user>.your-storagebox.de` adresini ve 23 numaralı portu doldurur. Anahtarı kutuya Hetzner'in kendi komutuyla kurun; komut kutunun parolasını bir kez sorar:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Form sunucunun adresini, kullanıcıyı ve bir uygulama parolasını ister. Uygulama parolasını hesabın güvenlik ayarlarında oluşturun ve e-posta adresi yerine kullanıcı kimliğini girin. BombVault ürünün kullandığı WebDAV yolunu oluşturur ve bağlantıyı restic'e rclone'un ortam değişkenleri üzerinden iletir; parola rclone'un gizlenmiş (obscured) biçimindedir. Adres `rclone:bvp<id>:<path>` şeklinde görünür; burada `bvp<id>` yalnızca o ortamda var olan bir uzak tanımdır. rclone yapılandırmasına hiçbir şey yazılmaz.

### Azure Blob {#kind-azure}

Adres `azure:<container>:/<path>` biçimindedir. Form depolama hesabını ve erişim anahtarını ister; **Bağlantıyı test et**'ten sonra hesabın kapsayıcılarını seçmeniz için listeler, ya da bir kapsayıcının adını kendiniz yazarsınız. BombVault hesabı ve anahtarı restic'e `AZURE_ACCOUNT_NAME` ve `AZURE_ACCOUNT_KEY` olarak iletir.

### rclone {#kind-rclone}

Adres `rclone:<remote>:<path>` biçimindedir. Form, BombVault'un rclone yapılandırmasındaki uzak tanımları seçmeniz için listeler. Bu yapılandırmayı değiştirmek için **rclone yapılandırması** altına bütün bir `rclone.conf` yapıştırın ve **Yapılandırmayı kaydet**'e tıklayın. Yapılandırma hemen kaydedilir ve pencere ardından bir konum eklese de eklemese de her rclone konumu için geçerli olur.

## Farklı kimlik bilgilerine sahip konumlar arasında kopyalar {#different-credentials}

Uzak bir konumda depolanan bir etki alanı, kopyalarının kaynağıdır. `restic copy` tek bir ortamla çalışır ve BombVault, ikisi aynı değişkene farklı değerler vermiyorsa kaynağın kimlik bilgilerini hedefinkilere ekler. Bir Nextcloud konumu ile bir B2 konumu farklı değişkenler kullanır, bu yüzden Nextcloud'da depolanan bir etki alanı B2'ye kopyalanabilir. İki S3 hesabı ya da iki rest-server kullanıcısı aynı değişkenlere farklı değerlerle ihtiyaç duyar; restic ikisini birden alamaz ve Alanlar kartındaki çip kimlik bilgilerinin uymadığını söyler.
