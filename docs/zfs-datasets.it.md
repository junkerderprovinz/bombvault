# Dataset ZFS

I dataset ZFS sono elementi nell'elenco **Backup**, come container e VM. Un elemento è un dataset insieme a ogni dataset sotto di esso. Per ogni backup BombVault fa un unico snapshot ZFS dell'intero albero, così ogni dataset al suo interno viene catturato nello stesso istante. Legge poi i file di ogni dataset da quello snapshot e li salva con restic, nello stesso modo in cui salva una cartella, e rimuove lo snapshot subito dopo. I backup sono deduplicati, puoi sfogliarli tutti e si possono ripristinare i singoli file.

Il backup non invia mai uno stream dentro restic e non riporta mai indietro un dataset. BombVault distrugge solo gli snapshot che ha creato lui. La [replica](#replica) facoltativa è l'unico punto che usa `zfs send`: copia i dataset su un secondo server ZFS e non tocca il backup.

## Requisiti {#requirements}

- **Il collegamento SSH a questo server.** I dataset ZFS usano la stessa chiave, lo stesso host e lo stesso utente dei backup delle VM. Se i backup delle VM funzionano già, funziona anche questo. Altrimenti segui la [guida al backup delle VM via SSH](vm-backup-ssh-setup.md). I campi del template si chiamano **Host SSH: Address**, **Host SSH: Port** e **Host SSH: User**.
- **Il comando `zfs` su quell'host.** Ce l'hanno Unraid 6.12 e successivi e TrueNAS SCALE.
- **Host Data mappato come `/mnt` con Access Mode Read/Write - Slave.** È il valore predefinito del template. Lo snapshot di un dataset compare dentro la cartella `.zfs/snapshot` del dataset solo dopo che BombVault è partito, quindi il container deve ricevere i mount che l'host crea in seguito.
- **I dataset montati sotto `/mnt`.** Su Unraid i pool stanno in `/mnt/<pool>`, quindi è già così.

Attiva la sezione in **Impostazioni, Generale** (Dataset ZFS). **Connessione ZFS a questo server** sotto **Altro** nella pagina Backup apre allora **Connessione a questo server**. Prova il collegamento SSH, nomina l'utente e l'host a cui si connette, e dice cosa manca quando qualcosa manca. Quando il collegamento è interrotto, l'elenco Backup lo dice con **Mostra connessione**, e la scheda **Integrazione host** nella panoramica mostra lo stesso risultato.

## Elementi e dataset figli {#items-and-children}

Scegli **Aggiungi** nella pagina Backup, poi **Un dataset ZFS**. L'elenco viene dal server. Scegli il dataset in cima a ciò che vuoi salvare, per esempio `cache/appdata`, e l'elemento copre quello e ogni dataset sotto di esso.

- **I nuovi dataset figli si uniscono da soli.** Un dataset creato in seguito sotto l'elemento viene salvato con l'esecuzione successiva, e quell'esecuzione lo nomina come nuovo. Il suo primo backup lo legge per intero una volta. Poi vengono letti solo i cambiamenti.
- **Puoi lasciare fuori singoli figli.** Disattiva un figlio nella scheda Impostazioni dell'elemento e viene lasciato fuori insieme a tutto ciò che sta sotto di lui. Un figlio lasciato fuori che non esiste più sul server è segnato come tale e si può rimuovere dall'elenco.
- **I figli che non si possono leggere vengono saltati, mai in silenzio.** L'esecuzione li elenca, l'elemento mostra quanti sono stati saltati, e la scheda **Senza backup** della panoramica conta ciascuno come non protetto. L'esecuzione salva comunque tutto il resto e non fallisce a causa di un figlio saltato. I motivi sono nella [tabella dei codici motivo](#reason-codes): un dataset non montato, con `canmount=off`, con mountpoint `legacy` o senza mountpoint, una chiave di cifratura non caricata, l'accesso agli snapshot disattivato, o un mountpoint che BombVault non vede.
- **Un dataset saltato non si porta dietro i suoi figli.** Un dataset `canmount=off` che contiene solo altri dataset viene saltato (mostrato come "solo struttura") e i suoi figli montati vengono salvati. Un dataset cifrato la cui chiave non è caricata viene saltato insieme ai figli che condividono la sua chiave.
- **I figli che sono dischi di VM o dati di sistema partono disattivati** nella finestra di aggiunta, con il motivo accanto all'interruttore. Aggiungere un intero pool chiede una conferma che elenca cosa contiene.

### Volumi {#volumes}

Un volume (zvol) contiene un disco virtuale invece di file, e un elemento ZFS non ne salva mai uno.

- Un volume usato da una VM viene salvato con quella VM.
- Un volume che nessuna VM usa (un extent iSCSI, un disco che hai scollegato) **non viene salvato da BombVault**. La finestra di aggiunta e la pagina Backup contano questi volumi e lo dicono. Una versione futura li salverà.

I volumi dentro l'albero di un elemento vengono saltati e nominati a ogni esecuzione.

### L'archiviazione di Docker {#docker-storage}

Con il driver di storage ZFS di Docker, ogni livello di immagine è un dataset con mountpoint `legacy`. La finestra di aggiunta li raggruppa in una riga per genitore. Un albero che contiene più di 20 dataset simili non può diventare un elemento: finché ne esiste uno snapshot, Docker non può rimuovere i livelli delle immagini. Aggiungi invece i dataset sotto di esso, per esempio `appdata`.

### Gli elementi non si sovrappongono mai {#overlap}

Un dataset può appartenere a un solo elemento. BombVault rifiuta un nuovo elemento che sta dentro uno esistente, o che ne conterrebbe uno. Per unire più elementi figli in un unico elemento genitore, elimina prima gli elementi figli scegliendo di tenere i loro backup, poi aggiungi il genitore. Ogni dataset mantiene la sua storia sotto il proprio nome, quindi il backup successivo prosegue da dove si erano fermati i vecchi elementi e non rilegge tutto.

## Fermare i container e comandi attorno allo snapshot {#consistency}

Lo snapshot di un database in esecuzione è come un'improvvisa interruzione di corrente: il database di solito si riprende, ma deve farlo. Ogni elemento può fare due cose a riguardo, ed entrambe coprono solo l'istante dello snapshot, non l'intero backup.

- **Ferma questi container per lo snapshot.** BombVault ferma i container elencati, fa lo snapshot e li riavvia subito. I container di uno stesso livello di dipendenze si fermano in parallelo, prima i dipendenti, quindi l'intera finestra dura di solito pochi secondi. L'esecuzione mostra quanto è durata. Il backup legge poi lo snapshot congelato mentre le app sono già di nuovo in funzione. Vengono fermati solo i container che erano in esecuzione.
- **Un comando prima e dopo lo snapshot.** Gira dentro un container che scegli, per esempio per scaricare un database nel dataset subito prima dello snapshot, senza fermare nulla. Se il comando prima dello snapshot fallisce, il backup fallisce e non viene fatto alcuno snapshot. Un comando dopo lo snapshot che fallisce viene mostrato nell'esecuzione ma non fa fallire il backup.

Cosa succede quando qualcosa va storto:

- Se un container non si può fermare, BombVault riavvia quelli che aveva già fermato e il backup fallisce, nominando il container. Non ripiega mai su uno snapshot di app in esecuzione.
- L'arresto aspetta che un backup di container in corso finisca (fino a 30 minuti per un'esecuzione manuale, fino al limite di tempo del backup per una pianificata), così i due non fermano e avviano mai lo stesso container nello stesso momento.
- Prima che si fermi il primo container, BombVault annota quali ferma. Se BombVault viene terminato dentro la finestra, riavvia quei container la volta successiva che parte, invia una notifica, e l'elemento mostra una nota rossa per ogni container che non è riuscito ad avviare.

