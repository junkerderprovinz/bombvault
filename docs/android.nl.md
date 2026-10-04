# Android-app

De Android-app zet elke BombVault-server van je groep op je telefoon. Hij start met een lijst van je servers met bovenaan het activiteitenlogboek van allemaal, dezelfde regels als op het Dashboard, en opent de telefoonweergave van de server die je aantikt. De app maakt zelf geen back-ups.

## De app downloaden {#install}

- **APK:** elke release heeft `bombvault-android.apk` op de releasepagina, en [deze link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) downloadt altijd de nieuwste build. De app heeft Android 10 of nieuwer nodig, en Android vraagt één keer of de app waarmee je het bestand opent apps mag installeren.
- **Google Play:** de app zit in een gesloten test tot hij openbaar kan worden. Google Play toont een app van een nieuw ontwikkelaarsaccount pas nadat minstens 12 testers hem 14 dagen lang geïnstalleerd hebben gehouden. Wil je helpen, word dan lid van de [testersgroep](https://groups.google.com/g/arrowloop-testers), open de [testpagina](https://play.google.com/apps/testing/bombvault.halleluja.design), tik op **Tester worden** en installeer BombVault vanuit Google Play.
- **F-Droid:** de vermelding volgt.

Koppel de app met servers op versie 9.7.0 of nieuwer. Een server op een oudere versie kan in dezelfde groep zitten, maar toont de telefoon als een gewone instantie, en de app leest de activiteit van die server pas na aanmelden.

## Koppelen via QR-code {#pairing}

1. Open op een willekeurige server van je groep **Instellingen, Koppeling** en kies **Frase tonen**. De twaalf woorden verschijnen met een QR-code ernaast.
2. Tik in de app op **QR-code scannen** en richt de telefoon op de code. Je kunt de woorden ook plakken of typen.
3. De app toont de servers van die groep voordat hij iets opslaat. **Alle toevoegen** voegt ze allemaal toe.

De telefoon treedt daarna tot de groep toe als een extra instantie. Hij leest via de groep zonder aanmelden wat er op elke server draait, thuis rechtstreeks en onderweg via de relay. Hoe de groep zelf werkt, staat bij [Koppeling van instanties](offsite-recovery.md#pairing).

!!! note "De interface heeft nog steeds een weg naar de server nodig"
    De serverlijst en het activiteitenlogboek komen via de groep. De interface van een server opent rechtstreeks, dus de telefoon moet het adres van de server kunnen bereiken, thuis of via een VPN.

## Aangemeld op een gekoppelde telefoon {#sign-in}

Een telefoon die met je groep is gekoppeld, opent elke server van die groep al aangemeld. Voordat hij een pagina laadt, vraagt hij die server via de groep om een sessie, en een server geeft er alleen een aan een lid dat een telefoon is. Een server die je via zijn adres hebt toegevoegd, vraagt net als in een browser om het wachtwoord. Wie de twaalf woorden heeft, kan al elke back-up van de groep openen, dus koppelen geeft geen nieuwe rechten.

## Servers buiten een groep {#other-servers}

- **Server toevoegen** neemt het adres waarmee je BombVault in een browser opent, zoals `192.168.1.10:3443`. Zonder `http://` of `https://` ervoor gebruikt de app https.
- Servers die zich op het lokale netwerk bekendmaken, staan onder **In dit netwerk** en openen met één tik. Dat doen ze zolang **Vinden op het netwerk** is ingeschakeld onder Instellingen, Integraties. Een server in een ander netwerk of achter een VPN verschijnt daar niet.
- Een zelfondertekend certificaat wordt één keer vertrouwd op basis van de SHA-256-vingerafdruk. Toont de server later een ander certificaat, dan waarschuwt de app je en opent hij de server pas nadat je het nieuwe certificaat vertrouwt.

## De telefoon op de Instanties-pagina {#instances}

De telefoon krijgt een eigen kaart op de Instanties-pagina van elke server in de groep, gemarkeerd als Android-app en met de naam die je de telefoon hebt gegeven. Hij heeft geen scorecard omdat hij geen back-ups maakt, en **Verwijderen** haalt hem van de pagina.

## Instellingen {#settings}

Het tandwiel naast het plusteken opent de instellingen van de app:

- de taal, en de naam die de telefoon op de Instanties-pagina toont (leeg betekent het model van de telefoon),
- het uiterlijk, dat de eerste server in de lijst volgt tot je zelf iets instelt, en de animaties, die een eigen instelling hebben,
- een rapport om te kopiëren als je een probleem meldt; het bevat geen adres, naam of frase,
- de kaart Over deze app met het privacybeleid,
- **Alle servers verwijderen**, dat elke server uit de app verwijdert en de groep verlaat. Op de servers zelf verandert er niets.

## Downloads en uploads {#files}

Exports, herstelkits, flash-ZIP's en databasedumps komen in de map Downloads van de telefoon terecht, net als vanuit een browser. Een import van instellingen opent de bestandskiezer van de telefoon.

## Op een touchscreen {#touch}

Onder een vinger zweeft niets, dus een bedieningselement wordt gedimd zolang je het vasthoudt, en een knop met een merklogo licht op in zijn merkkleur tot je je vinger optilt. Lang drukken op een knop telt als een trage tik en opent geen linkmenu.

## In plaats daarvan in een browser {#browser}

Chrome en Edge kunnen de webinterface van BombVault installeren als app in een eigen venster, op een telefoon net als op een computer. Er wordt niets in de cache bewaard, dus een update is meteen zichtbaar.

## Privacy {#privacy}

De app heeft geen accounts, geen advertenties en geen analytics, en hij draait niets op de achtergrond. Het [privacybeleid](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) vermeldt wat de app opslaat en wat hij waarheen stuurt.
