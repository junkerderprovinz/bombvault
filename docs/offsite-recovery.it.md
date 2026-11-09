# Off-site e ripristino

!!! note "Le copie off-site aspettano dopo una ricostruzione"
    Quando la Procedura guidata di emergenza o **Scopri backup** ricostruisce degli elementi senza le vecchie impostazioni, le copie off-site di quelle sezioni restano in attesa finché non adotti i loro backup per questo server. Vedi [Dopo una ricostruzione](#after-a-rebuild).

I backup locali ti proteggono da un container perso o da un aggiornamento sbagliato. Le copie off-site e un kit di ripristino collaudato ti proteggono dall'intero server, dal ransomware o da un incendio. Questa pagina tratta le destinazioni e le copie off-site, come rendere una copia a prova di manomissione, come dimostrare che sai ripristinare, come associare più server e come riavere tutto quando BombVault stesso non c'è più.

## Copie off-site {#off-site-copies}

Tieni il veloce backup locale e aggiungi una o più destinazioni fuori sede. BombVault copia lì i nuovi snapshot con `restic copy` al meglio delle possibilità, quindi un intoppo off-site non fa mai fallire il backup locale. In questo schema ogni sezione scrive prima nella propria cartella e il repository off-site è una copia. La cartella propria non deve per forza stare su questo server: vedi [La cartella propria di una sezione](#own-folder) per tenerla in una destinazione come S3 o un rest-server.

- **Più destinazioni per sezione.** Ogni sezione (container, VM, flash, auto-backup, cartelle e dataset ZFS) può copiare su più destinazioni off-site insieme, così puoi tenere, per esempio, un rest-server sul server di un amico e un bucket S3 in parallelo. La scheda **Destinazioni per sezione** nella pagina Destinazioni attiva o disattiva ogni collegamento da una sezione a una destinazione.
- **Quando girano le copie.** La scheda **Copia** di una destinazione imposta **Quando copiare**: **Dopo ogni backup**, oppure con un piano proprio che raccoglie gli snapshot e copia a orari fissi. In **Impostazioni, Calendario, Copie off-site per sezione**, **Eccezione** dà a una sezione un piano di copia proprio.
- **La scheda Copie off-site.** In **Impostazioni, Destinazioni** ogni sezione con una copia off-site ha una riga in **Copie off-site**, con **Prova connessione**, **Replica ora** per avviare una copia a richiesta, e **Configura** o **Modifica**. Il resto delle impostazioni di copia della sezione si apre da lì nella sua finestra.
- La **conservazione** si imposta per destinazione in **Per quanto tempo conservare**, così puoi tenere le copie off-site più a lungo come archivio. La copia nel campo off-site di una sezione la segue come ogni altra copia, e una sezione può tenere valori propri sotto **Regole per sezione**. Tutti zero non pota mai gli snapshot off-site. Vedi [Per quanto tempo conservare](#keep).
- I **limiti di banda** (**Larghezza di banda off-site** in **Impostazioni, Destinazioni**, oppure la scheda **Copia** di una destinazione) limitano la velocità di upload e download di restic così le copie non saturano la tua WAN. Si inseriscono in KiB/s, l'unità che restic accetta, e ogni campo mostra la stessa velocità in MB/s e Mbit/s mentre digiti: 2048 si legge come circa 2,0 MB/s (16,8 Mbit/s).
- La **compressione** è **Disattivato**, **Automatico** (il predefinito di restic) o **Massimo**, impostata su ogni destinazione e, per una sezione, nelle sue regole lì. Backup, copie e potature scrivono con essa, e il kit di ripristino la nomina, così restic semplice può continuare a scrivere allo stesso modo.
- **Prima lo streaming.** Mentre un media server come Plex, Jellyfin o Emby fa streaming, le copie caricano con un limite più basso e tornano a quello normale qualche minuto dopo la fine dello stream. La scheda **Streaming** nella pagina Destinazioni ha **Rallenta le copie off-site durante lo streaming**, **Media server** (container con Plex, Jellyfin o Emby nel nome dell'immagine finché non scegli), **Streaming oltre (Mbit/s)**, **Limite upload durante lo streaming (KiB/s)** e **Ritorno alla normalità dopo (min)**. BombVault legge il traffico da Docker, quindi un media server sulla rete host non si può misurare. REST, S3, B2, Azure, Google Cloud, Swift e rclone su HTTP rallentano nel mezzo di una copia. SFTP e le cartelle locali o montate prendono il limite più basso al passo di copia successivo.
- **Archiviazione fredda (S3).** Una destinazione S3 può usare una **Classe di archiviazione** più economica che restic riesce comunque a leggere subito, come Standard-IA, One Zone-IA, Intelligent-Tiering o Glacier Instant Retrieval. Glacier Flexible Retrieval e Deep Archive sono esclusi, perché restic non può ripristinare da essi. Un remote rclone imposta la sua classe nella configurazione di rclone.
- **Budget di crescita.** **Budget di crescita (GB, 0 = disattivato)** nella finestra di copia di una sezione fa scattare un allarme quando il repository cresce oltre quella dimensione. Non elimina mai nulla, quindi va bene per una copia append-only da cui da qui non pota nessuno.
- Mentre una copia gira, l'intestazione di stato della panoramica dice **Copia off-site in corso**. È un indicatore attivo, non una barra di percentuale, perché `restic copy` non espone un avanzamento leggibile da una macchina.

!!! note "Ripristino da qualsiasi posto"
    Ogni elemento elenca i suoi backup come un'unica linea del tempo attraverso tutti i posti in cui si trova un backup, nella sua scheda **Backup**. Un backup copiato su B2 compare una volta, contrassegnato con ogni posto che lo contiene. Un ripristino prende il primo posto che riesce a raggiungere, a partire dal repository su cui l'elemento scrive, e puoi scegliere un altro posto per ogni snapshot. Le destinazioni off-site vengono lette solo quando clicchi **Verifica**, perché elencare un archivio cloud richiede un po'. Eliminare in un solo posto controlla prima gli altri e dice se era l'ultima copia. Vedi [Ripristino](restore.md).

## Destinazioni {#destinations}

Una destinazione è un posto in cui vanno i backup: una cartella locale, una condivisione, un server tuo o un servizio cloud. Si configura una volta e poi serve ogni sezione. Apri **Destinazioni** dal suo riquadro delle impostazioni, oppure attiva lì **Mostra anche nella barra laterale** per fissarla.

![La pagina Destinazioni: l'elenco delle destinazioni e il flusso di quale sezione va dove.](assets/screenshots/storage/locations-dark.png#only-dark)
![La pagina Destinazioni: l'elenco delle destinazioni e il flusso di quale sezione va dove.](assets/screenshots/storage/locations-light.png#only-light)

**Aggiungi destinazione** apre una finestra in quattro passaggi:

1. **Fornitore.** Ogni servizio è elencato con il suo logo, con una ricerca in alto a destra nella finestra, in gruppi: **Locale** per un disco o una condivisione collegata a questo server, **NAS o server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV e altri), **Servizio S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 e altri), il tuo server S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Archiviazione cloud** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud e il resto che rclone supporta) e **Dal tuo gruppo**, i server riceventi del tuo gruppo. Ognuno dice quanto si adatta ai backup: le unità cloud rallentano sotto molte richieste, quindi il primo backup e la potatura richiedono più tempo lì. Per un servizio che nessun riquadro copre, il riquadro **Rclone** sotto **Configurazione rclone personale** accetta remote incollati da un rclone.conf.
2. **Accedi.** I campi dipendono dal servizio: una chiave di accesso per S3, un utente e una password per WebDAV e SMB, una password per app dove l'accesso a due fattori blocca quella normale, la chiave SSH pubblica di BombVault per SFTP e la Storage Box, oppure un token per servizi che accedono tramite un browser. Per questi, la finestra mostra un comando `rclone authorize` da eseguire su un computer con un browser, e il token che stampa va nel campo. Dove hai già delle [credenziali salvate](#sign-ins) per il servizio, **Credenziali salvate** le sceglie al posto di **Nuove chiavi**. **Prova connessione** controlla l'accesso prima che venga salvato qualsiasi cosa. La finestra elenca poi le cartelle della destinazione, con **Nuova cartella** per crearne una e lo spazio libero dove il servizio lo riporta. Una cartella vuota è la più sicura. BombVault vi mette una cartella per sezione.
3. **Protezione.** La finestra dice chiaramente cosa sa fare il servizio. Un rest-server in modalità append-only rifiuta l'eliminazione, e la prova di manomissione lo verifica. Un bucket S3 può conservare vecchie versioni con il versioning e l'object lock, cosa che BombVault non può ancora controllare. Un'unità cloud non può rifiutare l'eliminazione affatto: chi entra nel server entra anche in quella copia. Attiva **Protezione dall'eliminazione (append-only)** solo dove il lato remoto rifiuta davvero l'eliminazione. BombVault non pota mai lì. **Per le emergenze** dice cosa contiene il kit di ripristino per questa destinazione.
4. **Per quali sezioni.** Scegli le sezioni che salvano qui. La prima copia porta con sé l'intera storia. Gli elementi che hanno scelto destinazioni proprie non ricevono la nuova da soli, e la finestra li nomina.

![Aggiungi destinazione: il primo passaggio, con ogni fornitore come riquadro.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Aggiungi destinazione: il primo passaggio, con ogni fornitore come riquadro.](assets/screenshots/storage/add-providers-light.png#only-light)

![Aggiungi destinazione: il passaggio di accesso dopo una prova riuscita.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Aggiungi destinazione: il passaggio di accesso dopo una prova riuscita.](assets/screenshots/storage/add-signin-light.png#only-light)

Alcuni servizi chiedono qualcosa in più all'accesso:

- **SFTP e la Storage Box.** **Aggiungi chiave** mette la chiave pubblica di BombVault sul server con la **Password dell'account**, usata per quell'unico accesso e non conservata da nessuna parte. Il comando per farlo a mano è mostrato sotto. La prova mostra la **Chiave del server** con la sua impronta. BombVault la ricorda e rifiuta il server se in seguito ne mostra una diversa.
- **Chiavi S3 per un solo bucket**, come un token oggetto di Cloudflare R2 o una chiave AWS limitata, possono non elencare gli altri bucket. Scrivi il bucket in **Bucket**, e la prova e l'elenco delle cartelle aprono quel bucket.
- **Regioni e indirizzi S3.** La regione viene letta da un endpoint che la nomina (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage parte con una regione predefinita propria, e IDrive e2 chiede l'endpoint che mostra la sua dashboard. Un indirizzo incollato con il bucket, come lo mostra la dashboard di R2, compila il server e il bucket. Un bucket creato dal browser delle cartelle va nella regione che il modulo nomina.
- **Classe di archiviazione.** Il selettore offre solo le classi che il servizio accetta, e **Altre impostazioni** mostra solo le opzioni che determinano un nuovo bucket.
- **Google Drive e OneDrive.** I dati potati vengono eliminati in via definitiva invece di finire nel cestino, dove conterebbero comunque nella quota. Un'unità condivisa di Google va sotto **Altre impostazioni**.
- **I server SMB che non elencano le loro condivisioni** accettano il nome di una sotto **Condivisione**, e la prova apre direttamente quella condivisione.
- **FTP** chiede la sua **Crittografia**: **Nessuna**, **TLS esplicito (FTPES)** o **TLS implicito (FTPS, porta 990)**.
- **OneDrive e Zoho WorkDrive.** Quando un accesso apre più unità o spazi di lavoro, la finestra chiede in quale vanno i backup.
- **Un rest-server con un certificato autofirmato.** Metti il certificato come file PEM nella cartella `certs` della cartella dati di BombVault (`/config/certs` nel container, `appdata/bombvault/certs` su Unraid). Backup, copie e prova della protezione dall'eliminazione lo accettano senza riavvio.

I servizi S3 passano dal backend S3 di restic stesso, ed è questo che permette di applicare una classe di archiviazione e l'object lock. Un rest-server viene raggiunto direttamente da restic. Ogni altro servizio passa dal rclone che BombVault include, e la sua sezione della configurazione di rclone compare nella scheda **rclone.conf** nella pagina della destinazione, con i segreti nascosti. Un'esportazione delle impostazioni porta le destinazioni, e con le credenziali incluse porta anche i loro accessi.

Salvare una destinazione che contiene già backup di container, VM o cartelle, per esempio una che hai rimosso e aggiungi di nuovo, la aggiunge a quelle sezioni come copia non spuntata. Lì non viene copiato nulla finché non la spunti, ma **Ripristina** offre subito i vecchi backup.

La cartella su questo server si chiama **Locale** ovunque: nelle pagine di archiviazione, nel flusso, su un elemento e in Ripristino. Con più cartelle su questo server, ciascuna ha il suo nome. Una destinazione che il campo off-site di una sezione configura prende il nome dal suo posto, l'ultima cartella del suo indirizzo o, senza di essa, il suo server, e riceve un numero quando un'altra destinazione ha già quel nome. Ogni destinazione si può rinominare nel campo **Nome** della sua scheda **Connessione**, anche una cartella bucket che più sezioni condividono e la copia che descrive il campo off-site di una sezione, e un nome che le dai resta. Dove due destinazioni con lo stesso nome stanno una accanto all'altra, ciascuna porta il suo server o la sua cartella.

### La pagina di una destinazione {#location-page}

Ogni destinazione ha una pagina propria. Mostra solo le schede che si applicano a quella destinazione, in quest'ordine:

![La pagina di una destinazione: Cartelle proprie, Connessione e Credenziali.](assets/screenshots/storage/location-page-dark.png#only-dark)
![La pagina di una destinazione: Cartelle proprie, Connessione e Credenziali.](assets/screenshots/storage/location-page-light.png#only-light)

- **Cartelle proprie**: le sezioni che scrivono qui per primi i loro backup. Le credenziali, la protezione e i limiti di questa destinazione valgono per loro. **Usa come cartella propria** sposta qui la cartella propria di altre sezioni, e **Cambia** ne sposta una altrove. Vedi [La cartella propria di una sezione](#own-folder).
- **Salva direttamente qui**: ogni sezione i cui elementi salvano direttamente in questo posto invece che nella loro cartella propria, e quanti elementi lo fanno. Un repository diretto simile prende tutte le sue impostazioni da questa destinazione. Uno su cui non salva più nulla lo dice, e **Rimuovi** lo toglie dall'elenco. I backup nel suo posto restano. Vedi [Repository diretti](#direct-repositories).
- **Connessione**: il **Nome**, l'interruttore **Disponibile** (disattivato significa che BombVault non scrive più qui, e gli elementi che salvano qui falliscono con un messaggio chiaro invece di finire altrove), la cartella, **Compressione**, **Classe di archiviazione** e **Fuori sede**. Sulla destinazione **Locale** elenca la cartella di ogni sezione sotto **Cartelle proprie qui**. **Cambia connessione** prova e salva un nuovo indirizzo o nuove credenziali. Quando altri posti accedono con le stesse credenziali salvate, la finestra lo dice, e una modifica qui dà a questa destinazione credenziali proprie invece di riscrivere quelle condivise.
- **Credenziali**: quali credenziali salvate usa (**Accede con**), quali altre destinazioni le usano a loro volta (**Usate anche da**), e **Modifica**, che apre la finestra delle credenziali. Vedi [Credenziali](#sign-ins).
- **rclone.conf**: la sezione di questa destinazione della configurazione di rclone, per una destinazione che passa da rclone.
- **Protezione dall'eliminazione**, con **Protezione dall'eliminazione (append-only)**, **Accetta senza protezione dall'eliminazione**, **Prova subito la protezione dall'eliminazione** e **Chi pota**, che dice se BombVault pota la copia e le cartelle proprie qui, secondo la scheda **Per quanto tempo conservare** di questa destinazione o i valori personalizzati di una sezione sotto **Regole per sezione**, o lo lascia al lato remoto. Una cartella locale mostra questa scheda solo quando ha una protezione che puoi disattivare. Vedi [Protezione dall'eliminazione](#delete-protection-append-only).
- **Copia**: **Quando copiare**, e **Limite upload (KiB/s)** e **Limite download (KiB/s)**. Una destinazione che contiene solo cartelle proprie e nessuna copia mostra i limiti come una scheda chiamata **Larghezza di banda**.
- **Per quanto tempo conservare**: **Breve**, **Equilibrato**, **Lungo** o **Valori personalizzati**, con una linea del tempo di quali backup la regola conserva. Vedi [Per quanto tempo conservare](#keep).
- **Già sotto questa destinazione**: sezioni il cui indirizzo digitato a mano sta in una cartella di questa destinazione. Adottarne una lascia il repository dov'è. Credenziali, classe di archiviazione e protezione dall'eliminazione arrivano allora dalla destinazione. BombVault controlla prima che le credenziali della destinazione aprano il repository, e rifiuta di mettere un repository append-only sotto una destinazione che non è append-only.
- **Regole per sezione**: il repository, la conservazione o la compressione propri di una sezione a questa destinazione. **Modifica le regole** le apre, e **Torna alla destinazione** riporta una sezione indietro. Credenziali, larghezza di banda e quando girano le copie arrivano sempre dalla destinazione.
- Un recipiente che mostra quanto è piena la destinazione, con il segno di dove è piena e quante settimane restano alla crescita delle ultime settimane. Una destinazione che non riporta la sua dimensione dice **Dimensione sconosciuta**.

**Sposta** e **Rimuovi destinazione** stanno tra le azioni della pagina, descritte in [Spostare e rimuovere](#move-remove).

Una destinazione remota che conteneva backup e all'improvviso non contiene alcun repository, come un rest-server il cui disco non si è montato, non ne riceve da sola uno nuovo e vuoto. La sua pagina lo dice non appena un backup o una copia trova il repository sparito. Monta di nuovo il disco. Se hai svuotato di proposito il vecchio repository, **Crea qui un nuovo repository** ricomincia, per ogni sezione della destinazione e i suoi repository insieme. La protezione vale per una destinazione rimasta al suo posto. Una destinazione che aggiungi, o una sezione che spunti su una di esse, parte pulita: senza repository a quell'indirizzo, la prima copia ne crea uno, anche dove una destinazione rimossa in precedenza ne aveva uno.

Due rifiuti indicano cosa sistemare, nell'esecuzione della copia e nella pagina della destinazione. **Questo bucket non esiste sul server** compare non appena un server S3 lo dice, e BombVault lì non crea nulla, così un nome di bucket digitato male resta visibile. Un server che non risponde affatto è un altro caso: restic continua a riprovare per circa 15 minuti per chiamata, nel caso torni. **Qui c'è già un repository restic e la chiave di questa istanza non lo apre** significa che i backup lì appartengono a un'altra `APP_KEY`.

Un server ricevente che un'altra istanza del tuo gruppo gestisce compare tra i fornitori come **Dal tuo gruppo**. Vedi [Server ricevente](#receiving-server).

BombVault controlla le sue destinazioni a ogni avvio. Sistema ciò che non richiede una tua decisione, come un collegamento a una copia o a un repository che non c'è più, una copia su una cartella propria di una sezione, o un repository diretto la cui copia non c'è più, e scrive una riga nel log per ogni correzione. Ciò che non ha una correzione sicura viene solo registrato.

![La stessa pagina su un telefono.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![La stessa pagina su un telefono.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Per quanto tempo conservare {#keep}

Per quanto tempo i backup vengono conservati si imposta per destinazione, nella sua scheda **Per quanto tempo conservare**. La regola copre le copie a quella destinazione e le cartelle proprie che stanno lì, compresa la destinazione **Locale**. Una sezione può tenere valori propri a una destinazione sotto **Regole per sezione**, e questi prevalgono sulla scheda della destinazione per quella sezione.

Una regola conserva un backup per giorno, settimana, mese e anno, fin dove l'hai impostata, e toglie il resto dopo ogni backup o copia:

| Preimpostazione | Ultimi backup | Giorni | Settimane | Mesi | Anni |
|---|---|---|---|---|---|
| **Breve** | 0 | 7 | 4 | 3 | 0 |
| **Equilibrato** | 0 | 7 | 4 | 6 | 2 |
| **Lungo** | 0 | 14 | 8 | 12 | 3 |

**Valori personalizzati** imposta tu i cinque conteggi. Uno 0 disattiva quel conteggio, e una regola con ogni conteggio a 0 conserva tutto. I punti nella scheda mostrano quali di 40 backup di esempio una regola conserva, il più recente all'estrema destra. L'esempio è lo stesso per ogni destinazione, poiché l'immagine viene disegnata prima che BombVault abbia letto i backup lì. Conta all'indietro da oggi, quindi il numero di punti che restano cambia con la data.

Anche una cartella su questo host e una cartella remota digitata a mano hanno una scheda propria, che copre le cartelle proprie in quella cartella e nessun'altra. Finché non la salvi, una cartella del genere invecchia secondo la regola che BombVault aveva già per lei.

**Chiedi prima di conservare meno**, attivo di default, chiede conferma prima che venga salvata una modifica che conserva meno backup, e dice quanti dei 40 backup di esempio la nuova regola elimina. Vale per ogni destinazione. **Torna alla destinazione** sotto **Regole per sezione** pone la stessa domanda.

Una destinazione con protezione dall'eliminazione non viene mai potata da questo server. Il suo lato remoto conserva o scarta i vecchi backup con le proprie regole. Finché un elemento ha un'anomalia critica aperta su dati persi, la conservazione tiene i vecchi backup di quell'elemento finché non confermi la presa visione. Vedi [Anomalie](anomalies.md).

Un backup che contiene meno del dovuto non fa mai potare quelli precedenti. Si tratta di un backup che non legge alcun file dove i precedenti ne contenevano, a cui manca una cartella selezionata, o che non è riuscito a leggere alcuni file. La sua esecuzione termina con un avviso che nomina ciò che manca e dice che i backup più vecchi sono stati conservati, ricevi lo stesso messaggio sui tuoi canali di notifica, e nessuna potatura, né nella cartella propria dell'elemento né nelle sue copie, tocca i vecchi backup di quell'elemento finché non arriva un backup completo. Un primo backup, e una cartella selezionata che è sempre stata vuota, contano come completi.

Per vedere cosa eliminerebbe davvero la prossima esecuzione, apri **Anteprima** in **Impostazioni, Destinazioni**. Elenca **Cosa rimuoverebbe la prossima esecuzione** per la cartella propria della sezione e per ognuna delle sue copie, per repository e per elemento con i punti di ripristino nominati. Non prende alcun lock e non cambia nulla.

![Per quanto tempo conservare con Equilibrato scelto: i punti mostrano quali dei 40 backup di esempio restano.](assets/screenshots/storage/keep-dark.png#only-dark)
![Per quanto tempo conservare con Equilibrato scelto: i punti mostrano quali dei 40 backup di esempio restano.](assets/screenshots/storage/keep-light.png#only-light)

#### Esempio: cosa conserva Equilibrato {#example-retention}

Un container viene salvato ogni notte alle 03:00 dal 1 ottobre 2024, quindi mercoledì 4 novembre 2026 ha avuto 765 backup. La sua destinazione è impostata su **Equilibrato**: 7 giorni, 4 settimane, 6 mesi e 2 anni.

Ogni conteggio conserva il backup più recente del suo giorno, settimana, mese o anno, e un backup scelto da più conteggi viene conservato una volta. Le settimane vanno da lunedì a domenica:

| Regola | Backup che sceglie | Nuovi che aggiunge |
|---|---|---|
| 7 giorni | 4 nov, 3 nov, 2 nov, 1 nov, 31 ott, 30 ott, 29 ott | tutti e 7 |
| 4 settimane | 4 nov, 1 nov, 25 ott, 18 ott | 25 ott, 18 ott |
| 6 mesi | 4 nov, 31 ott, 30 set, 31 ago, 31 lug, 30 giu | 30 set, 31 ago, 31 lug, 30 giu |
| 2 anni | 4 nov, 31 dic 2025 | 31 dic 2025 |

Restano così 14 backup su 765. Il più vecchio è del 31 dicembre 2025, quindi oggi puoi tornare indietro di poco più di dieci mesi. Grazie alla deduplicazione occupano poco più di un backup completo più ciò che è cambiato nel frattempo.

**Anni** merita un secondo sguardo. Conta gli anni di calendario con un backup al loro interno, a partire da quello corrente, e il backup più recente di quest'anno è quello di oggi. Quindi il primo anno non aggiunge nulla agli altri conteggi, e il secondo aggiunge l'ultimo backup dell'anno precedente. Per questo la portata di una regola cambia nel corso dell'anno. Gli stessi backup con ogni preimpostazione:

| Preimpostazione | Backup conservati oggi | Backup più vecchio oggi | Portata nel corso dell'anno |
|---|---|---|---|
| **Breve** | 10 | 30 settembre 2026 | da 1 a 2 mesi |
| **Equilibrato** | 14 | 31 dicembre 2025 | circa 4 mesi a inizio primavera, un anno intero il 31 dicembre |
| **Lungo** | 30 | 31 dicembre 2024 | 1 anno il 1 gennaio, 2 anni il 31 dicembre |

Passare da **Equilibrato** a **Breve** chiede prima conferma, a causa di **Chiedi prima di conservare meno**. La domanda dice quanti dei 40 backup nell'immagine la nuova regola elimina. Quel numero viene dall'esempio nell'immagine, quindi cambia con la data e non conta i tuoi backup. Dei backup di questo container andrebbero via il 31 agosto, il 31 luglio, il 30 giugno e il 31 dicembre 2025. **Salva comunque** applica la regola, e la prossima potatura li elimina da quella destinazione in via definitiva.

Per tenere i container più a lungo del resto a questa destinazione, premi **Modifica le regole** sotto **Regole per sezione**, scegli **Container** e imposta **Valori personalizzati** in **Regole per Container**, per esempio 14 giorni, 8 settimane, 12 mesi e 2 anni. Le altre sezioni continuano a seguire la scheda della destinazione, e **Torna alla destinazione** riporta indietro Container.

### Credenziali {#sign-ins}

Le credenziali sono la chiave o la password che BombVault usa per un servizio S3 o un rest-server. Sono un unico elenco, la scheda **Credenziali** in **Impostazioni, Destinazioni**, e ogni riga dice quali destinazioni le usano (**Usate da**).

- **Salvate quando aggiungi una destinazione.** Aggiungi destinazione salva le chiavi che digiti come credenziali. La destinazione successiva sullo stesso servizio può sceglierle sotto **Credenziali salvate** invece di **Nuove chiavi**, e una modifica fatta dopo vale per ogni destinazione che le usa.
- **Quelle predefinite.** Una stella segna le credenziali predefinite. Si usano dove non è scelta alcuna credenziale, come un indirizzo digitato a mano o un ripristino su un'installazione nuova. Una sorgente di prelievo non le prende in prestito. **Imposta come predefinite** dà la stella ad altre.
- **Modifica e Prova.** **Modifica**, qui o nella scheda **Credenziali** di una destinazione, apre un'unica finestra per le credenziali. **Prova** accede a ogni destinazione che le usa con ciò che hai digitato, prima che venga salvato qualsiasi cosa. **Salva** le memorizza.
- **Rimozione.** Delle credenziali si possono rimuovere solo quando nessuna destinazione, sorgente di prelievo o repository ricevuto le usa e nessun vecchio backup ne ha bisogno, e quelle predefinite se ne vanno solo come ultime. I vecchi backup ne hanno bisogno quando una cartella propria si è spostata da un posto che accede con esse e li ha lasciati lì. BombVault dice allora di quale posto si tratta.
- **Cambiare la connessione di una destinazione** non riscrive mai credenziali condivise da altri posti. La destinazione ne riceve invece di proprie, e la finestra dice prima che le sue credenziali sono condivise. Per cambiare ovunque quelle condivise, modificale nella scheda **Credenziali**.

![La scheda Credenziali: ogni credenziale salvata con ciò che la usa, e la stella su quelle predefinite.](assets/screenshots/storage/signins-dark.png#only-dark)
![La scheda Credenziali: ogni credenziale salvata con ciò che la usa, e la stella su quelle predefinite.](assets/screenshots/storage/signins-light.png#only-light)

Una cartella su questo server non richiede credenziali. SMB, WebDAV, SFTP, la Storage Box e le unità cloud tengono le loro nella configurazione di rclone, che la scheda **rclone.conf** della destinazione mostra, quindi non compaiono in questo elenco.

### SMB, WebDAV e altri remote rclone {#smb-webdav}

Una condivisione Windows o Samba e un server WebDAV (Nextcloud, ownCloud, SharePoint o qualsiasi altro) sono riquadri di fornitori in **Aggiungi destinazione**, sotto **NAS o server**. Compila l'host e la condivisione (SMB) oppure l'indirizzo e il tipo di server (WebDAV), l'utente e la password, e BombVault scrive per te la sezione rclone. rclone offusca la password prima che venga salvata.

Per un servizio che nessun riquadro copre, il riquadro **Rclone** sotto **Configurazione rclone personale** accetta remote incollati da un rclone.conf. Vengono aggiunti ai remote che BombVault ha già e non li sostituiscono mai, e sono salvati cifrati. Per salvare una sezione su uno di essi, inserisci `rclone:<remote>:<folder>` come repository off-site di quella sezione, per esempio `rclone:nas:backups/bombvault`. La condivisione o il bucket è il primo segmento del percorso, non parte del nome.

Questa è la strada migliore rispetto a montare la condivisione su Unraid: restic sconsiglia di tenere un repository su una condivisione CIFS montata, e qui non si monta nulla. NFS non ha un riquadro perché né restic né rclone hanno un backend NFS. Per NFS, monta l'export sull'host e usalo come cartella locale.

### Spostare e rimuovere {#move-remove}

- **Sposta** porta la copia di una sezione su un'altra destinazione, con tutta la sua storia. Tutto resta nel vecchio posto. BombVault non elimina nulla lì. Lo spostamento gira in background e può durare a lungo: circa 9 minuti ogni 100 GB tra dischi locali, ore su un collegamento lento. La panoramica lo mostra mentre gira.
- **Rimuovi destinazione** offre due modi. **Rimuovi destinazione, mantieni i backup** smette di scrivere lì e lascia i backup al loro posto. Potrai leggerli più tardi in **Ripristino, Da una vecchia cartella di backup**. **Rimuovi la destinazione ed elimina i backup lì** legge prima ciò che è salvato lì e lo elenca: quali backup vanno via, quali di essi non esistono altrove e quali repository restano perché la protezione dall'eliminazione rifiuta, degli elementi salvano ancora direttamente su di essi, un'altra destinazione o sezione vi tiene le sue copie, oppure non si sono potuti leggere. Gli elementi che perderebbero il loro unico backup sono elencati a parte e vengono conservati a meno che tu attivi **Elimina anche queste**. Confermi digitando il nome della destinazione, e l'eliminazione gira in background.
- Questo vale per ogni tipo di destinazione: una che usano più sezioni, una destinazione configurata per una sola sezione, un repository che hai configurato e la cartella in cui una sezione scrive i propri backup. Gli elementi che salvano su un repository rimosso tornano a seguire la loro sezione, e lo stesso vale per un valore predefinito di sezione che puntava lì. La cartella di una sezione non può semplicemente sparire, perché ogni sezione ne ha bisogno di una. La finestra chiede la cartella in cui si spostano le sezioni, e ogni sezione mantiene sotto di essa il nome della propria cartella.
- Eliminare i backup di una destinazione funziona anche senza password di accesso. Se una password è impostata, richiede un accesso sull'istanza stessa.

### Esempio: una copia off-site su Backblaze B2 {#example-s3}

I container e l'auto-backup devono vivere anche su Backblaze B2, così un server morto non se li porta via. Gli stessi passaggi vanno bene per Wasabi, Hetzner Object Storage e la maggior parte degli altri servizi S3.

1. **In B2** crea un bucket privato, come `tower-backups`, in una regione vicina a te. In **Application Keys** crea una chiave con accesso in lettura e scrittura, limitata a quel bucket, e annota il suo ID chiave e la chiave dell'applicazione. B2 mostra la chiave dell'applicazione una volta sola. Imposta il ciclo di vita del bucket per conservare solo l'ultima versione, altrimenti B2 tiene i dati potati come versioni nascoste e te li fattura.
2. **Fornitore.** In BombVault apri **Destinazioni**, premi **Aggiungi destinazione** e scegli **Backblaze B2** sotto **Servizio S3**.
3. **Accedi.** Inserisci l'ID chiave come **ID chiave di accesso** e la chiave dell'applicazione come **Chiave di accesso segreta**. Inserisci l'endpoint del bucket sotto **Punto di connessione**, come `s3.eu-central-003.backblazeb2.com`, e BombVault ne legge la regione. Scrivi `tower-backups` in **Bucket**, perché questa chiave apre un solo bucket.
4. **Prova connessione.** Dovrebbe dire **Connessione stabilita** ed elencare le cartelle del bucket. Premi **Nuova cartella** e chiamala `bombvault`. BombVault vi mette una cartella per sezione, come `bombvault/containers`.
5. **Protezione.** La finestra spiega che B2 può conservare vecchie versioni con l'object lock e che BombVault non può verificarlo. Lascia disattivata **Protezione dall'eliminazione (append-only)** a meno che il bucket rifiuti davvero le cancellazioni, così BombVault può potare secondo le regole di conservazione della destinazione.
6. **Per quali sezioni.** Spunta **Container** e **Auto-backup**. L'auto-backup porta le tue impostazioni, così una nuova installazione può riaverle. La finestra dice quanti elementi e, dove noto, quanta storia porta con sé la prima copia.
7. Premi **Salva destinazione**.

La prima copia gira dopo il prossimo backup. Per avviarla subito, apri **Impostazioni, Destinazioni** e premi **Replica ora** nella riga della sezione in **Copie off-site**. L'intestazione di stato dice **Copia off-site in corso**. La prima copia porta l'intera storia, quindi 200 GB su un uplink da 100 Mbit/s richiedono circa quattro ore e mezza. Le copie successive inviano solo ciò che è nuovo.

Per tenere libera la tua linea, apri la pagina della destinazione e imposta **Limite upload (KiB/s)** nella scheda **Copia**, per esempio su `5120`, che si legge come circa 5,0 MB/s (41,9 Mbit/s). Già che ci sei, controlla **Per quanto tempo conservare** nella stessa pagina.

Per vedere che ha funzionato, guarda **Stato di protezione** nella panoramica, dove **Off-site** diventa **copia aggiornata**, e la scheda **Backup** di un container, dove i suoi snapshot nominano ora `tower-backups` come secondo posto. Poi premi di nuovo **Scarica il kit di ripristino** in **Impostazioni, Ripristino**, perché il kit elenca la nuova destinazione e le sue credenziali.

**Cloudflare R2 funziona allo stesso modo,** con un token oggetto invece di una chiave. Incolla l'indirizzo S3 che mostra la dashboard di R2, con il bucket, e BombVault compila il server e il bucket.

### Esempio: un rest-server gestito da te {#example-rest-server}

Un rest-server in modalità append-only rifiuta le cancellazioni, quindi una copia su di esso sopravvive anche se qualcuno prende il controllo del tuo server. Basta una seconda macchina, a casa di un amico o in garage. Un BombVault associato può configurarlo per te, vedi [Server ricevente](#receiving-server). Per avviarlo a mano su qualsiasi host Docker:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

L'ultima riga chiede una password e scrive l'utente nel file `.htpasswd` nella cartella dei dati.

1. In BombVault premi **Aggiungi destinazione** e scegli **rest-server** sotto **NAS o server**.
2. Inserisci l'indirizzo con l'utente come sua cartella, come `http://192.168.2.20:8000/bombvault`, e il **Nome utente** e la **Password**. Premi **Prova connessione**.
3. In **Protezione** attiva **Protezione dall'eliminazione (append-only)**. BombVault non pota allora mai lì e lo lascia al lato remoto.
4. Spunta le sezioni e premi **Salva destinazione**.
5. Quando la prima copia c'è, apri la pagina della destinazione e premi **Prova subito la protezione dall'eliminazione**. BombVault prova a eliminare un oggetto di prova. La scheda dovrebbe dire **Ultimo: eliminazione rifiutata**, e **Protezione dall'eliminazione** nella panoramica diventa **controllata**. Se dice **Ultimo: eliminazione non rifiutata**, manca `--append-only`.

Poiché da qui nulla pota questa copia, imposta **Budget di crescita (GB, 0 = disattivato)** nella finestra della sezione, aperta con **Modifica** nella sua riga di **Copie off-site**, così lo sai prima che il disco si riempia. Con un certificato autofirmato vedi la nota su `certs` sopra.

## La cartella propria di una sezione {#own-folder}

Ogni sezione ha una cartella propria: il repository in cui i suoi backup vengono scritti per primi. Le copie verso altre destinazioni partono da lì. Una nuova installazione tiene le cartelle proprie su questo server, sotto `/mnt/user/bombvault`, e insieme formano la destinazione **Locale**.

Una cartella propria può anche stare in una destinazione che hai già, come un NAS, un rest-server o un bucket S3. La sezione salva allora lì direttamente, senza una copia locale nel mezzo, e la cartella propria segue le credenziali, la protezione dall'eliminazione e i limiti di banda di quella destinazione. BombVault la pota dopo ogni backup secondo la scheda **Per quanto tempo conservare** della destinazione, e i valori personalizzati di una sezione sotto **Regole per sezione** hanno la precedenza sulla scheda. La destinazione elenca la sezione nella sua scheda **Cartelle proprie**, e **Destinazioni per sezione** mostra la destinazione come casa della sezione.

### Spostare una cartella propria {#move-own-folder}

1. Apri la destinazione che deve contenere la cartella propria e scegli **Usa come cartella propria**, oppure **Aggiungi sezioni** nella sua scheda **Cartelle proprie**. Per spostare più sezioni insieme, scegli **Sposta cartelle proprie** accanto a **Cartelle proprie qui** nella scheda **Connessione** della destinazione Locale.
2. Scegli le sezioni sotto **Sezioni** e il posto sotto **Sposta in**. **Un altro posto** accetta una cartella su questo host o l'indirizzo di un repository remoto digitato a mano, con delle credenziali tra quelle salvate.
3. Leggi **Cosa succede**. Per ogni sezione dice dove vanno i nuovi backup, se una copia che si trova già nel nuovo posto diventa la cartella propria e quali backup vengono copiati prima.
4. **Copia i backup già fatti** è attivo di default. Se lo disattivi, i vecchi backup restano dove sono, ma l'elemento non li elenca più. **Da una vecchia cartella di backup** nella pagina Ripristino li legge comunque con l'indirizzo della cartella e, per una cartella remota, le sue credenziali di accesso.
5. Scegli **Passa**. Quando la scheda **Per quanto tempo conservare** del nuovo posto conserva meno della regola attuale della sezione, BombVault chiede prima **Conservare meno?** Lì, **Mantieni i valori attuali** di quella sezione **in Regole per sezione** porta con sé i valori di oggi, **Salva comunque** va avanti e **Indietro** lascia tutto com'era. Quando i backup devono essere copiati prima, BombVault li copia e poi passa alla cartella propria. Quando ci sono già, li controlla prima del passaggio.

Dopo lo spostamento, i nuovi backup vanno nella cartella propria spostata, anche per una sezione che aveva **Locale** disattivato e salvava direttamente su una copia. Una copia che si trovava già nel nuovo posto diventa la cartella propria, e la copia lì termina. Dove quella copia teneva alcuni backup più a lungo delle regole di conservazione della sezione, la sezione conserva ciò che conserva una delle due regole, così nessuno va perso.

Quando lo spostamento non può proseguire, la finestra ne dice il motivo in un riquadro sotto **Cosa succede**. Quando a bloccare è solo la copia dei vecchi backup, disattivare **Copia i backup già fatti** lascia spostare la cartella propria senza di essi.

Una sezione ha sempre bisogno di una cartella propria. Rimuovere la destinazione che la contiene chiede la cartella in cui si sposta la sezione, vedi [Spostare e rimuovere](#move-remove). Una cartella propria su cui i nuovi backup non passano, perché la sezione salva direttamente su una copia, è segnata **(spento)** dove una destinazione elenca chi la usa.

![La finestra che sposta una cartella propria, con Cosa succede compilato per una sezione.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![La finestra che sposta una cartella propria, con Cosa succede compilato per una sezione.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Esempio: i container passano a un NAS {#example-own-folder}

Il pool cache si sta riempiendo, e hai una destinazione chiamata `nas`, una condivisione SMB su una seconda macchina. La sezione Container ha 61 backup nella sua cartella propria su questo server e copia su Backblaze B2.

1. Apri **Destinazioni**, poi `nas`, e premi **Usa come cartella propria**.
2. Spunta **Container** sotto **Sezioni**.
3. Leggi **Cosa succede**. Dice che i nuovi backup di Container vanno nella cartella su `nas`, che i 61 backup in Locale vengono copiati lì per primi, che le copie continuano verso Backblaze B2 e che protezione, limiti di banda e allarme di crescita arrivano da `nas`.
4. Lascia attivo **Copia i backup già fatti** e premi **Passa**.

BombVault copia i backup in background e poi passa alla cartella propria. La pagina `nas` elenca Container sotto **Cartelle proprie**, un elemento che segue la sua sezione nomina `nas` come casa, e il prossimo backup finisce lì, con la copia verso B2 che parte da `nas`. Da ora in poi la scheda **Per quanto tempo conservare** di `nas` pota la cartella. La vecchia cartella su questo server resta dov'è, e BombVault non elimina nulla lì.

### Digitare a mano un indirizzo remoto {#remote-primary-repositories}

La scheda **Connessione** della destinazione Locale accetta ancora un indirizzo remoto per la cartella propria di una sezione: **Cambia** accanto alla sezione apre una finestra con un interruttore **Locale / Remoto**. **Remoto** accetta un indirizzo restic come `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` o `rclone:remote:bucket/path`, e le **Impostazioni di sicurezza del repository primario remoto** aggiungono una prova di connessione, limiti di upload e download, la protezione append-only con la stessa prova di manomissione che riceve una destinazione off-site e un allarme sul budget di crescita.

Un indirizzo simile accede con le credenziali predefinite. Quando è esattamente la cartella che una destinazione che hai tiene per la sezione, segue da solo quella destinazione, compresi la scheda della destinazione e il suo interruttore **Disponibile**, a meno che non acceda in modo diverso o sia append-only dove la destinazione non lo è. Una regola che fino ad allora potava la cartella resta come regola propria della sezione in **Regole per sezione**. Quando sta in un'altra cartella di quella destinazione, la pagina di quella destinazione elenca la sezione sotto **Già sotto questa destinazione**, e adottarla le dà le credenziali, la classe di archiviazione e la protezione dall'eliminazione della destinazione. Spostare la cartella propria in una destinazione con **Usa come cartella propria** è la via più semplice, perché la finestra controlla i backup già fatti prima che cambi qualsiasi cosa.

## Destinazioni per elemento {#placement}

La scheda **Calendario e destinazione** nella scheda Impostazioni di un elemento ha una fila di pulsanti: **Locale** e un pulsante per ogni destinazione off-site della sezione. I pulsanti accesi ricevono i backup dell'elemento.

- Con **Locale** acceso, l'elemento viene scritto nel repository mostrato sotto **Salvato su** e copiato su ogni altra destinazione accesa. Spegni una destinazione e non riceve nulla di nuovo da questo elemento. Solo Locale non copia da nessuna parte, il che va bene per dati che hanno già una seconda copia, per esempio una condivisione che vive su un NAS.
- Con **Locale** spento, l'elemento viene scritto direttamente nel repository diretto della prima destinazione accesa e copiato da lì sulle altre accese. La prima volta, una finestra crea quel repository diretto.
- Un pulsante resta acceso, perché un backup ha bisogno di un posto dove andare. Per lasciare un elemento fuori dai backup, usa **Non fare il backup**.

BombVault non sposta mai i backup tra repository. Un elemento che segue la sua sezione la accompagna quando cambia il valore predefinito della sezione: il suo prossimo backup finisce nel nuovo posto, e i backup che ha restano dove sono. Una destinazione scelta sull'elemento stesso resta fissa dal suo primo backup in poi. La scheda **Repository** nella scheda Impostazioni la nomina. Le copie possono cambiare in qualsiasi momento. Una destinazione che non riceve più un elemento mantiene le copie che ha e le riduce alla propria conservazione alla successiva esecuzione off-site della sezione. Dalle destinazioni append-only non si può eliminare.

Il posto che un elemento ha lasciato mantiene i suoi backup. Nella scheda **Backup** dell'elemento e in **Ripristino** compare come **Solo backup precedenti**, un ripristino può ancora leggere da esso, e una potatura lì lascia in pace quei backup.

**Segui l'impostazione predefinita** riporta alla sua sezione un elemento con una scelta propria. Chiede prima conferma, dice dove vanno allora i nuovi backup e nomina cosa ogni destinazione smette di ricevere e cosa caricherebbe la prossima esecuzione.

Sotto la fila la scheda dice dove va l'elemento e cosa c'è davvero: quanti siti lo contengono, quando è stata vista l'ultima volta ogni destinazione e se il 3-2-1 è rispettato. Un sito è il server con i dati originali, ogni destinazione off-site e ogni repository segnato **Fuori sede**. BombVault controlla copie e siti. Non controlla la parte dei "due supporti" del 3-2-1.

### Valori predefiniti per sezione {#placement-defaults}

La scheda **Destinazioni per sezione** nella pagina Destinazioni disegna il valore predefinito di ogni sezione come un flusso 3-2-1. Ogni linea da una sezione a una destinazione è un interruttore, **Locale** compreso, per ogni sezione, anche per la chiavetta flash, i dataset ZFS e l'auto-backup. Un clic su una linea o su un punto la attiva o disattiva subito, e le linee tratteggiate sono spente. Solo spegnere **Locale** di una sezione chiede prima conferma, nella stessa finestra della scheda dei valori predefiniti, perché la sezione salva allora direttamente su una copia. **Annulla** lascia tutto com'era. **Grafico / Interruttori** mostra lo stesso come file di pulsanti. Un interruttore decide dove vanno i nuovi backup. Gli elementi senza una scelta propria, e le cartelle di progetto degli stack Compose, seguono subito, quindi il loro prossimo backup va nei nuovi posti. I backup già salvati restano dove sono e si possono ancora ripristinare da lì. Flash, dataset ZFS e auto-backup salvano nella loro cartella locale e copiano off-site da quel backup. Anche la loro linea verso **Locale** si può spegnere. I loro backup vanno allora direttamente alle loro destinazioni off-site, e un elemento ZFS con una replica e nessuna destinazione invia solo la sua replica.

Con **Locale** spento e solo destinazioni off-site accese, una sezione salva direttamente su di esse, e BombVault sceglie da sé il repository lì. Container, VM e set di cartelle vanno nel repository diretto della prima destinazione accesa, descritto sotto, e da lì vengono copiati sulle altre. La chiavetta flash, i dataset ZFS e l'auto-backup scrivono ogni backup su ogni destinazione accesa, e ciascuna destinazione li conserva con la propria conservazione.

Una sezione con ogni linea spenta dice **Non salvato**. I suoi elementi non ricevono backup e contano come non protetti.

Gli elementi che hanno scelto nella loro pagina destinazioni proprie le mantengono. Il flusso dice quanti sono in una sezione, per esempio **2 elementi differiscono**, e porta a essi: alla scheda Impostazioni dell'elemento quando ce n'è uno, a un elenco con dove salva ciascuno quando sono più d'uno. **Segui l'impostazione predefinita** lì riporta indietro un elemento.

Una nuova destinazione off-site riceve ogni elemento che non è impostato su Locale. La finestra che la aggiunge dice quanti elementi e, dove noto, quanta storia sono.

### Repository diretti {#direct-repositories}

Un repository diretto è dove gli elementi salvano direttamente su una destinazione off-site. BombVault lo mette accanto alla cartella della destinazione con `-direct` aggiunto al nome, oppure, quando la destinazione sta alla radice del suo bucket, del suo remote o di un utente rest-server, in una cartella al suo interno col nome della sezione, come `containers-direct`. Nel flusso della pagina Destinazioni viene creato senza domande. Spegnere Locale per un singolo elemento, così che una destinazione senza repository diretto diventi la sua casa, apre una finestra con la destinazione suggerita, per esempio `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, e una prova di connessione che non crea nulla. **Crea e usa** crea il repository e vi punta l'elemento. Un repository diretto non ha impostazioni proprie. Segue l'interruttore, le credenziali, la classe di archiviazione, i limiti, l'impostazione append-only e la conservazione della sua destinazione, e cambia con essi. La pagina della destinazione lo elenca sotto **Salva direttamente qui**, con ogni sezione che salva direttamente su di esso e quanti elementi lo fanno. Uno su cui non salva più nulla lo dice lì e si può rimuovere, e i backup nel suo posto restano. Quando una nuova chiave per la destinazione non riesce ad aprirlo, il repository diretto mantiene la chiave che ha e il salvataggio lo dice. Un elemento su un repository diretto viene copiato da lì sulle altre destinazioni accese, mai sulla destinazione a cui il repository appartiene. I suoi snapshot portano il tag `bv:direct`, e ogni altro passaggio di conservazione li mantiene, così un repository diretto che ha perso il collegamento con la sua destinazione non invecchia mai con le regole locali. B2 si raggiunge tramite il suo endpoint S3, con l'ID chiave e la chiave dell'applicazione inseriti come chiavi S3. Una chiave limitata alla cartella propria della destinazione non può raggiungere la cartella accanto, quindi limita invece la chiave alla cartella sopra.

### Fuori sede {#off-the-premises}

Una destinazione o un repository può essere segnato **Fuori sede**. Quelli remoti partono segnati. Disattivalo per un rest-server nello stesso edificio. Il segno conta soltanto siti e 3-2-1 nelle schede. Non cambia alcuna copia.

Una replica ZFS è una copia fuori sede, ma non un backup con storia. Conserva solo gli snapshot che imposti, e le vecchie versioni e i file singoli arrivano ancora dai backup. Va da un server ZFS direttamente all'altro, via SSH a un server ZFS o all'endpoint di ricezione di un'istanza associata. Il gruppo di associazione trasporta soltanto la richiesta. Vedi [Replica](zfs-datasets.md#replica).

### Dopo una ricostruzione {#after-a-rebuild}

Le scelte di copia vivono nelle impostazioni di BombVault stesso. Dopo una ricostruzione tramite **Scopri backup** o la Procedura guidata di emergenza senza impostazioni ripristinate non ci sono più, e copiare tutto rimanderebbe su B2 gli elementi che avevi lasciato fuori. Lo stesso vale quando una sezione che non ha mai copiato off-site da questo database trova backup propri su una destinazione, o backup più vecchi nella sua cartella propria, per esempio dopo che una destinazione è stata aggiunta sopra i backup di un'installazione precedente con la stessa `APP_KEY`. Le copie off-site di quella sezione attendono allora la tua decisione.

**Richiede attenzione** nella panoramica e la pagina di ogni destinazione su cui la sezione copia chiedono se i backup sono di questo server. **Adottali per questo server** conferma il valore predefinito della sezione, e le sue copie ripartono dal passaggio successivo. **Prima controlla le regole** porta alla riga della sezione in **Destinazioni per sezione**, dove vedi dove va ogni elemento e puoi impostare di nuovo una regola persa nella pagina dell'elemento. La copia resta in attesa lì finché non premi **Adottali per questo server**, che mostra prima ciò che copia la prossima esecuzione e i nomi nei backup che non hanno un elemento, che puoi lasciare fuori. Il backup più recente di ogni elemento in attesa lo dice nella sua esecuzione, con un link alla decisione, e parte una notifica quando le notifiche sono attive. Importare un file di impostazioni riporta indietro regole e valori predefiniti ma non prende la decisione al posto tuo.

![Richiede attenzione dopo una ricostruzione: i set di cartelle hanno trovato backup di prima di questo database, e le loro copie off-site attendono la decisione.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Richiede attenzione dopo una ricostruzione: i set di cartelle hanno trovato backup di prima di questo database, e le loro copie off-site attendono la decisione.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Protezione dall'eliminazione (append-only) {#delete-protection-append-only}

Attiva la protezione dall'eliminazione per una destinazione off-site così che un ransomware, o un host compromesso, non possa eliminare né riscrivere i tuoi backup. Il lato remoto (un `restic/rest-server` in modalità `--append-only`) la **impone**. BombVault si limita a **verificarla** e non mostra mai il verde sulla sola dichiarazione di configurazione. Una cartella su questo server non può rifiutare l'eliminazione, perché chi controlla BombVault controlla anche la cartella, quindi le destinazioni locali non offrono l'interruttore.

L'interruttore appartiene alla destinazione: una sezione che copia lì lo prende dalla destinazione, quindi attivalo dalla pagina della destinazione sotto **Destinazioni**. Il **Controlla la protezione dall'eliminazione** pianificato gira solo mentre una sezione copia su una destinazione con l'interruttore acceso. Il lato remoto deve essere avviato con `--append-only`. Con l'immagine `restic/rest-server`, su TrueNAS come su qualsiasi altro host Docker, va nella variabile `OPTIONS`.

Alcuni posti non possono proprio rifiutare un'eliminazione, per esempio un'unità cloud come OneDrive. Se tieni lì delle copie di proposito, attiva **Accetta senza protezione dall'eliminazione** nella pagina della destinazione. La panoramica mostra allora la protezione dall'eliminazione mancante di una sezione che copia solo su destinazioni accettate come un rischio accettato, un anello grigio invece di un punto ambra, e il colore della sezione segue gli altri suoi controlli. Non cambia altro: i backup lì si possono ancora eliminare, BombVault li pota ancora secondo le regole di conservazione della destinazione, e l'interruttore sparisce quando la protezione dall'eliminazione è attiva. Una copia che passa a un altro posto perde l'accettazione. Anche attivare la protezione dall'eliminazione la cancella.

La via più rapida a una copia append-only è un BombVault associato con un [server ricevente](#receiving-server): gestisce il rest-server per te, in modalità append-only.

!!! note "Un'eliminazione riuscita sotto `/locks/` è prevista"
    Append-only non significa che nulla possa mai essere rimosso. restic deve prendere e rilasciare i propri lock, quindi `/locks/` resta per scelta scrivibile ed eliminabile. Gli snapshot e i dati che stanno dietro di essi, che è ciò che il ransomware prenderebbe di mira, non si possono rimuovere. Se sondi tu stesso il lato remoto, un'eliminazione che riesce sotto `/locks/` è il comportamento corretto e non un buco nella protezione.

!!! warning "I repository append-only non vengono mai potati da questo server"
    Una destinazione append-only non pota mai da qui i vecchi snapshot. La potatura spetta al lato remoto. Imposta per essa un **allarme sul budget di crescita** così vieni avvisato prima che la dimensione del repository sfugga di mano.

## Prova di manomissione {#tamper-test}

BombVault dimostra periodicamente la garanzia append-only tentando davvero un'eliminazione sul repository off-site, rivolta a un oggetto inesistente. **Prova subito la protezione dall'eliminazione** nella pagina di una destinazione la esegue a richiesta, e **Controlla la protezione dall'eliminazione** in **Impostazioni, Verifica, Controlli automatici** la esegue a scadenza.

Disattivare **Controlla la protezione dall'eliminazione** non nasconde una lacuna. Una destinazione con protezione dall'eliminazione che non è mai stata provata si legge allora "non dimostrata, il suo controllo è disattivato", e una destinazione senza protezione dall'eliminazione si legge "nessuna" in entrambi i casi.

Una copia disattivata non viene provata, e la destinazione lo dice. La prova di connessione gira comunque, perché si limita a leggere, quindi una copia si può controllare prima di attivarla.

- **Rifiutata** significa protetta.
- **Accettata** significa non protetta.
- Un risultato **non conclusivo** (server irraggiungibile, errore di autenticazione) non cambia mai il verdetto salvato.

Un vero passaggio da protetta a non protetta fa scattare un solo avviso.

## Prove DR {#dr-drills}

BombVault offre due livelli di prova che i tuoi backup sono davvero ripristinabili, non solo presenti.

- **Prove di verifica del ripristino (locali).** BombVault esegue periodicamente `restic check --read-data-subset` (limitato, mai un ripristino completo che riempie il disco). La cadenza si trova in **Impostazioni, Verifica, Controlli automatici**, e la scheda **Controlla i backup** nello stesso riquadro mostra il risultato.
- **Ripristino di prova (off-site).** BombVault ripristina una destinazione reale dal repository off-site in una sandbox isolata, la verifica file per file e byte per byte, poi ripulisce. Questo dimostra che puoi recuperare dall'off-site, non solo che il repository risponde.

La scheda **Stato di protezione** nella panoramica riassume questo per sezione in quattro controlli: un backup aggiornato su questo server, una copia off-site, la protezione dall'eliminazione lì e un ripristino di prova da lì. Un clic su una sezione mostra i dettagli, e la scheda diventa verde solo sulla base di fatti verificati. Una destinazione che hai accettato senza protezione dall'eliminazione conta lì come rischio accettato, non come avviso.

## Associare le istanze {#pairing}

Più BombVault lavorano insieme come un unico gruppo: si mostrano a vicenda lo stato di protezione, assumono ruoli l'uno per l'altro e possono custodirsi a vicenda la chiave. Un'istanza entra nel gruppo con dodici parole.

![La pagina di associazione con le sue tre schede dei passaggi.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![La pagina di associazione con le sue tre schede dei passaggi.](assets/screenshots/offsite/pairing-light.png#only-light)

Sulla prima istanza apri **Impostazioni, Associazione** e premi **Genera frase**. Tre schede dei passaggi mostrano come procede. Dodici parole compaiono in una finestra con un pulsante **Copia**. Su ogni altra istanza apri lo stesso posto, premi **Inserisci frase** e incollale o digitale, oppure premi **Incolla** in quella finestra. Una parola che non è nell'elenco viene nominata con la sua posizione mentre digiti, e l'ultima parola porta un checksum, così una parola sbagliata o scambiata viene scoperta prima che si associ qualsiasi cosa. Genera la frase su una sola istanza: due istanze che ne generano una ciascuna formano due gruppi separati. Se per un minuto non compare nessuno, la pagina offre due vie d'uscita: mostrare di nuovo le parole per inserirle di là, oppure inserire le parole dell'altra istanza ed entrare nel suo gruppo in un solo passaggio. L'associazione funziona senza una password di accesso, ma impostane una: senza, chiunque possa aprire l'interfaccia web può leggere le parole e, attraverso il gruppo, chiedere ruoli che consegnano la password restic delle istanze al suo interno. La scheda di associazione lo dice finché non viene impostata una password. Con una password, mostrare di nuovo la frase la richiede. **Lascia il gruppo** fa uscire di nuovo un'istanza. La scheda **Istanze** nella pagina di associazione elenca i membri del gruppo. Il quadro di chi fa cosa per chi si trova nella pagina Istanze.

Chiunque conosca le parole può entrare nel gruppo, quindi trattale come una password.

**Come si raggiungono i membri.** Ogni istanza apprende il proprio indirizzo sulla rete dal tuo browser nel momento in cui accedi, elencato sotto **Indirizzi noti** nella scheda **Questa istanza** della pagina di associazione insieme a quelli che l'istanza ha trovato da sola. Correggilo lì se davanti c'è un reverse proxy o una porta insolita. Sulla stessa rete i membri annunciano quell'indirizzo in multicast e parlano direttamente, e dove il multicast non può attraversare una rete di container, come il bridge predefinito di Docker, un'istanza cerca invece nella propria subnet gli altri con una chiamata firmata a cui può rispondere solo un membro del gruppo, così l'associazione termina comunque in pochi secondi senza un relay. Se non compare nulla, **Non la trovi?** accetta un indirizzo a mano, per un'altra subnet o una porta non standard. Le istanze su reti diverse passano da un relay, scelto in una fila di quattro nella scheda **Relay** della stessa pagina. Ogni forma ha un'immagine:

- **Relay del progetto** (il predefinito): `parleyport.halleluja.design`, il relay che usa anche KnightLoader. Niente da configurare.
- **Relay proprio**: il container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) delle Community Apps di Unraid (anche in **Impostazioni, App**). Inserisci il suo indirizzo su ogni istanza che deve usarlo.
- **Istanza BombVault**: una delle tue istanze già raggiungibile da fuori, con **Funge da relay** attivo. Quell'istanza risponde allora su `/relay/connect` al proprio indirizzo, dietro il reverse proxy e il certificato che ha già, e lascia entrare solo il tuo gruppo. Inserisci il suo indirizzo su ogni istanza che deve usarlo.
- **Nessun relay**: i membri si trovano automaticamente sulla stessa rete, e da nessun'altra parte.

**Cosa vede il relay.** Un pulsante con lo stesso nome accanto alle quattro forme lo mostra per la forma che hai scelto. Ogni chiamata tra membri è sigillata con AES-256-GCM sotto una chiave derivata dalle dodici parole, e quella chiave non lascia mai le tue istanze. Il relay conosce un hash che raggruppa le connessioni, per quale istanza è un messaggio, quanto è grande e quando passa. Una chiamata diretta sulla rete locale è sigillata allo stesso modo e firmata anch'essa, quindi nulla dipende dal certificato autofirmato che un'istanza serve. Anche le chiamate che consegnano qualcosa (risposte ai ruoli, una password restic, accessi dei ricevitori, depositi della chiave) sono firmate con una chiave del membro che le invia, fissata alla prima vista. Vedi [Modello di sicurezza](configuration.md#security-model).

**Cosa viaggia nel gruppo.** Lo stato di protezione nella pagina Istanze, una richiesta di controllare subito una sezione, le richieste di ruolo e le loro risposte, le offerte di archiviazione e ciò di cui un ricevitore o un prelevatore ha bisogno una volta consentito il suo ruolo: le destinazioni del repository dell'altra istanza e la sua password restic. I dati di backup mai. Vanno comunque direttamente ai backend restic. Nemmeno la APP_KEY: la password restic apre i repository di quell'istanza e nient'altro, non i suoi segreti memorizzati, le sessioni o i codici di sblocco.

**Elementi precedenti all'associazione.** Le istanze aggiunte con un token di flotta, e i repository ricevuti e le sorgenti di prelievo configurati con la APP_KEY dell'altra istanza, restano dopo l'aggiornamento e sono segnati **Associa di nuovo**. Continuano a funzionare: al primo avvio BombVault sostituisce ogni APP_KEY salvata con la password restic che ne deriva. Associa entrambe le istanze, poi modifica l'elemento e scegli la sua istanza. Fino ad allora tali elementi stanno nella scheda **Senza istanza** della pagina Istanze.

L'unico posto che accetta ancora una APP_KEY a mano è [Da una vecchia cartella di backup](#restore-from-another-bombvault-repo), per il caso in cui l'altra istanza non c'è più e non può rispondere in un gruppo.

## Istanze e ruoli {#roles}

La pagina **Istanze** (un riquadro delle impostazioni che si può fissare alla barra laterale) mostra il gruppo come una rete di chi fa cosa per chi, una scheda per questa istanza e una scheda per ogni altro membro, i telefoni Android e i server ZFS. **Associazione** in basso a destra porta alla pagina di associazione. Quali parti sono attive si imposta in **Impostazioni, Istanze, Più server**: **Istanze**, **Ricevitore** e **Prelievo**.

![La pagina Istanze: il gruppo come una rete e una scheda per istanza con i suoi riquadri dei ruoli, uno di essi Inattiva.](assets/screenshots/offsite/instances-dark.png#only-dark)
![La pagina Istanze: il gruppo come una rete e una scheda per istanza con i suoi riquadri dei ruoli, uno di essi Inattiva.](assets/screenshots/offsite/instances-light.png#only-light)

Ogni scheda mostra lo stato di protezione del membro e i suoi ruoli come riquadri. Un ruolo è qualcosa che un'istanza fa per un'altra:

- **Ricevitore** tiene le copie off-site di questo server in un rest-server in modalità append-only. Questo server invia dopo ogni backup e non può eliminare nulla lì, anche se qualcuno ne prende il controllo.
- **Prelevatore** preleva i backup di questo server con una pianificazione propria. Questo server non deve essere acceso per questo. L'istanza che preleva deve avere **Prelievo** attivo prima di poter consentire la richiesta.
- **Server ZFS** accoglie le repliche ZFS di questo server. Ogni elemento ZFS lo sceglie sotto **Replica** nelle sue impostazioni.
- **Custode della chiave** custodisce una copia sigillata del kit di ripristino di questo server. Vedi [Custode della chiave](#key-keeper).

Attivare un ruolo invia una richiesta, e nulla parte finché qualcuno sull'altra istanza non preme **Consenti**. La richiesta compare sulla scheda di quell'istanza come badge, e i suoi **Dettagli** la elencano con **Consenti** e **Rifiuta**. Un ruolo consentito si può revocare lì in qualsiasi momento. Una linea tratteggiata nella rete è un ruolo ancora in attesa. Un'istanza con BombVault 9.x non può rispondere a una richiesta, quindi un ruolo è attivo lì non appena lo attivi, e una richiesta da un'istanza 9.x aspetta **Consenti** qui.

**Dettagli** su una scheda apre il resto: i repository ricevuti e le sorgenti di prelievo di quel membro, le offerte di archiviazione, il monitoraggio, **Rimuovi** e **Dimentica la chiave**.

Un membro può offrire la propria archiviazione off-site a un altro membro attraverso il gruppo. L'altro amministratore vede l'offerta nei **Dettagli** del membro e la accetta o la rifiuta. Accettare crea delle credenziali salvate e una normale destinazione, dopo gli stessi controlli di una destinazione che digiti tu. La destinazione arriva con la protezione dall'eliminazione attiva, perché la ricetta mostrata dall'offerta è un rest-server append-only, quindi **Per quanto tempo conservare** lì non si applica. Ogni offerta ha un login e una cartella propri, quindi offrire di nuovo una sezione, allo stesso membro o a un altro, lascia in pace le copie del primo partner: aggiungi la nuova riga a `.htpasswd` e tieni il server in funzione. In questo modo viaggiano solo i dettagli di connessione, mai i dati di backup. **Controlla ora** chiede a un membro di verificare il repository di una sezione. Nulla nella pagina può avviare un backup, un ripristino o eliminare qualcosa su un'altra macchina.

Le istanze spente e i telefoni con l'app chiusa mantengono la loro scheda come **Inattiva**, in grigio, con l'indicazione di quando sono stati visti l'ultima volta, e restano attenuati nella rete finché non li **Rimuovi**. Subito dopo l'avvio di questo server, una scheda simile dice **Controllo in corso** finché il gruppo non è tornato. **Rimuovi** funziona su una scheda simile qualunque parte sia attiva, e termina anche le richieste aperte del membro, i suoi slot di ricezione ZFS inattivi e le sue richieste di chiave in attesa. Anche la scheda **Istanze** nella pagina di associazione elenca questi membri come **Inattiva**.

## Ricevitore e prelevatore {#receiver}

Sulla macchina che **riceve** copie append-only da un altro BombVault, i repository ricevuti ti danno un monitoraggio indipendente e di sola lettura sull'hardware ricevente, così un errore silenzioso all'altro capo non passa inosservato. Attiva **Ricevitore** in **Impostazioni, Istanze**, poi apri i **Dettagli** dell'istanza che invia nella pagina Istanze e aggiungi il repository ricevuto (di sola lettura, aperto con la password restic dell'istanza che invia, che riceve attraverso il [gruppo di associazione](#pairing)). Ottieni:

- **Un inventario per sorgente**, così vedi con precisione quali container, VM e set di cartelle sono arrivati. I backup che l'istanza che invia ha fatto prima di avere un [host proprio](#backup-host) contano per la stessa sorgente.
- **Ultima ricezione** per sorgente, così sai quanto è fresca ciascuna.
- Un **`restic check` indipendente** eseguito sull'hardware ricevente, così l'integrità è verificata dove i dati stanno davvero, non solo sul mittente.
- **Un interruttore dell'uomo morto:** un avviso quando una sorgente smette di inviare entro una finestra che imposti. Controlla una volta al giorno, alle 09:15. Ricevi un avviso anche quando il repository non si può leggere affatto, qualunque cosa dica la pianificazione dei controlli, e quando non è arrivato nulla un'intera finestra dopo che l'hai aggiunto. Un repository che aggiungi prima della prima copia del mittente va bene fino ad allora.
- **Avvisi di integrità:** un avviso quando un controllo sul lato ricevente fallisce. Un controllo che non è riuscito ad aprire il repository non dice nulla sull'integrità, quindi non lascia alcun verdetto e riprova il giorno dopo alla stessa ora.

Un repository su un altro server ha bisogno del login di quel server: salvalo come [credenziale](#sign-ins) e sceglilo sotto **Credenziali** nella finestra.

Il ricevitore è rigorosamente di sola lettura. Non scrive mai nel repository ricevuto, quindi non può mai rompere la garanzia append-only su cui conta il mittente.

Un prelevatore funziona al contrario: con **Prelievo** attivo, un'istanza legge i backup di un altro membro con una pianificazione propria, in un repository suo per ogni sorgente, così non contano mai come backup propri di questo server. Le sue sorgenti stanno anche nei dettagli di quel membro: la destinazione del repository, che tipo di backup contiene e ogni quanto prelevare. La password restic arriva attraverso il gruppo, mai la APP_KEY, e il lato remoto non ha altro da configurare e non deve essere in funzione. La sorgente viene solo letta, e ogni lato conserva le proprie credenziali. Un prelievo parte nel giorno e all'ora che dice la sua pianificazione, e anche una nuova sorgente aspetta quel momento. **Preleva ora** non aspetta. Un prelievo saltato perché questa macchina era spenta parte appena torna. Uno fallito riprova il giorno dopo alla stessa ora, anche con una pianificazione settimanale, e un prelievo fallito compare nel registro attività e invia un solo avviso finché un prelievo non funziona di nuovo. Una sorgente `rclone:` viene rifiutata, perché rclone la raggiungerebbe con i remote di questa istanza.

Una copia prelevata cresce soltanto. BombVault vi aggiunge dati e non la pota né la controlla, quindi ciò che la sorgente ha dimenticato da tempo resta qui finché non elimini la cartella, e la pagina Destinazioni non la conta. Si trova in `pulled/<source id>` dentro la cartella propria della sezione, e quando quella cartella si sposta, la copia resta dov'è e i prelievi successivi continuano lì invece di copiare di nuovo tutto. Se la elimini, il prelievo successivo ricomincia nella cartella propria che la sezione ha a quel punto.

### Server ricevente {#receiving-server}

La macchina ricevente può anche gestire il rest-server su cui copiano gli altri. Si configura la prima volta che consenti una richiesta **Ricevitore** che lo chiede: la finestra dice che il server ha bisogno di un server ricevente e chiede una cartella su una condivisione, con **Nuova cartella** per crearne una, e una porta (8000 a meno che la usi un altro container). BombVault allora:

1. rifiuta se esiste già un container chiamato `rest-server` o se un altro container tiene la porta,
2. scarica `restic/rest-server` e lo avvia tramite il socket Docker in modalità append-only con repository privati e un file di accesso nella cartella,
3. scrive il suo template Unraid sulla chiavetta flash, così il container resta modificabile nella scheda Docker, oppure offre il template come download quando la chiavetta flash non è raggiungibile,
4. esegue la prova di manomissione su di esso e mostra se rifiuta le cancellazioni.

Le istanze del tuo gruppo trovano poi il server tra i fornitori sotto **Dal tuo gruppo**, col nome della macchina ricevente, e il ruolo **Ricevitore** lo propone come scelta sicura. Ogni istanza riceve un accesso proprio la prima volta che usa il server, e scrive solo nella propria cartella lì. Con **Ricevitore** attivo in **Impostazioni, Istanze**, **Server ricevente** sulla scheda di questa istanza riapre il server. La sua finestra elenca gli accessi sotto **Partner con accesso**, e **Revoca accesso** ne toglie uno. Ciò che quell'istanza ha già copiato resta nella cartella. La configurazione crea anche un **Accesso fuori dal gruppo**, la cui password la finestra mostra una volta sola.

Un'istanza che raggiunge la macchina ricevente solo tramite il relay non può usare il server, perché il relay non trasporta backup. Aggiungi prima l'indirizzo della macchina ricevente nella pagina di associazione. Quando BombVault gira su un indirizzo IP proprio (su br0, per esempio), compila **Indirizzo per i partner**, perché il server ascolta sull'indirizzo dell'host.

## Esempio: associare due server Unraid {#worked-example-two-unraid-boxes-end-to-end}

Questa è una configurazione completa delle parti sopra, perché sono più facili da assemblare dopo averle viste assemblate.

Due macchine: **TOWER** gestisce i container e invia i backup. **VAULT** li riceve e impone l'append-only. Sostituisci con i tuoi nomi, indirizzi e percorsi delle condivisioni.

**1. Associa le due macchine.** Su TOWER apri **Impostazioni, Associazione**, premi **Genera frase** e inserisci le dodici parole su VAULT in **Inserisci frase**. Imposta prima una password di accesso su entrambe.

**2. Su TOWER, chiedi a VAULT di essere il ricevitore.** Nella pagina Istanze attiva **Ricevitore** sulla scheda di VAULT, scegli le sezioni e il **server ricevente di VAULT** come posto, e invia la richiesta.

**3. Su VAULT, consentila.** La pagina Istanze di VAULT mostra la richiesta sulla scheda di TOWER. Premi **Consenti**. La prima volta, VAULT chiede una cartella per il suo server ricevente, come `user/appdata/rest-server`, avvia il rest-server in modalità append-only ed esegue la prova di manomissione su di esso.

**4. Di nuovo su TOWER.** Da quel momento TOWER copia su VAULT dopo ogni backup, e la nuova destinazione compare nella pagina Destinazioni di TOWER con la sua protezione dall'eliminazione.

Cosa risponde la prova di manomissione:

| Risultato | Cosa è successo |
| --- | --- |
| **protetta** | VAULT ha rifiutato l'eliminazione. È l'unico stato superato. |
| **non protetta** | VAULT ha accettato un'eliminazione. `--append-only` manca o è stato rimosso. |
| **non conclusivo** | Nessuno dei due. Di solito l'URL non è quello che restic stesso usa, oppure le credenziali sono cambiate. Non viene registrato nulla e non scatta alcun avviso. |

**5. Guarda cosa arriva, se vuoi.** Attiva **Ricevitore** in **Impostazioni, Istanze** su VAULT, apri i **Dettagli** di TOWER e aggiungi il repository ricevuto: fai clic su una delle posizioni che la finestra elenca. VAULT riceve la password restic di TOWER attraverso il gruppo, e poiché la posizione si trova sul server ricevente di VAULT stesso, legge la cartella direttamente. Nessuno digita una chiave o un login.

**6. Rendilo reciproco.** Ripeti i passaggi nell'altra direzione: VAULT chiede a TOWER di essere il suo ricevitore. Ogni macchina impone allora l'append-only per l'altra, e nessuna delle due può eliminare i backup dell'altra.

Funziona anche un rest-server che gestisci a mano: aggiungilo con **Aggiungi destinazione** come rest-server, con l'utente e la password del suo `.htpasswd`, e attiva la protezione dall'eliminazione. Tieni `--append-only` nelle sue opzioni. Senza, il lato remoto è di nuovo una condivisione ordinaria.

!!! warning "Un percorso locale è un percorso dentro il container"
    Dove BombVault chiede una cartella su questo server, inseriscila relativa al mount dell'host, come `user/appdata/rest-server/bombvault-containers`, non `/mnt/user/appdata/...`. BombVault gira in un container, dove `/mnt` dell'host è montato altrove. Un percorso assoluto dell'host al suo interno non esiste. Se ne incolli uno, BombVault ti dice invece il percorso relativo da usare.

## Procedura guidata di emergenza {#emergency}

La **Procedura guidata di emergenza** riporta in vita un server che non c'è più o è stato reinstallato. Parte dalla pagina **Ripristino** sotto **Emergenza**, e da **Ripristina da un backup** nella pagina di benvenuto di una nuova installazione. Procede in quattro passaggi: **Accesso**, **Impostazioni**, **Selezione** e **Ripristino**.

1. **Accesso.** Mostra a BombVault la chiave dei vecchi backup, in uno di tre modi:
    - **Carica il kit di ripristino**: il file che hai scaricato durante la configurazione. Contiene la chiave e ogni destinazione, e BombVault compila il resto da esso. Un kit sigillato con age chiede l'identità per cui è stato sigillato. Viene usato una volta e non salvato.
    - **Tramite il gruppo**: un'istanza del tuo gruppo custodisce la chiave come deposito. Ti servono le dodici parole del gruppo, il tuo codice di sblocco e qualcuno su quell'istanza. Vedi [Custode della chiave](#key-keeper).
    - **Digita la chiave**: la `APP_KEY` del vecchio server, e dove si trovano i suoi backup.

    BombVault prova poi la chiave su ogni destinazione e dice per ciascuna se è leggibile, non trovata, bloccata con un'altra chiave, non raggiungibile o priva ancora delle credenziali. Non è stato ancora salvato nulla, e la chiave resta in memoria per un tempo limitato. **Adotta la chiave** fa scrivere a questo server i suoi backup con quella chiave da ora in poi. **Annulla l'adozione** torna alla sua chiave finché non è stato eseguito il primo backup. Un server che ha già backup propri rifiuta l'adozione e rimanda a [Da una vecchia cartella di backup](#restore-from-another-bombvault-repo). **Salva le credenziali** memorizza le credenziali che il kit porta per le destinazioni remote, e chiede conferma prima di sostituire credenziali che questo server ha già.
2. **Impostazioni.** **Ripristina le impostazioni** ripristina le impostazioni di BombVault stesso dall'auto-backup più recente: destinazioni, credenziali, pianificazioni e regole. BombVault si riavvia per applicarle, tramite il socket Docker, così il database delle impostazioni in uso non viene mai sovrascritto mentre è aperto, e la procedura guidata prosegue da sola dopo. L'accesso di questo server resta: continui ad accedere con la password che usi ora. I segreti che il vecchio server aveva salvato e che non si possono adottare sono elencati, così puoi inserirli di nuovo. Il ripristino termina ogni sessione e rimuove le passkey, quindi impostale di nuovo dopo.
3. **Selezione.** BombVault scorre i tuoi backup ed elenca i container, le VM, i set di cartelle e i dataset ZFS che trova. Lo snapshot più recente è preselezionato ovunque. Ciò che questo server ha ancora parte disattivato, perché un ripristino lo sostituirebbe. I set di cartelle non sanno più da dove venivano dopo una ricostruzione, quindi vanno in una cartella che scegli.
4. **Ripristino.** Tutto ciò che è selezionato torna in un colpo solo, lasciato fermo se vuoi avviare tu ogni container e VM, il che è più sicuro dopo una perdita totale. Poi scarica un nuovo kit di ripristino: quello vecchio appartiene all'installazione precedente.

![La Procedura guidata di emergenza al primo passaggio, con i tre modi per entrare.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![La Procedura guidata di emergenza al primo passaggio, con i tre modi per entrare.](assets/screenshots/offsite/emergency-light.png#only-light)

### Esempio: un nuovo server dal kit di ripristino {#example-emergency}

Il server TOWER è morto con i suoi dischi. Teneva 23 container e 2 VM, e li copiava insieme al suo auto-backup su Backblaze B2. Hai `bombvault-recovery-kit.md` dal tuo gestore di password e una macchina nuova con Unraid. La sua vecchia `APP_KEY` è persa, e non ti serve.

1. **Installa BombVault** sulla macchina nuova e dagli una nuova `APP_KEY`, come in qualsiasi nuova installazione. Se vuoi riavere le VM, configura il collegamento delle VM in **Impostazioni, Integrazioni, SSH dell'host**, perché un ripristino di VM ne ha bisogno.
2. **Avvia la procedura guidata.** In **Ti diamo il benvenuto in BombVault** scegli **Ripristina da un backup**. Nella pagina Ripristino scegli **Avvia la procedura guidata di emergenza** sotto **Emergenza**.
3. **Accesso.** Scegli **Carica il kit di ripristino** e trascina il file. BombVault prova la sua chiave su ogni destinazione e dice, per esempio, **Kit di ripristino letto. La sua chiave apre 0 destinazioni su 2.** La cartella locale dice **non trovata**, dato che il disco è morto, e Backblaze B2 dice **credenziali mancanti**.
4. **Salva le credenziali.** Il kit porta le credenziali di ogni destinazione. Premi **Salva le credenziali**, e le destinazioni vengono riprovate, così Backblaze B2 dice ora **leggibile**. Un kit scritto prima della 10.5 contiene solo le credenziali predefinite. Dove una destinazione dice ancora **credenziali mancanti**, inserisci le sue credenziali in **Inserisci una credenziale mancante** e premi **Riprova le destinazioni**.
5. **Adotta la chiave.** Questo server scrive ora i suoi backup con la vecchia chiave, così i nuovi backup si adattano ai vecchi. Mantiene la propria `APP_KEY` nel template. **Annulla l'adozione** funziona finché non è stato eseguito il primo backup.
6. **Impostazioni.** Il passaggio si apre sul posto in cui la chiave ha trovato l'auto-backup, qui B2, con il suo indirizzo già compilato. Scegli **Ripristina le impostazioni**, che legge da lì l'auto-backup più recente. BombVault si riavvia per applicarlo, e la procedura guidata prosegue da sola. Le tue destinazioni, pianificazioni e regole sono tornate, e continui ad accedere con la password di questo server. Inserisci di nuovo i segreti che la procedura guidata elenca, come le impostazioni delle notifiche.
7. **Selezione.** **Dove si trovano i backup?** si apre sulle copie off-site quando non resta nulla in locale. BombVault legge ogni destinazione, così una non raggiungibile non nasconde le altre, elenca i 23 container e le 2 VM e ripristina ciascuno dalla destinazione che contiene il suo snapshot più recente.
8. **Ripristino.** Attiva **Poi lascia fermo**, che è più sicuro dopo una perdita totale, e avvia. **Cosa succede** dice cosa torna. La riga del risultato dice, per esempio, **25 ripristinati, 0 falliti. Avviali in Backup quando serve.**
9. **Concludi.** Avvia prima i tuoi database, poi le app che li usano. Apri **Impostazioni, Ripristino** e scarica un nuovo kit di ripristino, perché quello vecchio appartiene all'installazione precedente. Controlla **Richiede attenzione** nella panoramica, ed esegui **Esegui il backup di tutto ora** quando tutto funziona.

Se manca anche l'auto-backup, il passaggio si apre su **Salta: non ho un auto-backup** e lo dice. Le destinazioni arrivano allora dal solo kit, configuri di nuovo pianificazioni e regole, e le copie off-site restano in attesa finché non adotti i loro backup per questo server, vedi [Dopo una ricostruzione](#after-a-rebuild). **Selezione** elenca comunque ciò che contengono le copie off-site. I set di cartelle finiscono in una cartella a tua scelta. Container e VM tornano come semplici file in una cartella sotto **Possono tornare solo i file**, perché le loro definizioni vivono accanto al repository locale e nell'auto-backup, e una copia off-site non contiene né l'uno né l'altro.

!!! tip "Migrazione pianificata o disastro"
    La Procedura guidata di emergenza ripristina da un backup le impostazioni di BombVault stesso. Per un trasloco *pianificato* su una macchina nuova puoi invece portare la tua configurazione direttamente con la scheda **Esporta / importa impostazioni** (un file JSON portabile). Vedi [Configurazione](configuration.md#portable-settings-export-and-import).

### Da una vecchia cartella di backup {#restore-from-another-bombvault-repo}

**Da una vecchia cartella di backup** sotto **Altre sorgenti** nella pagina Ripristino apre il repository di un'istanza BombVault *diversa* (una condivisione montata sotto `/mnt`, o un indirizzo remoto) con il kit di ripristino di quell'istanza o la sua `APP_KEY` digitata, in una sessione monouso di sola lettura. Sfoglia i container, le VM, i set di cartelle e i dataset ZFS salvati lì, scegli uno snapshot e ripristinalo, e l'oggetto ripristinato diventa un normale container, VM o set di cartelle locale. Nulla viene mai scritto nell'altro repository, e le tue impostazioni di backup restano intatte (la sessione vive in memoria e scade da sola). Spostare un container dal server A al server B non significa ripuntare le tue impostazioni e poi riportarle indietro. Questa finestra è usa e getta: apre una sessione, ripristina ciò che scegli e dimentica l'altra istanza. Se vuoi invece un accordo stabile, in cui questa macchina preleva gli snapshot di un'altra istanza nel proprio repository con una pianificazione, quello è il ruolo **Prelevatore** nella pagina Istanze.

### Cosa ha prelevato questo server {#restore-what-was-fetched}

Ogni sorgente da cui questo server ha prelevato ha la sua riga **Recuperato da** sotto **Altre sorgenti** nella pagina Ripristino. Una copia prelevata è bloccata con la chiave di questo server, non con quella della sorgente, quindi non viene chiesto nulla: la chiave della sorgente non la apre. I set di cartelle e i dataset ZFS finiscono in una cartella a tua scelta. Container e VM tornano come semplici file in una cartella, perché un prelievo porta gli snapshot e non le definizioni. Per ricrearli, usa il kit di ripristino e l'auto-backup della sorgente nella Procedura guidata di emergenza.

Un container la cui rete non esiste su questo server, come una rete `br0` di Unraid su un normale host Docker, mostra un selettore di rete sotto la sua riga. BombVault lo crea sulla rete che scegli, insieme alle sue altre reti. L'IP fisso e l'indirizzo MAC appartenevano alla vecchia rete e vengono scartati, così la nuova rete li assegna.

## Custode della chiave {#key-keeper}

Un custode della chiave è un'istanza del tuo gruppo che custodisce una copia sigillata del kit di ripristino di questo server. Non può aprire la copia da solo. Se questo server dovesse andare perso, una nuova installazione che entra nel tuo gruppo chiede la copia al custode e la apre con il tuo codice di sblocco.

![La finestra del codice di sblocco: venti caratteri in quattro gruppi, mostrati una sola volta.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![La finestra del codice di sblocco: venti caratteri in quattro gruppi, mostrati una sola volta.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Nella pagina Istanze attiva **Custode della chiave** sulla scheda dell'istanza che deve custodirla, e scegli **Crea deposito**. Per creare un deposito serve una password di accesso.
2. Questo server mostra il **Codice di sblocco** una sola volta: venti lettere e cifre in quattro gruppi di cinque. Scrivilo e conferma con **Ho annotato il codice**. Questo server non salva il codice, solo ciò che gli serve per sigillare un deposito per esso.
3. Il custode ora tiene il deposito, e questo server lo rinnova quando il kit cambia. **Deposito chiave** sulla scheda di questa istanza elenca il **Deposito di questo server** con ogni custode e il suo stato, e i **Depositi custoditi da questo server** per gli altri. **Crea un nuovo codice** sostituisce il codice. Ogni custode passa al nuovo quando riceve il nuovo deposito.

Quel giorno ti servono tre cose: le dodici parole del tuo gruppo, il codice di sblocco e l'accesso all'interfaccia del custode, dove qualcuno deve premere **Consenti**. Nella Procedura guidata di emergenza scegli **Tramite il gruppo**, scegli il deposito, chiedilo e digita il codice di sblocco. Il codice viene usato sul nuovo server e non viene inviato al custode. Sul custode la richiesta compare sotto **Deposito chiave** con chi la fa, se quell'istanza è mai stata vista lì prima e quante volte ha chiesto. **Consenti** consegna la copia sigillata, e consentire richiede una password di accesso. La nuova installazione ha 30 minuti per ritirarla dopo il Consenti, una richiesta a cui nessuno risponde scade dopo un giorno, e un'istanza che chiede troppo spesso deve aspettare un'ora.

Senza il codice nessuno può aprire la copia, nemmeno il custode. Se il codice va perso, creane uno nuovo finché questo server gira ancora. Se il server non c'è già più, il kit di ripristino e la APP_KEY digitata funzionano comunque. Il custode può eliminare un deposito in qualsiasi momento, e il proprietario deve allora chiederlo di nuovo.

## Kit di ripristino {#recovery-kit}

È il pezzo che rende possibile il disaster recovery anche quando non c'è alcun BombVault in funzione.

**Scarica il kit di ripristino** nella scheda **Per le emergenze** in **Impostazioni, Ripristino** ti dà la **chiave**, la **password restic derivata**, ogni **destinazione** con il repository di ogni sezione, le credenziali di cui una nuova installazione ha bisogno per raggiungerle e i **comandi restic esatti**, così puoi ripristinare direttamente con la CLI di restic su qualsiasi macchina. Richiede una password di accesso. Il kit stampa la chiave anche con la crittografia disattivata, perché le definizioni di container e VM sono sempre sigillate con essa. Dopo un'adozione stampa la chiave dei backup adottata e la APP_KEY di questo server, di cui una nuova installazione ha bisogno solo per riportare indietro le impostazioni. Una scheda **Salva il tuo kit di ripristino** nella panoramica te lo ricorda finché non l'hai conservato.

!!! danger "Conserva il kit di ripristino fuori dal server"
    Il kit contiene il segreto che decifra i tuoi backup. Tienilo in un posto sicuro e separato dal server (un gestore di password, una copia stampata in una cassaforte). Se perdi sia BombVault sia `APP_KEY` senza kit di ripristino né deposito della chiave, i tuoi backup cifrati non si possono recuperare.

!!! warning "Lo snapshot più recente non è sempre quello da ripristinare"
    Da restic 0.17, `restic snapshots` mostra la dimensione di ogni snapshot. Dopo una perdita di dati lo snapshot più recente può essere quello svuotato, quindi non ripristinare uno snapshot molto più piccolo di quelli prima. Dopo un ransomware può essere quello cifrato alla dimensione consueta. Se BombVault gira ancora, controlla prima la sua pagina **Anomalie**: nomina l'ultimo backup buono, e la finestra di ripristino lo preseleziona. Un ripristino non ha bisogno di alcun dato sulle anomalie di BombVault, e la pausa della conservazione fa soltanto conservare più snapshot.

### L'host dei backup di un server {#backup-host}

Ogni server scrive i suoi backup sotto un host restic proprio: `bv-` e dodici caratteri ricavati dalla chiave dei backup, come `bv-3f9a0c41d2e7`. I backup di quando un server non ne aveva ancora uno portano l'host `bombvault`. Il kit indica l'host di questo server, e i suoi comandi li passano entrambi con `--host`, così `latest` non sceglie mai un backup che un altro server ha scritto nello stesso repository. Un server che adotta i backup di un altro prosegue sotto l'host di quel server, perché l'host deriva dalla chiave.

### Sigillare il kit {#sealing-the-kit}

Se hai attivato **Cifra le esportazioni con age** in **Impostazioni, Sistema**, anche il kit viene sigillato con essa e si scarica come `bombvault-recovery-kit.md.age`. È in formato ASCII armor invece che binario, quindi è ancora testo semplice: incollarlo in un gestore di password o stamparlo funziona esattamente come prima, il contenuto è illeggibile senza la tua chiave.

!!! warning "Non conservare la chiave age dentro il kit"
    Per aprire un kit sigillato ti serve la tua chiave **privata** age. Tienila in un posto che non dipenda dal kit stesso, altrimenti avrai due cose da recuperare invece di una. Sigillare vale la pena quando il kit è conservato in un posto che non controlli del tutto (un gestore di password condiviso, note nel cloud, una stampa in ufficio). Un kit nella tua cassaforte è già protetto dalla cassaforte.

    Con la cifratura attiva e nessun destinatario utilizzabile configurato, il download viene rifiutato del tutto. BombVault non ripiega mai sul consegnare la chiave principale in chiaro.

### Se non hai il kit sotto mano {#if-you-do-not-have-the-kit-to-hand}

La password non è salvata da nessuna parte, viene **calcolata** dalla chiave dei backup, quindi puoi riprodurla da solo con nient'altro che la chiave e una shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

È HMAC-SHA256 sulla stringa fissa `bombvault:restic-repo`, con chiave i byte grezzi della `APP_KEY` esadecimale, stampato come 64 caratteri esadecimali minuscoli. Lo stesso valore è nel kit, elencato come password restic derivata. È per il giorno in cui il kit è dove non sei tu. Su un server che ha adottato i backup di un altro, usa la chiave dei backup adottata, che è la vecchia APP_KEY di quel server.

L'[host](#backup-host) deriva dalla stessa chiave e dalla stringa `bombvault:host`: `bv-` e i primi dodici di quei caratteri.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Per un repository ricevuto, usa la chiave dell'istanza CHE INVIA"
    Un repository arrivato qui tramite una copia off-site è stato creato dalla macchina che lo ha inviato, con **la sua** chiave. Derivare dalla chiave della macchina ricevente produce una password che restic rifiuterà, e che si legge esattamente come un repository corrotto senza esserlo. È il motivo abituale per cui `restic check` su un repository ricevuto chiede la password ancora e ancora.

Poiché le definizioni di ripristino vivono **dentro** ogni repository (`<repo>/def`, `<repo>/vm-def`), una cartella di repository copiata è completamente autosufficiente, quindi il kit più il repository è tutto ciò che serve per un ripristino bare-metal. Una copia off-site e una copia prelevata contengono gli snapshot senza le definizioni: restituiscono file, e un container o una VM si ricrea dall'auto-backup.

## Riavere un dump del database {#database-dumps}

Un dump del database è un punto di ripristino a sé nel repository dei container, con il tag `dbdump:<container>`, che contiene l'unico file `/dbdump/<container>.sql`. BombVault li elenca, li scarica e li importa nella scheda **Backup** del container. Sotto trovi gli stessi passaggi con il solo restic, per il giorno in cui BombVault non c'è.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` è l'[host](#backup-host) del server indicato nel kit di ripristino.

I tag `dbversion:` e `dbname:` su ogni dump dicono da quale versione del server proviene e quali database contiene. Un file completo termina con `-- PostgreSQL database cluster dump complete` o `-- Dump completed`.

Importalo in un container della stessa versione o di una più recente (PostgreSQL), o della stessa versione maggiore (MySQL e MariaDB), avviato una volta con una cartella dati vuota così che si inizializzi. L'host non ha bisogno di alcun client di database, ce l'ha il container:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Per un singolo database da un dump completo, MySQL e MariaDB accettano `--one-database <name>` nel comando del client. Un dump PostgreSQL ha una sezione per database, ciascuna che inizia con una riga `\connect <name>`: copia quella sezione in un file a parte e importala con `-d <name>` dopo aver creato il database.

!!! warning "Un dump come root porta con sé gli utenti del server"
    Un dump completo di MySQL o MariaDB fatto come root contiene il database di sistema `mysql`, quindi importarlo sostituisce gli account del nuovo server, compresa la password di root, con quelli del dump. Su PostgreSQL, `role ... already exists` per l'utente creato dal container è previsto e innocuo.
