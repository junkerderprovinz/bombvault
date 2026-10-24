# Feilsøking

En kort FAQ. For den fullstendige host-side-feilsøkingstabellen for VM-over-SSH (permission-denied, host-key-verifisering, manglende malvariabler og mer), se [veiledningen for VM-sikkerhetskopiering over SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Noe er ikke riktig koblet opp

Legg kortet **Host-integrasjon** til oversikten under **Tilpass**. Host-integrasjonssjekken sonderer hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer manglende deler, og **Sjekk nå** kjører den på nytt. Start her før du antar en feil: en manglende montering eller en unåbar host dukker opp umiddelbart.

## Jeg når ikke webgrensesnittet

BombVault serverer HTTPS rett ut av boksen på port `3443` (selvsignert sertifikat), så åpne `https://<your-unraid-ip>:3443`. Godta advarselen om det selvsignerte sertifikatet, eller sett BombVault bak en revers-proxy med ditt eget sertifikat. Hvis du kjører med `HTTP_ONLY=true`, serverer den ren HTTP på port `3000` i stedet (ment for bruk bak en TLS-terminerende proxy).

## Jeg mistet APP_KEY-en min

`APP_KEY` utleder passordet til restic-repositoriet. Uten det, uten gjenopprettingssettet og uten en nøkkeldeponering i gruppen din kan ikke krypterte sikkerhetskopier gjenopprettes. Det er derfor oversikten viser **Lagre gjenopprettingssettet ditt** til du har lastet det ned. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md#recovery-kit). Generer en nøkkel med `openssl rand -hex 32` og oppbevar den bort fra serveren før du stoler på noen sikkerhetskopi.

## Jeg har glemt påloggingspassordet

Velg **Glemt passordet?** på påloggingssiden. BombVault skriver en kode på 12 tegn til loggen til containeren sin (på Unraid: Docker-fanen, BombVault-ikonet, **Logs**). Skriv den inn sammen med et nytt passord innen 10 minutter. Tilbakestillingen slår også av tofaktorautentisering, fjerner alle passkeys, trekker tilbake alle API-tokens, MCP-nøkler og OAuth-pålogginger, avslutter alle andre økter og slår av telefonparing. Legg til passkeysene dine igjen under **Innstillinger, Sikkerhet**, slå telefonparingen på igjen der, og lag nye tokens og nøkler der du fortsatt trenger dem. Se [Sikkerhetsmodell](configuration.md#login).

## Sikkerhetskopiene har stoppet og sier at nøkkelen ikke lenger kan åpnes

Denne serveren overtok en annen servers sikkerhetskopinøkkel i Nødveiviser, og `APP_KEY` ble endret etterpå. Den overtatte nøkkelen er forseglet med den gamle APP_KEY, så BombVault kan ikke åpne den og tar heller ingen sikkerhetskopier enn å skrive med feil nøkkel. **Krever oppmerksomhet** på oversikten nevner det. Sett APP_KEY tilbake til det den var, eller last opp gjenopprettingssettet på nytt på siden Gjenoppretting under **Nød**.

## En telefon havner på påloggingssiden

En paret telefon får økten sin over gruppen bare så lenge **Telefoner kan pare med gruppens tolv ord** er på under **Innstillinger, Sikkerhet**. En tilbakestilling av passordet slår det av. En app som er eldre enn serveren, får heller ingen økt før den er oppdatert. En server som er lagt til med adressen sin, spør alltid etter passordet. En telefon som er fjernet fra Instanser-siden, kommer tilbake slått av når appen åpnes igjen; slå på **Spør ut denne instansen** under **Overvåking** i detaljene for å slippe den inn.

## VM-sikkerhetskopiering vil ikke koble til

VM-sikkerhetskopiering snakker med libvirt over SSH, aldri en montering.

- Bekreft at SSH er aktivert på hosten og at BombVaults offentlige nøkkel er autorisert i `/root/.ssh/authorized_keys` (Innstillinger, Integrasjoner, Verts-SSH viser nøkkelen og en **Test tilkobling**-knapp).
- På et egendefinert `br0.x`-nettverk, sett `LIBVIRT_HOST` til din Unraid-LAN-IP (containeren kan ikke nå hosten via `host.docker.internal` der). Aktiver **Settings, Docker, Host access to custom networks**.
- Hvis du endret Unraids SSH-port, sett `LIBVIRT_SSH_PORT` til å matche.
- På TrueNAS, sett `LIBVIRT_URI` med TrueNAS sin egen libvirt-socket, gi appen capabilityen `SETPCAP`, og legg nøkkelen til root under **Credentials, Users** (se [TrueNAS SCALE](getting-started.md#truenas)).
- Full trinn-for-trinn-diagnose (nåbarhetstest, VLAN-ruting, `Permission denied (publickey)`, `Host key verification failed`) finnes i [veiledningen for VM-sikkerhetskopiering over SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Et live-VM-øyeblikksbilde kjørte ikke

Live-øyeblikksbilder trenger qemu guest agent installert i VM-en og disken på `/mnt/cache` (eller `/mnt/diskX`), ikke `/mnt/user`. På en avslått VM faller live automatisk tilbake til pen. En pen sikkerhetskopi slår VM-en av, sikkerhetskopierer diskene og starter den så på nytt, så den er alltid konsistent. Den venter opptil ti minutter på at gjesten slår seg av, og tvinger den aldri av, så en Windows-gjest som installerer oppdateringer, ikke blir avbrutt halvveis. En gjest som fortsatt kjører etter det, får være i fred, og den sikkerhetskopien feiler med årsaken.

## En sikkerhetskopi feilet med "repository is already locked"

Dette er vanligvis en foreldreløs restic-lås etterlatt da containeren ble oppdatert eller startet på nytt midt i en operasjon. BombVault oppdager en påviselig foreldreløs lås, tvangsfjerner den og forsøker på nytt én gang, automatisk. Hvis den vedvarer, åpne kortet **Kontroller sikkerhetskopier** under **Innstillinger, Verifisering** og velg **Lås opp** under **Mer** i raden til det berørte området for å fjerne en fastlåst lås manuelt. Et genuint problem dukker fortsatt opp i stedet for å bli skjult. Etter en omstart venter BombVault til en slik lås har gått ti minutter uten å bli fornyet. En restic som fortsatt kjører, for eksempel i en annen BombVault mot samme repository, fornyer låsen sin hvert femte minutt.

## Min eksterne kopi skjedde ikke etter en sikkerhetskopi

Ekstern replikering er best-effort av design, så en ekstern hikke feiler aldri den lokale sikkerhetskopien. Sjekk når målet kopierer: kortet **Kopier** sier **Etter hver sikkerhetskopi** eller nevner en egen plan, og **Innstillinger, Tidsplan, Off-site-kopier per område** kan gi et område en egen plan. Bruk **Replikér nå** i kortet **Off-site-kopi** til området under **Innstillinger, Mål** for en på-forespørsel-kjøring, og følg med på statushodet på oversikten, som sier **Off-site-kopi pågår** mens den kjører. En mislykket kopi vises under **Krever oppmerksomhet**.

## Sertifikatet til en rest-server blir ikke godtatt

restic avviser et sertifikat det ikke kjenner, og feilmeldingen sier hvor det skal ligge. Kopier det selvsignerte sertifikatet til rest-serveren, eller sertifikatet til utstederen som signerte det, som PEM-fil til mappen `certs` i BombVaults datamappe (`/config/certs` i containeren, `appdata/bombvault/certs` på Unraid), og test på nytt. Omstart trengs ikke.

## En sikkerhetskopi sier at data utenfor Host Data ikke ble sikkerhetskopiert {#data-outside-host-data}

Containeren har data i en mappe BombVault ikke kan lese gjennom Host Data. Sikkerhetskopien lagrer definisjonen og hver mappe den når, farger linjen sin i aktivitetsloggen oransje og navngir mappene den utelot. To tilfeller er vanlige: et ixVolume på TrueNAS under `/mnt/.ix-apps` og et navngitt Docker-volum under `/var/lib/docker/volumes`, for eksempel databasevolumet i en compose-stack. Kortet **Sikkerhetskopierte mapper** på fanen **Innstillinger** for containeren viser hver slik mappe med stien inne i containeren og vertsstien, merket med rødt.

Et anonymt Docker-volum blir ikke meldt. Docker lager det selv for en mappe imaget oppgir og ingen har koblet til, gir det et navn på 64 heksadesimale sifre og legger det under `/var/lib/docker/volumes`. Som regel inneholder det en hurtigbuffer, så sikkerhetskopien forblir grønn, og kortet viser det bare med en merknad om at det er anonymt.

- **Du vil ha de dataene sikkerhetskopiert.** Koble mappen til en vertssti under Host Data, for eksempel en mappe i appdata-delingen din, så tar neste sikkerhetskopi den med.
- **Du trenger dem ikke.** Legg til en av de to stiene som en linje i kortet **Ekskluderingsmønstre** på samme fane: stien inne i containeren, for eksempel `/cache`, eller vertsstien varselet nevner, for eksempel `/var/lib/docker/volumes/jellyfin_cache/_data`. Fra neste sikkerhetskopi utelater varselet den mappen, og forhåndsvisningen under feltet sier hvorfor. En linje for en mappe over den virker også, et rent navn eller `/` gjør ikke.

## En stor sikkerhetskopi ble drept fordi minnet tok slutt

Sett **Minnegrense per sikkerhetskopi (MiB, 0 = ingen)** i kortet **Ytelse** under **Innstillinger, System** godt under det containeren får bruke. Nær grensen frigjør restic minne oftere, noe som koster litt fart, så sikkerhetskopien går tregere i stedet for å bli drept.

## En gjenoppretting avbrøt før den startet

Før noe stoppes eller fjernes, kjører gjenopprettingen en pre-flight konfliktsjekk: den verifiserer at containerens statiske IP og publiserte host-porter er ledige. Hvis en annen container allerede holder en, avbryter den med en tydelig, handlingsrettet melding i stedet for å etterlate en halvferdig gjenoppretting. Frigjør den motstridende porten eller IP-en, og prøv på nytt.

Finnes ikke containerens nettverk, for eksempel fordi TrueNAS-appen ble slettet, nevner sjekken før gjenopprettingen nettverket, og gjenopprettingen avviser før den endrer noe. Installer appen på nytt i TrueNAS under samme navn, noe som oppretter nettverket igjen, og gjenopprett deretter dataene.

## En vanlig eksport feilet i stedet for å skrive en fil

Hvis **Krypter eksporter med age** er på under **Innstillinger, System** men ingen gyldig mottaker er satt, feiler en eksport med en tydelig feil i stedet for å skrive klartekst. Legg til en gyldig mottaker (en age-offentlig nøkkel eller en SSH-offentlig nøkkel), eller slå av kryptering hvis du har til hensikt at eksporten skal være klartekst. Se [Funksjoner](features.md).

## En databasedump mislyktes

En mislykket dump velter aldri sikkerhetskopien rundt den; den føres som sin egen mislykkede kjøring, og årsaken sier hva som må rettes.

- **Innlogging avvist.** Dumpen logger inn med containerens egne passordvariabler (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` eller `_FILE`-utgavene deres). Sjekk dem på databasecontaineren. En `_FILE`-variabel som peker på en hemmelighet containerens egen bruker ikke får lese, feiler på samme måte.
- **Manglende rettigheter.** Med et tilfeldig root-passord kan dumpen bare logge inn som appbrukeren og inneholder dermed bare den ene databasen, og MySQL 8.4 og nyere kan avvise den helt. Gi containeren et ekte root-passord, eller slå av dumpen for den.
- **Systemtabellene må oppgraderes.** MariaDB nekter å bli dumpet når systemtabellene stammer fra en eldre versjon (feil 1558). Legg til variabelen `MARIADB_AUTO_UPGRADE=1` og start containeren på nytt, eller kjør `mariadb-upgrade` inni den én gang.
- **Ingen dumpverktøy.** Et slankt eller egenbygd image uten `pg_dump`, `mysqldump` eller `mariadb-dump` kan ikke dumpes. Bruk det offisielle imaget, eller slå av dumpen.
- **En tidsgrense.** En dump får `DB_DUMP_MAX_HOURS` (6 som standard), sikkerhetskopien rundt får `BACKUP_MAX_HOURS`, og en dump som slutter å komme videre, kuttes etter `BACKUP_STALL_HOURS`. Det siste skyldes som regel en lås applikasjonen holder. Hev grensen som slo til, eller ta dumpen mens applikasjonen er rolig.
- **Containeren er satt på pause eller starter på nytt.** Dumpen snakker med serveren mens den kjører. Hvis containeren starter på nytt igjen og igjen, sier loggen dens hvorfor.
- **En skadet dump kunne ikke fjernes.** En dump BombVault ikke fikk fullført, slettes igjen. Når den slettingen feiler, blir dumpen stående i listen merket som skadet, og du kan slette den derfra.

## En import mislyktes

En import stopper containeren, setter datamappen til side og lar imaget lage en tom i stedet. Feiler et steg før selve importen, legges den gamle mappen tilbake av seg selv. Feiler importen, beholder containeren den ferske mappen, og den gamle blir liggende ved siden av som `<datamappe>.bombvault-before-import-<tidsstempel>`; feilmeldingen fra kjøringen nevner den nøyaktige stien.

En datamappe som er sitt eget datasett, som på TrueNAS, kan ikke få nytt navn. Importen lar da mappen ligge og flytter innholdet til den bevarte mappen ved siden av, så datasettet over trenger plass til det. Importen sjekker det før den stopper containeren.

Slik legger du den tilbake for hånd: stopp containeren, gi den nåværende datamappen et annet navn så den er av veien, gi den bevarte mappen tilbake det opprinnelige navnet, og start containeren. For et datasett flytter du innholdet i stedet for å gi mappene nye navn. På Unraid gjør filbehandleren under fanen Shares dette.

## En sikkerhetskopi av ZFS-datasett feilet eller hoppet over et datasett {#zfs-datasets}

Hvert problem har en årsakskode i hakeparenteser, og siden [ZFS-datasett](zfs-datasets.md#reason-codes) viser alle med løsningen. De tre vanligste:

- **`snapshot-loop`**: øyeblikksbildet nådde ikke BombVault fordi Host Data ikke sender nye monteringer videre. Rediger containeren, sett Access Mode for Host Data til Read/Write - Slave og start BombVault på nytt.
- **`key-not-loaded`**: et kryptert datasett der nøkkelen ikke er lastet, hoppes over. Last nøkkelen med `zfs load-key` og monter datasettet; neste sikkerhetskopi tar det med.
- **`ssh-auth`**: serveren avviste BombVaults nøkkel. **ZFS-tilkobling til denne serveren**, som åpnes med **Vis tilkoblingen** på siden Sikkerhetskopier, viser kommandoen som godkjenner den; kjør den én gang på serveren.

## Et element blir stående på "Lærer N/10"

De fleste avvikskontroller begynner etter 10 vellykkede sikkerhetskopier av et element, og tellingen begynner på nytt etter **Merk som forventet** og etter at elementets utvalg er endret. Et element uten tidsplan lærer ikke, og en container uten appdata har ingenting å lære av, noe merket også sier. **Læringsfremdrift** på siden Avvik viser en ring med ti deler per element.

## Oppbevaringen sletter ikke lenger gamle sikkerhetskopier av ett element

Et åpent kritisk avvik holder dem tilbake: elementets kilde er nesten tom, har krympet kraftig, eller en sikkerhetskopi har lagret det meste av dataene på nytt. Åpne avviket fra merket ved elementet. Mangler det data, eller er de kryptert, gjenopprett først fra den lenkede siste gode sikkerhetskopien. Kvitter deretter for avviket, eller merk det som forventet hvis endringen var din, så rydder neste kjøring som vanlig. Forhåndsvisningen av oppbevaringen merker et slikt element som beholdt. For et ZFS-element beholder bare datasettet avviket nevner, sine gamle sikkerhetskopier; de andre datasettene i treet ryddes som vanlig.

## Manuell opprydding sier at noen elementer ble beholdt

Samme årsak: oppryddingen lar de gamle sikkerhetskopiene av et element med et slikt avvik være og nevner elementet i meldingen sin. Alt annet ryddes som vanlig.

## Historikkimporten sier at et repository ikke kunne leses

Etter oppgraderingen leser BombVault én gang størrelsen på tidligere sikkerhetskopier fra hvert repository. Et repository som ikke kunne nås da, for eksempel et eksternt mål som var nede eller en share som ikke var montert, står i kortet **Avvik** under **Innstillinger, Verifisering** og prøves igjen én gang om dagen. I mellomtiden lærer elementene av nye sikkerhetskopier.

## Varselet om diskplass stemmer ikke med Unraids dashbord

På Unraids brukershare (`/mnt/user`) er den ledige plassen hele arrayets, ikke én disks. Eksterne repositorier måles bare via rclone-remoter som oppgir ledig plass; S3-, B2-, REST- og SFTP-repositorier har ikke noe tall og står som ikke målt i kortet **Avvik**.

## En AI-assistent får ikke koblet til

Siden [MCP-server](mcp.md#troubleshooting) viser hva hver statuskode og hvert avslag fra MCP-endepunktet betyr, og hva du kan gjøre med det.

## Containeren fortsetter å starte på nytt eller ser usunn ut

BombVault rapporterer sunn/usunn fra sin egen `/api/health`. Et auto-heal-verktøy (som Autoheal) kan starte den på nytt automatisk hvis motoren noen gang skulle sette seg fast. Sjekk containerloggen og kortet **Host-integrasjon** for den underliggende årsaken.

## Fortsatt fast?

- Les de fullstendige sidene [Konfigurasjon](configuration.md) og [Ekstern lagring og gjenoppretting](offsite-recovery.md).
- Spør på [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Åpne et [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
