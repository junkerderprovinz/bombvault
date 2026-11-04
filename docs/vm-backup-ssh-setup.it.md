# Backup delle VM

BombVault salva le VM KVM/libvirt senza montare alcun percorso libvirt. Esegue `virsh` sull'host via SSH (`qemu+ssh://`), legge e scrive i dischi delle VM attraverso il mount Host Data (`/mnt` sull'host è `/host/user` nel container) e copia la NVRAM UEFI via SSH. Poiché nulla che appartenga a libvirt viene montato con bind mount, BombVault non può mai intralciare il VM Manager di Unraid.

Questa guida tratta la configurazione, i metodi di backup, le VM con snapshot Unraid e TrueNAS SCALE. I backup dei container non richiedono nulla di tutto questo.

I backup dei dataset ZFS usano lo stesso collegamento SSH, con la stessa chiave, lo stesso host e lo stesso utente. Quando i backup delle VM si connettono, si connettono anche gli elementi dataset ZFS. Vedi [Dataset ZFS](zfs-datasets.md).

## Cosa configuri {#what-you-configure}

| Impostazione | Dove | Predefinito | Significato |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | variabile del template | `host.docker.internal` | L'host Unraid, raggiunto via SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | variabile del template | `22` | La porta SSH di Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | variabile del template | `root` | L'utente SSH sull'host |
| Chiave pubblica | **Impostazioni, Integrazioni, SSH dell'host** | creata al primo avvio | Autorizzala sull'host |

BombVault crea la sua coppia di chiavi SSH al primo avvio in `/config/ssh/id_ed25519`, conservata in appdata. La chiave dell'host viene fissata in `/config/ssh/known_hosts` alla prima connessione.

Il template compila **Host SSH: Address** con il segnaposto `192.168.x.x`. BombVault lo tratta come non impostato e prova `host.docker.internal`, che funziona sulla rete `bridge` predefinita. Su una rete personalizzata, sostituiscilo con l'IP LAN dell'host (passaggio 3 B).

## Passaggio 1: attiva SSH sull'host Unraid {#step-1}

1. In Unraid apri **Settings, Management Access** e imposta **Use SSH** su **Yes**.
2. Se hai cambiato la porta SSH, annotala. **Host SSH: Port** deve corrispondere.

## Passaggio 2: autorizza la chiave pubblica di BombVault {#step-2}

La `/root` di Unraid vive in memoria e viene cancellata a ogni avvio, quindi la chiave deve stare anche sulla chiavetta flash. Esegui questo nel terminale di Unraid:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 e successivi copiano `/boot/config/ssh/root.pubkeys` nel file `authorized_keys` di root a ogni avvio, quindi questo sopravvive ai riavvii. Vale per qualsiasi accesso `root`, qualunque nome host o porta usi BombVault.

## Passaggio 3: la rete {#step-3}

BombVault deve aprire una connessione TCP dal container alla porta SSH dell'host. Scegli il caso che corrisponde alla tua configurazione.

### A. BombVault sulla rete bridge predefinita {#bridge}

- Lascia **Host SSH: Address** su `host.docker.internal` o sul segnaposto del template. Il template aggiunge `--add-host=host.docker.internal:host-gateway`, che punta al gateway docker0, cioè all'host.
- Imposta **Host SSH: Port** sulla tua porta SSH.

### B. BombVault su una rete personalizzata {#custom-network}

Un container su `br0.x`, macvlan o ipvlan non può raggiungere l'host tramite `host.docker.internal`, perché 172.17.0.1 è docker0 e non è raggiungibile da `br0.x`. Invece:

1. In Unraid imposta **Settings, Docker, Host access to custom networks** su **Enabled**.
2. Imposta **Host SSH: Address** sull'IP LAN dell'host Unraid, l'indirizzo su cui apri la web UI.
3. Se la rete del container e l'host stanno su VLAN diverse, consenti la route sul router o sul firewall: dalla VLAN del container all'IP dell'host sulla porta SSH, via TCP.
4. Se l'IP LAN dell'host risulta ancora irraggiungibile, usa l'IP shim dell'host sulla subnet del container. Lo trovi sull'host con `ip -4 addr show | grep -B2 <container-subnet>`.

### Controlla che l'host risponda {#verify-reachability}

