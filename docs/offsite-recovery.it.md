# Off-site e ripristino

!!! note "Le copie off-site attendono dopo una ricostruzione"
    Quando la procedura guidata di emergenza o **Scopri backup** ricostruisce elementi senza le vecchie impostazioni, le copie off-site di quelle sezioni si mettono in pausa finché il loro valore predefinito non viene confermato. Vedi [Dopo una ricostruzione](#after-a-rebuild).

I backup locali ti proteggono da un container perso o da un aggiornamento andato male. Le copie off-site e un kit di ripristino testato ti proteggono dall'intera macchina, dal ransomware o da un incendio. Questa pagina copre le destinazioni e le copie off-site, il rendere una copia a prova di manomissione, il dimostrare di poter ripristinare, l'associazione di più server e il recupero di tutto quando BombVault stesso non c'è più.

## Copie off-site {#off-site-copies}

Mantieni il backup locale veloce e aggiungi una o più destinazioni fuori sede. BombVault vi copia i nuovi backup con `restic copy` su base best-effort, così un intoppo off-site non fa mai fallire il backup locale. In questa forma il repo locale resta il primario e il repo off-site è una copia, ma il repo primario di una sezione non deve per forza essere locale; vedi [Repository primari remoti](#remote-primary-repositories) più sotto per eseguire il backup direttamente su S3, un rest-server e simili invece di copiarvi.

- **Più destinazioni per sezione.** Ogni sezione (container, VM, flash, Auto-backup, cartelle e dataset ZFS) può copiare su più destinazioni off-site contemporaneamente, così puoi tenere, per esempio, un rest-server sulla macchina di un amico e un bucket S3 in parallelo. La scheda **Destinazioni per sezione** della pagina Destinazioni attiva o disattiva ogni linea da una sezione a una destinazione.
- **Quando girano le copie.** La scheda **Copia** di una destinazione imposta **Quando copiare**: **Dopo ogni backup**, oppure un piano proprio che raccoglie i backup e copia a orari fissi. In **Impostazioni, Calendario, Copie off-site per sezione**, **Eccezione** dà a una sezione un piano di copia tutto suo. **Replica ora** nella scheda **Copia off-site** di una sezione, in **Impostazioni, Destinazioni**, avvia una copia su richiesta.
- **La conservazione** si imposta per destinazione sotto **Per quanto tempo conservare**, così puoi tenere le copie off-site più a lungo come archivio. La copia nel campo off-site di una sezione segue la scheda **Conservazione off-site** in **Impostazioni, Destinazioni**; lasciala tutta a zero per non tagliare mai automaticamente i backup off-site.
- **I limiti di banda** (**Larghezza di banda off-site** in **Impostazioni, Destinazioni**, oppure la scheda **Copia** di una destinazione) limitano la velocità di upload/download di restic così le copie non saturano la tua WAN.
- Mentre una copia è in corso, l'intestazione di stato della panoramica dice **Copia off-site in corso**. È un indicatore attivo, non una barra di percentuale, perché `restic copy` non espone alcun progresso leggibile da una macchina.

!!! note "Ripristina da qualsiasi luogo"
    Ogni elemento elenca i propri backup come un'unica cronologia su tutti i luoghi in cui si trova un backup, nella sua scheda **Backup**. Un backup copiato su B2 compare una sola volta, contrassegnato con ogni luogo che lo contiene. Un ripristino usa il primo luogo che riesce a raggiungere, a partire dal repository su cui l'elemento viene scritto, e puoi scegliere un altro luogo per ogni backup. Le destinazioni off-site vengono lette solo quando fai clic su **Controlla**, perché elencare lo storage cloud richiede tempo. L'eliminazione in un luogo controlla prima gli altri e dice se era l'ultima copia.

## Destinazioni {#destinations}

Una destinazione è un posto in cui vanno i backup: una cartella locale, una condivisione, un server tuo o un servizio cloud. Si imposta una volta sola e poi serve tutte le sezioni. Apri **Destinazioni** dal suo riquadro nelle impostazioni, oppure attiva lì **Mostra anche nella barra laterale** per fissarla.

**Aggiungi destinazione** apre una finestra in quattro passi:

1. **Fornitore.** Ogni servizio è elencato con il suo logo, con una ricerca in alto a destra nella finestra, in gruppi: servizi di archiviazione con bucket S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 e altri), il tuo server S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), il tuo server e le condivisioni (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, un percorso montato), lo storage cloud (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud e tutto il resto che rclone supporta) e i server riceventi del tuo gruppo. Ognuno indica quanto è adatto ai backup: i cloud drive rallentano con molte richieste, quindi il primo backup e la potatura richiedono più tempo.
2. **Accedi.** I campi dipendono dal servizio: una chiave di accesso per S3, utente e password per WebDAV e SMB, una password per app dove l'accesso a due fattori blocca quella normale, la chiave SSH pubblica di BombVault per SFTP e la Storage Box, oppure un token per i servizi che accedono tramite browser. Per questi ultimi la finestra mostra un comando `rclone authorize` da eseguire su un computer con un browser; il token che stampa va nel campo. **Prova** verifica l'accesso prima che venga salvato qualcosa. La finestra elenca poi le cartelle della destinazione, con **Nuova cartella** per crearne una e lo spazio libero dove il servizio lo riporta. Una cartella vuota è la scelta più sicura; BombVault vi mette una cartella per sezione.
3. **Protezione.** La finestra dice senza giri di parole cosa può fare il servizio. Un rest-server in modalità append-only rifiuta l'eliminazione, e il tamper test lo verifica. Un bucket S3 può conservare le vecchie versioni con versioning e object lock, cosa che BombVault non sa ancora verificare. Un cloud drive non può rifiutare l'eliminazione in nessun caso: chi entra nel server entra anche in quella copia. Attiva **Protezione dall'eliminazione (append-only)** solo dove il lato remoto rifiuta davvero l'eliminazione; BombVault allora non pota mai lì. **Per le emergenze** dice cosa contiene il kit di ripristino per questa destinazione.
4. **Per quali sezioni.** Scegli le sezioni che salvano qui. La prima copia porta con sé tutta la cronologia. Gli elementi che hanno scelto destinazioni proprie non ricevono quella nuova da soli, e la finestra li nomina.

I servizi S3 passano dal backend S3 di restic, ed è questo che permette di applicare una classe di archiviazione e l'object lock. Ogni altro servizio passa dal rclone fornito con BombVault, e il suo remote compare poi nella scheda **Off-site (rclone)** in **Impostazioni, Destinazioni**. Un'esportazione delle impostazioni include le destinazioni; con le credenziali incluse porta con sé anche il loro accesso.

