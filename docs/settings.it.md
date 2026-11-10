# Impostazioni

Le **Impostazioni** sono quattordici riquadri. Ogni riquadro è una pagina di schede, e ogni modifica viene scritta mentre la fai, con un breve lampo sul controllo, oppure uno scossone quando il server la rifiuta. Alcune finestre tengono un pulsante **Salva** perché salvarle a metà sarebbe pericoloso, come le credenziali o la password di accesso. Le spiegazioni stanno nei fumetti (i) accanto a ogni controllo.

Premi `/` o Ctrl+K ovunque nelle impostazioni per cercarle tutte, anche con i nomi che le pagine avevano nelle versioni precedenti. L'ordine dei riquadri si può cambiare in **Aspetto**. Dove una colonna di riquadri accanto alla pagina la stringerebbe troppo, come in una finestra larga circa 1200 pixel o su un telefono, i riquadri diventano una striscia sopra la pagina che scorre di lato.

![Le impostazioni con il riquadro Generale aperto.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Le impostazioni con il riquadro Generale aperto.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Questa pagina elenca ogni riquadro e le sue schede, e rimanda alla pagina che spiega ciascuna a fondo.

## Generale {#general}

- **Sezioni** attiva o disattiva ogni sezione: **Container**, **VM**, **Flash**, **Cartelle**, **Dataset ZFS** e **Auto-backup**. Una sezione disattivata non viene salvata da nessuna pianificazione, compreso il piano proprio di un elemento, e i suoi elementi non contano. **Esegui backup ora** funziona comunque. **Flash** compare solo su Unraid.
- **Container** contiene ciò che vale per ogni container, come i **Dump automatici dei database**. Tutto ciò che riguarda un singolo container sta nella sua pagina. Vedi [Dump dei database](backups.md#database-dumps).

## Aspetto {#look}

- **Aspetto**: **Lingua** (42 lingue), **Tema** (**Scuro**, **Chiaro** o **Sistema**) con **OLED** sotto, che dà un nero vero agli schermi OLED e AMOLED mentre si vede il tema scuro, **Angoli**, **Animazione** (**Disattivato**, **Sobrio**, **Selvaggio** o **Scatenato**), **Notifiche silenziose**, che nasconde gli avvisi di successo come le conferme di salvataggio, e **Ordine delle pagine delle impostazioni**, dove tieni premuto un riquadro e lo trascini al suo posto, oppure usi Alt e i tasti freccia.
- **Etichette**: quanto di un controllo viene mostrato, separatamente per **Pulsanti**, **Barra laterale**, **Schede** e **Barra inferiore**: **Testo**, **Testo e simbolo**, **Simbolo**, oppure **Reattivo**, che mostra solo il simbolo e fa scorrere dentro le parole quando ci punti sopra. **Barra laterale** riguarda solo il layout desktop e **Barra inferiore** solo il layout per telefono.
- **Colori**: un **Colore di accento**, oppure la **Modalità arcobaleno**, in cui ogni riga di un elenco riceve un proprio colore da una tavolozza di otto. La Modalità arcobaleno aggiunge la **Modalità disco**, in cui i colori scivolano lentamente verso il successivo, la **Modalità reattiva**, in cui il colore di una riga si vede solo mentre ci punti sopra, è in corso o è selezionata, la **Rotazione dei colori** e la **Tavolozza dei colori** stessa. La bacchetta all'inizio dei colori di accento, **Colore di sistema**, prende il colore proprio del tuo dispositivo: Material You nell'app Android da Android 12 in poi, e il colore di sistema in Firefox e Safari. Compare solo dove il dispositivo ha un colore del genere.

BombVault tiene l'aspetto sul server. Un browser che arriva per la prima volta lo prende. **OLED** e **Colore di sistema** sono l'eccezione: ogni dispositivo li tiene per sé, perché dipendono dal suo schermo. Quando il tuo sistema chiede meno movimento, questo ha sempre la precedenza su **Animazione**.

## Calendario {#schedule}

**Settimana**, **Backup completo** con **Quando vengono eseguiti i backup**, **Recupera i backup mancati dopo l'avvio** e **Comandi prima e dopo**, **Pianificazioni per sezione** con **Copie off-site per sezione**, **Elementi pianificati**, **Ordine** con **Riavvio dopo il backup**, **Snapshot invece dello stop**, **Aggiornamenti dopo il backup** e **Inattività prima del backup**. Vedi [Pianificazioni](schedules.md).

## Destinazioni {#storage-locations}

- **Mostra anche nella barra laterale** fissa la pagina Destinazioni alla barra laterale.
- L'elenco delle tue destinazioni, con **Aggiungi destinazione**. Un clic apre la pagina di una destinazione, vedi [La pagina di una destinazione](offsite-recovery.md#location-page).
- **Credenziali**: le credenziali salvate con la stella della predefinita, vedi [Credenziali](offsite-recovery.md#sign-ins).
- **Destinazioni per sezione**: il flusso 3-2-1, in cui ogni linea è un interruttore, vedi [Valori predefiniti per sezione](offsite-recovery.md#placement-defaults).
- **Repository**: repository creati per singoli elementi, vedi [Destinazioni per elemento](offsite-recovery.md#placement).
- **Per quanto tempo conservare** rimanda alle destinazioni, dove si imposta la conservazione, e **Anteprima** mostra **Cosa rimuoverebbe la prossima esecuzione**, per repository e per elemento, prima che accada. Non cambia nulla e non blocca nulla, quindi risponde anche mentre gira un backup.
- **Copie off-site**: una riga per sezione con **Prova connessione**, **Replica ora** e **Configura** o **Modifica**, vedi [Copie off-site](offsite-recovery.md#off-site-copies).
- **Larghezza di banda off-site**: **Limite upload (KiB/s)** e **Limite download (KiB/s)** per ogni copia. 0 significa illimitato.

La scheda **Streaming**, che abbassa il limite di upload mentre Plex, Jellyfin o Emby fanno streaming, sta nella pagina Destinazioni stessa.

## Ripristino {#restore}

- Un link alla pagina **Ripristino**.
- **Cartelle di ripristino**: la **Cartella di ripristino predefinita** per un ripristino in una cartella, con **Sottocartella per sezione** e un'**Eccezione** per sezione. Vedi [Dove va un ripristino in una cartella](restore.md#folders).
- **Registry dei container**: credenziali per registry privati, come un'immagine `ghcr.io` riservata agli sponsor. Un ripristino le usa per scaricare un'immagine mancante, e un aggiornamento dopo il backup per cercarne una più recente. Le immagini pubbliche si scaricano senza. **Aggiungi registry** offre registry pubblici, self-hosted, quelli di un provider cloud e **Altro registry** per qualsiasi indirizzo.
- **Per le emergenze**: **Scarica il kit di ripristino**, che richiede una password di accesso. Vedi [Kit di ripristino](offsite-recovery.md#recovery-kit).

## Notifiche {#notifications}

**Notifiche** con i riepiloghi, **Segnala i backup in ritardo** e il **Riepilogo settimanale**, e **Canali**, un elenco di canali Unraid, ntfy, Gotify, Discord, Slack, Matrix, email, Apprise, webhook e Healthchecks, ognuno con il suo interruttore, **Notifica** e **Invia test**. Vedi [Notifiche](notifications.md).

## Verifica {#verify}

- **Controlla i backup**: il risultato dell'ultimo `restic check` per sezione, con **Sblocca** e **Pota** sotto **Altro** in ogni riga. **Sblocca** rimuove un lock rimasto e lascia quello che viene ancora rinnovato, e **Pota** applica a richiesta le regole di conservazione.
- **Controlli automatici**: la verifica di ripristino pianificata con il suo **Campione di dati (%)** e i suoi bersagli di prova, **Test di avvio**, che ripristina anche un container in una copia isolata e lo avvia, la **Prova DR off-site pianificata** e **Controlla la protezione dall'eliminazione**, la prova di manomissione pianificata. Vedi [Prove DR](offsite-recovery.md#dr-drills) e [Prova di manomissione](offsite-recovery.md#tamper-test).
- **Anomalie**: rilevamento, sensibilità e quali riscontri inviano un messaggio. Vedi [Anomalie](anomalies.md).

## Sicurezza {#security}

- **Password**: la password di accesso, di almeno 12 caratteri. Cambiarla o cancellarla richiede quella attuale e, con l'autenticazione a due fattori attiva, il codice.
- **Autenticazione a due fattori**: un codice da un'app di autenticazione oltre alla password, con otto codici di ripristino mostrati una sola volta.
- **Passkey**: accedi con una chiave sul telefono, sul portatile o su una chiavetta di sicurezza. Richiedono un vero nome di dominio e un certificato di cui il browser si fida.
- **App Android**: **I telefoni possono associarsi con le dodici parole del gruppo**, cosa che permette a un telefono associato di accedere senza la password.
- **Crittografia**: **Crittografa i backup**, attivo di default.

Diverse cose richiedono una password di accesso, come il kit di ripristino e i depositi della chiave. Vedi [Modello di sicurezza](configuration.md#security-model).

## Istanze {#instances}

- **Mostra anche nella barra laterale** fissa la pagina Istanze alla barra laterale.
- **Più server** attiva le parti di un gruppo: **Istanze**, la pagina con una scheda per ogni membro, **Ricevitore** e **Prelievo**. Vedi [Istanze e ruoli](offsite-recovery.md#roles).
- Un link alla pagina Istanze, con le parti attive.

## Associazione {#pairing}

**Associazione** con **Genera frase** o **Inserisci frase**, le schede dei passaggi, il nome di questa istanza, i dispositivi in attesa di **Approva**, la scheda **Relay** con le sue quattro forme e **Cosa vede il relay**, i membri del gruppo e **Server ZFS senza BombVault** per una destinazione di replica via SSH. Vedi [Associare le istanze](offsite-recovery.md#pairing) e [Invio a un server ZFS](zfs-datasets.md#replica-push).

## Integrazioni {#integrations}

- **Server MCP**: chiavi per assistenti IA, **Scegli un client** e il registro di ogni chiave. Vedi [Server MCP](mcp.md).
- **Token API**: token con nome per script e dashboard. Vedi [Token](api.md#tokens).
- **Servizi**: **Home Assistant** tramite MQTT, **Trova in rete** (mDNS come `bombvault.local`) ed **Esponi /metrics** per Prometheus, con un **Token di scrape (facoltativo)**. Vedi [Home Assistant](api.md#home-assistant) e [Trovare BombVault in rete](api.md#mdns).
- **Widget per dashboard**: un token di sola lettura e un indirizzo per qualsiasi dashboard che mostri un iframe, come Homepage o Heimdall, con solo il registro attività in tempo reale. Vedi [Widget per dashboard](api.md#dashboard-widget).
- **SSH dell'host**: la chiave pubblica di BombVault e **Prova connessione**, per i backup di VM e dataset ZFS e per le notifiche Unraid. Vedi la [guida al backup delle VM](vm-backup-ssh-setup.md).

## App {#apps}

- **App Android**: l'APK per la release che gira sul tuo server con un codice QR, e la scheda su F-Droid. Vedi [App Android](android.md#install).
- **ParleyPort**: il container relay con il suo template Unraid e il comando Docker che lo avvia.
- **BombVault Widget**: un riquadro nella Dashboard di Unraid con il registro attività di BombVault e la prossima esecuzione. Con una connessione SSH all'host la scheda installa e rimuove il plugin da sola. Senza, ti dà l'indirizzo `.plg` da installare in **Plugins, Install Plugin**.

## Sistema {#system}

- **Esporta / importa impostazioni**: tutta la tua configurazione come file JSON portabile, con o senza le credenziali. L'importazione mostra prima **Questo file contiene** e lascia fuori i collegamenti a destinazioni che non esistono più. **Cifra le esportazioni con age** sigilla le esportazioni semplici, lo ZIP della flash e il kit di ripristino per i **Destinatari age** che elenchi. Vedi [Impostazioni portabili](configuration.md#portable-settings-export-and-import).
- **Prestazioni**: **Thread CPU (0 = tutti)**, **Limite dimensione cache (MB, 0 = illimitato)** per la cache di restic sotto `/config`, e **Limite di memoria per backup (MiB, 0 = nessuno)**, un limite morbido che fa rallentare un backup grande invece di farlo terminare per mancanza di memoria.

## Info {#info}

- **Versione**: BombVault, GlimStone, restic e rclone, con **Cerca aggiornamenti** e **Novità**, che conta le modifiche che non hai ancora visto e apre una finestra che le elenca.
- **Aiuto**: **Manuale** apre questa documentazione, e **Segnala bug** scarica uno ZIP ripulito per una issue su GitHub. Richiede una password di accesso.
- **Informazioni su BombVault**: il progetto, la sua licenza e come raggiungere l'autore.
