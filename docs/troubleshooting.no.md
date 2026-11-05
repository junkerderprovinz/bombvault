# Feilsøking

En kort FAQ. For den fullstendige feilsøkingstabellen på vertssiden for VM-er over SSH (permission-denied, verifisering av vertsnøkkel, manglende malvariabler og mer), se [veiledningen for VM-sikkerhetskopiering over SSH](vm-backup-ssh-setup.md).

## Noe er ikke riktig koblet opp {#something-is-not-wired-up-correctly}

Legg kortet **Host-integrasjon** til oversikten under **Tilpass**. Host-integrasjonssjekken sonderer hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer manglende deler, og **Sjekk nå** kjører den på nytt. Start her før du antar en feil: en manglende montering eller en vert som ikke kan nås, dukker opp umiddelbart.

## Jeg når ikke webgrensesnittet {#i-cannot-reach-the-web-ui}

BombVault serverer HTTPS rett ut av boksen på port `3443` (selvsignert sertifikat), så åpne `https://<your-unraid-ip>:3443`. Godta advarselen om det selvsignerte sertifikatet, eller sett BombVault bak en revers-proxy med ditt eget sertifikat. Hvis du kjører med `HTTP_ONLY=true`, serverer den ren HTTP på port `3000` i stedet (ment for bruk bak en proxy som avslutter TLS).

## Jeg mistet APP_KEY-en min {#i-lost-my-app_key}

`APP_KEY` utleder passordet til restic-depotet. Uten den, uten gjenopprettingssettet og uten en nøkkeldeponering i gruppen din kan ikke krypterte sikkerhetskopier gjenopprettes. Det er derfor oversikten viser **Lagre gjenopprettingssettet ditt** til du har lastet det ned. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md#recovery-kit). Generer en nøkkel med `openssl rand -hex 32` og oppbevar den utenfor serveren før du stoler på noen sikkerhetskopi.

## Jeg har glemt innloggingspassordet {#i-forgot-the-login-password}