Esegui questo nel terminale di Unraid, con il tuo IP e la tua porta:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` significa che il percorso funziona. `UNREACHABLE` significa che la rete del passaggio 3 va sistemata prima di tutto il resto.

## Passaggio 4: imposta le variabili e prova {#step-4}

1. In Unraid apri **Docker**, clicca BombVault, scegli **Edit**, imposta **Host SSH: Address** (e **Host SSH: Port** se non è 22) e premi **Apply**. Se le variabili mancano, importa di nuovo il template, perché Unraid conserva la configurazione salvata di un container esistente.
2. In BombVault apri **Impostazioni, Integrazioni, SSH dell'host** e premi **Prova connessione**. Dovrebbe diventare verde.

La stessa chiamata dal terminale elenca le tue VM:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Passaggio 5: il metodo per ogni VM {#per-vm-method}

Ogni VM ha un **Metodo** nella scheda **VM** della sua scheda Impostazioni (apri la VM in **Backup**):

- **Ordinato (spegnimento)** è il predefinito. Spegne la VM, salva i dischi e la riavvia, quindi il backup è sempre coerente. Aspetta fino a dieci minuti che il guest si spenga e non lo forza mai. Un guest che gira ancora dopo quel tempo viene lasciato in pace, e quel backup fallisce con il motivo.
- **Snapshot a caldo** salva una VM in esecuzione tramite uno snapshot esterno e `blockcommit`. Richiede il guest agent di qemu nella VM, per uno snapshot quiescente, e il disco su `/mnt/cache` o `/mnt/diskX`, non su `/mnt/user`. Su una VM spenta, il metodo a caldo ripiega da solo su quello ordinato, e lo stesso fa quando lo snapshot non si può creare.

## Solo blocchi modificati {#changed-blocks}

Una VM con dischi qcow2 si può salvare leggendo solo ciò che è cambiato. Attiva **Solo blocchi modificati** nella stessa scheda, e BombVault usa l'API di backup di libvirt invece del metodo sopra:

- Avvia un job di backup in modalità pull (`virsh backup-begin`) che crea nello stesso istante un checkpoint chiamato `bombvault-<time>`, e legge i dischi attraverso il collegamento SSH tramite un socket Unix inoltrato. Quando il guest agent risponde, i filesystem del guest vengono congelati solo mentre il job parte. Uno scongelamento che fallisce viene ritentato per circa quindici secondi, e se il guest ancora non si scongela, BombVault invia una notifica.
- La prima esecuzione legge ogni blocco allocato. Le successive leggono solo i blocchi che la bitmap del checkpoint segna come scritti dal backup precedente.
- Ogni disco viene salvato come una cartella di segmenti di dimensione fissa che contengono ciò che vede il guest. I segmenti invariati vengono ripresi dallo snapshot precedente senza essere letti, così ogni snapshot è completo e si ripristina da solo.
- Un ripristino ricostruisce il disco dai suoi segmenti e lo riconverte in qcow2 con `qemu-img`. Il disco ha lo stesso contenuto di quando è stato fatto il backup, ma il file è un nuovo file qcow2, e una catena di backing torna appiattita.
- Il controllo prima di un ripristino simile elenca ogni disco che ricostruisce e lo conta alla dimensione piena del guest, due volte per qcow2, perché l'immagine raw e la sua copia convertita stanno fianco a fianco finché il vecchio file non viene sostituito. È il massimo che un disco può occupare, quindi un'insufficienza secondo questo conteggio lascia la riga dello spazio grigia invece di bloccare. La verifica del ripristino dopo il primo backup, e **Verifica ripristino**, controllano anche che lo snapshot contenga ogni segmento di ogni disco alla dimensione che dovrebbe avere.
- Tra un'esecuzione e l'altra la VM non ha alcun checkpoint registrato in libvirt. BombVault tiene la definizione del checkpoint `bombvault-` più recente sotto `/config/vm-checkpoints` e rimuove solo il record di libvirt (`virsh checkpoint-delete --metadata`). La sua bitmap resta nel file qcow2 e continua a contare le scritture, e l'esecuzione successiva rimette a posto il checkpoint (`virsh checkpoint-create --redefine`) prima di partire. libvirt rifiuta di rimuovere la definizione di una VM che ha checkpoint, quindi è questo che permette a **Remove VM** di Unraid e a un ripristino di funzionare come al solito.
- Un job di backup che BombVault ha lasciato in esecuzione, perché si è fermato nel mezzo di un backup, viene terminato quando BombVault riparte, insieme ai checkpoint che quell'esecuzione ha lasciato registrati.
- Disattivare l'interruttore, togliere la VM dalla pianificazione o eliminare i suoi backup termina anch'esso un job simile, ed elimina anche la bitmap finché la VM gira. Su una VM spenta, libvirt non può eliminarla, e resta inutilizzata nell'immagine.
- I checkpoint di altri strumenti vengono lasciati in pace. Una VM che ne ha non può essere ripristinata finché non vengono eliminati, e il ripristino si ferma prima di toccare la VM.

L'interruttore ripiega da solo. Un checkpoint mancante o rotto, un disco che ha cambiato dimensione, o uno snapshot più recente che non è di blocchi modificati significa che l'intero disco viene letto ancora una volta. Una VM spenta, un disco raw o zvol, nessun collegamento SSH all'host, un libvirt senza l'API di backup, o troppo poco spazio libero sotto `/config` significano il backup consueto con il metodo sopra. La scheda dice quale di questi ha usato l'ultima esecuzione e perché.

Requisiti: libvirt con l'API di backup (provato con libvirt 12.2 e QEMU 10.2 su Unraid 7.3), sshd con `AllowStreamLocalForwarding yes` (il predefinito, anche su Unraid, che disattiva l'inoltro TCP) e dischi qcow2. libvirt rifiuta i checkpoint sui dischi raw, e Unraid crea di default vdisk raw. La VM continua a girare durante il backup, come con il metodo a caldo, e i segmenti in attesa di restic occupano fino a 4 GiB sotto `/config`.

## VM con snapshot Unraid {#unraid-snapshots}

Uno snapshot che il VM Manager di Unraid crea trasforma un disco qcow2 in una catena: l'immagine originale in fondo e sopra di essa un file per ogni snapshot, ciascuno contenente solo ciò che è cambiato rispetto a quello sotto. La VM scrive nel file più recente. Un disco con due snapshot sono tre file.

BombVault salva una VM simile per intero:

- **Ogni file della catena.** Segue la catena di backing di ogni disco fino in fondo e salva ogni file al suo interno, insieme all'elenco degli snapshot di Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, letto via SSH). Una catena che non si può seguire fino in fondo, per esempio perché manca un file nel mezzo, fa fallire il backup e nomina il file.
- **Un backup a caldo conserva gli snapshot.** **Snapshot a caldo** mette sopra un overlay temporaneo mentre legge il disco e lo riunisce poi, solo nell'immagine appena sotto. La catena è la stessa dopo il backup come prima. Una VM senza snapshot si comporta come ha sempre fatto.
- **Un ripristino riporta la catena.** Un ripristino sul posto riscrive ogni file dov'era e rimette l'elenco degli snapshot di Unraid, così il VM Manager elenca di nuovo gli snapshot. Un ripristino accanto all'originale scrive i file nella cartella della copia e punta i dischi ripristinati ai file ripristinati, così la copia non legge mai dalle immagini dell'originale. L'elenco degli snapshot di Unraid torna solo con un ripristino sul posto, perché nomina i file dove sono stati salvati. Senza di esso la copia si avvia allo stesso modo, e il VM Manager semplicemente non elenca snapshot per essa.
- **Solo blocchi modificati** legge ciò che vede il guest, quindi il suo ripristino torna come un unico file qcow2 appiattito senza gli snapshot.

### Esempio: una VM con due snapshot {#example-unraid-snapshots}

La VM `homeassistant` ha un disco, `vdisk1.qcow2`, e due snapshot che hai creato nel VM Manager prima di due aggiornamenti. Il suo disco è ora una catena di tre file. Gira sempre, quindi usa **Snapshot a caldo**.

1. Apri **Backup**, poi `homeassistant`, poi la sua scheda **Impostazioni**. Nella scheda **VM** controlla che **Metodo** sia **Snapshot a caldo** e che **Solo blocchi modificati** sia disattivato.
2. Premi **Esegui backup ora** nell'intestazione della pagina.
3. Quando l'esecuzione è finita, la sua riga in **Esecuzioni recenti** nella scheda **Verifica** è verde. La VM ha continuato a girare per tutto il tempo, e il backup contiene tutti e tre i file del disco e l'elenco degli snapshot di Unraid.
4. Nel VM Manager i due snapshot ci sono ancora, e il disco legge ancora attraverso gli stessi tre file.

Un mese dopo un aggiornamento va storto e vuoi riavere lo stato di ieri accanto a quello rotto, per confrontarli:

1. Apri **Ripristino**, scegli **VM** e clicca `homeassistant`.
2. Tieni il backup consigliato di ieri notte e scegli **Accanto all'originale** sotto **Dove**.
3. Premi **Ripristina**. I tre file finiscono in una cartella con il nome della VM più `-restored`, e il disco ripristinato punta ai file ripristinati. Nulla cambia sulla VM in funzione.

Per riportare indietro l'intera VM, scegli invece **Sostituisci l'originale** con **Prima fai il backup dello stato attuale** attivo. BombVault salva lo stato rotto, spegne la VM, riscrive i tre file e l'elenco degli snapshot di Unraid e la riavvia.

## TrueNAS SCALE {#truenas-scale}

I passaggi sopra sono scritti per Unraid, ma lo stesso collegamento `qemu+ssh://` funziona con qualsiasi libvirtd che il container possa raggiungere, TrueNAS SCALE compreso, che ha di nuovo le VM su libvirt dalla 25.04.2. Su TrueNAS la stringa di connessione richiede una sostituzione e la chiave va sull'utente root nelle impostazioni utente di TrueNAS. Altre tre cose cambiano ciò che contiene un backup: dischi su zvol, stato del vTPM e i nomi che TrueNAS dà alle sue VM.

