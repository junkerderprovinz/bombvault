# Konfigurasjon

Denne siden dekker containerens miljøvariabler, monteringene malen tilbyr, sikkerhetsmodellen, VM-sikkerhetskopiering over SSH og oppsettet for ekstern lagring. Hvor sikkerhetskopiene havner, settes opp inne i appen som **mål**, ikke via miljøvariabler.

## Miljøvariabler

| Variabel | Påkrevd | Beskrivelse |
|---|---|---|
| `APP_KEY` | **Ja** | 32-byte hex-hemmelighet (64 hex-tegn) brukt til å utlede passordet til restic-repoet. Generer med `openssl rand -hex 32`. Hold dette trygt: mister du det, blir krypterte sikkerhetskopier umulige å gjenopprette. |
| `LIBVIRT_HOST` | For VM-er | Unraid-host nådd over SSH for VM-sikkerhetskopiering (standard `host.docker.internal`; malen forhåndsutfyller en LAN-IP-plassholder). Bruk din Unraid-LAN-IP, påkrevd på et egendefinert `br0.x`-nettverk. Brukes også til sikkerhetskopi av ZFS-datasett (malfelt **Host SSH: Address**); plassholderen `192.168.x.x` regnes som ikke satt. |
| `LIBVIRT_SSH_PORT` | Nei | Host-SSH-port for VM-sikkerhetskopiering (standard `22`). Malfelt **Host SSH: Port**, også for ZFS-datasett. |
| `LIBVIRT_SSH_USER` | Nei | SSH-bruker på hosten for VM-sikkerhetskopiering (standard `root`). Malfelt **Host SSH: User**, også for ZFS-datasett. |
| `LIBVIRT_URI` | Nei | Full libvirt-tilkoblings-URI, brukt **ordrett** i stedet for å bygge en fra de tre `LIBVIRT_*`-variablene over (som da ignoreres for tilkoblingsstrengen). Ikke satt som standard. Nødvendig på TrueNAS Scale, der libvirtd lytter på en ikke-standard socket som den bygde strengformen ikke kan uttrykke: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Se TrueNAS Scale-delen i [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). Er det en `qemu+ssh://`-URI, hentes hver av `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` og `LIBVIRT_SSH_PORT` som ikke er satt, fra den, også for BombVaults egne SSH-kommandoer (NVRAM-overføring, ZFS-datasett). |
| `PORT` | Nei | HTTP-port (standard `3000`; brukes kun med `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nei | HTTPS-port (standard `3443`; malen publiserer den 1:1, så WebUI-en svarer på `https://<ip>:3443`). |
| `HTTP_ONLY` | Nei | Sett `true` for å deaktivere den selvsignerte HTTPS-lytteren og kun servere ren HTTP (til bruk bak en TLS-terminerende revers-proxy). |
| `BIND_HOST` | Nei | Adressen WebUI-en lytter på (standard `0.0.0.0`, alle grensesnitt). La den stå usatt i containeren, der de publiserte portene trenger alle grensesnitt; `127.0.0.1` passer for en kjøring utenfor Docker. Helsesjekken spør på samme adresse. |
| `TRUSTED_PROXY` | Nei | Kommaseparerte adresser eller CIDR-områder for omvendt proxy foran BombVault (for eksempel `192.168.20.11` eller `10.0.0.0/8`). Bare fra disse hoppene tros `X-Forwarded-For`, og påloggingsbremsen teller da feil per ekte klient i stedet for å samle alle bak proxyen i én bøtte. Ikke satt (standard) betyr at ingen tros: en betingelsesløst trodd header ville la enhver kaller velge sin egen bøtte. |
| `HOST_SOURCE_ROOT` | Nei | Host-stien montert som **Host Data** (standard `/mnt`). BombVault oversetter bind-monterings-kildene Docker rapporterer til stier under denne monteringen. Endre kun hvis du monterte en annen host-rot. |
| `DATA_ROOT_SEGMENTS` | Nei | Kommaseparerte sti-segmentnavn som markerer en bind-monterings-kilde som sikkerhetskopidata (standard `appdata`, i tråd med Unraids `/mnt/user/appdata/<container>`-konvensjon). En containers bind-montering velges automatisk for sikkerhetskopiering når ETHVERT oppført segment forekommer som et fullt sti-segment i host-kilden dens. `DATA_ROOT_SEGMENTS=appdata,config` plukker for eksempel også opp en `.../config`-binding. Se [Oppdagelse av sikkerhetskopikilder](#backup-source-detection) for de andre, alltid-aktive måtene en containers datamappe blir funnet på. |
| `PLATFORM` | Nei | Tvinger hvilken plattform BombVault oppfatter seg selv som å kjøre på, i stedet for å auto-oppdage: `unraid`, `generic`, eller `truenas` (ikke satt som standard: auto-oppdager Unraid ved å sondere etter `dockerMan`-markøren under flash-monteringen, ellers `generic`; en ukjent verdi faller også tilbake til `generic`, logget). Sett den eksplisitt på en generisk Docker-host eller TrueNAS Scale i stedet for å stole på auto-sonderingen som er forbeholdt Unraid; det gjør den generiske compose-filen. Endrer appdata-fallback-konvensjonen, standardene for gjenopprettingsmål på tvers av instanser, og om varslings- og følgesvenn-plugin-trinnene som er forbeholdt Unraid i det hele tatt forsøkes (se `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nei | Navnet på selve BombVault-containeren, så den aldri sikkerhetskopierer (og dermed stopper) seg selv. |
| `BACKUP_MAX_HOURS` | Nei | Maksimalt antall klokketimer en enkelt sikkerhetskopieringskjøring kan holde låsen på området sitt før den tvangsavbrytes (en beskyttelse så en fastkjørt kjøring ikke kan blokkere området for alltid). Tom (standard) bruker `48`. Hev den for svært store eller trege sky-sikkerhetskopier (en kjøring avbrutt ved taket feiler med `context deadline exceeded`). Sett `0` for å deaktivere taket helt. |
| `BACKUP_STALL_HOURS` | Nei | Timer en sikkerhetskopiering kan gå **helt uten fremdrift** før den avbrytes. Tom (standard) bruker `2`; sett `0` for aldri å avbryte ved stillstand. Dette er den finere av de to beskyttelsene og som regel den som slår til: den følger med på om noe fortsatt skjer, ikke på hvor lenge kjøringen har pågått, så en treg men frisk sikkerhetskopiering på flere terabyte får være i fred, mens en som henger på en deling som ikke svarer, stoppes etter timer i stedet for dager. En advarsel logges etter 30 minutter med stillhet, før noe avbrytes. Skanning teller som fremdrift: restic skriver ingen byte mens det går gjennom et stort tre, og den fasen overvåkes via totalene for filer og byte i stedet for via skrevne byte. De to variablene er uavhengige, og `BACKUP_MAX_HOURS` begrenser fortsatt fasene etter selve sikkerhetskopieringen (oppbevaring, statistikk, ekstern kopi), der det ikke finnes tellere å følge med på. |
| `DB_DUMP_MAX_HOURS` | Nei | Timer én automatisk databasedump får kjøre før den stoppes. Tomt (standard) bruker `6`; tillatte verdier er `1` til `48`, og grensen holdes en time under `BACKUP_MAX_HOURS` (på halvparten av den når den er under to timer), slik at en lang dump kuttes av sin egen grense og meldes som det, i stedet for å dra sikkerhetskopien med seg. En dump som ikke kommer videre, stoppes tidligere, etter `BACKUP_STALL_HOURS`. En stoppet dump feiler for seg selv, og sikkerhetskopien av containeren fortsetter. På Unraid legger du variabelen til BombVault-containeren med **Add another Path, Port, Variable**. |
| `TZ` | Nei | Tidssone for planleggeren (for eksempel `Europe/Berlin`). **Hvis den ikke settes, kjører alle planer i UTC**: en plan satt til 02:30 starter da 02:30 UTC og ikke etter lokal tid. På Unraid setter du aldri dette selv: systemet sender sin egen tidssone videre til hver container. Oppstartsloggen viser hvilken sone som ble funnet. En sone med sommertid hopper over én kjøring om våren og kjører én to ganger om høsten, mens UTC ikke gjør noe av det, men til gjengjeld forskyves en time mot klokken din to ganger i året. |

## Monteringer

Monter Docker-socketen, flashen (`/boot`) og **Host Data**-roten (`/mnt`) som vist i CA-malen. Sikkerhetskopi-*kilder* og -*mål* ligger begge under Host Data, og den er montert **slave** så en fjerndeling som monteres etter at containeren starter (for eksempel under `/mnt/remotes`) blir synlig uten en omstart.

Sikkerhetskopi av ZFS-datasett trenger også denne modusen: verten monterer øyeblikksbildet av et datasett først etter at containeren har startet. Se [ZFS-datasett](zfs-datasets.md).

Lokale repositorier har som standard `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, opprettet ved den første sikkerhetskopieringen. De utgjør målet **Lokal mappe**. På siden til det målet under **Mål** endrer kortet **Tilkobling** mappen for hvert område, og vinduet som åpnes, har en **Lokal / Ekstern**-bryter: stien til et område kan være en restic-remote (`s3:...`, `rest:...`, `sftp:...`, `rclone:...`) i stedet for en lokal mappe, og da sikkerhetskopieres det rett dit uten egen lokal kopi; se [Eksterne primære arkiver](offsite-recovery.md#remote-primary-repositories).

!!! note "Host-integrasjonssjekk"
    Kortet **Host-integrasjon** på oversikten (legg det til under **Tilpass**) sonderer hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer manglende deler.

## Gjenkjenning av sikkerhetskopiens kilder {#backup-source-detection}

For hver container velger BombVault selv hvilke bind-monteringer og navngitte volumer som sikkerhetskopieres. En sti tas med så snart ett av punktene nedenfor gjelder (resultatet kan alltid overstyres i kortet **Sikkerhetskopierte mapper** på fanen Innstillinger til containeren):

- **Treff på et datarot-segment:** bindens vertskilde inneholder ett av segmentene i `DATA_ROOT_SEGMENTS` som en hel stikomponent (som standard bare `appdata`).
- **Navngitte Docker-volumer** tas alltid med, fordi de ikke har noen engangsutgave og det dermed ikke er noe å filtrere bort, **men bare når volumets virkelige lagringssti på verten selv er nåbar gjennom Host Data-monteringen**, akkurat som enhver annen vertssti BombVault sikkerhetskopierer. Standarddriveren for lokale volumer legger et volum under selve demonens datarot, altså `/var/lib/docker/volumes/<navn>/_data` med mindre det er endret (sjekk med `docker info -f '{{.DockerRootDir}}'`). Det stedet er IKKE dekket av den smale Host Data-monteringen med én enkelt katalog som den generiske `docker-compose.yml` bruker som standard. Et volum som ikke kan nås, hoppes stille over, det er ingen feil. For faktisk å sikkerhetskopiere navngitte volumer på en generisk vert må du peke Host Data (og `HOST_SOURCE_ROOT`) mot en felles overordnet katalog som også dekker Dockers datarot: avveiningen står i Host Data-kommentaren i compose-filen (Unraid går utenom dette ved av samme grunn å montere hele `/mnt`, sin egen allmenngyldige konvensjon på øverste nivå).
- **Prosjektkatalog for Docker Compose:** bærer containeren den vanlige etiketten `com.docker.compose.project.working_dir` (settes automatisk av `docker compose up`), blir den katalogen også lagt til, uavhengig av om noen bind traff et datarot-segment.
- **Overstyring med etiketten `bombvault.data`:** sett etiketten `bombvault.data=true` på en container for å ta med alle dens bind-monteringer, for et oppsett som ingen av konvensjonene over fanger opp (for eksempel én enkelt bind `/srv/plex/config` uten Compose-prosjekt). Enhver ikke-tom verdi utenom `false` teller som sann; en manglende etikett eller `bombvault.data=false` endrer ingenting.
- **Etiketten `bombvault.dbdump`:** sett `bombvault.dbdump=false` på en container for å slå av den automatiske databasedumpen (`0`, `no` og `off` gjør det samme), eller navngi motoren (`postgres`, `mysql`, `mariadb`) for å dumpe en container BombVault ikke gjenkjenner selv. Etiketten vinner over bryteren i kortet **Database** på fanen Innstillinger til containeren, som er den vanlige veien på Unraid.

## Sikkerhetsmodell {#security-model}

!!! warning "Root-ekvivalent kontroll over hosten"
    Gjennom Docker-socketen kan BombVault stoppe, fjerne og gjenskape containere og lese/skrive appdata, og for VM-sikkerhetskopiering logger den seg inn på hosten over SSH (`qemu+ssh://`, root som standard) for å kjøre `virsh`. Alle som når webgrensesnittet, har i praksis root på hosten.

### Pålogging {#login}

- **Valgfri passordbeskyttelse** (kortet **Passord** under **Innstillinger, Sikkerhet**): sett et passord for å kreve pålogging, tøm det for å slå av. Av som standard for bruk på et betrodd LAN. Passordet lagres med Argon2id over en verdi pepret med `APP_KEY`, så en kopiert `/config` er verdiløs uten nøkkelen og treg å angripe med den. Et nytt passord krever minst 12 tegn; et eksisterende kortere fungerer videre til det endres. Økter er signert (HMAC utledet fra `APP_KEY`), og en passordendring gjør dem ugyldige; pålogginger er begrenset til fem feil i minuttet per klient.
- **Noen ting krever et passord.** Å laste ned gjenopprettingssettet og feilrapporten, å deponere nøkkelen hos en nøkkelvokter, å tillate at en deponering utleveres, å lage en ny frigivelseskode, å slette sikkerhetskopiene på et mål, å eksportere innstillinger med legitimasjon, tofaktorautentisering og passnøkler avvises til et påloggingspassord er satt, og de krever en pålogging på selve instansen.
- **Tofaktorautentisering** (**Innstillinger, Sikkerhet**): en tidskode fra en autentiseringsapp i tillegg til passordet, pluss åtte engangs gjenopprettingskoder som deles ut én gang når den slås på. Den delte hemmeligheten lagres kryptert med `APP_KEY`, og det kreves en gyldig kode for å slå den av igjen.
- **Passnøkler** (WebAuthn) har sitt eget kort når et passord er satt. De brukes sammen med passordet og aldri i stedet for det, så den som fjerner alle passnøkler, låser ingen ute. De krever et ekte domenenavn og et sertifikat som nettleseren stoler på. Standardadressen `https://<ip>:3443` er nettopp det WebAuthn avviser, og kortet sier det i stedet for å tilby en knapp som feiler.
- **Et glemt passord** tilbakestilles på innloggingssiden. **Glemt passordet?** får BombVault til å skrive en kode på 12 tegn, i tre grupper på fire, til loggen til containeren sin (på Unraid: Docker-fanen, BombVault-ikonet, **Logs**). Den koden og et nytt passord logger deg inn. Tilbakestillingen slår også av tofaktorautentisering, fjerner alle passnøkler, trekker tilbake alle API-tokens, MCP-nøkler og OAuth-pålogginger, avslutter alle andre økter, logger ut hver paret telefon og slår av telefonparing; siden etterpå lister hva som ble borte. En kode er gyldig i 10 minutter, og gale gjetninger bruker den ikke opp. Widget- og metrikk-tokens blir, siden de bare leser.
- **Alle som kan lese containerloggen, kan tilbakestille passordet.** Det er root på hosten, men også en loggviser som Dozzle, enhver container som får Docker-socketen (skrivebeskyttet er nok) og en loggsender som videresender containerutdata. Gi dem bare til folk du ville stolt på med passordet. Den som bare når nettgrensesnittet, ser aldri koden.

### Telefoner og gruppemedlemmer {#phones-and-members}

- **Parede telefoner når en liste over det som er tillatt.** En telefon som kjenner gruppens tolv ord, logger inn uten passordet, så lenge **Telefoner kan pare med gruppens tolv ord** er på i kortet **Android-app** under **Innstillinger, Sikkerhet**. Økten når bare det appen viser: oversikten, kjøringene, sikkerhetskopiene man kan se på, starte og gjenopprette, avvikene man kan lese og Instanser-siden man kan se på. Mål og kommandoer når den uten adresser og hemmeligheter, og en gjenoppretting som erstatter originalen, tar først en sikkerhetskopi av dagens tilstand. Alt annet svarer at det krever en pålogging på selve instansen: nøkler, gjenopprettingssettet, nøkkeldeponeringer, innstillinger, sletting av sikkerhetskopier. En tilbakestilling av passordet slår av telefonparing til du slår den på igjen.
- **Medlemmer signerer det de ber om.** Rolleforespørsler og svarene på dem, utleveringen av et restic-passord til en mottaker eller henter, påloggingene til mottakere og nøkkeldeponeringer signeres med en nøkkel fra hvert medlem, som en instans fester første gang den ser medlemmet. Et kall signert med en annen nøkkel avvises. **Glem nøkkelen** i detaljene til en instans slipper den festede nøkkelen, for tilfellet der den som er kjent her, ikke er medlemmets egen; medlemmet bes om nøkkelen sin på nytt ved neste signerte kall. Medlemmer på BombVault 9.x signerer ikke og kan etterlignes av alle som har ordene, som i 9.9.
- **De tolv ordene er et passord.** Alle som har dem, blir med i gruppen, ser beskyttelsesstatusen til medlemmene og kan be om roller, som en person på den andre instansen fortsatt må tillate. Uten påloggingspassord kan alle som kan åpne nettgrensesnittet, lese dem, og paringskortet sier det til et passord er satt.

### Nøkler {#keys}

- **APP_KEY gjør to ting.** Den forsegler det denne installasjonen lagrer (passord, pålogginger, tokens), og den er nøkkelen til sikkerhetskopiene: restic-passordet utledes fra den, og container- og VM-definisjoner forsegles med den.
- **En overtatt nøkkel holdes for seg.** Når Nødveiviser tar over en annen servers sikkerhetskopier, forsegler denne installasjonen fortsatt med sin egen APP_KEY og lagrer den andre serverens nøkkel som sin **sikkerhetskopinøkkel**, forseglet som alle andre hemmeligheter. Gjenopprettingssettet skriver da ut begge. Endres APP_KEY etterpå, kan sikkerhetskopinøkkelen ikke lenger åpnes: BombVault slutter å ta sikkerhetskopier og sier fra under **Krever oppmerksomhet**, i stedet for å skrive med feil nøkkel. Last opp gjenopprettingssettet på nytt under **Gjenoppretting, Nød**, eller sett APP_KEY tilbake. Se [Nødveiviser](offsite-recovery.md#emergency).
- **Kryptering** er på som standard (**Krypter sikkerhetskopier** i kortet **Kryptering** under **Innstillinger, Sikkerhet**). Sikkerhetskopier krypteres av restic med passordet som utledes fra sikkerhetskopinøkkelen.
- **Nøkkeldeponeringer.** En nøkkelvokter i gruppen din holder en forseglet kopi av gjenopprettingssettet til denne serveren som den ikke kan åpne selv. Bare frigivelseskoden på 20 tegn åpner den, og vokteren leverer den ut først etter at noen der trykker **Tillat**. Se [Nøkkelvokter](offsite-recovery.md#key-keeper).

### Nettverk {#network}

- **Endringer krever JSON.** En forespørsel som endrer noe, må sende `Content-Type: application/json` og kan ikke være merket som cross-site av nettleseren, slik at en side på et annet nettsted ikke kan få nettleseren din til å endre innstillinger på en LAN-adresse. Et skript som styrer API-et, sender den headeren; alt annet avvises med `415`.
- Fordi porten er valgfri, er hele grensesnittet og API-et (inkludert målene, tamper-test-rutene og gjenopprettingsrutene) tilgjengelig for alle som når porten når den ikke er satt. Sett et påloggingspassord så snart ekstern lagring, append-only-sikkerhetskopier eller kryptering er i bruk.
- Kjør BombVault kun på et betrodd, ikke-eksponert nettverk. For fjerntilgang, sett den bak en revers-proxy som legger til autentisering og TLS. Svar bærer grunnleggende sikkerhetsheadere (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Bak en omvendt proxy bærer hver forespørsel proxyens adresse, så uten `TRUSTED_PROXY` teller påloggingsbremsen alle klienter i én bøtte, og en angripers feil låser deg også ute. Oppgi proxyen i `TRUSTED_PROXY` for å få telling per klient tilbake.
- En omvendt proxy foran BombVault må sende headeren `Authorization` eller `X-API-Key` videre til `/mcp` og må ikke bufre svarene, ellers kan ikke assistenter koble til. Se [MCP-server](mcp.md#tls).
- MCP-endepunktet `/mcp` svarer `404` til det finnes en nøkkel eller pålogging via OAuth er slått på, og det krever nøkkel eller token av hver klient, også når påloggingspassordet er slått av; ingen adresse er unntatt, heller ikke `localhost`. Det har ingen verktøy for å gjenopprette eller slette, og gjenoppretting av en konfigurasjonssikkerhetskopi tilbakekaller alle nøkler. Se [MCP-server](mcp.md#security).
- Med `HTTP_ONLY=true` mister øktinformasjonskapselen sitt `Secure`-flagg (det må den, for å fungere over ren HTTP), så aktiver bare passordet bak en TLS-terminerende proxy hvis konfidensialitet betyr noe.
- VM-sikkerhetskopi-SSH-tilkoblingen stoler på host-nøkkelen ved første tilkobling (TOFU) og fester den deretter. Verifiser hostens nøkkel utenfor båndet hvis container-til-host-veien din ikke er betrodd.

## MCP-server {#mcp-server}

MCP-serveren trenger ingen miljøvariabel. Du slår den på ved å lage en nøkkel under **Innstillinger, Integrasjoner, MCP-server**, og den svarer på `/mcp` på samme port som webgrensesnittet (for eksempel `https://192.168.1.10:3443/mcp`). Uten en aktiv nøkkel svarer den stien med `404`. Klienter, sertifikater og grenser er beskrevet på [MCP-server](mcp.md).

## VM-sikkerhetskopiering over SSH

BombVault sikkerhetskopierer KVM/libvirt-VM-er **uten å montere noen libvirt-sti**. Den kjører `virsh` på hosten over SSH (`qemu+ssh://`), så den kan aldri påvirke host-VM Manageren din.

Å montere hostens libvirt-socket i en container er skjørt på Unraid: VM Manager eier de stiene, og å slå "Enable VMs" av og på kan etterlate libvirt i en tilstand der den ikke starter. SSH-nøkkelen gir root på hosten, samme tillitsnivå som Docker-socketen BombVault allerede bruker.

Rask oppsett:

1. **Innstillinger, Integrasjoner, Verts-SSH:** kopier den viste offentlige nøkkelen.
2. Legg den til i Unraids `/root/.ssh/authorized_keys` (også lagret til flashen så den overlever omstarter).
3. Klikk **Test tilkobling**.

Malen legger til `--add-host=host.docker.internal:host-gateway` så containeren kan nå hosten. Sett `LIBVIRT_HOST` til din Unraid-LAN-IP hvis det navnet ikke løses (for eksempel når containeren kjører på et egendefinert `br0.x`-nettverk). Hvis du endret Unraids SSH-port, sett `LIBVIRT_SSH_PORT` til å matche. **Live-øyeblikksbilder** trenger i tillegg qemu guest agent i VM-en og disken på `/mnt/cache` (ikke `/mnt/user`).

!!! important "Full VM-oppsetts- og nettverksveiledning"
    Den fullstendige trinn-for-trinn-veiledningen (SSH-aktivering, vedvarende nøkkelautorisasjon, egendefinert-nettverk og VLAN-ruting, metode per VM og host-side-feilsøking) ligger på [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Oppsett for ekstern lagring

Eksterne kopier går til **mål**. Åpne **Mål** fra innstillingsflisen (eller fest den til sidefeltet) og velg **Legg til mål**. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md) for hele arbeidsflyten (slettebeskyttelse, tamper-testing og DR-øvelser). Kort fortalt:

- **Backender:** SMB/CIFS og WebDAV via rclone uten host-montering, NFS (monter eksporten på hosten og bruk den som lokal mappe), native restic-backender uten rclone (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`), eller en hvilken som helst rclone-remote (`rclone:<remote>:<bucket>/path`). Backblaze B2 har ingen egen backend her: nå det gjennom S3-endepunktet (`s3:https://s3.<region>.backblazeb2.com/<bucket>/<path>`) med nøkkel-ID-en og applikasjonsnøkkelen som S3-legitimasjon.
- **Legitimasjon** lagres kryptert, enten som delt skylegitimasjon eller som et navngitt sett. Begge er kort under **Innstillinger, Mål**, og siden til et mål velger settet det bruker under **Legitimasjon**.
- **SSH-mål trenger ingenting installert på den andre siden.** `sftp:` trenger bare en SSH-server. Legg til den offentlige nøkkelen fra **Innstillinger, Integrasjoner, Verts-SSH** (også på `/config/ssh/id_ed25519.pub`) til målbrukerens `~/.ssh/authorized_keys`.
- **Ekstern kopi:** BombVault kopierer nye sikkerhetskopier med `restic copy` på best-effort-basis, i tillegg til et (vanligvis lokalt) primærarkiv. Kortet **Kopier** til et mål bestemmer om det kopierer **Etter hver sikkerhetskopi** eller etter en egen plan, og **Innstillinger, Tidsplan, Off-site-kopier per område** gir ett område en egen kopieringsplan. **Replikér nå** ligger i kortet **Off-site-kopi** til hvert område under **Innstillinger, Mål**.
- **Flere mål per område:** kortet **Mål per område** på siden Mål slår hver linje fra et område til et mål av eller på. **Regler per område** hos et mål gir ett område sitt eget arkiv, sin egen oppbevaring eller komprimering der.
- **Plassering per element:** hver container, VM, hvert mappesett og hvert ZFS-datasett tenner **Lokal** og målene som får sikkerhetskopiene, i kortet **Tidsplan og mål** på fanen Innstillinger. Elementer uten eget valg følger området sitt. Se [Plassering per element](offsite-recovery.md#placement).
- **Oppbevaring:** hvert mål beholder sikkerhetskopier etter kortet **Hvor lenge det beholdes**, og et område kan ha egne verdier der under **Regler per område**. Kopien i et områdes eksterne felt følger reglene i kortet **Off-site-oppbevaring** under **Innstillinger, Mål**, som har **Oppbevaringsregler per kilde** for enkeltområder (la det stå på null for aldri å trimme eksterne sikkerhetskopier automatisk).
- **Båndbreddegrenser:** begrens resticts opplastings-/nedlastingshastighet i kortet **Ekstern båndbredde** under **Innstillinger, Mål**, eller per mål i kortet **Kopier**.
- **Strømming først:** velg i kortet **Strømming** på siden Mål medieserverne (Plex, Jellyfin og Emby er forhåndsvalgt etter imagenavn), sendehastigheten fra der en regnes som strømming, opplastingsgrensen mens den strømmer og hvor lenge etter en strøm den vanlige grensen kommer tilbake.
- **Kald og arkiv-lagringsklasse (S3):** for et S3-mål, velg et gjenopprettingslesbart nivå (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval) på siden. rclone-remoter setter klassen sin i rclone-konfigurasjonen.
- **Eksternt primærarkiv i stedet for lokalt:** den egne stien til et område kan selv være en av backendene ovenfor, uten lokal kopi og uten replikeringstrinn. Se [Eksterne primære arkiver](offsite-recovery.md#remote-primary-repositories) for bryteren **Lokal / Ekstern** og innstillingene for båndbredde, append-only og vekstbudsjett.

## Avvik {#anomalies}

Avviksoppdagelsen stilles inn i kortet **Avvik** under **Innstillinger, Verifisering**. Hver kontroll lagres så snart du endrer den, og de tre under bryteren er skjult mens oppdagelsen er slått av.

| Innstilling | Standard | Hva den gjør |
|---|---|---|
| **Oppdag avvik** | På | Sammenligner hver sikkerhetskopi med elementets egen historikk. Slått av kontrolleres ingenting nytt, og oppføringen **Avvik** forsvinner fra sidepanelet; kortet lenker fortsatt til tidligere funn. |
| **Følsomhet** | Balansert | Streng melder mindre endringer, Romslig bare store. |
| **Send varsel for** | Bare kritiske funn | Den laveste alvorlighetsgraden som sender en melding via kanalene som er satt opp under Varsler. Gjentatte mislykkede sikkerhetskopier og dumper og mislykkede planlagte gjenopprettingskontroller sender allerede en egen melding og sendes ikke to ganger. |
| **Behold gamle sikkerhetskopier når en kilde krymper kraftig eller skrives om** | På | Så lenge et element har et åpent funn for en nesten tom kilde, en kraftig krymping eller det meste av dataene lagret på nytt, lar oppbevaring og opprydding elementets gamle sikkerhetskopier være. Kvitter for funnet eller merk det som forventet for å slippe dem. |

Hvert element kan ha sin egen følsomhet og sitt eget varslingsminimum. Still dem inn på fanen **Integritet** til elementet, eller på siden **Avvik** under **Overvåking**. For et ZFS-element gjelder de for hvert datasett i treet.

## Portable innstillinger (eksporter og importer) {#portable-settings-export-and-import}

Kortet **Eksporter / importer innstillinger** under **Innstillinger, System** skriver hele BombVault-konfigurasjonen din (innstillinger per område, mål, tidsplaner, oppbevaring, varsler) til en portabel JSON-fil du kan importere på en annen instans, så å flytte til en ny boks eller klone et oppsett ikke betyr å taste inn alt på nytt for hånd. Import viser en forhåndsvisning og ber om bekreftelse, og den rører aldri sikkerhetskopidataene eller -historikken din.

!!! warning "Eksporten kan inneholde legitimasjon"
    Du velger om du vil inkludere ekstern-, varslings- og MQTT-megler-legitimasjonen i filen. Med legitimasjon inkludert er eksporten like sensitiv som gjenopprettingssettet ditt, så oppbevar den et trygt sted. Uten dem inneholder filen kun ikke-hemmelige innstillinger.
