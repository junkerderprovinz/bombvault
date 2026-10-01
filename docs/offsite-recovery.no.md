# Ekstern lagring og gjenoppretting

Lokale sikkerhetskopier beskytter deg mot en tapt container eller en dårlig oppdatering. Ekstern replikering og et testet gjenopprettingssett beskytter deg mot hele boksen, løsepengevirus eller en brann. Denne siden dekker å replikere eksternt, å gjøre den kopien manipuleringssikker, å bevise at du kan gjenopprette, og å komme deg tilbake når selve BombVault er borte.

## Ekstern replikering

Behold den raske lokale sikkerhetskopien og legg til én eller flere eksterne replikaer. Sett et repo per domene på **Innstillinger, Off-site**-siden. BombVault replikerer nye øyeblikksbilder dit med `restic copy` på best-effort-basis, så en ekstern hikke feiler aldri den lokale sikkerhetskopien. I denne formen forblir det lokale repoet primært og det eksterne repoet er en replika, men et domenes primære repo trenger slett ikke være lokalt; se [Eksterne primære arkiver](#remote-primary-repositories) nedenfor for å sikkerhetskopiere rett til S3, en rest-server osv. i stedet for å replikere dit.

- **Flere eksterne mål per domene.** Hvert domene (containere, VM-er, flash, config, filsett og ZFS-datasett) kan replikere til flere eksterne destinasjoner samtidig, ikke bare én, så du kan for eksempel beholde en rest-server på en venns boks og en S3-bucket parallelt. Legg til ekstra mål på Innstillinger, Off-site, hvert med sitt eget repository, sin S3-lagringsklasse, append-only-flagg, oppbevaring og vekstbudsjett. Et eksisterende enkelt ekstern-oppsett overføres urørt som det første målet, og hvert mål i et domene replikeres på det domenets eksterne tidsplan.
- **Ekstern tidsplan per domene** (redigert sammen med hver annen tidsplan på Innstillinger, Tidsplaner): la den stå tom for å replikere etter hver lokale sikkerhetskopi, eller sett en kadens (for eksempel `weekly Sun 03:00`) for å sende eksternt sjeldnere enn du sikkerhetskopierer lokalt. En **Replikér nå**-knapp dekker på-forespørsel-kjøringer.
- **Ekstern oppbevaring** ligger på Innstillinger, Oppbevaring så du kan beholde eksterne kopier lenger som et arkiv. La policyen stå helt på null for aldri å auto-trimme eksterne øyeblikksbilder.
- **Båndbreddegrenser** (Innstillinger, Off-site) begrenser resticts opplastings-/nedlastingshastighet så replikering ikke metter WAN-et ditt.
- En **replikeringsindikator** viser hvilket domene som replikerer mens det pågår (på siden sin og på Dashboardet). Det er en aktiv indikator, ikke en prosentbjelke, fordi `restic copy` ikke eksponerer noen maskinlesbar fremdrift.

!!! note "Gjenopprett rett fra ekstern"
    Hver sikkerhetskopileser har en **Lokal / Ekstern**-bryter, så hvis et lokalt repo går tapt eller blir korrupt, kan du liste og gjenopprette direkte fra den eksterne replikaen. Sletting er per kilde: å fjerne en sikkerhetskopi påvirker bare kopien du ser på.

## Eksterne primære arkiver {#remote-primary-repositories}

Et domenes sti for sikkerhetskopi (Innstillinger, Lagring) er ikke begrenset til en lokal mappe: pek den rett mot et restic-fjernarkiv (`s3:...`, `rest:http://vert:8000/arkiv`, `b2:...`, `sftp:bruker@vert:/arkiv`, `rclone:ekstern:bucket/sti`), så sikkerhetskopierer BombVault direkte dit, uten egen lokal kopi og uten replikeringssteg. Det er en virkelig annen form enn off-site-replikeringen over: der er det lokale arkivet det primære, og off-site-arkivet er et arkiv av det etter beste evne; her **er** fjernarkivet det primære, og det er den eneste kopien så lenge du ikke også setter opp off-site-replikering (eller et andre fjernarkiv) for det domenet.

Hvert av de seks stifeltene (Containere, Virtuelle maskiner, Flash, Konfigurasjon, Filer, ZFS-datasett) har en bryter **Lokal / Ekstern** rett ved siden av:

- **Lokal** viser den kjente mappeutforskeren.
- **Ekstern** bytter den ut med et enkelt URL-felt, pluss en knapp som åpner den samme dialogen for tilkoblingstest og påloggingsdetaljer som off-site-destinasjoner bruker, bare stilt inn for dette primære arkivet. Derfra får du:
    - **En tilkoblingstest** mot den virkelige stien, før du stoler på den.
    - **Båndbreddegrenser** (opplasting og nedlasting), slik at en planlagt sikkerhetskopi til et eksternt primærarkiv ikke metter WAN-linjen din: de samme restic-flaggene `--limit-upload` og `--limit-download` som off-site-replikeringen bruker, nå anvendt på selve sikkerhetskopien.
    - **Append-only-beskyttelse (uforanderlighet)**, kontrollert med den samme aktive manipulasjonstesten (en ekte DELETE-sonde mot den andre siden) som off-site-destinasjoner får. Er den på, nekter BombVault å beskjære arkivet selv: siden ingen egen lokal kopi står bak, må ikke påloggingsdetaljene på denne maskinen kunne slette den eneste kopien av sikkerhetskopien.
    - **En alarm for vekstbudsjettet**, hentet fra den samme utviklingen i arkivstørrelse som Lagringskortet allerede følger.

Ingenting av dette er påkrevd: en håndskrevet ekstern sti uten lagrede sikkerhetsinnstillinger sikkerhetskopierer nøyaktig som før (ubegrenset båndbredde, kan beskjæres, ingen budsjettalarm). Sikkerhetsdialogen finnes for når du vil ha den samme beskyttelsen som en off-site-kopi får, uten å måtte opprette en off-site-destinasjon bare for det.

!!! note "Sky- og REST-påloggingsdetaljer deles"
    Et eksternt primærarkiv godkjennes med de samme S3-/REST-detaljene som er satt opp under Innstillinger, Skytilgang, Delt skylegitimasjon. Det finnes ikke et eget lager for påloggingsdetaljer til primære arkiver.

### SMB og WebDAV uten vertsmontering {#smb-webdav}

Innstillinger, Skytilgang, rclone har et skjema for en Windows- eller Samba-deling og for en WebDAV-server (Nextcloud, ownCloud, SharePoint eller en annen). Fyll inn et kort navn, verten og delingen (SMB) eller URL-en og servertypen (WebDAV), brukeren og passordet, så skriver BombVault rclone-seksjonen for deg. rclone tilslører selv passordet før det lagres; legger du til en destinasjon med et navn som finnes fra før, erstatter den den seksjonen i stedet for å legge til en ny.

Skjemaet svarer med den ferdige plasseringen, for eksempel `rclone:nas:backups`. Sett den inn i en sikkerhetskopisti eller et eksternt mål og legg til en undermappe om du vil ha en (`rclone:nas:backups/bombvault`). Delingen er det første leddet i stien, ikke en del av navnet.

Dette er en bedre vei enn å montere delingen på Unraid: restic fraråder å ha et repository på en montert CIFS-deling, og her monteres ingenting. NFS er ikke med i skjemaet fordi verken restic eller rclone har en NFS-backend; for NFS monterer du eksporten på verten og peker en sikkerhetskopisti mot den.

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

## Koble instanser sammen {#pairing}

Mottakere, hentekilder, Instanser-siden og Mesh-ekstern snakker alle med en annen BombVault. De gjør det som medlemmer av én paringsgruppe, og en instans blir med i gruppen med tolv ord.

Åpne **Innstillinger → Paring** på den første instansen og trykk på **Generer frase** i paringskortene. Tolv ord dukker opp i et vindu med en **Kopier**-knapp. Åpne det samme stedet på hver av de andre instansene, trykk på **Angi frase** og lim inn eller tast dem inn, eller trykk på **Lim inn** i det vinduet. Et ord som ikke finnes på listen, blir navngitt med plasseringen sin mens du taster, og det siste ordet bærer en sjekksum, så et feiltastet eller ombyttet ord fanges opp før noe blir paret. Generer frasen på bare én instans: to instanser som begge genererer en frase, danner to atskilte grupper. Melder ingen seg i løpet av et minutt, tilbyr fanen to veier ut: vis ordene på nytt for å skrive dem inn der borte, eller skriv inn ordene til den andre instansen og bli med i gruppen dens i ett steg. Paring fungerer uten innloggingspassord, men sett et: uten det kan alle som kan åpne dette webgrensesnittet, lese ordene og via gruppen få tak i restic-passordet til hver instans i gruppen. Paringskortet sier fra om dette til et passord er satt. Med passord ber en ny visning av frasen om det. **Forlat gruppen** tar en instans ut igjen.

Alle som kjenner ordene kan bli med i gruppen, så behandle dem som et passord.

**Hvordan medlemmene når hverandre.** Hver instans lærer sin egen adresse på nettverket fra nettleseren din i det du logger inn, vist i relaykortet som **Denne instansen i nettverket ditt**; rett den der hvis en revers-proxy eller en uvanlig port ligger foran. På samme nettverk kunngjør medlemmene den adressen via multicast og snakker direkte sammen, og der multicast ikke kommer gjennom et containernettverk, som Dockers standard bridge-nettverk, søker en instans i stedet gjennom sitt eget subnett etter de andre med et signert kall bare et gruppemedlem kan svare på, så paringen likevel er ferdig på sekunder uten relay. Dukker det ikke opp noe, tar **Finner du den ikke?** under paringskortet imot en adresse for hånd, for et annet subnett eller en uvanlig port. Instanser på ulike nettverk går gjennom et relay, valgt på samme fane:

- **Prosjektrelay** (standarden): `relay.halleluja.design`, det samme relayet som KnightLoader også bruker. Ingenting å sette opp.
- **Eget relay**: containeren **BombVault Relay** fra Unraid Community Apps, eller en av instansene dine som allerede er tilgjengelig utenfra med **Fungere som relay** slått på. Den instansen svarer da på `/relay/connect` på sin egen adresse, bak revers-proxyen og sertifikatet den allerede har, og slipper bare inn din gruppe. Skriv inn relayets adresse på hver instans som skal bruke det.
- **Ingen relay**: medlemmer finner hverandre automatisk på samme nettverk, og ingen andre steder.

**Hva relayet ser.** Hvert kall mellom medlemmer er forseglet med AES-256-GCM under en nøkkel utledet fra de tolv ordene, og den nøkkelen forlater aldri instansene dine. Relayet får vite en hash som grupperer forbindelsene, hvilken instans en melding er til, hvor stor den er og når den passerer. Et direkte kall på det lokale nettverket er forseglet på samme måte og signert i tillegg, så ingenting avhenger av det selvsignerte sertifikatet en instans tilbyr.

**Hva som går over gruppen.** Poengkortene på Instanser-siden, en forespørsel om å sjekke ett domene nå, Mesh-tilbud om ekstern kopiering, og det en mottaker eller hentekilde trenger: den andre instansens repository-adresser og dens restic-passord. Sikkerhetskopidata går aldri denne veien, det går fortsatt rett til restic-backendene. Heller ikke APP_KEY: restic-passordet åpner bare den instansens repositorier og ingenting annet, verken lagrede hemmeligheter, økter eller gjenopprettingskoder.

**Oppføringer fra før paringen.** Instanser lagt til med et fleet-token, og mottakere og hentekilder satt opp med den andre instansens APP_KEY, blir stående etter oppdateringen og merkes **Par på nytt**. Mottakere og hentekilder fortsetter å virke: ved første oppstart erstatter BombVault hver lagrede APP_KEY med restic-passordet utledet fra den. Par begge instansene, rediger så oppføringen og velg instansen dens. En slik instans tar over sitt gamle kort så snart en instans med samme navn dukker opp i gruppen.

Det eneste stedet som fortsatt tar imot en APP_KEY for hånd, er [Gjenopprett fra et annet BombVault-repo](#restore-from-another-bombvault-repo), for tilfellet der den andre instansen er borte og ikke lenger kan svare i noen gruppe.

## Mottaker-dashboard (mottakssiden)

![Den mottakende siden, overvåket skrivebeskyttet, med en integritetssjekk kjørt på denne maskinen.](assets/screenshots/receiver.png)

*Den mottakende siden, overvåket skrivebeskyttet, med en integritetssjekk kjørt på denne maskinen.*

Alt ovenfor er *sende*-siden. På boksen som **mottar** uforanderlige eksterne kopier fra en annen BombVault, gir Mottaker-dashboardet deg uavhengig, skrivebeskyttet overvåking av disse repositoriene på mottaks-maskinvaren, så en stille feil i den andre enden ikke går ubemerket hen.

Slå på **Mottaker**-bryteren i Innstillinger for å avdekke en **Mottaker**-fane. Den er av som standard; aktiver den kun på en boks som faktisk mottar uforanderlige eksterne sikkerhetskopier. Registrer deretter et mottatt repository (skrivebeskyttet, åpnet med restic-passordet til den sendende instansen, som det får over [paringsgruppen](#pairing)) for å få:

- **Et øyeblikksbilde-inventar gruppert etter kilde**, så du kan se nøyaktig hvilke containere, VM-er og filsett som har landet.
- **Sist mottatt** per kilde, så du vet hvor fersk hver enkelt er.
- **En uavhengig `restic check`** kjørt på mottaks-maskinvaren, så integriteten verifiseres der dataene faktisk ligger, ikke bare hos senderen.
- **En dødmannsbryter:** et varsel når en kilde slutter å sende innenfor et vindu du setter.
- **Integritetsvarsler:** et varsel når en sjekk på mottakssiden feiler.

Mottakeren er strengt skrivebeskyttet. Den skriver aldri til det mottatte repositoriet, så den kan aldri bryte append-only-garantien senderen stoler på.

## Gjennomgått eksempel: to Unraid-maskiner, hele veien

Over beskrives delene. Her er ett komplett oppsett med ekte verdier, for deler er lettere å sette sammen når man har sett dem satt sammen én gang.

To maskiner: **TOWER** kjører containerne og sender sikkerhetskopiene, **VAULT** tar imot dem og håndhever uforanderligheten. Bytt ut med dine egne navn, adresser og delingsstier.

**1. Sett opp append-only-serveren på VAULT.** I BombVault på TOWER: gå til *Innstillinger → Off-site → veiledet oppsett*, velg **rest-server** og generer oppskriften. Kopier fanen **Unraid-mal (XML)**, lagre den på VAULT som `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, og velg deretter *Docker → Add Container* og **rest-server** fra mallisten. Skriv den viste `htpasswd`-linjen inn i `/mnt/user/appdata/rest-server/.htpasswd` på VAULT før du starter den. Engangspassordet vises én gang og lagres aldri, så kopier det nå. Den linjen bærer det samme passordet, allerede bcrypt-hashet for deg: klarteksten hører hjemme i REST-legitimasjonen på TOWER, den hashede linjen i `.htpasswd` på VAULT. Du skal ikke hashe noe selv.

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

**4. Se på VAULT hva som kommer inn.** Par de to boksene ([Koble instanser sammen](#pairing)), slå på *Innstillinger → Paring → Mottaker*, åpne fanen **Mottaker**, og registrer arkivet skrivebeskyttet med TOWER som sendende instans.

!!! warning "Plasseringen er en sti **inne i** containeren, skrevet relativt til vertsmonteringen"
    Skriv inn `user/appdata/rest-server/bombvault-containers/containers`, **ikke** `/mnt/user/appdata/…`. BombVault kjører i en container der vertens `/mnt` er montert et annet sted; en absolutt vertssti finnes ikke der. Limer du inn en, forteller BombVault deg nå den relative stien du skal bruke i stedet.

    VAULT får TOWERs restic-passord over gruppen når du lagrer; ingen behøver å taste inn en nøkkel.

**5. Gjør det gjensidig, hvis du vil.** Gjenta de samme fem trinnene motsatt vei: en rest-server på TOWER som tar imot VAULTs kopi. Da håndhever hver maskin uforanderligheten for den andre, og ingen kan slette den andres sikkerhetskopier.

## Veiledet gjenoppretting

En egen **Gjenoppretting**-fane leder en ny eller gjenoppbygd installasjon gjennom katastrofetilfellet, på ett sted:

1. **Gjenoppretter BombVaults egne innstillinger først**, så sikkerhetskopistiene, eksterne målene og legitimasjonen resten av flyten trenger, kommer forhåndsutfylt (brukt via en selv-omstart over Docker-socketen, så den kjørende innstillingsdatabasen aldri overskrives under en åpen handle).
2. **Sjekker at BombVault kan lese sikkerhetskopiene dine** (krypteringsnøkkel-fellen først).
3. Lar deg **peke mot ditt eksisterende repo** (lokalt eller eksternt).
4. **Oppdager** containerne, VM-ene, filsettene og ZFS-datasettene lagret i det.
5. **Gjenoppretter containerne og VM-ene på én gang** (la stå stoppet, så du starter dem bevisst) og viser filsettene og ZFS-elementene som du gjenoppretter ett om gangen; ZFS-elementer kommer tilbake slått av. Gjenopprettingssettet ditt er ett klikk unna.

!!! tip "Planlagt migrering versus katastrofe"
    Veiledet gjenoppretting gjenoppretter BombVaults egne innstillinger fra en sikkerhetskopi. For en *planlagt* flytting til en ny boks kan du i stedet ta med konfigurasjonen din direkte via kortet **Eksporter og importer innstillinger** (en portabel JSON-fil). Se [Konfigurasjon](configuration.md#portable-settings-export-and-import).

### Gjenopprett fra et annet BombVault-repo {#restore-from-another-bombvault-repo}

Et separat kort på **Gjenoppretting**-fanen åpner et *annet* BombVault-instans' repo (en deling montert under `/mnt`, eller en fjern-URL) med **den instansens `APP_KEY`**, i en engangs, skrivebeskyttet økt. Bla gjennom containerne, VM-ene og filsettene lagret der, velg et øyeblikksbilde og gjenopprett det, og det gjenopprettede objektet blir en normal lokal container, VM eller filsett. Ingenting skrives noensinne til det andre repoet, og dine egne sikkerhetskopiinnstillinger forblir urørte (økten lever i minnet og utløper av seg selv). Å flytte en container fra server A til server B betyr ikke lenger å peke om repo-innstillingene dine og reversere dem etterpå. Dette kortet er for én gang: det åpner en økt, gjenoppretter det du velger, og glemmer den andre instansen. Vil du heller ha en fast ordning, der denne boksen etter en tidsplan henter en annen instans' øyeblikksbilder inn i sitt eget repository, er det fanen **Henting** på siden **Instanser**.

## Gjenopprettingssett for krypteringsnøkkel

Dette er delen som gjør katastrofegjenoppretting mulig selv når det ikke finnes en kjørende BombVault.

Ett klikk laster ned **hovednøkkelen**, det **utledede restic-passordet** og de **nøyaktige repo-plasseringene og -kommandoene**, så du kan gjenopprette rett med restic-CLI-en på en hvilken som helst maskin. En Dashboard-påminnelse maser til du har lagret det.

!!! danger "Oppbevar gjenopprettingssettet bort fra serveren"
    Settet inneholder hemmeligheten som dekrypterer sikkerhetskopiene dine. Oppbevar det et trygt sted og adskilt fra serveren (en passordbehandler, en utskrevet kopi i et safe). Mister du både BombVault og `APP_KEY` uten et gjenopprettingssett, kan ikke de krypterte sikkerhetskopiene dine gjenopprettes.

!!! warning "Det nyeste snapshotet er ikke alltid det som skal gjenopprettes"
    Siden restic 0.17 viser `restic snapshots` størrelsen på hvert snapshot. Etter datatap kan det nyeste snapshotet være det tømte, så ikke gjenopprett et snapshot som er mye mindre enn de før det. Etter løsepengevirus kan det være det krypterte i vanlig størrelse. Kjører BombVault fortsatt, se først på siden **Avvik**: den nevner den siste gode sikkerhetskopien. En gjenoppretting trenger ingen avviksdata fra BombVault, og oppbevaringspausen beholder bare flere snapshots.

### Forsegle settet

Har du slått på age-kryptering for de vanlige eksportene (Innstillinger), forsegles settet også med den og lastes ned som `bombvault-recovery-kit.md.age`. Det er ASCII-armored i stedet for binært, så det er fortsatt vanlig tekst: å lime det inn i en passordbehandler eller skrive det ut fungerer akkurat som før, innholdet kan bare ikke leses uten nøkkelen din.

!!! warning "Ikke oppbevar age-nøkkelen i settet"
    Du trenger den **private** age-nøkkelen din for å åpne et forseglet sett. Oppbevar den et sted som ikke er avhengig av selve settet, ellers har du to ting å gjenopprette i stedet for én. Forsegling lønner seg når settet ligger et sted du ikke har full kontroll over (en delt passordbehandler, notater i skyen, en utskrift på et kontor); et sett i ditt eget safe er allerede beskyttet av safet.

    Med kryptering på og ingen brukbar mottaker satt opp nektes nedlastingen helt. BombVault faller aldri tilbake på å gi ut hovednøkkelen i klartekst.

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