I [dump automatici dei database](backups.md#database-dumps) girano con il backup di un container, non con un elemento ZFS. Un database il cui container viene salvato solo tramite il suo dataset non riceve alcun dump, quindi dagli un comando qui.

Un container può stare in questo elenco ed essere salvato anche come container. I suoi dati vengono allora salvati due volte, in due repository, e il backup completo lo ferma due volte. L'elemento lo dice.

## Ripristinare {#restore}

Apri l'elemento e scegli **Ripristina**, oppure sceglilo nella pagina **Ripristino**. La finestra di ripristino chiede il backup, poi il **Dataset**. Di default è il dataset in cima all'elemento. Sotto **Cosa** ripristini tutto oppure solo i file e le cartelle che scegli. **Dove** offre:

- **Ripristina dentro il dataset.** I file del backup vengono scritti nel mountpoint del dataset. I file con lo stesso nome vengono sovrascritti, gli altri file restano. Il dataset non viene mai riportato indietro né sostituito. BombVault controlla che il dataset sia montato, visibile e scrivibile, una volta prima di cominciare e di nuovo subito prima di scrivere. Dove dentro di esso è montato un dataset figlio, non viene scritto nulla: il figlio mantiene i suoi file, il proprietario e i permessi, e viene ripristinato dal proprio backup.
- **In una cartella.** Scegli una cartella sotto `/mnt`. BombVault controlla che la cartella sia su un pool o una condivisione montati e che ci sia abbastanza spazio libero. Funziona senza il collegamento SSH e per dataset che non esistono più.
- **In un nuovo dataset.** Dai un nome a un dataset che non esiste ancora. La finestra suggerisce `<dataset>-restored`. BombVault lo crea con le proprietà ZFS memorizzate nel backup e ripristina al suo interno, vedi [Ripristinare come nuovo dataset](#new-dataset).

Scegliere **Tutti i dataset di questo backup** come dataset scrive ogni dataset dell'albero nella sua sottocartella della cartella che scegli. I dataset che sono stati saltati in quel backup vengono nominati. Per ripristinare dal repository di un altro BombVault, usa **Da una vecchia cartella di backup** nella pagina Ripristino. Ripristina sempre in una cartella: tutti i dataset di un backup, ciascuno nella sua sottocartella, oppure un dataset dell'albero, intero o con file scelti.

L'elenco dei container da fermare dell'elemento viene offerto anche per un ripristino dentro il dataset. Quei container restano fermi per tutto il ripristino, e nel frattempo i backup dei container aspettano.

### Lo snapshot di sicurezza {#safety-snapshot}

Prima di scrivere in un dataset, BombVault fa uno snapshot ZFS di quel solo dataset, chiamato `bombvault-prerestore-<time>`. **Tieni prima uno snapshot ZFS dello stato attuale** è attivo di default. Disattivarlo richiede una seconda conferma. Se lo snapshot non si può fare, non viene ripristinato nulla.

BombVault non elimina mai da solo uno snapshot di sicurezza. L'elemento li elenca con la loro età e dimensione, ciascuno con un'azione **Elimina**, e avvisa quando il più vecchio ha più di 30 giorni, perché conserva sul pool dati eliminati e modificati.

Per tornare indietro dopo un ripristino, copia i singoli file da `.zfs/snapshot/bombvault-prerestore-<time>` dentro il dataset. `zfs rollback <dataset>@bombvault-prerestore-<time>` funziona solo finché è lo snapshot più recente di quel dataset. `zfs rollback -r` elimina ogni snapshot più recente, compresi quelli automatici.

### Ripristinare come nuovo dataset {#new-dataset}

BombVault salva con ogni backup le proprietà ZFS impostate localmente di ogni dataset: compressione, dimensione del record, quota, riserva, atime, xattr, acltype, distinzione tra maiuscole e minuscole e le tue proprietà utente. I valori ereditati e di sola lettura vengono lasciati fuori, perché tornano da soli. I backup precedenti al momento in cui BombVault ha iniziato a salvarle non ne hanno.

- **In un nuovo dataset** esegue `zfs create` con ogni proprietà salvata. La distinzione tra maiuscole e minuscole, la normalizzazione e utf8only si possono impostare solo così. Quote e riserve vengono impostate dopo i file, così non possono rifiutarli. Il mountpoint viene lasciato fuori, così la copia non entra in conflitto con l'originale, e lo stesso vale per `canmount`, `readonly` e le impostazioni di cifratura, così il ripristino può scrivere. Un nuovo dataset sotto uno cifrato ne eredita la cifratura. Il dataset sopra quello nuovo deve esistere. Se qualcosa fallisce dopo che il nuovo dataset è stato creato, resta sul server, perché BombVault non distrugge mai un dataset.
- **Ripristina dentro il dataset** elenca le proprietà salvate accanto al ripristino. **Imposta anche queste proprietà** imposta quelle che un dataset esistente può ancora accettare, prima che venga scritto qualsiasi file. Quote e riserve vengono impostate dopo i file, così non possono rifiutarli. Senza quell'interruttore il dataset mantiene le sue impostazioni.

## Cosa c'è nel backup {#contents}

Nel backup: i file e le cartelle di ogni dataset salvato, con proprietà, permessi, date e attributi estesi come li salva restic, e le proprietà ZFS impostate localmente di ogni dataset.

Non nel backup:

- il proprietario e i permessi della cartella in cima di ogni dataset in sé (tutto ciò che sta sotto è incluso). Un ripristino dentro il dataset lascia la cartella in cima esistente com'è, un ripristino in una cartella la crea con permessi `0755`,
- gli snapshot ZFS esistenti,
- i figli saltati o lasciati fuori,
- i volumi.

Per ripristinare su un nuovo pool, crea il pool e ripristina ogni dataset in un nuovo dataset. Non è ancora stato verificato se le ACL NFSv4, come le usa TrueNAS sui dataset SMB, tornino come ti aspetti, quindi controlla un ripristino sui tuoi dati prima di farci affidamento.

## Dataset cifrati {#encryption}

Un dataset cifrato viene salvato solo finché la sua chiave è caricata. Altrimenti viene saltato con un avviso. Carica la chiave con `zfs load-key` e monta il dataset. BombVault legge i dati decifrati e li salva nel repository di restic, che è cifrato. Se hai disattivato la cifratura in BombVault, quel repository non lo è.

## Replica {#replica}

Una replica è una copia dei dataset di un elemento su un secondo server ZFS. BombVault la tiene aggiornata con `zfs send` e `zfs receive`. La prima esecuzione invia tutto, poi viaggiano solo i blocchi modificati. Sull'altro server puoi montare subito la copia.

Una replica non sostituisce mai il backup. Le versioni più vecchie, i file singoli e la verifica vengono ancora dai backup, e la replica conserva solo tanti snapshot quanti ne imposti. Una replica aggiornata conta come copia fuori sede, ma non come backup.

La replica non aspetta il backup. **Dopo ogni backup** significa dopo l'esecuzione di backup dell'elemento, che sia riuscita, fallita o senza nulla da fare. Quando la sezione ZFS non ha alcuna destinazione attiva e l'elemento non ha un repository proprio, un'esecuzione di backup dell'elemento invia solo la replica, e la scheda Replica lo dice nel fumetto accanto a **Quando**. L'elenco Backup, l'intestazione dell'elemento e la cronologia delle esecuzioni mostrano la replica: **Replica in corso** mentre gira, e **Replicato** con l'ora quando ha finito.

**Solo replica, nessun backup** nella scheda Replica fa sì che l'intera sezione ZFS invii repliche e non faccia backup. La cartella propria e le destinazioni off-site mantengono i loro interruttori, quindi disattivarlo fa tornare a salvare su di esse, e la scheda di ogni elemento ZFS mostra l'interruttore della sezione com'è. Un elemento simile aspetta la sua prima replica invece di un backup, **Esegui backup ora** viene offerto come **Replica ora**, e i container dell'elemento si fermano e i suoi comandi girano attorno allo snapshot come per un backup. Le sue repliche contano per una pianificazione ogni pochi giorni, per l'avviso di ritardo e per la copertura.

Attivala nella scheda **Replica** nella scheda Impostazioni dell'elemento. La scheda elenca ogni dataset e volume con il suo stato, e **Replica ora** avvia un'esecuzione. Un'esecuzione di replica ha un lock proprio, quindi un primo trasferimento lungo non blocca mai i backup.

| Campo | Cosa imposta |
|---|---|
| **Replica su** | La destinazione: un server ZFS (segnato **via SSH**) oppure una **istanza associata**. **Aggiungi server ZFS** ne configura uno nuovo. |
| **Quando** | **Dopo ogni backup**, cioè dopo l'esecuzione di backup dell'elemento, oppure **Piano proprio**. |
| **Conserva sulla destinazione** | Quanti snapshot restano sul server ZFS: **Breve**, **Equilibrato**, **Lungo** o **Valori personalizzati**. Un'istanza associata lo decide da sola. |
| **Conserva su questo server** | Quanti degli snapshot di replica più recenti restano anche su questo server. 0 o 1 conserva solo l'ultimo. |
| **Solo replica, nessun backup** | L'intera sezione ZFS invia repliche e non fa backup. |

![La scheda Replica di un elemento ZFS.](assets/screenshots/zfs/replica-dark.png#only-dark)
![La scheda Replica di un elemento ZFS.](assets/screenshots/zfs/replica-light.png#only-light)

**Replica tutto ora**, accanto alle azioni di backup quando nella pagina Backup è scelto il riquadro ZFS, invia la replica di ogni elemento ZFS, uno dopo l'altro. Il server esegue il lotto, quindi prosegue anche quando lasci la pagina, e un elemento che non può replicare viene nominato e saltato.

### Invio a un server ZFS {#replica-push}

Qualsiasi macchina con ZFS e SSH può ricevere, per esempio un secondo Unraid o un TrueNAS. BombVault non deve girare lì.

1. In **Impostazioni, Associazione** scegli **Collega via SSH** in **Server ZFS senza BombVault**, oppure **Aggiungi server ZFS** nella scheda **Replica** dell'elemento.
2. Inserisci indirizzo, utente e porta. La finestra mostra la chiave pubblica di BombVault. Aggiungila al `~/.ssh/authorized_keys` dell'utente sul server. Su Unraid si trova in **Users, root, SSH keys**. A seconda di **Display Settings**, Users sta nel menu dell'intestazione o sotto Settings. Su TrueNAS modifica l'utente in **Credentials, Users**. Questa è la chiave della replica. Differisce dalla chiave del collegamento SSH delle VM e dell'host, quindi aggiungila anche se quella è già autorizzata.
3. Prova la connessione. La finestra elenca poi i pool del server. Scegline uno e imposta la radice, che di default è `<pool>/bombvault-replica`.
4. Scegli il nuovo server nella scheda **Replica** dell'elemento. Il server ha allora una scheda propria nella pagina Istanze, e **Apri** porta alla sua pagina.

Con root non serve altro. Un utente proprio ha bisogno di questi permessi sul pool della destinazione, che la finestra mostra anch'essa:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Sulla sorgente lo stesso tipo di utente ha bisogno di questi sul dataset in cima all'elemento:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

In questa direzione il BombVault che possiede l'elemento possiede anche una chiave che può scrivere sul server.

BombVault fissa la chiave host che il server mostra alla prima connessione e ne rifiuta una diversa in seguito. Se il server è stato reinstallato, premi **Dimentica chiave host** nella pagina del server, e la connessione successiva fissa la nuova chiave.

Quando l'host di questo server può raggiungere da solo il server ZFS, incanala `zfs send` direttamente nel proprio ssh verso il server, con la chiave della replica e la chiave host fissata, così lo stream non passa da BombVault. Altrimenti lo stream passa da BombVault.

### Invio a un'istanza associata {#replica-receive}

Un BombVault associato può ricevere da solo la replica. Nessuno ottiene accesso SSH all'altro host, e nessuna chiave finisce in un file `authorized_keys`.

1. Nella scheda **Replica** dell'elemento scegli l'istanza associata come destinazione, oppure attiva il ruolo **Server ZFS** sulla scheda di quell'istanza nella pagina Istanze e scegli lì gli elementi ZFS. La scheda mostra **In attesa di approvazione** finché non risponde.
2. Sull'istanza ricevente apri **Istanze**. La richiesta compare sulla scheda dell'istanza che invia come badge, e i suoi **Dettagli** la elencano. Scegli il pool e la radice su quel server e quanti snapshot restano, poi premi **Consenti** o **Rifiuta**.
3. Dopo **Consenti** la sorgente invia con il proprio piano, come per qualsiasi altra destinazione.

L'istanza ricevente accetta solo ciò che l'approvazione copre: i dataset dell'elemento, nella propria radice. Esegue lei stessa `zfs receive`, e la sorgente non ha modo di eliminare o riportare indietro nulla lì. Quindi la copia sopravvive a una sorgente di cui qualcuno ha preso il controllo. L'istanza ricevente mantiene la propria conservazione. La sorgente suggerisce una regola soltanto quando la chiede.

**Revoca accesso** sull'istanza ricevente termina l'approvazione in qualsiasi momento e lo comunica alla sorgente, che mostra allora che l'approvazione è stata ritirata e si ferma. Ciò che l'istanza ricevente già contiene resta lì. Una richiesta rifiutata resta rifiutata. Altri dataset, o una richiesta dopo una revoca, chiedono di nuovo.

L'istanza ricevente deve rispondere via HTTPS. La sorgente non invia nulla a un indirizzo http semplice, perché il token e i dati attraverserebbero la rete non cifrati. Ogni trasferimento lascia inoltre libero un decimo del pool ricevente e si ferma prima di intaccarlo, così una sola sorgente non può riempire il pool del server ricevente.

### Dove finiscono i dati {#replica-target}

Ogni dataset finisce in `<root>/<server>/<pool>/<path>`. La cartella del server è il nome dell'istanza sorgente, fissato al primo trasferimento, così due server con lo stesso nome di pool non si intralciano mai. Per esempio, `cache/appdata` di un server chiamato `tower` finisce in `backup/bombvault-replica/tower/cache/appdata`.

Dopo che BombVault è stato configurato di nuovo, la destinazione rifiuta la cartella che questo BombVault aveva riempito prima. La scheda Replica offre allora **Prendi la cartella**, e la replica successiva prosegue dalle copie al suo interno. Prendila solo quando nessun altro BombVault replica ancora in quella cartella.

La copia sulla destinazione è di sola lettura. Di default non è montata, quindi non copre mai nulla su quel server. Un server ZFS collegato via SSH ha nella sua pagina un interruttore **Monta in sola lettura sul server**: attivo, BombVault monta lì le copie in sola lettura dopo ogni esecuzione, così puoi leggere e copiare i loro file sulla destinazione. Le copie cifrate restano bloccate finché la loro chiave non è caricata, e disattivarlo non smonta nulla. Un'istanza associata che riceve la replica la tiene smontata. Le proprietà ZFS viaggiano insieme, tranne il mountpoint e `sharenfs` e `sharesmb`. Anche le riserve `reservation` e `refreservation` restano indietro, così la copia occupa solo lo spazio di cui i suoi dati hanno bisogno.

### Cosa viene incluso {#replica-contents}

Entra tutto ciò che l'elemento salva, e anche i volumi sotto di esso, che il backup salta. Un dataset figlio che hai disattivato nell'elemento resta fuori. Tutti i dataset di un'esecuzione vengono da un unico snapshot, come nel backup.

### Per quanto tempo restano gli snapshot {#replica-retention}

Sulla destinazione, una nuova replica conserva 7 snapshot giornalieri e 3 settimanali. Scegli invece **Breve**, **Equilibrato** o **Lungo**, oppure imposta **Valori personalizzati**. Lì vengono rimossi solo gli snapshot chiamati `bombvault-replica-<14 digits>`, e mai il più recente che entrambi i lati condividono.

Sulla sorgente, BombVault conserva l'ultimo snapshot della replica, più un bookmark per ogni stato che ha inviato. I bookmark non costano spazio. Il trasferimento successivo parte da essi.

Per conservare di più anche sulla sorgente, imposta **Conserva su questo server** nella scheda Replica, per esempio su 7. Tanti degli snapshot di replica più recenti restano allora su questo server dopo ogni replica, accanto a ciò che conserva la destinazione. 0 o 1 conserva solo l'ultimo. L'ultimo porta un hold, così la base del trasferimento successivo non viene distrutta per sbaglio. I più vecchi non ne hanno. Ogni snapshot trattiene i blocchi cambiati dopo di lui, quindi ognuno che conservi occupa spazio sul pool. Disattivare la replica o passare a un'altra destinazione, o eliminare l'elemento, li rimuove anche da questo server.

I file di uno snapshot conservato si possono leggere o copiare subito, dalla cartella `.zfs/snapshot` dentro il dataset o da un clone, senza toccare il dataset né la replica:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Un clone non occupa spazio finché qualcosa al suo interno non cambia. Distruggilo con `zfs destroy cache/appdata-20261006` quando hai finito, perché uno snapshot che ha un clone non può essere rimosso.

Tornare all'ultimo snapshot della replica richiede solo `zfs rollback cache/appdata@<latest>` qui, e nulla sulla destinazione. Riportare l'intero dataset a uno più vecchio distrugge ogni snapshot dopo di esso, su questo server e sulla destinazione. L'ultimo è trattenuto, quindi togli prima il suo hold, poi riporta indietro il dataset qui:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

Anche la copia sulla destinazione deve tornare indietro, ma non sempre allo stesso snapshot, poiché le sue regole di conservazione possono aver scartato quello. Esegui una volta la replica. Si ferma per quel dataset perché la destinazione contiene snapshot più recenti, e la scheda Replica e la cronologia delle esecuzioni nominano lo snapshot a cui riportare la destinazione: il più recente che entrambi i lati condividono ancora. Riporta la copia a quello sul server ZFS, o sull'host dell'istanza associata che riceve la replica, per esempio:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` agisce su un solo dataset, quindi ripeti i rollback per ogni dataset figlio che vuoi riavere. Dopo, **Replica ora** prosegue dallo snapshot condiviso.

### Dataset cifrati in una replica {#replica-encryption}

Un dataset cifrato viene inviato raw. Resta cifrato sulla destinazione, e la destinazione non vede mai la chiave. Tieni al sicuro la chiave: ti serve per aprire la copia dopo un ripristino, e una replica senza di essa è illeggibile.

### Usare la replica {#replica-use}

Apri la scheda **Backup** dell'elemento e clicca la riga **Replica ZFS** tra i posti che contengono i suoi snapshot. Il foglio elenca gli snapshot sulla destinazione e mostra i comandi con i tuoi nomi reali.

Per guardare uno stato vecchio, clona uno snapshot sulla destinazione. Un clone non occupa spazio finché qualcosa non cambia, e la replica resta intatta:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Se la sorgente si guasta, trasforma la copia in un normale dataset scrivibile sulla destinazione:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault smette poi di replicare su quel dataset, finché non avvii una nuova prima esecuzione.

Per riportare uno stato sulla sorgente, premi **Ripristina come nuovo dataset** nel foglio. BombVault invia lo snapshot in un nuovo dataset accanto all'originale, chiamato `<dataset>-bombvault-restore-` più un timestamp. Non scrive mai sopra l'originale.

### Esempio: una replica su un secondo server Unraid {#example-replica}

Il server `tower` salva il dataset `cache/appdata` come elemento ZFS. Un secondo server Unraid su `192.168.1.20`, senza BombVault, ha un pool chiamato `backup` e deve contenere una copia che puoi leggere in qualsiasi momento.

1. Apri **Backup**, poi `cache/appdata`, poi la sua scheda **Impostazioni**. Nella scheda **Replica** scegli **Aggiungi server ZFS** sotto **Replica su**.
2. Inserisci `192.168.1.20`, l'utente `root` e la porta `22`. La finestra mostra la chiave della replica. Sulla seconda macchina incollala in **Users, root, SSH keys** e premi **Apply**. Questa chiave differisce dalla chiave SSH dell'host, quindi aggiungila anche se quella c'è già.
3. Prova la connessione. Scegli il pool `backup` e mantieni la radice che suggerisce, `backup/bombvault-replica`.
4. Tornato nella scheda, il nuovo server è scelto sotto **Replica su**. Lascia **Quando** su **Dopo ogni backup**, lascia **Conserva sulla destinazione** ai suoi 7 snapshot giornalieri e 3 settimanali e imposta **Conserva su questo server** su `7`.
5. Premi **Replica ora**. La prima esecuzione invia tutto, quindi 300 GB su una linea da 1 Gbit/s richiedono almeno 40 minuti. La scheda e l'elenco Backup dicono **Replica in corso**, poi **Replicato** con l'ora. Le esecuzioni successive inviano solo i blocchi modificati.

I dataset stanno ora in `backup/bombvault-replica/tower/cache/appdata` sulla seconda macchina, in sola lettura. Nella panoramica, **Off-site** della sezione ZFS conta la replica come copia fuori sede.

Per riavere un file di martedì scorso hai tre vie:

- **Su questo server**, perché **Conserva su questo server** tiene sette snapshot:

    ```
    cp /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261103014100/plex/Preferences.xml /mnt/user/restore/
    ```

- **Sulla seconda macchina**, con un clone di uno snapshot sulla destinazione. La scheda **Backup** dell'elemento mostra i comandi con i tuoi nomi reali quando clicchi la riga **Replica ZFS**. Rimuovi il clone con `zfs destroy` quando hai finito.
- **Con un mount**, attivando **Monta in sola lettura sul server** nella pagina del server ZFS. Dopo l'esecuzione successiva le copie sono montate in sola lettura sulla seconda macchina.

Le versioni più vecchie di quelle che la replica conserva vengono ancora dai backup, come mostra l'esempio successivo.

### Esempio: un file da un backup ZFS {#example-zfs-file}

1. Apri **Ripristino**, scegli il riquadro **ZFS** e clicca `cache/appdata`.
2. Scegli il backup, e sotto **Dataset** il dataset che contiene il file.
3. Sotto **Cosa** scegli **File singoli** e spunta il file.
4. Sotto **Dove** scegli **In una cartella** e scegli una cartella sotto `/mnt`, come `user/restore`. Non richiede il collegamento SSH e funziona anche per un dataset che non esiste più.
5. Premi **Ripristina**. Il file finisce direttamente in quella cartella, senza il percorso sopra di esso.

## Snapshot rimasti {#leftover-snapshots}

Lo snapshot di un backup si chiama `<dataset>@bombvault-<14 digits>`, per esempio `cache/appdata@bombvault-20260924021500` (UTC). BombVault lo rimuove subito dopo il backup. Se non ci riesce, per esempio perché il dataset è occupato o BombVault è stato fermato, lo rimuove:

- prima del backup successivo di quell'elemento,
- all'avvio di BombVault, per ogni elemento, anche con la sezione disattivata,
- quando elimini l'elemento,
- quando premi **Rimuovi ora** sull'elemento, che mostra quanti ne restano.

Vengono rimossi solo i nomi che corrispondono esattamente a `bombvault-` più 14 cifre. Gli snapshot di sicurezza, i tuoi snapshot e quelli automatici non vengono mai toccati. Per rimuoverne uno a mano:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Lo snapshot di una replica si chiama `<dataset>@bombvault-replica-<14 digits>` e non è un resto. Resta sulla sorgente finché un'esecuzione di replica più recente non lo sostituisce, o finché **Conserva su questo server** lo mantiene, e sulla destinazione finché la conservazione lo mantiene. Lo spazzino non lo tocca mai, perché corrisponde solo a `bombvault-` seguito da esattamente 14 cifre.

## Anomalie {#anomalies}

Un figlio svuotato cambia appena il totale di un albero grande, quindi il rilevamento delle anomalie sorveglia da sola ogni dataset di un elemento: la sua dimensione, il numero di file, i dati nuovi e il tempo di restic hanno ciascuno una storia propria. Quella storia appartiene al nome del dataset, quindi resta quando l'albero viene poi salvato da un altro elemento.

Un dataset che l'esecuzione precedente ha salvato e che questa non ha potuto leggere conta come svuotato, finché la selezione dell'elemento non è cambiata. Questo copre una chiave non caricata, un dataset non montato e uno sparito dall'albero. Un figlio che escludi tu cambia la selezione, quindi la sua storia ricomincia da capo. Finché un riscontro su dati persi è aperto, la conservazione tiene i vecchi backup di quel solo dataset e pota il resto dell'albero come al solito.

Ogni dataset ha una riga propria nel monitoraggio dell'elemento, che si apre con **Monitoraggio** nella pagina **Anomalie** e sta nella scheda **Verifica** dell'elemento. I riscontri aperti compaiono accanto a ogni dataset. Il link in un riscontro apre la finestra di ripristino sull'ultimo backup buono del dataset. Se un'esecuzione termina viene giudicato per l'intero elemento, perché un'esecuzione riesce o fallisce nel suo insieme.

I controlli in sé sono descritti in [Anomalie](anomalies.md). Un assistente connesso tramite il [server MCP](mcp.md) può elencare i punti di ripristino di un elemento ZFS, avviarne il backup e leggere i riscontri, ma confermare la presa visione di uno avviene nella pagina **Anomalie**.

## Codici motivo {#reason-codes}

La pagina dell'elemento, la cronologia delle esecuzioni e le notifiche nominano un problema con uno di questi codici. Per la maggior parte c'è anche la soluzione accanto, nella pagina.

| Codice | Significato | Cosa fare |
|---|---|---|
| `ssh-missing` | La connessione SSH non è configurata in questo container. | Configura il collegamento SSH come per i backup delle VM. |
| `host-placeholder` | Host SSH: Address è ancora il segnaposto, e nemmeno `host.docker.internal` ha risposto. | Imposta Host SSH: Address sull'IP LAN di questo server. |
| `host-fallback` | Host SSH: Address è ancora il segnaposto, e `host.docker.internal` funziona. | Niente, oppure imposta l'IP LAN. |
| `ssh-unreachable` | Il server non è raggiungibile via SSH. | Controlla indirizzo e porta, e che SSH sia attivo. |
| `ssh-auth` | Il server ha rifiutato la chiave di BombVault. | Esegui una volta sul server il comando mostrato sotto **Connessione a questo server**. |
| `zfs-not-found` | L'host SSH non ha il comando `zfs`. | Punta Host SSH: Address alla macchina che possiede i pool. |
| `zfs-permission` | L'utente SSH non può eseguire questo comando zfs. | Usa root, oppure vedi [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nomina un host o un utente diverso dai campi SSH. | Falli coincidere, oppure svuota i campi SSH così entrambi vengono dall'URI. |
| `zfs-error` | zfs ha riportato un altro errore. | I dettagli mostrano il suo messaggio. |
| `propagation-missing` | I nuovi mount sull'host non raggiungono il container. | Imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault. |
| `invalid-name` | Un nome di dataset che BombVault non accetta. | Rinomina il dataset. |
| `name-too-long` | Un dataset nell'albero è troppo lungo per il nome di uno snapshot. | Rinominalo, oppure aggiungi come elemento un dataset sotto di esso. |
| `invalid-exclude` | Un pattern di esclusione o un figlio lasciato fuori non si adatta all'elemento. | Correggi l'elemento che il messaggio nomina. Per lasciare fuori un intero dataset figlio, disattivalo invece di scrivere un pattern. |
| `not-found` | Il dataset non esiste sul server. | Rimuovi l'elemento, oppure ricrea il dataset. I suoi backup restano ripristinabili. |
| `not-filesystem` | È un volume, non un filesystem. | Vedi [Volumi](#volumes). |
| `overlaps-item` | Il dataset si sovrappone a un elemento esistente. | Vedi [Gli elementi non si sovrappongono mai](#overlap). |
| `docker-storage` | L'albero contiene l'archiviazione delle immagini di Docker. | Vedi [L'archiviazione di Docker](#docker-storage). |
| `nothing-readable` | In questo momento nessun dataset dell'elemento si può leggere. | Guarda i codici dei dataset saltati. |
| `snapshot-failed` | Lo snapshot non si è potuto creare. | I dettagli mostrano il messaggio di zfs. |
| `containers-busy` | Un backup di container era ancora in corso quando i container dovevano fermarsi. | Riprova più tardi. Le esecuzioni pianificate aspettano da sole. |
| `consistency-stop-failed` | Un container non si è potuto fermare, quindi non è stato fatto alcuno snapshot. | Controlla il container, oppure toglilo dall'elenco. |
| `pre-snapshot-failed` | Il comando prima dello snapshot è fallito. | I dettagli dell'esecuzione mostrano il suo output. |
| `container-unknown` | Un container elencato non esiste. | Toglilo dall'elenco. |
| `container-is-self` | BombVault non può fermare il proprio container. | Toglilo dall'elenco. |
| `leftover-snapshots` | Ci sono ancora sul server snapshot che BombVault non ha potuto rimuovere. | Premi **Rimuovi ora**, vedi [Snapshot rimasti](#leftover-snapshots). |
| `zvol` | Un volume nell'albero, saltato. | Vedi [Volumi](#volumes). |
| `canmount-off` | Mai montato (`canmount=off`), saltato. | Se contiene dati, montalo oppure sposta i dati in un dataset figlio. |
| `legacy-mount` | Mountpoint legacy, saltato. | Dagli un mountpoint sotto `/mnt`. |
| `no-mountpoint` | Nessun mountpoint, saltato. | Dagli un mountpoint sotto `/mnt`. |
| `not-mounted` | Non montato sul server, saltato. | Montalo con `zfs mount`, oppure imposta `canmount=on`. |
| `key-not-loaded` | Cifrato e la chiave non è caricata, saltato. | `zfs load-key`, poi montalo. |
| `snapdir-disabled` | L'accesso agli snapshot è disattivato, saltato. | `zfs set snapdir=hidden <dataset>`. La cartella `.zfs` resta nascosta. |
| `not-visible` | BombVault non vede il mountpoint del dataset. | Sposta il mountpoint sotto il percorso di Host Data, oppure mappalo nel container allo stesso percorso con Read/Write - Slave. |
| `shfs-only` | Il dataset è visibile solo tramite `/mnt/user`, che nasconde gli snapshot. | Mappa `/mnt`, non `/mnt/user`, come Host Data. |
| `snapshot-not-visible` | Lo snapshot è stato creato ma non è comparso dentro BombVault. | Esegui **Prova l'accesso agli snapshot**. Vedi sotto. |
| `snapshot-loop` | Lo snapshot non è arrivato a BombVault perché Host Data non lascia passare i nuovi mount. | Imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault. |
| `backup-failed` | restic è fallito per questo dataset. | I dettagli dell'esecuzione dicono perché. |
| `not-reached` | L'esecuzione è finita prima di questo dataset. | Esegui di nuovo il backup. |
| `gone` | Il dataset non è più sul server. | Niente. I suoi backup restano ripristinabili. |
| `read-only-mount` | BombVault può solo leggere il dataset, quindi non può ripristinare al suo interno. | Imposta la mappatura su Read/Write - Slave, oppure ripristina in una cartella. |
| `destination-not-mounted` | La cartella non è su un pool o una condivisione montati. | Scegli una cartella su un pool o una condivisione. |
| `not-enough-space` | Non c'è abbastanza spazio libero nella destinazione. | Libera spazio o scegli un'altra cartella. |
| `safety-snapshot-failed` | Lo snapshot di sicurezza non si è potuto fare, quindi non è stato ripristinato nulla. | I dettagli mostrano il messaggio di zfs. |
| `safety-name-too-long` | Il nome del dataset è troppo lungo per uno snapshot di sicurezza. | Disattiva lo snapshot di sicurezza, oppure ripristina in una cartella. |
| `dataset-exists` | Esiste già un dataset con questo nome. | Scegli un nuovo nome, oppure ripristina dentro il dataset stesso. |
| `create-failed` | Il nuovo dataset non si è potuto creare. | I dettagli mostrano il messaggio di zfs. Controlla che il dataset sopra di esso esista. |
| `new-dataset-not-visible` | Il nuovo dataset è stato creato, ma BombVault non lo vede, quindi non è stato ripristinato nulla. | Il dataset resta sul server. Montalo sotto il percorso di Host Data e ripristina al suo interno. |
| `set-properties-failed` | Le proprietà salvate non si sono potute impostare, quindi non è stato ripristinato nulla. | I dettagli mostrano il messaggio di zfs. |
| `set-limits-failed` | I file sono stati ripristinati, ma la quota o la riserva salvate non si sono potute impostare. | I dettagli mostrano il messaggio di zfs. Imposta tu la quota o la riserva con `zfs set`. |

### Controllare cosa vede il container {#mountinfo}

**Prova l'accesso agli snapshot** nella scheda Impostazioni dell'elemento fa un vero snapshot del suo albero, lo cerca dentro BombVault per ogni dataset e lo rimuove di nuovo. È il modo più rapido per provare l'intero percorso prima della prima esecuzione pianificata.

Per guardare tu stesso, esegui questo sul server:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Ogni riga è un mount dentro il container. La riga di un dataset mostra il suo percorso dentro il container (sotto `/host/user`) e il nome del dataset. Un campo `master:N` su quella riga significa che il mount riceve i mount che l'host crea in seguito, ed è ciò di cui ha bisogno l'accesso agli snapshot. Quando manca, imposta l'Access Mode di Host Data su Read/Write - Slave e riavvia BombVault.

## TrueNAS SCALE {#truenas}

- Quando `LIBVIRT_URI` è impostato (come per i backup delle VM su TrueNAS), BombVault prende host SSH, utente e porta per i suoi comandi zfs dall'URI, ciascuno che non è impostato da solo. Senza backup delle VM non c'è nulla da impostare: `host.docker.internal` non si risolve in un'app TrueNAS, quindi BombVault raggiunge l'host tramite il gateway predefinito dell'app, come root sulla porta 22. Imposta `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` e `LIBVIRT_SSH_PORT` quando vuoi un altro indirizzo o utente, nell'`environment` dell'app.
- Aggiungi la chiave SSH dell'host da **Impostazioni, Integrazioni, SSH dell'host** a root in **Credentials, Users**, e attiva il servizio SSH in **System, Services**. Una [replica](#replica-push) verso questo o un altro TrueNAS richiede anche la chiave della replica, che è diversa.
- Un utente diverso da root ha bisogno del permesso sul dataset in cima all'elemento, che copre allora ogni dataset sotto di esso:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Ripristinare in un nuovo dataset richiede anche `create` sul dataset sopra di esso, e impostare le proprietà salvate richiede il permesso per quelle proprietà.

  Una sessione SSH non root su TrueNAS non ha `/usr/sbin` nel suo path. BombVault chiama allora direttamente `/usr/sbin/zfs`.
- **Host Data** dell'app deve essere un percorso dell'host sopra i dataset, per esempio `/mnt/tank`, non un ixVolume. Con un percorso dell'host l'app passa a BombVault i nuovi mount dell'host (`rslave`), di cui ha bisogno l'accesso agli snapshot. Vedi [TrueNAS SCALE](getting-started.md#truenas) per l'intera configurazione dell'app.
