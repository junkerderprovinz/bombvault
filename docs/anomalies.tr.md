# Anormallikler

Bir yedekte fidye yazılımı ile bozulmuş bir uygulama birbirine benzer: birden verinin çoğu yeni ya da çoğu yok olur. BombVault her yedeği o ögenin kendi geçmişiyle karşılaştırır ve bir şey yanlış görünüyorsa bir bulgu çıkarır. Kaybolan verilerle ilgili bir bulgu açıkken ögenin eski yedeklerini de saklar, böylece son iyi yedek eskiyip silinmez. Kenar çubuğundaki **Anormallikler** girişi, algılama açıkken görünür.

![Bir kritik bulgunun açık olduğu Anormallikler sayfası.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Bir kritik bulgunun açık olduğu Anormallikler sayfası.](assets/screenshots/anomalies/page-light.png#only-light)

## Sayfa {#page}

**Kritik**, **Uyarılar** ve **Notlar** kutucukları bir önem düzeyi seçer, listenin üstündeki **Açık** ve **Kapalı** ise açık olanlarla son 30 günde kapanmış olanlar arasında geçiş yapar. Kritik bir bulgu da diğerleriyle aynı yüzeyde durur, önem düzeyi rozetinden ve renkli kenarından anlaşılır. Bir bulgu önce çözümünü sunar, ayrıntılarında ise değer, olağan aralığın üzerinde bir eğri olarak görünür.

Hiçbir şey açık değilken **Öğrenme ilerlemesi**, her öge için on parçalı bir halka gösterir, BombVault'un öğrendiği her yedek için bir parça. **İzleme**, ögelerin kendi ayarlarıyla birlikte listesini açar.

## Neleri denetler {#checks}

Bir konteynerin, VM'in, klasör kümesinin, veritabanı dökümünün, flash sürücünün ve öz yedeğin her yedeği ögenin kendi geçmişiyle karşılaştırılır. Denetimler şunlara bakar:

- bir çalıştırmanın sakladığı yeni veri, yakın zamandaki yedeklerin en büyük olağan miktarlarına ve saat başına olağan hıza göre,
- verinin çoğunu yeniden saklayan bir yedek, yeniden adlandırılıp yeniden yazılan dosyalar da dahil,
- restic'in her öge ve her veritabanı dökümü için bildirdiği kaynak boyutu ve dosya sayısı,
- restic'in kendi yedekleme süresi,
- hata serileri ve aralıklı hatalar,
- artık geçmeyen geri yükleme denetimleri,
- yerel, SFTP ve rclone depolarının boş alanı, deponun büyümesinden tahmin edilerek.

Bir öge ilk 10 yedeğinden öğrenir. Neredeyse boş bir kaynak, verinin çoğunun yeniden yazılması ve hatalar en baştan denetlenir.

Bir kaynağın boyutu ya da dosya sayısıyla ilgili bir bulgu, bunun göründüğü ilk yedeğin tarihini alır. **Önceki yedekle karşılaştır**, dosyaların gittiği, geldiği ya da değiştiği klasörleri listeler, neredeyse hepsi bir arama dizininde, önbellekte ya da uygulamanın kendi kendine yeniden oluşturduğu küçük resimlerdeyse bir not düşer.

ZFS ögeleri veri kümesi veri kümesi izlenir, bkz. [ZFS veri kümeleri](zfs-datasets.md#anomalies). Bir asistan açık bulguları [MCP sunucusu](mcp.md#tools) üzerinden okuyabilir.

Denetimlerin sınırları var. S3, B2 ve REST depolarında boş alan değeri yoktur, Unraid kullanıcı paylaşımında boş alan tüm dizinin boş alanıdır ve restic 0.17'den önce alınan yedeklerde boyut geçmişi yoktur.

## Bir bulguyu kapatmak {#closing}

- **Uyarılar**, neden ortadan kalkınca kendiliğinden kapanır.
- Kaybolan veriler ve dolan bir disk hakkındaki **Kritik** bulgular, sen harekete geçene kadar kalır.
- **Onayla**, bir bulguyu görüldü olarak işaretler ve kapatır. Nedeni bir kez ortadan kalkmadan yeniden çıkarılmaz.
- **Beklenen olarak işaretle**, değişikliği senin yaptığını söyler. Yeni düzey 10 yedekten sonra normal olur, ama neredeyse boş denetimi hiçbir zaman kapatılmaz.
- Seçimde yapılan bir değişiklik, örneğin işaretlediğin ya da işaretini kaldırdığın bir klasör, ögenin geçmişini kendiliğinden baştan başlatır.

Bildirim bir olay için bir kez gider, zaten bildirim gönderen hatalar ve geri yükleme denetimleri iki kez bildirilmez.

## Eski yedekler bu arada saklanır {#retention-hold}

Bir kaynak neredeyse boşken, sert biçimde küçüldüğünde ya da verisinin çoğu yeniden saklandığında, saklama ve budama o ögenin eski yedeklerine dokunmaz, bulgu son iyi yedeğe bağlanır, geri yükleme penceresi de onu önceden seçer. Böyle bir bulguyu kapatmadan önce BombVault, sonraki temizliğin eski yedekleri yeniden saklama kurallarına göre sileceğini söyler. Bu yüzden veri eksikse ya da şifrelenmişse önce geri yükle, bulguyu sonra kapat.

## Ayarlar {#settings}

Genel ayarlar **Ayarlar, Doğrulama** altındaki **Anormallikler** kartındadır. Her denetim değiştirir değiştirmez kaydedilir, anahtarın altındaki üçü, algılama kapalıyken gizlenir.

| Ayar | Varsayılan | Ne yapar |
|---|---|---|
| **Anormallikleri algıla** | Açık | Her yedeği ögenin kendi geçmişiyle karşılaştırır. Kapatılırsa yeni bir şey denetlenmez ve **Anormallikler** girişi kenar çubuğundan kalkar. Kart yine de önceki bulgulara bağlanır. |
| **Duyarlılık** | Dengeli | Sıkı daha küçük değişiklikleri, esnek yalnızca büyük olanları bildirir. |
| **Şunlar için bildirim gönder** | Yalnızca kritik bulgular | [Bildirimler](notifications.md) altında kurulan kanallardan mesaj gönderen en düşük önem düzeyi. Tekrarlayan yedekleme ve döküm hataları ile başarısız zamanlanmış geri yükleme denetimleri zaten kendi mesajlarını gönderir ve iki kez gönderilmez. |
| **Bir kaynak sert biçimde küçüldüğünde veya yeniden yazıldığında eski yedekleri tut** | Açık | Bir ögede neredeyse boş bir kaynak, sert bir küçülme ya da verinin çoğunun yeniden saklanması için açık bir bulgu varken saklama ve budama o ögenin eski yedeklerine dokunmaz. Bırakılmalarını istiyorsan bulguyu onayla ya da beklenen olarak işaretle. |

Her öge kendi duyarlılığını ve bildirim alt sınırını kullanabilir. Bunları ögenin **Doğrulama** sekmesinde ya da **Anormallikler** sayfasında **İzleme** altında ayarla. ZFS ögesinde bunlar ağacındaki her veri kümesi için geçerlidir.
