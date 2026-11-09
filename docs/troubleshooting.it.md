# Risoluzione dei problemi

Una breve FAQ. Per la tabella completa di risoluzione dei problemi lato host del backup delle VM via SSH (permission-denied, verifica della chiave host, variabili del template mancanti e altro), vedi la [guida al backup delle VM via SSH](vm-backup-ssh-setup.md).

## Qualcosa non è collegato correttamente {#something-is-not-wired-up-correctly}

Aggiungi alla panoramica la scheda **Integrazione host** in **Personalizza**. La verifica dell'integrazione host sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti, e **Controlla ora** la esegue di nuovo. Inizia da qui prima di dare per scontato un bug: un mount mancante o un host irraggiungibile si presenta subito.

## Non riesco a raggiungere l'interfaccia web {#i-cannot-reach-the-web-ui}

BombVault serve HTTPS pronto all'uso sulla porta `3443` (certificato autofirmato), quindi apri `https://<your-unraid-ip>:3443`. Accetta l'avviso del certificato autofirmato, oppure metti BombVault dietro un reverse proxy con il tuo certificato. Se esegui con `HTTP_ONLY=true`, serve invece HTTP in chiaro sulla porta `3000` (pensato per l'uso dietro un proxy che termina il TLS).

## Ho perso la mia APP_KEY {#i-lost-my-app_key}

`APP_KEY` deriva la password del repository restic. Senza di essa, senza il kit di ripristino e senza un deposito della chiave nel tuo gruppo, i backup cifrati non possono essere recuperati. Ecco perché la panoramica mostra **Salva il tuo kit di ripristino** finché non l'hai scaricato. Vedi [Off-site e ripristino](offsite-recovery.md#recovery-kit). Genera una chiave con `openssl rand -hex 32` e conservala fuori dal server prima di affidarti a qualsiasi backup.

## Ho dimenticato la password di accesso {#i-forgot-the-login-password}

Scegli **Password dimenticata?** nella pagina di accesso. BombVault scrive un codice di 12 caratteri nel log del suo container (su Unraid: scheda Docker, icona di BombVault, **Logs**). Digitalo con una nuova password entro 10 minuti. Il reset disattiva anche l'autenticazione a due fattori, rimuove ogni passkey, revoca ogni token API, chiave MCP e accesso OAuth, chiude ogni altra sessione e disattiva l'associazione dei telefoni. Aggiungi di nuovo le tue passkey in **Impostazioni, Sicurezza**, riattiva lì l'associazione dei telefoni e crea nuovi token e chiavi dove ti servono ancora. Vedi [Modello di sicurezza](configuration.md#login).

## I backup si sono fermati e dicono che la chiave non si può più aprire {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Questo server ha adottato la chiave dei backup di un altro server nella Procedura guidata di emergenza, e `APP_KEY` è stata cambiata dopo. La chiave adottata è sigillata con la vecchia APP_KEY, quindi BombVault non può aprirla e non salva nulla piuttosto che scrivere con la chiave sbagliata. **Richiede attenzione** nella panoramica la nomina. Rimetti APP_KEY com'era, oppure carica di nuovo il kit di ripristino nella pagina Ripristino sotto **Emergenza**.

## Un telefono finisce nella pagina di accesso {#a-phone-lands-on-the-login-page}

Un telefono associato riceve la sessione tramite il gruppo solo finché **I telefoni possono associarsi con le dodici parole del gruppo** è attivo in **Impostazioni, Sicurezza**. Un reset della password lo disattiva. Anche un'app più vecchia del server non riceve una sessione finché non viene aggiornata. Un server aggiunto con il suo indirizzo chiede sempre la password. Un telefono rimosso dalla pagina Istanze torna disattivato quando la sua app si riapre. Attiva **Interroga questa istanza** sotto **Monitoraggio** nei suoi dettagli per farlo entrare.

## Il backup delle VM non si connette {#vm-backup-will-not-connect}

Il backup delle VM parla con libvirt via SSH, mai tramite un mount.

- Conferma che SSH sia attivo sull'host e che la chiave pubblica di BombVault sia autorizzata in `/root/.ssh/authorized_keys` (Impostazioni, Integrazioni, SSH dell'host mostra la chiave e un pulsante **Prova connessione**).
- Su una rete personalizzata `br0.x`, imposta `LIBVIRT_HOST` sull'IP LAN del tuo Unraid (lì il container non può raggiungere l'host tramite `host.docker.internal`). Attiva **Settings, Docker, Host access to custom networks**.
- Se hai cambiato la porta SSH di Unraid, imposta `LIBVIRT_SSH_PORT` di conseguenza.
- Su TrueNAS imposta `LIBVIRT_URI` con il socket libvirt di TrueNAS stesso, dai all'app la capability `SETPCAP` e aggiungi la chiave a root in **Credentials, Users** (vedi [TrueNAS SCALE](getting-started.md#truenas)).
- La diagnosi completa passo per passo (prova di raggiungibilità, instradamento VLAN, `Permission denied (publickey)`, `Host key verification failed`) è nella [guida al backup delle VM via SSH](vm-backup-ssh-setup.md).

