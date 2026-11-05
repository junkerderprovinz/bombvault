# VM yedekleri

BombVault, KVM/libvirt VM'lerini hiçbir libvirt yolunu bağlamadan yedekler. Ana makinede SSH üzerinden (`qemu+ssh://`) `virsh` çalıştırır, VM disklerini Host Data bağlaması üzerinden (ana makinede `/mnt`, konteynerde `/host/user`) okur ve yazar ve UEFI NVRAM'ı SSH üzerinden kopyalar. Libvirt'e ait hiçbir şey bağlanmadığı için BombVault Unraid VM Manager'ın yoluna asla çıkamaz.

Bu kılavuz kurulumu, yedekleme yöntemlerini, Unraid anlık görüntülü VM'leri ve TrueNAS SCALE'i kapsar. Konteyner yedeklerinin bunların hiçbirine ihtiyacı yoktur.

ZFS veri kümesi yedekleri aynı SSH bağlantısını, aynı anahtar, ana makine ve kullanıcıyla kullanır. VM yedekleri bağlandığında ZFS veri kümesi ögeleri de bağlanır. Bkz. [ZFS veri kümeleri](zfs-datasets.md).

## Neyi yapılandırırsın {#what-you-configure}

| Ayar | Nerede | Varsayılan | Anlamı |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | şablon değişkeni | `host.docker.internal` | SSH ile ulaşılan Unraid ana makinesi |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | şablon değişkeni | `22` | Unraid'in SSH portu |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | şablon değişkeni | `root` | Ana makinedeki SSH kullanıcısı |
| Açık anahtar | **Ayarlar, Entegrasyonlar, Ana makine SSH** | ilk başlangıçta oluşur | Ana makinede yetkilendir |

BombVault SSH anahtar çiftini ilk başlangıçta, appdata'da tutulan `/config/ssh/id_ed25519` konumunda oluşturur. Ana makinenin anahtarı ilk bağlantıda `/config/ssh/known_hosts` içine sabitlenir.

Şablon **Host SSH: Address** alanını `192.168.x.x` yer tutucusuyla doldurur. BombVault bunu ayarsız sayar ve varsayılan `bridge` ağında çalışan `host.docker.internal`'ı dener. Özel bir ağda onu ana makinenin LAN IP'siyle değiştir (adım 3 B).

## Adım 1: Unraid ana makinesinde SSH'ı etkinleştir {#step-1}

1. Unraid'de **Settings, Management Access**'i aç ve **Use SSH**'ı **Yes** yap.
2. SSH portunu değiştirdiysen not al. **Host SSH: Port** eşleşmelidir.

## Adım 2: BombVault'un açık anahtarını yetkilendir {#step-2}

Unraid'in `/root` dizini bellekte yaşar ve her açılışta silinir, bu yüzden anahtar flash sürücüde de olmalıdır. Bunu Unraid terminalinde çalıştır:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 ve sonrası her açılışta `/boot/config/ssh/root.pubkeys` dosyasını root'un `authorized_keys` dosyasına kopyalar, bu yüzden bu yeniden başlatmaları atlatır. BombVault hangi ana makine adını ya da portu kullanırsa kullansın her `root` girişi için geçerlidir.

## Adım 3: Ağ {#step-3}

BombVault'un konteynerden ana makinenin SSH portuna bir TCP bağlantısı açması gerekir. Kurulumuna uyan durumu seç.

### A. Varsayılan bridge ağındaki BombVault {#bridge}

- **Host SSH: Address**'i `host.docker.internal` ya da şablon yer tutucusunda bırak. Şablon, docker0 ağ geçidine, yani ana makineye, işaret eden `--add-host=host.docker.internal:host-gateway` ekler.
- **Host SSH: Port**'u SSH portuna ayarla.

### B. Özel bir ağdaki BombVault {#custom-network}

`br0.x`, macvlan ya da ipvlan üzerindeki bir konteyner ana makineye `host.docker.internal` ile ulaşamaz, çünkü 172.17.0.1 docker0'dır ve `br0.x`'ten erişilemez. Bunun yerine:

1. Unraid'de **Settings, Docker, Host access to custom networks**'ü **Enabled** yap.
2. **Host SSH: Address**'i Unraid ana makinesinin LAN IP'sine, yani web arayüzünü açtığın adrese, ayarla.
3. Konteynerin ağı ile ana makine farklı VLAN'lardaysa rotaya yönlendiricinde ya da güvenlik duvarında izin ver: konteyner VLAN'ından ana makine IP'sine SSH portu üzerinden, TCP ile.
4. Ana makinenin LAN IP'sine hâlâ ulaşılamıyorsa ana makinenin konteynerin alt ağındaki shim IP'sini kullan. Onu ana makinede `ip -4 addr show | grep -B2 <container-subnet>` ile bul.

### Ana makinenin yanıt verdiğini kontrol et {#verify-reachability}

