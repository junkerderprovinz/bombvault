# Risoluzione dei problemi

Una breve FAQ. Per la tabella completa di risoluzione dei problemi lato host del backup VM via SSH (permission-denied, verifica della chiave host, variabili del template mancanti e altro), vedi la [guida al backup delle VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) su GitHub.

## Qualcosa non è collegato correttamente

Aggiungi alla panoramica la scheda **Integrazione host** in **Personalizza**. La verifica dell'integrazione host sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti, e **Controlla ora** la esegue di nuovo. Inizia da qui prima di dare per scontato un bug: un mount mancante o un host irraggiungibile si presenta immediatamente.

## Non riesco a raggiungere l'interfaccia web

BombVault serve HTTPS pronto all'uso sulla porta `3443` (certificato autofirmato), quindi apri `https://<your-unraid-ip>:3443`. Accetta l'avviso del certificato autofirmato, oppure metti BombVault dietro un reverse proxy con il tuo certificato. Se esegui con `HTTP_ONLY=true`, serve invece HTTP in chiaro sulla porta `3000` (pensato per l'uso dietro un proxy che termina il TLS).

## Ho perso la mia APP_KEY

`APP_KEY` deriva la password del repository restic. Senza di essa, senza il kit di ripristino e senza un deposito della chiave nel tuo gruppo, i backup cifrati non possono essere recuperati. Ecco perché la panoramica mostra **Salva il tuo kit di ripristino** finché non l'hai scaricato. Vedi [Off-site e ripristino](offsite-recovery.md#recovery-kit). Genera una chiave con `openssl rand -hex 32` e conservala fuori dal server prima di affidarti a qualsiasi backup.

## Ho dimenticato la password di accesso

Scegli **Password dimenticata?** nella pagina di accesso. BombVault scrive un codice di 12 caratteri nel log del suo container (su Unraid: scheda Docker, icona di BombVault, **Logs**). Digitalo con una nuova password entro 10 minuti. Il ripristino disattiva anche l'autenticazione a due fattori, rimuove tutte le passkey, revoca tutti i token API, le chiavi MCP e gli accessi OAuth, chiude tutte le altre sessioni e disattiva l'associazione dei telefoni. Aggiungi di nuovo le tue passkey in **Impostazioni, Sicurezza**, riattiva lì l'associazione dei telefoni e crea nuovi token e chiavi dove ti servono ancora. Vedi [Modello di sicurezza](configuration.md#login).

## I backup si sono fermati e dicono che la chiave non si può più aprire

Questo server ha rilevato la chiave dei backup di un altro server nella Procedura guidata di emergenza, e `APP_KEY` è stata cambiata dopo. La chiave rilevata è sigillata con la vecchia APP_KEY, quindi BombVault non può aprirla e non esegue alcun backup piuttosto che scrivere con la chiave sbagliata. **Richiede attenzione** nella panoramica la nomina. Rimetti APP_KEY com'era, oppure carica di nuovo il kit di ripristino nella pagina Ripristino sotto **Emergenza**.

## Un telefono finisce nella pagina di accesso

Un telefono associato riceve la sessione tramite il gruppo solo finché **I telefoni possono associarsi con le dodici parole del gruppo** è attivo in **Impostazioni, Sicurezza**. Un ripristino della password lo disattiva. Anche un'app più vecchia del server non riceve una sessione finché non viene aggiornata. Un server aggiunto con il suo indirizzo chiede sempre la password. Un telefono rimosso dalla pagina Istanze torna disattivato quando la sua app si riapre; attiva **Interroga questa istanza** sotto **Monitoraggio** nei suoi dettagli per farlo entrare.

## Il backup delle VM non si connette

Il backup delle VM comunica con libvirt via SSH, mai un mount.

- Conferma che SSH sia abilitato sull'host e che la chiave pubblica di BombVault sia autorizzata in `/root/.ssh/authorized_keys` (Impostazioni, Integrazioni, SSH dell'host mostra la chiave e un pulsante **Prova connessione**).
- Su una rete `br0.x` personalizzata, imposta `LIBVIRT_HOST` sull'IP LAN del tuo Unraid (lì il container non può raggiungere l'host tramite `host.docker.internal`). Abilita **Settings, Docker, Host access to custom networks**.
- Se hai cambiato la porta SSH di Unraid, imposta `LIBVIRT_SSH_PORT` di conseguenza.
- Su TrueNAS, imposta `LIBVIRT_URI` con il socket libvirt proprio di TrueNAS, dai all'app la capability `SETPCAP` e aggiungi la chiave a root in **Credentials, Users** (vedi [TrueNAS SCALE](getting-started.md#truenas)).
- La diagnosi completa passo passo (test di raggiungibilità, routing VLAN, `Permission denied (publickey)`, `Host key verification failed`) è nella [guida al backup delle VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Uno snapshot a caldo di una VM non è stato eseguito

Gli snapshot a caldo necessitano del qemu guest agent installato nella VM e del disco su `/mnt/cache` (o `/mnt/diskX`), non `/mnt/user`. Su una VM spenta, il metodo a caldo ripiega automaticamente su quello ordinato. Un backup ordinato spegne la VM, esegue il backup dei dischi, poi la riavvia, così è sempre consistente. Aspetta fino a dieci minuti che il guest si spenga e non lo spegne mai a forza, così un guest Windows che sta installando aggiornamenti non viene interrotto a metà. Un guest ancora acceso dopo quel tempo viene lasciato stare, e quel backup fallisce indicando il motivo.

## Un backup è fallito con "repository is already locked"

Di solito è un lock restic orfano lasciato quando il container è stato aggiornato o riavviato a metà operazione. BombVault rileva un lock dimostrabilmente orfano, lo rimuove forzatamente e ritenta una volta, automaticamente. Se persiste, apri la scheda **Controlla i backup** in **Impostazioni, Verifica** e scegli **Sblocca** sotto **Altro** nella riga della sezione interessata per rimuovere a mano un lock bloccato. Un problema autentico si presenta comunque invece di essere nascosto. Dopo un riavvio BombVault aspetta che un lock del genere resti dieci minuti senza essere rinnovato. Un restic ancora in esecuzione, per esempio in un secondo BombVault sullo stesso repository, rinnova il suo lock ogni cinque minuti.

## La mia copia off-site non è avvenuta dopo un backup

La replica off-site è best-effort per progettazione, così un intoppo off-site non fa mai fallire il backup locale. Controlla quando copia la destinazione: la sua scheda **Copia** dice **Dopo ogni backup** o indica un piano proprio, e **Impostazioni, Calendario, Copie off-site per sezione** può dare a una sezione un piano suo. Usa **Replica ora** nella scheda **Copia off-site** della sezione in **Impostazioni, Destinazioni** per un'esecuzione su richiesta, e osserva la testata di stato della panoramica, che dice **Copia off-site in corso** mentre gira. Una copia fallita compare sotto **Richiede attenzione**.

## Il certificato di un rest-server non è considerato attendibile

restic rifiuta un certificato che non conosce, e l'errore dice dove metterlo. Copia il certificato autofirmato del rest-server, o quello dell'autorità che lo ha firmato, come file PEM nella cartella `certs` della cartella dati di BombVault (`/config/certs` nel container, `appdata/bombvault/certs` su Unraid) e prova di nuovo. Non serve riavviare.

## Un backup dice che i dati fuori da Host Data non sono stati salvati {#data-outside-host-data}

Il container tiene dati in una cartella che BombVault non può leggere tramite Host Data. Il backup salva la definizione e ogni cartella che raggiunge, colora di arancione la sua riga nel registro attività e nomina le cartelle lasciate fuori. Due casi sono frequenti: un ixVolume su TrueNAS sotto `/mnt/.ix-apps` e un volume Docker con nome sotto `/var/lib/docker/volumes`, come il volume del database di uno stack compose. La scheda **Cartelle da salvare** nella scheda **Impostazioni** del container mostra ognuna di queste cartelle con il suo percorso dentro il container e il suo percorso sull'host, segnata in rosso.

Un volume Docker anonimo non viene segnalato. Docker lo crea da solo per una cartella dichiarata dall'immagine e che nessuno ha mappato, gli dà un nome di 64 cifre esadecimali e lo tiene sotto `/var/lib/docker/volumes`. Di solito contiene una cache, quindi il backup resta verde e la scheda si limita a mostrarlo, con una nota che è anonimo.

- **Vuoi salvare quei dati.** Mappa la cartella su un percorso dell'host sotto Host Data, per esempio una cartella della tua condivisione appdata, e il prossimo backup la include.
- **Non ti servono.** Aggiungi uno dei suoi due percorsi come riga nella scheda **Pattern di esclusione** della stessa scheda: il percorso dentro il container, come `/cache`, o il percorso sull'host che l'avviso nomina, come `/var/lib/docker/volumes/jellyfin_cache/_data`. Dal prossimo backup l'avviso lascia fuori quella cartella, e l'anteprima sotto il campo dice perché. Funziona anche una riga per una cartella superiore, un nome semplice o `/` no.

## Un backup è segnato come Rinviato {#deferred-backups}

L'app del container era occupata, quindi il suo backup aspetta e viene riprovato da solo. La riga nel registro attività dice perché e quando arriva il prossimo tentativo; la scheda **Quando l'app è occupata** nella scheda **Impostazioni** del container dice lo stesso e quando finisce l'attesa. Non serve fare nulla. Per fare il backup subito, usa **Esegui backup ora** quando l'app è libera.

- **Un controllo tuo.** Fai uscire il **Comando pre-backup** con codice 75 mentre l'app è occupata, per esempio `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Qualsiasi altro codice diverso da 0 fa fallire il backup come sempre, quindi uno script rotto compare ancora in **Richiede attenzione**.
- **Plex, Jellyfin o Emby non vengono mai interrogati.** BombVault chiede prima al server all'indirizzo del container e, se lì non risponde, esegue curl o wget dentro il container. Un'immagine che non ha né l'uno né l'altro non si può interrogare tramite sé stessa; in quel caso il log lo dice. Con **Esegui il backup quando il server non risponde** attivo un backup del genere parte comunque, con l'opzione disattivata aspetta.
- **Jellyfin o Emby rifiuta.** Controlla la **Chiave API**. Si crea nella dashboard del server, sotto le chiavi API.
- **Plex rifiuta.** BombVault legge il token da `Preferences.xml` nella cartella montata su `/config`. Un server su cui non si è mai fatto l'accesso non ne ha uno, e risponde solo dove permette l'accesso senza token.
- **Un film in pausa fa aspettare il backup tutta la notte.** Imposta **Ignora le sessioni in pausa dopo (min)**, per esempio a 30.
- **Il controllo del traffico non lascia mai passare il backup.** Alza **Occupato da (KiB/s)** sopra quanto il container invia quando nessuno lo usa. Subito dopo l'avvio di BombVault, il controllo aspetta di aver osservato il container per **Tranquillo per (min)** minuti.
- **Il backup non parte mai di notte.** Con una finestra impostata, i tentativi arrivano solo al suo interno. Un'attesa che raggiunge la scadenza fuori dalla finestra finisce comunque alla scadenza.

## Un backup grande è stato terminato per mancanza di memoria

Imposta **Limite di memoria per backup (MiB, 0 = nessuno)** nella scheda **Prestazioni** in **Impostazioni, Sistema**, ben al di sotto di quanto il container può usare. Vicino al limite restic libera memoria più spesso, il che costa un po' di velocità, così il backup rallenta invece di essere terminato.

## Un ripristino si è interrotto prima di iniziare

Prima che qualcosa venga fermato o rimosso, il ripristino esegue una verifica dei conflitti pre-volo: verifica che l'IP statico del container e le porte host pubblicate siano liberi. Se un altro container ne detiene già uno, si interrompe con un messaggio chiaro e utilizzabile invece di lasciare un ripristino a metà. Libera la porta o l'IP in conflitto, poi riprova.

Se la rete del container non esiste, per esempio perché la sua app TrueNAS è stata eliminata, il controllo prima del ripristino nomina la rete e il ripristino si ferma prima di cambiare qualsiasi cosa. Reinstalla l'app in TrueNAS con lo stesso nome, così la rete viene ricreata, poi ripristina i suoi dati.

## Un'esportazione in chiaro è fallita invece di scrivere un file

Se **Cifra le esportazioni con age** è attivo in **Impostazioni, Sistema** ma non è impostato alcun destinatario valido, un'esportazione fallisce con un errore chiaro invece di scrivere testo in chiaro. Aggiungi un destinatario valido (una chiave pubblica age o una chiave pubblica SSH), oppure disattiva la cifratura se intendi che l'esportazione sia in chiaro. Vedi [Funzionalità](features.md).

## Un dump del database è fallito

Un dump fallito non fa mai fallire il backup che lo circonda: viene registrato come esecuzione fallita a sé, e il motivo dice cosa sistemare.

- **Accesso rifiutato.** Il dump entra con le variabili di password del container stesso (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` o le loro versioni `_FILE`). Controllale sul container del database. Una variabile `_FILE` che punta a un segreto illeggibile per l'utente del container fallisce allo stesso modo.
- **Privilegi mancanti.** Con una password di root casuale il dump può entrare solo come utente dell'applicazione, quindi contiene quel singolo database, e MySQL 8.4 e successivi possono rifiutarlo del tutto. Dai al container una vera password di root, oppure spegni il suo dump.
- **Le tabelle di sistema vanno aggiornate.** MariaDB rifiuta il dump quando le sue tabelle di sistema vengono da una versione più vecchia (errore 1558). Aggiungi la variabile `MARIADB_AUTO_UPGRADE=1` e riavvia il container, oppure esegui `mariadb-upgrade` una volta al suo interno.
- **Nessuno strumento di dump.** Un'immagine snella o costruita in casa senza `pg_dump`, `mysqldump` o `mariadb-dump` non si può dumpare. Usa l'immagine ufficiale, o spegni il dump.
- **Un limite di tempo.** Un dump ha `DB_DUMP_MAX_HOURS` (6 di default), il backup attorno ha `BACKUP_MAX_HOURS`, e un dump che smette di avanzare viene tagliato dopo `BACKUP_STALL_HOURS`. L'ultimo caso nasce quasi sempre da un lock tenuto dall'applicazione. Alza il limite che è scattato, oppure fai il dump mentre l'applicazione è tranquilla.
- **Il container è in pausa o si sta riavviando.** Il dump parla con il server in funzione. Se il container si riavvia di continuo, il suo log dice perché.
- **Un dump danneggiato non si è potuto rimuovere.** Un dump che BombVault non è riuscito a completare viene cancellato. Quando quella cancellazione fallisce, il dump resta nell'elenco segnato come danneggiato e lo puoi eliminare da lì.

## Un import è fallito

Un import ferma il container, sposta di lato la sua cartella dati e lascia che l'immagine ne crei una vuota al suo posto. Se fallisce un passo prima dell'import vero e proprio, la vecchia cartella torna al suo posto da sola. Se fallisce l'import, il container tiene la cartella nuova e quella vecchia resta accanto come `<cartella dati>.bombvault-before-import-<data e ora>`; il messaggio di errore dell'esecuzione indica il percorso esatto.

Una cartella dati che è un dataset a sé, come su TrueNAS, non si può rinominare. L'import allora lascia la cartella al suo posto e ne sposta il contenuto nella cartella conservata accanto, quindi il dataset superiore deve avere spazio per accoglierlo. L'import lo controlla prima di fermare il container.

Per rimetterla a mano: ferma il container, rinomina la cartella dati attuale per toglierla di mezzo, rinomina la cartella conservata al nome originale e avvia il container. Con un dataset, sposta il contenuto invece di rinominare le cartelle. Su Unraid lo fa il gestore file nella scheda Shares.

## Un backup di un dataset ZFS è fallito o ha saltato un dataset {#zfs-datasets}

Ogni problema porta un codice di motivo tra parentesi quadre, e la pagina [Dataset ZFS](zfs-datasets.md#reason-codes) li elenca tutti con la soluzione. I tre più comuni:

- **`snapshot-loop`**: lo snapshot non è arrivato a BombVault perché Host Data non inoltra i nuovi mount. Modifica il container, imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault.
- **`key-not-loaded`**: un dataset cifrato la cui chiave non è caricata viene saltato. Carica la chiave con `zfs load-key` e monta il dataset; il backup successivo lo include.
- **`ssh-auth`**: il server ha rifiutato la chiave di BombVault. **Connessione ZFS a questo server**, che si apre con **Mostra connessione** nella pagina Backup, mostra il comando che la autorizza; eseguilo una volta sul server.

## Un elemento resta su "Sta imparando N/10"

La maggior parte dei controlli di anomalia parte dopo 10 backup riusciti di un elemento, e il conteggio riparte dopo **Segna come attesa** e dopo una modifica della selezione dell'elemento. Un elemento non pianificato non impara, e un container senza appdata non ha nulla da cui imparare, come dice il suo badge. **Avanzamento dell'apprendimento** nella pagina Anomalie mostra un anello di dieci parti per elemento.

## La conservazione non elimina più i vecchi backup di un elemento

Li trattiene un'anomalia critica aperta: la sorgente dell'elemento è quasi vuota, si è ridotta molto, oppure un backup ha salvato di nuovo la maggior parte dei dati. Apri l'anomalia dal badge dell'elemento. Se mancano dati o sono stati cifrati, ripristina prima dall'ultimo backup buono collegato. Poi conferma l'anomalia, o segnala come prevista se il cambiamento è tuo, e l'esecuzione successiva pulisce come sempre. L'anteprima della conservazione indica un elemento del genere come mantenuto. Per un elemento ZFS solo il dataset indicato nell'anomalia conserva i vecchi backup; gli altri dataset dell'albero vengono ripuliti come sempre.

## La pulizia manuale dice che alcuni elementi sono stati mantenuti

La stessa causa: la pulizia lascia stare i vecchi backup di un elemento con un'anomalia di questo tipo e lo nomina nel suo messaggio. Tutto il resto viene pulito come sempre.

## L'importazione della cronologia dice che un repository non si è potuto leggere

Dopo l'aggiornamento BombVault legge una volta le dimensioni dei backup precedenti da ogni repository. Un repository non raggiungibile in quel momento, per esempio una destinazione esterna non disponibile o una condivisione non montata, compare nella scheda **Anomalie** di **Impostazioni, Verifica** e viene ritentato una volta al giorno. Nel frattempo i suoi elementi imparano dai nuovi backup.

## L'avviso sullo spazio su disco non corrisponde alla dashboard di Unraid

Sulla condivisione utente di Unraid (`/mnt/user`) lo spazio libero è quello dell'intero array, non di un singolo disco. I repository remoti vengono misurati solo tramite i remote rclone che riportano il loro spazio libero; i repository S3, B2, REST e SFTP non hanno un dato e compaiono come non misurati nella scheda **Anomalie**.

## Un assistente IA non riesce a collegarsi

La pagina [Server MCP](mcp.md#troubleshooting) spiega cosa significano ogni codice di stato e ogni rifiuto del punto di connessione MCP, e cosa fare.

## Il container continua a riavviarsi o sembra non sano

BombVault segnala sano/non sano dal proprio `/api/health`. Uno strumento di auto-heal (come Autoheal) può riavviarlo automaticamente se il motore dovesse mai incepparsi. Controlla il log del container e la scheda **Integrazione host** per la causa sottostante.

## Ancora bloccato?

- Leggi le pagine complete [Configurazione](configuration.md) e [Off-site e ripristino](offsite-recovery.md).
- Chiedi sul [thread di supporto Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Apri una [issue su GitHub](https://github.com/junkerderprovinz/bombvault/issues).
