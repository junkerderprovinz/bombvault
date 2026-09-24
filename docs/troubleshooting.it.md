# Risoluzione dei problemi

Una breve FAQ. Per la tabella completa di risoluzione dei problemi lato host del backup VM via SSH (permission-denied, verifica della chiave host, variabili del template mancanti e altro), vedi la [guida al backup delle VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) su GitHub.

## Qualcosa non è collegato correttamente

Apri `/spike` nell'interfaccia web. La verifica dell'integrazione host sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti. Inizia da qui prima di dare per scontato un bug: un mount mancante o un host irraggiungibile si presenta immediatamente.

## Non riesco a raggiungere l'interfaccia web

BombVault serve HTTPS pronto all'uso sulla porta `3443` (certificato autofirmato), quindi apri `https://<your-unraid-ip>:3443`. Accetta l'avviso del certificato autofirmato, oppure metti BombVault dietro un reverse proxy con il tuo certificato. Se esegui con `HTTP_ONLY=true`, serve invece HTTP in chiaro sulla porta `3000` (pensato per l'uso dietro un proxy che termina il TLS).

## Ho perso la mia APP_KEY

`APP_KEY` deriva la password del repository restic. Senza di essa (e senza il kit di ripristino della chiave di crittografia), i backup cifrati non possono essere recuperati. Ecco perché la Dashboard ti assilla per scaricare il kit di ripristino. Vedi [Off-site e ripristino](offsite-recovery.md). Genera una chiave con `openssl rand -hex 32` e conservala fuori dal server prima di affidarti a qualsiasi backup.

## Il backup delle VM non si connette

Il backup delle VM comunica con libvirt via SSH, mai un mount.

- Conferma che SSH sia abilitato sull'host e che la chiave pubblica di BombVault sia autorizzata in `/root/.ssh/authorized_keys` (Impostazioni, Sistema, Backup VM via SSH mostra la chiave e un pulsante **Prova connessione**).
- Su una rete `br0.x` personalizzata, imposta `LIBVIRT_HOST` sull'IP LAN del tuo Unraid (lì il container non può raggiungere l'host tramite `host.docker.internal`). Abilita **Impostazioni, Docker, Host access to custom networks**.
- Se hai cambiato la porta SSH di Unraid, imposta `LIBVIRT_SSH_PORT` di conseguenza.
- La diagnosi completa passo passo (test di raggiungibilità, routing VLAN, `Permission denied (publickey)`, `Host key verification failed`) è nella [guida al backup delle VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Uno snapshot a caldo di una VM non è stato eseguito

Gli snapshot a caldo necessitano del qemu guest agent installato nella VM e del disco su `/mnt/cache` (o `/mnt/diskX`), non `/mnt/user`. Su una VM spenta, il metodo a caldo ripiega automaticamente su quello ordinato. Un backup ordinato spegne la VM, esegue il backup dei dischi, poi la riavvia, così è sempre consistente.

## Un backup è fallito con "repository is already locked"

Di solito è un lock restic orfano lasciato quando il container è stato aggiornato o riavviato a metà operazione. BombVault rileva un lock dimostrabilmente orfano, lo rimuove forzatamente e ritenta una volta, automaticamente. Se persiste, usa **Impostazioni, Integrità e manutenzione, Sblocca** per il dominio interessato per rimuovere a mano un lock bloccato. Un problema autentico si presenta comunque invece di essere nascosto.

## La mia copia off-site non è avvenuta dopo un backup

La replica off-site è best-effort per progettazione, così un intoppo off-site non fa mai fallire il backup locale. Controlla il calendario off-site per quel dominio (Impostazioni, Calendari): un calendario vuoto replica dopo ogni backup locale, mentre una cadenza spedisce meno spesso. Usa **Replica ora** nella scheda Off-site per un'esecuzione su richiesta, e osserva l'indicatore di replica sulla Dashboard.

## Un ripristino si è interrotto prima di iniziare

Prima che qualcosa venga fermato o rimosso, il ripristino esegue una verifica dei conflitti pre-volo: verifica che l'IP statico del container e le porte host pubblicate siano liberi. Se un altro container ne detiene già uno, si interrompe con un messaggio chiaro e utilizzabile invece di lasciare un ripristino a metà. Libera la porta o l'IP in conflitto, poi riprova.

## Un'esportazione in chiaro è fallita invece di scrivere un file

Se la cifratura age è attiva (Impostazioni) ma non è impostato alcun destinatario valido, un'esportazione fallisce con un errore chiaro invece di scrivere testo in chiaro. Aggiungi un destinatario valido (una chiave pubblica age o una chiave pubblica SSH), oppure disattiva la cifratura se intendi che l'esportazione sia in chiaro. Vedi [Funzionalità](features.md).

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

Per rimetterla a mano: ferma il container, rinomina la cartella dati attuale per toglierla di mezzo, rinomina la cartella conservata al nome originale e avvia il container. Su Unraid lo fa il gestore file nella scheda Shares.

## Un assistente IA non riesce a collegarsi

La pagina [Server MCP](mcp.md#troubleshooting) spiega cosa significano ogni codice di stato e ogni rifiuto del punto di connessione MCP, e cosa fare.

## Il container continua a riavviarsi o sembra non sano

BombVault segnala sano/non sano dal proprio `/api/health`. Uno strumento di auto-heal (come Autoheal) può riavviarlo automaticamente se il motore dovesse mai incepparsi. Controlla il log del container e il report `/spike` per la causa sottostante.

## Ancora bloccato?

- Leggi le pagine complete [Configurazione](configuration.md) e [Off-site e ripristino](offsite-recovery.md).
- Chiedi sul [thread di supporto Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Apri una [issue su GitHub](https://github.com/junkerderprovinz/bombvault/issues).
