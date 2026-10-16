# Primi passi

Questa pagina ti accompagna da una macchina Unraid appena installata al tuo primo backup.

## Requisiti

| Requisito | Note |
|---|---|
| **Unraid 6.12+** | Le versioni precedenti non sono testate. Unraid è l'obiettivo principale, ma BombVault gira anche su un semplice host Docker e su TrueNAS Scale (vedi [Host Docker generico](#generic-docker-host) e [TrueNAS SCALE](#truenas)). |
| **Posizione del repo restic** | Un percorso locale (consigliato: il tuo array o la cache), SMB, NFS o qualsiasi backend rclone. |
| **Socket Docker** | Montato automaticamente dal template (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montato per intero automaticamente dal template (`/boot` in `/host/boot`). Alimenta il backup del flash e permette a un container ripristinato di riapparire come una normale app Unraid, modificabile. |
| **VM KVM** (opzionale) | Il backup delle VM comunica con libvirt via SSH, nessun mount libvirt. Configuralo in **Impostazioni, Integrazioni, SSH dell'host** (vedi [Configurazione](configuration.md)). |
| **Dataset ZFS** (opzionale) | Lo stesso collegamento SSH del backup delle VM, `zfs` sull'host e Host Data mappato come `/mnt` con modalità di accesso Read/Write - Slave, il valore predefinito del template. Vedi [Dataset ZFS](zfs-datasets.md). |
| **App Android** (facoltativa) | Android 10 o successivo, associata a server con versione 9.7.0 o successiva. Vedi [App Android](android.md). |

## Installazione su Unraid

Il percorso più semplice è **Community Applications**.

1. Apri la scheda **Apps** in Unraid.
2. Cerca **BombVault**.
3. Clicca **Install**, imposta le variabili richieste (sotto) e applica.

!!! tip "Installazione manuale del template"
    Se preferisci aggiungere il template a mano:

    1. Vai su **Docker, Add Container, Template repositories** e aggiungi:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Cerca **BombVault** in Templates.
    3. Imposta le variabili richieste e clicca **Apply**.

## Host Docker generico {#generic-docker-host}

Non sei su Unraid? BombVault gira anche come semplice contenitore su qualsiasi host Docker. TrueNAS SCALE ha i suoi passaggi in [TrueNAS SCALE](#truenas).

1. Prendi dal repository il file [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), pronto da modificare.
2. Imposta `APP_KEY` (vedi sotto) e punta il volume Host Data alla tua vera radice dati. I commenti nel file spiegano entrambe le cose.
3. `docker compose up -d`, poi apri `https://<ip-host>:3443/`.

Cosa cambia rispetto a Unraid:

- **Nessuna sezione Flash.** Non c'è una chiavetta di avvio da catturare o ripristinare, quindi la sezione Flash qui non ha nulla da fare. Al suo posto, quando Host Data raggiunge la `/etc` dell'host, **Aggiungi** nella pagina Backup propone **Configurazione di sistema** come suggerimento di cartelle: un insieme iniziale di cartelle `/etc` che rivedi e modifichi prima di salvare.
- **Nessuna notifica nativa di Unraid.** I canali di notifica propri di BombVault (webhook, avvisi di fallimento off-site e così via) funzionano normalmente; viene saltato soltanto l'inoltro specifico al sistema di notifiche di Unraid, dato che qui non esiste.
- **Il backup delle macchine virtuali è opzionale e richiede un host libvirtd separato raggiungibile via SSH.** Vedi il blocco commentato nel file compose. Un host Docker generico non porta con sé alcun gestore di macchine virtuali.
- **Nessun widget per la dashboard.** BombVault Widget è un plugin Unraid, quindi anche questo passaggio viene saltato.
- **Trovare i dati di un container.** Senza la convenzione `appdata` di Unraid, la cartella dati di un container viene trovata a partire dai segmenti in `DATA_ROOT_SEGMENTS`, dai volumi con nome di Docker, dalla directory di lavoro di un progetto Compose e dalla label `bombvault.data` (vedi [Rilevamento delle sorgenti di backup](configuration.md#backup-source-detection)). I volumi con nome e la preimpostazione `/etc` raggiungono solo percorsi all'interno del mount Host Data, quindi punta Host Data a una cartella antenata comune che copra anche la data root di Docker.
- **`PLATFORM`.** Impostala su `generic` o `truenas`. Se non è impostata, BombVault riconosce Unraid dal proprio marcatore sul mount del flash e tratta tutto il resto come generico, e i passaggi specifici di Unraid vengono saltati invece di essere tentati e fallire.

### TrueNAS SCALE {#truenas}

BombVault non è ancora nel catalogo app di TrueNAS, quindi installalo come app personalizzata: **Apps, Discover Apps**, poi **Install via YAML** nel menu accanto a **Custom App**. Lo YAML qui sotto imposta le stesse cose della voce di catalogo nella cartella `truenas-apps` del repository. Sostituisci `tank` con il tuo pool e compila `APP_KEY` come descrive la sezione successiva.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # Per i backup delle VM:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Poi apri `https://<truenas-ip>:30528/`. Quanto elencato in [Cosa cambia rispetto a Unraid](#generic-docker-host) vale anche qui, e su TrueNAS:

- **Capability.** BombVault ha bisogno delle sette qui sopra. Senza `SETPCAP` i backup delle VM falliscono, e senza `SYS_CHROOT` ogni container viene fermato per tutto il suo backup invece di essere letto da uno snapshot.
- **Host Data** è un percorso host su un dataset, qui `/mnt/tank`, montato allo stesso percorso dentro l'app. Per questo `HOST_SOURCE_ROOT` e `HOST_MOUNT_ROOT` lo indicano entrambe. `rslave` passa i mount che l'host crea in seguito, e i backup dei dataset ZFS ne hanno bisogno.
- **I dati in un ixVolume non finiscono nel backup.** Un'app il cui storage è un ixVolume tiene i suoi dati sotto `/mnt/.ix-apps`, fuori da Host Data. BombVault salva solo la definizione dell'app, e l'esecuzione nomina le cartelle che ha escluso. Per ogni app di cui vuoi salvare i dati, scegli lo storage **Host Path** su un dataset sotto Host Data.
- **I backup delle VM e dei dataset ZFS** raggiungono TrueNAS via SSH come root. Attiva il servizio SSH in **System, Services** e aggiungi la chiave pubblica di **Impostazioni, Integrazioni, SSH dell'host** all'utente root in **Credentials, Users**. I backup delle VM richiedono anche `LIBVIRT_URI` come sopra, perché il libvirtd di TrueNAS ascolta su un socket tutto suo. Se non sono impostate né `LIBVIRT_URI` né `LIBVIRT_HOST`, BombVault usa come host il gateway predefinito dell'app, dato che `host.docker.internal` non si risolve in un'app TrueNAS. La [guida al backup delle VM](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md#truenas-scale) copre i dettagli, comprese le VM su zvol.
- **Una replica ZFS usa un'altra chiave.** Una replica su un secondo server ZFS accede con la chiave mostrata dalla finestra della replica, non con la chiave SSH dell'host. Aggiungi quella sul server che riceve.
- **Un'app eliminata torna in due passi.** Prima reinstallala in TrueNAS con lo stesso nome, così la sua rete viene ricreata, poi ripristina i suoi dati in BombVault. Fino ad allora il controllo prima del ripristino segnala la rete mancante, e il ripristino si ferma prima di cambiare qualsiasi cosa.
- **Un ripristino Accanto all'originale** crea un semplice container Docker. Funziona, ma la schermata Apps di TrueNAS non lo elenca.
- **I dump dei database** funzionano sui mount dei dataset. Un import in una cartella dati che è un dataset a sé sposta il vecchio contenuto in una cartella accanto, quindi il dataset superiore deve avere spazio per accoglierlo.

## L'unica impostazione richiesta

L'unica variabile che devi impostare è `APP_KEY`, un segreto esadecimale di 32 byte (64 caratteri esadecimali) usato per derivare la password del repository restic.

Generane uno su qualsiasi macchina:

```bash
openssl rand -hex 32
```

Incolla il risultato nel campo `APP_KEY` del template (Unraid), oppure nella variabile d'ambiente `APP_KEY` in `docker-compose.yml` (host Docker generico).

!!! danger "Non perdere la tua APP_KEY"
    Perdere `APP_KEY` rende i tuoi backup cifrati irrecuperabili. Conservala in un luogo sicuro e separato dal server. Una volta che BombVault è in esecuzione, imposta una password di accesso e scarica il **Kit di ripristino** in **Impostazioni, Ripristino** (vedi [Off-site e ripristino](offsite-recovery.md#recovery-kit)). Contiene la chiave, ogni destinazione e i comandi restic.

Il template monta anche il socket Docker, il flash (`/boot`) e la radice **Host Data** (`/mnt`) per te. Le *origini* e le *destinazioni* dei backup risiedono entrambe sotto Host Data. Per il riferimento completo delle variabili e la configurazione off-site, vedi [Configurazione](configuration.md).

## Prima esecuzione

![La panoramica dopo un primo backup: cosa richiede la tua attenzione, la protezione di ogni sezione, le destinazioni e la cronologia.](assets/screenshots/dashboard.png)

*La panoramica dopo un primo backup: cosa richiede la tua attenzione, la protezione di ogni sezione, le destinazioni e la cronologia.*

1. Apri l'interfaccia web all'indirizzo `https://<your-unraid-ip>:3443` (certificato autofirmato pronto all'uso). Un'installazione nuova si apre su **Ti diamo il benvenuto in BombVault**, che elenca ciò che BombVault ha trovato sul server.
2. Scegli **Iniziamo**. La panoramica mostra poi la scheda **Primi passi** con tre passaggi:
    1. **Scegli le sezioni**: attiva ciò di cui BombVault fa il backup (Container, VM, Flash, Cartelle, Dataset ZFS, Auto-backup). **Scegli** apre l'elenco degli elementi di una sezione, dove **Sospendi pianificazione** ne lascia fuori uno. Le sezioni che attivi qui girano ogni notte, e lo cambi in **Impostazioni, Calendario**.
    2. **Aggiungi destinazione**: dove vanno i backup. Una cartella locale basta per iniziare. La scelta più sicura è una seconda destinazione fuori sede, come una Storage Box o Backblaze B2. I repository locali hanno come predefinito `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` e vengono creati al primo backup.
    3. **Primo backup**: dopo, va avanti da solo ogni notte.
3. Apri **Backup** per vedere ogni elemento in un unico elenco. **Esegui il backup di tutto ora** avvia un'esecuzione su tutte le sezioni attivate, ed **Esegui backup ora** nella pagina di un elemento ne salva solo quell'uno.

Hai già usato BombVault? **Ripristina da un backup** nella pagina di benvenuto apre la pagina Ripristino, dove la **Procedura guidata di emergenza** riporta indietro impostazioni, container, VM e cartelle di un server perso. Vedi [Off-site e ripristino](offsite-recovery.md#emergency).

!!! tip "Facoltativo: scegli un ordine di backup"
    Se alcuni container dovrebbero sempre essere sottoposti a backup prima di altri (per esempio un database prima dell'app che lo usa), apri la scheda **Ordine** in **Impostazioni, Calendario** e trascinali nella sequenza che vuoi. Le esecuzioni pianificate e il backup completo la seguono quindi; tutto ciò che lasci non ordinato viene sottoposto a backup dal più in ritardo per primo.

!!! note "Verifica integrazione host"
    La scheda **Integrazione host** della panoramica sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti, così puoi confermare che il container sia collegato correttamente prima di affidartici. Resta sotto **Personalizza** finché non la aggiungi.

## Orientarsi {#navigation}

![Le impostazioni come riquadri, con le schede del riquadro scelto accanto. Ogni modifica viene scritta mentre la fai.](assets/screenshots/settings.png)

*Le impostazioni come riquadri, con le schede del riquadro scelto accanto. Ogni modifica viene scritta mentre la fai.*

La barra laterale ha cinque voci: **Panoramica**, **Anomalie** (finché il rilevamento delle anomalie è attivo), **Backup**, **Ripristino** e, sotto **Esci**, **Impostazioni**. L'azione principale di una pagina fluttua in basso a destra, e **Altro** raccoglie quelle rare. Sul telefono la barra contiene Panoramica, Backup, Ripristino e Impostazioni, e **Altro** ha il resto.

Le pagine e le impostazioni usano tutta la larghezza della finestra. Una finestra tiene i suoi pulsanti nel piede, con quello che porta avanti a destra. Sul telefono un piede con più di tre pulsanti mantiene l'uscita e l'azione principale e mette gli altri sotto **Altro**.

- **Backup** elenca come elementi ogni container, VM, set di cartelle, la flash, ogni dataset ZFS e l'auto-backup. I riquadri sopra l'elenco scelgono un tipo, e **Filtri**, **Ordina** e **Cerca** restringono la scelta. Ogni elemento ha una pagina propria con tre schede: **Backup**, **Impostazioni** e **Verifica**. Un pulsante indietro a sinistra dell'icona dell'elemento porta all'elenco.
- **Ripristino** è un'unica pagina per tutto: scegli una sezione, cerca un elemento e scegli un backup. Anche la Procedura guidata di emergenza e le altre sorgenti partono da qui.
- **Impostazioni** sono quattordici riquadri: Generale, Aspetto, Calendario, Destinazioni, Ripristino, Notifiche, Verifica, Sicurezza, Istanze, Associazione, Integrazioni, App, Sistema e Info. Premi `/` o Ctrl+K per cercare in tutti, anche con i nomi che le pagine avevano nelle versioni precedenti. Trascina i riquadri nel tuo ordine in **Aspetto**. Dove una colonna di riquadri accanto alla pagina la stringerebbe, come in una finestra larga circa 1200 pixel, i riquadri diventano una striscia sopra la pagina che scorre di lato. Un telefono mostra la stessa striscia.
- **Destinazioni** e **Istanze** sono riquadri delle impostazioni con pagine proprie. **Mostra anche nella barra laterale** sul loro riquadro le fissa alla barra laterale.

Niente ha un pulsante Salva e niente si apre a fisarmonica: le cose rare hanno una scheda propria o una finestra, e le spiegazioni stanno nei fumetti (i). Dopo un aggiornamento si apre da sola una finestra con una scheda per ogni rilascio dall'ultima versione che avevi, e puntini verdi segnalano ciò che è cambiato. Un clic nasconde un puntino. **Novità** nella scheda Versione in **Impostazioni, Info** conta ciò che non hai ancora visto e riapre quella finestra, con le note di ogni rilascio e le sue modifiche con **Vai lì**.

## Compilare dai sorgenti {#build-from-source}

BombVault è un unico binario Go statico che serve un'API JSON e un'interfaccia React incorporata. Compila prima l'interfaccia, poi avvia il binario:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

La build dell'interfaccia serve anche per `go run`. Il repository traccia solo un marcatore vuoto sotto `web/dist`, quindi senza `npm --prefix web run build` il binario non incorpora nulla e risponde `500 SPA index not found`, ed è normale. Docker, libvirt e Unraid non si possono testare in CI, quindi verifica i mount, restic e il collegamento SSH delle VM su un host reale con la scheda **Integrazione host** della panoramica (o `GET /api/spike`) prima di aprire una pull request.

## Prossimi passi

- Sfoglia l'insieme completo delle **[Funzionalità](features.md)**.
- Porta tutti i server del tuo gruppo sul telefono con l'**[App Android](android.md)**.
- Aggiungi una o più destinazioni fuori sede (ogni sezione può copiare su più destinazioni contemporaneamente) e salva il tuo kit di ripristino, come descritto in **[Off-site e ripristino](offsite-recovery.md)**.
- Stai clonando una configurazione o passando a una nuova macchina? Porta con te l'intera configurazione con la scheda **Esporta / importa impostazioni**. Vedi [Configurazione](configuration.md#portable-settings-export-and-import).
- Hai incontrato un intoppo? Vedi **[Risoluzione dei problemi](troubleshooting.md)**.
