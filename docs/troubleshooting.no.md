# Feilsøking

En kort FAQ. For den fullstendige host-side-feilsøkingstabellen for VM-over-SSH (permission-denied, host-key-verifisering, manglende malvariabler og mer), se [veiledningen for VM-sikkerhetskopiering over SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Noe er ikke riktig koblet opp

Åpne `/spike` i webgrensesnittet. Host-integrasjonssjekken sonderer hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer manglende deler. Start her før du antar en feil: en manglende montering eller en unåbar host dukker opp umiddelbart.

## Jeg når ikke webgrensesnittet

BombVault serverer HTTPS rett ut av boksen på port `3443` (selvsignert sertifikat), så åpne `https://<your-unraid-ip>:3443`. Godta advarselen om det selvsignerte sertifikatet, eller sett BombVault bak en revers-proxy med ditt eget sertifikat. Hvis du kjører med `HTTP_ONLY=true`, serverer den ren HTTP på port `3000` i stedet (ment for bruk bak en TLS-terminerende proxy).

## Jeg mistet APP_KEY-en min

`APP_KEY` utleder passordet til restic-repositoriet. Uten det (og uten gjenopprettingssettet for krypteringsnøkkel) kan ikke krypterte sikkerhetskopier gjenopprettes. Det er derfor Dashboardet maser om at du skal laste ned gjenopprettingssettet. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md). Generer en nøkkel med `openssl rand -hex 32` og oppbevar den bort fra serveren før du stoler på noen sikkerhetskopi.

## VM-sikkerhetskopiering vil ikke koble til

VM-sikkerhetskopiering snakker med libvirt over SSH, aldri en montering.

- Bekreft at SSH er aktivert på hosten og at BombVaults offentlige nøkkel er autorisert i `/root/.ssh/authorized_keys` (Innstillinger, System, VM Backup over SSH viser nøkkelen og en **Test tilkobling**-knapp).
- På et egendefinert `br0.x`-nettverk, sett `LIBVIRT_HOST` til din Unraid-LAN-IP (containeren kan ikke nå hosten via `host.docker.internal` der). Aktiver **Settings, Docker, Host access to custom networks**.
- Hvis du endret Unraids SSH-port, sett `LIBVIRT_SSH_PORT` til å matche.
- Full trinn-for-trinn-diagnose (nåbarhetstest, VLAN-ruting, `Permission denied (publickey)`, `Host key verification failed`) finnes i [veiledningen for VM-sikkerhetskopiering over SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Et live-VM-øyeblikksbilde kjørte ikke

Live-øyeblikksbilder trenger qemu guest agent installert i VM-en og disken på `/mnt/cache` (eller `/mnt/diskX`), ikke `/mnt/user`. På en avslått VM faller live automatisk tilbake til pen. En pen sikkerhetskopi slår VM-en av, sikkerhetskopierer diskene og starter den så på nytt, så den er alltid konsistent.

## En sikkerhetskopi feilet med "repository is already locked"

Dette er vanligvis en foreldreløs restic-lås etterlatt da containeren ble oppdatert eller startet på nytt midt i en operasjon. BombVault oppdager en påviselig foreldreløs lås, tvangsfjerner den og forsøker på nytt én gang, automatisk. Hvis den vedvarer, bruk **Innstillinger, Integritet og vedlikehold, Lås opp** for det berørte domenet for å fjerne en fastlåst lås manuelt. Et genuint problem dukker fortsatt opp i stedet for å bli skjult.

## Min eksterne kopi skjedde ikke etter en sikkerhetskopi

Ekstern replikering er best-effort av design, så en ekstern hikke feiler aldri den lokale sikkerhetskopien. Sjekk den eksterne tidsplanen for det domenet (Innstillinger, Tidsplaner): en tom tidsplan replikerer etter hver lokale sikkerhetskopi, mens en kadens sender sjeldnere. Bruk **Replikér nå** på Ekstern-fanen for en på-forespørsel-kjøring, og følg med på replikeringsindikatoren på Dashboardet.

## En gjenoppretting avbrøt før den startet

Før noe stoppes eller fjernes, kjører gjenopprettingen en pre-flight konfliktsjekk: den verifiserer at containerens statiske IP og publiserte host-porter er ledige. Hvis en annen container allerede holder en, avbryter den med en tydelig, handlingsrettet melding i stedet for å etterlate en halvferdig gjenoppretting. Frigjør den motstridende porten eller IP-en, og prøv på nytt.

## En vanlig eksport feilet i stedet for å skrive en fil

Hvis age-kryptering er på (Innstillinger) men ingen gyldig mottaker er satt, feiler en eksport med en tydelig feil i stedet for å skrive klartekst. Legg til en gyldig mottaker (en age-offentlig nøkkel eller en SSH-offentlig nøkkel), eller slå av kryptering hvis du har til hensikt at eksporten skal være klartekst. Se [Funksjoner](features.md).

## Containeren fortsetter å starte på nytt eller ser usunn ut

BombVault rapporterer sunn/usunn fra sin egen `/api/health`. Et auto-heal-verktøy (som Autoheal) kan starte den på nytt automatisk hvis motoren noen gang skulle sette seg fast. Sjekk containerloggen og `/spike`-rapporten for den underliggende årsaken.

## Fortsatt fast?

- Les de fullstendige sidene [Konfigurasjon](configuration.md) og [Ekstern lagring og gjenoppretting](offsite-recovery.md).
- Spør på [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Åpne et [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
