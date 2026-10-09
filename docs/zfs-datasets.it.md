# Dataset ZFS

La pagina **ZFS** esegue il backup di dataset ZFS. Un elemento è un dataset insieme a tutti i dataset che stanno sotto di esso. Per ogni backup BombVault prende un solo snapshot ZFS dell'intero albero, così ogni dataset al suo interno viene catturato nello stesso istante. Poi legge i file di ogni dataset da quello snapshot, li salva con restic nello stesso modo in cui salva una cartella e rimuove subito lo snapshot. I backup sono deduplicati, puoi sfogliare ciascuno di essi e ripristinare singoli file.

Il backup non invia mai uno stream a restic e non riporta mai indietro un dataset. BombVault distrugge solo gli snapshot che ha creato lui stesso. La [replica](#replica) facoltativa è l'unico punto che usa `zfs send`: copia i dataset su un secondo server ZFS e non tocca il backup.

## Requisiti {#requirements}

- **Il collegamento SSH a questo server.** I dataset ZFS usano la stessa chiave, lo stesso host e lo stesso utente dei backup delle VM. Se i backup delle VM funzionano già, funziona anche questo. Altrimenti segui la [guida al backup VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) su GitHub. I campi del template si chiamano **Host SSH: Address**, **Host SSH: Port** e **Host SSH: User**.
- **Il comando `zfs` su quell'host.** Unraid 6.12 e successivi e TrueNAS SCALE lo hanno.
- **Host Data mappato come `/mnt` con Access Mode Read/Write - Slave.** È il valore predefinito del template. Lo snapshot di un dataset compare nella cartella `.zfs/snapshot` del dataset solo dopo l'avvio di BombVault, quindi il container deve ricevere i mount che l'host crea in seguito.
- **I dataset montati sotto `/mnt`.** Su Unraid i pool si trovano in `/mnt/<pool>`, quindi è già così.

Attiva il dominio in **Impostazioni, Generale** (Dataset ZFS). La pagina ZFS mostra allora una scheda **Connessione a questo server**. Prova il collegamento SSH, indica l'utente e l'host con cui si collega e dice cosa manca quando manca qualcosa. Il controllo dell'integrazione con l'host (`/spike`) mostra lo stesso risultato.

## Elementi e dataset figli {#items-and-children}

Apri **Aggiungi dataset** nella pagina ZFS. L'elenco arriva dal server. Scegli il dataset più in alto di ciò che vuoi salvare, per esempio `cache/appdata`, e l'elemento copre quel dataset e ogni dataset sotto di esso.

- **I nuovi dataset figli entrano da soli.** Un dataset creato più tardi sotto l'elemento viene salvato con l'esecuzione successiva, che lo segnala come nuovo. Il suo primo backup lo legge per intero una volta; poi vengono lette solo le modifiche.
- **Puoi escludere singoli figli.** Disattiva un figlio nelle impostazioni dell'elemento e viene escluso insieme a tutto ciò che sta sotto. Un figlio escluso che non esiste più sul server viene segnalato come tale e si può togliere dall'elenco.
- **I figli illeggibili vengono saltati, mai in silenzio.** L'esecuzione li elenca, l'elemento mostra quanti sono stati saltati e la scheda della copertura nella dashboard conta ciascuno come non protetto. L'esecuzione salva comunque tutto il resto e non fallisce per un figlio saltato. I motivi sono nella [tabella dei codici di motivo](#reason-codes): un dataset non montato, con `canmount=off`, con un mountpoint `legacy` o senza mountpoint, una chiave di cifratura non caricata, l'accesso agli snapshot disattivato o un mountpoint che BombVault non vede.
- **Un dataset saltato non si trascina dietro i figli.** Un dataset con `canmount=off` che contiene solo altri dataset viene saltato (mostrato come "solo struttura") e i suoi figli montati vengono salvati. Un dataset cifrato con la chiave non caricata viene saltato insieme ai figli che condividono la sua chiave.
- **I figli che sono dischi di VM o dati di sistema partono disattivati** nella finestra di aggiunta, con il motivo accanto all'interruttore. Aggiungere un intero pool chiede una conferma che elenca cosa contiene.

### Volumi {#volumes}

Un volume (zvol) contiene un disco virtuale invece di file, e la pagina ZFS non ne salva mai uno.

- Un volume usato da una VM viene salvato con quella VM nella pagina **VM**.
- Un volume che nessuna VM usa (un extent iSCSI, un disco scollegato) **non viene salvato da BombVault**. La finestra di aggiunta e la pagina ZFS contano questi volumi e lo dicono. Una versione successiva li salverà.

I volumi nell'albero di un elemento vengono saltati e nominati a ogni esecuzione.

### Lo storage di Docker {#docker-storage}

Con il driver di storage ZFS di Docker, ogni layer di immagine è un dataset con mountpoint `legacy`. La finestra di aggiunta li raggruppa in una riga per genitore. Un albero che contiene più di 20 di questi dataset non può diventare un elemento: finché ne esiste uno snapshot, Docker non può rimuovere layer di immagine. Aggiungi invece i dataset che stanno sotto, per esempio `appdata`.

### Gli elementi non si sovrappongono mai {#overlap}

Un dataset può appartenere a un solo elemento. BombVault rifiuta un nuovo elemento che si trova dentro uno esistente o che ne conterrebbe uno. Per unire più elementi figli in un elemento genitore, elimina prima gli elementi figli scegliendo di tenere i loro backup, poi aggiungi il genitore. Ogni dataset mantiene la sua cronologia con il proprio nome, quindi il backup successivo riprende da dove si erano fermati i vecchi elementi e non rilegge tutto.

## Fermare container ed eseguire comandi attorno allo snapshot {#consistency}

Uno snapshot di un database in esecuzione è come un'improvvisa mancanza di corrente: di solito il database si riprende, ma deve farlo. Ogni elemento può fare due cose al riguardo, ed entrambe coprono solo l'istante dello snapshot, non l'intero backup.

- **Ferma questi container per lo snapshot.** BombVault ferma i container elencati, prende lo snapshot e li riavvia subito. I container di uno stesso livello di dipendenze si fermano in parallelo, prima i dipendenti, così l'intera finestra dura di solito pochi secondi; l'esecuzione mostra quanto è durata. Il backup legge poi lo snapshot congelato mentre le app sono già di nuovo in esecuzione. Vengono fermati solo i container che erano in esecuzione.
- **Un comando prima e dopo lo snapshot.** Viene eseguito dentro un container a tua scelta, per esempio per esportare un database nel dataset subito prima dello snapshot, senza fermare nulla. Se il comando prima dello snapshot fallisce, il backup fallisce e non viene preso alcuno snapshot. Un comando dopo lo snapshot che fallisce viene mostrato sull'esecuzione ma non fa fallire il backup.

Cosa succede quando qualcosa va storto:

- Se un container non si può fermare, BombVault riavvia quelli che ha già fermato e il backup fallisce indicando il container. Non ripiega mai su uno snapshot di app in esecuzione.
- L'arresto aspetta che finisca un backup di container in corso (fino a 30 minuti per un'esecuzione manuale, fino al limite di tempo del backup per una pianificata), così i due non fermano e avviano mai lo stesso container nello stesso momento.
- Prima che si fermi il primo container, BombVault annota quali ferma. Se BombVault viene terminato dentro la finestra, riavvia quei container al suo avvio successivo, invia una notifica e l'elemento mostra una nota rossa per ogni container che non è riuscito ad avviare.

