# Noder

En node er en BombVault uden brugerflade, for eksempel på en VPS eller på en lille server hos en ven. Den tager sikkerhedskopier af containerne og mapperne på sin egen server, og den BombVault, du tilføjer den på, som denne side kalder **hjemmeinstansen**, fortæller den, hvornår den skal tage sikkerhedskopier, og hvor de skal hen. Du passer den fra hjemmeinstansens brugerflade, ved siden af dine egne poster.

![Kortet for noden vps-falkenstein på siden Instanser: Forbundet, dens version, dens sikkerhedskopier på modtageserveren, hvornår hvert område tager sikkerhedskopi, og Skift destination, Erstat og Frigiv.](assets/screenshots/nodes/card-dark.png#only-dark)
![Kortet for noden vps-falkenstein på siden Instanser: Forbundet, dens version, dens sikkerhedskopier på modtageserveren, hvornår hvert område tager sikkerhedskopi, og Skift destination, Erstat og Frigiv.](assets/screenshots/nodes/card-light.png#only-light)

## Sådan virker det {#how-it-works}

- **Hjemmeinstansen har nodens nøgle.** Når du tilføjer en node, laver hjemmeinstansen dens `APP_KEY`, beholder en forseglet kopi og giver dig den én gang, i nodens compose-fil. Med den kopi når hjemmeinstansen noden og åbner dens sikkerhedskopier.
- **Noden har ikke brug for en åben port.** Hjemmeinstansen og noden taler sammen gennem videresenderen over deres egen private forbindelse, adskilt fra din parringsgruppe. Hjemmeinstansen starter hvert kald og signerer det, og noden beder den ikke om andet end sit goddag. Den eneste destination, der kræver en åben port, er [at beholde sikkerhedskopierne på noden](#keep-on-node).
- **Sikkerhedskopier går aldrig over videresenderen.** Noden sender dem direkte til det sted, du har valgt: hjemmeinstansens modtageserver, en af dens destinationer eller en rest-server på selve noden.
- **Noden planlægger ikke noget selv.** Den har ingen tidsplaner, sender ingen notifikationer og melder sig ikke ind i nogen gruppe eller fæstning. Hjemmeinstansen starter dens sikkerhedskopier, holder øje med, hvordan de går, og sender advarslerne.
- **En node tager sikkerhedskopier af containere og mappesæt.** VM'er, flashdrevet, ZFS-datasæt og auto-sikkerhedskopien bliver hos fulde installationer.

Forbindelsen bruger hjemmeinstansens egen videresender, når dens gruppe har en, og ellers projektets videresender, også når hjemmeinstansen parrer uden videresender.

## Før du går i gang {#before-you-start}

- **En login-adgangskode på hjemmeinstansen.** At tilføje en node, vise dens compose-fil igen og åbne dens poster kræver et login med adgangskoden, se [Login](configuration.md#login).
- **Docker med Compose på nodens server.**
- **Et sted til sikkerhedskopierne**, et af de tre under [Hvor sikkerhedskopierne går hen](#placement). Modtageserveren er det sikreste valg, fordi noden ikke kan slette noget dér.

## Tilføj en node {#add}

1. Åbn **Instanser** på hjemmeinstansen, og vælg **Tilføj node** nederst til højre.
2. **Navn.** Skriv, hvad noden skal hedde her, for eksempel serverens navn, og vælg **Næste**.

    ![Vinduet Tilføj node ved første trin, med navnet vps-helsinki skrevet ind.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Vinduet Tilføj node ved første trin, med navnet vps-helsinki skrevet ind.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Sikkerhedskopier.** Vælg, hvor sikkerhedskopierne skal hen, og udfyld det, stedet beder om. **Sikkerhedskopierer dagligt kl.** er tidspunktet, hvor begge områder starter. Hjemmeinstansen foreslår en halv time efter sine egne containerkopier, så de to ikke bruger de samme diske på samme tid. Senere kan du give hvert område sin egen plan på kortet. Vælg **Næste**.

    ![Andet trin: modtageserveren valgt, Adresse til noder udfyldt med en https-adresse og Sikkerhedskopierer dagligt kl. 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Andet trin: modtageserveren valgt, Adresse til noder udfyldt med en https-adresse og Sikkerhedskopierer dagligt kl. 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Start.** Vinduet viser nodens compose-fil med `MODE`, `APP_KEY`, `NODE_HOME` og `NODE_NAME` allerede udfyldt. Kopiér den, og gem din `APP_KEY` et sikkert sted. På nodens server:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Før du starter den, så peg volumen Host Data på de data, du vil have sikkerhedskopieret, og sæt `HOST_SOURCE_ROOT` til den samme sti, som eksemplet herunder viser.

5. Vinduet venter på noden. Så snart noden svarer, fortæller hjemmeinstansen den, hvor dens sikkerhedskopier skal hen, og tjekker, at den kan nå dem, og vinduet siger **vps-helsinki er online og tager imod ordrer fra denne instans.**

    ![Sidste trin: compose-filen, derefter noden online og dens sikkerhedskopier på modtageserveren.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Sidste trin: compose-filen, derefter noden online og dens sikkerhedskopier på modtageserveren.](assets/screenshots/nodes/add-start-light.png#only-light)

Du kan lukke vinduet, før noden svarer. Dens kort siger så **Venter på noden**, og hjemmeinstansen fortæller den, hvor dens sikkerhedskopier skal hen, så snart den dukker op.

### Eksempel: en VPS, der tager sikkerhedskopier til modtageserveren {#example-vps}

Hjemmeinstansen er en Unraid-boks ved navn `tower`, der kører [modtageserveren](offsite-recovery.md#receiving-server). En reverse proxy udgiver den som `https://backup.tower.example.org`. VPS'en `vps-helsinki` kører Nextcloud og Caddy, med deres data under `/srv/appdata`.

1. På `tower`: **Instanser**, **Tilføj node**, navnet `vps-helsinki`, **Næste**.
2. Vælg **Denne instans' modtageserver**, og skriv `https://backup.tower.example.org` under **Adresse til noder**. Hjemmeinstansens containere tager sikkerhedskopi kl. 02:30, så vinduet foreslår 03:00. Behold det, og vælg **Næste**.
3. Gem filen fra vinduet på VPS'en, med volumenerne og `HOST_SOURCE_ROOT` ændret til `/srv/appdata`:

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

4. `docker compose up -d`. Et par sekunder senere siger vinduet på `tower`, at noden er online, og **Sikkerhedskopierne går til https://backup.tower.example.org.**
5. Kl. 03:00 starter hjemmeinstansen nodens sikkerhedskopier. Næste morgen viser kortet **Noder** i oversigten den seneste sikkerhedskopi af `vps-helsinki`, og **Sikkerhedskopier** viser dens containere i et område for sig.

Hvordan BombVault finder en containers data på en almindelig Docker-vært, står under [Genkendelse af sikkerhedskopiernes kilder](configuration.md#backup-source-detection).

## Hvor sikkerhedskopierne går hen {#placement}

Hver node har én destination for begge sine områder. **Skift destination** på dens kort ændrer den, når noden er online. Det nye sted får nye repositories, og sikkerhedskopierne på det gamle sted bliver, hvor de er.

### Denne instans' modtageserver {#to-receiver}

Noden skriver til hjemmeinstansens modtageserver i append-only-tilstand: den kan tilføje sikkerhedskopier, men ikke slette nogen. Den får sit eget login dér, opkaldt efter noden, og ét repository pr. område i sin egen mappe. Hjemmeinstansen tilføjer sin egen nøgle til hvert repository og beskærer dem selv, da noden ikke kan.

Noden når serveren på **Adresse til noder**, som du sætter i andet trin eller i vinduet **Modtageserver**. En node sender som regel fra internettet, så adressen skal være en af disse:

- `https://`, for eksempel bag en reverse proxy, Pangolin eller en tunnel. Viser serveren sit eget certifikat i stedet for et fra en offentlig udsteder, så indsæt det i PEM-format under **Certifikat**.
- `http://` med en privat IP-adresse, som `10.0.0.5` på en WireGuard-forbindelse eller `100.101.102.103` på Tailscale.

Almindelig `http://` til en offentlig adresse afvises, fordi noden ellers ville sende sine sikkerhedskopier ukrypteret over internettet. Når du ændrer adressen, giver hjemmeinstansen besked til hver node på modtageren, og en node, der er væk, hører det, så snart den er tilbage.

### En destination {#to-location}

Noden skriver til en af hjemmeinstansens destinationer, som den selv kan nå: en S3-tjeneste eller en REST-server, over `https://` eller på en privat adresse. Dens repositories ligger dér i `bombvault-nodes/<node id>/containers` og `bombvault-nodes/<node id>/files`.

Noden logger ind med [gemte adgangsoplysninger](offsite-recovery.md#sign-ins), der er lavet til den, og de skal være andre end dem, hjemmeinstansen skriver til destinationen med. Tilføj nogle først, hvis vinduet siger, at der ikke er nogen. Med de adgangsoplysninger kan noden som regel slette det, der ligger på destinationen, i modsætning til på modtageserveren, og vinduet siger det.

### Behold dem på noden {#keep-on-node}

Noden beholder selv sine sikkerhedskopier, bag sin egen rest-server i append-only-tilstand, og hjemmeinstansen henter en kopi af hvert område ind i sin egen lagring. Det passer til en node, som hjemmeinstansen ikke kan tilbyde plads. Det er den eneste destination, der kræver en åben port.

![Andet trin med Behold dem på noden valgt, adressen https://203.0.113.7:8443 og bemærkningen om den åbne port.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Andet trin med Behold dem på noden valgt, adressen https://203.0.113.7:8443 og bemærkningen om den åbne port.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Under **Nodens adresse** skriver du, hvor hjemmeinstansen når noden: dens offentlige adresse og en ledig port, som `https://203.0.113.7:8443`, uden mappe efter. Brug kun `http://` med en privat adresse, som ovenfor.
2. Noden starter containeren `bombvault-node-rest-server` på den port, med sine repositories i mappen `bombvault-node` under Host Data. Med `https://` laver den et certifikat til den adresse, og hjemmeinstansen fastlåser det.
3. Åbn porten i nodens firewall, så hjemmeinstansen kan nå den. En port, som en anden container på noden allerede holder, afvises med den containers navn.

Hjemmeinstansen henter et område lige efter, at nodens sikkerhedskopier af det er færdige, og som reserve hver time kvart over, også med **Hentning** slået fra. Det, hjemmeinstansen har hentet, bliver hos den som enhver [hentet kopi](offsite-recovery.md#receiver), også efter at noden er væk. Noden beskærer sine egne repositories, når hjemmeinstansen beder om det, da dens rest-server ikke tager imod sletninger, og dens mappesæt udelader mappen `bombvault-node`, så den aldrig tager sikkerhedskopi af sine egne sikkerhedskopier.

## Hvornår noder tager sikkerhedskopi {#schedule}

Hjemmeinstansen ejer hver nodes tidsplan. Kortet viser en plan pr. område, som **Containers** *Dagligt kl. 2:00* og **Mapper** *Ugentligt (søn) kl. 4:00*. Et klik åbner den samme editor som under [Tidsplaner](schedules.md#plans), og en ændring gemmes med det samme.

![Planen for en nodes containere åbnet fra dens kort, med Fra, Dagligt, Ugentligt, Hver N. dag og Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Planen for en nodes containere åbnet fra dens kort, med Fra, Dagligt, Ugentligt, Hver N. dag og Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **En node, der har været væk,** starter de sikkerhedskopier, den gik glip af, så snart den er tilbage. Hjemmeinstansen kigger hvert femte minut.
- **En node, der er optaget,** bliver spurgt igen fem minutter senere.
- **Mens hjemmeinstansen beskærer** en nodes gamle sikkerhedskopier, starter noden intet, før den er færdig. På modtageserveren og på selve noden åbner dette vindue hver søndag kl. 03:00 og varer op til seks timer. På en destination følger det hjemmeinstansens plan for off-site-kopien til den destination. Hjemmeinstansen beholder sikkerhedskopierne efter sin egen regel for området under **Hvor længe der gemmes**, eller efter destinationens regel, hvor den har en.
- **I nodens liste over kørsler** vises de sikkerhedskopieringer og gendannelser, hjemmeinstansen har startet, som startet af hjemmeinstansen.

## Hold øje med dine noder {#watch}

Kortet **Noder** i oversigten viser hver node med dens tilstand, dens seneste sikkerhedskopi, dens fejl og dens åbne afvigelser. Det dukker op, så snart du har en node.

![Kortet Noder i oversigten med tre noder: én forbundet, én inaktiv med to fejl og én, der stadig venter på noden.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Kortet Noder i oversigten med tre noder: én forbundet, én inaktiv med to fejl og én, der stadig venter på noden.](assets/screenshots/nodes/overview-card-light.png#only-light)

Hjemmeinstansen sender én advarsel pr. hændelse gennem sine [notifikationskanaler](notifications.md), og den næste først, når den hændelse er overstået:

- **En sikkerhedskopiering mislykkedes**, for eksempel *BombVault: vps-helsinki: backup failed*, med posten og årsagen. En senere sikkerhedskopiering af den post, der lykkes, afslutter hændelsen.
- **Noden kan ikke nås**, for eksempel *BombVault: vps-helsinki is out of reach*. Hjemmeinstansen venter lige så længe som den korteste dødmandsknap på nodens repositories, 26 timer, medmindre du ændrer det, eller 6 timer, så længe noden endnu ikke har en destination.

Hjemmeinstansens modtagerovervågning holder også øje med hvert repository for en node, med en dødmandsknap og et dagligt `restic check` kl. 04:00, se [Modtager og henter](offsite-recovery.md#receiver).

## En nodes poster {#entries}

Under **Sikkerhedskopier** har listerne Containers og Mapper et område for hver node under dine egne poster. Nodens egen container er ikke med. **Sikkerhedskopier nu** i en posts menu starter dens sikkerhedskopiering på noden, og **Åbn** åbner dens side.

![Området for noden vps-falkenstein i listen Containers: nextcloud, nextcloud-db, caddy og uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Området for noden vps-falkenstein i listen Containers: nextcloud, nextcloud-db, caddy og uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Siden for en post på en node siger **På** og nodens navn og viser dens sikkerhedskopier. **Gendan** på en af dem gendanner posten på noden og erstatter det, der er der nu. En container stoppes til det, ligesom på hjemmeinstansen.

![Siden for nextcloud på vps-falkenstein med dens sikkerhedskopier og en knap Gendan ved siden af hver.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Siden for nextcloud på vps-falkenstein med dens sikkerhedskopier og en knap Gendan ved siden af hver.](assets/screenshots/nodes/entry-light.png#only-light)

Gennem hjemmeinstansen når du en fast liste over nodens egne ruter: dens containere, mappesæt, kørsler og afvigelser, og at sikkerhedskopiere og gendanne dem. Nodens indstillinger, nøgler og lagring er uden for rækkevidde, og intet på den liste sletter noget. Se [Noder i API'et](api.md#nodes).

## Erstat og frigiv {#replace-release}

**Erstat** viser nodens compose-fil igen efter et login med adgangskoden. Brug det til at flytte noden til en anden server eller bygge den op igen: start filen dér, og luk den gamle server ned. Samme `APP_KEY` og `NODE_HOME` gør den til den samme node igen, med dens sikkerhedskopier og dens historik. To servere med samme `APP_KEY` tæller som én node, så kør aldrig begge.

**Frigiv** lader en node gå. Den holder op med at tage imod ordrer fra hjemmeinstansen og tager ikke flere sikkerhedskopier. Dens sikkerhedskopier bliver, hvor de er, og det gør hver kopi, hjemmeinstansen har hentet, også. Dens login på modtageserveren forsvinder, og hjemmeinstansen holder op med at overvåge dens repositories. En node, der er væk, når du frigiver den, beholder sit kort som **Frigives**, indtil den hører det. En frigivet node tager ikke imod nogen hjemmeinstans igen med samme `APP_KEY`. Vil du bruge serveren igen, så tilføj den som en ny node, og start den med den nye compose-fil og en tom mappe `config`.

## Sikkerhed {#security}

Bliver hjemmeinstansen overtaget, styrer angriberen også dens noder, og en node har Docker på sin server i hænderne, hvilket er root dér. Beskyt hjemmeinstansen med en adgangskode, og hold den væk fra internettet. Bliver en node overtaget, får angriberen ingen parringsgruppe, ingen fæstning, ingen anden node og intet andet repository og kan ikke slette historik på modtageserveren. Detaljerne står under [Noder i sikkerhedsmodellen](configuration.md#nodes).

[Gendannelseskittet](offsite-recovery.md#recovery-kit) har et kapitel **Nodes** med hver nodes `APP_KEY`, dens restic-adgangskode og dens repositories, så du kan få en node tilbage eller gendanne dens sikkerhedskopier med restic alene.
