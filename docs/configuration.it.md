# Configurazione

Questa pagina copre le variabili d'ambiente del container, i mount forniti dal template, il backup delle VM via SSH e la configurazione off-site. I **percorsi dei repository** di backup si configurano dentro l'app (Impostazioni, Percorsi di backup), non tramite variabili d'ambiente.

## Variabili d'ambiente

| Variabile | Richiesta | Descrizione |
|---|---|---|
| `APP_KEY` | **Sì** | Segreto esadecimale di 32 byte (64 caratteri esadecimali) usato per derivare la password del repo restic. Genera con `openssl rand -hex 32`. Tienilo al sicuro: perderlo rende i backup cifrati irrecuperabili. |
| `LIBVIRT_HOST` | Per le VM | Host Unraid raggiunto via SSH per il backup delle VM (predefinito `host.docker.internal`; il template precompila un placeholder di IP LAN). Usa l'IP LAN del tuo Unraid, richiesto su una rete `br0.x` personalizzata. |
| `LIBVIRT_SSH_PORT` | No | Porta SSH dell'host per il backup delle VM (predefinita `22`). |
| `LIBVIRT_SSH_USER` | No | Utente SSH sull'host per il backup delle VM (predefinito `root`). |
| `LIBVIRT_URI` | No | URI di connessione libvirt completo, usato **testualmente** al posto di comporne uno dalle tre variabili `LIBVIRT_*` sopra (che a quel punto vengono ignorate per la stringa di connessione). Predefinito non impostato. Necessario su TrueNAS Scale, il cui libvirtd resta in ascolto su un socket non standard che il formato costruito automaticamente non può esprimere: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Vedi la sezione TrueNAS Scale di [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | No | Porta HTTP (predefinita `3000`; usata solo con `HTTP_ONLY=true`). |
| `HTTPS_PORT` | No | Porta HTTPS (predefinita `3443`; il template la pubblica 1:1, così la WebUI risponde su `https://<ip>:3443`). |
| `HTTP_ONLY` | No | Imposta `true` per disabilitare il listener HTTPS autofirmato e servire solo HTTP in chiaro (per l'uso dietro un reverse proxy che termina il TLS). |
| `HOST_SOURCE_ROOT` | No | Il percorso host montato come **Host Data** (predefinito `/mnt`). BombVault traduce le origini dei bind-mount riportate da Docker in percorsi sotto questo mount. Cambia solo se hai montato una radice host diversa. |
| `DATA_ROOT_SEGMENTS` | No | Nomi di segmenti di percorso separati da virgola che contrassegnano una sorgente di bind-mount come dati di backup (predefinito `appdata`, in linea con la convenzione di Unraid `/mnt/user/appdata/<container>`). Il bind-mount di un container viene selezionato automaticamente per il backup quando QUALSIASI segmento elencato compare come segmento di percorso completo nella sua sorgente host: per esempio `DATA_ROOT_SEGMENTS=appdata,config` include anche un bind `.../config`. Vedi [Rilevamento della sorgente di backup](#backup-source-detection) per gli altri modi, sempre attivi, con cui viene trovata la cartella dati di un container. |
| `PLATFORM` | No | Forza la piattaforma su cui BombVault si considera in esecuzione, invece di rilevarla automaticamente: `unraid`, `generic` o `truenas` (predefinito non impostato: rileva automaticamente Unraid cercando il suo marcatore `dockerMan` sotto il mount flash, altrimenti `generic`; anche un valore non riconosciuto ricade su `generic`, e viene registrato nei log). Impostala esplicitamente su un host Docker generico o su TrueNAS Scale invece di affidarti alla sonda automatica valida solo per Unraid (il compose file generico lo fa già). Cambia la convenzione di fallback di appdata, i valori predefiniti della destinazione di ripristino tra istanze diverse, e se vengono anche solo tentati i passaggi di notifica e del plugin companion disponibili solo su Unraid (vedi `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | No | Il nome del container BombVault stesso, così non esegue mai il backup (e quindi non ferma mai) se stesso (predefinito `BombVault`; rilevato automaticamente tramite l'hostname su rete bridge). |
| `BACKUP_MAX_HOURS` | No | Numero massimo di ore reali per cui una singola esecuzione di backup può tenere il lock del suo dominio prima di essere forzatamente annullata (una salvaguardia così che un'esecuzione bloccata non possa bloccare il dominio per sempre). Vuoto (il predefinito) usa `48`. Aumentalo per backup cloud molto grandi o lenti (un'esecuzione annullata al limite fallisce con `context deadline exceeded`). Imposta `0` per disabilitare del tutto il limite. |
| `TZ` | No | Fuso orario per lo scheduler (per esempio `Europe/Berlin`). **Se non è impostato, tutte le pianificazioni vengono eseguite in UTC**: una pianificazione alle 02:30 parte quindi alle 02:30 UTC e non secondo l'ora locale. Su Unraid non lo imposti mai tu: il sistema passa il proprio fuso orario a ogni container. |

## Mount

Monta il socket Docker, il flash (`/boot`) e la radice **Host Data** (`/mnt`) come mostrato nel template CA. Le *origini* e le *destinazioni* dei backup risiedono entrambe sotto Host Data, ed è montato **rslave** così una condivisione remota che si monta dopo l'avvio del container (per esempio sotto `/mnt/remotes`) diventa visibile senza un riavvio.

I percorsi dei repository di backup hanno come predefinito `/mnt/user/bombvault/{container,vms,flash,config,files}`, creati al primo backup. Cambia la posizione in qualsiasi momento in **Impostazioni, Percorsi di backup**.

!!! note "Verifica dell'integrazione host"
    Apri `/spike` nell'interfaccia web dopo l'avvio del container. Sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti.

## Rilevamento delle sorgenti di backup {#backup-source-detection}

Per ogni contenitore, BombVault sceglie da sé quali bind mount e volumi con nome salvare. Un percorso viene preso non appena vale uno dei punti seguenti (il risultato si può sempre correggere per singolo contenitore nei suoi **Percorsi di backup**):

- **Corrispondenza di un segmento di radice dati:** l'origine host del bind contiene uno dei segmenti di `DATA_ROOT_SEGMENTS` come componente completa del percorso (per impostazione predefinita solo `appdata`).
- **I volumi Docker con nome** sono sempre inclusi, perché non hanno un equivalente usa e getta e quindi non c'è nulla da filtrare, **ma soltanto quando il percorso di archiviazione reale del volume sull'host è raggiungibile attraverso il mount Host Data**, esattamente come ogni altro percorso host salvato da BombVault. Il driver locale predefinito colloca un volume sotto la radice dati del demone, cioè `/var/lib/docker/volumes/<nome>/_data` salvo personalizzazioni (verificalo con `docker info -f '{{.DockerRootDir}}'`). Quel punto NON rientra nel mount Host Data stretto, a directory singola, che il `docker-compose.yml` generico usa per impostazione predefinita. Un volume irraggiungibile viene saltato in silenzio, non è un errore. Per salvare davvero i volumi con nome su un host generico, punta Host Data (e `HOST_SOURCE_ROOT`) a un antenato comune che copra anche la radice dati di Docker: vedi il commento Host Data nel file compose per il compromesso (Unraid aggira la cosa montando tutto `/mnt`, la sua convenzione universale di primo livello, per lo stesso motivo).
- **Directory di progetto Docker Compose:** se il contenitore porta l'etichetta standard `com.docker.compose.project.working_dir` (impostata automaticamente da `docker compose up`), anche quella directory viene aggiunta, indipendentemente dal fatto che un bind abbia corrisposto a un segmento di radice dati.
- **Forzatura tramite l'etichetta `bombvault.data`:** metti l'etichetta `bombvault.data=true` su un contenitore per includere TUTTI i suoi bind mount, per una disposizione che nessuna delle due convenzioni sopra intercetta (per esempio un unico bind `/srv/plex/config` senza progetto Compose). Qualsiasi valore non vuoto diverso da `false` conta come vero; un'etichetta assente o `bombvault.data=false` non cambia nulla.

## Modello di sicurezza

!!! warning "Controllo dell'host equivalente a root"
    Tramite il socket Docker BombVault può fermare, rimuovere e ricreare container e leggere/scrivere appdata, e per il backup delle VM accede all'host via SSH (`qemu+ssh://`, root di default) per eseguire `virsh`. Chiunque possa raggiungere la sua interfaccia web ha di fatto accesso root sull'host.

- **Protezione con password opzionale** (Impostazioni, Sicurezza): imposta una password per richiedere il login, cancellala per disabilitare. Disattivata di default per l'uso su LAN affidabile. Le sessioni sono firmate (HMAC derivato da `APP_KEY`) e cambiare la password le invalida; i login sono a velocità limitata.
- Poiché il gate è opt-in, quando non impostato l'intera interfaccia e API (inclusi la configurazione off-site, le route del tamper-test e il kit di ripristino) sono raggiungibili da chiunque possa raggiungere la porta. Abilita il gate una volta che sono in uso backup off-site, immutabili o la cifratura.
- Esegui BombVault solo su una rete affidabile e non esposta. Per l'accesso remoto mettilo dietro un reverse proxy che aggiunga autenticazione e TLS. Le risposte portano header di sicurezza di base (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Con `HTTP_ONLY=true` il cookie di sessione perde il suo flag `Secure` (deve, per funzionare su HTTP in chiaro), quindi abilita la password dietro un proxy che termina il TLS solo se la riservatezza è importante.
- La connessione SSH del backup VM si fida della chiave host al primo collegamento (TOFU) e la fissa in seguito. Verifica la chiave dell'host fuori banda se il tuo percorso container-verso-host non è affidabile.
- I backup vengono cifrati da restic quando la cifratura è abilitata (Impostazioni; attivata di default), con la chiave derivata da `APP_KEY`.

## Backup delle VM via SSH

BombVault esegue il backup delle VM KVM/libvirt **senza montare alcun percorso libvirt**. Esegue `virsh` sull'host via SSH (`qemu+ssh://`), così non può mai influire sul VM Manager del tuo host.

Configurazione rapida:

1. **Impostazioni, Sistema, Backup VM via SSH:** copia la chiave pubblica mostrata.
2. Aggiungila a `/root/.ssh/authorized_keys` di Unraid (anche persistita sul flash così sopravvive ai riavvii).
3. Clicca **Prova connessione**.

Il template aggiunge `--add-host=host.docker.internal:host-gateway` così il container può raggiungere l'host. Imposta `LIBVIRT_HOST` sull'IP LAN del tuo Unraid se quel nome non si risolve (per esempio quando il container gira su una rete `br0.x` personalizzata). Se hai cambiato la porta SSH di Unraid, imposta `LIBVIRT_SSH_PORT` di conseguenza. Gli **snapshot a caldo** necessitano inoltre del qemu guest agent nella VM e del disco su `/mnt/cache` (non `/mnt/user`).

!!! important "Guida completa alla configurazione delle VM e alla rete"
    La guida completa passo passo (abilitazione SSH, autorizzazione persistente della chiave, routing su rete personalizzata e VLAN, metodo per VM e risoluzione dei problemi lato host) si trova in [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) su GitHub.

## Configurazione off-site

Configura una replica off-site nella scheda **Impostazioni, Off-site**. Vedi [Off-site e ripristino](offsite-recovery.md) per il flusso di lavoro completo (immutabile/append-only, tamper testing ed esercitazioni DR). In breve:

- **Backend:** SMB/CIFS e NFS (monta la condivisione e puntaci un Percorso di backup), backend restic nativi senza rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), o qualsiasi remote rclone (`rclone:<remote>:<bucket>/path`).
- **Le credenziali cloud** vengono memorizzate cifrate sotto Impostazioni, Off-site, Credenziali cloud.
- **Le destinazioni SSH non richiedono nulla di installato sull'altro lato.** `sftp:` necessita solo di un server SSH. Aggiungi la chiave pubblica da **Impostazioni, Sistema, Backup VM via SSH** (anche in `/config/ssh/id_ed25519.pub`) al file `~/.ssh/authorized_keys` dell'utente di destinazione.
- **Copia off-site:** BombVault replica i nuovi snapshot con `restic copy` su base best-effort. Il repo locale resta primario. Ogni dominio ha il proprio calendario off-site, più un pulsante **Replica ora**.
- **Più destinazioni off-site per dominio:** ogni dominio può replicare verso più destinazioni off-site contemporaneamente. Aggiungi destinazioni extra in Impostazioni, Off-site, ciascuna con il proprio repository, classe di archiviazione S3, flag append-only, conservazione e budget di crescita; replicano tutte secondo il calendario off-site di quel dominio. Una configurazione off-site singola esistente viene riportata come prima destinazione.
- **Conservazione per sorgente:** la policy locale risiede su Impostazioni, Percorsi e Archiviazione; la policy off-site su Impostazioni, Off-site (lasciala tutta a zero per non tagliare mai automaticamente gli snapshot off-site).
- **Limiti di banda:** limita la velocità di upload/download di restic sotto Impostazioni, Off-site.
- **Classe di archiviazione fredda e d'archivio (S3):** per un repo off-site S3 nativo, scegli un livello leggibile in ripristino (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). I remote rclone impostano la loro classe nella configurazione rclone.

## Impostazioni portatili (esporta e importa) {#portable-settings-export-and-import}

La scheda **Esporta e importa impostazioni** nella pagina Impostazioni scrive l'intera configurazione BombVault (impostazioni di dominio, destinazioni off-site, calendari, conservazione, notifiche) in un file JSON portatile che puoi importare su un'altra istanza, così passare a una nuova macchina o clonare una configurazione non significa reinserire tutto a mano. L'importazione mostra un'anteprima e chiede conferma, e non tocca mai i tuoi dati di backup o la cronologia.

!!! warning "L'esportazione può contenere credenziali"
    Scegli tu se includere le credenziali off-site e di notifica nel file. Con le credenziali incluse, l'esportazione è sensibile quanto il tuo kit di ripristino, quindi conservala in un luogo sicuro. Senza di esse, il file contiene solo impostazioni non segrete.
