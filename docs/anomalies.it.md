# Anomalie

Un ransomware e un'app rotta si somigliano in un backup: all'improvviso quasi tutti i dati sono nuovi, oppure quasi tutti sono spariti. BombVault confronta ogni backup con la storia dell'elemento stesso e segnala un riscontro quando qualcosa non torna. Finché un riscontro su dati persi è aperto, conserva anche i backup vecchi dell'elemento, così l'ultimo buono non può scadere. La voce **Anomalie** nella barra laterale compare finché il rilevamento è attivo.

![La pagina Anomalie con un riscontro critico aperto.](assets/screenshots/anomalies/page-dark.png#only-dark)
![La pagina Anomalie con un riscontro critico aperto.](assets/screenshots/anomalies/page-light.png#only-light)

## La pagina {#page}

I riquadri **Critica**, **Avvisi** e **Note** scelgono una gravità, e **Aperte** e **Chiuse** sopra l'elenco passano tra ciò che è aperto e ciò che si è chiuso negli ultimi 30 giorni. Un riscontro critico sta sulla stessa superficie degli altri, e la sua gravità si vede dal badge e dal bordo colorato. Un riscontro propone prima la soluzione, e i suoi dettagli mostrano il valore come una curva sull'intervallo consueto.

Quando non c'è nulla di aperto, **Avanzamento dell'apprendimento** mostra per ogni elemento un anello di dieci parti, una per ogni backup da cui BombVault ha imparato. **Monitoraggio** apre l'elenco degli elementi con le loro impostazioni.

## Cosa controlla {#checks}

Ogni backup di un container, di una VM, di un set di cartelle, di un dump del database, della chiavetta flash e dell'auto-backup viene confrontato con la storia dell'elemento. I controlli guardano:

- i dati nuovi salvati da un'esecuzione, rispetto alle quantità consuete più grandi dei backup recenti e rispetto alla velocità consueta all'ora,
- un backup che ha salvato di nuovo la maggior parte dei dati, compresi i file rinominati e riscritti,
- la dimensione della sorgente e il numero di file che restic riporta per ogni elemento e ogni dump del database,
- il tempo di backup di restic stesso,
- le serie di errori e gli errori intermittenti,
- le verifiche di ripristino che non passano più,
- lo spazio libero dei repository locali, SFTP e rclone, proiettato a partire dalla crescita del repository.

Un elemento impara dai suoi primi 10 backup. Una sorgente quasi vuota, la riscrittura di gran parte dei dati e gli errori vengono controllati fin dall'inizio.

Un riscontro sulla dimensione o sul numero di file di una sorgente è datato al primo backup in cui si è visto. **Confronta con il backup precedente** elenca le cartelle in cui file sono spariti, arrivati o cambiati, con una nota quando quasi tutto sta in un indice di ricerca, in una cache o in miniature che l'app ricostruisce da sola.

Gli elementi ZFS sono sorvegliati dataset per dataset, vedi [Dataset ZFS](zfs-datasets.md#anomalies). Un assistente può leggere i riscontri aperti tramite il [server MCP](mcp.md#tools).

I controlli hanno dei limiti. I repository S3, B2 e REST non hanno un dato sullo spazio libero, sulla condivisione utente di Unraid lo spazio libero è quello dell'intero array e i backup fatti prima di restic 0.17 non hanno uno storico delle dimensioni.

## Chiudere un riscontro {#closing}

- Gli **Avvisi** si chiudono da soli quando la causa è sparita.
- I riscontri **Critica** su dati persi e su un disco che si riempie restano finché non intervieni.
- **Conferma presa visione** segna un riscontro come visto e lo chiude. Non viene segnalato di nuovo finché la sua causa non è sparita almeno una volta.
- **Segna come attesa** dice che il cambiamento era tuo. Il nuovo livello diventa normale dopo 10 backup, ma il controllo sulla sorgente quasi vuota non viene mai disattivato.
- Una selezione cambiata, per esempio una cartella che hai spuntato o deselezionato, fa ripartire da sola la storia dell'elemento.

Una notifica parte una volta per episodio, e gli errori e le verifiche di ripristino che già notificano non vengono segnalati due volte.

## Nel frattempo i backup vecchi restano {#retention-hold}

Finché una sorgente è quasi vuota, si è ridotta molto o ha avuto di nuovo salvata la maggior parte dei suoi dati, conservazione e potatura lasciano in pace i backup vecchi di quell'elemento, e il riscontro rimanda all'ultimo backup buono, che la finestra di ripristino preseleziona. Prima che tu chiuda un riscontro simile, BombVault avvisa che la prossima pulizia eliminerà di nuovo i backup vecchi secondo le regole di conservazione. Quindi, se mancano dati o sono stati cifrati, ripristina prima e chiudi il riscontro dopo.

## Impostazioni {#settings}

La scheda **Anomalie** in **Impostazioni, Verifica** contiene le impostazioni globali. Ogni controllo si salva appena lo cambi, e i tre sotto l'interruttore sono nascosti finché il rilevamento è disattivato.

| Impostazione | Predefinito | Cosa fa |
|---|---|---|
| **Rileva le anomalie** | Attivato | Confronta ogni backup con la storia dell'elemento. Se disattivato, nulla di nuovo viene controllato e la voce **Anomalie** lascia la barra laterale. La scheda rimanda ancora ai riscontri precedenti. |
| **Sensibilità** | Equilibrata | Rigorosa segnala anche cambiamenti più piccoli, Permissiva solo quelli grandi. |
| **Invia una notifica per** | Solo riscontri critici | La gravità minima che invia un messaggio attraverso i canali impostati in [Notifiche](notifications.md). Gli errori ripetuti di backup e di dump e le verifiche di ripristino pianificate fallite inviano già un messaggio proprio e non vengono inviati due volte. |
| **Conserva i backup vecchi quando una sorgente si riduce molto o viene riscritta** | Attivato | Finché un elemento ha un riscontro aperto per una sorgente quasi vuota, una forte riduzione o la maggior parte dei suoi dati salvata di nuovo, conservazione e potatura lasciano in pace i backup vecchi di quell'elemento. Conferma la presa visione del riscontro o segnalo come atteso per lasciarli andare. |

Ogni elemento può usare una propria sensibilità e un proprio minimo di notifica. Impostali nella scheda **Verifica** dell'elemento, oppure nella pagina **Anomalie** sotto **Monitoraggio**. Per un elemento ZFS valgono per ogni dataset del suo albero.
