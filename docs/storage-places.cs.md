# Úložná místa

Úložné místo je kterékoli místo, kde BombVault uchovává zálohy: složka na tomto Unraidu, sdílená složka na NAS, bucket u cloudového poskytovatele, rest-server, účet SFTP nebo Nextcloud. Každé místo připojíte jednou, v **Nastavení, Úložiště**, a jeho přihlašovací údaje, uchovávání, ochrana a poloha patří k němu. Pět domén (kontejnery, VM, flash, vlastní konfigurace BombVaultu a sady složek) si pak z míst vybírá: kde je každá doména uložena a kam se kopíruje.

## Přidání místa {#add-a-place}

**Přidat místo** otevře okno s jednou dlaždicí na poskytovatele, rozdělené do tří skupin: cloudové úložiště, služby na vlastním hostingu a nakonec zařízení NAS spolu s tímto serverem.

1. Vyberte dlaždici a vyplňte její formulář. Tlačítko s okem ukáže tajný údaj, který jste zadali.
2. **Otestovat připojení** místo zkontroluje a nic nezaloží. U složky každé domény oznámí, co našlo: prázdnou nebo zatím neexistující složku, složku, ve které už leží repozitář restic, nebo chybu, která test zastavila.
3. Pojmenujte místo; název poskytovatele je předvyplněný. U zařízení, které provozujete sami, odpovězte na **Kde je zařízení?**. Cloudoví poskytovatelé jsou vždy v jiné lokalitě a složka na tomto Unraidu je vždy tady.
4. **Přidat** místo uloží.

