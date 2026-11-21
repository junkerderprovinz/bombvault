# Noder

En nod är en BombVault utan webbgränssnitt, till exempel på en VPS eller på en liten server hemma hos en vän. Den säkerhetskopierar containrarna och mapparna på sin egen server, och den BombVault du lägger till den på, som den här sidan kallar **hemmainstansen**, talar om för den när den ska säkerhetskopiera och vart kopiorna ska. Du sköter den från hemmainstansens webbgränssnitt, bredvid dina egna poster.

![Kortet för noden vps-falkenstein på sidan Instanser: Ansluten, dess version, dess säkerhetskopior på mottagarservern, när varje avsnitt säkerhetskopieras, och Ändra mål, Ersätt och Släpp.](assets/screenshots/nodes/card-dark.png#only-dark)
![Kortet för noden vps-falkenstein på sidan Instanser: Ansluten, dess version, dess säkerhetskopior på mottagarservern, när varje avsnitt säkerhetskopieras, och Ändra mål, Ersätt och Släpp.](assets/screenshots/nodes/card-light.png#only-light)

## Så fungerar det {#how-it-works}

- **Hemmainstansen har nodens nyckel.** När du lägger till en nod skapar hemmainstansen dess `APP_KEY`, behåller en förseglad kopia och ger den till dig en enda gång, i nodens compose-fil. Med den kopian når hemmainstansen noden och öppnar dess säkerhetskopior.
- **Noden behöver ingen öppen port.** Hemmainstansen och noden pratar via reläet över en egen privat länk, skild från din parkopplingsgrupp. Hemmainstansen startar varje anrop och signerar det, och noden ber den inte om något annat än sitt hej. Det enda mål som behöver en öppen port är [att behålla säkerhetskopiorna på noden](#keep-on-node).
- **Säkerhetskopior går aldrig över reläet.** Noden skickar dem direkt till platsen du valde: hemmainstansens mottagarserver, ett av dess lagringsmål eller en rest-server på själva noden.
- **Noden planerar ingenting själv.** Den har inga scheman, skickar inga aviseringar och går inte med i någon grupp eller fästning. Hemmainstansen startar dess säkerhetskopior, följer hur de går och skickar larmen.
- **En nod säkerhetskopierar containrar och mappuppsättningar.** VM:ar, flashenheten, ZFS-datauppsättningar och auto-säkerhetskopian stannar hos fullständiga installationer.

Länken använder hemmainstansens eget relä när dess grupp har ett, och annars projektets relä, även när hemmainstansen parkopplar utan relä.

## Innan du börjar {#before-you-start}

- **Ett inloggningslösenord på hemmainstansen.** Att lägga till en nod, visa dess compose-fil igen och öppna dess poster kräver en inloggning med lösenordet, se [Inloggning](configuration.md#login).
- **Docker med Compose på nodens server.**
- **En plats för säkerhetskopiorna**, en av de tre under [Vart säkerhetskopiorna går](#placement). Mottagarservern är det säkraste valet, eftersom noden inte kan radera något där.

## Lägga till en nod {#add}

1. Öppna **Instanser** på hemmainstansen och välj **Lägg till nod** nere till höger.
2. **Namn.** Skriv vad noden ska heta här, till exempel serverns namn, och välj **Nästa**.

    ![Fönstret Lägg till nod i första steget, med namnet vps-helsinki inskrivet.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Fönstret Lägg till nod i första steget, med namnet vps-helsinki inskrivet.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Säkerhetskopior.** Välj vart säkerhetskopiorna ska och fyll i det som platsen frågar efter. **Säkerhetskopierar dagligen kl.** är tiden då båda avsnitten startar. Hemmainstansen föreslår en halvtimme efter sina egna containerkopior, så att de två inte använder samma diskar samtidigt. Senare kan du ge varje avsnitt ett eget schema på kortet. Välj **Nästa**.

    ![Det andra steget: mottagarservern vald, Adress för noder ifylld med en https-adress och Säkerhetskopierar dagligen kl. 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Det andra steget: mottagarservern vald, Adress för noder ifylld med en https-adress och Säkerhetskopierar dagligen kl. 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Starta.** Fönstret visar nodens compose-fil med `MODE`, `APP_KEY`, `NODE_HOME` och `NODE_NAME` redan ifyllda. Kopiera den och förvara `APP_KEY` på ett säkert ställe. På nodens server:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Innan du startar den, peka volymen Host Data mot datan du vill säkerhetskopiera och sätt `HOST_SOURCE_ROOT` till samma sökväg, som exemplet nedan visar.

5. Fönstret väntar på noden. Så snart noden svarar talar hemmainstansen om vart dess säkerhetskopior går och kontrollerar att den når dit, och fönstret säger **vps-helsinki är online och tar emot order från den här instansen.**

    ![Det sista steget: compose-filen, sedan noden online och dess säkerhetskopior på mottagarservern.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Det sista steget: compose-filen, sedan noden online och dess säkerhetskopior på mottagarservern.](assets/screenshots/nodes/add-start-light.png#only-light)

Du kan stänga fönstret innan noden svarar. Dess kort säger då **Väntar på noden**, och hemmainstansen talar om för den vart säkerhetskopiorna går så snart den dyker upp.

### Exempel: en VPS som säkerhetskopierar till mottagarservern {#example-vps}

Hemmainstansen är en Unraid-låda som heter `tower` och kör [mottagarservern](offsite-recovery.md#receiving-server). En omvänd proxy publicerar den som `https://backup.tower.example.org`. VPS:en `vps-helsinki` kör Nextcloud och Caddy, med sin data under `/srv/appdata`.

1. På `tower`: **Instanser**, **Lägg till nod**, namnet `vps-helsinki`, **Nästa**.
2. Välj **Den här instansens mottagarserver** och skriv `https://backup.tower.example.org` under **Adress för noder**. Hemmainstansens containrar säkerhetskopieras 02:30, så fönstret föreslår 03:00. Behåll det och välj **Nästa**.
3. Spara filen från fönstret på VPS:en, med volymerna och `HOST_SOURCE_ROOT` ändrade för `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Några sekunder senare säger fönstret på `tower` att noden är online och **Säkerhetskopiorna går till https://backup.tower.example.org.**
5. Klockan 03:00 startar hemmainstansen nodens säkerhetskopior. Nästa morgon visar kortet **Noder** i översikten den senaste säkerhetskopian av `vps-helsinki`, och **Säkerhetskopior** listar dess containrar i ett eget avsnitt.

Hur BombVault hittar en containers data på en vanlig Docker-värd beskrivs under [Igenkänning av säkerhetskopieringens källor](configuration.md#backup-source-detection).

## Vart säkerhetskopiorna går {#placement}

Varje nod har ett enda mål för båda sina avsnitt. **Ändra mål** på dess kort ändrar det när noden är online. Den nya platsen får nya repositorier, och säkerhetskopiorna på den gamla platsen ligger kvar där de är.

### Den här instansens mottagarserver {#to-receiver}

Noden skriver till hemmainstansens mottagarserver i append-only-läge: den kan lägga till säkerhetskopior men inte radera några. Den får en egen inloggning där, uppkallad efter noden, och ett repositorium per avsnitt i sin egen mapp. Hemmainstansen lägger till sin egen nyckel i varje repositorium och rensar dem själv, eftersom noden inte kan.

Noden når servern på **Adress för noder**, som du anger i det andra steget eller i fönstret **Mottagarserver**. En nod skickar oftast från internet, så adressen måste vara en av de här:

- `https://`, till exempel bakom en omvänd proxy, Pangolin eller en tunnel. När servern visar ett eget certifikat i stället för ett från en offentlig utfärdare, klistra in det i PEM-format under **Certifikat**.
- `http://` med en privat IP-adress, som `10.0.0.5` på en WireGuard-länk eller `100.101.102.103` på Tailscale.

Vanlig `http://` till en offentlig adress nekas, eftersom noden då skulle skicka sina säkerhetskopior okrypterade över internet. När du ändrar adressen meddelar hemmainstansen varje nod på mottagaren, och en nod som är borta får veta det så snart den är tillbaka.

### Ett lagringsmål {#to-location}

Noden skriver till ett av hemmainstansens lagringsmål som den själv kan nå: en S3-tjänst eller en REST-server, över `https://` eller på en privat adress. Dess repositorier ligger där i `bombvault-node-<node id>-containers` och `bombvault-node-<node id>-files`.

Noden loggar in med en [sparad inloggning](offsite-recovery.md#sign-ins) som skapats för den, och den måste vara en annan än den hemmainstansen skriver till lagringsmålet med. Lägg till en först om fönstret säger att det inte finns någon. Med den inloggningen kan noden oftast radera det som ligger på lagringsmålet, till skillnad från på mottagarservern, och fönstret säger det.

### Behåll dem på noden {#keep-on-node}

Noden behåller sina säkerhetskopior själv, bakom en egen rest-server i append-only-läge, och hemmainstansen hämtar en kopia av varje avsnitt till sin egen lagring. Det passar en nod som hemmainstansen inte kan erbjuda någon plats. Det är det enda mål som behöver en öppen port.

![Det andra steget med Behåll dem på noden valt, adressen https://203.0.113.7:8443 och anteckningen om den öppna porten.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Det andra steget med Behåll dem på noden valt, adressen https://203.0.113.7:8443 och anteckningen om den öppna porten.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Under **Nodens adress** skriver du var hemmainstansen når noden: dess offentliga adress och en ledig port, som `https://203.0.113.7:8443`, utan någon mapp efter. Använd `http://` bara med en privat adress, som ovan.
2. Noden startar containern `bombvault-node-rest-server` på den porten, med sina repositorier i mappen `bombvault-node` under Host Data. Med `https://` skapar den ett certifikat för den adressen, och hemmainstansen fäster det.
3. Släpp igenom porten i nodens brandvägg, så att hemmainstansen når den. En port som en annan container på noden redan håller nekas, med den containerns namn.

Hemmainstansen hämtar ett avsnitt direkt efter att nodens säkerhetskopior av det är klara, och som reserv varje timme kvart över, även med **Hämtning** avslaget. Det hemmainstansen har hämtat stannar hos den som alla [hämtade kopior](offsite-recovery.md#receiver), även när noden är borta. Noden rensar sina egna repositorier när hemmainstansen ber om det, eftersom dess rest-server inte tar emot några raderingar, och dess mappuppsättningar utesluter mappen `bombvault-node`, så att den aldrig säkerhetskopierar sina egna säkerhetskopior.

## När noder säkerhetskopierar {#schedule}

Hemmainstansen äger varje nods schema. Kortet visar ett schema per avsnitt, som **Containers** *Dagligen kl. 2:00* och **Mappar** *Veckovis (sön) kl. 4:00*. Ett klick öppnar samma redigerare som under [Scheman](schedules.md#plans), och en ändring sparas direkt.

![Schemat för en nods containrar öppnat från dess kort, med Av, Dagligen, Veckovis, Var N:e dag och Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Schemat för en nods containrar öppnat från dess kort, med Av, Dagligen, Veckovis, Var N:e dag och Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **En nod som var borta** startar de säkerhetskopior den missade så snart den är tillbaka. Hemmainstansen tittar var femte minut.
- **En nod som är upptagen** blir tillfrågad igen fem minuter senare.
- **Medan hemmainstansen rensar** en nods gamla säkerhetskopior startar noden ingenting förrän den är klar. På mottagarservern och på själva noden öppnas det här fönstret varje söndag kl. 03:00 och varar upp till sex timmar. På ett lagringsmål följer det hemmainstansens schema för off-site-kopian till det lagringsmålet. Hemmainstansen behåller säkerhetskopiorna efter sin egen regel för avsnittet under **Hur länge det bevaras**, eller efter lagringsmålets regel där det har en.
- **I nodens lista över körningar** visas de säkerhetskopieringar och återställningar som hemmainstansen startade som startade av hemmainstansen.

## Hålla koll på dina noder {#watch}

Kortet **Noder** i översikten listar varje nod med dess tillstånd, dess senaste säkerhetskopia, dess fel och dess öppna avvikelser. Det dyker upp så snart du har en nod.

![Kortet Noder i översikten med tre noder: en ansluten, en inaktiv med två fel och en som fortfarande väntar på noden.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Kortet Noder i översikten med tre noder: en ansluten, en inaktiv med två fel och en som fortfarande väntar på noden.](assets/screenshots/nodes/overview-card-light.png#only-light)

Hemmainstansen skickar ett larm per händelse via sina [aviseringskanaler](notifications.md), och nästa först när den händelsen är över:

- **En säkerhetskopiering misslyckades**, till exempel *BombVault: vps-helsinki: backup failed*, med posten och orsaken. En senare säkerhetskopiering av den posten som lyckas avslutar händelsen.
- **Noden går inte att nå**, till exempel *BombVault: vps-helsinki is out of reach*. Hemmainstansen väntar lika länge som den kortaste död mans knapp på nodens repositorier, 26 timmar om du inte ändrar det, eller 6 timmar medan noden ännu inte har något mål.

Hemmainstansens mottagarövervakning tar också hand om varje repositorium för en nod, med en död mans knapp och en daglig `restic check` kl. 04:00, se [Mottagare och hämtare](offsite-recovery.md#receiver).

## En nods poster {#entries}

Under **Säkerhetskopior** har listorna Containers och Mappar ett avsnitt för varje nod under dina egna poster. Nodens egen container visas inte där. **Säkerhetskopiera nu** i en posts meny startar dess säkerhetskopiering på noden, och **Öppna** öppnar dess sida.

I en nods avsnitt i listan Mappar lägger **Lägg till mappuppsättning** till en mappuppsättning på noden, och **Redigera** i en uppsättnings meny ändrar den. Båda öppnar samma fönster som för dina egna uppsättningar. Mappväljaren i det bläddrar bland nodens mappar men skapar inga nya mappar där.

![Avsnittet för noden vps-falkenstein i listan Containers: nextcloud, nextcloud-db, caddy och uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Avsnittet för noden vps-falkenstein i listan Containers: nextcloud, nextcloud-db, caddy och uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Sidan för en post på en nod säger **På** och nodens namn, och listar dess säkerhetskopior. **Återställ** på en av dem återställer posten på noden och ersätter det som finns där nu. En container stoppas för det, precis som på hemmainstansen.

![Sidan för nextcloud på vps-falkenstein med dess säkerhetskopior och en knapp Återställ bredvid var och en.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Sidan för nextcloud på vps-falkenstein med dess säkerhetskopior och en knapp Återställ bredvid var och en.](assets/screenshots/nodes/entry-light.png#only-light)

Genom hemmainstansen når du en fast lista över nodens egna rutter: dess containrar, mappuppsättningar, körningar och avvikelser, och att säkerhetskopiera och återställa dem. Nodens inställningar, nycklar och lagring ligger utom räckhåll, och ingenting på den listan raderar något. Se [Noder i API:et](api.md#nodes).

## Ersätta och släppa {#replace-release}

**Ersätt** visar nodens compose-fil igen, efter en inloggning med lösenordet. Använd det för att flytta noden till en annan server eller bygga upp den på nytt: starta filen där och stäng av den gamla servern. Samma `APP_KEY` och `NODE_HOME` gör den till samma nod igen, med dess säkerhetskopior och dess historik. Hemmainstansen sparar varje nods mappuppsättningar, dess containrar med det som behövs för att skapa dem på nytt och dess körningar. När noden startar med en tom mapp `config` tar hemmainstansen in den igen, berättar var dess säkerhetskopior ska och ger tillbaka allt detta före nästa säkerhetskopia, så att noden kan återställa en container som är borta. På en ny server börjar en nod som behåller sina säkerhetskopior själv med tomma repositorier. Dess tidigare säkerhetskopior finns kvar i kopiorna som hemmainstansen har hämtat. Två servrar med samma `APP_KEY` räknas som en nod, så kör aldrig båda.

**Släpp** låter en nod gå. Den slutar ta emot order från hemmainstansen och säkerhetskopierar ingenting längre. Dess säkerhetskopior ligger kvar där de är, liksom varje kopia som hemmainstansen hämtat. Dess inloggning på mottagarservern försvinner, och hemmainstansen slutar övervaka dess repositorier. En nod som är borta när du släpper den behåller sitt kort som **Släpps** tills den får veta det. En släppt nod tar inte emot någon hemmainstans igen med samma `APP_KEY`. För att använda servern igen lägger du till den som en ny nod och startar den med den nya compose-filen och en tom mapp `config`.

## Säkerhet {#security}

Om hemmainstansen tas över styr angriparen också dess noder, och en nod har Docker på sin server i sin hand, vilket är root där. Skydda hemmainstansen med ett lösenord och håll den borta från internet. Om en nod tas över får angriparen ingen parkopplingsgrupp, ingen fästning, ingen annan nod och inget annat repositorium, och kan inte radera historik på mottagarservern. Detaljerna finns under [Noder i säkerhetsmodellen](configuration.md#nodes).

[Återställningskitet](offsite-recovery.md#recovery-kit) har ett kapitel **Nodes** med varje nods `APP_KEY`, dess restic-lösenord och dess repositorier, så att du kan ta tillbaka en nod eller återställa dess säkerhetskopior med bara restic.
