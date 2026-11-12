# Kom igång

Den här sidan tar dig från en ny Unraid-box till din första säkerhetskopiering.

## Krav {#requirements}

| Krav | Anmärkningar |
|---|---|
| **Unraid 6.12+** | Tidigare versioner är inte testade. Unraid är huvudmålet, men BombVault kör också på en vanlig Docker-värd och på TrueNAS Scale (se [Generisk Docker-värd](#generic-docker-host) och [TrueNAS SCALE](#truenas)). |
| **Restic-repo-plats** | En lokal sökväg (rekommenderas: din array eller cache), SMB, NFS eller valfri rclone-backend. |
| **Docker-socket** | Monteras automatiskt av mallen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteras hel automatiskt av mallen (`/boot` till `/host/boot`). Driver flash-säkerhetskopiering och låter en återställd container dyka upp igen som en normal, redigerbar Unraid-app. |
| **KVM-VM:ar** (tillval) | VM-säkerhetskopiering pratar med libvirt över SSH, ingen libvirt-montering. Sätt upp det under **Inställningar, Integrationer, Värd-SSH** (se [Konfiguration](configuration.md)). |
| **ZFS-datauppsättningar** (tillval) | Samma SSH-anslutning som för VM-säkerhetskopiering, `zfs` på värden och Host Data mappad som `/mnt` med åtkomstläget Read/Write - Slave, mallens standard. Se [ZFS-datauppsättningar](zfs-datasets.md). |
| **Android-app** (valfritt) | Android 10 eller senare, parkopplad med servrar på version 9.7.0 eller senare. Se [Android-app](android.md). |

## Installera på Unraid {#install-on-unraid}

Den enklaste vägen är **Community Applications**.

1. Öppna fliken **Appar** i Unraid.
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

Inte Unraid? BombVault kör också som en vanlig container på vilken Docker-värd som helst. TrueNAS SCALE har egna steg under [TrueNAS SCALE](#truenas).

1. Hämta den färdiga att redigera [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) från arkivet.
2. Sätt `APP_KEY` (se nedan) och rikta Host Data-volymen mot din verkliga datarot. Kommentarerna i filen går igenom båda.
3. `docker compose up -d`, öppna sedan `https://<värd-ip>:3443/`.

Vad som skiljer mot Unraid:

- **Inget Flash-avsnitt.** Det finns inget start-USB att fånga eller återställa, så Flash-avsnittet har inget att göra här. I stället erbjuder **Lägg till** på sidan Säkerhetskopior, när Host Data når värdens `/etc`, **Systemkonfiguration** som mappförslag: en uppsättning `/etc`-mappar att börja med, som du granskar och redigerar innan du sparar.
- **Inga Unraid-egna aviseringar.** BombVaults egna aviseringskanaler (webhook, varningar vid misslyckad off-site och liknande) fungerar som vanligt. Bara den Unraid-specifika sändningen till dess eget aviseringssystem uteblir, eftersom något sådant system inte finns här.
- **Säkerhetskopiering av virtuella maskiner är valfri och kräver en separat libvirtd-värd nåbar över SSH.** Se det bortkommenterade blocket i compose-filen. En generisk Docker-värd har ingen inbyggd VM-hantering.
- **Ingen bricka på instrumentpanelen.** TollRoll är ett Unraid-plugin, så det steget hoppas också över.
- **Hitta en containers data.** Utan Unraids `appdata`-konvention hittas en containers datamapp utifrån segmenten i `DATA_ROOT_SEGMENTS`, namngivna Docker-volymer, ett Compose-projekts arbetskatalog och etiketten `bombvault.data` (se [Igenkänning av säkerhetskopieringens källor](configuration.md#backup-source-detection)). Namngivna volymer och `/etc`-förinställningen når bara sökvägar inuti Host Data-monteringen, så rikta Host Data mot en gemensam överordnad mapp som också täcker Dockers datarot.
- **`PLATFORM`.** Sätt den till `generic` eller `truenas`. Utan värde känner BombVault igen Unraid på dess egen markör på flash-monteringen och behandlar allt annat som generiskt, och stegen som bara gäller Unraid hoppas över i stället för att provas och misslyckas.

### TrueNAS SCALE {#truenas}

BombVault finns inte i TrueNAS appkatalog än, så installera den som en anpassad app: **Apps, Discover Apps**, sedan **Install via YAML** i menyn bredvid **Custom App**. YAML-koden nedan sätter samma saker som katalogposten i arkivets mapp `truenas-apps`. Byt ut `tank` mot din pool och fyll i `APP_KEY` så som nästa avsnitt beskriver.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Öppna sedan `https://<truenas-ip>:30528/`. Det som står under [Vad som skiljer mot Unraid](#generic-docker-host) gäller även här, och på TrueNAS:

- **Capabilities.** BombVault behöver de sju ovan. VM-säkerhetskopior misslyckas utan `SETPCAP`, och utan `SYS_CHROOT` stoppas varje container under hela sin säkerhetskopiering i stället för att läsas från en ögonblicksbild.
- **Host Data** är en värdsökväg på en datauppsättning, här `/mnt/tank`, monterad på samma sökväg inne i appen. Därför anger `HOST_SOURCE_ROOT` och `HOST_MOUNT_ROOT` båda den sökvägen. `rslave` skickar vidare de monteringar som värden gör senare, och dem behöver säkerhetskopior av ZFS-datauppsättningar.
- **Data i en ixVolume säkerhetskopieras inte.** En app vars lagring är en ixVolume sparar sina data under `/mnt/.ix-apps`, utanför Host Data. BombVault sparar då bara appens definition, och körningen nämner de mappar den lämnade utanför. Välj **Host Path**-lagring på en datauppsättning under Host Data för varje app vars data du vill få säkerhetskopierade.
- **Säkerhetskopior av VM:ar och ZFS-datauppsättningar** når TrueNAS över SSH som root. Slå på SSH-tjänsten under **System, Services** och lägg till den publika nyckeln från **Inställningar, Integrationer, Värd-SSH** hos användaren root under **Credentials, Users**. VM-säkerhetskopior kräver dessutom `LIBVIRT_URI` som ovan, eftersom TrueNAS libvirtd lyssnar på en egen socket. Är varken `LIBVIRT_URI` eller `LIBVIRT_HOST` satt använder BombVault appens standardgateway som värd, eftersom `host.docker.internal` inte går att slå upp i en TrueNAS-app. [Guiden för VM-säkerhetskopiering](vm-backup-ssh-setup.md#truenas-scale) går igenom detaljerna, även VM:ar på zvols.
- **En ZFS-replik använder en annan nyckel.** En replik på en andra ZFS-server loggar in med nyckeln som replikdialogen visar, inte med Värd-SSH-nyckeln. Lägg till den på den mottagande servern.
- **En borttagen app kommer tillbaka i två steg.** Installera om den i TrueNAS under samma namn först, vilket skapar dess nätverk igen, och återställ sedan dess data i BombVault. Fram till dess rapporterar kontrollen före återställningen det saknade nätverket, och återställningen vägrar innan den ändrar något.
- **Bredvid originalet** skapar en vanlig Docker-container. Den kör, men TrueNAS skärm Apps listar den inte.
- **Databasdumpar** fungerar på datauppsättningsmonteringar. En import till en datamapp som är en egen datauppsättning flyttar det gamla innehållet till en mapp bredvid, så datauppsättningen ovanför behöver plats för det.

### ZimaOS, CasaOS och andra värdar {#other-hosts}

- **ZimaOS och CasaOS:** lägg till [community-butiken](https://github.com/junkerderprovinz/zimaos-apps#install) och installera BombVault från den. Den kör där som `PLATFORM=generic` med Host Data på `/DATA`. ZimaOS håller appdata i `/DATA/AppData/<app>`, som BombVault hittar på egen hand, eftersom `DATA_ROOT_SEGMENTS` matchar `appdata` oavsett versaler.
- **MOS:** installera BombVault från [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Den kör där som `PLATFORM=generic` med Host Data på `/mnt/cache`.
- **Synology:** skapa ett projekt från compose-filen i Container Manager (DSM 7.2 eller nyare).
- **Proxmox:** kör den i en VM eller en LXC-container som har Docker.

Utan kommentarer är den generiska compose-filen kort. Byt ut `./data` mot värdmappen som innehåller dina containrars data, sätt `HOST_SOURCE_ROOT` till samma sökväg och fyll i `APP_KEY`:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

För att också säkerhetskopiera namngivna Docker-volymer monterar du en mapp som innehåller både dina data och Dockers datarot (`docker info -f '{{.DockerRootDir}}'`, oftast `/var/lib/docker`), på samma sätt som Unraid-mallen monterar hela `/mnt`.

## Den enda obligatoriska inställningen {#the-one-required-setting}

Den enda variabeln du måste sätta är `APP_KEY`, en 32-byte hex-hemlighet (64 hex-tecken) som används för att härleda restic-repositoriets lösenord.

Generera en på valfri maskin:

```bash
openssl rand -hex 32
```

Klistra in resultatet i `APP_KEY`-fältet i mallen (Unraid) eller i miljövariabeln `APP_KEY` i `docker-compose.yml` (generisk Docker-värd).

!!! danger "Förlora inte din APP_KEY"
    Att förlora `APP_KEY` gör dina krypterade säkerhetskopior oåterställbara. Förvara den på en säker plats åtskild från servern. När BombVault väl körs, sätt ett inloggningslösenord och ladda ner **återställningskitet** under **Inställningar, Återställning** (se [Off-site och återställning](offsite-recovery.md#recovery-kit)). Det innehåller nyckeln, varje lagringsmål och restic-kommandona.

Mallen monterar också Docker-socketen, flashen (`/boot`) och **Host Data**-roten (`/mnt`) åt dig. Både säkerhetskopieringens *källor* och *mål* ligger under Host Data. För den fullständiga variabelreferensen och off-site-uppsättningen, se [Konfiguration](configuration.md).

## Första körningen {#first-run}

![Välkomstsidan på en ny installation, med vad BombVault hittade på servern.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![Välkomstsidan på en ny installation, med vad BombVault hittade på servern.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Öppna webbgränssnittet på `https://<din-unraid-ip>:3443` (självsignerat certifikat direkt ur lådan). En ny installation öppnas på **Välkommen till BombVault**, som listar vad BombVault hittade på servern.
2. Välj **Kom igång**. Översikten visar sedan kortet **Första stegen** med tre steg:
    1. **Välj avsnitt**: slå på det BombVault säkerhetskopierar (Containers, VMs, Flash, Mappar, ZFS-datauppsättningar, Auto-säkerhetskopia). **Välj** öppnar listan över ett avsnitts poster, där **Pausa schemat** lämnar en utanför. Avsnitten du slår på här körs varje natt, vilket du ändrar under **Inställningar, Schema**.
    2. **Lägg till mål**: en andra plats för säkerhetskopiorna. Varje avsnitt har redan en mapp på den här servern, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, som skapas vid den första säkerhetskopian. Tryggast är ett andra lagringsmål utanför huset, till exempel en Storage Box eller Backblaze B2.
    3. **Första säkerhetskopian**: därefter körs den av sig själv varje natt. Kortet ligger kvar tills den första säkerhetskopian är klar.
3. Öppna **Säkerhetskopior** för att se alla poster i en enda lista. **Säkerhetskopiera allt nu** startar en körning över alla påslagna avsnitt, och **Säkerhetskopiera nu** på en posts sida säkerhetskopierar bara den.

![Kortet Första stegen med sina tre steg, det andra öppet.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![Kortet Första stegen med sina tre steg, det andra öppet.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Har du använt BombVault förut? **Återställ från en säkerhetskopia** på välkomstsidan öppnar sidan Återställning, där **Nödguide** tar tillbaka inställningarna, containrarna, VM:arna och mapparna på en förlorad server. Se [Off-site och återställning](offsite-recovery.md#emergency).

!!! tip "Valfritt: välj en säkerhetskopieringsordning"
    Om vissa containrar alltid ska säkerhetskopieras före andra (till exempel en databas före appen som använder den), öppna kortet **Ordning** under **Inställningar, Schema** och dra dem i den sekvens du vill ha. Schemalagda körningar och den totala säkerhetskopian följer den sedan. Allt du lämnar utan ordning säkerhetskopieras med den mest försenade först.

## Exempel: din första containersäkerhetskopia, från början till slut {#example-first-backup}

Det här går igenom en ny Unraid-box med några containrar, bland dem `plex`, tills den första säkerhetskopian är klar och du vet var du hittar den. Det tar ungefär tio minuter plus tiden för den första säkerhetskopian.

1. **Installera.** Sök efter **BombVault** i fliken **Appar** i Unraid och tryck på **Install**.
2. **Sätt nyckeln.** Kör `openssl rand -hex 32` på en dator och klistra in de 64 tecknen i mallens fält `APP_KEY`. Spara en kopia i din lösenordshanterare nu, före allt annat. Tryck på **Apply**.
3. **Öppna den.** Gå till `https://<din-unraid-ip>:3443` och godta det självsignerade certifikatet. **Välkommen till BombVault** listar vad den hittade, till exempel 12 containrar, 2 VM:ar och flashenheten.
4. **Kom igång.** Översikten öppnas med kortet **Första stegen**.
5. **Välj avsnitt.** **Containers** är på från början. Slå på det du vill ha utöver det, till exempel **Flash**. **Välj** öppnar listan över containrar den hittade, där **Pausa schemat** lämnar en utanför, till exempel en testcontainer du inte bryr dig om. Påslagna avsnitt körs varje natt i den totala säkerhetskopian.
6. **Lägg till mål.** Varje avsnitt har redan sin egen mapp på den här servern, under `/mnt/user/bombvault`, så den första säkerhetskopian har någonstans att ta vägen. Det här steget lägger till en andra plats, och den tryggaste ligger utanför huset. Du kan göra det nu med [S3-exemplet](offsite-recovery.md#example-s3), eller komma tillbaka till det när den första säkerhetskopian är klar. Kortet ligger kvar tills dess.
7. **Första säkerhetskopian.** Öppna **Säkerhetskopior** och tryck på **Säkerhetskopiera allt nu**, eller vänta till natten. Ringen i statushuvudet blir en förloppsring och räknar posterna, till exempel **Säkerhetskopierar nu: 3 av 12**. Varje container stoppas medan dess data läses och startas igen direkt efteråt. En container vars data ligger på ZFS är nere bara de få sekunder en ögonblicksbild tar. Den första säkerhetskopian läser allt, så den tar längst tid. Senare lagrar bara det som ändrats.
8. **Kontrollera den.** När ringen säger **Allt är säkerhetskopierat**, öppna **Historik** på översikten: varje container har en grön rad i **Körningar**. Öppna **Säkerhetskopior**, klicka på `plex` och titta på dess flik **Säkerhetskopior**: den första ögonblicksbilden finns där, med **Lokal** som platsen som har den. Några minuter senare visar dess flik **Verifiering** återställningskontrollen, som återställde ett urval av säkerhetskopian och läste tillbaka det.
9. **Skydda nyckeln.** Öppna **Inställningar, Säkerhet** och sätt ett inloggningslösenord. Öppna sedan **Inställningar, Återställning** och tryck på **Ladda ner återställningskit** under **För nödfall**. Förvara filen utanför servern, bredvid kopian av `APP_KEY` från steg 2. Påminnelsen **Spara ditt återställningskit** på översikten försvinner när du trycker på **Jag har förvarat det säkert**.

Från och med nu säkerhetskopieras containrarna varje natt. Bra nästa steg, i den här ordningen: ett [externt lagringsmål](offsite-recovery.md#example-s3), så att en död server inte tar säkerhetskopiorna med sig, sedan [aviseringar](notifications.md), så att ett fel når dig, och därefter en titt på [Scheman](schedules.md) om 03:00 inte passar dig.

![Statushuvudet medan den första säkerhetskopian körs, med förloppsringen och dess stationer.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![Statushuvudet medan den första säkerhetskopian körs, med förloppsringen och dess stationer.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Värdintegrationskontroll"
    Kortet **Värdintegration** på översikten sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar, så att du kan bekräfta att containern är korrekt inkopplad innan du förlitar dig på den. Det väntar under **Anpassa** tills du lägger till det.

## Hitta rätt {#navigation}

![Inställningarna som brickor, med korten för den valda brickan bredvid. Varje ändring skrivs medan du gör den.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Inställningarna som brickor, med korten för den valda brickan bredvid. Varje ändring skrivs medan du gör den.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Sidofältet har fem poster: **Översikt**, **Avvikelser** (så länge avvikelseupptäckten är på), **Säkerhetskopior**, **Återställning** och, under **Logga ut**, **Inställningar**. En sidas huvudåtgärd flyter nere till höger, och **Mer** rymmer de ovanliga. På en telefon har fältet Översikt, Säkerhetskopior, Återställning och Inställningar, och **Mer** har resten.

Sidor och inställningar använder fönstrets hela bredd. Ett fönster har sina knappar i foten, med den som går framåt till höger. På en telefon behåller en fot med fler än tre knappar vägen ut och huvudåtgärden och lägger de andra under **Mer**.

![På en telefon, fältet nere med Mer öppnat.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![På en telefon, fältet nere med Mer öppnat.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Översikt** visar om allt är säkerhetskopierat och vad som behöver dig. Se [Översikt](overview.md).
- **Säkerhetskopior** listar varje container, VM, mappuppsättning, flashenheten, varje ZFS-datauppsättning och auto-säkerhetskopian som poster. Brickor ovanför listan väljer slag, och **Filter**, **Sortera** och **Sök** snävar in den. Varje post har en egen sida med tre flikar: **Säkerhetskopior**, **Inställningar** och **Verifiering**. En tillbaka-knapp till vänster om postens ikon leder till listan. Se [Säkerhetskopior och poster](backups.md).
- **Återställning** är en enda sida för allt: välj ett avsnitt, sök efter en post och välj en säkerhetskopia. Nödguiden och de andra källorna startar också här. Se [Återställning](restore.md).
- **Inställningar** är fjorton brickor: Allmänt, Utseende, Schema, Lagringsmål, Återställning, Aviseringar, Verifiera, Säkerhet, Instanser, Parkoppling, Integrationer, Appar, System och Info. Se [Inställningar](settings.md). Tryck `/` eller Ctrl+K för att söka i alla, även efter de namn sidorna hade i tidigare versioner. Dra brickorna till din egen ordning under **Utseende**. Där en kolumn med brickor bredvid sidan skulle göra sidan för smal, som i ett fönster omkring 1200 pixlar brett, blir brickorna en remsa ovanför sidan som scrollar i sidled. En telefon visar samma remsa.
- **Lagringsmål** och **Instanser** är inställningsbrickor med egna sidor. **Visa även i sidofältet** på deras bricka fäster dem i sidofältet.

Inget har en Spara-knapp och inget fälls ut: ovanliga saker har ett eget kort eller ett fönster, och förklaringarna finns i (i)-bubblorna. Efter en uppdatering öppnas av sig själv ett fönster med ett kort för varje utgåva sedan din senaste version, och gröna prickar markerar vad som ändrats. Ett klick döljer en prick. **Ny** i kortet Version under **Inställningar, Info** räknar vad du inte sett ännu och öppnar det fönstret igen, med anteckningarna för varje utgåva och dess ändringar med **Gå dit**.

## Bygga från källkod {#build-from-source}

BombVault är en enda statisk Go-binär som serverar ett JSON-API och ett inbäddat React-gränssnitt. Bygg gränssnittet först och kör sedan binären:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Gränssnittsbygget behövs även för `go run`. Arkivet innehåller bara en tom markör under `web/dist`, så utan `npm --prefix web run build` bäddar binären inte in något och svarar `500 SPA index not found`, vilket är väntat. Docker, libvirt och Unraid går inte att testa i CI, så kontrollera monteringar, restic och SSH-anslutningen för VM:ar på en riktig värd med kortet **Värdintegration** på översikten (eller `GET /api/spike`) innan du öppnar en pull request.

## Nästa steg {#next-steps}

- Lägg till ett eller flera lagringsmål utanför huset (varje avsnitt kan kopiera till flera samtidigt) och spara ditt återställningskit, som beskrivs under **[Off-site och återställning](offsite-recovery.md)**. [S3-exemplet](offsite-recovery.md#example-s3) går igenom ett.
- Se till att ett fel når dig med **[Aviseringar](notifications.md)**.
- Kontrollera när saker körs under **[Scheman](schedules.md)**.
- Ha alla servrar i din grupp i telefonen med **[Android-appen](android.md)**.
- De **[genomgångna exemplen](guides.md)** går igenom de vanliga uppgifterna, och **[Funktioner](features.md)** listar allt BombVault gör.
- Klonar du en uppsättning eller flyttar till en ny box? Ta med hela din konfiguration med kortet **Exportera / importera inställningar**. Se [Konfiguration](configuration.md#portable-settings-export-and-import).
- Stötte på ett problem? Se **[Felsökning](troubleshooting.md)**.
