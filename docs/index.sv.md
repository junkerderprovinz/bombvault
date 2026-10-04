# BombVault

**Dina Unraid-data, förseglade i ett valv. Släpp en säkerhetskopia. Detonera en återställning.**

BombVault är en självhostad, Unraid-nativ webbapp för **säkerhetskopiering och fullständig katastrofåterställning** av dina Docker-containrar och KVM/libvirt-VM:ar. Den körs som en enda multi-arch Docker-container, ger dig ett modernt webbgränssnitt som följer systemets ljusa/mörka temainställning, och sköter hela livscykeln: säkerhetskopiera, schemalägg, verifiera och återställ.

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
| **ZFS-datauppsättningar** | En datauppsättning med alla uppsättningar under den, läst från en ZFS-ögonblicksbild och sparad som en mapp. Se [ZFS-datauppsättningar](zfs-datasets.md). |

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

BombVault använder Docker-socketen för att stoppa containrar före en säkerhetskopiering och återskapa dem efter en återställning. För VM:ar kör den `virsh` på värden över SSH (`qemu+ssh://`) för att stänga av en VM på ett kontrollerat sätt eller ta en live-ögonblicksbild. Den bind-monterar aldrig en libvirt-sökväg, så den kan inte störa VM Manager på värden.

BombVault är orkestrerings- och gränssnittslagret, inte lagringsmotorn. All faktisk dataförflyttning går genom restic.

## Snabbstart

Ny här? Gå till **[Kom igång](getting-started.md)** för att installera BombVault på Unraid via Community Applications och köra din första säkerhetskopiering. Utforska sedan alla **[Funktioner](features.md)**, finjustera din **[Konfiguration](configuration.md)** och sätt upp **[Off-site och återställning](offsite-recovery.md)**.

Off-site kan fördela till flera mål per domän samtidigt, en skrivskyddad **mottagarpanel** övervakar de kopiorna på boxen som tar emot dem, och du kan flytta hela din konfiguration till en ny box med kortet **Exportera / importera inställningar**. Se [Off-site och återställning](offsite-recovery.md) och [Konfiguration](configuration.md#portable-settings-export-and-import).

**[Android-appen](android.md)** ger dig alla servrar i din grupp i telefonen, med aktivitetsloggen för dem alla på en skärm.

## Tack till {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** av [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) gav BombVault dess utgångsidé: säkerhetskopiering med ett klick och automatisk ominstallation av Docker-containrar. BombVault är en separat implementation i Go och restic som för idén vidare till VM:ar, flashen och mer.
- **[restic](https://restic.net/)** är den snabba, säkra, deduplicerande säkerhetskopieringsmotorn som BombVault styr.
- **[rclone](https://rclone.org/)** står för molnbackendarna.
- De flesta symboler på knappar kommer från den kostnadsfria Core Solid-uppsättningen från **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [källa](https://github.com/webalys-hq/streamline-vectors)). Resten kommer från Font Awesome Free, Material Design Icons, Simple Icons och Tabler Icons eller har ritats för projektet.

## Licens {#license}

Copyright (C) 2026 Junker der Provinz. BombVault är fri programvara under **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Du får köra, studera, dela och ändra det. Om du distribuerar det, eller kör en ändrad version som en nätverkstjänst, måste du publicera din källkod under samma licens och behålla befintliga upphovsrätts- och attributionsmeddelanden.

Namnet och varumärket omfattas inte av licensen. AGPL täcker endast källkoden: "BombVault", dess logotyp och dess varumärke förblir förbehållna, så en fork måste använda ett eget namn och ett eget varumärke och får inte utge sig för att vara BombVault.

## Länkar

- **Källkod:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supporttråd:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Ärenden:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-likvärdig kontroll över värden"
    Via Docker-socketen kan BombVault stoppa, ta bort och återskapa containrar och läsa/skriva appdata, och för VM-säkerhetskopiering loggar den in på värden över SSH för att köra `virsh`. Vem som helst som kan nå dess webbgränssnitt har i praktiken root på värden. Kör BombVault endast på ett betrott, icke-exponerat nätverk, och aktivera den valfria lösenordsspärren (Inställningar, Säkerhet) när off-site- eller oföränderliga säkerhetskopior används. Se [Konfiguration](configuration.md) för hela säkerhetsmodellen.