### 1. Il socket libvirt {#truenas-socket}

Il libvirtd di TrueNAS non ascolta dove si aspetta la stringa di connessione costruita dai campi **Host SSH**. Imposta invece `LIBVIRT_URI`, con il parametro aggiuntivo `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault usa `LIBVIRT_URI` esattamente come scritto, e `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` e `LIBVIRT_SSH_PORT` non servono. Per ognuna di esse che non è impostata, BombVault prende host, utente e porta dall'URI anche per i propri comandi SSH, così la copia della NVRAM e i backup dei dataset ZFS raggiungono la stessa macchina di `virsh`. Una variabile che imposti tu prevale sempre, e il log di avvio nomina la destinazione presa dall'URI. Su TrueNAS un utente diverso da root richiede anche `zfs allow <user> snapshot,destroy,mount <dataset>` per i backup dei dataset ZFS, vedi [Dataset ZFS](zfs-datasets.md#truenas).

Metti nell'URI l'indirizzo LAN di TrueNAS. Se non sono impostati né `LIBVIRT_URI` né `LIBVIRT_HOST`, BombVault prende come host il gateway predefinito dell'app, perché `host.docker.internal` non si risolve in un'app TrueNAS, e il log di avvio nomina l'indirizzo scelto. Basta per i backup dei dataset ZFS. I backup delle VM richiedono comunque l'URI per il socket.

L'app ha bisogno della capability `SETPCAP` per i backup delle VM, accanto a `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` e `SETUID`. Senza, libvirt si ferma con "cannot apply process capabilities". Lo YAML in [TrueNAS SCALE](getting-started.md#truenas) le imposta tutte, insieme a `SYS_CHROOT`, necessaria per lo snapshot invece dello stop.

Su TrueNAS SCALE 25.10.0 il percorso del socket è stato verificato su hardware reale: il predefinito `qemu:///system` fallisce con `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, e la stessa chiamata con `?socket=/run/truenas_libvirt/libvirt-sock` elenca i domini.

