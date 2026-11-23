# Nodes

Een node is een BombVault zonder webinterface, bijvoorbeeld op een VPS of op een kleine server bij een vriend. Hij maakt back-ups van de containers en mappen op zijn eigen server, en de BombVault waarop je hem toevoegt, op deze pagina de **thuisinstantie** genoemd, vertelt hem wanneer hij een back-up maakt en waar de back-ups heen gaan. Je beheert hem vanuit de webinterface van de thuisinstantie, naast je eigen items.

![De kaart van de node vps-falkenstein op de pagina Instanties: Verbonden, zijn versie, zijn back-ups op de ontvangserver, wanneer elk onderdeel een back-up maakt, en Bestemming wijzigen, Vervangen en Vrijgeven.](assets/screenshots/nodes/card-dark.png#only-dark)
![De kaart van de node vps-falkenstein op de pagina Instanties: Verbonden, zijn versie, zijn back-ups op de ontvangserver, wanneer elk onderdeel een back-up maakt, en Bestemming wijzigen, Vervangen en Vrijgeven.](assets/screenshots/nodes/card-light.png#only-light)

## Zo werkt het {#how-it-works}

- **De thuisinstantie bewaart de sleutel van de node.** Als je een node toevoegt, maakt de thuisinstantie zijn `APP_KEY`, houdt er een verzegelde kopie van en geeft hem jou één keer, in het compose-bestand van de node. Met die kopie bereikt de thuisinstantie de node en opent ze zijn back-ups.
- **De node heeft geen open poort nodig.** De thuisinstantie en de node praten via de relay over een eigen privéverbinding, los van je koppelgroep. De thuisinstantie begint elke aanroep en ondertekent hem, en de node vraagt haar om niets behalve zijn hallo. De enige bestemming die een open poort nodig heeft, is [de back-ups op de node bewaren](#keep-on-node).
- **Back-ups gaan nooit over de relay.** De node stuurt ze rechtstreeks naar de plek die je hebt gekozen: de ontvangserver van de thuisinstantie, een van haar bestemmingen of een rest-server op de node zelf.
- **De node plant zelf niets.** Hij heeft geen planningen, stuurt geen meldingen en sluit zich bij geen groep en geen vesting aan. De thuisinstantie start zijn back-ups, houdt bij hoe ze verlopen en stuurt de waarschuwingen.
- **Een node maakt back-ups van containers en mappensets.** VM's, de flash, ZFS-datasets en de zelf-back-up blijven bij volledige installaties.

De verbinding gebruikt de eigen relay van de thuisinstantie als haar groep er een heeft, en anders de relay van het project, ook als de thuisinstantie zonder relay koppelt.

## Voordat je begint {#before-you-start}

- **Een inlogwachtwoord op de thuisinstantie.** Een node toevoegen, zijn compose-bestand opnieuw tonen en zijn items openen vragen een aanmelding met het wachtwoord, zie [Inloggen](configuration.md#login).
- **Docker met Compose op de server van de node.**
- **Een plek voor de back-ups**, een van de drie onder [Waar de back-ups heen gaan](#placement). De ontvangserver is de veiligste keuze, omdat de node daar niets kan verwijderen.

## Een node toevoegen {#add}

1. Open op de thuisinstantie **Instanties** en kies rechtsonder **Node toevoegen**.
2. **Naam.** Typ hoe de node hier heet, bijvoorbeeld de naam van de server, en kies **Volgende**.

    ![Het venster Node toevoegen bij de eerste stap, met de naam vps-helsinki ingevuld.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Het venster Node toevoegen bij de eerste stap, met de naam vps-helsinki ingevuld.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Back-ups.** Kies waar de back-ups heen gaan en vul in wat die plek vraagt. **Maakt dagelijks back-ups om** is het tijdstip waarop beide onderdelen starten. De thuisinstantie stelt een half uur na haar eigen containerback-ups voor, zodat de twee niet tegelijk dezelfde schijven gebruiken. Later kun je elk onderdeel op de kaart een eigen planning geven. Kies **Volgende**.

    ![De tweede stap: de ontvangserver gekozen, Adres voor nodes ingevuld met een https-adres en Maakt dagelijks back-ups om 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![De tweede stap: de ontvangserver gekozen, Adres voor nodes ingevuld met een https-adres en Maakt dagelijks back-ups om 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Starten.** Het venster toont het compose-bestand van de node met `MODE`, `APP_KEY`, `NODE_HOME` en `NODE_NAME` al ingevuld. Kopieer het en bewaar de `APP_KEY` op een veilige plek. Op de server van de node:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Richt voor het starten het volume Host Data op de data waarvan je een back-up wilt en zet `HOST_SOURCE_ROOT` op hetzelfde pad, zoals het voorbeeld hieronder laat zien.

5. Het venster wacht op de node. Zodra de node antwoordt, vertelt de thuisinstantie hem waar zijn back-ups heen gaan en controleert ze of hij die plek bereikt, en het venster zegt **vps-helsinki is online en neemt opdrachten aan van deze instantie.**

    ![De laatste stap: het compose-bestand, daarna de node online en zijn back-ups op de ontvangserver.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![De laatste stap: het compose-bestand, daarna de node online en zijn back-ups op de ontvangserver.](assets/screenshots/nodes/add-start-light.png#only-light)

Je kunt het venster sluiten voordat de node antwoordt. Zijn kaart zegt dan **Wacht op de node**, en de thuisinstantie vertelt hem waar zijn back-ups heen gaan zodra hij zich meldt.

### Voorbeeld: een VPS die back-ups maakt naar de ontvangserver {#example-vps}

De thuisinstantie is een Unraid-machine met de naam `tower` die de [ontvangserver](offsite-recovery.md#receiving-server) draait. Een reverse proxy publiceert hem als `https://backup.tower.example.org`. De VPS `vps-helsinki` draait Nextcloud en Caddy, met hun data onder `/srv/appdata`.

1. Op `tower`: **Instanties**, **Node toevoegen**, naam `vps-helsinki`, **Volgende**.
2. Kies **Ontvangserver van deze instantie** en typ `https://backup.tower.example.org` onder **Adres voor nodes**. De containers van de thuisinstantie maken om 02:30 een back-up, dus het venster stelt 03:00 voor. Laat dat zo en kies **Volgende**.
3. Sla op de VPS het bestand uit het venster op, met de volumes en `HOST_SOURCE_ROOT` aangepast voor `/srv/appdata`:

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

4. `docker compose up -d`. Een paar seconden later zegt het venster op `tower` dat de node online is en **Back-ups gaan naar https://backup.tower.example.org.**
5. Om 03:00 start de thuisinstantie de back-ups van de node. De volgende ochtend toont de kaart **Nodes** in het overzicht de laatste back-up van `vps-helsinki`, en **Back-ups** toont zijn containers in een eigen onderdeel.

Hoe BombVault de data van een container op een gewone Docker-host vindt, staat onder [Detectie van back-upbronnen](configuration.md#backup-source-detection).

## Waar de back-ups heen gaan {#placement}

Elke node heeft één bestemming voor beide onderdelen. **Bestemming wijzigen** op zijn kaart past die aan zodra de node online is. De nieuwe plek krijgt nieuwe repository's, en de back-ups op de oude plek blijven waar ze zijn.

### Ontvangserver van deze instantie {#to-receiver}

De node schrijft in append-only-modus naar de ontvangserver van de thuisinstantie: hij kan back-ups toevoegen, maar er geen verwijderen. Hij krijgt daar een eigen login, genoemd naar de node, en één repository per onderdeel in zijn eigen map. De thuisinstantie voegt haar eigen sleutel aan elk repository toe en schoont ze zelf op, omdat de node dat niet kan.

Draait de thuisinstantie nog geen ontvangserver, dan zegt de stap dat en biedt **Ontvangserver instellen** aan. De thuisinstantie heeft daar geen groep voor nodig. Kies een map op een share en een poort, kies **Instellen en doorgaan** en daarna **Klaar** zodra het venster de server toont, en de stap gaat ermee verder. De details staan onder [Ontvangserver](offsite-recovery.md#receiving-server).

De node bereikt de server op het **Adres voor nodes**, dat je instelt bij de tweede stap of in het venster **Ontvangserver**. Een node verstuurt meestal vanaf het internet, dus het adres moet een van deze zijn:

- `https://`, bijvoorbeeld achter een reverse proxy, Pangolin of een tunnel. Toont de server een eigen certificaat in plaats van een certificaat van een publieke instantie, plak het dan in PEM-formaat onder **Certificaat**.
- `http://` met een privé-IP-adres, zoals `10.0.0.5` op een WireGuard-verbinding of `100.101.102.103` op Tailscale.

Gewoon `http://` naar een publiek adres wordt geweigerd, omdat de node zijn back-ups dan onversleuteld over het internet zou sturen. Wijzig je het adres, dan laat de thuisinstantie het elke node op de ontvanger weten, en een node die weg is, hoort het zodra hij terug is.

### Een bestemming {#to-location}

De node schrijft naar een bestemming van de thuisinstantie die hij zelf kan bereiken: een S3-dienst of een REST-server, via `https://` of op een privéadres. Zijn repository's staan daar in `bombvault-node-<node id>-containers` en `bombvault-node-<node id>-files`.

De node meldt zich aan met [opgeslagen inloggegevens](offsite-recovery.md#sign-ins) die voor hem zijn gemaakt, en die moeten verschillen van de inloggegevens waarmee de thuisinstantie naar de bestemming schrijft. Voeg ze eerst toe als het venster zegt dat er geen zijn. Met die inloggegevens kan de node meestal verwijderen wat er op de bestemming staat, anders dan op de ontvangserver, en het venster wijst je daarop.

### Op de node bewaren {#keep-on-node}

De node bewaart zijn back-ups zelf, achter een eigen rest-server in append-only-modus, en de thuisinstantie haalt van elk onderdeel een kopie op naar haar eigen opslag. Dat past bij een node waaraan de thuisinstantie geen plek kan bieden. Het is de enige bestemming die een open poort nodig heeft.

![De tweede stap met Op de node bewaren gekozen, het adres https://203.0.113.7:8443 en de opmerking over de open poort.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![De tweede stap met Op de node bewaren gekozen, het adres https://203.0.113.7:8443 en de opmerking over de open poort.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Typ onder **Adres van de node** waar de thuisinstantie de node bereikt: zijn publieke adres en een vrije poort, zoals `https://203.0.113.7:8443`, zonder map erachter. Gebruik `http://` alleen met een privéadres, zoals hierboven.
2. De node start de container `bombvault-node-rest-server` op die poort, met zijn repository's in de map `bombvault-node` onder Host Data. Met `https://` maakt hij een certificaat voor dat adres, en de thuisinstantie pint het vast.
3. Zet de poort open in de firewall van de node, zodat de thuisinstantie hem kan bereiken. Een poort die een andere container op de node al bezet, wordt geweigerd met de naam van die container.

De thuisinstantie haalt een onderdeel op direct nadat de back-ups van de node voor dat onderdeel klaar zijn, en als reserve elk uur om kwart over, ook als **Ophalen** uitstaat. Wat de thuisinstantie heeft opgehaald, blijft bij haar zoals elke [opgehaalde kopie](offsite-recovery.md#receiver), ook als de node weg is. De node schoont zijn eigen repository's op als de thuisinstantie daarom vraagt, omdat zijn rest-server geen verwijderingen aanneemt, en zijn mappensets slaan de map `bombvault-node` over, zodat hij nooit een back-up van zijn eigen back-ups maakt.

### Off-site-kopieën samen met die van de thuisinstantie {#copy-onward}

Zet **Off-site kopiëren met de back-ups van deze instantie** aan in de stap **Back-ups** of onder **Bestemming wijzigen**, en de thuisinstantie kopieert de containers en mappensets van de node naar de bestemmingen waar haar eigen Containers en Mappen naartoe kopiëren, volgens dezelfde planning. De node heeft hiervoor geen eigen map en geen eigen kopieerplanning nodig. Onder **Bestemming wijzigen** werkt de schakelaar meteen.

- De kopieën liggen in dezelfde repository's als de kopieën van de thuisinstantie, onder de host van de node. De regels van de thuisinstantie op de bestemming laten ze met rust, en de thuisinstantie bewaart ze volgens de regel van de node, dezelfde waarmee ze [de back-ups van de node opschoont](#schedule).
- De thuisinstantie leest de repository's van de node met haar eigen sleutel en schrijft met haar eigen inloggegevens voor de bestemming. De node krijgt geen nieuwe inloggegevens, en de ontvangserver blijft append-only. Bij een node die zijn back-ups zelf bewaart, kopieert de thuisinstantie wat ze heeft opgehaald.
- De repository's van de node hebben eigen chunker-instellingen, dus hun kopieën delen op de bestemming weinig gegevens met die van de thuisinstantie.
- Op een append-only-bestemming wordt niets opgeschoond, net als bij de eigen kopieën van de thuisinstantie. Een [gedeelde repository](offsite-recovery.md#shared-repository) krijgt geen kopieën van een node, omdat de eigenaar haar opschoont volgens de regels van de servers van de groep.
- De kaart toont **Kopieën gaan naar** en de bestemmingen. Een kopie die mislukt, is een fout op de kaart en stuurt een waarschuwing, zie [Je nodes in de gaten houden](#watch).
- Uitgeschakeld kopieert de thuisinstantie niets meer. De kopieën die er al zijn, blijven, en de regels van de thuisinstantie laten ze nog steeds met rust.

#### Voorbeeld: een VPS naar B2 gekopieerd met de containers van de thuisinstantie {#example-copy-onward}

`tower` bewaart onder **Hoe lang bewaren** 7 back-ups van elke container en kopieert zijn Containers elke nacht om 04:00 naar een B2-bucket. De bucket bewaart de laatste 14. `vps-helsinki` maakt back-ups naar de ontvangserver van `tower`, waar zijn back-ups worden opgeschoond volgens dezelfde 7. Met de schakelaar aan kopieert `tower` om 04:00 ook de containers van de VPS naar de bucket. De bucket bevat dan 14 back-ups van elke container van `tower` en 7 van elke container van de VPS.

## Wanneer nodes back-ups maken {#schedule}

De thuisinstantie beheert de planning van elke node. De kaart toont een planning per onderdeel, zoals **Containers** *Dagelijks om 2:00* en **Mappen** *Wekelijks (zo) om 4:00*. Een klik opent dezelfde editor als onder [Planningen](schedules.md#plans), en een wijziging is meteen opgeslagen.

![De planning voor de containers van een node, geopend vanaf zijn kaart, met Uit, Dagelijks, Wekelijks, Elke N dagen en Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![De planning voor de containers van een node, geopend vanaf zijn kaart, met Uit, Dagelijks, Wekelijks, Elke N dagen en Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Een node die weg was**, start de gemiste back-ups zodra hij terug is. De thuisinstantie kijkt elke vijf minuten.
- **Een node die bezig is**, krijgt vijf minuten later opnieuw de vraag.
- **Terwijl de thuisinstantie** de oude back-ups van een node opschoont, start de node niets tot ze klaar is. Op de ontvangserver en op de node zelf gaat dat venster elke zondag om 03:00 open en duurt het tot zes uur. Op een bestemming volgt het de planning voor off-site-kopieën die de thuisinstantie voor die bestemming heeft. De thuisinstantie bewaart de back-ups volgens haar eigen regel voor het onderdeel onder **Hoe lang bewaren**, of volgens de regel van de bestemming als die er een heeft.
- **In de lijst met runs van de node** staan de back-ups en herstelacties die de thuisinstantie heeft gestart, als gestart door de thuisinstantie.

## Je nodes in de gaten houden {#watch}

De kaart **Nodes** in het overzicht toont elke node met zijn status, zijn laatste back-up, zijn fouten en zijn openstaande anomalieën. Ze verschijnt zodra je een node hebt.

![De kaart Nodes in het overzicht met drie nodes: een verbonden, een inactief met twee fouten en een die nog op de node wacht.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![De kaart Nodes in het overzicht met drie nodes: een verbonden, een inactief met twee fouten en een die nog op de node wacht.](assets/screenshots/nodes/overview-card-light.png#only-light)

De thuisinstantie stuurt één waarschuwing per voorval via haar [meldingskanalen](notifications.md), en de volgende pas als dat voorval voorbij is:

- **Een back-up is mislukt**, bijvoorbeeld *BombVault: vps-helsinki: backup failed*, met het item en de reden. Een latere back-up van dat item die slaagt, sluit het voorval af.
- **De node is onbereikbaar**, bijvoorbeeld *BombVault: vps-helsinki is out of reach*. De thuisinstantie wacht zo lang als de kortste dodemansknop op de repository's van de node, 26 uur tenzij je dat wijzigt, of 6 uur zolang de node nog geen bestemming heeft.
- **Een off-site-kopie is mislukt**, bijvoorbeeld *BombVault: vps-helsinki: off-site copy failed*, als de thuisinstantie de back-ups van de node niet naar een van haar bestemmingen kon kopiëren. De volgende kopie die slaagt, sluit het voorval af.

De ontvangerbewaking van de thuisinstantie houdt ook elk repository van een node in de gaten, met een dodemansknop en een dagelijkse `restic check` om 04:00, zie [Ontvanger en ophaler](offsite-recovery.md#receiver).

## De items van een node {#entries}

Onder **Back-ups** hebben de lijsten Containers en Mappen onder je eigen items een onderdeel voor elke node. De eigen container van de node staat er niet bij. **Nu back-up maken** in het menu van een item start zijn back-up op de node, en **Openen** opent zijn pagina.

In het onderdeel van een node in de lijst Mappen voegt **Mappenset toevoegen** een mappenset op de node toe, en **Bewerken** in het menu van een set wijzigt die. Beide openen hetzelfde venster als voor je eigen sets. De mappenkiezer daarin bladert door de mappen van de node, maar maakt daar geen nieuwe mappen aan.

![Het onderdeel van de node vps-falkenstein in de lijst Containers: nextcloud, nextcloud-db, caddy en uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Het onderdeel van de node vps-falkenstein in de lijst Containers: nextcloud, nextcloud-db, caddy en uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

De pagina van een item op een node zegt **Op** en de naam van de node, en toont zijn back-ups. **Herstellen** bij een daarvan herstelt het item op de node en vervangt wat er nu staat. Een container wordt daarvoor gestopt, net als op de thuisinstantie.

![De pagina van nextcloud op vps-falkenstein met zijn back-ups en een knop Herstellen naast elke back-up.](assets/screenshots/nodes/entry-dark.png#only-dark)
![De pagina van nextcloud op vps-falkenstein met zijn back-ups en een knop Herstellen naast elke back-up.](assets/screenshots/nodes/entry-light.png#only-light)

Via de thuisinstantie bereik je een vaste lijst van routes van de node zelf: zijn containers, mappensets, runs en anomalieën, en het back-uppen en herstellen daarvan. De instellingen, sleutels en opslag van de node blijven buiten bereik, en niets op die lijst verwijdert iets. Zie [Nodes in de API](api.md#nodes).

## Vervangen en vrijgeven {#replace-release}

**Vervangen** toont het compose-bestand van de node opnieuw, na een aanmelding met het wachtwoord. Gebruik het om de node naar een andere server te verhuizen of opnieuw op te bouwen: start het bestand daar en zet de oude server uit. Dezelfde `APP_KEY` en dezelfde `NODE_HOME` maken hem weer dezelfde node, met zijn back-ups en zijn geschiedenis. De thuisinstantie bewaart van elke node de mappensets, de containers met wat nodig is om ze opnieuw te maken, en de runs. Start de node met een lege map `config`, dan neemt de thuisinstantie hem weer op, vertelt hem waar zijn back-ups heen gaan en geeft hem dat allemaal terug vóór de volgende back-up, zodat de node een container kan herstellen die er niet meer is. Op een nieuwe server begint een node die zijn back-ups zelf bewaart met lege repository's. Zijn eerdere back-ups blijven in de kopieën die de thuisinstantie heeft opgehaald. Twee servers met dezelfde `APP_KEY` tellen als één node, laat ze dus nooit allebei draaien.

**Vrijgeven** laat een node gaan. Hij neemt geen opdrachten meer aan van de thuisinstantie en maakt geen back-ups meer. Zijn back-ups blijven waar ze zijn, net als elke kopie die de thuisinstantie heeft opgehaald. Zijn login op de ontvangserver verdwijnt, en de thuisinstantie houdt zijn repository's niet meer in de gaten. Een node die weg is als je hem vrijgeeft, houdt zijn kaart als **Wordt vrijgegeven** tot hij het hoort. Een vrijgegeven node neemt met dezelfde `APP_KEY` geen thuisinstantie meer aan. Wil je de server weer gebruiken, voeg hem dan toe als nieuwe node en start hem met het nieuwe compose-bestand en een lege map `config`.

## Beveiliging {#security}

Wordt de thuisinstantie overgenomen, dan beheerst de aanvaller ook haar nodes, en een node heeft Docker op zijn server in handen, en dus root. Bescherm de thuisinstantie met een wachtwoord en houd haar van het internet af. Wordt een node overgenomen, dan krijgt de aanvaller geen koppelgroep, geen vesting, geen andere node en geen ander repository, en kan hij op de ontvangserver geen geschiedenis wissen. De details staan onder [Nodes in het beveiligingsmodel](configuration.md#nodes).

De [herstelkit](offsite-recovery.md#recovery-kit) heeft een hoofdstuk **Nodes** met de `APP_KEY` van elke node, zijn restic-wachtwoord en zijn repository's, zodat je een node kunt terughalen of zijn back-ups met alleen restic kunt herstellen.
