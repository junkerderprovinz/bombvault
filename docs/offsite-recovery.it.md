# Off-site e ripristino

!!! note "Le copie off-site attendono dopo una ricostruzione"
    Quando il passo 4 ricostruisce voci senza le vecchie impostazioni, la replica off-site di quei domini si mette in pausa finché la collocazione predefinita non viene confermata. Vedi [Collocazione per elemento](#placement).

I backup locali ti proteggono da un container perso o da un aggiornamento andato male. La replica off-site e un kit di ripristino testato ti proteggono dall'intera macchina, dal ransomware o da un incendio. Questa pagina copre la replica off-site, il rendere quella copia a prova di manomissione, il dimostrare di poter ripristinare e il recuperare quando BombVault stesso non c'è più.

## Replica off-site

Mantieni il backup locale veloce e aggiungi una o più repliche off-site. Imposta un repo per dominio nella pagina **Impostazioni, Off-site**. BombVault vi replica i nuovi snapshot con `restic copy` su base best-effort, così un intoppo off-site non fa mai fallire il backup locale. In questa forma il repo locale resta il primario e il repo off-site è una replica, ma il repo primario di un dominio non deve per forza essere locale; vedi [Repository primari remoti](#remote-primary-repositories) più sotto per eseguire il backup direttamente su S3, rest-server ecc. invece di replicarvi.

- **Più destinazioni off-site per dominio.** Ogni dominio (container, VM, flash, config, set di file e dataset ZFS) può replicare verso più destinazioni off-site contemporaneamente, non solo una, così puoi tenere, per esempio, un rest-server sulla macchina di un amico e un bucket S3 in parallelo. Aggiungi destinazioni extra in Impostazioni, Off-site, ciascuna con il proprio repository, classe di archiviazione S3, flag append-only, conservazione e budget di crescita. Una configurazione off-site singola esistente viene riportata intatta come prima destinazione, e ogni destinazione di un dominio replica secondo il calendario off-site di quel dominio.
- **Calendario off-site per dominio** (modificato insieme a ogni altro calendario su Impostazioni, Pianificazioni): lascialo vuoto per replicare dopo ogni backup locale, oppure imposta una cadenza (per esempio `weekly Sun 03:00`) per spedire off-site meno spesso di quanto esegui il backup localmente. Un pulsante **Replica ora** copre le esecuzioni su richiesta.
- **La conservazione off-site** risiede su Impostazioni, Conservazione così puoi tenere le copie off-site più a lungo come archivio. Lascia la policy tutta a zero per non tagliare mai automaticamente gli snapshot off-site.
- **I limiti di banda** (Impostazioni, Off-site) limitano la velocità di upload/download di restic così la replica non satura la tua WAN.
- Un **indicatore di replica** mostra quale dominio sta replicando mentre è in corso (sulla sua pagina e sulla Dashboard). È un indicatore attivo, non una barra di percentuale, perché `restic copy` non espone alcun progresso leggibile da una macchina.

!!! note "Ripristina da qualsiasi luogo"
    Ogni container, VM, set di file, la flash e la configurazione dell'app elencano i propri backup come un'unica cronologia su tutti i luoghi in cui si trova un backup. Un backup copiato su B2 compare una sola volta, contrassegnato con ogni luogo che lo contiene. Un ripristino usa il primo luogo che riesce a raggiungere, a partire dal repository su cui l'elemento viene scritto, e puoi scegliere un altro luogo per ogni riga. I luoghi off-site vengono letti solo quando li apri. L'eliminazione in un luogo controlla prima gli altri e dice se era l'ultima copia.

## Destinazioni {#destinations}

Impostazioni, Off-site inizia con **Destinazioni**: i luoghi in cui vanno le copie off-site, impostati una sola volta per tutti i domini. Una destinazione compare poi come pulsante nella riga **Collocazione** di ogni dominio e di ogni elemento. La prima volta che viene accesa per un dominio, BombVault crea il repository di quel dominio in una cartella al suo interno, per esempio `rclone:onedrive:BombVault/containers`.

**Aggiungi destinazione** apre una procedura guidata in cinque passi:

1. **Dove devono andare i backup?** Ogni servizio è elencato con il suo logo, in quattro gruppi: servizi di archiviazione con bucket S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 e altri), il tuo server S3 (Garage, SeaweedFS, RustFS, Ceph, JuiceFS, Versity S3 Gateway), il tuo server e le condivisioni (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, un percorso montato) e lo storage cloud (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud e tutto il resto che rclone supporta). Ognuno indica quanto è adatto ai backup: i cloud drive rallentano con molte richieste, quindi il primo backup e la potatura richiedono più tempo.
2. **Accedi.** I campi dipendono dal servizio: una chiave di accesso per S3, utente e password per WebDAV e SMB, una password per app dove l'accesso a due fattori blocca quella normale, la chiave SSH pubblica di BombVault per SFTP e la Storage Box, oppure un token per i servizi che accedono tramite browser. Per questi ultimi la procedura mostra un comando `rclone authorize` da eseguire su un computer con un browser; il token che stampa va nel campo. **Prova connessione** verifica l'accesso prima che venga salvato qualcosa.
3. **Scegli una cartella.** La procedura elenca le cartelle della destinazione, con **Nuova cartella** per crearne una e lo spazio libero dove il servizio lo riporta. Una cartella vuota è la scelta più sicura.
4. **Protezione dall'eliminazione.** La procedura dice senza giri di parole cosa può fare il servizio. Un rest-server in modalità append-only rifiuta l'eliminazione, e il tamper test lo verifica. Un bucket S3 può conservare le vecchie versioni con versioning e object lock, cosa che BombVault non sa ancora verificare. Un cloud drive non può rifiutare l'eliminazione in nessun caso: chi entra nel server entra anche in quella copia. Attiva **Immutabile (append-only)** solo dove il lato remoto rifiuta davvero l'eliminazione; BombVault allora non pota mai lì.
5. **Per le emergenze.** Il kit di ripristino elenca ogni destinazione con il repository di ciascun dominio al suo interno. L'accesso torna con il backup delle impostazioni di BombVault; su un'installazione nuova senza di esso, imposta di nuovo la destinazione nello stesso posto.

I servizi S3 passano dal backend S3 di restic, ed è questo che permette di applicare una classe di archiviazione e l'object lock. Ogni altro servizio passa dal rclone fornito con BombVault, e il suo remote compare poi nella configurazione di rclone in Impostazioni, Accesso cloud. Un'esportazione delle impostazioni include le destinazioni; con le credenziali incluse porta con sé anche il loro accesso.

La destinazione che un dominio ottiene da una destinazione prende da questa nome, posizione, credenziali, classe di archiviazione e interruttore immutabile. La conservazione, la compressione e il budget di crescita restano per dominio, e la sua posizione non può spostarsi perché lì si trova il repository del dominio. **Aggiungi una destinazione solo per questo dominio** sotto ogni dominio accetta ancora un URL di repository scritto a mano.

## Collocazione per elemento {#placement}

Ogni scheda di container, VM e set di file ha una riga **Collocazione** di pulsanti: **Locale** e un pulsante per ogni destinazione off-site del dominio, seguiti dalle destinazioni sotto cui il dominio non ha ancora una destinazione. I pulsanti accesi ricevono i backup dell'elemento.

- Con **Locale** acceso, l'elemento viene scritto sul repository mostrato sotto **Salvato su** e copiato su ogni altra destinazione accesa. Spegni una destinazione e non riceve più nulla di nuovo da questo elemento. Locale da solo non copia da nessuna parte, il che va bene per dati che hanno già una seconda copia, per esempio una condivisione che vive su un NAS.
- Con **Locale** spento, l'elemento viene scritto direttamente sul repository diretto della prima destinazione accesa e da lì copiato sulle altre destinazioni accese. La prima volta, una finestra crea quel repository diretto.
- Un pulsante di destinazione crea la destinazione del dominio sotto quella destinazione e la accende solo per questo elemento. Ogni altro elemento parte senza copia lì.
- Un pulsante resta sempre acceso, perché un backup ha bisogno di un posto dove andare. Per escludere qualcosa dai backup, escludilo.

La posizione è fissa dal primo backup dell'elemento in poi, perché BombVault non sposta mai i backup tra repository. Le copie possono cambiare in qualsiasi momento. Una destinazione che non riceve più un elemento mantiene le copie che ha e le pota secondo la propria conservazione alla prossima esecuzione off-site del dominio; **Elimina in B2** sulla scheda le rimuove subito. Quando alcune di quelle copie non esistono da nessun'altra parte, la conferma le elenca per data e chiede il nome dell'elemento. Dalle destinazioni append-only non si può eliminare.

Sotto la riga la scheda indica dove va l'elemento e cosa c'è realmente: quante sedi lo custodiscono, quando ogni destinazione è stata vista l'ultima volta, e se il 3-2-1 è rispettato. Una sede è il server con i dati originali, ogni destinazione off-site e ogni repository contrassegnato **Fuori sede**. BombVault controlla le copie e le sedi; non controlla la parte «due supporti» del 3-2-1.

### Collocazioni predefinite

Impostazioni, Archiviazione, **Collocazioni predefinite** ha una riga per dominio con gli stessi pulsanti. Le copie si applicano subito a ogni elemento senza una scelta propria, e alle cartelle di progetto degli stack Compose. La posizione si applica a un nuovo elemento al suo primo backup; cambiarla non sposta nessun backup. Prima di salvare, la riga elenca ogni destinazione che guadagna o perde elementi e quanti snapshot significa. **Applica agli elementi senza backup** riporta al valore predefinito ogni elemento che non ha ancora un backup.

Una nuova destinazione off-site riceve ogni elemento non impostato su Locale. La finestra che la aggiunge indica quanti elementi e, dove noto, quanta cronologia significa, e offre di escludere gli elementi già esclusi dalle altre destinazioni.

### Repository diretti

Spegnere Locale per un elemento, in modo che una destinazione senza repository diretto diventi la sua casa, apre una finestra con una posizione suggerita accanto alla destinazione, per esempio `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, e un test di connessione che non crea nulla. **Crea e usa** crea il repository e vi punta l'elemento. Un repository diretto prende la chiave, la classe di archiviazione, i limiti, l'impostazione append-only e la conservazione della destinazione, e cambia insieme a essi; la scheda Repository lo mostra in sola lettura. Quando una nuova chiave della destinazione non riesce ad aprirlo, il repository diretto mantiene la chiave che ha e il salvataggio lo segnala. Un elemento su un repository diretto viene copiato da lì sulle altre destinazioni accese, mai sulla destinazione a cui il repository appartiene. I suoi snapshot portano il tag `bv:direct`, e ogni altro passaggio di conservazione li risparmia, così un repository diretto che ha perso il collegamento con la sua destinazione non invecchia mai secondo le regole locali. Si accede a B2 tramite il suo endpoint S3, inserendo l'ID chiave e la chiave applicativa come credenziali S3; una chiave limitata alla cartella propria della destinazione non riesce a raggiungere la cartella accanto, quindi limita invece la chiave alla cartella sopra la destinazione.

### Fuori sede

Un repository con nome può essere contrassegnato **Fuori sede** sulla scheda Repository. I repository remoti partono contrassegnati; disattivalo per un rest-server nello stesso edificio. Il contrassegno conta solo le sedi e il 3-2-1 sulle schede. Non cambia nessuna copia.

### Dopo una ricostruzione

Le scelte di copia vivono nelle impostazioni di BombVault stesso. Dopo una ricostruzione tramite Scopri backup senza un `/config` ripristinato sono perse, e copiare tutto rimanderebbe su B2 gli elementi che avevi escluso. La replica off-site di ogni dominio ricostruito quindi si mette in pausa. La Dashboard lo mostra in ambra, e Collocazioni predefinite offre **Conferma valore predefinito** con un'anteprima di cosa copia la prossima esecuzione e i nomi nei backup che non hanno una voce, che puoi escludere lì. Solo la conferma termina la pausa; importare un file di impostazioni riporta regole e valori predefiniti ma non la termina.

## Repository primari remoti {#remote-primary-repositories}

Il percorso di backup di un dominio (Impostazioni, Archiviazione) non si limita a una cartella locale: puntalo direttamente a un remoto restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:utente@host:/repo`, `rclone:remoto:bucket/percorso`) e BombVault salva lì direttamente, senza copia locale separata e senza passo di replica. È una forma davvero diversa dalla replica off-site vista sopra: là il repository locale è il primario e quello off-site ne è un archivio per quanto possibile; qui il repository remoto **è** il primario, ed è l'unica copia finché non configuri anche una replica off-site (o un secondo remoto) per quel dominio.

