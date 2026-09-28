# Configurazione

Questa pagina copre le variabili d'ambiente del container, i mount forniti dal template, il backup delle VM via SSH e la configurazione off-site. Dove vanno i backup lo imposti dentro l'app, in **Impostazioni, Archiviazione**, non tramite variabili d'ambiente.

## Variabili d'ambiente

| Variabile | Richiesta | Descrizione |
|---|---|---|
| `APP_KEY` | **Sì** | Segreto esadecimale di 32 byte (64 caratteri esadecimali) usato per derivare la password del repo restic. Genera con `openssl rand -hex 32`. Tienilo al sicuro: perderlo rende i backup cifrati irrecuperabili. |
| `LIBVIRT_HOST` | Per le VM | Host Unraid raggiunto via SSH per il backup delle VM (predefinito `host.docker.internal`; il template precompila un placeholder di IP LAN). Usa l'IP LAN del tuo Unraid, richiesto su una rete `br0.x` personalizzata. Usato anche per i backup dei dataset ZFS (campo del template **Host SSH: Address**); il segnaposto `192.168.x.x` conta come non impostato. |
| `LIBVIRT_SSH_PORT` | No | Porta SSH dell'host per il backup delle VM (predefinita `22`). Campo del template **Host SSH: Port**, anche per i dataset ZFS. |
| `LIBVIRT_SSH_USER` | No | Utente SSH sull'host per il backup delle VM (predefinito `root`). Campo del template **Host SSH: User**, anche per i dataset ZFS. |
| `LIBVIRT_URI` | No | URI di connessione libvirt completo, usato **testualmente** al posto di comporne uno dalle tre variabili `LIBVIRT_*` sopra (che a quel punto vengono ignorate per la stringa di connessione). Predefinito non impostato. Necessario su TrueNAS Scale, il cui libvirtd resta in ascolto su un socket non standard che il formato costruito automaticamente non può esprimere: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Vedi la sezione TrueNAS Scale di [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). Se è un URI `qemu+ssh://`, ognuna tra `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` e `LIBVIRT_SSH_PORT` non impostata viene presa da lì, anche per i comandi SSH di BombVault stesso (trasferimento NVRAM, dataset ZFS). |
| `PORT` | No | Porta HTTP (predefinita `3000`; usata solo con `HTTP_ONLY=true`). |
| `HTTPS_PORT` | No | Porta HTTPS (predefinita `3443`; il template la pubblica 1:1, così la WebUI risponde su `https://<ip>:3443`). |
| `HTTP_ONLY` | No | Imposta `true` per disabilitare il listener HTTPS autofirmato e servire solo HTTP in chiaro (per l'uso dietro un reverse proxy che termina il TLS). |
| `BIND_HOST` | No | Indirizzo su cui ascolta la WebUI (predefinito `0.0.0.0`, tutte le interfacce). Lascialo non impostato nel container, le cui porte pubblicate richiedono tutte le interfacce; `127.0.0.1` va bene per un'esecuzione fuori da Docker. L'healthcheck interroga lo stesso indirizzo. |
| `TRUSTED_PROXY` | No | Indirizzi o intervalli CIDR, separati da virgole, del reverse proxy davanti a BombVault (per esempio `192.168.20.11` o `10.0.0.0/8`). Solo da questi hop l'intestazione `X-Forwarded-For` viene creduta, e il freno agli accessi conta allora i fallimenti per client reale invece di mettere tutti quelli dietro il proxy nello stesso secchio. Non impostato (predefinito) significa non fidarsi di nessuno: un'intestazione creduta senza condizioni lascerebbe a chiunque la scelta del proprio secchio. |
| `HOST_SOURCE_ROOT` | No | Il percorso host montato come **Host Data** (predefinito `/mnt`). BombVault traduce le origini dei bind-mount riportate da Docker in percorsi sotto questo mount. Cambia solo se hai montato una radice host diversa. |
| `DATA_ROOT_SEGMENTS` | No | Nomi di segmenti di percorso separati da virgola che contrassegnano una sorgente di bind-mount come dati di backup (predefinito `appdata`, in linea con la convenzione di Unraid `/mnt/user/appdata/<container>`). Il bind-mount di un container viene selezionato automaticamente per il backup quando QUALSIASI segmento elencato compare come segmento di percorso completo nella sua sorgente host: per esempio `DATA_ROOT_SEGMENTS=appdata,config` include anche un bind `.../config`. Vedi [Rilevamento della sorgente di backup](#backup-source-detection) per gli altri modi, sempre attivi, con cui viene trovata la cartella dati di un container. |
| `PLATFORM` | No | Forza la piattaforma su cui BombVault si considera in esecuzione, invece di rilevarla automaticamente: `unraid`, `generic` o `truenas` (predefinito non impostato: rileva automaticamente Unraid cercando il suo marcatore `dockerMan` sotto il mount flash, altrimenti `generic`; anche un valore non riconosciuto ricade su `generic`, e viene registrato nei log). Impostala esplicitamente su un host Docker generico o su TrueNAS Scale invece di affidarti alla sonda automatica valida solo per Unraid (il compose file generico lo fa già). Cambia la convenzione di fallback di appdata, i valori predefiniti della destinazione di ripristino tra istanze diverse, e se vengono anche solo tentati i passaggi di notifica e del plugin companion disponibili solo su Unraid (vedi `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | No | Il nome del container BombVault stesso, così non esegue mai il backup (e quindi non ferma mai) se stesso. |
| `BACKUP_MAX_HOURS` | No | Numero massimo di ore reali per cui una singola esecuzione di backup può tenere il lock del suo dominio prima di essere forzatamente annullata (una salvaguardia così che un'esecuzione bloccata non possa bloccare il dominio per sempre). Vuoto (il predefinito) usa `48`. Aumentalo per backup cloud molto grandi o lenti (un'esecuzione annullata al limite fallisce con `context deadline exceeded`). Imposta `0` per disabilitare del tutto il limite. |
| `BACKUP_STALL_HOURS` | No | Ore in cui un backup può non fare **alcun progresso** prima di essere annullato. Vuoto (il valore predefinito) usa `2`; `0` non annulla mai per stallo. È la più fine delle due salvaguardie e di solito quella che scatta: guarda se succede ancora qualcosa invece di quanto dura l'esecuzione, per cui un backup di più terabyte lento ma sano viene lasciato stare, mentre uno bloccato su una condivisione che non risponde viene fermato in ore invece che in giorni. Dopo 30 minuti di silenzio viene registrato un avviso, prima di annullare qualcosa. La scansione conta come progresso: restic non scrive byte mentre percorre un albero grande, e quella fase viene sorvegliata tramite i suoi totali di file e byte anziché tramite i byte scritti. Le due variabili sono indipendenti, e `BACKUP_MAX_HOURS` limita ancora le fasi dopo il backup vero e proprio (conservazione, statistiche, copia off-site), dove non ci sono contatori da sorvegliare. |
| `DB_DUMP_MAX_HOURS` | No | Ore per cui un dump automatico di database può girare prima di essere fermato. Vuoto (il valore predefinito) usa `6`; sono ammessi valori da `1` a `48`, e il limite resta un'ora sotto `BACKUP_MAX_HOURS` (alla metà, quando questo è inferiore a due ore), così un dump lungo viene tagliato dal proprio limite e segnalato come tale invece di trascinarsi dietro il backup. Un dump che non avanza più viene fermato prima, dopo `BACKUP_STALL_HOURS`. Un dump fermato fallisce per conto suo e il backup del container prosegue. Su Unraid aggiungi la variabile al container BombVault con **Add another Path, Port, Variable**. |
| `TZ` | No | Fuso orario per lo scheduler (per esempio `Europe/Berlin`). **Se non è impostato, tutte le pianificazioni vengono eseguite in UTC**: una pianificazione alle 02:30 parte quindi alle 02:30 UTC e non secondo l'ora locale. Su Unraid non lo imposti mai tu: il sistema passa il proprio fuso orario a ogni container. |

## Mount

Monta il socket Docker, il flash (`/boot`) e la radice **Host Data** (`/mnt`) come mostrato nel template CA. Le *origini* e le *destinazioni* dei backup risiedono entrambe sotto Host Data, ed è montato **slave** così una condivisione remota che si monta dopo l'avvio del container (per esempio sotto `/mnt/remotes`) diventa visibile senza un riavvio.

Anche i backup dei dataset ZFS hanno bisogno di questa modalità: l'host monta lo snapshot di un dataset solo dopo l'avvio del container. Vedi [Dataset ZFS](zfs-datasets.md).

Un'installazione nuova salva ogni dominio nel luogo **Unraid**, in `/mnt/user/bombvault` con una cartella per dominio (`container`, `vms`, `flash`, `config`, `files`, `zfs`), creata al primo backup. Aggiungi altri luoghi, locali o remoti, in **Impostazioni, Archiviazione**; vedi [Luoghi di archiviazione](storage-places.md).

!!! note "Verifica dell'integrazione host"
    Apri `/spike` nell'interfaccia web dopo l'avvio del container. Sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti.

## Rilevamento delle sorgenti di backup {#backup-source-detection}

Per ogni contenitore, BombVault sceglie da sé quali bind mount e volumi con nome salvare. Un percorso viene preso non appena vale uno dei punti seguenti (il risultato si può sempre correggere per singolo contenitore nei suoi **Percorsi di backup**):

- **Corrispondenza di un segmento di radice dati:** l'origine host del bind contiene uno dei segmenti di `DATA_ROOT_SEGMENTS` come componente completa del percorso (per impostazione predefinita solo `appdata`).
- **I volumi Docker con nome** sono sempre inclusi, perché non hanno un equivalente usa e getta e quindi non c'è nulla da filtrare, **ma soltanto quando il percorso di archiviazione reale del volume sull'host è raggiungibile attraverso il mount Host Data**, esattamente come ogni altro percorso host salvato da BombVault. Il driver locale predefinito colloca un volume sotto la radice dati del demone, cioè `/var/lib/docker/volumes/<nome>/_data` salvo personalizzazioni (verificalo con `docker info -f '{{.DockerRootDir}}'`). Quel punto NON rientra nel mount Host Data stretto, a directory singola, che il `docker-compose.yml` generico usa per impostazione predefinita. Un volume irraggiungibile viene saltato in silenzio, non è un errore. Per salvare davvero i volumi con nome su un host generico, punta Host Data (e `HOST_SOURCE_ROOT`) a un antenato comune che copra anche la radice dati di Docker: vedi il commento Host Data nel file compose per il compromesso (Unraid aggira la cosa montando tutto `/mnt`, la sua convenzione universale di primo livello, per lo stesso motivo).
- **Directory di progetto Docker Compose:** se il contenitore porta l'etichetta standard `com.docker.compose.project.working_dir` (impostata automaticamente da `docker compose up`), anche quella directory viene aggiunta, indipendentemente dal fatto che un bind abbia corrisposto a un segmento di radice dati.
- **Forzatura tramite l'etichetta `bombvault.data`:** metti l'etichetta `bombvault.data=true` su un contenitore per includere TUTTI i suoi bind mount, per una disposizione che nessuna delle due convenzioni sopra intercetta (per esempio un unico bind `/srv/plex/config` senza progetto Compose). Qualsiasi valore non vuoto diverso da `false` conta come vero; un'etichetta assente o `bombvault.data=false` non cambia nulla.
- **Etichetta `bombvault.dbdump`:** metti `bombvault.dbdump=false` su un container per spegnere il suo dump automatico del database (`0`, `no` e `off` fanno lo stesso), oppure indica il motore (`postgres`, `mysql`, `mariadb`) per dumpare un container che BombVault non riconosce da solo. L'etichetta ha la meglio sull'interruttore nella scheda del container, che su Unraid è la via abituale.

## Modello di sicurezza

!!! warning "Controllo dell'host equivalente a root"
    Tramite il socket Docker BombVault può fermare, rimuovere e ricreare container e leggere/scrivere appdata, e per il backup delle VM accede all'host via SSH (`qemu+ssh://`, root di default) per eseguire `virsh`. Chiunque possa raggiungere la sua interfaccia web ha di fatto accesso root sull'host.

- **Protezione con password opzionale** (Impostazioni, Sicurezza): imposta una password per richiedere l'accesso, cancellala per disattivarla. Disattivata di default per l'uso su una LAN fidata. La password è memorizzata con Argon2id su un valore pepato con `APP_KEY`, quindi un `/config` copiato non vale nulla senza la chiave ed è lento da attaccare con essa. Una password nuova richiede almeno 12 caratteri; una più corta già presente continua a funzionare finché non viene cambiata. Le sessioni sono firmate (HMAC derivato da `APP_KEY`) e cambiare la password le invalida; gli accessi sono limitati a cinque fallimenti al minuto per client.
- **Autenticazione a due fattori** (Impostazioni): un codice temporale da un'app di autenticazione oltre alla password, più otto codici di recupero monouso consegnati una sola volta all'attivazione. Il segreto condiviso è memorizzato cifrato con `APP_KEY`, e disattivare il fattore richiede un codice attuale.
- Poiché il gate è opt-in, quando non impostato l'intera interfaccia e API (inclusi la configurazione off-site, le route del tamper-test e il kit di ripristino) sono raggiungibili da chiunque possa raggiungere la porta. Abilita il gate una volta che sono in uso backup off-site, immutabili o la cifratura.
- Esegui BombVault solo su una rete affidabile e non esposta. Per l'accesso remoto mettilo dietro un reverse proxy che aggiunga autenticazione e TLS. Le risposte portano header di sicurezza di base (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Dietro un reverse proxy ogni richiesta porta l'indirizzo del proxy, quindi senza `TRUSTED_PROXY` il freno conta tutti i client in un unico secchio e i fallimenti di un attaccante bloccano fuori anche te. Indica il proxy in `TRUSTED_PROXY` per riavere il conteggio per client.
- Un reverse proxy davanti a BombVault deve passare l'intestazione `Authorization` o `X-API-Key` a `/mcp` e non deve bufferizzarne le risposte, altrimenti gli assistenti non riescono a collegarsi. Vedi [Server MCP](mcp.md#tls).
- Con `HTTP_ONLY=true` il cookie di sessione perde il suo flag `Secure` (deve, per funzionare su HTTP in chiaro), quindi abilita la password dietro un proxy che termina il TLS solo se la riservatezza è importante.
- La connessione SSH del backup VM si fida della chiave host al primo collegamento (TOFU) e la fissa in seguito. Verifica la chiave dell'host fuori banda se il tuo percorso container-verso-host non è affidabile.
- I backup vengono cifrati da restic quando la cifratura è abilitata (Impostazioni; attivata di default), con la chiave derivata da `APP_KEY`.

## Server MCP {#mcp-server}

Il server MCP non richiede alcuna variabile d'ambiente. Lo attivi creando una chiave in **Impostazioni, Sistema, Server MCP**, e risponde su `/mcp` sulla stessa porta dell'interfaccia web (per esempio `https://192.168.1.10:3443/mcp`). Senza una chiave attiva quel percorso risponde `404`. Client, certificati e limiti sono descritti in [Server MCP](mcp.md).

## Backup delle VM via SSH

BombVault esegue il backup delle VM KVM/libvirt **senza montare alcun percorso libvirt**. Esegue `virsh` sull'host via SSH (`qemu+ssh://`), così non può mai influire sul VM Manager del tuo host.

Configurazione rapida:

1. **Impostazioni, Sistema, SSH dell'host:** copia la chiave pubblica mostrata.
2. Aggiungila a `/root/.ssh/authorized_keys` di Unraid (anche persistita sul flash così sopravvive ai riavvii).
3. Clicca **Prova connessione**.

Il template aggiunge `--add-host=host.docker.internal:host-gateway` così il container può raggiungere l'host. Imposta `LIBVIRT_HOST` sull'IP LAN del tuo Unraid se quel nome non si risolve (per esempio quando il container gira su una rete `br0.x` personalizzata). Se hai cambiato la porta SSH di Unraid, imposta `LIBVIRT_SSH_PORT` di conseguenza. Gli **snapshot a caldo** necessitano inoltre del qemu guest agent nella VM e del disco su `/mnt/cache` (non `/mnt/user`).

!!! important "Guida completa alla configurazione delle VM e alla rete"
    La guida completa passo passo (abilitazione SSH, autorizzazione persistente della chiave, routing su rete personalizzata e VLAN, metodo per VM e risoluzione dei problemi lato host) si trova in [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) su GitHub.

## Configurazione off-site

Le copie off-site vanno nei luoghi di archiviazione. Aggiungi il luogo in **Impostazioni, Archiviazione** con **Aggiungi luogo**, poi spuntalo in **Copiato su** sulla riga del dominio. [Luoghi di archiviazione](storage-places.md) copre ogni tipo di connessione, e [Off-site e ripristino](offsite-recovery.md) copre append-only, tamper testing ed esercitazioni DR. In breve:

- **Tipi di connessione:** una cartella su questo Unraid o una condivisione NAS montata sotto `/mnt/remotes`, archiviazione S3 (Backblaze B2 e gli altri provider cloud, oppure un servizio autogestito come MinIO o Garage), rest-server, SFTP inclusa una Hetzner Storage Box, WebDAV per Nextcloud, ownCloud e OpenCloud, Azure Blob e qualsiasi remote rclone. Backblaze B2 richiede solo la chiave: BombVault ne ricava il bucket e l'endpoint S3.
- **Le credenziali** vengono memorizzate cifrate insieme al luogo a cui appartengono. I set di credenziali usati dalle sorgenti di prelievo si trovano nella scheda **Prelievo** della pagina Istanze.
- **Le destinazioni SSH non richiedono nulla di installato sull'altro lato.** Un luogo SFTP necessita solo di un server SSH. Aggiungi la chiave pubblica mostrata nel modulo SFTP (anche in **Impostazioni, Sistema, Backup VM via SSH** e in `/config/ssh/id_ed25519.pub`) al file `~/.ssh/authorized_keys` dell'utente di destinazione.
- **Copia off-site:** BombVault copia i nuovi snapshot con `restic copy` su base best-effort, in aggiunta al luogo in cui è salvato un dominio. Ogni dominio ha il proprio calendario di copia in Impostazioni, Calendari, più **Copia ora** sulla sua riga.
- **Più luoghi di copia per dominio:** spunta in **Copiato su** tutti i luoghi che vuoi; ognuno copia secondo il calendario del dominio.
- **Conservazione, limiti, classe di archiviazione e budget di crescita appartengono al luogo** e si impostano nei suoi dettagli. La conservazione di un luogo vale per ogni repository che contiene, così un luogo off-site può tenere le copie più a lungo come archivio; un luogo con tutte le regole a zero non taglia mai.
- **Classe di archiviazione fredda e d'archivio (S3):** per un luogo S3, scegli un livello leggibile in ripristino (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). I remote rclone impostano la loro classe nella configurazione rclone.
- **Un dominio salvato in un luogo remoto:** vedi [Un dominio salvato in un luogo remoto](offsite-recovery.md#remote-primary-repositories).

## Anomalie {#anomalies}

Il rilevamento delle anomalie si imposta nella scheda **Anomalie** di **Impostazioni, Integrità**. Ogni controllo salva appena lo cambi, e i tre sotto l'interruttore sono nascosti finché il rilevamento è spento.

| Impostazione | Predefinito | Cosa fa |
|---|---|---|
| **Rileva le anomalie** | Attivo | Confronta ogni backup con la cronologia propria dell'elemento. Spento, non si controlla più nulla di nuovo e la voce **Anomalie** esce dalla barra laterale; la scheda continua a rimandare ai rilevamenti precedenti. |
| **Sensibilità** | Equilibrata | Rigorosa segnala cambiamenti più piccoli, Permissiva solo quelli grandi. |
| **Invia una notifica per** | Solo riscontri critici | La gravità minima che invia un messaggio tramite i canali configurati in Notifiche. Gli errori ripetuti di backup e dump e i controlli di ripristino pianificati falliti inviano già un proprio messaggio e non vengono inviati due volte. |
| **Conserva i backup vecchi quando una sorgente si riduce molto o viene riscritta** | Attivo | Finché un elemento ha un rilevamento aperto per una sorgente quasi vuota, una forte riduzione o la maggior parte dei dati salvata di nuovo, la conservazione e la pulizia lasciano stare i suoi vecchi backup. Conferma il rilevamento o segnalo come previsto per liberarli. |

Ogni elemento può avere una propria sensibilità e un proprio minimo di notifica. Impostali nella scheda **Elementi** della pagina **Anomalie**, oppure nel pannello dell'elemento stesso: la sezione cartelle di un container e le impostazioni di una VM (entrambe in modalità avanzata), l'editor delle cartelle di un set di cartelle e le pagine **Flash** e **Auto-backup**. Per un elemento ZFS si trovano nel suo editor nella pagina **ZFS** e valgono per ogni dataset del suo albero.

## Impostazioni portatili (esporta e importa) {#portable-settings-export-and-import}

La scheda **Esporta e importa impostazioni** nella pagina Impostazioni scrive l'intera configurazione BombVault (impostazioni di dominio, luoghi di archiviazione, calendari, notifiche) in un file JSON portatile che puoi importare su un'altra istanza, così passare a una nuova macchina o clonare una configurazione non significa reinserire tutto a mano. L'importazione mostra un'anteprima e chiede conferma, e non tocca mai i tuoi dati di backup o la cronologia. L'anteprima conta i luoghi di archiviazione nel file e, con le credenziali, i set di credenziali; per un file più vecchio senza luoghi, BombVault li ricava dalle impostazioni importate.

!!! warning "L'esportazione può contenere credenziali"
    Scegli tu se includere nel file le credenziali dei tuoi luoghi e delle notifiche. Con le credenziali incluse, l'esportazione è sensibile quanto il tuo kit di ripristino, quindi conservala in un luogo sicuro. Senza di esse, il file contiene solo impostazioni non segrete.