### 2. La chiave di BombVault su TrueNAS {#truenas-key}

Il passaggio 2 sopra scrive sulla chiavetta flash di Unraid e qui non si applica. Su TrueNAS:

1. Attiva il servizio SSH in **System, Services**.
2. In **Credentials, Users** modifica **root** e incolla la chiave pubblica di BombVault da **Impostazioni, Integrazioni, SSH dell'host** come sua chiave pubblica SSH. TrueNAS la conserva attraverso riavvii e aggiornamenti.

Le installazioni nuove di TrueNAS 24.10 e successive non danno a root alcuna password, e l'accesso con la chiave non ne ha bisogno. Se preferisci non usare root, autorizza la chiave per un account amministratore con sudo e metti il suo nome al posto di `root` in `LIBVIRT_URI`. I backup dei dataset ZFS richiedono allora `zfs allow` come descritto sopra.

Una replica ZFS verso un secondo server accede con una chiave propria, che mostra la finestra della replica. La chiave SSH dell'host non autorizza una replica, e la chiave della replica non autorizza i backup delle VM.

!!! warning "TrueNAS non vede le modifiche fatte con virsh"
    Il middleware di TrueNAS, lo strato dietro le sue schermate delle VM, è pensato per essere l'unico modo in cui le sue VM vengono modificate. Un comando `virsh` su questo collegamento SSH, che è ciò che eseguono il backup e il ripristino delle VM di BombVault, lo aggira, e TrueNAS può in seguito riconciliare o sovrascrivere uno stato che non ha creato. Questo deriva dal comportamento documentato di TrueNAS e non è stato visto su una macchina vera. È lo stesso tipo di rischio del VM Manager di Unraid che riconcilia modifiche fatte fuori da esso, solo più marcato su TrueNAS.