Ognuno dei sei campi di percorso (Container, VM, Flash, Auto-backup, Cartelle, Dataset ZFS) ha subito accanto un interruttore **Locale / Remoto**:

- **Locale** mostra il consueto sfoglia-cartelle.
- **Remoto** lo sostituisce con un semplice campo URL, più un pulsante che apre la stessa finestra di test connessione e credenziali usata dalle destinazioni off-site, configurata però per questo primario. Da lì ottieni:
    - **Un test di connessione** contro il percorso reale, prima di farci affidamento.
    - **Limiti di banda** (invio e ricezione), perché un backup pianificato verso un primario remoto non saturi la tua linea WAN: le stesse opzioni restic `--limit-upload` e `--limit-download` usate dalla replica off-site, applicate al backup stesso.
    - **Protezione append-only (immutabilità)**, verificata con lo stesso test di manomissione attivo (una vera sonda DELETE verso l'altro capo) che ricevono le destinazioni off-site. Con essa attiva, BombVault si rifiuta di potare il repository: poiché dietro non c'è una copia locale separata, le credenziali su questa macchina non devono poter cancellare l'unica copia del backup.
    - **Un allarme sul budget di crescita**, ricavato dallo stesso andamento della dimensione del repository che la scheda Archiviazione già segue.

Niente di tutto questo è obbligatorio: un percorso remoto scritto a mano senza impostazioni di sicurezza salvate esegue il backup esattamente come sempre (banda illimitata, potabile, nessun allarme di budget). La finestra di sicurezza serve per quando vuoi le stesse protezioni che ottiene una copia off-site, senza dover creare una destinazione off-site solo per quello.

!!! note "Le credenziali cloud e REST sono condivise"
    Un primario remoto si autentica con le stesse credenziali S3/REST configurate in Impostazioni, Accesso cloud, Credenziali cloud condivise: non esiste un archivio di credenziali separato per i repository primari.

### SMB e WebDAV senza mount sull'host {#smb-webdav}

Impostazioni, Accesso cloud, rclone ha un modulo per una condivisione Windows o Samba e per un server WebDAV (Nextcloud, ownCloud, SharePoint o qualsiasi altro). Inserisci un nome breve, host e condivisione (SMB) oppure URL e tipo di server (WebDAV), utente e password, e BombVault scrive la sezione rclone per te. rclone offusca la password da sé prima che venga salvata; aggiungere una destinazione con un nome già esistente sostituisce quella sezione invece di aggiungerne una seconda.

Il modulo risponde con la posizione completa, per esempio `rclone:nas:backups`. Inseriscila in un Percorso di backup o in una destinazione off-site e aggiungi una sottocartella se vuoi (`rclone:nas:backups/bombvault`). La condivisione è il primo segmento del percorso, non fa parte del nome.

È una strada migliore del montare la condivisione su Unraid: restic sconsiglia di tenere un repository su una condivisione CIFS montata, e qui non viene montato nulla. NFS non è nel modulo perché né restic né rclone hanno un backend NFS; per NFS, monta l'export sull'host e puntaci un Percorso di backup.

## Off-site immutabile (append-only)

Contrassegna un repo off-site come append-only così ransomware, o un host compromesso, non possano eliminare o riscrivere i tuoi backup. L'altro lato (un `restic/rest-server` in esecuzione in modalità `--append-only`) lo **impone**. BombVault lo **verifica** soltanto e non mostra mai verde sulla sola affermazione di una configurazione.

La procedura guidata di **configurazione off-site guidata** ti accompagna dalla scelta del backend (rest-server / rclone / S3) attraverso uno snippet di deploy del rest-server pronto da incollare, un test di connessione, l'interruttore immutabile (che esegue immediatamente il tamper test) e una strategia di conservazione, così l'off-site append-only è raggiungibile senza modificare a mano le configurazioni.

!!! note "Una cancellazione riuscita sotto `/locks/` è prevista"
    Append-only non significa che non si possa più cancellare nulla. restic deve prendere e rilasciare i propri lock, quindi `/locks/` resta scrivibile e cancellabile di proposito. Gli snapshot e i dati che stanno dietro, cioè proprio ciò che un ransomware cercherebbe, non possono essere rimossi. Se sondi tu stesso il lato remoto, una cancellazione che riesce sotto `/locks/` è il comportamento corretto e non una falla.

!!! warning "I repo immutabili non vengono mai potati da questa macchina"
    Un off-site immutabile deliberatamente non pota mai i vecchi snapshot. Impostagli un **allarme del budget di crescita** così vieni avvisato prima che la dimensione del repo sfugga di mano.

## Tamper test

BombVault dimostra periodicamente la garanzia append-only tentando effettivamente un'eliminazione contro il repo off-site, mirata a un oggetto inesistente:

- **Rifiutata** significa protetto.
- **Accettata** significa non protetto.
- Un risultato **inconcludente** (server irraggiungibile, errore di autenticazione) non ribalta mai il verdetto memorizzato.

Un reale passaggio da protetto a non protetto fa scattare un unico avviso.

## Esercitazioni DR

BombVault offre due livelli di prova che i tuoi backup siano effettivamente ripristinabili, non solo presenti.

- **Esercitazioni di verifica del ripristino (locali).** BombVault esegue periodicamente `restic check --read-data-subset` (limitato, mai un ripristino completo che riempie il disco) e mostra un badge *Ripristinabilità verificata* per dominio. La cadenza risiede su Impostazioni, Pianificazioni; il badge su Impostazioni, Integrità.
- **Esercitazioni DR (off-site).** BombVault ripristina una destinazione reale dal repo off-site in una sandbox usa e getta, la verifica file per file e byte per byte, poi ripulisce. Questo dimostra che puoi recuperare da off-site, non solo che il repo risponde.

La **scorecard della protezione dal ransomware** sulla Dashboard riassume tutto questo in una postura verde / ambra / rossa per dominio, con una checklist con marca temporale (off-site configurato, append-only verificato, replica aggiornata, esercitazione di ripristino superata, cifratura attiva, strategia di pota impostata). Ogni riga rossa collega direttamente alla soluzione, e la scheda diventa verde solo su fatti verificati.

## Associazione delle istanze {#pairing}

I Ricevitori, le fonti di Prelievo, la pagina Istanze e il Mesh fuori sede parlano tutti con un altro BombVault. Lo fanno come membri di un unico gruppo di associazione, e un'istanza si unisce al gruppo con dodici parole.

Sulla prima istanza apri **Impostazioni → Associazione** e premi **Genera frase** nelle schede di associazione. Appaiono dodici parole in una finestra con un pulsante **Copia**. Su ogni altra istanza apri lo stesso posto, premi **Inserisci frase** e incollale o digitale, oppure premi **Incolla** in quella finestra. Una parola che non è nell'elenco viene indicata con la sua posizione già mentre la digiti, e l'ultima parola porta un checksum, così una parola digitata male o scambiata viene rilevata prima che avvenga l'associazione. Genera la frase su una sola istanza: due istanze che creano entrambe una frase formano due gruppi separati. Se per un minuto non si presenta nessuno, la scheda offre due vie d'uscita: mostrare di nuovo le parole per inserirle dall'altra parte, oppure inserire le parole dell'altra istanza e unirsi al suo gruppo in un solo passaggio. L'associazione funziona anche senza una password di accesso, ma impostane una: senza, chiunque possa aprire questa interfaccia web può leggere le parole e ottenere, tramite il gruppo, la password restic di ogni istanza al suo interno. La scheda di associazione lo segnala finché non viene impostata una password. Con una password, mostrare di nuovo la frase la richiede. **Lascia il gruppo** fa uscire di nuovo un'istanza.

Chiunque conosca le parole può unirsi al gruppo, trattale quindi come una password.

**Come i membri si raggiungono.** Ogni istanza apprende il proprio indirizzo sulla rete dal tuo browser nel momento in cui accedi, mostrato nella scheda del relay come **Questa istanza sulla tua rete**; correggilo lì se davanti c'è un reverse proxy o una porta insolita. Sulla stessa rete i membri annunciano quell'indirizzo tramite multicast e si parlano direttamente, e dove il multicast non riesce ad attraversare una rete di contenitori, come la rete bridge predefinita di Docker, un'istanza cerca invece le altre nella propria subnet con una chiamata firmata a cui solo un membro del gruppo sa rispondere, così l'associazione si completa comunque in pochi secondi senza relay. Se non salta fuori nulla, **Non la trovi?**, sotto la scheda di associazione, accetta un indirizzo inserito a mano, per un'altra subnet o una porta non standard. Le istanze su reti diverse passano da un relay, scelto nella stessa scheda:

- **Relay del progetto** (predefinito): `parleyport.halleluja.design`, lo stesso relay usato anche da KnightLoader. Niente da configurare.
- **Relay proprio**: il container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) dalle Unraid Community Apps, oppure una delle tue istanze già raggiungibile dall'esterno con **Funge da relay** attivato. Quell'istanza risponde poi su `/relay/connect` al proprio indirizzo, dietro il reverse proxy e il certificato che già possiede, e fa entrare solo il tuo gruppo. Inserisci l'indirizzo del relay su ogni istanza che deve usarlo.
- **Nessun relay**: i membri si trovano automaticamente solo sulla stessa rete, e in nessun altro posto.