Bunu Unraid terminalinde, kendi IP'n ve portunla çalıştır:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` yolun çalıştığı anlamına gelir. `UNREACHABLE`, başka her şeyden önce adım 3'teki ağın düzeltilmesi gerektiği anlamına gelir.

## Adım 4: Değişkenleri ayarla ve test et {#step-4}

1. Unraid'de **Docker**'ı aç, BombVault'a tıkla, **Edit**'i seç, **Host SSH: Address**'i (ve 22 değilse **Host SSH: Port**'u) ayarla ve **Apply**'a bas. Değişkenler eksikse şablonu yeniden içe aktar, çünkü Unraid mevcut bir konteynerin kayıtlı yapılandırmasını tutar.
2. BombVault'ta **Ayarlar, Entegrasyonlar, Ana makine SSH**'ı aç ve **Bağlantıyı test et**'e bas. Yeşile dönmeli.

Terminalden aynı çağrı VM'lerini listeler:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Adım 5: VM başına yöntem {#per-vm-method}

Her VM'in Ayarlar sekmesindeki **VM** kartında bir **Yöntem** vardır (VM'i **Yedekler** altında aç):

- **Düzgün (kapatma)** varsayılandır. VM'i kapatır, diskleri yedekler ve yeniden başlatır, bu yüzden yedek her zaman tutarlıdır. Konuğun kapanmasını on dakikaya kadar bekler ve onu hiçbir zaman zorla kapatmaz. Bundan sonra hâlâ çalışan bir konuğa dokunulmaz ve o yedek nedeniyle başarısız olur. Konuk sonraki bir saat içinde kapanırsa BombVault onu yeniden başlatır. Bu başlatma başarısız olursa başarısız çalıştırma bunu söyler.
- **Canlı anlık görüntü**, çalışan bir VM'i harici bir anlık görüntü ve `blockcommit` ile yedekler. Diskin `/mnt/user` üzerinde değil `/mnt/cache` ya da `/mnt/diskX` üzerinde olmasına ihtiyaç duyar. VM'de qemu guest agent varsa anlık görüntü tutarlı (quiesced) alınır, yoksa çökme tutarlıdır (crash-consistent). Kapalı bir VM'de Canlı kendiliğinden Düzgün'e döner. Anlık görüntü alınamadığında o yedek başarısız olur ve VM çalışmaya devam eder, çünkü canlı yedeklemeyi seçtiğin bir VM bunun için hiçbir zaman kapatılmaz.

## Bir yedeğin dışarıda bıraktıkları {#left-out}

Bir yedek başarılı olup yine de VM'in tamamını tutmayabilir. Çalıştırma o zaman neyin eksik olduğunu söyleyen, uyarı tonunda bir not taşır:

- **BombVault'un okumadığı diskler.** Salt okunur bir disk, bir `lun` ve bir `network` ya da `volume` diski dışarıda bırakılır ve not her birini hedef aygıtı ve türüyle adlandırır. Yedekteki tanım onları yine de listeler. Bir cdrom ve bir disket sürücüsü adlandırılmaz. ZFS zvol olmayan düz bir blok aygıtı ise yedeği başarısız kılar.
- **Emüle edilmiş bir TPM'in durumu.** Domain XML'inde bir yolu yoktur. Unraid VM Manager böyle bir TPM'i **OVMF TPM** BIOS'lu bir VM için kurar, bu yüzden not Unraid'de de görünür. TPM'inde bir sır tutan bir konuk, BitLocker'ın yapabildiği gibi, o durumu kaybeden bir geri yüklemeden sonra kurtarma anahtarını isteyebilir, bu yüzden o anahtarı elinin altında tut.

## İnce diskler {#thin-disks}

Bir geri yükleme, diski değiştirse de yeni bir disk yazsa da ince bir diski ince tutar: restic sıfır dizilerini yeni bir dosyada delik olarak yazar ve BombVault değiştirilen bir dosyanın sıfır bloklarını dosya yazıldıktan sonra `fallocate --dig-holes` ile geri verir. restic bir dosyayı değiştirirken o dosya, havuzda yer varsa, tam boyutunu kaplar. Delik açmayı desteklemeyen bir dosya sistemi diski tam boyutunda tutar ve günlük bunu söyler. Yer denetimi yeni bir diski yine de tam boyutuyla sayar.

## Başarısız olan bir geri yükleme {#restore-fails}

Yerinde bir geri yükleme VM'i kapatır, diskleri yazar ve tanımı ile NVRAM'i ancak diskler yazıldıktan sonra değiştirir. Başarısız olan bir geri yükleme VM'i NVRAM'iyle birlikte tanımlı bırakır. Diskleri değişmediyse ve çalışıyorsa yeniden çalışır. Değişmiş olabilirlerse kapalı kalır, çünkü yarım yazılmış bir diskten açılmak onun üzerine yazardı, ve çalıştırma bunu söyler. Aynı yedeği yeniden geri yüklemek işi tamamlar.

