# Konfigurasjon

Denne siden dekker containerens miljøvariabler, monteringene malen tilbyr, VM-sikkerhetskopiering over SSH og oppsettet for ekstern lagring. Sikkerhetskopi-**repository-stier** konfigureres inne i appen (Innstillinger, Sikkerhetskopistier), ikke via miljøvariabler.

## Miljøvariabler

| Variabel | Påkrevd | Beskrivelse |
|---|---|---|
| `APP_KEY` | **Ja** | 32-byte hex-hemmelighet (64 hex-tegn) brukt til å utlede passordet til restic-repoet. Generer med `openssl rand -hex 32`. Hold dette trygt: mister du det, blir krypterte sikkerhetskopier umulige å gjenopprette. |
| `LIBVIRT_HOST` | For VM-er | Unraid-host nådd over SSH for VM-sikkerhetskopiering (standard `host.docker.internal`; malen forhåndsutfyller en LAN-IP-plassholder). Bruk din Unraid-LAN-IP, påkrevd på et egendefinert `br0.x`-nettverk. |
| `LIBVIRT_SSH_PORT` | Nei | Host-SSH-port for VM-sikkerhetskopiering (standard `22`). |
| `LIBVIRT_SSH_USER` | Nei | SSH-bruker på hosten for VM-sikkerhetskopiering (standard `root`). |
| `LIBVIRT_URI` | Nei | Full libvirt-tilkoblings-URI, brukt **ordrett** i stedet for å bygge en fra de tre `LIBVIRT_*`-variablene over (som da ignoreres for tilkoblingsstrengen). Ikke satt som standard. Nødvendig på TrueNAS Scale, der libvirtd lytter på en ikke-standard socket som den bygde strengformen ikke kan uttrykke: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Se TrueNAS Scale-delen i [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Nei | HTTP-port (standard `3000`; brukes kun med `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nei | HTTPS-port (standard `3443`; malen publiserer den 1:1, så WebUI-en svarer på `https://<ip>:3443`). |
| `HTTP_ONLY` | Nei | Sett `true` for å deaktivere den selvsignerte HTTPS-lytteren og kun servere ren HTTP (til bruk bak en TLS-terminerende revers-proxy). |
| `HOST_SOURCE_ROOT` | Nei | Host-stien montert som **Host Data** (standard `/mnt`). BombVault oversetter bind-monterings-kildene Docker rapporterer til stier under denne monteringen. Endre kun hvis du monterte en annen host-rot. |
| `DATA_ROOT_SEGMENTS` | Nei | Kommaseparerte sti-segmentnavn som markerer en bind-monterings-kilde som sikkerhetskopidata (standard `appdata`, i tråd med Unraids `/mnt/user/appdata/<container>`-konvensjon). En containers bind-montering velges automatisk for sikkerhetskopiering når ETHVERT oppført segment forekommer som et fullt sti-segment i host-kilden dens. `DATA_ROOT_SEGMENTS=appdata,config` plukker for eksempel også opp en `.../config`-binding. Se [Oppdagelse av sikkerhetskopikilder](#backup-source-detection) for de andre, alltid-aktive måtene en containers datamappe blir funnet på. |
| `PLATFORM` | Nei | Tvinger hvilken plattform BombVault oppfatter seg selv som å kjøre på, i stedet for å auto-oppdage: `unraid`, `generic`, eller `truenas` (ikke satt som standard: auto-oppdager Unraid ved å sondere etter `dockerMan`-markøren under flash-monteringen, ellers `generic`; en ukjent verdi faller også tilbake til `generic`, logget). Sett den eksplisitt på en generisk Docker-host eller TrueNAS Scale i stedet for å stole på auto-sonderingen som er forbeholdt Unraid; det gjør den generiske compose-filen. Endrer appdata-fallback-konvensjonen, standardene for gjenopprettingsmål på tvers av instanser, og om varslings- og følgesvenn-plugin-trinnene som er forbeholdt Unraid i det hele tatt forsøkes (se `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nei | Navnet på selve BombVault-containeren, så den aldri sikkerhetskopierer (og dermed stopper) seg selv (standard `BombVault`; auto-oppdaget via vertsnavnet på bridge-nettverk). |
| `BACKUP_MAX_HOURS` | Nei | Maksimalt antall klokketimer en enkelt sikkerhetskopieringskjøring kan holde domenelåsen sin før den tvangsavbrytes (en beskyttelse så en fastkjørt kjøring ikke kan blokkere domenet for alltid). Tom (standard) bruker `48`. Hev den for svært store eller trege sky-sikkerhetskopier (en kjøring avbrutt ved taket feiler med `context deadline exceeded`). Sett `0` for å deaktivere taket helt. |
| `TZ` | Nei | Tidssone for planleggeren (for eksempel `Europe/Berlin`). **Hvis den ikke settes, kjører alle planer i UTC**: en plan satt til 02:30 starter da 02:30 UTC og ikke etter lokal tid. |

## Monteringer

Monter Docker-socketen, flashen (`/boot`) og **Host Data**-roten (`/mnt`) som vist i CA-malen. Sikkerhetskopi-*kilder* og -*destinasjoner* ligger begge under Host Data, og den er montert **rslave** så en fjerndeling som monteres etter at containeren starter (for eksempel under `/mnt/remotes`) blir synlig uten en omstart.

Sikkerhetskopi-repository-stier har som standard `/mnt/user/bombvault/{container,vms,flash,config,files}`, opprettet ved den første sikkerhetskopieringen. Endre plasseringen når som helst i **Innstillinger, Sikkerhetskopistier**.

!!! note "Sjekk av host-integrasjon"
    Åpne `/spike` i webgrensesnittet etter at containeren har startet. Den sonderer hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer manglende deler.

## Sikkerhetsmodell

!!! warning "Root-ekvivalent kontroll over hosten"
    Gjennom Docker-socketen kan BombVault stoppe, fjerne og gjenskape containere og lese/skrive appdata, og for VM-sikkerhetskopiering logger den seg inn på hosten over SSH (`qemu+ssh://`, root som standard) for å kjøre `virsh`. Alle som når webgrensesnittet, har i praksis root på hosten.

- **Valgfri passordbeskyttelse** (Innstillinger, Sikkerhet): sett et passord for å kreve innlogging, fjern det for å deaktivere. Av som standard for bruk på betrodd LAN. Økter er signert (HMAC utledet fra `APP_KEY`) og å endre passordet ugyldiggjør dem; innlogginger er hastighetsbegrenset.
- Fordi porten er valgfri, er hele grensesnittet og API-et (inkludert ekstern-oppsettet, tamper-test-rutene og gjenopprettingssettet) tilgjengelig for alle som når porten når den ikke er satt. Aktiver porten så snart ekstern lagring, uforanderlige sikkerhetskopier eller kryptering er i bruk.
- Kjør BombVault kun på et betrodd, ikke-eksponert nettverk. For fjerntilgang, sett den bak en revers-proxy som legger til autentisering og TLS. Svar bærer grunnleggende sikkerhetsheadere (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Med `HTTP_ONLY=true` mister øktinformasjonskapselen sitt `Secure`-flagg (det må den, for å fungere over ren HTTP), så aktiver bare passordet bak en TLS-terminerende proxy hvis konfidensialitet betyr noe.
- VM-sikkerhetskopi-SSH-tilkoblingen stoler på host-nøkkelen ved første tilkobling (TOFU) og fester den deretter. Verifiser hostens nøkkel utenfor båndet hvis container-til-host-veien din ikke er betrodd.
- Sikkerhetskopier krypteres av restic når kryptering er aktivert (Innstillinger; på som standard), med nøkkelen utledet fra `APP_KEY`.

## VM-sikkerhetskopiering over SSH

BombVault sikkerhetskopierer KVM/libvirt-VM-er **uten å montere noen libvirt-sti**. Den kjører `virsh` på hosten over SSH (`qemu+ssh://`), så den kan aldri påvirke host-VM Manageren din.

Rask oppsett:

1. **Innstillinger, System, VM Backup over SSH:** kopier den viste offentlige nøkkelen.
2. Legg den til i Unraids `/root/.ssh/authorized_keys` (også lagret til flashen så den overlever omstarter).
3. Klikk **Test tilkobling**.

Malen legger til `--add-host=host.docker.internal:host-gateway` så containeren kan nå hosten. Sett `LIBVIRT_HOST` til din Unraid-LAN-IP hvis det navnet ikke løses (for eksempel når containeren kjører på et egendefinert `br0.x`-nettverk). Hvis du endret Unraids SSH-port, sett `LIBVIRT_SSH_PORT` til å matche. **Live-øyeblikksbilder** trenger i tillegg qemu guest agent i VM-en og disken på `/mnt/cache` (ikke `/mnt/user`).

!!! important "Full VM-oppsetts- og nettverksveiledning"
    Den fullstendige trinn-for-trinn-veiledningen (SSH-aktivering, vedvarende nøkkelautorisasjon, egendefinert-nettverk og VLAN-ruting, metode per VM og host-side-feilsøking) ligger på [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Oppsett for ekstern lagring

Sett opp en ekstern replika på **Innstillinger, Ekstern**-fanen. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md) for hele arbeidsflyten (uforanderlig/append-only, tamper-testing og DR-øvelser). I korthet:

- **Backender:** SMB/CIFS og NFS (monter delingen og pek en sikkerhetskopisti mot den), native restic-backender uten rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), eller en hvilken som helst rclone-remote (`rclone:<remote>:<bucket>/path`).
- **Sky-legitimasjon** lagres kryptert under Innstillinger, Ekstern, Sky-legitimasjon.
- **SSH-mål trenger ingenting installert på den andre siden.** `sftp:` trenger bare en SSH-server. Legg til den offentlige nøkkelen fra **Innstillinger, System, VM Backup over SSH** (også på `/config/ssh/id_ed25519.pub`) til målbrukerens `~/.ssh/authorized_keys`.
- **Ekstern kopi:** BombVault replikerer nye øyeblikksbilder med `restic copy` på best-effort-basis. Det lokale repoet forblir primært. Hvert domene har sin egen eksterne tidsplan, pluss en **Replikér nå**-knapp.
- **Flere eksterne mål per domene:** hvert domene kan replikere til flere eksterne destinasjoner samtidig. Legg til ekstra mål på Innstillinger, Ekstern, hvert med sitt eget repository, sin S3-lagringsklasse, append-only-flagg, oppbevaring og vekstbudsjett; de replikerer alle på det domenets eksterne tidsplan. Et eksisterende enkelt ekstern-oppsett overføres som det første målet.
- **Oppbevaring per kilde:** den lokale policyen ligger på Innstillinger, Stier og lagring; den eksterne policyen på Innstillinger, Ekstern (la den stå helt på null for aldri å auto-trimme eksterne øyeblikksbilder).
- **Båndbreddegrenser:** begrens resticts opplastings-/nedlastingshastighet under Innstillinger, Ekstern.
- **Kald og arkiv-lagringsklasse (S3):** for et native S3-eksternt repo, velg et gjenopprettingslesbart nivå (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-remoter setter klassen sin i rclone-konfigurasjonen.

## Portable innstillinger (eksporter og importer) {#portable-settings-export-and-import}

Kortet **Eksporter og importer innstillinger** på Innstillinger-siden skriver hele BombVault-konfigurasjonen din (domeneinnstillinger, eksterne mål, tidsplaner, oppbevaring, varsler) til en portabel JSON-fil du kan importere på en annen instans, så å flytte til en ny boks eller klone et oppsett ikke betyr å taste inn alt på nytt for hånd. Import viser en forhåndsvisning og ber om bekreftelse, og den rører aldri sikkerhetskopidataene eller -historikken din.

!!! warning "Eksporten kan inneholde legitimasjon"
    Du velger om du vil inkludere ekstern- og varslingslegitimasjonen i filen. Med legitimasjon inkludert er eksporten like sensitiv som gjenopprettingssettet ditt, så oppbevar den et trygt sted. Uten dem inneholder filen kun ikke-hemmelige innstillinger.
