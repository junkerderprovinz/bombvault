# Kom igång

Den här sidan tar dig från en ny Unraid-box till din första säkerhetskopiering.

## Krav

| Krav | Anmärkningar |
|---|---|
| **Unraid 6.12+** | Tidigare versioner är inte testade. Unraid är huvudmålet, men BombVault kör också på en vanlig Docker-värd och på TrueNAS Scale (se [Generisk Docker-värd](#generic-docker-host)). |
| **Restic-repo-plats** | En lokal sökväg (rekommenderas: din array eller cache), SMB, NFS eller valfri rclone-backend. |
| **Docker-socket** | Monteras automatiskt av mallen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteras hel automatiskt av mallen (`/boot` till `/host/boot`). Driver flash-säkerhetskopiering och låter en återställd container dyka upp igen som en normal, redigerbar Unraid-app. |
| **KVM-VM:ar** (tillval) | VM-säkerhetskopiering pratar med libvirt över SSH, ingen libvirt-montering. Sätt upp det i Inställningar (se [Konfiguration](configuration.md)). |
| **ZFS-datauppsättningar** (tillval) | Samma SSH-anslutning som för VM-säkerhetskopiering, `zfs` på värden och Host Data mappad som `/mnt` med åtkomstläget Read/Write - Slave, mallens standard. Se [ZFS-datauppsättningar](zfs-datasets.md). |
| **Android-app** (valfritt) | Android 10 eller senare, parkopplad med servrar på version 9.7.0 eller senare. Se [Android-app](android.md). |

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

## Generisk Docker-värd {#generic-docker-host}

Inte Unraid? BombVault kör också som en vanlig container på vilken Docker-värd som helst (det är också det som bär containerstödet på TrueNAS Scale, i väntan på en egen post i dess appkatalog).

1. Hämta den färdiga att redigera [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) från arkivet.
2. Sätt `APP_KEY` (se nedan) och rikta Host Data-volymen mot din verkliga datarot: kommentarerna i filen går igenom bådadera.
3. `docker compose up -d`, öppna sedan `https://<värd-ip>:3443/`.

Vad som skiljer mot Unraid:

- **Ingen flash-/USB-domän.** Det finns inget start-USB att fånga eller återställa, så Flash-domänen i inställningarna har inget att göra här. I stället erbjuder domänen Mappar ettklicksförslaget **Lägg till förinställning: värdsystemets konfiguration** (en uppsättning `/etc`-filer att börja med, som du granskar och redigerar innan du sparar) som praktisk generisk motsvarighet.
- **Inga Unraid-egna aviseringar.** BombVaults egna aviseringskanaler (webhook, varningar vid misslyckad off-site och liknande) fungerar som vanligt; bara den Unraid-specifika sändningen till dess eget aviseringssystem uteblir, eftersom något sådant system inte finns här.
- **Säkerhetskopiering av virtuella maskiner är valfri och kräver en separat libvirtd-värd nåbar över SSH.** Se det bortkommenterade blocket i compose-filen. En generisk Docker-värd har ingen inbyggd VM-hantering.
- **Ingen widget på instrumentpanelen.** BombVault Widget är ett Unraid-plugin, så det steget hoppas också över.
- **Hitta en containers data.** Utan Unraids `appdata`-konvention hittas en containers datamapp utifrån segmenten i `DATA_ROOT_SEGMENTS`, namngivna Docker-volymer, ett Compose-projekts arbetskatalog och etiketten `bombvault.data` (se [Igenkänning av säkerhetskopieringens källor](configuration.md#backup-source-detection)). Namngivna volymer och `/etc`-förinställningen når bara sökvägar inuti Host Data-monteringen, så rikta Host Data mot en gemensam överordnad mapp som också täcker Dockers datarot.
- **`PLATFORM`.** Sätt den till `generic` eller `truenas`. Utan värde känner BombVault igen Unraid på dess egen markör på flash-monteringen och behandlar allt annat som generiskt, och stegen som bara gäller Unraid hoppas över i stället för att provas och misslyckas.

**TrueNAS Scale** tar samma compose-väg; en katalogpost är förberedd i arkivet men ännu inte inskickad. VM-säkerhetskopiering kräver där `LIBVIRT_URI`, eftersom TrueNAS libvirtd lyssnar på en egen socket (`/run/truenas_libvirt/libvirt-sock`) som de tre `LIBVIRT_*`-variablerna inte kan uttrycka (se [Konfiguration](configuration.md)). Så långt är det beprövat: zvol-säkerhetskopieringen kördes mot en riktig TrueNAS Scale-box, på en zvol kopplad till en körande VM, och `zfs snapshot`, `zfs send`, restic och `zfs receive` tog den fram och tillbaka byte för byte. En fullständig återställning som styrs av BombVault självt har ännu inte körts på TrueNAS-hårdvara, och den zvolen var tunt allokerad, så genomströmningen vid många gigabyte är inte testad. Testa en återställning där innan du förlitar dig på den.

## Den enda obligatoriska inställningen

Den enda variabeln du måste sätta är `APP_KEY`, en 32-byte hex-hemlighet (64 hex-tecken) som används för att härleda restic-repositoriets lösenord.

Generera en på valfri maskin:

```bash
openssl rand -hex 32
```

Klistra in resultatet i `APP_KEY`-fältet i mallen (Unraid) eller i miljövariabeln `APP_KEY` i `docker-compose.yml` (generisk Docker-värd).

!!! danger "Förlora inte din APP_KEY"
    Att förlora `APP_KEY` gör dina krypterade säkerhetskopior oåterställbara. Förvara den på en säker plats åtskild från servern. När BombVault väl körs, använd dess **återställningskit för krypteringsnyckeln** med ett klick (se [Off-site och återställning](offsite-recovery.md)) för att spara hela återställningspaketet.

Mallen monterar också Docker-socketen, flashen (`/boot`) och **Host Data**-roten (`/mnt`) åt dig. Både säkerhetskopieringens *källor* och *mål* ligger under Host Data. För den fullständiga variabelreferensen och off-site-uppsättningen, se [Konfiguration](configuration.md).

## Första körningen

![Instrumentpanelen efter en första säkerhetskopia: vad som skyddas, vad som kör härnäst och en levande logg.](assets/screenshots/dashboard.png)

*Instrumentpanelen efter en första säkerhetskopia: vad som skyddas, vad som kör härnäst och en levande logg.*

1. Öppna webbgränssnittet på `https://<din-unraid-ip>:3443` (självsignerat certifikat direkt ur lådan).
2. I **Inställningar**, aktivera de säkerhetskopieringsdomäner du vill ha (Containers, VMs, Flash, Auto-säkerhetskopia, Mappar, ZFS-datauppsättningar) och välj en accentfärg.
3. På fliken **Containers**, välj en container och klicka på **Säkerhetskopiera nu** för att skapa din första återställningspunkt. Repository-sökvägar har standardvärdet `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` och skapas vid den första säkerhetskopieringen.
4. Sätt upp schemaläggning från **Inställningar, Scheman**. Det finns en *Inkludera alla i schemat* med ett klick för containrar och VM:ar.

!!! tip "Valfritt: välj en säkerhetskopieringsordning"
    Om vissa containrar alltid ska säkerhetskopieras före andra (till exempel en databas före appen som använder den), öppna panelen **Ordning för säkerhetskopiering** på Containers-sidan och dra dem i den sekvens du vill ha. Schemalagda och multi-select-körningar följer den sedan; allt du lämnar oordnat säkerhetskopieras mest-försenat-först, som tidigare.

!!! note "Värdintegrationskontroll"
    Öppna `/spike` i webbgränssnittet efter att containern startat. Den sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar, så att du kan bekräfta att containern är korrekt inkopplad innan du förlitar dig på den.

## Enkel kontra Avancerad

![Inställningarna har ingen Spara-knapp: varje ändring skrivs medan du gör den.](assets/screenshots/settings.png)

*Inställningarna har ingen Spara-knapp: varje ändring skrivs medan du gör den.*

Som standard visar gränssnittet endast det väsentliga (säkerhetskopiera, återställa, schemalägga). Använd omkopplaren **Enkel vy / Avancerad vy** i sidofältet för att avslöja expertkontrollerna: retention, off-site-kopia, pre/post-hooks, återställning på filnivå, aviseringar, Prometheus-mätvärden och integritets-/underhållsverktygen. Det är en inställning per webbläsare och avstängd som standard, så nykomlingar får ett rent gränssnitt och avancerade användare får allt.

## Bygga från källkod {#build-from-source}

BombVault är en enda statisk Go-binär som serverar ett JSON-API och ett inbäddat React-gränssnitt. Bygg gränssnittet först och kör sedan binären:

```bash
npm --prefix web ci
npm --prefix web run build     # skriver web/dist, som binären bäddar in
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # enhets- och integrationstester, med en riktig restic-runda fram och tillbaka
golangci-lint run ./...
go run ./cmd/bombvault         # serverar https://localhost:3443 med ett självsignerat certifikat
```

Gränssnittsbygget behövs även för `go run`. Arkivet innehåller bara en tom markör under `web/dist`, så utan `npm --prefix web run build` bäddar binären inte in något och svarar `500 SPA index not found`, vilket är väntat. Docker, libvirt och Unraid går inte att testa i CI, så kontrollera monteringar, restic och SSH-anslutningen för VM:ar på en riktig värd med värdintegrationskontrollen (`/spike`) innan du öppnar en pull request.

## Nästa steg

- Bläddra bland alla **[Funktioner](features.md)**.
- Ha alla servrar i din grupp i telefonen med **[Android-appen](android.md)**.
- Lägg till en eller flera **[Off-site och återställning](offsite-recovery.md)**-repliker (varje domän kan skicka till flera mål samtidigt) och spara ditt återställningskit.
- Klonar du en uppsättning eller flyttar till en ny box? Ta med hela din konfiguration med kortet **Exportera / importera inställningar**. Se [Konfiguration](configuration.md#portable-settings-export-and-import).
- Stötte på ett problem? Se **[Felsökning](troubleshooting.md)**.
