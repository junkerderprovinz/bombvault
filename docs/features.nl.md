# Functies

Een korte catalogus van wat BombVault doet. Elke regel linkt naar de pagina die het uitlegt.

## Back-upbereik {#backup-scope}

- **Containers**: de appdata-map en de containerdefinitie, met een keuze van mappen per container. [Back-ups en items](backups.md#what-is-saved)
- **VM's**: schijfimages, XML en UEFI NVRAM via SSH, met een nette afsluiting, een live-snapshot of alleen gewijzigde blokken. VM's met Unraid-snapshots en schijven op zvols worden volledig gedekt. [VM-back-ups](vm-backup-ssh-setup.md)
- **Flash**: de hele USB-flash van Unraid, met een geplande zip-export. [Back-ups en items](backups.md#settings-tab)
- **Zelf-back-up**: de eigen instellingen en inloggegevens van BombVault. [Noodassistent](offsite-recovery.md#emergency)
- **Mappen**: elke map op de server als mappenset. [Een map toevoegen](backups.md#add)
- **ZFS-datasets**: een datasetboom gelezen uit één snapshot, met losse bestanden die te herstellen zijn en een optionele replica op een tweede ZFS-server. [ZFS-datasets](zfs-datasets.md)
- **Databasedumps**: PostgreSQL, MySQL en MariaDB, gedumpt vanaf de draaiende server in een eigen herstelpunt. [Databasedumps](backups.md#database-dumps)

## Back-ups en itempagina's {#backups}

- **Eén lijst** met tegels, **Filters**, **Sorteren** en **Zoeken**, en de laatste veertien runs van elke rij als strook. [De lijst](backups.md#list)
- **Selecteren zoals in een bestandsbeheerder**, en een selectie op de server back-uppen of pauzeren. [Meerdere items kiezen](backups.md#select)
- **Een pagina per item** met **Back-ups**, **Instellingen** en **Verificatie**. [De pagina van een item](backups.md#entry-page)
- **Tags en vergelijken** voor snapshots, en **Exporteren** als tar, zip of een restic-repository. [Tabblad Back-ups](backups.md#backups-tab)
- **Uitsluitpatronen** met een live voorbeeld, en aanbevolen uitsluitingen voor bekende apps. [Uitsluitpatronen](backups.md#excludes)
- **Opdrachten ervoor en erna**, en **Andere containers stoppen** met een herstart op gezondheid in afhankelijkheidsvolgorde. [Opdrachten en andere containers](backups.md#commands)
- **Als de app bezig is**: een geplande back-up wacht op Plex, Jellyfin, Emby, je eigen controle of rustig verkeer, en probeert het uit zichzelf opnieuw. [Als de app bezig is](backups.md#app-busy)
- **Snapshot in plaats van stoppen**: een container op ZFS ligt maar een paar seconden stil. [Planningen](schedules.md#snapshot-instead-of-stop)
- **Gewone exports**, eventueel verzegeld met age. [Gewone exports](backups.md#exports)

## Herstel {#restore}

- **Eén pagina en één venster** voor elk soort, met de aanbevolen back-up vooraf gekozen. [Herstel](restore.md)
- **Naast het origineel** als standaard, en **Origineel vervangen** maakt eerst een back-up van de huidige staat. [Het herstelvenster](restore.md#window)
- **Losse bestanden** naar een map of terug op hun plek. [Eén bestand terughalen](restore.md#example-single-file)
- **Meerdere items tegelijk**, elk vanaf zijn aanbevolen back-up of vanaf één moment. [Meerdere items herstellen](restore.md#several)
- **Hele Compose-stacks**, gestart in afhankelijkheidsvolgorde. [Een stack herstellen](restore.md#stacks)
- **Containers en VM's worden opnieuw aangemaakt** op de image waarop ze liepen, met GPU, limieten en koppelingen. [Wat er terugkomt](restore.md#kinds)
- **Controles vóór het herstel**, en een lijst van wat het verandert. [Het herstelvenster](restore.md#window)
- **Flash als ZIP**, en één plug-in per keer terug in de draaiende flash. [Wat er terugkomt](restore.md#kinds)
- **ZFS** in de dataset met een veiligheidssnapshot, naar een map of in een nieuwe dataset met zijn eigenschappen. [ZFS-datasets](zfs-datasets.md#restore)
- **Noodassistent** voor een verloren server, met de herstelkit, via de groep of met de ingetypte sleutel. [Noodassistent](offsite-recovery.md#emergency)
- **Uit een oude back-upmap**, **Importeren uit de Appdata.Backup-plug-in** en **Taken uit Duplicati of Duplicacy**, waarmee hun back-uptaken mappensets worden. [Andere bronnen](restore.md#other-sources)

## Opslag en planning {#storage-scheduling}

- **Bestemmingen** van elk soort: lokale mappen, SMB, WebDAV, SFTP, rest-server, S3, cloudschijven en elke rclone-remote. [Bestemmingen](offsite-recovery.md#destinations)
- **Inloggegevens**: één lijst met een standaard. [Inloggegevens](offsite-recovery.md#sign-ins)
- **Eigen mappen** op deze server of in een bestemming. [De eigen map van een onderdeel](offsite-recovery.md#own-folder)
- **3-2-1 als een stroom** waarin elke lijn een schakelaar is, en **plaatsing per item**. [Plaatsing per item](offsite-recovery.md#placement)
- **Offsite-kopieën** naar meerdere bestemmingen per onderdeel, met bandbreedtelimieten, compressie, koude opslagklassen en een vertraging terwijl Plex streamt. [Off-site-kopieën](offsite-recovery.md#off-site-copies)
- **Directe repository's**, getoond onder **Back-upt rechtstreeks hierheen**. [Directe repository's](offsite-recovery.md#direct-repositories)
- **Bewaarbeleid als tijdlijn**: **Kort**, **Gebalanceerd**, **Lang** of **Eigen waarden** per bestemming, met regels per onderdeel. [Hoe lang bewaren](offsite-recovery.md#keep)
- **De geschiedenis van een onderdeel verplaatsen**, en **Bestemming verwijderen** met of zonder de back-ups. [Verplaatsen en verwijderen](offsite-recovery.md#move-remove)
- **Planningen die als zinnen lezen**, de band **Week**, een volledige back-up, planningen per onderdeel en een eigen planning per item. [Planningen](schedules.md)
- **Gemiste back-ups inhalen**, een handmatige volgorde, en updates na een geslaagde back-up. [Planningen](schedules.md)

## Inzicht, verificatie en bewaking {#insight-verification-monitoring}

- **Het overzicht**: statuskop, **Aandacht nodig**, **Beschermingsstatus**, opslagprognose, geschiedenis en optionele kaarten. [Overzicht](overview.md)
- **Anomaliedetectie** per item, die ook oude back-ups bewaart als een bron instort. [Anomalieën](anomalies.md)
- **Herstelcontrole** na de eerste back-up, en **Starttest** in een geïsoleerde kopie. [Tabblad Verificatie](backups.md#verify-tab)
- **Geplande controles en testherstellen** vanaf de off-site-kopie. [DR-oefeningen](offsite-recovery.md#dr-drills)
- **Grootte per map**, en waarom een back-up traag was. [Wat een run je vertelt](backups.md#run-notes)
- **Meldingen**: een lijst met kanalen voor Unraid, ntfy, Gotify, Discord, Slack, Matrix, e-mail, Apprise, elke webhook en Healthchecks, elk met een eigen regel, met een wekelijkse samenvatting en een melding van achterstallige back-ups. [Meldingen](notifications.md)
- **Prometheus**, een Home Assistant-apparaat, een HTTP-API en mDNS. [API en integraties](api.md)
- **Bugrapport**: een geredigeerde ZIP voor een GitHub-issue. [Problemen oplossen](troubleshooting.md#still-stuck)

## Bescherming tegen ransomware {#ransomware-protection}

- **Bescherming tegen verwijderen (append-only)**, afgedwongen door de andere kant en bewezen door een sabotagetest. [Bescherming tegen verwijderen](offsite-recovery.md#delete-protection-append-only)
- **Accepteren zonder bescherming tegen verwijderen** voor een plek die een verwijdering niet kan weigeren. [Bescherming tegen verwijderen](offsite-recovery.md#delete-protection-append-only)
- **Back-ups definitief verwijderen vraagt je eigen aanmelding** zodra er een inlogwachtwoord is ingesteld, en een gekoppelde telefoon kan het niet. [Beveiligingsmodel](configuration.md#security-model)
- **Herstelkit**, eventueel verzegeld met age, en een **sleutelbewaarder** in je groep. [Herstelkit](offsite-recovery.md#recovery-kit), [Sleutelbewaarder](offsite-recovery.md#key-keeper)

## Meerdere servers {#several-servers}

- **Koppelen met twaalf woorden**, end-to-end versleuteld, rechtstreeks of via een relay. [Instanties koppelen](offsite-recovery.md#pairing)
- **De pagina Instanties** met de groep als net en de rollen **Ontvanger**, **Ophaler**, **ZFS-server** en **Sleutelbewaarder**. [Instanties en rollen](offsite-recovery.md#roles)
- **Een ontvangende server** die voor je wordt ingesteld, in append-only-modus. [Ontvangserver](offsite-recovery.md#receiving-server)
- **Een gedeelde repository** waar meerdere servers naartoe kopiëren, zodat wat ze gemeen hebben maar één keer wordt opgeslagen. Alleen de server die er eigenaar van is, schoont op. [Gedeelde repository](offsite-recovery.md#shared-repository)

## AI-assistenten (MCP) {#mcp}

Een ingebouwde MCP-server laat een assistent de back-upstatus, de dekking, de runhistorie en de herstelpunten lezen, en met een sleutel die het toestaat een back-up starten. Herstellen, verwijderen, opschonen en instellingen blijven in de webinterface. [MCP-server](mcp.md)

## Apps en aanvullingen {#apps}

- **Android-app**: elke server van je groep op je telefoon. [Android-app](android.md)
- **De vesting**: BombVault sluit zich aan bij de andere apps van de familie. ShipLog en TowerTorch vragen om een back-up voordat ze een container veranderen, FireSquire kijkt wanneer de flash voor het laatst een back-up kreeg, en apps die een back-up van zichzelf kunnen maken, geven die aan BombVault. [De vesting](offsite-recovery.md#fortress)
- **TollRoll**: één tegel op het Unraid-dashboard voor BombVault en de andere apps van je vesting. [Apps](settings.md#apps)
- **Insluitbaar activiteitenlogboek** voor dashboards zoals Homepage of Heimdall. [Dashboardwidget](api.md#dashboard-widget)

## Overig {#other}

- **42 talen**, drie thema's, en labels, kleuren, hoeken en animatie naar keuze. [Uiterlijk](settings.md#look)
- **Instellingen die zichzelf opslaan**, met zoeken over alle veertien tegels. [Instellingen](settings.md)
- **Wat is er nieuw** na een update, met groene stippen bij wat is veranderd. [Info](settings.md#info)
- **HTTPS kant-en-klaar**, en een healthcheck voor auto-heal-tools. [Configuratie](configuration.md#network)
