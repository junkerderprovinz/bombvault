# Hibaelhárítás

Egy rövid GYIK. A teljes VM-SSH hoszt-oldali hibaelhárítási táblázatért (permission-denied, hoszt-kulcs ellenőrzés, hiányzó sablonváltozók és több) lásd a [VM-mentés SSH-n keresztül útmutatót](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) a GitHubon.

## Valami nincs helyesen bekötve

Nyisd meg a `/spike` oldalt a webes felületen. A hosztintegráció-ellenőrzés minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat. Itt kezdd, mielőtt hibát feltételeznél: egy hiányzó csatolás vagy egy elérhetetlen hoszt azonnal megjelenik.

## Nem érem el a webes felületet

A BombVault alapból HTTPS-t szolgál ki a `3443` porton (önaláírt tanúsítvánnyal), így nyisd meg a `https://<your-unraid-ip>:3443` címet. Fogadd el az önaláírt tanúsítvány figyelmeztetését, vagy tedd a BombVaultot egy reverse proxy mögé a saját tanúsítványoddal. Ha `HTTP_ONLY=true` mellett futtatod, akkor egyszerű HTTP-t szolgál ki a `3000` porton (egy TLS-lezáró proxy mögötti használatra szánva).

## Elveszítettem az APP_KEY-t

Az `APP_KEY` származtatja a restic tároló jelszavát. Nélküle (és a titkosításikulcs-helyreállító csomag nélkül) a titkosított mentések nem állíthatók helyre. Ezért nyaggat az irányítópult, hogy töltsd le a helyreállítási csomagot. Lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md). Generálj egy kulcsot az `openssl rand -hex 32` paranccsal, és tárold a szerveren kívül, mielőtt bármely mentésre hagyatkoznál.

## A VM-mentés nem csatlakozik

A VM-mentés SSH-n keresztül kommunikál a libvirttel, soha nem egy csatoláson.

- Ellenőrizd, hogy az SSH engedélyezve van-e a hoszton, és a BombVault nyilvános kulcsa engedélyezve van-e a `/root/.ssh/authorized_keys` fájlban (a Beállítások, Rendszer, VM-mentés SSH-n keresztül mutatja a kulcsot és egy **Kapcsolat tesztelése** gombot).
- Egy egyéni `br0.x` hálózaton állítsd a `LIBVIRT_HOST`-ot az Unraid LAN IP-jére (a konténer ott nem éri el a hosztot a `host.docker.internal`-on keresztül). Engedélyezd a **Beállítások, Docker, Host access to custom networks** opciót.
- Ha megváltoztattad az Unraid SSH-portját, állítsd be a `LIBVIRT_SSH_PORT`-ot, hogy egyezzen.
- A teljes, lépésről lépésre diagnózis (elérhetőségi teszt, VLAN-útválasztás, `Permission denied (publickey)`, `Host key verification failed`) a [VM-mentés SSH-n keresztül útmutatóban](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) található.

## Egy élő VM-pillanatkép nem futott le

Az élő pillanatképekhez szükség van a qemu guest agent telepítésére a VM-ben, és arra, hogy a lemez a `/mnt/cache`-en (vagy a `/mnt/diskX`-en) legyen, ne a `/mnt/user`-en. Egy kikapcsolt VM-en az élő automatikusan visszaesik szabályosra. Egy szabályos mentés leállítja a VM-et, menti a lemezeket, majd újraindítja, így mindig konzisztens.

## Egy mentés a "repository is already locked" hibával hiúsult meg

Ez általában egy árva restic zárolás, amely akkor maradt hátra, amikor a konténert egy művelet közben frissítették vagy újraindították. A BombVault észlel egy bizonyíthatóan árva zárolást, kényszerítve törli és egyszer újrapróbálja, automatikusan. Ha továbbra is fennáll, használd a **Beállítások, Integritás és karbantartás, Feloldás** funkciót az érintett tartományhoz, hogy kézzel törölj egy elavult zárolást. Egy valódi probléma továbbra is felszínre kerül, ahelyett hogy elrejtenék.

## A telephelyen kívüli másolatom nem történt meg egy mentés után

A telephelyen kívüli replikáció szándékosan legjobb szándék szerinti, így egy telephelyen kívüli zökkenő soha nem hibáztatja el a helyi mentést. Ellenőrizd az adott tartomány telephelyen kívüli ütemezését (Beállítások, Ütemezések): egy üres ütemezés minden helyi mentés után replikál, míg egy ütem ritkábban szállít. Használd a **Replikálás most** gombot a Telephelyen kívüli fülön egy igény szerinti futáshoz, és figyeld a replikációs jelzőt az irányítópulton.

## Egy visszaállítás megszakadt, mielőtt elindult volna

Mielőtt bármit is leállítanának vagy eltávolítanának, a visszaállítás lefuttat egy előzetes ütközésellenőrzést: ellenőrzi, hogy a konténer statikus IP-je és a közzétett hoszt-portjai szabadok-e. Ha egy másik konténer már foglal egyet, világos, cselekvésre késztető üzenettel megszakad, ahelyett hogy félig kész visszaállítást hagyna hátra. Szabadítsd fel az ütköző portot vagy IP-t, majd próbáld újra.

## Egy egyszerű export meghiúsult ahelyett, hogy fájlt írt volna

Ha az age-titkosítás be van kapcsolva (Beállítások), de nincs beállítva érvényes címzett, egy export világos hibával leáll, ahelyett hogy nyílt szöveget írna. Adj hozzá egy érvényes címzettet (egy age nyilvános kulcs vagy egy SSH nyilvános kulcs), vagy kapcsold ki a titkosítást, ha az exportot nyílt szövegnek szánod. Lásd: [Funkciók](features.md).

## A konténer folyamatosan újraindul vagy egészségtelennek tűnik

A BombVault a saját `/api/health`-jéből jelent egészségeset/egészségtelent. Egy automatikus gyógyító eszköz (mint az Autoheal) automatikusan újraindíthatja, ha a motor valaha beragadna. Ellenőrizd a konténer naplóját és a `/spike` jelentést a mögöttes okért.

## Még mindig elakadtál?

- Olvasd el a teljes [Konfiguráció](configuration.md) és [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md) oldalakat.
- Kérdezz az [Unraid támogatói témában](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Nyiss egy [GitHub-hibajegyet](https://github.com/junkerderprovinz/bombvault/issues).