### 3. Dischi su zvol {#truenas-zvols}

I dischi delle VM TrueNAS sono spesso zvol, dispositivi a blocchi ZFS in `/dev/zvol/<pool>/<dataset>`, non file qcow2. restic non può salvare un dispositivo a blocchi dal suo percorso come salva un file, quindi BombVault riconosce un disco zvol nell'XML del dominio (`<source dev="...">` invece di `<source file="...">`) e lo gestisce sullo stesso collegamento SSH. Il container BombVault non ha bisogno di strumenti ZFS.

Un backup di uno zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fissa un istante, anche mentre la VM gira.
2. `zfs clone -o volmode=dev -o readonly=on` rende quello snapshot leggibile come dispositivo. `volmode=dev` evita che l'host analizzi le partizioni del guest.
3. `dd` legge il dispositivo, e l'immagine raw entra in restic come un unico file, `/vm-disks/<dataset>.img`, con il tag `vm:<name>:zvol:<dev>`. restic la taglia in blocchi in base al contenuto, quindi i blocchi che non sono cambiati dall'ultimo backup non vengono salvati di nuovo.
4. Il clone e lo snapshot vengono distrutti dopo, sia che il backup sia riuscito o no. Se BombVault si è fermato nel mezzo di un backup, il backup successivo di quello zvol rimuove prima lo snapshot `bombvault-vm-<time>` e il suo clone `<dataset>-bombvault-vm-<time>`. Lascia in pace ogni altro snapshot e tiene quello da cui è stato clonato qualcos'altro. Il log nomina ciò che ha rimosso o tenuto.

BombVault non usa `zfs send` per questo. Ogni record di uno stream di send porta il GUID dello snapshot e un checksum progressivo, quindi restic non troverebbe nulla dell'ultimo backup da riusare e salverebbe di nuovo l'intero disco ogni volta. Misurato su TrueNAS SCALE 25.10 con uno zvol da 512 MiB che contiene 200 MiB di dati: il primo backup ha aggiunto 211 MB, un secondo senza modifiche 52 KB, e uno dopo la modifica di 1 MiB 3,6 MB. Lo stesso zvol tramite `zfs send` ha aggiunto 214 MB a ogni esecuzione.

Ogni esecuzione scrive anche un piccolo snapshot con il tag `vm:<name>` con l'XML del dominio, una volta dentro tutti i dischi. Una VM i cui dischi sono tutti zvol non ha alcuno snapshot di file, e questo è quello che elenca l'esecuzione sotto la VM e la rende ripristinabile.

Un ripristino riscrive ogni zvol sul posto, come un disco file viene riscritto sul suo file, così la VM ripristinata si avvia con i dati ripristinati:

1. Prima di toccare qualsiasi cosa, BombVault controlla che l'esecuzione contenga un'immagine di ogni disco zvol. Un'esecuzione che ne manca una viene rifiutata e la VM resta com'è.
2. Uno zvol esistente riceve prima uno snapshot, `<dataset>@bombvault-before-restore-<time>`, con l'ora in UTC come ogni nome di snapshot che BombVault crea, quindi può essere sfasato di ore rispetto all'orologio locale. Il log lo nomina, e `zfs rollback` su di esso annulla il ripristino finché è lo snapshot più recente dello zvol. Quando ne esiste uno più recente, come quello del ripristino successivo, lo fa `zfs rollback -r`, che distrugge gli snapshot più recenti. Resta finché non lo rimuovi con `zfs destroy` una volta che la VM ripristinata funziona.
3. Uno zvol che non esiste più viene creato alla dimensione che aveva quando è stato salvato, e uno esistente di un'altra dimensione viene riportato a quella dimensione.
4. `dd` scrive l'immagine su `/dev/zvol/<dataset>`. Con la compressione attiva, il predefinito di TrueNAS, le parti vuote dell'immagine tornano come buchi e non occupano spazio.

