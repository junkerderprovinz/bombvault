# Zálohy VM

BombVault zálohuje VM v KVM/libvirt bez připojení jakékoli cesty libvirt. Spouští `virsh` na hostiteli přes SSH (`qemu+ssh://`), čte a zapisuje disky VM přes připojení Host Data (`/mnt` na hostiteli je v kontejneru `/host/user`) a UEFI NVRAM kopíruje přes SSH. Protože se nic, co patří libvirt, nepřipojuje, nemůže BombVault nikdy překážet VM Manageru v Unraidu.

Tento průvodce pokrývá nastavení, metody zálohování, VM se snapshoty Unraidu a TrueNAS SCALE. Zálohy kontejnerů nic z toho nepotřebují.

Zálohy datových sad ZFS používají stejné spojení SSH, se stejným klíčem, hostitelem a uživatelem. Jakmile se zálohy VM připojí, připojí se i položky datových sad ZFS. Viz [Datové sady ZFS](zfs-datasets.md).

## Co nastavuješ {#what-you-configure}

| Nastavení | Kde | Výchozí | Význam |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | proměnná šablony | `host.docker.internal` | Hostitel Unraid, ke kterému se přistupuje přes SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | proměnná šablony | `22` | Port SSH Unraidu |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | proměnná šablony | `root` | Uživatel SSH na hostiteli |
| Veřejný klíč | **Nastavení, Integrace, SSH k hostiteli** | vytvoří se při prvním startu | Autorizuj ho na hostiteli |

BombVault vytvoří svůj pár klíčů SSH při prvním startu v `/config/ssh/id_ed25519`, uložený v appdata. Klíč hostitele se při prvním připojení připne do `/config/ssh/known_hosts`.

Šablona vyplní **Host SSH: Address** zástupnou hodnotou `192.168.x.x`. BombVault ji bere jako nenastavenou a zkusí `host.docker.internal`, což funguje na výchozí síti `bridge`. Na vlastní síti ji nahraď LAN IP hostitele (krok 3 B).

## Krok 1: Povol SSH na hostiteli Unraid {#step-1}

1. V Unraidu otevři **Settings, Management Access** a nastav **Use SSH** na **Yes**.
2. Pokud jsi změnil port SSH, poznamenej si ho. **Host SSH: Port** se s ním musí shodovat.

## Krok 2: Autorizuj veřejný klíč BombVaultu {#step-2}

`/root` v Unraidu žije v paměti a při každém startu se vymaže, takže klíč musí být i na flashce. Spusť to v terminálu Unraidu:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 a novější při každém startu zkopírují `/boot/config/ssh/root.pubkeys` do `authorized_keys` roota, takže to přežije restarty. Platí to pro každé přihlášení jako `root`, ať BombVault používá jakékoli jméno hostitele nebo port.

## Krok 3: Síť {#step-3}

BombVault musí otevřít spojení TCP z kontejneru na port SSH hostitele. Vyber případ, který odpovídá tvému nastavení.

### A. BombVault na výchozí síti bridge {#bridge}

- Nech **Host SSH: Address** na `host.docker.internal` nebo na zástupné hodnotě šablony. Šablona přidává `--add-host=host.docker.internal:host-gateway`, což ukazuje na bránu docker0, tedy na hostitele.
- Nastav **Host SSH: Port** na svůj port SSH.

### B. BombVault na vlastní síti {#custom-network}

Kontejner na `br0.x`, macvlan nebo ipvlan nedosáhne na hostitele přes `host.docker.internal`, protože 172.17.0.1 je docker0 a z `br0.x` není dosažitelná. Místo toho:

1. V Unraidu nastav **Settings, Docker, Host access to custom networks** na **Enabled**.
2. Nastav **Host SSH: Address** na LAN IP hostitele Unraid, tedy adresu, na které otevíráš webové rozhraní.
3. Pokud jsou síť kontejneru a hostitel v různých VLAN, povol trasu na routeru nebo firewallu: z VLAN kontejneru na IP hostitele na portu SSH, přes TCP.
4. Pokud se na LAN IP hostitele pořád nedá dostat, použij IP shim hostitele v podsíti kontejneru. Najdeš ji na hostiteli pomocí `ip -4 addr show | grep -B2 <container-subnet>`.