I dump automatici dei database (vedi [Funzionalità](features.md)) girano con il backup proprio di un container nella pagina **Container**, non con un elemento ZFS. Un database il cui container viene salvato solo tramite il suo dataset non riceve dump, quindi dagli qui un comando.

Un container può stare in questo elenco e nella pagina **Container** allo stesso tempo. I suoi dati vengono allora salvati due volte, in due repository, e il **Backup totale** lo ferma due volte. L'elemento lo segnala.

## Ripristino {#restore}

Apri **Backup** sull'elemento, scegli il backup, poi il dataset. Per impostazione predefinita è il dataset superiore dell'elemento.

- **Ripristina dentro il dataset.** I file del backup vengono scritti nel mountpoint del dataset. I file con lo stesso nome vengono sovrascritti, gli altri restano. Il dataset non viene mai riportato indietro né sostituito. BombVault controlla che il dataset sia montato, visibile e scrivibile, una volta prima di iniziare e di nuovo subito prima di scrivere. Dove al suo interno è montato un dataset figlio, non viene scritto nulla: il figlio mantiene i suoi file, il proprietario e i permessi e viene ripristinato dal proprio backup.
- **Ripristina in una cartella.** Scegli una cartella sotto `/mnt`. BombVault controlla che la cartella sia su un pool o una condivisione montati e che ci sia abbastanza spazio libero. Funziona senza il collegamento SSH e per dataset che non esistono più.
- **In un nuovo dataset.** Indica un dataset che non esiste ancora. BombVault lo crea con le proprietà ZFS salvate nel backup e ripristina al suo interno, vedi [Ripristinare come nuovo dataset](#new-dataset).
- **Scegli i file** (avanzato): riscrivere nel dataset solo i file e le cartelle che scegli.
- **Tutti i dataset di questo backup** (avanzato): ogni dataset dell'albero nella propria sottocartella della cartella che scegli. I dataset saltati in quel backup vengono nominati.
- **Da un altro server:** la pagina **Ripristino** ripristina dal repository di un altro BombVault, sempre in una cartella: tutti i dataset di un backup, ciascuno nella propria sottocartella, oppure un dataset dell'albero, intero o solo i file scelti.

L'elenco dei container da fermare dell'elemento viene proposto anche per un ripristino dentro il dataset. Quei container restano fermi per tutto il ripristino, e nel frattempo i backup dei container aspettano.

### Lo snapshot di sicurezza {#safety-snapshot}

Prima di scrivere in un dataset, BombVault prende uno snapshot ZFS di quel solo dataset, chiamato `bombvault-prerestore-<ora>`. È attivo per impostazione predefinita; disattivarlo richiede una seconda conferma. Se non si riesce a prendere lo snapshot, non viene ripristinato nulla.

BombVault non elimina mai da solo uno snapshot di sicurezza. L'elemento li elenca con età e dimensione, ciascuno con l'azione **Elimina**, e avvisa quando il più vecchio ha più di 30 giorni, perché trattiene sul pool dati eliminati e modificati.

Per tornare indietro dopo un ripristino, copia singoli file da `.zfs/snapshot/bombvault-prerestore-<ora>` dentro il dataset. `zfs rollback <dataset>@bombvault-prerestore-<ora>` funziona solo finché è lo snapshot più recente di quel dataset. `zfs rollback -r` elimina ogni snapshot più recente, compresi quelli automatici.

### Ripristinare come nuovo dataset {#new-dataset}

BombVault salva a ogni backup le proprietà ZFS impostate localmente di ogni dataset: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity e le tue proprietà utente. I valori ereditati e di sola lettura restano fuori, perché tornano da soli. I backup di prima che BombVault le salvasse non ne hanno.

- **In un nuovo dataset** esegue `zfs create` con ogni proprietà salvata. casesensitivity, normalization e utf8only si possono impostare solo così. Quote e prenotazioni vengono impostate dopo i file, così non possono rifiutarli. Il punto di montaggio resta fuori perché la copia non si scontri con l'originale, e così `canmount`, `readonly` e la cifratura, perché il ripristino possa scrivere. Un nuovo dataset sotto uno cifrato ne eredita la cifratura. Il dataset superiore deve esistere. Se qualcosa fallisce dopo la creazione, il nuovo dataset resta sul server, perché BombVault non distrugge mai un dataset.
- **Ripristina dentro il dataset** mostra le proprietà salvate accanto al ripristino. **Imposta anche queste proprietà** imposta quelle che un dataset esistente accetta ancora, prima di scrivere qualsiasi file. Quote e prenotazioni vengono impostate dopo i file, così non possono rifiutarli. Senza questo interruttore il dataset mantiene le sue impostazioni.

## Cosa c'è nel backup {#contents}

Nel backup: i file e le cartelle di ogni dataset salvato, con proprietario, permessi, marcature temporali e attributi estesi così come li salva restic, e le proprietà ZFS impostate localmente di ogni dataset.

Non nel backup:

- il proprietario e i permessi della cartella superiore di ogni dataset in sé (tutto ciò che sta sotto è incluso). Un ripristino dentro il dataset lascia com'è la cartella superiore esistente, un ripristino in una cartella la crea con permessi `0755`;
- gli snapshot ZFS esistenti;
- i figli saltati o esclusi;
- i volumi.

Per ripristinare su un nuovo pool, crea il pool e ripristina ogni dataset in un nuovo dataset. Non è ancora stato verificato se le ACL NFSv4, come TrueNAS le usa sui dataset SMB, tornino come ti aspetti, quindi prova un ripristino sui tuoi dati prima di farci affidamento.

## Dataset cifrati {#encryption}

Un dataset cifrato viene salvato solo mentre la sua chiave è caricata. Altrimenti viene saltato con un avviso; carica la chiave con `zfs load-key` e monta il dataset. BombVault legge i dati decifrati e li salva nel repository di restic, che è cifrato. Se hai disattivato la cifratura in BombVault, quel repository non lo è.

## Replica {#replica}

Una replica è una copia dei dataset di un elemento su un secondo server ZFS. BombVault la tiene aggiornata con `zfs send` e `zfs receive`. La prima esecuzione invia tutto, poi viaggiano solo i blocchi modificati. Sull'altro server puoi montare subito la copia.

Una replica non sostituisce mai il backup. Le versioni più vecchie, i singoli file e il controllo arrivano sempre dai backup, e la replica tiene solo tanti snapshot quanti ne imposti. Una replica aggiornata conta come copia off-site, ma un elemento con una replica e senza backup resta arancione.

Attivala nella scheda **Replica** delle impostazioni dell'elemento. Lì scegli dove va la replica, quando viene eseguita (**After every backup** o **Own plan**) e quanti snapshot restano sulla destinazione. La scheda elenca ogni dataset e ogni volume con il suo stato, e **Replicate now** avvia un'esecuzione. Un'esecuzione di replica ha un lock proprio, così un primo trasferimento lungo non blocca mai i backup.

### Invio a un server ZFS {#replica-push}

Qualsiasi macchina con ZFS e SSH può ricevere, per esempio un secondo Unraid o un TrueNAS. BombVault non deve girare lì.

1. Apri **Istanze, Server ZFS** e clicca su **Aggiungi server ZFS**.
2. Inserisci indirizzo, utente e porta. La finestra mostra la chiave pubblica di BombVault. Aggiungila al file `~/.ssh/authorized_keys` dell'utente sul server. Su Unraid si trova in **Settings, Users, root, SSH keys**.
3. Prova la connessione. La finestra elenca poi i pool del server. Scegline uno e imposta la radice, che di default è `<pool>/bombvault-replica`.
4. Scegli il nuovo server nella scheda **Replica** dell'elemento.

Con root non serve altro. Un utente dedicato ha bisogno di questi permessi sul pool della destinazione, che la finestra mostra a sua volta:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Sulla sorgente un utente dello stesso tipo ha bisogno di questi sul dataset superiore dell'elemento:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

In questa direzione il BombVault che tiene l'elemento tiene anche una chiave che può scrivere sul server.

### Invio a un'istanza abbinata {#replica-receive}

Un BombVault abbinato può ricevere la replica da sé. Nessuno ottiene accesso SSH all'altro host e nessuna chiave finisce in un file `authorized_keys`.

1. Nella scheda **Replica** dell'elemento scegli l'istanza abbinata come destinazione. La scheda mostra **In attesa di approvazione** finché non risponde.
2. Sull'istanza che riceve apri **Instances**, poi **Ricevi**. La scheda **ZFS** elenca la richiesta. Scegli il pool e la radice su quel server e quanti snapshot restano, poi premi **Allow** o **Decline**.
3. Dopo **Allow** la sorgente invia secondo il proprio piano, come per qualsiasi altra destinazione.

L'istanza che riceve accetta solo ciò che l'approvazione copre: i dataset dell'elemento, nella propria radice. Esegue `zfs receive` da sola, e la sorgente non ha modo di eliminare o ripristinare nulla lì. Così la copia sopravvive a una sorgente di cui qualcuno ha preso il controllo. L'istanza che riceve tiene la propria conservazione. La sorgente suggerisce una regola solo quando fa la richiesta.

**Revoke access** sull'istanza che riceve termina l'approvazione in qualsiasi momento e lo comunica alla sorgente, che allora mostra che l'approvazione è stata ritirata e si ferma. Ciò che l'istanza che riceve ha già resta lì. Una richiesta rifiutata resta rifiutata. Altri dataset, o una richiesta dopo una revoca, chiedono di nuovo.

### Dove finiscono i dati {#replica-target}

Ogni dataset finisce in `<root>/<server>/<pool>/<path>`. La cartella del server è il nome dell'istanza sorgente, fissato al primo trasferimento, così due server con lo stesso nome di pool non si ostacolano mai. Per esempio, `cache/appdata` di un server chiamato `tower` finisce in `backup/bombvault-replica/tower/cache/appdata`.

La copia sulla destinazione è di sola lettura e non montata, quindi non copre mai nulla su quel server. Le proprietà ZFS viaggiano con essa, tranne il mountpoint e `sharenfs` e `sharesmb`. Anche le riserve `reservation` e `refreservation` restano indietro, quindi la copia occupa solo lo spazio di cui hanno bisogno i suoi dati.

### Cosa entra {#replica-contents}

Entra tutto ciò che l'elemento salva, e anche i volumi sotto di esso, che il backup salta. Un dataset figlio che hai disattivato nell'elemento resta fuori. Tutti i dataset di un'esecuzione vengono da un solo snapshot, come nel backup.

### Quanto restano gli snapshot {#replica-retention}

Sulla destinazione una nuova replica tiene 7 snapshot giornalieri e 3 settimanali. Scegli invece **Short**, **Balanced** o **Long**, oppure imposta **Custom values**. Lì vengono rimossi solo gli snapshot chiamati `bombvault-replica-<14 cifre>`, e mai il più recente che i due lati condividono.

Sulla sorgente BombVault tiene solo l'ultimo snapshot di replica, più un bookmark per ogni stato inviato. I bookmark non occupano spazio. Il trasferimento successivo parte da quelli.

### Dataset cifrati in una replica {#replica-encryption}

Un dataset cifrato viene inviato raw. Resta cifrato sulla destinazione, e la destinazione non vede mai la chiave. Conserva bene la chiave: ti serve per aprire la copia dopo un ripristino, e una replica senza di essa è illeggibile.

### Usare la replica {#replica-use}

Apri la scheda **Backup** dell'elemento e clicca sulla riga della replica nella scheda **Storage locations**. Il pannello elenca gli snapshot sulla destinazione e mostra i comandi con i tuoi nomi reali.

Per guardare uno stato vecchio, clona uno snapshot sulla destinazione. Un clone non occupa spazio finché qualcosa non cambia, e la replica resta intatta:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Se la sorgente si guasta, trasforma la copia in un dataset normale e scrivibile sulla destinazione:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

Dopo di che BombVault smette di replicare su quel dataset, finché non avvii una nuova prima esecuzione.

Per riportare uno stato sulla sorgente, premi **Bring back as a new dataset** nel pannello. BombVault invia lo snapshot in un nuovo dataset accanto all'originale, chiamato `<dataset>-bombvault-restore-` più un timestamp. Non scrive mai sopra l'originale.

## Snapshot rimasti {#leftover-snapshots}

Lo snapshot di un backup si chiama `<dataset>@bombvault-<14 cifre>`, per esempio `cache/appdata@bombvault-20260924021500` (UTC). BombVault lo rimuove subito dopo il backup. Se non ci riesce, per esempio perché il dataset è occupato o BombVault è stato fermato, BombVault lo rimuove:

- prima del backup successivo di quell'elemento,
- all'avvio di BombVault, per ogni elemento, anche con il dominio disattivato,
- quando elimini l'elemento,
- quando premi **Rimuovi ora** sull'elemento, che mostra quanti ne restano.

Vengono rimossi solo i nomi che corrispondono esattamente a `bombvault-` più 14 cifre. Gli snapshot di sicurezza, i tuoi snapshot e quelli automatici non vengono mai toccati. Per rimuoverne uno a mano:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Lo snapshot di una replica si chiama `<dataset>@bombvault-replica-<14 cifre>` e non è un residuo. Resta sulla sorgente finché la prossima esecuzione di replica non lo sostituisce, e sulla destinazione finché la conservazione lo tiene. Lo spazzino non lo tocca mai, perché corrisponde solo a `bombvault-` seguito da esattamente 14 cifre.

## Anomalie {#anomalies}

Un figlio che è stato svuotato cambia appena il totale di un albero grande, quindi il rilevamento delle anomalie osserva ogni dataset di un elemento a sé: la sua dimensione, il numero di file, i dati nuovi e il tempo di restic hanno ciascuno la propria cronologia. Quella cronologia appartiene al nome del dataset, quindi resta quando in seguito l'albero viene salvato da un altro elemento.

Un dataset che l'esecuzione precedente ha salvato e che questa non è riuscita a leggere conta come svuotato, purché la selezione dell'elemento non sia cambiata. Questo copre una chiave non caricata, un dataset non montato e uno sparito dall'albero. Un figlio che escludi tu stesso cambia la selezione, quindi la sua cronologia riparte da zero. Finché un rilevamento di dati persi è aperto, la conservazione tiene i vecchi backup di quel solo dataset e sfoltisce il resto dell'albero come al solito.

Nella pagina **Anomalie** ogni dataset ha una riga propria nel pannello dell'elemento, che si apre con **Monitoraggio** sulla scheda dell'elemento o, quando per l'elemento non c'è niente di aperto, dalla sua riga nella scheda **Niente di aperto**. L'albero dell'elemento in questa pagina mostra i rilevamenti aperti accanto a ogni dataset. Il link di un rilevamento apre il pannello di ripristino dell'elemento sull'ultimo backup buono del dataset. Se un'esecuzione arriva in fondo viene giudicato per l'intero elemento, perché un'esecuzione riesce o fallisce nel suo insieme.

I controlli in sé sono descritti in [Funzionalità](features.md). Un assistente collegato tramite il [server MCP](mcp.md) può elencare i punti di ripristino di un elemento ZFS, avviarne il backup e leggere i rilevamenti, ma la presa visione di un rilevamento avviene nella pagina **Anomalie**.

## Codici di motivo {#reason-codes}

La pagina, la cronologia delle esecuzioni e le notifiche nominano un problema con uno di questi codici. Per la maggior parte la soluzione è indicata anche accanto, sulla pagina.

| Codice | Significato | Cosa fare |
|---|---|---|
| `ssh-missing` | La connessione SSH non è configurata in questo container. | Configura il collegamento SSH come per i backup delle VM. |
| `host-placeholder` | Host SSH: Address è ancora il valore di esempio, e nemmeno `host.docker.internal` ha risposto. | Imposta Host SSH: Address sull'IP LAN di questo server. |
| `host-fallback` | Host SSH: Address è ancora il valore di esempio, e `host.docker.internal` funziona. | Niente, oppure imposta l'IP LAN. |
| `ssh-unreachable` | Il server non è raggiungibile via SSH. | Controlla indirizzo e porta, e che SSH sia attivo. |
| `ssh-auth` | Il server ha rifiutato la chiave di BombVault. | Esegui una volta sul server il comando mostrato nella scheda della connessione. |
| `zfs-not-found` | L'host SSH non ha il comando `zfs`. | Fai puntare Host SSH: Address alla macchina che possiede i pool. |
| `zfs-permission` | L'utente SSH non può eseguire questo comando zfs. | Usa root, oppure vedi [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` indica un host o un utente diverso dai campi SSH. | Falli coincidere, oppure svuota i campi SSH così che entrambi vengano dall'URI. |
| `zfs-error` | zfs ha segnalato un altro errore. | I dettagli mostrano il suo messaggio. |
| `propagation-missing` | I nuovi mount sull'host non arrivano al container. | Imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault. |
| `invalid-name` | Un nome di dataset che BombVault non accetta. | Rinomina il dataset. |
| `name-too-long` | Un dataset dell'albero è troppo lungo per un nome di snapshot. | Rinominalo, oppure aggiungi come elemento un dataset che sta sotto. |
| `invalid-exclude` | Un pattern di esclusione o un figlio escluso non corrisponde all'elemento. | Correggi la voce indicata dal messaggio. Per escludere un intero dataset figlio, disattivalo invece di scrivere un pattern. |
| `not-found` | Il dataset non esiste sul server. | Rimuovi l'elemento o ricrea il dataset. I suoi backup restano ripristinabili. |
| `not-filesystem` | Questo è un volume, non un filesystem. | Vedi [Volumi](#volumes). |
| `overlaps-item` | Il dataset si sovrappone a un elemento esistente. | Vedi [Gli elementi non si sovrappongono mai](#overlap). |
| `docker-storage` | L'albero contiene lo storage delle immagini di Docker. | Vedi [Lo storage di Docker](#docker-storage). |
| `nothing-readable` | Al momento nessun dataset dell'elemento è leggibile. | Guarda i codici dei dataset saltati. |
| `snapshot-failed` | Non è stato possibile creare lo snapshot. | I dettagli mostrano il messaggio di zfs. |
| `containers-busy` | Un backup di container era ancora in corso quando i container dovevano fermarsi. | Riprova più tardi. Le esecuzioni pianificate aspettano da sole. |
| `consistency-stop-failed` | Un container non si è potuto fermare, quindi non è stato preso alcuno snapshot. | Controlla il container, oppure toglilo dall'elenco. |
| `pre-snapshot-failed` | Il comando prima dello snapshot è fallito. | I dettagli dell'esecuzione mostrano il suo output. |
| `container-unknown` | Un container elencato non esiste. | Toglilo dall'elenco. |
| `container-is-self` | BombVault non può fermare il proprio container. | Toglilo dall'elenco. |
| `leftover-snapshots` | Sul server ci sono ancora snapshot che BombVault non è riuscito a rimuovere. | Premi **Rimuovi ora**, vedi [Snapshot rimasti](#leftover-snapshots). |
| `zvol` | Un volume nell'albero, saltato. | Vedi [Volumi](#volumes). |
| `canmount-off` | Mai montato (`canmount=off`), saltato. | Se contiene dati, montalo o sposta i dati in un dataset figlio. |
| `legacy-mount` | Mountpoint legacy, saltato. | Dagli un mountpoint sotto `/mnt`. |
| `no-mountpoint` | Nessun mountpoint, saltato. | Dagli un mountpoint sotto `/mnt`. |
| `not-mounted` | Non montato sul server, saltato. | Montalo con `zfs mount`, oppure imposta `canmount=on`. |
| `key-not-loaded` | Cifrato e la chiave non è caricata, saltato. | `zfs load-key`, poi montalo. |
| `snapdir-disabled` | L'accesso agli snapshot è disattivato, saltato. | `zfs set snapdir=hidden <dataset>`. La cartella `.zfs` resta nascosta. |
| `not-visible` | BombVault non vede il mountpoint del dataset. | Sposta il mountpoint sotto il percorso di Host Data, oppure mappalo nel container allo stesso percorso con Read/Write - Slave. |
| `shfs-only` | Il dataset è visibile solo tramite `/mnt/user`, che nasconde gli snapshot. | Mappa `/mnt`, non `/mnt/user`, come Host Data. |
| `snapshot-not-visible` | Lo snapshot è stato creato ma non è comparso dentro BombVault. | Esegui **Prova l'accesso agli snapshot**; vedi sotto. |
| `snapshot-loop` | Lo snapshot non ha raggiunto BombVault perché Host Data non fa passare i nuovi mount. | Imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault. |
| `backup-failed` | restic è fallito per questo dataset. | I dettagli dell'esecuzione mostrano il motivo. |
| `not-reached` | L'esecuzione è terminata prima di questo dataset. | Esegui di nuovo il backup. |
| `gone` | Il dataset non è più sul server. | Niente. I suoi backup restano ripristinabili. |
| `read-only-mount` | BombVault può solo leggere il dataset, quindi non può ripristinarci dentro. | Imposta la mappatura su Read/Write - Slave, oppure ripristina in una cartella. |
| `destination-not-mounted` | La cartella non è su un pool o una condivisione montati. | Scegli una cartella su un pool o una condivisione. |
| `not-enough-space` | Spazio libero insufficiente nella destinazione. | Libera spazio o scegli un'altra cartella. |
| `safety-snapshot-failed` | Non è stato possibile prendere lo snapshot di sicurezza, quindi non è stato ripristinato nulla. | I dettagli mostrano il messaggio di zfs. |
| `safety-name-too-long` | Il nome del dataset è troppo lungo per uno snapshot di sicurezza. | Disattiva lo snapshot di sicurezza, oppure ripristina in una cartella. |
| `dataset-exists` | Esiste già un dataset con questo nome. | Scegli un nuovo nome, oppure ripristina nel dataset stesso. |
| `create-failed` | Non è stato possibile creare il nuovo dataset. | I dettagli mostrano il messaggio di zfs. Controlla che il dataset superiore esista. |
| `new-dataset-not-visible` | Il nuovo dataset è stato creato, ma BombVault non lo vede, quindi non è stato ripristinato nulla. | Il dataset resta sul server. Montalo sotto il percorso Host Data e ripristina al suo interno. |
| `set-properties-failed` | Non è stato possibile impostare le proprietà salvate, quindi non è stato ripristinato nulla. | I dettagli mostrano il messaggio di zfs. |
| `set-limits-failed` | I file sono stati ripristinati, ma non è stato possibile impostare la quota o la prenotazione salvata. | I dettagli mostrano il messaggio di zfs. Imposta tu la quota o la prenotazione con `zfs set`. |

### Controllare cosa vede il container {#mountinfo}

**Prova l'accesso agli snapshot** su un elemento prende un vero snapshot del suo albero, lo cerca dentro BombVault per ogni dataset e poi lo rimuove. È il modo più rapido per verificare l'intero percorso prima della prima esecuzione pianificata.

Per controllare da solo, esegui questo sul server:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Ogni riga è un mount dentro il container. La riga di un dataset mostra il suo percorso dentro il container (sotto `/host/user`) e il nome del dataset. Un campo `master:N` su quella riga significa che il mount riceve i mount che l'host crea in seguito, ed è ciò che serve all'accesso agli snapshot. Se manca, imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault.

## TrueNAS SCALE {#truenas}

- Quando `LIBVIRT_URI` è impostata (come per i backup delle VM su TrueNAS), BombVault prende dall'URI l'host, l'utente e la porta SSH per i suoi comandi zfs, ciascuno di quelli non impostati separatamente. Senza backup delle VM, imposta invece `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` e `LIBVIRT_SSH_PORT`. Aggiungi le variabili in **Additional Environment Variables**.
- Un utente diverso da root ha bisogno di permessi sul dataset superiore dell'elemento, che coprono poi ogni dataset sotto di esso:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Il ripristino in un nuovo dataset richiede anche `create` sul dataset superiore, e impostare le proprietà salvate richiede i permessi per quelle proprietà.

  Una sessione SSH non root su TrueNAS non ha `/usr/sbin` nel suo percorso; BombVault chiama allora direttamente `/usr/sbin/zfs`.
- L'**Host Data** dell'app deve essere un percorso dell'host sopra i dataset, per esempio `/mnt/tank`, non un ixVolume. Con un percorso dell'host, l'app passa a BombVault i nuovi mount dell'host (`rslave`), ed è ciò che serve all'accesso agli snapshot.
