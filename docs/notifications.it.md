# Notifiche

BombVault può avvisarti quando un backup fallisce, quando riesce, quando uno è in ritardo e, una volta a settimana, come sono andate le cose. Tutto si imposta in **Impostazioni, Notifiche**: cosa invia BombVault oltre ai singoli backup nella scheda **Notifiche**, e dove vanno i messaggi nella scheda **Canali**. Ogni canale decide da sé quando farsi sentire, e anche un controllo di Healthchecks è un canale.

![Impostazioni, Notifiche con il canale Gotify compilato.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Impostazioni, Notifiche con il canale Gotify compilato.](assets/screenshots/notifications/gotify-light.png#only-light)

## Cosa invia un messaggio {#what}

Gli interruttori della scheda **Notifiche** valgono per tutti i canali:

- **Riepiloga le esecuzioni pianificate**: un messaggio per ogni esecuzione pianificata, per esempio *42 su 45 riusciti*, invece di uno per ogni container o VM. Un backup che avvii a mano invia comunque il suo messaggio. Un guest congelato, una conservazione fallita o un budget di crescita superato partono subito anche con i riepiloghi attivi.
- **Notifica all'aggiornamento del container**: un messaggio per ogni container che **Aggiorna i container dopo un backup riuscito** ha portato a un'immagine più recente, così puoi controllare che funzioni ancora.
- **Segnala i backup in ritardo**: una volta al giorno alle 09:00 BombVault controlla se un backup attivato è in ritardo, cioè più vecchio del doppio della sua pianificazione, e invia un messaggio per ogni caso, con il nome dell'elemento rimasto indietro. Il successivo backup riuscito lo cancella.
- **Riepilogo settimanale**: un riepilogo a settimana con il numero di esecuzioni, i nuovi dati di backup, quali sezioni sono in ritardo, se le copie off-site sono aggiornate e gli errori principali. La sua pianificazione è un pulsante come ogni altra pianificazione, e **Disattivato** la spegne. Vedi [Pianificazioni](schedules.md#plans).

Se un canale riceve anche i successi si imposta sul canale stesso, in **Notifica**. Quali anomalie inviano un messaggio si imposta in **Impostazioni, Verifica**, vedi [Anomalie](anomalies.md). Un backup rinviato invia un messaggio quando comincia ad attendere e uno quando l'attesa finisce, non uno per ogni tentativo.

## Canali {#channels}

**Aggiungi canale** in fondo alla scheda **Canali** offre **Notifiche Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **Email (SMTP)**, **Apprise**, **Webhook** e **Healthchecks**. Aggiungine quanti vuoi, anche due dello stesso tipo, per esempio un topic ntfy per gli errori e un altro per tutto. Un nuovo canale parte disattivato, così viene salvato prima che i suoi campi siano compilati.

Ogni canale ha:

- un interruttore proprio, che lo spegne senza perdere i suoi campi.
- **Notifica**: **Mai**, **Solo in caso di errore** oppure **In caso di successo ed errore**. Un nuovo canale parte da **Solo in caso di errore**. Un backup riuscito che però ha lasciato fuori qualcosa arriva anche a un canale solo per gli errori.
- **Nome**, facoltativo, che compare al posto del tipo, comodo quando ne hai due dello stesso tipo.
- **Invia test**, che invia un messaggio di prova attraverso questo canale con i valori così come stanno nel modulo, salvati o no. La riga sotto il canale dice se il servizio l'ha accettato, e se no, perché.
- **Rimuovi canale**, il cestino accanto.

Ogni canale invia per conto suo con un proprio limite di tempo, così un host che non risponde mai non blocca gli altri. Quando un servizio fallisce in un modo che un altro tentativo potrebbe risolvere, il canale riprova fino a tre volte. Un avviso di ritardo, di budget o di anomalia che nessun canale ha preso viene riprovato più tardi, e lo stesso vale per un riepilogo settimanale.

| Canale | Campi | Note |
|---|---|---|
| **Notifiche Unraid** | nessuno | Invia al sistema di notifiche di Unraid, che può inoltrare a Pushover, email o Discord. Passa dalla connessione SSH di **Impostazioni, Integrazioni, SSH dell'host**, quindi quella chiave deve essere autorizzata. Su TrueNAS e altri host resta in silenzio, e il canale dice perché. |
| **ntfy** | **URL dell'argomento**, **Token di accesso (facoltativo)** | L'indirizzo finisce con il tuo topic. Un token serve solo per un topic protetto. |
| **Gotify** | **URL del server**, **Token dell'applicazione**, **Priorità in caso di errore**, **Priorità in caso di successo** | Le priorità vanno da 0 a 10 e partono da 8 per gli errori e da 5 per i successi. |
| **Discord** | **URL del Webhook** | L'indirizzo di un webhook preso dalle impostazioni del canale Discord. |
| **Slack** | **URL del Webhook** | L'indirizzo di un webhook in entrata della tua app Slack. |
| **Matrix** | **URL dell'Homeserver**, **Token di accesso**, **ID della stanza** | L'account dietro il token deve essere nella stanza. |
| **Email (SMTP)** | **Host SMTP**, **Porta**, **Crittografia**, **Nome utente**, **Password**, **Indirizzo mittente**, **Indirizzo destinatario** | **Crittografia** è **STARTTLS**, **TLS (implicit)** oppure **None**. |
| **Apprise** | **URL dell'API Apprise**, **Tag (opzionale)**, **Indirizzi Apprise (facoltativo)** | Punta all'endpoint `/notify/<key>` del tuo container Apprise API per raggiungere i suoi oltre 100 servizi, e i tag scelgono un sottoinsieme delle destinazioni di quella chiave. Oppure elenca indirizzi Apprise, uno per riga, per esempio `tgram://` per Telegram o `pover://` per Pushover, e fai finire l'URL dell'API con `/notify`, senza chiave. |
| **Webhook** | **URL del Webhook**, **Metodo**, **Intestazioni**, **Corpo** | Una richiesta che costruisci tu, vedi [Webhook](#webhook). |
| **Healthchecks** | **URL di ping** | Vedi [Healthchecks](#healthchecks). |

I segreti come i token, la password SMTP e i valori delle intestazioni del webhook vengono salvati cifrati, e il server non li restituisce mai alla pagina. Sotto un segreto salvato il campo dice **Salvato. Lascia il campo vuoto per mantenerlo.** Quando cambi dove invia un canale, BombVault non invia il vecchio segreto al nuovo posto: il campo dice **Il nuovo indirizzo viene salvato appena inserisci di nuovo questo.**, e il nuovo indirizzo viene salvato appena lo digiti.

Le impostazioni di una versione precedente alla lista dei canali vengono riprese al primo avvio, e non si perde nulla. Ogni canale che avevi compilato diventa un elemento a sé con la scelta di **Notifica** che avevi. Un webhook nel formato Discord, Slack, Gotify o ntfy diventa un canale di quel tipo, e un webhook Generic JSON continua a inviare `title`, `message` e `ok` attraverso il suo **Corpo**. L'interruttore Unraid diventa un canale Unraid, e l'URL di Healthchecks con le sue eccezioni un canale Healthchecks.

### Webhook {#webhook}

Un canale **Webhook** invia una richiesta HTTP a qualsiasi indirizzo. **Metodo** è GET, POST o PUT. **Aggiungi intestazione** aggiunge una riga sotto **Intestazioni**, e i valori delle intestazioni vengono conservati come segreti, quindi un'intestazione `Authorization` lì è al sicuro. **Corpo** viene inviato esattamente come lo scrivi, con questi segnaposto compilati:

| Segnaposto | Diventa |
|---|---|
| `%%title%%` | il titolo del messaggio |
| `%%body%%` | il messaggio stesso |
| `%%ok%%` | `true`, oppure `false` per un errore |
| `%%event%%` | cosa è successo, per esempio `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` oppure `critical` |
| `%%importance%%` | il livello che Unraid usa per questo: `normal`, `warning` oppure `alert` |
| `%%link%%` | un link che appartiene al messaggio, vuoto se non ce n'è |
| `%%key%%` | la sezione, per esempio `container` o `VM` |
| `%%time%%` | l'ora in UTC, per esempio `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

I segnaposto funzionano anche nell'indirizzo e nei valori delle intestazioni, e ogni valore viene codificato in base a dove si trova: come JSON quando l'intestazione `Content-Type` indica JSON, e come valore di query nell'indirizzo. Un nuovo webhook parte con quell'intestazione e invia `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Un nome che BombVault non conosce resta come l'hai scritto, così un errore di battitura si vede nel messaggio. Lascia **Corpo** vuoto per non inviarne nessuno.

## Healthchecks {#healthchecks}

Un servizio come [Healthchecks.io](https://healthchecks.io) dà l'allarme quando BombVault smette di farsi sentire. Aggiungi un canale **Healthchecks** e incolla l'**URL di ping** del controllo, e BombVault lo contatta per l'intero ciclo di vita di ogni backup: all'avvio, poi al successo oppure con `/fail`. Questo canale non ha la scelta **Notifica**, perché un controllo ha bisogno dei suoi ping di successo per restare verde. Una sezione senza un posto in cui salvare conta come fallita, così il suo controllo diventa rosso invece di ricevere un successo.

Le sezioni con lo stesso URL condividono un unico controllo. Quando un backup di una di esse fallisce, anche i backup riusciti delle altre contattano `/fail` e nominano l'elemento fallito, così il controllo resta rosso finché quell'elemento non ha di nuovo un backup riuscito o finché non confermi di aver preso visione dell'errore in **Richiede attenzione**.

Una sezione può contattare un indirizzo proprio: **Eccezione** nel canale Healthchecks ne aggiunge uno, e la riga dice allora **Tutte le altre sezioni inviano un ping a questo URL.** **Invia test** scrive una voce nel registro di ogni controllo, che lascia il suo stato com'è.

## Esempio: Gotify sul telefono {#example-gotify}

Hai Gotify su `https://gotify.example.com` e vuoi che gli errori suonino sul telefono mentre i successi arrivano in silenzio.

1. In Gotify apri **Apps**, crea un'applicazione chiamata `BombVault` e copia il suo token.
2. In BombVault apri **Impostazioni, Notifiche** e attiva **Riepiloga le esecuzioni pianificate**, così una notte di 45 backup invia un solo messaggio invece di 45.
3. In **Canali** premi **Aggiungi canale** e scegli **Gotify**.
4. Inserisci `https://gotify.example.com` come **URL del server**. BombVault aggiunge `/message` da solo.
5. Incolla il token in **Token dell'applicazione**.
6. Imposta **Notifica** su **In caso di successo ed errore**. Lascia **Priorità in caso di errore** a 8 e **Priorità in caso di successo** a 5. L'app Android di Gotify considera alta una priorità da 8 in su, quindi solo gli errori fanno rumore.
7. Attiva il canale e premi **Invia test**. La riga sotto il canale dice se Gotify l'ha accettato, e un token rifiutato viene indicato come tale.

La prossima esecuzione pianificata invia un solo riepilogo con priorità 5, oppure 8 se qualcosa al suo interno è fallito.

## Esempio: ntfy senza account {#example-ntfy}

ntfy non richiede un account per un topic pubblico. Scegli un nome di topic difficile da indovinare, perché chiunque lo conosca può leggerlo.

1. In **Canali** premi **Aggiungi canale** e scegli **ntfy**.
2. Inserisci l'indirizzo del topic come **URL dell'argomento**, per esempio `https://ntfy.sh/bombvault-tower-7f3k`, oppure un topic sul tuo server ntfy. Lascia vuoto **Token di accesso (facoltativo)** per un topic pubblico.
3. Iscriviti allo stesso topic nell'app ntfy, attiva il canale e premi **Invia test**.

BombVault invia il messaggio come corpo con il suo titolo nell'intestazione `Title`. Un errore parte con priorità alta e il tag `warning`, così nell'app si nota, e una VM rimasta congelata parte come urgente.

## Quando i messaggi non arrivano {#troubleshooting}

Vedi [Le notifiche non arrivano](troubleshooting.md#notifications-do-not-arrive) in Risoluzione dei problemi.
