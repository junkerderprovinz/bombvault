# Ripristino

Ogni ripristino parte dalla pagina **Ripristino**: scegli una sezione, cerca ciò che vuoi riavere e scegli un backup. Una sola finestra ripristina ogni tipo di elemento e chiede per passaggi. Di default non cambia nulla sull'originale, perché il backup torna accanto a esso.

![La pagina Ripristino con le sezioni come riquadri, la ricerca e la scheda Stack.](assets/screenshots/restore/page-dark.png#only-dark)
![La pagina Ripristino con le sezioni come riquadri, la ricerca e la scheda Stack.](assets/screenshots/restore/page-light.png#only-light)

## La pagina Ripristino {#page}

- **Sezioni** elenca le sezioni come riquadri, ciascuno con quanti elementi hanno snapshot.
- **Cosa vuoi riportare indietro?** cerca tra gli elementi della sezione scelta. Ogni riga mostra il suo snapshot più recente, e **Apri elemento** porta alla pagina dell'elemento.
- **Trova un file** cerca un file o una cartella per nome in ogni backup, vedi [Trovare un file](#find-a-file).
- **Stack** elenca i progetti Compose con almeno due container salvati, come `immich` con il suo database e Redis. La ricerca trova uno stack dal suo nome o da uno qualsiasi dei suoi container.
- **In esecuzione** mostra un ripristino mentre gira, anche dopo che ne hai chiuso la finestra.
- **Emergenza** avvia la Procedura guidata di emergenza per un server che non c'è più o è stato reinstallato, vedi [Procedura guidata di emergenza](offsite-recovery.md#emergency).
- **Altre sorgenti** contiene **Da una vecchia cartella di backup**, per i backup di un altro BombVault, e **Importa dal plugin Appdata.Backup**.

![La pagina Ripristino su un telefono.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![La pagina Ripristino su un telefono.](assets/screenshots/restore/page-phone-light.png#only-light)

## La finestra di ripristino {#window}

Un clic su un elemento apre **Ripristino** con il suo nome. Chiede per passaggi, e ciascuno si può cambiare prima di cominciare:

1. **Backup**: lo snapshot consigliato è già selezionato. È il più recente oppure, dopo un'anomalia aperta, l'ultimo prima di essa. Il backup fatto prima di un ripristino precedente contiene lo stato che quel ripristino ha sostituito, quindi viene scartato a meno che non ce ne siano altri. Quelli più vecchi sono nell'elenco, e **Confronta** mostra cosa differisce tra due di essi.
2. **Sorgente**: da dove vengono i file, quando uno snapshot si trova in più posti. **Locale** è il più veloce.
3. **Cosa**: **Tutto**, oppure **File singoli**, dove spunti i file e le cartelle che vuoi.
4. **Dove**: la destinazione del ripristino. Le scelte dipendono dal tipo di elemento:

    | Destinazione | Cosa fa |
    |---|---|
    | **Accanto all'originale** | Il valore predefinito. Un container torna come secondo container chiamato `<name>-restored` con porte proprie, i dischi e la NVRAM di una VM finiscono in una cartella chiamata `<folder>-restored`, e un set di cartelle torna come `<path>-restored`. Nulla cambia sull'originale. |
    | **Sostituisci l'originale** | Sovrascrive ciò che c'è ora. Con **Prima fai il backup dello stato attuale**, attivo di default, BombVault salva prima lo stato di oggi così puoi tornare indietro. Se quel backup fallisce, il ripristino non parte. Funziona anche per un container o una VM che non c'è più sul server: le cartelle che il ripristino sovrascrive vengono salvate a parte. |
    | **In una cartella** | Solo i file, da sfogliare. |
    | **Scarica come ZIP** | La chiavetta flash, per una chiavetta nuova. BombVault non scrive mai nella flash in funzione. |
    | **In un nuovo dataset** e **Ripristina dentro il dataset** | Dataset ZFS, vedi [Dataset ZFS](zfs-datasets.md#restore). |

5. **Cosa succede** dice a parole che cosa farà il ripristino, e una scena mostra la destinazione che hai scelto.

Prima che **Ripristina** si sblocchi, la finestra verifica che il repository risponda, che la chiave lo apra, che lo snapshot ci sia e che la destinazione abbia spazio. Finché una verifica fallisce il pulsante resta bloccato, e la (i) al suo interno dice quale.

La finestra mostra anche cosa cambia il ripristino rispetto a ciò che c'è ora: file nuovi, sostituiti e invariati, con l'elenco a richiesta, e i file nella destinazione che non sono nel backup e restano dove sono. restic lo calcola come prova a vuoto da dimensione e data di modifica, senza leggere i file, e un albero molto grande si ferma dopo 30 secondi e lo dice. Per container e VM confronta le impostazioni che il ripristino ricrea con quelle in esecuzione: immagine e tag, porte, nomi delle variabili e volumi, oppure memoria, vCPU, dischi e rete.

Alcune cose possono fermare un ripristino prima che cambi qualsiasi cosa. L'IP statico di un container e le porte pubblicate devono essere liberi, e la sua rete deve esistere. Su Unraid ogni ripristino sul posto rifiuta una cartella che non si trova su un pool o una condivisione montati, perché i file finirebbero in memoria e sparirebbero dopo un riavvio. Un ripristino sul posto nomina ogni altro container il cui bind mount arriva dentro una cartella in cui scrive, per esempio "questo percorso è usato anche da nextcloud-db", ma si limita ad avvisare.

Alcune opzioni compaiono dove servono: **Poi lascia fermo**, per guardare un container o una VM prima che riparta, e **Ripristina senza GPU e runtime** su un host che non ha la GPU o il runtime con cui è stato fatto il backup.

Il ripristino gira in background. Puoi chiudere la finestra, e l'avanzamento si vede anche sotto **In esecuzione**. Un ripristino lungo mostra una percentuale e si può annullare, e un ripristino annullato viene registrato come annullato, non come fallito. Un ripristino annullato mentre il container si sta fermando, o uno che non riesce a rimuovere il container, non ha ancora ripristinato nulla, quindi il container riparte. Un container che prende i suoi volumi da un altro con `--volumes-from` li mantiene, e il ripristino si rifiuta di iniziare finché quell'altro container non esiste.

![La finestra di ripristino al passaggio Cosa, con file singoli selezionati.](assets/screenshots/restore/window-what-dark.png#only-dark)
![La finestra di ripristino al passaggio Cosa, con file singoli selezionati.](assets/screenshots/restore/window-what-light.png#only-light)

### Dove va un ripristino in una cartella {#folders}

**Cartelle di ripristino** in **Impostazioni, Ripristino** imposta la **Cartella di ripristino predefinita**, con una **Sottocartella per sezione** se vuoi, e un'**Eccezione** per una sezione. I file scelti finiscono direttamente nella cartella di destinazione: un file torna come quel solo file, una cartella come la cartella con il suo contenuto, e le scelte da cartelle diverse mantengono solo il loro percorso sotto la cartella che condividono. BombVault rifiuta una destinazione che non sta su un'archiviazione montata, perché su Unraid una cartella simile vive in memoria e sparisce dopo un riavvio, e non ripristina mai nella propria cartella dei dati.

## Esempio: riavere un solo file {#example-single-file}

Qualcuno ha modificato ieri notte il `config.php` di Nextcloud e Nextcloud non parte più. Il backup notturno delle 03:00 ha ancora il file buono. Vuoi quel solo file, non l'intero container.

1. Apri **Ripristino**, scegli il riquadro **Container** e scrivi `nextcloud` in **Cosa vuoi riportare indietro?**.
2. Clicca `nextcloud`. La finestra si apre con il backup delle 03:00 già selezionato come consigliato.
3. Lascia **Sorgente** su **Locale**.
4. Sotto **Cosa** scegli **File singoli**. Apri le cartelle fino a `config` e spunta `config.php`.
5. Sotto **Dove** scegli **In una cartella** e scegli una cartella, per esempio `user/restore`, che sull'host è `/mnt/user/restore`.
6. Leggi **Cosa succede**: il file finisce nella cartella di destinazione, e nulla cambia sul container. Premi **Ripristina**.

La finestra termina con **Fatto: i file sono in /mnt/user/restore**. Trovi il file come `/mnt/user/restore/config.php`, senza le cartelle sopra di esso. Confrontalo con quello rotto, copialo di nuovo nella cartella appdata e riavvia Nextcloud.

Per rimettere il file direttamente al suo posto, al passaggio 5 scegli invece **Sostituisci l'originale**. Tieni attivo **Prima fai il backup dello stato attuale**, così BombVault salva lo stato rotto prima di scrivere, digita `nextcloud` per confermare e premi **Ripristina**. Nextcloud viene fermato mentre il file scelto viene riscritto al suo posto e poi riavviato, e vengono scritti solo i file che hai spuntato. Un ripristino che fallisce a metà lascia Nextcloud fermo, perché il file che legge potrebbe essere scritto solo in parte, e l'esecuzione lo segnala.

![La finestra di ripristino a Cosa succede, con Sostituisci l'originale scelto.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![La finestra di ripristino a Cosa succede, con Sostituisci l'originale scelto.](assets/screenshots/restore/window-replace-light.png#only-light)

## Trovare un file {#find-a-file}

Quando conosci il nome di un file ma non sai quale elemento o backup lo ha ancora, scrivi il nome in **Trova un file** e premi **Cerca**. Il testo semplice può trovarsi in qualsiasi punto del nome e non distingue tra maiuscole e minuscole, quindi `invoice` trova `Invoice-2024.pdf`. Con `*` o `?` il modello deve corrispondere al nome intero, quindi `*.pdf` trova ogni PDF. Scrivi solo il nome, senza cartella.

La ricerca legge ogni backup dei container, dei set di cartelle, dei dataset ZFS e della chiavetta flash. Ogni risultato mostra l'elemento, il percorso e ogni backup che contiene il file, dal più recente, con la data, la dimensione e il posto in cui è stato letto. Un backup in cui il file ha la stessa dimensione e la stessa data di modifica di quello precedente indica **invariato**, così capisci quando il file è cambiato l'ultima volta. Un clic su un backup apre su di esso la finestra di ripristino, con **File singoli** scelto e il file spuntato. Un file della chiavetta flash torna come parte dello ZIP della chiavetta flash, perché la chiavetta flash si ripristina solo per intero.

La ricerca richiede un po' di tempo, perché restic percorre ogni backup. **Ferma** la interrompe, e lo stesso succede se lasci la pagina. Sotto il campo è indicato quali posti ha letto:

- I posti su questo server e nella tua rete vengono sempre esaminati.
- Un backup che si trova in più posti viene letto una volta sola, nel primo. Un posto i cui backup sono stati tutti letti altrove indica **già esaminato in un altro posto**.
- I posti off-site come S3, B2 o SFTP restano fuori, a meno che tu non attivi **Cerca anche nei posti off-site**. Allora lì vengono letti solo i backup che nessun posto qui contiene, di solito quelli più vecchi che una conservazione più lunga tiene off-site. La prima ricerca in un posto del genere può richiedere molto tempo, e alcuni fornitori fanno pagare i dati scaricati.
- Un posto che non è stato possibile aprire indica **lettura non riuscita**, e la (i) accanto spiega il motivo. Gli altri posti vengono esaminati lo stesso.

Le macchine virtuali vengono salvate come dischi interi, quindi i file al loro interno non vengono cercati. Nemmeno l'auto-backup e i backup di un altro BombVault in **Altre sorgenti** vengono cercati. Una ricerca elenca i primi 200 file, quindi un nome corto come `log` può richiedere qualche lettera in più.

## Cosa torna indietro, per tipo {#kinds}

I **container** vengono ricreati dalla loro definizione salvata tramite l'API Docker, con entrypoint, hostname, impostazioni di arresto, volumi e mount, limiti di risorse, log driver, impostazioni DNS e link legacy. Un container torna sull'immagine su cui girava quando è stato fatto il backup, non su una più recente scaricata dopo, e un'immagine mancante viene scaricata prima, con le credenziali di **Registry dei container** in **Impostazioni, Ripristino**. Tornano anche la sua GPU o il suo runtime (`--gpus`, `--runtime=nvidia`). Su un host senza quel driver o runtime il ripristino lo dice e offre **Ripristina senza GPU e runtime**. Un link a un container che manca, o che è fermo mentre quello ripristinato parte, viene lasciato fuori, e la cronologia delle esecuzioni lo dice. Un ripristino che fallisce dopo che il container è stato rimosso lo rimette a posto, fermo.

Un backup può contenere cartelle che il container non ha più selezionate, per esempio uno più vecchio fatto prima che una cartella sparisse. Una cartella che non c'è più o che è vuota torna. Una che ora contiene file resta com'è, perché potresti averla deselezionata apposta, e il controllo prima del ripristino e la cronologia delle esecuzioni la nominano. Recuperala con **File singoli** o **In una cartella**.

Le **VM** hanno di nuovo definito il loro XML via SSH e ricollegati i loro dischi e la NVRAM UEFI, anche dopo che la VM è stata eliminata. Vedi [Backup delle VM](vm-backup-ssh-setup.md) per le VM con snapshot Unraid e dischi su zvol.

I **set di cartelle** tornano con proprietà, permessi e date come li salva restic. Dove la destinazione rifiuta solo la proprietà, come fa `/mnt/user`, l'esecuzione termina con una nota che lo dice. BombVault confronta allora i file con il backup, e un file che non è arrivato fa fallire il ripristino.

La **chiavetta flash** non tocca mai il `/boot` in funzione. Il ripristino è un download chiamato `flash-<id>.zip`, pronto da usare con Unraid USB Creator. Lo ZIP contiene `APP_KEY`, quindi per scaricarlo servono una password di accesso e un accesso sull'istanza. La finestra elenca anche i plugin presenti in ogni backup della flash con la loro versione e dimensione, e può rimettere un singolo plugin nella flash in funzione: il suo file `.plg`, la sua cartella sotto `config/plugins` e i file di pacchetto che il backup contiene. Unraid lo installa al prossimo avvio, oppure subito in **Plugins, Install Plugin**.

I **dataset ZFS** hanno destinazioni proprie e mantengono le loro proprietà ZFS, vedi [Ripristinare](zfs-datasets.md#restore).

L'**auto-backup** torna solo attraverso la Procedura guidata di emergenza, che riavvia BombVault, così il database in uso non viene mai sovrascritto mentre è aperto.

I **dump dei database** non vengono mai ricaricati da soli. Una riga sotto **Dump del database** si può scaricare, salvare in una cartella oppure importare in un database appena avviato. Vedi [Dump dei database](backups.md#database-dumps).

## Ripristinare più elementi insieme {#several}

Nella pagina Ripristino scegli gli elementi come scegli le righe nell'elenco Backup: Ctrl+clic, Maiusc+clic, Ctrl+A, una pressione lunga, oppure **Seleziona**. **Ripristina** apre allora una sola finestra per tutti.

- **Quale backup**: **Consigliato** prende per ogni elemento il backup che la sua finestra consiglierebbe. **Un momento per tutti** prende per ogni elemento il suo backup più recente da un **Momento** che scegli o precedente, per esempio la sera prima che qualcosa andasse storto.
- **Dove**: **Accanto all'originale** o **Sostituisci l'originale**, con **Prima fai il backup dello stato attuale**.

Ogni elemento viene verificato come un ripristino singolo, e poi girano uno dopo l'altro. Ogni riga mostra come è andato il suo: **In attesa**, **In esecuzione**, **Ripristinato**, **Fallito** o **Lasciato fuori**. **Ferma dopo questo** lascia finire il ripristino in corso e non ne avvia altri. La chiavetta flash arriva come ZIP da scaricare, e i dataset ZFS e l'auto-backup vengono ripristinati uno alla volta.

![Ripristino di più elementi: una riga per elemento con il suo avanzamento.](assets/screenshots/restore/several-dark.png#only-dark)
![Ripristino di più elementi: una riga per elemento con il suo avanzamento.](assets/screenshots/restore/several-light.png#only-light)

## Ripristinare uno stack {#stacks}

Un clic su uno stack nella scheda **Stack** apre **Ripristina stack**. Ripristina ogni membro dal suo ultimo backup nello stesso momento, e con **Avvia in ordine di dipendenza dopo il ripristino** li avvia nel loro ordine `depends_on`: prima database e cache, poi l'app. Quando la cartella del progetto di un membro manca nel posto che hai scelto, la finestra offre invece lo stato da Locale.

BombVault controlla prima i membri e la cartella del progetto, e un ripristino che non potrebbe riuscire non cambia nulla. Altrimenti ferma i membri in esecuzione, le app prima dei loro database, poi scrive la cartella del progetto, poi ripristina i membri. Se la cartella del progetto non torna, i membri vengono ripristinati comunque ma restano fermi, perché la cartella potrebbe essere scritta solo in parte ed è lì che di solito si trovano i bind mount di un membro. L'esecuzione di ogni membro lo segnala.

Un database che ha solo dump del database e nessun backup dei file, e il cui container e volume con nome non ci sono più, torna senza i suoi dati. Il controllo lo segnala prima del ripristino. Resta fermo, e così i container che dipendono da lui, perché nessuna app esegua la sua configurazione iniziale su un database vuoto. Avvialo, importa un dump e avvia gli altri. Vedi [Dump dei database](backups.md#database-dumps).

## Altre sorgenti {#other-sources}

- **Da una vecchia cartella di backup** apre il repository di un altro BombVault che non gira più, su un disco o in una destinazione off-site, con il suo kit di ripristino o la sua `APP_KEY` digitata. La sessione è di sola lettura e vive solo in memoria. Vedi [Da una vecchia cartella di backup](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importa dal plugin Appdata.Backup** legge la cartella di backup del plugin. Ogni archivio di container diventa un punto di ripristino del suo container, datato a quando il plugin lo ha creato. Gli archivi importati prima vengono saltati, e gli archivi stessi vengono solo letti. Il container ha bisogno prima di un backup in BombVault, così il ripristino ha la sua definizione. La conservazione lascia in pace i punti di ripristino importati, quindi eliminane uno tu quando non ti serve più.

## Dopo un ripristino {#after}

Una copia ripristinata accanto all'originale è elencata sotto **Backup** come esclusa, così non conta come compito aperto. Aggiungila alla pianificazione se vuoi tenerla. Un container o una VM che girava quando è stato fatto il backup torna in esecuzione, uno che era fermo resta fermo. Su Unraid un container ripristinato riappare nella scheda Docker come app normale e modificabile, e una VM nel VM Manager con i suoi dischi e la sua NVRAM ricollegati.
