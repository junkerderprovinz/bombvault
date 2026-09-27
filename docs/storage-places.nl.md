# Opslagplekken

Een opslagplek is een plek waar BombVault back-ups bewaart: een map op deze Unraid, een share op een NAS, een bucket bij een cloudprovider, een rest-server, een SFTP-account of een Nextcloud. Je verbindt elke plek één keer, op **Instellingen, Opslag**, en de inloggegevens, retentie, bescherming en locatie horen bij die plek. De vijf domeinen (containers, VM's, de flash, BombVaults eigen configuratie en bestandssets) kiezen daarna uit de plekken: waar elk domein wordt opgeslagen en waarheen het wordt gekopieerd.

## Een plek toevoegen {#add-a-place}

**Plek toevoegen** opent een venster met één tegel per provider, in drie groepen: cloudopslag, zelf gehoste diensten, en NAS-apparaten en deze server.

1. Kies een tegel en vul het formulier in. De knop met het oog toont een geheim dat je hebt ingetypt.
2. **Verbinding testen** controleert de plek en maakt niets aan. Voor de map van elk domein meldt de test wat hij aantrof: leeg of nog niet aanwezig, al een restic-repository, of de fout waarop hij stuitte.
3. Geef de plek een naam; de naam van de provider is al ingevuld. Beantwoord bij een apparaat dat je zelf beheert **Waar staat het apparaat?**. Cloudproviders staan altijd op een andere locatie, en een map op deze Unraid staat altijd hier.
4. **Toevoegen** slaat de plek op.

Een nieuwe plek wordt nog door geen enkel domein gebruikt. Kies de plek onder **Opgeslagen in** of **Gekopieerd naar** op de [kaart Domeinen](#domains), of op de kaart van een item voor alleen dat item.

## Mappen {#folders}

Een plek heeft één map per domein: `container`, `vms`, `flash`, `config` en `files`, dezelfde namen als de standaardlocaties voor back-ups. De mappen staan in de details van de plek en kunnen daar worden hernoemd (zie [Een adres wijzigen](#addresses)). Een domein zonder map op een plek kan die plek niet kiezen.

Gebruikt een domein een plek in beide rollen, dan krijgt de tweede rol een achtervoegsel en houdt de eerste zijn map. Een plek die al de kopieën van een domein ontvangt, bewaart de items die er rechtstreeks naartoe worden gestuurd in `<folder>-direct`; een plek waar een domein al is opgeslagen, ontvangt de kopieën ervan in `<folder>-copies`.

Sommige plekken zijn zelf een restic-repository: een adres waar al een repository stond toen de plek werd toegevoegd, een benoemde repository uit een bestaande setup, of een kopieerdoel in de hoofdmap van een bucket. Zo'n plek heeft geen mappen, alle domeinen delen die ene repository, en de plek neemt geen tweede rol op zich. Wil je meer opslaan bij dezelfde provider, verbind dan een andere bucket of map als eigen plek.

## Details van een plek {#details}

Elke plek is een rij met de provider, waarvoor de plek wordt gebruikt en de laatste test of kopie. **Testen** controleert elk adres op de plek, en **Details** opent de instellingen ervan. Elke wijziging in de details wordt meteen opgeslagen.

- **Algemeen**: de naam, de schakelaar die de plek aan- en uitzet, het adres en, bij een apparaat dat je zelf beheert, **Waar staat het apparaat?** (zie [Buiten het pand](#off-the-premises)).
- **Bewaarbeleid**: keep-last, dagelijks, wekelijks en maandelijks, voor elke repository op de plek. Een nieuwe plek begint met de standaardregels; een plek waar elke regel op nul staat, trimt nooit.
- **Bescherming**: de schakelaar **Append-only**. De andere kant moet append-only afdwingen; staat de schakelaar aan, dan schoont BombVault daar nooit iets op en verwijdert het er nooit iets. Bij een rest-server met append-only aan voert **Op append-only testen** de tamper-test uit tegen elk domeinpad, elke ingeschakelde kopie en elke repository op de plek, en toont één antwoord voor de hele plek: *verwijderen geweigerd* of *verwijderen toegestaan* (zie [Off-site en herstel](offsite-recovery.md)). Alleen externe plekken hebben dit onderdeel, omdat niets op deze machine kan voorkomen dat een lokale repository wordt verwijderd.
- **Toegang**: de inloggegevens en, bij S3, de opslagklasse. Een plek die de gedeelde inloggegevens gebruikt, krijgt bij de eerste wijziging een eigen set. Een directe repository op de plek die de nieuwe inloggegevens niet kunnen openen, houdt de oude, en het antwoord meldt dat. Map-, SFTP- en rclone-plekken hebben dit onderdeel niet.
- **Limieten**: de upload- en downloadsnelheid en het groeibudget.
- **Mappen**: één schakelaar per domein, met de naam van de map. Een domein dat hier is uitgeschakeld, kan de plek niet kiezen.

De retentie verlagen vraagt eerst om bevestiging en zegt hoeveel items dat raakt; append-only uitschakelen vraagt eerst om bevestiging en zegt hoeveel repository's op de plek die bescherming verliezen. Een plek uitschakelen schakelt elke repository op die plek uit; een plek waarin een domein is opgeslagen, kan niet worden uitgeschakeld.

## De kaart Domeinen {#domains}

De kaart heeft één rij per domein, met de planning, waar het domein is opgeslagen, waarheen het wordt gekopieerd en de uitzonderingen.

- **Opgeslagen in**: zolang de back-uplocatie van het domein geen back-ups bevat, wordt de gekozen plek de thuisplek van het domein en verhuist de locatie daarheen. Staan er al back-ups, dan wordt de keuze bij containers, VM's en bestandssets de standaard voor nieuwe items, die hem bij hun eerste back-up overnemen; items die al back-ups hebben, blijven waar ze zijn, omdat BombVault nooit een back-up verplaatst. Bij de flash en BombVaults eigen configuratie verhuist de thuisplek, en de back-ups die al zijn geschreven, blijven op de oude plek.
- **Gekopieerd naar**: één chip per plek die de kopieën van het domein kan opnemen. Een chip aanvinken maakt de plek een kopieerdoel van het domein; de eerste keer zegt BombVault vooraf hoeveel items en snapshots en hoeveel data de eerste run verstuurt. Uitvinken stopt nieuwe kopieën: de kopieën die er al staan, blijven en verouderen volgens de retentie van de plek, en items met een eigen keuze blijven daarheen kopiëren. De laatste chip uitvinken stopt alle kopieën, ook naar plekken die later worden toegevoegd, tot er weer een is aangevinkt. Een uitgeschakelde plek verschijnt als gedimde chip en kan niet worden gekozen.
- **Uitzonderingen**: de items met een eigen keuze, als lijst met links naar hun kaarten.
- **Nu kopiëren** voert de kopieën van het domein meteen uit.

Een domein dat na een rebuild via Ontdekken is gepauzeerd, toont de pauze op zijn rij, met **Standaard bevestigen** (zie [Plaatsing per item](offsite-recovery.md#placement)).

## Een adres wijzigen {#addresses}

De map van een domein kan in de details van de plek worden gewijzigd, en dat geldt ook voor het adres van een lokale plek, bijvoorbeeld nadat een repository met de hand naar een andere schijf is verplaatst. BombVault test elk adres dat de wijziging raakt en accepteert de wijziging wanneer elk nieuw adres leeg is en op het oude niets was opgeslagen, of wanneer elk nieuw adres dezelfde restic-repository bevat als het oude. Al het andere wordt geweigerd, met het aantal back-ups dat nog op het oude adres staat. Een externe plek houdt zijn adres; wil je ergens anders naartoe back-uppen, verbind dat dan als een eigen plek.

BombVault stelt de lijst met plekken samen uit zijn eigen database en leest daarvoor nooit een externe repository uit; de test draait alleen wanneer je iets wijzigt.

## Een plek verwijderen {#remove}

Een plek kan alleen worden verwijderd zolang niets de plek gebruikt: er is daar geen domein opgeslagen, geen standaard wijst ernaar, er is daar geen item opgeslagen en geen directe repository op de plek bevat items. Anders somt de weigering op wat de plek vasthoudt. Met de plek verdwijnen ook de kopieerdoelen ervan, en de eigen inloggegevens, tenzij een ophaalbron of een andere plek ze gebruikt. In de opslag zelf wordt niets verwijderd, en de bevestiging zegt hoeveel kopieën daar achterblijven.

## Zonder plek {#without-a-place}

Een adres dat niet past in de vorm van een plek plus een map, blijft werken en staat met zijn adres onder **Zonder plek**. Native `b2:`-, `gs:`- en `swift:`-adressen horen daarbij. **Aan plek toewijzen** koppelt zo'n rij aan een plek, na dezelfde test als bij [een adres wijzigen](#addresses). Een kopieerdoel zonder plek staat ook op de rij van zijn domein, naast de chips, en blijft kopiëren. Een externe rij daar heeft een eigen schakelaar **Append-only**, en die uitschakelen vraagt eerst om bevestiging met het aantal items dat back-ups op dat adres bewaart. Een directe repository volgt de schakelaar van zijn doel.

## Buiten het pand {#off-the-premises}

**Waar staat het apparaat?** heeft twee antwoorden: **Hier in huis** en **Op een andere locatie**. Een kopie telt alleen als eigen locatie, voor de 3-2-1-regel op de kaarten en voor de off-site controles van het Dashboard, wanneer de plek ervan op een andere locatie staat. Een tweede schijf of een NAS in hetzelfde huis is een tweede kopie, geen tweede locatie. Het antwoord verandert geen kopie. Cloudproviders staan altijd op een andere locatie en een map op deze Unraid altijd hier, dus het formulier vraagt er bij hen niet naar; bij elke andere plek wijzig je het antwoord in de details. Een plek op een andere locatie draagt op zijn rij de markering **Andere locatie**.

## Verbindingssoorten

### Map op deze Unraid of een NAS {#kind-local}

Het adres is een pad onder `/mnt`, geschreven zonder `/mnt`, bijvoorbeeld `user/bombvault`, en de map van elk domein staat daaronder: `user/bombvault/container`.

- **Map op deze Unraid** kiest uit de shares, schijven en pools.
- **Synology**, **QNAP**, **TrueNAS**, **Andere Unraid** en **Andere share** kiezen uit `/mnt/remotes`. Mount de share eerst in Unraid, bijvoorbeeld met de plugin Unassigned Devices. Host Data moet als Read/Write - Slave zijn gemount, anders blijft een share die pas na de start van BombVault wordt gemount onzichtbaar tot een herstart (zie [Configuratie](configuration.md)).

De mappenkiezer maakt met **Nieuwe map** een map aan op de plek waar hij staat. De test controleert of de map leeg is of ontbreekt en of BombVault er kan schrijven.

### S3 {#kind-s3}

Het adres is `s3:https://<endpoint>/<bucket>/<path>`, bijvoorbeeld `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** heeft alleen de sleutel-ID en de applicatiesleutel nodig. BombVault vraagt B2 tot welke bucket, welk S3-eindpunt en welke map de sleutel beperkt is en bouwt daar het adres mee op. Een sleutel die elke bucket mag bereiken, biedt zijn buckets aan om uit te kiezen.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** en **Vultr** vragen om de sleutel en, waar de provider dat nodig heeft, om de regio, het account-ID of het eindpunt. BombVault vult het eindpunt in en toont de buckets als de sleutel ze mag opsommen; typ anders de naam van de bucket.
- **Google Cloud Storage** loopt via de S3-interface met een HMAC-sleutel, die je in de instellingen van Cloud Storage onder Interoperabiliteit aanmaakt. Een bestand van een serviceaccount werkt hier niet.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** en **Andere S3-dienst** vragen om het adres van de dienst en een sleutel.

De opslagklasse stel je in de details van de plek in, beperkt tot tiers die een herstel kan lezen zonder eerst een thaw.

### rest-server {#kind-rest}

Het adres is `rest:<url>/<user>`, bijvoorbeeld `rest:https://nas.lan:8000/tower`. Het formulier vraagt om het adres van de server, een gebruiker en een wachtwoord. Met `--private-repos` mag een gebruiker alleen paden bereiken die met zijn eigen naam beginnen, dus zet BombVault de gebruiker vooraan, tenzij je een ander pad typt. Weigert de server een pad buiten dat van de gebruiker, dan zegt de foutmelding dat.

Het rest-server-formulier bevat een kant-en-klaar recept voor een rest-server in append-only-modus met één gebruiker voor deze BombVault. **Recept tonen** maakt een wachtwoord aan dat één keer wordt getoond, en geeft een `docker run`-regel, een compose-bestand en een Unraid-sjabloon, elk met de `htpasswd`-regel voor de server; de gebruiker en het wachtwoord komen meteen in het formulier.

**Andere BombVault** toont boven zijn eigen velden de openstaande aanbiedingen die andere instanties via Vloot hebben gestuurd. Een aanbieding accepteren voegt een plek toe die alleen kopieën van het aangeboden domein bewaart, omdat een aanbieding een gebruiker voor precies dat ene domein meebrengt. Accepteren op de pagina Vloot voegt dezelfde plek toe.

### SFTP {#kind-sftp}

Het adres is `sftp://<user>@<host>:<port>/<path>`, bijvoorbeeld `sftp://bv@backup.lan:22/bombvault`. Het formulier vraagt om host, poort en gebruiker en toont de publieke sleutel van BombVault. Voeg die sleutel op de server toe aan de `~/.ssh/authorized_keys` van de gebruiker; verder hoeft daar niets te worden geïnstalleerd. BombVault accepteert de host key van de server bij het eerste contact en controleert hem vanaf dan.

**Hetzner Storage Box** vult `<user>.your-storagebox.de` en poort 23 in. Installeer de sleutel op de box met het eigen commando van Hetzner, dat één keer om het wachtwoord van de box vraagt:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Het formulier vraagt om het adres van de server, de gebruiker en een app-wachtwoord. Maak het app-wachtwoord aan in de beveiligingsinstellingen van het account, en vul de gebruikers-ID in, geen e-mailadres. BombVault bouwt het WebDAV-pad dat het product gebruikt en geeft de verbinding aan restic door via de omgevingsvariabelen van rclone, met het wachtwoord in de versluierde vorm van rclone. Het adres luidt `rclone:bvp<id>:<path>`, waarbij `bvp<id>` een remote is die alleen in die omgeving bestaat; er wordt niets naar de rclone-config geschreven.

### Azure Blob {#kind-azure}

Het adres is `azure:<container>:/<path>`. Het formulier vraagt om het opslagaccount en de toegangssleutel ervan; na **Verbinding testen** toont het de containers van het account om uit te kiezen, of je typt de naam van een container. BombVault geeft het account en de sleutel aan restic door als `AZURE_ACCOUNT_NAME` en `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Het adres is `rclone:<remote>:<path>`. Het formulier toont de remotes uit de rclone-config van BombVault om uit te kiezen. Wil je die config vervangen, plak dan een volledige `rclone.conf` onder **rclone-configuratie** en klik op **Configuratie opslaan**. De config wordt meteen opgeslagen en geldt voor elke rclone-plek, of het venster daarna een plek toevoegt of niet.

## Kopieën tussen plekken met verschillende inloggegevens {#different-credentials}

Een domein dat op een externe plek is opgeslagen, is de bron van zijn kopieën. `restic copy` draait met één omgeving, en BombVault voegt de inloggegevens van de bron toe aan die van het doel zolang die twee niet dezelfde variabele op verschillende waarden zetten. Een Nextcloud-plek en een B2-plek gebruiken verschillende variabelen, dus een domein dat in Nextcloud is opgeslagen, kan naar B2 worden gekopieerd. Twee S3-accounts of twee rest-server-gebruikers zouden dezelfde variabelen met verschillende waarden nodig hebben; restic kan die niet allebei aannemen, en de chip op de kaart Domeinen zegt dat de inloggegevens niet bij elkaar passen.