**Cosa vede il relay.** Ogni chiamata tra membri è sigillata con AES-256-GCM sotto una chiave derivata dalle dodici parole, e quella chiave non lascia mai le tue istanze. Il relay apprende un hash che raggruppa le connessioni, a quale istanza è destinato un messaggio, quanto è grande e quando passa. Una chiamata diretta sulla rete locale è sigillata allo stesso modo e anche firmata, così nulla dipende dal certificato autofirmato che un'istanza serve.

**Cosa viaggia sul gruppo.** Le scorecard della pagina Istanze, una richiesta di controllare subito un dominio, le offerte di storage fuori sede del Mesh, e ciò di cui un Ricevitore o una fonte di Prelievo ha bisogno: le posizioni del repository dell'altra istanza e la sua password restic. I dati di backup invece mai: vanno sempre direttamente ai backend restic. Nemmeno l'APP_KEY: la password restic apre i repository di quell'istanza e nient'altro, non i suoi segreti memorizzati, non le sue sessioni, non i suoi codici di ripristino.

**Voci precedenti all'associazione.** Le istanze aggiunte con un token di flotta, e i Ricevitori e le fonti di Prelievo configurati con l'APP_KEY dell'altra istanza, restano dopo l'aggiornamento e sono contrassegnati **Associa di nuovo**. I Ricevitori e le fonti di Prelievo continuano a funzionare: al primo avvio BombVault sostituisce ogni APP_KEY memorizzato con la password restic da esso derivata. Associa entrambe le istanze, poi modifica la voce e scegli la sua istanza. Un'istanza di questo tipo riprende la sua vecchia scheda non appena un'istanza con lo stesso nome compare nel gruppo.

