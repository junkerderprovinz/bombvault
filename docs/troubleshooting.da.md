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

## Containeren bliver ved med at genstarte eller ser usund ud

BombVault rapporterer sund/usund fra sin egen `/api/health`. Et auto-heal-værktøj (såsom Autoheal) kan genstarte den automatisk, hvis motoren nogensinde går i baglås. Tjek containerloggen og `/spike`-rapporten for den underliggende årsag.

## Stadig fast?

- Læs de fulde sider [Konfiguration](configuration.md) og [Off-site og gendannelse](offsite-recovery.md).
- Spørg på [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Åbn et [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