### Ověř, že hostitel odpovídá {#verify-reachability}

Spusť v terminálu Unraidu, se svou IP a portem:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` znamená, že cesta funguje. `UNREACHABLE` znamená, že síť z kroku 3 je potřeba opravit dřív než cokoli jiného.

## Krok 4: Nastav proměnné a vyzkoušej {#step-4}

1. V Unraidu otevři **Docker**, klikni na BombVault, zvol **Edit**, nastav **Host SSH: Address** (a **Host SSH: Port**, pokud to není 22) a stiskni **Apply**. Pokud proměnné chybí, naimportuj šablonu znovu, protože Unraid drží uloženou konfiguraci existujícího kontejneru.
2. V BombVaultu otevři **Nastavení, Integrace, SSH k hostiteli** a stiskni **Otestovat připojení**. Mělo by zezelenat.

Stejné volání z terminálu vypíše tvé VM:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Krok 5: Metoda pro každou VM {#per-vm-method}

Každá VM má **Metoda** v kartě **VM** na své záložce Nastavení (otevři VM pod **Zálohy**):

- **Graceful (vypnutí)** je výchozí. VM vypne, zazálohuje disky a znovu ji spustí, takže je záloha vždy konzistentní. Čeká až deset minut, než se host vypne, a nikdy ho nevypíná násilím. Host, který potom stále běží, zůstane nedotčen a tato záloha selže s uvedením důvodu. Pokud se host vypne během další hodiny, BombVault ho znovu spustí. Pokud to spuštění selže, neúspěšný běh to řekne.
- **Živý snímek** zálohuje běžící VM přes externí snapshot a `blockcommit`. Potřebuje disk na `/mnt/cache` nebo `/mnt/diskX`, ne na `/mnt/user`. S qemu guest agentem ve VM je snapshot klidný a bez něj je crash-konzistentní. U VM, která je vypnutá, se Živý snímek vrátí ke Graceful sám. Když se snapshot nedá vytvořit, tato záloha selže a VM běží dál, protože VM, kterou sis vybral zálohovat živě, se kvůli tomu nikdy nevypne.

## Co záloha vynechá {#left-out}

Záloha může uspět, a přesto neobsahovat celou VM. Běh pak nese poznámku ve varovném tónu, která jmenuje, co chybí:

- **Disky, které BombVault nečte.** Disk jen pro čtení, `lun` a disk typu `network` nebo `volume` se vynechají a poznámka každý z nich jmenuje s cílovým zařízením a druhem. Definice v záloze je stále vypisuje. Cdrom a disketa se nejmenují. Obyčejné blokové zařízení, které není ZFS zvol, místo toho zálohu shodí.
- **Stav emulovaného TPM.** V XML domény nemá žádnou cestu. VM Manager Unraidu nastavuje takové TPM pro VM s BIOSem **OVMF TPM**, takže se poznámka objevuje i na Unraidu. Host, který drží tajemství ve svém TPM, jako to umí BitLocker, si po obnově, která ten stav ztratí, může říct o obnovovací klíč, takže ten klíč měj po ruce.

## Tenké disky {#thin-disks}

Obnova ponechá tenký disk tenký, ať disk nahrazuje, nebo zapisuje nový: restic zapisuje úseky nul jako díry v novém souboru a BombVault u nahrazeného souboru vrátí nulové bloky pomocí `fallocate --dig-holes`, jakmile je zapsaný. Zatímco restic soubor nahrazuje, zabírá ten soubor plnou velikost, pokud na to má pool místo. Souborový systém, který neumí vyrážet díry (hole punching), ponechá disk v plné velikosti a log to řekne. Kontrola místa i tak počítá nový disk v plné velikosti.

## Obnova, která selže {#restore-fails}

Obnova na místě VM vypne, zapíše disky a definici a NVRAM nahradí, až když jsou disky zapsané. Obnova, která selže, nechá VM definovanou i s její NVRAM. Pokud se její disky nezměnily a běžela, poběží znovu. Pokud se změnit mohly, zůstane vypnutá, protože nabootování napůl zapsaného disku by do něj zapisovalo, a běh to řekne. Opětovná obnova téže zálohy práci dokončí.

Snapshoty evidované v libvirtu, což není způsob, jakým je vytváří VM Manager Unraidu, zastaví obnovu na místě dřív, než na VM sáhne, protože libvirt nemůže odebrat VM, která je má. Nejdřív je smaž.

## Jen změněné bloky {#changed-blocks}

VM s disky qcow2 lze zálohovat tak, že se čte jen to, co se změnilo. Zapni **Jen změněné bloky** ve stejné kartě a BombVault místo výše uvedené metody použije záložní API libvirt:

- Spustí úlohu zálohy v režimu pull (`virsh backup-begin`), která ve stejném okamžiku vytvoří checkpoint jménem `bombvault-<time>`, a čte disky přes spojení SSH přes přeposlaný socket Unix. Když guest agent odpovídá, souborové systémy hosta se zmrazí jen po dobu startu úlohy. Rozmrazení, které selže, se zkouší znovu asi patnáct sekund, a pokud host stále nerozmrazí, BombVault pošle oznámení.
- První běh čte každý přidělený blok. Další běhy čtou jen bloky, které bitmapa checkpointu označuje jako zapsané od předchozí zálohy.
- Každý disk se ukládá jako složka segmentů pevné velikosti, které drží to, co host vidí. Nezměněné segmenty se přebírají z předchozího snapshotu bez čtení, takže každý snapshot je úplný a obnoví se sám o sobě.
- Obnova sestaví disk ze segmentů a převede ho zpět na qcow2 pomocí `qemu-img`. Disk má stejný obsah jako v době zálohy, ale soubor je nový soubor qcow2 a řetěz podkladových souborů se vrátí zploštělý.
- Kontrola před takovou obnovou vypíše každý disk, který sestavuje, a počítá ho v plné velikosti hosta, u qcow2 dvakrát, protože surový obraz a jeho převedená kopie leží vedle sebe, dokud se starý soubor nenahradí. To je nejvíc, co disk může zabrat, takže nedostatek podle tohoto počtu nechá řádek s místem šedý místo toho, aby blokoval. Kontrola obnovy po první záloze a **Ověřit obnovu** také ověřují, že snapshot drží každý segment každého disku v očekávané velikosti.
- Mezi běhy nemá VM v libvirt zapsaný žádný checkpoint. BombVault drží definici nejnovějšího checkpointu `bombvault-` v `/config/vm-checkpoints` a odstraní jen záznam o něm v libvirt (`virsh checkpoint-delete --metadata`). Jeho bitmapa zůstává v souboru qcow2 a počítá zápisy dál a další běh checkpoint před startem vrátí zpět (`virsh checkpoint-create --redefine`). libvirt odmítá zrušit definici VM, která má checkpointy, takže díky tomu fungují **Remove VM** v Unraidu a obnova jako obvykle.
- Záložní úloha, kterou BombVault nechal běžet, protože se zastavil uprostřed zálohy, se ukončí při dalším startu BombVaultu, spolu s checkpointy, které ten běh nechal zapsané.
- Vypnutí přepínače, vyřazení VM z plánu nebo smazání jejích záloh také takovou úlohu ukončí a bitmapu navíc smaže, dokud VM běží. U VM, která je vypnutá, ji libvirt smazat nemůže a zůstane nevyužitá v obrazu.
- Checkpointy jiných nástrojů se nechávají na pokoji. VM, která nějaké má, nejde obnovit, dokud se nesmažou, a obnova se zastaví dřív, než se VM dotkne.

Přepínač se vrací sám. Chybějící nebo rozbitý checkpoint, disk, který změnil velikost, nebo nejnovější snapshot, který není snapshotem jen změněných bloků, znamená, že se celý disk přečte ještě jednou. Vypnutá VM, surový disk nebo zvol, žádné spojení SSH k hostiteli, libvirt bez záložního API nebo příliš málo volného místa pod `/config` znamená běžnou zálohu metodou výše. Karta říká, kterou z nich poslední běh použil a proč.

Požadavky: libvirt se záložním API (vyzkoušeno s libvirt 12.2 a QEMU 10.2 na Unraidu 7.3), sshd s `AllowStreamLocalForwarding yes` (výchozí, i na Unraidu, který vypíná přeposílání TCP) a disky qcow2. libvirt odmítá checkpointy na surových discích a Unraid vytváří vdisky ve výchozím stavu surové. VM během zálohy běží dál jako u Živého snímku a segmenty čekající na restic zabírají pod `/config` až 4 GiB.

## VM se snapshoty Unraidu {#unraid-snapshots}

Snapshot, který pořídí VM Manager Unraidu, změní disk qcow2 v řetěz: původní obraz dole a nad ním jeden soubor za každý snapshot, každý drží jen to, co se změnilo od toho pod ním. VM zapisuje do nejnovějšího souboru. Disk se dvěma snapshoty jsou tři soubory.

BombVault takovou VM zálohuje celou:

- **Každý soubor řetězu.** Sleduje řetěz podkladových souborů každého disku až do konce a zálohuje každý soubor v něm, spolu se seznamem snapshotů Unraidu (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, čteným přes SSH). Řetěz, který nejde sledovat až do konce, například protože chybí soubor uprostřed, zálohu shodí a jmenuje soubor.
- **Živá záloha snapshoty zachová.** **Živý snímek** položí při čtení disku nahoru dočasný overlay a poté ho sloučí zpět, jen do obrazu těsně pod ním. Řetěz je po záloze stejný jako před ní. VM bez snapshotů se chová jako vždy.
- **Obnova vrátí řetěz.** Obnova na místě zapíše každý soubor zpátky, kde byl, a vrátí seznam snapshotů Unraidu, takže VM Manager snapshoty zase vypíše. Obnova vedle originálu zapíše soubory do složky kopie a obnovené disky nasměruje na obnovené soubory, takže kopie nikdy nečte z obrazů originálu. Seznam snapshotů Unraidu se vrací jen při obnově na místě, protože jmenuje soubory tam, kde byly zálohovány. Bez něj kopie nabootuje stejně, jen ji VM Manager vypíše bez snapshotů.
- **Jen změněné bloky** čte to, co host vidí, takže její obnova přijde jako jeden zploštělý soubor qcow2 bez snapshotů.
- **Obnova vrátí disk zálohy, kterou vybereš.** Snapshoty pořízené nebo sloučené od té zálohy na tom nic nemění. Záloha z doby před snapshotem definuje VM na souboru, který obsahuje, a záloha z doby před sloučením vrátí svůj overlay a běží na něm. Novější soubory zůstanou, kde jsou, nepoužité a nezměněné. Záloha, která obsahuje overlay, ale ne soubor pod ním, se odmítne dřív, než obnova na VM sáhne. Definice VM po obnově nevypisuje žádný řetěz, takže libvirt řetěz přečte z obnovených souborů.

### Příklad: VM se dvěma snapshoty {#example-unraid-snapshots}

VM `homeassistant` má jeden disk, `vdisk1.qcow2`, a dva snapshoty, které jsi pořídil ve VM Manageru před dvěma aktualizacemi. Její disk je teď řetěz tří souborů. Běží pořád, takže používá **Živý snímek**.

1. Otevři **Zálohy**, pak `homeassistant` a její záložku **Nastavení**. V kartě **VM** zkontroluj, že **Metoda** je **Živý snímek** a **Jen změněné bloky** je vypnuté.
2. Stiskni **Zálohovat nyní** v hlavičce stránky.
3. Až běh skončí, jeho řádek v **Nedávné běhy** na záložce **Ověření** je zelený. VM po celou dobu běžela dál a záloha drží všechny tři soubory disku a seznam snapshotů Unraidu.
4. Ve VM Manageru jsou oba snapshoty pořád tam a disk se pořád čte přes stejné tři soubory.

O měsíc později se aktualizace pokazí a chceš včerejší stav vedle rozbitého, abys je porovnal:

1. Otevři **Obnova**, vyber **VM** a klikni na `homeassistant`.
2. Nech doporučenou zálohu z minulé noci a v kroku **Kam** zvol **Vedle originálu**.
3. Stiskni **Obnova**. Tři soubory skončí ve složce pojmenované po VM s `-restored` a obnovený disk ukazuje na obnovené soubory. Na běžící VM se nic nemění.

Chceš-li místo toho vrátit celou VM zpět, zvol **Nahradit originál** se zapnutým **Nejdřív zálohovat současný stav**. BombVault zazálohuje rozbitý stav, VM vypne, zapíše zpět tři soubory a seznam snapshotů Unraidu a spustí ji znovu.

## TrueNAS SCALE {#truenas-scale}

Kroky výše jsou psané pro Unraid, ale stejné spojení `qemu+ssh://` funguje proti každému libvirtd, na který kontejner dosáhne, včetně TrueNAS SCALE, který má VM na libvirt zase od 25.04.2. Na TrueNAS potřebuje spojovací řetězec přepsání a klíč jde na roota v nastavení uživatelů TrueNAS. Tři další věci mění, co záloha drží: disky na zvolech, stav vTPM a jména, která TrueNAS svým VM dává.

