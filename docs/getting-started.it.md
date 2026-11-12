# Primi passi

Questa pagina ti accompagna da un server Unraid appena installato al tuo primo backup.

## Requisiti {#requirements}

| Requisito | Note |
|---|---|
| **Unraid 6.12+** | Le versioni precedenti non sono state provate. Unraid è l'obiettivo principale, ma BombVault gira anche su un normale host Docker e su TrueNAS Scale (vedi [Host Docker generico](#generic-docker-host) e [TrueNAS SCALE](#truenas)). |
| **Posizione del repository restic** | Un percorso locale (consigliato: il tuo array o la cache), SMB, NFS o qualsiasi backend rclone. |
| **Socket Docker** | Montato automaticamente dal template (`/var/run/docker.sock`). |
| **Flash di Unraid** (`/boot`) | Montata per intero automaticamente dal template (`/boot` su `/host/boot`). Rende possibile il backup della flash e permette a un container ripristinato di riapparire come una normale app Unraid modificabile. |
| **VM KVM** (opzionale) | Il backup delle VM parla con libvirt via SSH, senza mount libvirt. Configuralo in **Impostazioni, Integrazioni, SSH dell'host** (vedi [Configurazione](configuration.md)). |
| **Dataset ZFS** (opzionale) | Lo stesso collegamento SSH del backup delle VM, `zfs` sull'host, e Host Data mappato come `/mnt` con modalità di accesso Read/Write - Slave, il valore predefinito del template. Vedi [Dataset ZFS](zfs-datasets.md). |
| **App Android** (facoltativa) | Android 10 o successivo, associata a server con versione 9.7.0 o successiva. Vedi [App Android](android.md). |

## Installazione su Unraid {#install-on-unraid}

La via più semplice è **Community Applications**.

1. Apri la scheda **Apps** in Unraid.
2. Cerca **BombVault**.
3. Clicca **Install**, imposta le variabili obbligatorie (sotto) e applica.

!!! tip "Installazione manuale del template"
    Se preferisci aggiungere il template a mano:

    1. Vai a **Docker, Add Container, Template repositories** e aggiungi:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Cerca **BombVault** nei Templates.
    3. Imposta le variabili obbligatorie e clicca **Apply**.

## Host Docker generico {#generic-docker-host}

