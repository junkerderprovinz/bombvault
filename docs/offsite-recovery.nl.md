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

Omdat hersteldefinities **binnen** elke repo leven (`<repo>/def`, `<repo>/vm-def`), is een gekopieerde repo-map volledig zelfstandig, dus de kit plus de repo is alles wat een bare-metal-herstel nodig heeft.