L'unico posto che richiede ancora un APP_KEY a mano è [Ripristino da un altro repo BombVault](#restore-from-another-bombvault-repo), per il caso in cui l'altra istanza sia scomparsa e non possa rispondere in un gruppo.

## Dashboard ricevente (il lato ricevente)

![Il lato ricevente, osservato in sola lettura, con un controllo di integrità eseguito su questa macchina.](assets/screenshots/receiver.png)

*Il lato ricevente, osservato in sola lettura, con un controllo di integrità eseguito su questa macchina.*

Tutto quanto sopra è il lato *mittente*. Sulla macchina che **riceve** copie off-site immutabili da un altro BombVault, la dashboard Ricevitore ti offre un monitoraggio indipendente e in sola lettura di quei repository sull'hardware ricevente, così un fallimento silenzioso all'altra estremità non passa inosservato.

Attiva l'interruttore **Ricevitore** in Impostazioni per rivelare una scheda **Ricevitore**. È disattivato di default; abilitalo solo su una macchina che riceve effettivamente backup off-site immutabili. Poi registra un repository ricevuto (in sola lettura, aperto con la password restic dell'istanza mittente, che ottiene tramite il [gruppo di associazione](#pairing)) per ottenere:

- **Un inventario di snapshot raggruppato per sorgente**, così puoi vedere esattamente quali container, VM e set di file sono arrivati.
- **Ultimo ricevuto** per sorgente, così sai quanto è fresco ciascuno.
- **Un `restic check` indipendente** eseguito sull'hardware ricevente, così l'integrità viene verificata dove i dati effettivamente risiedono, non solo sul mittente.
- **Un dead-man's switch:** un avviso quando una sorgente smette di inviare entro una finestra che imposti.
- **Avvisi di integrità:** un avviso quando un controllo sul lato ricevente fallisce.

Il Ricevitore è rigorosamente in sola lettura. Non scrive mai nel repository ricevuto, così non può mai rompere la garanzia append-only su cui il mittente fa affidamento.

## Esempio completo: due macchine Unraid, dall'inizio alla fine

Sopra sono descritti i pezzi. Questa è un'installazione completa con valori reali, perché i pezzi si montano meglio dopo averli visti montati una volta.

Due macchine: **TOWER** esegue i container e invia i backup, **VAULT** li riceve e impone l'immutabilità. Sostituisci con i tuoi nomi, indirizzi e percorsi di condivisione.

**1. Su VAULT, avvia il server append-only.** In BombVault su TOWER vai su *Impostazioni → Off-site → Configura*, scegli **rest-server** e genera la ricetta. Copia la scheda **Modello Unraid (XML)**, salvala su VAULT come `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, poi *Docker → Add Container* e scegli **rest-server** dall'elenco dei modelli. Prima di avviarlo, scrivi la riga `htpasswd` mostrata in `/mnt/user/appdata/rest-server/.htpasswd` su VAULT. La password monouso viene mostrata una sola volta e non viene mai conservata: copiala ora. Quella riga porta la stessa password, già cifrata con bcrypt per te: il testo in chiaro va nelle credenziali REST su TOWER, la riga cifrata nel `.htpasswd` su VAULT. Non devi cifrare nulla tu.

    Lascia `--append-only` nel campo OPTIONS. È tutto il senso della cosa: senza, VAULT torna a essere una normale condivisione.

**2. Su TOWER, punta il repository off-site su di esso.** L'URL del repository segue lo schema stampato dalla ricetta:

    rest:http://VAULT:8000/bombvault-containers/containers

Il primo segmento del percorso è l'utente htpasswd, il secondo il repository. Inserisci l'utente e la password generati come credenziali REST della destinazione, poi esegui il **test di connessione**.

**3. Su TOWER, attiva «Immutabile».** Il test di manomissione parte subito e deve dire *protetto*. Cosa significano le risposte:

| Risultato | Cosa è successo |
| --- | --- |
| **protetto** | VAULT ha rifiutato la cancellazione. È l'unico stato che passa. |
| **NON protetto** | VAULT ha accettato una cancellazione. Manca `--append-only` oppure è stato rimosso. |
| **non conclusivo** | Né l'uno né l'altro. Di solito l'URL non è quello che usa restic, oppure le credenziali sono cambiate. Non viene registrato nulla e non scatta alcun avviso. |

**4. Su VAULT, guarda cosa arriva.** Associa le due macchine ([Associazione delle istanze](#pairing)), attiva *Impostazioni → Generale → Ricevitore*, apri la scheda **Ricevitore** e registra il repository in sola lettura con TOWER come istanza mittente.

!!! warning "La posizione è un percorso **dentro** il container, scritto relativo al mount dell'host"
    Inserisci `user/appdata/rest-server/bombvault-containers/containers`, **non** `/mnt/user/appdata/…`. BombVault gira in un container in cui il `/mnt` dell'host è montato altrove; un percorso host assoluto lì non esiste. Se ne incolli uno, BombVault ora ti indica il percorso relativo da usare.

    VAULT ottiene la password restic di TOWER tramite il gruppo quando salvi; nessuno deve digitare una chiave.

**5. Rendilo reciproco, se vuoi.** Ripeti gli stessi cinque passi nella direzione opposta: un rest-server su TOWER che riceve la copia di VAULT. Ogni macchina impone allora l'immutabilità all'altra, e nessuna può cancellare i backup dell'altra.

## Ripristino guidato

Una scheda **Ripristino** dedicata accompagna un'installazione pulita o ricostruita attraverso il caso di disastro, in un unico posto:

1. **Ripristina prima le impostazioni di BombVault stesso**, così i percorsi di backup, le destinazioni off-site e le credenziali di cui il resto del flusso ha bisogno risultano precompilati (applicato tramite un auto-riavvio sul socket Docker, così il database delle impostazioni in esecuzione non viene mai sovrascritto sotto un handle aperto).
2. **Verifica che BombVault possa leggere i tuoi backup** (l'insidia della chiave di crittografia messa in primo piano).
3. Ti permette di **puntare al tuo repo esistente** (locale o off-site).
4. **Scopre** i container, le VM, i set di file e i dataset ZFS memorizzati al suo interno.
5. **Ripristina container e VM in un colpo solo** (lasciati fermi, così li avvii deliberatamente) ed elenca i set di file e gli elementi ZFS da ripristinare uno alla volta; gli elementi ZFS tornano disattivati. Il tuo kit di ripristino è a un clic di distanza.

!!! tip "Migrazione pianificata contro disastro"
    Il ripristino guidato ripristina le impostazioni di BombVault stesso da un backup. Per uno spostamento *pianificato* su una nuova macchina, puoi invece portare la tua configurazione direttamente con la scheda **Esporta / importa impostazioni** (un file JSON portatile). Vedi [Configurazione](configuration.md#portable-settings-export-and-import).

### Ripristino da un altro repo BombVault {#restore-from-another-bombvault-repo}

Una scheda separata nella scheda **Ripristino** apre il repo di un'*altra* istanza BombVault (una condivisione montata sotto `/mnt`, o un URL remoto) con l'**`APP_KEY` di quell'istanza**, in una sessione monouso e in sola lettura. Sfoglia i container, le VM e i set di file memorizzati lì, scegli uno snapshot e ripristinalo, e l'oggetto ripristinato diventa un normale container, VM o set di file locale. Nulla viene mai scritto nell'altro repo, e le tue impostazioni di backup restano intatte (la sessione risiede in memoria e scade da sé). Spostare un container dal server A al server B non significa più ripuntare le impostazioni del tuo repo e riportarle indietro dopo. Questa scheda è monouso: apre una sessione, ripristina ciò che scegli e dimentica l'altra istanza. Se invece vuoi un accordo stabile, in cui questa macchina preleva secondo una pianificazione gli snapshot di un'altra istanza nel proprio repository, quella è la scheda **Prelievo** della pagina **Istanze**.

## Kit di ripristino della chiave di crittografia

Questo è il pezzo che rende possibile il disaster recovery anche quando non c'è alcun BombVault in esecuzione.

Un clic scarica la **chiave master**, la **password restic derivata** e le **posizioni e comandi esatti del repo**, così puoi ripristinare direttamente con la CLI di restic su qualsiasi macchina. Un promemoria sulla Dashboard ti assilla finché non l'hai conservato.

!!! danger "Conserva il kit di ripristino fuori dal server"
    Il kit contiene il segreto che decifra i tuoi backup. Tienilo in un luogo sicuro e separato dal server (un password manager, una copia stampata in una cassaforte). Se perdi sia BombVault che `APP_KEY` senza kit di ripristino, i tuoi backup cifrati non possono essere recuperati.

!!! warning "Lo snapshot più recente non è sempre quello da ripristinare"
    Da restic 0.17, `restic snapshots` mostra la dimensione di ogni snapshot. Dopo una perdita di dati lo snapshot più recente può essere quello svuotato, quindi non ripristinare uno snapshot molto più piccolo dei precedenti. Dopo un ransomware può essere quello cifrato, di dimensione normale. Se BombVault funziona ancora, controlla prima la sua pagina **Anomalie**: indica l'ultimo backup buono. Un ripristino non ha bisogno di alcun dato sulle anomalie di BombVault, e la pausa della conservazione mantiene sempre solo più snapshot.

### Sigillare il kit

Se hai attivato la cifratura age per le esportazioni in chiaro (Impostazioni), anche il kit viene sigillato con essa e si scarica come `bombvault-recovery-kit.md.age`. È in formato ASCII armor anziché binario, quindi resta testo: incollarlo in un password manager o stamparlo funziona esattamente come prima, solo che il contenuto è illeggibile senza la tua chiave.

!!! warning "Non conservare la chiave age dentro il kit"
    Ti serve la tua chiave age **privata** per aprire un kit sigillato. Tienila in un posto che non dipenda dal kit stesso, altrimenti avrai due cose da recuperare invece di una. Sigillare conviene quando il kit è conservato in un posto che non controlli del tutto (un password manager condiviso, note nel cloud, una stampa in ufficio); un kit nella tua cassaforte è già protetto dalla cassaforte.

    Con la cifratura attiva e nessun destinatario utilizzabile configurato, il download viene rifiutato del tutto. BombVault non ripiega mai sul consegnare la chiave master in chiaro.

### Se il kit non è a portata di mano

La password non è memorizzata da nessuna parte, viene **calcolata** dall'`APP_KEY`. Con la chiave e una shell puoi quindi riprodurla da solo:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

È un HMAC-SHA256 sulla stringa fissa `bombvault:restic-repo`, con i byte grezzi dell'`APP_KEY` esadecimale come chiave, stampato come 64 caratteri esadecimali minuscoli. Lo stesso valore è nel kit, come password restic derivata; questo serve per il giorno in cui il kit si trova altrove rispetto a te.

!!! warning "Per un repository ricevuto, usa la chiave dell'istanza MITTENTE"
    Un repository arrivato qui tramite replica off-site è stato creato dalla macchina che lo ha inviato, con la **sua** `APP_KEY`. Derivare dalla chiave della macchina ricevente produce una password che restic rifiuta, il che sembra esattamente un repository corrotto senza esserlo. È il motivo abituale per cui `restic check` su un repository ricevuto continua a chiedere la password.

Poiché le definizioni di ripristino risiedono **dentro** ogni repo (`<repo>/def`, `<repo>/vm-def`), una cartella di repo copiata è completamente autonoma, così il kit più il repo sono tutto ciò che serve per un ripristino bare-metal.

## Recuperare un dump del database {#database-dumps}

Un dump del database è un punto di ripristino a sé nel repository dei container, con l'etichetta `dbdump:<container>` e un solo file, `/dbdump/<container>.sql`. BombVault li elenca, li scarica e li importa sotto **Backup**; qui sotto ci sono gli stessi passi con il solo restic, per il giorno in cui BombVault non c'è.

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