**Un ripristino su un'altra istanza richiede un pool di destinazione.** Sposta i dischi file in una cartella di destinazione, ma una cartella non nomina alcun pool ZFS, quindi senza uno BombVault rifiuta il ripristino prima che parta. Passa il pool come parametro `zvolPool` di `POST /api/foreign/restore`: ogni zvol mantiene il suo percorso sotto il pool (`tank/vms/win10/disk0` su `flashpool` diventa `flashpool/vms/win10/disk0`), e l'XML del dominio punta lì. La finestra **Da una vecchia cartella di backup** nella pagina Ripristino non ha un campo per questo, quindi si raggiunge solo con una chiamata diretta all'API. Un ripristino sulla stessa istanza riscrive ogni zvol su sé stesso.

**L'esportazione come file lascia fuori gli zvol.** Una VM i cui dischi sono tutti zvol viene rifiutata con questo motivo, quindi ripristinala da un backup. Per una VM con anche dischi file, l'esportazione contiene i dischi file e una nota che nomina gli zvol lasciati fuori.

È stato provato su TrueNAS SCALE 25.10 con ZFS 2.3.4: tre backup di uno zvol di prova, un ripristino sul posto dopo che lo zvol era stato sovrascritto e ingrandito, e un ripristino su uno zvol che era stato eliminato. Ogni ripristino è stato riletto con lo stesso SHA-256 della sorgente, lo zvol aveva di nuovo la sua vecchia dimensione, e non è rimasto alcuno snapshot o clone. Una VM intera il cui unico disco è uno zvol è stata salvata in funzione, con il metodo ordinato e con quello a caldo, e da spenta, e anche un ripristino dell'intera VM (rimozione della definizione, scrittura degli zvol, definizione, avvio) è stato eseguito lì: lo zvol è stato riletto con il checksum della sorgente e la VM è partita.

I dischi file, come li usa Unraid, non sono toccati da nulla di tutto questo.

### 4. Stato del vTPM {#truenas-vtpm}

Una VM può avere un TPM virtuale, di cui hanno bisogno Secure Boot e i guest Windows 11. Il suo stato vive fuori dal disco e dalla NVRAM, quindi un backup solo di quei due riporterebbe un guest con un TPM nuovo e vuoto.

- **Un TPM passthrough**, un chip reale passato al guest, nomina il suo percorso nell'XML del dominio. BombVault salva il suo stato accanto alla NVRAM via SSH e lo riscrive prima di `virsh define`, al meglio delle possibilità come la NVRAM: una lettura o scrittura fallita viene registrata e non fa fallire il backup o il ripristino.
- **Un vTPM emulato**, quello che TrueNAS imposta per i guest Windows 11 e Secure Boot, non pubblica alcun percorso di stato nell'XML del dominio. BombVault riporta allora "no TPM path found" invece di indovinare un percorso. Tieni a portata di mano la chiave di recupero di un guest simile, come la sua chiave BitLocker, prima di ripristinarlo.
- **Non puoi collegare tu un TPM a una VM TrueNAS.** L'API dei dispositivi VM di TrueNAS SCALE 25.10 accetta solo `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` e `USB`, e rifiuta `TPM` come tipo sconosciuto.

La gestione del TPM è provata solo con test unitari e non ha girato su un vero guest TrueNAS con un vTPM. Una VM senza TPM, come su Unraid, viene salvata e ripristinata esattamente come prima.

