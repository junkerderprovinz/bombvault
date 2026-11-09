# Funktioner

En kort katalog över vad BombVault gör. Varje rad länkar till sidan som förklarar den.

## Vad som säkerhetskopieras {#backup-scope}

- **Containers**: appdata-mappen och containerdefinitionen, med ett val av mappar per container. [Säkerhetskopior och poster](backups.md#what-is-saved)
- **VMs**: diskavbilder, XML och UEFI NVRAM över SSH, med en kontrollerad avstängning, en live-ögonblicksbild eller bara ändrade block. VM:ar med Unraid-ögonblicksbilder och diskar på zvols täcks fullt ut. [VM-säkerhetskopior](vm-backup-ssh-setup.md)
- **Flash**: hela Unraid-USB-flashen, med en schemalagd zip-export. [Säkerhetskopior och poster](backups.md#settings-tab)
- **Auto-säkerhetskopia**: BombVaults egna inställningar och inloggningar. [Nödguide](offsite-recovery.md#emergency)
- **Mappar**: valfri mapp på servern som en mappuppsättning. [Lägga till en mapp](backups.md#add)
- **ZFS-datauppsättningar**: ett träd av datauppsättningar läst från en ögonblicksbild, med enskilda filer som går att återställa och en valfri replik på en andra ZFS-server. [ZFS-datauppsättningar](zfs-datasets.md)
- **Databasdumpar**: PostgreSQL, MySQL och MariaDB, dumpade från den körande servern till en egen återställningspunkt. [Databasdumpar](backups.md#database-dumps)

## Säkerhetskopior och postsidor {#backups}

- **En enda lista** med brickor, **Filter**, **Sortera** och **Sök**, och de senaste fjorton körningarna för varje rad som en remsa. [Listan](backups.md#list)
- **Markera som i en filhanterare**, och säkerhetskopiera eller pausa ett urval på servern. [Välja flera poster](backups.md#select)
- **En sida per post** med **Säkerhetskopior**, **Inställningar** och **Verifiering**. [Postens sida](backups.md#entry-page)
- **Taggar och jämförelse** för ögonblicksbilder, och **Exportera** som tar, zip eller ett restic-repositorium. [Fliken Säkerhetskopior](backups.md#backups-tab)
- **Uteslutningsmönster** med en levande förhandsvisning, och rekommenderade uteslutningar för välkända appar. [Uteslutningsmönster](backups.md#excludes)
- **Kommandon före och efter**, och **Stoppa andra containrar** med en hälsostyrd omstart i beroendeordning. [Kommandon och andra containrar](backups.md#commands)
- **När appen är upptagen**: en schemalagd säkerhetskopiering väntar på Plex, Jellyfin, Emby, din egen kontroll eller lugn trafik och försöker igen på egen hand. [När appen är upptagen](backups.md#app-busy)
- **Ögonblicksbild i stället för stopp**: en container på ZFS är nere bara några sekunder. [Scheman](schedules.md#snapshot-instead-of-stop)
- **Vanliga exporter**, på begäran förseglade med age. [Vanliga exporter](backups.md#exports)

## Återställning {#restore}

- **En sida och ett fönster** för alla slags poster, med den rekommenderade säkerhetskopian förvald. [Återställning](restore.md)
- **Bredvid originalet** som standard, och **Ersätt originalet** säkerhetskopierar först det nuvarande läget. [Återställningsfönstret](restore.md#window)
- **Enskilda filer** till en mapp eller tillbaka till sin plats. [Få tillbaka en enda fil](restore.md#example-single-file)
- **Flera poster på en gång**, var och en från sin rekommenderade säkerhetskopia eller från ett och samma ögonblick. [Återställa flera poster](restore.md#several)
- **Hela Compose-stackar**, startade i beroendeordning. [Återställa en stack](restore.md#stacks)
- **Containrar och VM:ar återskapas** på den image de körde, med GPU, gränser och länkar. [Vad som kommer tillbaka](restore.md#kinds)
- **Kontroller före återställningen**, och en lista över vad den ändrar. [Återställningsfönstret](restore.md#window)
- **Flash som ZIP**, och ett plugin i taget tillbaka in i den körande flashen. [Vad som kommer tillbaka](restore.md#kinds)
- **ZFS** in i datauppsättningen med en säkerhetsögonblicksbild, till en mapp eller till en ny datauppsättning med sina egenskaper. [ZFS-datauppsättningar](zfs-datasets.md#restore)
- **Nödguide** för en förlorad server, med återställningskitet, via gruppen eller med den inskrivna nyckeln. [Nödguide](offsite-recovery.md#emergency)
- **Från en gammal säkerhetskopiemapp**, **Importera från pluginet Appdata.Backup**, och **Jobb från Duplicati eller Duplicacy**, som gör om deras säkerhetskopieringsjobb till mappuppsättningar. [Andra källor](restore.md#other-sources)

## Lagring och scheman {#storage-scheduling}

- **Lagringsmål** av alla slag: lokala mappar, SMB, WebDAV, SFTP, rest-server, S3, molndiskar och vilken rclone-remote som helst. [Lagringsmål](offsite-recovery.md#destinations)
- **Inloggningar**: en lista med en standard. [Inloggningar](offsite-recovery.md#sign-ins)
- **Egna mappar** på den här servern eller i ett lagringsmål. [Ett avsnitts egna mapp](offsite-recovery.md#own-folder)
- **3-2-1 som ett flöde** där varje rad är en omkopplare, och **placering per post**. [Placering per post](offsite-recovery.md#placement)
- **Externa kopior** till flera mål per avsnitt, med bandbreddsgränser, komprimering, kalla lagringsklasser och en inbromsning medan Plex strömmar. [Externa kopior](offsite-recovery.md#off-site-copies)
- **Direkta repositorier**, listade under **Säkerhetskopieras direkt hit**. [Direkta repositorier](offsite-recovery.md#direct-repositories)
- **Gallring som en tidslinje**: **Kort**, **Balanserad**, **Lång** eller **Egna värden** per mål, med regler per avsnitt. [Hur länge det bevaras](offsite-recovery.md#keep)
- **Flytta ett avsnitts historik**, och **Ta bort mål** med eller utan dess säkerhetskopior. [Flytta och ta bort](offsite-recovery.md#move-remove)
- **Scheman som läses som meningar**, bandet **Vecka**, en total säkerhetskopia, planer per avsnitt och en egen plan per post. [Scheman](schedules.md)
- **Kör ikapp missade säkerhetskopior**, en manuell ordning och uppdateringar efter en lyckad säkerhetskopiering. [Scheman](schedules.md)

## Insikt, verifiering och övervakning {#insight-verification-monitoring}

- **Översikten**: statushuvud, **Behöver uppmärksamhet**, **Skyddsstatus**, lagringsprognos, historik och valfria kort. [Översikt](overview.md)
- **Avvikelseupptäckt** per post, som också behåller gamla säkerhetskopior när en källa rasar. [Avvikelser](anomalies.md)
- **Återställningskontroll** efter den första säkerhetskopian, och **Starttest** i en isolerad kopia. [Fliken Verifiering](backups.md#verify-tab)
- **Schemalagda kontroller och teståterställningar** från den externa kopian. [DR-övningar](offsite-recovery.md#dr-drills)
- **Storlek per mapp**, och varför en säkerhetskopiering var långsam. [Vad en körning berättar](backups.md#run-notes)
- **Aviseringar**: webhook, Gotify, Matrix, e-post, Apprise, Unraid och Healthchecks, med en veckosammanfattning och en rapport om förseningar. [Aviseringar](notifications.md)
- **Prometheus**, en Home Assistant-enhet, ett HTTP-API och mDNS. [API och integrationer](api.md)
- **Felrapport**: en redigerad ZIP-fil för ett GitHub-ärende. [Felsökning](troubleshooting.md#still-stuck)

## Skydd mot ransomware {#ransomware-protection}

- **Skydd mot radering (append-only)**, upprätthållet av andra sidan och bevisat med ett manipuleringstest. [Skydd mot radering](offsite-recovery.md#delete-protection-append-only)
- **Acceptera utan skydd mot radering** för en plats som inte kan vägra en radering. [Skydd mot radering](offsite-recovery.md#delete-protection-append-only)
- **Att radera säkerhetskopior för gott kräver din egen inloggning** när ett inloggningslösenord är satt, och en parkopplad telefon kan inte göra det. [Säkerhetsmodell](configuration.md#security-model)
- **Återställningskit**, på begäran förseglat med age, och en **nyckelförvarare** i din grupp. [Återställningskit](offsite-recovery.md#recovery-kit), [Nyckelförvarare](offsite-recovery.md#key-keeper)

## Flera servrar {#several-servers}

- **Parkoppling med tolv ord**, end-to-end-krypterad, direkt eller via ett relä. [Parkoppla instanser](offsite-recovery.md#pairing)
- **Sidan Instanser** med gruppen som ett nät och rollerna **Mottagare**, **Hämtare**, **ZFS-server** och **Nyckelförvarare**. [Instanser och roller](offsite-recovery.md#roles)
- **En mottagande server** som sätts upp åt dig, i append-only-läge. [Mottagande server](offsite-recovery.md#receiving-server)

## AI-assistenter (MCP) {#mcp}

En inbyggd MCP-server låter en assistent läsa säkerhetskopieringsstatus, täckning, körhistorik och återställningspunkter, och med en nyckel som tillåter det starta en säkerhetskopiering. Återställningar, raderingar, rensning och inställningar stannar i webbgränssnittet. [MCP-server](mcp.md)

## Appar och följeslagare {#apps}

- **Android-app**: alla servrar i din grupp i telefonen. [Android-app](android.md)
- **BombVault Widget**: en bricka på Unraids Dashboard. [Appar](settings.md#apps)
- **Inbäddningsbar aktivitetslogg** för instrumentpaneler som Homepage eller Heimdall. [Dashboard-widget](api.md#dashboard-widget)

## Övrigt {#other}

- **42 språk**, tre teman, och etiketter, färger, hörn och animation som du väljer. [Utseende](settings.md#look)
- **Inställningar som sparar sig själva**, med en sökning över alla fjorton brickor. [Inställningar](settings.md)
- **Nyheter** efter en uppdatering, med gröna prickar på det som ändrats. [Info](settings.md#info)
- **HTTPS direkt ur lådan**, och en hälsokontroll för verktyg som reparerar sig själva. [Konfiguration](configuration.md#network)
