# Noder

En node er en BombVault uten nettgrensesnitt, for eksempel på en VPS eller på en liten server hos en venn. Den tar sikkerhetskopier av containerne og mappene på sin egen server, og den BombVault du legger den til på, som denne siden kaller **hjemmeinstansen**, sier fra når den skal ta sikkerhetskopi og hvor kopiene skal. Du styrer den fra nettgrensesnittet til hjemmeinstansen, ved siden av dine egne elementer.

![Kortet til noden vps-falkenstein på siden Instanser: Tilkoblet, versjonen, sikkerhetskopiene på mottaksserveren, når hvert område tar sikkerhetskopi, og Endre mål, Erstatt og Frigi.](assets/screenshots/nodes/card-dark.png#only-dark)
![Kortet til noden vps-falkenstein på siden Instanser: Tilkoblet, versjonen, sikkerhetskopiene på mottaksserveren, når hvert område tar sikkerhetskopi, og Endre mål, Erstatt og Frigi.](assets/screenshots/nodes/card-light.png#only-light)

## Slik virker det {#how-it-works}

- **Hjemmeinstansen har nøkkelen til noden.** Når du legger til en node, lager hjemmeinstansen dens `APP_KEY`, beholder en forseglet kopi og gir den til deg én gang, i compose-filen til noden. Med den kopien når hjemmeinstansen noden og åpner sikkerhetskopiene dens.
- **Noden trenger ingen åpen port.** Hjemmeinstansen og noden snakker gjennom relayet over en egen privat forbindelse, atskilt fra paringsgruppen din. Hjemmeinstansen starter hvert kall og signerer det, og noden ber den ikke om noe annet enn sitt hei. Det eneste målet som trenger en åpen port, er [å beholde sikkerhetskopiene på noden](#keep-on-node).
- **Sikkerhetskopier går aldri over relayet.** Noden sender dem rett til stedet du valgte: mottaksserveren til hjemmeinstansen, et av målene dens eller en rest-server på selve noden.
- **Noden planlegger ingenting selv.** Den har ingen tidsplaner, sender ingen varsler og blir ikke med i noen gruppe eller festning. Hjemmeinstansen starter sikkerhetskopiene dens, følger med på hvordan de går, og sender alarmene.
- **En node tar sikkerhetskopi av containere og mappesett.** VM-er, flash-stasjonen, ZFS-datasett og auto-sikkerhetskopien blir hos fullstendige installasjoner.

Forbindelsen bruker hjemmeinstansens eget relay når gruppen har et, og ellers prosjektets relay, også når hjemmeinstansen parer uten relay.

## Før du begynner {#before-you-start}

- **Et påloggingspassord på hjemmeinstansen.** Å legge til en node, vise compose-filen dens på nytt og åpne elementene dens krever en pålogging med passordet, se [Pålogging](configuration.md#login).
- **Docker med Compose på serveren til noden.**
- **Et sted til sikkerhetskopiene**, ett av de tre under [Hvor sikkerhetskopiene går](#placement). Mottaksserveren er det tryggeste valget, fordi noden ikke kan slette noe der.

## Legge til en node {#add}

1. Åpne **Instanser** på hjemmeinstansen og velg **Legg til node** nede til høyre.
2. **Navn.** Skriv hva noden skal hete her, for eksempel navnet på serveren, og velg **Neste**.

    ![Vinduet Legg til node i første steg, med navnet vps-helsinki skrevet inn.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Vinduet Legg til node i første steg, med navnet vps-helsinki skrevet inn.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Sikkerhetskopier.** Velg hvor sikkerhetskopiene skal, og fyll ut det stedet ber om. **Tar sikkerhetskopi daglig kl.** er tidspunktet begge områdene starter. Hjemmeinstansen foreslår en halvtime etter sine egne containerkopier, så de to ikke bruker de samme diskene samtidig. Senere kan du gi hvert område sin egen plan på kortet. Velg **Neste**.

    ![Andre steg: mottaksserveren valgt, Adresse for noder fylt ut med en https-adresse og Tar sikkerhetskopi daglig kl. 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Andre steg: mottaksserveren valgt, Adresse for noder fylt ut med en https-adresse og Tar sikkerhetskopi daglig kl. 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Start.** Vinduet viser compose-filen til noden med `MODE`, `APP_KEY`, `NODE_HOME` og `NODE_NAME` allerede fylt ut. Kopier den og ta godt vare på `APP_KEY`. På serveren til noden:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Før du starter den, pek volumet Host Data mot dataene du vil ta sikkerhetskopi av, og sett `HOST_SOURCE_ROOT` til den samme stien, slik eksempelet nedenfor viser.

5. Vinduet venter på noden. Så snart noden svarer, sier hjemmeinstansen fra hvor sikkerhetskopiene dens skal, og sjekker at den når dit, og vinduet sier **vps-helsinki er tilkoblet og tar imot oppdrag fra denne instansen.**

    ![Siste steg: compose-filen, deretter noden tilkoblet og sikkerhetskopiene dens på mottaksserveren.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Siste steg: compose-filen, deretter noden tilkoblet og sikkerhetskopiene dens på mottaksserveren.](assets/screenshots/nodes/add-start-light.png#only-light)

Du kan lukke vinduet før noden svarer. Kortet sier da **Venter på noden**, og hjemmeinstansen sier fra hvor sikkerhetskopiene skal så snart noden dukker opp.

### Eksempel: en VPS som tar sikkerhetskopi til mottaksserveren {#example-vps}

Hjemmeinstansen er en Unraid-boks som heter `tower` og kjører [mottaksserveren](offsite-recovery.md#receiving-server). En omvendt proxy publiserer den som `https://backup.tower.example.org`. VPS-en `vps-helsinki` kjører Nextcloud og Caddy, med dataene under `/srv/appdata`.

1. På `tower`: **Instanser**, **Legg til node**, navnet `vps-helsinki`, **Neste**.
2. Velg **Mottaksserveren til denne instansen** og skriv `https://backup.tower.example.org` under **Adresse for noder**. Containerne til hjemmeinstansen tar sikkerhetskopi kl. 02:30, så vinduet foreslår 03:00. Behold det og velg **Neste**.
3. Lagre filen fra vinduet på VPS-en, med volumene og `HOST_SOURCE_ROOT` endret for `/srv/appdata`:

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

4. `docker compose up -d`. Noen sekunder senere sier vinduet på `tower` at noden er tilkoblet, og **Sikkerhetskopiene går til https://backup.tower.example.org.**
5. Kl. 03:00 starter hjemmeinstansen sikkerhetskopiene til noden. Neste morgen viser kortet **Noder** i oversikten den siste sikkerhetskopien av `vps-helsinki`, og **Sikkerhetskopier** lister containerne dens i et eget område.

Hvordan BombVault finner dataene til en container på en vanlig Docker-vert, står under [Gjenkjenning av sikkerhetskopiens kilder](configuration.md#backup-source-detection).

## Hvor sikkerhetskopiene går {#placement}

Hver node har ett mål for begge områdene sine. **Endre mål** på kortet endrer det når noden er tilkoblet. Det nye stedet får nye repositoryer, og sikkerhetskopiene på det gamle stedet blir der de er.

### Mottaksserveren til denne instansen {#to-receiver}

Noden skriver til mottaksserveren til hjemmeinstansen i append-only-modus: den kan legge til sikkerhetskopier, men ikke slette noen. Den får en egen pålogging der, oppkalt etter noden, og ett repository per område i sin egen mappe. Hjemmeinstansen legger sin egen nøkkel til i hvert repository og beskjærer dem selv, fordi noden ikke kan.

Noden når serveren på **Adresse for noder**, som du setter i andre steg eller i vinduet **Mottaksserver**. En node sender som regel fra internett, så adressen må være en av disse:

- `https://`, for eksempel bak en omvendt proxy, Pangolin eller en tunnel. Viser serveren sitt eget sertifikat i stedet for et fra en offentlig utsteder, lim det inn i PEM-format under **Sertifikat**.
- `http://` med en privat IP-adresse, som `10.0.0.5` på en WireGuard-forbindelse eller `100.101.102.103` på Tailscale.

Vanlig `http://` til en offentlig adresse avvises, fordi noden da ville sende sikkerhetskopiene sine ukryptert over internett. Når du endrer adressen, gir hjemmeinstansen beskjed til hver node på mottakeren, og en node som er borte, får vite det så snart den er tilbake.

### Et mål {#to-location}

Noden skriver til et av målene til hjemmeinstansen som den selv kan nå: en S3-tjeneste eller en REST-server, over `https://` eller på en privat adresse. Repositoryene dens ligger der i `bombvault-node-<node id>-containers` og `bombvault-node-<node id>-files`.

Noden logger på med en [lagret pålogging](offsite-recovery.md#sign-ins) som er laget for den, og den må være en annen enn den hjemmeinstansen skriver til målet med. Legg til en først hvis vinduet sier at det ikke finnes noen. Med den påloggingen kan noden som regel slette det som ligger på målet, i motsetning til på mottaksserveren, og vinduet sier fra om det.

### Behold dem på noden {#keep-on-node}

Noden beholder sikkerhetskopiene sine selv, bak sin egen rest-server i append-only-modus, og hjemmeinstansen henter en kopi av hvert område inn i sin egen lagring. Det passer for en node som hjemmeinstansen ikke kan tilby plass. Det er det eneste målet som trenger en åpen port.

![Andre steg med Behold dem på noden valgt, adressen https://203.0.113.7:8443 og merknaden om den åpne porten.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Andre steg med Behold dem på noden valgt, adressen https://203.0.113.7:8443 og merknaden om den åpne porten.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Under **Nodens adresse** skriver du hvor hjemmeinstansen når noden: den offentlige adressen og en ledig port, som `https://203.0.113.7:8443`, uten noen mappe etter. Bruk `http://` bare med en privat adresse, som ovenfor.
2. Noden starter containeren `bombvault-node-rest-server` på den porten, med repositoryene sine i mappen `bombvault-node` under Host Data. Med `https://` lager den et sertifikat for den adressen, og hjemmeinstansen fester det.
3. Slipp porten gjennom brannmuren til noden, så hjemmeinstansen når den. En port som en annen container på noden allerede har, avvises med navnet på den containeren.

Hjemmeinstansen henter et område rett etter at nodens sikkerhetskopier av det er ferdige, og som reserve hver time kvart over, også med **Henting** slått av. Det hjemmeinstansen har hentet, blir hos den som enhver [hentet kopi](offsite-recovery.md#receiver), også etter at noden er borte. Noden beskjærer sine egne repositoryer når hjemmeinstansen ber om det, fordi rest-serveren dens ikke tar imot slettinger, og mappesettene dens utelater mappen `bombvault-node`, så den aldri tar sikkerhetskopi av sine egne sikkerhetskopier.

## Når noder tar sikkerhetskopi {#schedule}

Hjemmeinstansen eier tidsplanen til hver node. Kortet viser en plan per område, som **Kontainere** *Daglig kl. 2:00* og **Mapper** *Ukentlig (søn) kl. 4:00*. Et klikk åpner den samme redigereren som under [Tidsplaner](schedules.md#plans), og en endring lagres med en gang.

![Planen for containerne til en node, åpnet fra kortet, med Av, Daglig, Ukentlig, Hver N. dag og Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Planen for containerne til en node, åpnet fra kortet, med Av, Daglig, Ukentlig, Hver N. dag og Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **En node som var borte,** starter sikkerhetskopiene den gikk glipp av så snart den er tilbake. Hjemmeinstansen ser etter hvert femte minutt.
- **En node som er opptatt,** blir spurt igjen fem minutter senere.
- **Mens hjemmeinstansen beskjærer** de gamle sikkerhetskopiene til en node, starter noden ingenting før den er ferdig. På mottaksserveren og på selve noden åpner dette vinduet hver søndag kl. 03:00 og varer opptil seks timer. På et mål følger det hjemmeinstansens plan for den eksterne kopien til det målet. Hjemmeinstansen beholder sikkerhetskopiene etter sin egen regel for området under **Hvor lenge det beholdes**, eller etter målets regel der målet har en.
- **I listen over kjøringer på noden** vises sikkerhetskopiene og gjenopprettingene som hjemmeinstansen startet, som startet av hjemmeinstansen.

## Følge med på nodene dine {#watch}

Kortet **Noder** i oversikten lister hver node med tilstanden, den siste sikkerhetskopien, feilene og de åpne avvikene. Det dukker opp så snart du har en node.

![Kortet Noder i oversikten med tre noder: én tilkoblet, én inaktiv med to feil og én som fortsatt venter på noden.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Kortet Noder i oversikten med tre noder: én tilkoblet, én inaktiv med to feil og én som fortsatt venter på noden.](assets/screenshots/nodes/overview-card-light.png#only-light)

Hjemmeinstansen sender én alarm per hendelse gjennom [varslingskanalene](notifications.md) sine, og den neste først når den hendelsen er over:

- **En sikkerhetskopi feilet**, for eksempel *BombVault: vps-helsinki: backup failed*, med elementet og årsaken. En senere sikkerhetskopi av det elementet som lykkes, avslutter hendelsen.
- **Noden kan ikke nås**, for eksempel *BombVault: vps-helsinki is out of reach*. Hjemmeinstansen venter like lenge som den korteste dødmannsbryteren på repositoryene til noden, 26 timer med mindre du endrer det, eller 6 timer så lenge noden ennå ikke har noe mål.

Mottakerovervåkingen til hjemmeinstansen passer også på hvert repository for en node, med en dødmannsbryter og en daglig `restic check` kl. 04:00, se [Mottaker og henter](offsite-recovery.md#receiver).

## Elementene til en node {#entries}

Under **Sikkerhetskopier** har listene Kontainere og Mapper et område for hver node under dine egne elementer. Nodens egen container er ikke med. **Sikkerhetskopier nå** i menyen til et element starter sikkerhetskopien på noden, og **Åpne** åpner siden til elementet.

I en nodes område i listen Mapper legger **Legg til mappesett** til et mappesett på noden, og **Rediger** i menyen til et sett endrer det. Begge åpner det samme vinduet som for dine egne sett. Mappevelgeren der blar gjennom nodens mapper, men lager ikke nye mapper der.

![Området for noden vps-falkenstein i listen Kontainere: nextcloud, nextcloud-db, caddy og uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Området for noden vps-falkenstein i listen Kontainere: nextcloud, nextcloud-db, caddy og uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Siden til et element på en node sier **På** og navnet på noden, og lister sikkerhetskopiene. **Gjenopprett** på en av dem gjenoppretter elementet på noden og erstatter det som er der nå. En container stoppes for det, akkurat som på hjemmeinstansen.

![Siden til nextcloud på vps-falkenstein med sikkerhetskopiene og en knapp Gjenopprett ved siden av hver.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Siden til nextcloud på vps-falkenstein med sikkerhetskopiene og en knapp Gjenopprett ved siden av hver.](assets/screenshots/nodes/entry-light.png#only-light)

Gjennom hjemmeinstansen når du en fast liste over nodens egne ruter: containerne, mappesettene, kjøringene og avvikene, og å ta sikkerhetskopi av og gjenopprette dem. Innstillingene, nøklene og lagringen til noden er utenfor rekkevidde, og ingenting på den listen sletter noe. Se [Noder i API-et](api.md#nodes).

## Erstatte og frigi {#replace-release}

**Erstatt** viser compose-filen til noden på nytt, etter en pålogging med passordet. Bruk det til å flytte noden til en annen server eller bygge den opp igjen: start filen der og slå av den gamle serveren. Samme `APP_KEY` og `NODE_HOME` gjør den til den samme noden igjen, med sikkerhetskopiene og historikken sin. To servere med samme `APP_KEY` regnes som én node, så kjør aldri begge.

**Frigi** lar en node gå. Den slutter å ta imot oppdrag fra hjemmeinstansen og tar ingen flere sikkerhetskopier. Sikkerhetskopiene dens blir der de er, og det samme gjør hver kopi hjemmeinstansen har hentet. Påloggingen dens på mottaksserveren forsvinner, og hjemmeinstansen slutter å overvåke repositoryene dens. En node som er borte når du frigir den, beholder kortet som **Frigis** til den får vite det. En frigitt node tar ikke imot noen hjemmeinstans igjen med samme `APP_KEY`. For å bruke serveren igjen legger du den til som en ny node og starter den med den nye compose-filen og en tom mappe `config`.

## Sikkerhet {#security}

Blir hjemmeinstansen tatt over, styrer angriperen også nodene dens, og en node har Docker på serveren sin i hendene, noe som er root der. Beskytt hjemmeinstansen med et passord og hold den unna internett. Blir en node tatt over, får angriperen ingen paringsgruppe, ingen festning, ingen annen node og ikke noe annet repository, og kan ikke slette historikk på mottaksserveren. Detaljene står under [Noder i sikkerhetsmodellen](configuration.md#nodes).

[Gjenopprettingssettet](offsite-recovery.md#recovery-kit) har et kapittel **Nodes** med `APP_KEY` til hver node, restic-passordet og repositoryene, så du kan hente tilbake en node eller gjenopprette sikkerhetskopiene dens med bare restic.