La NVRAM viene catturata come al solito. TrueNAS la tiene per VM in `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, e l'XML del dominio nomina quel percorso.

### 5. Nomi delle VM su TrueNAS {#truenas-names}

Su Unraid il nome libvirt di una VM è il nome che hai scelto. TrueNAS inventa il nome libvirt, e lo ha cambiato due volte:

| Versione di TrueNAS | Nome libvirt | Dove sta il tuo nome |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, come `1_debian` | nel nome libvirt, dopo `{id}_` |
| 26 | l'UUID della VM, come `550e8400-e29b-41d4-a716-446655440000` | solo nel `<title>` dell'XML del dominio |

Su un host che BombVault riconosce come TrueNAS, o che `PLATFORM=truenas` nomina, l'elenco Backup mostra il tuo nome: `debian` invece di `1_debian` sulla 25.10, e il `<title>` invece dell'UUID sulla 26, oppure l'UUID quando non c'è un titolo. È solo ciò che mostra l'elenco. Ogni chiamata `virsh` e le impostazioni per VM usano ancora il nome libvirt, quindi backup e ripristino agiscono sempre sulla VM giusta. Sulla TrueNAS 26 questo significa che una VM che TrueNAS ricrea con un nuovo UUID compare come un nuovo elemento senza le sue impostazioni, anche se il suo nome sembra lo stesso. Su Unraid non cambia nulla, nemmeno per una VM che hai chiamato `10_Windows`.

Lo schema della 25.10 è stato visto su una vera macchina TrueNAS SCALE 25.10. Lo schema della TrueNAS 26 viene dalla documentazione di TrueNAS ed è provato solo con test unitari.

## Risoluzione dei problemi {#troubleshooting}

| Sintomo | Causa e soluzione |
|---|---|
| **Prova connessione** si blocca, poi fallisce | L'host non è raggiungibile. Controlla il passaggio 3 (rete, VLAN, firewall) e **Host SSH: Address** e **Port**. |
| `Permission denied (publickey)` anche se la chiave è in `authorized_keys` | `StrictModes` di sshd rifiuta il file della chiave perché una cartella superiore ha proprietario o permessi sbagliati. È comune quando `/root/.ssh` è un link alla chiavetta flash FAT (`/boot/config/ssh/...`), che è scrivibile da tutti. Il log dell'host mostra `Authentication refused: bad ownership or modes for directory ...`. Aggiungi `StrictModes no` alla configurazione sshd dell'host nella sezione globale, prima di qualsiasi blocco `Match`, poi esegui `/etc/rc.d/rc.sshd restart`. Unraid riscrive `/etc/ssh/sshd_config` a ogni avvio, quindi rendilo persistente come descritto sotto. |
| `Permission denied (publickey)` e la chiave manca davvero | Ripeti il passaggio 2 e controlla che SSH sia attivo. |
| `Host key verification failed` | Esegui `docker exec BombVault rm -f /config/ssh/known_hosts` e prova di nuovo. |
| Il controllo del passaggio 3 dice `UNREACHABLE` | Il container non raggiunge la porta SSH dell'host. Vedi il passaggio 3: accesso dell'host alle reti personalizzate, instradamento VLAN, oppure l'IP shim o LAN. |
| Le variabili mancano sotto **Edit** | Importa di nuovo il template. Un container esistente conserva la vecchia configurazione salvata. |
| Una VM con snapshot Unraid fallisce e nomina un file | La catena di backing del disco non si può seguire fino in fondo. Controlla nel VM Manager che ogni snapshot di quel disco ci sia ancora e che il file nominato esista sotto `/mnt`. |

Altri sintomi delle VM sono in [Risoluzione dei problemi](troubleshooting.md#vm-backup-will-not-connect).

## Mantenerlo dopo i riavvii {#persistence}

Unraid riscrive `/etc/ssh/sshd_config` a ogni avvio, quindi le modifiche lato host vanno rifatte all'avvio:

- La **porta SSH** è conservata in `/boot/config/ident.cfg`, impostata tramite la web UI di Unraid.
- **`StrictModes no`**, se ti è servito sopra, non viene conservato. Applicalo all'avvio da `/boot/config/go`:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- La chiave SSH di BombVault e `~/.ssh/config` stanno in `/config` (appdata) e vengono ricreati a ogni avvio, quindi sopravvivono ai riavvii del container e dell'host.

## Sicurezza {#security}

La chiave autorizzata concede root sull'host, lo stesso livello di fiducia del socket Docker che BombVault già usa. Tieni BombVault su una rete affidabile. La chiave è ed25519, salvata con permessi `0600` in appdata, e la chiave dell'host è fissata. Ogni comando verso l'host viene passato come argomenti separati, mai tramite una shell remota, e la connessione SSH usa `BatchMode` e un timeout di connessione.