## Uno snapshot a caldo di una VM non è partito {#a-live-vm-snapshot-did-not-run}

Gli snapshot a caldo richiedono il disco su `/mnt/cache` (o `/mnt/diskX`), non su `/mnt/user`. Con il guest agent di qemu nella VM lo snapshot è quiescente, e senza è coerente come dopo un crash. Su una VM spenta, il metodo a caldo ripiega automaticamente su quello ordinato. Un backup ordinato spegne la VM, salva i dischi e poi la riavvia, quindi è sempre coerente. Aspetta fino a dieci minuti che il guest si spenga e non lo forza mai, così un guest Windows che sta installando aggiornamenti non viene interrotto a metà. Un guest che gira ancora dopo quel tempo viene lasciato in pace, e quel backup fallisce con il motivo.

## Un backup è fallito con "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Di solito è un lock restic orfano rimasto quando il container è stato aggiornato o riavviato nel mezzo di un'operazione. BombVault riconosce un lock dimostrabilmente orfano, lo rimuove e riprova una volta, in automatico. Dopo un riavvio aspetta che un lock simile sia rimasto dieci minuti senza essere rinnovato. Un restic ancora in esecuzione rinnova il suo lock ogni cinque minuti, e BombVault non rimuove mai un lock che viene ancora rinnovato, perché può appartenere a un secondo BombVault che scrive nello stesso repository. Un lock rimasto 30 minuti senza essere rinnovato viene rimosso, chiunque l'abbia preso. Se l'errore persiste, apri la scheda **Controlla i backup** in **Impostazioni, Verifica** e scegli **Sblocca** sotto **Altro** nella riga della sezione interessata per rimuovere a mano un lock rimasto. Anche **Sblocca** lascia un lock che viene ancora rinnovato e ti dice quando lo è stato l'ultima volta. Un problema vero emerge comunque e non viene nascosto.

## La mia copia off-site non è partita dopo un backup {#my-off-site-copy-did-not-happen-after-a-backup}

La replica off-site è per scelta al meglio delle possibilità, quindi un intoppo off-site non fa mai fallire il backup locale. Controlla quando la destinazione copia: la sua scheda **Copia** dice **Dopo ogni backup** o nomina un piano proprio, e **Impostazioni, Calendario, Copie off-site per sezione** può dare a una sezione un piano suo. Usa **Replica ora** nella riga della sezione della scheda **Copie off-site** in **Impostazioni, Destinazioni** per un'esecuzione a richiesta, e guarda l'intestazione di stato della panoramica, che dice **Copia off-site in corso** mentre gira. Una copia fallita compare sotto **Richiede attenzione**.

## Il certificato di un rest-server non è considerato attendibile {#a-rest-servers-certificate-is-not-trusted}