libvirt'te kayıtlı anlık görüntüler (Unraid VM Manager anlık görüntüleri böyle almaz), VM'e dokunulmadan önce yerinde bir geri yüklemeyi durdurur, çünkü libvirt bunlara sahip bir VM'i kaldıramaz. Önce onları sil.

## Yalnızca değişen bloklar {#changed-blocks}

qcow2 diskli bir VM yalnızca değişeni okuyarak yedeklenebilir. Aynı kartta **Yalnızca değişen bloklar**'ı aç, BombVault yukarıdaki yöntem yerine libvirt'in yedekleme API'sini kullanır:

- Aynı anda `bombvault-<time>` adlı bir kontrol noktası oluşturan bir pull kipi yedekleme işi (`virsh backup-begin`) başlatır ve diskleri SSH bağlantısı üzerinden iletilen bir Unix soketiyle okur. Konuk ajanı yanıt verdiğinde, konuğun dosya sistemleri yalnızca iş başlarken dondurulur. Başarısız bir çözme yaklaşık on beş saniye yeniden denenir ve konuk hâlâ çözülmezse BombVault bir bildirim gönderir.
- İlk çalıştırma ayrılmış her bloğu okur. Sonraki çalıştırmalar yalnızca kontrol noktasının bit eşlemi önceki yedekten beri yazılmış olarak işaretlediği blokları okur.
- Her disk, konuğun gördüğünü tutan sabit boyutlu parçalardan oluşan bir klasör olarak saklanır. Değişmeyen parçalar okunmadan önceki anlık görüntüden devralınır, bu yüzden her anlık görüntü eksiksizdir ve kendi başına geri yüklenir.
- Bir geri yükleme diski parçalarından yeniden kurar ve `qemu-img` ile yeniden qcow2'ye çevirir. Disk yedekleme zamanındaki içeriğe sahiptir, ama dosya yeni bir qcow2 dosyasıdır ve bir dayanak zinciri düzleştirilmiş olarak gelir.
- Böyle bir geri yüklemeden önceki denetim, yeniden kurduğu her diski listeler ve konuğun tam boyutuyla, qcow2 için iki kez sayar, çünkü ham image ve dönüştürülmüş kopyası eski dosya değiştirilene kadar yan yana durur. Bu bir diskin alabileceği en fazladır, bu yüzden bu sayımla bir eksiklik yer satırını engellemek yerine gri bırakır. İlk yedekten sonraki geri yükleme denetimi ve **Geri yüklemeyi denetle** de anlık görüntünün her diskin her parçasını olması gereken boyutta tuttuğundan emin olur.
- Çalıştırmalar arasında VM'in libvirt'te kayıtlı bir kontrol noktası yoktur. BombVault en yeni `bombvault-` kontrol noktasının tanımını `/config/vm-checkpoints` altında tutar ve yalnızca libvirt'in onun kaydını kaldırır (`virsh checkpoint-delete --metadata`). Bit eşlemi qcow2 dosyasında kalır ve yazmaları saymaya devam eder, sonraki çalıştırma başlamadan önce kontrol noktasını geri koyar (`virsh checkpoint-create --redefine`). libvirt kontrol noktaları olan bir VM'in tanımını kaldırmayı reddeder, bu yüzden Unraid'in **Remove VM**'inin ve bir geri yüklemenin her zamanki gibi çalışmasını sağlayan şey budur.
- BombVault'un, bir yedeğin ortasında durduğu için çalışır bıraktığı bir yedekleme işi, o çalıştırmanın kayıtlı bıraktığı kontrol noktalarıyla birlikte BombVault yeniden başladığında sonlandırılır.
- Anahtarı kapatmak, VM'i zamanlamadan çıkarmak ya da yedeklerini silmek de böyle bir işi sonlandırır ve VM çalışırken bit eşlemini de siler. Kapalı bir VM'de libvirt onu silemez ve kullanılmadan image'de kalır.
- Diğer araçların kontrol noktalarına dokunulmaz. Bazıları olan bir VM, silinene kadar geri yüklenemez ve geri yükleme VM'e dokunmadan durur.

Anahtar kendi kendine geri çekilir. Eksik ya da bozuk bir kontrol noktası, boyutu değişen bir disk ya da değişen bloklu olmayan en yeni bir anlık görüntü, tüm diskin bir kez daha okunması demektir. Kapalı bir VM, ham ya da zvol bir disk, ana makine SSH bağlantısının olmaması, yedekleme API'si olmayan bir libvirt ya da `/config` altında çok az boş alan, yukarıdaki yöntemle olağan yedek demektir. Kart son çalıştırmanın bunlardan hangisini kullandığını ve nedenini söyler.

