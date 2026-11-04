# Notifiche

BombVault può avvisarti quando un backup fallisce, quando riesce, quando uno è in ritardo e, una volta a settimana, come sono andate le cose. Tutto si imposta in **Impostazioni, Notifiche**: cosa fa partire un messaggio nella scheda **Notifiche**, dove vanno i messaggi nella scheda **Canali** e i ping di controllo nella scheda **Healthchecks**.

![Impostazioni, Notifiche con il canale Gotify compilato.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Impostazioni, Notifiche con il canale Gotify compilato.](assets/screenshots/notifications/gotify-light.png#only-light)

## Cosa invia un messaggio {#what}

La scheda **Notifiche** imposta le regole per tutti i canali insieme:

- **Notifica**: **Mai**, **Solo in caso di errore** oppure **In caso di successo ed errore**, per tutti i canali sotto.
- **Riepiloga le esecuzioni pianificate**: un messaggio per ogni esecuzione pianificata, per esempio *42 su 45 riusciti*, invece di uno per ogni container o VM. Un backup che avvii a mano invia comunque il suo messaggio. Un guest congelato, una conservazione fallita o un budget di crescita superato partono subito anche con i riepiloghi attivi.
- **Notifica all'aggiornamento del container**: un messaggio per ogni container che **Aggiorna i container dopo un backup riuscito** ha portato a un'immagine più recente, così puoi controllare che funzioni ancora.
- **Segnala i backup in ritardo**: una volta al giorno alle 09:00 BombVault controlla se un backup attivato è in ritardo, cioè più vecchio del doppio della sua pianificazione, e invia un messaggio per ogni caso. Il successivo backup riuscito lo cancella.
- **Notifiche Unraid**: invia al sistema di notifiche di Unraid, che può inoltrare a Pushover, email o Discord. Passa dalla connessione SSH di **Impostazioni, Integrazioni, SSH dell'host**, quindi quella chiave deve essere autorizzata. Su TrueNAS e altri host resta disattivato.
- **Riepilogo settimanale**: un riepilogo a settimana con il numero di esecuzioni, i nuovi dati di backup, se le copie off-site sono aggiornate e gli errori principali. La sua pianificazione è un pulsante come ogni altra pianificazione, e **Disattivato** la spegne. Vedi [Pianificazioni](schedules.md#plans).

Quali anomalie inviano un messaggio si imposta in **Impostazioni, Verifica**, vedi [Anomalie](anomalies.md). Un backup rinviato invia un messaggio quando comincia ad attendere e uno quando l'attesa finisce, non uno per ogni tentativo.

**Invia test** in fondo alla scheda invia un messaggio di prova attraverso ogni canale attivo, con i valori così come stanno nel modulo.

## Canali {#channels}

Ogni canale ha un interruttore proprio nella scheda **Canali**, e i suoi campi compaiono quando è attivo. Ogni canale invia per conto suo con un proprio limite di tempo, così un host che non risponde mai non blocca gli altri, e un messaggio che nessun canale ha preso viene riprovato.

| Canale | Campi | Note |
|---|---|---|
| **Webhook** | **URL del Webhook**, **Formato del Webhook** | Formati: **Generic JSON**, **Discord**, **Slack**, **Gotify** e **ntfy**. Generic JSON invia `title`, `message` e `ok`. |
| **Apprise** | **URL dell'API Apprise**, **Tag (opzionale)** | Punta all'endpoint `/notify/<key>` del tuo container Apprise API per raggiungere i suoi oltre 100 servizi. I tag scelgono un sottoinsieme delle destinazioni di quella chiave. |
| **Matrix** | **URL dell'Homeserver**, **Token di accesso**, **ID della stanza** | Il token resta sul server e non viene più mostrato. |
| **Gotify** | **URL del server**, **Token dell'applicazione**, **Priorità in caso di errore**, **Priorità in caso di successo** | Le priorità vanno da 0 a 10 e partono da 8 per gli errori e da 5 per i successi. |
| **Email (SMTP)** | **Host SMTP**, **Porta**, **Crittografia**, **Nome utente**, **Password**, **Indirizzo mittente**, **Indirizzo destinatario** | **Crittografia** è **STARTTLS**, **TLS (implicit)** oppure **None**. |

I segreti come il token Matrix, il token dell'applicazione Gotify e la password SMTP vengono salvati cifrati. Sotto un segreto salvato il campo dice **Salvato. Lascia il campo vuoto per mantenerlo.** Quando cambi l'indirizzo del server, BombVault non invia il vecchio segreto al nuovo posto: il campo lo chiede di nuovo, e il nuovo indirizzo viene salvato appena lo digiti.

## Healthchecks {#healthchecks}

Un servizio come [Healthchecks.io](https://healthchecks.io) dà l'allarme quando BombVault smette di farsi sentire. Incolla l'**URL di ping** del controllo, e BombVault lo contatta per l'intero ciclo di vita di ogni backup: all'avvio, poi al successo oppure con `/fail`. Lo fa sempre quando è impostato un URL, qualunque cosa dica **Notifica**, così il controllo resta verde in caso di successo anche con messaggi solo per gli errori.

Una sezione può contattare un indirizzo proprio: **Eccezione** nella scheda ne aggiunge uno, e la riga dice **Tutte le altre sezioni inviano un ping a questo URL.**

## Esempio: Gotify sul telefono {#example-gotify}

Hai Gotify su `https://gotify.example.com` e vuoi che gli errori suonino sul telefono mentre i successi arrivano in silenzio.

1. In Gotify apri **Apps**, crea un'applicazione chiamata `BombVault` e copia il suo token.
2. In BombVault apri **Impostazioni, Notifiche**. In **Notifiche** imposta **Notifica** su **In caso di successo ed errore** e attiva **Riepiloga le esecuzioni pianificate**, così una notte di 45 backup invia un solo messaggio invece di 45.
3. In **Canali** attiva **Gotify**.
4. Inserisci `https://gotify.example.com` come **URL del server**. BombVault aggiunge `/message` da solo.
5. Incolla il token in **Token dell'applicazione**.
6. Lascia **Priorità in caso di errore** a 8 e **Priorità in caso di successo** a 5. L'app Android di Gotify considera alta una priorità da 8 in su, quindi solo gli errori fanno rumore.
7. Premi **Invia test**. La riga sotto la scheda dice se Gotify l'ha accettato, e un token rifiutato viene indicato come tale.

La prossima esecuzione pianificata invia un solo riepilogo con priorità 5, oppure 8 se qualcosa al suo interno è fallito.

## Esempio: ntfy come webhook {#example-ntfy}

ntfy non richiede un account per un topic pubblico. Scegli un nome di topic difficile da indovinare, perché chiunque lo conosca può leggerlo.

1. In **Canali** attiva **Webhook**.
2. Inserisci l'indirizzo del topic come **URL del Webhook**, per esempio `https://ntfy.sh/bombvault-tower-7f3k`, oppure l'indirizzo del tuo server ntfy.
3. Imposta **Formato del Webhook** su **ntfy**.
4. Iscriviti allo stesso topic nell'app ntfy e premi **Invia test**.

BombVault invia il messaggio come corpo con il suo titolo in `X-Title`. Un errore parte con priorità alta e il tag `warning`, così nell'app si nota.

## Quando i messaggi non arrivano {#troubleshooting}

Vedi [Le notifiche non arrivano](troubleshooting.md#notifications-do-not-arrive) in Risoluzione dei problemi.