La cartella su questo server si chiama ovunque **Locale**: nelle pagine delle destinazioni, nel flusso, su un elemento e in Ripristino. Con più cartelle su questo server, ciascuna ha il proprio nome. Una destinazione creata dal campo off-site di una sezione prende il nome dal suo luogo, cioè l'ultima cartella del suo indirizzo o, se non ne ha, il suo server, e riceve un numero quando un'altra destinazione ha già quel nome. Un nome che le dai nella sua pagina resta. Dove due destinazioni con lo stesso nome stanno una accanto all'altra, ciascuna porta il suo server o la sua cartella.

Ogni destinazione ha una pagina propria:

- **Connessione** con il nome, l'interruttore **Disponibile** (spento significa che BombVault non scrive più qui, e gli elementi che salvano qui falliscono con un messaggio chiaro invece di finire altrove), la cartella, la compressione, la classe di archiviazione e **Fuori sede**. **Cambia connessione** prova e salva un nuovo indirizzo o un nuovo accesso.
- **Credenziali**: l'insieme condiviso oppure uno proprio.
- **Protezione dall'eliminazione**, con **Prova subito la protezione dall'eliminazione** e chi pota questa copia.
- **Copia**: quando copiare, e i limiti di upload e download.
- **Per quanto tempo conservare**: **Breve**, **Equilibrato**, **Lungo** o **Valori personalizzati**, con una linea del tempo dei backup che la regola conserva. **Chiedi prima di conservare meno** chiede conferma prima che una regola più breve elimini backup la cui unica copia è qui.
- **Regole per sezione**: il repository, la conservazione o la compressione propri di una sezione in questa destinazione. **Modifica le regole** le apre, e **Torna alla destinazione** riporta una sezione ai valori della destinazione.
- **Già sotto questa destinazione**: sezioni il cui indirizzo scritto a mano si trova in una cartella di questa destinazione. Prenderne una in carico lascia il repository dov'è; accesso, classe di archiviazione e protezione dall'eliminazione vengono poi dalla destinazione. BombVault controlla prima che l'accesso della destinazione apra il repository, e rifiuta di mettere un repository append-only sotto una destinazione che non lo è.
- Un recipiente che mostra quanto è piena la destinazione, con il segno di quando è piena e quante settimane restano alla crescita delle ultime settimane. Una destinazione che non riporta la propria dimensione dice **Dimensione sconosciuta**.

Un server ricevente gestito da un'altra istanza del tuo gruppo compare tra i fornitori come **Dal tuo gruppo**; vedi [Server ricevente](#receiving-server).

### Spostare e rimuovere {#move-remove}

- **Sposta** porta la copia di una sezione su un'altra destinazione, con tutta la sua cronologia. Tutto resta nel vecchio posto; BombVault non elimina nulla lì. Lo spostamento gira in background e può durare a lungo: circa 9 minuti per 100 GB tra dischi locali, ore su un collegamento lento. La panoramica lo mostra mentre è in corso.
- **Rimuovi destinazione** offre due strade. **Rimuovi destinazione, mantieni i backup** smette di scrivere lì e lascia i backup al loro posto; potrai leggerli poi in **Ripristino, Da una vecchia cartella di backup**. **Rimuovi la destinazione ed elimina i backup lì** legge prima cosa c'è memorizzato e lo elenca: quali backup spariscono, quali di questi non esistono altrove e quali repository restano perché la protezione dall'eliminazione rifiuta, degli elementi salvano ancora direttamente su di essi, un'altra destinazione o sezione vi tiene le sue copie, oppure non si sono potuti leggere. Gli elementi che perderebbero l'unico backup sono elencati a parte e vengono mantenuti, a meno che tu attivi **Elimina anche queste**. Confermi digitando il nome della destinazione, e l'eliminazione gira in background.
- Questo vale per ogni tipo di destinazione: una usata da più sezioni, una destinazione configurata per una sola sezione, un repository che hai creato tu e la cartella in cui una sezione scrive i propri backup. Gli elementi che salvano su un repository rimosso tornano a seguire la loro sezione, così come un valore predefinito della sezione che puntava lì. La cartella di una sezione non può semplicemente sparire, perché ogni sezione ne ha bisogno. La finestra chiede quindi la cartella in cui si spostano le sezioni, e ogni sezione mantiene sotto di essa il nome della propria cartella.
- Eliminare i backup di una destinazione richiede una password di accesso e un accesso all'istanza. Senza password la destinazione si può comunque rimuovere, solo i suoi backup non si possono eliminare da qui.

## Collocazione per elemento {#placement}

La scheda **Calendario e destinazione** nella scheda Impostazioni di un elemento ha una riga di pulsanti: **Locale** e un pulsante per ogni destinazione off-site della sezione. I pulsanti accesi ricevono i backup dell'elemento.

- Con **Locale** acceso, l'elemento viene scritto sul repository mostrato sotto **Salvato su** e copiato su ogni altra destinazione accesa. Spegni una destinazione e non riceve più nulla di nuovo da questo elemento. Locale da solo non copia da nessuna parte, il che va bene per dati che hanno già una seconda copia, per esempio una condivisione che vive su un NAS.
- Con **Locale** spento, l'elemento viene scritto direttamente sul repository diretto della prima destinazione accesa e da lì copiato sulle altre accese. La prima volta, una finestra crea quel repository diretto.
- Un pulsante resta sempre acceso, perché un backup ha bisogno di un posto dove andare. Per escludere un elemento dai backup, usa **Non fare il backup**.

BombVault non sposta mai i backup tra repository. Un elemento che segue la sua sezione si sposta con lei quando cambia il valore predefinito della sezione: il suo prossimo backup finisce nel nuovo posto, e i backup che ha restano dove sono. Una destinazione scelta sull'elemento stesso resta fissa dal suo primo backup in poi; la scheda **Repository** nella scheda Impostazioni la nomina. Le copie possono cambiare in qualsiasi momento. Una destinazione che non riceve più un elemento mantiene le copie che ha e le pota secondo la propria conservazione alla prossima esecuzione off-site della sezione. Dalle destinazioni append-only non si può eliminare.

Il posto che un elemento ha lasciato conserva i suoi backup. Nella scheda **Backup** dell'elemento e in **Ripristino** compare come **Solo backup precedenti**, un ripristino può ancora leggere da lì, e una potatura lì lascia stare quei backup.

**Segui l'impostazione predefinita** riporta alla sua sezione un elemento con una scelta propria. Chiede prima, dice dove vanno allora i nuovi backup e nomina cosa ogni destinazione smette di ricevere e cosa caricherebbe la prossima esecuzione.

Sotto la riga la scheda indica dove va l'elemento e cosa c'è realmente: quante sedi lo custodiscono, quando ogni destinazione è stata vista l'ultima volta, e se il 3-2-1 è rispettato. Una sede è il server con i dati originali, ogni destinazione off-site e ogni repository contrassegnato **Fuori sede**. BombVault controlla le copie e le sedi; non controlla la parte «due supporti» del 3-2-1.

### Valori predefiniti per sezione {#placement-defaults}