Non usi Unraid? BombVault gira anche come container normale su qualsiasi host Docker. TrueNAS SCALE ha i suoi passaggi in [TrueNAS SCALE](#truenas).

1. Prendi dal repository il file pronto da modificare [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml).
2. Imposta `APP_KEY` (vedi sotto) e punta il volume Host Data alla tua vera radice dei dati. I commenti del file spiegano entrambe le cose.
3. `docker compose up -d`, poi apri `https://<host-ip>:3443/`.

Cosa cambia rispetto a Unraid:

- **Nessuna sezione Flash.** Non c'è una chiavetta di avvio da catturare o ripristinare, quindi la sezione Flash qui non ha nulla da fare. In compenso, quando Host Data raggiunge `/etc` dell'host, **Aggiungi** nella pagina Backup propone **Configurazione di sistema** come cartella suggerita: un set di cartelle `/etc` di partenza che controlli e modifichi prima di salvare.
- **Nessuna notifica nativa di Unraid.** I canali di notifica propri di BombVault nell'app (webhook, avvisi di errore off-site e così via) funzionano come al solito. Viene saltato solo l'invio specifico di Unraid al suo sistema di notifiche nativo, dato che non esiste un sistema simile a cui inviare.
- **Il backup delle VM è opzionale e richiede un host libvirtd separato raggiungibile via SSH.** Vedi il blocco commentato nel file compose. Un host Docker generico non ha di suo alcun gestore di VM.
- **Nessun riquadro per la dashboard.** TollRoll è un plugin di Unraid, quindi anche quel passaggio viene saltato.
- **Trovare i dati di un container.** Senza la convenzione `appdata` di Unraid, la cartella dei dati di un container si trova dai segmenti in `DATA_ROOT_SEGMENTS`, dai volumi Docker con nome, dalla directory di lavoro di un progetto Compose e dalla label `bombvault.data` (vedi [Rilevamento delle sorgenti di backup](configuration.md#backup-source-detection)). I volumi con nome e il preset `/etc` raggiungono solo percorsi dentro il mount Host Data, quindi punta Host Data a un antenato comune che copra anche la radice dati di Docker.
- **`PLATFORM`.** Impostala su `generic` o `truenas`. Se la lasci non impostata, BombVault riconosce Unraid dal suo marcatore sul mount della flash e tratta tutto il resto come generico, e i passaggi solo Unraid vengono saltati invece di essere tentati e fallire.

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
      # For VM backups:
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

Poi apri `https://<truenas-ip>:30528/`. Ciò che è elencato in [Cosa cambia rispetto a Unraid](#generic-docker-host) vale anche qui, e su TrueNAS:

- **Capability.** BombVault ha bisogno delle sette qui sopra. I backup delle VM falliscono senza `SETPCAP`, e senza `SYS_CHROOT` ogni container viene fermato per tutto il suo backup invece di essere letto da uno snapshot.
- **Host Data** è un percorso dell'host su un dataset, qui `/mnt/tank`, montato allo stesso percorso dentro l'app. Per questo `HOST_SOURCE_ROOT` e `HOST_MOUNT_ROOT` lo nominano entrambe. `rslave` passa i mount che l'host crea in seguito, di cui hanno bisogno i backup dei dataset ZFS.
- **I dati in un ixVolume non vengono salvati.** Un'app il cui storage è un ixVolume tiene i suoi dati sotto `/mnt/.ix-apps`, fuori da Host Data. BombVault salva solo la definizione dell'app, e l'esecuzione nomina le cartelle che ha lasciato fuori. Per ogni app di cui vuoi salvare i dati, scegli lo storage **Host Path** su un dataset sotto Host Data.
- **I backup delle VM e dei dataset ZFS** raggiungono TrueNAS via SSH come root. Attiva il servizio SSH in **System, Services** e aggiungi la chiave pubblica da **Impostazioni, Integrazioni, SSH dell'host** all'utente root in **Credentials, Users**. I backup delle VM richiedono anche `LIBVIRT_URI` come sopra, perché il libvirtd di TrueNAS ascolta su un socket proprio. Se non sono impostati né `LIBVIRT_URI` né `LIBVIRT_HOST`, BombVault usa come host il gateway predefinito dell'app, dato che `host.docker.internal` non si risolve in un'app TrueNAS. La [guida al backup delle VM](vm-backup-ssh-setup.md#truenas-scale) copre i dettagli, comprese le VM su zvol.
- **Una replica ZFS usa una chiave diversa.** Una replica su un secondo server ZFS accede con la chiave che mostra la finestra della replica, non con la chiave SSH dell'host. Aggiungi quella sul server ricevente.
- **Un'app eliminata torna in due passaggi.** Reinstallala prima in TrueNAS con lo stesso nome, cosa che ricrea la sua rete, poi ripristina i suoi dati in BombVault. Fino ad allora la verifica del ripristino segnala la rete mancante, e il ripristino rifiuta prima di cambiare qualsiasi cosa.
- **Accanto all'originale** crea un normale container Docker. Gira, ma la schermata Apps di TrueNAS non lo elenca.
- **I dump dei database** funzionano sui mount di dataset. Un'importazione in una cartella dati che è a sua volta un dataset sposta il vecchio contenuto in una cartella accanto, quindi il dataset sopra ha bisogno di spazio per esso.

### ZimaOS, CasaOS e altri host {#other-hosts}

- **ZimaOS e CasaOS:** aggiungi lo [store della community](https://github.com/junkerderprovinz/zimaos-apps#install) e installa BombVault da lì. Lì gira come `PLATFORM=generic` con Host Data su `/DATA`. ZimaOS tiene i dati delle app in `/DATA/AppData/<app>`, che BombVault trova da solo, dato che `DATA_ROOT_SEGMENTS` corrisponde ad `appdata` in qualsiasi grafia.
- **MOS:** installa BombVault dal [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Lì gira come `PLATFORM=generic` con Host Data su `/mnt/cache`.
- **Synology:** in Container Manager crea un progetto dal file compose (DSM 7.2 o successivo).
- **Proxmox:** eseguilo in una VM o in un container LXC che abbia Docker.

Senza commenti, il file compose generico è breve. Sostituisci `./data` con la cartella dell'host che contiene i dati dei tuoi container, imposta `HOST_SOURCE_ROOT` su quello stesso percorso e compila `APP_KEY`:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Per salvare anche i volumi Docker con nome, monta una cartella che contenga sia i tuoi dati sia la radice dati di Docker (`docker info -f '{{.DockerRootDir}}'`, di solito `/var/lib/docker`), come il template Unraid monta tutto `/mnt`.

## L'unica impostazione obbligatoria {#the-one-required-setting}

L'unica variabile che devi impostare è `APP_KEY`, un segreto esadecimale di 32 byte (64 caratteri esadecimali) usato per derivare la password del repository restic.

Generane una su qualsiasi macchina:

```bash
openssl rand -hex 32
```

Incolla il risultato nel campo `APP_KEY` del template (Unraid), oppure nella variabile d'ambiente `APP_KEY` in `docker-compose.yml` (host Docker generico).

!!! danger "Non perdere la tua APP_KEY"
    Perdere `APP_KEY` rende i tuoi backup cifrati irrecuperabili. Conservala in un luogo sicuro e separato dal server. Quando BombVault è in funzione, imposta una password di accesso e scarica il **kit di ripristino** in **Impostazioni, Ripristino** (vedi [Off-site e ripristino](offsite-recovery.md#recovery-kit)). Contiene la chiave, ogni destinazione e i comandi restic.

Il template monta per te anche il socket Docker, la flash (`/boot`) e la radice **Host Data** (`/mnt`). Sia le *sorgenti* sia le *destinazioni* dei backup stanno sotto Host Data. Per il riferimento completo delle variabili e per la configurazione off-site vedi [Configurazione](configuration.md).

## Il primo avvio {#first-run}

![La pagina di benvenuto di una nuova installazione, con ciò che BombVault ha trovato sul server.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![La pagina di benvenuto di una nuova installazione, con ciò che BombVault ha trovato sul server.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Apri la web UI su `https://<your-unraid-ip>:3443` (certificato autofirmato di default). Una nuova installazione si apre su **Ti diamo il benvenuto in BombVault**, che elenca ciò che BombVault ha trovato sul server.
2. Scegli **Iniziamo**. La panoramica mostra allora la scheda **Primi passi** con tre passaggi:
    1. **Scegli le sezioni**: attiva ciò che BombVault salva (Container, VM, Flash, Cartelle, Dataset ZFS, Auto-backup). **Scegli** apre l'elenco degli elementi di una sezione, dove **Sospendi pianificazione** ne lascia fuori uno. Le sezioni che attivi qui girano ogni notte, cosa che cambi in **Impostazioni, Calendario**.
    2. **Aggiungi destinazione**: un secondo posto per i backup. Ogni sezione ha già una cartella su questo server, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, creata al primo backup. La cosa più sicura è una seconda destinazione fuori sede, come una Storage Box o Backblaze B2.
    3. **Primo backup**: dopo di che gira da solo ogni notte. La scheda resta finché il primo backup non è terminato.
3. Apri **Backup** per vedere ogni elemento in un unico elenco. **Esegui il backup di tutto ora** avvia un'esecuzione su tutte le sezioni attivate, ed **Esegui backup ora** nella pagina di un elemento salva solo quello.

![La scheda Primi passi con i suoi tre passaggi, il secondo aperto.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![La scheda Primi passi con i suoi tre passaggi, il secondo aperto.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Hai già usato BombVault? **Ripristina da un backup** nella pagina di benvenuto apre la pagina Ripristino, dove la **Procedura guidata di emergenza** riporta le impostazioni, i container, le VM e le cartelle di un server perso. Vedi [Off-site e ripristino](offsite-recovery.md#emergency).

!!! tip "Facoltativo: scegli un ordine di backup"
    Se alcuni container devono essere sempre salvati prima di altri (per esempio un database prima dell'app che lo usa), apri la scheda **Ordine** in **Impostazioni, Calendario** e trascinali nella sequenza che vuoi. Le esecuzioni pianificate e il backup completo la seguono. Quello che lasci senza ordine viene salvato partendo dal più in ritardo.

## Esempio: il tuo primo backup di un container, dall'inizio alla fine {#example-first-backup}

Questo esempio segue un server Unraid appena installato con alcuni container, tra cui `plex`, fino a quando il primo backup è fatto e sai dove trovarlo. Richiede circa dieci minuti più il tempo del primo backup.

1. **Installa.** Nella scheda **Apps** di Unraid cerca **BombVault** e premi **Install**.
2. **Imposta la chiave.** Su un computer esegui `openssl rand -hex 32` e incolla i 64 caratteri nel campo `APP_KEY` del template. Tienine subito una copia nel tuo gestore di password, prima di ogni altra cosa. Premi **Apply**.
3. **Aprilo.** Vai a `https://<your-unraid-ip>:3443` e accetta il certificato autofirmato. **Ti diamo il benvenuto in BombVault** elenca ciò che ha trovato, per esempio 12 container, 2 VM e la chiavetta flash.
4. **Iniziamo.** La panoramica si apre con la scheda **Primi passi**.
5. **Scegli le sezioni.** **Container** è attiva fin dall'inizio. Attiva ciò che vuoi in più, per esempio **Flash**. **Scegli** apre l'elenco dei container trovati, dove **Sospendi pianificazione** ne lascia fuori uno, per esempio un container di prova che non ti interessa. Le sezioni attivate girano ogni notte nel backup completo.
6. **Aggiungi destinazione.** Ogni sezione ha già la sua cartella su questo server, sotto `/mnt/user/bombvault`, così il primo backup ha un posto dove andare. Questo passaggio aggiunge un secondo posto, e il più sicuro è fuori sede. Puoi farlo subito con l'[esempio S3](offsite-recovery.md#example-s3), oppure tornarci quando il primo backup è finito. La scheda resta fino ad allora.
7. **Primo backup.** Apri **Backup** e premi **Esegui il backup di tutto ora**, oppure aspetta la notte. L'anello nell'intestazione di stato diventa un anello di avanzamento e conta gli elementi, per esempio **Backup in corso: 3 di 12**. Ogni container viene fermato mentre i suoi dati vengono letti e riavviato subito dopo. Un container i cui dati stanno su ZFS resta fermo solo per i pochi secondi di uno snapshot. Il primo backup legge tutto, quindi dura di più. Quelli successivi salvano solo ciò che è cambiato.
8. **Controllalo.** Quando l'anello dice **Tutto salvato**, apri **Cronologia** nella panoramica: ogni container ha una riga verde in **Esecuzioni**. Apri **Backup**, clicca `plex` e guarda la sua scheda **Backup**: il primo snapshot c'è, con **Locale** come posto che lo contiene. Qualche minuto dopo la sua scheda **Verifica** mostra la verifica del ripristino, che ha ripristinato un campione del backup e lo ha riletto.
9. **Proteggi la chiave.** Apri **Impostazioni, Sicurezza** e imposta una password di accesso. Poi apri **Impostazioni, Ripristino** e premi **Scarica il kit di ripristino** in **Per le emergenze**. Conserva il file fuori dal server, accanto alla copia di `APP_KEY` del passaggio 2. Il promemoria **Salva il tuo kit di ripristino** nella panoramica sparisce quando premi **L'ho conservato in modo sicuro**.

D'ora in poi i container vengono salvati ogni notte. Buoni passi successivi, in quest'ordine: una [destinazione off-site](offsite-recovery.md#example-s3), così un server morto non si porta via i backup, poi le [notifiche](notifications.md), così un errore ti raggiunge, poi un'occhiata alle [Pianificazioni](schedules.md) se le 03:00 non vanno bene per te.

![L'intestazione di stato mentre gira il primo backup, con l'anello di avanzamento e le sue tappe.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![L'intestazione di stato mentre gira il primo backup, con l'anello di avanzamento e le sue tappe.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Verifica dell'integrazione host"
    La scheda **Integrazione host** nella panoramica sonda ogni mount e ogni CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala ciò che manca, così puoi confermare che il container è collegato correttamente prima di fare affidamento su di esso. Aspetta in **Personalizza** finché non la aggiungi.

## Orientarsi {#navigation}

![Le impostazioni come riquadri, con le schede del riquadro scelto accanto. Ogni modifica viene scritta mentre la fai.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Le impostazioni come riquadri, con le schede del riquadro scelto accanto. Ogni modifica viene scritta mentre la fai.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

La barra laterale ha cinque voci: **Panoramica**, **Anomalie** (finché il rilevamento delle anomalie è attivo), **Backup**, **Ripristino** e, sotto **Esci**, **Impostazioni**. L'azione principale di una pagina galleggia in basso a destra, e **Altro** contiene quelle rare. Su un telefono la barra contiene Panoramica, Backup, Ripristino e Impostazioni, e **Altro** ha il resto.

Pagine e impostazioni usano tutta la larghezza della finestra. Una finestra tiene i suoi pulsanti nel piede, con quello che va avanti a destra. Su un telefono, un piede con più di tre pulsanti tiene l'uscita e l'azione principale e mette gli altri sotto **Altro**.

![Su un telefono, la barra in basso con Altro aperto.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Su un telefono, la barra in basso con Altro aperto.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Panoramica** mostra se tutto è salvato e cosa richiede attenzione. Vedi [Panoramica](overview.md).
- **Backup** elenca ogni container, VM, set di cartelle, la chiavetta flash, ogni dataset ZFS e l'auto-backup come elementi. I riquadri sopra l'elenco scelgono un tipo, e **Filtri**, **Ordina** e **Cerca** lo restringono. Ogni elemento ha una pagina propria con tre schede: **Backup**, **Impostazioni** e **Verifica**. Un pulsante indietro a sinistra dell'icona dell'elemento porta all'elenco. Vedi [Backup ed elementi](backups.md).
- **Ripristino** è un'unica pagina per tutto: scegli una sezione, cerca un elemento e scegli un backup. Anche la Procedura guidata di emergenza e le altre sorgenti partono da qui. Vedi [Ripristino](restore.md).
- **Impostazioni** sono quattordici riquadri: Generale, Aspetto, Calendario, Destinazioni, Ripristino, Notifiche, Verifica, Sicurezza, Istanze, Associazione, Integrazioni, App, Sistema e Info. Vedi [Impostazioni](settings.md). Premi `/` o Ctrl+K per cercarle tutte, anche con i nomi che le pagine avevano nelle versioni precedenti. Trascina i riquadri nel tuo ordine in **Aspetto**. Dove una colonna di riquadri accanto alla pagina la stringerebbe troppo, come in una finestra larga circa 1200 pixel, i riquadri diventano una striscia sopra la pagina che scorre di lato. Un telefono mostra la stessa striscia.
- **Destinazioni** e **Istanze** sono riquadri delle impostazioni con pagine proprie. **Mostra anche nella barra laterale** sul loro riquadro li fissa alla barra laterale.

Niente ha un pulsante Salva e niente si apre a tendina: le cose rare hanno una scheda propria o una finestra, e le spiegazioni stanno nei fumetti (i). Dopo un aggiornamento si apre da sola una finestra con una scheda per ogni release dall'ultima versione che avevi, e punti verdi segnano ciò che è cambiato. Un clic nasconde un punto. **Novità** nella scheda Versione in **Impostazioni, Info** conta ciò che non hai ancora visto e riapre quella finestra, con le note di ogni release e le sue modifiche con **Vai lì**.

## Compilare dal sorgente {#build-from-source}

BombVault è un unico binario Go statico che serve un'API JSON e un'interfaccia React incorporata. Compila prima l'interfaccia, poi esegui il binario:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

La compilazione dell'interfaccia serve anche per `go run`. Il repository tiene traccia solo di un marcatore vuoto sotto `web/dist`, quindi senza `npm --prefix web run build` il binario non incorpora nulla e risponde `500 SPA index not found`, come previsto. Docker, libvirt e Unraid non si possono provare in CI, quindi controlla mount, restic e il collegamento SSH delle VM su un host vero con la scheda **Integrazione host** della panoramica (o `GET /api/spike`) prima di aprire una pull request.

## Passi successivi {#next-steps}

- Aggiungi una o più destinazioni fuori sede (ogni sezione può copiare su più destinazioni insieme) e salva il tuo kit di ripristino, come descritto in **[Off-site e ripristino](offsite-recovery.md)**. L'[esempio S3](offsite-recovery.md#example-s3) ne illustra uno.
- Assicurati che un errore ti raggiunga con le **[Notifiche](notifications.md)**.
- Controlla quando girano le cose in **[Pianificazioni](schedules.md)**.
- Porta sul telefono tutti i server del tuo gruppo con l'**[App Android](android.md)**.
- Gli **[esempi pratici](guides.md)** ti guidano nei compiti più comuni, e **[Funzionalità](features.md)** elenca tutto ciò che BombVault sa fare.
- Cloni una configurazione o passi a una macchina nuova? Porta con te tutta la configurazione con la scheda **Esporta / importa impostazioni**. Vedi [Configurazione](configuration.md#portable-settings-export-and-import).
- Hai incontrato un intoppo? Vedi **[Risoluzione dei problemi](troubleshooting.md)**.
