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

## Eksterne primære arkiver {#remote-primary-repositories}

Et domenes sti for sikkerhetskopi (Innstillinger, Stier og lagring) er ikke begrenset til en lokal mappe: pek den rett mot et restic-fjernarkiv (`s3:...`, `rest:http://vert:8000/arkiv`, `b2:...`, `sftp:bruker@vert:/arkiv`, `rclone:ekstern:bucket/sti`), så sikkerhetskopierer BombVault direkte dit, uten egen lokal kopi og uten replikeringssteg. Det er en virkelig annen form enn off-site-replikeringen over: der er det lokale arkivet det primære, og off-site-arkivet er et arkiv av det etter beste evne; her **er** fjernarkivet det primære, og det er den eneste kopien så lenge du ikke også setter opp off-site-replikering (eller et andre fjernarkiv) for det domenet.

Hvert av de fem stifeltene (Containere, Virtuelle maskiner, Flash, Konfigurasjon, Filer) har en bryter **Lokal / Ekstern** rett ved siden av:

- **Lokal** viser den kjente mappeutforskeren.
- **Ekstern** bytter den ut med et enkelt URL-felt, pluss en knapp som åpner den samme dialogen for tilkoblingstest og påloggingsdetaljer som off-site-destinasjoner bruker, bare stilt inn for dette primære arkivet. Derfra får du:
    - **En tilkoblingstest** mot den virkelige stien, før du stoler på den.
    - **Båndbreddegrenser** (opplasting og nedlasting), slik at en planlagt sikkerhetskopi til et eksternt primærarkiv ikke metter WAN-linjen din: de samme restic-flaggene `--limit-upload` og `--limit-download` som off-site-replikeringen bruker, nå anvendt på selve sikkerhetskopien.
    - **Append-only-beskyttelse (uforanderlighet)**, kontrollert med den samme aktive manipulasjonstesten (en ekte DELETE-sonde mot den andre siden) som off-site-destinasjoner får. Er den på, nekter BombVault å beskjære arkivet selv: siden ingen egen lokal kopi står bak, må ikke påloggingsdetaljene på denne maskinen kunne slette den eneste kopien av sikkerhetskopien.
    - **En alarm for vekstbudsjettet**, hentet fra den samme utviklingen i arkivstørrelse som Lagringskortet allerede følger.

Ingenting av dette er påkrevd: en håndskrevet ekstern sti uten lagrede sikkerhetsinnstillinger sikkerhetskopierer nøyaktig som før (ubegrenset båndbredde, kan beskjæres, ingen budsjettalarm). Sikkerhetsdialogen finnes for når du vil ha den samme beskyttelsen som en off-site-kopi får, uten å måtte opprette en off-site-destinasjon bare for det.

!!! note "Sky- og REST-påloggingsdetaljer deles"
    Et eksternt primærarkiv godkjennes med de samme S3-/REST-detaljene som er satt opp under Innstillinger, Off-site, Skypåloggingsdetaljer. Det finnes ikke et eget lager for påloggingsdetaljer til primære arkiver.

## Uforanderlig (append-only) ekstern

Flagg et eksternt repo append-only så løsepengevirus, eller en kompromittert host, ikke kan slette eller skrive om sikkerhetskopiene dine. Den andre siden (en `restic/rest-server` som kjører i `--append-only`-modus) **håndhever** det. BombVault kun **verifiserer** det og viser aldri grønt på en konfigurasjonspåstand alene.

**Veiledet ekstern-oppsett**-veiviseren leder deg fra backend-valg (rest-server / rclone / S3) gjennom et klar-til-lim rest-server-deploysnippet, en tilkoblingstest, uforanderlig-bryteren (som kjører tamper-testen umiddelbart) og en oppbevaringsstrategi, så append-only ekstern er tilgjengelig uten å håndredigere konfigurasjoner.

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

## DR-øvelser

BombVault tilbyr to nivåer av bevis for at sikkerhetskopiene dine faktisk er gjenopprettbare, ikke bare til stede.

- **Gjenopprettingsverifiseringsøvelser (lokale).** BombVault kjører jevnlig `restic check --read-data-subset` (avgrenset, aldri en disk-fyllende full gjenoppretting) og viser et *sist verifisert gjenopprettbar*-merke per domene. Kadensen ligger på Innstillinger, Tidsplaner; merket på Innstillinger, Integritet.
- **DR-øvelser (ekstern).** BombVault gjenoppretter et ekte mål fra det eksterne repoet inn i en engangs-sandkasse, verifiserer det fil-for-fil og byte-for-byte, og rydder deretter opp. Dette beviser at du kan komme deg tilbake fra ekstern, ikke bare at repoet svarer.

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

**1. Sett opp append-only-serveren på VAULT.** I BombVault på TOWER: gå til *Innstillinger → Eksternt → veiledet oppsett*, velg **rest-server** og generer oppskriften. Kopier fanen **Unraid-mal (XML)**, lagre den på VAULT som `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, og velg deretter *Docker → Add Container* og **rest-server** fra mallisten. Skriv den viste `htpasswd`-linjen inn i `/mnt/user/appdata/rest-server/.htpasswd` på VAULT før du starter den. Engangspassordet vises én gang og lagres aldri, så kopier det nå.

    La `--append-only` bli stående i OPTIONS-feltet. Det er hele poenget: uten det er VAULT en vanlig deling igjen.

**2. Pek det eksterne arkivet dit på TOWER.** Arkivets adresse følger mønsteret oppskriften skriver ut:

    rest:http://VAULT:8000/bombvault-containers/containers

Første ledd i stien er htpasswd-brukeren, det andre er arkivet. Skriv inn den genererte brukeren og passordet som destinasjonens REST-legitimasjon, og kjør **tilkoblingstesten**.

**3. Slå på ”Uforanderlig” på TOWER.** Manipulasjonstesten kjører med én gang og må si *beskyttet*. Hva svarene betyr:

| Resultat | Hva som skjedde |
| --- | --- |
| **beskyttet** | VAULT avviste slettingen. Det er den eneste beståtte tilstanden. |
| **IKKE beskyttet** | VAULT godtok en sletting. `--append-only` mangler eller er fjernet. |
| **uavklart** | Verken eller. Som regel er adressen ikke den restic selv bruker, eller legitimasjonen er endret. Ingenting registreres, og ingen varsling utløses. |

**4. Se på VAULT hva som kommer inn.** Slå på *Innstillinger → Mottaker*, åpne fanen **Mottaker**, og registrer arkivet skrivebeskyttet.

!!! warning "Plasseringen er en sti **inne i** containeren, skrevet relativt til vertsmonteringen"
    Skriv inn `user/appdata/rest-server/bombvault-containers/containers`, **ikke** `/mnt/user/appdata/…`. BombVault kjører i en container der vertens `/mnt` er montert et annet sted; en absolutt vertssti finnes ikke der. Limer du inn en, forteller BombVault deg nå den relative stien du skal bruke i stedet.

    **Sendende APP_KEY** er TOWERs nøkkel, ikke VAULTs. Du finner den på TOWER under *Innstillinger → System*.

**5. Gjør det gjensidig, hvis du vil.** Gjenta de samme fem trinnene motsatt vei: en rest-server på TOWER som tar imot VAULTs kopi. Da håndhever hver maskin uforanderligheten for den andre, og ingen kan slette den andres sikkerhetskopier.

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
