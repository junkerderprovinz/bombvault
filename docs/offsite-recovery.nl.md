# Off-site en herstel

Lokale back-ups beschermen je tegen een verloren container of een slechte update. Off-site replicatie en een geteste herstelkit beschermen je tegen de hele machine, ransomware of een brand. Deze pagina behandelt off-site repliceren, die kopie manipulatiebestendig maken, bewijzen dat je kunt herstellen, en herstellen wanneer BombVault zelf weg is.

## Off-site replicatie

Houd de snelle lokale back-up en voeg een of meer off-site replica's toe. Stel een repo per domein in op het tabblad **Instellingen, Off-site**. BombVault repliceert nieuwe snapshots daarheen met `restic copy` op best-effort-basis, zodat een off-site hapering de lokale back-up nooit laat mislukken. De lokale repo blijft primair.

- **Meerdere off-site doelen per domein.** Elk domein (containers, VM's, flash, config en bestandssets) kan tegelijk naar meerdere off-site bestemmingen repliceren, niet slechts één, zodat je bijvoorbeeld een rest-server op de machine van een vriend en een S3-bucket parallel kunt houden. Voeg extra doelen toe op Instellingen, Off-site, elk met zijn eigen repository, S3-opslagklasse, append-only-vlag, retentie en groeibudget. Een bestaande enkele off-site setup wordt onaangeroerd overgenomen als het eerste doel, en elk doel van een domein repliceert op de off-site planning van dat domein.
- **Off-site planning per domein** (bewerkt naast elke andere planning op Instellingen, Planningen): laat het leeg om na elke lokale back-up te repliceren, of stel een cadans in (bijvoorbeeld `weekly Sun 03:00`) om minder vaak off-site te sturen dan je lokaal back-upt. Een knop **Nu repliceren** dekt runs op aanvraag.
- **Off-site retentie** staat op Instellingen, Off-site zodat je off-site kopieën langer als archief kunt bewaren. Laat het beleid geheel op nul om off-site snapshots nooit automatisch te trimmen.
- **Bandbreedtelimieten** (Instellingen, Off-site) begrenzen de restic-upload/downloadsnelheid zodat replicatie je WAN niet verzadigt.
- Een **replicatie-indicator** toont welk domein aan het repliceren is terwijl het draait (op zijn pagina en het Dashboard). Het is een actieve indicator, geen percentagebalk, omdat `restic copy` geen machine-leesbare voortgang blootgeeft.

!!! note "Herstel rechtstreeks vanaf off-site"
    Elke back-upbrowser heeft een schakelaar **Lokaal / Off-site**, zodat je bij een verloren of corrupte lokale repo direct vanaf de off-site replica kunt lijsten en herstellen. Verwijderen gaat per bron: een back-up verwijderen raakt alleen de kopie die je bekijkt.

## Externe primaire repositories {#remote-primary-repositories}

Het back-uppad van een domein (Instellingen, Paden en opslag) is niet beperkt tot een lokale map: richt het rechtstreeks op een restic-remote (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:gebruiker@host:/repo`, `rclone:remote:bucket/pad`) en BombVault back-upt daar direct naartoe, zonder aparte lokale kopie en zonder replicatiestap. Dat is een werkelijk andere vorm dan de off-sitereplicatie hierboven: daar is het lokale repository primair en is het off-site-repository er een archief van naar beste vermogen; hier **is** het externe repository het primaire, en is het de enige kopie zolang je voor dat domein niet ook off-sitereplicatie (of een tweede remote) instelt.

Elk van de vijf padvelden (Containers, Virtuele machines, Flash, Configuratie, Bestanden) heeft er direct naast een schakelaar **Lokaal / Extern**:

- **Lokaal** toont de vertrouwde mappenbrowser.
- **Extern** vervangt hem door een eenvoudig URL-veld, plus een knop die hetzelfde venster voor verbindingstest en inloggegevens opent dat off-sitebestemmingen gebruiken, maar dan ingesteld voor dit primaire repository. Daar krijg je:
    - **Een verbindingstest** tegen het echte pad, voordat je erop vertrouwt.
    - **Bandbreedtelimieten** (upload en download), zodat een geplande back-up naar een extern primair repository je WAN-lijn niet dichtslibt: dezelfde restic-opties `--limit-upload` en `--limit-download` die off-sitereplicatie gebruikt, nu toegepast op de back-up zelf.
    - **Append-only-bescherming (onveranderlijkheid)**, gecontroleerd met dezelfde actieve manipulatietest (een echte DELETE-sonde naar de overkant) die off-sitebestemmingen krijgen. Staat die aan, dan weigert BombVault het repository zelf op te schonen: omdat er geen aparte lokale kopie achter zit, mogen de inloggegevens op deze machine niet in staat zijn de enige kopie van de back-up te wissen.
    - **Een alarm voor het groeibudget**, afgeleid van dezelfde trend in repositorygrootte die de Opslag-kaart al bijhoudt.

Niets hiervan is verplicht: een met de hand ingetypt extern pad zonder opgeslagen veiligheidsinstellingen back-upt precies zoals altijd (onbeperkte bandbreedte, opschoonbaar, geen budgetalarm). Het veiligheidsvenster is er voor als je dezelfde bescherming wilt die een off-sitekopie krijgt, zonder daarvoor apart een off-sitebestemming te moeten aanmaken.

!!! note "Cloud- en REST-inloggegevens worden gedeeld"
    Een extern primair repository meldt zich aan met dezelfde S3-/REST-inloggegevens die onder Instellingen, Off-site, Cloud-inloggegevens staan. Een aparte opslag voor inloggegevens van primaire repositories bestaat niet.

## Onveranderlijk (append-only) off-site

Vlag een off-site repo als append-only zodat ransomware, of een gecompromitteerde host, je back-ups niet kan verwijderen of herschrijven. De andere kant (een `restic/rest-server` in `--append-only`-modus) **dwingt** het af. BombVault **verifieert** het alleen en toont nooit groen op basis van louter een configuratie-claim.

De wizard **begeleide off-site setup** leidt je van de backendkeuze (rest-server / rclone / S3) via een kant-en-klaar rest-server-deploysnippet, een verbindingstest, de onveranderlijk-schakelaar (die meteen de tamper-test draait) en een retentiestrategie, zodat append-only off-site bereikbaar is zonder configs met de hand te bewerken.

!!! warning "Onveranderlijke repo's worden nooit vanaf deze machine geprund"
    Een onveranderlijke off-site prunt bewust nooit oude snapshots. Stel er een **groeibudget-alarm** voor in zodat je gewaarschuwd wordt voordat de repo-grootte uit de hand loopt.

## Tamper-test

BombVault bewijst periodiek de append-only-garantie door daadwerkelijk een verwijdering te proberen tegen de off-site repo, gericht op een niet-bestaand object:

- **Geweigerd** betekent beschermd.
- **Geaccepteerd** betekent niet beschermd.
- Een **onduidelijk** resultaat (server onbereikbaar, auth-fout) draait het opgeslagen oordeel nooit om.

Een echte omslag van beschermd naar onbeschermd vuurt één enkele waarschuwing af.

## DR-oefeningen

BombVault biedt twee niveaus van bewijs dat je back-ups daadwerkelijk herstelbaar zijn, niet alleen aanwezig.

- **Herstelverificatie-oefeningen (lokaal).** BombVault draait periodiek `restic check --read-data-subset` (begrensd, nooit een schijfvullend volledig herstel) en toont een badge *laatst geverifieerd herstelbaar* per domein. De cadans staat op Instellingen, Planningen; de badge op Instellingen, Integriteit.
- **DR-oefeningen (off-site).** BombVault herstelt een echt doel vanuit de off-site repo in een wegwerp-sandbox, verifieert het bestand-voor-bestand en byte-voor-byte, en ruimt daarna op. Dit bewijst dat je vanaf off-site kunt herstellen, niet alleen dat de repo antwoordt.

De **ransomwarebeschermings-scorecard** op het Dashboard vat dit samen tot een groene / oranje / rode houding per domein, met een van datum voorziene checklist (off-site geconfigureerd, append-only geverifieerd, replicatie actueel, hersteloefening geslaagd, versleuteling aan, prune-strategie ingesteld). Elke rode rij linkt diep door naar de fix, en de kaart wordt alleen groen op geverifieerde feiten.

## Ontvanger-dashboard (de ontvangende kant)

Alles hierboven is de *zendende* kant. Op de machine die onveranderlijke off-site kopieën van een andere BombVault **ontvangt**, geeft het Ontvanger-dashboard je onafhankelijke, alleen-lezen monitoring van die repositories op de ontvangende hardware, zodat een stille fout aan de andere kant niet onopgemerkt blijft.

Zet de schakelaar **Ontvanger** in Instellingen aan om een tabblad **Ontvanger** te onthullen. Het is standaard uit; schakel het alleen in op een machine die daadwerkelijk onveranderlijke off-site back-ups ontvangt. Registreer daarna een ontvangen repository (alleen-lezen, geopend met de sleutel van de zendende instantie) om te krijgen:

- **Een snapshotinventaris gegroepeerd per bron**, zodat je precies kunt zien welke containers, VM's en bestandssets zijn geland.
- **Laatst ontvangen** per bron, zodat je weet hoe vers elk is.
- **Een onafhankelijke `restic check`** die op de ontvangende hardware draait, zodat de integriteit wordt geverifieerd waar de data daadwerkelijk zit, niet alleen bij de zender.
- **Een dead-man's switch:** een waarschuwing wanneer een bron stopt met verzenden binnen een venster dat je instelt.
- **Integriteitswaarschuwingen:** een waarschuwing wanneer een controle aan de ontvangende kant mislukt.

De Ontvanger is strikt alleen-lezen. Het schrijft nooit naar de ontvangen repository, dus het kan nooit de append-only-garantie breken waar de zender op vertrouwt.

## Uitgewerkt voorbeeld: twee Unraid-machines, van begin tot eind

Hierboven staan de onderdelen. Dit is één volledige opstelling met echte waarden, want onderdelen zijn makkelijker samen te voegen als je ze één keer samengevoegd hebt gezien.

Twee machines: **TOWER** draait de containers en stuurt de back-ups, **VAULT** ontvangt ze en dwingt onveranderlijkheid af. Vul je eigen namen, adressen en sharepaden in.

**1. Zet op VAULT de append-only-server op.** Ga in BombVault op TOWER naar *Instellingen → Extern → begeleide installatie*, kies **rest-server** en genereer het recept. Kopieer het tabblad **Unraid-sjabloon (XML)**, sla het op VAULT op als `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, dan *Docker → Add Container* en kies **rest-server** uit de sjabloonlijst. Schrijf vóór het starten de getoonde `htpasswd`-regel op VAULT in `/mnt/user/appdata/rest-server/.htpasswd`. Het eenmalige wachtwoord wordt één keer getoond en nooit bewaard: kopieer het nu.

    Laat `--append-only` in het OPTIONS-veld staan. Daar draait alles om: zonder dat is VAULT weer een gewone share.

**2. Richt op TOWER de externe repo erop.** De repo-URL volgt het patroon dat het recept afdrukt:

    rest:http://VAULT:8000/bombvault-containers/containers

Het eerste padsegment is de htpasswd-gebruiker, het tweede de repository. Vul de gegenereerde gebruiker en het wachtwoord in als REST-inloggegevens van de bestemming en voer de **verbindingstest** uit.

**3. Zet op TOWER «Onveranderlijk» aan.** De manipulatietest loopt meteen en moet *beschermd* melden. Wat de antwoorden betekenen:

| Resultaat | Wat er gebeurde |
| --- | --- |
| **beschermd** | VAULT weigerde de verwijdering. Dit is de enige geslaagde toestand. |
| **NIET beschermd** | VAULT accepteerde een verwijdering. `--append-only` ontbreekt of is verwijderd. |
| **niet doorslaggevend** | Geen van beide. Meestal is de URL niet die welke restic zelf gebruikt, of de inloggegevens zijn gewijzigd. Er wordt niets vastgelegd en geen waarschuwing gegeven. |

**4. Kijk op VAULT wat er binnenkomt.** Zet *Instellingen → Ontvanger* aan, open het tabblad **Ontvanger** en registreer de repository alleen-lezen.

!!! warning "De locatie is een pad **binnen** de container, geschreven ten opzichte van de host-mount"
    Vul `user/appdata/rest-server/bombvault-containers/containers` in, **niet** `/mnt/user/appdata/…`. BombVault draait in een container waar de `/mnt` van de host elders is gemount; een absoluut hostpad bestaat daar niet. Plak je er toch een, dan noemt BombVault nu het relatieve pad dat je nodig hebt.

    De **verzendende APP_KEY** is de sleutel van TOWER, niet die van VAULT. Je vindt hem op TOWER onder *Instellingen → Systeem*.

**5. Maak het wederzijds, als je wilt.** Herhaal dezelfde vijf stappen in de andere richting: een rest-server op TOWER die de kopie van VAULT ontvangt. Elke machine dwingt dan onveranderlijkheid af voor de andere, en geen van beide kan de back-ups van de ander verwijderen.

## Begeleid herstel

Een speciaal tabblad **Herstel** leidt een verse of herbouwde installatie op één plek door het noodgeval:

1. **Herstelt eerst BombVaults eigen instellingen**, zodat de back-uppaden, off-site doelen en inloggegevens die de rest van de flow nodig heeft al zijn ingevuld (toegepast via een self-restart over de Docker-socket, zodat de live instellingendatabase nooit onder een open handle wordt overschreven).
2. **Controleert of BombVault je back-ups kan lezen** (het encryptiesleutel-addertje vooraf).
3. Laat je **wijzen naar je bestaande repo** (lokaal of off-site).
4. **Ontdekt** de containers, VM's en bestandssets die erin zijn opgeslagen.
5. **Herstelt ze allemaal** (gestopt gelaten, zodat je ze bewust start), met je herstelkit één klik weg.

!!! tip "Geplande migratie versus noodgeval"
    Begeleid herstel herstelt BombVaults eigen instellingen vanuit een back-up. Voor een *geplande* verhuizing naar een nieuwe machine kun je in plaats daarvan je configuratie rechtstreeks meenemen met de kaart **Instellingen exporteren en importeren** (een portable JSON-bestand). Zie [Configuratie](configuration.md#portable-settings-export-and-import).

### Herstellen vanuit een andere BombVault-repo

Een aparte kaart op het tabblad **Herstel** opent de repo van een *andere* BombVault-instantie (een share gemount onder `/mnt`, of een remote URL) met **de `APP_KEY` van die instantie**, in een eenmalige, alleen-lezen sessie. Blader door de containers, VM's en bestandssets die daar zijn opgeslagen, kies een snapshot en herstel hem, en het herstelde object wordt een normale lokale container, VM of bestandsset. Er wordt nooit iets naar de andere repo geschreven, en je eigen back-upinstellingen blijven onaangeroerd (de sessie leeft in het geheugen en verloopt vanzelf). Een container van server A naar server B verplaatsen betekent niet langer je repo-instellingen omleiden en die achteraf terugdraaien. Live server-naar-server-federatie valt uitdrukkelijk buiten het bereik; dit is een bewuste eenmalige pull.

## Herstelkit voor de encryptiesleutel

Dit is het onderdeel dat noodherstel mogelijk maakt, zelfs wanneer er geen draaiende BombVault is.

Eén klik downloadt de **hoofdsleutel**, het **afgeleide restic-wachtwoord** en de **exacte repo-locaties en commando's**, zodat je rechtstreeks met de restic-CLI op elke machine kunt herstellen. Een Dashboard-herinnering zeurt totdat je hem hebt bewaard.

!!! danger "Bewaar de herstelkit buiten de server"
    De kit bevat het geheim dat je back-ups ontsleutelt. Bewaar hem ergens veilig en gescheiden van de server (een wachtwoordmanager, een geprinte kopie in een kluis). Als je zowel BombVault als `APP_KEY` verliest zonder herstelkit, kunnen je versleutelde back-ups niet worden hersteld.

### Als het pakket niet bij de hand is

Het wachtwoord staat nergens opgeslagen, het wordt **berekend** uit de `APP_KEY`. Met de sleutel en een shell kun je het dus zelf namaken:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Dat is HMAC-SHA256 over de vaste tekst `bombvault:restic-repo`, met de ruwe bytes van de hexadecimale `APP_KEY` als sleutel, weergegeven als 64 hexadecimale tekens in kleine letters. Dezelfde waarde staat in het pakket, als afgeleid restic-wachtwoord; dit is voor de dag dat het pakket ergens anders ligt dan jij.

!!! warning "Gebruik bij een ontvangen repository de sleutel van de VERZENDENDE instantie"
    Een repository dat hier via off-sitereplicatie is beland, is aangemaakt door de machine die het stuurde, met **diens** `APP_KEY`. Afleiden uit de sleutel van de ontvangende machine geeft een wachtwoord dat restic weigert, wat precies leest als een kapot repository terwijl het dat niet is. Dat is de gebruikelijke reden dat `restic check` op een ontvangen repository steeds opnieuw om het wachtwoord vraagt.

Omdat hersteldefinities **binnen** elke repo leven (`<repo>/def`, `<repo>/vm-def`), is een gekopieerde repo-map volledig zelfstandig, dus de kit plus de repo is alles wat een bare-metal-herstel nodig heeft.
