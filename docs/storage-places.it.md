# Luoghi di archiviazione

Un luogo di archiviazione è un posto in cui BombVault conserva i backup: una cartella su questo Unraid, una condivisione su un NAS, un bucket presso un provider cloud, un rest-server, un account SFTP o un Nextcloud. Colleghi ogni luogo una sola volta, in **Impostazioni, Archiviazione**, e credenziali, conservazione, protezione e posizione appartengono al luogo. I domini (container, VM, la flash, la configurazione di BombVault stesso, i set di file e i dataset ZFS) scelgono poi tra i luoghi: dove viene salvato ogni dominio e dove viene copiato. I dataset ZFS hanno la loro riga nella scheda Domini finché il dominio ZFS è attivo.

## Aggiungere un luogo {#add-a-place}

**Aggiungi luogo** apre una finestra con un riquadro per provider, in tre gruppi: archiviazione cloud, servizi autogestiti, e dispositivi NAS e questo server.

1. Scegli un riquadro e compila il suo modulo. Il pulsante con l'occhio mostra un segreto che hai digitato.
2. **Prova connessione** verifica il luogo e non crea nulla. Per la cartella di ogni dominio riporta cosa ha trovato: vuota o non ancora presente, già con un repository restic, oppure l'errore che l'ha fermata.
3. Dai un nome al luogo; il nome del provider è già inserito. Per un dispositivo che gestisci tu, rispondi a **Dove si trova il dispositivo?**. I provider cloud sono sempre in un altro sito, e una cartella su questo Unraid è sempre qui.
4. **Aggiungi** salva il luogo.

