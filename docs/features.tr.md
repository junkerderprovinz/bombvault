# Özellikler

BombVault'un neler yaptığının kısa bir kataloğu. Her satır, onu açıklayan sayfaya bağlanır.

## Yedekleme kapsamı {#backup-scope}

- **Konteynerler**: appdata klasörü ve konteyner tanımı, konteyner başına klasör seçimiyle. [Yedekler ve ögeler](backups.md#what-is-saved)
- **VM'ler**: disk image'leri, XML ve UEFI NVRAM SSH üzerinden, düzgün kapatma, canlı anlık görüntü ya da yalnızca değişen bloklarla. Unraid anlık görüntülü VM'ler ve zvol üzerindeki diskler tam olarak desteklenir. [VM yedekleri](vm-backup-ssh-setup.md)
- **Flash**: tüm Unraid USB flash, zamanlanmış bir zip dışa aktarmasıyla. [Yedekler ve ögeler](backups.md#settings-tab)
- **Öz yedek**: BombVault'un kendi ayarları ve oturum açma bilgileri. [Acil durum sihirbazı](offsite-recovery.md#emergency)
- **Klasörler**: sunucudaki herhangi bir klasör, bir klasör kümesi olarak. [Klasör ekleme](backups.md#add)
- **ZFS veri kümeleri**: tek bir anlık görüntüden okunan bir veri kümesi ağacı, tek tek dosyalar geri yüklenebilir ve ikinci bir ZFS sunucusunda isteğe bağlı replika. [ZFS veri kümeleri](zfs-datasets.md)
- **Veritabanı dökümleri**: PostgreSQL, MySQL ve MariaDB, çalışan sunucudan kendi geri yükleme noktasına dökülür. [Veritabanı dökümleri](backups.md#database-dumps)

## Yedekler ve öge sayfaları {#backups}

- **Tek liste**: kutucuklar, **Filtreler**, **Sırala** ve **Ara** ile, her satırın son on dört çalıştırması bir şerit olarak. [Liste](backups.md#list)
- **Dosya yöneticisi gibi seçim**, ve bir seçimi sunucuda yedekleme ya da duraklatma. [Birden çok öge seçmek](backups.md#select)
- **Her öge için bir sayfa**: **Yedekler**, **Ayarlar** ve **Doğrulama** sekmeleriyle. [Bir ögenin sayfası](backups.md#entry-page)
- Anlık görüntüler için **etiketler ve karşılaştırma**, ve tar, zip ya da bir restic deposu olarak **Dışa aktar**. [Yedekler sekmesi](backups.md#backups-tab)
- Canlı önizlemeli **Hariç tutma desenleri** ve tanınmış uygulamalar için önerilen hariç tutmalar. [Hariç tutma desenleri](backups.md#excludes)
- **Öncesi ve sonrası komutları** ve bağımlılık sırasıyla sağlık denetimli yeniden başlatmaya sahip **Diğer konteynerleri durdur**. [Komutlar ve diğer konteynerler](backups.md#commands)
- **Uygulama meşgulken**: zamanlanmış bir yedek Plex'i, Jellyfin'i, Emby'yi, kendi denetimini ya da sakin trafiği bekler ve kendiliğinden yeniden dener. [Uygulama meşgulken](backups.md#app-busy)
- **Durdurma yerine anlık görüntü**: ZFS üzerindeki bir konteyner yalnızca birkaç saniye kapalı kalır. [Zamanlamalar](schedules.md#snapshot-instead-of-stop)
- İsteğe bağlı olarak age ile mühürlenen **düz dışa aktarmalar**. [Düz dışa aktarmalar](backups.md#exports)

## Geri yükleme {#restore}

- Önerilen yedek önceden seçili, her tür için **tek sayfa ve tek pencere**. [Geri yükleme](restore.md)
- Varsayılan olarak **Orijinalin yanına**, ve **Orijinali değiştir** önce mevcut durumu yedekler. [Geri yükleme penceresi](restore.md#window)
- Bir klasöre ya da yerlerine **tekil dosyalar**. [Tek bir dosyayı geri al](restore.md#example-single-file)
- Her biri önerilen yedeğinden ya da tek bir andan **birden çok öge bir kerede**. [Birden çok ögeyi geri yüklemek](restore.md#several)
- Bağımlılık sırasıyla başlatılan **tüm Compose stack'leri**. [Bir stack'i geri yüklemek](restore.md#stacks)
- GPU, sınırlar ve bağlantılarla, çalıştıkları image üzerinde **yeniden oluşturulan konteynerler ve VM'ler**. [Neler geri gelir](restore.md#kinds)
- **Geri yüklemeden önce denetimler** ve neyi değiştirdiğinin bir listesi. [Geri yükleme penceresi](restore.md#window)
- **ZIP olarak flash** ve çalışan flash'a her seferinde bir eklenti. [Neler geri gelir](restore.md#kinds)
- Güvenlik anlık görüntüsüyle veri kümesinin içine, bir klasöre ya da özellikleriyle yeni bir veri kümesine **ZFS**. [ZFS veri kümeleri](zfs-datasets.md#restore)
- Kayıp bir sunucu için kurtarma kitiyle, grup üzerinden ya da yazılmış anahtarla **Acil durum sihirbazı**. [Acil durum sihirbazı](offsite-recovery.md#emergency)
- **Eski bir yedek klasöründen**, **Appdata.Backup eklentisinden içe aktar** ve onların yedekleme işlerini klasör kümelerine dönüştüren **Duplicati ya da Duplicacy işleri**. [Diğer kaynaklar](restore.md#other-sources)

## Depolama ve zamanlama {#storage-scheduling}

- Her türden **hedefler**: yerel klasörler, SMB, WebDAV, SFTP, rest-server, S3, bulut sürücüleri ve herhangi bir rclone remote'u. [Hedefler](offsite-recovery.md#destinations)
- **Oturum açma bilgileri**: varsayılanı olan tek liste. [Oturum açma bilgileri](offsite-recovery.md#sign-ins)
- Bu sunucuda ya da bir hedefte **kendi klasörleri**. [Bir bölümün kendi klasörü](offsite-recovery.md#own-folder)
- Her satırın bir anahtar olduğu bir akış olarak **3-2-1** ve **öge başına yerleşim**. [Öge başına yerleşim](offsite-recovery.md#placement)
- Bölüm başına birden çok konuma **site dışı kopyalar**, bant genişliği sınırları, sıkıştırma, soğuk depolama sınıfları ve Plex yayın yaparken yavaşlama ile. [Site dışı kopyalar](offsite-recovery.md#off-site-copies)
- **Doğrudan buraya yedekleniyor** altında listelenen **doğrudan depolar**. [Doğrudan depolar](offsite-recovery.md#direct-repositories)
- Bir zaman çizelgesi olarak **saklama**: hedef başına **Kısa**, **Dengeli**, **Uzun** ya da **Özel değerler**, bölüm başına kurallarla. [Ne kadar saklanır](offsite-recovery.md#keep)
- **Bir bölümün geçmişini taşı** ve yedekleriyle ya da yedeksiz **Hedefi kaldır**. [Taşıma ve kaldırma](offsite-recovery.md#move-remove)
- **Cümle gibi okunan zamanlamalar**, **Hafta** şeridi, tam yedekleme, bölüm başına planlar ve öge başına kendi plan. [Zamanlamalar](schedules.md)
- **Kaçırılan yedekleri telafi etme**, elle sıra ve başarılı bir yedeklemeden sonra güncellemeler. [Zamanlamalar](schedules.md)

## İçgörü, doğrulama ve izleme {#insight-verification-monitoring}

- **Genel bakış**: durum başlığı, **İlgi bekliyor**, **Koruma durumu**, depolama tahmini, geçmiş ve isteğe bağlı kartlar. [Genel bakış](overview.md)
- Bir kaynak çöktüğünde eski yedekleri de tutan, öge başına **anormallik algılama**. [Anormallikler](anomalies.md)
- İlk yedekten sonra **geri yükleme denetimi** ve yalıtılmış bir kopyada **Başlatma testi**. [Doğrulama sekmesi](backups.md#verify-tab)
- Site dışı kopyadan **zamanlanmış denetimler ve deneme geri yüklemeleri**. [DR tatbikatları](offsite-recovery.md#dr-drills)
- **Klasöre göre boyut** ve bir yedeğin neden yavaş olduğu. [Bir çalıştırma sana ne söyler](backups.md#run-notes)
- **Bildirimler**: Unraid, ntfy, Gotify, Discord, Slack, Matrix, e-posta, Apprise, herhangi bir webhook ve Healthchecks için, her birinin kendi kuralı olan bir kanal listesi, haftalık özet ve gecikme raporuyla. [Bildirimler](notifications.md)
- **Prometheus**, bir Home Assistant cihazı, bir HTTP API'si ve mDNS. [API ve entegrasyonlar](api.md)
- **Hata bildirimi**: bir GitHub issue'su için temizlenmiş bir ZIP. [Sorun Giderme](troubleshooting.md#still-stuck)

## Fidye yazılımı koruması {#ransomware-protection}

- Karşı tarafça uygulanan ve bir kurcalama testiyle kanıtlanan **silmeye karşı koruma (append-only)**. [Silmeye karşı koruma](offsite-recovery.md#delete-protection-append-only)
- Silmeyi reddedemeyen bir yer için **Silmeye karşı koruma olmadan kabul et**. [Silmeye karşı koruma](offsite-recovery.md#delete-protection-append-only)
- **Bir giriş parolası ayarlıysa yedekleri kalıcı silmek kendi oturum açmanı gerektirir**, eşleşmiş bir telefon bunu yapamaz. [Güvenlik modeli](configuration.md#security-model)
- İsteğe bağlı olarak age ile mühürlenen **kurtarma kiti** ve grubundaki bir **anahtar bekçisi**. [Kurtarma kiti](offsite-recovery.md#recovery-kit), [Anahtar bekçisi](offsite-recovery.md#key-keeper)

## Birden çok sunucu {#several-servers}

- Doğrudan ya da bir röle üzerinden, uçtan uca şifreli, **on iki kelimeyle eşleştirme**. [Örnekleri eşleştirme](offsite-recovery.md#pairing)
- Grubun bir ağ olarak göründüğü ve **Alıcı**, **Çeken**, **ZFS sunucusu** ve **Anahtar bekçisi** rollerinin olduğu **Örnekler sayfası**. [Örnekler ve roller](offsite-recovery.md#roles)
- Senin için kurulan, append-only kipinde **bir alıcı sunucu**. [Alıcı sunucu](offsite-recovery.md#receiving-server)

## Yapay zeka asistanları (MCP) {#mcp}

Yerleşik bir MCP sunucusu, bir asistanın yedek durumunu, kapsamı, çalıştırma geçmişini ve geri yükleme noktalarını okumasını, buna izin veren bir anahtarla da bir yedekleme başlatmasını sağlar. Geri yüklemeler, silmeler, budama ve ayarlar web arayüzünde kalır. [MCP sunucusu](mcp.md)

## Uygulamalar ve yardımcılar {#apps}

- **Android uygulaması**: grubundaki her sunucu telefonunda. [Android uygulaması](android.md)
- **BombVault Widget**: Unraid Panosunda bir kutucuk. [Uygulamalar](settings.md#apps)
- Homepage ya da Heimdall gibi panolar için **gömülebilir etkinlik günlüğü**. [Panel widget'ı](api.md#dashboard-widget)

## Diğer {#other}

- **42 dil**, üç tema ve istediğin etiketler, renkler, köşeler ve animasyon. [Görünüm](settings.md#look)
- Kendiliğinden kaydolan **ayarlar**, on dört kutucuğun hepsinde arama ile. [Ayarlar](settings.md)
- Bir güncellemeden sonra değişenlerde yeşil noktalarla **Yenilikler**. [Bilgi](settings.md#info)
- **Kutudan çıktığı gibi HTTPS** ve otomatik onarım araçları için bir sağlık denetimi. [Yapılandırma](configuration.md#network)
