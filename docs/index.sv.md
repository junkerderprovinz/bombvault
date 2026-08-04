# BombVault

**Dina Unraid-data, förseglade i ett valv. Släpp en säkerhetskopia. Detonera en återställning.**

BombVault är en självhostad, Unraid-nativ webbapp för **säkerhetskopiering och fullständig katastrofåterställning** av dina Docker-containrar och KVM/libvirt-VM:ar. Den körs som en enda multi-arch Docker-container, ger dig ett modernt mörkt webbgränssnitt och sköter hela livscykeln: säkerhetskopiera, schemalägg, verifiera och återställ.

Återställningar är automatiska. Containrar dyker upp igen i Unraids Docker-flik precis som förut, och VM:ar återdefinieras i VM Manager med sina diskar och UEFI NVRAM återkopplade. Ingen manuell ominstallation, ingen omkonfiguration, inget krångel.

Drivs av [restic](https://restic.net), så varje säkerhetskopia är deduplicerad, inkrementell och alltid krypterad.

!!! note "Skydda din APP_KEY"
    BombVault härleder restic-repositoriets lösenord från en 32-byte hemlighet vid namn `APP_KEY`. Att förlora den gör krypterade säkerhetskopior oåterställbara. Generera en med `openssl rand -hex 32` och förvara den på en säker plats. Se [Konfiguration](configuration.md).

## Vad BombVault skyddar

| Domän | Vad som sparas |
|---|---|
| **Docker-containrar** | Appdata-katalogen plus containerdefinitionen (image, miljövariabler, portar, etiketter, volymer). |
| **KVM / libvirt-VM:ar** | VM-diskavbild(er), XML-definitionen och UEFI NVRAM, säkerhetskopierade över SSH (ingen libvirt-montering). |
| **Unraid-flash** | Hela USB-flashen (`/boot`): OS, licens, array-config, resurser, nätverks- och plugin-config. |
| **App-konfiguration** | BombVaults egen `/config`: dess inställningsdatabas, off-site-uppgifter och libvirt-SSH-nyckelparet. |
| **Filer och mappar** | Namngivna **filuppsättningar**, valfri mapp på servern, var och en med valfria exkluderingsmönster per uppsättning. |

## Återställning är stjärnan

Efter att data har kopierats tillbaka från restic-ögonblicksbilden spelar BombVault upp den sparade containerdefinitionen mot Docker-API:et, så containern dyker upp igen i Unraids Docker-flik som om den alltid hade varit där (samma image, samma inställningar, samma portmappningar). VM:ar får sin XML återdefinierad över SSH och sina diskar och UEFI NVRAM återkopplade, även efter att VM:en har raderats.

När en säkerhetskopiering stoppar beroende containrar kommer de tillbaka i rätt ordning: BombVault startar om dem i deras Compose-`depends_on`-ordning och väntar på att var och en rapporterar frisk innan de som är beroende av den startas, så att inget rusar iväg före en databas eller en gateway som ännu inte är uppe. Se [Funktioner](features.md).

## Så fungerar det

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault är orkestrerings- och gränssnittslagret, inte lagringsmotorn. All faktisk dataförflyttning går genom restic.

## Snabbstart

Ny här? Gå till **[Kom igång](getting-started.md)** för att installera BombVault på Unraid via Community Applications och köra din första säkerhetskopiering. Utforska sedan alla **[Funktioner](features.md)**, finjustera din **[Konfiguration](configuration.md)** och sätt upp **[Off-site och återställning](offsite-recovery.md)**.

Off-site kan fördela till flera mål per domän samtidigt, en skrivskyddad **mottagarpanel** övervakar de kopiorna på boxen som tar emot dem, och du kan flytta hela din konfiguration till en ny box med kortet **Exportera och importera inställningar**. Se [Off-site och återställning](offsite-recovery.md) och [Konfiguration](configuration.md#portable-settings-export-and-import).

## Länkar

- **Källkod:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supporttråd:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Ärenden:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-likvärdig kontroll över värden"
    Via Docker-socketen kan BombVault stoppa, ta bort och återskapa containrar och läsa/skriva appdata, och för VM-säkerhetskopiering loggar den in på värden över SSH för att köra `virsh`. Vem som helst som kan nå dess webbgränssnitt har i praktiken root på värden. Kör BombVault endast på ett betrott, icke-exponerat nätverk, och aktivera den valfria lösenordsspärren (Inställningar, Säkerhet) när off-site- eller oföränderliga säkerhetskopior används. Se [Konfiguration](configuration.md) för hela säkerhetsmodellen.
