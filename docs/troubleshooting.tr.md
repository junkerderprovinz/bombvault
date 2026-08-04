# Sorun giderme

Kısa bir SSS. Tam VM-üzeri-SSH host tarafı sorun giderme tablosu için (izin reddedildi, host anahtarı doğrulaması, eksik şablon değişkenleri ve daha fazlası), GitHub'daki [SSH üzerinden VM yedekleme kılavuzu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'na bakın.

## Bir şey doğru bağlanmamış

Web arayüzünde `/spike`'ı açın. Host entegrasyon denetimi her bağlamayı ve CLI'ı (Docker soketi, libvirt, restic, qemu-img, rclone) yoklar ve eksik parçaları bildirir. Bir hata olduğunu varsaymadan önce buradan başlayın: eksik bir bağlama ya da ulaşılamayan bir host hemen ortaya çıkar.

## Web arayüzüne ulaşamıyorum

BombVault, kutudan çıktığı gibi `3443` portunda HTTPS sunar (kendinden imzalı sertifika), bu yüzden `https://<your-unraid-ip>:3443` adresini açın. Kendinden imzalı sertifika uyarısını kabul edin ya da BombVault'u kendi sertifikanızla bir ters proxy arkasına yerleştirin. `HTTP_ONLY=true` ile çalıştırırsanız, bunun yerine `3000` portunda düz HTTP sunar (TLS'yi sonlandıran bir proxy arkasında kullanım için tasarlanmıştır).

## APP_KEY'imi kaybettim

`APP_KEY`, restic depo parolasını türetir. O olmadan (ve şifreleme anahtarı kurtarma kiti olmadan), şifreli yedekler kurtarılamaz. Kontrol Paneli'nin kurtarma kitini indirmeniz için dırdır etmesinin nedeni budur. Bkz. [Site dışı ve kurtarma](offsite-recovery.md). `openssl rand -hex 32` ile bir anahtar oluşturun ve herhangi bir yedeğe güvenmeden önce onu sunucu dışında saklayın.

## VM yedeklemesi bağlanmıyor

VM yedeklemesi, bir bağlamayla değil, SSH üzerinden libvirt ile konuşur.

- Host'ta SSH'nin etkin olduğunu ve BombVault'un genel anahtarının `/root/.ssh/authorized_keys` içinde yetkilendirildiğini onaylayın (Ayarlar, Sistem, SSH üzerinden VM Yedeği anahtarı ve bir **Bağlantıyı test et** düğmesi gösterir).
- Özel bir `br0.x` ağında, `LIBVIRT_HOST`'u Unraid LAN IP'nize ayarlayın (konteyner orada host'a `host.docker.internal` üzerinden ulaşamaz). **Ayarlar, Docker, Özel ağlara host erişimi**'ni etkinleştirin.
- Unraid'in SSH portunu değiştirdiyseniz, eşleşmesi için `LIBVIRT_SSH_PORT`'u ayarlayın.
- Tam adım adım tanılama (ulaşılabilirlik testi, VLAN yönlendirme, `Permission denied (publickey)`, `Host key verification failed`) [SSH üzerinden VM yedekleme kılavuzu](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'nda yer alır.

## Bir canlı VM anlık görüntüsü çalışmadı

Canlı anlık görüntüler, VM'de qemu guest agent'ın kurulu olmasını ve diskin `/mnt/user` değil `/mnt/cache` (ya da `/mnt/diskX`) üzerinde olmasını gerektirir. Kapalı bir VM'de, canlı otomatik olarak düzgüne geri düşer. Düzgün bir yedekleme VM'yi kapatır, diskleri yedekler, ardından yeniden başlatır, böylece her zaman tutarlıdır.

## Bir yedekleme "repository is already locked" ile başarısız oldu

Bu genellikle, konteyner işlem ortasında güncellendiğinde ya da yeniden başlatıldığında geride kalan öksüz bir restic kilididir. BombVault, kanıtlanabilir biçimde öksüz bir kilidi algılar, zorla temizler ve otomatik olarak bir kez yeniden dener. Sürerse, eski bir kilidi elle temizlemek için etkilenen etki alanı için **Ayarlar, Bütünlük ve bakım, Kilidi aç**'ı kullanın. Gerçek bir sorun gizlenmek yerine yine de ortaya çıkar.

## Bir yedeklemeden sonra site dışı kopyam gerçekleşmedi

Site dışı çoğaltma tasarım gereği en iyi çabadır, böylece bir site dışı aksaklık asla yerel yedeklemeyi bozmaz. O etki alanı için site dışı zamanlamayı denetleyin (Ayarlar, Zamanlamalar): boş bir zamanlama her yerel yedeklemeden sonra çoğaltır, bir sıklık ise daha seyrek gönderir. İstek üzerine bir çalışma için Site dışı sekmesindeki **Şimdi çoğalt**'ı kullanın ve Kontrol Paneli'ndeki çoğaltma göstergesini izleyin.

## Bir geri yükleme başlamadan iptal oldu

Herhangi bir şey durdurulmadan ya da kaldırılmadan önce, geri yükleme bir uçuş öncesi çakışma denetimi çalıştırır: konteynerin statik IP'sinin ve yayımlanan host portlarının serbest olduğunu doğrular. Başka bir konteyner zaten birini tutuyorsa, yarım kalmış bir geri yükleme bırakmak yerine açık, uygulanabilir bir mesajla iptal eder. Çakışan portu ya da IP'yi serbest bırakın, ardından yeniden deneyin.

## Bir düz dışa aktarma, dosya yazmak yerine başarısız oldu

age şifrelemesi açıksa (Ayarlar) ama geçerli bir alıcı ayarlanmamışsa, bir dışa aktarma düz metin yazmak yerine açık bir hatayla başarısız olur. Geçerli bir alıcı ekleyin (bir age genel anahtarı ya da bir SSH genel anahtarı) ya da dışa aktarmanın düz metin olmasını istiyorsanız şifrelemeyi kapatın. Bkz. [Özellikler](features.md).

## Konteyner sürekli yeniden başlıyor ya da sağlıksız görünüyor

BombVault, kendi `/api/health`'inden sağlıklı/sağlıksız bildirir. Motor bir şekilde sıkışırsa bir otomatik onarma aracı (Autoheal gibi) onu otomatik olarak yeniden başlatabilir. Altta yatan neden için konteyner günlüğünü ve `/spike` raporunu denetleyin.

## Hâlâ takıldınız mı?

- Tam [Yapılandırma](configuration.md) ve [Site dışı ve kurtarma](offsite-recovery.md) sayfalarını okuyun.
- [Unraid destek başlığı](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)'nda sorun.
- Bir [GitHub sorunu](https://github.com/junkerderprovinz/bombvault/issues) açın.