Nové místo zatím žádná doména nepoužívá. Zvolte ho pod **Uloženo v** nebo **Kopírováno do** na [kartě Domény](#domains), nebo na kartě položky jen pro tuto položku.

## Složky {#folders}

Místo drží jednu složku na doménu: `container`, `vms`, `flash`, `config` a `files`, tedy názvy, které používají výchozí umístění záloh. Složky jsou uvedené v podrobnostech místa a lze je tam přejmenovat (viz [Změna adresy](#addresses)). Doména, která na místě nemá složku, si toto místo nemůže zvolit.

Když doména používá místo v obou rolích, druhá role dostane příponu a první si ponechá svou složku. Místo, které už přijímá kopie domény, ukládá položky poslané přímo na něj do `<folder>-direct`; místo, na kterém už je doména uložena, přijímá její kopie do `<folder>-copies`.

Některá místa jsou sama repozitářem restic: adresa, na které už repozitář ležel, když se místo přidávalo, pojmenovaný repozitář ze stávajícího nastavení, nebo cíl kopií v kořeni bucketu. Takové místo nemá složky, všechny domény sdílejí jeho jediný repozitář a druhou roli nepřebírá. Chcete-li u stejného poskytovatele ukládat víc, připojte další bucket nebo složku jako samostatné místo.

## Podrobnosti místa {#details}

Každé místo je řádek s poskytovatelem, s tím, k čemu slouží, a s posledním testem nebo kopií. **Otestovat** zkontroluje každou adresu na místě a **Podrobnosti** otevře jeho nastavení. Každá změna v podrobnostech se uloží hned, jak ji uděláte.

- **Obecné**: název, přepínač, který místo zapíná a vypíná, adresa a u zařízení, které provozujete sami, **Kde je zařízení?** (viz [Mimo objekt](#off-the-premises)).
- **Uchovávání**: keep-last, denní, týdenní a měsíční, pro každý repozitář na místě. Nové místo začíná s výchozími pravidly; místo se všemi pravidly na nule nikdy nic neprořezává.
- **Ochrana**: přepínač **Append-only**. Append-only musí vynucovat druhá strana; se zapnutým přepínačem tam BombVault nikdy neprořezává ani nemaže. Na rest-serveru se zapnutým append-only spustí **Otestovat append-only** test odolnosti proti každé cestě domény, zapnuté kopii a repozitáři na místě a ukáže jednu odpověď za celé místo, *mazání odmítnuto* nebo *mazání přijato* (viz [Mimo lokalitu a obnova](offsite-recovery.md)). Tuto sekci mají jen vzdálená místa, protože nic na tomto stroji nemůže zabránit smazání místního repozitáře.
- **Přístup**: přihlašovací údaje a u S3 třída úložiště. Místo, které používá sdílené přihlašovací údaje, dostane při první změně vlastní sadu. Přímý repozitář na místě, který nové údaje neotevřou, si ponechá staré a odpověď to oznámí. Místa typu složka, SFTP a rclone tuto sekci nemají.
- **Limity**: rychlost nahrávání a stahování a rozpočet růstu.
- **Složky**: jeden přepínač na doménu, s názvem její složky. Doména, která je tu vypnutá, si místo nemůže zvolit.

Snížení uchovávání se nejdřív zeptá a řekne, kolika položek se to týká; vypnutí append-only se nejdřív zeptá a řekne, kolik repozitářů na místě o ně přijde. Vypnutí místa vypne každý repozitář na něm; místo, na kterém je uložena některá doména, vypnout nelze.

## Karta Domény {#domains}

Karta má jeden řádek na doménu, s jejím plánem, s tím, kde je uložena a kam se kopíruje, a s jejími výjimkami.

- **Uloženo v**: dokud umístění záloh domény neobsahuje žádné zálohy, stane se zvolené místo místem uložení domény a umístění se přesune tam. Jakmile zálohy obsahuje, stane se volba u kontejnerů, VM a sad složek výchozí hodnotou pro nové položky, které ji převezmou při své první záloze; položky, které už zálohy mají, zůstanou, kde jsou, protože BombVault zálohu nikdy nepřesouvá. U flash a vlastní konfigurace BombVaultu se místo uložení přesune a zálohy, které už jsou zapsané, zůstanou na starém místě.
- **Kopírováno do**: jeden čip na každé místo, které může přijímat kopie domény. Zaškrtnutím čipu se místo stane cílem kopií domény; poprvé BombVault předem řekne, kolik položek a snímků a kolik dat pošle první běh. Odškrtnutí zastaví nové kopie: kopie, které tam už jsou, zůstanou a stárnou podle uchovávání místa, a položky s vlastní volbou tam kopírují dál. Odškrtnutí posledního čipu zastaví všechny kopie, i na místa přidaná později, dokud se znovu nějaký nezaškrtne. Vypnuté místo se zobrazí jako ztlumený čip a nelze ho zvolit.
- **Výjimky**: položky s vlastní volbou, jako seznam s odkazy na jejich karty.
- **Kopírovat nyní** spustí kopie domény okamžitě.

Doména pozastavená po opětovném sestavení přes Objevit zálohy ukazuje pozastavení na svém řádku, s tlačítkem **Potvrdit výchozí nastavení** (viz [Umístění pro jednotlivé položky](offsite-recovery.md#placement)).

## Změna adresy {#addresses}

Složku domény lze změnit v podrobnostech místa, stejně jako adresu místního úložného místa, například když byl repozitář ručně přesunut na jiný disk. BombVault otestuje každou adresu, které se změna týká, a přijme ji, když je každá nová adresa prázdná a na staré nic nebylo uloženo, nebo když každá nová adresa obsahuje tentýž repozitář restic jako stará. Cokoli jiného odmítne, s počtem záloh, které na staré adrese stále leží. Vzdálené místo si adresu ponechá; chcete-li zálohovat jinam, připojte to jako samostatné místo.

BombVault sestavuje seznam míst ze své vlastní databáze a kvůli němu nikdy nevypisuje žádný vzdálený repozitář; test běží, jen když něco změníte.

## Odebrání místa {#remove}

Místo lze odebrat, jen dokud ho nic nepoužívá: žádná doména na něm není uložena, žádná výchozí hodnota na něj neukazuje, žádná položka na něm není uložena a žádný přímý repozitář na něm nedrží položky. Jinak odmítnutí vypíše, co ho drží. Odebrání vezme s sebou jeho cíle kopií a také jeho vlastní přihlašovací údaje, pokud je nepoužívá zdroj stahování nebo jiné místo. V samotném úložišti se nic nesmaže a potvrzení řekne, kolik kopií tam zůstane.

## Bez místa {#without-a-place}

Adresa, která neodpovídá tvaru místa se složkou, dál funguje a je uvedena pod **Bez místa**, se svou adresou. Patří k nim nativní adresy `b2:`, `gs:` a `swift:`. **Přiřadit k místu** připojí takový řádek k místu, po stejném testu jako při [změně adresy](#addresses). Cíl kopií bez místa je také uveden na řádku své domény, vedle čipů, a kopíruje dál. Vzdálený řádek pod **Bez místa** má vlastní přepínač **Append-only** a jeho vypnutí se nejdřív zeptá s počtem položek, které mají na té adrese zálohy. Přímý repozitář se řídí přepínačem svého cíle.

## Mimo objekt {#off-the-premises}

**Kde je zařízení?** má dvě odpovědi: **Tady v domě** a **V jiné lokalitě**. Kopie se počítá jako samostatná lokalita, pro řádek 3-2-1 na kartách a pro kontroly mimo lokalitu na Přehledu, jen když je její místo v jiné lokalitě. Druhý disk nebo NAS ve stejném domě je druhá kopie, ne druhá lokalita. Odpověď nemění žádnou kopii. Cloudoví poskytovatelé jsou vždy v jiné lokalitě a složka na tomto Unraidu vždy tady, takže se na ně formulář neptá; u každého jiného místa odpověď změníte v jeho podrobnostech. Místo v jiné lokalitě nese na svém řádku označení **Jiná lokalita**.

## Druhy připojení

### Složka na tomto Unraidu nebo NAS {#kind-local}

Adresa je cesta pod `/mnt`, zapsaná bez `/mnt`, například `user/bombvault`, a složka každé domény leží pod ní: `user/bombvault/container`.

- **Složka na tomto Unraidu** vybírá ze sdílených složek, disků a poolů.
- **Synology**, **QNAP**, **TrueNAS**, **Jiný Unraid** a **Jiné sdílení** vybírají z `/mnt/remotes`. Sdílenou složku nejdřív připojte v Unraidu, například pluginem Unassigned Devices. Host Data musí být připojeno jako Read/Write - Slave, jinak sdílená složka, která se připojí až po spuštění BombVaultu, zůstane neviditelná až do restartu (viz [Konfigurace](configuration.md)).

Výběr složky vytvoří tlačítkem **Nová složka** složku tam, kde právě stojí. Test zkontroluje, že je složka prázdná nebo neexistuje a že do ní BombVault může zapisovat.

### S3 {#kind-s3}

Adresa je `s3:https://<endpoint>/<bucket>/<path>`, například `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** potřebuje jen ID klíče a aplikační klíč. BombVault se B2 zeptá, na který bucket, S3 endpoint a složku je klíč omezený, a sestaví z nich adresu. Klíč, který smí ke všem bucketům, nabídne své buckety k výběru.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** a **Vultr** se ptají na klíč a tam, kde to poskytovatel potřebuje, na region, ID účtu nebo endpoint. BombVault doplní endpoint a vypíše buckety, když je klíč smí vypsat; jinak zadejte název bucketu.
- **Google Cloud Storage** jde přes své rozhraní S3 s klíčem HMAC, který vytvoříte v nastavení Cloud Storage v části Interoperability. Soubor servisního účtu tu nefunguje.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** a **Jiná služba S3** přijímají adresu služby a klíč.

Třída úložiště se nastavuje v podrobnostech místa, omezená na úrovně, které obnova přečte bez rozmrazení.

### rest-server {#kind-rest}

Adresa je `rest:<url>/<user>`, například `rest:https://nas.lan:8000/tower`. Formulář se ptá na adresu serveru, uživatele a heslo. S `--private-repos` smí uživatel přistupovat jen k cestám, které začínají jeho vlastním jménem, proto BombVault dá uživatele na začátek, pokud nezadáte jinou cestu. Když server odmítne cestu mimo vlastní cestu uživatele, chyba to řekne.

Formulář rest-serveru obsahuje recept připravený ke vložení pro rest-server v režimu append-only s jedním uživatelem pro tento BombVault. **Zobrazit recept** vytvoří heslo, které se zobrazí jen jednou, a dá řádek `docker run`, soubor compose a šablonu pro Unraid, každý s řádkem `htpasswd`, který patří na server; uživatel a heslo jdou rovnou do formuláře.

**Jiný BombVault** vypisuje nad svými poli otevřené nabídky, které jiné instance poslaly přes Fleet. Přijetím nabídky se přidá místo, které uchovává kopie jen nabídnuté domény, protože nabídka nese uživatele pro tuto jedinou doménu. Přijetí na stránce Fleet přidá totéž místo.

### SFTP {#kind-sftp}

Adresa je `sftp://<user>@<host>:<port>/<path>`, například `sftp://bv@backup.lan:22/bombvault`. Formulář se ptá na hostitele, port a uživatele a ukazuje veřejný klíč BombVaultu. Přidejte tento klíč do `~/.ssh/authorized_keys` uživatele na serveru; nic jiného tam instalovat není třeba. BombVault přijme hostitelský klíč serveru při prvním kontaktu a od té doby ho kontroluje.

**Hetzner Storage Box** doplní `<user>.your-storagebox.de` a port 23. Klíč na box nainstalujte vlastním příkazem Hetzneru, který se jednou zeptá na heslo boxu:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Formulář se ptá na adresu serveru, uživatele a heslo aplikace. Heslo aplikace vytvořte v nastavení zabezpečení účtu a zadejte ID uživatele, ne e-mailovou adresu. BombVault sestaví cestu WebDAV, kterou daný produkt používá, a předá připojení resticu přes proměnné prostředí rclone, s heslem v podobě zamaskované pro rclone. Adresa zní `rclone:bvp<id>:<path>`, kde `bvp<id>` je remote, který existuje jen v tomto prostředí; do konfigurace rclone se nic nezapíše.

### Azure Blob {#kind-azure}

Adresa je `azure:<container>:/<path>`. Formulář se ptá na účet úložiště a jeho přístupový klíč; po **Otestovat připojení** vypíše kontejnery účtu k výběru, nebo zadáte název kontejneru sami. BombVault předá účet a klíč resticu jako `AZURE_ACCOUNT_NAME` a `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Adresa je `rclone:<remote>:<path>`. Formulář nabídne k výběru remotes z konfigurace rclone v BombVaultu. Chcete-li tuto konfiguraci nahradit, vložte celý `rclone.conf` pod **Konfigurace rclone** a klikněte na **Uložit konfiguraci**. Uloží se okamžitě a slouží všem místům rclone, ať už okno pak nějaké místo přidá, nebo ne.

## Kopie mezi místy s různými přihlašovacími údaji {#different-credentials}

Doména uložená na vzdáleném místě je zdrojem svých kopií. `restic copy` běží s jedním prostředím a BombVault přidá přihlašovací údaje zdroje k údajům cíle, pokud oba nenastavují stejnou proměnnou na různé hodnoty. Místo v Nextcloudu a místo v B2 používají různé proměnné, takže doménu uloženou v Nextcloudu lze kopírovat do B2. Dva účty S3 nebo dva uživatelé rest-serveru by potřebovali stejné proměnné s různými hodnotami; restic nemůže přijmout obojí a čip na kartě Domény řekne, že přihlašovací údaje k sobě nesedí.