restic rifiuta un certificato che non conosce, e l'errore dice dove metterlo. Copia il certificato autofirmato del rest-server, o quello dell'autorità che lo ha firmato, come file PEM nella cartella `certs` della cartella dati di BombVault (`/config/certs` nel container, `appdata/bombvault/certs` su Unraid) e prova di nuovo. Non serve alcun riavvio.

## Un backup dice che i dati fuori da Host Data non sono stati salvati {#data-outside-host-data}

Il container tiene dati in una cartella che BombVault non può leggere attraverso Host Data. Il backup salva la definizione e ogni cartella che riesce a raggiungere, colora di arancione la sua riga nel registro attività e nomina le cartelle che ha lasciato fuori. Due casi sono comuni: un ixVolume su TrueNAS sotto `/mnt/.ix-apps`, e un volume Docker con nome sotto `/var/lib/docker/volumes`, come il volume del database di uno stack compose. La scheda **Cartelle da salvare** nella scheda **Impostazioni** del container elenca ogni cartella simile con il suo percorso dentro il container e il suo percorso sull'host, segnata in rosso.

Un volume Docker anonimo non viene segnalato. Docker ne crea uno da solo per una cartella che l'immagine dichiara e che nessuno ha mappato, lo chiama con 64 cifre esadecimali e lo tiene sotto `/var/lib/docker/volumes`. Di solito contiene una cache, quindi il backup resta verde e la scheda si limita a elencarlo, con una nota che è anonimo.

- **Vuoi che quei dati siano salvati.** Mappa la cartella su un percorso dell'host sotto Host Data, per esempio una cartella nella tua condivisione appdata, e il prossimo backup la include.
- **Non ti servono.** Aggiungi uno dei suoi due percorsi come riga nella scheda **Pattern di esclusione** della stessa scheda: il percorso dentro il container, come `/cache`, oppure il percorso dell'host che l'avviso nomina, come `/var/lib/docker/volumes/jellyfin_cache/_data`. Dal prossimo backup l'avviso lascia fuori quella cartella, e l'anteprima sotto il campo ne spiega il motivo. Funziona anche una riga per una cartella sopra di essa, mentre un nome semplice o `/` no.

## Un backup dice Rinviato {#deferred-backups}

L'app del container era occupata, quindi il suo backup aspetta e viene riprovato da solo. La riga nel registro attività dice perché e quando è il prossimo tentativo. La scheda **Quando l'app è occupata** nella scheda **Impostazioni** del container dice lo stesso e quando finisce l'attesa. Non c'è nulla da fare. Per salvare subito usa **Esegui backup ora**: un backup che avvii tu non aspetta mai, nemmeno per un codice di uscita 75.

- **Un tuo controllo.** Fai uscire il **Comando pre-backup** con il codice 75 mentre l'app è occupata, per esempio `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Qualsiasi altro codice diverso da 0 fa fallire il backup come prima, quindi uno script rotto compare comunque sotto **Richiede attenzione**.
- **Plex, Jellyfin o Emby non viene mai interrogato.** BombVault interroga prima il server all'indirizzo del container stesso e, quando non risponde, esegue curl o wget dentro il container. Un'immagine con nessuno dei due non può essere interrogata tramite sé stessa. Il registro lo dice allora. Con **Esegui il backup quando il server non risponde** attivo, un backup simile parte comunque, con l'opzione disattivata aspetta.
- **Jellyfin o Emby rifiuta.** Controlla la **Chiave API**. Si crea nella dashboard del server sotto API keys.
- **Plex rifiuta.** BombVault legge il token da `Preferences.xml` nella cartella montata su `/config`. Un server in cui non si è mai fatto l'accesso non ne ha, e risponde solo dove consente l'accesso senza.
- **Un film in pausa tiene il backup in attesa tutta la notte.** Imposta **Ignora le sessioni in pausa dopo (min)**, per esempio a 30.
- **Il controllo del traffico non lascia mai passare il backup.** Alza **Occupato da (KiB/s)** sopra ciò che il container invia quando nessuno lo usa. Subito dopo l'avvio di BombVault, il controllo aspetta di aver osservato il container per **Tranquillo per (min)** minuti.
- **Il backup non gira mai di notte.** Con una fascia oraria impostata, i tentativi avvengono solo al suo interno. Un'attesa che raggiunge la scadenza fuori dalla fascia finisce comunque alla scadenza.

