# Fejlfinding

En kort FAQ. For den fulde fejlfindingstabel for VM-over-SSH på værtssiden (permission-denied, host-key-verifikation, manglende skabelonvariabler og mere), se [VM backup over SSH-guiden](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Noget er ikke forbundet korrekt

Åbn `/spike` i web-UI'en. Vært-integrationstjekket prober hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer eventuelle manglende dele. Start her, før du antager en fejl: en manglende montering eller en uopnåelig vært dukker op med det samme.

## Jeg kan ikke nå web-UI'en

BombVault serverer HTTPS fra start på port `3443` (selvsigneret certifikat), så åbn `https://<your-unraid-ip>:3443`. Accepter advarslen om det selvsignerede certifikat, eller sæt BombVault bag en reverse proxy med dit eget certifikat. Hvis du kører med `HTTP_ONLY=true`, serverer den i stedet almindelig HTTP på port `3000` (beregnet til brug bag en TLS-terminerende proxy).

## Jeg mistede min APP_KEY

`APP_KEY` udleder restic-repositoriets adgangskode. Uden den (og uden gendannelseskittet til krypteringsnøglen) kan krypterede sikkerhedskopier ikke gendannes. Derfor nager Oversigten dig til at downloade gendannelseskittet. Se [Off-site og gendannelse](offsite-recovery.md). Generer en nøgle med `openssl rand -hex 32`, og opbevar den uden for serveren, før du forlader dig på nogen sikkerhedskopi.

## VM-sikkerhedskopiering vil ikke oprette forbindelse

VM-sikkerhedskopiering taler med libvirt over SSH, aldrig en montering.

- Bekræft, at SSH er aktiveret på værten, og at BombVaults offentlige nøgle er autoriseret i `/root/.ssh/authorized_keys` (Indstillinger, System, VM Backup over SSH viser nøglen og en **Test connection**-knap).
- På et brugerdefineret `br0.x`-netværk, sæt `LIBVIRT_HOST` til din Unraid LAN-IP (containeren kan ikke nå værten via `host.docker.internal` der). Aktivér **Settings, Docker, Host access to custom networks**.
- Hvis du ændrede Unraids SSH-port, så sæt `LIBVIRT_SSH_PORT` til at matche.
- Fuld trin-for-trin-diagnose (opnåelighedstest, VLAN-routing, `Permission denied (publickey)`, `Host key verification failed`) findes i [VM backup over SSH-guiden](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Et live-VM-øjebliksbillede kørte ikke

Live-øjebliksbilleder kræver qemu guest agent installeret i VM'en og disken på `/mnt/cache` (eller `/mnt/diskX`), ikke `/mnt/user`. På en slukket VM falder live automatisk tilbage til yndefuld. En yndefuld sikkerhedskopi lukker VM'en ned, sikkerhedskopierer diskene og genstarter den så, så den altid er konsistent.

## En sikkerhedskopi fejlede med "repository is already locked"

Dette er som regel en forældreløs restic-lås efterladt, da containeren blev opdateret eller genstartet midt i en operation. BombVault detekterer en beviseligt forældreløs lås, tvangsrydder den og forsøger igen én gang, automatisk. Hvis den vedvarer, brug **Settings, Integrity & maintenance, Unlock** for det berørte domæne for at rydde en forældet lås manuelt. Et ægte problem dukker stadig op i stedet for at blive skjult.

## Min off-site-kopi skete ikke efter en sikkerhedskopi

Off-site-replikering er best-effort af design, så et off-site-hikke aldrig får den lokale sikkerhedskopi til at fejle. Tjek off-site-tidsplanen for det domæne (Indstillinger, Tidsplaner): en tom tidsplan replikerer efter hver lokal sikkerhedskopi, mens en kadence sender sjældnere. Brug **Replikér nu** på Off-site-fanen for en on-demand-kørsel, og hold øje med replikeringsindikatoren på Oversigten.

## En gendannelse blev afbrudt, før den startede

Før noget stoppes eller fjernes, kører gendannelsen et pre-flight-konflikttjek: den verificerer, at containerens statiske IP og publicerede værtsporte er ledige. Hvis en anden container allerede holder en, afbryder den med en klar, handlingsrettet besked i stedet for at efterlade en halvfærdig gendannelse. Frigør den konfliktende port eller IP, og prøv så igen.

## En almindelig eksport fejlede i stedet for at skrive en fil

Hvis age-kryptering er slået til (Indstillinger), men ingen gyldig modtager er sat, fejler en eksport med en klar fejl i stedet for at skrive klartekst. Tilføj en gyldig modtager (en age-offentlig nøgle eller en SSH-offentlig nøgle), eller slå kryptering fra, hvis du har til hensigt, at eksporten skal være klartekst. Se [Funktioner](features.md).

## Et databasedump mislykkedes

Et mislykket dump vælter aldrig sikkerhedskopien omkring det; det bogføres som sin egen mislykkede kørsel, og årsagen siger, hvad der skal rettes.

- **Login afvist.** Dumpet logger ind med containerens egne adgangskodevariabler (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` eller deres `_FILE`-udgaver). Tjek dem på databasecontaineren. En `_FILE`-variabel, der peger på en hemmelighed, containerens egen bruger ikke må læse, fejler på samme måde.
- **Manglende rettigheder.** Med en tilfældig root-adgangskode kan dumpet kun logge ind som appbrugeren og rummer derfor kun den ene database, og MySQL 8.4 og nyere kan afvise det helt. Giv containeren en rigtig root-adgangskode, eller slå dumpet fra for den.
- **Systemtabellerne skal opgraderes.** MariaDB nægter at blive dumpet, når dens systemtabeller stammer fra en ældre version (fejl 1558). Tilføj variablen `MARIADB_AUTO_UPGRADE=1` og genstart containeren, eller kør `mariadb-upgrade` inde i den én gang.
- **Intet dumpværktøj.** Et slankt eller selvbygget image uden `pg_dump`, `mysqldump` eller `mariadb-dump` kan ikke dumpes. Brug det officielle image, eller slå dumpet fra.
- **En tidsgrænse.** Et dump får `DB_DUMP_MAX_HOURS` (6 som standard), sikkerhedskopien omkring det får `BACKUP_MAX_HOURS`, og et dump, der holder op med at komme videre, afbrydes efter `BACKUP_STALL_HOURS`. Det sidste skyldes som regel en lås, applikationen holder. Hæv den grænse, der udløste, eller dump, mens applikationen er rolig.
- **Containeren er sat på pause eller genstarter.** Dumpet taler med den kørende server. Hvis containeren bliver ved med at genstarte, siger dens egen log hvorfor.
- **Et beskadiget dump kunne ikke fjernes.** Et dump, BombVault ikke kunne gøre færdigt, slettes igen. Når den sletning fejler, bliver dumpet stående på listen markeret som beskadiget, og du kan slette det derfra.

## En import mislykkedes

En import stopper containeren, sætter dens datamappe til side og lader imaget oprette en tom i stedet. Fejler et trin før selve importen, lægges den gamle mappe tilbage af sig selv. Fejler importen, beholder containeren den friske mappe, og den gamle bliver liggende ved siden af som `<datamappe>.bombvault-before-import-<tidsstempel>`; kørslens fejlbesked nævner den nøjagtige sti.

Sådan lægger du den tilbage i hånden: stop containeren, omdøb den nuværende datamappe væk, omdøb den gemte mappe tilbage til det oprindelige navn, og start containeren. På Unraid klarer filhåndteringen under fanen Shares det.

## Et element bliver stående på "Lærer N/10"

De fleste anomalitjek begynder efter 10 vellykkede sikkerhedskopier af et element, og optællingen starter forfra efter **Markér som forventet** og efter en ændring af elementets udvalg. Et element uden tidsplan lærer ikke, og en container uden appdata har intet at lære af, hvilket dens mærke også siger.

## Opbevaringen sletter ikke længere gamle sikkerhedskopier af ét element

En åben kritisk anomali holder dem tilbage: elementets kilde er næsten tom, er skrumpet kraftigt, eller en sikkerhedskopi har gemt det meste af dataene igen. Åbn anomalien fra mærket ved elementet. Hvis der mangler data, eller de er blevet krypteret, så gendan først fra den linkede seneste gode sikkerhedskopi. Kvittér derefter for anomalien, eller markér den som forventet, hvis ændringen kom fra dig, så rydder næste kørsel op som normalt. Forhåndsvisningen af opbevaringen markerer et sådant element som beholdt.

## Manuel oprydning siger, at nogle elementer blev beholdt

Samme årsag: oprydningen lader de gamle sikkerhedskopier af et element med sådan en anomali være og nævner elementet i sin besked. Alt andet ryddes op som normalt.

## Historikimporten siger, at et repository ikke kunne læses

Efter opgraderingen læser BombVault én gang størrelsen af tidligere sikkerhedskopier fra hvert repository. Et repository, der ikke kunne nås på det tidspunkt, for eksempel et offsite-mål, der var nede, eller et share, der ikke var monteret, står i kortet **Afvigelser** under **Indstillinger, Integritet** og forsøges igen én gang om dagen. Imens lærer dets elementer af nye sikkerhedskopier.

## Advarslen om diskplads passer ikke med Unraids dashboard

På Unraids brugershare (`/mnt/user`) er den ledige plads hele arrayets, ikke én disks. Fjerne repositorier måles kun via rclone-remotes, der oplyser deres ledige plads; S3-, B2-, REST- og SFTP-repositorier har intet tal og står som ikke målt i kortet **Afvigelser**.

## Containeren bliver ved med at genstarte eller ser usund ud

BombVault rapporterer sund/usund fra sin egen `/api/health`. Et auto-heal-værktøj (såsom Autoheal) kan genstarte den automatisk, hvis motoren nogensinde går i baglås. Tjek containerloggen og `/spike`-rapporten for den underliggende årsag.

## Stadig fast?

- Læs de fulde sider [Konfiguration](configuration.md) og [Off-site og gendannelse](offsite-recovery.md).
- Spørg på [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Åbn et [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