Gereksinimler: yedekleme API'li libvirt (Unraid 7.3'te libvirt 12.2 ve QEMU 10.2 ile denendi), `AllowStreamLocalForwarding yes` ile sshd (varsayılan, TCP iletmeyi kapatan Unraid'de de) ve qcow2 diskler. libvirt ham disklerde kontrol noktalarını reddeder ve Unraid varsayılan olarak ham vdisk oluşturur. VM yedekleme sırasında, Canlı'da olduğu gibi, çalışmaya devam eder ve restic'i bekleyen parçalar `/config` altında 4 GiB'a kadar yer kaplar.

## Unraid anlık görüntülü VM'ler {#unraid-snapshots}

Unraid VM Manager'ın aldığı bir anlık görüntü bir qcow2 diskini bir zincire çevirir: altta özgün image, üstünde her anlık görüntü için bir dosya, her biri altındakinden beri yalnızca değişeni tutar. VM en yeni dosyaya yazar. İki anlık görüntülü bir disk üç dosyadır.

BombVault böyle bir VM'i tam olarak yedekler:

- **Zincirin her dosyası.** Her diskin dayanak zincirini sonuna kadar izler ve içindeki her dosyayı, Unraid'in anlık görüntü listesiyle (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, SSH üzerinden okunur) birlikte yedekler. Sonuna kadar izlenemeyen bir zincir, örneğin ortadaki bir dosya yok olduğu için, yedeği başarısız kılar ve dosyayı adlandırır.
- **Canlı bir yedek anlık görüntüleri korur.** **Canlı anlık görüntü** diski okurken üstüne geçici bir katman koyar ve sonra onu yalnızca hemen altındaki image'e geri birleştirir. Zincir yedekten sonra öncekiyle aynıdır. Anlık görüntüsü olmayan bir VM her zamanki gibi davranır.
- **Bir geri yükleme zinciri geri getirir.** Yerinde bir geri yükleme her dosyayı olduğu yere geri yazar ve Unraid'in anlık görüntü listesini geri koyar, böylece VM Manager anlık görüntüleri yeniden listeler. Orijinalin yanına bir geri yükleme dosyaları kopyanın klasörüne yazar ve geri yüklenen diskleri geri yüklenen dosyalara yönlendirir, böylece kopya hiçbir zaman orijinalin image'lerinden okumaz. Unraid'in anlık görüntü listesi yalnızca yerinde bir geri yüklemeyle geri gelir, çünkü dosyaları yedeklendikleri yerde adlandırır. Onsuz kopya aynı şekilde açılır, VM Manager ise onun için yalnızca anlık görüntü listelemez.
- **Yalnızca değişen bloklar** konuğun gördüğünü okur, bu yüzden geri yüklemesi anlık görüntüler olmadan tek bir düzleştirilmiş qcow2 dosyası olarak gelir.
- **Bir geri yükleme, seçtiğin yedeğin diskini geri getirir.** O yedekten sonra alınan ya da birleştirilen anlık görüntüler bunu değiştirmez. Bir anlık görüntüden önceki bir yedek, VM'i tuttuğu dosya üzerinde tanımlar, bir birleştirmeden önceki bir yedek ise katmanını geri getirir ve onun üzerinde çalışır. Daha yeni dosyalar kullanılmadan ve değişmeden olduğu yerde kalır. Bir katman tutup altındaki dosyayı tutmayan bir yedek, VM'e dokunulmadan önce reddedilir. Bir geri yüklemeden sonra VM'in tanımı hiçbir zincir listelemez, böylece libvirt zinciri geri yüklenen dosyalardan okur.

### Örnek: iki anlık görüntülü bir VM {#example-unraid-snapshots}

`homeassistant` VM'inin tek bir diski `vdisk1.qcow2` ve iki güncellemeden önce VM Manager'da aldığın iki anlık görüntüsü var. Diski şimdi üç dosyalık bir zincir. Sürekli çalışıyor, bu yüzden **Canlı anlık görüntü** kullanıyor.

1. **Yedekler**'i, sonra `homeassistant`'ı, sonra **Ayarlar** sekmesini aç. **VM** kartında **Yöntem**'in **Canlı anlık görüntü** ve **Yalnızca değişen bloklar**'ın kapalı olduğunu kontrol et.
2. Sayfanın başlığındaki **Şimdi yedekle**'ye bas.
3. Çalıştırma bittiğinde **Doğrulama** sekmesindeki **Son çalışmalar** içindeki satırı yeşildir. VM bütün süre çalışmaya devam etti ve yedek diskin üç dosyasını ve Unraid'in anlık görüntü listesini tutuyor.
4. VM Manager'da iki anlık görüntü hâlâ orada ve disk hâlâ aynı üç dosyadan okuyor.

Bir ay sonra bir güncelleme ters gidiyor ve karşılaştırmak için dünkü durumu bozuğun yanına geri istiyorsun:

1. **Geri yükleme**'yi aç, **VM'ler**'i seç ve `homeassistant`'a tıkla.
2. Dün geceki önerilen yedeği tut ve **Nereye** altında **Orijinalin yanına**'yı seç.
3. **Geri yükle**'ye bas. Üç dosya VM'in adı ve `-restored` ile adlandırılan bir klasöre düşer ve geri yüklenen disk geri yüklenen dosyalara işaret eder. Çalışan VM'de hiçbir şey değişmez.

Tüm VM'i geri almak için bunun yerine **Önce mevcut durumu yedekle** açıkken **Orijinali değiştir**'i seç. BombVault bozuk durumu yedekler, VM'i kapatır, üç dosyayı ve Unraid'in anlık görüntü listesini geri yazar ve yeniden başlatır.

## TrueNAS SCALE {#truenas-scale}

Yukarıdaki adımlar Unraid için yazılmıştır, ama aynı `qemu+ssh://` bağlantısı konteynerin ulaşabildiği herhangi bir libvirtd ile, VM'leri 25.04.2'den beri yeniden libvirt üzerinde olan TrueNAS SCALE dahil, çalışır. TrueNAS'ta bağlantı dizesinin bir geçersiz kılmaya ihtiyacı vardır ve anahtar TrueNAS'ın kendi kullanıcı ayarlarında root'a eklenir. Üç şey daha bir yedeğin neyi tuttuğunu değiştirir: zvol üzerindeki diskler, vTPM durumu ve TrueNAS'ın VM'lerine verdiği adlar.

### 1. libvirt soketi {#truenas-socket}

TrueNAS'ın libvirtd'si, **Host SSH** alanlarından oluşturulan bağlantı dizesinin beklediği yeri dinlemez. Bunun yerine ek `?socket=` parametresiyle `LIBVIRT_URI` ayarla:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault `LIBVIRT_URI`'yi aynen yazıldığı gibi kullanır ve `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` ve `LIBVIRT_SSH_PORT` gerekmez. Ayarsız olanların her biri için BombVault ana makineyi, kullanıcıyı ve portu kendi SSH komutları için de URI'den alır, böylece NVRAM kopyası ve ZFS veri kümesi yedekleri `virsh` ile aynı makineye ulaşır. Kendin ayarladığın bir değişken her zaman kazanır ve açılış günlüğü URI'den aldığı hedefi adlandırır. TrueNAS'ta root dışında bir kullanıcı ZFS veri kümesi yedekleri için ayrıca `zfs allow <user> snapshot,destroy,mount <dataset>` ister, bkz. [ZFS veri kümeleri](zfs-datasets.md#truenas).

TrueNAS LAN adresini URI'ye koy. `LIBVIRT_URI` de `LIBVIRT_HOST` da ayarlı değilse BombVault, bir TrueNAS uygulamasında `host.docker.internal` çözümlenmediği için uygulamanın varsayılan ağ geçidini ana makine olarak alır ve açılış günlüğü seçtiği adresi adlandırır. Bu ZFS veri kümesi yedekleri için yeterlidir. VM yedekleri soket için hâlâ URI'ye ihtiyaç duyar.

Uygulamanın VM yedekleri için `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` ve `SETUID` yanında `SETPCAP` yeteneğine ihtiyacı vardır. Onsuz libvirt "cannot apply process capabilities" ile durur. [TrueNAS SCALE](getting-started.md#truenas) altındaki YAML hepsini, durdurma yerine anlık görüntünün ihtiyaç duyduğu `SYS_CHROOT` ile birlikte ayarlar.

TrueNAS SCALE 25.10.0'da soket yolu gerçek donanımda denendi: stok `qemu:///system` `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory` ile başarısız olur, aynı çağrı `?socket=/run/truenas_libvirt/libvirt-sock` ile domainleri listeler.

### 2. BombVault'un TrueNAS'taki anahtarı {#truenas-key}

Yukarıdaki adım 2 Unraid'in flash sürücüsüne yazar ve burada geçerli değildir. TrueNAS'ta:

1. **System, Services** altında SSH servisini aç.
2. **Credentials, Users** altında **root**'u düzenle ve **Ayarlar, Entegrasyonlar, Ana makine SSH** içindeki BombVault'un açık anahtarını SSH açık anahtarı olarak yapıştır. TrueNAS onu yeniden başlatmalar ve güncellemeler boyunca tutar.

TrueNAS 24.10 ve sonrasının temiz kurulumları root'a parola vermez ve anahtarla girişin parolaya ihtiyacı yoktur. Root kullanmak istemezsen anahtarı sudo'lu bir yönetici hesabı için yetkilendir ve `LIBVIRT_URI` içinde `root` yerine onun adını koy. ZFS veri kümesi yedekleri o zaman yukarıda anlatıldığı gibi `zfs allow` ister.

İkinci bir sunucuya replika, replika penceresinin gösterdiği kendi anahtarıyla oturum açar. Host SSH anahtarı bir replikayı yetkilendirmez ve replika anahtarı VM yedeklerini yetkilendirmez.

!!! warning "TrueNAS virsh ile yapılan değişiklikleri görmez"
    TrueNAS'ın VM ekranlarının arkasındaki katman olan middleware'inin VM'lerinin değiştirilebileceği tek yol olması amaçlanır. BombVault'un VM yedeklemesinin ve geri yüklemesinin çalıştırdığı şey olan, bu SSH bağlantısı üzerinden bir `virsh` komutu onu dolaşır ve TrueNAS sonradan kendisinin oluşturmadığı durumu uzlaştırabilir ya da üzerine yazabilir. Bu, TrueNAS'ın belgelenmiş davranışından gelir ve gerçek bir makinede görülmemiştir. Unraid'in VM Manager'ının kendi dışında yapılan değişiklikleri uzlaştırmasıyla aynı türden bir risktir, yalnızca TrueNAS'ta daha keskin.

### 3. zvol üzerindeki diskler {#truenas-zvols}

TrueNAS VM diskleri çoğu zaman zvol'dür, qcow2 dosyaları değil, `/dev/zvol/<pool>/<dataset>` adresindeki ZFS blok aygıtlarıdır. restic bir blok aygıtını yoluyla bir dosyayı yedeklediği gibi yedekleyemez, bu yüzden BombVault bir zvol diskini domain XML'inde (`<source file="...">` yerine `<source dev="...">`) tanır ve aynı SSH bağlantısı üzerinden ele alır. BombVault konteynerinin ZFS araçlarına ihtiyacı yoktur.

Bir zvol'ün yedeği:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` bir zaman noktasını, VM çalışırken de, sabitler.
2. `zfs clone -o volmode=dev -o readonly=on` o anlık görüntüyü bir aygıt olarak okunabilir yapar. `volmode=dev` ana makinenin konuğun bölümlerini taramasını önler.
3. `dd` aygıtı okur ve ham image restic'e `vm:<name>:zvol:<dev>` etiketli tek bir dosya, `/vm-disks/<dataset>.img`, olarak girer. restic onu içeriğe göre parçalara böler, bu yüzden son yedekten beri değişmeyen bloklar yeniden saklanmaz.
4. Klon ve anlık görüntü, yedek işe yarasa da yaramasa da sonra yok edilir. BombVault bir yedeğin ortasında durduysa o zvol'ün sonraki yedeği önce `bombvault-vm-<time>` anlık görüntüsünü ve `<dataset>-bombvault-vm-<time>` klonunu kaldırır. Diğer her anlık görüntüyü rahat bırakır ve başka bir şeyin klonlandığı birini tutar. Günlük neyi kaldırdığını ya da tuttuğunu adlandırır.

BombVault bunun için `zfs send` kullanmaz. Bir send akışının her kaydı anlık görüntünün GUID'ini ve süregelen bir sağlama toplamı taşır, bu yüzden restic son yedekten yeniden kullanılacak hiçbir şey bulamaz ve tüm diski her seferinde yeniden saklardı. 200 MiB veri tutan 512 MiB'lık bir zvol ile TrueNAS SCALE 25.10'da ölçüldü: ilk yedek 211 MB ekledi, hiçbir şeyin değişmediği ikincisi 52 KB, 1 MiB değiştikten sonrakisi 3,6 MB. Aynı zvol `zfs send` ile her çalıştırmada 214 MB ekledi.

Her çalıştırma ayrıca tüm diskler girdikten sonra domain XML'iyle `vm:<name>` etiketli küçük bir anlık görüntü yazar. Tüm diskleri zvol olan bir VM'in dosya anlık görüntüsü yoktur ve çalıştırmayı VM'in altında listeleyen ve onu geri yüklenebilir yapan budur.

Bir geri yükleme her zvol'ü, bir dosya diskinin kendi dosyasına geri yazılması gibi, yerinde geri yazar, böylece geri yüklenen VM geri yüklenen veriyi açar:

1. Bir şeye dokunulmadan önce BombVault çalıştırmanın her zvol diskinin bir image'ini tuttuğunu denetler. Birinin eksik olduğu bir çalıştırma reddedilir ve VM olduğu gibi bırakılır.
2. Var olan bir zvol önce `<dataset>@bombvault-before-restore-<time>` anlık görüntüsünü alır, zaman, BombVault'un yaptığı her anlık görüntü adı gibi UTC'dedir, bu yüzden yerel saatten saatlerce farklı olabilir. Günlük onu adlandırır ve zvol'ün en yeni anlık görüntüsü olduğu sürece ona `zfs rollback` geri yüklemeyi geri alır. Sonraki geri yüklemeninki gibi daha yeni biri varsa `zfs rollback -r` bunu yapar ve daha yeni anlık görüntüleri yok eder. Geri yüklenen VM çalışınca `zfs destroy` ile kaldırana kadar durur.
3. Artık var olmayan bir zvol yedeklendiği anki boyutta oluşturulur ve başka boyuttaki var olan biri o boyuta geri getirilir.
4. `dd` image'i `/dev/zvol/<dataset>` üzerine yazar. Sıkıştırma açıkken, TrueNAS varsayılanı, image'in boş kısımları delik olarak gelir ve yer kaplamaz.

**Başka bir örneğe geri yükleme bir hedef havuz ister.** Dosya disklerini bir hedef klasöre taşır, ama bir klasör bir ZFS havuzunu adlandırmaz, bu yüzden onsuz BombVault geri yüklemeyi başlamadan reddeder. Havuzu `POST /api/foreign/restore` içindeki `zvolPool` parametresi olarak geç: her zvol havuzun altındaki yolunu korur (`flashpool` üzerinde `tank/vms/win10/disk0`, `flashpool/vms/win10/disk0` olur) ve domain XML'i ona işaret eder. Geri yükleme sayfasındaki **Eski bir yedek klasöründen** penceresinde bunun için bir alan yoktur, bu yüzden yalnızca doğrudan bir API çağrısıyla ulaşılabilir. Aynı örnekte bir geri yükleme her zvol'ü kendi üzerine geri yazar.

**Dosya olarak dışa aktarma zvol'leri dışarıda bırakır.** Tüm diskleri zvol olan bir VM bu nedenle reddedilir, bu yüzden onu bir yedekten geri yükle. Dosya diskleri de olan bir VM için dışa aktarma dosya disklerini ve dışarıda bıraktığı zvol'leri adlandıran bir not tutar.

Bu, ZFS 2.3.4 ile TrueNAS SCALE 25.10'da denendi: bir deneme zvol'ünün üç yedeği, zvol üzerine yazılıp büyütüldükten sonra yerinde bir geri yükleme ve silinmiş bir zvol'e geri yükleme. Her geri yükleme kaynakla aynı SHA-256 ile geri okundu, zvol eski boyutuna yeniden kavuştu ve geride anlık görüntü ya da klon kalmadı. Tek diski zvol olan bir VM bütünüyle çalışırken, Düzgün ve Canlı ile, ve kapalıyken yedeklendi ve oradaki bütün bir VM geri yüklemesi de (tanımı kaldır, zvol'leri yaz, tanımla, aç) çalıştı: zvol kaynağın sağlama toplamıyla geri okundu ve VM başladı.

Unraid'in kullandığı dosya diskleri bunların hiçbirinden etkilenmez.

### 4. vTPM durumu {#truenas-vtpm}

Bir VM'in sanal bir TPM'i olabilir, Secure Boot ve Windows 11 konukları buna ihtiyaç duyar. Durumu diskin ve NVRAM'ın dışında yaşar, bu yüzden yalnızca bu ikisinin yedeği bir konuğu yeni, boş bir TPM ile geri getirirdi.

- **Bir passthrough TPM**, konuğa verilen gerçek bir çip, yolunu domain XML'inde adlandırır. BombVault durumunu NVRAM'ın yanında SSH üzerinden kaydeder ve `virsh define`'dan önce geri yazar, NVRAM gibi elinden geleni yaparak: başarısız bir okuma ya da yazma günlüğe yazılır ve yedeği ya da geri yüklemeyi başarısız kılmaz.
- **Emüle edilmiş bir vTPM**, TrueNAS'ın Windows 11 ve Secure Boot konukları için kurduğu türden, domain XML'inde bir durum yolu yayınlamaz, bu yüzden durumu yedeklenmez. BombVault bir yol tahmin etmez ve yedek, VM'in TPM durumunun içinde olmadığını söyleyen bir notla biter. Böyle bir konuğu geri yüklemeden önce kurtarma anahtarını, örneğin BitLocker anahtarını, elinin altında tut.
- **Bir TrueNAS VM'ine kendin TPM ekleyemezsin.** TrueNAS SCALE 25.10'un VM aygıt API'si yalnızca `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` ve `USB` kabul eder ve `TPM`'i bilinmeyen bir tür olarak reddeder.

TPM işleme yalnızca birim testlidir ve vTPM'li gerçek bir TrueNAS konuğuna karşı çalıştırılmamıştır. TPM'i olmayan bir VM tamamen eskisi gibi yedeklenir ve geri yüklenir.

NVRAM her zamanki gibi alınır. TrueNAS onu VM başına `/var/db/system/vm/nvram/<id>_<name>_VARS.fd` içinde tutar ve domain XML'i bu yolu adlandırır.

### 5. TrueNAS'taki VM adları {#truenas-names}

Unraid'de bir VM'in libvirt adı seçtiğin addır. TrueNAS libvirt adını uydurur ve bunu iki kez değiştirmiştir:

| TrueNAS sürümü | libvirt adı | Senin adın nerede durur |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, örneğin `1_debian` | libvirt adında, `{id}_`'nin arkasında |
| 26 | VM'in UUID'si, örneğin `550e8400-e29b-41d4-a716-446655440000` | yalnızca domain XML'inin `<title>` içinde |

BombVault'un TrueNAS olarak algıladığı ya da `PLATFORM=truenas` ile adlandırılan bir ana makinede Yedekler listesi senin adını gösterir: 25.10'da `1_debian` yerine `debian`, 26'da UUID yerine `<title>` ya da başlık yoksa UUID. Bu yalnızca listenin gösterdiğidir. Her `virsh` çağrısı ve VM başına ayarlar yine libvirt adını kullanır, bu yüzden yedekleme ve geri yükleme her zaman doğru VM üzerinde çalışır. TrueNAS 26'da bu, TrueNAS'ın yeni bir UUID ile yeniden oluşturduğu bir VM'in, adı aynı görünse bile, ayarları olmadan yeni bir öge olarak görüneceği anlamına gelir. Unraid'de, `10_Windows` adını verdiğin bir VM için de, hiçbir şey değişmez.

25.10 düzeni gerçek bir TrueNAS SCALE 25.10 makinesinde görüldü. TrueNAS 26 düzeni TrueNAS'ın belgelerinden gelir ve yalnızca birim testlidir.

## Sorun giderme {#troubleshooting}

| Belirti | Neden ve çözüm |
|---|---|
| **Bağlantıyı test et** takılıyor, sonra başarısız oluyor | Ana makineye ulaşılamıyor. Adım 3'ü (ağ, VLAN, güvenlik duvarı) ve **Host SSH: Address** ile **Port**'u kontrol et. |
| Anahtar `authorized_keys` içinde olduğu halde `Permission denied (publickey)` | sshd'nin `StrictModes` ayarı, bir üst klasörün sahibi ya da kipi yanlış olduğu için anahtar dosyasını reddediyor. `/root/.ssh`, herkes tarafından yazılabilir FAT flash sürücüye (`/boot/config/ssh/...`) bir bağlantı olduğunda yaygındır. Ana makine günlüğü `Authentication refused: bad ownership or modes for directory ...` gösterir. Ana makinenin sshd yapılandırmasına, genel bölüme, herhangi bir `Match` bloğundan önce, `StrictModes no` ekle, sonra `/etc/rc.d/rc.sshd restart` çalıştır. Unraid `/etc/ssh/sshd_config`'i her açılışta yeniden yazar, bu yüzden bunu aşağıda anlatıldığı gibi kalıcı yap. |
| `Permission denied (publickey)` ve anahtar gerçekten eksik | Adım 2'yi tekrarla ve SSH'ın açık olduğunu kontrol et. |
| `Host key verification failed` | `docker exec BombVault rm -f /config/ssh/known_hosts` çalıştır ve yeniden test et. |
| Adım 3'teki denetim `UNREACHABLE` diyor | Konteyner ana makinenin SSH portuna ulaşamıyor. Bkz. adım 3: özel ağlara ana makine erişimi, VLAN yönlendirmesi ya da shim veya LAN IP'si. |
| Değişkenler **Edit** altında eksik | Şablonu yeniden içe aktar. Mevcut bir konteyner eski kayıtlı yapılandırmasını tutar. |
| Unraid anlık görüntülü bir VM başarısız oluyor ve bir dosyayı adlandırıyor | Diskin dayanak zinciri sonuna kadar izlenemiyor. VM Manager'da o diskin her anlık görüntüsünün hâlâ orada olduğunu ve adlandırılan dosyanın `/mnt` altında var olduğunu kontrol et. |

Daha fazla VM belirtisi [Sorun Giderme](troubleshooting.md#vm-backup-will-not-connect) içindedir.

## Yeniden başlatmalar boyunca korumak {#persistence}

Unraid `/etc/ssh/sshd_config`'i her açılışta yeniden yazar, bu yüzden ana makine tarafındaki değişikliklerin açılışta yeniden yapılması gerekir:

- **SSH portu**, Unraid web arayüzünden ayarlanarak `/boot/config/ident.cfg` içinde tutulur.
- Yukarıda gerekiyorsa **`StrictModes no`** tutulmaz. Onu açılışta `/boot/config/go` içinden uygula:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- BombVault'un kendi SSH anahtarı ve `~/.ssh/config`, `/config` (appdata) içinde durur ve her başlangıçta yeniden oluşturulur, bu yüzden konteynerin ve ana makinenin yeniden başlatmalarını atlatır.

## Güvenlik {#security}

Yetkilendirilen anahtar ana makinede root verir, BombVault'un zaten kullandığı Docker soketiyle aynı güven düzeyi. BombVault'u güvenilir bir ağda tut. Anahtar ed25519'dur, appdata'da `0600` kipiyle saklanır ve ana makinenin anahtarı sabitlenir. Her ana makine komutu ayrı argümanlar olarak geçirilir, hiçbir zaman uzak bir kabuk üzerinden değil ve SSH bağlantısı `BatchMode` ve bir bağlantı zaman aşımı kullanır.