## Un backup grande è stato terminato per mancanza di memoria {#a-large-backup-was-killed-for-running-out-of-memory}

Imposta **Limite di memoria per backup (MiB, 0 = nessuno)** nella scheda **Prestazioni** in **Impostazioni, Sistema**, ben sotto quanto il container può usare. Vicino al limite restic libera memoria più spesso, il che costa un po' di velocità, così il backup rallenta invece di essere terminato.

## Un ripristino è stato interrotto prima di partire {#a-restore-aborted-before-it-started}

Prima che venga fermato o rimosso qualcosa, il ripristino esegue un controllo preliminare dei conflitti: verifica che l'IP statico del container e le porte pubblicate dell'host siano libere. Se un altro container ne tiene già una, si interrompe con un messaggio chiaro e utilizzabile invece di lasciare un ripristino a metà. Libera la porta o l'IP in conflitto, poi riprova.

Se la rete del container non esiste, per esempio perché la sua app TrueNAS è stata eliminata, la verifica del ripristino nomina la rete e il ripristino rifiuta prima di cambiare qualsiasi cosa. Reinstalla l'app in TrueNAS con lo stesso nome, cosa che ricrea la rete, poi ripristina i suoi dati.

## Un'esportazione semplice è fallita invece di scrivere un file {#a-plain-export-failed-instead-of-writing-a-file}

