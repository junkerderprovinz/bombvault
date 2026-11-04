# Funktioner

Et kort katalog over, hvad BombVault kan. Hver linje linker til den side, der forklarer det.

## Hvad der sikkerhedskopieres {#backup-scope}

- **Containers**: appdata-mappen og containerens definition, med et valg af mapper pr. container. [Sikkerhedskopier og poster](backups.md#what-is-saved)
- **VMs**: diskimages, XML og UEFI-NVRAM over SSH, med en pæn nedlukning, et live-snapshot eller kun ændrede blokke. VM'er med Unraid-snapshots og diske på zvols er dækket fuldt ud. [VM-sikkerhedskopier](vm-backup-ssh-setup.md)
- **Flash**: hele Unraids USB-flashdrev, med en planlagt zip-eksport. [Sikkerhedskopier og poster](backups.md#settings-tab)
- **Auto-sikkerhedskopi**: BombVaults egne indstillinger og adgangsoplysninger. [Nødguide](offsite-recovery.md#emergency)
- **Mapper**: enhver mappe på serveren som et mappesæt. [Tilføj en mappe](backups.md#add)
- **ZFS-datasæt**: et datasættræ læst fra ét snapshot, hvor enkelte filer kan gendannes, og med en valgfri replika på en anden ZFS-server. [ZFS-datasæt](zfs-datasets.md)
- **Databasedumps**: PostgreSQL, MySQL og MariaDB, dumpet fra den kørende server til et gendannelsespunkt for sig. [Databasedumps](backups.md#database-dumps)

## Sikkerhedskopier og postsider {#backups}

- **Én liste** med fliser, **Filtre**, **Sortér** og **Søg** og de seneste fjorten kørsler for hver række som en stribe. [Listen](backups.md#list)
- **Vælg som i en filhåndtering**, og sikkerhedskopiér eller sæt et valg på pause på serveren. [Vælg flere poster](backups.md#select)
- **En side pr. post** med **Sikkerhedskopier**, **Indstillinger** og **Verificering**. [En posts side](backups.md#entry-page)
- **Tags og sammenligning** for gendannelsespunkter og **Eksporter** som tar, zip eller et restic-repository. [Fanen Sikkerhedskopier](backups.md#backups-tab)
- **Udelukkelsesmønstre** med en live-forhåndsvisning og anbefalede udelukkelser for velkendte apps. [Udelukkelsesmønstre](backups.md#excludes)
- **Kommandoer før og efter** og **Stop andre containere** med en genstart, der venter på sundhed, i afhængighedsrækkefølge. [Kommandoer og andre containere](backups.md#commands)
- **Når appen er optaget**: En planlagt sikkerhedskopiering venter på Plex, Jellyfin, Emby, dit eget tjek eller rolig trafik og prøver igen af sig selv. [Når appen er optaget](backups.md#app-busy)
- **Snapshot i stedet for stop**: En container på ZFS er kun nede i nogle få sekunder. [Tidsplaner](schedules.md#snapshot-instead-of-stop)
- **Almindelige eksporter**, om du vil forseglet med age. [Almindelige eksporter](backups.md#exports)

## Gendannelse {#restore}

- **Én side og ét vindue** til alle slags, med den anbefalede sikkerhedskopi valgt på forhånd. [Gendannelse](restore.md)
- **Ved siden af originalen** som standard, og **Erstat originalen** tager først en sikkerhedskopi af den nuværende tilstand. [Gendannelsesvinduet](restore.md#window)
- **Enkelte filer** til en mappe eller tilbage på deres plads. [Hent én fil tilbage](restore.md#example-single-file)
- **Flere poster på én gang**, hver fra sin anbefalede sikkerhedskopi eller fra ét tidspunkt. [Gendan flere poster](restore.md#several)
- **Hele Compose-stakke**, startet i afhængighedsrækkefølge. [Gendan en stak](restore.md#stacks)
- **Containere og VM'er genskabes** på det image, de kørte på, med GPU, grænser og links. [Hvad der kommer tilbage](restore.md#kinds)
- **Tjek før gendannelsen** og en liste over, hvad den ændrer. [Gendannelsesvinduet](restore.md#window)
- **Flash som en ZIP** og ét plugin ad gangen tilbage i det kørende flashdrev. [Hvad der kommer tilbage](restore.md#kinds)
- **ZFS** ind i datasættet med et sikkerhedssnapshot, til en mappe eller til et nyt datasæt med dets egenskaber. [ZFS-datasæt](zfs-datasets.md#restore)
- **Nødguide** til en tabt server, med gendannelseskittet, gennem gruppen eller med den indtastede nøgle. [Nødguide](offsite-recovery.md#emergency)
- **Fra en gammel sikkerhedskopimappe** og **Importér fra Appdata.Backup-pluginnet**. [Andre kilder](restore.md#other-sources)

## Lager og tidsplaner {#storage-scheduling}

- **Destinationer** af alle slags: lokale mapper, SMB, WebDAV, SFTP, rest-server, S3, clouddrev og enhver rclone-remote. [Destinationer](offsite-recovery.md#destinations)
- **Adgangsoplysninger**: én liste med en standard. [Adgangsoplysninger](offsite-recovery.md#sign-ins)
- **Egne mapper** på denne server eller på en destination. [Et områdes egen mappe](offsite-recovery.md#own-folder)
- **3-2-1 som et flow**, hvor hver linje er en kontakt, og **placering pr. element**. [Placering pr. element](offsite-recovery.md#placement)
- **Off-site-kopier** til flere destinationer pr. område, med båndbreddegrænser, komprimering, kolde lagringsklasser og en opbremsning, mens Plex streamer. [Off-site-kopier](offsite-recovery.md#off-site-copies)
- **Direkte repositories**, vist under **Sikkerhedskopierer direkte hertil**. [Direkte repositories](offsite-recovery.md#direct-repositories)
- **Opbevaring som en tidslinje**: **Kort**, **Afbalanceret**, **Lang** eller **Egne værdier** pr. destination, med regler pr. område. [Hvor længe der gemmes](offsite-recovery.md#keep)
- **Flyt et områdes historik**, og **Fjern destination** med eller uden dens sikkerhedskopier. [Flyt og fjern](offsite-recovery.md#move-remove)
- **Tidsplaner, der læses som sætninger**, båndet **Uge**, en fuld sikkerhedskopi, planer pr. område og en egen plan pr. post. [Tidsplaner](schedules.md)
- **Indhent forpassede sikkerhedskopieringer**, en manuel rækkefølge og opdateringer efter en vellykket sikkerhedskopiering. [Tidsplaner](schedules.md)

## Indblik, verificering og overvågning {#insight-verification-monitoring}

- **Oversigten**: statusdel, **Kræver opmærksomhed**, **Beskyttelsesstatus**, lagerprognose, historik og valgfrie kort. [Oversigt](overview.md)
- **Afvigelsesovervågning** pr. post, som også beholder gamle sikkerhedskopier, når en kilde falder sammen. [Afvigelser](anomalies.md)
- **Gendannelsestjek** efter den første sikkerhedskopi og **Starttest** i en isoleret kopi. [Fanen Verificering](backups.md#verify-tab)
- **Planlagte tjek og testgendannelser** fra off-site-kopien. [DR-test](offsite-recovery.md#dr-drills)
- **Størrelse pr. mappe** og hvorfor en sikkerhedskopiering var langsom. [Hvad en kørsel fortæller dig](backups.md#run-notes)
- **Notifikationer**: webhook, Gotify, Matrix, e-mail, Apprise, Unraid og Healthchecks, med en ugentlig oversigt og en rapport om forsinkelser. [Notifikationer](notifications.md)
- **Prometheus**, en Home Assistant-enhed, et HTTP-API og mDNS. [API og integrationer](api.md)
- **Fejlrapport**: en redigeret ZIP til et GitHub-issue. [Fejlfinding](troubleshooting.md#still-stuck)

## Beskyttelse mod ransomware {#ransomware-protection}

- **Beskyttelse mod sletning (append-only)**, håndhævet af den fjerne side og bevist af en manipulationstest. [Beskyttelse mod sletning](offsite-recovery.md#delete-protection-append-only)
- **Accepter uden beskyttelse mod sletning** til et sted, der ikke kan afvise en sletning. [Beskyttelse mod sletning](offsite-recovery.md#delete-protection-append-only)
- **At slette sikkerhedskopier for altid kræver en loginadgangskode.** [Sikkerhedsmodel](configuration.md#security-model)
- **Gendannelseskit**, om du vil forseglet med age, og en **nøgleholder** i din gruppe. [Gendannelseskit](offsite-recovery.md#recovery-kit), [Nøgleholder](offsite-recovery.md#key-keeper)

## Flere servere {#several-servers}

- **Parring med tolv ord**, ende-til-ende-krypteret, direkte eller gennem en videresender. [Parring af instanser](offsite-recovery.md#pairing)
- **Siden Instanser** med gruppen som et net og rollerne **Modtager**, **Henter**, **ZFS-server** og **Nøgleholder**. [Instanser og roller](offsite-recovery.md#roles)
- **En modtagende server**, der sættes op for dig, i append-only-tilstand. [Modtagende server](offsite-recovery.md#receiving-server)

## AI-assistenter (MCP) {#mcp}

En indbygget MCP-server lader en assistent læse sikkerhedskopistatus, dækning, kørselshistorik og gendannelsespunkter og, med en nøgle der tillader det, starte en sikkerhedskopiering. Gendannelser, sletninger, beskæring og indstillinger bliver i webgrænsefladen. [MCP-server](mcp.md)

## Apps og ledsagere {#apps}

- **Android-app**: hver server i din gruppe på din telefon. [Android-app](android.md)
- **BombVault Widget**: en flise på Unraid Dashboard. [Apps](settings.md#apps)
- **Indlejrbar aktivitetslog** til dashboards som Homepage eller Heimdall. [Dashboard-widget](api.md#dashboard-widget)

## Andet {#other}

- **42 sprog**, tre temaer og etiketter, farver, hjørner og animation efter dit valg. [Udseende](settings.md#look)
- **Indstillinger, der gemmer sig selv**, med en søgning over alle fjorten fliser. [Indstillinger](settings.md)
- **Nyt** efter en opdatering, med grønne prikker ved det, der er ændret. [Info](settings.md#info)
- **HTTPS fra start** og et healthcheck til auto-heal-værktøjer. [Konfiguration](configuration.md#network)
