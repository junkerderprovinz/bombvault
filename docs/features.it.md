# Funzionalità

Un breve catalogo di ciò che BombVault sa fare. Ogni riga rimanda alla pagina che lo spiega.

## Cosa viene salvato {#backup-scope}

- **Container**: la cartella appdata e la definizione del container, con una scelta di cartelle per ogni container. [Backup ed elementi](backups.md#what-is-saved)
- **VM**: immagini disco, XML e NVRAM UEFI via SSH, con uno spegnimento ordinato, uno snapshot a caldo o solo i blocchi modificati. Le VM con snapshot Unraid e dischi su zvol sono coperte per intero. [Backup delle VM](vm-backup-ssh-setup.md)
- **Flash**: l'intera chiavetta USB di Unraid, con un'esportazione zip pianificata. [Backup ed elementi](backups.md#settings-tab)
- **Auto-backup**: le impostazioni e le credenziali di BombVault stesso. [Procedura guidata di emergenza](offsite-recovery.md#emergency)
- **Cartelle**: qualsiasi cartella del server come set di cartelle. [Aggiungere una cartella](backups.md#add)
- **Dataset ZFS**: un albero di dataset letto da un unico snapshot, con file singoli ripristinabili e una replica facoltativa su un secondo server ZFS. [Dataset ZFS](zfs-datasets.md)
- **Dump dei database**: PostgreSQL, MySQL e MariaDB, scaricati dal server in esecuzione in un punto di ripristino a sé. [Dump dei database](backups.md#database-dumps)

## Backup e pagine degli elementi {#backups}

- **Un solo elenco** con riquadri, **Filtri**, **Ordina** e **Cerca**, e le ultime quattordici esecuzioni di ogni riga come una striscia. [L'elenco](backups.md#list)
- **Seleziona come in un file manager**, ed esegui o sospendi il backup di una selezione sul server. [Selezionare più elementi](backups.md#select)
- **Una pagina per elemento** con **Backup**, **Impostazioni** e **Verifica**. [La pagina di un elemento](backups.md#entry-page)
- **Tag e confronto** per gli snapshot, ed **Esporta** come tar, zip o repository restic. [Scheda Backup](backups.md#backups-tab)
- **Pattern di esclusione** con anteprima in tempo reale, ed esclusioni consigliate per le app più note. [Pattern di esclusione](backups.md#excludes)
- **Comandi prima e dopo**, e **Arresta altri container** con un riavvio condizionato allo stato di salute, in ordine di dipendenza. [Comandi e altri container](backups.md#commands)
- **Quando l'app è occupata**: un backup pianificato aspetta Plex, Jellyfin, Emby, un tuo controllo o il traffico tranquillo, e riprova da solo. [Quando l'app è occupata](backups.md#app-busy)
- **Snapshot invece dello stop**: un container su ZFS resta fermo solo per pochi secondi. [Pianificazioni](schedules.md#snapshot-instead-of-stop)
- **Esportazioni semplici**, se vuoi sigillate con age. [Esportazioni semplici](backups.md#exports)

## Ripristino {#restore}

- **Una pagina e una finestra** per ogni tipo, con il backup consigliato già selezionato. [Ripristino](restore.md)
- **Accanto all'originale** di default, e **Sostituisci l'originale** esegue prima il backup dello stato attuale. [La finestra di ripristino](restore.md#window)
- **File singoli** in una cartella o di nuovo al loro posto. [Riavere un solo file](restore.md#example-single-file)
- **Più elementi insieme**, ciascuno dal suo backup consigliato o da un unico momento. [Ripristinare più elementi](restore.md#several)
- **Interi stack Compose**, avviati in ordine di dipendenza. [Ripristinare uno stack](restore.md#stacks)
- **Container e VM vengono ricreati** sull'immagine su cui giravano, con GPU, limiti e collegamenti. [Cosa torna indietro](restore.md#kinds)
- **Controlli prima del ripristino**, e un elenco di ciò che cambia. [La finestra di ripristino](restore.md#window)
- **La flash come ZIP**, e un plugin alla volta di nuovo nella flash in esecuzione. [Cosa torna indietro](restore.md#kinds)
- **ZFS** dentro il dataset con uno snapshot di sicurezza, in una cartella o in un nuovo dataset con le sue proprietà. [Dataset ZFS](zfs-datasets.md#restore)
- **Procedura guidata di emergenza** per un server perso, con il kit di ripristino, tramite il gruppo o con la chiave digitata. [Procedura guidata di emergenza](offsite-recovery.md#emergency)
- **Da una vecchia cartella di backup**, **Importa dal plugin Appdata.Backup** e **Job da Duplicati o Duplicacy**, che trasforma i loro job di backup in set di cartelle. [Altre sorgenti](restore.md#other-sources)

## Archiviazione e pianificazione {#storage-scheduling}

- **Destinazioni** di ogni tipo: cartelle locali, SMB, WebDAV, SFTP, rest-server, S3, unità cloud e qualsiasi remote rclone. [Destinazioni](offsite-recovery.md#destinations)
- **Credenziali**: un unico elenco con una predefinita. [Credenziali](offsite-recovery.md#sign-ins)
- **Cartelle proprie** su questo server o in una destinazione. [La cartella propria di una sezione](offsite-recovery.md#own-folder)
- **3-2-1 come un flusso** in cui ogni linea è un interruttore, e **destinazioni per elemento**. [Destinazioni per elemento](offsite-recovery.md#placement)
- **Copie off-site** su più destinazioni per sezione, con limiti di banda, compressione, classi di archiviazione fredda e un rallentamento mentre Plex fa streaming. [Copie off-site](offsite-recovery.md#off-site-copies)
- **Repository diretti**, elencati sotto **Salva direttamente qui**. [Repository diretti](offsite-recovery.md#direct-repositories)
- **Conservazione come linea del tempo**: **Breve**, **Equilibrato**, **Lungo** o **Valori personalizzati** per destinazione, con regole per sezione. [Per quanto tempo conservare](offsite-recovery.md#keep)
- **Sposta la storia di una sezione**, e **Rimuovi destinazione** con o senza i suoi backup. [Spostare e rimuovere](offsite-recovery.md#move-remove)
- **Pianificazioni che si leggono come frasi**, la fascia **Settimana**, un backup completo, pianificazioni per sezione e un piano proprio per elemento. [Pianificazioni](schedules.md)
- **Recupera i backup mancati**, un ordine manuale e aggiornamenti dopo un backup riuscito. [Pianificazioni](schedules.md)

## Panoramica, verifica e monitoraggio {#insight-verification-monitoring}

- **La panoramica**: intestazione di stato, **Richiede attenzione**, **Stato di protezione**, previsione dello spazio, cronologia e schede facoltative. [Panoramica](overview.md)
- **Rilevamento delle anomalie** per elemento, che conserva anche i backup vecchi quando una sorgente crolla. [Anomalie](anomalies.md)
- **Verifica del ripristino** dopo il primo backup, e **Test di avvio** in una copia isolata. [Scheda Verifica](backups.md#verify-tab)
- **Controlli pianificati e ripristini di prova** dalla copia off-site. [Prove DR](offsite-recovery.md#dr-drills)
- **Dimensione per cartella**, e perché un backup è stato lento. [Cosa ti dice un'esecuzione](backups.md#run-notes)
- **Notifiche**: un elenco di canali per Unraid, ntfy, Gotify, Discord, Slack, Matrix, email, Apprise, qualsiasi webhook e Healthchecks, ognuno con la sua regola, con un riepilogo settimanale e un rapporto sui ritardi. [Notifiche](notifications.md)
- **Prometheus**, un dispositivo Home Assistant, un'API HTTP e mDNS. [API e integrazioni](api.md)
- **Segnala bug**: uno ZIP ripulito per una issue su GitHub. [Risoluzione dei problemi](troubleshooting.md#still-stuck)

## Protezione dal ransomware {#ransomware-protection}

- **Protezione dall'eliminazione (append-only)**, imposta dal lato remoto e provata da una prova di manomissione. [Protezione dall'eliminazione](offsite-recovery.md#delete-protection-append-only)
- **Accetta senza protezione dall'eliminazione** per un posto che non sa rifiutare una cancellazione. [Protezione dall'eliminazione](offsite-recovery.md#delete-protection-append-only)
- **Eliminare i backup in via definitiva richiede il tuo accesso** quando è impostata una password di accesso, e un telefono associato non può farlo. [Modello di sicurezza](configuration.md#security-model)
- **Kit di ripristino**, se vuoi sigillato con age, e un **custode della chiave** nel tuo gruppo. [Kit di ripristino](offsite-recovery.md#recovery-kit), [Custode della chiave](offsite-recovery.md#key-keeper)

## Più server {#several-servers}

- **Associazione con dodici parole**, cifrata end-to-end, direttamente o tramite un relay. [Associare le istanze](offsite-recovery.md#pairing)
- **La pagina Istanze** con il gruppo come una rete e i ruoli **Ricevitore**, **Prelevatore**, **Server ZFS** e **Custode della chiave**. [Istanze e ruoli](offsite-recovery.md#roles)
- **Un server ricevente** configurato per te, in modalità append-only. [Server ricevente](offsite-recovery.md#receiving-server)
- **Un repository condiviso** in cui copiano più server, così ciò che hanno in comune viene salvato una volta sola. Pota solo il server che lo ospita. [Repository condiviso](offsite-recovery.md#shared-repository)

## Assistenti IA (MCP) {#mcp}

Un server MCP integrato permette a un assistente di leggere lo stato dei backup, la copertura, la cronologia delle esecuzioni e i punti di ripristino, e con una chiave che lo consente di avviare un backup. Ripristini, eliminazioni, potature e impostazioni restano nell'interfaccia web. [Server MCP](mcp.md)

## App e companion {#apps}

- **App Android**: tutti i server del tuo gruppo sul telefono. [App Android](android.md)
- **La fortezza**: BombVault si unisce alle altre app della famiglia. ShipLog e TowerTorch chiedono un backup prima di cambiare un container, FireSquire controlla quando è stato fatto l'ultimo backup della flash, e le app che sanno fare il backup di sé stesse lo consegnano a BombVault. [La fortezza](offsite-recovery.md#fortress)
- **TollRoll**: un solo riquadro nella Dashboard di Unraid per BombVault e le altre app della tua fortezza. [App](settings.md#apps)
- **Registro attività incorporabile** per dashboard come Homepage o Heimdall. [Widget per dashboard](api.md#dashboard-widget)

## Altro {#other}

- **42 lingue**, tre temi, ed etichette, colori, angoli e animazioni a tua scelta. [Aspetto](settings.md#look)
- **Impostazioni che si salvano da sole**, con una ricerca su tutte e quattordici le schede. [Impostazioni](settings.md)
- **Novità** dopo un aggiornamento, con punti verdi su ciò che è cambiato. [Info](settings.md#info)
- **HTTPS pronto all'uso**, e un healthcheck per gli strumenti di auto-riparazione. [Configurazione](configuration.md#network)