La scheda **Destinazioni per sezione** della pagina Destinazioni disegna il valore predefinito di ogni sezione come flusso 3-2-1. Ogni linea da una sezione a una destinazione è un interruttore, **Locale** compreso, per ogni sezione, quindi anche per la chiavetta Flash, i dataset ZFS e l'auto-backup. Un clic su una linea o su un punto la cambia subito, senza domande; le linee tratteggiate sono spente. **Grafico / Interruttori** mostra lo stesso come righe di pulsanti. Un interruttore decide dove vanno i nuovi backup. Gli elementi senza una scelta propria e le cartelle di progetto degli stack Compose lo seguono subito, quindi il loro prossimo backup va nei nuovi posti. I backup già salvati restano dove sono e da lì si possono ancora ripristinare. La flash, i dataset ZFS e l'auto-backup salvano nella loro cartella locale e copiano off-site da quel backup. Anche la loro linea verso **Locale** si può disattivare; i loro backup vanno allora direttamente alle loro destinazioni off-site, e un elemento ZFS con una replica e senza alcuna destinazione invia solo la sua replica.

Con **Locale** spento e solo destinazioni off-site accese, una sezione salva direttamente su di esse, e BombVault sceglie da sé il repository lì. Container, VM e set di cartelle vanno nel repository diretto della prima destinazione accesa, descritto sotto, e da lì sono copiati sulle altre. La chiavetta Flash, i dataset ZFS e l'auto-backup scrivono ogni backup su ogni destinazione accesa, e ognuna li conserva secondo la propria conservazione.

Una sezione con ogni linea spenta dice **Non salvato**. I suoi elementi non ricevono backup e contano come non protetti.

Gli elementi che hanno scelto proprie destinazioni nella loro pagina le tengono. Il flusso dice quanti sono in una sezione, per esempio **2 elementi differiscono**, e porta a essi: alla scheda Impostazioni dell'elemento se ce n'è uno, a un elenco con dove salva ciascuno se sono più. Lì **Segui l'impostazione predefinita** riporta indietro un elemento.

Una nuova destinazione off-site riceve ogni elemento non impostato su Locale. La finestra che la aggiunge indica quanti elementi e, dove noto, quanta cronologia significa.

### Repository diretti

Un repository diretto è il posto in cui gli elementi salvano direttamente su una destinazione off-site. BombVault lo mette accanto alla cartella della destinazione, con `-direct` aggiunto al nome, oppure, quando la destinazione sta alla radice del suo bucket, del suo remote o di un utente rest-server, in una cartella al suo interno col nome della sezione, per esempio `containers-direct`. Nel flusso della pagina Destinazioni viene creato senza domande. Spegnere Locale per un singolo elemento, in modo che una destinazione senza repository diretto diventi la sua casa, apre una finestra con la posizione suggerita, per esempio `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, e un test di connessione che non crea nulla. **Crea e usa** crea il repository e vi punta l'elemento. Un repository diretto prende la chiave, la classe di archiviazione, i limiti, l'impostazione append-only e la conservazione della destinazione, e cambia insieme a essi; la scheda Repository lo mostra in sola lettura. Quando una nuova chiave della destinazione non riesce ad aprirlo, il repository diretto mantiene la chiave che ha e il salvataggio lo segnala. Un elemento su un repository diretto viene copiato da lì sulle altre destinazioni accese, mai sulla destinazione a cui il repository appartiene. I suoi backup portano il tag `bv:direct`, e ogni altro passaggio di conservazione li risparmia, così un repository diretto che ha perso il collegamento con la sua destinazione non invecchia mai secondo le regole locali. Si accede a B2 tramite il suo endpoint S3, inserendo l'ID chiave e la chiave applicativa come credenziali S3; una chiave limitata alla cartella propria della destinazione non riesce a raggiungere la cartella accanto, quindi limita invece la chiave alla cartella sopra.

### Fuori sede

Una destinazione o un repository con nome può essere contrassegnato **Fuori sede**. Quelli remoti partono contrassegnati; disattivalo per un rest-server nello stesso edificio. Il contrassegno conta solo le sedi e il 3-2-1 sulle schede. Non cambia nessuna copia.

Una replica ZFS è una copia fuori sede, ma non un backup con cronologia. Conserva solo gli snapshot che imposti, e le versioni vecchie e i singoli file vengono sempre dai backup. Va da un server ZFS direttamente all'altro, via SSH verso un server ZFS o verso l'endpoint di ricezione di un'istanza associata. Il gruppo di associazione trasporta solo la richiesta. Vedi [Replica](zfs-datasets.md#replica).

### Dopo una ricostruzione {#after-a-rebuild}

Le scelte di copia vivono nelle impostazioni di BombVault stesso. Dopo una ricostruzione tramite **Scopri backup** o la procedura guidata di emergenza senza impostazioni ripristinate sono perse, e copiare tutto rimanderebbe su B2 gli elementi che avevi escluso. Le copie off-site di ogni sezione ricostruita quindi si mettono in pausa. La panoramica lo segnala, e la scheda **Destinazioni per sezione** offre **Conferma valore predefinito** con un'anteprima di cosa copia la prossima esecuzione e i nomi nei backup che non hanno un elemento, che puoi escludere lì. Solo la conferma termina la pausa; importare un file di impostazioni riporta regole e valori predefiniti ma non la termina.

## Repository primari remoti {#remote-primary-repositories}

Il percorso proprio di una sezione non si limita a una cartella locale: puntalo direttamente a un remoto restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:utente@host:/repo`, `rclone:remoto:bucket/percorso`) e BombVault salva lì direttamente, senza copia locale separata e senza passo di copia. È una forma diversa dalle copie off-site viste sopra: là il repository locale è il primario e quello off-site ne è un archivio per quanto possibile; qui il repository remoto **è** il primario, ed è l'unica copia finché non copi quella sezione anche su un'altra destinazione.

Il percorso di ogni sezione (Container, VM, Flash, Auto-backup, Cartelle, Dataset ZFS) si imposta nella pagina della destinazione **Locale**: **Cambia** accanto alla sezione, nella sua scheda **Connessione**, apre una finestra con un interruttore **Locale / Remoto**:

- **Locale** mostra il consueto sfoglia-cartelle.
- **Remoto** lo sostituisce con un semplice campo URL e le impostazioni di connessione che usa una destinazione off-site, configurate però per questo primario. Da lì ottieni:
    - **Un test di connessione** contro il percorso reale, prima di farci affidamento.
    - **Limiti di banda** (invio e ricezione), perché un backup pianificato verso un primario remoto non saturi la tua linea WAN: le stesse opzioni restic `--limit-upload` e `--limit-download` usate dalle copie off-site, applicate al backup stesso.
    - **Protezione append-only**, verificata con lo stesso test di manomissione attivo (una vera sonda DELETE verso l'altro capo) che ricevono le destinazioni off-site. Con essa attiva, BombVault si rifiuta di potare il repository, perché dietro non c'è una copia locale separata e le credenziali su questa macchina non devono poter cancellare l'unica copia del backup.
    - **Un allarme sul budget di crescita**, ricavato dallo stesso andamento della dimensione del repository che la scheda Archiviazione già segue.

Niente di tutto questo è obbligatorio: un percorso remoto scritto a mano senza impostazioni di sicurezza salvate esegue il backup senza limiti di banda, potabile e senza allarme di budget. Le impostazioni di sicurezza servono per quando vuoi le protezioni che ottiene una copia off-site, senza una destinazione separata solo per quello.

!!! note "Le credenziali cloud e REST sono condivise"
    Un primario remoto si autentica con le credenziali cloud condivise in **Impostazioni, Destinazioni**; non esiste un archivio di credenziali separato per i repository primari.

### SMB e WebDAV senza mount sull'host {#smb-webdav}

La scheda **Off-site (rclone)** in **Impostazioni, Destinazioni** ha un modulo per una condivisione Windows o Samba e per un server WebDAV (Nextcloud, ownCloud, SharePoint o qualsiasi altro). Inserisci un nome breve, host e condivisione (SMB) oppure URL e tipo di server (WebDAV), utente e password, e BombVault scrive la sezione rclone per te. rclone offusca la password da sé prima che venga salvata; aggiungerne una con un nome già esistente sostituisce quella sezione invece di aggiungerne una seconda. **Aggiungi destinazione** offre SMB e WebDAV come fornitori con gli stessi campi.

Il modulo risponde con la posizione completa, per esempio `rclone:nas:backups`. Usala come percorso di una sezione o come destinazione e aggiungi una sottocartella se vuoi (`rclone:nas:backups/bombvault`). La condivisione è il primo segmento del percorso, non fa parte del nome.

È una strada migliore del montare la condivisione su Unraid: restic sconsiglia di tenere un repository su una condivisione CIFS montata, e qui non viene montato nulla. NFS non è nel modulo perché né restic né rclone hanno un backend NFS; per NFS, monta l'export sull'host e usalo come cartella locale.

## Protezione dall'eliminazione (append-only)

Attiva la protezione dall'eliminazione per una destinazione off-site così ransomware, o un host compromesso, non possano eliminare o riscrivere i tuoi backup. L'altro lato (un `restic/rest-server` in esecuzione in modalità `--append-only`) la **impone**. BombVault la **verifica** soltanto e non mostra mai verde sulla sola affermazione di una configurazione. Una cartella su questo server non può rifiutare l'eliminazione, perché chi controlla BombVault controlla anche la cartella, quindi le destinazioni locali non offrono l'interruttore.

L'interruttore appartiene alla destinazione: una sezione che copia lì lo prende dalla destinazione, quindi si attiva nella pagina della destinazione, in **Destinazioni**. Il controllo pianificato **Controlla la protezione dall'eliminazione** gira solo finché una sezione copia verso una destinazione con l'interruttore attivo. L'altra parte va avviata con `--append-only`; con l'immagine `restic/rest-server`, su TrueNAS come su qualsiasi altro host Docker, va nella variabile `OPTIONS`.

La via più rapida a una copia append-only è un BombVault associato con un [server ricevente](#receiving-server): esegue il rest-server per te, in modalità append-only.

!!! note "Una cancellazione riuscita sotto `/locks/` è prevista"
    Append-only non significa che non si possa più cancellare nulla. restic deve prendere e rilasciare i propri lock, quindi `/locks/` resta scrivibile e cancellabile di proposito. Gli snapshot e i dati che stanno dietro, cioè proprio ciò che un ransomware cercherebbe, non possono essere rimossi. Se sondi tu stesso il lato remoto, una cancellazione che riesce sotto `/locks/` è il comportamento corretto e non una falla.

!!! warning "I repo append-only non vengono mai potati da questa macchina"
    Una destinazione append-only non pota mai i vecchi backup da qui; potare è compito dell'altro lato. Impostale un **allarme del budget di crescita** così vieni avvisato prima che la dimensione del repo sfugga di mano.

## Tamper test

BombVault dimostra periodicamente la garanzia append-only tentando effettivamente un'eliminazione contro il repo off-site, mirata a un oggetto inesistente. **Prova subito la protezione dall'eliminazione** nella pagina di una destinazione la esegue su richiesta, e **Controlla la protezione dall'eliminazione** in **Impostazioni, Verifica, Controlli automatici** la esegue secondo una pianificazione.

Una copia disattivata non viene testata, e la destinazione lo dice. Il test di connessione funziona comunque, perché legge soltanto, così una copia si può verificare prima di attivarla.

- **Rifiutata** significa protetto.
- **Accettata** significa non protetto.
- Un risultato **inconcludente** (server irraggiungibile, errore di autenticazione) non ribalta mai il verdetto memorizzato.

Un reale passaggio da protetto a non protetto fa scattare un unico avviso.

## Esercitazioni DR

BombVault offre due livelli di prova che i tuoi backup siano effettivamente ripristinabili, non solo presenti.

- **Esercitazioni di verifica del ripristino (locali).** BombVault esegue periodicamente `restic check --read-data-subset` (limitato, mai un ripristino completo che riempie il disco). La cadenza si imposta in **Impostazioni, Verifica, Controlli automatici**, e la scheda **Controlla i backup** nello stesso riquadro mostra il risultato.
- **Ripristino di prova (off-site).** BombVault ripristina una destinazione reale dal repo off-site in una sandbox isolata, la verifica file per file e byte per byte, poi ripulisce. Questo dimostra che puoi recuperare da off-site, non solo che il repo risponde.

La scheda **Stato di protezione** nella panoramica riassume tutto questo per sezione in quattro controlli: un backup recente su questo server, una copia off-site, la protezione dall'eliminazione lì e un ripristino di prova da lì. Un clic su una sezione mostra i dettagli, e la scheda diventa verde solo su fatti verificati.

## Associazione delle istanze {#pairing}

Più BombVault lavorano insieme come un unico gruppo: si mostrano a vicenda lo stato di protezione, si assumono ruoli l'uno per l'altro e possono custodire la chiave l'uno dell'altro. Un'istanza si unisce al gruppo con dodici parole.

Sulla prima istanza apri **Impostazioni, Associazione** e premi **Genera frase**. Tre schede di passaggio mostrano come procedere. Appaiono dodici parole in una finestra con un pulsante **Copia**. Su ogni altra istanza apri lo stesso posto, premi **Inserisci frase** e incollale o digitale, oppure premi **Incolla** in quella finestra. Una parola che non è nell'elenco viene indicata con la sua posizione già mentre la digiti, e l'ultima parola porta un checksum, così una parola digitata male o scambiata viene rilevata prima che avvenga l'associazione. Genera la frase su una sola istanza: due istanze che creano entrambe una frase formano due gruppi separati. Se per un minuto non si presenta nessuno, la pagina offre due vie d'uscita: mostrare di nuovo le parole per inserirle dall'altra parte, oppure inserire le parole dell'altra istanza e unirsi al suo gruppo in un solo passaggio. L'associazione funziona anche senza una password di accesso, ma impostane una: senza, chiunque possa aprire questa interfaccia web può leggere le parole e chiedere, tramite il gruppo, ruoli che consegnano la password restic delle istanze al suo interno. La scheda di associazione lo segnala finché non viene impostata una password. Con una password, mostrare di nuovo la frase la richiede. **Lascia il gruppo** fa uscire di nuovo un'istanza. La scheda **Istanze** della pagina di associazione elenca i membri del gruppo; l'immagine di chi fa cosa per chi è nella pagina Istanze.

Chiunque conosca le parole può unirsi al gruppo, trattale quindi come una password.

**Come i membri si raggiungono.** Ogni istanza apprende il proprio indirizzo sulla rete dal tuo browser nel momento in cui accedi, elencato sotto **Indirizzi noti** nella scheda **Questa istanza** della pagina di associazione insieme a quelli che l'istanza ha trovato da sé; correggilo lì se davanti c'è un reverse proxy o una porta insolita. Sulla stessa rete i membri annunciano quell'indirizzo tramite multicast e si parlano direttamente, e dove il multicast non riesce ad attraversare una rete di contenitori, come la rete bridge predefinita di Docker, un'istanza cerca invece le altre nella propria subnet con una chiamata firmata a cui solo un membro del gruppo sa rispondere, così l'associazione si completa comunque in pochi secondi senza relay. Se non salta fuori nulla, **Non la trovi?** accetta un indirizzo inserito a mano, per un'altra subnet o una porta non standard. Le istanze su reti diverse passano da un relay, scelto in una riga di quattro nella scheda **Relay** della stessa pagina; ogni forma ha un'immagine:

- **Relay del progetto** (predefinito): `parleyport.halleluja.design`, lo stesso relay usato anche da KnightLoader. Niente da configurare.
- **Relay proprio**: il container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) dalle Unraid Community Apps (anche in **Impostazioni, App**). Inserisci il suo indirizzo su ogni istanza che deve usarlo.
- **Istanza BombVault**: una delle tue istanze già raggiungibile dall'esterno, con **Funge da relay** attivato. Quell'istanza risponde poi su `/relay/connect` al proprio indirizzo, dietro il reverse proxy e il certificato che già possiede, e fa entrare solo il tuo gruppo. Inserisci il suo indirizzo su ogni istanza che deve usarla.
- **Nessun relay**: i membri si trovano automaticamente solo sulla stessa rete, e in nessun altro posto.

**Cosa vede il relay.** Un pulsante con lo stesso nome accanto alle quattro forme lo mostra per la forma scelta. Ogni chiamata tra membri è sigillata con AES-256-GCM sotto una chiave derivata dalle dodici parole, e quella chiave non lascia mai le tue istanze. Il relay apprende un hash che raggruppa le connessioni, a quale istanza è destinato un messaggio, quanto è grande e quando passa. Una chiamata diretta sulla rete locale è sigillata allo stesso modo e anche firmata, così nulla dipende dal certificato autofirmato che un'istanza serve. Le chiamate che consegnano qualcosa (risposte ai ruoli, una password restic, accessi dei riceventi, depositi di chiavi) sono inoltre firmate con una chiave del membro che le invia, fissata alla prima volta che la vede; vedi [Modello di sicurezza](configuration.md#security-model).

**Cosa viaggia sul gruppo.** Lo stato di protezione nella pagina Istanze, una richiesta di controllare subito una sezione, le richieste di ruolo e le loro risposte, le offerte di destinazione, e ciò di cui un ricevente o un prelevatore ha bisogno una volta consentito il suo ruolo: le posizioni del repository dell'altra istanza e la sua password restic. I dati di backup invece mai: vanno sempre direttamente ai backend restic. Nemmeno l'APP_KEY: la password restic apre i repository di quell'istanza e nient'altro, non i suoi segreti memorizzati, non le sue sessioni, non i suoi codici di ripristino.

**Voci precedenti all'associazione.** Le istanze aggiunte con un token di flotta, e i repository ricevuti e le fonti di prelievo configurati con l'APP_KEY dell'altra istanza, restano dopo l'aggiornamento e sono contrassegnati **Associa di nuovo**. Continuano a funzionare: al primo avvio BombVault sostituisce ogni APP_KEY memorizzato con la password restic da esso derivata. Associa entrambe le istanze, poi modifica la voce e scegli la sua istanza. Finché non lo fai, queste voci stanno nella scheda **Senza istanza** della pagina Istanze.

L'unico posto che richiede ancora un APP_KEY a mano è [Da una vecchia cartella di backup](#restore-from-another-bombvault-repo), per il caso in cui l'altra istanza sia scomparsa e non possa rispondere in un gruppo.

## Istanze e ruoli {#roles}

La pagina **Istanze** (un riquadro delle impostazioni che si può fissare alla barra laterale) mostra il gruppo come una rete di chi fa cosa per chi, una scheda per questa istanza e una per ogni altro membro, i telefoni Android e i server ZFS. **Associazione** in basso a destra porta alla pagina di associazione. Quali parti sono attive si imposta in **Impostazioni, Istanze, Più server**: **Istanze**, **Ricevitore** e **Prelievo**.

Ogni scheda mostra lo stato di protezione del membro e i suoi ruoli come riquadri. Un ruolo è qualcosa che un'istanza fa per un'altra:

- **Ricevitore** tiene le copie off-site di questo server in un rest-server in modalità append-only. Questo server invia dopo ogni backup e non può eliminare nulla lì, nemmeno se qualcuno lo prende in mano.
- **Prelevatore** preleva i backup di questo server secondo una pianificazione propria. Questo server non deve essere acceso per questo.
- **Server ZFS** accoglie le repliche ZFS di questo server. Ogni elemento ZFS lo sceglie sotto **Replica** nelle sue impostazioni.
- **Custode della chiave** custodisce una copia sigillata del kit di ripristino di questo server. Vedi [Custode della chiave](#key-keeper).

Attivare un ruolo invia una richiesta, e non parte nulla finché qualcuno sull'altra istanza non preme **Consenti**. La richiesta compare sulla scheda di quell'istanza come badge, e i suoi **Dettagli** la elencano con **Consenti** e **Rifiuta**; un ruolo consentito si può revocare lì in qualsiasi momento. Una linea tratteggiata nella rete è un ruolo ancora in attesa. Un'istanza con BombVault 9.x non può rispondere a una richiesta, quindi un ruolo è attivo lì appena lo attivi, e una richiesta da un'istanza 9.x aspetta **Consenti** qui.

**Dettagli** su una scheda apre il resto: i repository ricevuti e le fonti di prelievo di quel membro, le offerte di destinazione, il monitoraggio, **Rimuovi** e **Dimentica la chiave**.

## Ricevente e prelevatore {#receiver}

![Una copia off-site ricevuta nei dettagli di un'istanza, osservata in sola lettura, con un controllo di integrità eseguito su questa macchina.](assets/screenshots/receiver.png)

*Una copia off-site ricevuta nei dettagli di un'istanza, osservata in sola lettura, con un controllo di integrità eseguito su questa macchina.*

Sulla macchina che **riceve** copie append-only da un altro BombVault, i repository ricevuti ti offrono un monitoraggio indipendente e in sola lettura sull'hardware ricevente, così un fallimento silenzioso all'altra estremità non passa inosservato. Attiva **Ricevitore** in **Impostazioni, Istanze**, poi apri i **Dettagli** dell'istanza mittente nella pagina Istanze e aggiungi il repository ricevuto (in sola lettura, aperto con la password restic dell'istanza mittente, che ottiene tramite il [gruppo di associazione](#pairing)). Ottieni:

- **Un inventario per sorgente**, così puoi vedere esattamente quali container, VM e set di cartelle sono arrivati.
- **Ultimo ricevuto** per sorgente, così sai quanto è fresco ciascuno.
- **Un `restic check` indipendente** eseguito sull'hardware ricevente, così l'integrità viene verificata dove i dati effettivamente risiedono, non solo sul mittente.
- **Un dead-man's switch:** un avviso quando una sorgente smette di inviare entro una finestra che imposti.
- **Avvisi di integrità:** un avviso quando un controllo sul lato ricevente fallisce.

Il ricevente è rigorosamente in sola lettura. Non scrive mai nel repository ricevuto, così non può mai rompere la garanzia append-only su cui il mittente fa affidamento.

Un prelevatore lavora al contrario: con **Prelievo** attivo, un'istanza legge i backup di un altro membro nel proprio repository secondo una pianificazione propria. Le sue sorgenti stanno anch'esse nei dettagli di quel membro.

### Server ricevente {#receiving-server}

La macchina ricevente può anche eseguire il rest-server su cui copiano gli altri. Si imposta la prima volta che consenti una richiesta **Ricevitore** che lo richiede: la finestra dice che serve un server ricevente e chiede una cartella su una condivisione, con **Nuova cartella** per crearne una, e una porta (8000, a meno che la usi un altro container). BombVault poi:

1. rifiuta se esiste già un container chiamato `rest-server` o se un altro container occupa la porta;
2. scarica `restic/rest-server` e lo avvia tramite il socket Docker in modalità append-only, con repository privati e un file di accessi nella cartella;
3. scrive il suo template Unraid sul flash, così il container resta modificabile nella scheda Docker, oppure offre il template come download quando il flash non è raggiungibile;
4. esegue il tamper test e mostra se rifiuta le eliminazioni.

Le istanze del tuo gruppo trovano poi il server tra i fornitori sotto **Dal tuo gruppo**, con il nome della macchina ricevente, e il ruolo **Ricevitore** lo propone come scelta sicura. Ogni istanza riceve un accesso proprio la prima volta che usa il server e scrive lì solo nella propria cartella. Con **Ricevitore** attivo in **Impostazioni, Istanze**, **Server ricevente** sulla scheda di questa istanza riapre il server. La sua finestra elenca gli accessi sotto **Partner con accesso**, e **Revoca accesso** ne toglie uno; ciò che quell'istanza ha già copiato resta nella cartella. L'impostazione crea anche un **Accesso fuori dal gruppo**, la cui password la finestra mostra una sola volta.

Un'istanza che raggiunge la macchina ricevente solo tramite il relay non può usare il server, perché il relay non trasporta backup. Aggiungi prima l'indirizzo della macchina ricevente nella pagina di associazione. Quando BombVault gira su un indirizzo IP proprio (su br0, per esempio), compila **Indirizzo per i partner**, perché il server ascolta sull'indirizzo dell'host.

## Esempio completo: due macchine Unraid, dall'inizio alla fine

Sopra sono descritti i pezzi. Questa è un'installazione completa, perché i pezzi si montano meglio dopo averli visti montati una volta.

Due macchine: **TOWER** esegue i container e invia i backup, **VAULT** li riceve e impone l'append-only. Sostituisci con i tuoi nomi, indirizzi e percorsi di condivisione.

**1. Associa le due macchine.** Su TOWER apri **Impostazioni, Associazione**, premi **Genera frase** e inserisci le dodici parole su VAULT con **Inserisci frase**. Imposta prima una password di accesso su entrambe.

**2. Su TOWER, chiedi a VAULT di essere il ricevitore.** Nella pagina Istanze attiva **Ricevitore** sulla scheda di VAULT, scegli le sezioni e **il server ricevente di VAULT** come posto, e invia la richiesta.

**3. Su VAULT, consentila.** La pagina Istanze di VAULT mostra la richiesta sulla scheda di TOWER. Premi **Consenti**. La prima volta VAULT chiede una cartella per il suo server ricevente, come `user/appdata/rest-server`, avvia il rest-server in modalità append-only ed esegue il tamper test.

**4. Di nuovo su TOWER.** Da quel momento TOWER copia su VAULT dopo ogni backup, e la nuova destinazione compare nella pagina Destinazioni di TOWER con la sua protezione dall'eliminazione.

Cosa risponde il tamper test:

| Risultato | Cosa è successo |
| --- | --- |
| **protetto** | VAULT ha rifiutato la cancellazione. È l'unico stato che passa. |
| **non protetto** | VAULT ha accettato una cancellazione. Manca `--append-only` oppure è stato rimosso. |
| **non conclusivo** | Né l'uno né l'altro. Di solito l'URL non è quello che usa restic, oppure le credenziali sono cambiate. Non viene registrato nulla e non scatta alcun avviso. |

**5. Guarda cosa arriva, se vuoi.** Attiva **Ricevitore** in **Impostazioni, Istanze** su VAULT, apri i **Dettagli** di TOWER e aggiungi il repository ricevuto. VAULT ottiene la password restic di TOWER tramite il gruppo; nessuno digita una chiave.

**6. Rendilo reciproco.** Ripeti i passi nella direzione opposta: VAULT chiede a TOWER di essere il suo ricevitore. Ogni macchina impone allora l'append-only all'altra, e nessuna può cancellare i backup dell'altra.

Funziona anche un rest-server che gestisci a mano: aggiungilo con **Aggiungi destinazione** come rest-server, con utente e password dal suo `.htpasswd`, e attiva la protezione dall'eliminazione. Tieni `--append-only` nelle sue opzioni; senza, l'altro lato torna a essere una normale condivisione.

!!! warning "Un percorso locale è un percorso dentro il container"
    Dove BombVault chiede una cartella su questo server, inseriscila relativa al mount dell'host, come `user/appdata/rest-server/bombvault-containers`, non `/mnt/user/appdata/…`. BombVault gira in un container in cui il `/mnt` dell'host è montato altrove; un percorso host assoluto lì non esiste. Se ne incolli uno, BombVault ti indica il percorso relativo da usare.

## Procedura guidata di emergenza {#emergency}

La **Procedura guidata di emergenza** riporta in vita un server perso o configurato di nuovo. Parte dalla pagina **Ripristino** sotto **Emergenza**, e da **Ripristina da un backup** nella pagina di benvenuto di una nuova installazione. Si svolge in quattro passi: **Accesso**, **Impostazioni**, **Selezione** e **Ripristino**.

1. **Accesso.** Mostra a BombVault la chiave dei vecchi backup, in uno di tre modi:
    - **Carica il kit di ripristino**: il file che hai scaricato durante la configurazione. Contiene la chiave e ogni destinazione, e BombVault compila il resto da lì. Un kit sigillato con age chiede l'identità per cui è stato sigillato; viene usata una volta e non memorizzata.
    - **Tramite il gruppo**: un'istanza del tuo gruppo custodisce la chiave come deposito. Servono le dodici parole del gruppo, il tuo codice di sblocco e qualcuno presso quell'istanza. Vedi [Custode della chiave](#key-keeper).
    - **Digita la chiave**: l'`APP_KEY` del vecchio server, e dove si trovano i suoi backup.

    BombVault prova poi la chiave su ogni destinazione e dice per ciascuna se è leggibile, non trovata, bloccata con un'altra chiave, non raggiungibile o ancora priva del suo accesso. Non è stato ancora salvato nulla, e la chiave resta in memoria per un tempo limitato. **Adotta la chiave** fa scrivere a questo server i suoi backup con quella chiave da ora in poi; **Annulla l'adozione** torna alla chiave propria finché non è stato eseguito il primo backup. Un server che ha già backup propri rifiuta l'adozione e rimanda a [Da una vecchia cartella di backup](#restore-from-another-bombvault-repo). **Salva le credenziali** memorizza gli accessi che il kit porta per le destinazioni remote, e chiede conferma prima di sostituire gli accessi che questo server ha già.
2. **Impostazioni.** **Ripristina le impostazioni** ripristina le impostazioni di BombVault stesso dall'Auto-backup più recente: destinazioni, credenziali, pianificazioni e regole. BombVault si riavvia per applicarle, tramite il socket Docker, così il database delle impostazioni in esecuzione non viene mai sovrascritto sotto un handle aperto, e poi la procedura guidata prosegue da sola. L'accesso di questo server resta: continui ad accedere con la password che usi ora. I segreti che il vecchio server aveva memorizzato e che non si possono riprendere vengono elencati, così puoi inserirli di nuovo.
3. **Selezione.** BombVault passa in rassegna i tuoi backup ed elenca i container, le VM, i set di cartelle e i dataset ZFS che trova. Il backup più recente è preselezionato ovunque. Ciò che questo server ha ancora parte disattivato, perché un ripristino lo sostituirebbe. Dopo una ricostruzione i set di cartelle non sanno più da dove venivano, quindi vanno in una cartella che scegli.
4. **Ripristino.** Tutto ciò che è selezionato torna in un colpo solo, lasciato fermo se vuoi avviare tu ogni container e VM, il che è la scelta più sicura dopo una perdita totale. Poi scarica un nuovo kit di ripristino: quello vecchio appartiene all'installazione precedente.

!!! tip "Migrazione pianificata contro disastro"
    La procedura guidata di emergenza ripristina le impostazioni di BombVault stesso da un backup. Per uno spostamento *pianificato* su una nuova macchina, puoi invece portare la tua configurazione direttamente con la scheda **Esporta / importa impostazioni** (un file JSON portatile). Vedi [Configurazione](configuration.md#portable-settings-export-and-import).

### Da una vecchia cartella di backup {#restore-from-another-bombvault-repo}

**Da una vecchia cartella di backup** sotto **Altre sorgenti** nella pagina Ripristino apre il repository di un'*altra* istanza BombVault (una condivisione montata sotto `/mnt`, o un indirizzo remoto) con il kit di ripristino di quell'istanza o con l'`APP_KEY` digitato, in una sessione monouso e in sola lettura. Sfoglia i container, le VM, i set di cartelle e i dataset ZFS memorizzati lì, scegli un backup e ripristinalo, e l'oggetto ripristinato diventa un normale container, VM o set di cartelle locale. Nulla viene mai scritto nell'altro repository, e le tue impostazioni di backup restano intatte (la sessione risiede in memoria e scade da sé). Spostare un container dal server A al server B non significa ripuntare le tue impostazioni e riportarle indietro dopo. Questa finestra è monouso: apre una sessione, ripristina ciò che scegli e dimentica l'altra istanza. Se invece vuoi un accordo stabile, in cui questa macchina preleva secondo una pianificazione i backup di un'altra istanza nel proprio repository, quello è il ruolo **Prelevatore** nella pagina Istanze.

Un container la cui rete non esiste su questo server, per esempio una rete `br0` di Unraid su un normale host Docker, mostra una scelta della rete sotto la sua riga. BombVault lo crea sulla rete che scegli, insieme alle sue altre reti. L'IP fisso e l'indirizzo MAC appartenevano alla vecchia rete e vengono tolti, quindi li assegna la nuova rete.

## Custode della chiave {#key-keeper}

Un custode della chiave è un'istanza del tuo gruppo che tiene una copia sigillata del kit di ripristino di questo server. Non può aprire la copia da sé. Se questo server andasse perso, una nuova installazione che si unisce al tuo gruppo chiede la copia al custode e la apre con il tuo codice di sblocco.

1. Nella pagina Istanze attiva **Custode della chiave** sulla scheda dell'istanza che deve custodirla e scegli **Crea deposito**. Per creare un deposito serve una password di accesso.
2. Questo server mostra il **Codice di sblocco** una sola volta: venti lettere e cifre in quattro gruppi da cinque. Scrivilo e conferma con **Ho annotato il codice**. Questo server non memorizza il codice, solo ciò che gli serve per sigillare un deposito per esso.
3. Il custode ora tiene il deposito, e questo server lo rinnova quando il kit cambia. **Deposito chiave** sulla scheda di questa istanza elenca **Deposito di questo server** con ogni custode e il suo stato, e **Depositi custoditi da questo server** per gli altri. **Crea un nuovo codice** sostituisce il codice; ogni custode passa al nuovo quando riceve il nuovo deposito.

Quel giorno servono tre cose: le dodici parole del tuo gruppo, il codice di sblocco e l'accesso all'interfaccia del custode, dove qualcuno deve premere **Consenti**. Nella procedura guidata di emergenza scegli **Tramite il gruppo**, scegli il deposito, richiedilo e digita il codice di sblocco. Il codice viene usato sul nuovo server e non viene inviato al custode. Sul custode la richiesta compare sotto **Deposito chiave** con chi la fa, se quell'istanza era mai stata vista lì prima e quante volte ha chiesto; **Consenti** consegna la copia sigillata, e consentire richiede una password di accesso. La nuova installazione ha 30 minuti per ritirarla dopo il Consenti, una richiesta a cui nessuno risponde scade dopo un giorno, e un'istanza che chiede troppo spesso deve aspettare un'ora.

Senza il codice nessuno può aprire la copia, nemmeno il custode. Se il codice va perso, creane uno nuovo finché questo server gira ancora; se il server è già sparito, il kit di ripristino e l'APP_KEY digitato funzionano ancora. Il custode può eliminare un deposito in qualsiasi momento, e il proprietario deve poi chiederlo di nuovo.

## Kit di ripristino {#recovery-kit}

Questo è il pezzo che rende possibile il disaster recovery anche quando non c'è alcun BombVault in esecuzione.

**Scarica il kit di ripristino** nella scheda **Per le emergenze** in **Impostazioni, Ripristino** ti dà la **chiave**, la **password restic derivata**, ogni **destinazione** con il repository di ciascuna sezione, gli accessi di cui una nuova installazione ha bisogno per raggiungerle e i **comandi restic esatti**, così puoi ripristinare direttamente con la CLI di restic su qualsiasi macchina. Serve una password di accesso. Il kit stampa la chiave anche con la cifratura disattivata, perché le definizioni di container e VM sono sempre sigillate con essa. Dopo un'adozione stampa la chiave dei backup adottata e l'APP_KEY di questo server, che una nuova installazione usa solo per riportare le impostazioni. Una scheda **Salva il tuo kit di ripristino** nella panoramica te lo ricorda finché non l'hai conservato.

!!! danger "Conserva il kit di ripristino fuori dal server"
    Il kit contiene il segreto che decifra i tuoi backup. Tienilo in un luogo sicuro e separato dal server (un password manager, una copia stampata in una cassaforte). Se perdi sia BombVault che `APP_KEY` senza kit di ripristino e senza deposito della chiave, i tuoi backup cifrati non possono essere recuperati.

!!! warning "Lo snapshot più recente non è sempre quello da ripristinare"
    Da restic 0.17, `restic snapshots` mostra la dimensione di ogni snapshot. Dopo una perdita di dati lo snapshot più recente può essere quello svuotato, quindi non ripristinare uno snapshot molto più piccolo dei precedenti. Dopo un ransomware può essere quello cifrato, di dimensione normale. Se BombVault funziona ancora, controlla prima la sua pagina **Anomalie**: indica l'ultimo backup buono, e la finestra di ripristino lo preseleziona. Un ripristino non ha bisogno di alcun dato sulle anomalie di BombVault, e la pausa della conservazione mantiene sempre solo più snapshot.

### Sigillare il kit

Se hai attivato **Cifra le esportazioni con age** in **Impostazioni, Sistema**, anche il kit viene sigillato con essa e si scarica come `bombvault-recovery-kit.md.age`. È in formato ASCII armor anziché binario, quindi resta testo: incollarlo in un password manager o stamparlo funziona esattamente come prima, solo che il contenuto è illeggibile senza la tua chiave.

!!! warning "Non conservare la chiave age dentro il kit"
    Ti serve la tua chiave age **privata** per aprire un kit sigillato. Tienila in un posto che non dipenda dal kit stesso, altrimenti avrai due cose da recuperare invece di una. Sigillare conviene quando il kit è conservato in un posto che non controlli del tutto (un password manager condiviso, note nel cloud, una stampa in ufficio); un kit nella tua cassaforte è già protetto dalla cassaforte.

    Con la cifratura attiva e nessun destinatario utilizzabile configurato, il download viene rifiutato del tutto. BombVault non ripiega mai sul consegnare la chiave master in chiaro.

### Se il kit non è a portata di mano

La password non è memorizzata da nessuna parte, viene **calcolata** dalla chiave dei backup. Con la chiave e una shell puoi quindi riprodurla da solo:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

È un HMAC-SHA256 sulla stringa fissa `bombvault:restic-repo`, con i byte grezzi dell'`APP_KEY` esadecimale come chiave, stampato come 64 caratteri esadecimali minuscoli. Lo stesso valore è nel kit, come password restic derivata; questo serve per il giorno in cui il kit si trova altrove rispetto a te. Su un server che ha adottato i backup di un altro, usa la chiave dei backup adottata, cioè il vecchio APP_KEY di quel server.

!!! warning "Per un repository ricevuto, usa la chiave dell'istanza MITTENTE"
    Un repository arrivato qui tramite una copia off-site è stato creato dalla macchina che lo ha inviato, con la **sua** chiave. Derivare dalla chiave della macchina ricevente produce una password che restic rifiuta, il che sembra esattamente un repository corrotto senza esserlo. È il motivo abituale per cui `restic check` su un repository ricevuto continua a chiedere la password.

Poiché le definizioni di ripristino risiedono **dentro** ogni repo (`<repo>/def`, `<repo>/vm-def`), una cartella di repo copiata è completamente autonoma, così il kit più il repo sono tutto ciò che serve per un ripristino bare-metal.

## Recuperare un dump del database {#database-dumps}

Un dump del database è un punto di ripristino a sé nel repository dei container, con l'etichetta `dbdump:<container>` e un solo file, `/dbdump/<container>.sql`. BombVault li elenca, li scarica e li importa nella scheda **Backup** del container; qui sotto ci sono gli stessi passi con il solo restic, per il giorno in cui BombVault non c'è.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Le etichette `dbversion:` e `dbname:` su ogni dump dicono da quale versione del server proviene e quali database contiene. Un file completo termina con `-- PostgreSQL database cluster dump complete` oppure `-- Dump completed`.

Importalo in un container della stessa versione o di una più recente (PostgreSQL), o della stessa versione maggiore (MySQL e MariaDB), avviato una volta con la cartella dati vuota perché si inizializzi. L'host non ha bisogno di un client del database, il container ne ha uno:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Per un singolo database dentro un dump completo, MySQL e MariaDB accettano `--one-database <name>` sul comando del client. Un dump PostgreSQL ha una sezione per database, ciascuna aperta da una riga `\connect <name>`: copia quella sezione in un file a parte e importalo con `-d <name>` dopo aver creato il database.

!!! warning "Un dump preso come root porta con sé gli utenti del server"
    Un dump completo di MySQL o MariaDB preso come root contiene il database di sistema `mysql`, quindi importarlo sostituisce gli account del nuovo server, password di root compresa, con quelli del dump. Su PostgreSQL, `role ... already exists` per l'utente creato dal container è previsto e innocuo.
