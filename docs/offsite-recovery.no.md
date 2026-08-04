# Ekstern lagring og gjenoppretting

Lokale sikkerhetskopier beskytter deg mot en tapt container eller en dårlig oppdatering. Ekstern replikering og et testet gjenopprettingssett beskytter deg mot hele boksen, løsepengevirus eller en brann. Denne siden dekker å replikere eksternt, å gjøre den kopien manipuleringssikker, å bevise at du kan gjenopprette, og å komme deg tilbake når selve BombVault er borte.

## Ekstern replikering

Behold den raske lokale sikkerhetskopien og legg til én eller flere eksterne replikaer. Sett et repo per domene på **Innstillinger, Ekstern**-fanen. BombVault replikerer nye øyeblikksbilder dit med `restic copy` på best-effort-basis, så en ekstern hikke feiler aldri den lokale sikkerhetskopien. Det lokale repoet forblir primært.

- **Flere eksterne mål per domene.** Hvert domene (containere, VM-er, flash, config og filsett) kan replikere til flere eksterne destinasjoner samtidig, ikke bare én, så du kan for eksempel beholde en rest-server på en venns boks og en S3-bucket parallelt. Legg til ekstra mål på Innstillinger, Ekstern, hvert med sitt eget repository, sin S3-lagringsklasse, append-only-flagg, oppbevaring og vekstbudsjett. Et eksisterende enkelt ekstern-oppsett overføres urørt som det første målet, og hvert mål i et domene replikeres på det domenets eksterne tidsplan.
- **Ekstern tidsplan per domene** (redigert sammen med hver annen tidsplan på Innstillinger, Tidsplaner): la den stå tom for å replikere etter hver lokale sikkerhetskopi, eller sett en kadens (for eksempel `weekly Sun 03:00`) for å sende eksternt sjeldnere enn du sikkerhetskopierer lokalt. En **Replikér nå**-knapp dekker på-forespørsel-kjøringer.
- **Ekstern oppbevaring** ligger på Innstillinger, Ekstern så du kan beholde eksterne kopier lenger som et arkiv. La policyen stå helt på null for aldri å auto-trimme eksterne øyeblikksbilder.
- **Båndbreddegrenser** (Innstillinger, Ekstern) begrenser resticts opplastings-/nedlastingshastighet så replikering ikke metter WAN-et ditt.
- En **replikeringsindikator** viser hvilket domene som replikerer mens det pågår (på siden sin og på Dashboardet). Det er en aktiv indikator, ikke en prosentbjelke, fordi `restic copy` ikke eksponerer noen maskinlesbar fremdrift.

!!! note "Gjenopprett rett fra ekstern"
    Hver sikkerhetskopileser har en **Lokal / Ekstern**-bryter, så hvis et lokalt repo går tapt eller blir korrupt, kan du liste og gjenopprette direkte fra den eksterne replikaen. Sletting er per kilde: å fjerne en sikkerhetskopi påvirker bare kopien du ser på.

## Uforanderlig (append-only) ekstern

Flagg et eksternt repo append-only så løsepengevirus, eller en kompromittert host, ikke kan slette eller skrive om sikkerhetskopiene dine. Den andre siden (en `restic/rest-server` som kjører i `--append-only`-modus) **håndhever** det. BombVault kun **verifiserer** det og viser aldri grønt på en konfigurasjonspåstand alene.

**Veiledet ekstern-oppsett**-veiviseren leder deg fra backend-valg (rest-server / rclone / S3) gjennom et klar-til-lim rest-server-deploysnippet, en tilkoblingstest, uforanderlig-bryteren (som kjører tamper-testen umiddelbart) og en oppbevaringsstrategi, så append-only ekstern er tilgjengelig uten å håndredigere konfigurasjoner.

!!! warning "Uforanderlige repoer beskjæres aldri fra denne boksen"
    En uforanderlig ekstern beskjærer bevisst aldri gamle øyeblikksbilder. Sett en **vekstbudsjett-alarm** for den så du blir varslet før repo-størrelsen løper løpsk.

## Tamper-test

BombVault beviser jevnlig append-only-garantien ved faktisk å forsøke en sletting mot det eksterne repoet, rettet mot et ikke-eksisterende objekt:

- **Avvist** betyr beskyttet.
- **Akseptert** betyr ikke beskyttet.
- Et **usikkert** resultat (server unåbar, autentiseringsfeil) vender aldri den lagrede dommen.

En ekte beskyttet-til-ubeskyttet-vending utløser et enkelt varsel.

## DR-øvelser

BombVault tilbyr to nivåer av bevis for at sikkerhetskopiene dine faktisk er gjenopprettbare, ikke bare til stede.

- **Gjenopprettingsverifiseringsøvelser (lokale).** BombVault kjører jevnlig `restic check --read-data-subset` (avgrenset, aldri en disk-fyllende full gjenoppretting) og viser et *sist verifisert gjenopprettbar*-merke per domene. Kadensen ligger på Innstillinger, Tidsplaner; merket på Innstillinger, Integritet.
- **DR-øvelser (ekstern).** BombVault gjenoppretter et ekte mål fra det eksterne repoet inn i en engangs-sandkasse, verifiserer det fil-for-fil og byte-for-byte, og rydder deretter opp. Dette beviser at du kan komme deg tilbake fra ekstern, ikke bare at repoet svarer.

