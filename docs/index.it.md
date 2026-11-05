# BombVault

**I tuoi dati Unraid, sigillati in una cassaforte. Deposita un backup. Fai detonare un ripristino.**

![La panoramica: l'intestazione di stato, ciò che richiede attenzione, la protezione di ogni sezione, le destinazioni e la cronologia.](assets/screenshots/overview/home-dark.png#only-dark)
![La panoramica: l'intestazione di stato, ciò che richiede attenzione, la protezione di ogni sezione, le destinazioni e la cronologia.](assets/screenshots/overview/home-light.png#only-light)

BombVault è un'app web self-hosted per il **backup e il disaster recovery completo** su Unraid e su altri host Docker, come TrueNAS SCALE, ZimaOS o un semplice server Linux. Salva container Docker, VM KVM/libvirt, cartelle, dataset ZFS e la chiavetta flash di Unraid. Gira come un unico container Docker multi-arch, ti offre un'interfaccia web con tema scuro, chiaro o del sistema e gestisce l'intero ciclo di vita: backup, pianificazione, verifica e ripristino.

I ripristini sono automatici. Su Unraid i container riappaiono nella scheda Docker esattamente come prima, e le VM vengono ridefinite nel VM Manager con i loro dischi e la NVRAM UEFI ricollegati. Nessuna reinstallazione manuale, nessuna riconfigurazione, nessun dramma. Su TrueNAS un container ripristinato accanto all'originale gira come un normale container Docker che la schermata Apps non elenca.

Basato su [restic](https://restic.net), quindi ogni backup è deduplicato, incrementale e cifrato di default.

!!! note "Tieni al sicuro la tua APP_KEY"
    BombVault deriva la password del repository restic da un segreto di 32 byte chiamato `APP_KEY`. Perderlo, insieme al kit di ripristino, rende i backup cifrati irrecuperabili. Generane uno con `openssl rand -hex 32` e conservalo in un luogo sicuro. Vedi [Configurazione](configuration.md).

## Cosa protegge BombVault {#what-bombvault-protects}

Ogni container, VM, set di cartelle, la chiavetta flash, ogni dataset ZFS e le impostazioni di BombVault stesso sono un **elemento** con una pagina propria. Gli elementi dello stesso tipo formano una **sezione**, che attivi in **Impostazioni, Generale**.

| Sezione | Cosa viene salvato |
|---|---|
| **Container** | La directory appdata più la definizione del container (immagine, variabili d'ambiente, porte, label, volumi). |
| **VM** | Le immagini disco della VM, la definizione XML e la NVRAM UEFI, salvate via SSH (nessun mount libvirt). |
| **Flash** | L'intera chiavetta USB (`/boot`): sistema operativo, licenza, configurazione dell'array, condivisioni, rete e configurazione dei plugin. |
| **Auto-backup** | Il `/config` di BombVault stesso: il suo database delle impostazioni, le credenziali delle sue destinazioni, la coppia di chiavi SSH di libvirt, la chiave e le chiavi host fissate dei server ZFS e i certificati. |
| **Cartelle** | Qualsiasi cartella del server come set di cartelle, ciascuno con pattern di esclusione opzionali. |
| **Dataset ZFS** | Un dataset con tutti i dataset sottostanti, letto da uno snapshot ZFS e salvato come una cartella. Vedi [Dataset ZFS](zfs-datasets.md). |

## Il ripristino è la star {#restore-is-the-star}

Dopo aver ricopiato i dati dallo snapshot restic, BombVault riesegue la definizione del container salvata contro l'API Docker, così il container riappare nella scheda Docker di Unraid come se ci fosse sempre stato (stessa immagine, stesse impostazioni, stessi mapping delle porte). Le VM vengono ridefinite via SSH tramite il loro XML e i loro dischi e la NVRAM UEFI vengono ricollegati, anche dopo che la VM è stata eliminata.

Una sola finestra ripristina ogni tipo di elemento. Di norma è scelto **Accanto all'originale**, che riporta il backup come copia e lascia l'originale com'è. **Sostituisci l'originale** esegue prima il backup dello stato attuale, così un ripristino si può annullare. Una cartella o un nuovo dataset ZFS sono le altre destinazioni.

Quando un backup ferma i container dipendenti, questi tornano nell'ordine giusto: BombVault li riavvia nel loro ordine `depends_on` di Compose e attende che ciascuno risulti sano prima di avviare quelli che dipendono da esso, così nulla parte prima di un database o di un gateway non ancora attivo. Vedi [Ripristino](restore.md) e [Comandi e altri container](backups.md#commands).

## Come funziona {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault usa il socket Docker per fermare i container prima di un backup e per ricrearli dopo un ripristino. Per le VM esegue `virsh` sull'host via SSH (`qemu+ssh://`) per spegnere una VM in modo ordinato o fare uno snapshot a caldo. Non monta mai un percorso libvirt, quindi non può intralciare il VM Manager dell'host.

BombVault è il livello di orchestrazione e interfaccia, non il motore di archiviazione. Tutto il movimento effettivo dei dati passa attraverso restic.

## Avvio rapido {#quick-start}

Sei nuovo? Vai a **[Primi passi](getting-started.md)** per installare BombVault ed eseguire il tuo primo backup, poi imposta **[Off-site e ripristino](offsite-recovery.md)**. Gli **[esempi pratici](guides.md)** ti accompagnano dall'inizio alla fine nei compiti più comuni, la pagina **[Impostazioni](settings.md)** elenca ogni scheda e **[Funzionalità](features.md)** è il breve catalogo di tutto ciò che BombVault sa fare.

Ogni sezione può copiare su più destinazioni contemporaneamente, un BombVault associato può tenere le tue copie off-site in un server che rifiuta le cancellazioni, e la **Procedura guidata di emergenza** riporta in vita un server perso dal suo kit di ripristino, tramite il tuo gruppo o con la chiave digitata. Per portare la configurazione su una nuova macchina usa la scheda **Esporta / importa impostazioni**. Vedi [Off-site e ripristino](offsite-recovery.md) e [Configurazione](configuration.md#portable-settings-export-and-import).

L'**[App Android](android.md)** porta tutti i server del tuo gruppo sul telefono, con il registro attività di tutti quanti in un'unica schermata.

La documentazione è disponibile in 26 lingue e l'interfaccia in 42. Le pagine non ancora tradotte compaiono in inglese.

## Parole usate qui {#glossary}

Alcune parole hanno in BombVault, e in queste pagine, un significato fisso.

| Parola | Significato |
|---|---|
| **Elemento** | Una cosa che BombVault salva: un container, una VM, un set di cartelle, la chiavetta flash, un dataset ZFS con quelli sottostanti o le sue stesse impostazioni. Ognuno ha una pagina propria. |
| **Sezione** | Tutti gli elementi dello stesso tipo, per esempio Container. Le sezioni si attivano in **Impostazioni, Generale**. |
| **Destinazione** | Un posto in cui vanno i backup: una cartella di questo server, una condivisione, un server tuo o un servizio cloud. Vedi [Destinazioni](offsite-recovery.md#destinations). |
| **Locale** | La destinazione su questo server che contiene le cartelle delle sezioni. |
| **Cartella propria** | Il repository in cui una sezione scrive per primo i suoi backup. Di default si trova su questo server e può stare in una destinazione. Vedi [La cartella propria di una sezione](offsite-recovery.md#own-folder). |
| **Copia** | Un backup copiato dalla cartella propria su un'altra destinazione con `restic copy`. |
| **Repository diretto** | Un repository in una posizione off-site su cui gli elementi salvano direttamente, senza una copia locale nel mezzo. Vedi [Repository diretti](offsite-recovery.md#direct-repositories). |
| **Credenziali** | La chiave o la password salvata per un servizio S3 o un rest-server. Una di esse è quella predefinita. Vedi [Credenziali](offsite-recovery.md#sign-ins). |
| **Destinazioni di un elemento** | Quali destinazioni ricevono i backup di un elemento, impostate sull'elemento o come valore predefinito della sua sezione. Vedi [Destinazioni per elemento](offsite-recovery.md#placement). |
| **Replica** | Una copia identica a livello di blocchi di un elemento ZFS su un secondo server ZFS, fatta con `zfs send`. Sta accanto al backup e non lo sostituisce. Vedi [Replica](zfs-datasets.md#replica). |
| **Ruolo** | Qualcosa che un'istanza del tuo gruppo fa per un'altra: Ricevitore, Prelevatore, Server ZFS o Custode della chiave. Vedi [Istanze e ruoli](offsite-recovery.md#roles). |

## Riconoscimenti {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** di [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) ha dato a BombVault la sua idea di partenza: backup con un clic e reinstallazione automatica dei container Docker. BombVault è un'implementazione separata, basata su Go e restic, che estende l'idea alle VM, alla flash e ad altro ancora.
- **[restic](https://restic.net/)** è il motore di backup veloce, sicuro e con deduplicazione che BombVault pilota.
- **[rclone](https://rclone.org/)** fornisce i backend cloud.
- La maggior parte dei glifi sui pulsanti proviene dal set gratuito Core Solid di **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [sorgente](https://github.com/webalys-hq/streamline-vectors)). Gli altri provengono da Font Awesome Free, Material Design Icons, Simple Icons e Tabler Icons, oppure sono stati disegnati per il progetto.

## Licenza {#license}

Copyright (C) 2026 Junker der Provinz. BombVault è software libero rilasciato sotto la **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Puoi eseguirlo, studiarlo, condividerlo e modificarlo. Se lo distribuisci, o se esegui una versione modificata come servizio di rete, devi pubblicare il tuo codice sorgente con la stessa licenza e mantenere le note di copyright e di attribuzione esistenti.

Il nome e il marchio non sono concessi in licenza. L'AGPL copre solo il codice sorgente: "BombVault", il suo logo e la sua identità visiva restano riservati, quindi un fork deve usare un nome e un'identità visiva propri e non può presentarsi come BombVault.

## Link {#links}

- **Codice sorgente:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Thread di supporto Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issue:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Controllo dell'host equivalente a root"
    Tramite il socket Docker BombVault può fermare, rimuovere e ricreare container e leggere e scrivere appdata, e per il backup delle VM accede all'host via SSH per eseguire `virsh`. Chiunque possa raggiungere la sua interfaccia web ha di fatto accesso root all'host. Esegui BombVault solo su una rete affidabile e non esposta, e imposta una password di accesso in **Impostazioni, Sicurezza**. Il kit di ripristino, i depositi della chiave, lo ZIP della flash, l'esportazione di un backup della flash o di un auto-backup e l'eliminazione dei backup di una destinazione ne hanno bisogno. Vedi [Configurazione](configuration.md#security-model) per il modello di sicurezza completo.
