# Instellingen

**Instellingen** zijn veertien tegels. Elke tegel is een pagina met kaarten, en elke wijziging wordt opgeslagen terwijl je haar maakt, met een korte flits op het bedieningselement, of een schudding als de server haar weigert. Een paar vensters houden een knop **Opslaan**, omdat half opslaan onveilig zou zijn, zoals bij inloggegevens of het inlogwachtwoord. Uitleg staat in de (i)-bubbels naast elk bedieningselement.

Druk overal in de instellingen op `/` of Ctrl+K om ze allemaal te doorzoeken, ook op de namen die pagina's in eerdere versies hadden. De volgorde van de tegels kan worden gewijzigd onder **Uiterlijk**. Waar een kolom met tegels naast de pagina haar zou samenknijpen, zoals in een venster van ongeveer 1200 pixels breed of op een telefoon, worden de tegels een strook boven de pagina die zijwaarts scrolt.

![Instellingen met de tegel Algemeen geopend.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Instellingen met de tegel Algemeen geopend.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Deze pagina toont elke tegel en zijn kaarten, en linkt naar de pagina die elk ervan diepgaand uitlegt.

## Algemeen {#general}

- **Onderdelen** schakelt elk onderdeel in of uit: **Containers**, **VM's**, **Flash**, **Mappen**, **ZFS-datasets** en **Zelf-back-up**. Een onderdeel dat uitstaat, wordt volgens geen enkele planning geback-upt, ook niet volgens de eigen planning van een item, en zijn items tellen niet mee. **Nu back-up maken** werkt nog steeds. **Flash** verschijnt alleen op Unraid.
- **Containers** bevat wat voor elke container geldt, zoals **Automatische databasedumps**. Alles wat één container raakt, staat op zijn pagina. Zie [Databasedumps](backups.md#database-dumps).

## Uiterlijk {#look}

- **Uiterlijk**: **Taal** (42 talen), **Thema** (**Donker**, **Licht** of **Systeem**), **Hoeken**, **Animatie** (**Uit**, **Subtiel**, **Wild** of **Razend**), **Stille meldingen**, die succesmeldingen zoals bevestigingen van opslaan verbergt, en **Volgorde van de instellingenpagina's**, waar je een tegel vasthoudt en op zijn plek sleept, of Alt en de pijltjestoetsen gebruikt.
- **Labels**: hoeveel van een bedieningselement wordt getoond, apart voor **Knoppen**, **Zijbalk**, **Tabbladen** en **Onderbalk**: **Tekst**, **Tekst en symbool**, **Symbool**, of **Reactief**, dat alleen het symbool toont en de woorden inschuift als je erop wijst. **Zijbalk** geldt alleen voor de desktopindeling en **Onderbalk** alleen voor de telefoonindeling.
- **Kleuren**: één **Accentkleur**, of **Regenboogmodus**, waarin elke rij van een lijst een eigen kleur uit een palet van acht krijgt. De Regenboogmodus voegt **Discomodus** toe, waarin de kleuren langzaam naar de volgende glijden, **Reactieve modus**, waarin de kleur van een rij alleen zichtbaar is terwijl je erop wijst, hij loopt of is geselecteerd, **Kleurrotatie** en het **Kleurenpalet** zelf.

BombVault houdt het uiterlijk op de server. Een browser die voor het eerst komt, neemt het over. Vraagt je systeem om minder beweging, dan wint dat altijd van **Animatie**.

## Planning {#schedule}

**Week**, **Volledige back-up** met **Wanneer back-ups draaien**, **Gemiste back-ups na de start inhalen** en **Opdrachten ervoor en erna**, **Planningen per onderdeel** met **Off-site-kopieën per onderdeel**, **Geplande items**, **Volgorde** met **Herstarten na back-up**, **Snapshot in plaats van stoppen**, **Updates na de back-up** en **Rust voor de back-up**. Zie [Planningen](schedules.md).

## Bestemmingen {#storage-locations}

- **Ook in de zijbalk tonen** zet de pagina Bestemmingen vast in de zijbalk.
- De lijst met je bestemmingen, met **Bestemming toevoegen**. Een klik opent de pagina van een bestemming, zie [De pagina van een bestemming](offsite-recovery.md#location-page).
- **Inloggegevens**: de opgeslagen inloggegevens met de standaardster, zie [Inloggegevens](offsite-recovery.md#sign-ins).
- **Bestemmingen per onderdeel**: de 3-2-1-stroom, waarin elke lijn een schakelaar is, zie [Standaarden per onderdeel](offsite-recovery.md#placement-defaults).
- **Repository's**: repository's die voor losse items zijn gemaakt, zie [Plaatsing per item](offsite-recovery.md#placement).
- **Hoe lang bewaren** wijst naar de bestemmingen, waar het bewaarbeleid wordt ingesteld, en **Voorbeeld** toont **Wat de volgende uitvoering zou verwijderen**, per repository en per item, voordat het gebeurt. Het verandert niets en neemt geen lock, dus het antwoordt ook terwijl er een back-up loopt.
- **Offsite-kopieën**: één rij per onderdeel met **Verbinding testen**, **Nu repliceren** en **Instellen** of **Bewerken**, zie [Off-site-kopieën](offsite-recovery.md#off-site-copies).
- **Off-site bandbreedte**: **Uploadlimiet (KiB/s)** en **Downloadlimiet (KiB/s)** voor elke kopie. 0 betekent onbeperkt.

De kaart **Streaming**, die de uploadlimiet verlaagt terwijl Plex, Jellyfin of Emby streamen, staat op de pagina Bestemmingen zelf.

## Herstel {#restore}

- Een link naar de pagina **Herstel**.
- **Herstelmappen**: de **Standaard herstelmap** voor een herstel naar een map, met **Submap per onderdeel** en een **Uitzondering** per onderdeel. Zie [Waar een herstel naar een map terechtkomt](restore.md#folders).
- **Container-registry's**: inloggegevens voor privéregistry's, zoals een `ghcr.io`-image achter een sponsorpoort. Een herstel gebruikt ze om een ontbrekende image op te halen, en een update na de back-up om een nieuwere te zoeken. Openbare images worden zonder opgehaald. **Registry toevoegen** biedt openbare registry's, zelfgehoste, die van een cloudprovider en **Andere registry** voor elk adres.
- **Voor noodgevallen**: **Herstelkit downloaden**, wat een inlogwachtwoord vereist. Zie [Herstelkit](offsite-recovery.md#recovery-kit).

## Meldingen {#notifications}

**Meldingen** met **Melden**, de samenvattingen, **Achterstallige back-ups melden**, **Unraid-meldingen** en de **Wekelijkse samenvatting**, **Kanalen** met Webhook, Apprise, Matrix, Gotify en E-mail (SMTP), en **Healthchecks**. Zie [Meldingen](notifications.md).

## Verificatie {#verify}

- **Back-ups controleren**: het resultaat van de laatste `restic check` per onderdeel, met **Ontgrendelen** en **Opschonen** onder **Meer** in elke rij. **Ontgrendelen** wist een verouderde lock, en **Opschonen** past de bewaarregels op verzoek toe.
- **Automatische controles**: de geplande herstelcontrole met zijn **Datasteekproef (%)** en testdoelen, **Starttest**, die ook één container in een geïsoleerde kopie herstelt en start, de **Geplande off-site DR-test** en **Verwijderbeveiliging controleren**, de geplande sabotagetest. Zie [DR-oefeningen](offsite-recovery.md#dr-drills) en [Sabotagetest](offsite-recovery.md#tamper-test).
- **Anomalieën**: detectie, gevoeligheid en welke bevindingen een bericht sturen. Zie [Anomalieën](anomalies.md).

## Beveiliging {#security}

- **Wachtwoord**: het inlogwachtwoord, minstens 12 tekens. Wijzigen of wissen vraagt om het huidige en, met tweefactorauthenticatie aan, om de code.
- **Tweefactorauthenticatie**: een code uit een authenticator-app bovenop het wachtwoord, met acht herstelcodes die één keer worden getoond.
- **Passkeys**: meld je aan met een sleutel op je telefoon, laptop of beveiligingsstick. Ze hebben een echte domeinnaam en een certificaat nodig dat de browser vertrouwt.
- **Android-app**: **Telefoons kunnen koppelen met de twaalf woorden van de groep**, waarmee een gekoppelde telefoon zich zonder het wachtwoord kan aanmelden.
- **Versleuteling**: **Back-ups versleutelen**, standaard aan.

Een aantal dingen vraagt om een inlogwachtwoord, zoals de herstelkit en bewaarkopieën van de sleutel. Zie [Beveiligingsmodel](configuration.md#security-model).

## Instanties {#instances}

- **Ook in de zijbalk tonen** zet de pagina Instanties vast in de zijbalk.
- **Meerdere servers** schakelt de delen van een groep in: **Instanties**, de pagina met een kaart voor elk lid, **Ontvanger** en **Ophalen**. Zie [Instanties en rollen](offsite-recovery.md#roles).
- Een link naar de pagina Instanties, met de delen die aanstaan.

## Koppeling {#pairing}

**Koppeling** met **Frase genereren** of **Frase invoeren**, de stapkaarten, de naam van deze instantie en **Bekende adressen**, de kaart **Relay** met zijn vier vormen en **Wat de relay ziet**, de leden van de groep, en **ZFS-server zonder BombVault** voor een replicadoel via SSH. Zie [Instanties koppelen](offsite-recovery.md#pairing) en [Naar een ZFS-server pushen](zfs-datasets.md#replica-push).

## Integraties {#integrations}

- **MCP-server**: sleutels voor AI-assistenten, **Kies een client** en het logboek van elke sleutel. Zie [MCP-server](mcp.md).
- **API-tokens**: tokens met een naam voor scripts en dashboards. Zie [Tokens](api.md#tokens).
- **Diensten**: **Home Assistant** via MQTT, **Vinden op het netwerk** (mDNS als `bombvault.local`) en **/metrics beschikbaar maken** voor Prometheus, met een **Scrape-token (optioneel)**. Zie [Home Assistant](api.md#home-assistant) en [BombVault vinden op het netwerk](api.md#mdns).
- **Dashboardwidget**: een token om alleen te lezen en een adres voor elk dashboard dat een iframe toont, zoals Homepage of Heimdall, met alleen het live activiteitenlogboek. Zie [Dashboardwidget](api.md#dashboard-widget).
- **Host-SSH**: de publieke sleutel van BombVault en **Verbinding testen**, voor back-ups van VM's en ZFS-datasets en Unraid-meldingen. Zie de [VM-back-upgids](vm-backup-ssh-setup.md).

## Apps {#apps}

- **Android-app**: de APK voor de release die je server draait met een QR-code, en de vermelding op F-Droid. Zie [Android-app](android.md#install).
- **ParleyPort**: de relaycontainer met zijn Unraid-template en de Docker-opdracht die hem start.
- **BombVault Widget**: een tegel op het Unraid-dashboard met het activiteitenlogboek van BombVault en de volgende run. Met een host-SSH-verbinding installeert en verwijdert de kaart de plug-in zelf. Zonder geeft hij je het `.plg`-adres om te installeren onder **Plugins, Install Plugin**.

## Systeem {#system}

- **Instellingen exporteren / importeren**: je hele configuratie als draagbaar JSON-bestand, met of zonder de inloggegevens. Import toont eerst **Dit bestand bevat** en laat verwijzingen naar bestemmingen die weg zijn weg. **Exports versleutelen met age** verzegelt de gewone exports, de flash-ZIP en de herstelkit voor de **age-ontvangers** die je opgeeft. Zie [Draagbare instellingen](configuration.md#portable-settings-export-and-import).
- **Prestaties**: **CPU-threads (0 = alle)**, **Cachegroottelimiet (MB, 0 = onbeperkt)** voor de cache van restic onder `/config`, en **Geheugenlimiet per back-up (MiB, 0 = geen)**, een zachte limiet waardoor een grote back-up vertraagt in plaats van te worden gedood omdat het geheugen opraakt.

## Info {#info}

- **Versie**: BombVault, GlimStone, restic en rclone, met **Controleren op updates** en **Nieuw**, dat telt hoeveel wijzigingen je nog niet hebt gezien en een venster opent dat ze toont.
- **Help**: **Handleiding** opent deze documentatie, en **Bugrapport** downloadt een geredigeerde ZIP voor een GitHub-issue. Het vereist een inlogwachtwoord.
- **Over BombVault**: het project, zijn licentie en hoe je de auteur bereikt.