**Poengkortet for løsepengevirusbeskyttelse** på Dashboardet ruller dette opp i en grønn / gul / rød holdning per domene, med en aldersstemplet sjekkliste (ekstern konfigurert, append-only verifisert, replikering oppdatert, gjenopprettingsøvelse bestått, kryptering på, beskjæringsstrategi satt). Hver rød rad dyplenker til fiksen, og kortet blir bare grønt på verifiserte fakta.

## Mottaker-dashboard (mottakssiden)

Alt ovenfor er *sende*-siden. På boksen som **mottar** uforanderlige eksterne kopier fra en annen BombVault, gir Mottaker-dashboardet deg uavhengig, skrivebeskyttet overvåking av disse repositoriene på mottaks-maskinvaren, så en stille feil i den andre enden ikke går ubemerket hen.

Slå på **Mottaker**-bryteren i Innstillinger for å avdekke en **Mottaker**-fane. Den er av som standard; aktiver den kun på en boks som faktisk mottar uforanderlige eksterne sikkerhetskopier. Registrer deretter et mottatt repository (skrivebeskyttet, åpnet med den sendende instansens nøkkel) for å få:

- **Et øyeblikksbilde-inventar gruppert etter kilde**, så du kan se nøyaktig hvilke containere, VM-er og filsett som har landet.
- **Sist mottatt** per kilde, så du vet hvor fersk hver enkelt er.
- **En uavhengig `restic check`** kjørt på mottaks-maskinvaren, så integriteten verifiseres der dataene faktisk ligger, ikke bare hos senderen.
- **En dødmannsbryter:** et varsel når en kilde slutter å sende innenfor et vindu du setter.
- **Integritetsvarsler:** et varsel når en sjekk på mottakssiden feiler.

Mottakeren er strengt skrivebeskyttet. Den skriver aldri til det mottatte repositoriet, så den kan aldri bryte append-only-garantien senderen stoler på.

## Veiledet gjenoppretting

En egen **Gjenoppretting**-fane leder en ny eller gjenoppbygd installasjon gjennom katastrofetilfellet, på ett sted:

1. **Gjenoppretter BombVaults egne innstillinger først**, så sikkerhetskopistiene, eksterne målene og legitimasjonen resten av flyten trenger, kommer forhåndsutfylt (brukt via en selv-omstart over Docker-socketen, så den kjørende innstillingsdatabasen aldri overskrives under en åpen handle).
2. **Sjekker at BombVault kan lese sikkerhetskopiene dine** (krypteringsnøkkel-fellen først).
3. Lar deg **peke mot ditt eksisterende repo** (lokalt eller eksternt).
4. **Oppdager** containerne, VM-ene og filsettene lagret i det.
5. **Gjenoppretter dem alle** (la stå stoppet, så du starter dem bevisst), med gjenopprettingssettet ditt ett klikk unna.

!!! tip "Planlagt migrering versus katastrofe"
    Veiledet gjenoppretting gjenoppretter BombVaults egne innstillinger fra en sikkerhetskopi. For en *planlagt* flytting til en ny boks kan du i stedet ta med konfigurasjonen din direkte via kortet **Eksporter og importer innstillinger** (en portabel JSON-fil). Se [Konfigurasjon](configuration.md#portable-settings-export-and-import).

### Gjenopprett fra et annet BombVault-repo

Et separat kort på **Gjenoppretting**-fanen åpner et *annet* BombVault-instans' repo (en deling montert under `/mnt`, eller en fjern-URL) med **den instansens `APP_KEY`**, i en engangs, skrivebeskyttet økt. Bla gjennom containerne, VM-ene og filsettene lagret der, velg et øyeblikksbilde og gjenopprett det, og det gjenopprettede objektet blir en normal lokal container, VM eller filsett. Ingenting skrives noensinne til det andre repoet, og dine egne sikkerhetskopiinnstillinger forblir urørte (økten lever i minnet og utløper av seg selv). Å flytte en container fra server A til server B betyr ikke lenger å peke om repo-innstillingene dine og reversere dem etterpå. Live server-til-server-føderasjon er eksplisitt utenfor omfang; dette er en bevisst engangs-henting.

## Gjenopprettingssett for krypteringsnøkkel

Dette er delen som gjør katastrofegjenoppretting mulig selv når det ikke finnes en kjørende BombVault.

Ett klikk laster ned **hovednøkkelen**, det **utledede restic-passordet** og de **nøyaktige repo-plasseringene og -kommandoene**, så du kan gjenopprette rett med restic-CLI-en på en hvilken som helst maskin. En Dashboard-påminnelse maser til du har lagret det.

!!! danger "Oppbevar gjenopprettingssettet bort fra serveren"
    Settet inneholder hemmeligheten som dekrypterer sikkerhetskopiene dine. Oppbevar det et trygt sted og adskilt fra serveren (en passordbehandler, en utskrevet kopi i et safe). Mister du både BombVault og `APP_KEY` uten et gjenopprettingssett, kan ikke de krypterte sikkerhetskopiene dine gjenopprettes.

Fordi gjenopprettingsdefinisjoner ligger **inne i** hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopiert repo-mappe fullstendig selvstendig, så settet pluss repoet er alt en bare-metal-gjenoppretting trenger.