### 1. Socket libvirt {#truenas-socket}

libvirtd v TrueNAS nenaslouchá tam, kde to čeká spojovací řetězec sestavený z polí **Host SSH**. Nastav místo toho `LIBVIRT_URI`, s parametrem `?socket=` navíc:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault používá `LIBVIRT_URI` přesně tak, jak je napsané, a `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` a `LIBVIRT_SSH_PORT` nejsou potřeba. U každé z nich, která není nastavená, BombVault bere hostitele, uživatele a port z URI i pro své vlastní příkazy SSH, takže kopie NVRAM a zálohy datových sad ZFS míří na stejný stroj jako `virsh`. Proměnná, kterou nastavíš sám, vždy vyhraje, a log při startu jmenuje cíl, který vzal z URI. Na TrueNAS potřebuje uživatel jiný než root pro zálohy datových sad ZFS také `zfs allow <user> snapshot,destroy,mount <dataset>`, viz [Datové sady ZFS](zfs-datasets.md#truenas).

Do URI dej LAN adresu TrueNAS. Když není nastavená ani `LIBVIRT_URI`, ani `LIBVIRT_HOST`, vezme BombVault jako hostitele výchozí bránu aplikace, protože `host.docker.internal` se v aplikaci TrueNAS nepřeloží, a log při startu jmenuje adresu, kterou vybral. To stačí pro zálohy datových sad ZFS. Zálohy VM pořád potřebují URI kvůli socketu.

Aplikace potřebuje pro zálohy VM capability `SETPCAP`, vedle `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` a `SETUID`. Bez ní se libvirt zastaví s „cannot apply process capabilities“. YAML v [TrueNAS SCALE](getting-started.md#truenas) nastavuje všechny, spolu se `SYS_CHROOT`, které potřebuje snímek místo zastavení.

Na TrueNAS SCALE 25.10.0 byla cesta k socketu ověřena na skutečném železe: výchozí `qemu:///system` selže s `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory` a stejné volání s `?socket=/run/truenas_libvirt/libvirt-sock` domény vypíše.

### 2. Klíč BombVaultu na TrueNAS {#truenas-key}

Krok 2 výše zapisuje na flashku Unraidu a tady neplatí. Na TrueNAS:

1. Zapni službu SSH v **System, Services**.
2. V **Credentials, Users** uprav **root** a vlož veřejný klíč BombVaultu z **Nastavení, Integrace, SSH k hostiteli** jako jeho veřejný klíč SSH. TrueNAS ho drží přes restarty i aktualizace.

Čerstvé instalace TrueNAS 24.10 a novějších dávají rootu žádné heslo a přihlášení klíčem žádné nepotřebuje. Pokud root používat nechceš, autorizuj klíč pro účet správce se sudo a dej jeho jméno do `LIBVIRT_URI` místo `root`. Zálohy datových sad ZFS pak potřebují `zfs allow`, jak je popsáno výše.

Replika ZFS na druhý server se přihlašuje vlastním klíčem, který ukazuje dialog repliky. Klíč SSH k hostiteli repliku neautorizuje a klíč repliky neautorizuje zálohy VM.

!!! warning "TrueNAS nevidí změny provedené přes virsh"
    Middleware TrueNAS, vrstva za jeho obrazovkami pro VM, má být jediná cesta, kterou se jeho VM mění. Příkaz `virsh` přes toto spojení SSH, což je to, co spouští záloha a obnova VM v BombVaultu, ji obchází a TrueNAS může později sladit nebo přepsat stav, který nevytvořil. Vychází to z dokumentovaného chování TrueNAS a na skutečné krabici se to neprojevilo. Je to stejný druh rizika jako to, že VM Manager v Unraidu sladí změny provedené mimo něj, jen ostřejší na TrueNAS.

### 3. Disky na zvolech {#truenas-zvols}

Disky VM v TrueNAS jsou často zvoly, blokové zařízení ZFS na `/dev/zvol/<pool>/<dataset>`, ne soubory qcow2. restic nemůže zálohovat blokové zařízení podle jeho cesty tak, jak zálohuje soubor, proto BombVault pozná disk na zvolu v XML domény (`<source dev="...">` místo `<source file="...">`) a obslouží ho přes stejné spojení SSH. Kontejner BombVault nepotřebuje žádné nástroje ZFS.

Záloha zvolu:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` zafixuje okamžik, i když VM běží.
2. `zfs clone -o volmode=dev -o readonly=on` zpřístupní ten snapshot jako zařízení. `volmode=dev` brání hostiteli skenovat oddíly hosta.
3. `dd` přečte zařízení a surový obraz jde do resticu jako jeden soubor, `/vm-disks/<dataset>.img`, označený `vm:<name>:zvol:<dev>`. restic ho rozseká na kusy podle obsahu, takže bloky, které se od poslední zálohy nezměnily, se neukládají znovu.
4. Klon a snapshot se potom zničí, ať záloha uspěla, nebo ne. Pokud se BombVault zastavil uprostřed zálohy, další záloha toho zvolu nejdřív odstraní snapshot `bombvault-vm-<time>` a jeho klon `<dataset>-bombvault-vm-<time>`. Každý jiný snapshot nechá na pokoji a ponechá ten, ze kterého se klonovalo něco jiného. Log jmenuje, co odstranil nebo ponechal.

BombVault na to nepoužívá `zfs send`. Každý záznam proudu send nese GUID snapshotu a průběžný kontrolní součet, takže restic by z poslední zálohy nenašel nic k opětovnému použití a ukládal by celý disk pokaždé znovu. Změřeno na TrueNAS SCALE 25.10 se zvolem 512 MiB, který drží 200 MiB dat: první záloha přidala 211 MB, druhá bez změny 52 KB a ta po změně 1 MiB 3,6 MB. Stejný zvol přes `zfs send` přidal 214 MB při každém běhu.

Každý běh také zapíše malý snapshot označený `vm:<name>` s XML domény, jakmile jsou všechny disky uvnitř. VM, jejíž disky jsou všechny zvoly, nemá žádný souborový snapshot a právě tento zařadí běh pod VM a umožní její obnovu.

Obnova zapíše každý zvol zpátky na místo, tak jako se souborový disk zapisuje do svého souboru, takže obnovená VM nabootuje obnovená data:

1. Než se čehokoli dotkne, BombVault zkontroluje, že běh drží obraz každého disku na zvolu. Běh, kterému některý chybí, se odmítne a VM zůstane, jak je.
2. Existující zvol dostane napřed snapshot, `<dataset>@bombvault-before-restore-<time>`, s časem v UTC jako každé jméno snapshotu, které BombVault vytváří, takže se může lišit o hodiny od místních hodin. Log ho jmenuje a `zfs rollback` na něj obnovu vrátí, dokud je nejnovějším snapshotem zvolu. Jakmile existuje novější, třeba z příští obnovy, udělá to `zfs rollback -r` a novější snapshoty zničí. Zůstává, dokud ho nestrhneš pomocí `zfs destroy`, až obnovená VM funguje.
3. Zvol, který už neexistuje, se vytvoří ve velikosti, jakou měl v době zálohy, a existující o jiné velikosti se vrátí na tu velikost.
4. `dd` zapíše obraz na `/dev/zvol/<dataset>`. Se zapnutou kompresí, výchozí v TrueNAS, se prázdné části obrazu vrátí jako díry a nezabírají místo.

**Obnova na jinou instanci potřebuje cílový pool.** Přesouvá souborové disky do cílové složky, ale složka nejmenuje žádný pool ZFS, takže bez něj BombVault obnovu odmítne dřív, než začne. Předej pool jako parametr `zvolPool` v `POST /api/foreign/restore`: každý zvol si ponechá svou cestu pod poolem (`tank/vms/win10/disk0` na `flashpool` se stane `flashpool/vms/win10/disk0`) a XML domény na něj ukazuje. Okno **Ze staré složky se zálohami** na stránce Obnova pro to nemá pole, takže je dosažitelné jen přímým voláním API. Obnova na stejné instanci zapíše každý zvol zpět na sebe.

**Export jako soubory zvoly vynechává.** VM, jejíž disky jsou všechny zvoly, se s tímto důvodem odmítne, takže ji obnov ze zálohy. U VM se souborovými disky navíc export drží souborové disky a poznámku, která jmenuje zvoly, které vynechal.

Vyzkoušeno na TrueNAS SCALE 25.10 se ZFS 2.3.4: tři zálohy zkušebního zvolu, obnova na místě poté, co byl zvol přepsán a zvětšen, a obnova na zvol, který byl smazán. Každá obnova se přečetla zpět se stejným SHA-256 jako zdroj, zvol měl zase svou starou velikost a nezůstal žádný snapshot ani klon. Celá VM, jejímž jediným diskem je zvol, byla zálohována běžící, s Graceful i Živým snímkem, a vypnutá, a běžela tam i obnova celé VM (zrušení definice, zápis zvolů, definice, boot): zvol se přečetl zpět se kontrolním součtem zdroje a VM nastartovala.

Souborové disky, jaké používá Unraid, se z toho nedotýká nic.

### 4. Stav vTPM {#truenas-vtpm}

VM může mít virtuální TPM, které potřebují hosté se Secure Boot a Windows 11. Jeho stav žije mimo disk a NVRAM, takže záloha jen těchto dvou by vrátila hosta s novým, prázdným TPM.

- **Průchozí TPM**, skutečný čip předaný hostu, jmenuje svou cestu v XML domény. BombVault jeho stav uloží vedle NVRAM přes SSH a zapíše ho zpět před `virsh define`, na základě nejlepší snahy jako NVRAM: selhání čtení nebo zápisu se zaloguje a nezpůsobí selhání zálohy ani obnovy.
- **Emulované vTPM**, jaké TrueNAS nastavuje pro hosty s Windows 11 a Secure Boot, nezveřejňuje v XML domény žádnou cestu ke stavu, takže se jeho stav nezálohuje. BombVault cestu nehádá a záloha skončí s poznámkou, že stav TPM této VM v ní není. Obnovovací klíč takového hosta, například jeho klíč BitLockeru, měj po ruce, než ho obnovíš.
- **TPM k VM v TrueNAS sám připojit nemůžeš.** API zařízení VM v TrueNAS SCALE 25.10 přijímá jen `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` a `USB` a `TPM` odmítá jako neznámý typ.

Zacházení s TPM je testované jen jednotkovými testy a neběželo proti skutečnému hostu TrueNAS s vTPM. VM bez TPM se zálohuje a obnovuje přesně jako dřív.

NVRAM se zachytí jako obvykle. TrueNAS ji drží pro každou VM v `/var/db/system/vm/nvram/<id>_<name>_VARS.fd` a XML domény tuto cestu jmenuje.

### 5. Jména VM v TrueNAS {#truenas-names}

Na Unraidu je jméno VM v libvirt takové, jaké sis zvolil. TrueNAS jméno libvirt vymýšlí a dvakrát změnil, jak:

| Verze TrueNAS | Jméno v libvirt | Kde žije tvoje jméno |
|---|---|---|
| 25.10 „Goldeye“ | `{id}_{name}`, například `1_debian` | v jménu libvirt, za `{id}_` |
| 26 | UUID VM, například `550e8400-e29b-41d4-a716-446655440000` | jen v `<title>` XML domény |

Na hostiteli, kterého BombVault pozná jako TrueNAS nebo který jmenuje `PLATFORM=truenas`, ukazuje seznam Záloh tvé jméno: `debian` místo `1_debian` na 25.10 a `<title>` místo UUID na 26, nebo UUID, když titul není. To je jen to, co seznam ukazuje. Každé volání `virsh` a nastavení pro jednotlivé VM stále používají jméno libvirt, takže záloha a obnova vždy působí na správnou VM. Na TrueNAS 26 to znamená, že VM, kterou TrueNAS vytvoří znovu s novým UUID, se ukáže jako nová položka bez svého nastavení, i když její jméno vypadá stejně. Na Unraidu se nic nemění, ani u VM, kterou jsi pojmenoval `10_Windows`.

Schéma 25.10 bylo vidět na skutečné krabici s TrueNAS SCALE 25.10. Schéma TrueNAS 26 vychází z dokumentace TrueNAS a je testované jen jednotkovými testy.

## Řešení potíží {#troubleshooting}

| Příznak | Příčina a oprava |
|---|---|
| **Otestovat připojení** visí a pak selže | Na hostitele se nedá dostat. Zkontroluj krok 3 (síť, VLAN, firewall) a **Host SSH: Address** a **Port**. |
| `Permission denied (publickey)`, i když je klíč v `authorized_keys` | `StrictModes` v sshd odmítá soubor s klíčem, protože nadřazená složka má špatného vlastníka nebo režim. Je to časté, když je `/root/.ssh` odkaz na flashku FAT (`/boot/config/ssh/...`), která je zapisovatelná pro všechny. Log hostitele ukazuje `Authentication refused: bad ownership or modes for directory ...`. Přidej `StrictModes no` do konfigurace sshd hostitele v globální části, před jakýkoli blok `Match`, a spusť `/etc/rc.d/rc.sshd restart`. Unraid zapisuje `/etc/ssh/sshd_config` při každém startu znovu, takže to zachovej, jak je popsáno níže. |
| `Permission denied (publickey)` a klíč opravdu chybí | Zopakuj krok 2 a zkontroluj, že SSH je zapnuté. |
| `Host key verification failed` | Spusť `docker exec BombVault rm -f /config/ssh/known_hosts` a otestuj znovu. |
| Kontrola v kroku 3 říká `UNREACHABLE` | Kontejner nedosáhne na port SSH hostitele. Viz krok 3: přístup hostitele k vlastním sítím, směrování VLAN, nebo IP shim či LAN. |
| Proměnné chybí pod **Edit** | Naimportuj šablonu znovu. Existující kontejner drží svou starou uloženou konfiguraci. |
| VM se snapshoty Unraidu selže a jmenuje soubor | Řetěz podkladových souborů disku nejde sledovat až do konce. Zkontroluj ve VM Manageru, že každý snapshot toho disku tam pořád je a že jmenovaný soubor existuje pod `/mnt`. |

Další příznaky VM jsou v [Řešení problémů](troubleshooting.md#vm-backup-will-not-connect).

## Zachování přes restarty {#persistence}

Unraid zapisuje `/etc/ssh/sshd_config` při každém startu znovu, takže změny na straně hostitele je potřeba při startu udělat znovu:

- **Port SSH** je uložený v `/boot/config/ident.cfg`, nastavuje se přes webové rozhraní Unraidu.
- **`StrictModes no`**, pokud jsi ho výše potřeboval, se neuchovává. Použij ho při startu z `/boot/config/go`:

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

- Vlastní klíč SSH BombVaultu a `~/.ssh/config` žijí v `/config` (appdata) a vytvářejí se při každém startu znovu, takže přežijí restarty kontejneru i hostitele.

## Zabezpečení {#security}

Autorizovaný klíč dává root na hostiteli, stejnou úroveň důvěry jako Docker socket, který BombVault už používá. Drž BombVault v důvěryhodné síti. Klíč je ed25519, uložený s režimem `0600` v appdata a klíč hostitele je připnutý. Každý příkaz hostitele se předává jako samostatné argumenty, nikdy přes vzdálený shell, a spojení SSH používá `BatchMode` a časový limit připojení.
