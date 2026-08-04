# BombVault

**I tuoi dati Unraid, sigillati in una cassaforte. Deposita un backup. Fai detonare un ripristino.**

BombVault è un'app web self-hosted e nativa per Unraid per il **backup e il disaster recovery completo** dei tuoi container Docker e delle tue VM KVM/libvirt. Gira come un unico container Docker multi-arch, ti offre una moderna interfaccia web scura e gestisce l'intero ciclo di vita: backup, pianificazione, verifica e ripristino.

I ripristini sono automatici. I container riappaiono nella scheda Docker di Unraid esattamente come prima, e le VM vengono ridefinite nel VM Manager con i loro dischi e la NVRAM UEFI ricollegati. Nessuna reinstallazione manuale, nessuna riconfigurazione, nessun dramma.

Basato su [restic](https://restic.net), quindi ogni backup è deduplicato, incrementale e sempre cifrato.

!!! note "Tieni al sicuro la tua APP_KEY"
    BombVault deriva la password del repository restic da un segreto di 32 byte chiamato `APP_KEY`. Perderlo rende i backup cifrati irrecuperabili. Generane uno con `openssl rand -hex 32` e conservalo in un luogo sicuro. Vedi [Configurazione](configuration.md).

## Cosa protegge BombVault

| Dominio | Cosa viene salvato |
|---|---|
| **Container Docker** | La directory appdata più la definizione del container (immagine, variabili d'ambiente, porte, label, volumi). |
| **VM KVM / libvirt** | L'immagine (o le immagini) disco della VM, la definizione XML e la NVRAM UEFI, di cui viene eseguito il backup via SSH (nessun mount libvirt). |
| **Flash Unraid** | L'intera chiavetta USB flash (`/boot`): SO, licenza, configurazione dell'array, condivisioni, rete e configurazione dei plugin. |
| **Configurazione dell'app** | Il `/config` di BombVault stesso: il suo database delle impostazioni, le credenziali off-site e la coppia di chiavi SSH di libvirt. |
| **File e cartelle** | **Set di file** con nome, qualsiasi cartella sul server, ciascuno con pattern di esclusione opzionali per set. |

## Il ripristino è la star

Dopo aver ricopiato i dati dallo snapshot restic, BombVault riesegue la definizione del container salvata contro l'API Docker, così il container riappare nella scheda Docker di Unraid come se ci fosse sempre stato (stessa immagine, stesse impostazioni, stessi mapping delle porte). Le VM vengono ridefinite via SSH tramite il loro XML e i loro dischi e la NVRAM UEFI vengono ricollegati, anche dopo che la VM è stata eliminata.

Quando un backup ferma i container dipendenti, questi tornano nell'ordine giusto: BombVault li riavvia nel loro ordine `depends_on` di Compose e attende che ciascuno segnali di essere sano prima di avviare quelli che dipendono da esso, così nulla si avvia prima di un database o di un gateway che non è ancora attivo. Vedi [Funzionalità](features.md).

## Come funziona

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault è il livello di orchestrazione e interfaccia, non il motore di archiviazione. Tutto il movimento effettivo dei dati passa attraverso restic.

## Avvio rapido

Sei nuovo qui? Vai a **[Primi passi](getting-started.md)** per installare BombVault su Unraid tramite Community Applications ed eseguire il tuo primo backup. Poi esplora l'insieme completo delle **[Funzionalità](features.md)**, regola la tua **[Configurazione](configuration.md)** e imposta **[Off-site e ripristino](offsite-recovery.md)**.

L'off-site può diramarsi verso più destinazioni per dominio contemporaneamente, una **dashboard ricevente** in sola lettura monitora quelle copie sulla macchina che le riceve, e puoi portare l'intera configurazione su una nuova macchina con la scheda **Esporta e importa impostazioni**. Vedi [Off-site e ripristino](offsite-recovery.md) e [Configurazione](configuration.md#portable-settings-export-and-import).

## Link

- **Codice sorgente:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Thread di supporto Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issue:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Controllo dell'host equivalente a root"
    Tramite il socket Docker BombVault può fermare, rimuovere e ricreare container e leggere/scrivere appdata, e per il backup delle VM accede all'host via SSH per eseguire `virsh`. Chiunque possa raggiungere la sua interfaccia web ha di fatto accesso root sull'host. Esegui BombVault solo su una rete affidabile e non esposta, e abilita il gate password opzionale (Impostazioni, Sicurezza) una volta che sono in uso backup off-site o immutabili. Vedi [Configurazione](configuration.md) per il modello di sicurezza completo.
