# Ekstern lagring og gjenoppretting

Lokale sikkerhetskopier beskytter deg mot en tapt container eller en dårlig oppdatering. Ekstern replikering og et testet gjenopprettingssett beskytter deg mot hele boksen, løsepengevirus eller en brann. Denne siden dekker å replikere eksternt, å gjøre den kopien manipuleringssikker, å bevise at du kan gjenopprette, og å komme deg tilbake når selve BombVault er borte.

## Ekstern replikering

Behold den raske lokale sikkerhetskopien og kopier den til ett eller flere andre lagringssteder. Du velger hvilke lagringssteder et domene kopieres til, på kortet **Domener** under **Innstillinger, Lagring**, én chip per lagringssted (se [Lagringssteder](storage-places.md#domains)). BombVault kopierer nye øyeblikksbilder dit med `restic copy` på best-effort-basis, så en mislykket kopi feiler aldri den lokale sikkerhetskopien. Lagringsstedet et domene er lagret på, trenger ikke være lokalt; se [Et domene lagret på et eksternt lagringssted](#remote-primary-repositories).

- **Flere lagringssteder for kopier per domene.** Et domene kan kopieres til flere lagringssteder samtidig, for eksempel en rest-server hjemme hos en venn og en B2-bucket. Oppbevaring, lagringsklasse, append-only, grenser og vekstbudsjett hører til lagringsstedet, så hver kopi følger reglene til lagringsstedet den havner på.
- **Kopitidsplan per domene** (redigert sammen med hver annen tidsplan på Innstillinger, Tidsplaner): la den stå tom for å kopiere etter hver lokale sikkerhetskopi, eller sett en kadens (for eksempel `weekly Sun 03:00`) for å kopiere sjeldnere enn du sikkerhetskopierer. **Kopier nå** på raden til domenet kjører den ved behov.
- **Oppbevaring per lagringssted.** Hvert lagringssted har sine egne regler, så et eksternt lagringssted kan beholde kopier lenger som et arkiv. Et lagringssted der hver regel står på null, trimmer aldri.
- **Båndbreddegrenser** per lagringssted begrenser resticts opplastings- og nedlastingshastighet så kopieringen ikke metter WAN-et ditt.
- En **replikeringsindikator** viser hvilket domene som kopierer mens det pågår (på siden sin og på Dashboardet). Det er en aktiv indikator, ikke en prosentbjelke, fordi `restic copy` ikke eksponerer noen maskinlesbar fremdrift.

!!! note "Gjenopprett fra hvilket som helst sted"
    Hver container, VM, filsett, flashen og appkonfigurasjonen lister sikkerhetskopiene sine som én tidslinje på tvers av alle stedene en sikkerhetskopi finnes. En sikkerhetskopi kopiert til B2 vises én gang, merket med hvert sted som har den. En gjenoppretting bruker det første stedet den kan nå, med utgangspunkt i depotet elementet er skrevet til, og du kan velge et annet sted per rad. Eksterne steder leses bare når du åpner dem. Sletting på ett sted sjekker de andre først og sier om det var siste kopi.

## Plassering per element {#placement}

Hvert kort for container, VM og filsett har en **Plassering**-rad med tre segmenter:

- **Lokal** skriver elementet til depotet vist under **Lagret på** og kopierer det ingen steder. Bruk det for data som allerede har en ekstra kopi, for eksempel en deling som ligger på en NAS.
- **Lokal + ekstern** skriver det dit også og kopierer det til målene som er huket av under **Kopier til**, én chip per eksternt mål i domenet. Fjern haken fra en chip, og det målet får ingenting nytt fra dette elementet.
- **Kun ekstern** skriver elementet rett til lagringsstedet under **Send til**, et hvilket som helst lagringssted utenom domenets hjem. Der domenet allerede kopieres til det lagringsstedet, får elementet et direkte depot ved siden av kopiene; ellers oppretter BombVault et depot for domenet der.

Plasseringen er fast fra elementets første sikkerhetskopi, fordi BombVault aldri flytter sikkerhetskopier mellom depoter. Kopiene kan endres når som helst. Et mål som ikke lenger får et element, beholder kopiene det har og trimmer dem til sin egen oppbevaring ved domenets neste eksterne kjøring; **Slett hos B2** på kortet fjerner dem med det samme. Når noen av de kopiene ikke finnes noe annet sted, viser bekreftelsen dem etter dato og ber om elementets navn. Append-only-mål kan det ikke slettes fra.

Under raden sier kortet hvor elementet går og hva som faktisk finnes: hvor mange steder som har det, når hvert mål sist ble sett, og om 3-2-1 er oppfylt. Et sted er serveren med originaldataene og hvert lagringssted på en annen lokasjon (se [Utenfor bygningen](#off-the-premises-mark)). BombVault sjekker kopier og steder; det sjekker ikke «to medier»-delen av 3-2-1.

### Standarder per domene

Kortet **Domener** under Innstillinger, Lagring har én rad per domene. **Kopiert til** gjelder med det samme for hvert element uten eget valg, og for prosjektmappene til Compose-stabler. Når et domene har sikkerhetskopier, gjelder **Lagret på** for et nytt element ved dets første sikkerhetskopi, og å endre det flytter ingen sikkerhetskopier. Før lagring lister raden opp hvert lagringssted som får eller mister elementer, og hvor mange øyeblikksbilder det betyr, og spørsmålet har bryteren **Bruk på elementer uten sikkerhetskopier**, som også setter hvert element som ennå ikke har en sikkerhetskopi, på den nye standarden. **Unntak** lister elementene med et eget valg.

Å huke av et nytt lagringssted under **Kopiert til** gjør at det mottar hvert element som ikke er satt til Lokal. Bekreftelsen sier hvor mange elementer og, der det er kjent, hvor mye historikk det er.

### Direkte depoter

Å velge et lagringssted under Kun ekstern der domenet allerede kopieres, spør én gang og oppretter så et direkte depot ved siden av kopiene, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, og peker elementet dit. For et kopimål uten lagringssted åpner valget en dialog med en foreslått adresse og en tilkoblingstest som ikke oppretter noe, og **Opprett og bruk** oppretter depotet. Et direkte depot overtar lagringsstedets nøkkel, lagringsklasse, grenser, append-only-innstilling og oppbevaring, og endres med dem. Når en ny nøkkel for lagringsstedet ikke kan åpne det, beholder det direkte depotet nøkkelen det har, og lagringen sier fra om det. Øyeblikksbildene bærer taggen `bv:direct`, og hver annen oppbevaringsrunde lar dem være, så et direkte depot som har mistet lenken til lagringsstedet sitt, aldri eldes etter de lokale reglene. En B2-nøkkel som er begrenset til én mappe, må dekke adressen til lagringsstedet, ikke bare mappen til domenet, ellers er mappen ved siden av utenfor rekkevidde.

### Utenfor bygningen {#off-the-premises-mark}

En kopi teller bare som et eget sted når lagringsstedet står på en annen lokasjon. Et skylagringssted teller alltid, og en mappe på denne Unraid-serveren teller aldri; for en NAS, en rest-server eller en SFTP-server svarer du på **Hvor står enheten?** i detaljene til lagringsstedet med **Her i huset** eller **På en annen lokasjon**. Svaret teller bare med i steder og 3-2-1 på kortene og Dashboardet. Det endrer ingen kopi.

### Etter en ombygging

Kopivalgene lever i BombVaults egne innstillinger. Etter en ombygging via Oppdag uten et gjenopprettet `/config` er de borte, og å kopiere alt ville sendt elementene du hadde utelatt, til B2 igjen. Ekstern replikering av hvert ombygde domene settes derfor på pause. Dashboardet viser det i rav, og raden til domenet på Domener-kortet tilbyr **Bekreft standard** med en forhåndsvisning av hva neste kjøring kopierer, og navnene i sikkerhetskopiene som mangler en oppføring, som du kan utelate der. Bare bekreftelsen avslutter pausen; å importere en innstillingsfil bringer tilbake regler og standarder, men avslutter den ikke.

## Et domene lagret på et eksternt lagringssted {#remote-primary-repositories}

Et domene trenger ikke å lagres lokalt. Så lenge sikkerhetskopistien ikke inneholder noen sikkerhetskopier, kan du velge et eksternt lagringssted under **Lagret på** på Domener-kortet, og domenet sikkerhetskopierer da rett dit, uten lokal kopi og uten kopisteg. Det eksterne depotet er da den eneste kopien, med mindre domenet også kopieres til et annet lagringssted. Hvert eksternt lagringssted har de samme sikringene:

- **En tilkoblingstest** før noe skrives.
- **Båndbreddegrenser** for selve sikkerhetskopien, de samme flaggene `--limit-upload` og `--limit-download` som en kopi bruker.
- **Append-only-beskyttelse**, kontrollert med den samme aktive tamper-testen. Er den på, beskjærer BombVault aldri depotet, fordi legitimasjonen på denne boksen ikke må kunne slette den eneste kopien av sikkerhetskopien.
- **Et vekstbudsjett**, hentet fra den samme størrelsesutviklingen som Lagringskortet følger.

Et domene som er lagret på et eksternt lagringssted, er kilden til kopiene sine, akkurat som et lokalt; se [Kopier mellom lagringssteder med ulik legitimasjon](storage-places.md#different-credentials).

!!! note "Legitimasjonen hører til lagringsstedet"
    Et eksternt lagringssted har sin egen legitimasjon. Et lagringssted som ble satt opp med den delte skylegitimasjonen, fortsetter å bruke den til tilgangen endres i detaljene til lagringsstedet.

## Uforanderlig (append-only) ekstern

Flagg et eksternt repo append-only så løsepengevirus, eller en kompromittert host, ikke kan slette eller skrive om sikkerhetskopiene dine. Den andre siden (en `restic/rest-server` som kjører i `--append-only`-modus) **håndhever** det. BombVault kun **verifiserer** det og viser aldri grønt på en konfigurasjonspåstand alene.

Vinduet **Legg til lagringssted** har en oppskrift, klar til å lime inn, på en rest-server i append-only-modus, med én bruker for denne BombVault. På et rest-server-lagringssted med **Append-only** på kjører **Sjekk append-only** i detaljene til lagringsstedet tamper-testen mot hver domenesti, hver påslått kopi og hvert depot på lagringsstedet og gir ett svar for lagringsstedet, så append-only ekstern er tilgjengelig uten å håndredigere konfigurasjoner.

!!! note "En vellykket sletting under `/locks/` er forventet"
    Append-only betyr ikke at ingenting kan slettes lenger. restic må ta og frigi sine egne låser, så `/locks/` forblir bevisst skrivbar og slettbar. Øyeblikksbilder og dataene bak dem, altså nettopp det løsepengevirus ville gå etter, kan ikke fjernes. Tester du motparten selv, er en sletting som lykkes under `/locks/` korrekt oppførsel og ikke et hull i beskyttelsen.

!!! warning "Uforanderlige repoer beskjæres aldri fra denne boksen"
    En uforanderlig ekstern beskjærer bevisst aldri gamle øyeblikksbilder. Sett en **vekstbudsjett-alarm** for den så du blir varslet før repo-størrelsen løper løpsk.

## Tamper-test

BombVault beviser jevnlig append-only-garantien ved faktisk å forsøke en sletting mot det eksterne repoet, rettet mot et ikke-eksisterende objekt:

- **Avvist** betyr beskyttet.
- **Akseptert** betyr ikke beskyttet.
- Et **usikkert** resultat (server unåbar, autentiseringsfeil) vender aldri den lagrede dommen.

En ekte beskyttet-til-ubeskyttet-vending utløser et enkelt varsel.

På et lagringssted sjekker **Sjekk append-only** hver domenesti, hver påslått kopi og hvert depot der med deres egen legitimasjon og slår dommene sammen til ett svar: godtar ett eneste depot en sletting, blir hele lagringsstedet *sletting godtatt*.

## DR-øvelser

BombVault tilbyr to nivåer av bevis for at sikkerhetskopiene dine faktisk er gjenopprettbare, ikke bare til stede.

- **Gjenopprettingsverifiseringsøvelser (lokale).** BombVault kjører jevnlig `restic check --read-data-subset` (avgrenset, aldri en disk-fyllende full gjenoppretting) og viser et *sist verifisert gjenopprettbar*-merke per domene. Kadensen ligger på Innstillinger, Tidsplaner; merket på Innstillinger, Integritet.
- **DR-øvelser (ekstern).** BombVault gjenoppretter et ekte mål fra det eksterne repoet inn i en engangs-sandkasse, verifiserer det fil-for-fil og byte-for-byte, og rydder deretter opp. Dette beviser at du kan komme deg tilbake fra ekstern, ikke bare at repoet svarer. Bare lagringssteder på en annen lokasjon øves mot, for en kopi i samme hus beviser ingenting om å miste huset. Et domene som kopieres til flere av dem, øves mot ett per planlagte kjøring, etter tur, og Dashboardet viser lagringsstedet for den siste øvelsen.

**Poengkortet for løsepengevirusbeskyttelse** på Dashboardet ruller dette opp i en grønn / gul / rød holdning per domene, med en aldersstemplet sjekkliste (ekstern konfigurert, append-only verifisert, replikering oppdatert, gjenopprettingsøvelse bestått, kryptering på, beskjæringsstrategi satt). Hver rød rad dyplenker til fiksen, og kortet blir bare grønt på verifiserte fakta.

## Mottaker-dashboard (mottakssiden)

![Den mottakende siden, overvåket skrivebeskyttet, med en integritetssjekk kjørt på denne maskinen.](assets/screenshots/receiver.png)

*Den mottakende siden, overvåket skrivebeskyttet, med en integritetssjekk kjørt på denne maskinen.*

Alt ovenfor er *sende*-siden. På boksen som **mottar** uforanderlige eksterne kopier fra en annen BombVault, gir Mottaker-dashboardet deg uavhengig, skrivebeskyttet overvåking av disse repositoriene på mottaks-maskinvaren, så en stille feil i den andre enden ikke går ubemerket hen.

Slå på **Mottaker**-bryteren i Innstillinger for å avdekke en **Mottaker**-fane. Den er av som standard; aktiver den kun på en boks som faktisk mottar uforanderlige eksterne sikkerhetskopier. Registrer deretter et mottatt repository (skrivebeskyttet, åpnet med den sendende instansens nøkkel) for å få:

- **Et øyeblikksbilde-inventar gruppert etter kilde**, så du kan se nøyaktig hvilke containere, VM-er og filsett som har landet.
- **Sist mottatt** per kilde, så du vet hvor fersk hver enkelt er.
- **En uavhengig `restic check`** kjørt på mottaks-maskinvaren, så integriteten verifiseres der dataene faktisk ligger, ikke bare hos senderen.
- **En dødmannsbryter:** et varsel når en kilde slutter å sende innenfor et vindu du setter.
- **Integritetsvarsler:** et varsel når en sjekk på mottakssiden feiler.

Mottakeren er strengt skrivebeskyttet. Den skriver aldri til det mottatte repositoriet, så den kan aldri bryte append-only-garantien senderen stoler på.

## Gjennomgått eksempel: to Unraid-maskiner, hele veien

Over beskrives delene. Her er ett komplett oppsett med ekte verdier, for deler er lettere å sette sammen når man har sett dem satt sammen én gang.

To maskiner: **TOWER** kjører containerne og sender sikkerhetskopiene, **VAULT** tar imot dem og håndhever uforanderligheten. Bytt ut med dine egne navn, adresser og delingsstier.

**1. Sett opp append-only-serveren på VAULT.** I BombVault på TOWER: åpne *Innstillinger → Lagring*, klikk **Legg til lagringssted**, velg **rest-server** og klikk **Vis oppskrift**. Kopier blokken **Unraid-mal**, lagre den på VAULT som `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, og velg deretter *Docker → Add Container* og **rest-server** fra mallisten. Skriv den viste `htpasswd`-linjen inn i `/mnt/user/appdata/rest-server/.htpasswd` på VAULT før du starter den. Passordet vises én gang og lagres aldri; oppskriften har allerede fylt det og brukeren inn i skjemaet på TOWER, så la det vinduet stå åpent. `htpasswd`-linjen bærer det samme passordet, allerede bcrypt-hashet for deg, så du skal ikke hashe noe selv.

    La `--append-only` bli stående i OPTIONS-feltet. Uten det er VAULT bare en vanlig deling igjen.

**2. Legg til lagringsstedet på TOWER.** Skriv inn adressen til VAULT, `http://VAULT:8000`, ved siden av brukeren og passordet oppskriften fylte inn, og klikk **Test tilkobling**. BombVault bygger adressen ut fra dem:

    rest:http://VAULT:8000/tower

Første ledd i stien er htpasswd-brukeren, her `tower`, og hvert domene får mappen sin under den, for eksempel `rest:http://VAULT:8000/tower/container`. Svar **På en annen lokasjon** på **Hvor står enheten?**, klikk **Legg til**, og huk av lagringsstedet under **Kopiert til** for domenene som skal dit.

**3. Slå på Append-only på TOWER** under **Beskyttelse** i detaljene til lagringsstedet, og klikk **Sjekk append-only**. Testen sjekker hver domenesti, hver kopi og hvert depot på lagringsstedet og gir ett svar for lagringsstedet, og det må være *sletting avvist*. Hva svarene betyr:

| Resultat | Hva som skjedde |
| --- | --- |
| **sletting avvist** | VAULT avviste slettingen. Det er den eneste beståtte tilstanden. |
| **sletting godtatt** | VAULT godtok en sletting. `--append-only` mangler eller er fjernet. |
| en melding i stedet for et resultat | Testen kunne ikke kjøre. Som regel er adressen ikke den restic selv bruker, eller legitimasjonen er endret. Ingenting registreres, og ingen varsling utløses. |

**4. Se på VAULT hva som kommer inn.** Slå på *Innstillinger → Mottaker*, åpne fanen **Mottaker**, og registrer arkivet skrivebeskyttet.

!!! warning "Plasseringen er en sti **inne i** containeren, skrevet relativt til vertsmonteringen"
    Skriv inn `user/appdata/rest-server/tower/container`, **ikke** `/mnt/user/appdata/…`. BombVault kjører i en container der vertens `/mnt` er montert et annet sted; en absolutt vertssti finnes ikke der. Limer du inn en, forteller BombVault deg den relative stien du skal bruke i stedet.

    **Sendende APP_KEY** er TOWERs nøkkel, ikke VAULTs. Du finner den på TOWER under *Innstillinger → System*.

**5. Gjør det gjensidig, hvis du vil.** Gjenta de samme fem trinnene motsatt vei: en rest-server på TOWER som tar imot VAULTs kopi. Da håndhever hver maskin uforanderligheten for den andre, og ingen kan slette den andres sikkerhetskopier.

## Veiledet gjenoppretting

En egen **Gjenoppretting**-fane leder en ny eller gjenoppbygd installasjon gjennom katastrofetilfellet, på ett sted:

1. **Sjekker at BombVault kan lese sikkerhetskopiene dine** (krypteringsnøkkel-fellen først).
2. **Gjenoppretter BombVaults egne innstillinger**, så sikkerhetskopistiene, eksterne målene og legitimasjonen resten av flyten trenger, kommer forhåndsutfylt. Innstillings-sikkerhetskopien leses fra lagringsstedet som raden Auto-sikkerhetskopi nevner under **Lagret på**, eller fra Auto-sikkerhetskopiens kopi under **Kopiert til**, og trinnet viser det lagringsstedet med adressen. For å lese fra et annet lagringssted endrer du først raden Auto-sikkerhetskopi i trinn 3. Gjenopprettingen brukes via en selv-omstart over Docker-socketen, så den kjørende innstillingsdatabasen aldri overskrives under en åpen handle.
3. **Kobler til de eksisterende sikkerhetskopiene dine** via radene på Domener-kortet: på raden til hvert domene velger du under **Lagret på** lagringsstedet der sikkerhetskopiene ligger, og under **Kopiert til** lagringsstedene med kopiene. Et lagringssted som ingen rad tilbyr ennå, for eksempel en delt mappe, en server eller en skybøtte, kobles til med **Legg til lagringssted**, det samme vinduet som på Innstillinger, Lagring. **Koble til og forhåndsvis** sjekker deretter at sikkerhetskopiene kan leses.
4. **Oppdager** containerne, VM-ene, filsettene og ZFS-datasettene lagret i det.
5. **Gjenoppretter containerne og VM-ene på én gang** (la stå stoppet, så du starter dem bevisst) og viser filsettene og ZFS-elementene som du gjenoppretter ett om gangen; ZFS-elementer kommer tilbake slått av. Gjenopprettingssettet ditt er ett klikk unna.

!!! note "Eksterne kopier venter etter en ombygging"
    Når steg 4 bygger opp igjen oppføringer uten de gamle innstillingene, settes ekstern replikering av de domenene på pause til standardplasseringen er bekreftet. Se [Plassering per element](#placement).

!!! tip "Planlagt migrering versus katastrofe"
    Veiledet gjenoppretting gjenoppretter BombVaults egne innstillinger fra en sikkerhetskopi. For en *planlagt* flytting til en ny boks kan du i stedet ta med konfigurasjonen din direkte via kortet **Eksporter og importer innstillinger** (en portabel JSON-fil). Se [Konfigurasjon](configuration.md#portable-settings-export-and-import).

### Gjenopprett fra et annet BombVault-repo

Et separat kort på **Gjenoppretting**-fanen åpner et *annet* BombVault-instans' repo (en deling montert under `/mnt`, eller en fjern-URL) med **den instansens `APP_KEY`**, i en engangs, skrivebeskyttet økt. Bla gjennom containerne, VM-ene og filsettene lagret der, velg et øyeblikksbilde og gjenopprett det, og det gjenopprettede objektet blir en normal lokal container, VM eller filsett. Ingenting skrives noensinne til det andre repoet, og dine egne sikkerhetskopiinnstillinger forblir urørte (økten lever i minnet og utløper av seg selv). Å flytte en container fra server A til server B betyr ikke lenger å peke om repo-innstillingene dine og reversere dem etterpå. Live server-til-server-føderasjon er eksplisitt utenfor omfang; dette er en bevisst engangs-henting.

## Gjenopprettingssett for krypteringsnøkkel

Dette er delen som gjør katastrofegjenoppretting mulig selv når det ikke finnes en kjørende BombVault.

Ett klikk laster ned **hovednøkkelen**, det **utledede restic-passordet** og de **nøyaktige repo-plasseringene og -kommandoene**, så du kan gjenopprette rett med restic-CLI-en på en hvilken som helst maskin. En Dashboard-påminnelse maser til du har lagret det.

!!! danger "Oppbevar gjenopprettingssettet bort fra serveren"
    Settet inneholder hemmeligheten som dekrypterer sikkerhetskopiene dine. Oppbevar det et trygt sted og adskilt fra serveren (en passordbehandler, en utskrevet kopi i et safe). Mister du både BombVault og `APP_KEY` uten et gjenopprettingssett, kan ikke de krypterte sikkerhetskopiene dine gjenopprettes.

!!! warning "Det nyeste snapshotet er ikke alltid det som skal gjenopprettes"
    Siden restic 0.17 viser `restic snapshots` størrelsen på hvert snapshot. Etter datatap kan det nyeste snapshotet være det tømte, så ikke gjenopprett et snapshot som er mye mindre enn de før det. Etter løsepengevirus kan det være det krypterte i vanlig størrelse. Kjører BombVault fortsatt, se først på siden **Avvik**: den nevner den siste gode sikkerhetskopien. En gjenoppretting trenger ingen avviksdata fra BombVault, og oppbevaringspausen beholder bare flere snapshots.

### Når settet ikke er for hånden

Passordet lagres ingen steder, det **beregnes** ut fra `APP_KEY`. Med nøkkelen og et skall kan du altså gjenskape det selv:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Det er HMAC-SHA256 over den faste strengen `bombvault:restic-repo`, med de rå bytene i den heksadesimale `APP_KEY` som nøkkel, skrevet ut som 64 små heksadesimale tegn. Samme verdi står i settet som det utledede restic-passordet; dette er for dagen da settet ligger et annet sted enn deg.

!!! warning "For et mottatt arkiv, bruk den SENDENDE instansens nøkkel"
    Et arkiv som havnet her via off-site-replikering, ble opprettet av maskinen som sendte det, med **dens** `APP_KEY`. Å utlede fra den mottakende maskinens nøkkel gir et passord restic avviser, noe som ser nøyaktig ut som et ødelagt arkiv uten å være det. Det er den vanlige grunnen til at `restic check` på et mottatt arkiv spør om passordet igjen og igjen.

Fordi gjenopprettingsdefinisjoner ligger **inne i** hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopiert repo-mappe fullstendig selvstendig, så settet pluss repoet er alt en bare-metal-gjenoppretting trenger.

## Hente en databasedump tilbake {#database-dumps}

En databasedump er sitt eget gjenopprettingspunkt i container-repositoriet, med etiketten `dbdump:<container>` og den ene filen `/dbdump/<container>.sql`. BombVault lister, laster ned og importerer dem under **Sikkerhetskopier**; under står de samme stegene med restic alene, til dagen BombVault ikke er der.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Etikettene `dbversion:` og `dbname:` på hver dump sier hvilken serverversjon den kommer fra, og hvilke databaser den inneholder. En komplett fil slutter med `-- PostgreSQL database cluster dump complete` eller `-- Dump completed`.

Importer den i en container med samme eller nyere versjon (PostgreSQL), eller samme hovedversjon (MySQL og MariaDB), startet én gang med tom datamappe slik at den initialiserer seg. Verten trenger ingen databaseklient, containeren har en:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

For én enkelt database ut av en full dump tar MySQL og MariaDB `--one-database <name>` på klientkommandoen. En PostgreSQL-dump har én seksjon per database, hver innledet med linjen `\connect <name>`: kopier den seksjonen til sin egen fil og importer den med `-d <name>` etter at databasen er opprettet.

!!! warning "En dump tatt som root bringer med seg serverens brukere"
    En full MySQL- eller MariaDB-dump tatt som root inneholder systemdatabasen `mysql`, så import erstatter den nye serverens kontoer, root-passordet inkludert, med dem fra dumpen. På PostgreSQL er `role ... already exists` for brukeren containeren selv opprettet, forventet og ufarlig.
