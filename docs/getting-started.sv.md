# Kom igång

Den här sidan tar dig från en ny Unraid-box till din första säkerhetskopiering.

## Krav

| Krav | Anmärkningar |
|---|---|
| **Unraid 6.12+** | Tidigare versioner är inte testade. Unraid är huvudmålet, men BombVault kör också på en vanlig Docker-värd och på TrueNAS Scale (se [Generisk Docker-värd](#generic-docker-host)). |
| **Restic-repo-plats** | En lokal sökväg (rekommenderas: din array eller cache), SMB, NFS eller valfri rclone-backend. |
| **Docker-socket** | Monteras automatiskt av mallen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteras hel automatiskt av mallen (`/boot` till `/host/boot`). Driver flash-säkerhetskopiering och låter en återställd container dyka upp igen som en normal, redigerbar Unraid-app. |
| **KVM-VM:ar** (tillval) | VM-säkerhetskopiering pratar med libvirt över SSH, ingen libvirt-montering. Sätt upp det under **Inställningar, Integrationer, Värd-SSH** (se [Konfiguration](configuration.md)). |
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

- **Inget Flash-avsnitt.** Det finns inget start-USB att fånga eller återställa, så Flash-avsnittet har inget att göra här. I stället erbjuder **Lägg till** på sidan Säkerhetskopior **Systemkonfiguration** som mappförslag: en uppsättning `/etc`-mappar att börja med, som du granskar innan du sparar.
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
    Att förlora `APP_KEY` gör dina krypterade säkerhetskopior oåterställbara. Förvara den på en säker plats åtskild från servern. När BombVault väl körs, sätt ett inloggningslösenord och ladda ner **återställningskitet** under **Inställningar, Återställning** (se [Off-site och återställning](offsite-recovery.md#recovery-kit)). Det innehåller nyckeln, varje mål och restic-kommandona.

Mallen monterar också Docker-socketen, flashen (`/boot`) och **Host Data**-roten (`/mnt`) åt dig. Både säkerhetskopieringens *källor* och *mål* ligger under Host Data. För den fullständiga variabelreferensen och off-site-uppsättningen, se [Konfiguration](configuration.md).

## Första körningen

![Översikten efter en första säkerhetskopia: vad som behöver dig, skyddet för varje avsnitt, målen och historiken.](assets/screenshots/dashboard.png)

*Översikten efter en första säkerhetskopia: vad som behöver dig, skyddet för varje avsnitt, målen och historiken.*

1. Öppna webbgränssnittet på `https://<din-unraid-ip>:3443` (självsignerat certifikat direkt ur lådan). En ny installation öppnas på **Välkommen till BombVault**, som listar vad BombVault hittade på servern.
2. Välj **Kom igång**. Översikten visar sedan kortet **Första stegen** med tre steg:
    1. **Välj avsnitt**: slå på det BombVault säkerhetskopierar (Containers, VMs, Flash, Mappar, ZFS-datauppsättningar, Auto-säkerhetskopia). **Välj** öppnar listan över ett avsnitts poster, där **Pausa schemat** lämnar en utanför. Avsnitten du slår på här körs varje natt, vilket du ändrar under **Inställningar, Schema**.
    2. **Lägg till mål**: dit säkerhetskopiorna går. En lokal mapp räcker för att börja. Tryggast är ett andra mål utanför huset, till exempel en Storage Box eller Backblaze B2. Lokala repositorier har som standard `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` och skapas vid den första säkerhetskopian.
    3. **Första säkerhetskopian**: därefter körs den av sig själv varje natt.
3. Öppna **Säkerhetskopior** för att se alla poster i en enda lista. **Säkerhetskopiera allt nu** startar en körning över alla påslagna avsnitt, och **Säkerhetskopiera nu** på en posts sida säkerhetskopierar bara den.

Har du använt BombVault förut? **Återställ från en säkerhetskopia** på välkomstsidan öppnar sidan Återställning, där **Nödguide** tar tillbaka inställningarna, containrarna, VM:arna och mapparna på en förlorad server. Se [Off-site och återställning](offsite-recovery.md#emergency).

!!! tip "Valfritt: välj en säkerhetskopieringsordning"
    Om vissa containrar alltid ska säkerhetskopieras före andra (till exempel en databas före appen som använder den), öppna kortet **Ordning** under **Inställningar, Schema** och dra dem i den sekvens du vill ha. Schemalagda körningar och hela säkerhetskopian följer den sedan; allt du lämnar oordnat säkerhetskopieras mest-försenat-först.

!!! note "Värdintegrationskontroll"
    Kortet **Värdintegration** på översikten sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar, så att du kan bekräfta att containern är korrekt inkopplad innan du förlitar dig på den. Det väntar under **Anpassa** tills du lägger till det.

## Hitta rätt {#navigation}

![Inställningarna som brickor, med kortet för den valda brickan bredvid. Varje ändring skrivs medan du gör den.](assets/screenshots/settings.png)

*Inställningarna som brickor, med kortet för den valda brickan bredvid. Varje ändring skrivs medan du gör den.*

Sidofältet har fem poster: **Översikt**, **Avvikelser** (så länge avvikelsedetektering är på), **Säkerhetskopior**, **Återställning** och, under **Logga ut**, **Inställningar**. En sidas huvudåtgärd flyter nere till höger, och **Mer** rymmer de ovanliga. På en telefon har fältet Översikt, Säkerhetskopior, Återställning och Inställningar, och **Mer** har resten.

- **Säkerhetskopior** listar varje container, VM, mappuppsättning, flashenheten, varje ZFS-datauppsättning och auto-säkerhetskopian som poster. Brickor ovanför listan väljer slag, och **Filter**, **Sortera** och **Sök** snävar in den. Varje post har en egen sida med tre flikar: **Säkerhetskopior**, **Inställningar** och **Integritet**.
- **Återställning** är en enda sida för allt: välj ett avsnitt, sök efter en post och välj en säkerhetskopia. Nödguiden och de andra källorna startar också här.
- **Inställningarna** är fjorton brickor: Allmänt, Utseende, Schema, Mål, Återställning, Aviseringar, Verifiering, Säkerhet, Instanser, Parkoppling, Integrationer, Appar, System och Info. Tryck `/` eller Ctrl+K för att söka i alla, även efter de namn sidorna hade i tidigare versioner. Dra brickorna till din egen ordning under **Utseende**.
- **Mål** och **Instanser** är inställningsbrickor med egna sidor. **Visa även i sidofältet** på deras bricka fäster dem i sidofältet.

Inget har en Spara-knapp och inget fälls ut: ovanliga saker har ett eget kort eller ett fönster, och förklaringarna finns i (i)-bubblorna. Efter en uppdatering markerar gröna prickar vad som ändrats. Ett klick döljer en prick, och **Inställningar, Info, Nytt** listar varje ändring med **Gå dit**.

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

Gränssnittsbygget behövs även för `go run`. Arkivet innehåller bara en tom markör under `web/dist`, så utan `npm --prefix web run build` bäddar binären inte in något och svarar `500 SPA index not found`, vilket är väntat. Docker, libvirt och Unraid går inte att testa i CI, så kontrollera monteringar, restic och SSH-anslutningen för VM:ar på en riktig värd med kortet **Värdintegration** på översikten (eller `GET /api/spike`) innan du öppnar en pull request.

## Nästa steg

- Bläddra bland alla **[Funktioner](features.md)**.
- Ha alla servrar i din grupp i telefonen med **[Android-appen](android.md)**.
- Lägg till ett eller flera mål utanför huset (varje avsnitt kan kopiera till flera samtidigt) och spara ditt återställningskit, som beskrivs under **[Off-site och återställning](offsite-recovery.md)**.
- Klonar du en uppsättning eller flyttar till en ny box? Ta med hela din konfiguration med kortet **Exportera / importera inställningar**. Se [Konfiguration](configuration.md#portable-settings-export-and-import).
- Stötte på ett problem? Se **[Felsökning](troubleshooting.md)**.