Velg **Glemt passordet?** på innloggingssiden. BombVault skriver en kode på 12 tegn til containerloggen sin (på Unraid: Docker-fanen, BombVault-ikonet, **Logs**). Skriv den inn sammen med et nytt passord innen 10 minutter. Tilbakestillingen slår også av tofaktorautentisering, fjerner alle passnøkler, trekker tilbake alle API-tokener, MCP-nøkler og OAuth-pålogginger, avslutter alle andre økter og slår av telefonparing. Legg til passnøklene dine igjen under **Innstillinger, Sikkerhet**, slå telefonparingen på igjen der, og lag nye tokener og nøkler der du fortsatt trenger dem. Se [Sikkerhetsmodell](configuration.md#login).

## Sikkerhetskopiene har stoppet og sier at nøkkelen ikke lenger kan åpnes {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Denne serveren overtok en annen servers sikkerhetskopinøkkel i nødveiviseren, og `APP_KEY` ble endret etterpå. Den overtatte nøkkelen er forseglet med den gamle APP_KEY-en, så BombVault kan ikke åpne den og tar heller ingen sikkerhetskopier enn å skrive med feil nøkkel. **Krever oppmerksomhet** på oversikten nevner det. Sett APP_KEY tilbake til det den var, eller last opp gjenopprettingssettet på nytt på siden Gjenoppretting under **Nød**.

## En telefon havner på innloggingssiden {#a-phone-lands-on-the-login-page}

En paret telefon får økten sin over gruppen bare så lenge **Telefoner kan pare med gruppens tolv ord** er på under **Innstillinger, Sikkerhet**. En tilbakestilling av passordet slår det av. En app som er eldre enn serveren, får heller ingen økt før den er oppdatert. En server som er lagt til med adressen sin, spør alltid etter passordet. En telefon som er fjernet fra siden Instanser, kommer tilbake slått av når appen åpnes igjen. Slå på **Spør ut denne instansen** under **Overvåking** i detaljene for å slippe den inn.

## VM-sikkerhetskopiering vil ikke koble til {#vm-backup-will-not-connect}

VM-sikkerhetskopiering snakker med libvirt over SSH, aldri via en montering.

- Bekreft at SSH er aktivert på verten og at BombVaults offentlige nøkkel er autorisert i `/root/.ssh/authorized_keys` (Innstillinger, Integrasjoner, Verts-SSH viser nøkkelen og en knapp **Test tilkobling**).
- På et egendefinert `br0.x`-nettverk setter du `LIBVIRT_HOST` til din Unraid-LAN-IP (containeren kan ikke nå verten via `host.docker.internal` der). Slå på **Settings, Docker, Host access to custom networks**.
- Hvis du endret Unraids SSH-port, setter du `LIBVIRT_SSH_PORT` til å samsvare.
- På TrueNAS setter du `LIBVIRT_URI` med TrueNAS sin egen libvirt-socket, gir appen capabilityen `SETPCAP` og legger nøkkelen til root under **Credentials, Users** (se [TrueNAS SCALE](getting-started.md#truenas)).
- Full trinn-for-trinn-diagnose (nåbarhetstest, VLAN-ruting, `Permission denied (publickey)`, `Host key verification failed`) står i [veiledningen for VM-sikkerhetskopiering over SSH](vm-backup-ssh-setup.md).

## Et live-VM-øyeblikksbilde kjørte ikke {#a-live-vm-snapshot-did-not-run}

Live-øyeblikksbilder trenger disken på `/mnt/cache` (eller `/mnt/diskX`), ikke `/mnt/user`. Med qemu guest agent i VM-en blir øyeblikksbildet stilnet, og uten den er øyeblikksbildet krasjkonsistent. På en avslått VM faller live automatisk tilbake til pen. En pen sikkerhetskopi slår VM-en av, sikkerhetskopierer diskene og starter den så på nytt, så den er alltid konsistent. Den venter opptil ti minutter på at gjesten slår seg av, og tvinger den aldri av, så en Windows-gjest som installerer oppdateringer, ikke blir avbrutt halvveis. En gjest som fortsatt kjører etter det, får være i fred, og den sikkerhetskopien feiler med årsaken.

## En sikkerhetskopi feilet med "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Dette er vanligvis en foreldreløs restic-lås som ble igjen da containeren ble oppdatert eller startet på nytt midt i en operasjon. BombVault oppdager en påviselig foreldreløs lås, fjerner den med makt og prøver på nytt én gang, automatisk. Hvis den vedvarer, åpner du kortet **Kontroller sikkerhetskopier** under **Innstillinger, Verifisering** og velger **Lås opp** under **Mer** i raden til det berørte området for å fjerne en fastlåst lås for hånd. Et ekte problem dukker fortsatt opp i stedet for å bli skjult. Etter en omstart venter BombVault til en slik lås har gått ti minutter uten å bli fornyet. En restic som fortsatt kjører, for eksempel i en annen BombVault mot samme depot, fornyer låsen sin hvert femte minutt.

## Min eksterne kopi ble ikke tatt etter en sikkerhetskopi {#my-off-site-copy-did-not-happen-after-a-backup}

Ekstern replikering er best-effort av design, så et problem med den eksterne kopien feiler aldri den lokale sikkerhetskopien. Sjekk når målet kopierer: kortet **Kopier** sier **Etter hver sikkerhetskopi** eller nevner en egen plan, og **Innstillinger, Tidsplan, Off-site-kopier per område** kan gi et område en egen plan. Bruk **Replikér nå** i raden til området i kortet **Eksterne kopier** under **Innstillinger, Mål** for en kjøring på forespørsel, og følg med på statushodet på oversikten, som sier **Off-site-kopi pågår** mens den kjører. En mislykket kopi vises under **Krever oppmerksomhet**.

## Sertifikatet til en rest-server blir ikke godtatt {#a-rest-servers-certificate-is-not-trusted}

restic avviser et sertifikat det ikke kjenner, og feilmeldingen sier hvor det skal ligge. Kopier det selvsignerte sertifikatet til rest-serveren, eller sertifikatet til utstederen som signerte det, som PEM-fil til mappen `certs` i BombVaults datamappe (`/config/certs` i containeren, `appdata/bombvault/certs` på Unraid), og test på nytt. Omstart trengs ikke.

## En sikkerhetskopi sier at data utenfor Host Data ikke ble sikkerhetskopiert {#data-outside-host-data}

Containeren har data i en mappe BombVault ikke kan lese gjennom Host Data. Sikkerhetskopien lagrer definisjonen og hver mappe den når, farger linjen sin i aktivitetsloggen oransje og navngir mappene den utelot. To tilfeller er vanlige: et ixVolume på TrueNAS under `/mnt/.ix-apps` og et navngitt Docker-volum under `/var/lib/docker/volumes`, for eksempel databasevolumet i en compose-stack. Kortet **Sikkerhetskopierte mapper** på fanen **Innstillinger** for containeren viser hver slik mappe med stien inne i containeren og vertsstien, merket med rødt.

Et anonymt Docker-volum blir ikke meldt. Docker lager det selv for en mappe imaget oppgir og ingen har koblet til, gir det et navn på 64 heksadesimale sifre og legger det under `/var/lib/docker/volumes`. Som regel inneholder det en hurtigbuffer, så sikkerhetskopien forblir grønn, og kortet viser det bare med en merknad om at det er anonymt.

- **Du vil ha de dataene sikkerhetskopiert.** Koble mappen til en vertssti under Host Data, for eksempel en mappe i appdata-delingen din, så tar neste sikkerhetskopi den med.
- **Du trenger dem ikke.** Legg til en av de to stiene som en linje i kortet **Ekskluderingsmønstre** på samme fane: stien inne i containeren, for eksempel `/cache`, eller vertsstien varselet nevner, for eksempel `/var/lib/docker/volumes/jellyfin_cache/_data`. Fra neste sikkerhetskopi utelater varselet den mappen, og forhåndsvisningen under feltet sier hvorfor. En linje for en mappe over den virker også, et rent navn eller `/` gjør det ikke.

## En sikkerhetskopi står som Utsatt {#deferred-backups}

Appen i containeren var opptatt, så sikkerhetskopien venter og prøves igjen av seg selv. Linjen i aktivitetsloggen sier hvorfor og når neste forsøk er. Kortet **Når appen er opptatt** på fanen **Innstillinger** til containeren sier det samme og når ventetiden er over. Du trenger ikke gjøre noe. Vil du ta sikkerhetskopi med en gang, bruker du **Sikkerhetskopier nå**: en sikkerhetskopi du starter selv, venter aldri, heller ikke ved en avslutningskode 75.

- **Din egen sjekk.** La **Pre-sikkerhetskopikommando** avslutte med kode 75 mens appen er opptatt, for eksempel `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Enhver annen kode enn 0 får sikkerhetskopien til å feile som før, så et ødelagt skript dukker fortsatt opp under **Krever oppmerksomhet**.
- **Plex, Jellyfin eller Emby blir aldri spurt.** BombVault spør først serveren på containerens egen adresse og kjører, når det ikke kommer noe svar der, curl eller wget inne i containeren. Et image med ingen av dem kan ikke spørres gjennom seg selv. Loggen sier det da. Med **Ta sikkerhetskopi når serveren ikke svarer** på kjører en slik sikkerhetskopi likevel, med den av venter den.
- **Jellyfin eller Emby avviser.** Sjekk **API-nøkkel**. Den lages i serverens dashbord under API-nøkler.
- **Plex avviser.** BombVault leser tokenet fra `Preferences.xml` i mappen som er montert på `/config`. En server som aldri er logget inn, har ikke noe token og svarer bare der den tillater tilgang uten.
- **En pauset film holder sikkerhetskopien ventende hele natten.** Sett **Ignorer pausede økter etter (min)**, for eksempel til 30.
- **Trafikksjekken slipper aldri sikkerhetskopien gjennom.** Hev **Opptatt fra (KiB/s)** over det containeren sender når ingen bruker den. Rett etter at BombVault starter, venter sjekken til den har fulgt med på containeren i **Rolig i (min)** minutter.
- **Sikkerhetskopien kjører aldri om natten.** Med et vindu satt kommer forsøkene bare innenfor det. En ventetid som når fristen utenfor vinduet, slutter likevel ved fristen.

## En stor sikkerhetskopi ble drept fordi minnet tok slutt {#a-large-backup-was-killed-for-running-out-of-memory}

Sett **Minnegrense per sikkerhetskopi (MiB, 0 = ingen)** i kortet **Ytelse** under **Innstillinger, System** godt under det containeren får bruke. Nær grensen frigjør restic minne oftere, noe som koster litt fart, så sikkerhetskopien går tregere i stedet for å bli drept.

## En gjenoppretting avbrøt før den startet {#a-restore-aborted-before-it-started}

Før noe stoppes eller fjernes, kjører gjenopprettingen en konfliktsjekk på forhånd: den verifiserer at containerens statiske IP og publiserte vertsporter er ledige. Hvis en annen container allerede holder en, avbryter den med en tydelig melding du kan handle på, i stedet for å etterlate en halvferdig gjenoppretting. Frigjør den motstridende porten eller IP-en, og prøv på nytt.

Finnes ikke containerens nettverk, for eksempel fordi TrueNAS-appen ble slettet, nevner sjekken før gjenopprettingen nettverket, og gjenopprettingen avviser før den endrer noe. Installer appen på nytt i TrueNAS under samme navn, noe som oppretter nettverket igjen, og gjenopprett deretter dataene.

## En vanlig eksport feilet i stedet for å skrive en fil {#a-plain-export-failed-instead-of-writing-a-file}

Hvis **Krypter eksporter med age** er på under **Innstillinger, System** men ingen gyldig mottaker er satt, feiler en eksport med en tydelig feil i stedet for å skrive klartekst. Legg til en gyldig mottaker (en offentlig age-nøkkel eller en offentlig SSH-nøkkel), eller slå av krypteringen hvis du vil at eksporten skal være klartekst. Se [Rene eksporter](backups.md#exports).

## En databasedump mislyktes {#a-database-dump-failed}

En mislykket dump får aldri sikkerhetskopien rundt den til å feile. Den føres som sin egen mislykkede kjøring, og årsaken sier hva som må rettes.

- **Innlogging avvist.** Dumpen logger inn med containerens egne passordvariabler (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` eller `_FILE`-utgavene deres). Sjekk dem på databasecontaineren. En `_FILE`-variabel som peker på en hemmelighet containerens egen bruker ikke får lese, feiler på samme måte.
- **Manglende rettigheter.** Med et tilfeldig root-passord kan dumpen bare logge inn som appbrukeren og inneholder dermed bare den ene databasen, og MySQL 8.4 og nyere kan avvise den helt. Gi containeren et ekte root-passord, eller slå av dumpen for den.
- **Systemtabellene må oppgraderes.** MariaDB nekter å bli dumpet når systemtabellene stammer fra en eldre versjon (feil 1558). Legg til variabelen `MARIADB_AUTO_UPGRADE=1` og start containeren på nytt, eller kjør `mariadb-upgrade` inni den én gang.
- **Ingen dumpverktøy.** Et slankt eller egenbygd image uten `pg_dump`, `mysqldump` eller `mariadb-dump` kan ikke dumpes. Bruk det offisielle imaget, eller slå av dumpen.
- **En tidsgrense.** En dump får `DB_DUMP_MAX_HOURS` (6 som standard), sikkerhetskopien rundt får `BACKUP_MAX_HOURS`, og en dump som slutter å komme videre, kuttes etter `BACKUP_STALL_HOURS`. Det siste skyldes som regel en lås applikasjonen holder. Hev grensen som slo til, eller ta dumpen mens applikasjonen er rolig.
- **Containeren er satt på pause eller starter på nytt.** Dumpen snakker med serveren mens den kjører. Hvis containeren starter på nytt igjen og igjen, sier loggen dens hvorfor.
- **En skadet dump kunne ikke fjernes.** En dump BombVault ikke fikk fullført, slettes igjen. Når den slettingen feiler, blir dumpen stående i listen merket som skadet, og du kan slette den derfra.

## En import mislyktes {#an-import-failed}

En import stopper containeren, setter datamappen til side og lar imaget lage en tom i stedet. Feiler et steg før selve importen, legges den gamle mappen tilbake av seg selv. Feiler importen, beholder containeren den ferske mappen, og den gamle blir liggende ved siden av som `<datamappe>.bombvault-before-import-<tidsstempel>`. Feilmeldingen fra kjøringen nevner den nøyaktige stien.

En datamappe som er sitt eget datasett, som på TrueNAS, kan ikke få nytt navn. Importen lar da mappen ligge og flytter innholdet til den bevarte mappen ved siden av, så datasettet over trenger plass til det. Importen sjekker det før den stopper containeren.

Slik legger du den tilbake for hånd: stopp containeren, gi den nåværende datamappen et annet navn så den er av veien, gi den bevarte mappen tilbake det opprinnelige navnet, og start containeren. For et datasett flytter du innholdet i stedet for å gi mappene nye navn. På Unraid gjør filbehandleren under fanen Shares dette.

## En sikkerhetskopi av ZFS-datasett feilet eller hoppet over et datasett {#zfs-datasets}

Hvert problem har en årsakskode i hakeparenteser, og siden [ZFS-datasett](zfs-datasets.md#reason-codes) viser alle med løsningen. De tre vanligste:

- **`snapshot-loop`**: øyeblikksbildet nådde ikke BombVault fordi Host Data ikke sender nye monteringer videre. Rediger containeren, sett Access Mode for Host Data til Read/Write - Slave og start BombVault på nytt.
- **`key-not-loaded`**: et kryptert datasett der nøkkelen ikke er lastet, hoppes over. Last nøkkelen med `zfs load-key` og monter datasettet. Neste sikkerhetskopi tar det med.
- **`ssh-auth`**: serveren avviste BombVaults nøkkel. **ZFS-tilkobling til denne serveren**, som åpnes med **Vis tilkoblingen** på siden Sikkerhetskopier, viser kommandoen som godkjenner den. Kjør den én gang på serveren.

## Et element blir stående på "Lærer N/10" {#an-entry-stays-at-learning-n10}

De fleste avvikskontroller begynner etter 10 vellykkede sikkerhetskopier av et element, og tellingen begynner på nytt etter **Merk som forventet** og etter at elementets utvalg er endret. Et element uten tidsplan lærer ikke, og en container uten appdata har ingenting å lære av, noe merket også sier. **Læringsfremdrift** på siden Avvik viser en ring med ti deler per element.

## Oppbevaringen sluttet å slette gamle sikkerhetskopier av ett element {#retention-stopped-deleting-old-backups-of-one-entry}

Et åpent kritisk avvik holder dem tilbake: elementets kilde er nesten tom, har krympet kraftig, eller en sikkerhetskopi har lagret det meste av dataene på nytt. Åpne avviket fra merket ved elementet. Mangler det data, eller er de kryptert, gjenopprett først fra den lenkede siste gode sikkerhetskopien. Kvitter deretter for avviket, eller merk det som forventet hvis endringen var din, så rydder neste kjøring som vanlig. Forhåndsvisningen av oppbevaringen merker et slikt element som beholdt. For et ZFS-element beholder bare datasettet avviket nevner, sine gamle sikkerhetskopier, og de andre datasettene i treet ryddes som vanlig.

## Manuell beskjæring sier at noen elementer ble beholdt {#manual-prune-says-some-entries-were-kept}

Samme årsak: beskjæringen lar de gamle sikkerhetskopiene av et element med et slikt avvik være og nevner elementet i meldingen sin. Alt annet beskjæres som vanlig.

## En sikkerhetskopi feiler med "on no mounted pool or share" {#no-mounted-pool}

På Unraid ligger `/mnt` i minnet, så en mappe der uten en montert pool er borte ved neste omstart og skjult når poolen er tilbake. BombVault nekter å skrive en sikkerhetskopi eller en flash-ZIP dit, også når et depot fra en tidligere skriving allerede ligger i den mappen. Monter poolen og kjør sikkerhetskopien på nytt.

## Historikkimporten sier at et depot ikke kunne leses {#history-import-says-a-repository-could-not-be-read}

Etter oppgraderingen leser BombVault én gang størrelsen på tidligere sikkerhetskopier fra hvert depot. Et depot som ikke kunne nås da, for eksempel et eksternt mål som var nede eller en deling som ikke var montert, telles i kortet **Avvik** under **Innstillinger, Verifisering** og prøves igjen én gang om dagen. I mellomtiden lærer elementene av nye sikkerhetskopier.

## Varselet om diskplass stemmer ikke med Unraid-dashbordet {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

På Unraids brukerdeling (`/mnt/user`) er den ledige plassen hele arrayets, ikke én disks. Eksterne mål som BombVault når via rclone, for eksempel SFTP, Hetzner Storage Box, SMB, WebDAV eller en skystasjon, måles når den andre siden oppgir ledig plass, og det samme gjelder en `sftp:`-adresse som er skrevet inn for hånd. S3-, B2- og REST-depoter har ikke noe tall: målet deres sier **Størrelse ukjent**, og kortet **Avvik** lister dem som ikke målt.

## En AI-assistent får ikke koblet til {#an-ai-assistant-cannot-connect}

Siden [MCP-server](mcp.md#troubleshooting) viser hva hver statuskode og hvert avslag fra MCP-endepunktet betyr, og hva du kan gjøre med det.

## Containeren fortsetter å starte på nytt eller ser usunn ut {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault rapporterer sunn/usunn fra sin egen `/api/health`. Et auto-heal-verktøy (som Autoheal) kan starte den på nytt automatisk hvis motoren noen gang skulle sette seg fast. Sjekk containerloggen og kortet **Host-integrasjon** for den underliggende årsaken.

## Et mål nekter å lagre {#location-refused}

BombVault sjekker et mål før det lagres, og avslaget sier hvorfor i en setning. De vanligste:

- **«Dette målet ligger inni et annet depot eller mål, eller inneholder et.»** Et depot inni et depot ville ødelagt begge. Meldingen nevner det andre stedet. Velg en annen mappe, eller fjern det andre først.
- **Det er et depot du har satt opp.** Målet er en av radene under **Depoter**, eller ligger inni eller rundt en. Velg et annet sted, eller fjern det depotet først.
- **Samme sted som et områdes egen mappe.** En kopi til et områdes egen mappe ville kopiert sikkerhetskopiene dets til seg selv ved hver kjøring, så den avvises. Velg et annet sted, eller se [Et områdes egen mappe](offsite-recovery.md#own-folder).
- **«That is not an off-site target of this domain»** kom fra en lenke til et mål som er fjernet. En nåværende versjon rydder slike ved start og i en import. Ser du den fortsatt, oppdater BombVault.
- **En full vertssti** som `/mnt/user/backups`. BombVault kjører i en container, så den vil ha stien under Host Data, `user/backups`, og meldingen nevner den du skal skrive inn.

## Et eksternt mål sier at depotet er borte {#repository-gone}

Et mål som hadde sikkerhetskopier og plutselig ikke har noe depot, får ikke et nytt, tomt av seg selv, for det ville skjult det ekte når en disk er tilbake. En rest-server hvis disk ikke ble montert, er den vanlige årsaken, så monter disken igjen og kjør sikkerhetskopien eller kopien en gang til. Bare hvis du tømte eller slettet det gamle depotet med vilje, trykker du på **Opprett et nytt depot her** på målets side. Det starter på nytt for hvert område i det målet samtidig.

## En skypålogging har utløpt {#cloud-sign-in-expired}

Testen eller den neste kopien sier da at påloggingen er utløpt. **Endre tilkobling** på målets side logger inn på nytt.

- **Box, Jottacloud og andre med engangs-refresh-tokener:** BombVault beholder tokenet rclone fornyet, også gjennom omstarter og i gjenopprettingssettet. Etter en gjenoppretting fra en eldre innstillingssikkerhetskopi kan tokenet i den ha løpt ut, så logg inn en gang til. Depotet forblir som det er.
- **iCloud Drive:** Apple avslutter påloggingen etter 30 dager. Den trenger et nytt tillitstoken og nye informasjonskapsler fra `rclone config`.
- **OneDrive:** en pålogging med flere stasjoner spør hvilken som skal brukes, så et mål som står uten valg, kan ikke brukes før du velger. For OneDrive Business og Google Drive, se merknaden om beskjærte data under [Mål](offsite-recovery.md#destinations).

## «Den eksterne kopien av den siste sikkerhetskopien kjører fortsatt» {#copy-still-running}

Sikkerhetskopien før den ble ferdig, og kopien er fortsatt på vei, noe som blokkerer en ny sikkerhetskopi av det området. Vent til statushodet ikke lenger sier **Off-site-kopi pågår**. Knappene som starter en sikkerhetskopi, sier det samme mens de venter. Den første kopien til et nytt mål kan ta timer, se [S3-eksempelet](offsite-recovery.md#example-s3).

## En ZFS-replika stopper fordi målet har nyere øyeblikksbilder {#replica-target-newer}

Målet har øyeblikksbilder som er nyere enn det nyeste det deler med denne serveren, for eksempel fordi datasettet her ble rullet tilbake eller kopien på målet ble endret. Kortet Replika og kjørehistorikken nevner øyeblikksbildet under **Rull målet tilbake til**. Kjør `zfs rollback -r` til det øyeblikksbildet på målet, for hvert datasett kjøringen nevner, og trykk på **Replikér nå**. Se [Hvor lenge øyeblikksbilder blir liggende](zfs-datasets.md#replica-retention).

## En VM med Unraid-øyeblikksbilder feiler og nevner en fil {#vm-chain}

BombVault følger hver disks kjede av underliggende filer og sikkerhetskopierer hver fil i den. En kjede som ikke kan følges til slutten, får sikkerhetskopien til å feile i stedet for å utelate en del av disken, og meldingen nevner filen. Sjekk at filen finnes under `/mnt` og at øyeblikksbildet fortsatt er oppført i VM Manager. En fil utenfor Host Data kan ikke leses i det hele tatt. Se [VM-er med Unraid-øyeblikksbilder](vm-backup-ssh-setup.md#unraid-snapshots).

## Eksterne kopier venter på avgjørelsen din etter en ombygging {#copies-paused}

Etter **Oppdag sikkerhetskopier** eller nødveiviseren uten gjenopprettede innstillinger, eller når et område finner egne sikkerhetskopier fra før denne databasen, venter eksterne kopier av det området, fordi BombVault ikke lenger vet hvilke elementer du lot stå utenfor. **Krever oppmerksomhet** og siden til målet spør om det. Trykk på **Overta dem som denne serverens egne** for å fortsette, eller på **Sjekk reglene først** for å se på reglene til området før du gjør det. Å importere en innstillingsfil tar ikke avgjørelsen for deg. Se [Etter en ombygging](offsite-recovery.md#after-a-rebuild).

## Varsler kommer ikke fram {#notifications-do-not-arrive}

Begynn med **Send test** under **Innstillinger, Varsler**. Linjen under kortet sier hva som skjedde.

- **Ingenting sendes i det hele tatt.** Sjekk **Varsle**. Med **Aldri** går ingen melding ut, bare pingene til en Healthchecks-URL, og med **Kun ved feil** forblir en vellykket kjøring stille. Med **Oppsummer planlagte kjøringer** på sender en planlagt kjøring én oppsummering når den er ferdig.
- **En ny adresse.** Etter at du endrer serveren til Matrix, Gotify eller e-post, ber feltet om tokenet eller passordet på nytt, og den nye adressen lagres først når du har skrevet det inn. BombVault sender ikke den gamle hemmeligheten til et nytt sted.
- **Gotify** sier at serveren avviste app-tokenet: kopier det på nytt fra programmet i Gotify. **Server-URL** er basisadressen, uten `/message`.
- **Webhook:** sjekk at **Webhook-format** passer til tjenesten. En Discord-adresse trenger **Discord**, og ntfy trenger **ntfy**.
- **Unraid-varsler** går over Verts-SSH-tilkoblingen, så test den først under **Innstillinger, Integrasjoner**. På en annen plattform forblir de av, og kortet sier det.
- **En varselmail kommer aldri.** Sjekk **Kryptering** mot porten, vanligvis **STARTTLS** på 587 eller **TLS (implicit)** på 465.

Healthchecks-pingene er uavhengige av alt dette, se [Healthchecks](notifications.md#healthchecks).

## Android-appen kan ikke åpne en server {#android-cannot-open}

Startskjermen sier hvorfor på serverens kort:

- **Ikke tilkoblet:** verken serverens adresse eller relayet svarer. Ved **Ingen relay** når appen en server bare på dens egen adresse.
- **Sjekk sertifikat:** serveren viser et annet sertifikat enn det du stolte på. Appen åpner den først når du stoler på det nye, så pass på at endringen er din.
- **Innlogging kreves:** en server som er lagt til med adressen sin, spør etter passordet. Se også [En telefon havner på innloggingssiden](#a-phone-lands-on-the-login-page).
- **Nøkkelen er endret:** startskjermen viser fingeravtrykket til den nye nøkkelen, og siden åpnes først når du stoler på den. Se [Borte fra hjemmet](android.md#away).

## En annen bruker kan ikke lese det lokale depotet {#repository-permissions}

Det lokale depotet forblir lesbart for andre brukere, for eksempel en synkroniseringsbruker over SMB. Før 10.4.1 kunne filer som en beskjæring, tagging, henting, områdeflytting, sletting eller stack-sikkerhetskopi skrev, bli uleselige til neste natt. Oppdater BombVault, så er de lesbare med en gang. Depotet til auto-sikkerhetskopien forblir bare lesbart for root, med vilje, fordi det inneholder BombVaults innstillinger.

## Fortsatt fast? {#still-stuck}

- Les de fullstendige sidene [Konfigurasjon](configuration.md) og [Ekstern lagring og gjenoppretting](offsite-recovery.md).
- **Feilrapport** i kortet **Hjelp** under **Innstillinger, Info** laster ned en ZIP for en GitHub-sak: vertssjekken, konfigurasjonen din med alle hemmeligheter fjernet, de siste kjøringene, hva som er planlagt neste gang og den nylige loggen siden containeren sist startet. Den inneholder også den siste dumpen av hver database, ZFS-elementene med monteringene deres, de åpne avvikene og hvor mange MCP-nøkler som finnes. Passord, tokener og rclone-konfigurasjonen er fjernet, og pakken sier det i sitt eget manifest. Den krever et innloggingspassord. For et krasj som startet containeren på nytt, se på `docker logs`.
- Spør i [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Åpne en [GitHub-sak](https://github.com/junkerderprovinz/bombvault/issues).
