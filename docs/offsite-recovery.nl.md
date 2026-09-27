# Off-site en herstel

Lokale back-ups beschermen je tegen een verloren container of een slechte update. Off-site replicatie en een geteste herstelkit beschermen je tegen de hele machine, ransomware of een brand. Deze pagina behandelt off-site repliceren, die kopie manipulatiebestendig maken, bewijzen dat je kunt herstellen, en herstellen wanneer BombVault zelf weg is.

## Off-site replicatie

Houd de snelle lokale back-up en kopieer hem naar een of meer andere plekken. De plekken waarnaar een domein wordt gekopieerd, kies je op de kaart **Domeinen** onder **Instellingen, Opslag**, één chip per plek (zie [Opslagplekken](storage-places.md#domains)). BombVault kopieert nieuwe snapshots daarheen met `restic copy` op best-effort-basis, zodat een mislukte kopie de lokale back-up nooit laat mislukken. De plek waarin een domein is opgeslagen, hoeft niet lokaal te zijn; zie [Een domein op een externe plek](#remote-primary-repositories).

- **Meerdere kopieerplekken per domein.** Een domein kan tegelijk naar meerdere plekken worden gekopieerd, bijvoorbeeld een rest-server bij een vriend thuis en een B2-bucket. Retentie, opslagklasse, append-only, limieten en groeibudget horen bij de plek, dus elke kopie volgt de regels van de plek waar ze terechtkomt.
- **Kopieerplanning per domein** (bewerkt naast elke andere planning op Instellingen, Planningen): laat het leeg om na elke lokale back-up te kopiëren, of stel een cadans in (bijvoorbeeld `weekly Sun 03:00`) om minder vaak te kopiëren dan je back-upt. **Nu kopiëren** op de rij van het domein voert het op aanvraag uit.
- **Retentie per plek.** Elke plek houdt zijn eigen regels aan, zodat een off-site plek kopieën langer als archief kan bewaren. Een plek waar elke regel op nul staat, trimt nooit.
- **Bandbreedtelimieten** per plek begrenzen de upload- en downloadsnelheid van restic zodat kopiëren je WAN niet verzadigt.
- Een **replicatie-indicator** toont welk domein aan het kopiëren is terwijl het draait (op zijn pagina en het Dashboard). Het is een actieve indicator, geen percentagebalk, omdat `restic copy` geen machine-leesbare voortgang blootgeeft.

!!! note "Herstel vanaf elke plek"
    Elke container, VM, bestandsset, de flash en de app-configuratie tonen hun back-ups als één tijdlijn over alle plekken waar een back-up ligt. Een back-up die naar B2 is gekopieerd, verschijnt één keer, gemarkeerd met elke plek die hem bevat. Een herstel neemt de eerste plek die het kan bereiken, te beginnen bij de repository waarnaar het item wordt geschreven, en je kunt per rij een andere plek kiezen. Off-site plekken worden alleen gelezen als je ze opent. Verwijderen op één plek controleert eerst de andere en zegt of het de laatste kopie was.

## Plaatsing per item {#placement}

Elke kaart van een container, VM en bestandsset heeft een rij **Plaatsing** met drie segmenten:

- **Lokaal** schrijft het item naar de repository die onder **Opgeslagen op** staat en kopieert het nergens naartoe. Gebruik dit voor data die al een tweede kopie heeft, bijvoorbeeld een share die op een NAS leeft.
- **Lokaal + off-site** schrijft het ook daar en kopieert het naar de doelen die onder **Kopieer naar** zijn aangevinkt, één chip per off-site doel van het domein. Vink een chip uit en dat doel krijgt niets nieuws meer van dit item.
- **Alleen off-site** schrijft het item rechtstreeks naar de plek onder **Verstuur naar**, elke plek behalve de thuisplek van het domein. Wordt het domein al naar die plek gekopieerd, dan krijgt het item een directe repository naast de kopieën; anders maakt BombVault daar een repository voor het domein aan.

De locatie ligt vast vanaf de eerste back-up van het item, omdat BombVault back-ups nooit tussen repositories verplaatst. De kopieën kunnen op elk moment veranderen. Een doel dat een item niet meer krijgt, houdt de kopieën die het heeft en trimt ze naar zijn eigen retentie bij de volgende off-site run van het domein; **Verwijderen bij B2** op de kaart verwijdert ze meteen. Als sommige van die kopieën nergens anders bestaan, toont de bevestiging ze op datum en vraagt om de naam van het item. Bij append-only doelen kan niet worden verwijderd.

Onder de rij zegt de kaart waar het item naartoe gaat en wat er werkelijk is: hoeveel locaties het bevatten, wanneer elk doel voor het laatst is gezien, en of aan 3-2-1 wordt voldaan. Een locatie is de server met de oorspronkelijke data en elke plek op een andere locatie (zie [Buiten het pand](#off-the-premises-mark)). BombVault controleert kopieën en locaties; het controleert niet het «twee media» deel van 3-2-1.

### Standaarden per domein

De kaart **Domeinen** onder Instellingen, Opslag heeft één rij per domein. **Gekopieerd naar** geldt meteen voor elk item zonder eigen keuze, en voor de projectmappen van Compose-stacks. Zodra een domein back-ups heeft, geldt **Opgeslagen in** voor een nieuw item bij zijn eerste back-up, en wijzigen verplaatst geen back-ups. Voor het opslaan noemt de rij elke plek die items wint of verliest en hoeveel snapshots dat betekent, en de vraag bevat de schakelaar **Toepassen op items zonder back-ups**, die ook elk item dat nog geen back-up heeft op de nieuwe standaard zet. **Uitzonderingen** toont de items met een eigen keuze.

Vink je een nieuwe plek aan onder **Gekopieerd naar**, dan ontvangt die plek elk item dat niet op Lokaal staat. De bevestiging zegt hoeveel items en, waar bekend, hoeveel geschiedenis dat is.

### Directe repository's

Een plek kiezen onder Alleen off-site waar het domein al naartoe wordt gekopieerd, vraagt één keer om bevestiging, maakt dan een directe repository aan naast de kopieën, bijvoorbeeld `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, en wijst het item ernaartoe. Voor een kopieerdoel zonder plek opent de keuze een venster met een voorgesteld adres en een verbindingstest die niets aanmaakt, en **Aanmaken en gebruiken** maakt de repository aan. Een directe repository neemt de sleutel, opslagklasse, limieten, append-only-instelling en retentie van de plek over, en verandert daarmee mee. Als een nieuwe sleutel van de plek hem niet kan openen, houdt de directe repository de sleutel die hij heeft, en het opslaan meldt dat. Zijn snapshots dragen de tag `bv:direct`, en elke andere retentieronde spaart ze, zodat een directe repository die zijn link met zijn plek kwijt is, nooit veroudert volgens de lokale regels. Een B2-sleutel die tot één map is beperkt, moet het adres van de plek dekken en niet alleen de map van het domein, anders is de map ernaast onbereikbaar.

### Buiten het pand {#off-the-premises-mark}

Een kopie telt alleen als eigen locatie wanneer de plek ervan op een andere locatie staat. Een cloudplek telt altijd mee en een map op deze Unraid nooit; beantwoord voor een NAS, een rest-server of een SFTP-server **Waar staat het apparaat?** in de details van de plek met **Hier in huis** of **Op een andere locatie**. Het antwoord telt alleen mee voor locaties en 3-2-1 op de kaarten en het Dashboard. Er verandert geen kopie door.

### Na een rebuild

Kopieerkeuzes leven in BombVaults eigen instellingen. Na een rebuild via Ontdekken zonder hersteld `/config` zijn ze weg, en alles kopiëren zou de items die je had uitgesloten opnieuw naar B2 sturen. De off-site replicatie van elk herbouwd domein pauzeert daarom. Het Dashboard toont dit in oranje, en de rij van het domein op de kaart Domeinen biedt **Standaard bevestigen** met een voorbeeld van wat de volgende run kopieert en de namen in de back-ups zonder item, die je daar kunt uitsluiten. Alleen de bevestiging beëindigt de pauze; het importeren van een instellingenbestand brengt regels en standaarden terug maar beëindigt de pauze niet.

## Een domein op een externe plek {#remote-primary-repositories}

Een domein hoeft niet lokaal te worden opgeslagen. Zolang de back-uplocatie ervan geen back-ups bevat, kies je een externe plek onder **Opgeslagen in** op de kaart Domeinen, en het domein back-upt daar rechtstreeks naartoe, zonder lokale kopie en zonder kopieerstap. De externe repository is dan de enige kopie, tenzij het domein ook naar een andere plek wordt gekopieerd. Elke externe plek heeft dezelfde beveiligingen:

- **Een verbindingstest** voordat er iets wordt geschreven.
- **Bandbreedtelimieten** voor de back-up zelf, dezelfde opties `--limit-upload` en `--limit-download` die een kopie gebruikt.
- **Append-only-bescherming**, gecontroleerd met dezelfde actieve tamper-test. Staat die aan, dan schoont BombVault de repository nooit op, omdat de inloggegevens op deze machine de enige kopie van de back-up niet mogen kunnen verwijderen.
- **Een groeibudget**, afgeleid van dezelfde groottetrend die de Opslag-kaart bijhoudt.

Een domein op een externe plek is net als een lokaal domein de bron van zijn kopieën; zie [Kopieën tussen plekken met verschillende inloggegevens](storage-places.md#different-credentials).

!!! note "Inloggegevens horen bij de plek"
    Een externe plek bewaart zijn eigen inloggegevens. Een plek die met de gedeelde cloud-inloggegevens is ingesteld, blijft die gebruiken tot de toegang ervan in de details wordt gewijzigd.

## Onveranderlijk (append-only) off-site

Vlag een off-site repo als append-only zodat ransomware, of een gecompromitteerde host, je back-ups niet kan verwijderen of herschrijven. De andere kant (een `restic/rest-server` in `--append-only`-modus) **dwingt** het af. BombVault **verifieert** het alleen en toont nooit groen op basis van louter een configuratie-claim.

Het venster **Plek toevoegen** bevat een kant-en-klaar recept voor een rest-server in append-only-modus, met één gebruiker voor deze BombVault. Bij een rest-server-plek met **Append-only** aan voert **Op append-only testen** in de details van de plek de tamper-test uit tegen elk domeinpad, elke ingeschakelde kopie en elke repository op de plek en geeft één antwoord voor de plek, zodat append-only off-site bereikbaar is zonder configs met de hand te bewerken.

!!! note "Een geslaagde verwijdering onder `/locks/` is verwacht"
    Append-only betekent niet dat er niets meer verwijderd kan worden. restic moet zijn eigen locks zetten en weer vrijgeven, dus `/locks/` blijft met opzet schrijfbaar en verwijderbaar. Snapshots en de data erachter, precies waar ransomware op mikt, kunnen niet worden verwijderd. Als je de andere kant zelf test, is een geslaagde verwijdering onder `/locks/` correct gedrag en geen gat in de bescherming.

!!! warning "Onveranderlijke repo's worden nooit vanaf deze machine geprund"
    Een onveranderlijke off-site prunt bewust nooit oude snapshots. Stel er een **groeibudget-alarm** voor in zodat je gewaarschuwd wordt voordat de repo-grootte uit de hand loopt.

## Tamper-test

BombVault bewijst periodiek de append-only-garantie door daadwerkelijk een verwijdering te proberen tegen de off-site repo, gericht op een niet-bestaand object:

- **Geweigerd** betekent beschermd.
- **Geaccepteerd** betekent niet beschermd.
- Een **onduidelijk** resultaat (server onbereikbaar, auth-fout) draait het opgeslagen oordeel nooit om.

Een echte omslag van beschermd naar onbeschermd vuurt één enkele waarschuwing af.

Op een plek test **Op append-only testen** elk domeinpad, elke ingeschakelde kopie en elke repository daar met hun eigen inloggegevens en voegt de oordelen samen tot één antwoord: accepteert één enkele repository een verwijdering, dan krijgt de hele plek *verwijderen toegestaan*.

## DR-oefeningen

BombVault biedt twee niveaus van bewijs dat je back-ups daadwerkelijk herstelbaar zijn, niet alleen aanwezig.

- **Herstelverificatie-oefeningen (lokaal).** BombVault draait periodiek `restic check --read-data-subset` (begrensd, nooit een schijfvullend volledig herstel) en toont een badge *laatst geverifieerd herstelbaar* per domein. De cadans staat op Instellingen, Planningen; de badge op Instellingen, Integriteit.
- **DR-oefeningen (off-site).** BombVault herstelt een echt doel vanuit de off-site repo in een wegwerp-sandbox, verifieert het bestand-voor-bestand en byte-voor-byte, en ruimt daarna op. Dit bewijst dat je vanaf off-site kunt herstellen, niet alleen dat de repo antwoordt. Alleen plekken op een andere locatie worden getest, want een kopie in hetzelfde huis bewijst niets over het verlies van dat huis. Een domein dat naar meerdere daarvan wordt gekopieerd, wordt per geplande run tegen één ervan getest, om de beurt, en het Dashboard noemt de plek van de laatste oefening.

De **ransomwarebeschermings-scorecard** op het Dashboard vat dit samen tot een groene / oranje / rode houding per domein, met een van datum voorziene checklist (off-site geconfigureerd, append-only geverifieerd, replicatie actueel, hersteloefening geslaagd, versleuteling aan, prune-strategie ingesteld). Elke rode rij linkt diep door naar de fix, en de kaart wordt alleen groen op geverifieerde feiten.

## Ontvanger-dashboard (de ontvangende kant)

![De ontvangende kant, alleen-lezen bewaakt, met een integriteitscontrole op deze machine.](assets/screenshots/receiver.png)

*De ontvangende kant, alleen-lezen bewaakt, met een integriteitscontrole op deze machine.*

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

**1. Zet op VAULT de append-only-server op.** Open in BombVault op TOWER *Instellingen → Opslag*, klik op **Plek toevoegen**, kies **rest-server** en klik op **Recept tonen**. Kopieer het blok **Unraid-sjabloon**, sla het op VAULT op als `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, dan *Docker → Add Container* en kies **rest-server** uit de sjabloonlijst. Schrijf vóór het starten de getoonde `htpasswd`-regel op VAULT in `/mnt/user/appdata/rest-server/.htpasswd`. Het wachtwoord wordt één keer getoond en nooit bewaard; het recept heeft het samen met de gebruiker al in het formulier op TOWER gezet, dus laat dat venster open. De `htpasswd`-regel bevat hetzelfde wachtwoord, al voor je gehasht met bcrypt, dus je hoeft zelf niets te hashen.

    Laat `--append-only` in het OPTIONS-veld staan. Zonder die optie is VAULT gewoon weer een normale share.

**2. Voeg op TOWER de plek toe.** Vul het adres van VAULT in, `http://VAULT:8000`, naast de gebruiker en het wachtwoord die het recept heeft ingevuld, en klik op **Verbinding testen**. BombVault bouwt het adres daaruit op:

    rest:http://VAULT:8000/tower

Het eerste padsegment is de htpasswd-gebruiker, hier `tower`, en elk domein krijgt zijn map daaronder, bijvoorbeeld `rest:http://VAULT:8000/tower/container`. Beantwoord **Waar staat het apparaat?** met **Op een andere locatie**, klik op **Toevoegen**, en vink de plek aan onder **Gekopieerd naar** bij de domeinen die daarheen moeten.

**3. Zet op TOWER Append-only aan** onder **Bescherming** in de details van de plek, en klik daarna op **Op append-only testen**. De test controleert elk domeinpad, elke kopie en elke repository op de plek en geeft één antwoord voor de plek, en dat moet *verwijderen geweigerd* zijn. Wat de antwoorden betekenen:

| Resultaat | Wat er gebeurde |
| --- | --- |
| **verwijderen geweigerd** | VAULT weigerde de verwijdering. Dit is de enige geslaagde toestand. |
| **verwijderen toegestaan** | VAULT accepteerde een verwijdering. `--append-only` ontbreekt of is verwijderd. |
| een melding in plaats van een resultaat | De test kon niet draaien. Meestal is het adres niet het adres dat restic zelf gebruikt, of de inloggegevens zijn gewijzigd. Er wordt niets vastgelegd en geen waarschuwing gegeven. |

**4. Kijk op VAULT wat er binnenkomt.** Zet *Instellingen → Ontvanger* aan, open het tabblad **Ontvanger** en registreer de repository alleen-lezen.

!!! warning "De locatie is een pad **binnen** de container, geschreven ten opzichte van de host-mount"
    Vul `user/appdata/rest-server/tower/container` in, **niet** `/mnt/user/appdata/…`. BombVault draait in een container waar de `/mnt` van de host elders is gemount; een absoluut hostpad bestaat daar niet. Plak je er toch een, dan noemt BombVault het relatieve pad dat je nodig hebt.

    De **verzendende APP_KEY** is de sleutel van TOWER, niet die van VAULT. Je vindt hem op TOWER onder *Instellingen → Systeem*.

**5. Maak het wederzijds, als je wilt.** Herhaal dezelfde vijf stappen in de andere richting: een rest-server op TOWER die de kopie van VAULT ontvangt. Elke machine dwingt dan onveranderlijkheid af voor de andere, en geen van beide kan de back-ups van de ander verwijderen.

## Begeleid herstel

Een speciaal tabblad **Herstel** leidt een verse of herbouwde installatie op één plek door het noodgeval:

1. **Controleert of BombVault je back-ups kan lezen** (het encryptiesleutel-addertje vooraf).
2. **Herstelt BombVaults eigen instellingen**, zodat de back-uppaden, off-site doelen en inloggegevens die de rest van de flow nodig heeft al zijn ingevuld. De instellingen-back-up wordt gelezen van de plek die de rij Zelf-back-up onder **Opgeslagen in** noemt, of van de kopie van de Zelf-back-up onder **Gekopieerd naar**, en de stap toont die plek met haar adres. Wil je van een andere plek lezen, wijzig dan eerst de rij Zelf-back-up in stap 3. Het herstel wordt toegepast via een self-restart over de Docker-socket, zodat de live instellingendatabase nooit onder een open handle wordt overschreven.
3. **Koppelt je bestaande back-ups** via de rijen van de kaart Domeinen: kies op de rij van elk domein onder **Opgeslagen in** de plek waar de back-ups staan en onder **Gekopieerd naar** de plekken met de kopieën. Een plek die nog geen rij aanbiedt, zoals een share, een server of een cloudbucket, verbind je met **Plek toevoegen**, hetzelfde venster als op Instellingen, Opslag. **Verbinden & voorbeeld** controleert daarna of de back-ups te lezen zijn.
4. **Ontdekt** de containers, VM's en bestandssets die erin zijn opgeslagen.
5. **Herstelt ze allemaal** (gestopt gelaten, zodat je ze bewust start), met je herstelkit één klik weg.

!!! note "Off-site kopieën wachten na een rebuild"
    Als stap 4 items herbouwt zonder de oude instellingen, pauzeert de off-site replicatie van die domeinen tot de standaardplaatsing is bevestigd. Zie [Plaatsing per item](#placement).

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
