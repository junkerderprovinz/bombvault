# App Android

L'app Android porta sul telefono tutti i server BombVault del tuo gruppo. Si apre su un elenco dei tuoi server con in cima il registro attività di tutti quanti, le stesse righe che mostra la Dashboard, e apre la vista per telefono del server che tocchi. L'app non esegue nessun backup da sola.

## Ottenere l'app {#install}

- **Impostazioni, App:** la scheda dell'app Android offre l'APK per la release che esegue il tuo server, con un codice QR da inquadrare dal telefono.
- **APK:** ogni release ha `bombvault-android.apk` nella sua pagina, e [questo link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) scarica sempre l'ultima build. Richiede Android 10 o successivo, e Android chiede una volta se l'app con cui apri il file può installare app.
- **Google Play:** l'app è in un test chiuso finché non potrà diventare pubblica. Google Play pubblica un'app di un nuovo account sviluppatore solo dopo che almeno 12 tester l'hanno tenuta installata per 14 giorni. Per dare una mano, unisciti al [gruppo dei tester](https://groups.google.com/g/arrowloop-testers), apri la [pagina del test](https://play.google.com/apps/testing/bombvault.halleluja.design), tocca **Diventa un tester** e installa BombVault da Google Play.
- **F-Droid:** la scheda arriverà in seguito.

Associa l'app a server con versione 9.7.0 o successiva. Un server con una versione più vecchia può far parte dello stesso gruppo, ma mostra il telefono come una semplice istanza, e l'app legge l'attività di quel server solo dopo un accesso.

## Associazione tramite codice QR {#pairing}

1. Su un qualsiasi server del tuo gruppo, apri **Impostazioni, Associazione** e scegli **Mostra frase**. Le dodici parole compaiono con un codice QR accanto.
2. Nell'app, tocca **Inquadra codice QR** e punta il telefono verso il codice. Puoi anche incollare o digitare le parole.
3. Prima di salvare qualsiasi cosa, l'app elenca i server di quel gruppo. **Aggiungi tutte e N** li aggiunge tutti.

Da quel momento il telefono entra nel gruppo come un'altra istanza. Legge cosa gira su ogni server tramite il gruppo senza accesso, direttamente a casa e tramite il relay fuori casa. Il funzionamento del gruppo in sé è descritto in [Associazione delle istanze](offsite-recovery.md#pairing).

!!! note "L'interfaccia ha comunque bisogno di una strada verso il server"
    L'elenco dei server e il registro attività arrivano tramite il gruppo. L'interfaccia di un server si apre direttamente, quindi il telefono deve raggiungere l'indirizzo del server, a casa o tramite una VPN.

## Accesso già eseguito su un telefono associato {#sign-in}

Un telefono associato al tuo gruppo apre ciascuno dei suoi server con l'accesso già eseguito. Prima di caricare una pagina chiede a quel server una sessione tramite il gruppo, e un server la concede solo a un membro che è un telefono. Un server aggiunto tramite il suo indirizzo chiede la password, come in un browser. Chiunque abbia le dodici parole può già aprire ogni backup del gruppo, quindi l'associazione non concede nulla di nuovo.

## Server fuori da un gruppo {#other-servers}

- **Aggiungi server** accetta l'indirizzo con cui apri BombVault in un browser, per esempio `192.168.1.10:3443`. Senza `http://` o `https://` davanti, l'app usa https.
- I server che si annunciano sulla rete locale compaiono in **In questa rete** e si aprono con un tocco. Lo fanno finché **Trova in rete** è attivo in Impostazioni, Integrazioni. Un server in un'altra rete o dietro una VPN lì non compare.
- Un certificato autofirmato viene accettato una volta tramite la sua impronta SHA-256. Se in seguito il server ne mostra uno diverso, l'app ti avvisa e lo apre solo dopo che hai accettato il nuovo certificato.

## Il telefono nella pagina Istanze {#instances}

Il telefono riceve una scheda propria nella pagina Istanze di ogni server del gruppo, contrassegnata come app Android e con il nome che hai dato al telefono. Non ha una scorecard perché non esegue backup, e **Rimuovi** la toglie dalla pagina.

## Impostazioni {#settings}

L'ingranaggio accanto al pulsante + apre le impostazioni dell'app:

- la lingua e il nome che il telefono mostra nella pagina Istanze (vuoto significa il modello del telefono),
- l'aspetto, che segue il primo server dell'elenco finché non imposti il tuo, e le animazioni, che hanno un'impostazione propria,
- un report da copiare quando segnali un problema; non contiene indirizzi, nomi né la frase,
- la scheda **Informazioni** con l'informativa sulla privacy,
- **Rimuovi tutti i server**, che toglie ogni server dall'app e lascia il gruppo. Sui server stessi non cambia nulla.

## Download e caricamenti {#files}

Esportazioni, kit di ripristino, ZIP del flash e dump dei database finiscono nella cartella Download del telefono, come da un browser. Un'importazione delle impostazioni apre il selettore di file del telefono.

## Su un touch screen {#touch}

Sotto un dito non c'è hover, quindi un controllo si scurisce mentre lo tieni premuto, e un pulsante con il logo di un marchio si accende nel colore di quel marchio finché non sollevi il dito. Una pressione prolungata su un pulsante conta come un tocco lento e non apre nessun menu dei link.

## Oppure in un browser {#browser}

Chrome ed Edge possono installare l'interfaccia web di BombVault come app in una finestra propria, sia su un telefono sia su un computer. Non viene memorizzato nulla nella cache, quindi un aggiornamento si vede subito.

## Privacy {#privacy}

L'app non ha account, pubblicità né analisi statistiche, e non esegue nulla in background. La sua [informativa sulla privacy](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) elenca cosa memorizza e cosa invia, e dove.