Se **Cifra le esportazioni con age** è attivo in **Impostazioni, Sistema** ma non è impostato alcun destinatario valido, un'esportazione fallisce con un errore chiaro invece di scrivere testo in chiaro. Aggiungi un destinatario valido (una chiave pubblica age o una chiave pubblica SSH), oppure disattiva la cifratura se vuoi che l'esportazione sia in chiaro. Vedi [Esportazioni semplici](backups.md#exports).

## Un dump del database è fallito {#a-database-dump-failed}

Un dump fallito non fa mai fallire il backup che lo circonda. Viene registrato come esecuzione fallita a sé, e il motivo nomina cosa sistemare.

- **Accesso rifiutato.** Il dump accede con le variabili di password del container stesso (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` o le loro versioni `_FILE`). Controllale sul container del database. Una variabile `_FILE` che punta a un segreto che l'utente del container stesso non può leggere fallisce allo stesso modo.
- **Privilegi mancanti.** Con una password root casuale il dump può accedere solo come utente dell'app, quindi contiene quell'unico database, e MySQL 8.4 e successivi possono rifiutarlo del tutto. Dai al container una vera password root, oppure disattiva il dump per esso.
- **Le tabelle di sistema richiedono un aggiornamento.** MariaDB rifiuta di fare il dump quando le sue tabelle di sistema vengono da una versione più vecchia (errore 1558). Aggiungi la variabile `MARIADB_AUTO_UPGRADE=1` e riavvia il container, oppure esegui `mariadb-upgrade` al suo interno una volta.
- **Nessuno strumento di dump.** Un'immagine ridotta o costruita a mano senza `pg_dump`, `mysqldump` o `mariadb-dump` non può essere scaricata in un dump. Usa l'immagine ufficiale, oppure disattiva il dump.
- **Un limite di tempo.** Un dump ottiene `DB_DUMP_MAX_HOURS` (6 di default), il backup che lo circonda ottiene `BACKUP_MAX_HOURS`, e un dump che smette di fare progressi viene interrotto dopo `BACKUP_STALL_HOURS`. La causa abituale dell'ultimo è un lock tenuto dall'applicazione. Alza il limite che è scattato, oppure fai il dump quando l'applicazione è tranquilla.
- **Il container è in pausa o si riavvia di continuo.** Il dump parla con il server in esecuzione. Se il container si riavvia di continuo, il suo log dice perché.
- **Un dump danneggiato non è stato rimosso.** Un dump che BombVault non ha potuto finire viene eliminato di nuovo. Quando quell'eliminazione fallisce, il dump resta nell'elenco segnato come danneggiato, e lo puoi eliminare da lì.

## Un'importazione è fallita {#an-import-failed}

Un'importazione ferma il container, sposta da parte la sua cartella dei dati e lascia che l'immagine ne crei una vuota al suo posto. Se un passaggio prima dell'importazione vera e propria fallisce, la vecchia cartella viene rimessa automaticamente. Se l'importazione fallisce, il container tiene la cartella nuova e quella vecchia resta accanto come `<data folder>.bombvault-before-import-<timestamp>`. Il messaggio di errore dell'esecuzione nomina il percorso esatto.

Una cartella dei dati che è un dataset a sé, come su TrueNAS, non si può rinominare. L'importazione lascia allora la cartella al suo posto e sposta il suo contenuto nella cartella conservata accanto, quindi il dataset sopra ha bisogno di spazio per esso. L'importazione lo verifica prima di fermare il container.

Per rimetterla a mano: ferma il container, rinomina la cartella dei dati attuale togliendola di mezzo, rinomina la cartella conservata con il nome originale e avvia il container. Per un dataset, sposta il contenuto invece di rinominare le cartelle. Su Unraid, il file manager lo fa dalla scheda Shares.

## Un backup di un dataset ZFS è fallito o ha saltato un dataset {#zfs-datasets}

Ogni problema porta un codice motivo tra parentesi, e la pagina [Dataset ZFS](zfs-datasets.md#reason-codes) li elenca tutti con la soluzione. I tre più comuni:

- **`snapshot-loop`**: lo snapshot non è arrivato a BombVault perché Host Data non lascia passare i nuovi mount. Modifica il container, imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault.
- **`key-not-loaded`**: un dataset cifrato la cui chiave non è caricata viene saltato. Carica la chiave con `zfs load-key` e monta il dataset. Il prossimo backup lo include.
- **`ssh-auth`**: il server ha rifiutato la chiave di BombVault. **Connessione ZFS a questo server**, aperta con **Mostra connessione** nella pagina Backup, mostra il comando che la autorizza. Eseguilo una volta sul server.

## Un elemento resta a "Sta imparando N/10" {#an-entry-stays-at-learning-n10}

La maggior parte dei controlli sulle anomalie parte dopo 10 backup riusciti di un elemento, e il conteggio riparte dopo **Segna come attesa** e dopo che la selezione dell'elemento è cambiata. Un elemento che non è pianificato non impara, e un container senza appdata non ha nulla da cui imparare, cosa che il suo badge dice. **Avanzamento dell'apprendimento** nella pagina Anomalie mostra per ogni elemento un anello di dieci parti.

## La conservazione ha smesso di eliminare i backup vecchi di un elemento {#retention-stopped-deleting-old-backups-of-one-entry}

Li tiene fermi un'anomalia critica aperta: la sorgente dell'elemento è quasi vuota, si è ridotta molto, oppure un backup ha salvato di nuovo la maggior parte dei suoi dati. Apri l'anomalia dal badge sull'elemento. Se mancano dati o sono stati cifrati, ripristina prima dall'ultimo backup buono collegato. Poi conferma la presa visione dell'anomalia, oppure segnala come attesa se il cambiamento era tuo, e la prossima esecuzione pota come al solito. L'anteprima della conservazione segna un elemento simile come conservato. Per un elemento ZFS solo il dataset nominato nell'anomalia mantiene i suoi backup vecchi, e gli altri dataset dell'albero vengono potati come al solito.

## La potatura manuale dice che alcuni elementi sono stati conservati {#manual-prune-says-some-entries-were-kept}

Stessa causa: la potatura lascia in pace i backup vecchi di un elemento con un'anomalia simile e nomina l'elemento nel suo messaggio. Tutto il resto viene potato come al solito.

## Un backup fallisce con "on no mounted pool or share" {#no-mounted-pool}

Su Unraid `/mnt` vive in memoria, quindi una cartella lì dentro senza un pool montato sopra sparisce al riavvio successivo e resta nascosta quando il pool torna. BombVault si rifiuta di scriverci un backup o uno ZIP della flash, anche quando in quella cartella c'è già un repository di una scrittura precedente. Monta il pool ed esegui di nuovo il backup.

## L'importazione della cronologia dice che un repository non si è potuto leggere {#history-import-says-a-repository-could-not-be-read}

Dopo l'aggiornamento BombVault legge una volta da ogni repository le dimensioni dei backup precedenti. Un repository che in quel momento non ha potuto raggiungere, come una destinazione off-site spenta o una condivisione non montata, viene contato nella scheda **Anomalie** in **Impostazioni, Verifica** e riprovato una volta al giorno. Nel frattempo i suoi elementi imparano dai nuovi backup.

## L'avviso sullo spazio su disco non corrisponde alla dashboard di Unraid {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Sulla condivisione utente di Unraid (`/mnt/user`) lo spazio libero è quello dell'intero array, non di un solo disco. Le destinazioni remote che BombVault raggiunge tramite rclone, come SFTP, la Hetzner Storage Box, SMB, WebDAV o un'unità cloud, vengono misurate quando il lato remoto riporta il suo spazio libero, e lo stesso vale per un indirizzo `sftp:` digitato a mano. I repository S3, B2 e REST non hanno un valore: la loro destinazione dice **Dimensione sconosciuta**, e la scheda **Anomalie** li elenca come non misurati.

## Un assistente IA non riesce a connettersi {#an-ai-assistant-cannot-connect}

La [pagina del server MCP](mcp.md#troubleshooting) elenca cosa significa ogni codice di stato e ogni rifiuto dell'endpoint MCP e cosa fare.

## Il container si riavvia di continuo o sembra non sano {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault riporta sano o non sano dal proprio `/api/health`. Uno strumento di auto-riparazione (come Autoheal) può riavviarlo automaticamente se il motore dovesse mai bloccarsi. Controlla il log del container e la scheda **Integrazione host** per la causa di fondo.

## Una destinazione rifiuta di salvarsi {#location-refused}

BombVault controlla una destinazione prima di salvarla, e il rifiuto dice il perché in una frase. I casi più comuni:

- **"This location lies inside another repository or target, or contains one."** Un repository dentro un repository romperebbe entrambi. Il messaggio nomina l'altro posto. Scegli un'altra cartella, oppure rimuovi prima quella.
- **È un repository che hai configurato.** La destinazione è una delle righe sotto **Repository**, oppure sta dentro o attorno a una. Scegli un altro posto, oppure rimuovi prima quel repository.
- **Lo stesso posto della cartella propria di una sezione.** Una copia sulla cartella propria di una sezione copierebbe i suoi backup su sé stessi a ogni esecuzione, quindi viene rifiutata. Scegli un altro posto, oppure vedi [La cartella propria di una sezione](offsite-recovery.md#own-folder).
- **"That is not an off-site target of this domain"** veniva da un collegamento a una destinazione che era stata rimossa. Una versione attuale li cancella all'avvio e in un'importazione. Se lo vedi ancora, aggiorna BombVault.
- **Un percorso completo dell'host** come `/mnt/user/backups`. BombVault gira in un container, quindi vuole il percorso sotto Host Data, `user/backups`, e il messaggio nomina quello da inserire.

## Una destinazione remota dice che il suo repository non c'è più {#repository-gone}

Una destinazione che conteneva backup e all'improvviso non contiene alcun repository non ne riceve da sola uno nuovo e vuoto, perché quello nasconderebbe il vero appena il disco torna. Un rest-server il cui disco non si è montato è la causa abituale, quindi monta di nuovo il disco ed esegui ancora il backup o la copia. Solo se hai svuotato o eliminato di proposito il vecchio repository, premi **Crea qui un nuovo repository** nella pagina della destinazione. Ricomincia per tutte le sezioni di quella destinazione insieme.

## Un accesso al cloud è scaduto {#cloud-sign-in-expired}

La prova o la copia successiva dice allora che l'accesso è scaduto. **Cambia connessione** nella pagina della destinazione fa accedere di nuovo.

- **Box, Jottacloud e altri con token di refresh monouso:** BombVault conserva il token che rclone ha rinnovato, anche dopo i riavvii e nel kit di ripristino. Dopo un ripristino da un backup delle impostazioni più vecchio, il token al suo interno può essere scaduto, quindi accedi ancora una volta. Il repository resta com'è.
- **iCloud Drive:** Apple termina l'accesso dopo 30 giorni. Richiede un nuovo trust token e cookie da `rclone config`.
- **OneDrive:** un accesso con più unità chiede quale usare, quindi una destinazione lasciata senza scelta resta inutilizzabile finché non scegli. Per OneDrive Business e Google Drive vedi la nota sui dati potati in [Destinazioni](offsite-recovery.md#destinations).

## "The off-site copy of the last backup is still running" {#copy-still-running}

Il backup precedente è finito e la sua copia è ancora in viaggio, il che blocca un nuovo backup di quella sezione. Aspetta che l'intestazione di stato non dica più **Copia off-site in corso**. I pulsanti che avviano un backup dicono lo stesso mentre aspettano. La prima copia verso una nuova destinazione può durare ore, vedi l'[esempio S3](offsite-recovery.md#example-s3).

## Una replica ZFS si ferma perché la destinazione ha snapshot più recenti {#replica-target-newer}

La destinazione contiene snapshot più recenti del più recente che condivide con questo server, per esempio perché il dataset qui è stato riportato indietro o la copia sulla destinazione è stata modificata. La scheda Replica e la cronologia delle esecuzioni nominano lo snapshot sotto **Riporta la destinazione a**. Esegui `zfs rollback -r` su quello snapshot sulla destinazione, per ogni dataset che l'esecuzione nomina, e premi **Replica ora**. Vedi [Per quanto tempo restano gli snapshot](zfs-datasets.md#replica-retention).

## Una VM con snapshot Unraid fallisce e nomina un file {#vm-chain}

BombVault segue la catena di backing di ogni disco e salva ogni file al suo interno. Una catena che non si può seguire fino in fondo fa fallire il backup invece di lasciare fuori parte del disco, e il messaggio nomina il file. Controlla che il file esista sotto `/mnt` e che lo snapshot sia ancora elencato nel VM Manager. Un file fuori da Host Data non si può leggere affatto. Vedi [VM con snapshot Unraid](vm-backup-ssh-setup.md#unraid-snapshots).

## Le copie off-site attendono la tua decisione dopo una ricostruzione {#copies-paused}

Dopo **Scopri backup** o la Procedura guidata di emergenza senza impostazioni ripristinate, o quando una sezione trova backup propri di prima di questo database, le copie off-site di quella sezione aspettano, perché BombVault non sa più quali elementi hai lasciato fuori. **Richiede attenzione** e la pagina della destinazione lo chiedono. Premi **Adottali per questo server** per proseguire, o **Prima controlla le regole** per guardare le regole della sezione prima di farlo. Importare un file di impostazioni non prende la decisione al posto tuo. Vedi [Dopo una ricostruzione](offsite-recovery.md#after-a-rebuild).

## Le notifiche non arrivano {#notifications-do-not-arrive}

Comincia con **Invia test** in **Impostazioni, Notifiche**. La riga sotto la scheda dice cosa è successo.

- **Non viene inviato proprio nulla.** Controlla **Notifica**. Con **Mai** non esce nessun messaggio, solo i ping di un URL di Healthchecks, e con **Solo in caso di errore** un'esecuzione riuscita resta silenziosa. Con **Riepiloga le esecuzioni pianificate** attivo, un'esecuzione pianificata invia un riepilogo alla fine.
- **Un nuovo indirizzo.** Dopo che cambi il server di Matrix, Gotify o email, il campo chiede di nuovo il token o la password, e il nuovo indirizzo viene salvato solo quando li digiti. BombVault non invia il vecchio segreto a un posto nuovo.
- **Gotify** dice che il server ha rifiutato il token dell'applicazione: copialo di nuovo dall'applicazione in Gotify. L'**URL del server** è l'indirizzo di base, senza `/message`.
- **Webhook:** controlla che il **Formato del Webhook** corrisponda al servizio. Un indirizzo Discord richiede **Discord**, e ntfy richiede **ntfy**.
- Le **Notifiche Unraid** passano dalla connessione SSH dell'host, quindi provala prima in **Impostazioni, Integrazioni**. Su un'altra piattaforma restano disattivate, e la scheda lo dice.
- **Una mail di notifica non arriva mai.** Controlla **Crittografia** rispetto alla porta, di solito **STARTTLS** sulla 587 o **TLS (implicit)** sulla 465.

I ping di Healthchecks non dipendono da nulla di tutto questo, vedi [Healthchecks](notifications.md#healthchecks).

## L'app Android non riesce ad aprire un server {#android-cannot-open}

La schermata iniziale dice il perché sulla scheda del server:

- **Non collegato:** né l'indirizzo del server né il relay rispondono. Con **Nessun relay**, l'app raggiunge un server solo al suo indirizzo.
- **Verifica certificato:** il server mostra un certificato diverso da quello di cui ti sei fidato. L'app lo apre solo dopo che ti fidi del nuovo, quindi assicurati che il cambio sia tuo.
- **Accesso necessario:** un server aggiunto con il suo indirizzo chiede la password. Vedi anche [Un telefono finisce nella pagina di accesso](#a-phone-lands-on-the-login-page).
- **La chiave è cambiata:** la schermata iniziale mostra l'impronta della nuova chiave, e la pagina si apre solo dopo che ti fidi. Vedi [Fuori casa](android.md#away).

## Un altro utente non riesce a leggere il repository locale {#repository-permissions}

Il repository locale resta leggibile per altri utenti, come un utente di sincronizzazione via SMB. Prima della 10.4.1, i file scritti da una potatura, un tag, un prelievo, uno spostamento di sezione, un'eliminazione o un backup di stack potevano restare illeggibili fino alla notte successiva. Aggiorna BombVault, e sono leggibili subito. Il repository dell'auto-backup resta leggibile solo da root, di proposito, perché contiene le impostazioni di BombVault.

## Ancora bloccato? {#still-stuck}

- Leggi le pagine complete [Configurazione](configuration.md) e [Off-site e ripristino](offsite-recovery.md).
- **Segnala bug** nella scheda **Aiuto** in **Impostazioni, Info** scarica uno ZIP per una issue su GitHub: la verifica dell'host, la tua configurazione con ogni segreto rimosso, le esecuzioni recenti, cosa è pianificato dopo e il log recente dall'ultimo avvio del container. Contiene anche l'ultimo dump di ogni database, gli elementi ZFS con i loro mount, le anomalie aperte e quante chiavi MCP esistono. Password, token e la configurazione di rclone vengono rimossi, e il pacchetto lo dice nel proprio manifest. Richiede una password di accesso. Per un crash che ha riavviato il container, guarda `docker logs`.
- Chiedi nel [thread di supporto Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Apri una [issue su GitHub](https://github.com/junkerderprovinz/bombvault/issues).
