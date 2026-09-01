# Kom igång

Den här sidan tar dig från en ny Unraid-box till din första säkerhetskopiering.

## Krav

| Krav | Anmärkningar |
|---|---|
| **Unraid 6.12+** | Tidigare versioner är inte testade. |
| **Restic-repo-plats** | En lokal sökväg (rekommenderas: din array eller cache), SMB, NFS eller valfri rclone-backend. |
| **Docker-socket** | Monteras automatiskt av mallen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteras hel automatiskt av mallen (`/boot` till `/host/boot`). Driver flash-säkerhetskopiering och låter en återställd container dyka upp igen som en normal, redigerbar Unraid-app. |
| **KVM-VM:ar** (tillval) | VM-säkerhetskopiering pratar med libvirt över SSH, ingen libvirt-montering. Sätt upp det i Inställningar (se [Konfiguration](configuration.md)). |

## Installera på Unraid

Den enklaste vägen är **Community Applications**.

1. Öppna fliken **Apps** i Unraid.
2. Sök efter **BombVault**.
3. Klicka på **Install**, ställ in de nödvändiga variablerna (nedan) och tillämpa.

!!! tip "Manuell mallinstallation"
    Om du föredrar att lägga till mallen för hand:

    1. Gå till **Docker, Add Container, Template repositories** och lägg till:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Sök efter **BombVault** i Templates.
    3. Ställ in de nödvändiga variablerna och klicka på **Apply**.

## Generisk Docker-värd

Inte Unraid? BombVault kör också som en vanlig container på vilken Docker-värd som helst (det är också det som bär containerstödet på TrueNAS Scale, i väntan på en egen post i dess appkatalog).

1. Hämta den färdiga att redigera [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) från arkivet.
2. Sätt `APP_KEY` (se nedan) och rikta Host Data-volymen mot din verkliga datarot: kommentarerna i filen går igenom bådadera.
3. `docker compose up -d`, öppna sedan `https://<värd-ip>:3443/`.

Vad som skiljer mot Unraid:

- **Ingen flash-/USB-domän.** Det finns inget start-USB att fånga eller återställa, så Flash-domänen i inställningarna har inget att göra här. I stället erbjuder Fil-domänen ettklicksförslaget **Lägg till förinställning: värdsystemets konfiguration** (en uppsättning `/etc`-filer att börja med, som du granskar och redigerar innan du sparar) som praktisk generisk motsvarighet.
- **Inga Unraid-egna aviseringar.** BombVaults egna aviseringskanaler (webhook, varningar vid misslyckad off-site och liknande) fungerar som vanligt; bara den Unraid-specifika sändningen till dess eget aviseringssystem uteblir, eftersom något sådant system inte finns här.
- **Säkerhetskopiering av virtuella maskiner är valfri och kräver en separat libvirtd-värd nåbar över SSH.** Se det bortkommenterade blocket i compose-filen. En generisk Docker-värd har ingen inbyggd VM-hantering.

## Den enda obligatoriska inställningen

Den enda variabeln du måste sätta är `APP_KEY`, en 32-byte hex-hemlighet (64 hex-tecken) som används för att härleda restic-repositoriets lösenord.

Generera en på valfri maskin:

```bash
openssl rand -hex 32
```

Klistra in resultatet i `APP_KEY`-fältet i mallen.

!!! danger "Förlora inte din APP_KEY"
    Att förlora `APP_KEY` gör dina krypterade säkerhetskopior oåterställbara. Förvara den på en säker plats åtskild från servern. När BombVault väl körs, använd dess **återställningskit för krypteringsnyckeln** med ett klick (se [Off-site och återställning](offsite-recovery.md)) för att spara hela återställningspaketet.

Mallen monterar också Docker-socketen, flashen (`/boot`) och **Host Data**-roten (`/mnt`) åt dig. Både säkerhetskopieringens *källor* och *mål* ligger under Host Data. För den fullständiga variabelreferensen och off-site-uppsättningen, se [Konfiguration](configuration.md).

## Första körningen

1. Öppna webbgränssnittet på `https://<din-unraid-ip>:3443` (självsignerat certifikat direkt ur lådan).
2. I **Inställningar**, aktivera de säkerhetskopieringsdomäner du vill ha (Containers, VMs, Flash, Config, Files) och välj en accentfärg.
3. På fliken **Containers**, välj en container och klicka på **Säkerhetskopiera** för att skapa din första återställningspunkt. Repository-sökvägar har standardvärdet `/mnt/user/bombvault/{container,vms,flash,config,files}` och skapas vid den första säkerhetskopieringen.
4. Sätt upp schemaläggning från **Inställningar, Scheman**. Det finns en *inkludera alla i schema* med ett klick för containrar och VM:ar.

!!! tip "Valfritt: välj en säkerhetskopieringsordning"
    Om vissa containrar alltid ska säkerhetskopieras före andra (till exempel en databas före appen som använder den), öppna panelen **säkerhetskopieringsordning** på Containers-sidan och dra dem i den sekvens du vill ha. Schemalagda och multi-select-körningar följer den sedan; allt du lämnar oordnat säkerhetskopieras mest-försenat-först, som tidigare.

!!! note "Värdintegrationskontroll"
    Öppna `/spike` i webbgränssnittet efter att containern startat. Den sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar, så att du kan bekräfta att containern är korrekt inkopplad innan du förlitar dig på den.

## Enkel kontra Avancerad

Som standard visar gränssnittet endast det väsentliga (säkerhetskopiera, återställa, schemalägga). Använd omkopplaren **Enkel / Avancerad** i sidofältet för att avslöja expertkontrollerna: retention, off-site-kopia, pre/post-hooks, återställning på filnivå, aviseringar, Prometheus-mätvärden och integritets-/underhållsverktygen. Det är en inställning per webbläsare och avstängd som standard, så nykomlingar får ett rent gränssnitt och avancerade användare får allt.

## Nästa steg

- Bläddra bland alla **[Funktioner](features.md)**.
- Lägg till en eller flera **[Off-site och återställning](offsite-recovery.md)**-repliker (varje domän kan skicka till flera mål samtidigt) och spara ditt återställningskit.
- Klonar du en uppsättning eller flyttar till en ny box? Ta med hela din konfiguration med kortet **Exportera och importera inställningar**. Se [Konfiguration](configuration.md#portable-settings-export-and-import).
- Stötte på ett problem? Se **[Felsökning](troubleshooting.md)**.