Un nuovo luogo non è ancora usato da nessun dominio. Sceglilo in **Salvato su** o **Copiato su** nella [scheda Domini](#domains), oppure sulla scheda di un elemento solo per quell'elemento.

## Cartelle {#folders}

Un luogo tiene una cartella per dominio: `container`, `vms`, `flash`, `config`, `files` e `zfs`, gli stessi nomi usati dalle posizioni di backup predefinite. Le cartelle sono elencate nei dettagli del luogo e lì si possono rinominare (vedi [Cambiare un indirizzo](#addresses)). Un dominio senza cartella in un luogo non può scegliere quel luogo.

Quando un dominio usa un luogo in entrambi i ruoli, il secondo ruolo riceve un suffisso e il primo mantiene la sua cartella. Un luogo che riceve già le copie di un dominio salva in `<folder>-direct` gli elementi inviati direttamente a lui; un luogo che salva già un dominio ne riceve le copie in `<folder>-copies`.

Alcuni luoghi sono essi stessi un repository restic: un indirizzo che conteneva già un repository quando il luogo è stato aggiunto, un repository con nome di una configurazione esistente, oppure una destinazione di copia alla radice di un bucket. Un luogo del genere non ha cartelle, tutti i domini ne condividono l'unico repository e non accetta un secondo ruolo. Per salvare altro presso lo stesso provider, collega un altro bucket o un'altra cartella come luogo a sé.

## Dettagli del luogo {#details}

Ogni luogo è una riga con il suo provider, l'uso che se ne fa e l'ultima prova o copia. **Prova** verifica ogni indirizzo del luogo, e **Dettagli** ne apre le impostazioni. Ogni modifica nei dettagli viene salvata mentre la fai.

- **Generale**: il nome, l'interruttore che attiva e disattiva il luogo, l'indirizzo e, per un dispositivo che gestisci tu, **Dove si trova il dispositivo?** (vedi [Fuori sede](#off-the-premises)).
- **Conservazione**: ultimi, giornalieri, settimanali e mensili, per ogni repository del luogo. Un nuovo luogo parte con le regole predefinite; un luogo con tutte le regole a zero non taglia mai.
- **Protezione**: l'interruttore **Append-only**. È l'altro lato a dover imporre l'append-only; con l'interruttore attivo, BombVault non pota né elimina mai nulla lì. Su un rest-server con append-only attivo, **Verifica append-only** esegue il test di manomissione su ogni percorso di dominio, copia attiva e repository del luogo e mostra un'unica risposta per l'intero luogo, *eliminazione rifiutata* o *eliminazione accettata* (vedi [Off-site e ripristino](offsite-recovery.md)). Solo i luoghi remoti hanno questa sezione, perché nulla su questa macchina può impedire che un repository locale venga eliminato.
- **Accesso**: le credenziali e, per S3, la classe di archiviazione. Un luogo che usa le credenziali condivise riceve un set proprio alla prima modifica. Un repository diretto del luogo che le nuove credenziali non riescono ad aprire mantiene quelle vecchie, e la risposta lo segnala. I luoghi di tipo cartella, SFTP e rclone non hanno questa sezione.
- **Limiti**: la velocità di upload e di download e il budget di crescita.
- **Cartelle**: un interruttore per dominio, con il nome della sua cartella. Un dominio disattivato qui non può scegliere il luogo.

Ridurre la conservazione chiede prima conferma e indica quanti elementi riguarda; disattivare l'append-only chiede prima conferma e indica quanti repository del luogo lo perdono. Disattivare un luogo disattiva ogni repository che contiene; un luogo in cui è salvato un dominio non si può disattivare.

## La scheda Domini {#domains}

La scheda ha una riga per dominio, con il suo calendario, dove viene salvato, dove viene copiato e le sue eccezioni.

- **Salvato su**: finché la posizione di backup del dominio non contiene backup, il luogo scelto diventa il luogo principale del dominio e la posizione si sposta lì. Quando contiene già dei backup, per container, VM e set di file la scelta diventa il valore predefinito per i nuovi elementi, che lo adottano al loro primo backup; gli elementi che hanno già dei backup restano dove sono, perché BombVault non sposta mai un backup. Per la flash e la configurazione di BombVault stesso si sposta il luogo principale, e i backup già scritti restano nel vecchio luogo.
- **Copiato su**: un chip per ogni luogo che può ricevere le copie del dominio. Spuntare un chip rende il luogo una destinazione di copia del dominio; la prima volta BombVault indica in anticipo quanti elementi e snapshot e quanti dati invia la prima esecuzione. Togliere la spunta ferma le nuove copie: le copie già presenti restano e invecchiano secondo la conservazione del luogo, e gli elementi con una scelta propria continuano a copiare lì. Togliere la spunta all'ultimo chip ferma tutte le copie, anche verso i luoghi aggiunti in seguito, finché non se ne spunta di nuovo uno. Un luogo disattivato compare come chip attenuato e non si può scegliere.
- **Eccezioni**: gli elementi con una scelta propria, in un elenco con i link alle loro schede.
- **Copia ora** esegue subito le copie del dominio.

Un dominio messo in pausa dopo una ricostruzione tramite Scopri mostra la pausa sulla sua riga, con **Conferma valore predefinito** (vedi [Collocazione per elemento](offsite-recovery.md#placement)).

## Cambiare un indirizzo {#addresses}

La cartella di un dominio si può cambiare nei dettagli del luogo, e così anche l'indirizzo di un luogo locale, per esempio dopo aver spostato a mano un repository su un altro disco. BombVault verifica ogni indirizzo toccato dalla modifica e la accetta quando ogni nuovo indirizzo è vuoto e al vecchio non era salvato nulla, oppure quando ogni nuovo indirizzo contiene lo stesso repository restic del vecchio. Qualsiasi altro caso viene rifiutato, con il numero di backup ancora presenti al vecchio indirizzo. Un luogo remoto mantiene il suo indirizzo; per eseguire il backup altrove, collega il nuovo indirizzo come luogo a sé.

BombVault costruisce l'elenco dei luoghi dal proprio database e non elenca mai un repository remoto per riempirlo; la prova viene eseguita solo quando cambi qualcosa.

## Rimuovere un luogo {#remove}

Un luogo si può rimuovere solo finché nulla lo usa: nessun dominio è salvato lì, nessun valore predefinito punta a esso, nessun elemento è salvato lì e nessun repository diretto al suo interno contiene elementi. Altrimenti il rifiuto elenca cosa lo trattiene. Rimuoverlo porta via anche le sue destinazioni di copia, e le sue credenziali proprie, a meno che non le usi una sorgente di prelievo o un altro luogo. Nello spazio di archiviazione stesso non viene eliminato nulla, e la conferma indica quante copie restano lì.

## Senza luogo {#without-a-place}

Un indirizzo che non rientra nella forma di un luogo più una cartella continua a funzionare ed è elencato in **Senza luogo**, con il suo indirizzo. Tra questi ci sono gli indirizzi nativi `b2:`, `gs:` e `swift:`. **Assegna a un luogo** collega una riga del genere a un luogo, dopo la stessa verifica di quando [cambi un indirizzo](#addresses). Una destinazione di copia senza luogo compare anche sulla riga del suo dominio, accanto ai chip, e continua a copiare. Lì una riga remota ha il suo interruttore **Append-only**, e disattivarlo chiede prima conferma indicando quanti elementi conservano backup a quell'indirizzo. Un repository diretto segue l'interruttore della sua destinazione.

## Fuori sede {#off-the-premises}

**Dove si trova il dispositivo?** ha due risposte: **Qui in casa** e **In un altro sito**. Una copia conta come sede a sé, per la riga 3-2-1 sulle schede e per i controlli off-site della Dashboard, solo quando il suo luogo si trova in un altro sito. Un secondo disco o un NAS nella stessa casa è una seconda copia, non una seconda sede. La risposta non cambia nessuna copia. I provider cloud sono sempre in un altro sito e una cartella su questo Unraid è sempre qui, quindi il modulo non lo chiede per loro; per ogni altro luogo, cambia la risposta nei suoi dettagli. Un luogo in un altro sito porta il contrassegno **Altro sito** sulla sua riga.

## Tipi di connessione

### Cartella su questo Unraid o un NAS {#kind-local}

L'indirizzo è un percorso sotto `/mnt`, scritto senza `/mnt`, per esempio `user/bombvault`, e la cartella di ogni dominio sta al di sotto: `user/bombvault/container`.

- **Cartella su questo Unraid** sceglie tra condivisioni, dischi e pool.
- **Synology**, **QNAP**, **TrueNAS**, **Un altro Unraid** e **Altra condivisione** scelgono da `/mnt/remotes`. Monta prima la condivisione su Unraid, per esempio con il plugin Unassigned Devices. Host Data deve essere montato Read/Write - Slave, altrimenti una condivisione che si monta dopo l'avvio di BombVault resta invisibile fino a un riavvio (vedi [Configurazione](configuration.md)).

Il selettore di cartelle crea una cartella nel punto in cui si trova con **Nuova cartella**. La prova verifica che la cartella sia vuota o assente e che BombVault possa scriverci.

### S3 {#kind-s3}

L'indirizzo è `s3:https://<endpoint>/<bucket>/<path>`, per esempio `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** richiede solo l'ID chiave e la chiave applicativa. BombVault chiede a B2 a quale bucket, endpoint S3 e cartella è limitata la chiave e costruisce l'indirizzo da questi dati. Se la chiave può raggiungere tutti i bucket, BombVault li propone tra cui scegliere.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** e **Vultr** chiedono la chiave e, dove il provider lo richiede, la regione, l'ID account o l'endpoint. BombVault compila l'endpoint ed elenca i bucket quando la chiave può elencarli; altrimenti digita il nome del bucket.
- **Google Cloud Storage** passa per la sua interfaccia S3 con una chiave HMAC, che si crea nelle impostazioni di Cloud Storage alla voce Interoperabilità. Un file di account di servizio qui non funziona.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** e **Altro servizio S3** chiedono l'indirizzo del servizio e una chiave.

La classe di archiviazione si imposta nei dettagli del luogo, limitata ai livelli che un ripristino può leggere senza scongelamento.

### rest-server {#kind-rest}

L'indirizzo è `rest:<url>/<user>`, per esempio `rest:https://nas.lan:8000/tower`. Il modulo chiede l'indirizzo del server, un utente e una password. Con `--private-repos` un utente può raggiungere solo i percorsi che iniziano con il proprio nome, quindi BombVault mette l'utente all'inizio, a meno che tu non digiti un altro percorso. Quando il server rifiuta un percorso fuori da quello dell'utente, l'errore lo dice.

Il modulo del rest-server contiene una ricetta pronta da incollare per un rest-server in modalità append-only con un utente per questo BombVault. **Mostra ricetta** crea una password, mostrata una sola volta, e fornisce una riga `docker run`, un file compose e un template Unraid, ciascuno con la riga `htpasswd` da mettere sul server; l'utente e la password finiscono direttamente nel modulo.

**Un altro BombVault** elenca sopra i propri campi le offerte aperte che altre istanze hanno inviato tramite Flotta. Accettarne una aggiunge un luogo che conserva solo le copie del dominio offerto, perché un'offerta porta un utente per quel solo dominio. Accettarla dalla pagina Flotta aggiunge lo stesso luogo.

### SFTP {#kind-sftp}

L'indirizzo è `sftp://<user>@<host>:<port>/<path>`, per esempio `sftp://bv@backup.lan:22/bombvault`. Il modulo chiede host, porta e utente e mostra la chiave pubblica di BombVault. Aggiungi quella chiave al file `~/.ssh/authorized_keys` dell'utente sul server; lì non serve installare nient'altro. BombVault accetta la chiave host del server al primo contatto e da allora in poi la verifica.

**Hetzner Storage Box** compila `<user>.your-storagebox.de` e la porta 23. Installa la chiave sulla Storage Box con il comando di Hetzner, che chiede una volta la password della Storage Box:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Il modulo chiede l'indirizzo del server, l'utente e una password per le applicazioni. Crea la password per le applicazioni nelle impostazioni di sicurezza dell'account, e inserisci l'ID utente anziché un indirizzo e-mail. BombVault costruisce il percorso WebDAV usato dal prodotto e passa la connessione a restic tramite le variabili d'ambiente di rclone, con la password nella forma offuscata di rclone. L'indirizzo è `rclone:bvp<id>:<path>`, dove `bvp<id>` è un remote che esiste solo in quell'ambiente; nella configurazione rclone non viene scritto nulla.

### Azure Blob {#kind-azure}

L'indirizzo è `azure:<container>:/<path>`. Il modulo chiede l'account di archiviazione e la sua chiave di accesso; dopo **Prova connessione** elenca i container dell'account tra cui scegliere, oppure digiti il nome di un container. BombVault passa l'account e la chiave a restic come `AZURE_ACCOUNT_NAME` e `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

L'indirizzo è `rclone:<remote>:<path>`. Il modulo elenca i remote della configurazione rclone di BombVault tra cui scegliere. Per sostituire quella configurazione, incolla un intero `rclone.conf` in **Configurazione rclone** e clicca **Salva configurazione**. Viene salvata subito e vale per ogni luogo rclone, che la finestra poi ne aggiunga uno o no.

## Copie tra luoghi con credenziali diverse {#different-credentials}

Un dominio salvato in un luogo remoto è la sorgente delle sue copie. `restic copy` gira con un solo ambiente, e BombVault aggiunge le credenziali della sorgente a quelle della destinazione quando le due non assegnano valori diversi alla stessa variabile. Un luogo Nextcloud e un luogo B2 usano variabili diverse, quindi un dominio salvato su Nextcloud può essere copiato su B2. Due account S3 o due utenti di rest-server avrebbero bisogno delle stesse variabili con valori diversi; restic non può accettarli entrambi, e il chip sulla scheda Domini dice che le credenziali non sono compatibili.
