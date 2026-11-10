# Funksjoner

Et kort katalog over hva BombVault gjør. Hver linje lenker til siden som forklarer det.

## Omfang av sikkerhetskopiering {#backup-scope}

- **Kontainere**: appdata-mappen og containerdefinisjonen, med valg av mapper per container. [Sikkerhetskopier og elementer](backups.md#what-is-saved)
- **VM-er**: diskbilder, XML og UEFI-NVRAM over SSH, med en pen nedstenging, et live-øyeblikksbilde eller bare endrede blokker. VM-er med Unraid-øyeblikksbilder og disker på zvols er dekket fullt ut. [VM-sikkerhetskopier](vm-backup-ssh-setup.md)
- **Flash**: hele Unraid-USB-flashen, med planlagt zip-eksport. [Sikkerhetskopier og elementer](backups.md#settings-tab)
- **Auto-sikkerhetskopi**: BombVaults egne innstillinger og pålogginger. [Nødveiviser](offsite-recovery.md#emergency)
- **Mapper**: hvilken som helst mappe på serveren som en mappesamling. [Legge til en mappe](backups.md#add)
- **ZFS-datasett**: et datasetttre lest fra ett øyeblikksbilde, med enkeltfiler som kan gjenopprettes og en valgfri replika på en andre ZFS-server. [ZFS-datasett](zfs-datasets.md)
- **Databasedumper**: PostgreSQL, MySQL og MariaDB, dumpet fra den kjørende serveren til et eget gjenopprettingspunkt. [Databasedumper](backups.md#database-dumps)

## Sikkerhetskopier og elementsider {#backups}

- **Én liste** med fliser, **Filtre**, **Sorter** og **Søk**, og de siste fjorten kjøringene til hver rad som en stripe. [Listen](backups.md#list)
- **Velg som i en filbehandler**, og sikkerhetskopier eller sett et utvalg på pause på serveren. [Velge flere elementer](backups.md#select)
- **En side per element** med **Sikkerhetskopier**, **Innstillinger** og **Verifisering**. [Siden til et element](backups.md#entry-page)
- **Tagger og sammenligning** for øyeblikksbilder, og **Eksporter** som tar-arkiv, zip eller et restic-depot. [Fanen Sikkerhetskopier](backups.md#backups-tab)
- **Ekskluderingsmønstre** med levende forhåndsvisning, og anbefalte unntak for kjente apper. [Ekskluderingsmønstre](backups.md#excludes)
- **Kommandoer før og etter**, og **Stopp andre containere** med en helsestyrt omstart i avhengighetsrekkefølge. [Kommandoer og andre containere](backups.md#commands)
- **Når appen er opptatt**: en planlagt sikkerhetskopi venter på Plex, Jellyfin, Emby, din egen sjekk eller rolig trafikk, og prøver igjen av seg selv. [Når appen er opptatt](backups.md#app-busy)
- **Øyeblikksbilde i stedet for stopp**: en container på ZFS er nede bare noen sekunder. [Tidsplaner](schedules.md#snapshot-instead-of-stop)
- **Rene eksporter**, eventuelt forseglet med age. [Rene eksporter](backups.md#exports)

## Gjenoppretting {#restore}

- **Én side og ett vindu** for alle typer, med den anbefalte sikkerhetskopien forhåndsvalgt. [Gjenoppretting](restore.md)
- **Ved siden av originalen** som standard, og **Erstatt originalen** tar først sikkerhetskopi av dagens tilstand. [Gjenopprettingsvinduet](restore.md#window)
- **Enkeltfiler** til en mappe eller tilbake på plassen sin. [Hent tilbake én fil](restore.md#example-single-file)
- **Flere elementer samtidig**, hver fra sin anbefalte sikkerhetskopi eller fra ett tidspunkt. [Gjenopprette flere elementer](restore.md#several)
- **Hele Compose-stacker**, startet i avhengighetsrekkefølge. [Gjenopprette en stack](restore.md#stacks)
- **Containere og VM-er opprettes på nytt** på imaget de kjørte, med GPU, grenser og lenker. [Hva som kommer tilbake](restore.md#kinds)
- **Sjekker før gjenopprettingen**, og en liste over hva den endrer. [Gjenopprettingsvinduet](restore.md#window)
- **Flash som ZIP**, og ett programtillegg om gangen tilbake på den kjørende flash-disken. [Hva som kommer tilbake](restore.md#kinds)
- **ZFS** inn i datasettet med et sikkerhetsøyeblikksbilde, til en mappe eller til et nytt datasett med egenskapene sine. [ZFS-datasett](zfs-datasets.md#restore)
- **Nødveiviser** for en tapt server, med gjenopprettingssettet, gjennom gruppen eller med den innskrevne nøkkelen. [Nødveiviser](offsite-recovery.md#emergency)
- **Fra en gammel sikkerhetskopimappe**, **Importer fra Appdata.Backup-programtillegget**, og **Jobber fra Duplicati eller Duplicacy**, som gjør sikkerhetskopijobbene deres om til mappesett. [Andre kilder](restore.md#other-sources)

## Lagring og tidsplan {#storage-scheduling}

- **Mål** av alle slag: lokale mapper, SMB, WebDAV, SFTP, rest-server, S3, skystasjoner og hvilken som helst rclone-remote. [Mål](offsite-recovery.md#destinations)
- **Pålogginger**: én liste med en standard. [Pålogginger](offsite-recovery.md#sign-ins)
- **Egne mapper** på denne serveren eller i et mål. [Et områdes egen mappe](offsite-recovery.md#own-folder)
- **3-2-1 som en flyt** der hver linje er en bryter, og **plassering per element**. [Plassering per element](offsite-recovery.md#placement)
- **Eksterne kopier** til flere mål per område, med båndbreddegrenser, komprimering, kalde lagringsklasser og en nedsakking mens Plex strømmer. [Eksterne kopier](offsite-recovery.md#off-site-copies)
- **Direkte depoter**, listet under **Sikkerhetskopieres rett hit**. [Direkte depoter](offsite-recovery.md#direct-repositories)
- **Oppbevaring som tidslinje**: **Kort**, **Balansert**, **Lang** eller **Egne verdier** per mål, med regler per område. [Hvor lenge det beholdes](offsite-recovery.md#keep)
- **Flytt historikken til et område**, og **Fjern mål** med eller uten sikkerhetskopiene. [Flytte og fjerne](offsite-recovery.md#move-remove)
- **Tidsplaner som leses som setninger**, båndet **Uke**, en full sikkerhetskopi, planer per område og en egen plan per element. [Tidsplaner](schedules.md)
- **Ta igjen tapte sikkerhetskopier**, en manuell rekkefølge og oppdateringer etter en vellykket sikkerhetskopi. [Tidsplaner](schedules.md)

## Innsikt, verifisering og overvåking {#insight-verification-monitoring}

- **Oversikten**: statushode, **Krever oppmerksomhet**, **Beskyttelsesstatus**, lagringsprognose, historikk og valgfrie kort. [Oversikt](overview.md)
- **Avviksoppdagelse** per element, som også beholder gamle sikkerhetskopier når en kilde kollapser. [Avvik](anomalies.md)
- **Gjenopprettingskontroll** etter den første sikkerhetskopien, og **Starttest** i en isolert kopi. [Fanen Verifisering](backups.md#verify-tab)
- **Planlagte kontroller og prøvegjenopprettinger** fra den eksterne kopien. [DR-øvelser](offsite-recovery.md#dr-drills)
- **Størrelse per mappe**, og hvorfor en sikkerhetskopi var treg. [Hva en kjøring forteller deg](backups.md#run-notes)
- **Varsler**: webhook, Gotify, Matrix, e-post, Apprise, Unraid og Healthchecks, med en ukentlig oppsummering og en rapport om forsinkelser. [Varsler](notifications.md)
- **Prometheus**, en Home Assistant-enhet, et HTTP-API og mDNS. [API og integrasjoner](api.md)
- **Feilrapport**: en renset ZIP for en GitHub-sak. [Feilsøking](troubleshooting.md#still-stuck)

## Beskyttelse mot løsepengevirus {#ransomware-protection}

- **Beskyttelse mot sletting (append-only)**, håndhevet av den andre siden og bevist med en tamper-test. [Beskyttelse mot sletting](offsite-recovery.md#delete-protection-append-only)
- **Godta uten beskyttelse mot sletting** for et sted som ikke kan nekte en sletting. [Beskyttelse mot sletting](offsite-recovery.md#delete-protection-append-only)
- **Å slette sikkerhetskopier for godt krever din egen innlogging** når et innloggingspassord er satt, og en paret telefon kan ikke gjøre det. [Sikkerhetsmodell](configuration.md#security-model)
- **Gjenopprettingssett**, eventuelt forseglet med age, og en **nøkkelvokter** i gruppen din. [Gjenopprettingssett](offsite-recovery.md#recovery-kit), [Nøkkelvokter](offsite-recovery.md#key-keeper)

## Flere servere {#several-servers}

- **Paring med tolv ord**, ende-til-ende-kryptert, direkte eller gjennom et relay. [Paring av instanser](offsite-recovery.md#pairing)
- **Siden Instanser** med gruppen som et nett og rollene **Mottaker**, **Henter**, **ZFS-server** og **Nøkkelvokter**. [Instanser og roller](offsite-recovery.md#roles)
- **En mottaksserver** som settes opp for deg, i append-only-modus. [Mottaksserver](offsite-recovery.md#receiving-server)
- **Et felles repositorium** som flere servere kopierer til, så det de har felles, lagres bare én gang. Bare serveren som eier det, beskjærer. [Felles repositorium](offsite-recovery.md#shared-repository)

## AI-assistenter (MCP) {#mcp}

En innebygd MCP-server lar en assistent lese sikkerhetskopistatus, dekning, kjørehistorikk og gjenopprettingspunkter, og med en nøkkel som tillater det, starte en sikkerhetskopi. Gjenopprettinger, slettinger, beskjæring og innstillinger blir i nettgrensesnittet. [MCP-server](mcp.md)

## Apper og følgesvenner {#apps}

- **Android-app**: alle serverne i gruppen din på telefonen. [Android-app](android.md)
- **BombVault Widget**: en flis på Unraid-dashbordet. [Apper](settings.md#apps)
- **Innebyggbar aktivitetslogg** for dashbord som Homepage eller Heimdall. [Dashbord-widget](api.md#dashboard-widget)

## Annet {#other}

- **42 språk**, tre temaer, og etiketter, farger, hjørner og animasjon etter eget valg. [Utseende](settings.md#look)
- **Innstillinger som lagrer seg selv**, med søk over alle fjorten fliser. [Innstillinger](settings.md)
- **Nytt** etter en oppdatering, med grønne prikker på det som endret seg. [Info](settings.md#info)
- **HTTPS rett ut av boksen**, og en helsesjekk for verktøy som reparerer seg selv. [Konfigurasjon](configuration.md#network)
